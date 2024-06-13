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
	"bytes"
	"fmt"
	"path/filepath"
	"testing"
)

func Test_renderTemplate(t *testing.T) {
	tests := []struct {
		name    string
		src     []byte
		wantDst []byte
		params  map[string]string
	}{
		{
			name:    "no tags",
			src:     []byte("no tags"),
			wantDst: []byte("no tags"),
		},
		{
			name:    "with tags",
			src:     []byte("tag1: {{tag1}}\ntag2: {{tag2}}\r\n"),
			params:  map[string]string{"tag1": "test-tag-1", "tag2": "test-tag-2"},
			wantDst: []byte("tag1: test-tag-1\ntag2: test-tag-2\r\n"),
		},
		{
			name:    "ends with new line",
			src:     []byte("no tags\n"),
			wantDst: []byte("no tags\n"),
		},
		{
			name:    "multi-line linux LF",
			src:     []byte("line 1\nline2\n"),
			wantDst: []byte("line 1\nline2\n"),
		},
		{
			name:    "multi-line windows CR LF",
			src:     []byte("line 1\r\nline2\r\n"),
			wantDst: []byte("line 1\r\nline2\r\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := bytes.NewBuffer(tt.src)
			dst := new(bytes.Buffer)

			if err := renderTemplate(src, tt.params, dst); err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			if !bytes.Equal(dst.Bytes(), tt.wantDst) {
				t.Fatalf("expected:\n%s\ngot:\n%s", tt.wantDst, dst.Bytes())
			}
		})
	}
}

func Test_renderTemplateLine(t *testing.T) {
	tests := []struct {
		name   string
		line   []byte
		params map[string]string

		want []byte
	}{
		{
			name:   "no tags",
			line:   []byte("test 1"),
			want:   []byte("test 1"),
			params: map[string]string{"tag": "1"},
		},
		{
			name:   "tag with spaces",
			line:   []byte("test {{ tag }}"),
			want:   []byte("test 1"),
			params: map[string]string{"tag": "1"},
		},
		{
			name:   "starts with tag",
			line:   []byte("{{ tag }} test"),
			want:   []byte("1 test"),
			params: map[string]string{"tag": "1"},
		},
		{
			name:   "tag without spaces",
			line:   []byte("test {{tag}}"),
			want:   []byte("test 1"),
			params: map[string]string{"tag": "1"},
		},
		{
			name:   "more than one tag per line",
			line:   []byte("test {{  tag1}} {{tag2   }}"),
			want:   []byte("test 1 2"),
			params: map[string]string{"tag1": "1", "tag2": "2"},
		},
		{
			name: "unclosed tag",
			line: []byte("test {{tag"),
			want: []byte("test {{tag"),
		},
		{
			name: "unknown tag",
			line: []byte("test {{tag}}"),
			want: []byte("test {{tag}}"),
		},
		{
			name:   "unknown properties and missing closing",
			line:   []byte("test {{tag1}} {{tag2}} {{valid}} {{ unclosed \n"),
			want:   []byte("test {{tag1}} {{tag2}} 1 {{ unclosed \n"),
			params: map[string]string{"valid": "1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderTemplateLine(tt.line, tt.params)
			if err != nil {
				t.Fatalf("unexpected error = %v", err)
			}

			if !bytes.Equal(got, tt.want) {
				t.Errorf("got line = `%s`, want `%s`", got, tt.want)
			}
		})
	}
}

func Test_resolveDestinationPath(t *testing.T) {
	tempDir := t.TempDir()
	tests := []struct {
		name        string
		source      string
		destination string
		want        string
	}{
		{
			name:        "regular path",
			source:      "/test/source",
			destination: filepath.Join(tempDir, "destination"),
			want:        filepath.Join(tempDir, "destination"),
		},
		{
			name:        "dir path with trailing slash",
			source:      "/test/source",
			destination: fmt.Sprint(tempDir, "/"),
			want:        filepath.Join(tempDir, "source"),
		},
		{
			name:        "dir path without trailing slash",
			source:      "/test/source",
			destination: tempDir,
			want:        filepath.Join(tempDir, "source"),
		},
		{
			name:        "regular path",
			source:      "source",
			destination: tempDir,
			want:        filepath.Join(tempDir, "source"),
		},
		{
			name:        "from local source",
			source:      "file://source",
			destination: tempDir,
			want:        filepath.Join(tempDir, "source"),
		},
		{
			name:        "illegal path that shoould return empty string",
			source:      "source",
			destination: fmt.Sprintf("%s/notallowed/", tempDir),
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveDestinationPath(tt.source, tt.destination)
			if err != nil {
				if got != tt.want {
					t.Fatalf("unexpected error = %v", err)
				}
			}
			if got != tt.want {
				t.Errorf("got = `%s`, want `%s`", got, tt.want)
			}
		})
	}
}
