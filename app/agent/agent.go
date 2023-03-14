package agent

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"path/filepath"
	"sync"
	"time"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/configuration"
	"github.com/qbee-io/qbee-agent/app/inventory"
	"github.com/qbee-io/qbee-agent/app/log"
	"github.com/qbee-io/qbee-agent/app/metrics"
)

type Agent struct {
	cfg *Config

	privateKey  *ecdsa.PrivateKey
	certificate *x509.Certificate
	caCertPool  *x509.CertPool

	api *api.Client

	Inventory     *inventory.Service
	Configuration *configuration.Service
	Metrics       *metrics.Service
}

// Run the main control loop of the agent.
func (agent *Agent) Run(ctx context.Context) error {
	// TODO: handle panics
	// TODO: handle signals (ctrl-c to gracefully exit)
	// TODO: check for updates
	// TODO: run inventory checks and metrics recording
	// TODO: get config changes

	for {
		start := time.Now()

		if err := agent.RunOnce(ctx); err != nil {
			log.Errorf("run error: %v", err)
		}

		runTime := time.Since(start)
		agentRunInterval := agent.Configuration.RunInterval()
		if runTime < agentRunInterval {
			time.Sleep(agentRunInterval - runTime)
		}
	}
}

// RunOnce performs a single run of the agent routines.
func (agent *Agent) RunOnce(ctx context.Context) error {
	configData, err := agent.Configuration.Get(ctx)
	if err != nil {
		return err
	}

	if err = agent.Configuration.UpdateSettings(ctx, configData); err != nil {
		return err
	}

	currentCommitID := agent.Configuration.CurrentCommitID()

	waitGroup := new(sync.WaitGroup)
	agent.doMetrics(ctx, waitGroup)
	agent.doInventories(ctx, waitGroup)
	agent.doConfig(ctx, waitGroup, configData)
	waitGroup.Wait()

	// in case new configuration was applied, do system inventory again
	if currentCommitID != agent.Configuration.CurrentCommitID() {
		agent.doSystemInventory(ctx, waitGroup)
		waitGroup.Wait()
	}

	if agent.Configuration.ShouldReboot() {
		// TODO: reboot
	}

	return nil
}

// doMetrics collects system metrics - if enabled - and delivers them to the device hub API.
func (agent *Agent) doMetrics(ctx context.Context, waitGroup *sync.WaitGroup) {
	if !agent.Configuration.MetricsEnabled() {
		return
	}

	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()

		if err := agent.Metrics.Send(ctx, metrics.Collect(ctx)); err != nil {
			log.Errorf("failed to send metrics: %v", err)
		}
	}()
}

// doConfig executes the committed configuration.
func (agent *Agent) doConfig(ctx context.Context, configData *configuration.CommittedConfig) {
	currentCommitID := agent.Configuration.CurrentCommitID()

	if err := agent.Configuration.Execute(ctx, configData); err != nil {
		log.Errorf("failed to apply configuration: %v", err)
	}

	// when new config has a different commitID then applied to the system, let's push a new system inventory out
	if currentCommitID != configData.CommitID {
		waitGroup := new(sync.WaitGroup)
		agent.doSystemInventory(ctx, waitGroup)
		waitGroup.Wait()
	}
}

// NewWithoutCredentials returns a new instance of Agent without loaded credentials.
func NewWithoutCredentials(cfg *Config) (*Agent, error) {
	agent := &Agent{
		cfg: cfg,
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
