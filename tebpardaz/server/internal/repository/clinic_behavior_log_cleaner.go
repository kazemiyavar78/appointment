package repository

import (
	"log"
	"sync"
	"time"
)

// clinicBehaviorLogPurger حذف لاگ‌های منقضی رفتار مرکز را انجام می‌دهد.
type clinicBehaviorLogPurger interface {
	DeleteOlderThan(before time.Time) (int64, error)
}

// ClinicBehaviorLogCleaner لاگ‌های قدیمی‌تر از ۲ روز را دوره‌ای کامل پاک می‌کند.
type ClinicBehaviorLogCleaner struct {
	purger   clinicBehaviorLogPurger
	interval time.Duration
	now      func() time.Time

	stopCh    chan struct{}
	doneCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewClinicBehaviorLogCleaner سازنده پاک‌کننده لاگ رفتار مرکز است.
// ورودی: repo لاگ، interval بازه اجرا (صفر یعنی ClinicBehaviorLogCleanupInterval).
// خروجی: اشاره‌گر به ClinicBehaviorLogCleaner (هنوز شروع نشده).
func NewClinicBehaviorLogCleaner(repo *ClinicBehaviorLogRepo, interval time.Duration) *ClinicBehaviorLogCleaner {
	if interval <= 0 {
		interval = ClinicBehaviorLogCleanupInterval
	}
	return &ClinicBehaviorLogCleaner{
		purger:   repo,
		interval: interval,
		now:      time.Now,
		stopCh:   make(chan struct{}),
	}
}

// Start حلقه پاکسازی را فقط یک‌بار در پس‌زمینه اجرا می‌کند.
// ورودی: ندارد.
// خروجی: ندارد (goroutine در پس‌زمینه؛ ابتدا یک‌بار فوری پاکسازی می‌شود).
func (c *ClinicBehaviorLogCleaner) Start() {
	if c == nil {
		return
	}
	c.startOnce.Do(func() {
		c.doneCh = make(chan struct{})
		go c.loop()
	})
}

// Stop حلقه پاکسازی را متوقف می‌کند و تا خروج آن منتظر می‌ماند.
// ورودی: ندارد.
// خروجی: ندارد.
func (c *ClinicBehaviorLogCleaner) Stop() {
	if c == nil {
		return
	}
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	if c.doneCh != nil {
		<-c.doneCh
	}
}

// PurgeNow لاگ‌هایی را که بیش از ۲ روز از زمان ثبت‌شان گذشته کامل حذف می‌کند.
// ورودی: ندارد (آستانه از زمان فعلی منهای ClinicBehaviorLogRetention محاسبه می‌شود).
// خروجی: تعداد ردیف حذف‌شده، یا خطای پایگاه‌داده.
func (c *ClinicBehaviorLogCleaner) PurgeNow() (int64, error) {
	if c == nil || c.purger == nil {
		return 0, nil
	}
	nowFn := c.now
	if nowFn == nil {
		nowFn = time.Now
	}
	n, err := c.purger.DeleteOlderThan(ClinicBehaviorLogCutoff(nowFn()))
	if err != nil {
		return n, err
	}
	if n > 0 {
		log.Printf("clinic-log: purged %d log(s) older than 2 days", n)
	}
	return n, nil
}

// loop بلافاصله یک‌بار پاکسازی می‌کند، سپس تا Stop هر interval تکرار می‌کند.
func (c *ClinicBehaviorLogCleaner) loop() {
	defer close(c.doneCh)
	if _, err := c.PurgeNow(); err != nil {
		log.Printf("clinic-log: purge older than 2 days: %v", err)
	}
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if _, err := c.PurgeNow(); err != nil {
				log.Printf("clinic-log: purge older than 2 days: %v", err)
			}
		case <-c.stopCh:
			return
		}
	}
}
