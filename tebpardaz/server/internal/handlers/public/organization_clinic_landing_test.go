package public

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/components"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestOrganizationClinicLanding(t *testing.T) {
	slugA := "مشهد-عبدالمطلب"
	slugB := "clinic-b"
	clinicA := publicClinic(slugA, "درمانگاه عبدالمطلب")
	clinicA.ID = 515151
	clinicA.OrganizationID = 909090
	clinicA.Code = 434343
	ownDomain := "chamranclinic.ir"
	clinicA.Domain = &ownDomain
	clinicB := publicClinic(slugB, "مرکز بی")
	clinicB.ID = 62
	clinicB.OrganizationID = 808080
	inactive := publicClinic("hidden-clinic", "پنهان")
	inactive.IsActiveOnWebsite = false
	inactive.OrganizationID = 909090
	h := &ClinicHandler{
		Clinics: &memClinics{rows: []models.Clinic{clinicA, clinicB, inactive}},
		Doctors: &memDoctors{rows: []models.Doctor{{
			Model:          gorm.Model{ID: 8},
			Name:           "رضا احمدی",
			Slug:           "reza",
			ClinicID:       515151,
			NationalID:     "0011223344",
			Mobile:         "09120000000",
			DoctorSystemID: 424242,
			Specialty:      models.Specialty{Name: "داخلی", Slug: "داخلی"},
		}}},
	}
	orgA := organLandingTenant(909090, "org-a.example")
	orgB := organLandingTenant(808080, "org-b.example")
	engineA := clinicEngineFor(h, orgA)
	engineB := clinicEngineFor(h, orgB)
	pathA := "/clinics/" + url.PathEscape(slugA)
	pathB := "/clinics/" + url.PathEscape(slugB)

	own := doClinicOn(t, engineA, http.MethodGet, pathA, "org-a.example")
	body := own.Body.String()
	canon := seo.AbsoluteURL("https://org-a.example", seo.ClinicPath(slugA))
	script := jsonLDScript(t, body)
	if own.Code != http.StatusOK ||
		!strings.Contains(body, `rel="canonical" href="`+canon+`"`) ||
		strings.Contains(script, "tebpardaz.ir") ||
		strings.Contains(body, "%25") ||
		strings.Contains(body, `href="/clinics"`) {
		t.Fatalf("own landing status=%d canonical=%v scriptHost=%v directory=%v", own.Code, strings.Contains(body, canon), strings.Contains(script, "tebpardaz.ir"), strings.Contains(body, `href="/clinics"`))
	}
	doc := mustJSONLD(t, body)
	if countNodeType(doc, "Organization") != 1 || countNodeType(doc, "MedicalClinic") != 1 || countNodeType(doc, "WebSite") != 1 {
		t.Fatalf("duplicate graph:\n%s", jsonLDScript(t, body))
	}
	orgNode := nodeType(doc, "Organization")
	clinicNode := nodeType(doc, "MedicalClinic")
	pageNode := nodeType(doc, "WebPage")
	if orgNode["@id"] != "https://org-a.example/#organization" || clinicNode["@id"] == orgNode["@id"] {
		t.Fatalf("identity org=%#v clinic=%#v", orgNode["@id"], clinicNode["@id"])
	}
	if clinicNode["url"] != canon || clinicNode["@id"] != canon+"#clinic" {
		t.Fatalf("clinic identity %#v", clinicNode)
	}
	parent, _ := clinicNode["parentOrganization"].(map[string]interface{})
	mainEntity, _ := pageNode["mainEntity"].(map[string]interface{})
	publisher, _ := pageNode["publisher"].(map[string]interface{})
	if parent["@id"] != orgNode["@id"] || mainEntity["@id"] != clinicNode["@id"] || publisher["@id"] != orgNode["@id"] {
		t.Fatalf("relations parent=%#v main=%#v publisher=%#v", parent, mainEntity, publisher)
	}
	list := nodeType(doc, "ItemList")
	item := list["itemListElement"].([]interface{})[0].(map[string]interface{})["item"].(map[string]interface{})
	works, _ := item["worksFor"].(map[string]interface{})
	if works["@type"] != "MedicalClinic" || works["@id"] != clinicNode["@id"] || works["@id"] == orgNode["@id"] {
		t.Fatalf("worksFor %#v", works)
	}
	if !strings.Contains(body, `href="/booking/`+url.PathEscape(slugA)+`/reza"`) && !strings.Contains(body, `href="/booking/`+slugA+`/reza"`) {
		t.Fatal("booking link left the organization surface")
	}
	for _, banned := range []string{"0011223344", "09120000000", "424242", "515151", "909090", "434343", "national_id", "DoctorSystemID", "SECRET-WS-KEY"} {
		if strings.Contains(body, banned) {
			t.Fatalf("%s leaked", banned)
		}
	}

	head := doClinicOn(t, engineA, http.MethodHead, pathA, "org-a.example")
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("head status=%d body=%d", head.Code, head.Body.Len())
	}
	for _, path := range []string{pathB, "/clinics/" + url.PathEscape(*inactive.Slug), "/clinics/missing", "/clinics"} {
		res := doClinicOn(t, engineA, http.MethodGet, path, "org-a.example")
		if res.Code != http.StatusNotFound || !strings.Contains(res.Body.String(), "noindex, nofollow") {
			t.Fatalf("%s status=%d", path, res.Code)
		}
		resHead := doClinicOn(t, engineA, http.MethodHead, path, "org-a.example")
		if resHead.Code != http.StatusNotFound || resHead.Body.Len() != 0 {
			t.Fatalf("head %s status=%d body=%d", path, resHead.Code, resHead.Body.Len())
		}
	}
	foreign := doClinicOn(t, engineB, http.MethodGet, pathA, "org-b.example")
	if foreign.Code != http.StatusNotFound || !strings.Contains(foreign.Body.String(), "noindex, nofollow") {
		t.Fatalf("foreign status=%d", foreign.Code)
	}
	ownHost := clinicEngineFor(h, &tenant.Context{Layout: constants.LayoutPrivate, Clinic: &clinicA, ClinicID: &clinicA.ID})
	ownRes := doClinicOn(t, ownHost, http.MethodGet, pathA, "chamranclinic.ir")
	if ownRes.Code != http.StatusNotFound || strings.Contains(ownRes.Body.String(), `"parentOrganization"`) {
		t.Fatalf("own-domain status=%d", ownRes.Code)
	}

	platform := clinicEngine(h, constants.LayoutPlatform)
	platformRes := doClinic(t, platform, http.MethodGet, pathA)
	platformBody := platformRes.Body.String()
	if platformRes.Code != http.StatusOK ||
		!strings.Contains(platformBody, `rel="canonical" href="`+seo.AbsoluteURL("https://tebpardaz.ir", seo.ClinicPath(slugA))+`"`) ||
		strings.Contains(platformBody, "org-a.example") ||
		strings.Contains(platformBody, "parentOrganization") {
		t.Fatalf("platform regression status=%d", platformRes.Code)
	}
}

func TestOrganizationClinicLandingPageIdentity(t *testing.T) {
	slug := "مشهد-عبدالمطلب"
	clinic := publicClinic(slug, "درمانگاه عبدالمطلب")
	clinic.OrganizationID = 909090
	doctors := make([]models.Doctor, 0, 13)
	for i := 0; i < 13; i++ {
		doctors = append(doctors, models.Doctor{
			Model: gorm.Model{ID: uint(i + 1)}, Name: "پزشک", Slug: "dr", ClinicID: clinic.ID,
			NationalID: "0099887766", Mobile: "09121111111", DoctorSystemID: 616161,
		})
	}
	h := &ClinicHandler{Clinics: &memClinics{rows: []models.Clinic{clinic}}, Doctors: &memDoctors{rows: doctors}}
	engine := clinicEngineFor(h, organLandingTenant(909090, "org-a.example"))
	base := "/clinics/" + url.PathEscape(slug)
	page2 := doClinicOn(t, engine, http.MethodGet, base+"?page=2&foo=bar", "org-a.example")
	body := page2.Body.String()
	entity := seo.AbsoluteURL("https://org-a.example", seo.ClinicPath(slug))
	pageCanon := seo.AbsoluteURL("https://org-a.example", seo.ClinicPagePath(slug, 2))
	if page2.Code != http.StatusOK || strings.Contains(body, "foo=bar") || !strings.Contains(body, `href="`+pageCanon+`"`) || strings.Contains(body, "?page=2#clinic") || strings.Contains(body, "%25") {
		t.Fatalf("page2 status=%d", page2.Code)
	}
	assertClinicEntityIdentity(t, body, entity, pageCanon)
	doc := mustJSONLD(t, body)
	webPage := nodeType(doc, "WebPage")
	if webPage["@id"] != pageCanon+"#webpage" {
		t.Fatalf("webpage id %#v", webPage["@id"])
	}
	mainEntity, _ := webPage["mainEntity"].(map[string]interface{})
	if mainEntity["@id"] != entity+"#clinic" {
		t.Fatalf("mainEntity %#v", mainEntity)
	}
}

func TestOrganizationClinicWithoutDoctorsStaysIndexable(t *testing.T) {
	slug := "مشهد-عبدالمطلب"
	clinic := publicClinic(slug, "درمانگاه عبدالمطلب")
	clinic.OrganizationID = 909090
	h := &ClinicHandler{Clinics: &memClinics{rows: []models.Clinic{clinic}}, Doctors: &memDoctors{}}
	res := doClinicOn(t, clinicEngineFor(h, organLandingTenant(909090, "org-a.example")), http.MethodGet, "/clinics/"+url.PathEscape(slug), "org-a.example")
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, "index,follow") || !strings.Contains(body, "در حال حاضر پزشک فعالی برای نوبت‌دهی آنلاین در این مرکز موجود نیست.") || strings.Contains(body, `"@type":"Physician"`) {
		t.Fatalf("empty status=%d", res.Code)
	}
}

func TestOrganizationHomeClinicLandingLinks(t *testing.T) {
	slug := "مشهد-عبدالمطلب"
	if !homeClinicLandingsEnabled(organLandingTenant(1, "org-a.example")) || homeClinicLandingsEnabled(&tenant.Context{Layout: constants.LayoutPrivate}) {
		t.Fatal("home landing gate")
	}
	legacy := organLandingTenant(1, "org-a.example")
	legacy.Host = "tebpardaz.ir"
	if homeClinicLandingsEnabled(legacy) {
		t.Fatal("legacy organ host must not gain clinic landings")
	}
	view := []components.ClinicCardView{{ID: 4, Name: "عبدالمطلب"}}
	rows := []models.Clinic{{Model: gorm.Model{ID: 4}, Slug: &slug, IsActiveOnWebsite: true}}
	applyPlatformClinicHrefs(view, rows)
	if view[0].Href != seo.ClinicPath(slug) || strings.Contains(view[0].Href, "%25") {
		t.Fatalf("href %q", view[0].Href)
	}
}

func organLandingTenant(id uint, host string) *tenant.Context {
	org := &models.Organization{Name: "سازمان آزمایش", Domain: &host, Code: 434343, Status: "paused"}
	org.ID = id
	orgID := id
	return &tenant.Context{Layout: constants.LayoutOrgan, Host: host, Organization: org, OrganizationID: &orgID}
}

func clinicEngineFor(h *ClinicHandler, tc *tenant.Context) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := *tc
		ctx.Host = c.Request.Host
		c.Set(tenant.ContextKey, &ctx)
		c.Next()
	})
	pub := r.Group("/")
	GETAndHEAD(pub, "/clinics", h.Index)
	GETAndHEAD(pub, "/clinics/:clinic_slug", h.Detail)
	return r
}

func doClinicOn(t *testing.T, r http.Handler, method, path, host string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Host = host
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func countNodeType(doc map[string]interface{}, typ string) int {
	raw, _ := doc["@graph"].([]interface{})
	n := 0
	for _, item := range raw {
		node, _ := item.(map[string]interface{})
		if node["@type"] == typ {
			n++
		}
	}
	return n
}
