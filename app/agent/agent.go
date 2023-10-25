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
	"crypto/x509"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/configuration"
	"github.com/qbee-io/qbee-agent/app/inventory"
	"github.com/qbee-io/qbee-agent/app/log"
	"github.com/qbee-io/qbee-agent/app/metrics"
	"github.com/qbee-io/qbee-agent/app/remoteaccess"
	"github.com/qbee-io/qbee-agent/app/utils"
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
	stop       chan bool
	update     chan bool
	reboot     chan bool

	Inventory     *inventory.Service
	Configuration *configuration.Service
	Metrics       *metrics.Service
	remoteAccess  *remoteaccess.Service
	// disableRemoteAccess is used to disable remote access for RunOnce
	disableRemoteAccess bool
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

	// remote access state change notification
	remoteAccessStateChange := agent.remoteAccess.GetNotificationChannel()

	// ticker won't trigger the first run immediately, so let's do that ourselves
	go agent.RunOnce(ctx, FullRun)

	log.Infof("starting agent scheduler")
	for {
		select {
		case <-agent.stop:
			log.Infof("stopping the agent")

			// let all the processing finish
			agent.Wait()

			if agent.disableRemoteAccess {
				return nil
			}

			// stop the remote access service
			if err := agent.remoteAccess.Stop(); err != nil {
				log.Errorf("failed to stop remote access: %s", err)
			}

			// and return
			return nil

		case <-stopSignalCh:
			log.Debugf("received interrupt signal")

			agent.stop <- true

		case newInterval := <-intervalChange:
			log.Debugf("run interval updated: %s", newInterval)
			agent.loopTicker.Reset(newInterval)

		case <-agent.loopTicker.C:
			go agent.RunOnce(ctx, FullRun)

		case <-remoteAccessStateChange:
			log.Debugf("remote access state changed, sending system inventory")
			agent.do(ctx, "inventories-on-remote-access", agent.doInventoriesSimple)

		case <-updateSignalCh:
			log.Debugf("received update signal")
			// reset the ticker, so we don't run the update twice (scheduled and manually triggered)
			agent.loopTicker.Reset(agent.Configuration.RunInterval())

			go agent.RunOnce(ctx, FullRun)

		case <-agent.update:
			log.Infof("starting agent update")

			// wait for all the processing to finish
			agent.Wait()

			// and run the update (this will block until the update is finished)
			if err := agent.updateAgent(ctx); err != nil {
				log.Errorf("failed to update the agent: %v", err)
			}

		case <-agent.reboot:
			agent.RebootSystem(ctx)
		}
	}
}

// RunOnceMode defines the mode of the RunOnce function.
type RunOnceMode int

const (
	// FullRun performs a full run of the agent routines.
	FullRun RunOnceMode = iota

	// QuickRun performs only essential reporting required by the bootstrap process.
	QuickRun
)

// RunOnce performs a single run of the agent routines.
func (agent *Agent) RunOnce(ctx context.Context, mode RunOnceMode) {
	agent.lock.Lock()
	defer func() {
		agent.lock.Unlock()

		if err := recover(); err != nil {
			log.Errorf("fatal agent error: %v", err)
		}
	}()

	log.Debugf("starting agent run")

	configData, err := agent.Configuration.Get(ctx)
	if err != nil {
		log.Errorf("failed to get device configuration from the device hub: %v", err)
		return
	}

	agent.Configuration.UpdateSettings(configData)

	if mode == FullRun {
		agent.do(ctx, "check-in", agent.doCheckIn)
		agent.do(ctx, "metrics", agent.doMetrics)
		agent.do(ctx, "remote-access", agent.doRemoteAccess)
		agent.do(ctx, "config", agent.doConfig(configData))
		agent.do(ctx, "inventories", agent.doInventories)
	} else {
		agent.do(ctx, "system-inventory", agent.doSystemInventory)
	}
}

// do execute a named function and report on errors.
func (agent *Agent) do(ctx context.Context, name string, fn func(ctx context.Context) error) {
	log.Debugf("starting %s", name)

	if err := fn(ctx); err != nil {
		log.Errorf("failed to do %s: %v", name, err)
	}

	log.Debugf("stopping %s", name)
}

// doCheckIn sends a heartbeat to the device hub and checks for updates.
func (agent *Agent) doCheckIn(ctx context.Context) error {
	response, err := agent.checkIn(ctx, agent.cfg.AutoUpdate)
	if err != nil {
		return fmt.Errorf("failed to check-in: %w", err)
	}

	if agent.cfg.AutoUpdate && response.UpdateAvailable() {
		agent.update <- true
	}

	return nil
}

// doMetrics collects system metrics - if enabled - and delivers them to the device hub API.
func (agent *Agent) doMetrics(ctx context.Context) error {
	if !agent.Configuration.MetricsEnabled() {
		return nil
	}

	if err := agent.Metrics.Send(ctx, agent.Metrics.Collect()); err != nil {
		return fmt.Errorf("failed to send metrics: %w", err)
	}

	return nil
}

// doConfig returns a function which executes the committed configuration.
func (agent *Agent) doConfig(configData *configuration.CommittedConfig) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if err := agent.Configuration.Execute(ctx, configData); err != nil {
			return fmt.Errorf("failed to apply configuration: %w", err)
		}

		// Send reboot command if it is scheduled
		if agent.Configuration.ShouldReboot() {
			log.Warnf("reboot condition detected, scheduling system reboot")
			agent.reboot <- true
		}

		return nil
	}
}

// doRemoteAccess maintains remote access for the agent - if enabled.
func (agent *Agent) doRemoteAccess(ctx context.Context) error {
	// do not run remote access if it is disabled
	if agent.disableRemoteAccess {
		return nil
	}
	return agent.remoteAccess.UpdateState(ctx, agent.Configuration.RemoteAccessEnabled())
}

const shutdownBinPath = "/sbin/shutdown"

// RebootSystem reboots the host system.
func (agent *Agent) RebootSystem(ctx context.Context) {
	if _, err := exec.LookPath(shutdownBinPath); err != nil {
		log.Errorf("cannot reboot: %s - %v", shutdownBinPath, err)
		return
	}
	log.Debugf("waiting for agent routines to finish")
	agent.Wait()

	if output, err := utils.RunCommand(ctx, []string{"/sbin/shutdown", "-r", "+1"}); err != nil {
		log.Errorf("scheduling system reboot failed: %v", err)
	} else {
		log.Infof("scheduling system reboot completed: %s", output)
	}

	agent.stop <- true
}

// Wait for the agent to finish any ongoing processing to finish.
func (agent *Agent) Wait() {
	agent.lock.Lock()
	agent.lock.Unlock()
}

// NewWithoutCredentials returns a new instance of Agent without loaded credentials.
func NewWithoutCredentials(cfg *Config) (*Agent, error) {
	if err := prepareDirectories(cfg.Directory, cfg.StateDirectory); err != nil {
		return nil, err
	}

	agent := &Agent{
		cfg:    cfg,
		stop:   make(chan bool, 1),
		update: make(chan bool, 1),
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

	if err := agent.loadCACertificatesPool(); err != nil {
		return nil, err
	}

	agent.api = api.NewClient(cfg.DeviceHubServer, cfg.DeviceHubPort, agent.caCertPool)

	appDir := filepath.Join(cfg.StateDirectory, appWorkingDirectory)
	binDir := filepath.Join(appDir, binDirectory)
	cacheDir := filepath.Join(appDir, cacheDirectory)
	certDir := filepath.Join(cfg.Directory, credentialsDirectory)

	agent.Inventory = inventory.New(agent.api)
	agent.Configuration = configuration.New(agent.api, appDir, cacheDir)
	agent.Metrics = metrics.New(agent.api)
	agent.remoteAccess = remoteaccess.New(agent.api, cfg.VPNServer, certDir, binDir, proxy)
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

	agent.api.UseTLSCredentials(agent.privateKey, agent.certificate)

	return agent, nil
}
