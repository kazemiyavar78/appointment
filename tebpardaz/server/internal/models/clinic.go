package models

import (
	"time"

	"gorm.io/gorm"
)

// Clinic represents a medical center tenant site.
type Clinic struct {
	gorm.Model
	Code           int    `gorm:"type:int;not null" json:"code"`
	Name           string `gorm:"type:nvarchar(100);not null" json:"name"`
	Address        string `gorm:"type:nvarchar(255);not null" json:"address"`
	Phone          string `gorm:"type:nvarchar(255);not null" json:"phone_number"`
	Status         string `gorm:"type:nvarchar(10);not null" json:"status"`
	Description    string `gorm:"type:nvarchar(255);not null" json:"description"`
	OrganizationID uint   `gorm:"not null" json:"organization_id"`

	//شهر
	CityID uint `gorm:"not null;default:0" json:"city_id"`
	City   City `gorm:"foreignKey:CityID;references:ID"`

	//relationship
	Domain        *string    `gorm:"type:nvarchar(100);default:null" json:"domain"`                              // دامنه شخصی - nullable, می‌تونه خالی باشه
	Slug          *string    `gorm:"type:nvarchar(100);default:null" json:"slug"`                                // {clinic_name} در URL - یکتا در کل سیستم
	TenantType    string     `gorm:"type:nvarchar(100);not null;default:'private_tebpardaz'" json:"tenant_type"` // "private_own_domain" | "private_tebpardaz" | "organ_subsidiary"
	WSClientKey   string     `gorm:"type:nvarchar(255);not null;default:''" json:"-"`                            // ciphertext کلید وب‌سوکت (همان رمز clinic backend)
	EncryptionKey string     `gorm:"type:nvarchar(255);not null;default:''" json:"-"`                            // ciphertext کلید رمز پیام
	IsOnline      bool       `gorm:"type:bit;not null;default:false" json:"is_online"`                           // وضعیت آخرین اتصال (برای تصمیم‌گیری real-time نوبت‌دهی)
	LastSyncAt    *time.Time `gorm:"type:datetime;not null;default:GETDATE()" json:"last_sync_at"`               // آخرین sync موفق نوبت‌ها

	IsActiveOnWebsite bool `gorm:"type:bit;not null;default:false" json:"is_active_on_website"`

	LogoURL    string `gorm:"type:nvarchar(500);not null;default:''" json:"logo_url"`
	FaviconURL string `gorm:"type:nvarchar(500);not null;default:''" json:"favicon_url"`

	Organization Organization `gorm:"foreignKey:OrganizationID;references:ID"`
}
