package repository

import (
	"testing"

	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// TestFilterPublicSectionDoctors پزشکان تأییدنشده یا غیرفعال را از لیست عمومی حذف می‌کند.
func TestFilterPublicSectionDoctors(t *testing.T) {
	bookable := models.Specialty{Model: gorm.Model{ID: 10}, IsApproved: true, ShowInBooking: true}
	hidden := models.Specialty{Model: gorm.Model{ID: 11}, IsApproved: true, ShowInBooking: false}
	links := []models.SectionDoctor{
		{Doctor: models.Doctor{Name: "hidden-empty"}},
		{Doctor: models.Doctor{Model: gorm.Model{ID: 1}, Name: "approved-active", IsApproved: true, IsActive: true, Specialty: bookable}},
		{Doctor: models.Doctor{Model: gorm.Model{ID: 2}, Name: "unapproved", IsApproved: false, IsActive: true, Specialty: bookable}},
		{Doctor: models.Doctor{Model: gorm.Model{ID: 3}, Name: "inactive", IsApproved: true, IsActive: false, Specialty: bookable}},
		{Doctor: models.Doctor{Model: gorm.Model{ID: 4}, Name: "also-ok", IsApproved: true, IsActive: true, Specialty: bookable}},
		{Doctor: models.Doctor{Model: gorm.Model{ID: 5}, Name: "hidden-specialty", IsApproved: true, IsActive: true, Specialty: hidden}},
	}

	got := filterPublicSectionDoctors(links)
	if len(got) != 2 {
		t.Fatalf("expected 2 public doctors, got %d", len(got))
	}
	if got[0].Name != "approved-active" || got[1].Name != "also-ok" {
		t.Fatalf("unexpected order or names: %+v", got)
	}
}
