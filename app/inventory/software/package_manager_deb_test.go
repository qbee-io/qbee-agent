package software

import (
	"reflect"
	"testing"
)

func TestDebPackageManager_parseUpdateAvailableLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want *Package
	}{
		{
			name: "single source",
			line: "Inst libudev1 [249.11-0ubuntu3.4] (249.11-0ubuntu3.6 jammy-updates [amd64])",
			want: &Package{
				Name:         "libudev1",
				Version:      "249.11-0ubuntu3.4",
				Architecture: "amd64",
				Update:       "249.11-0ubuntu3.6",
			},
		},
		{
			name: "multiple source",
			line: "Inst libudev1 [249.11-0ubuntu3.4] (249.11-0ubuntu3.6 jammy-updates, jammy-security [amd64])",
			want: &Package{
				Name:         "libudev1",
				Version:      "249.11-0ubuntu3.4",
				Architecture: "amd64",
				Update:       "249.11-0ubuntu3.6",
			},
		},
		{
			name: "extra postfix",
			line: "Inst libudev1 [249.11-0ubuntu3.4] (249.11-0ubuntu3.6 jammy-updates [amd64]) []",
			want: &Package{
				Name:         "libudev1",
				Version:      "249.11-0ubuntu3.4",
				Architecture: "amd64",
				Update:       "249.11-0ubuntu3.6",
			},
		},
		{
			name: "no current version",
			line: "Inst libudev1 (249.11-0ubuntu3.6 jammy-updates [amd64])",
			want: &Package{
				Name:         "libudev1",
				Architecture: "amd64",
				Update:       "249.11-0ubuntu3.6",
			},
		},
		{
			name: "invalid line",
			line: "Conf libudev1 (249.11-0ubuntu3.6 jammy-updates [amd64])",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deb := &DebianPackageManager{}
			if got := deb.parseUpdateAvailableLine(tt.line); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseUpdateAvailableLine() = %v, want %v", got, tt.want)
			}
		})
	}
}
