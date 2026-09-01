package public

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RobotsHandler مدیریت و پاسخ‌دهی فایل robots.txt را برعهده دارد.
type RobotsHandler struct{}

// NewRobotsHandler نمونه جدیدی از RobotsHandler می‌سازد.
// ورودی: ندارد.
// خروجی: اشاره‌گر به RobotsHandler.
func NewRobotsHandler() *RobotsHandler {
	return &RobotsHandler{}
}

// ServeRobots محتوای متنی robots.txt را متناسب با دامنه فعال به همراه لینک Sitemap.xml تولید می‌کند.
// ورودی: کانتکست Gin.
// خروجی: متن تکست با وضعیت ۲۰۰ و هدر text/plain.
func (h *RobotsHandler) ServeRobots(c *gin.Context) {
	scheme := "https"
	if c.Request.TLS == nil && !strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	robotsContent := fmt.Sprintf(`User-agent: *
Allow: /
Allow: /static/
Disallow: /admin/
Disallow: /otp/
Disallow: /patient/
Disallow: /ws/
Disallow: /*?*date=
Disallow: /*?*shift=
Disallow: /*?*q=

Sitemap: %s/sitemap.xml
`, baseURL)

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, robotsContent)
}
