package agent

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"net/http"
	"os"
	"path"

	"github.com/qbee-io/qbee-agent/app/log"
)

const (
	credentialsDirectory     = "ppkeys"
	credentialsDirectoryMode = 0700

	caCertificateURL      = "https://cdn.qbee.io/app/device/ca-cert.pem"
	caCertificateFilename = "qbee-ca-cert.pem"

	privateKeyFilename  = "qbee.key"
	certificateFilename = "qbee.cert"
	credentialsFileMode = 0600
)

// createCredentialsDirectory checks whether agent's credentials directory exists and has correct permissions.
// Directory will be created if not found.
func (agent *Agent) createCredentialsDirectory() error {
	log.Infof("Preparing agent directories")

	credentialsDir := path.Join(agent.cfg.Directory, credentialsDirectory)

	if err := os.MkdirAll(credentialsDir, credentialsDirectoryMode); err != nil {
		return fmt.Errorf("error creating credentials directory %s: %w", credentialsDir, err)
	}

	stats, err := os.Stat(credentialsDir)
	if err != nil {
		return fmt.Errorf("error checking status of the credentials directory %s: %w", credentialsDir, err)
	}

	if !stats.IsDir() {
		return fmt.Errorf("credentials directory is not a directory %s: %w", credentialsDir, err)
	}

	if stats.Mode() != credentialsDirectoryMode|fs.ModeDir {
		return fmt.Errorf("credentials directory %s has incorrect permissions %s", credentialsDir, stats.Mode())
	}

	return nil
}

// loadCACertificate attempts to load CA certificate from the filesystem or download it from CDN when not found.
func (agent *Agent) loadCACertificate() error {
	caCertPath := path.Join(agent.cfg.Directory, credentialsDirectory, caCertificateFilename)

	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("error opening CA certificate file %s: %w", caCertPath, err)
		}

		log.Infof("Downloading CA certificate")
		caCertPEM, err = agent.downloadCACertificate()
	}

	pemBlock, _ := pem.Decode(caCertPEM)
	if pemBlock == nil || pemBlock.Type != "CERTIFICATE" {
		return fmt.Errorf("error parsing CA certificate file %s: %w", caCertPath, err)
	}

	var certificate *x509.Certificate
	if certificate, err = x509.ParseCertificate(pemBlock.Bytes); err != nil {
		return fmt.Errorf("error parsing CA certificate: %w", err)
	}

	agent.rootCAPool = x509.NewCertPool()
	agent.rootCAPool.AddCert(certificate)

	return nil
}

// downloadCACertificate and store it on the filesystem for future reference.
func (agent *Agent) downloadCACertificate() ([]byte, error) {
	response, err := http.Get(caCertificateURL)
	if err != nil {
		return nil, fmt.Errorf("error downloading CA certificate from %s: %w", caCertificateURL, err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status when downloading CA certificate: %d", response.StatusCode)
	}

	var caCertPEM []byte
	if caCertPEM, err = io.ReadAll(response.Body); err != nil {
		return nil, fmt.Errorf("error reading CA certificate from %s: %w", caCertificateURL, err)
	}

	caCertPath := path.Join(agent.cfg.Directory, credentialsDirectory, caCertificateFilename)

	if err = os.WriteFile(caCertPath, caCertPEM, credentialsFileMode); err != nil {
		return nil, fmt.Errorf("error writing CA certificate to disk %s: %w", caCertPath, err)
	}

	return caCertPEM, nil
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
	keyPath := path.Join(agent.cfg.Directory, credentialsDirectory, privateKeyFilename)

	if err = os.WriteFile(keyPath, pemBytes, credentialsFileMode); err != nil {
		return fmt.Errorf("")
	}

	agent.privateKey = privateKey

	return nil
}

// loadPrivateKey loads private key from the config directory.
func (agent *Agent) loadPrivateKey() error {
	keyPath := path.Join(agent.cfg.Directory, credentialsDirectory, privateKeyFilename)

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

	keyPath := path.Join(agent.cfg.Directory, credentialsDirectory, certificateFilename)

	if err = os.WriteFile(keyPath, pemCertificate, credentialsFileMode); err != nil {
		return fmt.Errorf("error writing certificate: %w", err)
	}

	agent.certificate = certificate

	return nil
}

// loadCertificate loads agent's client certificate.
func (agent *Agent) loadCertificate() error {
	certPath := path.Join(agent.cfg.Directory, credentialsDirectory, certificateFilename)

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

// loadCredentials loads agent's TLS credentials and CA root certificate.
func (agent *Agent) loadCredentials() error {
	if err := agent.loadCACertificate(); err != nil {
		return err
	}

	if err := agent.loadPrivateKey(); err != nil {
		return err
	}

	if err := agent.loadCertificate(); err != nil {

	}

	return nil
}
