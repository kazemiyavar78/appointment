package repository

import (
	"strings"
	"testing"
)

// TestTrimReviewIP فاصله و طول آی‌پی ذخیره‌شده را بررسی می‌کند.
func TestTrimReviewIP(t *testing.T) {
	if got := trimReviewIP("  203.0.113.10  "); got != "203.0.113.10" {
		t.Fatalf("trim spaces: %q", got)
	}
	if got := trimReviewIP(strings.Repeat("a", 50)); len(got) != 45 {
		t.Fatalf("len = %d", len(got))
	}
}
