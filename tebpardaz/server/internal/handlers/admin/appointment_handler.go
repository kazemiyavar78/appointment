package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/websocket"
	adminviews "tebpardaz/server/views/admin"
	"tebpardaz/shared/protocol"

	"github.com/gin-gonic/gin"
)

const liveAppointmentListTimeout = 20 * time.Second

// AppointmentHandler صفحه ادمین نوبت‌های پزشک را مدیریت می‌کند (خواندن از کش، نه دیتابیس).
type AppointmentHandler struct {
	Doctors *repository.DoctorRepo
	Clinics *repository.ClinicRepo
	// Slots کش اسلات‌های رزرو با TTL چهار ساعته
	Slots *cache.SlotCache
	Scope *auth.ClinicScope
	Hub   *websocket.Hub
}

// NewAppointmentHandler سازنده AppointmentHandler است.
// ورودی: ریپوی پزشک/مرکز، کش اسلات، محدوده دسترسی، هاب WebSocket.
// خروجی: اشاره‌گر به AppointmentHandler.
func NewAppointmentHandler(
	doctors *repository.DoctorRepo,
	clinics *repository.ClinicRepo,
	slots *cache.SlotCache,
	scope *auth.ClinicScope,
	hub *websocket.Hub,
) *AppointmentHandler {
	return &AppointmentHandler{
		Doctors: doctors,
		Clinics: clinics,
		Slots:   slots,
		Scope:   scope,
		Hub:     hub,
	}
}

// List صفحه ادمین لیست نوبت‌ها را رندر می‌کند.
// ورودی: c (queryهای clinic_id، fetch=1، doctor_external_id، q).
// خروجی: HTML صفحه یا خطای HTTP.
// جریان: تعیین مرکز مجاز → (اختیاری) درخواست بروزرسانی از کلاینت → خواندن از کش → نمایش.
func (h *AppointmentHandler) List(c *gin.Context) {
	// --- احراز هویت ---
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// q = جستجو | fetch=1 = درخواست بروزرسانی زنده | doctor_external_id = فقط یک پزشک
	search := strings.TrimSpace(c.Query("q"))
	fetchLive := c.Query("fetch") == "1"
	doctorExternalID := strings.TrimSpace(c.Query("doctor_external_id"))

	// --- مراکز مجاز برای نقش این کاربر ---
	allowed, err := h.Scope.AllowedClinics(user)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load clinics")
		return
	}
	if len(allowed) == 0 {
		h.render(c, user, adminviews.AppointmentListView{
			Nav:     adminviews.BuildAdminNav(adminviews.NavAppointments, user.Role),
			Message: "هیچ مرکزی برای حساب شما تعریف نشده است.",
		})
		return
	}

	clinicID := h.resolveClinicID(c, allowed)
	if clinicID == 0 {
		clinicID = allowed[0].ID
	}
	if !h.Scope.CanAccess(user, clinicID) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	onlineSet := make(map[uint]struct{})
	for _, id := range h.Hub.OnlineClinicIDs() {
		onlineSet[id] = struct{}{}
	}

	view := adminviews.AppointmentListView{
		Nav:         adminviews.BuildAdminNav(adminviews.NavAppointments, user.Role),
		ClinicID:    clinicID,
		Clinics:     h.clinicOptions(allowed, onlineSet, clinicID),
		SearchQuery: search,
		FetchLive:   fetchLive,
	}

	doctors, err := h.Doctors.ListApprovedByClinic(clinicID)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load doctors")
		return
	}

	// --- درخواست بروزرسانی از کلاینت: یک پزشک یا همه پزشکان ---
	if fetchLive {
		if _, online := onlineSet[clinicID]; !online {
			view.Message = "مرکز انتخاب‌شده آنلاین نیست. فقط نوبت‌های موجود در کش نمایش داده می‌شوند."
		} else {
			req := buildAppointmentListRequest(doctors, doctorExternalID)
			push, liveErr := h.Hub.RequestAppointmentList(clinicID, liveAppointmentListTimeout, req)
			if liveErr != nil {
				view.Message = liveAppointmentListErrorMessage(liveErr)
			} else if _, upsertErr := h.Slots.UpsertFromPush(clinicID, push, doctors); upsertErr != nil {
				c.String(http.StatusInternalServerError, "failed to cache appointments")
				return
			} else if view.Message == "" {
				if req.Scope == protocol.ScopeOneDoctor {
					view.Message = "لیست نوبت‌های پزشک از مرکز دریافت و در کش ذخیره شد."
				} else {
					view.Message = "لیست نوبت‌ها از مرکز دریافت و در کش ذخیره شد."
				}
			}
		}
	}

	// --- خواندن اسلات‌ها از کش (نه دیتابیس) و گروه‌بندی زیر هر پزشک ---
	slots, err := h.Slots.ListByClinic(clinicID)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load appointments")
		return
	}

	slotsByDoctor := groupSlotsByDoctor(slots)
	view.Doctors = buildDoctorItems(doctors, slotsByDoctor, search)

	if view.Message == "" && len(view.Doctors) == 0 {
		view.Message = "پزشک تأیید‌شده‌ای برای این مرکز یافت نشد."
	} else if view.Message == "" && !fetchLive && len(slots) == 0 {
		view.Message = "نوبتی در کش نیست. «دریافت لیست زنده» را بزنید یا منتظر سینک خودکار کلاینت بمانید."
	}

	h.render(c, user, view)
}

// buildAppointmentListRequest درخواست بروزرسانی یک پزشک یا همه پزشکان را می‌سازد.
// ورودی: پزشکان تأیید‌شده مرکز، doctorExternalID اختیاری برای scope=one.
// خروجی: AppointmentListRequest مناسب برای ارسال روی WebSocket.
func buildAppointmentListRequest(doctors []models.Doctor, doctorExternalID string) protocol.AppointmentListRequest {
	if doctorExternalID != "" {
		return protocol.AppointmentListRequest{
			Scope:            protocol.ScopeOneDoctor,
			DoctorExternalID: doctorExternalID,
		}
	}
	externalIDs := make([]string, 0, len(doctors))
	for _, doc := range doctors {
		if doc.ExternalID != "" {
			externalIDs = append(externalIDs, doc.ExternalID)
		}
	}
	return protocol.AppointmentListRequest{
		Scope:             protocol.ScopeAllDoctors,
		DoctorExternalIDs: externalIDs,
	}
}

func (h *AppointmentHandler) resolveClinicID(c *gin.Context, allowed []models.Clinic) uint {
	if raw := c.Query("clinic_id"); raw != "" {
		if id, err := strconv.ParseUint(raw, 10, 64); err == nil {
			return uint(id)
		}
	}
	return 0
}

func (h *AppointmentHandler) clinicOptions(allowed []models.Clinic, online map[uint]struct{}, selected uint) []adminviews.ClinicOption {
	opts := make([]adminviews.ClinicOption, 0, len(allowed))
	for _, c := range allowed {
		_, isOnline := online[c.ID]
		opts = append(opts, adminviews.ClinicOption{
			ID:       c.ID,
			Name:     c.Name,
			IsOnline: isOnline,
			Selected: c.ID == selected,
		})
	}
	return opts
}

func groupSlotsByDoctor(slots []models.DoctorSlot) map[uint][]models.DoctorSlot {
	out := make(map[uint][]models.DoctorSlot)
	for _, slot := range slots {
		out[slot.DoctorID] = append(out[slot.DoctorID], slot)
	}
	return out
}

func buildDoctorItems(doctors []models.Doctor, slotsByDoctor map[uint][]models.DoctorSlot, search string) []adminviews.DoctorAppointmentItem {
	search = strings.ToLower(strings.TrimSpace(search))
	items := make([]adminviews.DoctorAppointmentItem, 0, len(doctors))
	for _, doc := range doctors {
		docSlots := slotsByDoctor[doc.ID]
		slotItems := toSlotItems(docSlots)
		if search != "" && !doctorMatchesSearch(doc, slotItems, search) {
			continue
		}
		spec := ""
		if doc.Specialty.ID != 0 {
			spec = doc.Specialty.Name
		}
		items = append(items, adminviews.DoctorAppointmentItem{
			DoctorID:   doc.ID,
			LocalCode:  doc.LocalCode,
			Name:       doc.Name,
			Specialty:  spec,
			ExternalID: doc.ExternalID,
			SlotCount:  len(slotItems),
			Slots:      slotItems,
		})
	}
	return items
}

// toSlotItems اسلات‌های کش را برای جدول لیست نوبت‌ها آماده می‌کند.
// ورودی: اسلات‌های یک پزشک. خروجی: ردیف‌های قابل نمایش؛ نوبت امروز پس از گذشتن زمان حضور پزشک حذف می‌شود.
func toSlotItems(slots []models.DoctorSlot) []adminviews.AppointmentSlotItem {
	now := time.Now()
	items := make([]adminviews.AppointmentSlotItem, 0, len(slots))
	for _, slot := range slots {
		if cache.TodayPresencePassed(slot, now) {
			continue
		}
		items = append(items, adminviews.AppointmentSlotItem{
			StartsAt:    slot.StartsAt,
			EndsAt:      slot.EndsAt,
			Capacity:    slot.Capacity,
			BookedCount: slot.BookedCount,
			IsAvailable: slot.IsAvailable,
		})
	}
	return items
}

func doctorMatchesSearch(doc models.Doctor, slots []adminviews.AppointmentSlotItem, q string) bool {
	fields := []string{
		doc.Name, doc.ExternalID, doc.FirstName, doc.LastName,
		strconv.Itoa(doc.LocalCode),
	}
	if doc.Specialty.ID != 0 {
		fields = append(fields, doc.Specialty.Name)
	}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

func (h *AppointmentHandler) render(c *gin.Context, user *models.AppointmentUser, view adminviews.AppointmentListView) {
	if view.Nav.NavItems == nil {
		view.Nav = adminviews.BuildAdminNav(adminviews.NavAppointments, user.Role)
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.AppointmentList(view).Render(c.Request.Context(), c.Writer)
}

func liveAppointmentListErrorMessage(err error) string {
	switch err {
	case websocket.ErrClinicOffline:
		return "مرکز انتخاب‌شده آنلاین نیست."
	case websocket.ErrRequestTimeout:
		return "پاسخ لیست نوبت‌ها از مرکز به‌موقع نرسید."
	default:
		return "خطا در دریافت لیست نوبت‌ها از مرکز: " + err.Error()
	}
}
