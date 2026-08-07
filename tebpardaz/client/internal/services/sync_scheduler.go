package services

import (
	"time"
)

// SyncScheduler کارهای دوره‌ای سینک کلاینت→سرور را اجرا می‌کند (مثلاً نوبت همه پزشکان).
type SyncScheduler struct {
	Doctors      *DoctorService
	Appointments *AppointmentService
	DoctorIDs    func() []string
	Interval     time.Duration // فاصله سینک خودکار نوبت‌ها (پیش‌فرض ۱ دقیقه)
	stop         chan struct{}
}

// NewSyncScheduler سازنده SyncScheduler با فاصله پیش‌فرض ۱ دقیقه است.
// ورودی: سرویس پزشکان، سرویس نوبت‌ها، تأمین‌کننده DoctorIDs (ExternalIDهای سینک‌شده).
// خروجی: اشاره‌گر به SyncScheduler.
func NewSyncScheduler(doctors *DoctorService, appointments *AppointmentService, doctorIDs func() []string) *SyncScheduler {
	return &SyncScheduler{
		Doctors:      doctors,
		Appointments: appointments,
		DoctorIDs:    doctorIDs,
		Interval:     time.Minute, // بروزرسانی خودکار نوبت‌ها هر ۱ دقیقه
		stop:         make(chan struct{}),
	}
}

// Start حلقه پس‌زمینه را شروع می‌کند (بروزرسانی نوبت همه پزشکان).
// ورودی: ندارد (گیرنده).
// خروجی: ندارد؛ تا فراخوانی Stop ادامه می‌یابد.
func (s *SyncScheduler) Start() {
	go s.loop()
}

// Stop حلقه پس‌زمینه را برای خروج سیگنال می‌دهد.
func (s *SyncScheduler) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
}

// RunOnce یک چرخه کامل سینک را اجرا می‌کند: پوش پزشکان + پوش نوبت همه پزشکان.
// ورودی: requestID پیشوند/شناسه همبستگی.
// خروجی: اولین خطای رخ‌داده.
func (s *SyncScheduler) RunOnce(requestID string) error {
	if s.Doctors != nil {
		if err := s.Doctors.PushDoctorList(requestID + "-doctors"); err != nil {
			return err
		}
	}
	if s.Appointments != nil {
		// پوش کامل نوبت‌ها؛ سرور در کش با TTL چهار ساعته ذخیره می‌کند
		if err := s.Appointments.PushAppointmentsForAllDoctors(requestID+"-appointments", true); err != nil {
			return err
		}
	}
	return nil
}

// loop هر Interval یک‌بار RunOnce را صدا می‌زند تا Stop شود.
func (s *SyncScheduler) loop() {
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case t := <-ticker.C:
			_ = s.RunOnce(t.UTC().Format("20060102T150405"))
		}
	}
}
