package repository

import (
	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// InsuranceRepo provides persistence for the insurance catalog and clinic assignments.
type InsuranceRepo struct {
	DB *gorm.DB
}

// NewInsuranceRepo constructs an InsuranceRepo.
// Inputs: db (appointment_tapesh GORM handle).
// Output: pointer to InsuranceRepo.
func NewInsuranceRepo(db *gorm.DB) *InsuranceRepo {
	return &InsuranceRepo{DB: db}
}

// ListAll returns every insurance ordered by name for admin catalog and assignment.
// Inputs: none.
// Output: insurance rows or DB error.
func (r *InsuranceRepo) ListAll() ([]models.Insurance, error) {
	var rows []models.Insurance
	err := r.DB.Order("name asc").Find(&rows).Error
	return rows, err
}

// GetByID loads one insurance by primary key.
// Inputs: id.
// Output: insurance pointer or DB error.
func (r *InsuranceRepo) GetByID(id uint) (*models.Insurance, error) {
	var row models.Insurance
	if err := r.DB.First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// Create inserts a new insurance catalog row.
// Inputs: row to insert.
// Output: DB error, if any.
func (r *InsuranceRepo) Create(row *models.Insurance) error {
	return r.DB.Create(row).Error
}

// Update saves changes to an existing insurance catalog row.
// Inputs: row with ID set.
// Output: DB error, if any.
func (r *InsuranceRepo) Update(row *models.Insurance) error {
	return r.DB.Save(row).Error
}

// Delete removes an insurance and its clinic assignments.
// Inputs: insurance id.
// Output: DB error, if any.
func (r *InsuranceRepo) Delete(id uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("insurance_id = ?", id).Delete(&models.ClinicInsurance{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Insurance{}, id).Error
	})
}

// ListByClinicID returns insurances assigned for display on one clinic site.
// Inputs: clinicID.
// Output: insurance rows ordered by name, or DB error.
func (r *InsuranceRepo) ListByClinicID(clinicID uint) ([]models.Insurance, error) {
	return r.ListByClinicIDs([]uint{clinicID})
}

// ListByClinicIDs returns distinct insurances assigned to any of the given clinics.
// Used for organ/platform home pages that aggregate all subsidiary centers.
// Inputs: clinicIDs (empty yields an empty slice).
// Output: unique insurance rows ordered by name, or DB error.
func (r *InsuranceRepo) ListByClinicIDs(clinicIDs []uint) ([]models.Insurance, error) {
	if len(clinicIDs) == 0 {
		return nil, nil
	}
	var ids []uint
	if err := r.DB.Model(&models.ClinicInsurance{}).
		Where("clinic_id IN ?", clinicIDs).
		Distinct().
		Pluck("insurance_id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []models.Insurance
	err := r.DB.Where("id IN ?", ids).Order("name asc").Find(&rows).Error
	return rows, err
}

// ListIDsByClinicID returns insurance IDs currently assigned to a clinic.
// Inputs: clinicID.
// Output: insurance ID slice or DB error.
func (r *InsuranceRepo) ListIDsByClinicID(clinicID uint) ([]uint, error) {
	var ids []uint
	err := r.DB.Model(&models.ClinicInsurance{}).
		Where("clinic_id = ?", clinicID).
		Pluck("insurance_id", &ids).Error
	return ids, err
}

// ReplaceClinicInsurances replaces the displayed insurance set for one clinic.
// Inputs: clinicID, insuranceIDs (empty clears all assignments).
// Output: DB error, if any.
func (r *InsuranceRepo) ReplaceClinicInsurances(clinicID uint, insuranceIDs []uint) error {
	return r.ReplaceClinicsInsurances([]uint{clinicID}, insuranceIDs)
}

// ReplaceClinicsInsurances بیمه‌های انتصاب داده شده به چند مرکز درمانی را در قالب تراکنش جایگزین می‌کند.
// ورودی: clinicIDs لیست شناسه‌های مراکز درمانی، insuranceIDs لیست شناسه‌های بیمه‌ها.
// خروجی: خطای دیتابیس در صورت وقوع.
func (r *InsuranceRepo) ReplaceClinicsInsurances(clinicIDs []uint, insuranceIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if len(clinicIDs) == 0 {
			return nil
		}
		if err := tx.Where("clinic_id IN ?", clinicIDs).Delete(&models.ClinicInsurance{}).Error; err != nil {
			return err
		}
		if len(insuranceIDs) == 0 {
			return nil
		}
		rows := make([]models.ClinicInsurance, 0, len(clinicIDs)*len(insuranceIDs))
		for _, clinicID := range clinicIDs {
			if clinicID == 0 {
				continue
			}
			for _, id := range insuranceIDs {
				if id == 0 {
					continue
				}
				rows = append(rows, models.ClinicInsurance{ClinicID: clinicID, InsuranceID: id})
			}
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}
