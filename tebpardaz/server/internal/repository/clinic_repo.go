package repository

import (
	"tebpardaz/server/internal/models"
	"time"

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
	err := r.DB.Preload("City").Where("domain = ? AND is_active_on_website = ?", domain, true).First(&clinic).Error
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
	err := r.DB.Preload("City").Where("slug = ? AND is_active_on_website = ?", slug, true).First(&clinic).Error
	if err != nil {
		return nil, err
	}
	return &clinic, nil
}

// GetByWSClientKey finds a clinic by its WebSocket client key.
// Inputs: key (Clinic.WSClientKey).
// Output: clinic pointer or DB error.
// update LastSyncAt now
func (r *ClinicRepo) GetByWSClientKey(key string) (*models.Clinic, error) {
	if r.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var clinic models.Clinic
	err := r.DB.Where("ws_client_key = ? AND is_active_on_website = ?", key, true).First(&clinic).Error
	if err != nil {
		return nil, err
	}

	// update online clinics
	r.DB.Model(&clinic).Update("last_sync_at", time.Now())
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
	err := r.DB.Where("is_active_on_website = ?", true).First(&clinic, id).Error
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
	err := r.DB.Preload("City").Where("is_active_on_website = ?", true).Order("name asc").Find(&clinics).Error
	return clinics, err
}

// ListByOrganizationID returns clinics belonging to one organization (organ admin scope).
func (r *ClinicRepo) ListByOrganizationID(orgID uint) ([]models.Clinic, error) {
	if r.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var clinics []models.Clinic
	err := r.DB.Preload("City").Where("organization_id = ? AND is_active_on_website = ?", orgID, true).Order("name asc").Find(&clinics).Error
	return clinics, err
}

// ListAllForAdmin returns all clinics for user-assignment dropdowns (no website filter).
// Inputs: none (uses repo DB).
// Output: slice of clinics or DB error.
func (r *ClinicRepo) ListAllForAdmin() ([]models.Clinic, error) {
	if r.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var clinics []models.Clinic
	err := r.DB.Where("is_active_on_website = ?", true).Order("name asc").Find(&clinics).Error
	return clinics, err
}

// GetByIDForAdmin loads a clinic by id without requiring is_active_on_website.
// Inputs: id.
// Output: clinic pointer or DB error.
func (r *ClinicRepo) GetByIDForAdmin(id uint) (*models.Clinic, error) {
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
