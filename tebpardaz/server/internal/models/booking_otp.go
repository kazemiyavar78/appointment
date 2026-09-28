package models

import (
	"time"

	"gorm.io/gorm"
)

// BookingOTP is the persisted booking OTP challenge for one mobile number.
// The code itself is stored only as a SHA-256 hash. Identity fields stay in clear text
// so staff can follow up when the patient received a code but never booked.
type BookingOTP struct {
	gorm.Model
	ClinicID   uint       `gorm:"not null;index" json:"clinic_id"`
	PatientID  uint       `gorm:"not null;default:0;index" json:"patient_id"`
	Mobile     string     `gorm:"type:nvarchar(20);not null;uniqueIndex" json:"mobile"`
	NationalID string     `gorm:"type:nvarchar(20);not null;default:'';index" json:"national_id"`
	FirstName  string     `gorm:"type:nvarchar(100);not null;default:''" json:"first_name"`
	LastName   string     `gorm:"type:nvarchar(100);not null;default:''" json:"last_name"`
	CodeHash   string     `gorm:"type:nvarchar(128);not null;default:''" json:"-"`
	ExpiresAt  time.Time  `gorm:"type:datetime;not null" json:"expires_at"`
	Attempts   int        `gorm:"not null;default:0" json:"attempts"`
	LastSentAt time.Time  `gorm:"type:datetime;not null;index" json:"last_sent_at"`
	SendCount  int        `gorm:"not null;default:0" json:"send_count"`
	SendWindow time.Time  `gorm:"type:datetime;not null" json:"send_window"`
	IPAddress  string     `gorm:"type:nvarchar(45);not null;default:''" json:"ip_address"`
	SMSSentAt  *time.Time `gorm:"type:datetime" json:"sms_sent_at"`
	VerifiedAt *time.Time `gorm:"type:datetime" json:"verified_at"`
	BookedAt   *time.Time `gorm:"type:datetime" json:"booked_at"`
}

// TableName fixes the SQL Server table used for booking OTP challenges.
// Inputs: none. Output: table name booking_otps.
func (BookingOTP) TableName() string {
	return "booking_otps"
}
