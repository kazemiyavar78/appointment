package models

import "gorm.io/gorm"

// Specialty classifies doctors for filtering and display.
type Specialty struct {
	gorm.Model
	Name       string `gorm:"type:nvarchar(100);not null" json:"name"`
	NameEN     string `gorm:"type:nvarchar(100);not null;default:''" json:"name_en"`
	IsApproved bool   `gorm:"type:bit;not null;default:false" json:"is_approved"`
}
