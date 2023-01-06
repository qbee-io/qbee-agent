package utils

import (
	"reflect"
	"testing"
)

func TestTailBuffer(t *testing.T) {
	tests := []struct {
		name          string
		maxLines      int
		chunksToWrite [][]byte
		wantLines     [][]byte
	}{
		{
			name:     "single line in chunks",
			maxLines: 1,
			chunksToWrite: [][]byte{
				[]byte("thi"),
				[]byte("s is one"),
				[]byte(" line\n"),
			},
			wantLines: [][]byte{
				[]byte("this is one line"),
			},
		},
		{
			name:     "single line without new-line character",
			maxLines: 1,
			chunksToWrite: [][]byte{
				[]byte("this is one line"),
			},
			wantLines: [][]byte{
				[]byte("this is one line"),
			},
		},
		{
			name:     "two lines",
			maxLines: 2,
			chunksToWrite: [][]byte{
				[]byte("this is one line\n"),
				[]byte("this is another line\n"),
			},
			wantLines: [][]byte{
				[]byte("this is one line"),
				[]byte("this is another line"),
			},
		},
		{
			name:     "two lines with one line limit",
			maxLines: 1,
			chunksToWrite: [][]byte{
				[]byte("this is one line\n"),
				[]byte("this is another line\n"),
			},
			wantLines: [][]byte{
				[]byte("this is another line"),
			},
		},
		{
			name:     "trimming spaces from lines",
			maxLines: 1,
			chunksToWrite: [][]byte{
				[]byte(" this is one line \r\n"),
			},
			wantLines: [][]byte{
				[]byte("this is one line"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tailBuffer := NewTailBuffer(tt.maxLines)

			for _, chunk := range tt.chunksToWrite {
				_, _ = tailBuffer.Write(chunk)
			}

			gotLines := tailBuffer.Close()

			if !reflect.DeepEqual(gotLines, tt.wantLines) {
				t.Errorf("got lines: %s\nwant %s", gotLines, tt.wantLines)
			}
		})
	}
}
