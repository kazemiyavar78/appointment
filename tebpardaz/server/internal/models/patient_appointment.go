package models

import (
	"time"

	"gorm.io/gorm"
)

// PatientAppointment links a patient to a reserved doctor time on the central site.
type PatientAppointment struct {
	gorm.Model
	ClinicID       uint       `gorm:"not null;index" json:"clinic_id"`
	PatientID      uint       `gorm:"not null;index" json:"patient_id"`
	DoctorID       uint       `gorm:"not null;index" json:"doctor_id"`
	SlotID         *uint      `gorm:"default:null" json:"slot_id"`
	Status         string     `gorm:"type:nvarchar(20);not null;default:'pending';index" json:"status"`
	ExternalID     string     `gorm:"type:nvarchar(100);not null;default:''" json:"external_id"`
	IdempotencyKey string     `gorm:"type:nvarchar(100);not null;uniqueIndex" json:"idempotency_key"`
	StartsAt       time.Time  `gorm:"type:datetime;not null" json:"starts_at"`
	EndsAt         time.Time  `gorm:"type:datetime;not null" json:"ends_at"`
	Source         string     `gorm:"type:nvarchar(40);not null;default:'website'" json:"source"`
	FailReason     string     `gorm:"type:nvarchar(255);not null;default:''" json:"fail_reason"`
	ConfirmedAt    *time.Time `gorm:"type:datetime;default:null" json:"confirmed_at"`

	Patient Patient     `gorm:"foreignKey:PatientID;references:ID"`
	Doctor  Doctor      `gorm:"foreignKey:DoctorID;references:ID"`
	Slot    *DoctorSlot `gorm:"foreignKey:SlotID;references:ID"`
}
