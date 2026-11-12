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

package cache

import (
	"testing"
	"time"
)

func TestUpdateTTL(t *testing.T) {
	defer Clear()

	const key = "test"

	// item cached 2 hours ago with 3 hours TTL
	Set(key, "data", 3*time.Hour)
	mutex.Lock()
	cachedItem := items[key]
	cachedItem.created = cachedItem.created.Add(-2 * time.Hour)
	cachedItem.expires = cachedItem.expires.Add(-2 * time.Hour)
	items[key] = cachedItem
	mutex.Unlock()

	// extending the TTL to 4 hours makes the item expire 2 hours from now
	UpdateTTL(key, 4*time.Hour)
	if _, ok := Get(key); !ok {
		t.Fatalf("expected item to be cached after extending TTL")
	}

	mutex.Lock()
	expiresIn := time.Until(items[key].expires)
	mutex.Unlock()

	if expiresIn < 2*time.Hour-time.Minute || expiresIn > 2*time.Hour {
		t.Fatalf("expected item to expire in 2 hours, expires in %v", expiresIn)
	}

	// reducing the TTL below the item's age expires it immediately
	UpdateTTL(key, time.Hour)
	if _, ok := Get(key); ok {
		t.Fatalf("expected item to be expired after reducing TTL")
	}

	// updating TTL of non-existing item is a no-op
	UpdateTTL("missing", time.Hour)
	if _, ok := Get("missing"); ok {
		t.Fatalf("expected missing item not to be cached")
	}
}
