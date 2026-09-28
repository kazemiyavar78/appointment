package repository

import (
	"errors"
	"strings"
	"time"

	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

var (
	// ErrOTPLeadNotFound is returned when no booking OTP row matches the id.
	ErrOTPLeadNotFound = errors.New("otp lead not found")
)

// OTPRepo reads persisted booking OTP challenges for the admin follow-up list.
type OTPRepo struct {
	DB *gorm.DB
}

// NewOTPRepo constructs an OTPRepo.
// Inputs: appointment DB handle. Output: pointer to OTPRepo.
func NewOTPRepo(db *gorm.DB) *OTPRepo {
	return &OTPRepo{DB: db}
}

// OTPLeadFilter limits the admin list of patients who received an OTP and did not book.
type OTPLeadFilter struct {
	AllowedClinicIDs []uint
	ClinicID         uint
	Query            string
	From             *time.Time
	To               *time.Time
	Page             int
	PerPage          int
	// MaxPerPage raises the page-size cap for an internal merge window. Zero keeps 200.
	MaxPerPage int
}

// OTPLeadListResult is a page of unbooked OTP challenges.
type OTPLeadListResult struct {
	Rows  []models.BookingOTP
	Total int64
}

// ListUnbooked returns delivered OTP rows that were not followed by a booking.
// Inputs: clinic scope, optional search and send-time range, pagination.
// Output: newest sends first, plus the total count.
func (r *OTPRepo) ListUnbooked(filter OTPLeadFilter) (OTPLeadListResult, error) {
	out := OTPLeadListResult{Rows: []models.BookingOTP{}}
	if r == nil || r.DB == nil || len(filter.AllowedClinicIDs) == 0 {
		return out, nil
	}
	page, perPage := normalizeAppointmentPage(filter.Page, filter.PerPage, filter.MaxPerPage)
	q := r.DB.Model(&models.BookingOTP{})
	q = applyUnbookedOTPFilter(q, filter)
	if err := q.Count(&out.Total).Error; err != nil {
		return out, err
	}
	err := q.Order("booking_otps.last_sent_at desc, booking_otps.id desc").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&out.Rows).Error
	return out, err
}

// GetByID loads one booking OTP challenge.
// Inputs: booking_otps.id. Output: row, ErrOTPLeadNotFound, or a database error.
func (r *OTPRepo) GetByID(id uint) (*models.BookingOTP, error) {
	if r == nil || r.DB == nil || id == 0 {
		return nil, ErrOTPLeadNotFound
	}
	var row models.BookingOTP
	err := r.DB.First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOTPLeadNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// applyUnbookedOTPFilter keeps OTP sends that have no later appointment for the same patient and clinic.
// A patient who only received the code, or verified it and stopped, stays in this set.
func applyUnbookedOTPFilter(q *gorm.DB, filter OTPLeadFilter) *gorm.DB {
	q = q.Where("booking_otps.clinic_id IN ?", filter.AllowedClinicIDs).
		Where("booking_otps.sms_sent_at IS NOT NULL").
		Where("booking_otps.booked_at IS NULL").
		Where(`booking_otps.patient_id = 0 OR NOT EXISTS (
			SELECT 1 FROM patient_appointments
			WHERE patient_appointments.deleted_at IS NULL
			  AND patient_appointments.patient_id = booking_otps.patient_id
			  AND patient_appointments.clinic_id = booking_otps.clinic_id
			  AND patient_appointments.created_at >= booking_otps.last_sent_at
		)`)
	if filter.ClinicID > 0 {
		q = q.Where("booking_otps.clinic_id = ?", filter.ClinicID)
	}
	if filter.From != nil {
		q = q.Where("booking_otps.last_sent_at >= ?", *filter.From)
	}
	if filter.To != nil {
		q = q.Where("booking_otps.last_sent_at <= ?", *filter.To)
	}
	if search := strings.TrimSpace(filter.Query); search != "" {
		like := "%" + search + "%"
		q = q.Where(`booking_otps.ip_address LIKE ? OR booking_otps.first_name LIKE ? OR booking_otps.last_name LIKE ?
			OR booking_otps.national_id LIKE ? OR booking_otps.mobile LIKE ?
			OR CAST(booking_otps.id AS NVARCHAR(20)) LIKE ?`,
			like, like, like, like, like, like)
	}
	return q
}
