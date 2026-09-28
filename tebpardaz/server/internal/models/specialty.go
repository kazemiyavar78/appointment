package models

import "gorm.io/gorm"

// Specialty classifies doctors for filtering and display.
type Specialty struct {
	gorm.Model
	Name       string `gorm:"type:nvarchar(100);not null" json:"name"`
	NameEN     string `gorm:"type:nvarchar(100);not null;default:''" json:"name_en"`
	ShortDescription string `gorm:"type:nvarchar(100);not null;default:''" json:"short_description"`
	Description string `gorm:"type:nvarchar(max);not null;default:''" json:"description"`
	Icon          string `gorm:"type:nvarchar(100);not null;default:''" json:"icon"`
	Color         string `gorm:"type:nvarchar(100);not null;default:''" json:"color"`
	SortOrder     int    `gorm:"type:int;not null;default:0" json:"sort_order"`
	ShowInBooking bool   `gorm:"type:bit;not null;default:true" json:"show_in_booking"`
	IsApproved    bool   `gorm:"type:bit;not null;default:false" json:"is_approved"`
}

// IsVisibleInBooking مشخص می‌کند تخصص در نوبت‌دهی عمومی نمایش داده شود یا نه.
// ورودی: ندارد (گیرنده مدل). خروجی: true اگر تخصص موجود، تأییدشده و نمایش در نوبت‌دهی فعال باشد.
func (s Specialty) IsVisibleInBooking() bool {
	return s.ID > 0 && s.IsApproved && s.ShowInBooking
}
