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
	"fmt"
	"sync"
	"time"

	"github.com/xtaci/smux"
	"go.qbee.io/transport"
)

// New creates a new instance of the remote access service.
func New() *Service {
	return &Service{
		consoleMap: make(map[string]*Console),
		connErrReporter: connectionErrorReporter{
			report: func(context.Context, error) {},
		},
	}
}

// Service controls remote access for the agent.
type Service struct {
	client    *transport.DeviceClient
	tlsConfig *tls.Config
	mutex     sync.Mutex

	// connErrReporter reports connection errors to the device hub.
	connErrReporter connectionErrorReporter

	// consoleMap is a map of all active consoles.
	consoleMap map[string]*Console

	// consoleMapMutex is a mutex to protect the consoleMap from concurrent access.
	consoleMapMutex sync.Mutex

	// configReloadNotifierChannel is a function that is called when the agent configuration reload is requested remotely.
	configReloadNotifierChannel chan bool
}

// WithConnectionErrorReporter sets a function which is called (at most once per outage) when the connection fails.
func (s *Service) WithConnectionErrorReporter(reporter func(ctx context.Context, err error)) *Service {
	s.connErrReporter.report = reporter
	return s
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
		WithHandler(transport.MessageTypePTYCommand, s.HandleConsoleCommand).
		WithHandler(transport.MessageTypeFile, transport.HandleFileTransfer).
		WithHandler(transport.MessageTypeReload, s.HandleReloadCommand).
		WithConnectHandler(s.connErrReporter.onConnect).
		WithErrorHandler(s.connErrReporter.onError)

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

	s.connErrReporter.clear()

	return s.client.Close()
}

// UpdateState ensures that remote access is enabled or disabled based on the enabled parameter.
func (s *Service) UpdateState(ctx context.Context, edgeURL string, enabled bool) error {
	if err := s.ensureInit(edgeURL); err != nil {
		return err
	}

	if !enabled && s.client.IsRunning() {
		s.connErrReporter.clear()
		return s.client.Close()
	}

	if !s.client.IsRunning() && enabled {
		s.connErrReporter.setContext(ctx)
		s.client.Start(ctx)
	}

	return nil
}

// connErrReportInterval is the minimum time between two reported connection errors.
const connErrReportInterval = time.Minute * 5

// connectionErrorReporter reports remote access connection errors once per outage
// and at most once per connErrReportInterval.
type connectionErrorReporter struct {
	mutex sync.Mutex

	// ctx is the context passed to report.
	ctx context.Context

	// report is called asynchronously to report a connection error.
	report func(ctx context.Context, err error)

	// reported is set once an error has been reported and cleared when the connection resumes.
	reported bool

	// reportedAt is the time when a connection error was last reported.
	reportedAt time.Time

	// connected is set while a connection is established.
	connected bool
}

// setContext sets the context passed to report.
func (r *connectionErrorReporter) setContext(ctx context.Context) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.ctx = ctx
}

// onConnect is called when connection is established.
func (r *connectionErrorReporter) onConnect() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.reported = false
	r.connected = true
}

// clear is called when remote access is disabled.
func (r *connectionErrorReporter) clear() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.reported = false
	r.connected = false
}

// onError is called on every failed connection attempt and when an established connection is lost.
func (r *connectionErrorReporter) onError(err error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// dropped connections (e.g. edge redeployments) are not reported, only failures to connect
	if r.connected {
		r.connected = false
		return
	}

	if r.reported {
		return
	}

	if time.Since(r.reportedAt) < connErrReportInterval {
		return
	}

	r.reported = true
	r.reportedAt = time.Now()

	// the connection loop must not block, so report in the background
	go r.report(r.ctx, err)
}

// WithConfigReloadNotifier registers a channel to be notified when a remote configuration reload is requested.
func (s *Service) WithConfigReloadNotifier(ch chan bool) *Service {
	s.configReloadNotifierChannel = ch
	return s
}

// HandleReloadCommand handles the reload command.
func (s *Service) HandleReloadCommand(_ context.Context, stream *smux.Stream, _ []byte) error {
	if s.configReloadNotifierChannel == nil {
		return transport.WriteError(stream, fmt.Errorf("config reload notifier not set"))
	}

	s.configReloadNotifierChannel <- true

	return transport.WriteOK(stream, nil)
}
