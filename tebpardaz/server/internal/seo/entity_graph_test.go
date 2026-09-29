package seo

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func graphNodes(t *testing.T, raw string) []map[string]interface{} {
	t.Helper()
	var doc struct {
		Graph []map[string]interface{} `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, raw)
	}
	return doc.Graph
}

func nodeByType(nodes []map[string]interface{}, typ string) map[string]interface{} {
	for _, n := range nodes {
		if n["@type"] == typ {
			return n
		}
	}
	return nil
}

func TestPlatformHomeGraph(t *testing.T) {
	raw := PlatformHomeGraph("https://tebpardaz.ir", "https://tebpardaz.ir/")
	if strings.Contains(raw, "MedicalOrganization") || strings.Contains(raw, "chamran") {
		t.Fatalf("platform graph: %s", raw)
	}
	nodes := graphNodes(t, raw)
	org := nodeByType(nodes, "Organization")
	site := nodeByType(nodes, "WebSite")
	if org == nil || site == nil {
		t.Fatalf("missing nodes: %s", raw)
	}
	if org["@id"] != "https://tebpardaz.ir/#organization" || org["name"] != PlatformBrand {
		t.Fatalf("org = %#v", org)
	}
	if org["legalName"] != PlatformLegalName || org["telephone"] != PlatformPhone {
		t.Fatalf("org identity = %#v", org)
	}
	if org["url"] != "https://tebpardaz.ir/" || !strings.HasPrefix(org["logo"].(string), "https://") {
		t.Fatalf("org url/logo = %#v", org)
	}
	if !strings.Contains(org["description"].(string), "طب‌پرداز، سیستم مدیریت کلینیک") {
		t.Fatalf("description = %#v", org["description"])
	}
	pub := site["publisher"].(map[string]interface{})
	if site["@id"] != "https://tebpardaz.ir/#website" || pub["@id"] != org["@id"] {
		t.Fatalf("website = %#v", site)
	}
}

func TestTenantHomeGraph(t *testing.T) {
	raw := TenantHomeGraph(TenantHomeInput{
		Origin:      "https://chamranclinic.ir",
		PageURL:     "https://chamranclinic.ir/",
		Name:        "درمانگاه چمران",
		Description: "توضیح واقعی مرکز",
		Phone:       "0513000",
		LogoURL:     "https://chamranclinic.ir/static/clinics/logo.png",
		Street:      "بلوار چمران",
		City:        "مشهد",
		Province:    "خراسان رضوی",
	})
	nodes := graphNodes(t, raw)
	clinic := nodeByType(nodes, "MedicalClinic")
	site := nodeByType(nodes, "WebSite")
	if clinic["@id"] != "https://chamranclinic.ir/#clinic" || clinic["name"] != "درمانگاه چمران" {
		t.Fatalf("clinic = %#v", clinic)
	}
	if clinic["url"] != "https://chamranclinic.ir/" || clinic["telephone"] != "0513000" {
		t.Fatalf("clinic contact = %#v", clinic)
	}
	addr := clinic["address"].(map[string]interface{})
	if addr["streetAddress"] != "بلوار چمران" || addr["addressLocality"] != "مشهد" || addr["addressRegion"] != "خراسان رضوی" || addr["addressCountry"] != "IR" {
		t.Fatalf("address = %#v", addr)
	}
	if _, ok := addr["postalCode"]; ok {
		t.Fatal("postal code invented")
	}
	if clinic["logo"] != "https://chamranclinic.ir/static/clinics/logo.png" {
		t.Fatalf("logo = %#v", clinic["logo"])
	}
	for _, banned := range []string{"openingHours", "geo", "email", "sameAs", "postalCode"} {
		if strings.Contains(raw, banned) {
			t.Fatalf("banned %s in %s", banned, raw)
		}
	}
	pub := site["publisher"].(map[string]interface{})
	if pub["@id"] != "https://chamranclinic.ir/#clinic" || site["@id"] != "https://chamranclinic.ir/#website" {
		t.Fatalf("site = %#v", site)
	}
	if strings.Contains(raw, "طب‌پرداز") {
		t.Fatalf("tenant picked platform brand: %s", raw)
	}
}

func TestBookingPhysicianGraph(t *testing.T) {
	in := BookingPageInput{
		BaseURL:      "https://chamranclinic.ir",
		Canonical:    "https://chamranclinic.ir/booking/reza",
		Title:        "نوبت دکتر رضا | داخلی | درمانگاه چمران",
		Description:  "مشاهده نوبت‌های رضا، داخلی در درمانگاه چمران و رزرو نوبت آنلاین.",
		DoctorName:   "رضا احمدی",
		DoctorSlug:   "reza",
		Specialty:    "داخلی",
		PhotoURL:     "/static/uploads/reza.jpg",
		ClinicName:   "درمانگاه چمران",
		ClinicOrigin: "https://chamranclinic.ir",
	}
	raw := BookingPageGraph(in)
	for _, banned := range []string{"national_id", "nationalId", "DoctorSystemID", "identifier", "hoursAvailable", "openingHours", "sameAs"} {
		if strings.Contains(raw, banned) {
			t.Fatalf("banned %s in %s", banned, raw)
		}
	}
	nodes := graphNodes(t, raw)
	page := nodeByType(nodes, "WebPage")
	doc := nodeByType(nodes, "Physician")
	crumb := nodeByType(nodes, "BreadcrumbList")
	if page["url"] != in.Canonical || page["name"] != in.Title {
		t.Fatalf("page = %#v", page)
	}
	main := page["mainEntity"].(map[string]interface{})
	if doc["@id"] != "https://chamranclinic.ir/booking/reza#physician" || main["@id"] != doc["@id"] {
		t.Fatalf("ids page=%#v doc=%#v", page, doc)
	}
	if doc["name"] != "رضا احمدی" || doc["medicalSpecialty"] != "داخلی" || doc["url"] != in.Canonical {
		t.Fatalf("physician = %#v", doc)
	}
	if doc["image"] != "https://chamranclinic.ir/static/uploads/reza.jpg" {
		t.Fatalf("image = %#v", doc["image"])
	}
	works := doc["worksFor"].(map[string]interface{})
	if works["@id"] != "https://chamranclinic.ir/#clinic" || works["name"] != "درمانگاه چمران" {
		t.Fatalf("worksFor = %#v", works)
	}
	if _, ok := works["url"]; ok {
		t.Fatal("worksFor invented a clinic url")
	}
	items := crumb["itemListElement"].([]interface{})
	if len(items) != 3 {
		t.Fatalf("breadcrumb len %d", len(items))
	}
	if items[0].(map[string]interface{})["position"].(float64) != 1 {
		t.Fatal("breadcrumb position")
	}
	if items[2].(map[string]interface{})["item"] != in.Canonical {
		t.Fatalf("last crumb = %#v", items[2])
	}
}

func TestPhysicianOmitsClinicLogoAndKeepsPlatformCanonical(t *testing.T) {
	logo := BookingPageGraph(BookingPageInput{
		BaseURL:         "https://tebpardaz.ir",
		Canonical:       "https://tebpardaz.ir/booking/chamran/reza",
		Title:           "نوبت دکتر رضا",
		DoctorName:      "رضا احمدی",
		DoctorSlug:      "reza",
		PhotoURL:        "/static/clinics/1-logo.jpg",
		UseClinicLogo:   true,
		ClinicName:      "درمانگاه چمران",
		ClinicEntityID:  "https://tebpardaz.ir/clinics/chamran#clinic",
		ClinicEntityURL: "https://tebpardaz.ir/clinics/chamran",
	})
	nodes := graphNodes(t, logo)
	doc := nodeByType(nodes, "Physician")
	if _, ok := doc["image"]; ok {
		t.Fatalf("clinic logo used as physician image: %#v", doc)
	}
	page := nodeByType(nodes, "WebPage")
	if page["url"] != "https://tebpardaz.ir/booking/chamran/reza" {
		t.Fatalf("canonical changed: %#v", page["url"])
	}
	if doc["@id"] != "https://tebpardaz.ir/booking/chamran/reza#physician" || doc["url"] != "https://tebpardaz.ir/booking/chamran/reza" {
		t.Fatalf("id/url = %#v", doc)
	}
	works := doc["worksFor"].(map[string]interface{})
	if works["@id"] != "https://tebpardaz.ir/clinics/chamran#clinic" || works["name"] != "درمانگاه چمران" {
		t.Fatalf("worksFor = %#v", works)
	}
	if strings.Contains(logo, "chamranclinic.ir") || strings.Contains(logo, "sameAs") {
		t.Fatalf("platform graph leaked domain or sameAs:\n%s", logo)
	}

	local := BookingPageGraph(BookingPageInput{
		BaseURL:    "https://tebpardaz.ir",
		Canonical:  "https://tebpardaz.ir/booking/no-domain/ali",
		Title:      "نوبت دکتر علی",
		DoctorName: "علی",
		DoctorSlug: "ali",
		ClinicName: "مرکز بدون دامنه",
	})
	doc = nodeByType(graphNodes(t, local), "Physician")
	if doc["@id"] != "https://tebpardaz.ir/booking/no-domain/ali#physician" {
		t.Fatalf("fallback id = %#v", doc["@id"])
	}
	works = doc["worksFor"].(map[string]interface{})
	if _, ok := works["@id"]; ok || works["url"] != nil {
		t.Fatalf("fake clinic url: %#v", works)
	}
	if works["name"] != "مرکز بدون دامنه" {
		t.Fatalf("works = %#v", works)
	}
}

func TestDoctorsItemListPositionsAndFilters(t *testing.T) {
	if ListPosition(1, 12, 0) != 1 || ListPosition(2, 12, 0) != 13 || ListPosition(2, 12, 1) != 14 {
		t.Fatalf("positions %d %d", ListPosition(2, 12, 0), ListPosition(2, 12, 1))
	}
	page2 := DoctorsPageGraph("https://chamranclinic.ir/", "https://chamranclinic.ir/doctors", []ItemListElementDTO{
		{Position: ListPosition(2, 12, 0), Name: "رضا", URL: "https://chamranclinic.ir/booking/reza"},
	}, true, 0)
	nodes := graphNodes(t, page2)
	list := nodeByType(nodes, "ItemList")
	items := list["itemListElement"].([]interface{})
	if items[0].(map[string]interface{})["position"].(float64) != 13 {
		t.Fatalf("item = %#v", items[0])
	}
	if _, ok := list["numberOfItems"]; ok {
		t.Fatalf("page length must not be announced as numberOfItems: %#v", list["numberOfItems"])
	}
	withTotal := DoctorsPageGraph("https://chamranclinic.ir/", "https://chamranclinic.ir/doctors", []ItemListElementDTO{
		{Position: ListPosition(2, 12, 0), Name: "رضا", URL: "https://chamranclinic.ir/booking/reza"},
	}, true, 25)
	totalList := nodeByType(graphNodes(t, withTotal), "ItemList")
	if totalList["numberOfItems"].(float64) != 25 {
		t.Fatalf("numberOfItems = %#v", totalList["numberOfItems"])
	}
	if totalList["itemListElement"].([]interface{})[0].(map[string]interface{})["position"].(float64) != 13 {
		t.Fatal("total changed position")
	}
	crumb := nodeByType(nodes, "BreadcrumbList")
	if len(crumb["itemListElement"].([]interface{})) != 2 {
		t.Fatal("doctors breadcrumb")
	}
	filtered := DoctorsPageGraph("https://tebpardaz.ir/", "https://tebpardaz.ir/doctors", []ItemListElementDTO{
		{Position: 1, Name: "رضا", URL: "https://tebpardaz.ir/booking/x/reza"},
	}, false, 25)
	if strings.Contains(filtered, "ItemList") {
		t.Fatalf("filter produced ItemList: %s", filtered)
	}
}

func TestSectionSchemaRegressions(t *testing.T) {
	clinic := BuildMedicalClinicSchema(MedicalClinicDTO{Name: "چمران", URL: "https://chamranclinic.ir/"})
	if strings.Contains(clinic, "openingHours") || strings.Contains(clinic, "MedicalBusiness") {
		t.Fatal(clinic)
	}
	article := BuildArticleSchema(ArticleDTO{
		Headline:      "پیام بخش",
		URL:           "https://chamranclinic.ir/section/lab/پیام-به-مراجعین",
		PublisherName: "چمران",
	})
	if strings.Contains(article, "مسئول بخش") || strings.Contains(article, `"author"`) {
		t.Fatal(article)
	}
	eq := BuildEquipmentListSchema("https://chamranclinic.ir/section/lab/تجهیزات", []EquipmentItemDTO{
		{Name: "سی‌تی", Description: "دستگاه واقعی", Category: equipmentBadgePlaceholder},
	})
	if strings.Contains(eq, "MedicalBusiness") || strings.Contains(eq, equipmentBadgePlaceholder) {
		t.Fatal(eq)
	}
	if !strings.Contains(eq, "MedicalDevice") || !strings.Contains(eq, "سی‌تی") {
		t.Fatal(eq)
	}
}

func TestNewsBreadcrumbHasNoArticle(t *testing.T) {
	raw := NewsBreadcrumbGraph("https://chamranclinic.ir/", "https://chamranclinic.ir/news", "افتتاح بخش", "https://chamranclinic.ir/news/4")
	if strings.Contains(raw, "Article") || strings.Contains(raw, "author") {
		t.Fatal(raw)
	}
	nodes := graphNodes(t, raw)
	crumb := nodeByType(nodes, "BreadcrumbList")
	items := crumb["itemListElement"].([]interface{})
	if len(items) != 3 || items[2].(map[string]interface{})["name"] != "افتتاح بخش" {
		t.Fatalf("news crumb %#v", items)
	}
}

func TestAbsoluteSchemaURLRejectsPlainHTTP(t *testing.T) {
	if got := AbsoluteSchemaURL("https://chamranclinic.ir", "/a.jpg"); got != "https://chamranclinic.ir/a.jpg" {
		t.Fatal(got)
	}
	if got := AbsoluteSchemaURL("https://chamranclinic.ir", "http://cdn.example/a.jpg"); got != "" {
		t.Fatal(got)
	}
	if OfficialClinicOrigin(nil, true) != "" {
		t.Fatal("nil domain")
	}
	domain := "ChamranClinic.ir"
	if OfficialClinicOrigin(&domain, false) != "" {
		t.Fatal("inactive domain")
	}
	if OfficialClinicOrigin(&domain, true) != "https://chamranclinic.ir" {
		t.Fatal(OfficialClinicOrigin(&domain, true))
	}
}

func TestClinicDetailGraphIdentityIgnoresPageQuery(t *testing.T) {
	entity := "https://tebpardaz.ir/clinics/" + url.PathEscape("چمران-مشهد")
	page2 := entity + "?page=2"
	raw := ClinicDetailGraph(
		"https://tebpardaz.ir/",
		"https://tebpardaz.ir/clinics",
		entity,
		page2,
		"درمانگاه چمران | پزشکان و نوبت‌دهی آنلاین - صفحه 2",
		"توضیح",
		MedicalClinicDTO{Name: "درمانگاه چمران", URL: page2},
		[]PhysicianDTO{{
			Name:       "رضا",
			URL:        "https://tebpardaz.ir/booking/x/reza",
			ClinicID:   page2 + "#clinic",
			ClinicName: "درمانگاه چمران",
		}},
		[]int{13},
		true,
		13,
	)
	if strings.Contains(raw, "%25") || strings.Contains(raw, "?page=2#clinic") {
		t.Fatal(raw)
	}
	nodes := graphNodes(t, raw)
	page := nodeByType(nodes, "WebPage")
	clinic := nodeByType(nodes, "MedicalClinic")
	if page["@id"] != page2+"#webpage" || page["url"] != page2 {
		t.Fatalf("webpage = %#v", page)
	}
	main := page["mainEntity"].(map[string]interface{})
	if main["@id"] != entity+"#clinic" || clinic["@id"] != entity+"#clinic" || clinic["url"] != entity {
		t.Fatalf("clinic identity page=%#v clinic=%#v", page, clinic)
	}
	if strings.Contains(clinic["@id"].(string), "?page=") || strings.Contains(clinic["url"].(string), "?page=") {
		t.Fatalf("paged clinic %#v", clinic)
	}
	list := nodeByType(nodes, "ItemList")
	elements := list["itemListElement"].([]interface{})
	item := elements[0].(map[string]interface{})["item"].(map[string]interface{})
	works := item["worksFor"].(map[string]interface{})
	if works["@id"] != entity+"#clinic" || strings.Contains(works["@id"].(string), "?page=") {
		t.Fatalf("worksFor = %#v", works)
	}
	crumb := nodeByType(nodes, "BreadcrumbList")
	crumbs := crumb["itemListElement"].([]interface{})
	last := crumbs[len(crumbs)-1].(map[string]interface{})
	if last["item"] != entity {
		t.Fatalf("breadcrumb = %#v", last)
	}
}
