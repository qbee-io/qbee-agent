package agent

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/qbee-io/qbee-agent/app/inventory"
	"github.com/qbee-io/qbee-agent/app/log"
)

const boostrapWaitTime = 5 * time.Second

// Bootstrap device using agent's config and provided bootstrap key.
func Bootstrap(ctx context.Context, cfg *Config, bootstrapKey string) error {
	if err := prepareDirectories(cfg.Directory, cfg.StateDirectory); err != nil {
		return err
	}

	agent, err := NewWithoutCredentials(cfg)
	if err != nil {
		return err
	}

	if err = agent.createPrivateKey(); err != nil {
		return err
	}

	var bootstrapRequest *BootstrapRequest
	bootstrapRequest, err = agent.newBootstrapRequest()
	if err != nil {
		return err
	}

	var response *BootstrapResponse

	log.Infof("Sending bootstrap request to %s:%s", agent.cfg.DeviceHubServer, agent.cfg.DeviceHubPort)

	for {

		if response, err = agent.sendBootstrapRequest(ctx, bootstrapKey, bootstrapRequest); err != nil {
			return fmt.Errorf("error sending bootstrap request: %w", err)
		}

		if response.CertificateRequestsStatus == "authorized" {
			break
		}

		log.Infof("Awaiting to be approved.")
		time.Sleep(boostrapWaitTime)
	}

	pemCert := []byte(strings.Join(response.Certificate, "\n"))

	if err = agent.saveCertificate(pemCert); err != nil {
		return err
	}

	if err = agent.saveConfig(); err != nil {
		return err
	}

	// re-initialize agent to make use of new credentials
	agent, err = New(cfg)
	if err != nil {
		return err
	}

	agent.RunOnce(ctx)
	agent.inProgress.Wait()

	log.Infof("Bootstrap successfully completed")

	return nil
}

// getRawPublicKey returns a slice of PEM-encoded public key lines.
func (agent *Agent) getRawPublicKey() ([]string, error) {
	if agent.privateKey == nil {
		return nil, fmt.Errorf("agent does not have a private key set")
	}

	publicKey := &agent.privateKey.PublicKey

	publicKeyDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("error marshaling public key: %w", err)
	}

	pemBlock := pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	}

	return strings.Split(string(pem.EncodeToMemory(&pemBlock)), "\n"), nil
}

// newBootstrapRequest returns a new BootstrapRequest for the agent.
func (agent *Agent) newBootstrapRequest() (*BootstrapRequest, error) {
	log.Infof("Gathering system information")

	systemInventory, err := inventory.CollectSystemInventory()
	if err != nil {
		return nil, fmt.Errorf("error collecting system info: %w", err)
	}

	var rawPublicKey []string
	if rawPublicKey, err = agent.getRawPublicKey(); err != nil {
		return nil, err
	}

	systemInfo := systemInventory.System

	bootstrapRequest := &BootstrapRequest{
		Host:         systemInfo.Host,
		FQHost:       systemInfo.FQHost,
		UQHost:       systemInfo.UQHost,
		HardwareMAC:  systemInfo.HardwareMAC,
		IPDefault:    systemInfo.IPv4First,
		IPv4:         systemInfo.IPv4,
		RawPublicKey: rawPublicKey,
	}

	return bootstrapRequest, nil
}
