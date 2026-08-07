package repository

import (
	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// SpecialtyRepo provides persistence for doctor specialties.
type SpecialtyRepo struct {
	DB *gorm.DB
}

// NewSpecialtyRepo constructs a SpecialtyRepo.
func NewSpecialtyRepo(db *gorm.DB) *SpecialtyRepo {
	return &SpecialtyRepo{DB: db}
}

// ListAll returns all specialties ordered by name for admin assignment.
func (r *SpecialtyRepo) ListAll() ([]models.Specialty, error) {
	var rows []models.Specialty
	err := r.DB.Order("name asc").Find(&rows).Error
	return rows, err
}

// ListApproved returns approved specialties for public booking filters.
// Inputs: none.
// Output: specialty rows ordered by name.
func (r *SpecialtyRepo) ListApproved() ([]models.Specialty, error) {
	var rows []models.Specialty
	err := r.DB.Where("is_approved = ?", true).Order("name asc").Find(&rows).Error
	return rows, err
}

// GetByID loads one specialty by primary key.
func (r *SpecialtyRepo) GetByID(id uint) (*models.Specialty, error) {
	var row models.Specialty
	if err := r.DB.First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// Create inserts a new specialty row.
func (r *SpecialtyRepo) Create(row *models.Specialty) error {
	return r.DB.Create(row).Error
}

// Update saves changes to an existing specialty.
func (r *SpecialtyRepo) Update(row *models.Specialty) error {
	return r.DB.Save(row).Error
}

// Delete removes a specialty by id.
func (r *SpecialtyRepo) Delete(id uint) error {
	return r.DB.Delete(&models.Specialty{}, id).Error
}
