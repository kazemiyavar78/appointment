package analytics

import (
	"context"
	"time"
)

const (
	// BrowserOther is stored when the User-Agent does not match a known browser.
	BrowserOther = "Other"
	// BrowserChrome is Google Chrome (including Chromium-based mobile Chrome).
	BrowserChrome = "Chrome"
	// BrowserFirefox is Mozilla Firefox.
	BrowserFirefox = "Firefox"
	// BrowserSafari is Apple Safari.
	BrowserSafari = "Safari"
	// BrowserEdge is Microsoft Edge.
	BrowserEdge = "Edge"
	// BrowserOpera is Opera.
	BrowserOpera = "Opera"
	// BrowserSamsung is Samsung Internet.
	BrowserSamsung = "Samsung Internet"
	// BrowserIE is legacy Internet Explorer.
	BrowserIE = "Internet Explorer"

	// OSUnknown is stored when the User-Agent does not match a known OS.
	OSUnknown = "Unknown"
	// OSWindows is any Microsoft Windows desktop/server version.
	OSWindows = "Windows"
	// OSAndroid is Google Android.
	OSAndroid = "Android"
	// OSIOS is Apple iPhone/iPad/iPod OS.
	OSIOS = "iOS"
	// OSMacOS is Apple macOS.
	OSMacOS = "macOS"
	// OSLinux is Linux other than Android.
	OSLinux = "Linux"
)

// VisitInfo is the request snapshot buffered in cache and later persisted.
type VisitInfo struct {
	IPAddress    string
	Host         string
	ClinicID     *uint
	Path         string
	FullURL      string
	UserAgent    string
	Referrer     string
	Browser      string
	OS           string
	IsFromGoogle bool
	VisitedAt    time.Time
}

// VisitTracker records a visit without exposing cache or database details.
type VisitTracker interface {
	Track(ctx context.Context, info VisitInfo) error
}

// VisitCacheStore buffers visits in the process cache until the flush worker persists them.
type VisitCacheStore interface {
	PushVisit(ctx context.Context, info VisitInfo) error
	FlushAll(ctx context.Context) ([]VisitInfo, error)
}

// VisitRepository upserts visitor_ips and inserts visitor_visit_details.
type VisitRepository interface {
	PersistVisits(ctx context.Context, visits []VisitInfo) error
}

// UserAgentParser extracts a known Browser and OS name from a User-Agent string.
type UserAgentParser interface {
	Parse(userAgent string) (browser string, osName string)
}

// ReferrerAnalyzer decides whether a visit originated from Google.
type ReferrerAnalyzer interface {
	IsFromGoogle(referrer, fullURL string) bool
}
