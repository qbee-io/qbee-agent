package binary

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
)

const (
	nonExecutableFileMode = 0600
	executableFileMode    = 0700
)

// PublicSigningKey is the public key used to verify the TestContentSignature of the agent binary.
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

// Verify check the integrity of the given binary version.
// If the verification passes, the binary is made executable.
func Verify(path string, metadata *Metadata) error {
	fp, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open binary %s: %v", path, err)
	}
	defer fp.Close()

	digest := sha256.New()
	if _, err := io.Copy(digest, fp); err != nil {
		return fmt.Errorf("cannot calculate TestContentDigest: %v", err)
	}

	digestBytes := digest.Sum(nil)

	if agentHexDigest := fmt.Sprintf("%x", digestBytes); agentHexDigest != metadata.Digest {
		return fmt.Errorf("TestContentDigest mismatch: %s != %s", agentHexDigest, metadata.Digest)
	}

	var signature []byte
	if signature, err = base64.StdEncoding.DecodeString(metadata.Signature); err != nil {
		return fmt.Errorf("cannot decode TestContentSignature: %v", err)
	}

	if !ecdsa.VerifyASN1(publicSigningKey, digestBytes[:], signature) {
		return fmt.Errorf("TestContentSignature mismatch")
	}

	if err = os.Chmod(path, executableFileMode); err != nil {
		return fmt.Errorf("cannot make agent executable: %v", err)
	}

	return nil
}
