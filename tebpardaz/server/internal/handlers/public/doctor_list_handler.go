package public

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/components"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	ptime "github.com/yaa110/go-persian-calendar"
)

const doctorListPageSize = 12

// DoctorListHandler لیست عمومی پزشکان برای رزرو را سرو می‌کند.
// نزدیک‌ترین نوبت هر پزشک از کش اسلات‌ها خوانده می‌شود (نه از دیتابیس).
type DoctorListHandler struct {
	Listing *booking.ListingService
	Clinics *repository.ClinicRepo
}

// NewDoctorListHandler سازنده DoctorListHandler است.
// ورودی: سرویس لیست‌بندی (با SlotSource مبتنی بر کش)، ریپوی مراکز.
// خروجی: اشاره‌گر به DoctorListHandler.
func NewDoctorListHandler(listing *booking.ListingService, clinics *repository.ClinicRepo) *DoctorListHandler {
	return &DoctorListHandler{Listing: listing, Clinics: clinics}
}

// Get لیست فیلتر‌شده پزشکان را برای مستأجر فعلی رندر می‌کند.
// ورودی: queryهای q، specialty_id، clinic_id، date (شمسی یا میلادی)، page.
// خروجی: HTML صفحه لیست؛ نوبت‌ها از کش (TTL چهار ساعته، بروزرسانی هر ۱ دقیقه توسط کلاینت).
func (h *DoctorListHandler) Get(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	if tc.Layout != constants.LayoutOrgan && tc.Layout != constants.LayoutPrivate && tc.Layout != constants.LayoutPlatform {
		c.Status(http.StatusNotFound)
		return
	}

	clinicIDs, showClinicBadge, err := resolveTenantClinicIDs(tc, h.Clinics)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	filter := booking.ListFilter{
		ClinicIDs:   clinicIDs,
		Query:       strings.TrimSpace(c.Query("q")),
		SpecialtyID: parseUintQuery(c.Query("specialty_id")),
		ClinicID:    parseUintQuery(c.Query("clinic_id")),
		Date:        parseDateQuery(c.Query("date")),
		Layout:      tc.Layout,
	}

	result, err := h.Listing.ListDoctors(filter, showClinicBadge)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	page, totalPages, pageDoctors := paginateDoctorCards(result.Doctors, parsePageQuery(c.Query("page")), doctorListPageSize)

	view := pages.DoctorListView{
		Doctors:          toDoctorCards(pageDoctors),
		Specialties:      toFilterOptions(result.Specialties),
		Clinics:          toClinicFilterOptions(result.Clinics),
		ShowClinicFilter: result.ShowClinicFilter,
		Query:            filter.Query,
		Date:             formatShamsiDateQuery(filter.Date),
		SpecialtyID:      filter.SpecialtyID,
		ClinicID:         filter.ClinicID,
		FormAction:       "/doctors",
		EmptyMessage:     "پزشکی با این فیلترها یافت نشد.",
		Page:             page,
		TotalPages:       totalPages,
		TotalCount:       len(result.Doctors),
		PageSize:         doctorListPageSize,
	}
	renderPublicLayout(c, tc, pages.DoctorList(view), "doctors")
}

// paginateDoctorCards slices doctors for the requested page.
// Input: all cards, 1-based page, page size. Output: clamped page, total pages, page slice.
func paginateDoctorCards(all []booking.DoctorCard, page, pageSize int) (int, int, []booking.DoctorCard) {
	if pageSize < 1 {
		pageSize = doctorListPageSize
	}
	total := len(all)
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	if page < 1 {
		page = 1
	}
	if totalPages == 0 {
		return 1, 0, all
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > total {
		end = total
	}
	return page, totalPages, all[start:end]
}

func toDoctorCards(items []booking.DoctorCard) []components.DoctorCardView {
	out := make([]components.DoctorCardView, 0, len(items))
	for _, item := range items {
		out = append(out, components.DoctorCardView{
			Name:            item.Name,
			SpecialtyName:   item.SpecialtyName,
			DoctorSystemID:  item.DoctorSystemID,
			PhotoURL:        item.PhotoURL,
			ClinicName:      item.ClinicName,
			ShowClinicBadge: item.ShowClinicBadge,
			HasSlot:         item.HasSlot,
			NearestStartsAt: item.NearestStartsAt,
			BookingURL:      item.BookingURL,
		})
	}
	return out
}

func toFilterOptions(items []booking.SpecialtyOption) []pages.DoctorListFilterOption {
	out := make([]pages.DoctorListFilterOption, 0, len(items))
	for _, item := range items {
		out = append(out, pages.DoctorListFilterOption{ID: item.ID, Name: item.Name})
	}
	return out
}

func toClinicFilterOptions(items []booking.ClinicOption) []pages.DoctorListFilterOption {
	out := make([]pages.DoctorListFilterOption, 0, len(items))
	for _, item := range items {
		out = append(out, pages.DoctorListFilterOption{ID: item.ID, Name: item.Name})
	}
	return out
}

func parseUintQuery(raw string) uint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return uint(n)
}

// parsePageQuery reads a 1-based page number from a query string.
// Input: raw page query. Output: page >= 1 (defaults to 1).
func parsePageQuery(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 1
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// parseDateQuery converts Shamsi YYYY/MM/DD (persian-datepicker) or Gregorian yyyy-MM-dd to local Time.
// Only today through today+15 (inclusive) are accepted; otherwise zero Time.
// Input: raw date query. Output: local midnight Time, or zero when empty/invalid/out of range.
func parseDateQuery(raw string) time.Time {
	raw = normalizeDigits(strings.TrimSpace(raw))
	if raw == "" {
		return time.Time{}
	}
	raw = strings.ReplaceAll(raw, "-", "/")
	parts := strings.Split(raw, "/")
	if len(parts) != 3 {
		return time.Time{}
	}
	y, errY := strconv.Atoi(parts[0])
	m, errM := strconv.Atoi(parts[1])
	d, errD := strconv.Atoi(parts[2])
	if errY != nil || errM != nil || errD != nil {
		return time.Time{}
	}
	var t time.Time
	// Shamsi years are typically 1300–1500 in this product's lifetime.
	if y >= 1200 && y <= 1600 {
		pt := ptime.Date(y, ptime.Month(m), d, 0, 0, 0, 0, time.Local)
		t = pt.Time() // Gregorian
	} else {
		parsed, err := time.ParseInLocation("2006/01/02", raw, time.Local)
		if err != nil {
			return time.Time{}
		}
		t = parsed
	}
	if t.IsZero() {
		return time.Time{}
	}
	return clampBookingFilterDate(t)
}

const doctorListMaxFutureDays = 15

// clampBookingFilterDate keeps only dates from today through today+15 days.
// Input: parsed local date. Output: same day at midnight, or zero when outside the window.
func clampBookingFilterDate(t time.Time) time.Time {
	now := time.Now()
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	max := today.AddDate(0, 0, doctorListMaxFutureDays)
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	if day.Before(today) || day.After(max) {
		return time.Time{}
	}
	return day
}

// normalizeDigits maps Persian/Arabic-Indic digits to ASCII 0-9.
// Input: mixed-digit string. Output: ASCII-digit string.
func normalizeDigits(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= '۰' && r <= '۹':
			b.WriteRune('0' + (r - '۰'))
		case r >= '٠' && r <= '٩':
			b.WriteRune('0' + (r - '٠'))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// formatShamsiDateQuery formats a filter date for the Shamsi date input.
// Input: Gregorian/local time. Output: yyyy/MM/dd Jalali, or empty when zero.
func formatShamsiDateQuery(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return ptime.New(t).Format("yyyy/MM/dd")
}
