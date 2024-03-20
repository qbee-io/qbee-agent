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

package software

import (
	"context"
	"os/exec"
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
	if gotPath, _ := exec.LookPath(dpkgPath); gotPath == "" {
		t.Skip("dpkg not found")
	}

	ctx := context.Background()

	_, currentFile, _, _ := runtime.Caller(0)
	testPkg := filepath.Join(filepath.Dir(currentFile), "test_repository", "debian", "test_1.0.1.deb")

	deb := &DebianPackageManager{}
	pkgInfo, err := deb.ParsePackageFile(ctx, testPkg)
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
