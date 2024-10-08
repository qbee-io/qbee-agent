// Copyright 2023 qbee.io
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
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.qbee.io/agent/app/api"
	"go.qbee.io/agent/app/utils/assert"
)

func TestService_reportsBuffer(t *testing.T) {
	srv := New(nil, t.TempDir(), "")

	t.Run("read empty buffer", func(t *testing.T) {
		if bufferedReports, err := srv.readReportsBuffer(); err != nil {
			t.Fatalf("failed to read reports from buffer: %v", err)
		} else {
			assert.Empty(t, bufferedReports)
		}
	})

	t.Run("add expired reports", func(t *testing.T) {
		expiredReports := []Report{{
			Text:      "expired report",
			Timestamp: time.Now().Add(-reportsBufferExpiration - time.Second).Unix(),
		}}

		if err := srv.addReportsToBuffer(expiredReports); err != nil {
			t.Fatalf("failed to add reports to buffer: %v", err)
		}

		if bufferedReports, err := srv.readReportsBuffer(); err != nil {
			t.Fatalf("failed to read reports from buffer: %v", err)
		} else {
			assert.Empty(t, bufferedReports)
		}
	})

	newReports := []Report{{Text: "new report 1", Timestamp: time.Now().Unix()}}

	t.Run("add fresh reports", func(t *testing.T) {
		if err := srv.addReportsToBuffer(newReports); err != nil {
			t.Fatalf("failed to add reports to buffer: %v", err)
		}

		if bufferedReports, err := srv.readReportsBuffer(); err != nil {
			t.Fatalf("failed to read reports from buffer: %v", err)
		} else {
			assert.Equal(t, bufferedReports, newReports)
		}
	})

	t.Run("add more fresh reports", func(t *testing.T) {
		newReports2 := []Report{{Text: "new report 2", Timestamp: time.Now().Unix()}}
		if err := srv.addReportsToBuffer(newReports2); err != nil {
			t.Fatalf("failed to add reports to buffer: %v", err)
		}

		if bufferedReports, err := srv.readReportsBuffer(); err != nil {
			t.Fatalf("failed to read reports from buffer: %v", err)
		} else {
			assert.Equal(t, bufferedReports, append(newReports, newReports2...))
		}
	})

	t.Run("clearing the buffer", func(t *testing.T) {
		// clearing the buffer wipes all added reports
		if err := srv.clearReportsBuffer(); err != nil {
			t.Fatalf("failed to clear reports buffer: %v", err)
		}

		if bufferedReports, err := srv.readReportsBuffer(); err != nil {
			t.Fatalf("failed to read reports from buffer: %v", err)
		} else {
			assert.Empty(t, bufferedReports)
		}

		// clearing empty buffer shouldn't fail
		if err := srv.clearReportsBuffer(); err != nil {
			t.Fatalf("failed to clear reports buffer: %v", err)
		}
	})
}

func TestInvalidBytesReportsBuffer(t *testing.T) {
	tmpDir := t.TempDir()
	srv := New(nil, tmpDir, "")
	reportsBuffer := filepath.Join(tmpDir, "reports.jsonl")

	invalidChars := []byte{0x00, 0x01, 0x02, 0x03, 0x04}
	// create invalid reports buffer
	if err := os.WriteFile(reportsBuffer, invalidChars, 0644); err != nil {
		t.Fatalf("failed to create nil reports buffer: %v", err)
	}

	// add new reports to the buffer
	newReports := []Report{{Text: "new report", Timestamp: time.Now().Unix()}}
	if err := srv.addReportsToBuffer(newReports); err != nil {
		t.Fatalf("failed to add to reports buffer: %v", err)
	}

	// Add invalid chars after a valid report
	f, err := os.OpenFile(reportsBuffer, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		t.Fatalf("failed to open reports buffer: %v", err)
	}
	defer f.Close()

	_, _ = f.Write(invalidChars)

	t.Run("read invalid reports buffer", func(t *testing.T) {
		// adding reports to invalid buffer should fail
		if bufferedReports, err := srv.readReportsBuffer(); err != nil {
			t.Fatalf("failed to read reports from buffer: %v", err)
		} else {
			assert.Equal(t, bufferedReports, newReports)
		}
	})
}

func TestService_persistConfig(t *testing.T) {
	apiClient := api.NewClient("invalid-host.example", "12345")
	srv := New(apiClient, t.TempDir(), "")

	cfg := &CommittedConfig{
		CommitID: "abc",
		Bundles:  []string{BundleSettings},
		BundleData: BundleData{
			Settings: SettingsBundle{
				RunInterval: 10,
			},
		},
	}

	srv.persistConfig(cfg)

	t.Run("load config from file", func(t *testing.T) {
		loadedCfg := new(CommittedConfig)

		if err := srv.loadConfig(loadedCfg); err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		assert.Equal(t, loadedCfg, cfg)
	})

	t.Run("load config through public Get method", func(t *testing.T) {
		committedConfig, err := srv.Get(context.Background())
		if err != nil {
			t.Fatalf("failed to get config: %v", err)
		}

		assert.Equal(t, committedConfig, cfg)
	})
}
