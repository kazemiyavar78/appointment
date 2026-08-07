package models

import (
	"time"

	"gorm.io/gorm"
)

// DoctorApprovalRequest tracks pending doctor profile approvals for admin review.
type DoctorApprovalRequest struct {
	gorm.Model
	ClinicID      uint       `gorm:"not null;index" json:"clinic_id"`
	DoctorID      *uint      `gorm:"default:null" json:"doctor_id"`
	ExternalID    string     `gorm:"type:nvarchar(100);not null;default:'';index" json:"external_id"`
	RequestedName string     `gorm:"type:nvarchar(100);not null" json:"requested_name"`
	SpecialtyName string     `gorm:"type:nvarchar(100);not null;default:''" json:"specialty_name"`
	Status        string     `gorm:"type:nvarchar(20);not null;default:'pending';index" json:"status"`
	PayloadJSON   string     `gorm:"type:nvarchar(max);not null;default:''" json:"payload_json"`
	ReviewedBy    *uint      `gorm:"default:null" json:"reviewed_by"`
	ReviewedAt    *time.Time `gorm:"type:datetime;default:null" json:"reviewed_at"`
	Note          string     `gorm:"type:nvarchar(255);not null;default:''" json:"note"`

	Doctor *Doctor `gorm:"foreignKey:DoctorID;references:ID"`
}
