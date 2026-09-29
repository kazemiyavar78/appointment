package repository

import (
	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// OrganizationRepo provides persistence for organizations.
type OrganizationRepo struct {
	DB *gorm.DB
}

// NewOrganizationRepo constructs an OrganizationRepo.
// Inputs: db (GORM handle; may be nil until connection is wired).
// Output: pointer to OrganizationRepo.
func NewOrganizationRepo(db *gorm.DB) *OrganizationRepo {
	return &OrganizationRepo{DB: db}
}

// GetByDomain finds an organization by its custom Domain.
// Inputs: domain (normalized host without port).
// Output: organization pointer or DB error.
func (r *OrganizationRepo) GetByDomain(domain string) (*models.Organization, error) {
	if r.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var org models.Organization
	err := r.DB.Where("domain = ?", domain).First(&org).Error
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// GetBySlug finds an organization by its unique slug.
// Inputs: slug (path segment on the platform base domain).
// Output: organization pointer or DB error.
func (r *OrganizationRepo) GetBySlug(slug string) (*models.Organization, error) {
	if r.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var org models.Organization
	err := r.DB.Where("slug = ?", slug).First(&org).Error
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// GetByID loads an organization by primary key.
// Inputs: id.
// Output: organization pointer or DB error.
func (r *OrganizationRepo) GetByID(id uint) (*models.Organization, error) {
	if r.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var org models.Organization
	err := r.DB.First(&org, id).Error
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// ListAll returns every organization ordered by name for admin dropdowns.
// Inputs: none (uses repo DB).
// Output: slice of organizations or DB error.
func (r *OrganizationRepo) ListAll() ([]models.Organization, error) {
	if r.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var orgs []models.Organization
	err := r.DB.Order("name asc").Find(&orgs).Error
	return orgs, err
}
