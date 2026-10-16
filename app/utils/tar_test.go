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
	"os"
	"path/filepath"
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

// Test_TarZipSlipSiblingPrefixBypass covers traversal into a sibling directory whose cleaned path
// still shares the destination directory as a string prefix (e.g. "context" -> "context-outside").
func Test_TarZipSlipSiblingPrefixBypass(t *testing.T) {
	baseDir := t.TempDir()
	destPath := filepath.Join(baseDir, "context")
	if err := os.MkdirAll(destPath, 0755); err != nil {
		t.Fatalf("failed to create destination directory: %v", err)
	}

	const payload = "zip-slip-sibling-prefix"

	var tarBuffer bytes.Buffer
	tw := tar.NewWriter(&tarBuffer)

	dirHeader := &tar.Header{
		Name:     "../context-outside",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}
	if err := tw.WriteHeader(dirHeader); err != nil {
		t.Fatalf("failed to write dir header: %v", err)
	}

	fileHeader := &tar.Header{
		Name:     "../context-outside/payload.txt",
		Mode:     0644,
		Size:     int64(len(payload)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(fileHeader); err != nil {
		t.Fatalf("failed to write file header: %v", err)
	}
	if _, err := tw.Write([]byte(payload)); err != nil {
		t.Fatalf("failed to write file content: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}

	err := unpackTar(&tarBuffer, destPath)

	escapedDir := filepath.Join(baseDir, "context-outside")
	if _, statErr := os.Stat(escapedDir); statErr == nil {
		t.Fatalf("path traversal: directory was created outside destPath at %s", escapedDir)
	}

	escapedPath := filepath.Join(escapedDir, "payload.txt")
	if contents, readErr := os.ReadFile(escapedPath); readErr == nil {
		t.Fatalf("path traversal: payload was written outside destPath at %s; got %q", escapedPath, contents)
	}

	assert.NotEqual(t, err, nil)
}
