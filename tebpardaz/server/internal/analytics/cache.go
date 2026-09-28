package analytics

import (
	"context"
	"sync"
)

// MemoryVisitCache is a thread-safe in-memory append list for buffered visits.
// The server uses patrickmn/go-cache (not Redis); this store is the list/stream
// equivalent so concurrent Track calls and the flush worker cannot race.
type MemoryVisitCache struct {
	mu  sync.Mutex
	buf []VisitInfo
}

// NewMemoryVisitCache constructs an empty VisitCacheStore.
// Inputs: none.
// Output: pointer to MemoryVisitCache ready for concurrent PushVisit/FlushAll.
func NewMemoryVisitCache() *MemoryVisitCache {
	return &MemoryVisitCache{}
}

// PushVisit appends one visit to the buffer.
// Inputs: ctx (unused; reserved for Redis-backed implementations), info to buffer.
// Output: always nil error.
func (c *MemoryVisitCache) PushVisit(_ context.Context, info VisitInfo) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	c.buf = append(c.buf, info)
	c.mu.Unlock()
	return nil
}

// FlushAll atomically takes ownership of the current buffer and leaves it empty.
// Inputs: ctx (unused; reserved for Redis-backed implementations).
// Output: the drained visits (possibly empty/nil) and nil error.
func (c *MemoryVisitCache) FlushAll(_ context.Context) ([]VisitInfo, error) {
	if c == nil {
		return nil, nil
	}
	c.mu.Lock()
	out := c.buf
	c.buf = nil
	c.mu.Unlock()
	return out, nil
}

// Len returns the number of buffered visits (for tests and diagnostics).
// Inputs: none.
// Output: current buffer length.
func (c *MemoryVisitCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	n := len(c.buf)
	c.mu.Unlock()
	return n
}
