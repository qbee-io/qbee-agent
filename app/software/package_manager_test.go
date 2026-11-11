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

func TestPkgCacheTTLSetting(t *testing.T) {
	s := new(pkgCacheTTLSetting)

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
