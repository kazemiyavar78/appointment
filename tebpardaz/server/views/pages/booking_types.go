package pages

import (
	"strings"

	"tebpardaz/server/views/components"
)

// bookingPhotoAlt متن alt عکس پزشک را از نام نمایشی می‌سازد.
// ورودی: نام پزشک. خروجی: همان نام، یا «پزشک» اگر خالی باشد.
func bookingPhotoAlt(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "پزشک"
	}
	return name
}

// InsuranceBadgeItem مشخصات یک بیمه پوشش‌دهنده خدمت را نگه می‌دارد.
type InsuranceBadgeItem struct {
	ID      uint
	Name    string
	LogoURL string
}

// DoctorServiceItemView اطلاعات یک خدمت ارائه شده توسط پزشک همراه با لیست بیمه‌های پوشش‌دهنده آن را نگه می‌دارد.
type DoctorServiceItemView struct {
	ID          uint
	Name        string
	Description string
	Insurances  []InsuranceBadgeItem
}

// BookingView دادهٔ صفحه عمومی رزرو پزشک را نگه می‌دارد.
type BookingView struct {
	DoctorName    string
	SpecialtyName string
	SpecialtyURL  string
	ClinicName    string
	ClinicURL     string
	ClinicPath    string // slug یا c{id} برای OTP در لایوت ارگان/پلتفرم
	PhotoURL      string
	LongDesc      string
	ShowClinic    bool
	BackURL       string
	SubmitURL     string
	WSURL         string
	CSRFToken     string
	Slots         components.SlotPickerView
	ErrorMessage  string
	Reviews       components.ReviewSectionView
	Services      []DoctorServiceItemView
}
