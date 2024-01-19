package cache

import (
	"sync"
	"time"
)

type cacheMap struct {
	mutex sync.Mutex
	items map[string]cacheItem
}

type cacheItem struct {
	expires time.Time
	item    any
}

var appCacheMap cacheMap

func init() {
	appCacheMap.items = make(map[string]cacheItem)
}

// GetCachedItem returns cached item and true for provided cache key if item is fresh in cache.
func GetCachedItem(key string) (any, bool) {
	appCacheMap.mutex.Lock()
	defer appCacheMap.mutex.Unlock()

	cacheItem, ok := appCacheMap.items[key]
	if !ok {
		return nil, false
	}

	if cacheItem.expires.Before(time.Now()) {
		return nil, false
	}

	return cacheItem.item, true
}

// SetCachedItem for provided cache key.
func SetCachedItem(key string, item any, ttl time.Duration) {
	appCacheMap.mutex.Lock()
	defer appCacheMap.mutex.Unlock()

	appCacheMap.items[key] = cacheItem{
		expires: time.Now().Add(ttl),
		item:    item,
	}
}

// InvalidateCache for provided cache key.
func InvalidateCache(key string) {
	appCacheMap.mutex.Lock()
	defer appCacheMap.mutex.Unlock()

	delete(appCacheMap.items, key)
}
