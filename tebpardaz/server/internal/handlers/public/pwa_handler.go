package public

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// PWAHandler فایل‌های نصب صفحه وضعیت نوبت را از ریشه سایت سرو می‌کند تا محدوده سرویس‌ورکر کل دامنه باشد.
type PWAHandler struct{}

// NewPWAHandler سازنده سروکننده manifest و service worker است.
// ورودی: ندارد. خروجی: اشاره‌گر هندلر.
func NewPWAHandler() *PWAHandler {
	return &PWAHandler{}
}

// ServiceWorker فایل static/sw.js را روی /sw.js برمی‌گرداند.
// ورودی: gin context. خروجی: جاوااسکریپت سرویس‌ورکر.
func (h *PWAHandler) ServiceWorker(c *gin.Context) {
	c.Header("Service-Worker-Allowed", "/")
	serveStaticRoot(c, "static/sw.js", "application/javascript; charset=utf-8")
}

// Manifest فایل static/manifest.webmanifest را روی /manifest.webmanifest برمی‌گرداند.
// ورودی: gin context. خروجی: مانیفست PWA.
func (h *PWAHandler) Manifest(c *gin.Context) {
	serveStaticRoot(c, "static/manifest.webmanifest", "application/manifest+json; charset=utf-8")
}

// serveStaticRoot یک فایل ثابت را با نوع مشخص و بدون کش طولانی می‌فرستد.
// ورودی: context، مسیر فایل نسبت به پوشه اجرا، و Content-Type. خروجی: ندارد.
func serveStaticRoot(c *gin.Context, path, contentType string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, contentType, raw)
}
