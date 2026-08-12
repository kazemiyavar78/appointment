package cache

import (
	"fmt"
	"sort"
	"time"

	"tebpardaz/shared/protocol"
)

const (
	// WeeklyReserveTTL مدت نگهداری لیست نوبت هفتگی در کش (۱ ساعت؛ کلاینت هر ۱۵ دقیقه تازه می‌کند).
	WeeklyReserveTTL = 1 * time.Hour
	// weeklyReserveCleanup فاصله پاکسازی آیتم‌های منقضی‌شده.
	weeklyReserveCleanup = 30 * time.Minute
)

// ClinicWeeklyReserves کیف نوبت‌های هفتگی یک مرکز در حافظه است.
type ClinicWeeklyReserves struct {
	Reserves  []protocol.WeeklyReserveDTO `json:"reserves"`
	Shifts    map[string][]string         `json:"shifts"`
	UpdatedAt time.Time                   `json:"updated_at"`
}

// WeeklyReserveCache مخزن در‌حافظه‌ای لیست نوبت هفتگی پزشکان (بدون دیتابیس).
type WeeklyReserveCache struct {
	store *Store
}

// NewWeeklyReserveCache یک WeeklyReserveCache روی Store موجود یا Store اختصاصی می‌سازد.
// ورودی: store (می‌تواند nil باشد).
// خروجی: اشاره‌گر به WeeklyReserveCache آماده استفاده.
func NewWeeklyReserveCache(store *Store) *WeeklyReserveCache {
	if store == nil {
		store = New(WeeklyReserveTTL, weeklyReserveCleanup)
	}
	return &WeeklyReserveCache{store: store}
}

// clinicWeeklyKey کلید کش نوبت هفتگی یک مرکز را برمی‌گرداند.
func clinicWeeklyKey(clinicID uint) string {
	return fmt.Sprintf("weekly_reserves:%d", clinicID)
}

// ReplaceFromPush لیست نوبت هفتگی یک مرکز را با دادهٔ جدید جایگزین می‌کند.
// ورودی: clinicID و payload پوش از کلاینت.
// خروجی: تعداد ردیف پذیرفته‌شده.
func (c *WeeklyReserveCache) ReplaceFromPush(clinicID uint, push *protocol.WeeklyReserveListPush) int {
	if c == nil || c.store == nil || clinicID == 0 || push == nil {
		return 0
	}
	reserves := make([]protocol.WeeklyReserveDTO, 0, len(push.Reserves))
	reserves = append(reserves, push.Reserves...)
	sort.SliceStable(reserves, func(i, j int) bool {
		if reserves[i].ReserveDate != reserves[j].ReserveDate {
			return reserves[i].ReserveDate < reserves[j].ReserveDate
		}
		if reserves[i].ReserveTime != reserves[j].ReserveTime {
			return reserves[i].ReserveTime < reserves[j].ReserveTime
		}
		return reserves[i].Speciality < reserves[j].Speciality
	})
	shifts := push.Shifts
	if shifts == nil {
		shifts = map[string][]string{}
	}
	bag := ClinicWeeklyReserves{
		Reserves:  reserves,
		Shifts:    shifts,
		UpdatedAt: time.Now(),
	}
	c.store.SetWithTTL(clinicWeeklyKey(clinicID), bag, WeeklyReserveTTL)
	return len(reserves)
}

// Get لیست نوبت هفتگی یک مرکز را از کش می‌خواند.
// ورودی: clinicID.
// خروجی: داده مرکز و true در صورت وجود؛ در غیر این صورت صفر و false.
func (c *WeeklyReserveCache) Get(clinicID uint) (ClinicWeeklyReserves, bool) {
	if c == nil || c.store == nil || clinicID == 0 {
		return ClinicWeeklyReserves{}, false
	}
	raw, ok := c.store.Get(clinicWeeklyKey(clinicID))
	if !ok {
		return ClinicWeeklyReserves{}, false
	}
	bag, ok := raw.(ClinicWeeklyReserves)
	if !ok {
		return ClinicWeeklyReserves{}, false
	}
	return bag, true
}
