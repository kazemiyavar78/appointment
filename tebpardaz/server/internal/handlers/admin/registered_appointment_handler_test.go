package admin

import (
	"testing"
	"time"

	"tebpardaz/server/internal/models"
	adminviews "tebpardaz/server/views/admin"

	"gorm.io/gorm"
)

// TestAppointmentStatusLabel maps stored statuses to Persian labels.
func TestAppointmentStatusLabel(t *testing.T) {
	cases := map[string]string{
		"pending":      "در انتظار",
		"confirmed":    "تأیید شده",
		"failed":       "ناموفق",
		"otp_pending":  "کد ارسال شد، بدون نوبت",
		"otp_verified": "موبایل تایید شد، بدون نوبت",
		"":             "—",
		"other":        "other",
	}
	for in, want := range cases {
		if got := appointmentStatusLabel(in); got != want {
			t.Fatalf("status %q: got %q want %q", in, got, want)
		}
	}
}

// TestDoctorDisplayName prefers the combined name field.
func TestDoctorDisplayName(t *testing.T) {
	if got := doctorDisplayName(models.Doctor{Name: "دکتر احمدی", FirstName: "علی"}); got != "دکتر احمدی" {
		t.Fatalf("named doctor: %q", got)
	}
	if got := doctorDisplayName(models.Doctor{FirstName: "علی", LastName: "محمدی"}); got != "علی محمدی" {
		t.Fatalf("split name: %q", got)
	}
}

// TestToRegisteredAppointmentRows copies patient, doctor, clinic, and IP onto the table row.
func TestToRegisteredAppointmentRows(t *testing.T) {
	starts := time.Date(2026, 9, 17, 10, 0, 0, 0, time.Local)
	rows := toRegisteredAppointmentRows([]models.PatientAppointment{
		{
			Model:     gorm.Model{ID: 42, CreatedAt: starts},
			ClinicID:  7,
			Status:    "confirmed",
			IPAddress: "203.0.113.10",
			StartsAt:  starts,
			Patient:   models.Patient{FirstName: "سارا", LastName: "رضایی"},
			Doctor:    models.Doctor{Name: "دکتر کریمی"},
		},
	}, map[uint]string{7: "کلینیک نور"})
	if len(rows) != 1 {
		t.Fatalf("len=%d", len(rows))
	}
	row := rows[0]
	if row.ID != 42 || row.TrackingCode != "42" || row.PatientName != "سارا رضایی" {
		t.Fatalf("identity: %+v", row)
	}
	if row.DoctorName != "دکتر کریمی" || row.ClinicName != "کلینیک نور" {
		t.Fatalf("names: %+v", row)
	}
	if row.StatusLabel != "تأیید شده" || row.IPAddress != "203.0.113.10" || row.Kind != "appointment" {
		t.Fatalf("status/ip: %+v", row)
	}
}

// TestToUnbookedOTPRowsShowsPatientsWhoReceivedACodeAndDidNotBook maps an OTP lead onto the admin table.
func TestToUnbookedOTPRowsShowsPatientsWhoReceivedACodeAndDidNotBook(t *testing.T) {
	sent := time.Date(2026, 9, 23, 8, 30, 0, 0, time.Local)
	verified := sent.Add(time.Minute)
	rows := toUnbookedOTPRows([]models.BookingOTP{
		{
			Model:      gorm.Model{ID: 9, CreatedAt: sent},
			ClinicID:   7,
			FirstName:  "سارا",
			LastName:   "رضایی",
			Mobile:     "09120000000",
			IPAddress:  "203.0.113.10",
			LastSentAt: sent,
		},
		{
			Model:      gorm.Model{ID: 10},
			ClinicID:   7,
			FirstName:  "علی",
			LastName:   "کاظمی",
			LastSentAt: sent,
			VerifiedAt: &verified,
		},
	}, map[uint]string{7: "کلینیک نور"})
	if len(rows) != 2 {
		t.Fatalf("len=%d", len(rows))
	}
	if rows[0].Kind != "otp" || rows[0].TrackingCode != "OTP-9" || rows[0].PatientName != "سارا رضایی" {
		t.Fatalf("lead: %+v", rows[0])
	}
	if rows[0].DoctorName != "—" || rows[0].StatusLabel != "کد ارسال شد، بدون نوبت" {
		t.Fatalf("unverified label: %+v", rows[0])
	}
	if rows[1].Status != "otp_verified" || rows[1].StatusLabel != "موبایل تایید شد، بدون نوبت" {
		t.Fatalf("verified label: %+v", rows[1])
	}
}

// TestPageMergedRowsOrdersNewestFirstAndPages mixes appointments and OTP leads onto one page.
func TestPageMergedRowsOrdersNewestFirstAndPages(t *testing.T) {
	older := time.Date(2026, 9, 1, 10, 0, 0, 0, time.Local)
	newer := older.Add(2 * time.Hour)
	got := pageMergedRows([]timedAdminRow{
		{At: older, Row: adminviews.RegisteredAppointmentRow{ID: 1, Kind: "appointment"}},
		{At: newer, Row: adminviews.RegisteredAppointmentRow{ID: 2, Kind: "otp"}},
	}, 1, 1)
	if len(got) != 1 || got[0].ID != 2 || got[0].Kind != "otp" {
		t.Fatalf("page: %+v", got)
	}
}

// TestToPatientInfoPayloadHidesPlaceholderBirthDate keeps the OTP placeholder out of the popup.
func TestToPatientInfoPayloadHidesPlaceholderBirthDate(t *testing.T) {
	got := toPatientInfoPayload(models.Patient{
		NationalID: "0012345678",
		FirstName:  "سارا",
		LastName:   "رضایی",
		Mobile:     "09120000000",
		BirthDate:  models.UnknownBirthDate(),
		Sex:        models.OTHER,
	})
	if got["birth_date"] != "—" {
		t.Fatalf("birth date: %#v", got["birth_date"])
	}
}

// TestToPatientInfoPayloadIncludesPatientModelFields copies identity fields for the popup.
func TestToPatientInfoPayloadIncludesPatientModelFields(t *testing.T) {
	p := models.Patient{
		NationalID: "0012345678",
		FirstName:  "سارا",
		LastName:   "رضایی",
		Mobile:     "09120000000",
		BirthDate:  time.Date(1990, 4, 1, 0, 0, 0, 0, time.Local),
		Sex:        models.FEMALE,
	}
	got := toPatientInfoPayload(p)
	if got["national_id"] != "0012345678" || got["first_name"] != "سارا" || got["last_name"] != "رضایی" {
		t.Fatalf("name fields: %#v", got)
	}
	if got["mobile"] != "09120000000" || got["sex"] != "زن" {
		t.Fatalf("contact/sex: %#v", got)
	}
	if got["birth_date"] == "" || got["birth_date"] == "—" {
		t.Fatalf("birth date missing: %#v", got)
	}
}

// TestToBehaviorVisitPayloadsMapsVisitDetail copies visit fields used in the behavior popup.
func TestToBehaviorVisitPayloadsMapsVisitDetail(t *testing.T) {
	clinicID := uint(7)
	rows := toBehaviorVisitPayloads([]models.VisitDetail{
		{
			Host:         "clinic.example.ir",
			ClinicID:     &clinicID,
			Path:         "/booking",
			FullURL:      "https://clinic.example.ir/booking",
			Browser:      "Chrome",
			OS:           "Windows",
			Referrer:     "https://google.com",
			IsFromGoogle: true,
			CreatedAt:    time.Date(2026, 9, 17, 9, 0, 0, 0, time.Local),
		},
	}, map[uint]string{7: "کلینیک نور"})
	if len(rows) != 1 {
		t.Fatalf("len=%d", len(rows))
	}
	if rows[0]["path"] != "/booking" || rows[0]["clinic_name"] != "کلینیک نور" {
		t.Fatalf("visit: %#v", rows[0])
	}
	if rows[0]["browser"] != "Chrome" || rows[0]["is_from_google"] != true {
		t.Fatalf("ua/google: %#v", rows[0])
	}
}
