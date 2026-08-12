package cache

import (
	"fmt"
	"sort"
	"time"

	"tebpardaz/server/internal/models"
	"tebpardaz/shared/protocol"
)

const (
	// SlotTTL مدت نگهداری اسلات‌های رزرو پزشک در کش (۴ ساعت).
	SlotTTL = 4 * time.Hour
	// slotCleanupInterval فاصله پاکسازی آیتم‌های منقضی‌شده در go-cache.
	slotCleanupInterval = 30 * time.Minute
)

// clinicSlotBag داده‌های اسلات یک مرکز در حافظه است (بدون ذخیره در دیتابیس).
type clinicSlotBag struct {
	// ByDoctor نگاشت doctor_id سرور → لیست اسلات‌های همان پزشک
	ByDoctor map[uint][]models.DoctorSlot
}

// SlotCache مخزن در‌حافظه‌ای نوبت/اسلات پزشکان با انقضای ۴ ساعته.
// جایگزین SlotRepo برای سینک کلاینت است؛ دیتابیس استفاده نمی‌شود.
type SlotCache struct {
	store *Store
}

// NewSlotCache یک SlotCache روی Store موجود یا یک Store اختصاصی می‌سازد.
// ورودی: store (می‌تواند nil باشد؛ در این صورت Store جدا با TTL پیش‌فرض اسلات ساخته می‌شود).
// خروجی: اشاره‌گر به SlotCache آماده استفاده.
func NewSlotCache(store *Store) *SlotCache {
	if store == nil {
		store = New(SlotTTL, slotCleanupInterval)
	}
	return &SlotCache{store: store}
}

// clinicKey کلید کش برای اسلات‌های یک مرکز را برمی‌گرداند.
func clinicKey(clinicID uint) string {
	return fmt.Sprintf("appointment_slots:%d", clinicID)
}

// UpsertFromPush اسلات‌های دریافتی از کلاینت را در کش می‌نویسد (بدون دیتابیس).
// ورودی: clinicID، payload پوش، لیست پزشکان تأیید‌شده برای نگاشت ExternalID.
// خروجی: تعداد اسلات پذیرفته‌شده و خطا (در صورت وجود).
// رفتار FullReplace:
//   - scope=one → فقط اسلات‌های همان پزشک جایگزین می‌شوند
//   - scope=all → کل اسلات‌های مرکز جایگزین می‌شوند
func (c *SlotCache) UpsertFromPush(clinicID uint, push *protocol.AppointmentListPush, doctors []models.Doctor) (int, error) {
	if c == nil || c.store == nil || push == nil {
		return 0, nil
	}

	byExternal := make(map[string]models.Doctor, len(doctors))
	for _, doc := range doctors {
		if doc.ExternalID == "" {
			continue
		}
		byExternal[doc.ExternalID] = doc
	}

	incoming := make(map[uint][]models.DoctorSlot)
	written := 0
	for _, dto := range push.Appointments {
		doc, ok := byExternal[dto.DoctorExternalID]
		if !ok {
			continue
		}
		slot := models.DoctorSlot{
			DoctorID:       doc.ID,
			ClinicID:       clinicID,
			StartsAt:       time.Unix(dto.StartsAtUnix, 0),
			EndsAt:         time.Unix(dto.EndsAtUnix, 0),
			Capacity:       dto.Capacity,
			BookedCount:    dto.BookedCount,
			IsAvailable:    dto.IsAvailable,
			ExternalSlotID: slotExternalID(dto),
		}
		incoming[doc.ID] = append(incoming[doc.ID], slot)
		written++
	}

	existing := c.loadClinic(clinicID)
	if existing.ByDoctor == nil {
		existing.ByDoctor = make(map[uint][]models.DoctorSlot)
	}

	if push.FullReplace {
		switch push.Scope {
		case protocol.ScopeOneDoctor:
			// جایگزینی فقط برای یک پزشک (بر اساس DoctorExternalID پوش یا اولین پزشک ورودی)
			doctorID := c.resolveOneDoctorID(push, byExternal, incoming)
			if doctorID != 0 {
				if slots, ok := incoming[doctorID]; ok {
					existing.ByDoctor[doctorID] = slots
				} else {
					delete(existing.ByDoctor, doctorID)
				}
			}
		default:
			// جایگزینی کامل همه پزشکان مرکز
			existing.ByDoctor = incoming
		}
	} else {
		// ادغام بدون حذف کامل: اسلات‌های هر پزشک ورودی جایگزین همان پزشک می‌شوند
		for doctorID, slots := range incoming {
			existing.ByDoctor[doctorID] = slots
		}
	}

	c.store.SetWithTTL(clinicKey(clinicID), existing, SlotTTL)
	return written, nil
}

// resolveOneDoctorID شناسه سرور پزشک هدف برای scope=one را پیدا می‌کند.
func (c *SlotCache) resolveOneDoctorID(
	push *protocol.AppointmentListPush,
	byExternal map[string]models.Doctor,
	incoming map[uint][]models.DoctorSlot,
) uint {
	if push.DoctorExternalID != "" {
		if doc, ok := byExternal[push.DoctorExternalID]; ok {
			return doc.ID
		}
	}
	for id := range incoming {
		return id
	}
	return 0
}

// ListByClinic همه اسلات‌های کش‌شده یک مرکز را برمی‌گرداند (مرتب‌شده بر اساس زمان شروع).
// ورودی: clinicID.
// خروجی: برش اسلات‌ها یا خطای خالی در صورت نبود کش.
func (c *SlotCache) ListByClinic(clinicID uint) ([]models.DoctorSlot, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("slot cache unavailable")
	}
	bag := c.loadClinic(clinicID)
	out := make([]models.DoctorSlot, 0)
	for _, slots := range bag.ByDoctor {
		out = append(out, slots...)
	}
	sortSlotsByStart(out)
	return out, nil
}

// NearestAvailableByDoctors نزدیک‌ترین اسلات قابل‌رزرو هر پزشک را از کش برمی‌گرداند.
// ورودی: clinicIDs و doctorIDs برای فیلتر؛ from (شامل)؛ to (غیرشامل، صفر = بدون سقف).
// خروجی: نگاشت doctorID → نزدیک‌ترین DoctorSlot قابل رزرو.
// اسلات قابل رزرو: is_available، در بازه زمانی، و booked_count < capacity.
func (c *SlotCache) NearestAvailableByDoctors(clinicIDs, doctorIDs []uint, from, to time.Time) (map[uint]models.DoctorSlot, error) {
	out := make(map[uint]models.DoctorSlot)
	if c == nil || c.store == nil {
		return out, fmt.Errorf("slot cache unavailable")
	}
	if len(clinicIDs) == 0 || len(doctorIDs) == 0 {
		return out, nil
	}
	if from.IsZero() {
		from = time.Now()
	}

	doctorSet := make(map[uint]struct{}, len(doctorIDs))
	for _, id := range doctorIDs {
		doctorSet[id] = struct{}{}
	}
	clinicSet := make(map[uint]struct{}, len(clinicIDs))
	for _, id := range clinicIDs {
		clinicSet[id] = struct{}{}
	}

	for clinicID := range clinicSet {
		bag := c.loadClinic(clinicID)
		for doctorID, slots := range bag.ByDoctor {
			if _, wanted := doctorSet[doctorID]; !wanted {
				continue
			}
			for _, slot := range slots {
				// if !slot.IsAvailable {
				// 	continue
				// }
				if slot.StartsAt.Before(from) {
					continue
				}
				if !to.IsZero() && !slot.StartsAt.Before(to) {
					continue
				}
				fmt.Println("slot", slot.BookedCount, slot.Capacity , slot.BookedCount >= slot.Capacity , doctorID)
				if slot.BookedCount <= 0{
					continue
				}
				prev, ok := out[doctorID]
				if !ok || slot.StartsAt.Before(prev.StartsAt) {
					out[doctorID] = slot
				}
			}
		}
	}
	return out, nil
}

// AvailableForDoctor returns available future slots for one doctor from cache.
// Inputs: clinicID, doctorID, from (inclusive lower bound), limit (0 = all).
// Output: slots sorted by StartsAt ascending.
func (c *SlotCache) AvailableForDoctor(clinicID, doctorID uint, from time.Time, limit int) []models.DoctorSlot {
	out := make([]models.DoctorSlot, 0)
	if c == nil || c.store == nil || clinicID == 0 || doctorID == 0 {
		return out
	}
	if from.IsZero() {
		from = time.Now()
	}
	bag := c.loadClinic(clinicID)
	slots := bag.ByDoctor[doctorID]
	
	for _, slot := range slots {
		if !slot.IsAvailable {
			continue
		}
		if slot.StartsAt.Before(from) {
			continue
		}

		out = append(out, slot)
	}
	sortSlotsByStart(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// loadClinic محتوای کش یک مرکز را می‌خواند؛ در صورت نبود، کیف خالی برمی‌گرداند.
func (c *SlotCache) loadClinic(clinicID uint) clinicSlotBag {
	bag, ok := GetTyped[clinicSlotBag](c.store, clinicKey(clinicID))
	if !ok || bag.ByDoctor == nil {
		return clinicSlotBag{ByDoctor: make(map[uint][]models.DoctorSlot)}
	}
	return bag
}

// slotExternalID شناسه خارجی اسلات را از DTO می‌سازد (یا fallback پایدار).
func slotExternalID(dto protocol.AppointmentDTO) string {
	if dto.ExternalSlotID != "" {
		return dto.ExternalSlotID
	}
	return fmt.Sprintf("%s:%d", dto.DoctorExternalID, dto.StartsAtUnix)
}

// sortSlotsByStart اسلات‌ها را بر اساس StartsAt صعودی مرتب می‌کند.
func sortSlotsByStart(slots []models.DoctorSlot) {
	sort.Slice(slots, func(i, j int) bool {
		return slots[i].StartsAt.Before(slots[j].StartsAt)
	})
}
