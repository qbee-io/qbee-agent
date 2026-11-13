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
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"go.qbee.io/agent/app/utils/assert"
)

// newTestExecutionMonitor returns an execution monitor with short intervals suitable for tests.
func newTestExecutionMonitor(timeout time.Duration) *executionMonitor {
	m := newExecutionMonitor(timeout)
	m.stallTimeout = 100 * time.Millisecond
	m.checkInterval = 5 * time.Millisecond
	return m
}

// startWatch starts the monitor watchdog and returns the execution context and a channel closed when watch returns.
func startWatch(t *testing.T, m *executionMonitor) (context.Context, <-chan struct{}) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	done := make(chan struct{})
	go func() {
		defer close(done)
		m.watch(ctx, cancel)
	}()

	return ctx, done
}

func waitFor(t *testing.T, ch <-chan struct{}, timeout time.Duration) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatalf("timed out after %s", timeout)
	}
}

func Test_executionMonitor_CancelsAtDeadlineWithoutDownloads(t *testing.T) {
	m := newTestExecutionMonitor(30 * time.Millisecond)

	start := time.Now()
	ctx, done := startWatch(t, m)

	waitFor(t, ctx.Done(), time.Second)
	waitFor(t, done, time.Second)

	assert.True(t, time.Since(start) >= 30*time.Millisecond)
	assert.True(t, m.inOvertime())

	_, err := m.startTransfer("file.txt")
	assert.True(t, errors.Is(err, errExecutionTimeExceeded))
}

func Test_executionMonitor_WaitsForProgressingDownload(t *testing.T) {
	m := newTestExecutionMonitor(30 * time.Millisecond)

	tr, err := m.startTransfer("file.txt")
	assert.NoError(t, err)

	ctx, done := startWatch(t, m)

	// keep the download progressing well past the deadline
	progressUntil := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(progressUntil) {
		tr.lastProgress.Store(time.Now().UnixNano())

		if ctx.Err() != nil {
			t.Fatalf("execution cancelled while download was progressing")
		}

		time.Sleep(10 * time.Millisecond)
	}

	assert.True(t, m.inOvertime())

	// no new downloads are allowed in overtime
	_, err = m.startTransfer("other.txt")
	assert.True(t, errors.Is(err, errExecutionTimeExceeded))

	// once the download finishes, execution is cancelled
	m.finishTransfer(tr)
	waitFor(t, ctx.Done(), time.Second)
	waitFor(t, done, time.Second)
}

func Test_executionMonitor_CancelsStalledDownload(t *testing.T) {
	m := newTestExecutionMonitor(30 * time.Millisecond)

	// registered, but never progressing
	_, err := m.startTransfer("file.txt")
	assert.NoError(t, err)

	start := time.Now()
	ctx, done := startWatch(t, m)

	waitFor(t, ctx.Done(), time.Second)
	waitFor(t, done, time.Second)

	// cancelled once the download is considered stalled
	assert.True(t, time.Since(start) >= m.stallTimeout)
}

func Test_executionMonitor_CallsOvertimeTick(t *testing.T) {
	m := newTestExecutionMonitor(10 * time.Millisecond)

	ticked := make(chan struct{})
	var once bool
	m.onOvertimeTick = func() {
		if !once {
			once = true
			close(ticked)
		}
	}

	tr, err := m.startTransfer("file.txt")
	assert.NoError(t, err)

	_, done := startWatch(t, m)
	waitFor(t, ticked, time.Second)

	m.finishTransfer(tr)
	waitFor(t, done, time.Second)
}

// overtimeReader switches the monitor to overtime on the first read, emulating deadline passing mid-download.
type overtimeReader struct {
	reader  io.Reader
	monitor *executionMonitor
}

func (r *overtimeReader) Read(p []byte) (int, error) {
	r.monitor.overtime.Store(true)
	return r.reader.Read(p)
}

func Test_downloadHTTP_DownloadCompletedInOvertimeIsInstalledNextRun(t *testing.T) {
	contents := testFileContents()
	srv, mock := newMockedFileService(t)

	m := newExecutionMonitor(time.Hour)
	mock.AddHandler(func(req *http.Request) (*http.Response, error) {
		return fileResponse(http.StatusOK, &overtimeReader{reader: bytes.NewReader(contents), monitor: m}), nil
	})

	dst := filepath.Join(t.TempDir(), "file.txt")
	fileMetadata := testFileMetadata(contents)
	partialPath := GetPartialDownloadFilePath(dst, fileMetadata.SHA256())

	ctx := withExecutionMonitor(t.Context(), m)

	created, err := srv.downloadMetadataCompare(ctx, "", "file.txt", dst, fileMetadata)
	assert.True(t, errors.Is(err, errExecutionTimeExceeded))
	assert.False(t, created)
	assertNotExist(t, dst)
	assertFileContents(t, partialPath, contents)

	// downloads are not started in overtime
	other := filepath.Join(t.TempDir(), "other.txt")
	created, err = srv.downloadMetadataCompare(ctx, "", "file.txt", other, fileMetadata)
	assert.True(t, errors.Is(err, errExecutionTimeExceeded))
	assert.False(t, created)

	// next run installs the complete partial download without requesting it again
	created, err = srv.downloadMetadataCompare(withExecutionMonitor(t.Context(), newExecutionMonitor(time.Hour)),
		"", "file.txt", dst, fileMetadata)
	assert.NoError(t, err)
	assert.True(t, created)
	assertFileContents(t, dst, contents)
	assertNoPartialDownload(t, dst, fileMetadata)
}

func Test_downloadHTTP_TransferIsTracked(t *testing.T) {
	contents := testFileContents()
	srv, mock := newMockedFileService(t)

	m := newExecutionMonitor(time.Hour)

	var trackedDuringDownload bool
	mock.AddHandler(func(req *http.Request) (*http.Response, error) {
		trackedDuringDownload = m.progressingTransfer(time.Now()) != nil
		return fileResponse(http.StatusOK, bytes.NewReader(contents)), nil
	})

	dst := filepath.Join(t.TempDir(), "file.txt")

	created, err := srv.downloadMetadataCompare(withExecutionMonitor(t.Context(), m), "", "file.txt", dst,
		testFileMetadata(contents))
	assert.NoError(t, err)
	assert.True(t, created)
	assert.True(t, trackedDuringDownload)

	// transfer is de-registered once finished
	assert.True(t, m.progressingTransfer(time.Now()) == nil)
}
