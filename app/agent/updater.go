package agent

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/qbee-io/qbee-agent/app/log"
	"github.com/qbee-io/qbee-agent/app/utils"
)

// PublicSigningKey is the public key used to verify the signature of the agent binary.
// The key is in the format of "x.y" where x and y are the key coordinates encoded using Base64.RawURLEncoding.
// Following value is set for testing purposes. Production builds must override it.
var PublicSigningKey = "xSHbUBG7LTuNfXd3zod4EX8_Es8FTCINgrjvx1WXFE4.plCHzlDAeb3IWW1wK6P6paMRYO4f8qceV3lrNCqNpWo"
var publicSigningKey *ecdsa.PublicKey

func init() {
	pubKeyParts := strings.Split(PublicSigningKey, ".")
	if len(pubKeyParts) != 2 {
		panic(fmt.Errorf("invalid public signing key: %s", PublicSigningKey))
	}

	publicSigningKey = &ecdsa.PublicKey{
		Curve: elliptic.P256(),
	}

	if xBytes, err := base64.RawURLEncoding.DecodeString(pubKeyParts[0]); err != nil {
		panic(fmt.Errorf("failed to decode signing key: %w", err))
	} else {
		publicSigningKey.X = big.NewInt(0).SetBytes(xBytes)
	}

	if yBytes, err := base64.RawURLEncoding.DecodeString(pubKeyParts[1]); err != nil {
		panic(fmt.Errorf("failed to decode signing key: %w", err))
	} else {
		publicSigningKey.Y = big.NewInt(0).SetBytes(yBytes)
	}
}

// Update updates the agent binary.
func Update(ctx context.Context, cfg *Config) error {
	agent, err := New(cfg)
	if err != nil {
		return fmt.Errorf("cannot initialize agent: %w", err)
	}

	return agent.updateAgent(ctx)
}

type Metadata struct {
	Version   string `json:"version"`
	Digest    string `json:"digest"`
	Signature string `json:"signature"`
}

const nonExecutableFileMode = 0600
const executableFileMode = 0700

func (agent *Agent) updateAgent(ctx context.Context) error {
	// let's not block for more than the run interval
	ctxWithTimeout, cancel := context.WithTimeout(ctx, agent.Configuration.RunInterval())
	defer cancel()

	agentBinPath, err := exec.LookPath(os.Args[0])
	if err != nil {
		return fmt.Errorf("cannot determine agent path: %w", err)
	}

	log.Debugf("path#1: %s", agentBinPath)

	if agentBinPath, err = filepath.Abs(agentBinPath); err != nil {
		return fmt.Errorf("cannot determine absolute agent path: %w", err)
	}

	log.Debugf("path#2: %s", agentBinPath)

	var fp *os.File
	if fp, err = os.CreateTemp(filepath.Dir(agentBinPath), filepath.Base(agentBinPath)+".*.tmp"); err != nil {
		return fmt.Errorf("cannot create temporary agent binary: %w", err)
	}
	defer fp.Close()

	if err = fp.Chmod(nonExecutableFileMode); err != nil {
		return fmt.Errorf("cannot set permissions on temporary agent binary: %w", err)
	}

	tmpAgentPath := fp.Name()

	defer os.Remove(tmpAgentPath)

	log.Debugf("path#3: %s", tmpAgentPath)

	var metadata *Metadata
	if metadata, err = agent.downloadUpdate(ctxWithTimeout, fp); err != nil {
		return fmt.Errorf("cannot download update: %w", err)
	}

	if err = fp.Close(); err != nil {
		return fmt.Errorf("cannot close temporary agent binary: %w", err)
	}

	if err = agent.verifyVersion(ctxWithTimeout, tmpAgentPath, metadata); err != nil {
		return fmt.Errorf("cannot verify new agent version: %w", err)
	}

	if err = os.Rename(tmpAgentPath, agentBinPath); err != nil {
		return fmt.Errorf("cannot replace agent binary: %w", err)
	}

	// stop the agent
	agent.stop <- true

	return nil
}

// verifyVersion check the integrity of the given version.
func (agent *Agent) verifyVersion(ctx context.Context, agentPath string, metadata *Metadata) error {
	fp, err := os.Open(agentPath)
	if err != nil {
		return fmt.Errorf("cannot open agent binary: %v", err)
	}

	defer fp.Close()

	digest := sha256.New()
	if _, err := io.Copy(digest, fp); err != nil {
		return fmt.Errorf("cannot calculate digest: %v", err)
	}

	digestBytes := digest.Sum(nil)

	if agentHexDigest := fmt.Sprintf("%x", digestBytes); agentHexDigest != metadata.Digest {
		return fmt.Errorf("digest mismatch: %s != %s", agentHexDigest, metadata.Digest)
	}

	var signature []byte
	if signature, err = base64.StdEncoding.DecodeString(metadata.Signature); err != nil {
		return fmt.Errorf("cannot decode signature: %v", err)
	}

	if !ecdsa.VerifyASN1(publicSigningKey, digestBytes[:], signature) {
		return fmt.Errorf("signature mismatch")
	}

	if err = os.Chmod(agentPath, executableFileMode); err != nil {
		return fmt.Errorf("cannot make agent executable: %v", err)
	}

	// check if the downloaded binary version matches
	var output []byte
	if output, err = utils.RunCommand(ctx, []string{agentPath, "version"}); err != nil {
		return fmt.Errorf("cannot run new agent: %v - %s", err, output)
	}

	if strings.TrimSpace(string(output)) != metadata.Version {
		return fmt.Errorf("version mismatch: %s != %s", string(output), metadata.Version)
	}

	return nil
}
