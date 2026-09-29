package public

import (
	"net/http"
	"strings"

	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// LegacyPlatformClinicRedirect alias قدیمی /{clinicSlug} را روی دامنه پلتفرم به landing اصلی می‌فرستد.
// ورودی: دامنه پایه پلتفرم، مثل tebpardaz.ir. خروجی: handler.
// فقط مرکز public و مسیر تک‌بخشی redirect می‌شود. مقصد از دامنه پیکربندی‌شده ساخته می‌شود، نه از Host کلاینت.
// route واقعی به NoRoute نمی‌رسد. دامنه سازمان و دامنه اختصاصی از اینجا عبور داده می‌شوند.
func LegacyPlatformClinicRedirect(baseDomain string) gin.HandlerFunc {
	base := strings.ToLower(strings.TrimSpace(baseDomain))
	return func(c *gin.Context) {
		loc := legacyPlatformClinicLocation(c, base)
		if loc == "" {
			c.Next()
			return
		}
		c.Header("Location", loc)
		c.Status(http.StatusMovedPermanently)
		if c.Request == nil || c.Request.Method != http.MethodHead {
			_, _ = c.Writer.WriteString("Moved Permanently.\n")
		}
		c.Abort()
	}
}

// legacyPlatformClinicLocation مقصد ۳۰۱ را برمی‌گرداند.
// ورودی: درخواست و دامنه پایه. خروجی: URL مطلق HTTPS یا خالی اگر این درخواست alias نباشد.
func legacyPlatformClinicLocation(c *gin.Context, baseDomain string) string {
	if c == nil || c.Request == nil || c.Request.URL == nil || baseDomain == "" {
		return ""
	}
	if tenant.IsUnresolved(c) || !legacyAliasPath(c.Request.URL.Path) {
		return ""
	}
	tc, ok := tenant.FromGin(c)
	if !ok || tc == nil || tc.Layout != constants.LayoutPrivate || tc.Clinic == nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(tc.Host))
	if host != baseDomain && host != "www."+baseDomain {
		return ""
	}
	clinic := tc.Clinic
	if clinic.DeletedAt.Valid || !seo.ClinicIndexable(clinic.IsActiveOnWebsite, clinic.Slug) {
		return ""
	}
	path := seo.ClinicPath(*clinic.Slug)
	origin := seo.HTTPSOrigin(baseDomain)
	loc := seo.AbsoluteURL(origin, path)
	if loc == "" || !strings.HasPrefix(loc, "https://"+baseDomain+"/clinics/") {
		return ""
	}
	return loc
}

// legacyAliasPath می‌گوید مسیر فقط یک بخش دارد یا نه.
// ورودی: path. خروجی: true برای /{slug}. query و مسیرهای چندبخشی alias نیستند.
func legacyAliasPath(path string) bool {
	path = strings.Trim(path, "/")
	return path != "" && !strings.Contains(path, "/")
}
