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
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	// the execution doesn't count as in progress anymore and released its lock
	assert.Equal(t, srv.ActiveExecutionTimeout(), time.Duration(0))

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

func TestService_Execute_DownloadInProgressAtTimeout(t *testing.T) {
	dir := t.TempDir()

	srv, mock := newMockedFileService(t)
	srv.WithLockFile(filepath.Join(dir, "config.lock"))

	// the setting doesn't accept values below 5 minutes, so the time limit is set directly
	srv.executionTimeout.Store(int64(200 * time.Millisecond))

	// the download takes longer than the time limit (about 600ms), but keeps receiving data
	contents := []byte("downloaded across the deadline\n")
	mock.AddHandler(func(req *http.Request) (*http.Response, error) {
		body := &trickleReader{ctx: req.Context(), data: contents, interval: 20 * time.Millisecond}
		return fileResponse(http.StatusOK, body), nil
	})
	downloaded := filepath.Join(dir, "downloaded.txt")

	template := filepath.Join(dir, "template.txt")
	assert.NoError(t, os.WriteFile(template, []byte("value={{key}}\n"), 0600))
	rendered := filepath.Join(dir, "rendered.txt")

	afterCommandMarker := filepath.Join(dir, "after-command-ran")

	config := &CommittedConfig{
		CommitID: "test-commit",
		Bundles:  []string{BundleFileDistribution, BundleConnectivityWatchdog},
		BundleData: BundleData{
			FileDistribution: &FileDistributionBundle{
				Metadata: Metadata{Enabled: true},
				FileSets: []FileSet{
					{
						Files: []File{{
							Source:      "downloaded.txt",
							Destination: downloaded,
							Digest:      sha256Hex(contents),
							Size:        int64(len(contents)),
						}},
						AfterCommand: "touch " + afterCommandMarker,
					},
					{
						Files:              []File{{Source: "file://" + template, Destination: rendered, IsTemplate: true}},
						TemplateParameters: []TemplateParameter{{Key: "key", Value: "value"}},
					},
				},
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
		// reporting is disabled, so the time limit isn't reported as an error
		assert.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatalf("configuration execution didn't stop after the download finished")
	}

	// the download wasn't cancelled at the deadline, and the execution stopped as soon as it finished
	elapsed := time.Since(start)
	assert.True(t, elapsed > 500*time.Millisecond)
	assert.True(t, elapsed < 3*time.Second)

	// the download finished past the deadline, so it's kept for the next run instead of being installed
	assertNotExist(t, downloaded)
	assertFileContents(t, GetPartialDownloadFilePath(downloaded, sha256Hex(contents)), contents)

	// no further work was done past the deadline
	assertNotExist(t, afterCommandMarker)
	assertNotExist(t, rendered)
	assert.Equal(t, srv.connectivityWatchdogThreshold.Load(), int64(0))

	// the execution released its lock
	_, err := os.Stat(srv.lockFilePath())
	assert.True(t, os.IsNotExist(err))

	// the next run installs the file without downloading it again
	srv.executionTimeout.Store(int64(time.Minute))
	assert.NoError(t, srv.Execute(t.Context(), config))
	assertFileContents(t, downloaded, contents)
	assertNoPartialDownload(t, downloaded, testFileMetadata(contents))
}
