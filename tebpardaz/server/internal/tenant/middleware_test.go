package tenant

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// TestFallbackPlatformSetsPlatformLayout builds a renderable 404 tenant.
func TestFallbackPlatformSetsPlatformLayout(t *testing.T) {
	tc := FallbackPlatform("LocalHost:8080")
	if tc == nil {
		t.Fatal("expected fallback context")
	}
	if tc.Layout != constants.LayoutPlatform {
		t.Fatalf("layout = %q, want %q", tc.Layout, constants.LayoutPlatform)
	}
	if tc.Host != "localhost" {
		t.Fatalf("host = %q, want localhost", tc.Host)
	}
}

// TestMiddlewareUnknownHostMarksUnresolved continues the chain instead of empty 404.
func TestMiddlewareUnknownHostMarksUnresolved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(NewResolver("tebpardaz.ir", nil, nil), nil))
	r.GET("/", func(c *gin.Context) {
		if !IsUnresolved(c) {
			t.Error("expected unresolved tenant")
		}
		tc, ok := FromGin(c)
		if !ok || tc == nil || tc.Layout != constants.LayoutPlatform {
			t.Error("expected platform fallback context")
		}
		c.String(http.StatusOK, "continued")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "unknown.example"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (empty 404 would be 404)", w.Code, http.StatusOK)
	}
	if got := w.Body.String(); got != "continued" {
		t.Fatalf("body = %q, want continued", got)
	}
}

// TestMiddlewarePlatformUnknownSlugMarksUnresolved treats missing path slugs as 404 tenants.
func TestMiddlewarePlatformUnknownSlugMarksUnresolved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(NewResolver("tebpardaz.ir", nil, nil), nil))
	r.NoRoute(func(c *gin.Context) {
		if !IsUnresolved(c) {
			t.Error("expected unresolved tenant for unknown slug")
		}
		c.String(http.StatusNotFound, "ready-for-page")
	})

	req := httptest.NewRequest(http.MethodGet, "/not-a-real-clinic", nil)
	req.Host = "tebpardaz.ir"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	if got := w.Body.String(); got != "ready-for-page" {
		t.Fatalf("body = %q; middleware aborted empty 404 before NoRoute", got)
	}
}

// TestMiddlewareReservedPathStaysResolved keeps known public routes on the platform host.
func TestMiddlewareReservedPathStaysResolved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(NewResolver("tebpardaz.ir", nil, nil), nil))
	r.GET("/doctors", func(c *gin.Context) {
		if IsUnresolved(c) {
			t.Error("reserved /doctors should resolve as platform")
		}
		c.String(http.StatusOK, "doctors")
	})

	req := httptest.NewRequest(http.MethodGet, "/doctors", nil)
	req.Host = "tebpardaz.ir"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Body.String(); got != "doctors" {
		t.Fatalf("body = %q", got)
	}
}

// TestMiddlewareSectionsPathStaysResolved keeps /sections off the tenant-slug lookup.
func TestMiddlewareSectionsPathStaysResolved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(NewResolver("tebpardaz.ir", nil, nil), nil))
	r.GET("/sections", func(c *gin.Context) {
		if IsUnresolved(c) {
			t.Error("reserved /sections should resolve as platform")
		}
		c.String(http.StatusOK, "sections")
	})

	req := httptest.NewRequest(http.MethodGet, "/sections", nil)
	req.Host = "tebpardaz.ir"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Body.String(); got != "sections" {
		t.Fatalf("body = %q", got)
	}
}
