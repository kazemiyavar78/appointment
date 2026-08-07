package public

import (
	"net/http"
	"strings"
	"time"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

var bookingUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		return strings.Contains(origin, r.Host)
	},
}

// bookingWSSubmit is the first (and only) client message on the public booking WebSocket.
// Fields mirror the HTML form; booking data is accepted only from this form-shaped payload.
type bookingWSSubmit struct {
	CSRFToken  string `json:"csrf_token"`
	SlotID     string `json:"slot_id"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	NationalID string `json:"national_id"`
	Mobile     string `json:"mobile"`
	BirthDate  string `json:"birth_date"`
	Sex        string `json:"sex"`
}

// ServeBookingWS keeps one WebSocket open from form submit until the clinic result is reported.
// Protocol: client sends one JSON form payload; server streams ProgressEvent until Done=true.
func (h *BookingHandler) ServeBookingWS(c *gin.Context) {
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

	clinicID, doctorSlugResolved, err := h.resolveBookingTarget(tc, clinicSlug, doctorSlug)
	if err != nil || clinicID == 0 {
		c.Status(http.StatusNotFound)
		return
	}

	doctor, err := h.Doctors.GetPublicByClinicAndSlug(clinicID, doctorSlugResolved)
	if err != nil || doctor == nil {
		c.Status(http.StatusNotFound)
		return
	}
	_ = h.Doctors.EnsureSlug(doctor)

	ip := c.ClientIP()
	if h.Guard != nil {
		if allowed, reason := h.Guard.Allow(ip); !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{"ok": false, "message": reason})
			return
		}
	}

	conn, err := bookingUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	_ = conn.SetWriteDeadline(time.Now().Add(90 * time.Second))

	emit := func(ev booking.ProgressEvent) {
		_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		_ = conn.WriteJSON(ev)
	}

	var submit bookingWSSubmit
	if err := conn.ReadJSON(&submit); err != nil {
		emit(booking.NewFinal(false, "دریافت اطلاعات فرم ناموفق بود"))
		return
	}

	if h.CSRF == nil || !h.CSRF.Verify(c.Request, submit.CSRFToken) {
		emit(booking.NewFinal(false, "توکن امنیتی نامعتبر است؛ صفحه را تازه کنید"))
		return
	}

	if h.Bookings == nil {
		emit(booking.NewFinal(false, "سرویس نوبت در دسترس نیست"))
		return
	}

	result, bookErr := h.Bookings.Book(booking.BookRequest{
		ClinicID:   clinicID,
		Doctor:     doctor,
		ClientIP:   ip,
		SlotID:     submit.SlotID,
		FirstName:  submit.FirstName,
		LastName:   submit.LastName,
		NationalID: submit.NationalID,
		Mobile:     submit.Mobile,
		BirthDate:  submit.BirthDate,
		Sex:        submit.Sex,
	}, emit)

	if result == nil {
		msg := "خطای داخلی"
		if bookErr != nil {
			msg = bookErr.Error()
		}
		emit(booking.NewFinal(false, msg))
		return
	}
	emit(booking.NewFinal(result.OK, result.Message))
}

// resolveBookingTarget resolves clinic ID and doctor slug for the current tenant layout.
func (h *BookingHandler) resolveBookingTarget(tc *tenant.Context, clinicSlug, doctorSlug string) (clinicID uint, doctorSlugOut string, err error) {
	switch {
	case doctorSlug == "" && tc.Layout == constants.LayoutPrivate && tc.ClinicID != nil:
		return *tc.ClinicID, clinicSlug, nil
	case doctorSlug != "" && tc.Layout == constants.LayoutOrgan:
		clinic, e := h.resolveOrganClinic(tc, clinicSlug)
		if e != nil || clinic == nil {
			return 0, "", e
		}
		return clinic.ID, doctorSlug, nil
	case doctorSlug != "" && tc.Layout == constants.LayoutPlatform:
		clinic, e := h.Clinics.GetBySlug(clinicSlug)
		if e != nil || clinic == nil {
			return 0, "", e
		}
		return clinic.ID, doctorSlug, nil
	default:
		return 0, "", gorm.ErrRecordNotFound
	}
}
