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

// pkgIndexRetryInterval is the minimum time between package index refresh attempts,
// used to retry failed refreshes and to limit on-demand refreshes (e.g. when a package to install
// is not found in the local package index).
const pkgIndexRetryInterval = time.Hour

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

// pkgCacheState keeps track of the package cache TTL and package index refreshes.
// When embedded in a package manager struct, it satisfies the SetCacheTTL method of the PackageManager interface.
type pkgCacheState struct {
	// ttl is stored atomically, so it can be set without waiting for ongoing package manager operations.
	ttl atomic.Int64

	// lastRefresh is the time of the last successful package index refresh.
	// It must only be accessed while holding the package manager lock.
	lastRefresh time.Time

	// lastRefreshAttempt is the time of the last package index refresh attempt (successful or not).
	// It must only be accessed while holding the package manager lock.
	lastRefreshAttempt time.Time
}

// SetCacheTTL sets the TTL for cached package information. Non-positive value resets it to DefaultPkgCacheTTL.
func (s *pkgCacheState) SetCacheTTL(ttl time.Duration) {
	if ttl <= 0 {
		ttl = DefaultPkgCacheTTL
	}

	s.ttl.Store(int64(ttl))
}

// cacheTTL returns the TTL for cached package information.
func (s *pkgCacheState) cacheTTL() time.Duration {
	if ttl := time.Duration(s.ttl.Load()); ttl > 0 {
		return ttl
	}

	return DefaultPkgCacheTTL
}

// canAttemptRefresh returns true if enough time has passed since the last package index refresh attempt.
func (s *pkgCacheState) canAttemptRefresh() bool {
	return time.Since(s.lastRefreshAttempt) >= pkgIndexRetryInterval
}

// refreshDue returns true if the package index should be refreshed, because the last successful refresh
// is older than the current cache TTL. Failed refreshes are retried at most once per pkgIndexRetryInterval.
func (s *pkgCacheState) refreshDue() bool {
	return time.Since(s.lastRefresh) >= s.cacheTTL() && s.canAttemptRefresh()
}

// markRefreshAttempt records a package index refresh attempt.
func (s *pkgCacheState) markRefreshAttempt() {
	s.lastRefreshAttempt = time.Now()
}

// markRefreshed records a successful package index refresh.
func (s *pkgCacheState) markRefreshed() {
	s.lastRefresh = time.Now()
}
