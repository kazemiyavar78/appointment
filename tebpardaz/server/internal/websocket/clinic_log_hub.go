package websocket

import (
	"sync"

	"tebpardaz/server/internal/models"
)

// ClinicLogHub مرکز انتشار رویدادهای لاگ رفتار مراکز برای WebSocket ادمین است.
// مسئولیت واحد: fan-out غیرمسدودکننده به مشترکین.
type ClinicLogHub struct {
	mu   sync.RWMutex
	subs map[chan models.ClinicBehaviorLog]struct{}
}

// clinicLogHub نمونه سراسری hub لاگ است.
var clinicLogHub = &ClinicLogHub{subs: make(map[chan models.ClinicBehaviorLog]struct{})}

// Subscribe یک کانال دریافت لاگ‌های جدید باز می‌کند.
// ورودی: ندارد.
// خروجی: کانال buffered و تابع unsubscribe.
func (h *ClinicLogHub) Subscribe() (chan models.ClinicBehaviorLog, func()) {
	ch := make(chan models.ClinicBehaviorLog, 64)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	unsub := func() {
		h.mu.Lock()
		delete(h.subs, ch)
		close(ch)
		h.mu.Unlock()
	}
	return ch, unsub
}

// Publish یک ردیف ذخیره‌شده را به همه مشترکین ارسال می‌کند.
// ورودی: row لاگ با شناسه پایگاه‌داده.
// خروجی: ندارد؛ ارسال غیرمسدودکننده است.
func (h *ClinicLogHub) Publish(row models.ClinicBehaviorLog) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs {
		select {
		case ch <- row:
		default:
			// مشترک کند است؛ ردیف رها می‌شود تا مسیر WebSocket کلینیک مسدود نشود.
		}
	}
}

// ClinicLogHubInstance hub سراسری لاگ را برمی‌گرداند.
func ClinicLogHubInstance() *ClinicLogHub {
	return clinicLogHub
}
