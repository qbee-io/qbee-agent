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
