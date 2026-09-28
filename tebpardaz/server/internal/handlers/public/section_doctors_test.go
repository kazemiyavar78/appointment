package public

import (
	"testing"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/models"
	"tebpardaz/shared/constants"
)

// TestToSectionDoctorCardViews فیلدهای کارت رزرو را به مدل نما کپی می‌کند.
func TestToSectionDoctorCardViews(t *testing.T) {
	cards := toSectionDoctorCardViews([]booking.DoctorCard{
		{
			Name:            "دکتر احمدی",
			SpecialtyName:   "داخلی",
			DoctorSystemID:  88,
			PhotoURL:        "/p.jpg",
			ShortDesc:       "توضیح",
			ClinicName:      "مرکز الف",
			ShowClinicBadge: true,
			HasSlot:         true,
			BookingURL:      "/booking/ahmad",
		},
	})
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
	if cards[0].Name != "دکتر احمدی" || cards[0].BookingURL != "/booking/ahmad" || !cards[0].HasSlot {
		t.Fatalf("unexpected card: %+v", cards[0])
	}
}

// TestFallbackSectionDoctorCards کارت ساده را بدون اسلات و با نام ترکیب‌شده می‌سازد.
func TestFallbackSectionDoctorCards(t *testing.T) {
	cards := fallbackSectionDoctorCards([]models.Doctor{
		{
			FirstName:  "سارا",
			LastName:   "کریمی",
			Specialty:  models.Specialty{Name: "زنان"},
			Slug:       "sara-karimi",
			ShortDesc:  " متخصص ",
			IsApproved: true,
			IsActive:   true,
		},
	}, constants.LayoutPrivate, false, "کلینیک نمونه")
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
	if cards[0].Name != "سارا کریمی" {
		t.Fatalf("name = %q", cards[0].Name)
	}
	if cards[0].BookingURL != "/booking/sara-karimi" {
		t.Fatalf("booking url = %q", cards[0].BookingURL)
	}
	if cards[0].ClinicName != "کلینیک نمونه" || cards[0].HasSlot {
		t.Fatalf("unexpected fallback card: %+v", cards[0])
	}
}
