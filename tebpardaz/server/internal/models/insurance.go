package models

import "gorm.io/gorm"

// Insurance is a platform-wide insurance catalog entry (name, logo, short description).
// Superadmin creates/edits/deletes rows; clinics only choose which ones to display.
type Insurance struct {
	gorm.Model
	Name        string `gorm:"type:nvarchar(100);not null" json:"name"`
	LogoURL     string `gorm:"type:nvarchar(500);not null;default:''" json:"logo_url"`
	Description string `gorm:"type:nvarchar(120);not null;default:''" json:"description"`
}

// ClinicInsurance links a clinic (tp_managment) to a displayed insurance (appointment_tapesh).
// ClinicID is stored without a cross-database foreign key.
type ClinicInsurance struct {
	ClinicID    uint `gorm:"primaryKey;not null" json:"clinic_id"`
	InsuranceID uint `gorm:"primaryKey;not null;index" json:"insurance_id"`
}

// TableName returns the SQL table name for ClinicInsurance.
func (ClinicInsurance) TableName() string {
	return "clinic_insurances"
}
