package repository

import (
	"strings"
	"time"

	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// ClinicBehaviorLogFilter فیلترهای صفحه‌بندی‌شده لاگ رفتار مراکز را نگه می‌دارد.
type ClinicBehaviorLogFilter struct {
	AllowedClinicIDs []uint
	ClinicID         uint
	Action           string
	MsgType          string
	RequestID        string
	Query            string
	From             *time.Time
	To               *time.Time
	Page             int
	PerPage          int
	SinceID          uint
}

// ClinicBehaviorLogListResult نتیجه لیست لاگ با تعداد کل رکوردها است.
type ClinicBehaviorLogListResult struct {
	Rows  []models.ClinicBehaviorLog
	Total int64
}

const (
	// ClinicBehaviorLogRetention حداکثر مدت نگهداری لاگ رفتار مرکز است.
	ClinicBehaviorLogRetention = 2 * 24 * time.Hour
	// ClinicBehaviorLogCleanupInterval فاصله اجرای پاکسازی لاگ‌های منقضی است.
	ClinicBehaviorLogCleanupInterval = 1 * time.Hour
)

// ClinicBehaviorLogRepo دسترسی خواندن و پاکسازی لاگ رفتار مراکز روی WebSocket را فراهم می‌کند.
type ClinicBehaviorLogRepo struct {
	DB *gorm.DB
}

// NewClinicBehaviorLogRepo سازنده ClinicBehaviorLogRepo است.
// ورودی: db اتصال appointment.
// خروجی: اشاره‌گر به ClinicBehaviorLogRepo.
func NewClinicBehaviorLogRepo(db *gorm.DB) *ClinicBehaviorLogRepo {
	return &ClinicBehaviorLogRepo{DB: db}
}

// List لاگ‌ها را با فیلتر و صفحه‌بندی برمی‌گرداند.
// ورودی: ClinicBehaviorLogFilter شامل محدوده مراکز مجاز و فیلترهای UI.
// خروجی: ردیف‌ها و تعداد کل، یا خطای پایگاه‌داده.
func (r *ClinicBehaviorLogRepo) List(filter ClinicBehaviorLogFilter) (ClinicBehaviorLogListResult, error) {
	out := ClinicBehaviorLogListResult{Rows: []models.ClinicBehaviorLog{}}
	if r == nil || r.DB == nil {
		return out, nil
	}

	q := r.DB.Model(&models.ClinicBehaviorLog{})
	q = r.applyFilter(q, filter)

	if err := q.Count(&out.Total).Error; err != nil {
		return out, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage <= 0 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}

	err := q.Order("id desc").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&out.Rows).Error
	return out, err
}

// ListSince ردیف‌های جدیدتر از sinceID را برای WebSocket زنده برمی‌گرداند.
// ورودی: sinceID آخرین شناسه دیده‌شده، filter برای اعمال همان فیلترهای UI.
// خروجی: حداکثر ۱۰۰ ردیف جدید مرتب‌شده صعودی.
func (r *ClinicBehaviorLogRepo) ListSince(sinceID uint, filter ClinicBehaviorLogFilter) ([]models.ClinicBehaviorLog, error) {
	if r == nil || r.DB == nil {
		return nil, nil
	}
	filter.SinceID = sinceID
	q := r.DB.Model(&models.ClinicBehaviorLog{})
	q = r.applyFilter(q, filter)
	var rows []models.ClinicBehaviorLog
	err := q.Where("id > ?", sinceID).Order("id asc").Limit(100).Find(&rows).Error
	return rows, err
}

// applyFilter شرط‌های مشترک لیست و WebSocket را روی query اعمال می‌کند.
// لاگ‌های قدیمی‌تر از ۲ روز از نتیجه خواندن حذف می‌شوند تا قبل از پاکسازی دوره‌ای دیده نشوند.
func (r *ClinicBehaviorLogRepo) applyFilter(q *gorm.DB, filter ClinicBehaviorLogFilter) *gorm.DB {
	q = q.Where("created_at >= ?", ClinicBehaviorLogCutoff(time.Now()))
	if len(filter.AllowedClinicIDs) > 0 {
		q = q.Where("clinic_id IN ?", filter.AllowedClinicIDs)
	}
	if filter.ClinicID > 0 {
		q = q.Where("clinic_id = ?", filter.ClinicID)
	}
	if action := strings.TrimSpace(filter.Action); action != "" {
		q = q.Where("action = ?", action)
	}
	if msgType := strings.TrimSpace(filter.MsgType); msgType != "" {
		q = q.Where("msg_type = ?", msgType)
	}
	if reqID := strings.TrimSpace(filter.RequestID); reqID != "" {
		q = q.Where("request_id = ?", reqID)
	}
	if filter.From != nil {
		q = q.Where("created_at >= ?", *filter.From)
	}
	if filter.To != nil {
		q = q.Where("created_at <= ?", *filter.To)
	}
	if search := strings.TrimSpace(filter.Query); search != "" {
		like := "%" + search + "%"
		q = q.Where("(detail LIKE ? OR error LIKE ? OR msg_type LIKE ? OR request_id LIKE ?)", like, like, like, like)
	}
	return q
}

// ClinicBehaviorLogCutoff زمان قطع نگهداری لاگ را از لحظه now حساب می‌کند.
// ورودی: now زمان مبنا.
// خروجی: لحظه‌ای که لاگ‌های قبل از آن باید کامل حذف شوند.
func ClinicBehaviorLogCutoff(now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now()
	}
	return now.Add(-ClinicBehaviorLogRetention)
}

// DeleteOlderThan لاگ‌هایی را که زمان ثبت‌شان قبل از before است کامل حذف می‌کند.
// ورودی: before آستانه created_at (معمولاً اکنون منهای ۲ روز).
// خروجی: تعداد ردیف حذف‌شده، یا خطای پایگاه‌داده.
func (r *ClinicBehaviorLogRepo) DeleteOlderThan(before time.Time) (int64, error) {
	if r == nil || r.DB == nil || before.IsZero() {
		return 0, nil
	}
	res := r.DB.Where("created_at < ?", before).Delete(&models.ClinicBehaviorLog{})
	return res.RowsAffected, res.Error
}
