package test

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// getPublicKeyHexDigest returns SHA256 of device's public key
func getPublicKeyHexDigest(privateKeyPEM []byte) string {
	pemBlock, _ := pem.Decode(privateKeyPEM)
	if pemBlock == nil {
		panic("error decoding private key's PEM block")
	}

	privateKey, err := x509.ParseECPrivateKey(pemBlock.Bytes)
	if err != nil {
		panic(err)
	}

	var publicKeyDER []byte

	if publicKeyDER, err = x509.MarshalPKIXPublicKey(&privateKey.PublicKey); err != nil {
		panic(err)
	}

	publicKeyDigest := sha256.Sum256(publicKeyDER)

	return fmt.Sprintf("%x", publicKeyDigest)
}
