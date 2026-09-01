package models

import "time"

// ClinicBehaviorLog لاگ رفتار مرکز روی WebSocket است (اتصال، پیام، رفت‌وبرگشت، خطا).
type ClinicBehaviorLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `gorm:"type:datetime;not null;index" json:"created_at"` // زمان رویداد
	ClinicID  uint      `gorm:"not null;index;default:0" json:"clinic_id"`       // شناسه مرکز (۰ اگر هنوز مشخص نباشد)
	Action    string    `gorm:"type:nvarchar(50);not null;index" json:"action"`  // نوع رفتار
	MsgType   string    `gorm:"type:nvarchar(100);not null;default:''" json:"msg_type"` // نوع پیام پروتکل
	RequestID string    `gorm:"type:nvarchar(100);not null;default:'';index" json:"request_id"` // شناسه درخواست
	Detail    string    `gorm:"type:nvarchar(500);not null;default:''" json:"detail"` // توضیح کوتاه
	Error     string    `gorm:"type:nvarchar(1000);not null;default:''" json:"error"` // متن خطا
}
