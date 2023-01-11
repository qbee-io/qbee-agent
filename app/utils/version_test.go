package utils

import (
	"reflect"
	"testing"
)

func Test_parseSemanticVersion(t *testing.T) {
	tests := []struct {
		name          string
		versionString string
		want          SemanticVersion
	}{
		{
			name:          "numeric major.minor.patch",
			versionString: "0.1.2",
			want:          SemanticVersion{0, 1, 2},
		},
		{
			name:          "major.minor.patch-ascii",
			versionString: "0.1.2-alpha",
			want:          SemanticVersion{0, 1, 2},
		},
		{
			name:          "with v-prefix",
			versionString: "v1.1.2",
			want:          SemanticVersion{1, 1, 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSemanticVersion(tt.versionString); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSemanticVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{
			name: "a is newer",
			a:    "1.0.1",
			b:    "1.0.0",
			want: true,
		},
		{
			name: "a is older",
			a:    "1.0.0",
			b:    "1.0.1",
			want: false,
		},
		{
			name: "both are equal",
			a:    "1.0.1",
			b:    "1.0.1",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNewerVersion(tt.a, tt.b); got != tt.want {
				t.Errorf("IsNewerVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
