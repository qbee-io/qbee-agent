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
	"sync/atomic"
	"time"
)

// DefaultPackageManager is set to the supporter package manager for the OS.
// If there are no support package managers available, this will be nil.
var DefaultPackageManager PackageManager

// DefaultPkgCacheTTL is the default TTL for cached package information.
// It also defines how often the package index is refreshed from remote repositories.
const DefaultPkgCacheTTL = 24 * time.Hour

// pkgIndexOnDemandRefreshInterval limits how often the package index can be refreshed on demand
// (e.g. when a package to install is not found in the local index).
const pkgIndexOnDemandRefreshInterval = time.Hour

const pkgCacheKeyPrefix = "packages"

// PackageManagers provides a map of all package managers supported by the agent.
var PackageManagers = map[PackageManagerType]PackageManager{
	PackageManagerTypeDebian: new(DebianPackageManager),
	PackageManagerTypeRpm:    new(RpmPackageManager),
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
	// Type returns the package manager type.
	Type() PackageManagerType

	// FileSuffix returns the file suffix for the package manager.
	FileSuffix() string

	// IsSupported returns true if package manager is supported by the host system.
	IsSupported() bool

	// SetCacheTTL sets the TTL for cached package information.
	// This also defines how often the package index is refreshed from remote repositories.
	// Non-positive value resets it to DefaultPkgCacheTTL.
	SetCacheTTL(ttl time.Duration)

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
	PackageArchitecture(ctx context.Context) (string, error)

	// IsSupportedArchitecture returns true if architecture is supported by the system
	IsSupportedArchitecture(ctx context.Context, arch string) error

	// ParsePackageFile returns a package from a file path.
	ParsePackageFile(ctx context.Context, filePath string) (*Package, error)
}

// pkgCacheTTLSetting implements the cache TTL setting of package managers.
// The TTL is stored atomically, so it can be set without waiting for ongoing package manager operations.
type pkgCacheTTLSetting struct {
	ttl atomic.Int64
}

// SetCacheTTL sets the TTL for cached package information. Non-positive value resets it to DefaultPkgCacheTTL.
// When pkgCacheTTLSetting is embedded in package manager struct, it satisfies the SetCacheTTL method of
// the PackageManager interface.
func (s *pkgCacheTTLSetting) SetCacheTTL(ttl time.Duration) {
	if ttl <= 0 {
		ttl = DefaultPkgCacheTTL
	}

	s.ttl.Store(int64(ttl))
}

// cacheTTL returns the TTL for cached package information.
func (s *pkgCacheTTLSetting) cacheTTL() time.Duration {
	if ttl := time.Duration(s.ttl.Load()); ttl > 0 {
		return ttl
	}

	return DefaultPkgCacheTTL
}
