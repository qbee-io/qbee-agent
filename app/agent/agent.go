package agent

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
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
	"github.com/qbee-io/qbee-agent/app/utils"
)

type Agent struct {
	cfg *Config

	privateKey  *ecdsa.PrivateKey
	certificate *x509.Certificate
	caCertPool  *x509.CertPool

	api        *api.Client
	inProgress *sync.WaitGroup
	loopTicker *time.Ticker
	stop       chan bool

	Inventory     *inventory.Service
	Configuration *configuration.Service
	Metrics       *metrics.Service
}

// Run the main control loop of the agent.
func (agent *Agent) Run(ctx context.Context) error {
	// TODO: check for updates

	// look for run interval changes
	intervalChange := agent.Configuration.RunIntervalChangedNotifier()

	// catch SIGINT and SIGKILL to gracefully shut down the agent
	stopSignalCh := make(chan os.Signal, 1)
	signal.Notify(stopSignalCh, os.Interrupt, os.Kill)

	// use SIGUSR1 to force processing outside normal schedule
	updateSignalCh := make(chan os.Signal, 1)
	signal.Notify(updateSignalCh, syscall.SIGUSR1)

	// ticker won't trigger the first run immediately, so let's do that ourselves
	go agent.RunOnce(ctx)

	log.Infof("starting agent scheduler")
	for {
		select {
		case <-agent.stop:
			log.Infof("stopping the agent")

			// let all the processing finish
			agent.inProgress.Wait()

			// and return
			return nil

		case <-stopSignalCh:
			log.Debugf("received interrupt signal")

			agent.stop <- true

		case newInterval := <-intervalChange:
			log.Debugf("run interval updated: %s", newInterval)
			agent.loopTicker.Reset(newInterval)

		case <-agent.loopTicker.C:
			go agent.RunOnce(ctx)

		case <-updateSignalCh:
			log.Debugf("received update signal")
			// reset the ticker, so we don't run the update twice (scheduled and manually triggered)
			agent.loopTicker.Reset(agent.Configuration.RunInterval())

			go agent.RunOnce(ctx)
		}
	}
}

// RunOnce performs a single run of the agent routines.
func (agent *Agent) RunOnce(ctx context.Context) {
	defer func() {
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

	agent.doHeartbeat(ctx)
	agent.doMetrics(ctx)
	agent.doInventories(ctx)
	agent.doConfig(ctx, configData)

	if agent.Configuration.ShouldReboot() {
		agent.RebootSystem(ctx)
	}
}

// doHeartbeat sends a heartbeat to the device hub.
func (agent *Agent) doHeartbeat(ctx context.Context) {
	agent.inProgress.Add(1)

	go func() {
		defer agent.inProgress.Done()

		if err := agent.sendHeartbeat(ctx); err != nil {
			log.Errorf("failed to send heartbeat: %v", err)
		}
	}()
}

// doMetrics collects system metrics - if enabled - and delivers them to the device hub API.
func (agent *Agent) doMetrics(ctx context.Context) {
	if !agent.Configuration.MetricsEnabled() {
		return
	}

	agent.inProgress.Add(1)

	go func() {
		defer agent.inProgress.Done()

		if err := agent.Metrics.Send(ctx, metrics.Collect()); err != nil {
			log.Errorf("failed to send metrics: %v", err)
		}
	}()
}

// doConfig executes the committed configuration.
func (agent *Agent) doConfig(ctx context.Context, configData *configuration.CommittedConfig) {
	agent.inProgress.Add(1)
	defer agent.inProgress.Done()

	currentCommitID := agent.Configuration.CurrentCommitID()

	if err := agent.Configuration.Execute(ctx, configData); err != nil {
		log.Errorf("failed to apply configuration: %v", err)
	}

	// when new config has a different commitID then applied to the system, let's push new inventories out
	if currentCommitID != configData.CommitID {
		agent.doInventories(ctx)
	}
}

const shutdownBinPath = "/sbin/shutdown"

// RebootSystem reboots the host system.
func (agent *Agent) RebootSystem(ctx context.Context) {
	if _, err := exec.LookPath(shutdownBinPath); err != nil {
		log.Errorf("cannot reboot: %s - %v", shutdownBinPath, err)
		return
	}

	if output, err := utils.RunCommand(ctx, []string{"/sbin/shutdown", "-r", "+1"}); err != nil {
		log.Errorf("scheduling system reboot failed: %v", err)
	} else {
		log.Infof("scheduling system reboot completed: %s", output)
	}

	agent.stop <- true
}

// NewWithoutCredentials returns a new instance of Agent without loaded credentials.
func NewWithoutCredentials(cfg *Config) (*Agent, error) {
	agent := &Agent{
		cfg:        cfg,
		inProgress: new(sync.WaitGroup),
		stop:       make(chan bool, 1),
	}

	if err := api.UseProxy(cfg.ProxyServer, cfg.ProxyPort, cfg.ProxyUser, cfg.ProxyPassword); err != nil {
		return nil, err
	}

	if err := agent.loadCACertificatesPool(); err != nil {
		return nil, err
	}

	agent.api = api.NewClient(cfg.DeviceHubServer, cfg.DeviceHubPort, agent.caCertPool)

	cacheDir := filepath.Join(cfg.StateDirectory, appWorkingDirectory, cacheDirectory)
	agent.Inventory = inventory.New(agent.api)
	agent.Configuration = configuration.New(agent.api, cacheDir)
	agent.Metrics = metrics.New(agent.api)
	agent.loopTicker = time.NewTicker(agent.Configuration.RunInterval())

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
