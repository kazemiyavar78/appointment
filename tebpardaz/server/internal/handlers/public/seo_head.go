package public

import (
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/layouts"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// publicSite نوع سایت و نام برند همان مستأجر را برمی‌گرداند.
// ورودی: کانتکست مستأجر. خروجی: SiteKind و نام مرکز یا سازمان؛ پلتفرم نام خالی دارد تا برند طب‌پرداز جدا بماند.
func publicSite(tc *tenant.Context) (seo.SiteKind, string) {
	if tc == nil {
		return seo.SitePlatform, ""
	}
	switch tc.Layout {
	case constants.LayoutPrivate:
		if tc.Clinic != nil {
			return seo.SiteClinic, tc.Clinic.Name
		}
		return seo.SiteClinic, ""
	case constants.LayoutOrgan:
		if tc.Organization != nil {
			return seo.SiteOrgan, tc.Organization.Name
		}
		return seo.SiteOrgan, ""
	default:
		return seo.SitePlatform, ""
	}
}

// headFromMeta فیلدهای سئو را به PageHead موجود منتقل می‌کند.
// ورودی: Meta. خروجی: PageHead. تصویر و JSON-LD اینجا ست نمی‌شوند تا انتخاب تصویر یا اسکیما عوض نشود.
func headFromMeta(meta seo.Meta) layouts.PageHead {
	return layouts.PageHead{
		Title:           meta.Title,
		MetaDescription: meta.Description,
		CanonicalURL:    meta.Canonical,
		Robots:          meta.Robots,
	}
}

// requestCanonical مسیر تمیز همین درخواست را به URL مطلق همان میزبان تبدیل می‌کند.
// ورودی: کانتکست Gin. خروجی: canonical بدون query. query ردیابی وارد آن نمی‌شود.
func requestCanonical(c *gin.Context) string {
	path := "/"
	if c != nil && c.Request != nil && c.Request.URL != nil && c.Request.URL.Path != "" {
		path = c.Request.URL.Path
	}
	return absolutePublicURL(c, path)
}
