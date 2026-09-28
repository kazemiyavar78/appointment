package repository

import (
	"fmt"
	"strings"

	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// ReviewRepo دسترسی ساده به نظرات و امتیازها است.
type ReviewRepo struct {
	DB *gorm.DB
}

// NewReviewRepo سازنده ReviewRepo است.
func NewReviewRepo(db *gorm.DB) *ReviewRepo {
	return &ReviewRepo{DB: db}
}

// ReviewSummary خلاصه امتیاز برای نمایش عمومی است.
type ReviewSummary struct {
	Count   int
	Average float64
}

// Create نظر جدید را با امتیاز اجباری ذخیره می‌کند (هنوز تأییدنشده).
// ورودی: مدل Review. خروجی: خطا در صورت نامعتبر بودن.
func (r *ReviewRepo) Create(review *models.Review) error {
	if r == nil || r.DB == nil || review == nil {
		return fmt.Errorf("review create unavailable")
	}
	if review.Rating < 1 || review.Rating > 5 {
		return fmt.Errorf("rating must be 1-5")
	}
	if review.TargetType != models.ReviewTargetDoctor && review.TargetType != models.ReviewTargetClinic {
		return fmt.Errorf("invalid target type")
	}
	if review.TargetID == 0 || review.ClinicID == 0 {
		return fmt.Errorf("target and clinic required")
	}
	review.AuthorName = strings.TrimSpace(review.AuthorName)
	review.Body = strings.TrimSpace(review.Body)
	review.IPAddress = trimReviewIP(review.IPAddress)
	review.IsApproved = false
	return r.DB.Create(review).Error
}

// trimReviewIP آی‌پی را برای ستون reviews.ip_address کوتاه و تمیز می‌کند.
// ورودی: رشته آی‌پی. خروجی: حداکثر ۴۵ نویسه، بدون فاصله اضافه.
func trimReviewIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if len(ip) > 45 {
		return ip[:45]
	}
	return ip
}

// Summary میانگین و تعداد همه امتیازهای یک هدف را برمی‌گرداند.
func (r *ReviewRepo) Summary(targetType string, targetID uint) (ReviewSummary, error) {
	var out ReviewSummary
	if r == nil || r.DB == nil || targetID == 0 {
		return out, nil
	}
	type row struct {
		Count int
		Avg   float64
	}
	var res row
	err := r.DB.Model(&models.Review{}).
		Select("COUNT(*) as count, AVG(CAST(rating AS float)) as avg").
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Scan(&res).Error
	if err != nil {
		return out, err
	}
	out.Count = res.Count
	out.Average = res.Avg
	return out, nil
}

// ListApprovedBodies متن نظرات تأییدشده را برمی‌گرداند (جدیدترین اول).
func (r *ReviewRepo) ListApprovedBodies(targetType string, targetID uint, limit int) ([]models.Review, error) {
	if r == nil || r.DB == nil || targetID == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	var rows []models.Review
	err := r.DB.Where(
		"target_type = ? AND target_id = ? AND is_approved = ? AND body <> ''",
		targetType, targetID, true,
	).Order("id desc").Limit(limit).Find(&rows).Error
	return rows, err
}

// ListPending نظرات در انتظار تأیید را برای مراکز مشخص برمی‌گرداند.
func (r *ReviewRepo) ListPending(clinicIDs []uint) ([]models.Review, error) {
	if r == nil || r.DB == nil || len(clinicIDs) == 0 {
		return nil, nil
	}
	var rows []models.Review
	err := r.DB.Where("clinic_id IN ? AND is_approved = ?", clinicIDs, false).
		Order("id desc").Find(&rows).Error
	return rows, err
}

// SetApproved وضعیت تأیید نظر را تغییر می‌دهد.
func (r *ReviewRepo) SetApproved(id uint, approved bool) error {
	if r == nil || r.DB == nil || id == 0 {
		return fmt.Errorf("review update unavailable")
	}
	return r.DB.Model(&models.Review{}).Where("id = ?", id).Update("is_approved", approved).Error
}

// GetByID یک نظر را با شناسه برمی‌گرداند.
func (r *ReviewRepo) GetByID(id uint) (*models.Review, error) {
	if r == nil || r.DB == nil || id == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var row models.Review
	if err := r.DB.First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// Delete نظر را به‌صورت نرم حذف می‌کند.
func (r *ReviewRepo) Delete(id uint) error {
	if r == nil || r.DB == nil || id == 0 {
		return fmt.Errorf("review delete unavailable")
	}
	return r.DB.Delete(&models.Review{}, id).Error
}
