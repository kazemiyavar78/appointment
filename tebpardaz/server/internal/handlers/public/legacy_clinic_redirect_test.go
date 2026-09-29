package public

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestLegacyPlatformClinicRedirect(t *testing.T) {
	slug := "چمران-مشهد"
	other := "مشهد-عبدالمطلب"
	active := legacyClinic(4, slug, true)
	inactive := legacyClinic(5, "hidden-clinic", false)
	deleted := legacyClinic(6, "removed-clinic", true)
	deleted.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
	engine := legacyEngine(map[string]*tenant.Context{
		"/" + slug:            {Host: "tebpardaz.ir", Layout: constants.LayoutPrivate, Clinic: active, ClinicID: &active.ID},
		"/" + other:           {Host: "tebpardaz.ir", Layout: constants.LayoutPrivate, Clinic: legacyClinic(7, other, true)},
		"/hidden-clinic":      {Host: "tebpardaz.ir", Layout: constants.LayoutPrivate, Clinic: inactive, ClinicID: &inactive.ID},
		"/removed-clinic":     {Host: "tebpardaz.ir", Layout: constants.LayoutPrivate, Clinic: deleted, ClinicID: &deleted.ID},
		"/missing":            tenant.FallbackPlatform("tebpardaz.ir"),
		"/org-only":           {Host: "tebpardaz.ir", Layout: constants.LayoutOrgan},
		"/" + slug + "/extra": {Host: "tebpardaz.ir", Layout: constants.LayoutPrivate, Clinic: active, ClinicID: &active.ID},
	}, map[string]bool{"/missing": true})

	for _, raw := range []string{slug, other} {
		res := doLegacy(t, engine, http.MethodGet, "/"+url.PathEscape(raw)+"?foo=bar&page=2", "tebpardaz.ir")
		want := seo.AbsoluteURL("https://tebpardaz.ir", seo.ClinicPath(raw))
		if res.Code != http.StatusMovedPermanently || res.Header().Get("Location") != want || strings.Contains(res.Header().Get("Location"), "%25") || strings.Contains(res.Header().Get("Location"), "foo") || strings.Contains(res.Header().Get("Location"), "page=") {
			t.Fatalf("%s status=%d location=%q", raw, res.Code, res.Header().Get("Location"))
		}
		if strings.Contains(res.Body.String(), "application/ld+json") || strings.Contains(res.Header().Get("Location"), raw+"#clinic") {
			t.Fatal("redirect created an entity url")
		}
	}
	head := doLegacy(t, engine, http.MethodHead, "/"+url.PathEscape(slug), "evil.example")
	if head.Code != http.StatusMovedPermanently || head.Body.Len() != 0 || head.Header().Get("Location") != seo.AbsoluteURL("https://tebpardaz.ir", seo.ClinicPath(slug)) {
		t.Fatalf("head status=%d body=%d location=%q", head.Code, head.Body.Len(), head.Header().Get("Location"))
	}
	for _, path := range []string{"/missing", "/hidden-clinic", "/removed-clinic", "/org-only", "/" + url.PathEscape(slug) + "/extra"} {
		res := doLegacy(t, engine, http.MethodGet, path, "tebpardaz.ir")
		if res.Code != http.StatusNotFound || res.Header().Get("Location") != "" || !strings.Contains(res.Body.String(), "noindex, nofollow") {
			t.Fatalf("%s status=%d location=%q", path, res.Code, res.Header().Get("Location"))
		}
	}

	doctors := doLegacy(t, engine, http.MethodGet, "/doctors", "tebpardaz.ir")
	if doctors.Code != http.StatusOK || doctors.Body.String() != "doctors" || doctors.Header().Get("Location") != "" {
		t.Fatalf("reserved doctors status=%d body=%q", doctors.Code, doctors.Body.String())
	}
	for _, reserved := range []struct{ path, body string }{
		{"/clinics", "clinics"},
		{"/specialties", "specialties"},
		{"/specialties/داخلی", "specialty"},
		{"/booking/chamran/reza", "booking"},
		{"/news", "news"},
		{"/sitemap.xml", "sitemap"},
		{"/robots.txt", "robots"},
	} {
		res := doLegacy(t, engine, http.MethodGet, reserved.path, "tebpardaz.ir")
		if res.Code != http.StatusOK || res.Body.String() != reserved.body || res.Header().Get("Location") != "" {
			t.Fatalf("%s status=%d body=%q location=%q", reserved.path, res.Code, res.Body.String(), res.Header().Get("Location"))
		}
	}
	landing := doLegacy(t, engine, http.MethodGet, seo.ClinicPath(slug), "tebpardaz.ir")
	if landing.Code != http.StatusOK || landing.Body.String() != "landing" || landing.Header().Get("Location") != "" {
		t.Fatalf("canonical landing status=%d", landing.Code)
	}

	own := doLegacy(t, engine, http.MethodGet, "/"+url.PathEscape(slug), "chamranclinic.ir")
	if own.Code != http.StatusNotFound || own.Header().Get("Location") != "" {
		t.Fatalf("own-domain alias status=%d location=%q", own.Code, own.Header().Get("Location"))
	}
	www := doLegacy(t, engine, http.MethodGet, "/"+url.PathEscape(slug), "www.tebpardaz.ir")
	if www.Code != http.StatusMovedPermanently || www.Header().Get("Location") != seo.AbsoluteURL("https://tebpardaz.ir", seo.ClinicPath(slug)) {
		t.Fatalf("www location=%q", www.Header().Get("Location"))
	}
}

func legacyClinic(id uint, slug string, active bool) *models.Clinic {
	clinic := &models.Clinic{Name: slug, Slug: &slug, IsActiveOnWebsite: active}
	clinic.ID = id
	return clinic
}

func legacyEngine(paths map[string]*tenant.Context, unresolved map[string]bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		if unresolved[path] {
			c.Set(tenant.ContextKey, tenant.FallbackPlatform(c.Request.Host))
			c.Set(tenant.UnresolvedKey, true)
			c.Next()
			return
		}
		if tc := paths[path]; tc != nil {
			copied := *tc
			if copied.Host == "" {
				copied.Host = "tebpardaz.ir"
			}
			if c.Request.Host == "www.tebpardaz.ir" {
				copied.Host = "www.tebpardaz.ir"
			}
			if c.Request.Host == "chamranclinic.ir" {
				copied.Host = "chamranclinic.ir"
				copied.Layout = constants.LayoutPrivate
			}
			c.Set(tenant.ContextKey, &copied)
			c.Next()
			return
		}
		c.Set(tenant.ContextKey, &tenant.Context{Host: "tebpardaz.ir", Layout: constants.LayoutPlatform})
		c.Next()
	})
	r.GET("/doctors", func(c *gin.Context) { c.String(http.StatusOK, "doctors") })
	r.GET("/clinics", func(c *gin.Context) { c.String(http.StatusOK, "clinics") })
	r.GET("/clinics/:clinic_slug", func(c *gin.Context) { c.String(http.StatusOK, "landing") })
	r.GET("/specialties", func(c *gin.Context) { c.String(http.StatusOK, "specialties") })
	r.GET("/specialties/:slug", func(c *gin.Context) { c.String(http.StatusOK, "specialty") })
	r.GET("/booking/*path", func(c *gin.Context) { c.String(http.StatusOK, "booking") })
	r.GET("/news", func(c *gin.Context) { c.String(http.StatusOK, "news") })
	r.GET("/sitemap.xml", func(c *gin.Context) { c.String(http.StatusOK, "sitemap") })
	r.GET("/robots.txt", func(c *gin.Context) { c.String(http.StatusOK, "robots") })
	r.NoRoute(LegacyPlatformClinicRedirect("tebpardaz.ir"), NotFound)
	return r
}

func doLegacy(t *testing.T, r http.Handler, method, path, host string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Host = host
	req.Header.Set("X-Forwarded-Host", "evil.example")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
