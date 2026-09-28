package waitingqueue

import (
	"log"
	"sync"
	"time"

	"tebpardaz/server/internal/websocket"
	"tebpardaz/shared/protocol"

	"github.com/google/uuid"
)

const (
	// IdleRefreshInterval وقتی هیچ بیماری وصل نیست، صف هر ۱۰ دقیقه از مرکز پرسیده می‌شود.
	IdleRefreshInterval = 10 * time.Minute
	// LiveRefreshInterval وقتی حداقل یک بیمار وصل است، صف هر ۵ ثانیه یک‌بار (برای همه مراکز متصل) تازه می‌شود.
	LiveRefreshInterval = 5 * time.Second
	// tickInterval بازه بررسی حلقه زمان‌بند.
	tickInterval = 1 * time.Second
)

// Refresher بروزرسانی تطبیقی صف انتظار را مدیریت می‌کند:
// بدون بیننده → ۱۰ دقیقه؛ با بیننده → ۵ ثانیه (یک درخواست برای هر مرکز، نه برای هر بیمار).
type Refresher struct {
	Hub *websocket.Hub

	// HasPushInterest اگر true برگرداند، مرکز مثل صفحه باز هر ۵ ثانیه تازه می‌شود (مثلاً اشتراک Web Push).
	HasPushInterest func(clinicID uint) bool

	mu          sync.Mutex
	viewerCount map[uint]int
	lastRefresh map[uint]time.Time
	stopCh      chan struct{}
	startOnce   sync.Once
}

// NewRefresher سازنده Refresher است.
// ورودی: hub ارتباط با کلاینت‌های مرکز.
// خروجی: اشاره‌گر به Refresher.
func NewRefresher(hub *websocket.Hub) *Refresher {
	return &Refresher{
		Hub:         hub,
		viewerCount: make(map[uint]int),
		lastRefresh: make(map[uint]time.Time),
		stopCh:      make(chan struct{}),
	}
}

// Start حلقه زمان‌بند بروزرسانی را آغاز می‌کند.
// ورودی: ندارد.
// خروجی: ندارد (goroutine در پس‌زمینه).
func (r *Refresher) Start() {
	if r == nil {
		return
	}
	r.startOnce.Do(func() {
		go r.loop()
	})
}

// Stop حلقه زمان‌بند را متوقف می‌کند.
func (r *Refresher) Stop() {
	if r == nil {
		return
	}
	select {
	case <-r.stopCh:
	default:
		close(r.stopCh)
	}
}

// AddViewer تعداد بینندگان زنده یک مرکز را افزایش می‌دهد.
// ورودی: clinicID.
func (r *Refresher) AddViewer(clinicID uint) {
	if r == nil || clinicID == 0 {
		return
	}
	r.mu.Lock()
	wasZero := r.viewerCount[clinicID] == 0
	r.viewerCount[clinicID]++
	r.mu.Unlock()
	if wasZero {
		r.RequestNow(clinicID)
	}
}

// RemoveViewer تعداد بینندگان زنده یک مرکز را کاهش می‌دهد.
// ورودی: clinicID.
func (r *Refresher) RemoveViewer(clinicID uint) {
	if r == nil || clinicID == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.viewerCount[clinicID] <= 1 {
		delete(r.viewerCount, clinicID)
		return
	}
	r.viewerCount[clinicID]--
}

// loop هر ثانیه مراکز آنلاین را بررسی و در صورت نیاز درخواست صف می‌فرستد.
func (r *Refresher) loop() {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stopCh:
			return
		case now := <-ticker.C:
			r.tick(now)
		}
	}
}

// tick برای هر مرکز آنلاین، در صورت رسیدن به فاصله مناسب، یک درخواست صف می‌فرستد.
func (r *Refresher) tick(now time.Time) {
	if r.Hub == nil {
		return
	}
	online := r.Hub.OnlineClinicIDs()
	for _, clinicID := range online {
		interval := r.refreshIntervalFor(clinicID)
		r.mu.Lock()
		last, ok := r.lastRefresh[clinicID]
		if ok && now.Sub(last) < interval {
			r.mu.Unlock()
			continue
		}
		r.lastRefresh[clinicID] = now
		r.mu.Unlock()
		r.requestQueue(clinicID)
	}
}

// refreshIntervalFor فاصله بروزرسانی مرکز را بر اساس تعداد بیننده برمی‌گرداند.
func (r *Refresher) refreshIntervalFor(clinicID uint) time.Duration {
	r.mu.Lock()
	count := r.viewerCount[clinicID]
	r.mu.Unlock()
	if count > 0 || (r.HasPushInterest != nil && r.HasPushInterest(clinicID)) {
		return LiveRefreshInterval
	}
	return IdleRefreshInterval
}

// requestQueue یک درخواست waiting_queue.list.request به مرکز می‌فرستد.
func (r *Refresher) requestQueue(clinicID uint) {
	if r.Hub == nil {
		return
	}
	ok := r.Hub.SendToClinic(clinicID, protocol.TypeWaitingQueueListRequest, protocol.WaitingQueueListRequest{})
	if !ok {
		log.Printf("waiting-queue: clinic %d offline during refresh", clinicID)
	}
}

// RequestNow بلافاصله یک درخواست صف برای مرکز می‌فرستد (مثلاً پس از اتصال کلاینت).
func (r *Refresher) RequestNow(clinicID uint) {
	if r == nil || clinicID == 0 {
		return
	}
	r.mu.Lock()
	r.lastRefresh[clinicID] = time.Now()
	r.mu.Unlock()
	r.requestQueue(clinicID)
}

// RequestNowSync درخواست صف را می‌فرستد و تا دریافت پوش یا timeout منتظر می‌ماند.
func (r *Refresher) RequestNowSync(clinicID uint, timeout time.Duration) error {
	if r == nil || r.Hub == nil || clinicID == 0 {
		return websocket.ErrClinicOffline
	}
	requestID := uuid.NewString()
	r.mu.Lock()
	r.lastRefresh[clinicID] = time.Now()
	r.mu.Unlock()
	_, err := r.Hub.RequestWaitingQueueList(clinicID, timeout, requestID)
	return err
}
