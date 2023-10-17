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

package api

import (
	"fmt"
	"os"
)

const proxyEnvVar = "HTTP_PROXY"

// Proxy represents a proxy server configuration.
type Proxy struct {
	Host     string
	Port     string
	User     string
	Password string
}

// UseProxy sets HTTP_PROXY environmental variable, so HTTP clients can make use of it.
func UseProxy(proxy *Proxy) error {
	// if proxy server is not specified or proxy is already set in the environment, return nil.
	if proxy == nil || os.Getenv(proxyEnvVar) != "" {
		return nil
	}

	proxyURL := fmt.Sprintf("%s:%s", proxy.Host, proxy.Port)

	if proxy.User != "" {
		proxyURL = fmt.Sprintf("%s:%s@%s", proxy.User, proxy.Password, proxyURL)
	}

	proxyURL = "http://" + proxyURL

	if err := os.Setenv(proxyEnvVar, proxyURL); err != nil {
		return fmt.Errorf("error setting up HTTP proxy: %w", err)
	}

	return nil
}
