package protocol

// WaitingQueuePatientDTO یک بیمار در صف انتظار پذیرش/ویزیت است.
type WaitingQueuePatientDTO struct {
	PatientName       string `json:"patient_name"`
	PatientCode       int    `json:"patient_code"` // شماره پذیرش
	NationalID        string `json:"national_id"`
	VisitTime         string `json:"visit_time"` // ساعت نوبت (paz_tm)
	DoctorCode        int    `json:"doctor_code"`
	DoctorName        string `json:"doctor_name"`
	ReceptionUserName string `json:"reception_user_name"`
}

// WaitingQueueDoctorDTO لیست بیماران منتظر یک پزشک را نگه می‌دارد.
type WaitingQueueDoctorDTO struct {
	DoctorName string                   `json:"doctor_name"`
	DoctorCode int                      `json:"doctor_code"`
	Patients   []WaitingQueuePatientDTO `json:"patients"`
}

// WaitingQueueListRequest از کلاینت مرکز می‌خواهد صف انتظار را پوش کند.
// جهت: سرور → کلاینت.
type WaitingQueueListRequest struct{}

// WaitingQueueListPush صف انتظار روز جاری را از HIS به سرور می‌فرستد.
// جهت: کلاینت → سرور.
type WaitingQueueListPush struct {
	Doctors []WaitingQueueDoctorDTO `json:"doctors"`
}

// WaitingQueueListAck دریافت پوش صف انتظار را تأیید می‌کند.
// جهت: سرور → کلاینت.
type WaitingQueueListAck struct {
	Accepted int  `json:"accepted"`
	OK       bool `json:"ok"`
}

// WaitingQueueStatus وضعیت یک بیمار در صف را برای نمایش زنده برمی‌گرداند.
type WaitingQueueStatus struct {
	Found         bool   `json:"found"`
	Message       string `json:"message,omitempty"`
	PatientName   string `json:"patient_name,omitempty"`
	DoctorName    string `json:"doctor_name,omitempty"`
	VisitTime     string `json:"visit_time,omitempty"`
	AheadCount    int    `json:"ahead_count"`    // چند نفر جلوتر از بیمار هستند
	TotalInQueue  int    `json:"total_in_queue"` // کل افراد در صف همان پزشک
	TimeNotPassed bool   `json:"time_not_passed"`
	UpdatedAtUnix int64  `json:"updated_at_unix"`
}
