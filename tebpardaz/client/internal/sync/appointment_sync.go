package sync

import "tebpardaz/client/internal/services"

// AppointmentSync پوشش نازک دور سرویس نوبت برای سینک دوره‌ای است.
type AppointmentSync struct {
	Service   *services.AppointmentService
	DoctorIDs func() []string
}

// NewAppointmentSync سازنده worker سینک نوبت است.
// ورودی: service (هماهنگی نوبت)، doctorIDs برای سینک کامل.
// خروجی: اشاره‌گر به AppointmentSync.
func NewAppointmentSync(service *services.AppointmentService, doctorIDs func() []string) *AppointmentSync {
	return &AppointmentSync{Service: service, DoctorIDs: doctorIDs}
}

// RunOnce یک چرخه سینک نوبت همه پزشکان را اجرا می‌کند.
// ورودی: ندارد.
// خروجی: خطا از PushAppointmentsForAllDoctors.
// داده روی سرور فقط در کش (۴ ساعت) نگه داشته می‌شود، نه در دیتابیس.
func (s *AppointmentSync) RunOnce() error {
	if s.Service == nil {
		return nil
	}
	return s.Service.PushAppointmentsForAllDoctors("appointment-sync", true)
}
