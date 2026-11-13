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
	"testing"
	"time"

	"go.qbee.io/agent/app/utils/assert"
)

func TestService_setExecutionTimeout(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{name: "unset", timeout: 0, expected: defaultExecutionTimeout},
		{name: "negative", timeout: -time.Minute, expected: defaultExecutionTimeout},
		{name: "below minimum", timeout: time.Minute, expected: minimumExecutionTimeout},
		{name: "valid", timeout: 3 * time.Hour, expected: 3 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := New(nil, t.TempDir(), "")
			srv.setExecutionTimeout(tt.timeout)
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
