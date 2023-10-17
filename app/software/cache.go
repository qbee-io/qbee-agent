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

import "time"

var pkgCache map[PackageManagerType]pkgCacheItem

const pkgCacheTTL = 24 * time.Hour

type pkgCacheItem struct {
	expires  time.Time
	packages []Package
}

func init() {
	pkgCache = make(map[PackageManagerType]pkgCacheItem)
}

// getCachedPackages returns cached packages and true for provided package manager type if packages are fresh in cache.
// Otherwise, it returns (nil, false).
func getCachedPackages(pkgManagerType PackageManagerType) ([]Package, bool) {
	cacheItem, ok := pkgCache[pkgManagerType]
	if !ok {
		return nil, false
	}

	if cacheItem.expires.Before(time.Now()) {
		return nil, false
	}

	return cacheItem.packages, true
}

// setCachedPackages for provided package manager type.
func setCachedPackages(pkgManagerType PackageManagerType, packages []Package) {
	pkgCache[pkgManagerType] = pkgCacheItem{
		expires:  time.Now().Add(pkgCacheTTL),
		packages: packages,
	}
}

// InvalidateCache for provided package manager type (e.g. when agent installs new packages).
func InvalidateCache(pkgManagerType PackageManagerType) {
	delete(pkgCache, pkgManagerType)
}
