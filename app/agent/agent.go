package agent

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
)

type Agent struct {
	cfg *Config

	privateKey  *ecdsa.PrivateKey
	certificate *x509.Certificate
	rootCAPool  *x509.CertPool
}

// New returns a new instance of Agent.
func New(cfg *Config) (*Agent, error) {
	agent := &Agent{cfg: cfg}

	if err := agent.useProxy(); err != nil {
		return nil, err
	}

	return agent, nil
}

// PublicKey returns public key of the agent.
func (agent *Agent) PublicKey() (*ecdsa.PublicKey, error) {
	if agent.privateKey == nil {
		return nil, fmt.Errorf("agent does not have a private key set")
	}

	return &agent.privateKey.PublicKey, nil
}

// rawPublicKey returns a slice of PEM-encoded public key lines.
func (agent *Agent) rawPublicKey() ([]string, error) {
	publicKey, err := agent.PublicKey()
	if err != nil {
		return nil, err
	}

	var publicKeyDER []byte
	if publicKeyDER, err = x509.MarshalPKIXPublicKey(publicKey); err != nil {
		return nil, fmt.Errorf("error marshaling private key: %w", err)
	}

	pemBlock := pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	}

	return strings.Split(string(pem.EncodeToMemory(&pemBlock)), "\n"), nil
}

// Start starts the agent.
func Start(ctx context.Context, cfg *Config) error {
	return nil
}

// StartWithAutoUpdate starts the agent with auto-update functionality.
func StartWithAutoUpdate(ctx context.Context, cfg *Config) error {
	return nil
}
