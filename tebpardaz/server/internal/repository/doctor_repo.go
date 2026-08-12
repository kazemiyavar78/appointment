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

// SyncFromClinic updates approved doctors by clinic_id + national_id and reports which DTOs remain pending.
// Unapproved doctors are never written to the database; callers must store them in cache.
// Inputs: clinicID, doctors from the clinic push.
// Output: results for ack, pending DTOs (not in DB as approved), and error.
func (r *DoctorRepo) SyncFromClinic(clinicID uint, doctors []protocol.DoctorDTO) (results []UpsertResult, pending []protocol.DoctorDTO, err error) {
	results = make([]UpsertResult, 0, len(doctors))
	pending = make([]protocol.DoctorDTO, 0)
	for _, dto := range doctors {
		res, isPending, syncErr := r.syncOne(clinicID, dto)
		if syncErr != nil {
			return results, pending, syncErr
		}
		if isPending {
			pending = append(pending, dto)
			results = append(results, UpsertResult{
				LocalCode:  dto.LocalCode,
				ExternalID: "",
				DoctorID:   0,
				Pending:    true,
			})
			continue
		}
		results = append(results, res)
	}
	return results, pending, nil
}

// syncOne updates an approved doctor matched by national_id (+ clinic) or returns pending=true.
// Inputs: clinicID, dto.
// Output: UpsertResult when updated, pending flag when not in DB as approved, error on failure.
func (r *DoctorRepo) syncOne(clinicID uint, dto protocol.DoctorDTO) (UpsertResult, bool, error) {
	doctor, found, err := r.findApproved(clinicID, dto)
	if err != nil {
		return UpsertResult{}, false, err
	}
	if !found {
		// پزشک تأییدنشده نباید در DB بماند؛ ردیف‌های قدیمی حذف می‌شوند
		_ = r.deleteUnapprovedMatch(clinicID, dto)
		return UpsertResult{}, true, nil
	}

	r.applyHISFields(&doctor, dto)
	if err := r.DB.Save(&doctor).Error; err != nil {
		return UpsertResult{}, false, err
	}
	return UpsertResult{
		LocalCode:  dto.LocalCode,
		ExternalID: doctor.ExternalID,
		DoctorID:   doctor.ID,
		Pending:    false,
	}, false, nil
}

// deleteUnapprovedMatch removes leftover unapproved DB rows matching national_id or local_code.
// Inputs: clinicID, dto.
// Output: error from delete (ignored by callers for sync continuity).
func (r *DoctorRepo) deleteUnapprovedMatch(clinicID uint, dto protocol.DoctorDTO) error {
	q := r.DB.Where("clinic_id = ? AND is_approved = ?", clinicID, false)
	nid := strings.TrimSpace(dto.NationalID)
	switch {
	case nid != "":
		q = q.Where("national_id = ?", nid)
	case dto.LocalCode > 0:
		q = q.Where("local_code = ?", dto.LocalCode)
	default:
		return nil
	}
	return q.Delete(&models.Doctor{}).Error
}

// findApproved locates an approved doctor by clinic_id + national_id (primary), then external_id.
// Inputs: clinicID, dto.
// Output: doctor, found flag, error.
func (r *DoctorRepo) findApproved(clinicID uint, dto protocol.DoctorDTO) (models.Doctor, bool, error) {
	var doctor models.Doctor
	nid := strings.TrimSpace(dto.NationalID)
	if nid != "" {
		tx := r.DB.Where(
			"clinic_id = ? AND national_id = ? AND is_approved = ? AND external_id <> ''",
			clinicID, nid, true,
		).Limit(1).Find(&doctor)
		if tx.Error != nil {
			return doctor, false, tx.Error
		}
		if tx.RowsAffected > 0 {
			return doctor, true, nil
		}
	}
	if dto.ExternalID != "" {
		tx := r.DB.Where(
			"clinic_id = ? AND external_id = ? AND is_approved = ?",
			clinicID, dto.ExternalID, true,
		).Limit(1).Find(&doctor)
		if tx.Error != nil {
			return doctor, false, tx.Error
		}
		if tx.RowsAffected > 0 {
			return doctor, true, nil
		}
	}
	return doctor, false, nil
}

// FindExistingPublic locates an approved doctor by national_id or external_id within a clinic.
// Inputs: clinicID, dto.
// Output: doctor, found flag, error.
func (r *DoctorRepo) FindExistingPublic(clinicID uint, dto protocol.DoctorDTO) (models.Doctor, bool, error) {
	return r.findApproved(clinicID, dto)
}

// applyHISFields copies HIS-sourced fields onto an approved doctor without overwriting admin specialty/photo.
// Inputs: doctor pointer, dto from clinic.
// Output: none (mutates doctor).
func (r *DoctorRepo) applyHISFields(doctor *models.Doctor, dto protocol.DoctorDTO) {
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
	if dto.LocalCode > 0 {
		doctor.LocalCode = dto.LocalCode
	}
	doctor.IsActive = dto.IsActive
}

// ApproveInput holds admin-assigned fields required to persist a doctor for the first time.
type ApproveInput struct {
	ClinicID       uint
	LocalCode      int
	NationalID     string
	FirstName      string
	LastName       string
	Name           string
	Mobile         string
	DoctorSystemID int
	SpecialtyCode  string
	SpecialtyID    uint
	PhotoURL       string
	IsActive       bool
	ReviewerID     uint
	Note           string
}

// CreateApproved persists a newly approved doctor (photo + specialty required) and an audit approval row.
// Inputs: ApproveInput with SpecialtyID and PhotoURL set.
// Output: created DoctorApprovalRequest (with ExternalID) or error.
func (r *DoctorRepo) CreateApproved(in ApproveInput) (*models.DoctorApprovalRequest, error) {
	if strings.TrimSpace(in.PhotoURL) == "" {
		return nil, fmt.Errorf("photo_url required")
	}
	if in.SpecialtyID == 0 {
		return nil, fmt.Errorf("specialty_id required")
	}
	nid := strings.TrimSpace(in.NationalID)
	if nid == "" {
		return nil, fmt.Errorf("national_id required")
	}

	// جلوگیری از تکرار پزشک تأییدشده با همان کد ملی در مرکز
	var existing models.Doctor
	tx := r.DB.Where("clinic_id = ? AND national_id = ? AND is_approved = ?", in.ClinicID, nid, true).
		Limit(1).Find(&existing)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected > 0 {
		return nil, fmt.Errorf("doctor already approved")
	}

	externalID := uuid.NewString()
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = strings.TrimSpace(in.FirstName + " " + in.LastName)
	}
	doctor := models.Doctor{
		ClinicID:       in.ClinicID,
		SpecialtyID:    in.SpecialtyID,
		Name:           name,
		FirstName:      in.FirstName,
		LastName:       in.LastName,
		Mobile:         in.Mobile,
		NationalID:     nid,
		DoctorSystemID: in.DoctorSystemID,
		SpecialtyCode:  in.SpecialtyCode,
		PhotoURL:       in.PhotoURL,
		ExternalID:     externalID,
		LocalCode:      in.LocalCode,
		IsApproved:     true,
		IsActive:       in.IsActive,
	}
	if err := r.DB.Create(&doctor).Error; err != nil {
		return nil, err
	}
	if err := r.EnsureSlug(&doctor); err != nil {
		return nil, err
	}

	payload, _ := json.Marshal(in)
	now := time.Now()
	req := models.DoctorApprovalRequest{
		ClinicID:      in.ClinicID,
		DoctorID:      &doctor.ID,
		ExternalID:    externalID,
		RequestedName: name,
		Status:        string(constants.ApprovalApproved),
		PayloadJSON:   string(payload),
		ReviewedBy:    &in.ReviewerID,
		ReviewedAt:    &now,
		Note:          in.Note,
	}
	if err := r.DB.Create(&req).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

// UpdateApprovedProfile updates photo and/or specialty for an already-approved doctor.
// Inputs: doctorID, specialtyID (0 = leave), photoURL (empty = leave).
// Output: updated doctor or error.
func (r *DoctorRepo) UpdateApprovedProfile(doctorID, specialtyID uint, photoURL string) (*models.Doctor, error) {
	doc, err := r.GetByID(doctorID)
	if err != nil {
		return nil, err
	}
	if doc == nil || !doc.IsApproved {
		return nil, fmt.Errorf("doctor not approved")
	}
	updates := map[string]any{}
	if specialtyID > 0 {
		updates["specialty_id"] = specialtyID
	}
	if strings.TrimSpace(photoURL) != "" {
		updates["photo_url"] = strings.TrimSpace(photoURL)
	}
	if len(updates) == 0 {
		return doc, nil
	}
	if err := r.DB.Model(doc).Updates(updates).Error; err != nil {
		return nil, err
	}
	return r.GetByID(doctorID)
}

// GetByID loads a doctor by primary key.
// Inputs: id.
// Output: doctor with Specialty preloaded, or error.
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
	slugVal := base
	for i := 0; i < 100; i++ {
		if i > 0 {
			slugVal = fmt.Sprintf("%s-%d", base, i+1)
		}
		var count int64
		err := r.DB.Model(&models.Doctor{}).
			Where("clinic_id = ? AND slug = ? AND id <> ?", doctor.ClinicID, slugVal, doctor.ID).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count == 0 {
			doctor.Slug = slugVal
			return r.DB.Model(doctor).Update("slug", slugVal).Error
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
// Inputs: clinicID.
// Output: doctor rows or error.
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
