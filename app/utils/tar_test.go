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
	"archive/tar"
	"bytes"
	"testing"

	"go.qbee.io/agent/app/utils/assert"
)

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

func Test_TarZipSlip(t *testing.T) {
	tt := []struct {
		name     string
		filePath string
		wantErr  bool
	}{
		{
			name:     "valid tar",
			filePath: "filename.txt",
			wantErr:  false,
		},
		{
			name:     "zip slip attack",
			filePath: "../outside.txt",
			wantErr:  true,
		},
		{
			name:     "nested zip slip attack",
			filePath: "../../subdir/filename.txt",
			wantErr:  true,
		},
		{
			name:     "valid nested tar",
			filePath: "subdir/../filename.txt",
			wantErr:  false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			var tarBuffer bytes.Buffer
			tw := tar.NewWriter(&tarBuffer)

			header := &tar.Header{
				Name: tc.filePath,
				Mode: 0644,
				Size: int64(len("hello")),
			}

			err := tw.WriteHeader(header)

			if err != nil {
				t.Fatalf("failed to write header: %v", err)
			}
			_, err = tw.Write([]byte("hello"))
			if err != nil {
				t.Fatalf("failed to write file content: %v", err)
			}

			if err := tw.Close(); err != nil {
				t.Fatalf("failed to close tar writer: %v", err)
			}

			err = unpackTar(&tarBuffer, t.TempDir())

			if tc.wantErr {
				assert.NotEqual(t, err, nil)
			} else {
				assert.NoError(t, err)
			}
		})
	}

}
