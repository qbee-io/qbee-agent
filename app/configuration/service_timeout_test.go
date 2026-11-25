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
	"context"
	"crypto/sha256"
	"encoding/hex"
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
