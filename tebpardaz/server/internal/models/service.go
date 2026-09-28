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

// ServicePackage بستهٔ خدمات است که چند خدمت را برای انتصاب گروهی گروه‌بندی می‌کند.
type ServicePackage struct {
	gorm.Model
	// Name نام بسته خدمات است
	Name string `gorm:"type:nvarchar(150);not null" json:"name"`
	// Description توضیحات بسته خدمات است
	Description string `gorm:"type:nvarchar(1000);not null;default:''" json:"description"`
}

// TableName نام جدول دیتابیس را برای مدل ServicePackage برمی‌گرداند.
// ورودی: ندارد (گیرنده متد). خروجی: نام جدول در دیتابیس (service_packages).
func (ServicePackage) TableName() string {
	return "service_packages"
}

// ServicePackageItem جدول واسط انتصاب خدمت به بسته است.
// یک خدمت می‌تواند در چند بسته حضور داشته باشد.
type ServicePackageItem struct {
	PackageID uint `gorm:"primaryKey;not null" json:"package_id"`
	ServiceID uint `gorm:"primaryKey;not null;index" json:"service_id"`
}

// TableName نام جدول دیتابیس را برای مدل ServicePackageItem برمی‌گرداند.
// ورودی: ندارد (گیرنده متد). خروجی: نام جدول در دیتابیس (service_package_items).
func (ServicePackageItem) TableName() string {
	return "service_package_items"
}

// SectionServicePackage جدول واسط انتصاب بسته خدمات به بخش یک مرکز درمانی است.
// یک بخش می‌تواند چند بسته داشته باشد و یک بسته می‌تواند به چند بخش منتسب شود.
type SectionServicePackage struct {
	SectionID uint `gorm:"primaryKey;not null" json:"section_id"`
	PackageID uint `gorm:"primaryKey;not null;index" json:"package_id"`
}

// TableName نام جدول دیتابیس را برای مدل SectionServicePackage برمی‌گرداند.
// ورودی: ندارد (گیرنده متد). خروجی: نام جدول در دیتابیس (section_service_packages).
func (SectionServicePackage) TableName() string {
	return "section_service_packages"
}
