package public

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/tenant"

	"github.com/gin-gonic/gin"
)

// TestPlatformSubdomainIsNotIndexable زیردامنه را ۴۰۴ بدون canonical و JSON-LD نگه می‌دارد.
func TestPlatformSubdomainIsNotIndexable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	slug := "foo"
	domain := "chamranclinic.ir"
	clinic := &models.Clinic{Name: "مرکز فو", Slug: &slug, Domain: &domain, IsActiveOnWebsite: true}
	clinic.ID = 9
	orgSlug := "mehr"
	orgDomain := "mehrshafaclinics.ir"
	org := &models.Organization{Name: "سازمان مهر", Slug: &orgSlug, Domain: &orgDomain}
	org.ID = 4
	tc := cache.NewTenantCache(nil)
	tc.SetClinicBySlug(slug, clinic)
	tc.SetOrgBySlug(orgSlug, org)
	resolver := tenant.NewResolver("tebpardaz.ir", nil, nil)
	resolver.UseCache(tc)

	engine := gin.New()
	engine.Use(tenant.Middleware(resolver, nil, nil))
	engine.Use(AbortIfTenantMissing)
	engine.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "platform-home") })
	engine.GET("/clinics/:clinic_slug", func(c *gin.Context) { c.String(http.StatusOK, "clinic-landing") })
	engine.GET("/doctors", func(c *gin.Context) { c.String(http.StatusOK, "doctors") })
	engine.NoRoute(LegacyPlatformClinicRedirect("tebpardaz.ir"), NotFound)

	for _, host := range []string{"foo.tebpardaz.ir", "mehr.tebpardaz.ir", "random.tebpardaz.ir"} {
		for _, path := range []string{"/", "/doctors", "/clinics/foo", "/foo"} {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Host = host
			req.Header.Set("X-Forwarded-Host", "tebpardaz.ir")
			engine.ServeHTTP(w, req)
			if w.Code != http.StatusNotFound {
				t.Fatalf("%s %s status = %d, want 404", host, path, w.Code)
			}
			if w.Header().Get("Location") != "" {
				t.Fatalf("%s %s location = %q", host, path, w.Header().Get("Location"))
			}
			body := w.Body.String()
			if strings.Contains(body, "platform-home") || strings.Contains(body, "clinic-landing") || strings.Contains(body, "application/ld+json") || strings.Contains(body, "rel=\"canonical\"") || strings.Contains(body, host) {
				t.Fatalf("%s %s leaked tenant content: %s", host, path, body)
			}
			if !strings.Contains(body, "noindex") {
				t.Fatalf("%s %s missing noindex", host, path)
			}
		}
	}

	home := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "tebpardaz.ir"
	req.Header.Set("X-Forwarded-Host", "foo.tebpardaz.ir")
	engine.ServeHTTP(home, req)
	if home.Code != http.StatusOK || home.Body.String() != "platform-home" {
		t.Fatalf("untrusted forwarded host changed platform home: %d %s", home.Code, home.Body.String())
	}

	landing := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/clinics/foo", nil)
	req.Host = "tebpardaz.ir"
	engine.ServeHTTP(landing, req)
	if landing.Code != http.StatusOK || landing.Body.String() != "clinic-landing" {
		t.Fatalf("platform landing = %d %s", landing.Code, landing.Body.String())
	}

	alias := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/foo?page=2", nil)
	req.Host = "tebpardaz.ir"
	req.Header.Set("X-Forwarded-Host", "evil.example")
	engine.ServeHTTP(alias, req)
	if alias.Code != http.StatusMovedPermanently {
		t.Fatalf("legacy status = %d", alias.Code)
	}
	if got := alias.Header().Get("Location"); got != "https://tebpardaz.ir/clinics/foo" {
		t.Fatalf("legacy location = %q", got)
	}

	doctors := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/doctors", nil)
	req.Host = "tebpardaz.ir"
	engine.ServeHTTP(doctors, req)
	if doctors.Code != http.StatusOK || doctors.Body.String() != "doctors" {
		t.Fatalf("doctors = %d %s", doctors.Code, doctors.Body.String())
	}
}
