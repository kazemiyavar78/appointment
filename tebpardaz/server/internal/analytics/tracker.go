package analytics

import (
	"context"
	"strings"
	"sync"
)

// Tracker enriches a visit (browser/OS/Google) and pushes it into the cache store.
type Tracker struct {
	cache    VisitCacheStore
	ua       UserAgentParser
	referrer ReferrerAnalyzer
	wg       sync.WaitGroup
}

// NewTracker constructs a VisitTracker that depends only on the given abstractions.
// Inputs: cache store, UA parser, referrer analyzer (nil parser/analyzer get defaults).
// Output: pointer to Tracker.
func NewTracker(cache VisitCacheStore, ua UserAgentParser, referrer ReferrerAnalyzer) *Tracker {
	if ua == nil {
		ua = NewUserAgentParser()
	}
	if referrer == nil {
		referrer = NewReferrerAnalyzer()
	}
	return &Tracker{cache: cache, ua: ua, referrer: referrer}
}

// Track parses UA/referrer, then buffers the visit. It never panics on nil fields.
// Inputs: ctx forwarded to the cache store, info copied from the HTTP request.
// Output: cache error, if any; empty IP is ignored.
func (t *Tracker) Track(ctx context.Context, info VisitInfo) error {
	if t == nil || t.cache == nil {
		return nil
	}
	if strings.TrimSpace(info.IPAddress) == "" {
		return nil
	}
	info.Browser, info.OS = t.ua.Parse(info.UserAgent)
	info.IsFromGoogle = t.referrer.IsFromGoogle(info.Referrer, info.FullURL)
	if info.ClinicID != nil && *info.ClinicID == 0 {
		info.ClinicID = nil
	}
	return t.cache.PushVisit(ctx, info)
}

// TrackAsync runs Track in a goroutine and records it on an internal WaitGroup.
// Inputs: info copied from the request (do not pass gin.Context into the goroutine).
// Output: none; errors are returned only via the WaitGroup-tracked Track call site in middleware.
func (t *Tracker) TrackAsync(info VisitInfo, onErr func(error)) {
	if t == nil {
		return
	}
	t.wg.Add(1)
	go func(info VisitInfo) {
		defer t.wg.Done()
		if err := t.Track(context.Background(), info); err != nil && onErr != nil {
			onErr(err)
		}
	}(info)
}

// Wait blocks until in-flight TrackAsync calls finish (used on graceful shutdown).
// Inputs: none.
// Output: none.
func (t *Tracker) Wait() {
	if t == nil {
		return
	}
	t.wg.Wait()
}
