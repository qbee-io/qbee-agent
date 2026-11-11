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
	"testing"
	"time"

	"go.qbee.io/agent/app/utils/cache"
)

func TestSetCacheTTL(t *testing.T) {
	defer cache.Clear()

	deb := new(DebianPackageManager)

	if got := deb.cacheTTL(); got != pkgCacheTTL {
		t.Fatalf("expected default TTL %v, got %v", pkgCacheTTL, got)
	}

	cache.Set(pkgListCacheKey, []Package{}, deb.cacheTTL())

	deb.SetCacheTTL(72 * time.Hour)
	if got := deb.cacheTTL(); got != 72*time.Hour {
		t.Fatalf("expected TTL 72h, got %v", got)
	}

	if _, ok := cache.Get(pkgListCacheKey); !ok {
		t.Fatalf("expected package list to remain cached after extending TTL")
	}

	// a TTL shorter than the age of the cached package list expires it
	deb.SetCacheTTL(time.Nanosecond)
	if _, ok := cache.Get(pkgListCacheKey); ok {
		t.Fatalf("expected package list to be expired after reducing TTL")
	}

	deb.SetCacheTTL(0)
	if got := deb.cacheTTL(); got != pkgCacheTTL {
		t.Fatalf("expected default TTL %v after reset, got %v", pkgCacheTTL, got)
	}
}
