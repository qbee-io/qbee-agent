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

package utils

import (
	"bytes"
	"testing"
)

func TestTailBuffer(t *testing.T) {
	tests := []struct {
		name          string
		maxBytes      int
		chunksToWrite [][]byte
		want          []byte
	}{
		{
			name:          "empty",
			maxBytes:      16,
			chunksToWrite: nil,
			want:          []byte{},
		},
		{
			name:     "single line in chunks",
			maxBytes: 1024,
			chunksToWrite: [][]byte{
				[]byte("thi"),
				[]byte("s is one"),
				[]byte(" line\n"),
			},
			want: []byte("this is one line\n"),
		},
		{
			name:          "single line without new-line character",
			maxBytes:      1024,
			chunksToWrite: [][]byte{[]byte("this is one line")},
			want:          []byte("this is one line"),
		},
		{
			name:     "two lines",
			maxBytes: 1024,
			chunksToWrite: [][]byte{
				[]byte("this is one line\n"),
				[]byte("this is another line\n"),
			},
			want: []byte("this is one line\nthis is another line\n"),
		},
		{
			name:     "exactly fills the ring without truncation",
			maxBytes: 10,
			chunksToWrite: [][]byte{
				[]byte("abc\n"),
				[]byte("defgh\n"),
			},
			want: []byte("abc\ndefgh\n"),
		},
		{
			name:     "two lines with small byte limit",
			maxBytes: 25,
			chunksToWrite: [][]byte{
				[]byte("this is one line\n"),
				[]byte("this is another line\n"),
			},
			want: []byte("this is another line\n"),
		},
		{
			name:     "byte limit enforcement",
			maxBytes: 50,
			chunksToWrite: [][]byte{
				[]byte("this is the first line that is quite long\n"),
				[]byte("this is the second line\n"),
				[]byte("this is the third line\n"),
			},
			want: []byte("this is the second line\nthis is the third line\n"),
		},
		{
			name:     "single write larger than the ring",
			maxBytes: 8,
			chunksToWrite: [][]byte{
				[]byte("line1\nline2\nline3\n"),
			},
			want: []byte("line3\n"),
		},
		{
			name:     "truncated without line boundary keeps raw tail",
			maxBytes: 4,
			chunksToWrite: [][]byte{
				[]byte("abcdef"),
				[]byte("gh"),
			},
			want: []byte("efgh"),
		},
		{
			name:     "many small writes wrapping the ring",
			maxBytes: 7,
			chunksToWrite: [][]byte{
				[]byte("1\n"), []byte("2\n"), []byte("3\n"), []byte("4\n"), []byte("5\n"),
			},
			want: []byte("3\n4\n5\n"),
		},
		{
			name:          "zero size buffer",
			maxBytes:      0,
			chunksToWrite: [][]byte{[]byte("data")},
			want:          []byte{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tailBuffer := NewTailBuffer(tt.maxBytes)

			for _, chunk := range tt.chunksToWrite {
				n, err := tailBuffer.Write(chunk)
				if err != nil || n != len(chunk) {
					t.Fatalf("Write() = %d, %v", n, err)
				}
			}

			got := tailBuffer.Close()

			if !bytes.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTailBuffer_WriteAfterClose(t *testing.T) {
	tailBuffer := NewTailBuffer(16)
	_ = tailBuffer.Close()

	if _, err := tailBuffer.Write([]byte("data")); err == nil {
		t.Fatalf("expected error writing to closed buffer")
	}
}

func TestTailBuffer_WriteDoesNotAllocate(t *testing.T) {
	tailBuffer := NewTailBuffer(64)
	chunk := []byte("some output line that wraps the ring\n")

	allocs := testing.AllocsPerRun(1000, func() {
		_, _ = tailBuffer.Write(chunk)
	})

	if allocs != 0 {
		t.Fatalf("expected Write to not allocate, got %v allocations per run", allocs)
	}
}
