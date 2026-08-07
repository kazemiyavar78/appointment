package models

import (
	"time"

	"gorm.io/gorm"
)

// News is a public announcement shown on clinic/organ sites.
// Title, Excerpt, and Body store sanitized HTML for rich formatting.
type News struct {
	gorm.Model
	ClinicID    uint      `gorm:"not null;index" json:"clinic_id"`
	Title       string    `gorm:"type:nvarchar(max);not null" json:"title"`
	Excerpt     string    `gorm:"type:nvarchar(500);not null" json:"excerpt"`
	CoverURL    string    `gorm:"type:nvarchar(500);not null;default:''" json:"cover_url"`
	Body        string    `gorm:"type:nvarchar(max);not null" json:"body"`
	PublishedAt time.Time `gorm:"type:datetime;not null" json:"published_at"`
	IsPublished bool      `gorm:"type:bit;not null;default:false;index" json:"is_published"`
}
