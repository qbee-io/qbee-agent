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

package agent

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"go.qbee.io/agent/app/api"
	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/metrics"
	"go.qbee.io/agent/app/utils/assert"
)

// newRebootTestAgent returns an agent with a scheduled reboot and a fake reboot command.
func newRebootTestAgent(t *testing.T, rebootErr error) (*Agent, *atomic.Int32) {
	calls := new(atomic.Int32)

	agent := &Agent{
		update:        make(chan bool, 1),
		stop:          make(chan bool, 1),
		reboot:        make(chan bool, 1),
		Configuration: configuration.New(nil, t.TempDir(), t.TempDir()),
		rebootCommand: func(context.Context) ([]byte, error) {
			calls.Add(1)
			return nil, rebootErr
		},
	}

	agent.Configuration.RebootAfterRun(t.Context())

	return agent, calls
}
func TestAgent_RebootSystem_SchedulesAgentStop(t *testing.T) {
	agent, calls := newRebootTestAgent(t, nil)

	agent.RebootSystem(t.Context())

	assert.Equal(t, calls.Load(), int32(1))
	assert.Equal(t, len(agent.stop), 1)
}

func TestAgent_RebootSystem_FailureDoesNotStopAgent(t *testing.T) {
	agent, calls := newRebootTestAgent(t, errors.New("reboot failed"))

	agent.RebootSystem(t.Context())

	assert.Equal(t, calls.Load(), int32(1))
	assert.Equal(t, len(agent.stop), 0)
}

const (
	testConfigAPIPath  = "/v1/org/device/auth/config"
	testMetricsAPIPath = "/v1/org/device/auth/metric"
)

// checkInTestAgent is an agent with mocked device hub API and fake configuration execution and inventories.
type checkInTestAgent struct {
	*Agent

	mock *api.Mock

	// configRequests and metricsRequests count check-in requests received by the mocked API
	configRequests  atomic.Int32
	metricsRequests atomic.Int32

	// executions counts configuration executions, inventories counts inventories collections
	executions  atomic.Int32
	inventories atomic.Int32

	// order records the order in which the work was done
	order chan string
}

func newCheckInTestAgent(t *testing.T) *checkInTestAgent {
	apiClient, mock := api.NewMockedClient()

	a := &checkInTestAgent{
		mock:  mock,
		order: make(chan string, 10),
	}

	a.Agent = &Agent{
		update:              make(chan bool, 1),
		stop:                make(chan bool, 1),
		reboot:              make(chan bool, 1),
		api:                 apiClient,
		Configuration:       configuration.New(apiClient, t.TempDir(), t.TempDir()),
		Metrics:             metrics.New(apiClient),
		disableRemoteAccess: true,
	}

	a.executeConfig = func(context.Context, *configuration.CommittedConfig) error {
		a.executions.Add(1)
		a.order <- "config"
		return nil
	}

	a.collectInventories = func(context.Context) error {
		a.inventories.Add(1)
		a.order <- "inventories"
		return nil
	}

	return a
}

// checkIn runs a full agent run and fails the test if the check-in doesn't return promptly.
func (a *checkInTestAgent) checkIn(t *testing.T) {
	t.Helper()

	// respond to configuration and metrics requests, in whichever order they come
	handler := func(req *http.Request) (*http.Response, error) {
		body := ""
		switch req.URL.Path {
		case testConfigAPIPath:
			a.configRequests.Add(1)
			body = `{"commit_id":"test-commit","bundles":[]}`
		case testMetricsAPIPath:
			a.metricsRequests.Add(1)
		}

		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(body))}, nil
	}
	a.mock.AddHandler(handler)
	a.mock.AddHandler(handler)

	done := make(chan struct{})
	go func() {
		defer close(done)
		a.RunOnce(t.Context(), FullRun)
	}()

	waitFor(t, done, "check-in to return")
}

func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func TestAgent_RunOnce_ExecutesConfigBeforeInventories(t *testing.T) {
	a := newCheckInTestAgent(t)

	a.checkIn(t)
	a.Wait()

	assert.Equal(t, a.configRequests.Load(), int32(1))
	assert.Equal(t, a.metricsRequests.Load(), int32(1))
	assert.Equal(t, a.executions.Load(), int32(1))
	assert.Equal(t, a.inventories.Load(), int32(1))

	assert.Equal(t, <-a.order, "config")
	assert.Equal(t, <-a.order, "inventories")
}

func TestNewWithoutCredentials_SetsDefaultImplementations(t *testing.T) {
	agent, err := NewWithoutCredentials(&Config{Directory: t.TempDir(), StateDirectory: t.TempDir()})
	assert.NoError(t, err)
	t.Cleanup(agent.loopTicker.Stop)

	assert.True(t, agent.rebootCommand != nil)
	assert.True(t, agent.executeConfig != nil)
	assert.True(t, agent.collectInventories != nil)
}
