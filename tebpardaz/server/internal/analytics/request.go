package analytics

import (
	"net"
	"strings"
	"time"

	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// ShouldSkipPath reports paths that must not be recorded as user visits
// (assets, admin, health, websockets).
// Inputs: request URL path.
// Output: true when tracking should be skipped.
func ShouldSkipPath(path string) bool {
	p := strings.ToLower(strings.TrimSpace(path))
	if p == "" {
		return false
	}
	switch p {
	case "/healthz", "/favicon.ico", "/robots.txt", "/sitemap.xml":
		return true
	}
	if strings.HasPrefix(p, "/static/") || p == "/static" {
		return true
	}
	if strings.HasPrefix(p, "/admin") {
		return true
	}
	if strings.HasPrefix(p, "/ws/") || strings.Contains(p, "/ws/") {
		return true
	}
	return false
}

// ClinicIDForVisit returns a copied clinic id for storage, or nil when the visit
// is on the platform/organization layer without a specific clinic.
// Inputs: layout, tenantType (currently unused beyond documentation of the rule), clinicID from tenant.Context.
// Output: *uint copy, or nil (never a pointer to 0).
func ClinicIDForVisit(layout constants.LayoutKind, _ constants.TenantType, clinicID *uint) *uint {
	if clinicID == nil || *clinicID == 0 {
		return nil
	}
	if layout == constants.LayoutPlatform || layout == constants.LayoutOrgan {
		// Slug-resolved clinic pages still have a real ClinicID; keep it.
		return cloneClinicID(clinicID)
	}
	return cloneClinicID(clinicID)
}

// cloneClinicID copies id so the buffered visit does not share the tenant pointer.
func cloneClinicID(id *uint) *uint {
	if id == nil || *id == 0 {
		return nil
	}
	v := *id
	return &v
}

// BuildVisitInfo copies request fields needed for tracking. It must run on the
// request goroutine; the returned value is safe to pass into a background Track call.
// Inputs: gin context, clinicID already passed through ClinicIDForVisit.
// Output: VisitInfo with Path/FullURL taken from the request URL (no extra normalization).
func BuildVisitInfo(c *gin.Context, clinicID *uint) VisitInfo {
	info := VisitInfo{ClinicID: cloneClinicID(clinicID), VisitedAt: time.Now()}
	if c == nil || c.Request == nil {
		return info
	}
	path := c.Request.URL.Path
	rawQuery := c.Request.URL.RawQuery
	fullURL := path
	if rawQuery != "" {
		fullURL = path + "?" + rawQuery
	}
	info.IPAddress = ClientIP(c)
	info.Host = requestHost(c)
	info.Path = path
	info.FullURL = fullURL
	info.UserAgent = c.GetHeader("User-Agent")
	info.Referrer = c.GetHeader("Referer")
	if info.Referrer == "" {
		info.Referrer = c.GetHeader("Referrer")
	}
	return info
}

// ClientIP returns the real client IP using X-Forwarded-For, X-Real-IP, then gin ClientIP.
// Inputs: gin context.
// Output: trimmed IP string, possibly empty.
func ClientIP(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	if xff := strings.TrimSpace(c.GetHeader("X-Forwarded-For")); xff != "" {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if ip := normalizeIP(first); ip != "" {
			return ip
		}
	}
	if xri := strings.TrimSpace(c.GetHeader("X-Real-IP")); xri != "" {
		if ip := normalizeIP(xri); ip != "" {
			return ip
		}
	}
	return strings.TrimSpace(c.ClientIP())
}

// requestHost prefers X-Forwarded-Host over Request.Host.
func requestHost(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	if xf := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); xf != "" {
		return strings.TrimSpace(strings.Split(xf, ",")[0])
	}
	return c.Request.Host
}

// normalizeIP strips an optional port from an IP or host:port value.
func normalizeIP(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(s); err == nil {
		return host
	}
	s = strings.Trim(s, "[]")
	if ip := net.ParseIP(s); ip != nil {
		return ip.String()
	}
	return s
}
