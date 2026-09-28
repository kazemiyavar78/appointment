package public

import (
	"net/http"

	"tebpardaz/server/internal/seo"

	"github.com/gin-gonic/gin"
)

// publicProxyTrust همان فهرست TRUSTED_PROXIES سرور است.
// بدون آن X-Forwarded-Proto و X-Forwarded-Host نادیده گرفته می‌شوند.
var publicProxyTrust *seo.ProxyTrust

// SetPublicProxyTrust matcher پروکسی مورد اعتماد را برای URLهای عمومی ثبت می‌کند.
// ورودی: trust ساخته‌شده از تنظیم TRUSTED_PROXIES. nil یعنی هیچ peerی قابل اعتماد نیست.
// خروجی: ندارد.
func SetPublicProxyTrust(trust *seo.ProxyTrust) {
	publicProxyTrust = trust
}

// peerTrusted گزارش می‌دهد اتصال جاری از پروکسی پیکربندی‌شده آمده است یا نه.
// ورودی: درخواست HTTP. خروجی: true فقط وقتی RemoteAddr داخل ProxyTrust باشد.
func peerTrusted(r *http.Request) bool {
	if publicProxyTrust == nil || r == nil {
		return false
	}
	return publicProxyTrust.Trusts(r.RemoteAddr)
}

// publicBaseURL مبدأ scheme://host درخواست جاری را برمی‌گرداند.
// ورودی: کانتکست Gin. خروجی: base بدون اسلش انتهایی، یا خالی اگر درخواستی نباشد.
func publicBaseURL(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	return seo.PublicBaseURL(seo.RequestInfoFromHTTP(c.Request, peerTrusted(c.Request)))
}

// absolutePublicURL مسیر را با مبدأ همان درخواست به URL مطلق تبدیل می‌کند.
// ورودی: کانتکست Gin و path (خالی به معنی ریشه). خروجی: URL مطلق.
func absolutePublicURL(c *gin.Context, path string) string {
	return seo.AbsoluteURL(publicBaseURL(c), path)
}
