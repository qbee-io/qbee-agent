// Copyright 2024 qbee.io
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

package utils

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCommandTimeout(t *testing.T) {

	tests := []struct {
		name          string
		command       []string
		expectedError string
	}{
		{
			name:          "command runs successfully",
			command:       []string{"echo", "hello"},
			expectedError: "",
		},
		{
			name:          "simple command killed by timeout",
			command:       []string{"sleep", "1"},
			expectedError: "signal: killed",
		},
		{
			name:          "child process killed by timeout",
			command:       []string{"sh", "-c", `(trap 'echo "cleanup completed"; exit' TERM; sleep 1)`},
			expectedError: "signal: killed",
		},
		{
			name:          "endless loop killed by timeout",
			command:       []string{"sh", "-c", "while true; do echo 'hello'; sleep 1; done"},
			expectedError: "signal: killed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Millisecond*100))
			defer cancel()

			start := time.Now()
			_, err := RunCommand(ctx, tt.command)
			if err == nil && tt.expectedError == "" {
				return
			}

			elapsed := time.Since(start)
			if elapsed.Seconds() > 1 {
				t.Fatalf("%s: unexpected elapsed time: %v", tt.name, elapsed)
			}

			if err == nil {
				t.Fatalf("%s: expected error", tt.name)
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Fatalf("%s: unexpected error: %v", tt.name, err)
			}
		})
	}
}
