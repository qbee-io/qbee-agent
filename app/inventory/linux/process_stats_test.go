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

//go:build unix

package linux

import (
	"reflect"
	"testing"
)

func TestNewProcessStats(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    ProcessStats
		wantErr bool
	}{
		{
			name: "command with no spaces",
			line: "275809 (test-cmd) S 264817",
			want: ProcessStats{"275809", "test-cmd", "S", "264817"},
		},
		{
			name: "command with spaces",
			line: "275809 (test cmd) S 264817",
			want: ProcessStats{"275809", "test cmd", "S", "264817"},
		},
		{
			name:    "invalid format",
			line:    "275809 test cmd) S 264817",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProcessStats(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProcessStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewProcessStats() got = %v, want %v", got, tt.want)
			}
		})
	}
}
