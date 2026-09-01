package models

import "gorm.io/gorm"

// Service مدل خدمت در کاتالوگ پلتفرم است که توسط سوپرادمین مدیریت می‌شود.
type Service struct {
	gorm.Model
	// Name نام خدمت است
	Name string `gorm:"type:nvarchar(150);not null" json:"name"`
	// Description توضیحات خدمت است
	Description string `gorm:"type:nvarchar(1000);not null;default:''" json:"description"`
}

// TableName نام جدول دیتابیس را برای مدل Service برمی‌گرداند.
// ورودی: ندارد (گیرنده متد). خروجی: نام جدول در دیتابیس (services).
func (Service) TableName() string {
	return "services"
}

// ClinicInsuranceService جدول واسط انتصاب خدمت به بیمه‌های انتخابی یک مرکز درمانی است.
// مرکز -> بیمه -> خدمات انتصاب داده شده
type ClinicInsuranceService struct {
	ClinicID    uint `gorm:"primaryKey;not null" json:"clinic_id"`
	InsuranceID uint `gorm:"primaryKey;not null;index" json:"insurance_id"`
	ServiceID   uint `gorm:"primaryKey;not null;index" json:"service_id"`
}

// TableName نام جدول دیتابیس را برای مدل ClinicInsuranceService برمی‌گرداند.
// ورودی: ندارد (گیرنده متد). خروجی: نام جدول در دیتابیس (clinic_insurance_services).
func (ClinicInsuranceService) TableName() string {
	return "clinic_insurance_services"
}

// DoctorService جدول واسط انتصاب خدمت به پزشک است.
// پزشک -> خدمات
type DoctorService struct {
	DoctorID  uint `gorm:"primaryKey;not null" json:"doctor_id"`
	ServiceID uint `gorm:"primaryKey;not null;index" json:"service_id"`
}

// TableName نام جدول دیتابیس را برای مدل DoctorService برمی‌گرداند.
// ورودی: ندارد (گیرنده متد). خروجی: نام جدول در دیتابیس (doctor_services).
func (DoctorService) TableName() string {
	return "doctor_services"
}
