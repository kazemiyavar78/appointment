package repository

import (
	"tebpardaz/server/internal/models"
	"tebpardaz/shared/constants"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserRepo provides persistence for AppointmentUser accounts.
type UserRepo struct {
	DB *gorm.DB
}

// NewUserRepo constructs a UserRepo.
// Inputs: db (appointment_tapesh GORM handle).
// Output: pointer to UserRepo.
func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{DB: db}
}

// FindByUsername loads an active-or-inactive user by unique username.
// Inputs: username.
// Output: user pointer or gorm.ErrRecordNotFound / DB error.
func (r *UserRepo) FindByUsername(username string) (*models.AppointmentUser, error) {
	var user models.AppointmentUser
	err := r.DB.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID loads a user by primary key.
// Inputs: id (AppointmentUser.ID).
// Output: user pointer or DB error.
func (r *UserRepo) FindByID(id uint) (*models.AppointmentUser, error) {
	var user models.AppointmentUser
	err := r.DB.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// EnsureSuperAdmin creates the platform superadmin if missing.
// Inputs: username, plainPassword (hashed with bcrypt before insert).
// Output: error from bcrypt or DB; nil when user already exists or is created.
func (r *UserRepo) EnsureSuperAdmin(username, plainPassword string) error {
	if r.DB == nil {
		return gorm.ErrInvalidDB
	}
	var count int64
	if err := r.DB.Model(&models.AppointmentUser{}).
		Where("username = ?", username).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user := models.AppointmentUser{
		Username:     username,
		PasswordHash: string(hash),
		Role:         string(constants.UserRoleSuperAdmin),
		IsActive:     true,
		ClinicID:     nil,
	}
	return r.DB.Create(&user).Error
}
