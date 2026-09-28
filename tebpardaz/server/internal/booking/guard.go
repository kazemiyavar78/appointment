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
	onBan    func(ip, reason string, until time.Time)
}

type strikeEntry struct {
	Count   int
	Expires time.Time
}

// SetBanPersister تابعی را نگه می‌دارد که هنگام بلاک تازه آی‌پی صدا زده می‌شود.
// ورودی: fn؛ nil یعنی ذخیره بلاک خاموش است. خروجی: ندارد.
func (g *Guard) SetBanPersister(fn func(ip, reason string, until time.Time)) {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.onBan = fn
	g.mu.Unlock()
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

// Allow مشخص می‌کند این آی‌پی الان اجازه ثبت نوبت دارد یا نه.
// ورودی: آی‌پی کلاینت. خروجی: مجاز بودن و دلیل فارسی در صورت رد.
// بعد از یک تلاش تکراری کد ملی، درخواست بعدی همان آی‌پی بلاک می‌شود.
func (g *Guard) Allow(ip string) (bool, string) {
	if g == nil {
		return true, ""
	}
	ip = normalizeIP(ip)
	now := time.Now()
	g.mu.Lock()
	allowed, msg, banFn, banUntil := g.allowLocked(ip, now)
	g.mu.Unlock()
	if banFn != nil {
		banFn(ip, "تلاش تکراری ثبت نوبت", banUntil)
	}
	return allowed, msg
}

// allowLocked تصمیم درخواست نوبت را در حالی که قفل نگه داشته شده می‌گیرد.
// ورودی: ip و now. خروجی: مجاز بودن، پیام فارسی، تابع بلاک اختیاری، زمان پایان بلاک.
func (g *Guard) allowLocked(ip string, now time.Time) (bool, string, func(string, string, time.Time), time.Time) {
	g.cleanupLocked(now)

	if until, banned := g.bans[ip]; banned && now.Before(until) {
		return false, "دسترسی این IP به ثبت نوبت مسدود شده است", nil, time.Time{}
	}
	if entry, ok := g.strikes[ip]; ok && entry.Count >= 1 && now.Before(entry.Expires) {
		until := now.Add(banTTL)
		g.bans[ip] = until
		delete(g.strikes, ip)
		return false, "دسترسی این IP به ثبت نوبت مسدود شده است", g.onBan, until
	}
	lim := g.limiterLocked(ip)
	if !lim.Allow() {
		return false, "تعداد درخواست‌ها زیاد است؛ کمی بعد دوباره تلاش کنید", nil, time.Time{}
	}
	return true, "", nil, time.Time{}
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
