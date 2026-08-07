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
)

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
// ورودی: queryهای q، specialty_id، clinic_id، date.
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

	view := pages.DoctorListView{
		Doctors:          toDoctorCards(result.Doctors),
		Specialties:      toFilterOptions(result.Specialties),
		Clinics:          toClinicFilterOptions(result.Clinics),
		ShowClinicFilter: result.ShowClinicFilter,
		Query:            filter.Query,
		Date:             formatDateQuery(filter.Date),
		SpecialtyID:      filter.SpecialtyID,
		ClinicID:         filter.ClinicID,
		FormAction:       "/doctors",
		EmptyMessage:     "پزشکی با این فیلترها یافت نشد.",
	}
	renderPublicLayout(c, tc, pages.DoctorList(view), "doctors")
}

func toDoctorCards(items []booking.DoctorCard) []components.DoctorCardView {
	out := make([]components.DoctorCardView, 0, len(items))
	for _, item := range items {
		out = append(out, components.DoctorCardView{
			Name:            item.Name,
			SpecialtyName:   item.SpecialtyName,
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

func parseDateQuery(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	t, err := time.ParseInLocation("2006-01-02", raw, time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}

func formatDateQuery(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}
