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

package agent

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
)

// BootstrapRequest is the request sent to the device hub during device bootstrap.
type BootstrapRequest struct {
	// Host - The name of the current host, according to the kernel.
	// It is undefined whether this is qualified or unqualified with a domain name.
	Host string `json:"host"`

	// FQHost - The fully qualified name of the host (e.g. "device1.example.com").
	FQHost string `json:"fqhost"`

	// UQHost - The unqualified name of the host (e.g. "device1").
	UQHost string `json:"uqhost"`

	// HardwareMAC - This contains the MAC address of the named interface map[interface]macAddress.
	// Note: The keys in this array are canonified.
	// For example, the entry for wlan0.1 would be found under the wlan0_1 key.
	//
	// Example:
	// {
	// 	"ens1": "52:54:00:4a:db:ee",
	//  "qbee0": "00:00:00:00:00:00"
	// }
	HardwareMAC map[string]string `json:"hardware_mac"`

	// IPDefault - All four octets of the IPv4 address of the first system interface.
	//Note: If the system has a single ethernet interface, this variable will contain the IPv4 address.
	// However, if the system has multiple interfaces, then this variable will simply be the IPv4 address of the first
	// interface in the list that has an assigned address.
	// Use IPv4[interface_name] for details on obtaining the IPv4 addresses of all interfaces on a system.
	IPDefault string `json:"ip_default"`

	// IPv4 - All IPv4 addresses of the system mapped by interface name.
	// Example:
	// {
	//	"ens1": "192.168.122.239",
	//	"qbee0": "100.64.39.78"
	// }
	IPv4 map[string]string `json:"ipv4"`

	// RawPublicKey of the device as slice of PEM-encoded lines.
	// Example:
	// []string{
	//    "-----BEGIN PUBLIC KEY-----",
	//    "MIGbMBAGByqGSM49AgEGBSuBBAAjA4GGAAQBvDALiaU+dyvd1DhMUCEXnuX4h5r5",
	//    "ikBVNSl88QBtBoxtQy1XsCJ7Dm/tzoQ1YPYT80oVTdExK/oFnZFvI89SX8sBN89L",
	//    "Y8q+8BBQPLf1nA3DG7apq7xq11Zkpde2eK0pCUG7nZPisXlU96C44NLE62TzDYEZ",
	//    "RYkhJQhFeNOlFSpF/xA=",
	//    "-----END PUBLIC KEY-----"
	// }
	RawPublicKey []string `json:"pub_key"`

	// DeviceName is the name of the device to use. Only in use during bootstrap
	DeviceName string `json:"device_name,omitempty"`
}

// BootstrapResponse is the response sent by the device hub during device bootstrap.
type BootstrapResponse struct {
	// CertificateRequestsStatus is the status of the certificate requests.
	// Possible values are:
	// - registered - the request is registered, but requires approval
	// - authorized - the request is approved and device is authorized
	CertificateRequestsStatus string `json:"cert_req"`

	// Certificate is the PEM-encoded certificate.
	Certificate []string `json:"cert"`

	// CACertificate is the PEM-encoded CA certificate.
	CACertificate []string `json:"ca_cert"`
}

const bootstrapAPIPath = "/v1/org/device/xauth/bootstrap"

// sendBootstrapRequest sends bootstrap request to the device hub.
func (agent *Agent) sendBootstrapRequest(
	ctx context.Context,
	bootstrapKey string,
	req *BootstrapRequest,
) (*BootstrapResponse, error) {
	request, err := agent.api.NewRequest(ctx, http.MethodPost, bootstrapAPIPath, req)
	if err != nil {
		return nil, fmt.Errorf("error preparing bootstrap request: %w", err)
	}

	request.Header.Set("Authorization", fmt.Sprintf("token %s", bootstrapKey))

	bootstrapResponse := new(BootstrapResponse)

	if err = agent.api.Make(request, bootstrapResponse); err != nil {
		return nil, err
	}

	return bootstrapResponse, nil
}

var checkInPath = fmt.Sprintf("/v1/org/device/auth/agent/%s/checkin", runtime.GOARCH)

// checkIn sends a heartbeat to the device hub and retrieves agent metadata.
func (agent *Agent) checkIn(ctx context.Context) error {
	return agent.api.Get(ctx, checkInPath, nil)
}
