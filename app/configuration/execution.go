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

package configuration

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"go.qbee.io/agent/app/log"
)

const (
	// downloadStallTimeout defines for how long a download may receive no data before it is considered hanging.
	downloadStallTimeout = 2 * time.Minute

	// executionWatchdogInterval defines how often the execution watchdog checks the execution state.
	executionWatchdogInterval = 10 * time.Second
)

// errExecutionTimeExceeded is returned for work which was stopped, or not started, because configuration execution
// exceeded its time limit.
var errExecutionTimeExceeded = errors.New("configuration execution time limit exceeded")

// transfer is a file download in progress.
type transfer struct {
	src string

	// lastProgress is the time (in unix nanoseconds) the transfer last received data.
	lastProgress atomic.Int64
}

// executionMonitor enforces the time limit of a configuration execution.
//
// When the time limit is reached, the execution is cancelled, unless file downloads are still in progress and
// are not hanging (i.e. received data within the stall timeout). In that case, the execution goes into overtime:
// no new downloads are started and the execution is cancelled as soon as there are no progressing downloads left.
type executionMonitor struct {
	timeout       time.Duration
	stallTimeout  time.Duration
	checkInterval time.Duration

	// onOvertimeTick is called on every watchdog check during overtime.
	onOvertimeTick func()

	overtime atomic.Bool

	mutex     sync.Mutex
	transfers map[*transfer]struct{}
}

// newExecutionMonitor returns a new execution monitor for the provided timeout.
func newExecutionMonitor(timeout time.Duration) *executionMonitor {
	return &executionMonitor{
		timeout:       timeout,
		stallTimeout:  downloadStallTimeout,
		checkInterval: executionWatchdogInterval,
		transfers:     make(map[*transfer]struct{}),
	}
}

// inOvertime returns true when the execution exceeded its time limit.
func (m *executionMonitor) inOvertime() bool {
	return m.overtime.Load()
}

// startTransfer registers a new download.
// Returns errExecutionTimeExceeded when execution is in overtime, as no new downloads are allowed then.
func (m *executionMonitor) startTransfer(src string) (*transfer, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.inOvertime() {
		return nil, errExecutionTimeExceeded
	}

	t := &transfer{src: src}
	t.lastProgress.Store(time.Now().UnixNano())
	m.transfers[t] = struct{}{}

	return t, nil
}

// finishTransfer de-registers a download.
func (m *executionMonitor) finishTransfer(t *transfer) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.transfers, t)
}

// progressingTransfer returns a download which received data within the stall timeout (if any).
func (m *executionMonitor) progressingTransfer(now time.Time) *transfer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for t := range m.transfers {
		if now.Sub(time.Unix(0, t.lastProgress.Load())) < m.stallTimeout {
			return t
		}
	}

	return nil
}

// watch the execution and call cancel when it should be stopped.
// Returns when ctx is done or after cancel was called.
func (m *executionMonitor) watch(ctx context.Context, cancel context.CancelFunc) {
	deadline := time.Now().Add(m.timeout)

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if now.Before(deadline) {
				continue
			}

			t := m.progressingTransfer(now)
			if t == nil {
				log.Warnf("configuration execution exceeded time limit of %s - stopping", m.timeout)
				m.overtime.Store(true)
				cancel()
				return
			}

			if !m.overtime.Swap(true) {
				log.Infof("configuration execution exceeded time limit of %s - waiting for download of %s to finish",
					m.timeout, t.src)
			}

			if m.onOvertimeTick != nil {
				m.onOvertimeTick()
			}
		}
	}
}

type executionMonitorContextKey struct{}

// withExecutionMonitor returns a copy of ctx with the execution monitor attached.
func withExecutionMonitor(ctx context.Context, m *executionMonitor) context.Context {
	return context.WithValue(ctx, executionMonitorContextKey{}, m)
}

// executionMonitorFromContext returns the execution monitor attached to ctx (if any).
func executionMonitorFromContext(ctx context.Context) *executionMonitor {
	m, _ := ctx.Value(executionMonitorContextKey{}).(*executionMonitor)
	return m
}

// progressReader records download progress on every successful read.
type progressReader struct {
	reader   io.Reader
	transfer *transfer
}

// Read implements io.Reader.
func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.transfer.lastProgress.Store(time.Now().UnixNano())
	}

	return n, err
}
