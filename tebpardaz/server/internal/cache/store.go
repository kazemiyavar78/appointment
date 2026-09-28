package cache

import (
	"sync"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

const (
	// DefaultExpiration is the default TTL for cache entries (5 minutes).
	DefaultExpiration = 5 * time.Minute
	// CleanupInterval is how often expired items are purged (10 minutes).
	CleanupInterval = 10 * time.Minute
)

var (
	initOnce sync.Once
	global   *Store
)

// Store is a process-wide in-memory key/value cache.
// It wraps patrickmn/go-cache, which is safe for concurrent use by multiple goroutines.
type Store struct {
	c *gocache.Cache
}

// New constructs a Store with the given default TTL and cleanup interval.
// Inputs: defaultExpiration (TTL for Set with DefaultExpiration), cleanupInterval (janitor period).
// Output: a ready-to-use, concurrency-safe Store.
func New(defaultExpiration, cleanupInterval time.Duration) *Store {
	return &Store{
		c: gocache.New(defaultExpiration, cleanupInterval),
	}
}

// Init creates the process-wide Store exactly once (safe under concurrent callers).
// Inputs: defaultExpiration and cleanupInterval for the underlying go-cache instance.
// Output: the singleton Store; later Init calls return the same instance and ignore new durations.
func Init(defaultExpiration, cleanupInterval time.Duration) *Store {
	initOnce.Do(func() {
		global = New(defaultExpiration, cleanupInterval)
	})
	return global
}

// Default returns the Store created by Init.
// Panics if Init has not been called yet.
func Default() *Store {
	if global == nil {
		panic("cache: Init must be called before Default")
	}
	return global
}

// Set stores value under key using the store's default expiration.
// Inputs: key, value (any).
// Output: none.
func (s *Store) Set(key string, value any) {
	s.c.Set(key, value, gocache.DefaultExpiration)
}

// SetWithTTL stores value under key for the given TTL.
// Inputs: key, value, ttl (use NoExpiration for no expiry).
// Output: none.
func (s *Store) SetWithTTL(key string, value any, ttl time.Duration) {
	s.c.Set(key, value, ttl)
}

// SetNoExpire stores value under key with no expiration.
// Inputs: key, value.
// Output: none.
func (s *Store) SetNoExpire(key string, value any) {
	s.c.Set(key, value, gocache.NoExpiration)
}

// Get returns the raw cached value for key.
// Inputs: key.
// Output: value and found flag.
func (s *Store) Get(key string) (any, bool) {
	return s.c.Get(key)
}

// ReplaceKeepTTL مقدار را عوض می‌کند ولی زمان انقضای قبلی کلید را نگه می‌دارد.
// ورودی: key، value، fallback (اگر کلید نبود یا منقضی شده بود).
// خروجی: ندارد.
func (s *Store) ReplaceKeepTTL(key string, value any, fallback time.Duration) {
	if s == nil || s.c == nil {
		return
	}
	_, exp, found := s.c.GetWithExpiration(key)
	if !found {
		s.c.Set(key, value, fallback)
		return
	}
	ttl := time.Until(exp)
	if ttl <= 0 {
		s.c.Set(key, value, fallback)
		return
	}
	s.c.Set(key, value, ttl)
}

// GetTyped retrieves key and asserts it to T.
// Inputs: store and key.
// Output: typed value and true when present and of type T; otherwise zero value and false.
func GetTyped[T any](s *Store, key string) (T, bool) {
	var zero T
	raw, found := s.c.Get(key)
	if !found {
		return zero, false
	}
	v, ok := raw.(T)
	if !ok {
		return zero, false
	}
	return v, true
}

// Delete removes key from the cache.
// Inputs: key.
// Output: none.
func (s *Store) Delete(key string) {
	s.c.Delete(key)
}

// Flush removes all items from the cache.
// Inputs: none.
// Output: none.
func (s *Store) Flush() {
	s.c.Flush()
}

// ItemCount returns the number of items in the cache (including expired, not yet purged).
// Inputs: none.
// Output: item count.
func (s *Store) ItemCount() int {
	return s.c.ItemCount()
}

// NoExpiration is an alias for go-cache's no-expiry sentinel.
const NoExpiration = gocache.NoExpiration
