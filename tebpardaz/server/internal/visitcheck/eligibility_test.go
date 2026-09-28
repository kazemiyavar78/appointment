package visitcheck

import (
	"errors"
	"testing"
	"time"

	"tebpardaz/server/internal/models"

	ptime "github.com/yaa110/go-persian-calendar"
	"gorm.io/gorm"
)

// TestFormatPazDateUsesTwoDigitShamsi verifies the HIS date shape 04/12/16.
func TestFormatPazDateUsesTwoDigitShamsi(t *testing.T) {
	starts := ptime.Date(1404, ptime.Esfand, 16, 9, 30, 0, 0, ptime.Iran()).Time()
	if got := FormatPazDate(starts); got != "04/12/16" {
		t.Fatalf("paz date: got %q", got)
	}
	if got := FormatPazDate(time.Time{}); got != "" {
		t.Fatalf("zero date: got %q", got)
	}
}

// TestAppointmentDayPassedRequiresALaterShamsiDay blocks same-day and future checks.
func TestAppointmentDayPassedRequiresALaterShamsiDay(t *testing.T) {
	starts := ptime.Date(1404, ptime.Esfand, 16, 18, 0, 0, 0, ptime.Iran()).Time()
	sameDay := ptime.Date(1404, ptime.Esfand, 16, 23, 50, 0, 0, ptime.Iran()).Time()
	nextDay := ptime.Date(1404, ptime.Esfand, 17, 0, 5, 0, 0, ptime.Iran()).Time()
	if AppointmentDayPassed(starts, sameDay) {
		t.Fatal("same shamsi day should not be ready")
	}
	if !AppointmentDayPassed(starts, nextDay) {
		t.Fatal("next shamsi day should be ready")
	}
	if AppointmentDayPassed(nextDay, starts) {
		t.Fatal("future appointment should not be ready")
	}
}

// TestStatusFromCountTreatsZeroAsAbsent maps the admission count onto the stored tag.
func TestStatusFromCountTreatsZeroAsAbsent(t *testing.T) {
	if StatusFromCount(0) != models.VisitStatusAbsent || Label(models.VisitStatusAbsent) != "مراجعه نکرده" {
		t.Fatal("zero count")
	}
	if StatusFromCount(2) != models.VisitStatusVisited || Label(models.VisitStatusVisited) != "مراجعه کرده" {
		t.Fatal("positive count")
	}
	if Label("") != "بررسی نشده" {
		t.Fatal("unchecked label")
	}
}

// TestPrepareAppointmentRejectsPendingAndSameDay keeps only past confirmed bookings.
func TestPrepareAppointmentRejectsPendingAndSameDay(t *testing.T) {
	starts := ptime.Date(1404, ptime.Esfand, 16, 10, 0, 0, 0, ptime.Iran()).Time()
	now := ptime.Date(1404, ptime.Esfand, 17, 8, 0, 0, 0, ptime.Iran()).Time()
	base := &models.PatientAppointment{
		Model:    gorm.Model{ID: 9},
		Status:   "confirmed",
		StartsAt: starts,
		Patient:  models.Patient{NationalID: "0012345678"},
		Doctor:   models.Doctor{LocalCode: 42},
	}
	item, err := PrepareAppointment(base, now)
	if err != nil {
		t.Fatalf("ready appointment: %v", err)
	}
	if item.Kind != KindAppointment || item.ID != 9 || item.DoctorCode != 42 || item.PazDate != "04/12/16" || item.NationalID != "0012345678" {
		t.Fatalf("item: %+v", item)
	}

	pending := *base
	pending.Status = "pending"
	if _, err := PrepareAppointment(&pending, now); !errors.Is(err, ErrNotTaken) {
		t.Fatalf("pending: %v", err)
	}
	if _, err := PrepareAppointment(base, starts); !errors.Is(err, ErrTooSoon) {
		t.Fatalf("same day: %v", err)
	}
	noDoctor := *base
	noDoctor.Doctor.LocalCode = 0
	if _, err := PrepareAppointment(&noDoctor, now); !errors.Is(err, ErrNoDoctorCode) {
		t.Fatalf("doctor: %v", err)
	}
}
