package models

import "gorm.io/gorm"

// نوع هدف نظر: پزشک یا مرکز.
const (
	ReviewTargetDoctor = "doctor"
	ReviewTargetClinic = "clinic"
)

// Review نظر و امتیاز کاربر برای پزشک یا مرکز است.
type Review struct {
	gorm.Model
	// TargetType یکی از ReviewTargetDoctor یا ReviewTargetClinic است.
	TargetType string `gorm:"type:nvarchar(20);not null;index" json:"target_type"`
	TargetID   uint   `gorm:"not null;index" json:"target_id"`
	// ClinicID برای محدوده تأیید ادمین مرکز استفاده می‌شود.
	ClinicID   uint   `gorm:"not null;index" json:"clinic_id"`
	AuthorName string `gorm:"type:nvarchar(100);not null;default:''" json:"author_name"`
	// Rating امتیاز ۱ تا ۵ است.
	Rating     int    `gorm:"type:int;not null" json:"rating"`
	Body       string `gorm:"type:nvarchar(1000);not null;default:''" json:"body"`
	// IsApproved پس از تأیید ادمین، متن نظر نمایش داده می‌شود.
	IsApproved bool `gorm:"type:bit;not null;default:false;index" json:"is_approved"`
}
