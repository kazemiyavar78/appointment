package admin

import (
	"context"
	"strings"
	"testing"
	"time"

	"tebpardaz/shared/constants"
)

// TestRegisteredAppointmentsRendersRowsAndActions checks the booked-appointment table and popup buttons.
func TestRegisteredAppointmentsRendersRowsAndActions(t *testing.T) {
	var buf strings.Builder
	view := RegisteredAppointmentsView{
		Nav: BuildAdminNav(NavRegisteredAppointments, string(constants.UserRoleSuperAdmin)),
		Rows: []RegisteredAppointmentRow{
			{
				ID:           42,
				TrackingCode: "42",
				PatientName:  "سارا رضایی",
				DoctorName:   "دکتر کریمی",
				ClinicName:   "کلینیک نور",
				StartsAt:     time.Date(2026, 9, 17, 10, 0, 0, 0, time.Local),
				StatusLabel:  "تأیید شده",
				IPAddress:    "203.0.113.10",
			},
		},
		Total:      1,
		Page:       1,
		TotalPages: 1,
	}
	if err := RegisteredAppointments(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		"نوبت‌های ثبت‌شده",
		"کد تایید گرفته‌اند",
		"بدون ثبت نوبت",
		"سارا رضایی",
		"دکتر کریمی",
		"203.0.113.10",
		`data-action="patient"`,
		`data-action="behavior"`,
		`data-action="visit"`,
		"بررسی مراجعه",
		"/visit-check",
		`data-id="42"`,
		"اطلاعات بیمار",
		"رفتار بیمار",
		"/admin/bookings/",
		"/patient",
		"/behavior",
		"admin-modal",
		"admin-modal-body",
		"admin-modal-table-wrap",
		"admin-modal-open",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered page missing %q", want)
		}
	}
}

// TestRegisteredAppointmentsHrefKeepsFilters builds a paged URL with clinic and status filters.
func TestRegisteredAppointmentsHrefKeepsFilters(t *testing.T) {
	got := registeredAppointmentsHref(RegisteredAppointmentsView{
		SearchQuery: "سارا",
		ClinicID:    7,
		Status:      "confirmed",
		FromDate:    "2026-09-01",
	}, 2)
	for _, want := range []string{"/admin/bookings?", "q=", "clinic_id=7", "status=confirmed", "from=2026-09-01", "page=2"} {
		if !strings.Contains(got, want) {
			t.Fatalf("href %q missing %q", got, want)
		}
	}
}
