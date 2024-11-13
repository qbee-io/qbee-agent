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
	"time"
)

// DefaultPackageManager is set to the supporter package manager for the OS.
// If there are no support package managers available, this will be nil.
var DefaultPackageManager PackageManager

const pkgCacheTTL = 24 * time.Hour
const pkgCacheKeyPrefix = "packages"

// PackageManagers provides a map of all package managers supported by the agent.
var PackageManagers = map[PackageManagerType]PackageManager{
	PackageManagerTypeDebian: new(DebianPackageManager),
	PackageManagerTypeRpm:    new(RpmPackageManager),
	PackageManagerTypeAPK:    new(ApkPackageManager),
	PackageManagerTypeOpkg:   new(OpkgPackageManager),
}

func init() {
	// set default package manager
	for _, pkgManager := range PackageManagers {
		if pkgManager.IsSupported() {
			DefaultPackageManager = pkgManager
			return
		}
	}
}

// PackageManagerType defines package manager type.
type PackageManagerType string

// PackageManager defines package manager interface.
type PackageManager interface {
	Type() PackageManagerType

	// FileSuffix returns the file suffix for the package manager.
	FileSuffix() string

	// IsSupported returns true if package manager is supported by the host system.
	IsSupported() bool

	// Busy returns true if package manager is currently busy.
	Busy() (bool, error)

	// ListPackages returns a list of packages with available updates.
	ListPackages(ctx context.Context) ([]Package, error)

	// UpgradeAll performs upgrade of all packages.
	// On success, return number of packages upgraded, output of the upgrade command and nil error.
	UpgradeAll(ctx context.Context) (int, []byte, error)

	// Install ensures a package with provided version number is installed in the system.
	Install(ctx context.Context, pkgName, version string) ([]byte, error)

	// InstallLocal package.
	InstallLocal(ctx context.Context, pkgFilePath string) ([]byte, error)

	// PackageArchitecture returns the architecture of the package manager
	PackageArchitecture() (string, error)

	// IsSupportedArchitecture returns true if architecture is supported by the system
	IsSupportedArchitecture(arch string) error

	// ParsePackageFile returns a package from a file path.
	ParsePackageFile(ctx context.Context, filePath string) (*Package, error)
}
