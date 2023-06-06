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

type Agent struct {
	cfg *Config

	privateKey  *ecdsa.PrivateKey
	certificate *x509.Certificate
	caCertPool  *x509.CertPool

	api        *api.Client
	inProgress *sync.WaitGroup
	loopTicker *time.Ticker
	stop       chan bool
	update     chan bool

	Inventory     *inventory.Service
	inventoryLock sync.Mutex
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
			agent.inProgress.Wait()

			// and return
			return nil

		case <-stopSignalCh:
			log.Debugf("received interrupt signal")
			if err := agent.remoteAccess.Stop(); err != nil {
				log.Errorf("failed to stop remote access: %s", err)
			}
			agent.stop <- true

		case newInterval := <-intervalChange:
			log.Debugf("run interval updated: %s", newInterval)
			agent.loopTicker.Reset(newInterval)

		case <-agent.loopTicker.C:
			go agent.RunOnce(ctx, FullRun)

		case <-remoteAccessStateChange:
			log.Debugf("remote access state changed, sending system inventory")
			agent.do(ctx, "system-inventory", agent.doSystemInventory)

		case <-updateSignalCh:
			log.Debugf("received update signal")
			// reset the ticker, so we don't run the update twice (scheduled and manually triggered)
			agent.loopTicker.Reset(agent.Configuration.RunInterval())

			go agent.RunOnce(ctx, FullRun)

		case <-agent.update:
			log.Infof("starting agent update")

			// wait for all the processing to finish
			agent.inProgress.Wait()

			// and run the update (this will block until the update is finished)
			if err := agent.updateAgent(ctx); err != nil {
				log.Errorf("failed to update the agent: %v", err)
			}
		}
	}
}

type RunOnceMode int

const (
	FullRun RunOnceMode = iota
	QuickRun
)

// RunOnce performs a single run of the agent routines.
func (agent *Agent) RunOnce(ctx context.Context, mode RunOnceMode) {
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

	if mode == FullRun {
		agent.do(ctx, "inventories", agent.doInventories)
		agent.do(ctx, "check-in", agent.doCheckIn)
		agent.do(ctx, "metrics", agent.doMetrics)
		agent.do(ctx, "config", agent.doConfig(configData))
		agent.do(ctx, "remote-access", agent.doRemoteAccess)
	} else {
		agent.do(ctx, "system-inventory", agent.doSystemInventory)
	}

	if agent.Configuration.ShouldReboot() {
		agent.RebootSystem(ctx)
	}
}

// do execute a function in a background goroutine.
func (agent *Agent) do(ctx context.Context, name string, fn func(ctx context.Context) error) {
	log.Debugf("starting %s", name)
	agent.inProgress.Add(1)

	go func() {
		defer func() {
			log.Debugf("stopping %s", name)
			agent.inProgress.Done()
		}()

		if err := fn(ctx); err != nil {
			log.Errorf("failed to do %s: %v", name, err)
		}
	}()
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

	if err := agent.Metrics.Send(ctx, metrics.Collect()); err != nil {
		return fmt.Errorf("failed to send metrics: %w", err)
	}

	return nil
}

// doConfig returns a function which executes the committed configuration.
func (agent *Agent) doConfig(configData *configuration.CommittedConfig) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		currentCommitID := agent.Configuration.CurrentCommitID()

		if err := agent.Configuration.Execute(ctx, configData); err != nil {
			return fmt.Errorf("failed to apply configuration: %w", err)
		}

		// when new config has a different commitID then applied to the system, let's push new inventories out
		if currentCommitID != configData.CommitID {
			agent.do(ctx, "inventories", agent.doInventories)
		}

		return nil
	}
}

// doRemoteAccess maintains remote access for the agent - if enabled.
func (agent *Agent) doRemoteAccess(ctx context.Context) error {
	// remote access is disabled on RunOnce
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

	agent.inProgress.Wait()

	if output, err := utils.RunCommand(ctx, []string{"/sbin/shutdown", "-r", "+1"}); err != nil {
		log.Errorf("scheduling system reboot failed: %v", err)
	} else {
		log.Infof("scheduling system reboot completed: %s", output)
	}
	if err := agent.remoteAccess.Stop(); err != nil {
		log.Errorf("failed to stop remote access: %v", err)
	}
	agent.stop <- true
}

// NewWithoutCredentials returns a new instance of Agent without loaded credentials.
func NewWithoutCredentials(cfg *Config) (*Agent, error) {
	if err := prepareDirectories(cfg.Directory, cfg.StateDirectory); err != nil {
		return nil, err
	}

	agent := &Agent{
		cfg:        cfg,
		inProgress: new(sync.WaitGroup),
		stop:       make(chan bool, 1),
		update:     make(chan bool, 1),
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
