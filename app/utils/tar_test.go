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
