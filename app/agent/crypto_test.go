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

package agent

import (
	"testing"
	"time"

	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/utils/assert"
)

func TestAgent_signedURLTTL(t *testing.T) {
	tests := []struct {
		name             string
		executionTimeout int
		expected         time.Duration
	}{
		{name: "default execution timeout", executionTimeout: 0, expected: time.Hour},
		{name: "shorter execution timeout", executionTimeout: 10, expected: time.Hour},
		{name: "longer execution timeout", executionTimeout: 180, expected: 3 * time.Hour},
		{name: "execution timeout above maximum", executionTimeout: 100000, expected: 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &Agent{Configuration: configuration.New(nil, t.TempDir(), t.TempDir())}

			agent.Configuration.UpdateSettings(&configuration.CommittedConfig{
				Bundles: []string{configuration.BundleSettings},
				BundleData: configuration.BundleData{
					Settings: configuration.SettingsBundle{
						Metadata:         configuration.Metadata{Enabled: true},
						ExecutionTimeout: tt.executionTimeout,
					},
				},
			})

			assert.Equal(t, agent.signedURLTTL(), tt.expected)
		})
	}
}

func TestAgent_signedURLTTL_ExecutionInProgress(t *testing.T) {
	exec := newRealExecution(t)
	exec.executionTimeout = 180

	a := newCheckInTestAgent(t)
	a.useRealExecution(exec)
	a.configResponse = exec.config(t, "")

	// the execution starts with a 3 hour time limit and blocks downloading the file
	a.checkIn(t)
	writer := exec.waitForDownload(t)

	// a check-in lowers the time limit while the execution is still in progress
	a.Configuration.UpdateSettings(&configuration.CommittedConfig{})
	assert.Equal(t, a.Configuration.ExecutionTimeout(), time.Hour)

	// URLs signed during the execution remain valid for its time limit
	assert.Equal(t, a.signedURLTTL(), 3*time.Hour)

	exec.finishDownload(t, writer)
	a.Wait()

	assert.Equal(t, a.signedURLTTL(), time.Hour)
}
