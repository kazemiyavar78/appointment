package cache

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"tebpardaz/server/internal/text"
	"tebpardaz/shared/protocol"
)

const (
	// PendingDoctorTTL مدت نگهداری پزشکان تأییدنشده در کش (۲۴ ساعت؛ هر سینک کلینیک آن را تازه می‌کند).
	PendingDoctorTTL = 24 * time.Hour
	// pendingDoctorCleanup فاصله پاکسازی آیتم‌های منقضی‌شده.
	pendingDoctorCleanup = 1 * time.Hour
)

// PendingDoctor پزشک ارسال‌شده از کلینیک است که هنوز تأیید نشده و فقط در کش نگه داشته می‌شود.
type PendingDoctor struct {
	ClinicID       uint      `json:"clinic_id"`
	LocalCode      int       `json:"local_code"`
	NationalID     string    `json:"national_id"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Name           string    `json:"name"`
	Mobile         string    `json:"mobile"`
	DoctorSystemID int       `json:"doctor_system_id"`
	SpecialtyCode  string    `json:"specialty_code"`
	PhotoURL       string    `json:"photo_url"`
	IsActive       bool      `json:"is_active"`
	ExternalID     string    `json:"external_id"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// clinicPendingBag کیف پزشکان تأییدنشده یک مرکز در حافظه است.
type clinicPendingBag struct {
	ByNationalID map[string]PendingDoctor
}

// DoctorCache مخزن در‌حافظه‌ای پزشکان تأییدنشده (بدون ذخیره در دیتابیس).
type DoctorCache struct {
	store *Store
}

// NewDoctorCache یک DoctorCache روی Store موجود یا Store اختصاصی می‌سازد.
// ورودی: store (می‌تواند nil باشد).
// خروجی: اشاره‌گر به DoctorCache آماده استفاده.
func NewDoctorCache(store *Store) *DoctorCache {
	if store == nil {
		store = New(PendingDoctorTTL, pendingDoctorCleanup)
	}
	return &DoctorCache{store: store}
}

// pendingKey کلید کش پزشکان تأییدنشده یک مرکز را برمی‌گرداند.
func pendingKey(clinicID uint) string {
	return fmt.Sprintf("pending_doctors:%d", clinicID)
}

// ReplacePending لیست پزشکان تأییدنشده یک مرکز را با دادهٔ جدید جایگزین می‌کند.
// ورودی: clinicID و برش DTO پزشکان (فقط آن‌هایی که در DB تأیید نشده‌اند).
// خروجی: تعداد رکورد نوشته‌شده.
func (c *DoctorCache) ReplacePending(clinicID uint, doctors []protocol.DoctorDTO) int {
	if c == nil || c.store == nil || clinicID == 0 {
		return 0
	}
	bag := clinicPendingBag{ByNationalID: make(map[string]PendingDoctor, len(doctors))}
	now := time.Now()
	for _, dto := range doctors {
		nid := strings.TrimSpace(dto.NationalID)
		if nid == "" {
			continue
		}
		firstName := text.NormalizePersianText(dto.FirstName)
		lastName := text.NormalizePersianText(dto.LastName)
		name := text.NormalizePersianText(dto.Name)
		if name == "" {
			name = strings.TrimSpace(firstName + " " + lastName)
		}
		bag.ByNationalID[nid] = PendingDoctor{
			ClinicID:       clinicID,
			LocalCode:      dto.LocalCode,
			NationalID:     nid,
			FirstName:      firstName,
			LastName:       lastName,
			Name:           name,
			Mobile:         dto.Mobile,
			DoctorSystemID: dto.DoctorSystemID,
			SpecialtyCode:  dto.SpecialtyCode,
			PhotoURL:       dto.PhotoURL,
			IsActive:       dto.IsActive,
			ExternalID:     dto.ExternalID,
			UpdatedAt:      now,
		}
	}
	c.store.SetWithTTL(pendingKey(clinicID), bag, PendingDoctorTTL)
	return len(bag.ByNationalID)
}

// ListPending همه پزشکان تأییدنشده کش‌شده یک مرکز را برمی‌گرداند (مرتب بر اساس نام).
// ورودی: clinicID.
// خروجی: برش PendingDoctor.
func (c *DoctorCache) ListPending(clinicID uint) []PendingDoctor {
	if c == nil || c.store == nil || clinicID == 0 {
		return nil
	}
	bag := c.loadClinic(clinicID)
	out := make([]PendingDoctor, 0, len(bag.ByNationalID))
	for _, d := range bag.ByNationalID {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// GetPending یک پزشک تأییدنشده را با کد ملی از کش می‌خواند.
// ورودی: clinicID و nationalID.
// خروجی: PendingDoctor و true در صورت وجود.
func (c *DoctorCache) GetPending(clinicID uint, nationalID string) (PendingDoctor, bool) {
	nid := strings.TrimSpace(nationalID)
	if c == nil || c.store == nil || clinicID == 0 || nid == "" {
		return PendingDoctor{}, false
	}
	bag := c.loadClinic(clinicID)
	d, ok := bag.ByNationalID[nid]
	return d, ok
}

// RemovePending یک پزشک تأییدنشده را از کش مرکز حذف می‌کند.
// ورودی: clinicID و nationalID.
// خروجی: true اگر حذف شد.
func (c *DoctorCache) RemovePending(clinicID uint, nationalID string) bool {
	nid := strings.TrimSpace(nationalID)
	if c == nil || c.store == nil || clinicID == 0 || nid == "" {
		return false
	}
	bag := c.loadClinic(clinicID)
	if _, ok := bag.ByNationalID[nid]; !ok {
		return false
	}
	delete(bag.ByNationalID, nid)
	c.store.SetWithTTL(pendingKey(clinicID), bag, PendingDoctorTTL)
	return true
}

// loadClinic کیف کش یک مرکز را می‌خواند؛ در صورت نبود، کیف خالی برمی‌گرداند.
func (c *DoctorCache) loadClinic(clinicID uint) clinicPendingBag {
	bag, ok := GetTyped[clinicPendingBag](c.store, pendingKey(clinicID))
	if !ok || bag.ByNationalID == nil {
		return clinicPendingBag{ByNationalID: make(map[string]PendingDoctor)}
	}
	return bag
}
