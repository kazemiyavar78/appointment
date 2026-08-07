package repository

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/slug"
	"tebpardaz/shared/constants"
	"tebpardaz/shared/protocol"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DoctorRepo provides persistence for doctors, specialties, and approval requests.
type DoctorRepo struct {
	DB *gorm.DB
}

// NewDoctorRepo constructs a DoctorRepo.
func NewDoctorRepo(db *gorm.DB) *DoctorRepo {
	return &DoctorRepo{DB: db}
}

// UpsertResult is one doctor sync outcome returned to the clinic client.
type UpsertResult struct {
	LocalCode  int
	ExternalID string
	DoctorID   uint
	Pending    bool
}

// UpsertFromSync creates or updates doctors from a clinic push and queues approval when needed.
// ExternalID is assigned only after admin approval, not during sync.
func (r *DoctorRepo) UpsertFromSync(clinicID uint, doctors []protocol.DoctorDTO) ([]UpsertResult, error) {
	results := make([]UpsertResult, 0, len(doctors))
	for _, dto := range doctors {
		res, err := r.upsertOne(clinicID, dto)
		if err != nil {
			return results, err
		}
		results = append(results, res)
	}
	return results, nil
}

func (r *DoctorRepo) upsertOne(clinicID uint, dto protocol.DoctorDTO) (UpsertResult, error) {
	doctor, found, err := r.findExisting(clinicID, dto)
	if err != nil {
		return UpsertResult{}, err
	}

	if found {
		r.applyDTO(&doctor, dto)
		if err := r.DB.Save(&doctor).Error; err != nil {
			return UpsertResult{}, err
		}
		return UpsertResult{
			LocalCode:  dto.LocalCode,
			ExternalID: doctor.ExternalID,
			DoctorID:   doctor.ID,
			Pending:    !doctor.IsApproved,
		}, nil
	}

	doctor = models.Doctor{
		ClinicID:       clinicID,
		SpecialtyID:    0,
		Name:           dto.Name,
		FirstName:      dto.FirstName,
		LastName:       dto.LastName,
		Mobile:         dto.Mobile,
		NationalID:     dto.NationalID,
		DoctorSystemID: dto.DoctorSystemID,
		SpecialtyCode:  dto.SpecialtyCode,
		PhotoURL:       dto.PhotoURL,
		ExternalID:     "",
		LocalCode:      dto.LocalCode,
		IsApproved:     false,
		IsActive:       dto.IsActive,
	}
	if doctor.Name == "" {
		doctor.Name = dto.FirstName + " " + dto.LastName
	}
	if err := r.DB.Create(&doctor).Error; err != nil {
		return UpsertResult{}, err
	}
	if err := r.EnsureSlug(&doctor); err != nil {
		return UpsertResult{}, err
	}

	payload, _ := json.Marshal(dto)
	req := models.DoctorApprovalRequest{
		ClinicID:      clinicID,
		DoctorID:      &doctor.ID,
		ExternalID:    "",
		RequestedName: doctor.Name,
		Status:        string(constants.ApprovalPending),
		PayloadJSON:   string(payload),
	}
	if err := r.DB.Create(&req).Error; err != nil {
		return UpsertResult{}, err
	}

	return UpsertResult{
		LocalCode:  dto.LocalCode,
		ExternalID: "",
		DoctorID:   doctor.ID,
		Pending:    true,
	}, nil
}

func (r *DoctorRepo) applyDTO(doctor *models.Doctor, dto protocol.DoctorDTO) {
	if dto.Name != "" {
		doctor.Name = dto.Name
	}
	if dto.FirstName != "" {
		doctor.FirstName = dto.FirstName
	}
	if dto.LastName != "" {
		doctor.LastName = dto.LastName
	}
	if dto.Mobile != "" {
		doctor.Mobile = dto.Mobile
	}
	if dto.NationalID != "" {
		doctor.NationalID = dto.NationalID
	}
	if dto.DoctorSystemID > 0 {
		doctor.DoctorSystemID = dto.DoctorSystemID
	}
	if dto.SpecialtyCode != "" {
		doctor.SpecialtyCode = dto.SpecialtyCode
	}
	if dto.PhotoURL != "" {
		doctor.PhotoURL = dto.PhotoURL
	}
	if dto.LocalCode > 0 {
		doctor.LocalCode = dto.LocalCode
	}
	doctor.IsActive = dto.IsActive
}

// FindExistingPublic locates a doctor by ExternalID, LocalCode, or NationalID within a clinic.
func (r *DoctorRepo) FindExistingPublic(clinicID uint, dto protocol.DoctorDTO) (models.Doctor, bool, error) {
	return r.findExisting(clinicID, dto)
}

func (r *DoctorRepo) findExisting(clinicID uint, dto protocol.DoctorDTO) (models.Doctor, bool, error) {
	var doctor models.Doctor
	if dto.ExternalID != "" {
		tx := r.DB.Where("clinic_id = ? AND external_id = ?", clinicID, dto.ExternalID).Limit(1).Find(&doctor)
		if tx.Error != nil {
			return doctor, false, tx.Error
		}
		if tx.RowsAffected > 0 {
			return doctor, true, nil
		}
	}
	if dto.LocalCode > 0 {
		tx := r.DB.Where("clinic_id = ? AND local_code = ?", clinicID, dto.LocalCode).Limit(1).Find(&doctor)
		if tx.Error != nil {
			return doctor, false, tx.Error
		}
		if tx.RowsAffected > 0 {
			return doctor, true, nil
		}
	}
	if dto.NationalID != "" {
		tx := r.DB.Where("clinic_id = ? AND national_id = ?", clinicID, dto.NationalID).Limit(1).Find(&doctor)
		if tx.Error != nil {
			return doctor, false, tx.Error
		}
		if tx.RowsAffected > 0 {
			return doctor, true, nil
		}
	}
	return doctor, false, nil
}

// DecideApproval sets approved/rejected, assigns ExternalID and specialty on approve.
func (r *DoctorRepo) DecideApproval(requestID, reviewerID uint, approve bool, specialtyID uint, note string) (*models.DoctorApprovalRequest, error) {
	var req models.DoctorApprovalRequest
	if err := r.DB.First(&req, requestID).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	req.ReviewedBy = &reviewerID
	req.ReviewedAt = &now
	req.Note = note
	if approve {
		req.Status = string(constants.ApprovalApproved)
	} else {
		req.Status = string(constants.ApprovalRejected)
	}
	if err := r.DB.Save(&req).Error; err != nil {
		return nil, err
	}
	if req.DoctorID == nil {
		return &req, nil
	}

	updates := map[string]any{"is_approved": approve}
	if approve {
		externalID := uuid.NewString()
		req.ExternalID = externalID
		updates["external_id"] = externalID
		if specialtyID > 0 {
			updates["specialty_id"] = specialtyID
		}
		if err := r.DB.Save(&req).Error; err != nil {
			return nil, err
		}
	}
	if err := r.DB.Model(&models.Doctor{}).Where("id = ?", *req.DoctorID).Updates(updates).Error; err != nil {
		return nil, err
	}
	if approve {
		if doc, err := r.GetByID(*req.DoctorID); err == nil && doc != nil {
			_ = r.EnsureSlug(doc)
		}
	}
	return &req, nil
}

// GetByID loads a doctor by primary key.
func (r *DoctorRepo) GetByID(id uint) (*models.Doctor, error) {
	var doctor models.Doctor
	if err := r.DB.Preload("Specialty").First(&doctor, id).Error; err != nil {
		return nil, err
	}
	return &doctor, nil
}

// GetPublicByClinicAndSlug loads an approved active doctor by clinic and URL slug.
// Inputs: clinicID, slug (public path segment).
// Output: doctor with Specialty preloaded, or gorm.ErrRecordNotFound.
func (r *DoctorRepo) GetPublicByClinicAndSlug(clinicID uint, slug string) (*models.Doctor, error) {
	slug = strings.TrimSpace(slug)
	if r == nil || r.DB == nil || clinicID == 0 || slug == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var doctor models.Doctor
	err := r.DB.Where(
		"clinic_id = ? AND slug = ? AND is_approved = ? AND is_active = ? AND external_id <> ''",
		clinicID, slug, true, true,
	).Preload("Specialty").First(&doctor).Error
	if err != nil {
		return nil, err
	}
	return &doctor, nil
}

// EnsureSlug assigns a unique per-clinic slug when missing.
// Inputs: doctor pointer (must have ID and ClinicID).
// Output: error when persistence fails.
func (r *DoctorRepo) EnsureSlug(doctor *models.Doctor) error {
	if r == nil || r.DB == nil || doctor == nil || doctor.ID == 0 {
		return fmt.Errorf("doctor slug ensure unavailable")
	}
	if strings.TrimSpace(doctor.Slug) != "" {
		return nil
	}
	base := slugifyDoctorName(doctor)
	slug := base
	for i := 0; i < 100; i++ {
		if i > 0 {
			slug = fmt.Sprintf("%s-%d", base, i+1)
		}
		var count int64
		err := r.DB.Model(&models.Doctor{}).
			Where("clinic_id = ? AND slug = ? AND id <> ?", doctor.ClinicID, slug, doctor.ID).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count == 0 {
			doctor.Slug = slug
			return r.DB.Model(doctor).Update("slug", slug).Error
		}
	}
	return fmt.Errorf("unable to allocate unique doctor slug")
}

// slugifyDoctorName builds a base slug from doctor display name fields.
func slugifyDoctorName(doctor *models.Doctor) string {
	name := strings.TrimSpace(doctor.Name)
	if name == "" {
		name = strings.TrimSpace(doctor.FirstName + " " + doctor.LastName)
	}
	return slug.Make(name)
}

// ListByClinic returns all doctors stored for a clinic (newest first).
func (r *DoctorRepo) ListByClinic(clinicID uint) ([]models.Doctor, error) {
	var rows []models.Doctor
	err := r.DB.Where("clinic_id = ?", clinicID).
		Preload("Specialty").
		Order("id desc").
		Find(&rows).Error
	return rows, err
}

// ListApprovedByClinic returns approved doctors with an external ID for a clinic.
// Inputs: clinicID.
// Output: approved doctor rows ordered by name.
func (r *DoctorRepo) ListApprovedByClinic(clinicID uint) ([]models.Doctor, error) {
	var rows []models.Doctor
	err := r.DB.Where(
		"clinic_id = ? AND is_approved = ? AND external_id <> ''",
		clinicID, true,
	).
		Preload("Specialty").
		Order("name asc").
		Find(&rows).Error
	return rows, err
}

// DoctorPublicFilter scopes public doctor listing for booking pages.
type DoctorPublicFilter struct {
	ClinicIDs   []uint
	SpecialtyID uint
	Query       string
}

// ListPublic returns approved, active doctors matching the public booking filters.
// Inputs: filter (clinic IDs required; optional specialty and name query).
// Output: doctor rows with Specialty preloaded, ordered by name.
func (r *DoctorRepo) ListPublic(filter DoctorPublicFilter) ([]models.Doctor, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("doctor repo unavailable")
	}
	if len(filter.ClinicIDs) == 0 {
		return nil, nil
	}
	q := r.DB.Where(
		"clinic_id IN ? AND is_approved = ? AND is_active = ? AND external_id <> ''",
		filter.ClinicIDs, true, true,
	)
	if filter.SpecialtyID > 0 {
		q = q.Where("specialty_id = ?", filter.SpecialtyID)
	}
	if name := strings.TrimSpace(filter.Query); name != "" {
		like := "%" + name + "%"
		q = q.Where(
			"(name LIKE ? OR first_name LIKE ? OR last_name LIKE ? OR (first_name + ' ' + last_name) LIKE ?)",
			like, like, like, like,
		)
	}
	var rows []models.Doctor
	err := q.Preload("Specialty").Order("name asc").Find(&rows).Error
	return rows, err
}

// FindPendingApprovalByDoctorID returns the pending approval request for a doctor, if any.
func (r *DoctorRepo) FindPendingApprovalByDoctorID(doctorID uint) (*models.DoctorApprovalRequest, error) {
	var rows []models.DoctorApprovalRequest
	err := r.DB.Where("doctor_id = ? AND status = ?", doctorID, string(constants.ApprovalPending)).
		Order("id DESC").
		Limit(1).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}
