package models

import "gorm.io/gorm"

// City is a geographic lookup used for clinic listing filters.
type City struct {
	gorm.Model
	Name     string `gorm:"type:nvarchar(100);not null" json:"name"`
	Province string `gorm:"type:nvarchar(100);not null;default:''" json:"province"`
}
