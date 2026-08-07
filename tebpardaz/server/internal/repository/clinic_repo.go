package repository

import (
	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// ClinicRepo provides persistence for clinics.
type ClinicRepo struct {
	DB *gorm.DB
}

// NewClinicRepo constructs a ClinicRepo.
// Inputs: db (GORM handle; may be nil until connection is wired).
// Output: pointer to ClinicRepo.
func NewClinicRepo(db *gorm.DB) *ClinicRepo {
	return &ClinicRepo{DB: db}
}

// GetByDomain finds a clinic by its custom Domain field.
// Inputs: domain (normalized host without port).
// Output: clinic pointer, gorm.ErrRecordNotFound, or other DB error.
func (r *ClinicRepo) GetByDomain(domain string) (*models.Clinic, error) {
	if r.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var clinic models.Clinic
	err := r.DB.Where("domain = ?", domain).First(&clinic).Error
	if err != nil {
		return nil, err
	}
	return &clinic, nil
}

// GetBySlug finds a clinic by its unique Slug.
// Inputs: slug (subdomain or path segment on the platform base domain).
// Output: clinic pointer or DB error.
func (r *ClinicRepo) GetBySlug(slug string) (*models.Clinic, error) {
	if r.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var clinic models.Clinic
	err := r.DB.Where("slug = ?", slug).First(&clinic).Error
	if err != nil {
		return nil, err
	}
	return &clinic, nil
}

// GetByWSClientKey finds a clinic by its WebSocket client key.
// Inputs: key (Clinic.WSClientKey).
// Output: clinic pointer or DB error.
func (r *ClinicRepo) GetByWSClientKey(key string) (*models.Clinic, error) {
	if r.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var clinic models.Clinic
	err := r.DB.Where("ws_client_key = ?", key).First(&clinic).Error
	if err != nil {
		return nil, err
	}
	return &clinic, nil
}

// GetByID loads a clinic by primary key from tp_managment.
// Inputs: id.
// Output: clinic pointer or DB error.
func (r *ClinicRepo) GetByID(id uint) (*models.Clinic, error) {
	if r.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var clinic models.Clinic
	err := r.DB.First(&clinic, id).Error
	if err != nil {
		return nil, err
	}
	return &clinic, nil
}

// ListAll returns every clinic (superadmin scope).
func (r *ClinicRepo) ListAll() ([]models.Clinic, error) {
	if r.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var clinics []models.Clinic
	err := r.DB.Order("name asc").Find(&clinics).Error
	return clinics, err
}

// ListByOrganizationID returns clinics belonging to one organization (organ admin scope).
func (r *ClinicRepo) ListByOrganizationID(orgID uint) ([]models.Clinic, error) {
	if r.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var clinics []models.Clinic
	err := r.DB.Where("organization_id = ?", orgID).Order("name asc").Find(&clinics).Error
	return clinics, err
}
