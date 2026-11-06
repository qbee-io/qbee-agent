// Copyright 2026 qbee.io
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
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xtaci/smux"
	"go.qbee.io/transport"

	"go.qbee.io/agent/app/utils/assert"
)

func TestService_ConnectionErrorAndRecovery(t *testing.T) {
	var failing atomic.Bool
	failing.Store(true)
	connected := make(chan *smux.Session, 10)

	edge := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		session, err := transport.UpgradeHandler(w, r)
		if err != nil {
			return
		}
		connected <- session
		<-session.CloseChan()
	}))
	t.Cleanup(edge.Close)

	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(edge.Certificate())

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	service := New().WithTLSConfig(&tls.Config{ServerName: "example.com", RootCAs: caCertPool})
	t.Cleanup(func() { _ = service.Stop() })

	assert.NoError(t, service.UpdateState(ctx, edge.URL, true))

	// failure gets reported once with the error string
	var connErr error
	assert.EventuallyTrue(t, func() bool {
		connErr = service.ConnectionError()
		return connErr != nil
	}, 5*time.Second)
	assert.Equal(t, connErr.Error(), "failed to upgrade, got 503")
	assert.NoError(t, service.ConnectionError())

	// connection resumes and the flag is cleared
	failing.Store(false)
	var session *smux.Session
	select {
	case session = <-connected:
	case <-time.After(15 * time.Second):
		t.Fatalf("device did not reconnect")
	}

	assert.EventuallyTrue(t, func() bool {
		service.connErrMutex.Lock()
		defer service.connErrMutex.Unlock()
		return service.connErr == nil && !service.connErrReported
	}, time.Second)
	assert.NoError(t, service.ConnectionError())

	// a new outage within the report interval is not reported
	failing.Store(true)
	_ = session.Close()
	assert.EventuallyTrue(t, func() bool {
		service.connErrMutex.Lock()
		defer service.connErrMutex.Unlock()
		return service.connErr != nil
	}, 15*time.Second)
	assert.NoError(t, service.ConnectionError())

	// but is reported once the interval has passed
	service.connErrMutex.Lock()
	service.connErrReportedAt = time.Now().Add(-connErrReportInterval)
	service.connErrMutex.Unlock()
	assert.NotEqual(t, service.ConnectionError(), nil)
	assert.NoError(t, service.ConnectionError())
}
