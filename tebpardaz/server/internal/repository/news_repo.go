package repository

import (
	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// NewsRepo provides persistence for news articles.
type NewsRepo struct {
	DB *gorm.DB
}

// NewNewsRepo constructs a NewsRepo.
// Inputs: db (appointment_tapesh GORM handle).
// Output: pointer to NewsRepo.
func NewNewsRepo(db *gorm.DB) *NewsRepo {
	return &NewsRepo{DB: db}
}

// ListByClinicIDs returns all news for the given clinics (admin list, newest first).
// Inputs: clinicIDs (empty returns empty slice).
// Output: rows or DB error.
func (r *NewsRepo) ListByClinicIDs(clinicIDs []uint) ([]models.News, error) {
	if r.DB == nil || len(clinicIDs) == 0 {
		return nil, nil
	}
	var rows []models.News
	err := r.DB.Where("clinic_id IN ?", clinicIDs).
		Order("published_at desc, id desc").
		Find(&rows).Error
	return rows, err
}

// ListPublishedByClinicIDs returns published news for the given clinics.
// Inputs: clinicIDs, limit (0 = no limit).
// Output: rows or DB error.
func (r *NewsRepo) ListPublishedByClinicIDs(clinicIDs []uint, limit int) ([]models.News, error) {
	if r.DB == nil || len(clinicIDs) == 0 {
		return nil, nil
	}
	q := r.DB.Where("clinic_id IN ? AND is_published = ?", clinicIDs, true).
		Order("published_at desc, id desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var rows []models.News
	err := q.Find(&rows).Error
	return rows, err
}

// GetByID loads a news row by primary key.
// Inputs: id.
// Output: news pointer or DB error.
func (r *NewsRepo) GetByID(id uint) (*models.News, error) {
	if r.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var row models.News
	err := r.DB.First(&row, id).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Create inserts a news row.
// Inputs: row pointer to persist.
// Output: DB error.
func (r *NewsRepo) Create(row *models.News) error {
	if r.DB == nil {
		return gorm.ErrInvalidDB
	}
	return r.DB.Create(row).Error
}

// Update saves an existing news row.
// Inputs: row with ID set.
// Output: DB error.
func (r *NewsRepo) Update(row *models.News) error {
	if r.DB == nil {
		return gorm.ErrInvalidDB
	}
	return r.DB.Save(row).Error
}

// Delete soft-deletes a news row by id.
// Inputs: id.
// Output: DB error.
func (r *NewsRepo) Delete(id uint) error {
	if r.DB == nil {
		return gorm.ErrInvalidDB
	}
	return r.DB.Delete(&models.News{}, id).Error
}
