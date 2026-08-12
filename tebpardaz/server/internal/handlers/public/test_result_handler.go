package public

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/csrf"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/internal/testresult"
	"tebpardaz/server/internal/websocket"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"
	"tebpardaz/shared/protocol"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	msgResultNotReady  = "هنوز جواب آزمایش آماده نشده است یا مرکز برای سرور ارسال نکرده است"
	msgRateLimited     = "تعداد درخواست‌ها زیاد است؛ کمی بعد دوباره تلاش کنید"
	msgCSRFInvalid     = "نشست نامعتبر است؛ صفحه را تازه کنید و دوباره تلاش کنید"
	msgInvalidForm     = "لطفاً همه فیلدهای لازم را به‌درستی وارد کنید"
	msgClinicInvalid   = "کلینیک انتخاب‌شده معتبر نیست"
	msgClinicOffline   = "ارتباط با مرکز برقرار نیست؛ کمی بعد دوباره تلاش کنید"
	msgClinicTimeout   = "پاسخ مرکز به‌موقع نرسید؛ کمی بعد دوباره تلاش کنید"
	clinicLookupTimeout = 45 * time.Second
)

// TestResultHandler serves lab result lookup form and PDF download links.
type TestResultHandler struct {
	Clinics *repository.ClinicRepo
	Service *testresult.Service
	Limiter *testresult.RateLimiter
	CSRF    *csrf.Manager
	Hub     *websocket.Hub
}

// NewTestResultHandler constructs a TestResultHandler.
// Inputs: clinic repo, PDF store service, rate limiter, CSRF manager, clinic WebSocket hub.
// Output: pointer to TestResultHandler.
func NewTestResultHandler(
	clinics *repository.ClinicRepo,
	svc *testresult.Service,
	limiter *testresult.RateLimiter,
	csrfMgr *csrf.Manager,
	hub *websocket.Hub,
) *TestResultHandler {
	return &TestResultHandler{
		Clinics: clinics,
		Service: svc,
		Limiter: limiter,
		CSRF:    csrfMgr,
		Hub:     hub,
	}
}

// GetForm renders the test result lookup form for the current tenant layout.
func (h *TestResultHandler) GetForm(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	h.renderForm(c, tc, pages.TestResultFormView{
		FormAction: "/test-results",
	})
}

// PostLookup validates CSRF + rate limit, resolves clinic, and asks the clinic client for the PDF via WebSocket.
func (h *TestResultHandler) PostLookup(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}

	view := pages.TestResultFormView{
		FormAction:       "/test-results",
		SelectedClinicID: parseUintForm(c.PostForm("clinic_id")),
		AdmissionNo:      strings.TrimSpace(c.PostForm("admission_no")),
		Password:         strings.TrimSpace(c.PostForm("password")),
	}

	if h.CSRF == nil || !h.CSRF.Verify(c.Request, c.PostForm("csrf_token")) {
		view.ErrorMessage = msgCSRFInvalid
		h.renderForm(c, tc, view)
		return
	}

	if h.Limiter != nil && !h.Limiter.Allow(c.ClientIP()) {
		view.ErrorMessage = msgRateLimited
		h.renderForm(c, tc, view)
		return
	}

	if view.AdmissionNo == "" || view.Password == "" {
		view.ErrorMessage = msgInvalidForm
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

	if h.Hub == nil {
		view.InfoMessage = msgClinicOffline
		h.renderForm(c, tc, view)
		return
	}

	resp, err := h.Hub.RequestTestResult(clinic.ID, clinicLookupTimeout, protocol.TestResultRequest{
		ClinicCode:  clinic.Code,
		AdmissionNo: view.AdmissionNo,
		Password:    view.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, websocket.ErrClinicOffline):
			view.ErrorMessage = msgClinicOffline
		case errors.Is(err, websocket.ErrRequestTimeout):
			view.ErrorMessage = msgClinicTimeout
		default:
			c.Status(http.StatusInternalServerError)
			return
		}
		h.renderForm(c, tc, view)
		return
	}
	if resp == nil || !resp.Found || strings.TrimSpace(resp.PDFBase64) == "" {
		view.InfoMessage = msgResultNotReady
		h.renderForm(c, tc, view)
		return
	}

	if h.Service == nil {
		view.InfoMessage = msgResultNotReady
		h.renderForm(c, tc, view)
		return
	}

	stored, err := h.Service.StoreFromBase64(clinic.Code, view.AdmissionNo, view.Password, resp.PDFBase64)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if !stored.Found {
		view.InfoMessage = msgResultNotReady
		h.renderForm(c, tc, view)
		return
	}

	view.DownloadURL = stored.DownloadURL
	h.renderForm(c, tc, view)
}

// resolveClinic returns the clinic whose WebSocket client should serve the PDF for this tenant.
// Inputs: tenant context and optional selected clinic ID (organ/platform).
// Output: clinic or a Persian error message when invalid/out of scope.
func (h *TestResultHandler) resolveClinic(tc *tenant.Context, selectedID uint) (*models.Clinic, string) {
	switch tc.Layout {
	case constants.LayoutPrivate:
		if tc.Clinic != nil {
			return tc.Clinic, ""
		}
		if tc.ClinicID == nil || h.Clinics == nil {
			return nil, msgClinicInvalid
		}
		clinic, err := h.Clinics.GetByID(*tc.ClinicID)
		if err != nil || clinic == nil {
			return nil, msgClinicInvalid
		}
		return clinic, ""

	case constants.LayoutOrgan, constants.LayoutPlatform:
		if selectedID == 0 {
			return nil, msgInvalidForm
		}
		clinic, err := h.loadScopedClinic(tc, selectedID)
		if err != nil || clinic == nil {
			return nil, msgClinicInvalid
		}
		return clinic, ""

	default:
		return nil, msgClinicInvalid
	}
}

// loadScopedClinic loads a clinic by ID and ensures it belongs to the current organ/platform scope.
func (h *TestResultHandler) loadScopedClinic(tc *tenant.Context, clinicID uint) (*models.Clinic, error) {
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
		// All active clinics are allowed on the platform site.
	default:
		return nil, gorm.ErrRecordNotFound
	}
	return clinic, nil
}

// renderForm fills clinics/CSRF and wraps content in the tenant public layout.
func (h *TestResultHandler) renderForm(c *gin.Context, tc *tenant.Context, view pages.TestResultFormView) {
	view.ShowClinicSelect = tc.Layout == constants.LayoutOrgan || tc.Layout == constants.LayoutPlatform
	if view.ShowClinicSelect {
		view.Clinics = h.clinicOptions(tc)
	}
	if view.FormAction == "" {
		view.FormAction = "/test-results"
	}
	if h.CSRF != nil {
		if tok, err := h.CSRF.Issue(c.Writer); err == nil {
			view.CSRFToken = tok
		}
	}
	renderPublicLayout(c, tc, pages.TestResultForm(view), "test-results")
}

// clinicOptions lists clinics available in the current organ/platform tenant.
func (h *TestResultHandler) clinicOptions(tc *tenant.Context) []pages.TestResultClinicOption {
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
	out := make([]pages.TestResultClinicOption, 0, len(rows))
	for _, clinic := range rows {
		out = append(out, pages.TestResultClinicOption{ID: clinic.ID, Name: clinic.Name})
	}
	return out
}

// parseUintForm parses a positive uint from a form field; invalid values become 0.
func parseUintForm(raw string) uint {
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
