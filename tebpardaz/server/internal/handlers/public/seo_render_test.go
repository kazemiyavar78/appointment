package public

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/layouts"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

func TestSSRHeadIncludesSEOTags(t *testing.T) {
	gin.SetMode(gin.TestMode)
	meta := seo.HomeMeta(seo.SiteClinic, "درمانگاه چمران مشهد", "https://chamranclinic.ir/")
	body := renderPublicHead(t, &tenant.Context{
		Layout: constants.LayoutPrivate,
	}, headFromMeta(meta))

	checks := []string{
		"<title>" + meta.Title + "</title>",
		`<meta name="description" content="` + meta.Description + `">`,
		`<link rel="canonical" href="` + meta.Canonical + `">`,
		`<meta name="robots" content="index,follow">`,
		`<meta property="og:title" content="` + meta.Title + `">`,
		`<meta property="og:description" content="` + meta.Description + `">`,
		`<meta property="og:url" content="` + meta.Canonical + `">`,
	}
	for _, want := range checks {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s\n%s", want, body)
		}
	}
	if strings.Contains(body, "طب‌پرداز") {
		t.Fatalf("tenant SSR head used platform brand:\n%s", body)
	}
}

func TestSSRHeadNoindexFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	meta := seo.DoctorsMeta(seo.SitePlatform, "", "https://tebpardaz.ir", seo.DoctorListQuery{Q: "test"})
	body := renderPublicHead(t, &tenant.Context{Layout: constants.LayoutPlatform}, headFromMeta(meta))
	if !strings.Contains(body, `<meta name="robots" content="noindex,follow">`) {
		t.Fatalf("robots missing:\n%s", body)
	}
	if !strings.Contains(body, `<link rel="canonical" href="https://tebpardaz.ir/doctors">`) {
		t.Fatalf("canonical missing:\n%s", body)
	}
	if !strings.Contains(body, `<meta property="og:url" content="https://tebpardaz.ir/doctors">`) {
		t.Fatalf("og:url missing:\n%s", body)
	}
}

func renderPublicHead(t *testing.T, tc *tenant.Context, head layouts.PageHead) string {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	RenderPublicLayoutWithHead(c, tc, pages.NotFound(pages.NotFoundView{HomeURL: "/"}), "home", head)
	return w.Body.String()
}
