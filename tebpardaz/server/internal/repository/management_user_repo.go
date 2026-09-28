package repository

import (
	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// ManagementUserRepo reads clinic-backend users from tp_managment.
type ManagementUserRepo struct {
	DB *gorm.DB
}

// NewManagementUserRepo constructs a ManagementUserRepo.
// Input: db — اتصال tp_managment (ممکن است nil باشد).
// Output: اشاره‌گر ریپو.
func NewManagementUserRepo(db *gorm.DB) *ManagementUserRepo {
	return &ManagementUserRepo{DB: db}
}

// GetActiveByRole returns active management users with the given role.
// Input: role — نقش هدف (مثلاً super_admin).
// Output: کاربران فعال یا خطای کوئری.
func (r *ManagementUserRepo) GetActiveByRole(role string) ([]models.ManagementUser, error) {
	if r == nil || r.DB == nil || role == "" {
		return []models.ManagementUser{}, nil
	}
	var users []models.ManagementUser
	err := r.DB.Where("role = ? AND active = ?", role, true).Find(&users).Error
	return users, err
}

// GetByClinicAndAppointmentStatus returns active clinic users who opted into appointment-site status SMS.
// Input: clinicID — شناسه کلینیک در tp_managment.
// Output: کاربران یا خطای کوئری.
func (r *ManagementUserRepo) GetByClinicAndAppointmentStatus(clinicID uint) ([]models.ManagementUser, error) {
	if r == nil || r.DB == nil || clinicID == 0 {
		return []models.ManagementUser{}, nil
	}
	var users []models.ManagementUser
	err := r.DB.Where("clinic_id = ? AND send_appointment_status = ? AND active = ?", clinicID, true, true).Find(&users).Error
	return users, err
}
