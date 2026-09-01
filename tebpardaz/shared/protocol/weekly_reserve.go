package protocol

// WeeklyReserveDTO یک ردیف نوبت هفتگی پزشک از HIS مرکز است.
type WeeklyReserveDTO struct {
	DoctorFullName  string `json:"doctor_full_name"`
	ReserveCount    int    `json:"reserve_count"`
	ReserveMaxCount int    `json:"reserve_max_count"`
	ReserveDate     string `json:"reserve_date"`
	ReserveTime     string `json:"reserve_time"`
	Speciality      string `json:"speciality"`
	ShiftName       string `json:"shift_name"`
	VisitTime       int    `json:"visit_time"`
	OutTime         string `json:"out_time"`
}

// WeeklyReserveListRequest از کلاینت مرکز می‌خواهد لیست نوبت هفتگی را پوش کند.
// جهت: سرور → کلاینت.
type WeeklyReserveListRequest struct{}

// WeeklyReserveListPush ردیف‌های نوبت هفتگی را از کلاینت به سرور می‌برد.
// جهت: کلاینت → سرور.
// سرور این داده را در دیتابیس ذخیره نمی‌کند؛ فقط در کش نگه می‌دارد.
type WeeklyReserveListPush struct {
	Reserves []WeeklyReserveDTO    `json:"reserves"`
	Shifts   map[string][]string   `json:"shifts"` // نام شیفت → [نام شیفت، تاریخ نمونه]
}

// WeeklyReserveListAck دریافت پوش لیست نوبت هفتگی را تأیید می‌کند.
// جهت: سرور → کلاینت.
type WeeklyReserveListAck struct {
	Accepted int  `json:"accepted"`
	OK       bool `json:"ok"`
}
