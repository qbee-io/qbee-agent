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
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.qbee.io/agent/app/utils/assert"
)

// newTestExecutionMonitor returns an execution monitor with short intervals suitable for tests.
func newTestExecutionMonitor(timeout time.Duration) *executionMonitor {
	m := newExecutionMonitor(timeout)
	m.stallTimeout = 100 * time.Millisecond
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

	err := m.startTransfer()
	assert.True(t, errors.Is(err, errExecutionTimeExceeded))
}

func Test_executionMonitor_WaitsForDownloadInProgress(t *testing.T) {
	m := newTestExecutionMonitor(30 * time.Millisecond)

	err := m.startTransfer()
	assert.NoError(t, err)

	ctx, done := startWatch(t, m)

	// the download is still in progress well past the deadline
	select {
	case <-ctx.Done():
		t.Fatalf("execution cancelled while download was in progress")
	case <-time.After(300 * time.Millisecond):
	}

	assert.True(t, m.inOvertime())

	// no new downloads are allowed in overtime
	err = m.startTransfer()
	assert.True(t, errors.Is(err, errExecutionTimeExceeded))

	// once the download finishes, execution is cancelled
	m.finishTransfer()
	waitFor(t, ctx.Done(), time.Second)
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

// lateReader returns its data only after the monitor's deadline passed, emulating a download completing just
// after the deadline, before the watchdog noticed it.
type lateReader struct {
	reader  io.Reader
	monitor *executionMonitor
}

func (r *lateReader) Read(p []byte) (int, error) {
	time.Sleep(time.Until(r.monitor.deadline) + 10*time.Millisecond)
	return r.reader.Read(p)
}

func Test_downloadHTTP_DownloadCompletedPastDeadlineIsNotInstalled(t *testing.T) {
	contents := testFileContents()
	srv, mock := newMockedFileService(t)

	// the watchdog isn't running, so only the deadline itself can stop the download from being installed
	m := newExecutionMonitor(50 * time.Millisecond)
	mock.AddHandler(func(req *http.Request) (*http.Response, error) {
		return fileResponse(http.StatusOK, &lateReader{reader: bytes.NewReader(contents), monitor: m}), nil
	})

	dst := filepath.Join(t.TempDir(), "file.txt")
	fileMetadata := testFileMetadata(contents)
	partialPath := GetPartialDownloadFilePath(dst, fileMetadata.SHA256())

	created, err := srv.downloadMetadataCompare(withExecutionMonitor(t.Context(), m), "", "file.txt", dst,
		fileMetadata)
	assert.True(t, errors.Is(err, errExecutionTimeExceeded))
	assert.False(t, created)
	assertNotExist(t, dst)
	assertFileContents(t, partialPath, contents)
	assert.True(t, m.inOvertime())
}

// activeTransfers returns the number of downloads registered with the monitor.
func activeTransfers(m *executionMonitor) int {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.transfers
}

func Test_downloadHTTP_TransferIsTracked(t *testing.T) {
	contents := testFileContents()
	srv, mock := newMockedFileService(t)

	m := newExecutionMonitor(time.Hour)

	var trackedDuringDownload bool
	mock.AddHandler(func(req *http.Request) (*http.Response, error) {
		trackedDuringDownload = activeTransfers(m) == 1
		return fileResponse(http.StatusOK, bytes.NewReader(contents)), nil
	})

	dst := filepath.Join(t.TempDir(), "file.txt")

	created, err := srv.downloadMetadataCompare(withExecutionMonitor(t.Context(), m), "", "file.txt", dst,
		testFileMetadata(contents))
	assert.NoError(t, err)
	assert.True(t, created)
	assert.True(t, trackedDuringDownload)

	// transfer is de-registered once finished
	assert.Equal(t, activeTransfers(m), 0)
}

// trickleReader returns a single byte at a time, every interval, until data is exhausted.
// When data is exhausted, it blocks until ctx is done, emulating a hanging connection (if hang is set).
type trickleReader struct {
	ctx      context.Context
	data     []byte
	interval time.Duration
	hang     bool
}

func (r *trickleReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		if !r.hang {
			return 0, io.EOF
		}

		<-r.ctx.Done()
		return 0, r.ctx.Err()
	}

	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	case <-time.After(r.interval):
	}

	p[0] = r.data[0]
	r.data = r.data[1:]

	return 1, nil
}

func Test_downloadHTTP_StalledDownloadIsCancelled(t *testing.T) {
	contents := testFileContents()
	srv, mock := newMockedFileService(t)

	// half of the file is received, then the connection hangs
	mock.AddHandler(func(req *http.Request) (*http.Response, error) {
		reader := &trickleReader{ctx: req.Context(), data: contents[:len(contents)/2], hang: true}
		return fileResponse(http.StatusOK, reader), nil
	})

	m := newTestExecutionMonitor(time.Hour)
	dst := filepath.Join(t.TempDir(), "file.txt")
	fileMetadata := testFileMetadata(contents)

	start := time.Now()
	created, err := srv.downloadMetadataCompare(withExecutionMonitor(t.Context(), m), "", "file.txt", dst, fileMetadata)
	assert.True(t, errors.Is(err, errDownloadStalled))
	assert.False(t, created)
	assert.True(t, time.Since(start) >= m.stallTimeout)
	assertNotExist(t, dst)

	// the data received so far is kept, so the next run resumes the download
	partial, err := os.ReadFile(GetPartialDownloadFilePath(dst, fileMetadata.SHA256()))
	assert.NoError(t, err)
	assert.Equal(t, partial, contents[:len(contents)/2])
}

func Test_downloadHTTP_SlowDownloadIsNotCancelled(t *testing.T) {
	contents := []byte("slow download")
	srv, mock := newMockedFileService(t)

	m := newTestExecutionMonitor(time.Hour)

	// the whole download takes longer than the stall timeout, but data keeps coming
	interval := m.stallTimeout / 4
	mock.AddHandler(func(req *http.Request) (*http.Response, error) {
		return fileResponse(http.StatusOK, &trickleReader{ctx: req.Context(), data: contents, interval: interval}), nil
	})

	dst := filepath.Join(t.TempDir(), "file.txt")

	start := time.Now()
	created, err := srv.downloadMetadataCompare(withExecutionMonitor(t.Context(), m), "", "file.txt", dst,
		testFileMetadata(contents))
	assert.NoError(t, err)
	assert.True(t, created)
	assert.True(t, time.Since(start) > m.stallTimeout)
	assertFileContents(t, dst, contents)
}

func Test_executionMonitor_RejectsDownloadsPastDeadline(t *testing.T) {
	// the watchdog isn't running, so the deadline must be enforced when the download is registered
	m := newExecutionMonitor(10 * time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	err := m.startTransfer()
	assert.True(t, errors.Is(err, errExecutionTimeExceeded))
	assert.True(t, m.inOvertime())
	assert.Equal(t, activeTransfers(m), 0)
}

// pastDeadlineContext returns a context of an execution which is past its deadline, but which the watchdog
// didn't notice yet (it isn't running), so only the deadline itself can stop work from being done.
func pastDeadlineContext(t *testing.T) context.Context {
	return withExecutionMonitor(t.Context(), newExecutionMonitor(-time.Second))
}

func Test_downloadMetadataCompare_CompletePartialIsNotInstalledPastDeadline(t *testing.T) {
	contents := testFileContents()
	dst := filepath.Join(t.TempDir(), "file.txt")
	fileMetadata := testFileMetadata(contents)

	// a previous run downloaded the file completely, but didn't install it
	partialPath := GetPartialDownloadFilePath(dst, fileMetadata.SHA256())
	assert.NoError(t, os.Mkdir(filepath.Dir(partialPath), 0700))
	assert.NoError(t, os.WriteFile(partialPath, contents, 0600))

	created, err := new(Service).downloadMetadataCompare(pastDeadlineContext(t), "", "file.txt", dst, fileMetadata)
	assert.True(t, errors.Is(err, errExecutionTimeExceeded))
	assert.False(t, created)
	assertNotExist(t, dst)

	// it's kept for the next run
	assertFileContents(t, partialPath, contents)
}

func Test_downloadFile_CachedFileIsNotUsedPastDeadline(t *testing.T) {
	contents := testFileContents()
	cached := filepath.Join(t.TempDir(), "package.deb")
	assert.NoError(t, os.WriteFile(cached, contents, 0600))

	// callers act on the cached file when downloadFile succeeds (e.g. installing a package), so it must fail
	file := File{Digest: testFileMetadata(contents).SHA256(), Size: int64(len(contents))}
	created, err := new(Service).downloadFile(pastDeadlineContext(t), "", "package.deb", cached, file)
	assert.True(t, errors.Is(err, errExecutionTimeExceeded))
	assert.False(t, created)
}

func Test_downloadTemplateFile_IsNotRenderedPastDeadline(t *testing.T) {
	dir := t.TempDir()
	template := filepath.Join(dir, "template.txt")
	assert.NoError(t, os.WriteFile(template, []byte("value={{ key }}\n"), 0600))
	dst := filepath.Join(dir, "rendered.txt")

	created, err := new(Service).downloadTemplateFile(pastDeadlineContext(t), "", "file://"+template, dst, File{},
		map[string]string{"key": "value"})
	assert.True(t, errors.Is(err, errExecutionTimeExceeded))
	assert.False(t, created)
	assertNotExist(t, dst)
}

func Test_executionMonitor_StopsAsSoonAsDownloadsFinish(t *testing.T) {
	// production intervals, so the execution must not wait for a watchdog check
	m := newExecutionMonitor(20 * time.Millisecond)

	assert.NoError(t, m.startTransfer())
	ctx, done := startWatch(t, m)

	// the deadline passed, but the download in progress keeps the execution going
	time.Sleep(100 * time.Millisecond)
	assert.True(t, m.inOvertime())
	assert.NoError(t, ctx.Err())

	m.finishTransfer()
	waitFor(t, ctx.Done(), 100*time.Millisecond)
	waitFor(t, done, time.Second)
}
