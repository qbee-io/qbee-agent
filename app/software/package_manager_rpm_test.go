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

package software

import (
	"reflect"
	"testing"
)

func TestRpmUpdatesParse(t *testing.T) {
	tests := []struct {
		name string
		line string
		want *Package
	}{
		{
			name: "empty line",
			line: "",
			want: nil,
		},
		{
			name: "valid line",
			line: "glibc.x86_64                        2.34-83.el9_3.12        ubi-9-baseos-rpms",
			want: &Package{
				Name:         "glibc",
				Update:       "2.34-83.el9_3.12",
				Architecture: "x86_64",
			},
		},
		{
			name: "incomplete line",
			line: "glibc.x86_64                        2.34-83.el9_3.12",
			want: nil,
		},
		{
			name: "no architecture",
			line: "glibc                           2.34-83.el9_3.12        ubi-9-baseos-rpms",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rpm := &RpmPackageManager{}
			if got := rpm.parseUpdateAvailableLine(tt.line); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseUpdateAvailableLine() = %v, want %v", got, tt.want)
			}
		})
	}

}
