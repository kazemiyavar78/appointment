package public

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/csrf"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/sms"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/components"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	ptime "github.com/yaa110/go-persian-calendar"
	"gorm.io/gorm"
)

// BookingHandler serves dedicated public booking pages per tenant layout.
type BookingHandler struct {
	Doctors  *repository.DoctorRepo
	Clinics  *repository.ClinicRepo
	Slots    *cache.SlotCache
	Bookings *booking.Service
	CSRF     *csrf.Manager
	Guard    *booking.Guard
	Reviews  *repository.ReviewRepo
	OTP      *booking.OTPStore
	SMS      *sms.Client
	Services *repository.ServiceRepo
}

// NewBookingHandler constructs a BookingHandler.
func NewBookingHandler(
	doctors *repository.DoctorRepo,
	clinics *repository.ClinicRepo,
	slots *cache.SlotCache,
	bookings *booking.Service,
	csrfMgr *csrf.Manager,
	guard *booking.Guard,
	reviews *repository.ReviewRepo,
	otp *booking.OTPStore,
	smsClient *sms.Client,
	services *repository.ServiceRepo,
) *BookingHandler {
	return &BookingHandler{
		Doctors:  doctors,
		Clinics:  clinics,
		Slots:    slots,
		Bookings: bookings,
		CSRF:     csrfMgr,
		Guard:    guard,
		Reviews:  reviews,
		OTP:      otp,
		SMS:      smsClient,
		Services: services,
	}
}

// Get renders the booking page from /booking/*path.
// Path shapes:
//   - private:  /booking/{doctorSlug}
//   - organ/platform: /booking/{clinicSlug}/{doctorSlug}
func (h *BookingHandler) Get(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	clinicSlug, doctorSlug, ok := bookingPathSlugs(c.Param("path"))
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}

	switch {
	case doctorSlug == "" && tc.Layout == constants.LayoutPrivate && tc.ClinicID != nil:
		h.renderBooking(c, tc, *tc.ClinicID, clinicName(tc), clinicSlug, false)
	case doctorSlug != "" && tc.Layout == constants.LayoutOrgan:
		clinic, err := h.resolveOrganClinic(tc, clinicSlug)
		if err != nil || clinic == nil {
			c.Status(http.StatusNotFound)
			return
		}
		h.renderBooking(c, tc, clinic.ID, clinic.Name, doctorSlug, true)
	case doctorSlug != "" && tc.Layout == constants.LayoutPlatform:
		clinic, err := h.resolveClinicByPathKey(clinicSlug)
		if err != nil || clinic == nil {
			c.Status(http.StatusNotFound)
			return
		}
		h.renderBooking(c, tc, clinic.ID, clinic.Name, doctorSlug, true)
	default:
		c.Status(http.StatusNotFound)
	}
}

// PostSubmit rejects classic HTTP form posts so refresh cannot resubmit; booking uses WebSocket only.
func (h *BookingHandler) PostSubmit(c *gin.Context) {
	c.Header("Allow", "GET")
	c.JSON(http.StatusMethodNotAllowed, gin.H{
		"ok":      false,
		"message": "ثبت نوبت فقط از طریق فرم صفحه (WebSocket) انجام می‌شود",
	})
}

// bookingPathSlugs parses /booking/*path into clinic/doctor segments.
// Inputs: catch-all path from Gin (may include a leading slash).
// Output: clinicSlug, doctorSlug (empty when private one-segment), and ok=false when invalid.
func bookingPathSlugs(raw string) (clinicSlug, doctorSlug string, ok bool) {
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return "", "", false
	}
	parts := strings.Split(raw, "/")
	switch len(parts) {
	case 1:
		if strings.TrimSpace(parts[0]) == "" {
			return "", "", false
		}
		return strings.TrimSpace(parts[0]), "", true
	case 2:
		a, b := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if a == "" || b == "" {
			return "", "", false
		}
		return a, b, true
	default:
		return "", "", false
	}
}

// resolveOrganClinic loads a clinic by slug/path-key and checks organization membership.
func (h *BookingHandler) resolveOrganClinic(tc *tenant.Context, clinicKey string) (*models.Clinic, error) {
	if tc.OrganizationID == nil || h.Clinics == nil {
		return nil, gorm.ErrRecordNotFound
	}
	clinic, err := h.resolveClinicByPathKey(clinicKey)
	if err != nil || clinic == nil {
		return nil, err
	}
	if clinic.OrganizationID != *tc.OrganizationID {
		return nil, gorm.ErrRecordNotFound
	}
	return clinic, nil
}

// resolveClinicByPathKey مرکز را با slug یا کلید c{id} پیدا می‌کند.
func (h *BookingHandler) resolveClinicByPathKey(key string) (*models.Clinic, error) {
	if h.Clinics == nil {
		return nil, gorm.ErrRecordNotFound
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, gorm.ErrRecordNotFound
	}
	if id := booking.ParseClinicPathKey(key); id > 0 {
		return h.Clinics.GetByID(id)
	}
	return h.Clinics.GetBySlug(key)
}

func (h *BookingHandler) renderBooking(
	c *gin.Context,
	tc *tenant.Context,
	clinicID uint,
	clinicName string,
	doctorSlug string,
	showClinic bool,
) {
	if doctorSlug == "" || h.Doctors == nil {
		c.Status(http.StatusNotFound)
		return
	}
	doctor, err := h.Doctors.GetPublicByClinicAndSlug(clinicID, doctorSlug)
	if err != nil || doctor == nil {
		c.Status(http.StatusNotFound)
		return
	}
	_ = h.Doctors.EnsureSlug(doctor)

	csrfToken := ""
	if h.CSRF != nil {
		if tok, err := h.CSRF.Issue(c.Writer); err == nil {
			csrfToken = tok
		}
	}

	path := strings.Trim(c.Param("path"), "/")
	wsURL := "/ws/booking/" + path
	clinicPathKey := clinicSlugFromClinicID(h.Clinics, clinicID)
	submitURL := booking.BuildBookingURL(tc.Layout, clinicPathKey, doctor.Slug)
	slots := []models.DoctorSlot{}
	if h.Slots != nil {
		slots = h.Slots.AvailableForDoctor(clinicID, doctor.ID, time.Now(), 40)
	}
	

	backURL := booking.SafeReturnPath(c.Query("return"))
	if backURL == "" {
		backURL = "/doctors"
	}

	reviews := h.buildDoctorReviews(c, doctor, csrfToken)
	services := h.buildDoctorServices(clinicID, doctor.ID)
	view := pages.BookingView{
		DoctorName:    doctorDisplayName(*doctor),
		SpecialtyName: doctor.Specialty.Name,
		ClinicName:    clinicName,
		ClinicPath:    clinicPathKey,
		PhotoURL:      resolveDoctorPhoto(doctor),
		LongDesc:      strings.TrimSpace(doctor.LongDesc),
		ShowClinic:    showClinic,
		BackURL:       backURL,
		SubmitURL:     submitURL,
		WSURL:         wsURL,
		CSRFToken:     csrfToken,
		Slots: components.SlotPickerView{
			InputName: "slot_id",
			Options:   toSlotOptions(slots),
			EmptyText: "هنوز نوبت آزادی برای این پزشک ثبت نشده است.",
		},
		Reviews:  reviews,
		Services: services,
	}
	h.renderWithLayout(c, tc, pages.Booking(view), "booking")
}

// buildDoctorServices لیست خدمات ارائه شده توسط پزشک و بیمه‌های پوشش‌دهنده در مرکز را استخراج می‌کند.
// ورودی: clinicID شناسه مرکز درمانی، doctorID شناسه پزشک.
// خروجی: اسلایس DoctorServiceItemView برای نمایش در صفحه رزرو.
func (h *BookingHandler) buildDoctorServices(clinicID uint, doctorID uint) []pages.DoctorServiceItemView {
	if h.Services == nil || doctorID == 0 {
		return nil
	}
	items, err := h.Services.GetDoctorServicesWithInsurances(clinicID, doctorID)
	if err != nil || len(items) == 0 {
		return nil
	}
	out := make([]pages.DoctorServiceItemView, 0, len(items))
	for _, item := range items {
		insList := make([]pages.InsuranceBadgeItem, 0, len(item.Insurances))
		for _, ins := range item.Insurances {
			insList = append(insList, pages.InsuranceBadgeItem{
				ID:      ins.ID,
				Name:    ins.Name,
				LogoURL: ins.LogoURL,
			})
		}
		out = append(out, pages.DoctorServiceItemView{
			ID:          item.Service.ID,
			Name:        item.Service.Name,
			Description: item.Service.Description,
			Insurances:  insList,
		})
	}
	return out
}

// resolveDoctorPhoto returns the best available photo URL for a doctor.
// Inputs: doc (*models.Doctor).
// Output: photo URL string.
func resolveDoctorPhoto(doc *models.Doctor) string {
	if doc == nil {
		return ""
	}
	if doc.Photo1200 != "" {
		return doc.Photo1200
	}
	if doc.Photo900 != "" {
		return doc.Photo900
	}
	if doc.Photo600 != "" {
		return doc.Photo600
	}
	if doc.Photo300 != "" {
		return doc.Photo300
	}
	return doc.PhotoURL
}

// buildDoctorReviews بلوک نظرات پزشک را برای صفحه رزرو می‌سازد.
// Inputs: gin context, doctor, csrfToken از قبل صادرشده برای همین صفحه (بدون Issue دوباره).
// Output: ReviewSectionView با همان توکن CSRF فرم رزرو.
func (h *BookingHandler) buildDoctorReviews(c *gin.Context, doctor *models.Doctor, csrfToken string) components.ReviewSectionView {
	view := components.ReviewSectionView{
		Title:       "نظرات درباره پزشک",
		TargetType:  models.ReviewTargetDoctor,
		TargetID:    strconv.FormatUint(uint64(doctor.ID), 10),
		ClinicID:    strconv.FormatUint(uint64(doctor.ClinicID), 10),
		FormAction:  "/reviews",
		RedirectURL: c.Request.URL.RequestURI(),
		CSRFToken:   csrfToken,
	}
	if flash := c.Query("review"); flash == "ok" {
		view.InfoMessage = "نظر شما ثبت شد و پس از تأیید نمایش داده می‌شود."
	} else if flash == "error" {
		view.ErrorMessage = "ثبت نظر ناموفق بود. امتیاز ۱ تا ۵ الزامی است."
	}
	if h.Reviews == nil {
		return view
	}
	sum, _ := h.Reviews.Summary(models.ReviewTargetDoctor, doctor.ID)
	view.Count = sum.Count
	view.Average = sum.Average
	rows, _ := h.Reviews.ListApprovedBodies(models.ReviewTargetDoctor, doctor.ID, 20)
	for _, row := range rows {
		name := row.AuthorName
		if name == "" {
			name = "کاربر"
		}
		view.Items = append(view.Items, components.ReviewItemView{
			AuthorName: name,
			Rating:     row.Rating,
			Body:       row.Body,
		})
	}
	return view
}

func (h *BookingHandler) renderWithLayout(c *gin.Context, tc *tenant.Context, child templ.Component, activeNav string) {
	renderPublicLayout(c, tc, child, activeNav)
}

func clinicName(tc *tenant.Context) string {
	if tc != nil && tc.Clinic != nil {
		return tc.Clinic.Name
	}
	return ""
}

func clinicSlugFromClinicID(clinics *repository.ClinicRepo, clinicID uint) string {
	if clinics == nil || clinicID == 0 {
		return ""
	}
	c, err := clinics.GetByID(clinicID)
	if err != nil || c == nil {
		return ""
	}
	return booking.ClinicPathKey(c)
}

func doctorDisplayName(d models.Doctor) string {
	if name := strings.TrimSpace(d.Name); name != "" {
		return name
	}
	return strings.TrimSpace(d.FirstName + " " + d.LastName)
}

func toSlotOptions(slots []models.DoctorSlot) []components.SlotOption {
	out := make([]components.SlotOption, 0, len(slots))
	for _, slot := range slots {
		value := strings.TrimSpace(slot.ExternalSlotID)
		if value == "" {
			value = slot.StartsAt.UTC().Format(time.RFC3339)
		}
		pt := ptime.New(slot.StartsAt)
		date := pt.Format("yyyy/MM/dd")
		timeLabel := pt.Format("HH:mm")
		out = append(out, components.SlotOption{
			Value:     value,
			Date:      date,
			Time:      timeLabel,
			Label:     date + " " + timeLabel,
			DayNum:    strconv.Itoa(pt.Day()),
			MonthAbbr: persianMonthAbbr(int(pt.Month())),
		})
	}
	return out
}

// persianMonthAbbr مخفف فارسی نام ماه شمسی را برمی‌گرداند.
func persianMonthAbbr(month int) string {
	abbrs := []string{"", "فرو", "ارد", "خرد", "تیر", "مرد", "شهر", "مهر", "آبان", "آذر", "دی", "بهم", "اسف"}
	if month < 1 || month > 12 {
		return ""
	}
	return abbrs[month]
}
