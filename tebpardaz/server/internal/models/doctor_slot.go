package models

import (
	"time"

	"gorm.io/gorm"
)

// DoctorSlot پنجره زمانی قابل‌رزرو سینک‌شده از کلاینت مرکز است.
// توجه: اسلات‌های زنده دیگر در دیتابیس نوشته نمی‌شوند و فقط در کش (۴ ساعت) نگه داشته می‌شوند.
// مدل برای سازگاری با PatientAppointment و مهاجرت قدیمی باقی مانده است.
type DoctorSlot struct {
	gorm.Model
	DoctorID       uint      `gorm:"not null;index" json:"doctor_id"`
	ClinicID       uint      `gorm:"not null;index" json:"clinic_id"`
	StartsAt       time.Time `gorm:"type:datetime;not null;index" json:"starts_at"`
	EndsAt         time.Time `gorm:"type:datetime;not null" json:"ends_at"`
	Capacity       int       `gorm:"type:int;not null;default:1" json:"capacity"`
	BookedCount    int       `gorm:"type:int;not null;default:0" json:"booked_count"`
	IsAvailable    bool      `gorm:"type:bit;not null;default:true;index" json:"is_available"`
	ExternalSlotID string    `gorm:"type:nvarchar(100);not null;default:'';index" json:"external_slot_id"`

	Doctor Doctor `gorm:"foreignKey:DoctorID;references:ID"`
}
