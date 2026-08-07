package protocol

// AppointmentListScope انتخاب یک پزشک یا همه پزشکان سینک‌شده را مشخص می‌کند.
type AppointmentListScope string

const (
	// ScopeOneDoctor درخواست نوبت برای یک ExternalID.
	ScopeOneDoctor AppointmentListScope = "one"
	// ScopeAllDoctors درخواست نوبت برای همه پزشکان قبلاً ارسال‌شده به سایت.
	ScopeAllDoctors AppointmentListScope = "all"
)

// AppointmentListRequest از کلاینت مرکز می‌خواهد داده نوبت/اسلات را پوش کند.
// جهت: سرور → کلاینت.
// کاربرد: بروزرسانی یک پزشک (scope=one) یا همه پزشکان (scope=all) به‌صورت درخواستی.
// کلاینت همچنین هر ۱ دقیقه نوبت همه پزشکان را به‌صورت خودکار پوش می‌کند.
type AppointmentListRequest struct {
	Scope             AppointmentListScope `json:"scope"`
	DoctorExternalID  string               `json:"doctor_external_id,omitempty"`
	DoctorExternalIDs []string             `json:"doctor_external_ids,omitempty"`
	FromUnix          int64                `json:"from_unix,omitempty"`
	ToUnix            int64                `json:"to_unix,omitempty"`
}

// AppointmentDTO یک اسلات قابل‌رزرو یا نوبت موجود سینک‌شده از HIS است.
type AppointmentDTO struct {
	ExternalSlotID   string `json:"external_slot_id"`
	DoctorExternalID string `json:"doctor_external_id"`
	StartsAtUnix     int64  `json:"starts_at_unix"`
	EndsAtUnix       int64  `json:"ends_at_unix"`
	Capacity         int    `json:"capacity"`
	BookedCount      int    `json:"booked_count"`
	IsAvailable      bool   `json:"is_available"`
}

// AppointmentListPush ردیف‌های نوبت/اسلات را از کلاینت به سرور می‌برد.
// جهت: کلاینت → سرور.
// سرور این داده را در دیتابیس ذخیره نمی‌کند؛ فقط در کش با انقضای ۴ ساعت نگه می‌دارد.
type AppointmentListPush struct {
	Scope            AppointmentListScope `json:"scope"`
	DoctorExternalID string               `json:"doctor_external_id,omitempty"`
	Appointments     []AppointmentDTO     `json:"appointments"`
	FullReplace      bool                 `json:"full_replace"`
}

// AppointmentListAck دریافت پوش لیست نوبت را تأیید می‌کند.
// جهت: سرور → کلاینت.
type AppointmentListAck struct {
	Accepted int  `json:"accepted"`
	OK       bool `json:"ok"`
}
