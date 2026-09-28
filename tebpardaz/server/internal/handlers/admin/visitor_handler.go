package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/analytics"
	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	adminviews "tebpardaz/server/views/admin"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// VisitorHandler serves admin lists of visitor IPs and page visits.
type VisitorHandler struct {
	Visits  *analytics.GormVisitRepository
	Clinics *repository.ClinicRepo
	Scope   *auth.ClinicScope
}

// NewVisitorHandler constructs VisitorHandler.
// Inputs: visit repository, clinic repo, clinic scope.
// Output: pointer to VisitorHandler.
func NewVisitorHandler(visits *analytics.GormVisitRepository, clinics *repository.ClinicRepo, scope *auth.ClinicScope) *VisitorHandler {
	return &VisitorHandler{Visits: visits, Clinics: clinics, Scope: scope}
}

// ListIPs renders the unique visitor IP table with filters.
// Inputs: gin context (q, clinic_id, from, to, page).
// Output: HTML page or HTTP error.
func (h *VisitorHandler) ListIPs(c *gin.Context) {
	user, allowed, ok := h.actor(c)
	if !ok {
		return
	}
	if len(allowed) == 0 && !isSuperAdmin(user.Role) {
		h.renderIPs(c, user, adminviews.VisitorIPsView{Message: "هیچ مرکزی برای حساب شما تعریف نشده است."})
		return
	}

	filter, clinicID, platformOnly := h.parseIPFilter(c, user, allowed)
	result, err := h.Visits.ListVisitorIPs(c.Request.Context(), filter)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت لیست آی‌پی‌ها")
		return
	}
	h.renderIPs(c, user, adminviews.VisitorIPsView{
		Clinics:        h.clinicOptions(allowed, clinicID),
		Rows:           toIPRows(result.Rows),
		Total:          result.Total,
		Page:           filter.Page,
		PerPage:        filter.PerPage,
		TotalPages:     analytics.TotalPages(result.Total, filter.PerPage),
		SearchQuery:    filter.IPQuery,
		ClinicID:       clinicID,
		PlatformOnly:   platformOnly,
		FromDate:       formatFilterDate(filter.From),
		ToDate:         formatFilterDate(endDateForForm(filter.To)),
		CanSeePlatform: isSuperAdmin(user.Role),
	})
}

// ListVisits renders the page-visit table with filters.
// Inputs: gin context (q, ip, visitor_ip_id, host, browser, os, from_google, clinic_id, from, to, page).
// Output: HTML page or HTTP error.
func (h *VisitorHandler) ListVisits(c *gin.Context) {
	user, allowed, ok := h.actor(c)
	if !ok {
		return
	}
	if len(allowed) == 0 && !isSuperAdmin(user.Role) {
		h.renderVisits(c, user, adminviews.VisitorVisitsView{Message: "هیچ مرکزی برای حساب شما تعریف نشده است."})
		return
	}

	filter, clinicID, platformOnly := h.parseVisitFilter(c, user, allowed)
	result, err := h.Visits.ListVisitDetails(c.Request.Context(), filter)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت بازدید صفحات")
		return
	}
	h.renderVisits(c, user, adminviews.VisitorVisitsView{
		Clinics:          h.clinicOptions(allowed, clinicID),
		Browsers:         analytics.KnownBrowsers(),
		OperatingSystems: analytics.KnownOperatingSystems(),
		Rows:             toVisitRows(result.Rows, h.clinicNameMap(allowed)),
		Total:            result.Total,
		Page:             filter.Page,
		PerPage:          filter.PerPage,
		TotalPages:       analytics.TotalPages(result.Total, filter.PerPage),
		SearchQuery:      filter.PathQuery,
		IPQuery:          filter.IPQuery,
		VisitorIPID:      filter.VisitorIPID,
		Host:             filter.Host,
		Browser:          filter.Browser,
		OS:               filter.OS,
		GoogleFilter:     strings.TrimSpace(c.Query("from_google")),
		ClinicID:         clinicID,
		PlatformOnly:     platformOnly,
		FromDate:         formatFilterDate(filter.From),
		ToDate:           formatFilterDate(endDateForForm(filter.To)),
		CanSeePlatform:   isSuperAdmin(user.Role),
	})
}

// renderIPs writes the visitor IP HTML page.
func (h *VisitorHandler) renderIPs(c *gin.Context, user *models.AppointmentUser, view adminviews.VisitorIPsView) {
	view.Nav = adminviews.BuildAdminNav(adminviews.NavVisitorIPs, user.Role)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.VisitorIPs(view).Render(c.Request.Context(), c.Writer)
}

// renderVisits writes the page-visit HTML page.
func (h *VisitorHandler) renderVisits(c *gin.Context, user *models.AppointmentUser, view adminviews.VisitorVisitsView) {
	view.Nav = adminviews.BuildAdminNav(adminviews.NavVisitorVisits, user.Role)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.VisitorVisits(view).Render(c.Request.Context(), c.Writer)
}

// actor loads the current admin and the clinics they may inspect.
func (h *VisitorHandler) actor(c *gin.Context) (*models.AppointmentUser, []models.Clinic, bool) {
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

// parseIPFilter reads IP-list query params and applies clinic scope.
func (h *VisitorHandler) parseIPFilter(c *gin.Context, user *models.AppointmentUser, allowed []models.Clinic) (analytics.VisitorIPFilter, uint, bool) {
	clinicID, platformOnly := h.parseClinicFilter(c, user, allowed)
	page, perPage := clampPage(parseIntQuery(c.Query("page"), 1), parseIntQuery(c.Query("per_page"), 50))
	filter := analytics.VisitorIPFilter{
		IPQuery:           strings.TrimSpace(c.Query("q")),
		ClinicID:          clinicID,
		PlatformOnly:      platformOnly,
		RestrictClinicIDs: restrictClinicIDs(user, allowed),
		Page:              page,
		PerPage:           perPage,
	}
	if from := parseDateQuery(c.Query("from")); from != nil {
		filter.From = from
	}
	if to := parseDateQuery(c.Query("to")); to != nil {
		end := to.Add(24*time.Hour - time.Second)
		filter.To = &end
	}
	return filter, clinicID, platformOnly
}

// parseVisitFilter reads page-visit query params and applies clinic scope.
func (h *VisitorHandler) parseVisitFilter(c *gin.Context, user *models.AppointmentUser, allowed []models.Clinic) (analytics.VisitDetailFilter, uint, bool) {
	clinicID, platformOnly := h.parseClinicFilter(c, user, allowed)
	page, perPage := clampPage(parseIntQuery(c.Query("page"), 1), parseIntQuery(c.Query("per_page"), 50))
	filter := analytics.VisitDetailFilter{
		IPQuery:           strings.TrimSpace(c.Query("ip")),
		VisitorIPID:       parseUintQuery(c.Query("visitor_ip_id")),
		Host:              strings.TrimSpace(c.Query("host")),
		PathQuery:         strings.TrimSpace(c.Query("q")),
		Page:              page,
		PerPage:           perPage,
		ClinicID:          clinicID,
		PlatformOnly:      platformOnly,
		RestrictClinicIDs: restrictClinicIDs(user, allowed),
	}
	if browser := strings.TrimSpace(c.Query("browser")); analytics.IsKnownBrowser(browser) {
		filter.Browser = browser
	}
	if osName := strings.TrimSpace(c.Query("os")); analytics.IsKnownOS(osName) {
		filter.OS = osName
	}
	switch strings.TrimSpace(c.Query("from_google")) {
	case "1", "yes", "true":
		yes := true
		filter.FromGoogle = &yes
	case "0", "no", "false":
		no := false
		filter.FromGoogle = &no
	}
	if from := parseDateQuery(c.Query("from")); from != nil {
		filter.From = from
	}
	if to := parseDateQuery(c.Query("to")); to != nil {
		end := to.Add(24*time.Hour - time.Second)
		filter.To = &end
	}
	return filter, clinicID, platformOnly
}

// parseClinicFilter reads clinic_id (numeric or "none" for platform-only visits).
func (h *VisitorHandler) parseClinicFilter(c *gin.Context, user *models.AppointmentUser, allowed []models.Clinic) (uint, bool) {
	raw := strings.TrimSpace(c.Query("clinic_id"))
	if raw == "none" && isSuperAdmin(user.Role) {
		return 0, true
	}
	id := parseUintQuery(raw)
	if id == 0 {
		return 0, false
	}
	if isSuperAdmin(user.Role) || clinicAllowed(allowed, id) {
		return id, false
	}
	return 0, false
}

// clinicOptions builds the clinic filter dropdown (real clinics only; "all"/"none" are in the view).
func (h *VisitorHandler) clinicOptions(allowed []models.Clinic, selected uint) []adminviews.ClinicOption {
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
func (h *VisitorHandler) clinicNameMap(allowed []models.Clinic) map[uint]string {
	out := make(map[uint]string, len(allowed))
	for _, cl := range allowed {
		out[cl.ID] = cl.Name
	}
	return out
}

// restrictClinicIDs returns a clinic allow-list for non-superadmin users; superadmin is unrestricted.
func restrictClinicIDs(user *models.AppointmentUser, allowed []models.Clinic) []uint {
	if isSuperAdmin(user.Role) {
		return nil
	}
	ids := make([]uint, 0, len(allowed))
	for _, cl := range allowed {
		ids = append(ids, cl.ID)
	}
	return ids
}

// isSuperAdmin reports whether the role may see every clinic plus platform-layer visits.
func isSuperAdmin(role string) bool {
	return constants.UserRole(role) == constants.UserRoleSuperAdmin
}

// clinicAllowed reports whether clinicID is in allowed.
func clinicAllowed(allowed []models.Clinic, clinicID uint) bool {
	for _, cl := range allowed {
		if cl.ID == clinicID {
			return true
		}
	}
	return false
}

// clampPage applies the same page bounds as the analytics query layer.
func clampPage(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}
	return page, perPage
}

// endDateForForm converts an inclusive end-of-day timestamp back to a date input value.
func endDateForForm(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return &day
}

// toIPRows maps persistence models onto the admin IP table.
func toIPRows(rows []models.VisitorIP) []adminviews.VisitorIPRow {
	out := make([]adminviews.VisitorIPRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, adminviews.VisitorIPRow{
			ID:          row.ID,
			IPAddress:   row.IPAddress,
			VisitCount:  row.VisitCount,
			LastVisitAt: row.LastVisitAt,
			CreatedAt:   row.CreatedAt,
		})
	}
	return out
}

// toVisitRows maps persistence models onto the admin visit table.
func toVisitRows(rows []models.VisitDetail, clinicNames map[uint]string) []adminviews.VisitorVisitRow {
	out := make([]adminviews.VisitorVisitRow, 0, len(rows))
	for _, row := range rows {
		clinicName := "—"
		if row.ClinicID != nil && *row.ClinicID > 0 {
			if name := clinicNames[*row.ClinicID]; name != "" {
				clinicName = name
			} else {
				clinicName = strconv.FormatUint(uint64(*row.ClinicID), 10)
			}
		}
		out = append(out, adminviews.VisitorVisitRow{
			ID:           row.ID,
			VisitorIPID:  row.VisitorIPID,
			IPAddress:    row.VisitorIP.IPAddress,
			Host:         row.Host,
			ClinicName:   clinicName,
			Path:         row.Path,
			FullURL:      row.FullURL,
			Browser:      row.Browser,
			OS:           row.OS,
			Referrer:     row.Referrer,
			IsFromGoogle: row.IsFromGoogle,
			CreatedAt:    row.CreatedAt,
		})
	}
	return out
}
