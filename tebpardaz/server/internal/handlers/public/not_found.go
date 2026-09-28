package public

import (
	"net/http"
	"strings"

	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/layouts"
	"tebpardaz/server/views/pages"

	"github.com/gin-gonic/gin"
)

// AbortIfTenantMissing stops the chain and renders 404 when the host/slug is unknown.
// Input: gin context with tenant middleware already applied.
// Output: none; aborts with the public 404 page when tenant.IsUnresolved is true.
func AbortIfTenantMissing(c *gin.Context) {
	if tenant.IsUnresolved(c) {
		NotFound(c)
		return
	}
	c.Next()
}

// NotFound renders the public 404 page (or a short admin/API response).
// Input: gin context, optionally with a resolved tenant for branded layout.
// Output: HTTP 404 with HTML page, JSON, or a plain admin message.
func NotFound(c *gin.Context) {
	if c == nil {
		return
	}
	path := ""
	if c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
	}
	if strings.HasPrefix(path, "/admin") {
		c.String(http.StatusNotFound, "صفحه ادمین پیدا نشد")
		c.Abort()
		return
	}
	if wantsJSONNotFound(c) {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "صفحه پیدا نشد"})
		c.Abort()
		return
	}

	tc, ok := tenant.FromGin(c)
	if !ok || tc == nil {
		host := ""
		if c.Request != nil {
			host = c.Request.Host
		}
		tc = tenant.FallbackPlatform(host)
	}

	c.Status(http.StatusNotFound)
	head := layouts.PageHead{
		Title:  "صفحه پیدا نشد | طب‌پرداز",
		Robots: "noindex, nofollow",
	}
	RenderPublicLayoutWithHead(c, tc, pages.NotFound(pages.NotFoundView{HomeURL: "/"}), "", head)
	c.Abort()
}

// wantsJSONNotFound reports API/WebSocket clients that should not receive HTML 404.
// Input: gin context. Output: true when Accept is JSON-only or the request is a WebSocket upgrade.
func wantsJSONNotFound(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	if strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
		return true
	}
	accept := strings.ToLower(c.GetHeader("Accept"))
	return strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html")
}
