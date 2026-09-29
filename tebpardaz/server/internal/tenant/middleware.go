package tenant

import (
	"context"
	"errors"
	"log"
	"net/http"

	"tebpardaz/server/internal/analytics"
	"tebpardaz/server/internal/seo"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

const (
	// ContextKey is the gin.Context key for *tenant.Context.
	ContextKey = "tenant"
	// LayoutKey is the gin.Context key for constants.LayoutKind.
	LayoutKey = "layout"
	// UnresolvedKey marks hosts/slugs that did not map to a tenant.
	UnresolvedKey = "tenant_unresolved"
)

// FallbackPlatform builds a platform layout context so a 404 page can still render.
// Input: request host (port is stripped). Output: tenant Context with LayoutPlatform.
func FallbackPlatform(host string) *Context {
	return &Context{
		Host:   normalizeHost(host),
		Layout: constants.LayoutPlatform,
	}
}

// IsUnresolved reports whether tenant resolution failed for this request.
// Input: gin context. Output: true when the host/slug was not a known tenant.
func IsUnresolved(c *gin.Context) bool {
	if c == nil {
		return false
	}
	v, ok := c.Get(UnresolvedKey)
	if !ok {
		return false
	}
	flag, _ := v.(bool)
	return flag
}

// Middleware resolves the request host/path into a tenant Context and layout.
// Unknown hosts or slugs continue with a platform fallback and UnresolvedKey
// so callers can render the public 404 page instead of an empty status.
// After layout is set, a visit is tracked asynchronously and never aborts the request.
// Inputs: resolver (domain/slug mapper), tracker (optional; nil disables analytics), trust (same proxy list as public URLs; nil trusts nobody).
// Output: gin.HandlerFunc that sets tenant + layout on the request context.
func Middleware(resolver *Resolver, tracker analytics.VisitTracker, trust *seo.ProxyTrust) gin.HandlerFunc {
	return func(c *gin.Context) {
		if resolver == nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		host := ""
		if c.Request != nil {
			host = seo.PublicRequestHost(c.Request, trust)
		}

		tc, err := resolver.Resolve(host, c.Request.URL.Path)
		if err != nil {
			if !errors.Is(err, ErrTenantNotFound) {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			tc = FallbackPlatform(host)
			c.Set(ContextKey, tc)
			c.Set(LayoutKey, tc.Layout)
			c.Set(UnresolvedKey, true)
			c.Next()
			return
		}

		c.Set(ContextKey, tc)
		c.Set(LayoutKey, tc.Layout)

		trackVisit(c, tc, tracker)

		c.Next()
	}
}

// trackVisit copies request fields and records the visit off the hot path.
// Tracking errors are logged and never abort or change the user request.
// Inputs: gin context, resolved tenant context, tracker (nil disables tracking).
// Output: none.
func trackVisit(c *gin.Context, tc *Context, tracker analytics.VisitTracker) {
	if tracker == nil || c == nil || tc == nil {
		return
	}
	if analytics.ShouldSkipPath(c.Request.URL.Path) {
		return
	}
	info := analytics.BuildVisitInfo(c, analytics.ClinicIDForVisit(tc.Layout, tc.TenantType, tc.ClinicID))
	if async, ok := tracker.(interface {
		TrackAsync(analytics.VisitInfo, func(error))
	}); ok {
		async.TrackAsync(info, func(err error) {
			log.Printf("analytics: failed to track visit: %v", err)
		})
		return
	}
	go func(info analytics.VisitInfo) {
		if err := tracker.Track(context.Background(), info); err != nil {
			log.Printf("analytics: failed to track visit: %v", err)
		}
	}(info)
}

// FromGin returns the resolved tenant Context from a gin request.
// Inputs: gin context.
// Output: tenant Context and true when present.
func FromGin(c *gin.Context) (*Context, bool) {
	v, ok := c.Get(ContextKey)
	if !ok {
		return nil, false
	}
	tc, ok := v.(*Context)
	return tc, ok
}

// LayoutFromGin returns which templ layout should be rendered.
// Inputs: gin context.
// Output: LayoutKind (platform/organ/private) and true when present.
func LayoutFromGin(c *gin.Context) (constants.LayoutKind, bool) {
	v, ok := c.Get(LayoutKey)
	if !ok {
		return "", false
	}
	layout, ok := v.(constants.LayoutKind)
	return layout, ok
}
