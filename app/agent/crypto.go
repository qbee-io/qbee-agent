// Copyright 2024 qbee.io
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
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"time"
)

const signedURLTTL = time.Hour

// SignURL signs the path portion of the given URL with the agent's private key
// and returns the signed URL string pointing to the device hub.
// NOTE: query parameters are not signed and should not be trusted!
func (agent *Agent) SignURL(rawURL string) (string, error) {
	if agent.privateKey == nil {
		return "", fmt.Errorf("private key not set")
	}

	orgID, err := agent.GetOrganizationID()
	if err != nil {
		return "", fmt.Errorf("error getting organization ID: %w", err)
	}

	var parsedURL *url.URL
	if parsedURL, err = url.Parse(rawURL); err != nil {
		return "", fmt.Errorf("error parsing URL: %w", err)
	}

	parsedURL.Scheme = "https"
	parsedURL.Host = net.JoinHostPort(agent.cfg.DeviceHubServer, agent.cfg.DeviceHubPort)

	expirationTimestamp := time.Now().Add(signedURLTTL).Unix()

	strToSign := fmt.Sprintf("%s%s%d", parsedURL.Path, orgID, expirationTimestamp)

	hash := sha256.Sum256([]byte(strToSign))

	var signature []byte
	if signature, err = ecdsa.SignASN1(rand.Reader, agent.privateKey, hash[:]); err != nil {
		return "", err
	}

	var pubKeyBytes []byte
	if pubKeyBytes, err = x509.MarshalPKIXPublicKey(&agent.privateKey.PublicKey); err != nil {
		return "", err
	}

	query := parsedURL.Query()
	query.Set("o", orgID)
	query.Set("k", base64.RawURLEncoding.EncodeToString(pubKeyBytes))
	query.Set("s", base64.RawURLEncoding.EncodeToString(signature))
	query.Set("e", fmt.Sprintf("%d", expirationTimestamp))
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}
