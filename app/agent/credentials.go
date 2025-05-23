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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"

	"go.qbee.io/agent/app/log"
	"go.qbee.io/agent/app/utils"
)

const (
	caCertFilename      = "ca.cert"
	privateKeyFilename  = "qbee.key"
	certificateFilename = "qbee.cert"
	credentialsFileMode = 0600
)

// loadCACertificatesPool loads trusted CA certificate.
func (agent *Agent) loadCACertificatesPool(caCert string) error {
	caCertPath := filepath.Join(agent.cfg.Directory, credentialsDirectory, caCertFilename)

	if caCert != "" {
		caCertPath = caCert
	}

	agent.caCertPool = x509.NewCertPool()

	pemCert, err := os.ReadFile(caCertPath)
	if err != nil {
		// we want to allow local use without CA certificate, as the agent won't have to talk to the device hub
		if errors.Is(err, fs.ErrNotExist) {
			log.Warnf("CA certificate %s not found - only local use possible", caCertPath)
			return nil
		}

		return fmt.Errorf("error reading CA certificate %s: %w", caCertPath, err)
	}

	for len(pemCert) > 0 {
		var pemBlock *pem.Block
		if pemBlock, pemCert = pem.Decode(pemCert); pemBlock == nil {
			return fmt.Errorf("error decoding CA certificate %s: %w", caCertPath, err)
		}

		var envCACert *x509.Certificate
		if envCACert, err = x509.ParseCertificate(pemBlock.Bytes); err != nil {
			return fmt.Errorf("error parsing CA certificate %s: %w", caCertPath, err)
		}

		agent.caCertPool.AddCert(envCACert)
	}

	return nil
}

const (
	ecPrivateKeyPEMHeader       = "EC PRIVATE KEY"
	sealedECPrivateKeyPEMHeader = "SEALED PRIVATE KEY"
)

// createPrivateKey tries to load private key file from the config directory, if not found, a new key is generated.
// If TPM is available, key will be sealed before storing on the filesystem.
func (agent *Agent) createPrivateKey() error {
	if err := agent.loadPrivateKey(); err == nil {
		log.Infof("Using existing private key")
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	log.Infof("Generating new private key")
	privateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return fmt.Errorf("error generating new private key: %w", err)
	}

	var privateKeyDER []byte
	if privateKeyDER, err = x509.MarshalECPrivateKey(privateKey); err != nil {
		return fmt.Errorf("error marshaling private key: %w", err)
	}

	pemBlock := &pem.Block{
		Type:  ecPrivateKeyPEMHeader,
		Bytes: privateKeyDER,
	}

	if agent.cfg.TPMDevice != "" {
		pemBlock.Type = sealedECPrivateKeyPEMHeader
		if pemBlock.Bytes, err = agent.SealSecret(privateKey.D.Bytes()); err != nil {
			return err
		}
	}

	pemBytes := pem.EncodeToMemory(pemBlock)
	keyPath := filepath.Join(agent.cfg.Directory, credentialsDirectory, privateKeyFilename)

	if err = utils.WriteFileSync(keyPath, pemBytes, credentialsFileMode); err != nil {
		return fmt.Errorf("unable to write private key to %s: %w", keyPath, err)
	}

	agent.privateKey = privateKey

	return nil
}

// loadPrivateKey loads private key from the config directory.
func (agent *Agent) loadPrivateKey() error {
	keyPath := filepath.Join(agent.cfg.Directory, credentialsDirectory, privateKeyFilename)

	pemBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("error loading private key file %s: %w", keyPath, err)
	}

	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		return fmt.Errorf("error decoding private key's PEM block")
	}

	if pemBlock.Type == sealedECPrivateKeyPEMHeader {
		var dBytes []byte
		if dBytes, err = agent.UnsealSecret(pemBlock.Bytes); err != nil {
			return err
		}

		x, y := elliptic.P521().ScalarBaseMult(dBytes)
		agent.privateKey = &ecdsa.PrivateKey{
			D: new(big.Int).SetBytes(dBytes),
			PublicKey: ecdsa.PublicKey{
				Curve: elliptic.P521(),
				X:     x,
				Y:     y,
			},
		}
	} else {
		if agent.privateKey, err = x509.ParseECPrivateKey(pemBlock.Bytes); err != nil {
			return fmt.Errorf("error parsing private key: %w", err)
		}
	}

	return nil
}

func (agent *Agent) saveCertificate(pemCertificate []byte) error {
	pemBlock, _ := pem.Decode(pemCertificate)
	if pemBlock == nil || pemBlock.Type != "CERTIFICATE" {
		return fmt.Errorf("got invalid certificate")
	}

	certificate, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("error parsing certificate: %w", err)
	}

	keyPath := filepath.Join(agent.cfg.Directory, credentialsDirectory, certificateFilename)

	if err = utils.WriteFileSync(keyPath, pemCertificate, credentialsFileMode); err != nil {
		return fmt.Errorf("error writing certificate: %w", err)
	}

	agent.certificate = certificate

	return nil
}

// loadCertificate loads agent's client certificate.
func (agent *Agent) loadCertificate() error {
	certPath := filepath.Join(agent.cfg.Directory, credentialsDirectory, certificateFilename)

	pemBytes, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("error loading certificate file %s: %w", certPath, err)
	}

	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		return fmt.Errorf("error decoding certificate's PEM block")
	}

	agent.certificate, err = x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("error parsing certificate: %w", err)
	}

	return nil
}

// GetOrganizationID returns the organization ID from the certificate.
func (agent *Agent) GetOrganizationID() (string, error) {
	if agent.certificate == nil {
		return "", errors.New("certificate not loaded")
	}

	return string(agent.certificate.AuthorityKeyId), nil
}
