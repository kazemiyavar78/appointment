package pages

import "tebpardaz/server/views/components"

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
	ClinicName    string
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
