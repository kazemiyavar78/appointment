package models

import (
	"time"

	"gorm.io/gorm"
)

// TestResultCache stores recently fetched LIS results to reduce clinic load.
type TestResultCache struct {
	gorm.Model
	ClinicID    uint      `gorm:"not null;index" json:"clinic_id"`
	NationalID  string    `gorm:"type:nvarchar(20);not null;index" json:"national_id"`
	Barcode     string    `gorm:"type:nvarchar(100);not null;default:'';index" json:"barcode"`
	PayloadJSON string    `gorm:"type:nvarchar(max);not null" json:"payload_json"`
	FetchedAt   time.Time `gorm:"type:datetime;not null" json:"fetched_at"`
	ExpiresAt   time.Time `gorm:"type:datetime;not null;index" json:"expires_at"`

}
