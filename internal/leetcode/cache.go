package leetcode

import (
	"sync"
	"time"
)

// cacheEntry holds a cached value along with its expiration time.
type cacheEntry struct {
	value     any
	expiresAt time.Time
}

// Cache is a thread-safe in-memory key-value cache with per-entry TTL expiry.
// Values are stored as interface{} keyed by string. Expired entries are lazily
// removed on access — no background goroutine is needed.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

// NewCache creates a new empty Cache.
func NewCache() *Cache {
	return &Cache{
		entries: make(map[string]cacheEntry),
	}
}

// Get retrieves a value from the cache by key. It returns the value and true
// if the key exists and has not expired. If the key is missing or expired,
// it returns (nil, false). Expired entries are deleted on access.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		// Entry has expired — remove it.
		c.mu.Lock()
		// Double-check under write lock to avoid race with concurrent Set.
		if e, exists := c.entries[key]; exists && time.Now().After(e.expiresAt) {
			delete(c.entries, key)
		}
		c.mu.Unlock()
		return nil, false
	}

	return entry.value, true
}

// Set stores a value in the cache with the given TTL duration. If the key
// already exists, its value and expiry are overwritten.
func (c *Cache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	c.entries[key] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

// Default TTL values for different types of cached data.
const (
	// StatsCacheTTL is the TTL for user stats data (10 minutes).
	// Stats change relatively frequently as users solve problems.
	StatsCacheTTL = 10 * time.Minute

	// ProfileCacheTTL is the TTL for user profile data (1 hour).
	// Profile information changes infrequently.
	ProfileCacheTTL = 1 * time.Hour

	// DailyChallengeCacheTTL is the TTL for the daily challenge (1 hour).
	// The daily challenge changes once per day, but we use a shorter TTL
	// to pick up the new challenge reasonably quickly.
	DailyChallengeCacheTTL = 1 * time.Hour
)
