package public

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/shared/constants"
)

// TestPhysicianWorksForFollowsSurface هویت worksFor را روی سه surface و landing مرکز چک می‌کند.
func TestPhysicianWorksForFollowsSurface(t *testing.T) {
	doctor := &models.Doctor{
		Name:           "رضا احمدی",
		Slug:           "reza",
		NationalID:     "0012345678",
		Mobile:         "09120000000",
		ExternalID:     "ext-9",
		DoctorSystemID: 445566,
		Specialty:      models.Specialty{Name: "داخلی"},
	}
	plain := organClinic(5, 3, "center", "")
	withDomain := organClinic(6, 3, "center", "chamranclinic.ir")
	platformTC := &tenant.Context{Layout: constants.LayoutPlatform, Host: "tebpardaz.ir"}

	t.Run("platform without domain", func(t *testing.T) {
		raw := platformBookingGraph(t, plain, doctor, "https://tebpardaz.ir/booking/center/reza")
		assertWorksFor(t, raw, "https://tebpardaz.ir/clinics/center#clinic")
		assertPhysicianIdentity(t, raw, "https://tebpardaz.ir/booking/center/reza")
		assertNoPrivate(t, raw)
	})

	t.Run("platform with own domain", func(t *testing.T) {
		raw := platformBookingGraph(t, withDomain, doctor, "https://tebpardaz.ir/booking/center/reza")
		assertWorksFor(t, raw, "https://tebpardaz.ir/clinics/center#clinic")
		if strings.Contains(raw, "chamranclinic.ir") {
			t.Fatalf("worksFor followed Clinic.Domain:\n%s", raw)
		}
		assertPhysicianIdentity(t, raw, "https://tebpardaz.ir/booking/center/reza")
	})

	t.Run("platform organization member", func(t *testing.T) {
		raw := platformBookingGraph(t, withDomain, doctor, "https://tebpardaz.ir/booking/center/reza")
		assertWorksFor(t, raw, "https://tebpardaz.ir/clinics/center#clinic")
		if strings.Contains(raw, "#organization") || strings.Contains(raw, "parentOrganization") {
			t.Fatalf("platform graph gained an organization:\n%s", raw)
		}
	})

	t.Run("organization surface", func(t *testing.T) {
		org := organTenant(4, "")
		member := organClinic(6, 4, "center", "chamranclinic.ir")
		c := schemaGin("mehrshafaclinics.ir", "/booking/center/reza")
		in := bookingSchemaInput(c, org, member, doctor, member.Name, "نوبت", "", "https://mehrshafaclinics.ir/booking/center/reza")
		raw := seo.BookingPageGraph(in)
		assertWorksFor(t, raw, "https://mehrshafaclinics.ir/clinics/center#clinic")
		if strings.Contains(raw, "chamranclinic.ir") || strings.Contains(raw, "tebpardaz.ir/clinics") {
			t.Fatalf("organization worksFor left its surface:\n%s", raw)
		}
		assertPhysicianIdentity(t, raw, "https://mehrshafaclinics.ir/booking/center/reza")
	})

	t.Run("own domain", func(t *testing.T) {
		domain := "chamranclinic.ir"
		own := organClinic(9, 3, "center", domain)
		tc := &tenant.Context{Layout: constants.LayoutPrivate, Host: domain, Clinic: own, ClinicID: &own.ID}
		c := schemaGin(domain, "/booking/reza")
		in := bookingSchemaInput(c, tc, own, doctor, own.Name, "نوبت", "", "https://chamranclinic.ir/booking/reza")
		raw := seo.BookingPageGraph(in)
		assertWorksFor(t, raw, "https://chamranclinic.ir/#clinic")
		if strings.Contains(raw, "/clinics/") {
			t.Fatalf("own-domain invented a clinic path:\n%s", raw)
		}
		assertPhysicianIdentity(t, raw, "https://chamranclinic.ir/booking/reza")
	})

	t.Run("platform clinic landing matches worksFor", func(t *testing.T) {
		slug := "چمران-مشهد"
		row := organClinic(7, 3, slug, "chamranclinic.ir")
		escaped := url.PathEscape(slug)
		entity := "https://tebpardaz.ir/clinics/" + escaped
		page2 := entity + "?page=2"
		c := schemaGin("tebpardaz.ir", "/clinics/"+escaped)
		meta := seo.Meta{Title: "مرکز", Canonical: page2, Robots: seo.RobotsIndexFollow}
		raw := clinicDetailJSONLD(c, platformTC, row, meta, []models.Doctor{*doctor}, 2, true, 1)
		if strings.Contains(raw, "%25") || strings.Contains(raw, "?page=2#clinic") || strings.Contains(raw, "chamranclinic.ir") {
			t.Fatalf("clinic landing identity:\n%s", raw)
		}
		clinicID := medicalClinicID(t, raw)
		worksID := worksForID(t, raw)
		if clinicID != entity+"#clinic" || worksID != clinicID {
			t.Fatalf("clinic=%s worksFor=%s", clinicID, worksID)
		}
		assertNoPrivate(t, raw)
	})

	t.Run("persian booking slug", func(t *testing.T) {
		slug := "چمران-مشهد"
		row := organClinic(8, 3, slug, "chamranclinic.ir")
		escaped := url.PathEscape(slug)
		canon := "https://tebpardaz.ir/booking/" + escaped + "/reza"
		raw := platformBookingGraph(t, row, doctor, canon)
		want := "https://tebpardaz.ir/clinics/" + escaped + "#clinic"
		assertWorksFor(t, raw, want)
		if strings.Contains(raw, "%25") || strings.Contains(raw, "chamranclinic.ir") {
			t.Fatalf("encoding:\n%s", raw)
		}
		assertPhysicianIdentity(t, raw, canon)
	})

	t.Run("platform doctor list and specialty", func(t *testing.T) {
		slug := "center"
		c := schemaGin("tebpardaz.ir", "/doctors")
		list := doctorsJSONLD(c, platformTC, []booking.DoctorCard{{
			Name: "رضا", BookingURL: "/booking/center/reza", ClinicName: "مرکز", ClinicLandingSlug: slug,
		}}, 1, true, 1)
		assertWorksFor(t, list, "https://tebpardaz.ir/clinics/center#clinic")
		spec := specialtyDetailJSONLD(c, "dakheli", "داخلی", seo.Meta{Canonical: "https://tebpardaz.ir/specialties/dakheli"}, []models.Doctor{{
			Name: "رضا", Slug: "reza", ClinicID: withDomain.ID, Specialty: models.Specialty{Name: "داخلی"},
		}}, []models.Clinic{*withDomain}, 1, true, 1)
		assertWorksFor(t, spec, "https://tebpardaz.ir/clinics/center#clinic")
		if strings.Contains(spec, "chamranclinic.ir") {
			t.Fatalf("specialty worksFor followed domain:\n%s", spec)
		}
	})

	t.Run("own domain doctor list", func(t *testing.T) {
		domain := "chamranclinic.ir"
		own := organClinic(9, 3, "center", domain)
		tc := &tenant.Context{Layout: constants.LayoutPrivate, Host: domain, Clinic: own, ClinicID: &own.ID}
		c := schemaGin(domain, "/doctors")
		raw := doctorsJSONLD(c, tc, []booking.DoctorCard{{
			Name: "رضا", BookingURL: "/booking/reza", ClinicName: own.Name, ClinicLandingSlug: "center",
		}}, 1, true, 1)
		assertWorksFor(t, raw, "https://chamranclinic.ir/#clinic")
		if strings.Contains(raw, "/clinics/") {
			t.Fatalf("own-domain list used a clinic path:\n%s", raw)
		}
	})

	t.Run("missing slug is name only", func(t *testing.T) {
		nameless := organClinic(10, 3, "center", "chamranclinic.ir")
		nameless.Slug = nil
		c := schemaGin("tebpardaz.ir", "/booking/c10/reza")
		in := bookingSchemaInput(c, platformTC, nameless, doctor, nameless.Name, "نوبت", "", "https://tebpardaz.ir/booking/c10/reza")
		raw := seo.BookingPageGraph(in)
		works := topPhysician(t, raw)["worksFor"].(map[string]interface{})
		if _, ok := works["@id"]; ok || strings.Contains(raw, "chamranclinic.ir") || strings.Contains(raw, "/clinics/") {
			t.Fatalf("fabricated clinic identity: %#v\n%s", works, raw)
		}
	})
}

// platformBookingGraph گراف رزرو پلتفرم را برای یک مرکز می‌سازد.
// ورودی: مرکز، پزشک و canonical. خروجی: JSON-LD.
func platformBookingGraph(t *testing.T, clinic *models.Clinic, doctor *models.Doctor, canonical string) string {
	t.Helper()
	c := schemaGin("tebpardaz.ir", "/booking/center/reza")
	tc := &tenant.Context{Layout: constants.LayoutPlatform, Host: "tebpardaz.ir"}
	in := bookingSchemaInput(c, tc, clinic, doctor, clinic.Name, "نوبت", "", canonical)
	return seo.BookingPageGraph(in)
}

// assertWorksFor شناسه worksFor را با هویت مورد انتظار مقایسه می‌کند.
// ورودی: JSON-LD و @id. خروجی: ندارد.
func assertWorksFor(t *testing.T, raw, want string) {
	t.Helper()
	if got := worksForID(t, raw); got != want {
		t.Fatalf("worksFor = %s, want %s\n%s", got, want, raw)
	}
}

// assertPhysicianIdentity canonical پزشک و @id را با صفحهٔ booking مقایسه می‌کند.
// ورودی: JSON-LD و canonical. خروجی: ندارد.
func assertPhysicianIdentity(t *testing.T, raw, canonical string) {
	t.Helper()
	doc := topPhysician(t, raw)
	if doc["@id"] != canonical+"#physician" || doc["url"] != canonical {
		t.Fatalf("physician = %#v", doc)
	}
}

// assertNoPrivate فیلدهای خصوصی را در JSON-LD رد می‌کند.
// ورودی: JSON-LD. خروجی: ندارد.
func assertNoPrivate(t *testing.T, raw string) {
	t.Helper()
	for _, banned := range []string{"0012345678", "09120000000", "445566", "ext-9", "national_id", "DoctorSystemID", "external_id"} {
		if strings.Contains(raw, banned) {
			t.Fatalf("private %s leaked", banned)
		}
	}
}

// worksForID اولین @id از worksFor پزشک را برمی‌گرداند.
// ورودی: JSON-LD. خروجی: شناسه.
func worksForID(t *testing.T, raw string) string {
	t.Helper()
	var doc struct {
		Graph []map[string]interface{} `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	for _, node := range doc.Graph {
		switch node["@type"] {
		case "Physician":
			works, _ := node["worksFor"].(map[string]interface{})
			id, _ := works["@id"].(string)
			return id
		case "ItemList":
			elements, _ := node["itemListElement"].([]interface{})
			if len(elements) == 0 {
				continue
			}
			first, _ := elements[0].(map[string]interface{})
			item, _ := first["item"].(map[string]interface{})
			works, _ := item["worksFor"].(map[string]interface{})
			id, _ := works["@id"].(string)
			if id != "" {
				return id
			}
		}
	}
	t.Fatalf("worksFor missing:\n%s", raw)
	return ""
}

// medicalClinicID شناسهٔ گره MedicalClinic را برمی‌گرداند.
// ورودی: JSON-LD. خروجی: @id.
func medicalClinicID(t *testing.T, raw string) string {
	t.Helper()
	var doc struct {
		Graph []map[string]interface{} `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	for _, node := range doc.Graph {
		if node["@type"] == "MedicalClinic" {
			id, _ := node["@id"].(string)
			return id
		}
	}
	t.Fatalf("MedicalClinic missing:\n%s", raw)
	return ""
}

// topPhysician گره Physician بالای گراف رزرو را برمی‌گرداند.
// ورودی: JSON-LD. خروجی: شیء پزشک.
func topPhysician(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var doc struct {
		Graph []map[string]interface{} `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	for _, node := range doc.Graph {
		if node["@type"] == "Physician" {
			return node
		}
	}
	t.Fatalf("Physician missing:\n%s", raw)
	return nil
}
