package analytics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

func TestMemoryVisitCache_ConcurrentPushAndFlush(t *testing.T) {
	c := NewMemoryVisitCache()
	var wg sync.WaitGroup
	const n = 50
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_ = c.PushVisit(context.Background(), VisitInfo{IPAddress: "1.1.1.1", Path: "/"})
		}()
	}
	wg.Wait()
	if c.Len() != n {
		t.Fatalf("len = %d, want %d", c.Len(), n)
	}
	got, err := c.FlushAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != n {
		t.Fatalf("flushed = %d, want %d", len(got), n)
	}
	if c.Len() != 0 {
		t.Fatalf("buffer should be empty after flush, len=%d", c.Len())
	}
}

func TestTracker_FillsBrowserOSAndGoogle(t *testing.T) {
	cache := NewMemoryVisitCache()
	tracker := NewTracker(cache, NewUserAgentParser(), NewReferrerAnalyzer())
	err := tracker.Track(context.Background(), VisitInfo{
		IPAddress: "8.8.8.8",
		Path:      "/",
		FullURL:   "/?utm_source=google",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Referrer:  "https://www.google.com/",
	})
	if err != nil {
		t.Fatal(err)
	}
	items, _ := cache.FlushAll(context.Background())
	if len(items) != 1 {
		t.Fatalf("len=%d", len(items))
	}
	v := items[0]
	if v.Browser != BrowserChrome {
		t.Fatalf("browser=%q", v.Browser)
	}
	if v.OS != OSWindows {
		t.Fatalf("os=%q", v.OS)
	}
	if !v.IsFromGoogle {
		t.Fatal("expected IsFromGoogle")
	}
}

func TestTracker_SkipsEmptyIP(t *testing.T) {
	cache := NewMemoryVisitCache()
	tracker := NewTracker(cache, NewUserAgentParser(), NewReferrerAnalyzer())
	if err := tracker.Track(context.Background(), VisitInfo{Path: "/"}); err != nil {
		t.Fatal(err)
	}
	if cache.Len() != 0 {
		t.Fatal("empty IP must not be buffered")
	}
}

func TestClinicIDForVisit_NilOnPlatformWithoutClinic(t *testing.T) {
	if got := ClinicIDForVisit(constants.LayoutPlatform, "", nil); got != nil {
		t.Fatalf("want nil, got %v", *got)
	}
	zero := uint(0)
	if got := ClinicIDForVisit(constants.LayoutOrgan, "", &zero); got != nil {
		t.Fatalf("want nil for zero id, got %v", *got)
	}
	id := uint(12)
	got := ClinicIDForVisit(constants.LayoutPrivate, "", &id)
	if got == nil || *got != 12 {
		t.Fatalf("want 12, got %v", got)
	}
	if got == &id {
		t.Fatal("clinic id pointer must be copied")
	}
}

func TestClientIP_PrefersForwardedFor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/doctors?q=1", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.RemoteAddr = "127.0.0.1:1234"
	c.Request = req
	if ip := ClientIP(c); ip != "203.0.113.10" {
		t.Fatalf("ip=%q", ip)
	}
}

func TestBuildVisitInfo_KeepsExactPathAndQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/news/12?utm_source=google&x=1", nil)
	req.Host = "clinic.example.ir"
	req.Header.Set("User-Agent", "curl/8.0.0")
	req.Header.Set("Referer", "https://www.google.com/")
	c.Request = req
	id := uint(7)
	info := BuildVisitInfo(c, &id)
	if info.Path != "/news/12" {
		t.Fatalf("path=%q", info.Path)
	}
	if info.FullURL != "/news/12?utm_source=google&x=1" {
		t.Fatalf("fullURL=%q", info.FullURL)
	}
	if info.Host != "clinic.example.ir" {
		t.Fatalf("host=%q", info.Host)
	}
	if info.ClinicID == nil || *info.ClinicID != 7 {
		t.Fatalf("clinicID=%v", info.ClinicID)
	}
}

func TestShouldSkipPath(t *testing.T) {
	if !ShouldSkipPath("/static/css/output.css") {
		t.Fatal("static should skip")
	}
	if !ShouldSkipPath("/ws/booking/x") {
		t.Fatal("ws should skip")
	}
	if ShouldSkipPath("/doctors") {
		t.Fatal("public page should be tracked")
	}
}

func TestFlusher_RequeuesOnPersistError(t *testing.T) {
	cache := NewMemoryVisitCache()
	_ = cache.PushVisit(context.Background(), VisitInfo{IPAddress: "1.2.3.4", Path: "/"})
	f := NewFlusher(cache, failRepo{}, timeIntervalForTest)
	if err := f.FlushNow(context.Background()); err == nil {
		t.Fatal("expected persist error")
	}
	if cache.Len() != 1 {
		t.Fatalf("visit should be requeued, len=%d", cache.Len())
	}
}

type failRepo struct{}

// PersistVisits always fails so FlushNow can be tested for re-queue behavior.
func (failRepo) PersistVisits(context.Context, []VisitInfo) error {
	return context.DeadlineExceeded
}

const timeIntervalForTest = DefaultFlushInterval
