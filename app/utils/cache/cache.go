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
	"sync"
	"time"
)

type item struct {
	expires time.Time
	data    any
}

var mutex sync.Mutex
var items = make(map[string]item)

// Get returns cached item and true for provided cache key if item is fresh in cache.
func Get(key string) (any, bool) {
	mutex.Lock()
	defer mutex.Unlock()

	cacheItem, ok := items[key]
	if !ok {
		return nil, false
	}

	if cacheItem.expires.Before(time.Now()) {
		return nil, false
	}

	return cacheItem.data, true
}

// Set for provided cache key.
func Set(key string, data any, ttl time.Duration) {
	mutex.Lock()
	defer mutex.Unlock()

	items[key] = item{
		expires: time.Now().Add(ttl),
		data:    data,
	}
}

// Delete for provided cache key.
func Delete(key string) {
	mutex.Lock()
	defer mutex.Unlock()

	delete(items, key)
}
