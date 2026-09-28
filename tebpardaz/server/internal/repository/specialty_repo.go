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

// ListAll تمام تخصص‌ها را برای ادمین و انتساب پزشک برمی‌گرداند.
// ورودی: ندارد. خروجی: ردیف‌ها به ترتیب نمایش سپس شناسه.
func (r *SpecialtyRepo) ListAll() ([]models.Specialty, error) {
	var rows []models.Specialty
	err := r.DB.Order("sort_order asc, id asc").Find(&rows).Error
	return rows, err
}

// ListApproved تخصص‌های تأییدشده و قابل‌نمایش در نوبت‌دهی را برمی‌گرداند.
// ورودی: ندارد. خروجی: ردیف‌های عمومی به ترتیب نمایش سپس شناسه.
func (r *SpecialtyRepo) ListApproved() ([]models.Specialty, error) {
	var rows []models.Specialty
	err := r.DB.Where("is_approved = ? AND show_in_booking = ?", true, true).
		Order("sort_order asc, id asc").
		Find(&rows).Error
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
