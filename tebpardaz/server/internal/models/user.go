package models

import "gorm.io/gorm"

// AppointmentUser is a back-office account stored in appointment_tapesh.
// Superadmin: ClinicID and OrganizationID are nil (all clinics).
// Clinic admin: ClinicID set.
// Organ admin: OrganizationID set.
type AppointmentUser struct {
	gorm.Model
	ClinicID       *uint  `gorm:"index" json:"clinic_id"`
	OrganizationID *uint  `gorm:"index" json:"organization_id"`
	Username       string `gorm:"type:nvarchar(100);not null;uniqueIndex" json:"username"`
	PasswordHash   string `gorm:"type:nvarchar(255);not null" json:"password_hash"`
	Role           string `gorm:"type:nvarchar(100);not null" json:"role"`
	IsActive       bool   `gorm:"type:bit;not null;default:true" json:"is_active"`
}

// TableName returns the SQL table name for AppointmentUser.
func (AppointmentUser) TableName() string {
	return "appointment_users"
}
