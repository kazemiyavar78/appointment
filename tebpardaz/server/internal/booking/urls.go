package booking

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"tebpardaz/server/internal/models"
	"tebpardaz/shared/constants"
)

// BuildBookingURL مسیر عمومی صفحه رزرو را برای layout فعلی می‌سازد.
// ورودی: نوع layout، اسلاگ/کلید مرکز (برای ارگان/پلتفرم)، اسلاگ پزشک.
// خروجی: مسیر نسبی رزرو.
func BuildBookingURL(layout constants.LayoutKind, clinicSlug, doctorSlug string) string {
	doctorSlug = strings.Trim(doctorSlug, "/")
	clinicSlug = strings.Trim(clinicSlug, "/")
	if doctorSlug == "" {
		return "/doctors"
	}
	switch layout {
	case constants.LayoutPrivate:
		return "/booking/" + doctorSlug
	case constants.LayoutOrgan, constants.LayoutPlatform:
		if clinicSlug == "" {
			return "/doctors"
		}
		return "/booking/" + clinicSlug + "/" + doctorSlug
	default:
		return "/doctors"
	}
}

// ClinicPathKey بخش مسیر مرکز را برای URL رزرو برمی‌گرداند.
// اگر slug خالی باشد از c{id} استفاده می‌شود تا لینک نشکند.
func ClinicPathKey(clinic *models.Clinic) string {
	if clinic == nil || clinic.ID == 0 {
		return ""
	}
	if clinic.Slug != nil {
		if s := strings.TrimSpace(*clinic.Slug); s != "" {
			return s
		}
	}
	return fmt.Sprintf("c%d", clinic.ID)
}

// ParseClinicPathKey شناسه مرکز را از کلید مسیر (slug یا c{id}) استخراج می‌کند.
// ورودی: بخش مسیر. خروجی: clinicID وقتی الگوی c{id} باشد، وگرنه ۰.
func ParseClinicPathKey(segment string) uint {
	segment = strings.TrimSpace(segment)
	if !strings.HasPrefix(segment, "c") || len(segment) < 2 {
		return 0
	}
	n, err := strconv.ParseUint(segment[1:], 10, 64)
	if err != nil || n == 0 {
		return 0
	}
	return uint(n)
}

// WithReturn پارامتر return امن را به URL رزرو اضافه می‌کند تا فیلتر لیست حفظ شود.
// ورودی: URL رزرو و مسیر بازگشت نسبی. خروجی: URL با query return.
func WithReturn(bookingURL, returnPath string) string {
	bookingURL = strings.TrimSpace(bookingURL)
	returnPath = SafeReturnPath(returnPath)
	// اگر URL رزرو ساخته نشده، return را به /doctors نچسبان
	if bookingURL == "" || bookingURL == "/doctors" || returnPath == "" {
		return bookingURL
	}
	sep := "?"
	if strings.Contains(bookingURL, "?") {
		sep = "&"
	}
	return bookingURL + sep + "return=" + url.QueryEscape(returnPath)
}

// SafeReturnPath فقط مسیرهای نسبی امن فهرست پزشکان را می‌پذیرد.
// ورودی: مقدار خام query. خروجی: مسیر امن یا خالی.
func SafeReturnPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") || strings.HasPrefix(raw, "//") {
		return ""
	}
	if !strings.HasPrefix(raw, "/doctors") {
		return ""
	}
	if strings.ContainsAny(raw, " \t\n\r") {
		return ""
	}
	return raw
}
