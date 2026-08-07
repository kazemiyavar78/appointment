package models

import "gorm.io/gorm"

// Doctor is a physician shown on public booking pages.
type Doctor struct {
	gorm.Model
	ClinicID       uint   `gorm:"not null;index" json:"clinic_id"`
	SpecialtyID    uint   `gorm:"not null;default:0;index" json:"specialty_id"`
	Name           string `gorm:"type:nvarchar(100);not null" json:"name"`
	FirstName      string `gorm:"type:nvarchar(100);not null;default:''" json:"first_name"`
	LastName       string `gorm:"type:nvarchar(100);not null;default:''" json:"last_name"`
	// Slug is the public URL segment for this doctor within a clinic (name-based).
	Slug           string `gorm:"type:nvarchar(120);not null;default:'';index" json:"slug"`
	Mobile         string `gorm:"type:nvarchar(20);not null;default:''" json:"mobile"`
	NationalID     string `gorm:"type:nvarchar(20);not null;default:''" json:"national_id"`
	DoctorSystemID int    `gorm:"type:int;not null;default:0" json:"doctor_system_id"`
	SpecialtyCode  string `gorm:"type:nvarchar(50);not null;default:''" json:"specialty_code"`
	PhotoURL       string `gorm:"type:nvarchar(255);not null;default:''" json:"photo_url"`
	ExternalID     string `gorm:"type:nvarchar(100);not null;default:'';index" json:"external_id"`
	LocalCode      int    `gorm:"type:int;not null;default:0;index" json:"local_code"`
	IsApproved     bool   `gorm:"type:bit;not null;default:false;index" json:"is_approved"`
	IsActive       bool   `gorm:"type:bit;not null;default:true" json:"is_active"`

	Specialty Specialty `gorm:"foreignKey:SpecialtyID;references:ID"`
}
