package testresult

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// defaultEvery: sustained ~1 lookup every 3 seconds.
	defaultEvery = 3 * time.Second
	// defaultBurst: allow a short burst of 5 attempts.
	defaultBurst = 5
)

// RateLimiter limits test-result lookups per client IP.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	every    rate.Limit
	burst    int
}

// NewRateLimiter constructs a per-IP RateLimiter with default limits.
// Inputs: none.
// Output: pointer to RateLimiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		every:    rate.Every(defaultEvery),
		burst:    defaultBurst,
	}
}

// Allow reports whether another lookup is permitted for the given IP.
// Inputs: client IP (empty treated as "unknown").
// Output: true when under the rate limit.
func (r *RateLimiter) Allow(ip string) bool {
	if r == nil {
		return true
	}
	ip = normalizeIP(ip)
	r.mu.Lock()
	defer r.mu.Unlock()
	lim, ok := r.limiters[ip]
	if !ok {
		lim = rate.NewLimiter(r.every, r.burst)
		r.limiters[ip] = lim
	}
	return lim.Allow()
}

// normalizeIP returns a non-empty IP key for the limiter map.
func normalizeIP(ip string) string {
	if ip == "" {
		return "unknown"
	}
	return ip
}
