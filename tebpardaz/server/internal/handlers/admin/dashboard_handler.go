package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/dashboard"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	adminviews "tebpardaz/server/views/admin"

	"github.com/gin-gonic/gin"
)

// DashboardHandler serves the clinic-home management dashboard (HTML + JSON).
type DashboardHandler struct {
	Dash    *dashboard.Service
	Clinics *repository.ClinicRepo
	Scope   *auth.ClinicScope
}

// NewDashboardHandler constructs DashboardHandler.
// Inputs: dashboard service, clinic repo, clinic scope.
// Output: pointer to DashboardHandler.
func NewDashboardHandler(dash *dashboard.Service, clinics *repository.ClinicRepo, scope *auth.ClinicScope) *DashboardHandler {
	return &DashboardHandler{Dash: dash, Clinics: clinics, Scope: scope}
}

// Page renders the dashboard HTML for the selected range and clinic.
// Inputs: gin context (range, clinic_id).
// Output: HTML page or HTTP error.
func (h *DashboardHandler) Page(c *gin.Context) {
	user, allowed, ok := h.actor(c)
	if !ok {
		return
	}
	rangeKey := dashboard.ParseRangeKey(c.Query("range"))
	clinicID := h.parseClinicID(c, user, allowed)
	scope := h.scope(user, allowed, clinicID)

	view := adminviews.DashboardView{
		Range:          string(rangeKey),
		ClinicID:       clinicID,
		Clinics:        h.clinicOptions(allowed, clinicID),
		ShowClinicPick: len(allowed) > 1,
	}

	if len(allowed) == 0 && !isSuperAdmin(user.Role) {
		view.Message = "هیچ مرکزی برای حساب شما تعریف نشده است."
		view.LoadError = true
		h.render(c, user, view)
		return
	}

	snap, err := h.Dash.Load(c.Request.Context(), scope, rangeKey, time.Now())
	if err != nil {
		view.Message = "خطا در دریافت اطلاعات داشبورد."
		view.LoadError = true
		h.render(c, user, view)
		return
	}
	view.Data = snap
	view.DataJSON = mustJSON(snap)
	h.render(c, user, view)
}

// Data returns the dashboard snapshot as JSON for manual refresh and range changes.
// Inputs: gin context (range, clinic_id).
// Output: JSON {ok, data} or {ok:false, message}.
func (h *DashboardHandler) Data(c *gin.Context) {
	user, allowed, ok := h.actor(c)
	if !ok {
		return
	}
	if len(allowed) == 0 && !isSuperAdmin(user.Role) {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "هیچ مرکزی برای حساب شما تعریف نشده است."})
		return
	}
	rangeKey := dashboard.ParseRangeKey(c.Query("range"))
	clinicID := h.parseClinicID(c, user, allowed)
	scope := h.scope(user, allowed, clinicID)
	snap, err := h.Dash.Load(c.Request.Context(), scope, rangeKey, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "خطا در دریافت اطلاعات داشبورد."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": snap})
}

func (h *DashboardHandler) render(c *gin.Context, user *models.AppointmentUser, view adminviews.DashboardView) {
	view.Nav = adminviews.BuildAdminNav(adminviews.NavDashboard, user.Role)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.Dashboard(view).Render(c.Request.Context(), c.Writer)
}

func (h *DashboardHandler) actor(c *gin.Context) (*models.AppointmentUser, []models.Clinic, bool) {
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

func (h *DashboardHandler) parseClinicID(c *gin.Context, user *models.AppointmentUser, allowed []models.Clinic) uint {
	id := parseUintQuery(strings.TrimSpace(c.Query("clinic_id")))
	if id == 0 {
		return 0
	}
	if isSuperAdmin(user.Role) || clinicAllowed(allowed, id) {
		return id
	}
	return 0
}

func (h *DashboardHandler) scope(user *models.AppointmentUser, allowed []models.Clinic, clinicID uint) dashboard.Scope {
	ids := make([]uint, 0, len(allowed))
	for _, cl := range allowed {
		ids = append(ids, cl.ID)
	}
	sc := dashboard.Scope{
		ClinicID:  clinicID,
		ClinicIDs: ids,
	}
	if clinicID == 0 && isSuperAdmin(user.Role) {
		sc.IncludeNullClinicVisits = true
	}
	return sc
}

func (h *DashboardHandler) clinicOptions(allowed []models.Clinic, selected uint) []adminviews.ClinicOption {
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

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return strings.ReplaceAll(string(b), "<", "\\u003c")
}