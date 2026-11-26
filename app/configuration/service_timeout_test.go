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

//go:build linux

package configuration

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"go.qbee.io/agent/app/api"
	"go.qbee.io/agent/app/utils/assert"
)

func TestService_Execute_StopsAtExecutionTimeout(t *testing.T) {
	dir := t.TempDir()

	// reports can't be delivered, so they end up in the reports buffer where we can check them
	srv := New(api.NewClient("localhost", "99999"), dir, t.TempDir()).
		WithLockFile(filepath.Join(dir, "config.lock"))
	srv.reportingEnabled.Store(true)

	// the setting doesn't accept values below 5 minutes, so the time limit is set directly
	srv.executionTimeout.Store(int64(time.Second))

	contents := []byte("distributed file\n")
	digest := sha256.Sum256(contents)
	source := filepath.Join(dir, "source.txt")
	destination := filepath.Join(dir, "destination.txt")
	assert.NoError(t, os.WriteFile(source, contents, 0600))

	config := &CommittedConfig{
		CommitID: "test-commit",
		Bundles:  []string{BundleFileDistribution, BundleConnectivityWatchdog},
		BundleData: BundleData{
			FileDistribution: &FileDistributionBundle{
				Metadata: Metadata{Enabled: true},
				FileSets: []FileSet{{
					Files: []File{{
						Source:      "file://" + source,
						Destination: destination,
						Digest:      hex.EncodeToString(digest[:]),
						Size:        int64(len(contents)),
					}},
					// the background process keeps the output open, so the command runs until it's killed
					AfterCommand: "sleep 600 &",
				}},
			},
			ConnectivityWatchdog: &ConnectivityWatchdogBundle{
				Metadata:  Metadata{Enabled: true},
				Threshold: "5",
			},
		},
	}

	start := time.Now()
	done := make(chan error, 1)
	go func() { done <- srv.Execute(t.Context(), config) }()

	select {
	case err := <-done:
		// sending the reports fails, as there's no API to send them to
		assert.True(t, err != nil)
	case <-time.After(30 * time.Second):
		t.Fatalf("configuration execution didn't stop at its time limit")
	}

	assert.True(t, time.Since(start) < 15*time.Second)

	// the file was installed before the after-command was killed
	installed, err := os.ReadFile(destination)
	assert.NoError(t, err)
	assert.Equal(t, installed, contents)

	// the bundle after the time limit wasn't executed
	assert.Equal(t, srv.connectivityWatchdogThreshold.Load(), int64(0))

	// the execution released its lock
	_, err = os.Stat(srv.lockFilePath())
	assert.True(t, os.IsNotExist(err))

	// the time limit was reported
	reports, err := srv.readReportsBuffer()
	assert.NoError(t, err)

	reported := false
	for _, report := range reports {
		if strings.HasPrefix(report.Text, "Configuration execution exceeded time limit of 1s.") {
			reported = true
		}
	}
	assert.True(t, reported)
}

// newBlockingExecution returns a service reporting into its reports buffer and a configuration with a single
// bundle, whose after-command runs until it's killed.
func newBlockingExecution(t *testing.T) (*Service, *CommittedConfig) {
	dir := t.TempDir()

	srv := New(api.NewClient("localhost", "99999"), dir, t.TempDir()).
		WithLockFile(filepath.Join(dir, "config.lock"))
	srv.reportingEnabled.Store(true)

	contents := []byte("distributed file\n")
	digest := sha256.Sum256(contents)
	source := filepath.Join(dir, "source.txt")
	assert.NoError(t, os.WriteFile(source, contents, 0600))

	config := &CommittedConfig{
		CommitID: "test-commit",
		Bundles:  []string{BundleFileDistribution},
		BundleData: BundleData{
			FileDistribution: &FileDistributionBundle{
				Metadata: Metadata{Enabled: true},
				FileSets: []FileSet{{
					Files: []File{{
						Source:      "file://" + source,
						Destination: filepath.Join(dir, "destination.txt"),
						Digest:      hex.EncodeToString(digest[:]),
						Size:        int64(len(contents)),
					}},
					AfterCommand: "sleep 600 &",
				}},
			},
		},
	}

	return srv, config
}

// timeLimitReports returns how many times the execution reported exceeding its time limit.
func timeLimitReports(t *testing.T, srv *Service) int {
	reports, err := srv.readReportsBuffer()
	assert.NoError(t, err)

	count := 0
	for _, report := range reports {
		if strings.HasPrefix(report.Text, "Configuration execution exceeded time limit") {
			count++
		}
	}

	return count
}

func TestService_Execute_ReportsTimeLimitInLastBundle(t *testing.T) {
	srv, config := newBlockingExecution(t)
	srv.executionTimeout.Store(int64(500 * time.Millisecond))

	// the only bundle is cut off by the time limit, which is reported once
	_ = srv.Execute(t.Context(), config)

	assert.Equal(t, timeLimitReports(t, srv), 1)
}

func TestService_Execute_DoesNotReportCancellationAsTimeLimit(t *testing.T) {
	srv, config := newBlockingExecution(t)

	// the execution is cancelled (e.g. the agent is stopping) well before its time limit
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	time.AfterFunc(500*time.Millisecond, cancel)

	_ = srv.Execute(ctx, config)

	assert.Equal(t, timeLimitReports(t, srv), 0)
}

func TestService_Execute_DoesNotReportCallerDeadlineAsTimeLimit(t *testing.T) {
	srv, config := newBlockingExecution(t)

	// the caller's deadline is reached well before the execution's own time limit
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	_ = srv.Execute(ctx, config)

	assert.Equal(t, timeLimitReports(t, srv), 0)
}

func TestService_Execute_KeepsLockFreshAfterCancellation(t *testing.T) {
	defaultInterval := lockRefreshInterval
	lockRefreshInterval = 10 * time.Millisecond
	t.Cleanup(func() { lockRefreshInterval = defaultInterval })

	// the file is downloaded from a FIFO, so reading it blocks without being cancellable
	srv, config := newBlockingExecution(t)
	fifo := filepath.Join(t.TempDir(), "source")
	assert.NoError(t, syscall.Mkfifo(fifo, 0600))
	config.BundleData.FileDistribution.FileSets[0].Files[0].Source = "file://" + fifo

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = srv.Execute(ctx, config)
		close(done)
	}()

	// opening the write end without blocking only succeeds once the execution opened the FIFO for reading
	var writer *os.File
	deadline := time.Now().Add(10 * time.Second)
	for writer == nil {
		if time.Now().After(deadline) {
			t.Fatal("execution didn't start reading the file")
		}

		if file, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			writer = file
		} else {
			time.Sleep(10 * time.Millisecond)
		}
	}
	defer func() { _ = writer.Close() }()

	// the execution is cancelled (e.g. the agent is stopping), but is still blocked reading the file
	cancel()

	modTime := time.Now().Add(-lockStaleTimeout - time.Minute)
	assert.NoError(t, os.Chtimes(srv.lockFilePath(), modTime, modTime))
	time.Sleep(100 * time.Millisecond)

	// the lock is still refreshed, so other processes don't take it over while the execution is running
	select {
	case <-done:
		t.Fatal("execution didn't block reading the file")
	default:
	}

	stat, err := os.Stat(srv.lockFilePath())
	assert.NoError(t, err)
	assert.True(t, time.Since(stat.ModTime()) < time.Minute)

	// once the read finishes, the execution ends and releases its lock
	assert.NoError(t, writer.Close())

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("execution didn't finish")
	}

	_, err = os.Stat(srv.lockFilePath())
	assert.True(t, os.IsNotExist(err))
}

func TestService_Execute_StopsDownloadAtExecutionTimeout(t *testing.T) {
	const fileSize = 1 << 30

	var reportsMutex sync.Mutex
	var reports []string
	var downloadStarted atomic.Bool

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/org/device/auth/filemetadata/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"ok","data":{"md5":"md5","last_modified":1,"size":`+strconv.Itoa(fileSize)+`}}`)
	})
	mux.HandleFunc("/v1/org/device/auth/files/", func(w http.ResponseWriter, r *http.Request) {
		// the download is slow, so it would take much longer than the execution's time limit
		downloadStarted.Store(true)
		w.Header().Set("Content-Length", strconv.Itoa(fileSize))
		w.WriteHeader(http.StatusOK)

		giveUp := time.After(30 * time.Second)
		for {
			select {
			case <-r.Context().Done():
				return
			case <-giveUp:
				return
			case <-time.After(50 * time.Millisecond):
				_, _ = w.Write(make([]byte, 1024))
				w.(http.Flusher).Flush()
			}
		}
	})
	mux.HandleFunc("/v1/org/device/auth/report", func(w http.ResponseWriter, r *http.Request) {
		if body, err := gzip.NewReader(r.Body); err == nil {
			data, _ := io.ReadAll(body)

			reportsMutex.Lock()
			reports = append(reports, string(data))
			reportsMutex.Unlock()
		}
		_, _ = io.WriteString(w, "{}")
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	assert.NoError(t, err)
	host, port, err := net.SplitHostPort(serverURL.Host)
	assert.NoError(t, err)

	tlsConfig := server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	apiClient := api.NewClient(host, port).WithTLSConfig(tlsConfig)

	dir := t.TempDir()
	srv := New(apiClient, dir, t.TempDir()).WithLockFile(filepath.Join(dir, "config.lock"))
	srv.reportingEnabled.Store(true)
	srv.executionTimeout.Store(int64(time.Second))

	digest := sha256.Sum256([]byte("contents"))
	config := &CommittedConfig{
		CommitID: "test-commit",
		Bundles:  []string{BundleFileDistribution},
		BundleData: BundleData{
			FileDistribution: &FileDistributionBundle{
				Metadata: Metadata{Enabled: true},
				FileSets: []FileSet{{
					Files: []File{{
						Source:      "files/large.bin",
						Destination: filepath.Join(dir, "large.bin"),
						Digest:      hex.EncodeToString(digest[:]),
						Size:        fileSize,
					}},
				}},
			},
		},
	}

	done := make(chan struct{})
	go func() {
		_ = srv.Execute(t.Context(), config)
		close(done)
	}()

	// the download is cut off by the execution's time limit
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("download didn't stop at the execution's time limit")
	}

	assert.True(t, downloadStarted.Load())

	reportsMutex.Lock()
	sent := strings.Join(reports, "\n")
	reportsMutex.Unlock()
	assert.True(t, strings.Contains(sent, "Configuration execution exceeded time limit of 1s."))

	_, err = os.Stat(srv.lockFilePath())
	assert.True(t, os.IsNotExist(err))
}
