package main

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
)

func main() {
	// only keys with curve P256 are supported
	signingKeyPath := flag.String("signingKey", "", "Private signing EC key DER-encoded")
	flag.Parse()

	if *signingKeyPath == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	keyBytes, err := os.ReadFile(*signingKeyPath)
	if err != nil {
		panic(err)
	}

	var key *ecdsa.PrivateKey
	if key, err = x509.ParseECPrivateKey(keyBytes); err != nil {
		panic(err)
	}

	x := base64.RawURLEncoding.EncodeToString(key.X.Bytes())
	y := base64.RawURLEncoding.EncodeToString(key.Y.Bytes())

	fmt.Printf("Public Signing Key: %s.%s\n", x, y)
}
