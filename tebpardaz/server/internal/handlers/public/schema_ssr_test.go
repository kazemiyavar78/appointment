package public

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/layouts"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

func TestSSRPlatformHomeJSONLD(t *testing.T) {
	c := schemaGin("tebpardaz.ir", "/")
	raw := homeJSONLD(c, &tenant.Context{Layout: constants.LayoutPlatform})
	body := renderJSONLD(t, &tenant.Context{Layout: constants.LayoutPlatform}, raw)
	doc := mustJSONLD(t, body)
	if !hasType(doc, "Organization") || !hasType(doc, "WebSite") {
		t.Fatalf("platform graph: %s", body)
	}
	if strings.Contains(body, "MedicalOrganization") || strings.Contains(body, "national_id") {
		t.Fatalf("unexpected platform fields:\n%s", body)
	}
}

func TestSSRTenantHomeJSONLD(t *testing.T) {
	domain := "chamranclinic.ir"
	tc := &tenant.Context{
		Layout: constants.LayoutPrivate,
		Clinic: &models.Clinic{
			Name:              "درمانگاه چمران",
			Phone:             "05130001111",
			Address:           "بلوار چمران",
			Description:       "توضیح واقعی مرکز",
			Domain:            &domain,
			IsActiveOnWebsite: true,
			LogoURL:           "/static/clinics/chamran.jpg",
			City:              models.City{Name: "مشهد", Province: "خراسان رضوی"},
		},
	}
	c := schemaGin("chamranclinic.ir", "/")
	raw := homeJSONLD(c, tc)
	body := renderJSONLD(t, tc, raw)
	doc := mustJSONLD(t, body)
	clinic := nodeType(doc, "MedicalClinic")
	if clinic["@id"] != "https://chamranclinic.ir/#clinic" || clinic["telephone"] != "05130001111" {
		t.Fatalf("clinic = %#v", clinic)
	}
	for _, banned := range []string{"openingHours", "postalCode", "geo", "sameAs", `"email"`} {
		if strings.Contains(body, banned) {
			t.Fatalf("banned %s in tenant home", banned)
		}
	}
}

func TestSSRBookingJSONLD(t *testing.T) {
	domain := "chamranclinic.ir"
	clinic := &models.Clinic{
		Name:              "درمانگاه چمران",
		Domain:            &domain,
		IsActiveOnWebsite: true,
	}
	doctor := &models.Doctor{
		Name:           "رضا احمدی",
		Slug:           "reza",
		Photo1200:      "/uploads/reza.jpg",
		Mobile:         "09120000000",
		NationalID:     "0012345678",
		DoctorSystemID: 445566,
		Specialty:      models.Specialty{Name: "داخلی"},
	}
	c := schemaGin("chamranclinic.ir", "/booking/reza")
	tc := &tenant.Context{Layout: constants.LayoutPrivate, Clinic: clinic}
	in := bookingSchemaInput(c, tc, clinic, doctor, clinic.Name, "نوبت دکتر رضا احمدی", "توضیح صفحه", "https://chamranclinic.ir/booking/reza")
	body := renderJSONLD(t, tc, seo.BookingPageGraph(in))
	doc := mustJSONLD(t, body)
	page := nodeType(doc, "WebPage")
	phys := nodeType(doc, "Physician")
	if page["url"] != "https://chamranclinic.ir/booking/reza" || page["name"] != "نوبت دکتر رضا احمدی" {
		t.Fatalf("page = %#v", page)
	}
	main := page["mainEntity"].(map[string]interface{})
	if phys["@id"] != "https://chamranclinic.ir/booking/reza#physician" || main["@id"] != phys["@id"] {
		t.Fatalf("physician link page=%#v phys=%#v", page, phys)
	}
	if phys["medicalSpecialty"] != "داخلی" {
		t.Fatalf("specialty = %#v", phys["medicalSpecialty"])
	}
	if phys["image"] != "https://chamranclinic.ir/uploads/reza.jpg" {
		t.Fatalf("image = %#v", phys["image"])
	}
	crumb := nodeType(doc, "BreadcrumbList")
	if len(crumb["itemListElement"].([]interface{})) != 3 {
		t.Fatal("booking breadcrumb")
	}
	for _, banned := range []string{"national_id", "nationalId", "0012345678", "DoctorSystemID", "445566", "09120000000", "identifier"} {
		if strings.Contains(body, banned) {
			t.Fatalf("private field %s leaked", banned)
		}
	}
}

func TestSSRDoctorsListJSONLD(t *testing.T) {
	c := schemaGin("tebpardaz.ir", "/doctors")
	cards := []booking.DoctorCard{{Name: "رضا احمدی", BookingURL: "/booking/chamran/reza"}}
	page1 := doctorsJSONLD(c, cards, 1, true, 0)
	body := renderJSONLD(t, &tenant.Context{Layout: constants.LayoutPlatform}, page1)
	doc := mustJSONLD(t, body)
	list := nodeType(doc, "ItemList")
	item := list["itemListElement"].([]interface{})[0].(map[string]interface{})
	if item["position"].(float64) != 1 {
		t.Fatalf("page1 position = %#v", item["position"])
	}
	if _, ok := list["numberOfItems"]; ok {
		t.Fatalf("unknown total must not emit numberOfItems: %#v", list["numberOfItems"])
	}
	if nodeType(doc, "BreadcrumbList")["itemListElement"].([]interface{}) == nil {
		t.Fatal("missing doctors breadcrumb")
	}
	if len(nodeType(doc, "BreadcrumbList")["itemListElement"].([]interface{})) != 2 {
		t.Fatal("doctors breadcrumb length")
	}

	page2 := doctorsJSONLD(c, cards, 2, true, 20)
	body2 := renderJSONLD(t, &tenant.Context{Layout: constants.LayoutPlatform}, page2)
	doc2 := mustJSONLD(t, body2)
	item2 := nodeType(doc2, "ItemList")["itemListElement"].([]interface{})[0].(map[string]interface{})
	if item2["position"].(float64) != float64(seo.ListPosition(2, doctorListPageSize, 0)) || doctorListPageSize != 12 {
		t.Fatalf("page2 position = %#v size=%d", item2["position"], doctorListPageSize)
	}
	if nodeType(doc2, "ItemList")["numberOfItems"].(float64) != 20 {
		t.Fatalf("numberOfItems = %#v", nodeType(doc2, "ItemList")["numberOfItems"])
	}

	filteredMeta := seo.DoctorsMeta(seo.SitePlatform, "", "https://tebpardaz.ir", seo.DoctorListQuery{Q: "قلب"})
	filtered := doctorsJSONLD(c, cards, 1, filteredMeta.Robots == seo.RobotsIndexFollow, len(cards))
	body3 := renderJSONLD(t, &tenant.Context{Layout: constants.LayoutPlatform}, filtered)
	if strings.Contains(body3, "ItemList") {
		t.Fatalf("filtered doctors emitted ItemList:\n%s", body3)
	}
	if !strings.Contains(body3, "BreadcrumbList") {
		t.Fatal("filtered page lost breadcrumb")
	}
}

func TestSSRSectionScheduleJSONLD(t *testing.T) {
	c := schemaGin("chamranclinic.ir", "/section/lab/ساعات-کاری")
	raw := seo.BuildGraph(sectionBreadcrumb(c, "/section/lab", "آزمایشگاه", "ساعات کاری", "/section/lab/ساعات-کاری"))
	body := renderJSONLD(t, &tenant.Context{Layout: constants.LayoutPrivate}, raw)
	doc := mustJSONLD(t, body)
	if nodeType(doc, "MedicalClinic") != nil || strings.Contains(body, "openingHours") || strings.Contains(body, "مسئول بخش") {
		t.Fatalf("section hours schema:\n%s", body)
	}
	items := nodeType(doc, "BreadcrumbList")["itemListElement"].([]interface{})
	if len(items) != 4 {
		t.Fatalf("section crumb len %d", len(items))
	}
	if items[1].(map[string]interface{})["item"] != "https://chamranclinic.ir/sections" {
		t.Fatalf("sections url = %#v", items[1])
	}

	platform := schemaGin("tebpardaz.ir", "/clinics/chamran/section/lab/ساعات-کاری")
	scoped := seo.BuildGraph(sectionBreadcrumb(platform, "/clinics/chamran/section/lab", "آزمایشگاه", "ساعات کاری", "/clinics/chamran/section/lab/ساعات-کاری"))
	if !strings.Contains(scoped, "https://tebpardaz.ir/clinics/chamran/sections") {
		t.Fatalf("clinic-scoped sections url missing: %s", scoped)
	}
	if sectionListPath("/section/lab") != "/sections" || sectionListPath("/clinics/chamran/section/lab") != "/clinics/chamran/sections" {
		t.Fatal(sectionListPath("/clinics/chamran/section/lab"))
	}
}

// schemaGin درخواست آزمایشی با Host عمومی می‌سازد.
// ورودی: host و path. خروجی: کانتکست Gin.
func schemaGin(host, path string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	c.Request.Host = host
	return c
}

// renderJSONLD HTML اولیهٔ layout را با همان JSON-LD رندر می‌کند.
// ورودی: مستأجر و رشتهٔ اسکیما. خروجی: بدنهٔ HTML.
func renderJSONLD(t *testing.T, tc *tenant.Context, raw string) string {
	t.Helper()
	if strings.TrimSpace(raw) == "" {
		t.Fatal("empty json-ld")
	}
	return renderPublicHead(t, tc, layouts.PageHead{Title: "تست", JSONLD: raw})
}

// mustJSONLD اسکریپت JSON-LD داخل HTML اولیه را parse می‌کند.
// ورودی: HTML. خروجی: سند JSON. در صورت نبودن یا نامعتبر بودن تست را متوقف می‌کند.
func mustJSONLD(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	const open = `<script type="application/ld+json">`
	start := strings.Index(body, open)
	if start < 0 {
		t.Fatalf("script missing:\n%s", body)
	}
	start += len(open)
	end := strings.Index(body[start:], "</script>")
	if end < 0 {
		t.Fatalf("script not closed:\n%s", body)
	}
	payload := body[start : start+end]
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &doc); err != nil {
		t.Fatalf("invalid json-ld: %v\n%s", err, payload)
	}
	if doc["@context"] != "https://schema.org" {
		t.Fatalf("context = %#v", doc["@context"])
	}
	return doc
}

// hasType وجود یک @type را در @graph گزارش می‌کند.
// ورودی: سند و نام نوع. خروجی: true اگر گره باشد.
func hasType(doc map[string]interface{}, typ string) bool {
	return nodeType(doc, typ) != nil
}

// nodeType اولین گره با @type خواسته‌شده را برمی‌گرداند.
// ورودی: سند و نام نوع. خروجی: گره یا nil.
func nodeType(doc map[string]interface{}, typ string) map[string]interface{} {
	raw, _ := doc["@graph"].([]interface{})
	for _, item := range raw {
		node, _ := item.(map[string]interface{})
		if node["@type"] == typ {
			return node
		}
	}
	return nil
}
