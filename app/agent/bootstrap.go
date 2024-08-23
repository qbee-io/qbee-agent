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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"go.qbee.io/agent/app/api"
	"go.qbee.io/agent/app/inventory"
	"go.qbee.io/agent/app/log"
	"go.qbee.io/agent/app/utils"
)

const bootstrapWaitTime = 5 * time.Second

// Bootstrap device using agent's config and provided bootstrap key.
func Bootstrap(ctx context.Context, cfg *Config) error {

	// make sure the custom CA certificate path is not the same as the default path
	// as this would be overwritten in an agent upgrade
	if cfg.CACert == filepath.Join(cfg.Directory, credentialsDirectory, caCertFilename) {
		return fmt.Errorf("custom CA certificate path cannot be the same as the default path")
	}

	agent, err := NewWithoutCredentials(cfg)
	if err != nil {
		return err
	}

	// we cannot perform bootstrap without a CA certificate
	if agent.caCertPool.Equal(x509.NewCertPool()) {
		return fmt.Errorf("CA certificate pool is empty, bootstrap not possible")
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

		if response, err = agent.sendBootstrapRequest(ctx, cfg.BootstrapKey, bootstrapRequest); err != nil {
			// We should only break the loop if the error is an unauthorized error
			// There might be other errors that we can recover from, like network errors, clock skew, etc.
			asErr, ok := err.(*api.Error)
			if ok && asErr.ResponseCode == http.StatusUnauthorized {
				return fmt.Errorf("bootstrap key is invalid: %w", err)
			}
			log.Errorf("error sending bootstrap request: %v", err)
		}

		if response != nil {
			if response.CertificateRequestsStatus == "authorized" {
				break
			}
		}

		log.Infof("Awaiting to be approved.")
		time.Sleep(bootstrapWaitTime)
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

	agent.RunOnce(ctx, QuickRun)
	agent.Wait()

	log.Infof("Bootstrap successfully completed")
	log.Infof("Please remember to start the qbee-agent service as administrative user")

	upstartCmd, err := utils.GenerateServiceCommand(ctx, "qbee-agent", "start")
	if err != nil {
		log.Infof("Could not detect start command based on OS attributes: %v", err)
	} else {
		log.Infof("Detected start command based on OS attributes is: $ %s", strings.Join(upstartCmd, " "))
	}
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

	systemInventory, err := inventory.CollectSystemInventory(agent.IsTPMEnabled())
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

	if agent.cfg.DeviceName != "" {
		bootstrapRequest.DeviceName = agent.cfg.DeviceName
	}

	return bootstrapRequest, nil
}
