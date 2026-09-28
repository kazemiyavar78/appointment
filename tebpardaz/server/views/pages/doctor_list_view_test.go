package pages

import (
	"context"
	"regexp"
	"strings"
	"testing"
)

// TestDoctorListFilterUsesClinicSlug renders the clinic select with slug values, not numeric ids.
func TestDoctorListFilterUsesClinicSlug(t *testing.T) {
	var buf strings.Builder
	view := DoctorListView{
		FormAction:       "/doctors",
		ShowClinicFilter: true,
		ClinicSlug:       "chamran",
		Clinics: []DoctorListFilterOption{
			{ID: 9, Name: "چمران", Slug: "chamran"},
		},
		EmptyMessage: "پزشکی با این فیلترها یافت نشد.",
	}
	if err := DoctorList(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if strings.Contains(html, "clinic_id") || strings.Contains(html, "name=\"clinic_id\"") {
		t.Fatalf("public clinic filter still exposes clinic_id: %s", html)
	}
	if !strings.Contains(html, `name="clinic"`) {
		t.Fatal("expected clinic select name")
	}
	if !strings.Contains(html, `value="chamran"`) {
		t.Fatal("expected clinic option to use slug")
	}
	if strings.Contains(html, `value="9"`) {
		t.Fatal("clinic option should not use numeric id")
	}
	if !strings.Contains(html, `id="clinic-d-0"`) || !strings.Contains(html, `checked`) {
		t.Fatal("expected the matching clinic radio to be checked")
	}
}

// TestDoctorListFilterExpandsHiddenSpecialtySelection opens the disclosure when the active specialty is not in the first five.
func TestDoctorListFilterExpandsHiddenSpecialtySelection(t *testing.T) {
	specialties := make([]DoctorListFilterOption, 6)
	for i := range specialties {
		specialties[i] = DoctorListFilterOption{ID: uint(i + 1), Name: "تخصص", Description: "توضیح"}
	}
	var buf strings.Builder
	view := DoctorListView{
		FormAction:  "/doctors",
		SpecialtyID: 6,
		Specialties: specialties,
	}
	if err := DoctorList(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	tag := regexp.MustCompile(`<details[^>]*id="specialty-more-d"[^>]*>`).FindString(html)
	if tag == "" || !strings.Contains(tag, "open") {
		t.Fatalf("desktop disclosure should be open, tag=%q", tag)
	}
	if strings.Contains(html, `id="clinic-more-d"`) || strings.Contains(html, `name="clinic"`) {
		t.Fatal("clinic filter should stay hidden when ShowClinicFilter is false")
	}
	if !strings.Contains(html, "توضیح") || !strings.Contains(html, `name="specialty_id"`) {
		t.Fatal("specialty description and field name should remain")
	}
}

// TestDoctorListFilterHidesShowAllWhenPreviewFits omits the disclosure control for five or fewer specialties.
func TestDoctorListFilterHidesShowAllWhenPreviewFits(t *testing.T) {
	var buf strings.Builder
	view := DoctorListView{
		FormAction: "/doctors",
		Specialties: []DoctorListFilterOption{
			{ID: 1, Name: "داخلی"},
		},
	}
	if err := DoctorList(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if strings.Contains(html, `id="specialty-more-d"`) || strings.Contains(html, "مشاهده همه تخصص‌ها") {
		t.Fatal("show-all control should be absent for a short specialty list")
	}
	if !strings.Contains(html, "همه تخصص‌ها") || !strings.Contains(html, "۱ تخصص") {
		t.Fatal("all-option and count should still render")
	}
}
