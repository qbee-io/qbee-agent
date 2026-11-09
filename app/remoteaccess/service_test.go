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

	reported := make(chan error, 10)
	service := New().
		WithTLSConfig(&tls.Config{ServerName: "example.com", RootCAs: caCertPool}).
		WithConnectionErrorReporter(func(_ context.Context, err error) { reported <- err })
	t.Cleanup(func() { _ = service.Stop() })

	assertNotReported := func() {
		t.Helper()
		select {
		case err := <-reported:
			t.Fatalf("unexpected report: %v", err)
		default:
		}
	}

	assert.NoError(t, service.UpdateState(ctx, edge.URL, true))

	// failure gets reported once with the error string, without any agent run
	select {
	case err := <-reported:
		assert.Equal(t, err.Error(), "failed to upgrade, got 503")
	case <-time.After(5 * time.Second):
		t.Fatalf("connection error was not reported")
	}

	// subsequent failures within the same outage are not reported
	assert.EventuallyTrue(t, func() bool {
		service.connErrReporter.mutex.Lock()
		defer service.connErrReporter.mutex.Unlock()
		return service.connErrReporter.reported
	}, time.Second)
	time.Sleep(11 * time.Second)
	assertNotReported()

	// connection resumes and the flag is cleared
	failing.Store(false)
	var session *smux.Session
	select {
	case session = <-connected:
	case <-time.After(15 * time.Second):
		t.Fatalf("device did not reconnect")
	}

	assert.EventuallyTrue(t, func() bool {
		service.connErrReporter.mutex.Lock()
		defer service.connErrReporter.mutex.Unlock()
		return !service.connErrReporter.reported
	}, time.Second)

	// a dropped connection is not reported, even after the report interval has passed
	service.connErrReporter.mutex.Lock()
	service.connErrReporter.reportedAt = time.Now().Add(-connErrReportInterval)
	service.connErrReporter.mutex.Unlock()
	_ = session.Close()
	select {
	case session = <-connected:
	case <-time.After(15 * time.Second):
		t.Fatalf("device did not reconnect")
	}
	assertNotReported()

	// a new outage within the report interval is not reported
	service.connErrReporter.mutex.Lock()
	service.connErrReporter.reportedAt = time.Now()
	service.connErrReporter.mutex.Unlock()
	failing.Store(true)
	_ = session.Close()
	time.Sleep(11 * time.Second)
	assertNotReported()

	// but is reported on the next failure once the interval has passed
	service.connErrReporter.mutex.Lock()
	service.connErrReporter.reportedAt = time.Now().Add(-connErrReportInterval)
	service.connErrReporter.mutex.Unlock()
	select {
	case err := <-reported:
		assert.Equal(t, err.Error(), "failed to upgrade, got 503")
	case <-time.After(15 * time.Second):
		t.Fatalf("connection error was not reported after the report interval")
	}
}
