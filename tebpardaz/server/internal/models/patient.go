package models

import (
	"time"

	"gorm.io/gorm"
)

// Patient stores minimal identity used for booking and test-result lookup.
type Patient struct {
	gorm.Model
	NationalID string    `gorm:"type:nvarchar(100);not null;uniqueIndex" json:"national_id"`
	FirstName  string    `gorm:"type:nvarchar(100);not null" json:"first_name"`
	LastName   string    `gorm:"type:nvarchar(100);not null" json:"last_name"`
	Mobile     string    `gorm:"type:nvarchar(100);not null;index" json:"mobile"`
	BirthDate  time.Time `gorm:"type:datetime;not null" json:"birth_date"`
	Sex        Sex       `gorm:"type:nvarchar(100);not null;default:'نامشخص'" json:"sex"`
}

// Sex is the patient sex label stored in Persian for display.
type Sex string

const (
	MALE   Sex = "مرد"
	FEMALE Sex = "زن"
	OTHER  Sex = "نامشخص"
)
