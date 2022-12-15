package agent

import (
	"fmt"

	"github.com/google/go-tpm-tools/client"
	"github.com/google/go-tpm-tools/proto/tpm"
	"github.com/google/go-tpm/tpm2"
	"google.golang.org/protobuf/proto"

	"github.com/qbee-io/qbee-agent/app/log"
)

var tpmPCRSelection = tpm2.PCRSelection{Hash: tpm2.AlgSHA256, PCRs: []int{7}}

// SealSecret seals a secret with TPM storage root key.
func (agent *Agent) SealSecret(secret []byte) ([]byte, error) {
	if agent.cfg.TPMDevice == "" {
		return nil, fmt.Errorf("cannot seal the secret, TPM device not configured")
	}

	// open TPM device
	tpmDevice, err := tpm2.OpenTPM(agent.cfg.TPMDevice)
	if err != nil {
		return nil, fmt.Errorf("cannot open TPM device %s: %w", agent.cfg.TPMDevice, err)
	}

	// always close TPM device when sealing is done
	defer func() {
		if err = tpmDevice.Close(); err != nil {
			log.Errorf("error closing TPM device %s: %v", agent.cfg.TPMDevice, err)
		}
	}()

	// get ECC storage root key
	var storageRootKey *client.Key
	if storageRootKey, err = client.StorageRootKeyECC(tpmDevice); err != nil {
		return nil, fmt.Errorf("error creating a TPM storage root key: %w", err)
	}

	// seal the secret
	var sealedSecret *tpm.SealedBytes
	sealedSecret, err = storageRootKey.Seal(secret, client.SealOpts{Current: tpmPCRSelection})
	if err != nil {
		return nil, fmt.Errorf("error sealing secret with SRK: %w", err)
	}

	// marshal to bytes and return
	var sealedSecretBytes []byte
	sealedSecretBytes, err = proto.Marshal(sealedSecret)
	if err != nil {
		return nil, fmt.Errorf("error marshaling the sealed secret: %w", err)
	}

	return sealedSecretBytes, nil
}

// UnsealSecret unseals a secret with TPM storage root key.
func (agent *Agent) UnsealSecret(sealedSecretBytes []byte) ([]byte, error) {
	// unmarshal sealedSecret from bytes
	sealedSecret := new(tpm.SealedBytes)
	if err := proto.Unmarshal(sealedSecretBytes, sealedSecret); err != nil {
		return nil, fmt.Errorf("error marshaling the sealed secret: %w", err)
	}

	if agent.cfg.TPMDevice == "" {
		return nil, fmt.Errorf("cannot unseal the secret, TPM device not configured")
	}

	// open TPM device
	tpmDevice, err := tpm2.OpenTPM(agent.cfg.TPMDevice)
	if err != nil {
		return nil, fmt.Errorf("cannot open TPM device %s: %w", agent.cfg.TPMDevice, err)
	}

	// always close TPM device when sealing is done
	defer func() {
		if err = tpmDevice.Close(); err != nil {
			log.Errorf("error closing TPM device %s: %v", agent.cfg.TPMDevice, err)
		}
	}()

	// get ECC storage root key
	var storageRootKey *client.Key
	if storageRootKey, err = client.StorageRootKeyECC(tpmDevice); err != nil {
		return nil, fmt.Errorf("error creating a TPM storage root key: %w", err)
	}

	// unseal and return
	var secret []byte
	if secret, err = storageRootKey.Unseal(sealedSecret, client.UnsealOpts{CertifyCurrent: tpmPCRSelection}); err != nil {
		return nil, fmt.Errorf("error unsealing secret: %w", err)
	}

	return secret, nil
}
