package models

import "gorm.io/gorm"

// Organization groups multiple clinics under one organ brand/site.
type Organization struct {
	gorm.Model
	Code        int     `gorm:"type:int;not null;default:0" json:"code"`
	Name        string  `gorm:"type:nvarchar(100);not null" json:"name"`
	Description string  `gorm:"type:nvarchar(255);not null;default:''" json:"description"`
	LogoURL     string  `gorm:"type:nvarchar(255);not null;default:''" json:"logo_url"`
	Status      string  `gorm:"type:nvarchar(10);not null;default:'active'" json:"status"`
	Domain      *string `gorm:"type:nvarchar(100);default:null" json:"domain"`
	Slug        *string `gorm:"type:nvarchar(100);default:null;uniqueIndex" json:"slug"`
}
