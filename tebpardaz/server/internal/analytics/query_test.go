package analytics

import (
	"context"
	"testing"
)

// TestNormalizePage checks default and clamped pagination values.
func TestNormalizePage(t *testing.T) {
	page, per := normalizePage(0, 0)
	if page != 1 || per != 50 {
		t.Fatalf("defaults: page=%d per=%d", page, per)
	}
	page, per = normalizePage(3, 500)
	if page != 3 || per != 200 {
		t.Fatalf("clamp per-page: page=%d per=%d", page, per)
	}
}

// TestTotalPages checks page-count math for empty and overflowing lists.
func TestTotalPages(t *testing.T) {
	if got := TotalPages(0, 50); got != 1 {
		t.Fatalf("empty list pages=%d", got)
	}
	if got := TotalPages(101, 50); got != 3 {
		t.Fatalf("101/50 pages=%d", got)
	}
}

// TestIsKnownBrowserAndOS rejects raw User-Agent strings as filter values.
func TestIsKnownBrowserAndOS(t *testing.T) {
	if !IsKnownBrowser(BrowserChrome) || IsKnownBrowser("Mozilla/5.0") {
		t.Fatal("browser allow-list failed")
	}
	if !IsKnownOS(OSAndroid) || IsKnownOS("Windows NT 10.0") {
		t.Fatal("os allow-list failed")
	}
}

// TestGetVisitorIPByAddressNilRepo returns nil without querying when the repository is empty.
func TestGetVisitorIPByAddressNilRepo(t *testing.T) {
	var r *GormVisitRepository
	got, err := r.GetVisitorIPByAddress(context.Background(), "203.0.113.10")
	if err != nil || got != nil {
		t.Fatalf("nil repo: got=%v err=%v", got, err)
	}
	r = &GormVisitRepository{}
	got, err = r.GetVisitorIPByAddress(context.Background(), "   ")
	if err != nil || got != nil {
		t.Fatalf("blank ip: got=%v err=%v", got, err)
	}
}
