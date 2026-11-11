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
)

func TestPkgCacheState_TTL(t *testing.T) {
	s := new(pkgCacheState)

	if got := s.cacheTTL(); got != DefaultPkgCacheTTL {
		t.Fatalf("expected default TTL %v, got %v", DefaultPkgCacheTTL, got)
	}

	s.SetCacheTTL(72 * time.Hour)
	if got := s.cacheTTL(); got != 72*time.Hour {
		t.Fatalf("expected TTL 72h, got %v", got)
	}

	s.SetCacheTTL(0)
	if got := s.cacheTTL(); got != DefaultPkgCacheTTL {
		t.Fatalf("expected default TTL %v after reset, got %v", DefaultPkgCacheTTL, got)
	}
}

func TestPkgCacheState_refresh(t *testing.T) {
	s := new(pkgCacheState)

	if !s.refreshDue() || !s.canAttemptRefresh() {
		t.Fatalf("expected refresh to be due when never refreshed")
	}

	// failed refresh is not retried before pkgIndexRetryInterval has passed
	s.markRefreshAttempt()
	if s.refreshDue() || s.canAttemptRefresh() {
		t.Fatalf("expected no refresh right after a failed attempt")
	}

	s.lastRefreshAttempt = time.Now().Add(-pkgIndexRetryInterval)
	if !s.refreshDue() || !s.canAttemptRefresh() {
		t.Fatalf("expected failed refresh to be retried after retry interval")
	}

	// successful refresh is valid until the cache TTL has passed
	s.markRefreshAttempt()
	s.markRefreshed()
	s.lastRefreshAttempt = time.Now().Add(-2 * time.Hour)
	s.lastRefresh = s.lastRefreshAttempt
	if s.refreshDue() {
		t.Fatalf("expected no refresh due within cache TTL")
	}

	// reducing the TTL makes the refresh due immediately
	s.SetCacheTTL(time.Hour)
	if !s.refreshDue() {
		t.Fatalf("expected refresh to be due after reducing the TTL")
	}
}
