package public

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/layouts"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

func TestPrivateLayoutUsesClinicFavicon(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := renderLayout(t, &tenant.Context{
		Layout: constants.LayoutPrivate,
		Clinic: &models.Clinic{
			Name:       "چمران",
			FaviconURL: "/static/uploads/clinics/favicons/chamran.ico",
		},
	}, layouts.PageHead{Title: "چمران"})
	if !strings.Contains(body, `href="/static/uploads/clinics/favicons/chamran.ico"`) {
		t.Fatalf("clinic favicon missing from head: %s", body)
	}
}

func TestPrivateLayoutFallsBackWhenFaviconEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := renderLayout(t, &tenant.Context{
		Layout: constants.LayoutPrivate,
		Clinic: &models.Clinic{Name: "چمران", LogoURL: "/static/uploads/clinics/logo.png"},
	}, layouts.PageHead{Title: "چمران"})
	if !strings.Contains(body, `href="/static/uploads/clinics/logo.png"`) {
		t.Fatalf("logo favicon fallback missing: %s", body)
	}

	plain := renderLayout(t, &tenant.Context{
		Layout: constants.LayoutPrivate,
		Clinic: &models.Clinic{Name: "چمران"},
	}, layouts.PageHead{Title: "چمران"})
	if !strings.Contains(plain, `href="/static/clinics/logo.jpg"`) {
		t.Fatalf("platform favicon fallback missing: %s", plain)
	}
}

func TestPlatformLayoutKeepsDefaultFavicon(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := renderLayout(t, &tenant.Context{Layout: constants.LayoutPlatform}, layouts.PageHead{})
	if !strings.Contains(body, `href="/static/clinics/logo.jpg"`) {
		t.Fatalf("platform favicon missing: %s", body)
	}
	if strings.Contains(body, "chamran.ico") {
		t.Fatalf("platform layout picked up a clinic favicon: %s", body)
	}
}

func renderLayout(t *testing.T, tc *tenant.Context, head layouts.PageHead) string {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	RenderPublicLayoutWithHead(c, tc, pages.NotFound(pages.NotFoundView{HomeURL: "/"}), "home", head)
	if w.Code != 0 && w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
	return w.Body.String()
}
