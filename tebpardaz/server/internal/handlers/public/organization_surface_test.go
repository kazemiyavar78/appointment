package public

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/layouts"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

func TestSSROrganizationHomepageGraph(t *testing.T) {
	body := renderOrganSurface(t, "mehrshafaclinics.ir", "/", organTenant(424242, ""), layouts.PageHead{
		Title:  "موسسه",
		Robots: "index, follow",
	})
	doc := mustJSONLD(t, body)
	org := nodeType(doc, "Organization")
	site := nodeType(doc, "WebSite")
	if org["@type"] != "Organization" || org["@id"] != "https://mehrshafaclinics.ir/#organization" {
		t.Fatalf("organization = %#v", org)
	}
	if site["@id"] != "https://mehrshafaclinics.ir/#website" {
		t.Fatalf("website = %#v", site)
	}
	publisher, _ := site["publisher"].(map[string]interface{})
	if publisher["@id"] != org["@id"] {
		t.Fatalf("publisher = %#v", site["publisher"])
	}
	for _, key := range []string{"logo", "telephone", "address", "legalName"} {
		if _, ok := org[key]; ok {
			t.Fatalf("%s leaked into organization", key)
		}
	}
	for _, banned := range []string{"05130001111", "MedicalOrganization", "MedicalClinic", "424242", "434343", "hidden-org-slug", "legalName"} {
		if strings.Contains(jsonLDScript(t, body), banned) {
			t.Fatalf("%s leaked into homepage graph", banned)
		}
	}
}

func TestSSROrganizationChildPageKeepsHostIdentity(t *testing.T) {
	body := renderOrganSurface(t, "mehrshafaclinics.ir", "/doctors", organTenant(424242, "/media/org.png"), layouts.PageHead{
		Title:        "پزشکان",
		Robots:       "index, follow",
		CanonicalURL: "https://mehrshafaclinics.ir/doctors",
	})
	if !strings.Contains(body, `rel="canonical" href="https://mehrshafaclinics.ir/doctors"`) {
		t.Fatal("canonical left the organization host")
	}
	script := jsonLDScript(t, body)
	if strings.Contains(script, "tebpardaz.ir") || strings.Contains(script, "/doctors#organization") {
		t.Fatalf("cross-domain or page-local id:\n%s", script)
	}
	doc := mustJSONLD(t, body)
	org := nodeType(doc, "Organization")
	if org["@id"] != "https://mehrshafaclinics.ir/#organization" || org["logo"] != "https://mehrshafaclinics.ir/media/org.png" {
		t.Fatalf("organization = %#v", org)
	}
	if _, ok := org["telephone"]; ok || org["url"] != "https://mehrshafaclinics.ir/" {
		t.Fatalf("organization url/telephone = %#v", org)
	}
}

func TestSSROrganizationClinicEntityAndBooking(t *testing.T) {
	slug := "چمران-مشهد"
	clinic := organClinic(515151, 424242, slug, "")
	clinic.Phone = "05130001111"
	clinic.Address = "خیابان چمران"
	clinic.Description = "توضیح مرکز"
	clinic.LogoURL = "/media/clinic.png"
	clinic.WSClientKey = "ws-key-secret-9"
	tc := organTenant(424242, "")
	c := schemaGin("mehrshafaclinics.ir", "/clinics/"+slug+"/sections")
	c.Set(tenant.ContextKey, tc)
	raw := organizationClinicJSONLD(c, tc, clinic)
	body := renderOrganSurface(t, "mehrshafaclinics.ir", "/clinics/"+slug+"/sections", tc, layouts.PageHead{
		Title:        clinic.Name,
		Robots:       "index, follow",
		CanonicalURL: "https://mehrshafaclinics.ir/clinics/" + slug + "/sections",
		JSONLD:       raw,
	})
	script := jsonLDScript(t, body)
	doc := mustJSONLD(t, body)
	org := nodeType(doc, "Organization")
	med := nodeType(doc, "MedicalClinic")
	if med == nil || org == nil || med["@id"] == org["@id"] {
		t.Fatalf("clinic and organization were collapsed:\n%s", script)
	}
	parent, _ := med["parentOrganization"].(map[string]interface{})
	if parent["@id"] != "https://mehrshafaclinics.ir/#organization" || med["@type"] != "MedicalClinic" {
		t.Fatalf("parent = %#v clinic = %#v", parent, med)
	}
	if strings.Contains(med["@id"].(string), "page=") || strings.Contains(script, "%25") {
		t.Fatalf("clinic id = %#v", med["@id"])
	}
	if med["telephone"] != "05130001111" || org["telephone"] != nil {
		t.Fatalf("telephone copied onto organization: org=%#v clinic=%#v", org["telephone"], med["telephone"])
	}
	if _, ok := org["address"]; ok {
		t.Fatal("clinic address copied onto organization")
	}

	other := organClinic(88, 99, slug, "")
	if organizationClinicJSONLD(c, tc, other) != "" {
		t.Fatal("clinic from another organization was graphed")
	}

	doctor := &models.Doctor{Name: "رضا احمدی", Slug: "reza", Mobile: "09125556677", NationalID: "0011223344", DoctorSystemID: 616161}
	doctor.Specialty.Name = "قلب"
	bookingCtx := schemaGin("mehrshafaclinics.ir", "/booking/"+slug+"/reza")
	in := bookingSchemaInput(bookingCtx, tc, clinic, doctor, clinic.Name, "نوبت", "", "https://mehrshafaclinics.ir/booking/"+slug+"/reza?page=2")
	if in.ClinicOrigin != "" || in.ClinicEntityID != med["@id"] || strings.Contains(in.ClinicEntityID, "page=") {
		t.Fatalf("booking clinic identity = %#v origin=%s", in.ClinicEntityID, in.ClinicOrigin)
	}
	graph := seo.BookingPageGraph(in)
	booked := renderOrganSurface(t, "mehrshafaclinics.ir", "/booking/"+slug+"/reza", tc, layouts.PageHead{
		Title:  "نوبت",
		Robots: "index, follow",
		JSONLD: graph,
	})
	bookedScript := jsonLDScript(t, booked)
	bookedDoc := mustJSONLD(t, booked)
	physician := nodeType(bookedDoc, "Physician")
	worksFor, _ := physician["worksFor"].(map[string]interface{})
	if worksFor["@type"] != "MedicalClinic" || worksFor["@id"] != in.ClinicEntityID || worksFor["@id"] == org["@id"] {
		t.Fatalf("worksFor = %#v", worksFor)
	}
	page := nodeType(bookedDoc, "WebPage")
	pagePublisher, _ := page["publisher"].(map[string]interface{})
	if pagePublisher["@id"] != "https://mehrshafaclinics.ir/#organization" {
		t.Fatalf("booking publisher = %#v", page["publisher"])
	}
	for _, banned := range []string{"0011223344", "616161", "09125556677", "ws-key-secret-9", "424242", "515151", "national_id", "DoctorSystemID"} {
		if strings.Contains(bookedScript, banned) {
			t.Fatalf("%s leaked", banned)
		}
	}
}

func TestSSROrganizationDoctorsWorksForClinic(t *testing.T) {
	c := schemaGin("mehrshafaclinics.ir", "/doctors?page=2")
	tc := organTenant(7, "")
	cards := []booking.DoctorCard{{
		Name:              "رضا احمدی",
		BookingURL:        "/booking/chamran/reza",
		ClinicName:        "چمران",
		ClinicLandingSlug: "چمران-مشهد",
		DoctorSystemID:    616161,
	}}
	raw := doctorsJSONLD(c, tc, cards, 2, true, 20)
	if strings.Contains(raw, "page=2#clinic") || strings.Contains(raw, "%25") || strings.Contains(raw, "616161") {
		t.Fatalf("doctors graph:\n%s", raw)
	}
	body := renderOrganSurface(t, "mehrshafaclinics.ir", "/doctors", tc, layouts.PageHead{
		Title:        "پزشکان",
		Robots:       "index, follow",
		CanonicalURL: "https://mehrshafaclinics.ir/doctors?page=2",
		JSONLD:       raw,
	})
	doc := mustJSONLD(t, body)
	list := nodeType(doc, "ItemList")
	item := list["itemListElement"].([]interface{})[0].(map[string]interface{})["item"].(map[string]interface{})
	worksFor, _ := item["worksFor"].(map[string]interface{})
	if worksFor["@type"] != "MedicalClinic" || !strings.HasSuffix(worksFor["@id"].(string), "#clinic") {
		t.Fatalf("worksFor = %#v", worksFor)
	}
	if worksFor["@id"] == "https://mehrshafaclinics.ir/#organization" || strings.Contains(worksFor["@id"].(string), "page=") {
		t.Fatalf("worksFor id = %#v", worksFor["@id"])
	}
	med := nodeType(doc, "MedicalClinic")
	parent, _ := med["parentOrganization"].(map[string]interface{})
	if parent["@id"] != "https://mehrshafaclinics.ir/#organization" {
		t.Fatalf("doctors clinic parent = %#v", med["parentOrganization"])
	}

	nameOnly := doctorsJSONLD(c, tc, []booking.DoctorCard{{
		Name: "بدون اسلاگ", BookingURL: "/booking/c1/dr", ClinicName: "مرکز بدون مسیر",
	}}, 1, true, 1)
	if strings.Contains(nameOnly, "#clinic") || strings.Contains(nameOnly, "/clinics/c") {
		t.Fatalf("fabricated clinic url:\n%s", nameOnly)
	}
}

func TestOrganizationBookingDoesNotChangePlatformOrOwnDomain(t *testing.T) {
	slug := "center"
	clinic := organClinic(5, 3, slug, "")
	doctor := &models.Doctor{Name: "پزشک", Slug: "dr"}
	platform := schemaGin("tebpardaz.ir", "/booking/center/dr")
	platformIn := bookingSchemaInput(platform, &tenant.Context{Layout: constants.LayoutPlatform, Host: "tebpardaz.ir"}, clinic, doctor, clinic.Name, "نوبت", "", "https://tebpardaz.ir/booking/center/dr")
	if platformIn.ClinicOrigin != "" || platformIn.ParentOrganizationID != "" || platformIn.ClinicEntityID != "https://tebpardaz.ir/clinics/center#clinic" {
		t.Fatalf("platform booking changed: %#v", platformIn)
	}

	domain := "chamranclinic.ir"
	own := organClinic(9, 3, slug, domain)
	ownCtx := schemaGin(domain, "/booking/dr")
	ownTC := &tenant.Context{Layout: constants.LayoutPrivate, Host: domain, Clinic: own, ClinicID: &own.ID}
	ownIn := bookingSchemaInput(ownCtx, ownTC, own, doctor, own.Name, "نوبت", "", "https://chamranclinic.ir/booking/dr")
	if ownIn.ClinicEntityID != "" || ownIn.ClinicOrigin != "https://chamranclinic.ir" {
		t.Fatalf("own-domain booking changed: origin=%s id=%s", ownIn.ClinicOrigin, ownIn.ClinicEntityID)
	}
	home := seo.TenantHomeGraph(seo.TenantHomeInput{Origin: "https://chamranclinic.ir", PageURL: "https://chamranclinic.ir/", Name: "چمران", Phone: "05131112233"})
	ownBody := renderSurface(t, domain, "/", ownTC, layouts.PageHead{Title: "چمران", Robots: "index, follow", JSONLD: home})
	if strings.Contains(ownBody, "parentOrganization") || strings.Contains(ownBody, "#organization") || !strings.Contains(ownBody, "https://chamranclinic.ir/#clinic") {
		t.Fatalf("own-domain graph changed:\n%s", jsonLDScript(t, ownBody))
	}

	detail := seo.ClinicDetailGraph(
		"https://tebpardaz.ir/",
		"https://tebpardaz.ir/clinics",
		"https://tebpardaz.ir/clinics/center",
		"https://tebpardaz.ir/clinics/center?page=2",
		"مرکز", "",
		seo.MedicalClinicDTO{Name: "مرکز"},
		nil, nil, false, 0,
	)
	platformBody := renderSurface(t, "tebpardaz.ir", "/clinics/center", &tenant.Context{Layout: constants.LayoutPlatform, Host: "tebpardaz.ir"}, layouts.PageHead{
		Title:        "مرکز",
		Robots:       "index, follow",
		CanonicalURL: "https://tebpardaz.ir/clinics/center?page=2",
		JSONLD:       detail,
	})
	if !strings.Contains(platformBody, `rel="canonical" href="https://tebpardaz.ir/clinics/center?page=2"`) {
		t.Fatal("platform canonical changed")
	}
	platformScript := jsonLDScript(t, platformBody)
	if strings.Contains(platformScript, "parentOrganization") || strings.Contains(platformScript, "page=2#clinic") || strings.Contains(platformScript, "mehrshafaclinics.ir") {
		t.Fatalf("platform clinic graph changed:\n%s", platformScript)
	}
}

func TestOrganizationDomainDoesNotGraphOnPlatformHost(t *testing.T) {
	tc := organTenant(7, "")
	tc.Host = "tebpardaz.ir"
	body := renderSurface(t, "tebpardaz.ir", "/hidden-org", tc, layouts.PageHead{Title: "سازمان", Robots: "index, follow"})
	if strings.Contains(body, "application/ld+json") || strings.Contains(body, "#organization") {
		t.Fatalf("legacy platform host gained an organization graph:\n%s", body)
	}
}

func TestSpecialtiesStayPlatformOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/specialties", nil)
	c.Request.Host = "mehrshafaclinics.ir"
	c.Set(tenant.ContextKey, organTenant(7, ""))
	(&SpecialtyHandler{}).Index(c)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "noindex, nofollow") {
		t.Fatalf("organ specialties status=%d", w.Code)
	}
	if strings.Contains(w.Body.String(), "#organization") || strings.Contains(w.Body.String(), "/specialties/") {
		t.Fatalf("organ specialties gained a landing:\n%s", w.Body.String())
	}
}

func organTenant(id uint, logo string) *tenant.Context {
	domain := "mehrshafaclinics.ir"
	org := &models.Organization{
		Name:        "موسسه خدمات درمانی بسیجیان",
		Description: "",
		LogoURL:     logo,
		Status:      "paused",
		Domain:      &domain,
		Slug:        strPtr("hidden-org-slug"),
		Code:        434343,
	}
	org.ID = id
	orgID := id
	return &tenant.Context{Layout: constants.LayoutOrgan, Host: domain, Organization: org, OrganizationID: &orgID}
}

func organClinic(id, orgID uint, slug, domain string) *models.Clinic {
	clinic := &models.Clinic{
		Name:              "درمانگاه چمران",
		OrganizationID:    orgID,
		IsActiveOnWebsite: true,
		Slug:              strPtr(slug),
	}
	clinic.ID = id
	if domain != "" {
		clinic.Domain = strPtr(domain)
	}
	return clinic
}

func renderOrganSurface(t *testing.T, host, path string, tc *tenant.Context, head layouts.PageHead) string {
	t.Helper()
	return renderSurface(t, host, path, tc, head)
}

func renderSurface(t *testing.T, host, path string, tc *tenant.Context, head layouts.PageHead) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	c.Request.Host = host
	RenderPublicLayoutWithHead(c, tc, pages.NotFound(pages.NotFoundView{HomeURL: "/"}), "home", head)
	return w.Body.String()
}

func jsonLDScript(t *testing.T, body string) string {
	t.Helper()
	const open = `<script type="application/ld+json">`
	start := strings.Index(body, open)
	if start < 0 {
		t.Fatalf("script missing:\n%s", body)
	}
	start += len(open)
	end := strings.Index(body[start:], "</script>")
	if end < 0 {
		t.Fatalf("script not closed")
	}
	return body[start : start+end]
}
