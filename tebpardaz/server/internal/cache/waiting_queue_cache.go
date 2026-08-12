package cache

import (
	"fmt"
	"strings"
	"time"

	"tebpardaz/shared/protocol"
)

const (
	// WaitingQueueTTL مدت نگهداری صف انتظار در کش (۲ ساعت؛ کلاینت هر ۱۰ دقیقه یا در حالت زنده هر ۵ ثانیه تازه می‌کند).
	WaitingQueueTTL = 2 * time.Hour
	// waitingQueueCleanup فاصله پاکسازی آیتم‌های منقضی‌شده.
	waitingQueueCleanup = 30 * time.Minute
)

// ClinicWaitingQueue کیف صف انتظار یک مرکز در حافظه است.
type ClinicWaitingQueue struct {
	Doctors   []protocol.WaitingQueueDoctorDTO `json:"doctors"`
	UpdatedAt time.Time                        `json:"updated_at"`
}

// WaitingQueueCache مخزن در‌حافظه‌ای صف انتظار بیماران (بدون دیتابیس).
type WaitingQueueCache struct {
	store *Store
}

// NewWaitingQueueCache یک WaitingQueueCache روی Store موجود یا Store اختصاصی می‌سازد.
// ورودی: store (می‌تواند nil باشد).
// خروجی: اشاره‌گر به WaitingQueueCache آماده استفاده.
func NewWaitingQueueCache(store *Store) *WaitingQueueCache {
	if store == nil {
		store = New(WaitingQueueTTL, waitingQueueCleanup)
	}
	return &WaitingQueueCache{store: store}
}

// clinicWaitingQueueKey کلید کش صف انتظار یک مرکز را برمی‌گرداند.
func clinicWaitingQueueKey(clinicID uint) string {
	return fmt.Sprintf("waiting_queue:%d", clinicID)
}

// ReplaceFromPush صف انتظار یک مرکز را با دادهٔ جدید جایگزین می‌کند.
// ورودی: clinicID و payload پوش از کلاینت.
// خروجی: تعداد بیمار پذیرفته‌شده.
func (c *WaitingQueueCache) ReplaceFromPush(clinicID uint, push *protocol.WaitingQueueListPush) int {
	if c == nil || c.store == nil || clinicID == 0 || push == nil {
		return 0
	}
	doctors := make([]protocol.WaitingQueueDoctorDTO, 0, len(push.Doctors))
	accepted := 0
	for _, doc := range push.Doctors {
		patients := make([]protocol.WaitingQueuePatientDTO, 0, len(doc.Patients))
		patients = append(patients, doc.Patients...)
		accepted += len(patients)
		doctors = append(doctors, protocol.WaitingQueueDoctorDTO{
			DoctorName: doc.DoctorName,
			DoctorCode: doc.DoctorCode,
			Patients:   patients,
		})
	}
	bag := ClinicWaitingQueue{
		Doctors:   doctors,
		UpdatedAt: time.Now(),
	}
	c.store.SetWithTTL(clinicWaitingQueueKey(clinicID), bag, WaitingQueueTTL)
	return accepted
}

// Get صف انتظار یک مرکز را از کش می‌خواند.
// ورودی: clinicID.
// خروجی: داده مرکز و true در صورت وجود؛ در غیر این صورت صفر و false.
func (c *WaitingQueueCache) Get(clinicID uint) (ClinicWaitingQueue, bool) {
	if c == nil || c.store == nil || clinicID == 0 {
		return ClinicWaitingQueue{}, false
	}
	raw, ok := c.store.Get(clinicWaitingQueueKey(clinicID))
	if !ok {
		return ClinicWaitingQueue{}, false
	}
	bag, ok := raw.(ClinicWaitingQueue)
	if !ok {
		return ClinicWaitingQueue{}, false
	}
	return bag, true
}

// LookupPatient بیمار را با شماره پذیرش در صف مرکز پیدا می‌کند.
// ورودی: clinicID، کد ملی (اختیاری)، شماره پذیرش.
// خروجی: وضعیت نمایشی بیمار.
// قاعده امنیتی: اگر پذیرش کد ملی دارد، کد ملی ورودی باید دقیقاً همان باشد
// (تا نتوان با حدس شماره پذیرش پشت‌سرهم، نوبت دیگران را دید).
// اگر پذیرش کد ملی ندارد، فقط با کد ملی خالی مجاز است.
func (c *WaitingQueueCache) LookupPatient(clinicID uint, nationalID string, admissionNo int) protocol.WaitingQueueStatus {
	status := protocol.WaitingQueueStatus{
		Found:         false,
		TimeNotPassed: false,
	}
	bag, ok := c.Get(clinicID)
	if !ok {
		status.Message = "اطلاعات صف هنوز دریافت نشده؛ چند لحظه بعد دوباره تلاش کنید"
		return status
	}
	status.UpdatedAtUnix = bag.UpdatedAt.Unix()

	nationalID = normalizeNationalIDParam(nationalID)
	for _, doc := range bag.Doctors {
		for idx, patient := range doc.Patients {
			if patient.PatientCode != admissionNo {
				continue
			}
			storedNID := normalizeNationalIDParam(patient.NationalID)
			if !nationalIDMatchesAdmission(storedNID, nationalID) {
				// پذیرش پیدا شد ولی کد ملی مطابقت ندارد → عمداً «یافت نشد» تا لو نرود.
				status.Message = "بیماری با این مشخصات در صف انتظار یافت نشد"
				return status
			}
			timeNotPassed := visitTimeNotPassed(patient.VisitTime)
			if !timeNotPassed {
				status.Message = "زمان نوبت شما گذشته است؛ لطفاً به پذیرش مراجعه کنید"
				return status
			}
			status.Found = true
			status.TimeNotPassed = true
			status.PatientName = patient.PatientName
			status.DoctorName = patient.DoctorName
			status.VisitTime = patient.VisitTime
			status.TotalInQueue = len(doc.Patients)
			status.AheadCount = idx
			return status
		}
	}
	status.Message = "بیماری با این مشخصات در صف انتظار یافت نشد"
	return status
}

// normalizeNationalIDParam ارقام را یکسان می‌کند و placeholder خالی (- / _ / 0) را تهی می‌سازد.
func normalizeNationalIDParam(raw string) string {
	raw = normalizeDigits(raw)
	switch raw {
	case "-", "_", "0", "":
		return ""
	default:
		return raw
	}
}

// nationalIDMatchesAdmission قاعده دسترسی بر اساس وجود کد ملی روی پذیرش را اعمال می‌کند.
// ورودی: کد ملی ذخیره‌شده روی پذیرش، کد ملی ارسال‌شده توسط کاربر.
// خروجی: true اگر مجاز به مشاهده باشد.
func nationalIDMatchesAdmission(storedNID, providedNID string) bool {
	if storedNID != "" {
		// پذیرش دارای کد ملی است → باید دقیقاً همان ارسال شود.
		return providedNID == storedNID
	}
	// پذیرش بدون کد ملی → فقط با کد ملی خالی (لینک QR بدون NID) مجاز است.
	return providedNID == ""
}

// normalizeDigits فقط رقم‌های فارسی/عربی را به انگلیسی تبدیل می‌کند.
func normalizeDigits(raw string) string {
	raw = strings.TrimSpace(raw)
	var b strings.Builder
	for _, r := range raw {
		switch r {
		case '۰', '٠':
			b.WriteByte('0')
		case '۱', '١':
			b.WriteByte('1')
		case '۲', '٢':
			b.WriteByte('2')
		case '۳', '٣':
			b.WriteByte('3')
		case '۴', '٤':
			b.WriteByte('4')
		case '۵', '٥':
			b.WriteByte('5')
		case '۶', '٦':
			b.WriteByte('6')
		case '۷', '٧':
			b.WriteByte('7')
		case '۸', '٨':
			b.WriteByte('8')
		case '۹', '٩':
			b.WriteByte('9')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// visitTimeNotPassed بررسی می‌کند ساعت نوبت (HH:MM) هنوز نرسیده یا در همان دقیقه است.
// ورودی: visitTime رشته ساعت از HIS.
// خروجی: true اگر زمان فعلی قبل از پایان پنجره ۳۰ دقیقه‌ای نوبت باشد.
func visitTimeNotPassed(visitTime string) bool {
	visitTime = strings.TrimSpace(visitTime)
	if visitTime == "" {
		return true
	}
	parts := strings.Split(visitTime, ":")
	if len(parts) < 2 {
		return true
	}
	now := time.Now()
	hour, min := 0, 0
	fmt.Sscanf(parts[0], "%d", &hour)
	fmt.Sscanf(parts[1], "%d", &min)
	slot := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
	// تا ۳۰ دقیقه بعد از ساعت نوبت هم هنوز «زمان نگذشته» در نظر گرفته می‌شود.
	return !now.After(slot.Add(30 * time.Minute))
}
