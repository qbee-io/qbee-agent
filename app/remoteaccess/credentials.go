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

package remoteaccess

import "strings"

type Credentials struct {
	// CA - the CA certificate as a slice of new-lines in PEM format.
	CA []string `json:"vpn_ca_cert"`

	// Certificate - the certificate as a slice of new-lines in PEM format.
	Certificate []string `json:"vpn_cert"`

	// Expiry - time of the certificate expiration in seconds since epoch.
	Expiry int64 `json:"vpn_cert_expiry"`

	Status string `json:"status"`
}

// CertificatePEM returns the certificate in PEM format.
func (c Credentials) CertificatePEM() []byte {
	return []byte(strings.Join(c.Certificate, "\n"))
}

// CACertificatePEM returns the CA certificate in PEM format.
func (c Credentials) CACertificatePEM() []byte {
	return []byte(strings.Join(c.CA, "\n"))
}
