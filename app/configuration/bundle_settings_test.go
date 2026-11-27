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
	"encoding/json"
	"math"
	"testing"
	"time"

	"go.qbee.io/agent/app/utils/assert"
)

func TestService_setExecutionTimeout(t *testing.T) {
	tests := []struct {
		name     string
		minutes  int
		expected time.Duration
	}{
		{name: "unset", minutes: 0, expected: defaultExecutionTimeout},
		{name: "negative", minutes: -1, expected: defaultExecutionTimeout},
		{name: "negative overflowing as duration", minutes: -153722868, expected: defaultExecutionTimeout},
		{name: "below minimum", minutes: 1, expected: minimumExecutionTimeout},
		{name: "valid", minutes: 180, expected: 3 * time.Hour},
		{name: "maximum", minutes: 1440, expected: 24 * time.Hour},
		{name: "above maximum", minutes: 1441, expected: maximumExecutionTimeout},
		{name: "overflowing as duration", minutes: math.MaxInt, expected: maximumExecutionTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := New(nil, t.TempDir(), "")
			srv.setExecutionTimeout(tt.minutes)
			assert.Equal(t, srv.ExecutionTimeout(), tt.expected)
		})
	}
}

func TestSettingsBundle_ExecutionTimeout(t *testing.T) {
	srv := New(nil, t.TempDir(), "")
	assert.Equal(t, srv.ExecutionTimeout(), defaultExecutionTimeout)

	SettingsBundle{ExecutionTimeout: 120}.Execute(srv)
	assert.Equal(t, srv.ExecutionTimeout(), 2*time.Hour)

	// disabling settings bundle restores the default
	srv.UpdateSettings(&CommittedConfig{})
	assert.Equal(t, srv.ExecutionTimeout(), defaultExecutionTimeout)
}

func TestSettingsBundle_ExecutionTimeoutFromJSON(t *testing.T) {
	bundle := new(SettingsBundle)
	assert.NoError(t, json.Unmarshal([]byte(`{"enabled": true, "execution_timeout": 180}`), bundle))

	srv := New(nil, t.TempDir(), "")
	bundle.Execute(srv)

	assert.Equal(t, srv.ExecutionTimeout(), 3*time.Hour)
}
