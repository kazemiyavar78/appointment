package repository

import (
	"errors"
	"strings"
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

// GetByNationalID بیمار ثبت‌شده را با کد ملی برمی‌گرداند.
// ورودی: کد ملی ۱۰ رقمی. خروجی: Patient یا ErrAppointmentNotFound.
func (r *AppointmentRepo) GetByNationalID(nationalID string) (*models.Patient, error) {
	if r == nil || r.DB == nil {
		return nil, gorm.ErrInvalidData
	}
	nationalID = strings.TrimSpace(nationalID)
	if nationalID == "" {
		return nil, ErrAppointmentNotFound
	}
	var p models.Patient
	err := r.DB.Where("national_id = ?", nationalID).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAppointmentNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
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

// UpsertPatientIdentity creates a patient from the OTP form or refreshes name and mobile.
// Inputs: national ID, first name, last name, mobile. Birth date and sex are not overwritten when the patient already exists.
// Output: persisted Patient, or a database error.
func (r *AppointmentRepo) UpsertPatientIdentity(nationalID, firstName, lastName, mobile string) (*models.Patient, error) {
	if r == nil || r.DB == nil {
		return nil, gorm.ErrInvalidData
	}
	nationalID = strings.TrimSpace(nationalID)
	firstName = strings.TrimSpace(firstName)
	lastName = strings.TrimSpace(lastName)
	mobile = strings.TrimSpace(mobile)
	if nationalID == "" || firstName == "" || lastName == "" || mobile == "" {
		return nil, gorm.ErrInvalidData
	}

	var existing models.Patient
	err := r.DB.Where("national_id = ?", nationalID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		created := &models.Patient{
			NationalID: nationalID,
			FirstName:  firstName,
			LastName:   lastName,
			Mobile:     mobile,
			BirthDate:  models.UnknownBirthDate(),
			Sex:        models.OTHER,
		}
		if err := r.DB.Create(created).Error; err != nil {
			return nil, err
		}
		return created, nil
	}
	if err != nil {
		return nil, err
	}
	existing.FirstName = firstName
	existing.LastName = lastName
	existing.Mobile = mobile
	if err := r.DB.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

// GetPatientByID loads one patient by primary key.
// Inputs: patients.id. Output: patient, ErrAppointmentNotFound, or a database error.
func (r *AppointmentRepo) GetPatientByID(id uint) (*models.Patient, error) {
	if r == nil || r.DB == nil || id == 0 {
		return nil, ErrAppointmentNotFound
	}
	var p models.Patient
	err := r.DB.First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAppointmentNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
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

// RegisteredAppointmentFilter filters the admin list of website-booked appointments.
type RegisteredAppointmentFilter struct {
	AllowedClinicIDs []uint
	ClinicID         uint
	Status           string
	Query            string
	From             *time.Time
	To               *time.Time
	Page             int
	PerPage          int
	// MaxPerPage raises the page-size cap for an internal merge window. Zero keeps the public cap of 200.
	MaxPerPage int
}

// RegisteredAppointmentListResult is a paginated list of patient appointments.
type RegisteredAppointmentListResult struct {
	Rows  []models.PatientAppointment
	Total int64
}

// ListRegistered returns website-booked appointments with Patient and Doctor preloaded.
// Inputs: filter with clinic scope, optional status/search/date, and pagination.
// Output: rows ordered newest-first and total count, or a database error.
func (r *AppointmentRepo) ListRegistered(filter RegisteredAppointmentFilter) (RegisteredAppointmentListResult, error) {
	out := RegisteredAppointmentListResult{Rows: []models.PatientAppointment{}}
	if r == nil || r.DB == nil {
		return out, nil
	}
	if len(filter.AllowedClinicIDs) == 0 {
		return out, nil
	}

	page, perPage := normalizeAppointmentPage(filter.Page, filter.PerPage, filter.MaxPerPage)
	q := r.DB.Model(&models.PatientAppointment{})
	q = applyRegisteredAppointmentFilter(q, filter)
	if err := q.Count(&out.Total).Error; err != nil {
		return out, err
	}
	err := q.Preload("Patient").
		Preload("Doctor").
		Order("patient_appointments.id desc").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&out.Rows).Error
	return out, err
}

// GetByID loads one patient appointment with Patient and Doctor associations.
// Inputs: appointment primary key.
// Output: appointment pointer, ErrAppointmentNotFound, or a database error.
func (r *AppointmentRepo) GetByID(id uint) (*models.PatientAppointment, error) {
	if r == nil || r.DB == nil || id == 0 {
		return nil, ErrAppointmentNotFound
	}
	var a models.PatientAppointment
	err := r.DB.Preload("Patient").Preload("Doctor").First(&a, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAppointmentNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ListUnreviewedVisits returns confirmed appointments whose start is before today and whose visit status is still empty.
// Inputs: clinic ID, exclusive start-of-today bound, and a positive row cap.
// Output: rows with Patient and Doctor loaded, oldest first, or a database error.
func (r *AppointmentRepo) ListUnreviewedVisits(clinicID uint, before time.Time, limit int) ([]models.PatientAppointment, error) {
	if r == nil || r.DB == nil || clinicID == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 200
	}
	var rows []models.PatientAppointment
	err := r.DB.Select("patient_appointments.*").
		Preload("Patient").Preload("Doctor").
		Joins("JOIN patients ON patients.id = patient_appointments.patient_id AND patients.deleted_at IS NULL").
		Joins("JOIN doctors ON doctors.id = patient_appointments.doctor_id AND doctors.deleted_at IS NULL").
		Where("patient_appointments.clinic_id = ?", clinicID).
		Where("patient_appointments.status = ?", "confirmed").
		Where("(patient_appointments.visit_status = '' OR patient_appointments.visit_status IS NULL)").
		Where("patient_appointments.starts_at < ?", before).
		Where("patients.national_id <> ''").
		Where("doctors.local_code > 0").
		Order("patient_appointments.starts_at asc, patient_appointments.id asc").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// MarkVisitStatus stores the admission outcome for one appointment.
// Inputs: appointment ID, visited or absent, and the check time. Output: database error.
func (r *AppointmentRepo) MarkVisitStatus(id uint, status string, at time.Time) error {
	if r == nil || r.DB == nil || id == 0 {
		return gorm.ErrInvalidData
	}
	return r.DB.Model(&models.PatientAppointment{}).Where("id = ?", id).Updates(map[string]any{
		"visit_status":     status,
		"visit_checked_at": at,
	}).Error
}

// applyRegisteredAppointmentFilter restricts the appointment list query by clinic, status, search, and date.
func applyRegisteredAppointmentFilter(q *gorm.DB, filter RegisteredAppointmentFilter) *gorm.DB {
	q = q.Where("patient_appointments.clinic_id IN ?", filter.AllowedClinicIDs)
	if filter.ClinicID > 0 {
		q = q.Where("patient_appointments.clinic_id = ?", filter.ClinicID)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		q = q.Where("patient_appointments.status = ?", status)
	}
	if filter.From != nil {
		q = q.Where("patient_appointments.starts_at >= ?", *filter.From)
	}
	if filter.To != nil {
		q = q.Where("patient_appointments.starts_at <= ?", *filter.To)
	}
	if search := strings.TrimSpace(filter.Query); search != "" {
		like := "%" + search + "%"
		// Join identity tables so admins can search by patient, doctor, tracking code, or booking IP.
		q = q.Joins("LEFT JOIN patients ON patients.id = patient_appointments.patient_id AND patients.deleted_at IS NULL").
			Joins("LEFT JOIN doctors ON doctors.id = patient_appointments.doctor_id AND doctors.deleted_at IS NULL").
			Where(`patient_appointments.ip_address LIKE ? OR patient_appointments.status LIKE ? OR patient_appointments.external_id LIKE ?
				OR CAST(patient_appointments.id AS NVARCHAR(20)) LIKE ?
				OR patients.first_name LIKE ? OR patients.last_name LIKE ? OR patients.national_id LIKE ? OR patients.mobile LIKE ?
				OR doctors.name LIKE ? OR doctors.first_name LIKE ? OR doctors.last_name LIKE ?`,
				like, like, like, like, like, like, like, like, like, like, like)
	}
	return q
}

// normalizeAppointmentPage clamps list pagination.
// Inputs: requested page and page size, plus optional max page size (0 means 200).
// Output: page starting at 1 and a per-page size inside the cap.
func normalizeAppointmentPage(page, perPage, maxPerPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	if maxPerPage <= 0 {
		maxPerPage = 200
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	return page, perPage
}
