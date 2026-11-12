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

	"go.qbee.io/agent/app/utils/cache"
)

// DefaultPackageManager is set to the supporter package manager for the OS.
// If there are no support package managers available, this will be nil.
var DefaultPackageManager PackageManager

// pkgCacheTTL is the default TTL for cached package information.
const pkgCacheTTL = 24 * time.Hour
const pkgCacheKeyPrefix = "packages"

// pkgListCacheKey is the cache key of the package list.
// It is shared by all package managers, as only one package manager is used on a system.
const pkgListCacheKey = pkgCacheKeyPrefix + ":list"

// pkgArchCacheKey is the cache key of the package architecture.
// It is shared by all package managers, as only one package manager is used on a system.
const pkgArchCacheKey = pkgCacheKeyPrefix + ":arch"

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

	// SetCacheTTL sets the TTL of the cached package list. Non-positive value resets it to the default TTL.
	// The expiry of an already cached package list is adjusted relative to when the list was cached.
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

// pkgListCacheTTL holds the package list cache TTL of a package manager instance.
// When embedded in a package manager struct, it implements the SetCacheTTL method of the PackageManager interface.
type pkgListCacheTTL struct {
	// ttl is stored atomically, so it can be changed without waiting for ongoing package manager operations.
	ttl atomic.Int64
}

// SetCacheTTL sets the TTL of the cached package list. Non-positive value resets it to the default TTL.
// The expiry of an already cached package list is adjusted relative to when the list was cached.
func (c *pkgListCacheTTL) SetCacheTTL(ttl time.Duration) {
	if ttl <= 0 {
		ttl = pkgCacheTTL
	}

	c.ttl.Store(int64(ttl))

	cache.UpdateTTL(pkgListCacheKey, ttl)
}

// cacheTTL returns the TTL of the cached package list.
func (c *pkgListCacheTTL) cacheTTL() time.Duration {
	if ttl := time.Duration(c.ttl.Load()); ttl > 0 {
		return ttl
	}

	return pkgCacheTTL
}
