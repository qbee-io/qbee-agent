package configuration

import (
	"bytes"
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
		lineNo int
		want   []byte
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
			name:   "unclosed tag",
			line:   []byte("test {{tag"),
			want:   []byte("test {{tag"),
			lineNo: 123,
		},
		{
			name:   "unknown tag",
			line:   []byte("test {{tag}}"),
			want:   []byte("test {{tag}}"),
			lineNo: 123,
		},
		{
			name:   "unknown properties and missing closing",
			line:   []byte("test {{tag1}} {{tag2}} {{valid}} {{ unclosed \n"),
			want:   []byte("test {{tag1}} {{tag2}} 1 {{ unclosed \n"),
			params: map[string]string{"valid": "1"},
			lineNo: 123,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderTemplateLine(tt.line, tt.params, tt.lineNo)
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
	tests := []struct {
		name        string
		source      string
		destination string
		want        string
	}{
		{
			name:        "regular path",
			source:      "/test/source",
			destination: "/tmp/destination",
			want:        "/tmp/destination",
		},
		{
			name:        "dir path with trailing slash",
			source:      "/test/source",
			destination: "/tmp/",
			want:        "/tmp/source",
		},
		{
			name:        "dir path without trailing slash",
			source:      "/test/source",
			destination: "/tmp",
			want:        "/tmp/source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveDestinationPath(tt.source, tt.destination)
			if err != nil {
				t.Fatalf("unexpected error = %v", err)
			}
			if got != tt.want {
				t.Errorf("got = `%s`, want `%s`", got, tt.want)
			}
		})
	}
}
