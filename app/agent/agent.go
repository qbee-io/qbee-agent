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
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"go.qbee.io/agent/app/api"
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
			agent.RebootSystem(ctx)

		case <-stopSignalCh:
			log.Debugf("received interrupt signal")

			agent.stop <- true

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

			go agent.RunOnce(ctx, FullRun)
		}
	}
}

// Stop the agent.
func (agent *Agent) Stop() {
	agent.stop <- true
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
	// lock the agent to prevent concurrent runs, but if the lock is already held, skip the run
	// we do that, so we don't 'accumulate' runs if the agent is slow
	if !agent.lock.TryLock() {
		return
	}

	defer func() {
		agent.lock.Unlock()

		if agent.Configuration.ShouldReboot() {
			log.Warnf("reboot condition detected, scheduling system reboot")
			agent.reboot <- true
		}

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
	agent.Configuration.UpdateMetricsMonitorState(configData)

	if mode == FullRun {
		agent.do(ctx, "check-in", agent.checkIn)
		agent.do(ctx, "remote-access", agent.doRemoteAccess(configData))
		agent.do(ctx, "config", agent.doConfig(configData))
		agent.do(ctx, "metrics", agent.doMetrics)
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

		return agent.remoteAccess.UpdateState(ctx, cfg.EdgeURL, cfg.BundleData.Settings.EnableRemoteConsole)
	}
}

// RebootSystem reboots the host system.
func (agent *Agent) RebootSystem(ctx context.Context) {

	rebootCmd, err := utils.RebootCommand()

	if err != nil {
		log.Errorf("reboot command not found: %v", err)
		return
	}

	if output, err := utils.RunCommand(ctx, rebootCmd); err != nil {
		log.Errorf("scheduling system reboot failed: %v", err)
	} else {
		log.Infof("scheduling system reboot completed: %s", output)
	}

	agent.stop <- true
}

// Wait for the agent to finish any ongoing processing to finish.
func (agent *Agent) Wait() {
	agent.lock.Lock()
	defer agent.lock.Unlock()
}

// IsTPMEnabled returns true if the agent uses TPM to seal its private key.
func (agent *Agent) IsTPMEnabled() bool {
	return agent.cfg.TPMDevice != ""
}

// NewWithoutCredentials returns a new instance of Agent without loaded credentials.
func NewWithoutCredentials(cfg *Config) (*Agent, error) {
	if err := prepareConfigDirectories(cfg.Directory); err != nil {
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
	agent.Configuration = configuration.New(agent.api, appDir, cacheDir).
		WithURLSigner(agent).
		WithMetricsService(agent.Metrics).
		WithElevationCommand(cfg.ElevationCommand)
	agent.remoteAccess = remoteaccess.New().
		WithConfigReloadNotifier(agent.update)
	agent.loopTicker = time.NewTicker(agent.Configuration.RunInterval())
	agent.disableRemoteAccess = cfg.DisableRemoteAccess

	return agent, nil
}

type execUser struct {
	uid      int
	gid      int
	groupIDs []int
	userInfo *user.User
}

// setExecUser changes the user and group of the running process.
func resolveExecUser(cfg *Config) (execUser, error) {
	// we can't change user if we are not root
	if os.Geteuid() != 0 {
		return execUser{uid: os.Geteuid(), gid: os.Getegid()}, nil
	}

	// no user specified, nothing to do
	if cfg.ExecUser == "" {
		return execUser{uid: os.Geteuid(), gid: os.Getegid()}, nil
	}

	// lookup user
	userInfo, err := user.Lookup(cfg.ExecUser)
	if err != nil {
		return execUser{}, err
	}

	// parse uid and gid
	uid, err := strconv.Atoi(userInfo.Uid)
	if err != nil {
		return execUser{}, err
	}

	// parse gid
	gid, err := strconv.Atoi(userInfo.Gid)
	if err != nil {
		return execUser{}, err
	}

	// we are already root user, nothing to do
	if uid == 0 {
		return execUser{uid: os.Geteuid(), gid: os.Getegid()}, nil
	}

	// get all user groups
	groupIds, err := userInfo.GroupIds()
	if err != nil {
		return execUser{}, err
	}

	var gids []int
	for _, gidStr := range groupIds {
		// skip primary gid, already included
		if gidStr == userInfo.Gid {
			continue
		}
		gid, err := strconv.Atoi(gidStr)
		if err != nil {
			return execUser{}, err
		}
		gids = append(gids, gid)
	}

	return execUser{uid: uid, gid: gid, groupIDs: gids, userInfo: userInfo}, nil
}

// New returns a new instance of Agent with loaded credentials.
func New(cfg *Config) (*Agent, error) {
	agent, err := NewWithoutCredentials(cfg)
	if err != nil {
		return nil, err
	}

	if !cfg.SkipLoadingCredentials {
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
	}

	execUser, err := resolveExecUser(cfg)
	if err != nil {
		return nil, err
	}

	// prepare lock file directory
	if err = prepareDirectories([]string{agent.Configuration.LockFileDir()}, execUser.uid, execUser.gid); err != nil {
		return nil, err
	}

	// prepare state directories
	if err = prepareStateDirectories(cfg.StateDirectory, execUser.uid, execUser.gid); err != nil {
		return nil, err
	}

	if execUser.uid == os.Geteuid() {
		return agent, nil
	}

	if err := setupUnprivilegedUser(execUser); err != nil {
		return nil, err
	}

	log.Infof("dropping privileges to user %s (uid: %d, gid: %d, groups: %v)", execUser.userInfo.Username, execUser.uid, execUser.gid, execUser.groupIDs)
	return agent, nil
}

// setupUnprivilegedUser prepares the agent to run as an unprivileged user.
func setupUnprivilegedUser(execUser execUser) error {

	// enable lingering privileges to be able to switch user (important for podman containers etc.)
	_, err := exec.LookPath("loginctl")
	if _, err2 := os.Stat("/.dockerenv"); err == nil && err2 != nil {
		if _, err := utils.RunCommand(context.Background(), []string{"loginctl", "enable-linger", execUser.userInfo.Username}); err != nil {
			return fmt.Errorf("failed to enable lingering for user %s: %w", execUser.userInfo.Username, err)
		}
	}

	// set environment variables
	if execUser.userInfo.HomeDir != "" {
		os.Setenv("HOME", execUser.userInfo.HomeDir)
	}
	os.Setenv("USER", execUser.userInfo.Username)

	if len(execUser.groupIDs) > 0 {
		if err := syscall.Setgroups(execUser.groupIDs); err != nil {
			return fmt.Errorf("failed to set supplementary groups for user %s: %w", execUser.userInfo.Username, err)
		}
	}

	if err := syscall.Setgid(execUser.gid); err != nil {
		return fmt.Errorf("failed to set group ID to %d: %w", execUser.gid, err)
	}

	if err := syscall.Setuid(execUser.uid); err != nil {
		return fmt.Errorf("failed to set user ID to %d: %w", execUser.uid, err)
	}

	return nil
}
