package admin

import (
	"context"
	"strings"
	"testing"

	"tebpardaz/shared/constants"
)

// TestSpecialtiesPageRendersOrderAndBookingFields فرم و جدول تخصص را برای ترتیب نمایش و نوبت‌دهی بررسی می‌کند.
func TestSpecialtiesPageRendersOrderAndBookingFields(t *testing.T) {
	var buf strings.Builder
	view := SpecialtyPageView{
		Nav: BuildAdminNav(NavSpecialties, string(constants.UserRoleSuperAdmin)),
		EditShowInBooking: true,
		Items: []SpecialtyRow{
			{ID: 1, Name: "قلب", SortOrder: 3, IsApproved: true, ShowInBooking: false},
		},
	}
	if err := Specialties(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`name="sort_order"`,
		`name="show_in_booking"`,
		"ترتیب نمایش",
		"نمایش در نوبت‌دهی",
		">مخفی<",
		">3<",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered page missing %q", want)
		}
	}
}
