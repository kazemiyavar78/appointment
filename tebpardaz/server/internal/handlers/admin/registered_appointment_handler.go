package admin

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/analytics"
	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/visitcheck"
	"tebpardaz/server/internal/websocket"
	adminviews "tebpardaz/server/views/admin"
	"tebpardaz/shared/protocol"

	"github.com/gin-gonic/gin"
	ptime "github.com/yaa110/go-persian-calendar"
	"gorm.io/gorm"
)

const registeredAppointmentBehaviorLimit = 50

const (
	// statusOTPUnbooked is the admin filter for patients who received an OTP and did not book.
	statusOTPUnbooked = "otp_unbooked"
	// statusOTPPending means the code was sent and never verified.
	statusOTPPending = "otp_pending"
	// statusOTPVerified means the mobile was verified and no appointment was created.
	statusOTPVerified = "otp_verified"
	// registeredMergeMax is the largest window pulled from each source before merging a page.
	registeredMergeMax = 10000
)

// RegisteredAppointmentHandler serves the admin list of website-booked patient appointments.
type RegisteredAppointmentHandler struct {
	Appointments *repository.AppointmentRepo
	OTPs         *repository.OTPRepo
	Visits       *analytics.GormVisitRepository
	Clinics      *repository.ClinicRepo
	Scope        *auth.ClinicScope
	Hub          *websocket.Hub
}

// NewRegisteredAppointmentHandler constructs RegisteredAppointmentHandler.
// Inputs: appointment repo, OTP lead repo, visit repo, clinic repo, clinic scope, websocket hub.
// Output: pointer to RegisteredAppointmentHandler.
func NewRegisteredAppointmentHandler(
	appointments *repository.AppointmentRepo,
	otps *repository.OTPRepo,
	visits *analytics.GormVisitRepository,
	clinics *repository.ClinicRepo,
	scope *auth.ClinicScope,
	hub *websocket.Hub,
) *RegisteredAppointmentHandler {
	return &RegisteredAppointmentHandler{
		Appointments: appointments,
		OTPs:         otps,
		Visits:       visits,
		Clinics:      clinics,
		Scope:        scope,
		Hub:          hub,
	}
}

// List renders the registered-appointment table with clinic/status/search filters.
// Inputs: gin context (clinic_id, status, q, from, to, page).
// Output: HTML page or HTTP error.
func (h *RegisteredAppointmentHandler) List(c *gin.Context) {
	user, allowed, ok := h.actor(c)
	if !ok {
		return
	}
	if len(allowed) == 0 {
		h.render(c, user, adminviews.RegisteredAppointmentsView{
			Message: "هیچ مرکزی برای حساب شما تعریف نشده است.",
		})
		return
	}

	filter, clinicID := h.parseListFilter(c, allowed)
	rows, total, err := h.loadRegisteredRows(filter, h.clinicNameMap(allowed))
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت نوبت‌های ثبت‌شده")
		return
	}

	h.render(c, user, adminviews.RegisteredAppointmentsView{
		Clinics:     h.clinicOptions(allowed, clinicID),
		Rows:        rows,
		Total:       total,
		Page:        filter.Page,
		PerPage:     filter.PerPage,
		TotalPages:  analytics.TotalPages(total, filter.PerPage),
		ClinicID:    clinicID,
		Status:      filter.Status,
		SearchQuery: filter.Query,
		FromDate:    formatFilterDate(filter.From),
		ToDate:      formatFilterDate(endDateForForm(filter.To)),
	})
}

// PatientJSON returns Patient fields for the appointment popup.
// Inputs: gin context with :id path param.
// Output: JSON patient payload or HTTP error.
func (h *RegisteredAppointmentHandler) PatientJSON(c *gin.Context) {
	appt, ok := h.loadScopedAppointment(c)
	if !ok {
		return
	}
	if appt.Patient.ID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "اطلاعات بیمار یافت نشد."})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"patient": toPatientInfoPayload(appt.Patient),
	})
}

// OTPPatientJSON returns identity for a patient who received an OTP and did not book.
// Inputs: gin context with :id path param of a booking_otps row.
// Output: JSON patient payload or HTTP error.
func (h *RegisteredAppointmentHandler) OTPPatientJSON(c *gin.Context) {
	row, ok := h.loadScopedOTP(c)
	if !ok {
		return
	}
	patient := models.Patient{
		NationalID: row.NationalID,
		FirstName:  row.FirstName,
		LastName:   row.LastName,
		Mobile:     row.Mobile,
		Sex:        models.OTHER,
	}
	if row.PatientID > 0 && h.Appointments != nil {
		saved, err := h.Appointments.GetPatientByID(row.PatientID)
		if err == nil && saved != nil {
			patient = *saved
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"patient": toPatientInfoPayload(patient),
	})
}

// OTPBehaviorJSON returns visitor analytics for the IP stored on an unbooked OTP.
// Inputs: gin context with :id path param of a booking_otps row.
// Output: JSON visit summary or HTTP error.
func (h *RegisteredAppointmentHandler) OTPBehaviorJSON(c *gin.Context) {
	user, allowed, ok := h.actor(c)
	if !ok {
		return
	}
	row, ok := h.loadScopedOTP(c)
	if !ok {
		return
	}
	h.writeBehavior(c, user, allowed, row.IPAddress)
}

// BehaviorJSON returns visitor analytics for the appointment's stored IP.
// Inputs: gin context with :id path param.
// Output: JSON visit summary or HTTP error.
func (h *RegisteredAppointmentHandler) BehaviorJSON(c *gin.Context) {
	user, allowed, ok := h.actor(c)
	if !ok {
		return
	}
	appt, ok := h.loadScopedAppointmentFor(c, user)
	if !ok {
		return
	}

	h.writeBehavior(c, user, allowed, appt.IPAddress)
}

// VisitJSON asks the clinic client whether this confirmed appointment was admitted.
// Inputs: gin context with :id. Output: JSON label مراجعه کرده or مراجعه نکرده, or an error message.
func (h *RegisteredAppointmentHandler) VisitJSON(c *gin.Context) {
	appt, ok := h.loadScopedAppointment(c)
	if !ok {
		return
	}
	item, err := visitcheck.PrepareAppointment(appt, time.Now())
	if err != nil {
		msg, code := visitCheckHTTPError(err)
		c.JSON(code, gin.H{"ok": false, "message": msg})
		return
	}
	if h.Hub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "message": "کلاینت در دسترس نیست"})
		return
	}
	resp, err := h.Hub.RequestVisitCheck(appt.ClinicID, 20*time.Second, protocol.VisitCheckRequest{
		Items: []protocol.VisitCheckItem{item},
	})
	if err != nil {
		msg, code := visitCheckHTTPError(err)
		c.JSON(code, gin.H{"ok": false, "message": msg})
		return
	}
	count, readErr := visitCount(resp, item.ID)
	if readErr != "" {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "message": "خواندن پذیرش در مرکز ناموفق بود."})
		return
	}
	status := visitcheck.StatusFromCount(count)
	if h.Appointments == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "ذخیره نتیجه بررسی ناموفق بود."})
		return
	}
	if err := h.Appointments.MarkVisitStatus(appt.ID, status, time.Now()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "ذخیره نتیجه بررسی ناموفق بود."})
		return
	}
	label := visitcheck.Label(status)
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"kind":    visitcheck.KindAppointment,
		"id":      appt.ID,
		"label":   label,
		"message": label,
		"count":   count,
	})
}

// OTPVisitJSON answers a manual check for a patient who only received an OTP.
// Inputs: gin context with :id of a booking_otps row.
// Output: JSON explaining that doctor code and appointment date are not stored on an OTP-only lead.
func (h *RegisteredAppointmentHandler) OTPVisitJSON(c *gin.Context) {
	if _, ok := h.loadScopedOTP(c); !ok {
		return
	}
	c.JSON(http.StatusConflict, gin.H{
		"ok":      false,
		"message": "برای بیماری که فقط کد تایید گرفته، پزشک و تاریخ نوبت ثبت نشده و بررسی پذیرش ممکن نیست.",
	})
}

// visitCount finds the admission count for one appointment in the client response.
// Inputs: response and appointment ID. Output: count, or a non-empty error string when that row failed.
func visitCount(resp *protocol.VisitCheckResponse, id uint) (int, string) {
	if resp == nil {
		return 0, "empty response"
	}
	for _, result := range resp.Results {
		if result.Kind == visitcheck.KindAppointment && result.ID == id {
			return result.Count, strings.TrimSpace(result.Error)
		}
	}
	return 0, "missing result"
}

// visitCheckHTTPError maps a visit-check failure onto a Persian message and HTTP status.
// Inputs: error from preparation or the clinic client. Output: message and status code.
func visitCheckHTTPError(err error) (string, int) {
	switch {
	case errors.Is(err, visitcheck.ErrNotTaken):
		return "نوبت گرفته نشده است.", http.StatusConflict
	case errors.Is(err, visitcheck.ErrTooSoon):
		return "بررسی مراجعه تا یک روز پس از تاریخ نوبت ممکن نیست.", http.StatusConflict
	case errors.Is(err, visitcheck.ErrNoDoctorCode):
		return "کد پزشک در سیستم مرکز ثبت نشده است.", http.StatusConflict
	case errors.Is(err, visitcheck.ErrNoNationalID):
		return "کد ملی بیمار ثبت نشده است.", http.StatusConflict
	case errors.Is(err, visitcheck.ErrOTPIncomplete):
		return "برای بیماری که فقط کد تایید گرفته، پزشک و تاریخ نوبت ثبت نشده و بررسی پذیرش ممکن نیست.", http.StatusConflict
	case errors.Is(err, websocket.ErrClinicOffline):
		return "کلاینت در دسترس نیست", http.StatusServiceUnavailable
	case errors.Is(err, websocket.ErrRequestTimeout):
		return "مهلت پاسخ کلاینت تمام شد.", http.StatusGatewayTimeout
	default:
		return "بررسی مراجعه انجام نشد.", http.StatusInternalServerError
	}
}

// writeBehavior writes the visitor-analytics popup payload for a stored client IP.
// Inputs: gin context, admin user, allowed clinics, and the IP captured with the row.
// Output: JSON visit summary or HTTP error.
func (h *RegisteredAppointmentHandler) writeBehavior(c *gin.Context, user *models.AppointmentUser, allowed []models.Clinic, ipAddress string) {
	ip := strings.TrimSpace(ipAddress)
	if ip == "" {
		c.JSON(http.StatusOK, gin.H{
			"ok":      true,
			"ip":      "",
			"visits":  []any{},
			"total":   0,
			"message": "برای این نوبت آی‌پی ثبت نشده است.",
		})
		return
	}

	restrict := restrictClinicIDs(user, allowed)
	vip, err := h.Visits.GetVisitorIPByAddress(c.Request.Context(), ip)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت رفتار بیمار")
		return
	}

	payload := gin.H{
		"ok":         true,
		"ip":         ip,
		"visits":     []any{},
		"total":      int64(0),
		"visits_url": "/admin/visitors/visits?ip=" + url.QueryEscape(ip),
	}
	if vip == nil {
		payload["message"] = "بازدیدی برای این آی‌پی یافت نشد."
		c.JSON(http.StatusOK, payload)
		return
	}

	result, err := h.Visits.ListVisitDetails(c.Request.Context(), analytics.VisitDetailFilter{
		VisitorIPID:       vip.ID,
		RestrictClinicIDs: restrict,
		Page:              1,
		PerPage:           registeredAppointmentBehaviorLimit,
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت بازدید صفحات")
		return
	}

	payload["visitor_ip_id"] = vip.ID
	payload["visit_count"] = vip.VisitCount
	payload["last_visit_at"] = formatJSONDateTime(vip.LastVisitAt)
	payload["first_visit_at"] = formatJSONDateTime(vip.CreatedAt)
	payload["total"] = result.Total
	payload["visits"] = toBehaviorVisitPayloads(result.Rows, h.clinicNameMap(allowed))
	payload["visits_url"] = "/admin/visitors/visits?visitor_ip_id=" + strconv.FormatUint(uint64(vip.ID), 10)
	if result.Total == 0 {
		payload["message"] = "بازدیدی در محدوده مراکز شما برای این آی‌پی یافت نشد."
	}
	c.JSON(http.StatusOK, payload)
}

// render writes the registered-appointment HTML page.
func (h *RegisteredAppointmentHandler) render(c *gin.Context, user *models.AppointmentUser, view adminviews.RegisteredAppointmentsView) {
	view.Nav = adminviews.BuildAdminNav(adminviews.NavRegisteredAppointments, user.Role)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.RegisteredAppointments(view).Render(c.Request.Context(), c.Writer)
}

// actor loads the current admin and the clinics they may inspect.
func (h *RegisteredAppointmentHandler) actor(c *gin.Context) (*models.AppointmentUser, []models.Clinic, bool) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return nil, nil, false
	}
	allowed, err := h.Scope.AllowedClinics(user)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت مراکز")
		return nil, nil, false
	}
	return user, allowed, true
}

// loadScopedAppointment loads :id and rejects access outside the admin's clinic scope.
func (h *RegisteredAppointmentHandler) loadScopedAppointment(c *gin.Context) (*models.PatientAppointment, bool) {
	user, _, ok := h.actor(c)
	if !ok {
		return nil, false
	}
	return h.loadScopedAppointmentFor(c, user)
}

// loadScopedAppointmentFor loads :id using a previously resolved admin.
func (h *RegisteredAppointmentHandler) loadScopedAppointmentFor(
	c *gin.Context,
	user *models.AppointmentUser,
) (*models.PatientAppointment, bool) {
	id := parseUintQuery(c.Param("id"))
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "شناسه نوبت نامعتبر است."})
		return nil, false
	}
	appt, err := h.Appointments.GetByID(id)
	if errors.Is(err, repository.ErrAppointmentNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "نوبت یافت نشد."})
		return nil, false
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت نوبت")
		return nil, false
	}
	if !h.Scope.CanAccess(user, appt.ClinicID) {
		c.AbortWithStatus(http.StatusForbidden)
		return nil, false
	}
	return appt, true
}

// loadScopedOTP loads a booking OTP lead and rejects clinics outside the admin scope.
// Inputs: gin context with :id. Output: OTP row, or false after writing an HTTP error.
func (h *RegisteredAppointmentHandler) loadScopedOTP(c *gin.Context) (*models.BookingOTP, bool) {
	user, _, ok := h.actor(c)
	if !ok {
		return nil, false
	}
	id := parseUintQuery(c.Param("id"))
	if id == 0 || h.OTPs == nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "شناسه نامعتبر است."})
		return nil, false
	}
	row, err := h.OTPs.GetByID(id)
	if errors.Is(err, repository.ErrOTPLeadNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "مورد یافت نشد."})
		return nil, false
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت اطلاعات")
		return nil, false
	}
	if !h.Scope.CanAccess(user, row.ClinicID) {
		c.AbortWithStatus(http.StatusForbidden)
		return nil, false
	}
	return row, true
}

// loadRegisteredRows returns one admin page of appointments plus unbooked OTP leads.
// Inputs: list filter and clinic display names. Output: rows, combined total, or a database error.
func (h *RegisteredAppointmentHandler) loadRegisteredRows(filter repository.RegisteredAppointmentFilter, clinicNames map[uint]string) ([]adminviews.RegisteredAppointmentRow, int64, error) {
	wantAppt := includeBookedAppointments(filter.Status)
	wantOTP := includeUnbookedOTP(filter.Status) && h.OTPs != nil
	if !wantAppt && !wantOTP {
		return []adminviews.RegisteredAppointmentRow{}, 0, nil
	}
	if wantAppt && !wantOTP {
		result, err := h.Appointments.ListRegistered(filter)
		if err != nil {
			return nil, 0, err
		}
		return toRegisteredAppointmentRows(result.Rows, clinicNames), result.Total, nil
	}
	otpFilter := toOTPLeadFilter(filter)
	if wantOTP && !wantAppt {
		result, err := h.OTPs.ListUnbooked(otpFilter)
		if err != nil {
			return nil, 0, err
		}
		return toUnbookedOTPRows(result.Rows, clinicNames), result.Total, nil
	}

	window := filter.Page * filter.PerPage
	if window < 1 {
		window = filter.PerPage
	}
	if window > registeredMergeMax {
		window = registeredMergeMax
	}
	apptFilter := filter
	apptFilter.Page = 1
	apptFilter.PerPage = window
	apptFilter.MaxPerPage = registeredMergeMax
	otpFilter.Page = 1
	otpFilter.PerPage = window
	otpFilter.MaxPerPage = registeredMergeMax

	apptResult, err := h.Appointments.ListRegistered(apptFilter)
	if err != nil {
		return nil, 0, err
	}
	otpResult, err := h.OTPs.ListUnbooked(otpFilter)
	if err != nil {
		return nil, 0, err
	}
	merged := make([]timedAdminRow, 0, len(apptResult.Rows)+len(otpResult.Rows))
	for _, row := range toRegisteredAppointmentRows(apptResult.Rows, clinicNames) {
		merged = append(merged, timedAdminRow{At: row.CreatedAt, Row: row})
	}
	for _, row := range toUnbookedOTPRows(otpResult.Rows, clinicNames) {
		merged = append(merged, timedAdminRow{At: row.CreatedAt, Row: row})
	}
	return pageMergedRows(merged, filter.Page, filter.PerPage), apptResult.Total + otpResult.Total, nil
}

// timedAdminRow is one admin table row tagged with the time used to merge appointments and OTP leads.
type timedAdminRow struct {
	At  time.Time
	Row adminviews.RegisteredAppointmentRow
}

// includeBookedAppointments reports whether the status filter should list real appointments.
// Inputs: status query value. Output: true for empty, pending, confirmed, and failed.
func includeBookedAppointments(status string) bool {
	switch status {
	case "", "pending", "confirmed", "failed":
		return true
	default:
		return false
	}
}

// includeUnbookedOTP reports whether the status filter should list OTP leads without a booking.
// Inputs: status query value. Output: true for empty or otp_unbooked.
func includeUnbookedOTP(status string) bool {
	return status == "" || status == statusOTPUnbooked
}

// toOTPLeadFilter copies the appointment list filter onto the OTP lead query.
// Inputs: appointment filter. Output: OTP filter using the same clinic, search, and date bounds.
func toOTPLeadFilter(filter repository.RegisteredAppointmentFilter) repository.OTPLeadFilter {
	return repository.OTPLeadFilter{
		AllowedClinicIDs: filter.AllowedClinicIDs,
		ClinicID:         filter.ClinicID,
		Query:            filter.Query,
		From:             filter.From,
		To:               filter.To,
		Page:             filter.Page,
		PerPage:          filter.PerPage,
		MaxPerPage:       filter.MaxPerPage,
	}
}

// pageMergedRows sorts mixed rows newest-first and returns the requested page.
// Inputs: tagged rows, 1-based page, page size. Output: the page slice, possibly empty.
func pageMergedRows(items []timedAdminRow, page, perPage int) []adminviews.RegisteredAppointmentRow {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].At.Equal(items[j].At) {
			return items[i].Row.ID > items[j].Row.ID
		}
		return items[i].At.After(items[j].At)
	})
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	start := (page - 1) * perPage
	if start >= len(items) {
		return []adminviews.RegisteredAppointmentRow{}
	}
	end := start + perPage
	if end > len(items) {
		end = len(items)
	}
	out := make([]adminviews.RegisteredAppointmentRow, 0, end-start)
	for _, item := range items[start:end] {
		out = append(out, item.Row)
	}
	return out
}

// parseListFilter reads list query params and applies clinic scope.
func (h *RegisteredAppointmentHandler) parseListFilter(c *gin.Context, allowed []models.Clinic) (repository.RegisteredAppointmentFilter, uint) {
	allowedIDs := make([]uint, 0, len(allowed))
	for _, cl := range allowed {
		allowedIDs = append(allowedIDs, cl.ID)
	}
	clinicID := parseUintQuery(c.Query("clinic_id"))
	if clinicID > 0 && !clinicAllowed(allowed, clinicID) {
		clinicID = 0
	}
	page, perPage := clampPage(parseIntQuery(c.Query("page"), 1), parseIntQuery(c.Query("per_page"), 50))
	filter := repository.RegisteredAppointmentFilter{
		AllowedClinicIDs: allowedIDs,
		ClinicID:         clinicID,
		Status:           strings.TrimSpace(c.Query("status")),
		Query:            strings.TrimSpace(c.Query("q")),
		Page:             page,
		PerPage:          perPage,
	}
	if from := parseDateQuery(c.Query("from")); from != nil {
		filter.From = from
	}
	if to := parseDateQuery(c.Query("to")); to != nil {
		end := to.Add(24*time.Hour - time.Second)
		filter.To = &end
	}
	return filter, clinicID
}

// clinicOptions builds the clinic filter dropdown.
func (h *RegisteredAppointmentHandler) clinicOptions(allowed []models.Clinic, selected uint) []adminviews.ClinicOption {
	opts := make([]adminviews.ClinicOption, 0, len(allowed))
	for _, cl := range allowed {
		opts = append(opts, adminviews.ClinicOption{
			ID:       cl.ID,
			Name:     cl.Name,
			Selected: cl.ID == selected,
		})
	}
	return opts
}

// clinicNameMap maps clinic IDs to display names.
func (h *RegisteredAppointmentHandler) clinicNameMap(allowed []models.Clinic) map[uint]string {
	out := make(map[uint]string, len(allowed))
	for _, cl := range allowed {
		out[cl.ID] = cl.Name
	}
	return out
}

// toRegisteredAppointmentRows maps persistence rows onto the admin table.
func toRegisteredAppointmentRows(rows []models.PatientAppointment, clinicNames map[uint]string) []adminviews.RegisteredAppointmentRow {
	out := make([]adminviews.RegisteredAppointmentRow, 0, len(rows))
	for _, row := range rows {
		clinicName := clinicNames[row.ClinicID]
		if clinicName == "" {
			clinicName = strconv.FormatUint(uint64(row.ClinicID), 10)
		}
		out = append(out, adminviews.RegisteredAppointmentRow{
			ID:           row.ID,
			Kind:         "appointment",
			TrackingCode: booking.FormatTrackingCode(row.ID),
			PatientName:  strings.TrimSpace(row.Patient.FirstName + " " + row.Patient.LastName),
			DoctorName:   doctorDisplayName(row.Doctor),
			ClinicName:   clinicName,
			StartsAt:     row.StartsAt,
			CreatedAt:    row.CreatedAt,
			Status:       row.Status,
			FailedReason: row.FailReason,
			StatusLabel:  appointmentStatusLabel(row.Status),
			VisitStatus:  row.VisitStatus,
			VisitLabel:   visitcheck.Label(row.VisitStatus),
			IPAddress:    strings.TrimSpace(row.IPAddress),
		})
	}
	return out
}

// toUnbookedOTPRows maps delivered OTP challenges that never became an appointment.
// Inputs: OTP rows and clinic names. Output: admin table rows with kind otp.
func toUnbookedOTPRows(rows []models.BookingOTP, clinicNames map[uint]string) []adminviews.RegisteredAppointmentRow {
	out := make([]adminviews.RegisteredAppointmentRow, 0, len(rows))
	for _, row := range rows {
		clinicName := clinicNames[row.ClinicID]
		if clinicName == "" {
			clinicName = strconv.FormatUint(uint64(row.ClinicID), 10)
		}
		status := statusOTPPending
		if row.VerifiedAt != nil {
			status = statusOTPVerified
		}
		sentAt := row.LastSentAt
		if sentAt.IsZero() {
			sentAt = row.CreatedAt
		}
		out = append(out, adminviews.RegisteredAppointmentRow{
			ID:           row.ID,
			Kind:         "otp",
			TrackingCode: "OTP-" + strconv.FormatUint(uint64(row.ID), 10),
			PatientName:  strings.TrimSpace(row.FirstName + " " + row.LastName),
			DoctorName:   "—",
			ClinicName:   clinicName,
			CreatedAt:    sentAt,
			Status:       status,
			StatusLabel:  appointmentStatusLabel(status),
			VisitLabel:   "—",
			IPAddress:    strings.TrimSpace(row.IPAddress),
		})
	}
	return out
}

// toPatientInfoPayload maps a Patient model onto the popup JSON.
func toPatientInfoPayload(p models.Patient) gin.H {
	birth := "—"
	if models.BirthDateKnown(p.BirthDate) {
		birth = formatJSONDate(p.BirthDate)
	}
	sex := string(p.Sex)
	if strings.TrimSpace(sex) == "" {
		sex = string(models.OTHER)
	}
	return gin.H{
		"national_id": p.NationalID,
		"first_name":  p.FirstName,
		"last_name":   p.LastName,
		"mobile":      p.Mobile,
		"birth_date":  birth,
		"sex":         sex,
	}
}

// toBehaviorVisitPayloads maps visit rows onto the behavior popup JSON.
func toBehaviorVisitPayloads(rows []models.VisitDetail, clinicNames map[uint]string) []gin.H {
	out := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		clinicName := "—"
		if row.ClinicID != nil && *row.ClinicID > 0 {
			if name := clinicNames[*row.ClinicID]; name != "" {
				clinicName = name
			} else {
				clinicName = strconv.FormatUint(uint64(*row.ClinicID), 10)
			}
		}
		out = append(out, gin.H{
			"created_at":     formatJSONDateTime(row.CreatedAt),
			"host":           row.Host,
			"clinic_name":    clinicName,
			"path":           row.Path,
			"full_url":       row.FullURL,
			"browser":        row.Browser,
			"os":             row.OS,
			"referrer":       row.Referrer,
			"is_from_google": row.IsFromGoogle,
		})
	}
	return out
}

// doctorDisplayName prefers Doctor.Name and falls back to first + last name.
func doctorDisplayName(d models.Doctor) string {
	if name := strings.TrimSpace(d.Name); name != "" {
		return name
	}
	return strings.TrimSpace(d.FirstName + " " + d.LastName)
}

// appointmentStatusLabel returns the Persian label for a booking status.
func appointmentStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "confirmed":
		return "تأیید شده"
	case "failed":
		return "ناموفق"
	case "pending":
		return "در انتظار"
	case statusOTPPending:
		return "کد ارسال شد، بدون نوبت"
	case statusOTPVerified:
		return "موبایل تایید شد، بدون نوبت"
	case "":
		return "—"
	default:
		return status
	}
}

// formatJSONDate formats a civil date in Shamsi for popup display.
func formatJSONDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return ptime.New(t).Format("yyyy/MM/dd")
}

// formatJSONDateTime formats a timestamp in Shamsi for popup display.
func formatJSONDateTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return ptime.New(t).Format("yyyy/MM/dd HH:mm:ss")
}
