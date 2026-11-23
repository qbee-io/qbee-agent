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
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"go.qbee.io/agent/app/log"
)

const (
	// downloadStallTimeout defines for how long a download may receive no data before it is cancelled as hanging.
	downloadStallTimeout = 2 * time.Minute
)

// errExecutionTimeExceeded is returned for work which was stopped, or not started, because configuration execution
// exceeded its time limit.
var errExecutionTimeExceeded = errors.New("configuration execution time limit exceeded")

// errDownloadStalled is returned for downloads which were cancelled, because they received no data within the
// stall timeout.
var errDownloadStalled = errors.New("download received no data within the stall timeout")

// executionMonitor enforces the time limit of a configuration execution.
//
// When the time limit is reached, the execution is cancelled, unless file downloads are still in progress.
// Downloads are cancelled when they receive no data within the stall timeout (see stallWatch), so a download in
// progress is not hanging. In that case, the execution goes into overtime: no new downloads are started and
// the execution is cancelled as soon as there are no downloads in progress left.
type executionMonitor struct {
	timeout      time.Duration
	deadline     time.Time
	stallTimeout time.Duration

	overtime atomic.Bool

	// mutex protects transfers and makes the overtime transition atomic with download registration,
	// so a download is either rejected or awaited
	mutex sync.Mutex

	// transfers is the number of downloads in progress
	transfers int

	// cancel stops the execution (set by watch)
	cancel context.CancelFunc
}

// newExecutionMonitor returns a new execution monitor for an execution starting now, limited by timeout.
func newExecutionMonitor(timeout time.Duration) *executionMonitor {
	return &executionMonitor{
		timeout:      timeout,
		deadline:     time.Now().Add(timeout),
		stallTimeout: downloadStallTimeout,
	}
}

// inOvertime returns true when the execution exceeded its time limit.
func (m *executionMonitor) inOvertime() bool {
	return m.overtime.Load()
}

// pastDeadline returns true when the execution exceeded its time limit, switching it into overtime.
// Unlike inOvertime, it doesn't depend on the watchdog having noticed the deadline yet, so work finishing or
// starting just after the deadline is treated consistently.
func (m *executionMonitor) pastDeadline() bool {
	if m.overtime.Load() {
		return true
	}

	if time.Now().Before(m.deadline) {
		return false
	}

	m.overtime.Store(true)

	return true
}

// startTransfer registers a new download.
// Returns errExecutionTimeExceeded when the deadline passed, as no new downloads are allowed then.
func (m *executionMonitor) startTransfer() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// the deadline is checked here as well, so no download is started between the deadline and the watchdog
	// noticing it
	if m.pastDeadline() {
		return errExecutionTimeExceeded
	}

	m.transfers++

	return nil
}

// finishTransfer de-registers a download.
// In overtime, the execution is stopped as soon as the last download in progress finished, so no further work
// is done past the deadline.
func (m *executionMonitor) finishTransfer() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.transfers--

	if m.transfers == 0 && m.inOvertime() && m.cancel != nil {
		log.Warnf("configuration execution exceeded time limit of %s - downloads finished, stopping", m.timeout)
		m.cancel()
	}
}

// enterOvertime switches the execution into overtime and returns the number of downloads in progress.
func (m *executionMonitor) enterOvertime() int {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.overtime.Store(true)

	return m.transfers
}

// watch the execution and call cancel when it should be stopped: at the deadline, or when the deadline passed
// while downloads were in progress, once they finished (see finishTransfer).
// Returns when ctx is done.
func (m *executionMonitor) watch(ctx context.Context, cancel context.CancelFunc) {
	m.mutex.Lock()
	m.cancel = cancel
	m.mutex.Unlock()

	deadline := time.NewTimer(time.Until(m.deadline))
	defer deadline.Stop()

	select {
	case <-ctx.Done():
		return
	case <-deadline.C:
	}

	transfers := m.enterOvertime()
	if transfers == 0 {
		log.Warnf("configuration execution exceeded time limit of %s - stopping", m.timeout)
		cancel()
		return
	}

	// download sources are not logged, as they may contain secrets
	log.Infof("configuration execution exceeded time limit of %s - waiting for %d download(s) to finish",
		m.timeout, transfers)

	<-ctx.Done()
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

// checkExecutionDeadline returns errExecutionTimeExceeded when the execution of ctx (if any) is past its deadline.
// File operations check it before acting, so nothing is installed after the deadline, even when it needs no
// download (e.g. a file which was already downloaded completely by a previous run, or is in the cache).
func checkExecutionDeadline(ctx context.Context) error {
	if m := executionMonitorFromContext(ctx); m != nil && m.pastDeadline() {
		return errExecutionTimeExceeded
	}

	return nil
}

// stallWatch cancels a download which receives no data within the stall timeout.
type stallWatch struct {
	ctx     context.Context
	cancel  context.CancelCauseFunc
	timer   *time.Timer
	timeout time.Duration
}

// watchStall returns a stall watch, whose context is used for the download.
// The context is cancelled when the download receives no data within timeout. Call stop when the download is done.
func watchStall(ctx context.Context, timeout time.Duration) *stallWatch {
	w := &stallWatch{timeout: timeout}
	w.ctx, w.cancel = context.WithCancelCause(ctx)
	w.timer = time.AfterFunc(timeout, func() { w.cancel(errDownloadStalled) })

	return w
}

// progress records that the download received data.
func (w *stallWatch) progress() {
	w.timer.Reset(w.timeout)
}

// stop the stall watch.
func (w *stallWatch) stop() {
	w.timer.Stop()
	w.cancel(nil)
}

// wrap returns err, marked as errDownloadStalled if the download was cancelled for receiving no data.
func (w *stallWatch) wrap(err error) error {
	if errors.Is(context.Cause(w.ctx), errDownloadStalled) {
		return fmt.Errorf("%w: %w", errDownloadStalled, err)
	}

	return err
}

// progressReader records download progress on every successful read.
type progressReader struct {
	reader io.Reader
	stall  *stallWatch
}

// Read implements io.Reader.
func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.stall.progress()
	}

	return n, err
}
