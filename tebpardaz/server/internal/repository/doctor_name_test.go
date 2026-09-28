package repository

import (
	"testing"

	"tebpardaz/server/internal/models"
	"tebpardaz/shared/protocol"
)

func TestNormalizeDoctorNameKeepsDigits(t *testing.T) {
	if got := normalizeDoctorName("علي"); got != "علی" {
		t.Fatalf("yeh = %q", got)
	}
	if got := normalizeDoctorName("كريم"); got != "کریم" {
		t.Fatalf("kaf = %q", got)
	}
	if got := normalizeDoctorName("كيارش"); got != "کیارش" {
		t.Fatalf("search yeh = %q", got)
	}
	if got := normalizeDoctorName("دکتر 123"); got != "دکتر 123" {
		t.Fatalf("digits = %q", got)
	}
	if normalizeDoctorName("کیارش") != normalizeDoctorName("كيارش") {
		t.Fatal("arabic query does not match persian name")
	}
	if got := normalizeDoctorName("رئیس"); got != "رئیس" {
		t.Fatalf("hamza flipped: %q", got)
	}
	if got := normalizeDoctorName("رییس"); got != "رییس" {
		t.Fatalf("legacy storage reversed: %q", got)
	}
	for _, stored := range []string{"رییس", "رئیس", "علی", "دکتر 123"} {
		once := normalizeDoctorName(stored)
		if normalizeDoctorName(once) != once {
			t.Fatalf("migration not idempotent for %q", stored)
		}
	}
}

func TestResolveApprovedDoctorNamesRejectsBlank(t *testing.T) {
	name, first, last, err := resolveApprovedDoctorNames("  علي  ", "", "")
	if err != nil || name != "علی" || first != "" || last != "" {
		t.Fatalf("got %q %q %q %v", name, first, last, err)
	}
	name, _, _, err = resolveApprovedDoctorNames("", "علي", "كريمي")
	if err != nil || name != "علی کریمی" {
		t.Fatalf("composed = %q %v", name, err)
	}
	if _, _, _, err = resolveApprovedDoctorNames("  ", " ", ""); err == nil {
		t.Fatal("blank doctor name accepted")
	}
}

func TestApplyHISFieldsKeepsSlugAndPreviousName(t *testing.T) {
	repo := &DoctorRepo{}
	doctor := &models.Doctor{Name: "علی", FirstName: "علی", Slug: "ali-old"}
	repo.applyHISFields(doctor, protocol.DoctorDTO{Name: "   ", FirstName: ""})
	if doctor.Name != "علی" || doctor.Slug != "ali-old" {
		t.Fatalf("empty sync cleared name/slug: %#v", doctor)
	}
	repo.applyHISFields(doctor, protocol.DoctorDTO{Name: "علي"})
	if doctor.Name != "علی" || doctor.Slug != "ali-old" {
		t.Fatalf("rename changed slug: %#v", doctor)
	}
}

func TestSlugifyDoctorNameUsesLegacyMake(t *testing.T) {
	got := slugifyDoctorName(&models.Doctor{Name: "دندان\u200cپزشکی"})
	if got != "دندانپزشکی" {
		t.Fatalf("doctor slug = %q", got)
	}
}
