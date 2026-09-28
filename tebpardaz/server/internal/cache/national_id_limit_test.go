package cache

import (
	"testing"
	"time"
)

func TestNationalIDLimiter_DailyCap(t *testing.T) {
	store := New(time.Hour, time.Minute)
	limiter := NewNationalIDLimiter(store)
	fixed := time.Date(2026, 9, 27, 18, 0, 0, 0, time.FixedZone("IRST", 3*3600+30*60))
	limiter.now = func() time.Time { return fixed }

	ip := "203.0.113.10"
	ids := []string{"0011111111", "0022222222", "0033333333", "0044444444", "0055555555"}
	for _, id := range ids {
		if !limiter.WouldAllow(ip, id) || !limiter.Allow(ip, id) {
			t.Fatalf("expected %s to be allowed", id)
		}
	}
	if limiter.WouldAllow(ip, "0066666666") || limiter.Allow(ip, "0066666666") {
		t.Fatal("sixth national id must be rejected")
	}
	if !limiter.WouldAllow(ip, ids[0]) || !limiter.Allow(ip, ids[0]) {
		t.Fatal("repeat national id must stay allowed")
	}
	if !limiter.Allow(ip, "") {
		t.Fatal("empty national id must not consume a slot")
	}
}

func TestTehranDayWindow_UntilMidnight(t *testing.T) {
	loc := time.FixedZone("IRST", 3*3600+30*60)
	now := time.Date(2026, 9, 27, 23, 0, 0, 0, loc)
	day, ttl := tehranDayWindow(now)
	if day != "2026-09-27" {
		t.Fatalf("day = %s", day)
	}
	if ttl < 59*time.Minute || ttl > 61*time.Minute {
		t.Fatalf("ttl = %s, want about 1h", ttl)
	}
}
