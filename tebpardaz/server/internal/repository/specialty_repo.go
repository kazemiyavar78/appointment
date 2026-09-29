package repository

import (
	"fmt"
	"strings"
	"time"

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

// SpecialtyPublicCount تخصص عمومی به‌همراه تعداد پزشک قابل‌نمایش است.
type SpecialtyPublicCount struct {
	ID          uint
	Name        string
	Slug        string
	UpdatedAt   time.Time
	DoctorCount int
}

// ListWithPublicDoctors تخصص‌های قابل‌نوبت را که حداقل یک پزشک عمومی دارند برمی‌گرداند.
// ورودی: شناسه مراکز مجاز. خروجی: نام، slug و تعداد. نام پزشک با متن مقایسه نمی‌شود.
func (r *SpecialtyRepo) ListWithPublicDoctors(clinicIDs []uint) ([]SpecialtyPublicCount, error) {
	if r == nil || r.DB == nil || len(clinicIDs) == 0 {
		return nil, nil
	}
	var rows []SpecialtyPublicCount
	err := r.DB.Model(&models.Specialty{}).
		Select("specialties.id AS id, specialties.name AS name, specialties.slug AS slug, specialties.updated_at AS updated_at, COUNT(doctors.id) AS doctor_count").
		Joins("INNER JOIN doctors ON doctors.specialty_id = specialties.id AND doctors.deleted_at IS NULL AND doctors.is_approved = ? AND doctors.is_active = ? AND doctors.external_id <> '' AND doctors.clinic_id IN ?", true, true, clinicIDs).
		Where("specialties.is_approved = ? AND specialties.show_in_booking = ?", true, true).
		Group("specialties.id, specialties.name, specialties.slug, specialties.updated_at, specialties.sort_order").
		Having("COUNT(doctors.id) > 0").
		Order("specialties.sort_order asc, specialties.id asc").
		Scan(&rows).Error
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

// Create تخصص را با اسلاگ نهایی و پایدار ذخیره می‌کند.
// ورودی: ردیف با Name نرمال‌شده. خروجی: خطا. slug قبل از commit معتبر است و placeholder مشترک ندارد.
func (r *SpecialtyRepo) Create(row *models.Specialty) error {
	if r == nil || r.DB == nil || row == nil {
		return fmt.Errorf("specialty repo unavailable")
	}
	if strings.TrimSpace(row.Slug) != "" {
		if err := validateSpecialtySlug(row.Slug); err != nil {
			return err
		}
		return r.DB.Transaction(func(tx *gorm.DB) error {
			if err := lockSpecialtyCreates(tx); err != nil {
				return err
			}
			return tx.Create(row).Error
		})
	}
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSpecialtyCreates(tx); err != nil {
			return err
		}
		var rows []models.Specialty
		if err := tx.Unscoped().Find(&rows).Error; err != nil {
			return err
		}
		chosen, explicitID, err := decideSpecialtyCreateSlug(row.Name, rows, nextSpecialtyID(rows))
		if err != nil {
			return err
		}
		row.Slug = chosen
		if explicitID == 0 {
			return tx.Create(row).Error
		}
		row.ID = explicitID
		return insertSpecialtyWithID(tx, row)
	})
}

// Update نام تخصص را ذخیره می‌کند و اسلاگ موجود را نگه می‌دارد.
// ورودی: ردیف بارگذاری‌شده. خروجی: خطا. اگر اسلاگ خالی باشد فقط همان‌جا ساخته می‌شود.
func (r *SpecialtyRepo) Update(row *models.Specialty) error {
	if r == nil || r.DB == nil || row == nil {
		return fmt.Errorf("specialty repo unavailable")
	}
	if kept, ok := specialtySlugOnUpdate(row.Slug); ok {
		row.Slug = kept
		return r.DB.Save(row).Error
	}
	var rows []models.Specialty
	if err := r.DB.Unscoped().Find(&rows).Error; err != nil {
		return err
	}
	others := make([]models.Specialty, 0, len(rows))
	for _, existing := range rows {
		if existing.ID == row.ID {
			continue
		}
		others = append(others, existing)
	}
	plan, err := PlanSpecialtySlugs(append(others, *row))
	if err != nil {
		return err
	}
	row.Slug = plan[row.ID]
	return r.DB.Save(row).Error
}

// Delete removes a specialty by id.
func (r *SpecialtyRepo) Delete(id uint) error {
	return r.DB.Delete(&models.Specialty{}, id).Error
}
