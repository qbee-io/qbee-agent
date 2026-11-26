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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
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

func TestAgent_rebootIfScheduled_DeduplicatesRequests(t *testing.T) {
	agent, _ := newRebootTestAgent(t, nil)

	agent.rebootIfScheduled()

	// the first request is consumed, as the scheduler would do
	<-agent.reboot

	// another completion path requests the reboot again, while the first one is in flight
	agent.rebootIfScheduled()

	select {
	case <-agent.reboot:
		t.Fatalf("duplicate reboot request queued")
	default:
	}
}

func TestAgent_rebootIfScheduled_NotScheduled(t *testing.T) {
	agent := &Agent{
		reboot:        make(chan bool, 1),
		Configuration: configuration.New(nil, t.TempDir(), t.TempDir()),
	}

	agent.rebootIfScheduled()

	assert.Equal(t, len(agent.reboot), 0)
	assert.False(t, agent.rebootPending.Load())
}

func TestAgent_RebootSystem_WaitsForOngoingProcessing(t *testing.T) {
	for _, tt := range []struct {
		name string
		lock func(agent *Agent) func()
	}{
		{
			name: "configuration execution",
			lock: func(agent *Agent) func() { agent.configLock.Lock(); return agent.configLock.Unlock },
		},
		{
			name: "check-in",
			lock: func(agent *Agent) func() { agent.lock.Lock(); return agent.lock.Unlock },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			agent, calls := newRebootTestAgent(t, nil)

			unlock := tt.lock(agent)

			done := make(chan struct{})
			go func() {
				defer close(done)
				agent.RebootSystem(t.Context())
			}()

			time.Sleep(50 * time.Millisecond)
			assert.Equal(t, calls.Load(), int32(0))

			unlock()

			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatalf("reboot didn't proceed after processing finished")
			}

			assert.Equal(t, calls.Load(), int32(1))
			assert.Equal(t, len(agent.stop), 1)
		})
	}
}

func TestAgent_RebootPending_BlocksConfigurationExecution(t *testing.T) {
	agent, _ := newRebootTestAgent(t, nil)
	agent.rebootPending.Store(true)

	// would fail on the nil configuration, if it was executed
	agent.startConfigRun(t.Context(), nil)

	// lock is released immediately, so no execution is running
	assert.True(t, agent.configLock.TryLock())
	agent.configLock.Unlock()
}

func TestAgent_RebootScheduledByConfigExecution_BlocksNextExecution(t *testing.T) {
	a := newCheckInTestAgent(t)
	a.executeConfig = func(ctx context.Context, _ *configuration.CommittedConfig) error {
		a.executions.Add(1)
		a.Configuration.RebootAfterRun(ctx)
		return nil
	}

	a.checkIn(t)
	a.Wait()

	// the execution requested the reboot
	assert.True(t, a.rebootPending.Load())
	assert.Equal(t, len(a.reboot), 1)

	// no execution is started while the reboot is pending
	a.checkIn(t)
	a.Wait()
	assert.Equal(t, a.executions.Load(), int32(1))
}

func TestAgent_RebootSystem_FailureAllowsRetry(t *testing.T) {
	agent, calls := newRebootTestAgent(t, errors.New("reboot failed"))

	agent.rebootIfScheduled()
	<-agent.reboot
	agent.RebootSystem(t.Context())

	assert.Equal(t, calls.Load(), int32(1))
	assert.False(t, agent.rebootPending.Load())
	assert.Equal(t, len(agent.stop), 0)

	// next agent run retries the reboot
	agent.rebootIfScheduled()
	assert.Equal(t, len(agent.reboot), 1)
}

func TestAgent_RebootSystem_DoesNotBlockOnQueuedStop(t *testing.T) {
	agent, _ := newRebootTestAgent(t, nil)

	// stop already queued (e.g. by a signal)
	agent.stop <- true

	done := make(chan struct{})
	go func() {
		defer close(done)
		agent.RebootSystem(t.Context())
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("reboot blocked on queued agent stop")
	}
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

	// executionStarted and inventoriesStarted receive a value when the corresponding work starts
	executionStarted   chan struct{}
	inventoriesStarted chan struct{}

	// releaseExecution and releaseInventories block the corresponding work until closed (if set)
	releaseExecution   chan struct{}
	releaseInventories chan struct{}

	// order records the order in which the work was done
	order chan string

	// configResponse is the configuration returned by the mocked API (if set)
	configResponse []byte

	// failConfigRequests makes configuration requests fail with a connection error
	failConfigRequests atomic.Bool
}

func newCheckInTestAgent(t *testing.T) *checkInTestAgent {
	apiClient, mock := api.NewMockedClient()

	a := &checkInTestAgent{
		mock:               mock,
		executionStarted:   make(chan struct{}, 10),
		inventoriesStarted: make(chan struct{}, 10),
		order:              make(chan string, 10),
	}

	a.Agent = &Agent{
		update:              make(chan bool, 1),
		stop:                make(chan bool, 1),
		reboot:              make(chan bool, 1),
		api:                 apiClient,
		Configuration:       configuration.New(apiClient, t.TempDir(), t.TempDir()).WithLockFile(filepath.Join(t.TempDir(), "config.lock")),
		Metrics:             metrics.New(apiClient),
		disableRemoteAccess: true,
	}

	release := func(ch chan struct{}) {
		if ch != nil {
			<-ch
		}
	}

	a.executeConfig = func(context.Context, *configuration.CommittedConfig) error {
		a.executions.Add(1)
		a.order <- "config"
		a.executionStarted <- struct{}{}
		release(a.releaseExecution)
		return nil
	}

	a.collectInventories = func(context.Context) error {
		a.inventories.Add(1)
		a.order <- "inventories"
		a.inventoriesStarted <- struct{}{}
		release(a.releaseInventories)
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

			if a.failConfigRequests.Load() {
				return nil, errors.New("connection refused")
			}

			body = `{"commit_id":"test-commit","bundles":[]}`
			if a.configResponse != nil {
				body = string(a.configResponse)
			}
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

// assertBlocked fails the test if ch is ready within a short period.
func assertBlocked(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()

	select {
	case <-ch:
		t.Fatalf("%s didn't wait", what)
	case <-time.After(50 * time.Millisecond):
	}
}

// waitDone runs Wait() in the background and returns a channel closed once it returns.
func (a *checkInTestAgent) waitDone() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		a.Wait()
	}()

	return done
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

func TestAgent_RunOnce_CheckInNotBlockedByConfigExecution(t *testing.T) {
	a := newCheckInTestAgent(t)
	a.releaseExecution = make(chan struct{})

	// first check-in starts the configuration execution, which keeps running
	a.checkIn(t)
	waitFor(t, a.executionStarted, "configuration execution to start")

	// further check-ins complete and deliver metrics while the execution is still running
	a.checkIn(t)
	a.checkIn(t)

	assert.Equal(t, a.configRequests.Load(), int32(3))
	assert.Equal(t, a.metricsRequests.Load(), int32(3))

	// no overlapping configuration execution was started
	assert.Equal(t, a.executions.Load(), int32(1))
	assert.Equal(t, a.inventories.Load(), int32(0))

	close(a.releaseExecution)
	waitFor(t, a.waitDone(), "Wait() to return")

	assert.Equal(t, a.executions.Load(), int32(1))
	assert.Equal(t, a.inventories.Load(), int32(1))

	// once the execution is finished, the next check-in starts a new one
	a.checkIn(t)
	waitFor(t, a.waitDone(), "Wait() to return")

	assert.Equal(t, a.executions.Load(), int32(2))
	assert.Equal(t, a.inventories.Load(), int32(2))
}

func TestAgent_Wait_WaitsForConfigExecutionAndInventories(t *testing.T) {
	a := newCheckInTestAgent(t)
	a.releaseExecution = make(chan struct{})
	a.releaseInventories = make(chan struct{})

	a.checkIn(t)
	waitFor(t, a.executionStarted, "configuration execution to start")

	// waits for the configuration execution
	waitDone := a.waitDone()
	assertBlocked(t, waitDone, "Wait() during configuration execution")

	// waits for inventories collection following the configuration execution
	close(a.releaseExecution)
	waitFor(t, a.inventoriesStarted, "inventories collection to start")
	assertBlocked(t, waitDone, "Wait() during inventories collection")

	close(a.releaseInventories)
	waitFor(t, waitDone, "Wait() to return")

	// inventories are collected after the configuration execution
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

// realExecution is a configuration which is executed for real. It distributes a file from a FIFO,
// so the execution blocks while downloading the file, until the test provides the file contents.
type realExecution struct {
	source   string
	dst      string
	contents []byte
}

func newRealExecution(t *testing.T) *realExecution {
	dir := t.TempDir()

	r := &realExecution{
		source:   filepath.Join(dir, "source.fifo"),
		dst:      filepath.Join(dir, "destination.txt"),
		contents: []byte("distributed by a real configuration execution\n"),
	}

	assert.NoError(t, syscall.Mkfifo(r.source, 0600))

	return r
}

// config returns the configuration, with the connectivity watchdog enabled when watchdogThreshold is set.
func (r *realExecution) config(t *testing.T, watchdogThreshold string) []byte {
	digest := sha256.Sum256(r.contents)

	cfg := configuration.CommittedConfig{
		CommitID: "test-commit",
		Bundles:  []string{configuration.BundleSettings},
		BundleData: configuration.BundleData{
			// reporting is disabled, so only the check-in uses the mocked API
			Settings: configuration.SettingsBundle{
				Metadata:      configuration.Metadata{Enabled: true},
				EnableMetrics: true,
				RunInterval:   5,
			},
			FileDistribution: &configuration.FileDistributionBundle{
				Metadata: configuration.Metadata{Enabled: true},
				FileSets: []configuration.FileSet{{
					Files: []configuration.File{{
						Source:      "file://" + r.source,
						Destination: r.dst,
						Digest:      hex.EncodeToString(digest[:]),
						Size:        int64(len(r.contents)),
					}},
				}},
			},
		},
	}

	if watchdogThreshold != "" {
		cfg.Bundles = append(cfg.Bundles, configuration.BundleConnectivityWatchdog)
		cfg.BundleData.ConnectivityWatchdog = &configuration.ConnectivityWatchdogBundle{
			Metadata:  configuration.Metadata{Enabled: true},
			Threshold: watchdogThreshold,
		}
	}

	cfg.Bundles = append(cfg.Bundles, configuration.BundleFileDistribution)

	data, err := json.Marshal(cfg)
	assert.NoError(t, err)

	return data
}

// waitForDownload waits until the execution started downloading the file and returns the FIFO's write end.
func (r *realExecution) waitForDownload(t *testing.T) *os.File {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		// opening the write end without blocking only succeeds once the reader opened the FIFO
		writer, err := os.OpenFile(r.source, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			// if the test fails before finishing the download, closing the writer ends the execution,
			// so it doesn't keep holding the configuration lock
			t.Cleanup(func() { _ = writer.Close() })
			return writer
		}

		if !errors.Is(err, syscall.ENXIO) || time.Now().After(deadline) {
			t.Fatalf("configuration execution didn't start downloading the file: %v", err)
		}

		time.Sleep(5 * time.Millisecond)
	}
}

// finishDownload provides the file contents, which lets the execution finish.
func (r *realExecution) finishDownload(t *testing.T, writer *os.File) {
	t.Helper()

	_, err := writer.Write(r.contents)
	assert.NoError(t, err)
	assert.NoError(t, writer.Close())
}

// useRealExecution makes the agent execute configuration for real, counting the executions.
func (a *checkInTestAgent) useRealExecution() {
	a.executeConfig = func(ctx context.Context, configData *configuration.CommittedConfig) error {
		a.executions.Add(1)
		return a.Configuration.Execute(ctx, configData)
	}
}

func TestAgent_CheckInsDuringRealConfigExecution(t *testing.T) {
	exec := newRealExecution(t)

	a := newCheckInTestAgent(t)
	a.useRealExecution()
	a.configResponse = exec.config(t, "")

	// first check-in starts the execution, which blocks downloading the file
	a.checkIn(t)
	writer := exec.waitForDownload(t)

	// check-ins apply settings and deliver metrics while the real execution is running
	a.checkIn(t)
	a.checkIn(t)

	assert.Equal(t, a.configRequests.Load(), int32(3))
	assert.Equal(t, a.metricsRequests.Load(), int32(3))
	assert.Equal(t, a.executions.Load(), int32(1))

	exec.finishDownload(t, writer)
	a.Wait()

	assert.Equal(t, a.executions.Load(), int32(1))
	assert.Equal(t, a.inventories.Load(), int32(1))
	assert.Equal(t, a.Configuration.CurrentCommitID(), "test-commit")

	distributed, err := os.ReadFile(exec.dst)
	assert.NoError(t, err)
	assert.Equal(t, distributed, exec.contents)
}

func TestAgent_ConnectivityWatchdogRebootWaitsForRealConfigExecution(t *testing.T) {
	exec := newRealExecution(t)

	a := newCheckInTestAgent(t)
	a.useRealExecution()
	a.configResponse = exec.config(t, "1")

	rebootCalls := new(atomic.Int32)
	a.rebootCommand = func(context.Context) ([]byte, error) {
		rebootCalls.Add(1)
		return nil, nil
	}

	// the execution enables the connectivity watchdog, then blocks downloading the file
	a.checkIn(t)
	writer := exec.waitForDownload(t)

	// a check-in failing to reach the device hub triggers the watchdog, which schedules a reboot
	a.failConfigRequests.Store(true)
	a.checkIn(t)

	assert.True(t, a.Configuration.ShouldReboot())
	assert.Equal(t, len(a.reboot), 1)

	// the scheduler reboots the system, which waits for the execution to finish
	<-a.reboot
	rebooted := make(chan struct{})
	go func() {
		defer close(rebooted)
		a.RebootSystem(t.Context())
	}()

	assertBlocked(t, rebooted, "reboot during configuration execution")
	assert.Equal(t, rebootCalls.Load(), int32(0))

	exec.finishDownload(t, writer)
	waitFor(t, rebooted, "reboot after configuration execution")

	assert.Equal(t, rebootCalls.Load(), int32(1))
	assert.Equal(t, len(a.stop), 1)
	assert.Equal(t, a.executions.Load(), int32(1))
	assert.Equal(t, a.inventories.Load(), int32(1))

	distributed, err := os.ReadFile(exec.dst)
	assert.NoError(t, err)
	assert.Equal(t, distributed, exec.contents)
}
