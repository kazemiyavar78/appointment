package models

import (
	"testing"

	"gorm.io/gorm"
)

// TestSpecialtyIsVisibleInBooking فقط تخصص تأییدشده با نمایش نوبت‌دهی را قابل‌نمایش می‌داند.
func TestSpecialtyIsVisibleInBooking(t *testing.T) {
	cases := []struct {
		name string
		spec Specialty
		want bool
	}{
		{name: "empty", spec: Specialty{}, want: false},
		{name: "unapproved", spec: Specialty{Model: gorm.Model{ID: 1}, IsApproved: false, ShowInBooking: true}, want: false},
		{name: "hidden", spec: Specialty{Model: gorm.Model{ID: 2}, IsApproved: true, ShowInBooking: false}, want: false},
		{name: "visible", spec: Specialty{Model: gorm.Model{ID: 3}, IsApproved: true, ShowInBooking: true}, want: true},
	}
	for _, tc := range cases {
		if got := tc.spec.IsVisibleInBooking(); got != tc.want {
			t.Fatalf("%s: IsVisibleInBooking() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
