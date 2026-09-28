package public

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestShowRejectsUnknownCategory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewFileDownloadHandler(nil, nil)
	r.GET("/storage/:category/:month/:clinic_code/:filename", h.Show)

	req := httptest.NewRequest(http.MethodGet, "/storage/nope/2026-09/12/result.pdf", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestAdsRejectsUnknownCategory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewFileDownloadHandler(nil, nil)
	r.POST("/storage/:category/:month/:clinic_code/:filename/ads", h.Ads)

	req := httptest.NewRequest(http.MethodPost, "/storage/nope/2026-09/12/result.pdf/ads", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestSanitizePublicLink(t *testing.T) {
	if got := sanitizePublicLink(" https://clinic.example/book "); got != "https://clinic.example/book" {
		t.Fatalf("https link: %q", got)
	}
	if got := sanitizePublicLink("/booking"); got != "/booking" {
		t.Fatalf("path: %q", got)
	}
	for _, raw := range []string{"javascript:alert(1)", "//evil.test", "data:text/html,hi", ""} {
		if sanitizePublicLink(raw) != "" {
			t.Fatalf("kept unsafe link %q", raw)
		}
	}
}

func TestAdIsLiveHonorsSchedule(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.Local)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	live := downloadAd{ImageFile: "a.png", StartsAt: &past, EndsAt: &future}
	if !adIsLive(live, now) {
		t.Fatal("expected live ad")
	}
	expired := downloadAd{ImageFile: "a.png", EndsAt: &past}
	if adIsLive(expired, now) {
		t.Fatal("expired ad was live")
	}
	if adIsLive(downloadAd{ImageFile: "../a.png"}, now) {
		t.Fatal("unsafe image was accepted")
	}
}
