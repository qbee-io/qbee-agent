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

import (
	"context"
	"crypto/tls"
	"sync"

	"go.qbee.io/transport"
)

// New creates a new instance of the remote access service.
func New() *Service {
	return &Service{}
}

// Service controls remote access for the agent.
type Service struct {
	client    *transport.DeviceClient
	tlsConfig *tls.Config
	mutex     sync.Mutex

	// consoleMap is a map of all active consoles.
	consoleMap map[string]*Console

	// consoleMapMutex is a mutex to protect the consoleMap from concurrent access.
	consoleMapMutex sync.Mutex
}

// WithTLSConfig sets the TLS configuration for the remote access service.
func (s *Service) WithTLSConfig(tlsConfig *tls.Config) *Service {
	s.tlsConfig = tlsConfig
	return s
}

// ensureInit initializes the remote access client if not already initialized.
// We do this lazy initialization, as we want to use the edgeURL which comes from the agent config,
// and that's not immediately available when the service is created.
func (s *Service) ensureInit(edgeURL string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// return immediately if already initialized
	if s.client != nil {
		return nil
	}

	client, err := transport.NewDeviceClient(edgeURL, s.tlsConfig)
	if err != nil {
		return err
	}

	s.client = client.
		WithHandler(transport.MessageTypeTCPTunnel, transport.HandleTCPTunnel).
		WithHandler(transport.MessageTypeUDPTunnel, transport.HandleUDPTunnel).
		WithHandler(transport.MessageTypePTY, s.HandleConsole).
		WithHandler(transport.MessageTypePTYCommand, s.HandleConsoleCommand)

	return nil
}

// Stop disables remote access.
func (s *Service) Stop() error {
	if s.client == nil {
		return nil
	}

	if !s.client.IsRunning() {
		return nil
	}

	return s.client.Close()
}

// UpdateState ensures that remote access is enabled or disabled based on the enabled parameter.
func (s *Service) UpdateState(ctx context.Context, edgeURL string, enabled bool) error {
	if err := s.ensureInit(edgeURL); err != nil {
		return err
	}

	if !enabled && s.client.IsRunning() {
		return s.client.Close()
	}

	if !s.client.IsRunning() && enabled {
		s.client.Start(ctx)
	}

	return nil
}
