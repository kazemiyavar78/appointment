package public

import (
	"net/http"
	"strings"
	"time"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/csrf"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/components"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	ptime "github.com/yaa110/go-persian-calendar"
	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
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
}

// NewBookingHandler constructs a BookingHandler.
// Inputs: doctor/clinic repos, slot cache, booking service, CSRF manager, rate/abuse guard.
// Output: pointer to BookingHandler.
func NewBookingHandler(
	doctors *repository.DoctorRepo,
	clinics *repository.ClinicRepo,
	slots *cache.SlotCache,
	bookings *booking.Service,
	csrfMgr *csrf.Manager,
	guard *booking.Guard,
) *BookingHandler {
	return &BookingHandler{
		Doctors:  doctors,
		Clinics:  clinics,
		Slots:    slots,
		Bookings: bookings,
		CSRF:     csrfMgr,
		Guard:    guard,
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
		clinic, err := h.Clinics.GetBySlug(clinicSlug)
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

// resolveOrganClinic loads a clinic by slug and checks organization membership.
func (h *BookingHandler) resolveOrganClinic(tc *tenant.Context, clinicSlug string) (*models.Clinic, error) {
	if tc.OrganizationID == nil || h.Clinics == nil {
		return nil, gorm.ErrRecordNotFound
	}
	clinic, err := h.Clinics.GetBySlug(clinicSlug)
	if err != nil || clinic == nil {
		return nil, err
	}
	if clinic.OrganizationID != *tc.OrganizationID {
		return nil, gorm.ErrRecordNotFound
	}
	return clinic, nil
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
	submitURL := booking.BuildBookingURL(tc.Layout, clinicSlugFromClinicID(h.Clinics, clinicID), doctor.Slug)
	slots := []models.DoctorSlot{}
	if h.Slots != nil {
		slots = h.Slots.AvailableForDoctor(clinicID, doctor.ID, time.Now(), 40)
	}

	view := pages.BookingView{
		DoctorName:    doctorDisplayName(*doctor),
		SpecialtyName: doctor.Specialty.Name,
		ClinicName:    clinicName,
		PhotoURL:      doctor.PhotoURL,
		ShowClinic:    showClinic,
		BackURL:       "/doctors",
		SubmitURL:     submitURL,
		WSURL:         wsURL,
		CSRFToken:     csrfToken,
		Slots: components.SlotPickerView{
			InputName: "slot_id",
			Options:   toSlotOptions(slots),
			EmptyText: "هنوز نوبت آزادی برای این پزشک ثبت نشده است.",
		},
	}
	h.renderWithLayout(c, tc, pages.Booking(view), "booking")
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
	if err != nil || c == nil || c.Slug == nil {
		return ""
	}
	return strings.TrimSpace(*c.Slug)
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
			Value: value,
			Date:  date,
			Time:  timeLabel,
			Label: date + " " + timeLabel,
		})
	}
	return out
}
