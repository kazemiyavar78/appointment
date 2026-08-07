package booking

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// defaultBookingRate: 1 request per 3 seconds sustained, burst 3.
	defaultBookingEvery = 3 * time.Second
	defaultBookingBurst = 3
	strikeTTL           = 24 * time.Hour
	banTTL              = 24 * time.Hour
)

// Guard enforces per-IP rate limits and bans after repeated duplicate-booking attempts.
type Guard struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	strikes  map[string]strikeEntry
	bans     map[string]time.Time
	every    rate.Limit
	burst    int
}

type strikeEntry struct {
	Count   int
	Expires time.Time
}

// NewGuard constructs a booking Guard with default limits.
// Inputs: none.
// Output: pointer to Guard.
func NewGuard() *Guard {
	return &Guard{
		limiters: make(map[string]*rate.Limiter),
		strikes:  make(map[string]strikeEntry),
		bans:     make(map[string]time.Time),
		every:    rate.Every(defaultBookingEvery),
		burst:    defaultBookingBurst,
	}
}

// Allow reports whether the IP may attempt a booking right now.
// Inputs: client IP.
// Output: allowed flag and Persian reason when blocked.
// After one duplicate-NID strike, the next request from the same IP is banned.
func (g *Guard) Allow(ip string) (bool, string) {
	if g == nil {
		return true, ""
	}
	ip = normalizeIP(ip)
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cleanupLocked(now)

	if until, banned := g.bans[ip]; banned && now.Before(until) {
		return false, "دسترسی این IP به ثبت نوبت مسدود شده است"
	}
	if entry, ok := g.strikes[ip]; ok && entry.Count >= 1 && now.Before(entry.Expires) {
		g.bans[ip] = now.Add(banTTL)
		delete(g.strikes, ip)
		return false, "دسترسی این IP به ثبت نوبت مسدود شده است"
	}
	lim := g.limiterLocked(ip)
	if !lim.Allow() {
		return false, "تعداد درخواست‌ها زیاد است؛ کمی بعد دوباره تلاش کنید"
	}
	return true, ""
}

// RecordDuplicateAttempt records that this IP tried to book with a national ID that already has a booking.
// Inputs: client IP.
// Output: none. The next Allow() call for this IP will ban it.
func (g *Guard) RecordDuplicateAttempt(ip string) {
	if g == nil {
		return
	}
	ip = normalizeIP(ip)
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cleanupLocked(now)

	entry := g.strikes[ip]
	if entry.Expires.Before(now) {
		entry = strikeEntry{}
	}
	entry.Count++
	entry.Expires = now.Add(strikeTTL)
	g.strikes[ip] = entry
}

func (g *Guard) limiterLocked(ip string) *rate.Limiter {
	if lim, ok := g.limiters[ip]; ok {
		return lim
	}
	lim := rate.NewLimiter(g.every, g.burst)
	g.limiters[ip] = lim
	return lim
}

func (g *Guard) cleanupLocked(now time.Time) {
	for ip, until := range g.bans {
		if now.After(until) {
			delete(g.bans, ip)
		}
	}
	for ip, entry := range g.strikes {
		if now.After(entry.Expires) {
			delete(g.strikes, ip)
		}
	}
}

func normalizeIP(ip string) string {
	if ip == "" {
		return "unknown"
	}
	return ip
}
