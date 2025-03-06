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

import "testing"

func Test_GetExtension(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "no extension",
			path: "/path/to/file",
			want: "",
		},
		{
			name: "single extension",
			path: "/path/to/file.tar",
			want: "tar",
		},
		{
			name: "multiple extensions",
			path: "/path/to/file.tar.gz",
			want: "tar.gz",
		},
		{
			name: "local path",
			path: "file:///path/to/file.tar.gz",
			want: "tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetTarExtension(tt.path); got != tt.want {
				t.Errorf("GetExtension() = %v, want %v", got, tt.want)
			}
		})
	}
}
