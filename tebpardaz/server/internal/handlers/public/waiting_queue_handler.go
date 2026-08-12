package public

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/csrf"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/internal/waitingqueue"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"
	"tebpardaz/shared/protocol"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

const (
	msgWQCSRFInvalid   = "نشست نامعتبر است؛ صفحه را تازه کنید"
	msgWQInvalidForm   = "لطفاً شماره پذیرش را درست وارد کنید"
	msgWQClinicInvalid = "مرکز انتخاب‌شده معتبر نیست"
	// nationalIDPathEmpty در مسیر URL به‌جای کد ملی خالی استفاده می‌شود (برای QR بدون کد ملی).
	nationalIDPathEmpty = "-"
	wqPatientPushEvery  = 5 * time.Second
)

var waitingQueueUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WaitingQueueHandler فرم و WebSocket عمومی «وضعیت نوبت» را سرو می‌کند.
type WaitingQueueHandler struct {
	Clinics   *repository.ClinicRepo
	Cache     *cache.WaitingQueueCache
	CSRF      *csrf.Manager
	Refresher *waitingqueue.Refresher
}

// NewWaitingQueueHandler سازنده WaitingQueueHandler است.
// ورودی: ریپوی مراکز، کش صف، CSRF، زمان‌بند بروزرسانی تطبیقی.
// خروجی: اشاره‌گر به WaitingQueueHandler.
func NewWaitingQueueHandler(
	clinics *repository.ClinicRepo,
	queueCache *cache.WaitingQueueCache,
	csrfMgr *csrf.Manager,
	refresher *waitingqueue.Refresher,
) *WaitingQueueHandler {
	return &WaitingQueueHandler{
		Clinics:   clinics,
		Cache:     queueCache,
		CSRF:      csrfMgr,
		Refresher: refresher,
	}
}

// GetForm فرم جستجوی وضعیت نوبت را نمایش می‌دهد.
func (h *WaitingQueueHandler) GetForm(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	h.renderForm(c, tc, pages.WaitingQueueFormView{FormAction: "/waiting-queue"})
}

// GetDeepLink صفحه مانیتورینگ را از لینک QR (path params) باز می‌کند.
// مسیر خصوصی: /:admission_no/:national_id/waiting-queue
// مسیر ارگان/پلتفرم: /:admission_no/:national_id/:clinic_id/waiting-queue
func (h *WaitingQueueHandler) GetDeepLink(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}

	admissionNo, nationalID, clinicID, errMsg := h.parsePathParams(c, tc)
	view := pages.WaitingQueueFormView{
		FormAction:       "/waiting-queue",
		SelectedClinicID: clinicID,
		NationalID:       displayNationalID(nationalID),
		AdmissionNo:      strconv.Itoa(admissionNo),
	}
	if errMsg != "" {
		view.ErrorMessage = errMsg
		h.renderForm(c, tc, view)
		return
	}

	clinic, resolveMsg := h.resolveClinic(tc, clinicID)
	if resolveMsg != "" || clinic == nil {
		view.ErrorMessage = resolveMsg
		if view.ErrorMessage == "" {
			view.ErrorMessage = msgWQClinicInvalid
		}
		h.renderForm(c, tc, view)
		return
	}
	view.SelectedClinicID = clinic.ID
	h.applyLookupResult(c, tc, view, clinic.ID, nationalID, admissionNo)
}

// PostLookup کد ملی و شماره پذیرش را در کش صف جستجو می‌کند.
func (h *WaitingQueueHandler) PostLookup(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}

	view := pages.WaitingQueueFormView{
		FormAction:       "/waiting-queue",
		SelectedClinicID: parseUintForm(c.PostForm("clinic_id")),
		NationalID:       strings.TrimSpace(c.PostForm("national_id")),
		AdmissionNo:      strings.TrimSpace(c.PostForm("admission_no")),
	}

	if h.CSRF == nil || !h.CSRF.Verify(c.Request, c.PostForm("csrf_token")) {
		view.ErrorMessage = msgWQCSRFInvalid
		h.renderForm(c, tc, view)
		return
	}

	admissionNo, err := strconv.Atoi(view.AdmissionNo)
	if err != nil || admissionNo <= 0 {
		view.ErrorMessage = msgWQInvalidForm
		h.renderForm(c, tc, view)
		return
	}
	// کد ملی اختیاری است؛ فقط اگر پر شده باید ۱۰ رقم معتبر باشد.
	nid := normalizeNationalID(view.NationalID)
	if view.NationalID != "" && len(nid) < 10 {
		view.ErrorMessage = "کد ملی واردشده معتبر نیست"
		h.renderForm(c, tc, view)
		return
	}

	clinic, errMsg := h.resolveClinic(tc, view.SelectedClinicID)
	if errMsg != "" {
		view.ErrorMessage = errMsg
		h.renderForm(c, tc, view)
		return
	}
	view.SelectedClinicID = clinic.ID
	h.applyLookupResult(c, tc, view, clinic.ID, nid, admissionNo)
}

// applyLookupResult نتیجه جستجو را روی view اعمال و رندر می‌کند.
func (h *WaitingQueueHandler) applyLookupResult(
	c *gin.Context,
	tc *tenant.Context,
	view pages.WaitingQueueFormView,
	clinicID uint,
	nationalID string,
	admissionNo int,
) {
	status := h.lookupStatus(clinicID, nationalID, admissionNo)
	if !status.Found {
		view.InfoMessage = status.Message
		h.renderForm(c, tc, view)
		return
	}

	needClinic := tc.Layout == constants.LayoutOrgan || tc.Layout == constants.LayoutPlatform
	view.ShowLivePanel = true
	view.Status = pages.WaitingQueueStatusView{
		PatientName:  status.PatientName,
		DoctorName:   status.DoctorName,
		VisitTime:    status.VisitTime,
		AheadCount:   status.AheadCount,
		TotalInQueue: status.TotalInQueue,
	}
	view.WSPath = buildWaitingQueueWSPath(admissionNo, nationalID, clinicID, needClinic)
	h.renderForm(c, tc, view)
}

// ServeWS اتصال WebSocket بیمار را با پارامترهای مسیر نگه می‌دارد.
// خصوصی: /:admission_no/:national_id/ws/waiting-queue
// ارگان/پلتفرم: /:admission_no/:national_id/:clinic_id/ws/waiting-queue
func (h *WaitingQueueHandler) ServeWS(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}

	admissionNo, nationalID, clinicID, errMsg := h.parsePathParams(c, tc)
	if errMsg != "" || admissionNo <= 0 {
		c.Status(http.StatusBadRequest)
		return
	}

	clinic, resolveMsg := h.resolveClinic(tc, clinicID)
	if resolveMsg != "" || clinic == nil {
		c.Status(http.StatusNotFound)
		return
	}

	conn, err := waitingQueueUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	if h.Refresher != nil {
		h.Refresher.AddViewer(clinic.ID)
		defer h.Refresher.RemoveViewer(clinic.ID)
	}

	_ = conn.SetReadDeadline(time.Time{})
	ticker := time.NewTicker(wqPatientPushEvery)
	defer ticker.Stop()

	push := func() bool {
		status := h.lookupStatus(clinic.ID, nationalID, admissionNo)
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := conn.WriteJSON(status); err != nil {
			return false
		}
		return true
	}

	if !push() {
		return
	}
	for {
		select {
		case <-ticker.C:
			if !push() {
				return
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}

// parsePathParams شماره پذیرش، کد ملی و clinic_id را از مسیر می‌خواند.
// ورودی: gin context و tenant.
// خروجی: admissionNo، nationalID نرمال‌شده، clinicID، پیام خطای فارسی.
func (h *WaitingQueueHandler) parsePathParams(c *gin.Context, tc *tenant.Context) (admissionNo int, nationalID string, clinicID uint, errMsg string) {
	admissionNo, err := strconv.Atoi(strings.TrimSpace(c.Param("admission_no")))
	if err != nil || admissionNo <= 0 {
		return 0, "", 0, msgWQInvalidForm
	}
	nationalID = normalizeNationalID(c.Param("national_id"))

	needClinic := tc.Layout == constants.LayoutOrgan || tc.Layout == constants.LayoutPlatform
	if needClinic {
		clinicID = parseUintForm(c.Param("clinic_id"))
		if clinicID == 0 {
			return admissionNo, nationalID, 0, msgWQClinicInvalid
		}
	}
	return admissionNo, nationalID, clinicID, ""
}

// buildWaitingQueueWSPath مسیر وب‌سوکت path-based را برای QR/فرم می‌سازد.
// ورودی: شماره پذیرش، کد ملی، clinicID، و اینکه آیا clinic در مسیر لازم است.
// خروجی: مسیر نسبی مثل /123/0012345678/ws/waiting-queue
func buildWaitingQueueWSPath(admissionNo int, nationalID string, clinicID uint, needClinic bool) string {
	nid := nationalID
	if nid == "" {
		nid = nationalIDPathEmpty
	}
	if needClinic {
		return fmt.Sprintf("/%d/%s/%d/ws/waiting-queue", admissionNo, nid, clinicID)
	}
	return fmt.Sprintf("/%d/%s/ws/waiting-queue", admissionNo, nid)
}

// displayNationalID برای نمایش در فرم؛ placeholder مسیر را خالی نشان می‌دهد.
func displayNationalID(nationalID string) string {
	if nationalID == "" {
		return ""
	}
	return nationalID
}

// lookupStatus وضعیت بیمار را از کش می‌خواند.
func (h *WaitingQueueHandler) lookupStatus(clinicID uint, nationalID string, admissionNo int) protocol.WaitingQueueStatus {
	if h.Cache == nil {
		return protocol.WaitingQueueStatus{
			Found:   false,
			Message: "سرویس صف در دسترس نیست",
		}
	}
	return h.Cache.LookupPatient(clinicID, nationalID, admissionNo)
}

// resolveClinic مرکز معتبر برای لایه فعلی را برمی‌گرداند.
func (h *WaitingQueueHandler) resolveClinic(tc *tenant.Context, selectedID uint) (*models.Clinic, string) {
	switch tc.Layout {
	case constants.LayoutPrivate:
		if tc.Clinic != nil {
			return tc.Clinic, ""
		}
		if tc.ClinicID == nil || h.Clinics == nil {
			return nil, msgWQClinicInvalid
		}
		clinic, err := h.Clinics.GetByID(*tc.ClinicID)
		if err != nil || clinic == nil {
			return nil, msgWQClinicInvalid
		}
		return clinic, ""

	case constants.LayoutOrgan, constants.LayoutPlatform:
		if selectedID == 0 {
			return nil, "لطفاً مرکز درمانی را انتخاب کنید"
		}
		clinic, err := h.loadScopedClinic(tc, selectedID)
		if err != nil || clinic == nil {
			return nil, msgWQClinicInvalid
		}
		return clinic, ""

	default:
		return nil, msgWQClinicInvalid
	}
}

// loadScopedClinic مرکز را با محدوده ارگان/پلتفرم بارگذاری می‌کند.
func (h *WaitingQueueHandler) loadScopedClinic(tc *tenant.Context, clinicID uint) (*models.Clinic, error) {
	if h.Clinics == nil {
		return nil, gorm.ErrRecordNotFound
	}
	clinic, err := h.Clinics.GetByID(clinicID)
	if err != nil || clinic == nil {
		return nil, err
	}
	switch tc.Layout {
	case constants.LayoutOrgan:
		if tc.OrganizationID == nil || clinic.OrganizationID != *tc.OrganizationID {
			return nil, gorm.ErrRecordNotFound
		}
	case constants.LayoutPlatform:
		// همه مراکز فعال
	default:
		return nil, gorm.ErrRecordNotFound
	}
	return clinic, nil
}

// renderForm صفحه را با لایه عمومی مستأجر رندر می‌کند.
func (h *WaitingQueueHandler) renderForm(c *gin.Context, tc *tenant.Context, view pages.WaitingQueueFormView) {
	view.ShowClinicSelect = tc.Layout == constants.LayoutOrgan || tc.Layout == constants.LayoutPlatform
	if view.ShowClinicSelect {
		view.Clinics = h.clinicOptions(tc)
	}
	if view.FormAction == "" {
		view.FormAction = "/waiting-queue"
	}
	if h.CSRF != nil {
		if tok, err := h.CSRF.Issue(c.Writer); err == nil {
			view.CSRFToken = tok
		}
	}
	renderPublicLayout(c, tc, pages.WaitingQueueForm(view), "waiting-queue")
}

// clinicOptions لیست مراکز قابل انتخاب در لایه ارگان/پلتفرم را برمی‌گرداند.
func (h *WaitingQueueHandler) clinicOptions(tc *tenant.Context) []pages.WaitingQueueClinicOption {
	if h.Clinics == nil {
		return nil
	}
	var rows []models.Clinic
	var err error
	switch tc.Layout {
	case constants.LayoutOrgan:
		if tc.OrganizationID == nil {
			return nil
		}
		rows, err = h.Clinics.ListByOrganizationID(*tc.OrganizationID)
	case constants.LayoutPlatform:
		rows, err = h.Clinics.ListAll()
	default:
		return nil
	}
	if err != nil {
		return nil
	}
	out := make([]pages.WaitingQueueClinicOption, 0, len(rows))
	for _, clinic := range rows {
		out = append(out, pages.WaitingQueueClinicOption{ID: clinic.ID, Name: clinic.Name})
	}
	return out
}

// normalizeNationalID ارقام فارسی/عربی را یکسان می‌کند و placeholder خالی را تهی می‌سازد.
func normalizeNationalID(raw string) string {
	replacer := strings.NewReplacer(
		"۰", "0", "۱", "1", "۲", "2", "۳", "3", "۴", "4",
		"۵", "5", "۶", "6", "۷", "7", "۸", "8", "۹", "9",
		"٠", "0", "١", "1", "٢", "2", "٣", "3", "٤", "4",
		"٥", "5", "٦", "6", "٧", "7", "٨", "8", "٩", "9",
	)
	raw = replacer.Replace(strings.TrimSpace(raw))
	switch raw {
	case "-", "_", "0", "":
		return ""
	default:
		return raw
	}
}
