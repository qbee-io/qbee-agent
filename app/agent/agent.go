// Copyright 2023 qbee.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.qbee.io/agent/app/api"
	"go.qbee.io/agent/app/attributes"
	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/inventory"
	"go.qbee.io/agent/app/log"
	"go.qbee.io/agent/app/metrics"
	"go.qbee.io/agent/app/remoteaccess"
	"go.qbee.io/agent/app/utils"
)

// Agent is the main agent structure.
// It contains all the services and the main control loop.
type Agent struct {
	cfg *Config

	privateKey  *ecdsa.PrivateKey
	certificate *x509.Certificate
	caCertPool  *x509.CertPool

	api        *api.Client
	lock       sync.Mutex
	loopTicker *time.Ticker
	update     chan bool
	stop       chan bool
	reboot     chan bool

	// configLock is held during configuration execution and the inventories collection which follows it.
	// It's separate from lock (held during check-in), so a long-running execution doesn't stop the device
	// from checking in. Code which needs both must lock configLock first, to avoid deadlocks.
	configLock sync.Mutex

	// rebootPending is set when a reboot is requested and cleared if the reboot fails.
	// While set, no further reboot is requested and no new configuration execution is started.
	rebootPending atomic.Bool

	Inventory     *inventory.Service
	Configuration *configuration.Service
	Metrics       *metrics.Service
	Attributes    *attributes.Service
	remoteAccess  *remoteaccess.Service
	// disableRemoteAccess is used to disable remote access for RunOnce
	disableRemoteAccess bool

	// The functions below implement parts of the agent's work. NewWithoutCredentials sets them to their default
	// implementations, tests replace them, so the agent can be tested without side effects on the system.

	// rebootCommand schedules the system reboot.
	rebootCommand func(ctx context.Context) ([]byte, error)

	// executeConfig executes the configuration.
	executeConfig func(ctx context.Context, configData *configuration.CommittedConfig) error

	// collectInventories collects and delivers inventories.
	collectInventories func(ctx context.Context) error
}

// Run the main control loop of the agent.
func (agent *Agent) Run(ctx context.Context) error {
	// look for run interval changes
	intervalChange := agent.Configuration.RunIntervalChangedNotifier()

	// catch SIGINT and SIGTERM to gracefully shut down the agent
	stopSignalCh := make(chan os.Signal, 1)
	signal.Notify(stopSignalCh, os.Interrupt, syscall.SIGTERM)

	// use SIGUSR1 to force processing outside normal schedule
	updateSignalCh := make(chan os.Signal, 1)
	signal.Notify(updateSignalCh, syscall.SIGUSR1)

	// ticker won't trigger the first run immediately, so let's do that ourselves
	go agent.RunOnce(ctx, FullRun)

	log.Infof("starting agent scheduler")
	for {
		select {
		case <-agent.stop:
			log.Infof("stopping the agent")

			// stop the remote access service (if running)
			if err := agent.remoteAccess.Stop(); err != nil {
				log.Errorf("failed to stop remote access: %s", err)
			}

			// let all the processing finish
			agent.Wait()

			// and return
			return nil
		case <-agent.reboot:
			// the reboot waits for configuration execution to finish, so don't block this loop meanwhile
			go agent.RebootSystem(ctx)

		case <-stopSignalCh:
			log.Debugf("received interrupt signal")

			agent.Stop()

		case newInterval := <-intervalChange:
			log.Debugf("run interval updated: %s", newInterval)
			agent.loopTicker.Reset(newInterval)

		case <-agent.loopTicker.C:
			go agent.RunOnce(ctx, FullRun)

		case <-updateSignalCh:
			log.Debugf("received update signal")

			agent.update <- true

		case <-agent.update:
			// reset the ticker, so we don't run the update twice (scheduled and manually triggered)
			agent.loopTicker.Reset(agent.Configuration.RunInterval())

			go agent.RunOnce(ctx, FullRunFresh)
		}
	}
}

// Stop the agent.
//
// It only requests the stop: the Run loop receives it, waits for ongoing processing to finish and returns.
// Stop never blocks, so it's safe to call from anywhere, any number of times:
//   - the stop channel has room for one request. If a request is already queued, sending another would
//     block until the Run loop receives the first one. The select's default case skips the send instead,
//     as the queued request will stop the agent anyway.
//   - the Run loop itself calls Stop (on SIGTERM), and it's the only receiver of the stop channel.
//     A blocking send there would wait for the loop itself, which would never happen (a deadlock).
//   - after the Run loop returned, nothing receives from the stop channel anymore.
func (agent *Agent) Stop() {
	select {
	case agent.stop <- true:
	default:
	}
}

// RunOnceMode defines the mode of the RunOnce function.
type RunOnceMode int

const (
	// FullRun performs a full run of the agent routines.
	FullRun RunOnceMode = iota

	// FullRunFresh performs a full run of the agent routines based on a non-cached configuration.
	FullRunFresh

	// QuickRun performs only essential reporting required by the bootstrap process.
	QuickRun
)

// RunOnce performs a single run of the agent routines.
//
// The check-in (configuration retrieval, remote access and metrics) is done synchronously.
// Configuration execution and inventories collection are started in the background,
// unless a previous configuration execution is still in progress.
func (agent *Agent) RunOnce(ctx context.Context, mode RunOnceMode) {
	// lock the agent to prevent concurrent runs, but if the lock is already held, skip the run
	// we do that, so we don't 'accumulate' runs if the agent is slow
	if !agent.lock.TryLock() {
		return
	}

	defer func() {
		agent.lock.Unlock()

		agent.rebootIfScheduled()

		if err := recover(); err != nil {
			log.Errorf("fatal agent error: %v", err)
		}
	}()

	log.Debugf("starting agent run")

	requestFreshConfig := mode == FullRunFresh
	configData, err := agent.Configuration.Get(ctx, requestFreshConfig)
	if err != nil {
		log.Errorf("failed to get device configuration from the device hub: %v", err)
		return
	}

	agent.Configuration.UpdateSettings(configData)

	// Check if we have enough goroutine budget to run the agent, if not, skip the run.
	// Do this after we have updated the configuration, so that the device is online.
	if exhausted, err := agent.Configuration.ReportExhaustedGoroutineBudget(ctx); exhausted {
		if err != nil {
			log.Errorf("failed to report goroutine budget: %v", err)
		}
		return
	}

	switch mode {
	case FullRun, FullRunFresh:
		agent.do(ctx, "remote-access", agent.doRemoteAccess(configData))
		agent.startConfigRun(ctx, configData)
		agent.do(ctx, "metrics", agent.doMetrics)
	case QuickRun:
		agent.do(ctx, "system-inventory", agent.doSystemInventory)
	}
}

// startConfigRun executes the configuration and collects inventories in the background.
// If a configuration execution is already in progress, or a reboot is pending, it does nothing.
func (agent *Agent) startConfigRun(ctx context.Context, configData *configuration.CommittedConfig) {
	// acquire the lock before starting the goroutine, so Wait() can't miss the execution
	if !agent.configLock.TryLock() {
		log.Infof("configuration execution still in progress - skipping")
		return
	}

	// don't start work which the pending reboot would interrupt
	if agent.rebootPending.Load() {
		agent.configLock.Unlock()
		log.Infof("system reboot pending - skipping configuration execution")
		return
	}

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf("fatal configuration execution error: %v", err)
			}

			// request the reboot (if scheduled) before releasing the lock, so no new execution starts in between
			agent.rebootIfScheduled()
			agent.configLock.Unlock()
		}()

		// Reset metrics monitor state here rather than during check-in, so it can't race with
		// an execution of an older configuration, which could restore the state after the reset.
		agent.Configuration.UpdateMetricsMonitorState(configData)

		agent.do(ctx, "config", agent.doConfig(configData))

		// inventories reflect the result of the configuration execution
		// and may conflict with it (e.g. package manager locks), so collect them afterward
		agent.do(ctx, "inventories", agent.collectInventories)
	}()
}

// rebootIfScheduled requests a system reboot from the Run loop, if one was scheduled and isn't pending already.
//
// It's called after every check-in and every configuration execution, as both can schedule a reboot
// (e.g. connectivity watchdog during check-in, a bundle during configuration execution). They run concurrently,
// so both may detect the same scheduled reboot. Only the first one requests it.
func (agent *Agent) rebootIfScheduled() {
	if !agent.Configuration.ShouldReboot() {
		return
	}

	// CompareAndSwap sets rebootPending and returns true only if it wasn't set already. This check-and-set is
	// a single atomic operation, so two concurrent callers can't both succeed.
	// rebootPending stays set until RebootSystem fails, so the reboot is requested once, not after every run.
	// After a failure, it's cleared, so the next run requests the reboot again.
	if !agent.rebootPending.CompareAndSwap(false, true) {
		return
	}

	log.Warnf("reboot condition detected, scheduling system reboot")

	// This never blocks: the channel has room for one request, and it's empty whenever rebootPending
	// is not set, because the Run loop has already received the previous request.
	agent.reboot <- true
}

// do execute a named function and report on errors.
func (agent *Agent) do(ctx context.Context, name string, fn func(ctx context.Context) error) {
	log.Debugf("starting %s", name)

	if err := fn(ctx); err != nil {
		log.Errorf("failed to do %s: %v", name, err)
	}

	log.Debugf("stopping %s", name)
}

// doMetrics collects system metrics - if enabled - and delivers them to the device hub API.
func (agent *Agent) doMetrics(ctx context.Context) error {
	if !agent.Configuration.MetricsEnabled() {
		return nil
	}

	if err := agent.Metrics.Send(ctx, agent.Metrics.Collect(ctx)); err != nil {
		return fmt.Errorf("failed to send metrics: %w", err)
	}

	return nil
}

// doConfig returns a function which executes the committed configuration.
func (agent *Agent) doConfig(configData *configuration.CommittedConfig) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if err := agent.executeConfig(ctx, configData); err != nil {
			return fmt.Errorf("failed to apply configuration: %w", err)
		}
		return nil
	}
}

// doRemoteAccess maintains remote access for the agent - if enabled.
func (agent *Agent) doRemoteAccess(cfg *configuration.CommittedConfig) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// do not run remote access if it is disabled in local configuration
		if agent.disableRemoteAccess {
			return nil
		}

		// capture the current reporting settings and commit ID for use in the connection error reporter.
		// We capture it here to avoid any agent run changing the reporting settings or commit ID while
		// the connection error reporter is using them.
		reportingEnabled := agent.Configuration.ReportingEnabled()
		commitID := agent.Configuration.CurrentCommitID()

		// initialize the connection error reporter with the captured settings
		connectionErrorReporter := func(ctx context.Context, connErr error) {
			err := agent.Configuration.ReportRemoteAccessError(ctx, connErr, reportingEnabled, commitID)
			if err != nil {
				log.Debugf("failed to deliver remote access error report: %v", err)
			}
		}

		// the reporter runs outside the agent run, so it gets a snapshot of the reporting settings
		agent.remoteAccess.SetConnectionErrorReporter(connectionErrorReporter)

		return agent.remoteAccess.UpdateState(ctx, cfg.EdgeURL, cfg.BundleData.Settings.EnableRemoteConsole)
	}
}

// RebootSystem reboots the host system.
// It waits for the configuration execution and check-in in progress to finish, so the reboot doesn't interrupt them.
// No new configuration execution starts meanwhile, as rebootPending is set.
func (agent *Agent) RebootSystem(ctx context.Context) {
	agent.Wait()

	output, err := agent.rebootCommand(ctx)
	if err != nil {
		log.Errorf("scheduling system reboot failed: %v", err)

		// let the next run request the reboot again
		agent.rebootPending.Store(false)
		return
	}

	log.Infof("scheduling system reboot completed: %s", output)

	agent.Stop()
}

// runSystemRebootCommand schedules the system reboot using the system's reboot command.
func runSystemRebootCommand(ctx context.Context) ([]byte, error) {
	rebootCmd, err := utils.RebootCommand()
	if err != nil {
		return nil, fmt.Errorf("reboot command not found: %w", err)
	}

	return utils.RunPrivilegedCommand(ctx, rebootCmd)
}

// Wait for the agent to finish any ongoing processing to finish.
func (agent *Agent) Wait() {
	agent.configLock.Lock()
	defer agent.configLock.Unlock()

	agent.lock.Lock()
	defer agent.lock.Unlock()
}

// IsTPMEnabled returns true if the agent uses TPM to seal its private key.
func (agent *Agent) IsTPMEnabled() bool {
	return agent.cfg.TPMDevice != ""
}

// NewWithoutCredentials returns a new instance of Agent without loaded credentials.
func NewWithoutCredentials(cfg *Config) (*Agent, error) {
	if err := prepareDirectories(cfg.Directory, cfg.StateDirectory); err != nil {
		return nil, err
	}

	agent := &Agent{
		cfg:    cfg,
		update: make(chan bool, 1),
		stop:   make(chan bool, 1),
		reboot: make(chan bool, 1),
	}

	var proxy *api.Proxy
	if cfg.ProxyServer != "" {
		proxy = &api.Proxy{
			Host:     cfg.ProxyServer,
			Port:     cfg.ProxyPort,
			User:     cfg.ProxyUser,
			Password: cfg.ProxyPassword,
		}

		if err := api.UseProxy(proxy); err != nil {
			return nil, err
		}
	}

	if err := agent.loadCACertificatesPool(cfg.CACert); err != nil {
		return nil, err
	}

	agent.api = api.NewClient(cfg.DeviceHubServer, cfg.DeviceHubPort).
		WithTLSConfig(&tls.Config{RootCAs: agent.caCertPool})

	appDir := filepath.Join(cfg.StateDirectory, appWorkingDirectory)
	cacheDir := filepath.Join(appDir, cacheDirectory)

	agent.Inventory = inventory.New(agent.api)
	agent.Metrics = metrics.New(agent.api)
	agent.Attributes = attributes.New(agent.api)
	agent.Configuration = configuration.New(agent.api, appDir, cacheDir).WithURLSigner(agent).WithMetricsService(agent.Metrics)
	agent.rebootCommand = runSystemRebootCommand
	agent.executeConfig = agent.Configuration.Execute
	agent.collectInventories = agent.doInventories
	agent.remoteAccess = remoteaccess.New().
		WithConfigReloadNotifier(agent.update)
	agent.loopTicker = time.NewTicker(agent.Configuration.RunInterval())
	agent.disableRemoteAccess = cfg.DisableRemoteAccess

	return agent, nil
}

// New returns a new instance of Agent with loaded credentials.
func New(cfg *Config) (*Agent, error) {
	agent, err := NewWithoutCredentials(cfg)
	if err != nil {
		return nil, err
	}

	if err = agent.loadPrivateKey(); err != nil {
		return nil, err
	}

	if err = agent.loadCertificate(); err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		RootCAs: agent.caCertPool,
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{agent.certificate.Raw},
				PrivateKey:  agent.privateKey,
			},
		},
	}

	agent.api.WithTLSConfig(tlsConfig)
	agent.remoteAccess.WithTLSConfig(tlsConfig)

	return agent, nil
}
