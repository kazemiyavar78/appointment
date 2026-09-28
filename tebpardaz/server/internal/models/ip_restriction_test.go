package models

import (
	"testing"
	"time"
)

// TestIPRestrictionAllowHit رفتار بلاک، انقضا و پنجره rate limit را بررسی می‌کند.
func TestIPRestrictionAllowHit(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	block := &IPRestriction{Kind: IPRestrictionKindBlock}
	if block.AllowHit(now) {
		t.Fatal("block should deny")
	}
	past := now.Add(-time.Minute)
	block.ExpiresAt = &past
	if !block.AllowHit(now) {
		t.Fatal("expired block should allow")
	}

	limit := &IPRestriction{Kind: IPRestrictionKindRateLimit, MaxRequests: 2, WindowSeconds: 60}
	if !limit.AllowHit(now) || limit.HitCount != 1 {
		t.Fatalf("first hit = %d", limit.HitCount)
	}
	if !limit.AllowHit(now.Add(time.Second)) || limit.HitCount != 2 {
		t.Fatalf("second hit = %d", limit.HitCount)
	}
	if limit.AllowHit(now.Add(2 * time.Second)) {
		t.Fatal("third hit should be denied")
	}
	if !limit.AllowHit(now.Add(61*time.Second)) || limit.HitCount != 1 {
		t.Fatalf("new window hit = %d", limit.HitCount)
	}
}
