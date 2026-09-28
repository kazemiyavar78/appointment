package models

import "time"

// WaitingQueueSubmission یک جستجوی موفق وضعیت نوبت است و فقط برای سابقه نگه داشته می‌شود.
// تصمیم سقف روزانه از این جدول خوانده نمی‌شود.
type WaitingQueueSubmission struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TenantID    uint      `gorm:"not null;index" json:"tenant_id"`
	AdmissionNo int       `gorm:"not null;index" json:"admission_no"`
	NationalID  string    `gorm:"type:nvarchar(20);not null;default:''" json:"national_id"`
	ClientIP    string    `gorm:"type:nvarchar(45);not null;default:'';index" json:"client_ip"`
	CreatedAt   time.Time `gorm:"type:datetime;not null;index" json:"created_at"`
}

// TableName نام جدول سابقه جستجوی صف انتظار را برمی‌گرداند.
// ورودی: ندارد. خروجی: waiting_queue_submissions.
func (WaitingQueueSubmission) TableName() string {
	return "waiting_queue_submissions"
}
