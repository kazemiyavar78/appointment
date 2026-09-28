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
	// VisitStatus is empty until the clinic admission table is checked.
	// visited means the patient was admitted; absent means the check found no row.
	VisitStatus string `gorm:"type:nvarchar(20);not null;default:'';index" json:"visit_status"`
	// VisitCheckedAt is when the server stored the admission check result.
	VisitCheckedAt *time.Time `gorm:"type:datetime" json:"visit_checked_at"`
	// IPAddress is the booking client IP captured at registration time.
	IPAddress string `gorm:"type:nvarchar(45);not null;default:'';index" json:"ip_address"`

	Patient Patient     `gorm:"foreignKey:PatientID;references:ID"`
	Doctor  Doctor      `gorm:"foreignKey:DoctorID;references:ID"`
	Slot    *DoctorSlot `gorm:"foreignKey:SlotID;references:ID"`
}

const (
	// VisitStatusVisited means paziresh had at least one matching admission.
	VisitStatusVisited = "visited"
	// VisitStatusAbsent means the admission check ran and the count was zero.
	VisitStatusAbsent = "absent"
)
