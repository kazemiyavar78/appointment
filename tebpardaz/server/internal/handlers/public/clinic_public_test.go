package public

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/components"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func clinicEngine(h *ClinicHandler, layout constants.LayoutKind) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(tenant.ContextKey, &tenant.Context{Layout: layout, Host: c.Request.Host})
		c.Next()
	})
	pub := r.Group("/")
	GETAndHEAD(pub, "/clinics", h.Index)
	GETAndHEAD(pub, "/clinics/:clinic_slug", h.Detail)
	return r
}

func TestClinicLandingSharesSectionWildcard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	pub := r.Group("/")
	GETAndHEAD(pub, "/clinics/:clinic_slug", func(c *gin.Context) {})
	GETAndHEAD(pub, "/clinics/:clinic_slug/sections", func(c *gin.Context) {})
	if r == nil {
		t.Fatal("router")
	}
}

func doClinic(t *testing.T, r http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Host = "tebpardaz.ir"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func publicClinic(slug, name string) models.Clinic {
	return models.Clinic{
		Model:             gorm.Model{ID: 4, UpdatedAt: time.Date(2026, 4, 2, 8, 0, 0, 0, time.UTC)},
		Name:              name,
		Slug:              &slug,
		IsActiveOnWebsite: true,
		Address:           "خیابان احمدآباد",
		Phone:             "051-3000000",
		Description:       "درمانگاه واقعی چمران",
		LogoURL:           "/static/uploads/clinics/chamran.png",
		OrganizationID:    2,
		WSClientKey:       "SECRET-WS-KEY",
		City:              models.City{Name: "مشهد", Province: "خراسان رضوی"},
	}
}

func TestClinicIndexAndDetailPlatform(t *testing.T) {
	slug := "چمران-مشهد"
	clinic := publicClinic(slug, "درمانگاه چمران")
	h := &ClinicHandler{
		Clinics: &memClinics{rows: []models.Clinic{clinic}},
		Doctors: &memDoctors{rows: []models.Doctor{{
			Model:          gorm.Model{ID: 8},
			Name:           "رضا احمدی",
			Slug:           "reza",
			SpecialtyID:    3,
			ClinicID:       4,
			NationalID:     "0011223344",
			Mobile:         "09120000000",
			DoctorSystemID: 424242,
			Specialty:      models.Specialty{Name: "داخلی", Slug: "داخلی"},
		}}},
	}
	platform := clinicEngine(h, constants.LayoutPlatform)
	index := doClinic(t, platform, http.MethodGet, "/clinics")
	indexBody := index.Body.String()
	path := seo.ClinicPath(slug)
	if index.Code != http.StatusOK ||
		!strings.Contains(indexBody, "مراکز درمانی و نوبت‌دهی آنلاین | طب‌پرداز") ||
		!strings.Contains(indexBody, `<h1 class="ui-page-title">مراکز درمانی</h1>`) ||
		!strings.Contains(indexBody, `rel="canonical" href="https://tebpardaz.ir/clinics"`) ||
		!strings.Contains(indexBody, `content="index,follow"`) ||
		!strings.Contains(indexBody, `property="og:title"`) ||
		!strings.Contains(indexBody, `href="`+path+`"`) ||
		strings.Contains(indexBody, "%25") ||
		strings.Contains(indexBody, "SECRET-WS-KEY") {
		t.Fatalf("index status=%d", index.Code)
	}
	head := doClinic(t, platform, http.MethodHead, "/clinics")
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("index head status=%d body=%d", head.Code, head.Body.Len())
	}

	detail := doClinic(t, platform, http.MethodGet, "/clinics/"+url.PathEscape(slug))
	body := detail.Body.String()
	canon := seo.AbsoluteURL("https://tebpardaz.ir", path)
	if detail.Code != http.StatusOK ||
		!strings.Contains(body, "درمانگاه چمران | پزشکان و نوبت‌دهی آنلاین") ||
		!strings.Contains(body, `<h1 class="ui-page-title">درمانگاه چمران</h1>`) ||
		!strings.Contains(body, `href="`+canon+`"`) ||
		!strings.Contains(body, `content="index,follow"`) ||
		!strings.Contains(body, `property="og:description"`) ||
		!strings.Contains(body, "درمانگاه واقعی چمران") ||
		!strings.Contains(body, "مشهد") ||
		!strings.Contains(body, `href="/booking/چمران-مشهد/reza"`) && !strings.Contains(body, `href="/booking/`+url.PathEscape(slug)+`/reza"`) ||
		!strings.Contains(body, canon+"#clinic") ||
		!strings.Contains(body, `"@type":"MedicalClinic"`) ||
		!strings.Contains(body, `"mainEntity"`) ||
		!strings.Contains(body, `"@type":"Physician"`) ||
		!strings.Contains(body, `"numberOfItems":1`) ||
		strings.Contains(body, "%25") ||
		strings.Contains(body, "0011223344") ||
		strings.Contains(body, "09120000000") ||
		strings.Contains(body, "424242") ||
		strings.Contains(body, "SECRET-WS-KEY") ||
		strings.Contains(body, "parentOrganization") {
		t.Fatalf("detail status=%d", detail.Code)
	}
	assertClinicEntityIdentity(t, body, canon, canon)
	detailHead := doClinic(t, platform, http.MethodHead, "/clinics/"+url.PathEscape(slug))
	if detailHead.Code != http.StatusOK || detailHead.Body.Len() != 0 {
		t.Fatalf("detail head status=%d body=%d", detailHead.Code, detailHead.Body.Len())
	}
}

func TestClinicMissingAndHiddenAreNotFound(t *testing.T) {
	slug := "چمران-مشهد"
	hidden := publicClinic(slug, "پنهان")
	hidden.IsActiveOnWebsite = false
	h := &ClinicHandler{Clinics: &memClinics{rows: []models.Clinic{hidden}}, Doctors: &memDoctors{}}
	platform := clinicEngine(h, constants.LayoutPlatform)
	for _, path := range []string{"/clinics/missing", "/clinics/" + url.PathEscape(slug)} {
		res := doClinic(t, platform, http.MethodGet, path)
		if res.Code != http.StatusNotFound || !strings.Contains(res.Body.String(), "noindex, nofollow") {
			t.Fatalf("%s status=%d", path, res.Code)
		}
	}
	head := doClinic(t, platform, http.MethodHead, "/clinics/missing")
	if head.Code != http.StatusNotFound || head.Body.Len() != 0 {
		t.Fatalf("head status=%d body=%d", head.Code, head.Body.Len())
	}
}

func TestClinicRoutesRejectedOffPlatform(t *testing.T) {
	slug := "چمران-مشهد"
	h := &ClinicHandler{Clinics: &memClinics{rows: []models.Clinic{publicClinic(slug, "چمران")}}}
	for _, layout := range []constants.LayoutKind{constants.LayoutPrivate, constants.LayoutOrgan} {
		r := clinicEngine(h, layout)
		for _, path := range []string{"/clinics", "/clinics/" + url.PathEscape(slug)} {
			res := doClinic(t, r, http.MethodGet, path)
			if res.Code != http.StatusNotFound || !strings.Contains(res.Body.String(), "noindex, nofollow") {
				t.Fatalf("%s %s status=%d", layout, path, res.Code)
			}
		}
	}
}

func TestClinicWithoutDoctorsStaysIndexable(t *testing.T) {
	slug := "چمران-مشهد"
	h := &ClinicHandler{
		Clinics: &memClinics{rows: []models.Clinic{publicClinic(slug, "درمانگاه چمران")}},
		Doctors: &memDoctors{},
	}
	platform := clinicEngine(h, constants.LayoutPlatform)
	detail := doClinic(t, platform, http.MethodGet, "/clinics/"+url.PathEscape(slug))
	body := detail.Body.String()
	if detail.Code != http.StatusOK ||
		!strings.Contains(body, "index,follow") ||
		!strings.Contains(body, "در حال حاضر پزشک فعالی برای نوبت‌دهی آنلاین در این مرکز موجود نیست.") ||
		strings.Contains(body, `"@type":"Physician"`) ||
		strings.Contains(body, `"@type":"ItemList"`) {
		t.Fatalf("empty status=%d", detail.Code)
	}
	index := doClinic(t, platform, http.MethodGet, "/clinics")
	if !strings.Contains(index.Body.String(), `href="`+seo.ClinicPath(slug)+`"`) {
		t.Fatal("empty clinic missing from index")
	}
}

func TestClinicPageBoundsAndUnknownQuery(t *testing.T) {
	slug := "چمران-مشهد"
	doctors := make([]models.Doctor, 0, 13)
	for i := 0; i < 13; i++ {
		doctors = append(doctors, models.Doctor{
			Model: gorm.Model{ID: uint(i + 1)}, Name: "پزشک", Slug: "dr", SpecialtyID: 1, ClinicID: 4,
			NationalID: "0099887766", Mobile: "09121111111", DoctorSystemID: 515151,
		})
	}
	h := &ClinicHandler{
		Clinics: &memClinics{rows: []models.Clinic{publicClinic(slug, "درمانگاه چمران")}},
		Doctors: &memDoctors{rows: doctors},
	}
	platform := clinicEngine(h, constants.LayoutPlatform)
	base := "/clinics/" + url.PathEscape(slug)
	canon := seo.AbsoluteURL("https://tebpardaz.ir", seo.ClinicPath(slug))
	page1 := doClinic(t, platform, http.MethodGet, base+"?page=1")
	if page1.Code != http.StatusOK || !strings.Contains(page1.Body.String(), `href="`+canon+`"`) || strings.Contains(page1.Body.String(), canon+"?page=1") {
		t.Fatalf("page1 status=%d", page1.Code)
	}
	page2 := doClinic(t, platform, http.MethodGet, base+"?page=2&foo=bar")
	pageCanon := seo.AbsoluteURL("https://tebpardaz.ir", seo.ClinicPagePath(slug, 2))
	body := page2.Body.String()
	if page2.Code != http.StatusOK || strings.Contains(body, "foo=bar") || !strings.Contains(body, `href="`+pageCanon+`"`) || !strings.Contains(body, `"position":13`) || strings.Contains(body, "0099887766") || strings.Contains(body, "515151") {
		t.Fatalf("page2 status=%d", page2.Code)
	}
	assertClinicEntityIdentity(t, body, canon, pageCanon)
	for _, raw := range []string{"0", "-1", "abc", "999"} {
		res := doClinic(t, platform, http.MethodGet, base+"?page="+raw)
		if res.Code != http.StatusNotFound {
			t.Fatalf("page %s status=%d", raw, res.Code)
		}
	}
}

// assertClinicEntityIdentity هویت MedicalClinic را از canonical صفحه‌بندی‌شده جدا می‌کند.
// ورودی: HTML، URL بدون page، و URL همین صفحه. خروجی: شکست تست اگر @id مرکز page داشته باشد.
func assertClinicEntityIdentity(t *testing.T, body, entityURL, pageURL string) {
	t.Helper()
	if strings.Contains(body, "%25") || strings.Contains(entityURL, "%25") {
		t.Fatal("double encoded slug")
	}
	const open = `<script type="application/ld+json">`
	i := strings.Index(body, open)
	if i < 0 {
		t.Fatal("missing json-ld")
	}
	rest := body[i+len(open):]
	j := strings.Index(rest, "</script>")
	if j < 0 {
		t.Fatal("unclosed json-ld")
	}
	var doc struct {
		Graph []map[string]interface{} `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(rest[:j]), &doc); err != nil {
		t.Fatal(err)
	}
	var page, clinic, list map[string]interface{}
	for _, node := range doc.Graph {
		switch node["@type"] {
		case "WebPage":
			page = node
		case "MedicalClinic":
			clinic = node
		case "ItemList":
			list = node
		}
	}
	if page["@id"] != pageURL+"#webpage" || page["url"] != pageURL {
		t.Fatalf("webpage = %#v", page)
	}
	main, _ := page["mainEntity"].(map[string]interface{})
	if main["@id"] != entityURL+"#clinic" || clinic["@id"] != entityURL+"#clinic" || clinic["url"] != entityURL {
		t.Fatalf("clinic identity webpage=%#v clinic=%#v", page, clinic)
	}
	if strings.Contains(clinic["@id"].(string), "?page=") || strings.Contains(clinic["url"].(string), "?page=") {
		t.Fatalf("paged clinic %#v", clinic)
	}
	if list == nil {
		return
	}
	elements, _ := list["itemListElement"].([]interface{})
	if len(elements) == 0 {
		t.Fatal("empty physician list")
	}
	item := elements[0].(map[string]interface{})["item"].(map[string]interface{})
	works, _ := item["worksFor"].(map[string]interface{})
	worksID, _ := works["@id"].(string)
	if worksID != entityURL+"#clinic" || strings.Contains(worksID, "?page=") {
		t.Fatalf("worksFor = %#v", works)
	}
}

func TestApplyPlatformClinicHrefs(t *testing.T) {
	slug := "چمران-مشهد"
	view := []components.ClinicCardView{{ID: 2, Name: "چمران"}, {ID: 3, Name: "بی‌اسلاگ"}}
	rows := []models.Clinic{
		{Model: gorm.Model{ID: 2}, Slug: &slug, IsActiveOnWebsite: true},
		{Model: gorm.Model{ID: 3}, IsActiveOnWebsite: true},
	}
	applyPlatformClinicHrefs(view, rows)
	if view[0].Href != seo.ClinicPath(slug) || strings.Contains(view[0].Href, "%25") || view[1].Href != "" {
		t.Fatalf("hrefs = %#v", view)
	}
}
