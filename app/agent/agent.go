package agent

import (
	"crypto/ecdsa"
	"crypto/x509"
	"path/filepath"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/configuration"
	"github.com/qbee-io/qbee-agent/app/inventory"
)

type Agent struct {
	cfg *Config

	privateKey  *ecdsa.PrivateKey
	certificate *x509.Certificate
	caCertPool  *x509.CertPool

	api *api.Client

	Inventory     *inventory.Service
	Configuration *configuration.Service
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
