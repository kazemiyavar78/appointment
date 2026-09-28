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

// UnknownBirthDate is stored when a patient is captured at OTP time, before birth date is known.
// Inputs: none. Output: 1900-01-01 UTC, which SQL Server datetime accepts.
func UnknownBirthDate() time.Time {
	return time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
}

// BirthDateKnown reports whether t is a real birth date rather than empty or the OTP placeholder.
// Inputs: birth date loaded from patients. Output: false for zero or year 1900 and earlier.
func BirthDateKnown(t time.Time) bool {
	return !t.IsZero() && t.Year() > 1900
}
