package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/websocket"
	adminviews "tebpardaz/server/views/admin"

	"github.com/gin-gonic/gin"
	gorillaws "github.com/gorilla/websocket"
)

var clinicLogUpgrader = gorillaws.Upgrader{
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

// ClinicLogHandler صفحه ادمین لاگ رفتار کلاینت‌های مرکز را مدیریت می‌کند.
type ClinicLogHandler struct {
	Logs    *repository.ClinicBehaviorLogRepo
	Clinics *repository.ClinicRepo
	Scope   *auth.ClinicScope
}

// NewClinicLogHandler سازنده ClinicLogHandler است.
// ورودی: repo لاگ، repo مرکز، محدوده دسترسی.
// خروجی: اشاره‌گر به ClinicLogHandler.
func NewClinicLogHandler(
	logs *repository.ClinicBehaviorLogRepo,
	clinics *repository.ClinicRepo,
	scope *auth.ClinicScope,
) *ClinicLogHandler {
	return &ClinicLogHandler{Logs: logs, Clinics: clinics, Scope: scope}
}

// List صفحه HTML لاگ‌ها را با فیلترها رندر می‌کند.
// ورودی: c (query clinic_id، action، msg_type، request_id، q، from، to، page).
// خروجی: HTML یا خطای HTTP.
func (h *ClinicLogHandler) List(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	allowed, err := h.Scope.AllowedClinics(user)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load clinics")
		return
	}
	if len(allowed) == 0 {
		h.render(c, user, adminviews.ClinicLogsView{
			Nav:     adminviews.BuildAdminNav(adminviews.NavClinicLogs, user.Role),
			Message: "هیچ مرکزی برای حساب شما تعریف نشده است.",
		})
		return
	}

	filter, clinicID := h.parseFilter(c, allowed)
	result, err := h.Logs.List(filter)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load logs")
		return
	}

	clinicNames := h.clinicNameMap(allowed)
	rows := make([]adminviews.ClinicLogRow, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, toLogRow(row, clinicNames))
	}

	h.render(c, user, adminviews.ClinicLogsView{
		Nav:          adminviews.BuildAdminNav(adminviews.NavClinicLogs, user.Role),
		ClinicID:     clinicID,
		Clinics:      h.clinicOptions(allowed, clinicID),
		Rows:         rows,
		Total:        result.Total,
		Page:         filter.Page,
		PerPage:      filter.PerPage,
		Action:       filter.Action,
		MsgType:      filter.MsgType,
		RequestID:    filter.RequestID,
		SearchQuery:  filter.Query,
		FromDate:     formatFilterDate(filter.From),
		ToDate:       formatFilterDate(filter.To),
		ActionLabels: adminviews.ClinicLogActionOptions(),
	})
}

// ServeWS WebSocket زنده برای دریافت لاگ‌های جدید با همان فیلترهای صفحه را برقرار می‌کند.
// ورودی: c (query فیلترها + since_id).
// خروجی: upgrade WebSocket و stream JSON.
func (h *ClinicLogHandler) ServeWS(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	allowed, err := h.Scope.AllowedClinics(user)
	if err != nil || len(allowed) == 0 {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	filter, _ := h.parseFilter(c, allowed)
	sinceID := parseUintQuery(c.Query("since_id"))

	conn, err := clinicLogUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	clinicNames := h.clinicNameMap(allowed)

	// ارسال ردیف‌های از دست‌رفته از آخرین since_id
	if sinceID > 0 {
		missed, listErr := h.Logs.ListSince(sinceID, filter)
		if listErr == nil {
			for _, row := range missed {
				if err := writeLogWS(conn, toLogRow(row, clinicNames)); err != nil {
					return
				}
				if row.ID > sinceID {
					sinceID = row.ID
				}
			}
		}
	}

	ch, unsub := websocket.ClinicLogHubInstance().Subscribe()
	defer unsub()

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case row, ok := <-ch:
			if !ok {
				return
			}
			if row.ID <= sinceID {
				continue
			}
			if !logMatchesFilter(row, filter) {
				continue
			}
			if err := writeLogWS(conn, toLogRow(row, clinicNames)); err != nil {
				return
			}
			sinceID = row.ID
		case <-pingTicker.C:
			if err := conn.WriteControl(gorillaws.PingMessage, []byte("ping"), time.Now().Add(5*time.Second)); err != nil {
				return
			}
		}
	}
}

func (h *ClinicLogHandler) parseFilter(c *gin.Context, allowed []models.Clinic) (repository.ClinicBehaviorLogFilter, uint) {
	allowedIDs := make([]uint, 0, len(allowed))
	for _, cl := range allowed {
		allowedIDs = append(allowedIDs, cl.ID)
	}

	clinicID := h.resolveClinicID(c, allowed)
	filter := repository.ClinicBehaviorLogFilter{
		AllowedClinicIDs: allowedIDs,
		ClinicID:         clinicID,
		Action:           strings.TrimSpace(c.Query("action")),
		MsgType:          strings.TrimSpace(c.Query("msg_type")),
		RequestID:        strings.TrimSpace(c.Query("request_id")),
		Query:            strings.TrimSpace(c.Query("q")),
		Page:             parseIntQuery(c.Query("page"), 1),
		PerPage:          parseIntQuery(c.Query("per_page"), 50),
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

func (h *ClinicLogHandler) resolveClinicID(c *gin.Context, allowed []models.Clinic) uint {
	if raw := c.Query("clinic_id"); raw != "" {
		if id, err := strconv.ParseUint(raw, 10, 64); err == nil {
			return uint(id)
		}
	}
	return 0
}

func (h *ClinicLogHandler) clinicOptions(allowed []models.Clinic, selected uint) []adminviews.ClinicOption {
	opts := make([]adminviews.ClinicOption, 0, len(allowed)+1)
	opts = append(opts, adminviews.ClinicOption{
		ID:       0,
		Name:     "همه مراکز",
		Selected: selected == 0,
	})
	for _, cl := range allowed {
		opts = append(opts, adminviews.ClinicOption{
			ID:       cl.ID,
			Name:     cl.Name,
			Selected: cl.ID == selected,
		})
	}
	return opts
}

func (h *ClinicLogHandler) clinicNameMap(allowed []models.Clinic) map[uint]string {
	out := make(map[uint]string, len(allowed))
	for _, cl := range allowed {
		out[cl.ID] = cl.Name
	}
	return out
}

func (h *ClinicLogHandler) render(c *gin.Context, user *models.AppointmentUser, view adminviews.ClinicLogsView) {
	if view.Nav.NavItems == nil {
		view.Nav = adminviews.BuildAdminNav(adminviews.NavClinicLogs, user.Role)
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.ClinicLogs(view).Render(c.Request.Context(), c.Writer)
}

func toLogRow(row models.ClinicBehaviorLog, clinicNames map[uint]string) adminviews.ClinicLogRow {
	clinicName := clinicNames[row.ClinicID]
	if clinicName == "" && row.ClinicID == 0 {
		clinicName = "—"
	} else if clinicName == "" {
		clinicName = strconv.FormatUint(uint64(row.ClinicID), 10)
	}
	return adminviews.ClinicLogRow{
		ID:         row.ID,
		CreatedAt:  row.CreatedAt,
		ClinicID:   row.ClinicID,
		ClinicName: clinicName,
		Action:     row.Action,
		MsgType:    row.MsgType,
		RequestID:  row.RequestID,
		Detail:     row.Detail,
		Error:      row.Error,
	}
}

func logMatchesFilter(row models.ClinicBehaviorLog, filter repository.ClinicBehaviorLogFilter) bool {
	if len(filter.AllowedClinicIDs) > 0 {
		allowed := false
		for _, id := range filter.AllowedClinicIDs {
			if row.ClinicID == id {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}
	if filter.ClinicID > 0 && row.ClinicID != filter.ClinicID {
		return false
	}
	if filter.Action != "" && row.Action != filter.Action {
		return false
	}
	if filter.MsgType != "" && row.MsgType != filter.MsgType {
		return false
	}
	if filter.RequestID != "" && row.RequestID != filter.RequestID {
		return false
	}
	if filter.From != nil && row.CreatedAt.Before(*filter.From) {
		return false
	}
	if filter.To != nil && row.CreatedAt.After(*filter.To) {
		return false
	}
	if filter.Query != "" {
		q := strings.ToLower(filter.Query)
		fields := []string{row.Detail, row.Error, row.MsgType, row.RequestID}
		found := false
		for _, f := range fields {
			if strings.Contains(strings.ToLower(f), q) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func writeLogWS(conn *gorillaws.Conn, row adminviews.ClinicLogRow) error {
	payload, err := json.Marshal(row)
	if err != nil {
		return err
	}
	return conn.WriteMessage(gorillaws.TextMessage, payload)
}

func parseIntQuery(raw string, def int) int {
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return def
	}
	return v
}

func parseUintQuery(raw string) uint {
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return uint(v)
}

func parseDateQuery(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	t, err := time.ParseInLocation("2006-01-02", raw, time.Local)
	if err != nil {
		return nil
	}
	return &t
}

func formatFilterDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}
