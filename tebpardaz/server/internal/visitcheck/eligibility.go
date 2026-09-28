package visitcheck

import (
	"errors"
	"strings"
	"time"

	"tebpardaz/server/internal/models"
	"tebpardaz/shared/protocol"

	ptime "github.com/yaa110/go-persian-calendar"
)

const (
	// KindAppointment is a confirmed website booking.
	KindAppointment = "appointment"
	// KindOTP is a patient who received a code and did not book a slot.
	KindOTP = "otp"
	// batchLimit is how many unchecked appointments one automatic pass sends to the clinic.
	batchLimit = 200
)

var (
	// ErrNotTaken means the row is neither a confirmed appointment nor an OTP lead.
	ErrNotTaken = errors.New("appointment was not taken")
	// ErrTooSoon means the appointment calendar day is still today or in the future.
	ErrTooSoon = errors.New("appointment day has not passed")
	// ErrNoDoctorCode means the clinic doctor code needed by paziresh is missing.
	ErrNoDoctorCode = errors.New("doctor local code missing")
	// ErrNoNationalID means the patient national ID needed by paziresh is missing.
	ErrNoNationalID = errors.New("national id missing")
	// ErrOTPIncomplete means an OTP-only lead has no doctor and no appointment date to query.
	ErrOTPIncomplete = errors.New("otp lead has no appointment")
)

// BatchLimit returns how many unchecked appointments one automatic pass may send.
// Inputs: none. Output: positive cap.
func BatchLimit() int {
	return batchLimit
}

// FormatPazDate converts an appointment start instant to the HIS Shamsi date yy/MM/dd.
// Inputs: startsAt (PatientAppointment.StartsAt). Output: for example 04/12/16, or empty when zero.
func FormatPazDate(startsAt time.Time) string {
	if startsAt.IsZero() {
		return ""
	}
	return ptime.New(startsAt).Format("yy/MM/dd")
}

// AppointmentDayPassed reports whether at least one Shamsi day has passed after startsAt.
// Inputs: startsAt (appointment day), now (clock used for today). Output: true only for earlier calendar days.
func AppointmentDayPassed(startsAt, now time.Time) bool {
	if startsAt.IsZero() || now.IsZero() {
		return false
	}
	return ptime.New(startsAt).Format("yyyy/MM/dd") < ptime.New(now).Format("yyyy/MM/dd")
}

// StartOfToday returns midnight at the beginning of today's Shamsi/Iran day.
// Inputs: now. Output: exclusive upper bound for appointment starts that are already in the past.
func StartOfToday(now time.Time) time.Time {
	pt := ptime.New(now)
	return ptime.Date(pt.Year(), pt.Month(), pt.Day(), 0, 0, 0, 0, ptime.Iran()).Time()
}

// StatusFromCount maps a paziresh COUNT(*) onto the stored visit status.
// Inputs: count from the admission table. Output: visited when count > 0, otherwise absent.
func StatusFromCount(count int) string {
	if count > 0 {
		return models.VisitStatusVisited
	}
	return models.VisitStatusAbsent
}

// Label returns the Persian tag for a stored visit status.
// Inputs: visit status (empty, visited, or absent). Output: display text.
func Label(status string) string {
	switch strings.TrimSpace(status) {
	case models.VisitStatusVisited:
		return "مراجعه کرده"
	case models.VisitStatusAbsent:
		return "مراجعه نکرده"
	case "":
		return "بررسی نشده"
	default:
		return status
	}
}

// PrepareAppointment builds one client lookup for a confirmed appointment whose day has passed.
// Inputs: appointment with Patient and Doctor loaded, and the current time.
// Output: protocol item, or ErrNotTaken, ErrTooSoon, ErrNoDoctorCode, or ErrNoNationalID.
func PrepareAppointment(a *models.PatientAppointment, now time.Time) (protocol.VisitCheckItem, error) {
	if a == nil || strings.TrimSpace(a.Status) != "confirmed" {
		return protocol.VisitCheckItem{}, ErrNotTaken
	}
	if !AppointmentDayPassed(a.StartsAt, now) {
		return protocol.VisitCheckItem{}, ErrTooSoon
	}
	nationalID := strings.TrimSpace(a.Patient.NationalID)
	if nationalID == "" {
		return protocol.VisitCheckItem{}, ErrNoNationalID
	}
	if a.Doctor.LocalCode <= 0 {
		return protocol.VisitCheckItem{}, ErrNoDoctorCode
	}
	pazDate := FormatPazDate(a.StartsAt)
	if pazDate == "" {
		return protocol.VisitCheckItem{}, ErrTooSoon
	}
	return protocol.VisitCheckItem{
		Kind:       KindAppointment,
		ID:         a.ID,
		DoctorCode: a.Doctor.LocalCode,
		PazDate:    pazDate,
		NationalID: nationalID,
	}, nil
}
