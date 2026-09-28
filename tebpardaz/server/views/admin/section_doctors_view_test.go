package admin

import (
	"context"
	"strings"
	"testing"

	"tebpardaz/shared/constants"
)

// TestSectionDoctorOptionLabel نام و تخصص را برای گزینه دراپ‌داون ترکیب می‌کند.
func TestSectionDoctorOptionLabel(t *testing.T) {
	if got := sectionDoctorOptionLabel(SectionDoctorOption{Name: "دکتر رضایی"}); got != "دکتر رضایی" {
		t.Fatalf("got %q", got)
	}
	if got := sectionDoctorOptionLabel(SectionDoctorOption{Name: "دکتر رضایی", SpecialtyName: "قلب"}); got != "دکتر رضایی — قلب" {
		t.Fatalf("got %q", got)
	}
}

// TestSectionDoctorsPageRendersAssignForm فرم افزودن و جدول پزشکان بخش را رندر می‌کند.
func TestSectionDoctorsPageRendersAssignForm(t *testing.T) {
	var buf strings.Builder
	view := SectionDoctorsView{
		Nav:          BuildAdminNavWithSections("section_9_doctors", string(constants.UserRoleClinicAdmin), []SectionNavSummary{{ID: 9, Title: "آزمایشگاه"}}),
		SectionID:    9,
		SectionTitle: "آزمایشگاه",
		ClinicID:     3,
		Available: []SectionDoctorOption{
			{ID: 12, Name: "دکتر رضایی", SpecialtyName: "قلب"},
		},
		Assigned: []SectionDoctorRow{
			{DoctorID: 4, Name: "دکتر نوری", SpecialtyName: "داخلی", SystemID: 555, IsApproved: true, IsActive: true},
		},
	}
	if err := SectionDoctors(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		"پزشکان بخش: آزمایشگاه",
		`name="doctor_id"`,
		"/admin/sections/9/doctors",
		"دکتر رضایی — قلب",
		"دکتر نوری",
		"/admin/sections/9/doctors/4/delete",
		"حذف از بخش",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered page missing %q", want)
		}
	}
}
