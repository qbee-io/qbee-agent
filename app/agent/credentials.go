package agent

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"

	"github.com/qbee-io/qbee-agent/app/log"
)

const (
	privateKeyFilename  = "qbee.key"
	certificateFilename = "qbee.cert"
	credentialsFileMode = 0600
)

//go:embed ca/dev.crt
var devRootCA []byte

//go:embed ca/prod.crt
var prodRootCA []byte

// loadCACertificatesPool loads all trusted CA certificate.
func (agent *Agent) loadCACertificatesPool() error {
	prodCACert, err := x509.ParseCertificate(prodRootCA)
	if err != nil {
		return fmt.Errorf("error parsing CA certificate: %w", err)
	}

	agent.caCertPool = x509.NewCertPool()
	agent.caCertPool.AddCert(prodCACert)

	// for non-production device-hub host, allow dev CA
	if agent.cfg.DeviceHubServer != DefaultDeviceHubServer {
		var devCACert *x509.Certificate
		if devCACert, err = x509.ParseCertificate(devRootCA); err != nil {
			return fmt.Errorf("error parsing dev-CA certificate: %w", err)
		}

		agent.caCertPool.AddCert(devCACert)
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

	if err = os.WriteFile(keyPath, pemBytes, credentialsFileMode); err != nil {
		return fmt.Errorf("")
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

	if err = os.WriteFile(keyPath, pemCertificate, credentialsFileMode); err != nil {
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
