package models

import "time"

const (
	// IPRestrictionKindBlock هر درخواست این آی‌پی را تا زمان انقضا رد می‌کند.
	IPRestrictionKindBlock = "block"
	// IPRestrictionKindRateLimit تعداد درخواست را داخل یک پنجره زمانی می‌شمارد.
	IPRestrictionKindRateLimit = "rate_limit"
	// IPRestrictionScopeGlobal قانون را روی کل سرویس اعمال می‌کند.
	IPRestrictionScopeGlobal = ""
)

// IPRestriction یک قانون ذخیره‌شده برای بلاک یا rate limit آی‌پی کلاینت است.
// همین ساختار در سرور نوبت، SmsService و بک‌اند کلینیک استفاده می‌شود.
type IPRestriction struct {
	ID            uint       `gorm:"primaryKey" json:"id"`                                                                      // شناسه
	IPAddress     string     `gorm:"type:nvarchar(45);not null;uniqueIndex:ux_ip_restriction_ip_scope" json:"ip_address"`       // آی‌پی
	Scope         string     `gorm:"type:nvarchar(64);not null;default:'';uniqueIndex:ux_ip_restriction_ip_scope" json:"scope"` // محدوده؛ خالی یعنی سراسری
	Kind          string     `gorm:"type:nvarchar(20);not null;index" json:"kind"`                                              // block یا rate_limit
	Reason        string     `gorm:"type:nvarchar(255);not null;default:''" json:"reason"`                                      // دلیل
	MaxRequests   int        `gorm:"not null;default:0" json:"max_requests"`                                                    // سقف درخواست در پنجره
	WindowSeconds int        `gorm:"not null;default:0" json:"window_seconds"`                                                  // طول پنجره به ثانیه
	HitCount      int        `gorm:"not null;default:0" json:"hit_count"`                                                       // تعداد مصرف‌شده در پنجره جاری
	WindowStart   *time.Time `gorm:"type:datetime" json:"window_start"`                                                         // شروع پنجره جاری
	ExpiresAt     *time.Time `gorm:"type:datetime;index" json:"expires_at"`                                                     // پایان اعتبار؛ خالی یعنی بدون انقضا
	CreatedAt     time.Time  `gorm:"type:datetime;not null" json:"created_at"`                                                  // زمان ایجاد
	UpdatedAt     time.Time  `gorm:"type:datetime;not null" json:"updated_at"`                                                  // زمان آخرین تغییر
}

// TableName نام جدول مشترک بلاک و rate limit را برمی‌گرداند.
// ورودی: ندارد. خروجی: نام جدول.
func (IPRestriction) TableName() string {
	return "ip_restrictions"
}

// AllowHit مشخص می‌کند این قانون در زمان now یک درخواست را مجاز می‌داند یا نه.
// بلاک درخواست را رد می‌کند. rate limit یک ضربه از پنجره جاری مصرف می‌کند.
// ورودی: now. خروجی: true اگر درخواست مجاز باشد. قانون منقضی اجازه می‌دهد.
func (r *IPRestriction) AllowHit(now time.Time) bool {
	if r == nil {
		return true
	}
	if r.ExpiresAt != nil && !now.Before(*r.ExpiresAt) {
		return true
	}
	switch r.Kind {
	case IPRestrictionKindBlock:
		return false
	case IPRestrictionKindRateLimit:
		if r.MaxRequests <= 0 || r.WindowSeconds <= 0 {
			return true
		}
		window := time.Duration(r.WindowSeconds) * time.Second
		if r.WindowStart == nil || !now.Before(r.WindowStart.Add(window)) {
			start := now
			r.WindowStart = &start
			r.HitCount = 1
			return true
		}
		r.HitCount++
		return r.HitCount <= r.MaxRequests
	default:
		return true
	}
}
