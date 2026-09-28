package cache

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const nationalIDDailyCap = 5

// NationalIDLimiter سقف کدملی تازه هر IP را تا پایان روز تهران در go-cache نگه می‌دارد.
// این منبع تصمیم بلادرنگ است و به جدول سابقه دیتابیس وابسته نیست.
type NationalIDLimiter struct {
	mu    sync.Mutex
	store *Store
	now   func() time.Time
}

// NewNationalIDLimiter سازنده محدودیت روزانه است.
// ورودی: store حافظه. خروجی: محدودکننده با ساعت سیستم.
func NewNationalIDLimiter(store *Store) *NationalIDLimiter {
	return &NationalIDLimiter{store: store, now: time.Now}
}

// WouldAllow بدون ثبت کردن می‌گوید این کدملی امروز برای IP مجاز است یا نه.
// کدملی خالی همیشه مجاز است و در سقف پنج‌تایی شمرده نمی‌شود.
// ورودی: IP کلاینت و کدملی نرمال‌شده. خروجی: true اگر رد نشود.
func (l *NationalIDLimiter) WouldAllow(ip, nationalID string) bool {
	if strings.TrimSpace(nationalID) == "" {
		return true
	}
	if l == nil || l.store == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	seen := l.seen(ip)
	if _, ok := seen[nationalID]; ok {
		return true
	}
	return len(seen) < nationalIDDailyCap
}

// Allow کدملی جدید را در مجموعه امروز IP ثبت می‌کند.
// اگر از قبل بوده باشد دوباره مجاز است. اگر مجموعه به ۵ رسیده باشد false برمی‌گرداند و چیزی اضافه نمی‌کند.
// ورودی: IP کلاینت و کدملی نرمال‌شده. خروجی: true اگر درخواست مجاز باشد.
func (l *NationalIDLimiter) Allow(ip, nationalID string) bool {
	nationalID = strings.TrimSpace(nationalID)
	if nationalID == "" {
		return true
	}
	if l == nil || l.store == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	seen := l.seen(ip)
	if _, ok := seen[nationalID]; ok {
		return true
	}
	if len(seen) >= nationalIDDailyCap {
		return false
	}
	next := make(map[string]struct{}, len(seen)+1)
	for id := range seen {
		next[id] = struct{}{}
	}
	next[nationalID] = struct{}{}
	day, ttl := tehranDayWindow(l.current())
	l.store.SetWithTTL(nationalLimitKey(ip, day), next, ttl)
	return true
}

func (l *NationalIDLimiter) current() time.Time {
	if l != nil && l.now != nil {
		return l.now()
	}
	return time.Now()
}

func (l *NationalIDLimiter) seen(ip string) map[string]struct{} {
	day, _ := tehranDayWindow(l.current())
	raw, ok := l.store.Get(nationalLimitKey(ip, day))
	if !ok {
		return map[string]struct{}{}
	}
	seen, ok := raw.(map[string]struct{})
	if !ok || seen == nil {
		return map[string]struct{}{}
	}
	return seen
}

func nationalLimitKey(ip, day string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		ip = "unknown"
	}
	return fmt.Sprintf("ratelimit:ip:%s:%s", ip, day)
}

// tehranDayWindow تاریخ امروز تهران و مدت باقی‌مانده تا نیمه‌شب را برمی‌گرداند.
// ورودی: لحظه فعلی. خروجی: روز YYYY-MM-DD و TTL تا ابتدای روز بعد.
func tehranDayWindow(now time.Time) (string, time.Duration) {
	loc, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		loc = time.FixedZone("IRST", 3*3600+30*60)
	}
	local := now.In(loc)
	next := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc)
	ttl := next.Sub(now)
	if ttl < time.Second {
		ttl = time.Second
	}
	return local.Format("2006-01-02"), ttl
}
