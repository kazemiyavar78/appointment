package repository

import (
	"fmt"
	"strings"
	"time"

	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// WaitingQueueSubmissionRepo درج سابقه جستجوی موفق صف انتظار در دیتابیس نوبت است.
type WaitingQueueSubmissionRepo struct {
	DB *gorm.DB
}

// NewWaitingQueueSubmissionRepo سازنده مخزن سابقه است.
// ورودی: اتصال appointment. خروجی: اشاره‌گر مخزن.
func NewWaitingQueueSubmissionRepo(db *gorm.DB) *WaitingQueueSubmissionRepo {
	return &WaitingQueueSubmissionRepo{DB: db}
}

// Create یک ردیف سابقه برای پذیرش پیدا‌شده می‌سازد.
// ورودی: شناسه مرکز (tenant)، شماره پذیرش، کد ملی و IP کلاینت.
// خروجی: خطای دیتابیس.
func (r *WaitingQueueSubmissionRepo) Create(tenantID uint, admissionNo int, nationalID, clientIP string) error {
	if r == nil || r.DB == nil {
		return fmt.Errorf("waiting queue submission repo unavailable")
	}
	if tenantID == 0 || admissionNo <= 0 {
		return fmt.Errorf("waiting queue submission incomplete")
	}
	row := models.WaitingQueueSubmission{
		TenantID:    tenantID,
		AdmissionNo: admissionNo,
		NationalID:  strings.TrimSpace(nationalID),
		ClientIP:    strings.TrimSpace(clientIP),
		CreatedAt:   time.Now(),
	}
	return r.DB.Create(&row).Error
}
