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
	"fmt"
	"runtime"
	"testing"
)

func TestCheckCondition(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		want      bool
	}{
		{name: "empty", condition: "", want: true},
		{name: "exit 0", condition: "true", want: true},
		{name: "exit 1", condition: "false", want: false},
		{name: "exit 0 with stdout", condition: "echo out", want: true},
		{name: "exit 0 with stderr", condition: "echo err >&2", want: true},
		{name: "exit 1 with output", condition: "echo out; echo err >&2; exit 1", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckCondition(context.Background(), tt.condition); got != tt.want {
				t.Errorf("CheckCondition(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}

func TestCheckCondition_OutputIsNotBuffered(t *testing.T) {
	const outputSize = 64 << 20     // 64 MB
	const maxAllowedAlloc = 4 << 20 // 4 MB

	condition := fmt.Sprintf("head -c %d /dev/zero; head -c %d /dev/zero >&2; true", outputSize, outputSize)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	ok := CheckCondition(context.Background(), condition)

	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc

	t.Logf("condition wrote %d bytes to stdout and stderr, CheckCondition allocated %d bytes", 2*outputSize, allocated)

	if !ok {
		t.Fatalf("CheckCondition(%q) = false, want true", condition)
	}

	if allocated > maxAllowedAlloc {
		t.Errorf("CheckCondition allocated %d bytes, expected at most %d", allocated, maxAllowedAlloc)
	}
}
