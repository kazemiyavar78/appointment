package repository

import (
	"errors"
	"time"

	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

var (
	// ErrAppointmentNotFound is returned when no matching patient appointment exists.
	ErrAppointmentNotFound = errors.New("appointment not found")
)

// AppointmentRepo persists patients and website appointments on the central site.
type AppointmentRepo struct {
	DB *gorm.DB
}

// NewAppointmentRepo constructs an AppointmentRepo.
// Inputs: appointment DB handle.
// Output: pointer to AppointmentRepo.
func NewAppointmentRepo(db *gorm.DB) *AppointmentRepo {
	return &AppointmentRepo{DB: db}
}

// UpsertPatient creates or updates a patient by national ID.
// Inputs: patient fields to store.
// Output: persisted Patient or DB error.
func (r *AppointmentRepo) UpsertPatient(p *models.Patient) (*models.Patient, error) {
	if r == nil || r.DB == nil || p == nil {
		return nil, gorm.ErrInvalidData
	}
	var existing models.Patient
	err := r.DB.Where("national_id = ?", p.NationalID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := r.DB.Create(p).Error; err != nil {
			return nil, err
		}
		return p, nil
	}
	if err != nil {
		return nil, err
	}
	existing.FirstName = p.FirstName
	existing.LastName = p.LastName
	existing.Mobile = p.Mobile
	existing.BirthDate = p.BirthDate
	existing.Sex = p.Sex
	if err := r.DB.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

// CreatePending inserts a pending PatientAppointment with a unique idempotency key.
// Inputs: appointment row (Status should be pending).
// Output: created row or DB error.
func (r *AppointmentRepo) CreatePending(a *models.PatientAppointment) error {
	if r == nil || r.DB == nil || a == nil {
		return gorm.ErrInvalidData
	}
	if a.Status == "" {
		a.Status = "pending"
	}
	return r.DB.Create(a).Error
}

// MarkConfirmed updates appointment status after clinic ack success.
// Inputs: appointment ID, external HIS id, confirmed timestamp.
// Output: DB error.
func (r *AppointmentRepo) MarkConfirmed(id uint, externalID string, at time.Time) error {
	if r == nil || r.DB == nil {
		return gorm.ErrInvalidData
	}
	return r.DB.Model(&models.PatientAppointment{}).Where("id = ?", id).Updates(map[string]any{
		"status":       "confirmed",
		"external_id":  externalID,
		"confirmed_at": at,
		"fail_reason":  "",
	}).Error
}

// MarkFailed updates appointment status after clinic ack failure.
// Inputs: appointment ID, fail reason.
// Output: DB error.
func (r *AppointmentRepo) MarkFailed(id uint, reason string) error {
	if r == nil || r.DB == nil {
		return gorm.ErrInvalidData
	}
	return r.DB.Model(&models.PatientAppointment{}).Where("id = ?", id).Updates(map[string]any{
		"status":      "failed",
		"fail_reason": reason,
	}).Error
}
