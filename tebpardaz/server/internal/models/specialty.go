package models

import "gorm.io/gorm"

// Specialty classifies doctors for filtering and display.
type Specialty struct {
	gorm.Model
	Name       string `gorm:"type:nvarchar(100);not null" json:"name"`
	NameEN     string `gorm:"type:nvarchar(100);not null;default:''" json:"name_en"`
	ShortDescription string `gorm:"type:nvarchar(100);not null;default:''" json:"short_description"`
	Description string `gorm:"type:nvarchar(max);not null;default:''" json:"description"`
	Icon string `gorm:"type:nvarchar(100);not null;default:''" json:"icon"`
	Color string `gorm:"type:nvarchar(100);not null;default:''" json:"color"`
	IsApproved bool   `gorm:"type:bit;not null;default:false" json:"is_approved"`
}
