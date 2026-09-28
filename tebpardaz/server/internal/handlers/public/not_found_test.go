package public

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tebpardaz/server/internal/tenant"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// TestNotFoundRendersHTMLPage checks the public 404 page is HTML with status 404.
func TestNotFoundRendersHTMLPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/this-page-does-not-exist", nil)
	c.Set(tenant.ContextKey, tenant.FallbackPlatform("localhost"))
	c.Set(tenant.LayoutKey, constants.LayoutPlatform)

	NotFound(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	body := w.Body.String()
	if !strings.Contains(body, "صفحه پیدا نشد") {
		t.Fatalf("body missing 404 title, got %q", body)
	}
	if !strings.Contains(body, "noindex") {
		t.Fatalf("body missing noindex robots, got %q", body)
	}
}

// TestNotFoundAdminPathKeepsPlainMessage keeps /admin unmatched routes as text.
func TestNotFoundAdminPathKeepsPlainMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/missing", nil)

	NotFound(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	if got := w.Body.String(); got != "صفحه ادمین پیدا نشد" {
		t.Fatalf("body = %q", got)
	}
}

// TestAbortIfTenantMissingBlocksResolvedRoute renders 404 instead of the matched handler.
func TestAbortIfTenantMissingBlocksResolvedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(tenant.ContextKey, tenant.FallbackPlatform("unknown.example"))
		c.Set(tenant.LayoutKey, constants.LayoutPlatform)
		c.Set(tenant.UnresolvedKey, true)
		c.Next()
	}, AbortIfTenantMissing)
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "home-should-not-render")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	if strings.Contains(w.Body.String(), "home-should-not-render") {
		t.Fatal("matched home handler ran for unresolved tenant")
	}
	if !strings.Contains(w.Body.String(), "صفحه پیدا نشد") {
		t.Fatalf("missing 404 page, body=%q", w.Body.String())
	}
}

// TestNoRouteUnknownPathRendersNotFound mirrors unmatched public URLs.
func TestNoRouteUnknownPathRendersNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	resolver := tenant.NewResolver("tebpardaz.ir", nil, nil)
	r.NoRoute(tenant.Middleware(resolver, nil), NotFound)

	req := httptest.NewRequest(http.MethodGet, "/this-page-does-not-exist", nil)
	req.Host = "tebpardaz.ir"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	if !strings.Contains(w.Body.String(), "صفحه پیدا نشد") {
		t.Fatalf("missing 404 page, body=%q", w.Body.String())
	}
}
