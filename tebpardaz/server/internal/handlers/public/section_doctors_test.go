package public

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/views/components"
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
	clinic := &models.Clinic{Name: "کلینیک نمونه"}
	clinic.ID = 3
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
	}, constants.LayoutPrivate, false, clinic)
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

// TestFallbackSectionDoctorCardHrefMatchesNormalPath لینک fallback را با ClinicPathKey مسیر عادی یکی می‌کند.
func TestFallbackSectionDoctorCardHrefMatchesNormalPath(t *testing.T) {
	clinicSlug := "چمران-مشهد"
	doctorSlug := "آقای-جواد-منزه"
	clinic := &models.Clinic{Name: "درمانگاه چمران", Slug: &clinicSlug}
	clinic.ID = 9
	doctor := models.Doctor{
		Name:      "آقای جواد منزه",
		Slug:      doctorSlug,
		ClinicID:  clinic.ID,
		Specialty: models.Specialty{Name: "داخلی"},
	}
	doctor.ID = 4

	cases := []struct {
		name   string
		layout constants.LayoutKind
		want   string
	}{
		{name: "platform", layout: constants.LayoutPlatform, want: "/booking/" + clinicSlug + "/" + doctorSlug},
		{name: "organization", layout: constants.LayoutOrgan, want: "/booking/" + clinicSlug + "/" + doctorSlug},
		{name: "own-domain", layout: constants.LayoutPrivate, want: "/booking/" + doctorSlug},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fallback := fallbackSectionDoctorCards([]models.Doctor{doctor}, tt.layout, false, clinic)
			normal := booking.PublicDoctorCards([]models.Doctor{doctor}, []models.Clinic{*clinic}, nil, tt.layout, false)
			if len(fallback) != 1 || len(normal) != 1 {
				t.Fatalf("cards fallback=%d normal=%d", len(fallback), len(normal))
			}
			if fallback[0].BookingURL != tt.want || normal[0].BookingURL != tt.want {
				t.Fatalf("fallback=%q normal=%q want=%q", fallback[0].BookingURL, normal[0].BookingURL, tt.want)
			}
			if strings.Contains(fallback[0].BookingURL, "/doctors") || strings.Contains(fallback[0].BookingURL, "c9") || strings.Contains(fallback[0].BookingURL, "%25") {
				t.Fatalf("bad href %q", fallback[0].BookingURL)
			}
			views := toSectionDoctorCardViews(fallback)
			var buf bytes.Buffer
			if err := components.DoctorCard(views[0]).Render(context.Background(), &buf); err != nil {
				t.Fatal(err)
			}
			html := buf.String()
			if !strings.Contains(html, `href="`+tt.want+`"`) || strings.Contains(html, "%25") || strings.Contains(html, "tebpardaz.ir") || strings.Contains(html, "chamranclinic.ir") {
				t.Fatalf("href html:\n%s", html)
			}
		})
	}
}
