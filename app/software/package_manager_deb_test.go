package software

import (
	"context"
	"path/filepath"
	"reflect"
	"runtime"
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

func TestParseDebianPackage(t *testing.T) {
	ctx := context.Background()

	_, currentFile, _, _ := runtime.Caller(0)
	testPkg := filepath.Join(filepath.Dir(currentFile), "test_repository", "debian", "test_1.0.1.deb")

	pkgInfo, err := ParseDebianPackage(ctx, testPkg)
	if err != nil {
		t.Fatalf("got unexpected error: %v", err)
	}

	expectedPkg := &Package{
		Name:         "qbee-test",
		Version:      "1.0.1",
		Architecture: "all",
	}

	if !reflect.DeepEqual(pkgInfo, expectedPkg) {
		t.Fatalf("expected %v, got %v", expectedPkg, pkgInfo)
	}
}
