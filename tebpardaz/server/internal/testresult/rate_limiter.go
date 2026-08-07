package testresult

import "golang.org/x/time/rate"

// RateLimiter limits test-result lookups per IP/national ID.
// TODO: choose keying strategy and limits from config.
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter constructs a RateLimiter with a placeholder limit.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(1), 5),
	}
}

// Allow reports whether another lookup is permitted for the key.
// TODO: use per-key limiters instead of a single shared limiter.
func (r *RateLimiter) Allow(_ string) bool {
	return r.limiter.Allow()
}
