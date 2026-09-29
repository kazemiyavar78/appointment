package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"tebpardaz/server/internal/seo"
	"tebpardaz/server/views/components"
)

func TestHomeSpecialtyLinks(t *testing.T) {
	path := seo.SpecialtyPath("داخلی")
	var platform bytes.Buffer
	err := Home(HomeView{
		Specialties: []components.SpecialtyCardView{{
			ID:   2,
			Name: "داخلی",
			Href: path,
		}},
		ShowSpecialtiesLink: true,
		SpecialtiesListURL:  "/specialties",
		NewsListURL:         "/news",
	}).Render(context.Background(), &platform)
	if err != nil {
		t.Fatal(err)
	}
	html := platform.String()
	if !strings.Contains(html, `href="`+path+`"`) ||
		!strings.Contains(html, `href="/specialties"`) ||
		!strings.Contains(html, "مشاهده همه تخصص‌ها") ||
		strings.Contains(html, "%25") {
		t.Fatal(html)
	}

	var tenant bytes.Buffer
	err = Home(HomeView{
		Specialties: []components.SpecialtyCardView{{ID: 2, Name: "داخلی"}},
		NewsListURL: "/news",
	}).Render(context.Background(), &tenant)
	if err != nil {
		t.Fatal(err)
	}
	tenantHTML := tenant.String()
	if strings.Contains(tenantHTML, "/specialties") || !strings.Contains(tenantHTML, `href="/doctors?specialty_id=2"`) {
		t.Fatal(tenantHTML)
	}
}

func TestHomeClinicLinks(t *testing.T) {
	path := seo.ClinicPath("چمران-مشهد")
	var platform bytes.Buffer
	err := Home(HomeView{
		Clinics: []components.ClinicCardView{{
			ID:   4,
			Name: "چمران",
			Href: path,
		}},
		ShowClinicCards: true,
		ShowClinicsLink: true,
		ClinicsListURL:  "/clinics",
		NewsListURL:     "/news",
	}).Render(context.Background(), &platform)
	if err != nil {
		t.Fatal(err)
	}
	html := platform.String()
	if !strings.Contains(html, `href="`+path+`"`) || !strings.Contains(html, `href="/clinics"`) || !strings.Contains(html, "مشاهده همه مراکز درمانی") || strings.Contains(html, "%25") {
		t.Fatal(html)
	}

	var tenant bytes.Buffer
	err = Home(HomeView{
		Clinics:         []components.ClinicCardView{{ID: 4, Name: "چمران", Slug: "chamran"}},
		ShowClinicCards: true,
		NewsListURL:     "/news",
	}).Render(context.Background(), &tenant)
	if err != nil {
		t.Fatal(err)
	}
	tenantHTML := tenant.String()
	if strings.Contains(tenantHTML, `href="/clinics`) || !strings.Contains(tenantHTML, `href="/doctors?clinic=chamran"`) {
		t.Fatal(tenantHTML)
	}
}
