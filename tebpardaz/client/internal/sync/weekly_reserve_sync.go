package sync

import "tebpardaz/client/internal/services"

// WeeklyReserveSync پوشش نازک دور سرویس نوبت هفتگی برای سینک دوره‌ای است.
type WeeklyReserveSync struct {
	Service *services.WeeklyReserveService
}

// NewWeeklyReserveSync سازنده worker سینک نوبت هفتگی است.
// ورودی: service هماهنگی نوبت هفتگی.
// خروجی: اشاره‌گر به WeeklyReserveSync.
func NewWeeklyReserveSync(service *services.WeeklyReserveService) *WeeklyReserveSync {
	return &WeeklyReserveSync{Service: service}
}

// RunOnce یک چرخه سینک نوبت هفتگی را اجرا می‌کند.
// ورودی: ندارد.
// خروجی: خطا از PushWeeklyReserves.
// داده روی سرور فقط در کش نگه داشته می‌شود، نه در دیتابیس.
func (s *WeeklyReserveSync) RunOnce() error {
	if s == nil || s.Service == nil {
		return nil
	}
	return s.Service.PushWeeklyReserves("weekly-reserve-sync")
}
