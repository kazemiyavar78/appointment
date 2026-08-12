package repository

import (
	"errors"
	"strings"

	"tebpardaz/server/internal/models"
	"tebpardaz/shared/constants"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ErrUsernameTaken is returned when Create/Update would violate unique username.
var ErrUsernameTaken = errors.New("username already taken")

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

// ListAll returns every appointment user ordered by id ascending.
// Inputs: none (uses repo DB).
// Output: slice of users or DB error.
func (r *UserRepo) ListAll() ([]models.AppointmentUser, error) {
	if r.DB == nil {
		return nil, gorm.ErrInvalidDB
	}
	var users []models.AppointmentUser
	err := r.DB.Order("id asc").Find(&users).Error
	return users, err
}

// Create inserts a new user after hashing plainPassword with bcrypt.
// Inputs: user fields (Username, Role, ClinicID, OrganizationID, IsActive), plainPassword.
// Output: error from bcrypt, uniqueness check, or DB insert.
func (r *UserRepo) Create(user *models.AppointmentUser, plainPassword string) error {
	if r.DB == nil {
		return gorm.ErrInvalidDB
	}
	hash, err := hashPassword(plainPassword)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	if err := r.DB.Create(user).Error; err != nil {
		if isUniqueViolation(err) {
			return ErrUsernameTaken
		}
		return err
	}
	return nil
}

// Update saves mutable fields on an existing user.
// Inputs: user with ID set; plainPassword empty means keep existing hash.
// Output: error from bcrypt, uniqueness, or DB update.
func (r *UserRepo) Update(user *models.AppointmentUser, plainPassword string) error {
	if r.DB == nil {
		return gorm.ErrInvalidDB
	}
	fields := []string{"Username", "Role", "ClinicID", "OrganizationID", "IsActive"}
	if plainPassword != "" {
		hash, err := hashPassword(plainPassword)
		if err != nil {
			return err
		}
		user.PasswordHash = hash
		fields = append(fields, "PasswordHash")
	}
	err := r.DB.Model(user).Select(fields).Updates(user).Error
	if err != nil {
		if isUniqueViolation(err) {
			return ErrUsernameTaken
		}
		return err
	}
	// Explicitly clear nullable FKs when role no longer needs them (Updates skips nil pointers).
	return r.DB.Model(user).Updates(map[string]interface{}{
		"clinic_id":       user.ClinicID,
		"organization_id": user.OrganizationID,
	}).Error
}

// SetActive sets IsActive for the given user id.
// Inputs: id, active flag.
// Output: DB error if any.
func (r *UserRepo) SetActive(id uint, active bool) error {
	if r.DB == nil {
		return gorm.ErrInvalidDB
	}
	return r.DB.Model(&models.AppointmentUser{}).Where("id = ?", id).Update("is_active", active).Error
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
	hash, err := hashPassword(plainPassword)
	if err != nil {
		return err
	}
	user := models.AppointmentUser{
		Username:     username,
		PasswordHash: hash,
		Role:         string(constants.UserRoleSuperAdmin),
		IsActive:     true,
		ClinicID:     nil,
	}
	return r.DB.Create(&user).Error
}

// hashPassword returns a bcrypt hash for plainPassword.
// Inputs: plainPassword string.
// Output: hash string or bcrypt error.
func hashPassword(plainPassword string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// isUniqueViolation reports whether err looks like a unique index conflict.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}
