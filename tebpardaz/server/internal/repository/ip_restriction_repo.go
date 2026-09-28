package repository

import (
	"errors"
	"strings"
	"sync"
	"time"

	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

const (
	// ipAllowCacheTTL مدت کش «این آی‌پی قانون ندارد» است تا هر درخواست به دیتابیس نرود.
	ipAllowCacheTTL = 20 * time.Second
)

// IPRestrictionRepo قانون بلاک و rate limit آی‌پی را در دیتابیس نوبت نگه می‌دارد.
type IPRestrictionRepo struct {
	DB *gorm.DB

	mu         sync.Mutex
	allowUntil map[string]time.Time
}

// NewIPRestrictionRepo یک IPRestrictionRepo می‌سازد.
// ورودی: db دیتابیس نوبت. خروجی: اشاره‌گر به مخزن.
func NewIPRestrictionRepo(db *gorm.DB) *IPRestrictionRepo {
	return &IPRestrictionRepo{
		DB:         db,
		allowUntil: make(map[string]time.Time),
	}
}

// Evaluate قانون فعال آی‌پی و scope را اعمال می‌کند.
// قانون rate limit یک ضربه مصرف می‌کند. نبودن یا انقضای قانون یعنی اجازه.
// ورودی: ip، scope (خالی یعنی سراسری)، now. خروجی: allowed، kind (خالی اگر قانون فعالی نباشد)، خطای دیتابیس.
func (r *IPRestrictionRepo) Evaluate(ip, scope string, now time.Time) (bool, string, error) {
	ip = normalizeRestrictionIP(ip)
	if ip == "" || r == nil || r.DB == nil {
		return true, "", nil
	}
	if r.allowCached(ip, scope, now) {
		return true, "", nil
	}

	var allowed bool
	var kind string
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		var row models.IPRestriction
		err := tx.Where("ip_address = ? AND scope = ?", ip, scope).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			allowed = true
			return nil
		}
		if err != nil {
			return err
		}
		kind = row.Kind
		allowed = row.AllowHit(now)
		if row.Kind != models.IPRestrictionKindRateLimit || (row.ExpiresAt != nil && !now.Before(*row.ExpiresAt)) {
			if allowed {
				kind = ""
			}
			return nil
		}
		return tx.Model(&models.IPRestriction{}).Where("id = ?", row.ID).Updates(map[string]any{
			"hit_count":    row.HitCount,
			"window_start": row.WindowStart,
			"updated_at":   now,
		}).Error
	})
	if err != nil {
		return true, "", err
	}
	if allowed && kind == "" {
		r.rememberAllow(ip, scope, now.Add(ipAllowCacheTTL))
	}
	return allowed, kind, nil
}

// Block قانون مسدودسازی را درج یا جایگزین می‌کند.
// ورودی: ip، scope، reason، expiresAt. nil یعنی بلاک بدون انقضا.
// خروجی: خطای دیتابیس.
func (r *IPRestrictionRepo) Block(ip, scope, reason string, expiresAt *time.Time) error {
	now := time.Now()
	row := &models.IPRestriction{
		IPAddress: normalizeRestrictionIP(ip),
		Scope:     scope,
		Kind:      models.IPRestrictionKindBlock,
		Reason:    reason,
		ExpiresAt: expiresAt,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := r.upsert(row); err != nil {
		return err
	}
	r.forgetAllow(row.IPAddress, scope)
	return nil
}

// SetRateLimit قانون محدودیت نرخ را درج یا جایگزین می‌کند و پنجره جاری را صفر می‌کند.
// ورودی: ip، scope، reason، maxRequests، windowSeconds، expiresAt. nil یعنی بدون انقضا.
// خروجی: خطای دیتابیس.
func (r *IPRestrictionRepo) SetRateLimit(ip, scope, reason string, maxRequests, windowSeconds int, expiresAt *time.Time) error {
	now := time.Now()
	row := &models.IPRestriction{
		IPAddress:     normalizeRestrictionIP(ip),
		Scope:         scope,
		Kind:          models.IPRestrictionKindRateLimit,
		Reason:        reason,
		MaxRequests:   maxRequests,
		WindowSeconds: windowSeconds,
		HitCount:      0,
		ExpiresAt:     expiresAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := r.upsert(row); err != nil {
		return err
	}
	r.forgetAllow(row.IPAddress, scope)
	return nil
}

// upsert ردیف را بر اساس آی‌پی و scope درج یا به‌روز می‌کند.
// ورودی: row. خروجی: خطای دیتابیس.
func (r *IPRestrictionRepo) upsert(row *models.IPRestriction) error {
	if r == nil || r.DB == nil {
		return gorm.ErrInvalidDB
	}
	if row == nil || row.IPAddress == "" {
		return gorm.ErrInvalidData
	}
	var existing models.IPRestriction
	err := r.DB.Where("ip_address = ? AND scope = ?", row.IPAddress, row.Scope).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.DB.Create(row).Error
	}
	if err != nil {
		return err
	}
	return r.DB.Model(&existing).Updates(map[string]any{
		"kind":           row.Kind,
		"reason":         row.Reason,
		"max_requests":   row.MaxRequests,
		"window_seconds": row.WindowSeconds,
		"hit_count":      row.HitCount,
		"window_start":   nil,
		"expires_at":     row.ExpiresAt,
		"updated_at":     row.UpdatedAt,
	}).Error
}

// allowCached اگر همین آی‌پی به‌تازگی بدون قانون دیده شده باشد true برمی‌گرداند.
// ورودی: ip، scope، now. خروجی: true یعنی تا پایان مهلت کش نیازی به دیتابیس نیست.
func (r *IPRestrictionRepo) allowCached(ip, scope string, now time.Time) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	until, ok := r.allowUntil[ipCacheKey(ip, scope)]
	if !ok {
		return false
	}
	if !now.Before(until) {
		delete(r.allowUntil, ipCacheKey(ip, scope))
		return false
	}
	return true
}

// rememberAllow نبودن قانون را برای مدت کوتاه کش می‌کند.
// ورودی: ip، scope، until. خروجی: ندارد.
func (r *IPRestrictionRepo) rememberAllow(ip, scope string, until time.Time) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.allowUntil == nil {
		r.allowUntil = make(map[string]time.Time)
	}
	r.allowUntil[ipCacheKey(ip, scope)] = until
}

// forgetAllow کش اجازه این آی‌پی را پاک می‌کند تا بلاک جدید فوری اعمال شود.
// ورودی: ip و scope. خروجی: ندارد.
func (r *IPRestrictionRepo) forgetAllow(ip, scope string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.allowUntil, ipCacheKey(ip, scope))
}

// ipCacheKey کلید کش اجازه را از آی‌پی و scope می‌سازد.
func ipCacheKey(ip, scope string) string {
	return ip + "\x00" + scope
}

// normalizeRestrictionIP فاصله را حذف و حروف را کوچک می‌کند.
// ورودی: ip. خروجی: آی‌پی نرمال‌شده.
func normalizeRestrictionIP(ip string) string {
	return strings.ToLower(strings.TrimSpace(ip))
}
