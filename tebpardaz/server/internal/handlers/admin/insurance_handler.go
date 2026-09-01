package admin

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	adminviews "tebpardaz/server/views/admin"

	"github.com/gin-gonic/gin"
)

const (
	insuranceUploadDir  = "static/uploads/insurances"
	insuranceDescMaxLen = 120
	insuranceNameMaxLen = 100
)

// InsuranceHandler manages the insurance catalog (superadmin) and clinic display assignment.
type InsuranceHandler struct {
	Insurances *repository.InsuranceRepo
	Scope      *auth.ClinicScope
}

// NewInsuranceHandler constructs an InsuranceHandler.
// Inputs: insurance repo, clinic scope.
// Output: pointer to InsuranceHandler.
func NewInsuranceHandler(insurances *repository.InsuranceRepo, scope *auth.ClinicScope) *InsuranceHandler {
	return &InsuranceHandler{Insurances: insurances, Scope: scope}
}

// List renders the superadmin insurance catalog with optional edit prefill.
func (h *InsuranceHandler) List(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	view := h.catalogView(user)
	view.Message = insuranceFlashMessage(c.Query("msg"))

	editID, _ := strconv.ParseUint(c.Query("edit"), 10, 64)
	if editID > 0 {
		row, err := h.Insurances.GetByID(uint(editID))
		if err != nil {
			if view.Message == "" {
				view.Message = "بیمه مورد نظر یافت نشد."
			}
		} else {
			view.EditID = row.ID
			view.EditName = row.Name
			view.EditLogoURL = row.LogoURL
			view.EditDescription = row.Description
		}
	}

	rows, err := h.Insurances.ListAll()
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load insurances")
		return
	}
	view.Items = toInsuranceRows(rows)
	h.renderCatalog(c, view)
}

// Create adds a new insurance catalog entry from form fields and optional logo.
func (h *InsuranceHandler) Create(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	name, description, msg := parseInsuranceFields(c)
	if msg != "" {
		h.renderCatalogWithMessage(c, user, msg)
		return
	}
	logoURL, err := h.saveInsuranceLogo(c, "")
	if err != nil {
		h.renderCatalogWithMessage(c, user, err.Error())
		return
	}

	row := &models.Insurance{Name: name, Description: description, LogoURL: logoURL}
	if err := h.Insurances.Create(row); err != nil {
		h.renderCatalogWithMessage(c, user, "خطا در ایجاد بیمه.")
		return
	}
	c.Redirect(http.StatusFound, "/admin/insurances?msg=created")
}

// Update saves changes to an existing insurance catalog entry.
func (h *InsuranceHandler) Update(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	row, err := h.Insurances.GetByID(id)
	if err != nil {
		h.renderCatalogWithMessage(c, user, "بیمه مورد نظر یافت نشد.")
		return
	}

	name, description, msg := parseInsuranceFields(c)
	if msg != "" {
		c.Redirect(http.StatusFound, "/admin/insurances?edit="+strconv.FormatUint(uint64(id), 10)+"&msg=name_required")
		return
	}
	logoURL, err := h.saveInsuranceLogo(c, row.LogoURL)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/insurances?edit="+strconv.FormatUint(uint64(id), 10)+"&msg=logo_failed")
		return
	}

	row.Name = name
	row.Description = description
	row.LogoURL = logoURL
	if err := h.Insurances.Update(row); err != nil {
		c.Redirect(http.StatusFound, "/admin/insurances?edit="+strconv.FormatUint(uint64(id), 10)+"&msg=update_failed")
		return
	}
	c.Redirect(http.StatusFound, "/admin/insurances?msg=updated")
}

// Delete removes an insurance catalog entry and its clinic assignments.
func (h *InsuranceHandler) Delete(c *gin.Context) {
	if _, ok := auth.UserFromGin(c); !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err := h.Insurances.Delete(id); err != nil {
		c.Redirect(http.StatusFound, "/admin/insurances?msg=delete_failed")
		return
	}
	c.Redirect(http.StatusFound, "/admin/insurances?msg=deleted")
}

// AssignForm renders clinic insurance selection for the current admin scope.
func (h *InsuranceHandler) AssignForm(c *gin.Context) {
	user, allowed, ok := h.requireUserClinics(c)
	if !ok {
		return
	}

	selectedID := h.resolveAssignClinicID(c, user, allowed)
	view := h.assignView(user, allowed, selectedID)
	view.Message = insuranceFlashMessage(c.Query("msg"))

	if selectedID > 0 {
		if err := h.fillAssignOptions(&view, selectedID); err != nil {
			c.String(http.StatusInternalServerError, "failed to load insurances")
			return
		}
	}
	h.renderAssign(c, view)
}

// SaveAssign stores which catalog insurances the scoped clinic(s) should display.
// ورودی: c کانتکست Gin (حاوی clinic_id یا clinic_ids و insurance_ids).
// خروجی: ریدایرکت با پارامترهای فیلتر و پیام وضعیت.
func (h *InsuranceHandler) SaveAssign(c *gin.Context) {
	user, _, ok := h.requireUserClinics(c)
	if !ok {
		return
	}

	rawClinicIDs := c.PostFormArray("clinic_ids")
	if len(rawClinicIDs) == 0 {
		if singleClinic := c.PostForm("clinic_id"); singleClinic != "" {
			rawClinicIDs = []string{singleClinic}
		}
	}

	var validClinicIDs []uint
	for _, raw := range rawClinicIDs {
		cid, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || cid == 0 {
			continue
		}
		if h.Scope.CanAccess(user, uint(cid)) {
			validClinicIDs = append(validClinicIDs, uint(cid))
		}
	}

	if len(validClinicIDs) == 0 {
		c.Redirect(http.StatusFound, "/admin/insurances/assign?msg=clinic_required")
		return
	}

	rawIDs := c.PostFormArray("insurance_ids")
	selected := make([]uint, 0, len(rawIDs))
	for _, raw := range rawIDs {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			continue
		}
		selected = append(selected, uint(id))
	}
	valid, err := h.filterKnownInsuranceIDs(selected)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/insurances/assign?clinic_id="+strconv.FormatUint(uint64(validClinicIDs[0]), 10)+"&msg=assign_failed")
		return
	}
	if err := h.Insurances.ReplaceClinicsInsurances(validClinicIDs, valid); err != nil {
		c.Redirect(http.StatusFound, "/admin/insurances/assign?clinic_id="+strconv.FormatUint(uint64(validClinicIDs[0]), 10)+"&msg=assign_failed")
		return
	}
	c.Redirect(http.StatusFound, "/admin/insurances/assign?clinic_id="+strconv.FormatUint(uint64(validClinicIDs[0]), 10)+"&msg=assigned")
}

// catalogView returns the empty catalog page model with nav for user.
func (h *InsuranceHandler) catalogView(user *models.AppointmentUser) adminviews.InsurancePageView {
	return adminviews.InsurancePageView{
		Nav: adminviews.BuildAdminNav(adminviews.NavInsurances, user.Role),
	}
}

// assignView builds the assignment page model for the user's allowed clinics.
func (h *InsuranceHandler) assignView(user *models.AppointmentUser, allowed []models.Clinic, selected uint) adminviews.InsuranceAssignView {
	return adminviews.InsuranceAssignView{
		Nav:            adminviews.BuildAdminNav(adminviews.NavInsuranceAssign, user.Role),
		Clinics:        toInsuranceClinicOptions(allowed, selected),
		ShowClinicPick: len(allowed) > 1,
		ClinicID:       selected,
	}
}

// fillAssignOptions loads catalog insurances and marks those already assigned to clinicID.
// Inputs: view to fill, clinicID.
// Output: DB error, if any.
func (h *InsuranceHandler) fillAssignOptions(view *adminviews.InsuranceAssignView, clinicID uint) error {
	rows, err := h.Insurances.ListAll()
	if err != nil {
		return err
	}
	assigned, err := h.Insurances.ListIDsByClinicID(clinicID)
	if err != nil {
		return err
	}
	selected := make(map[uint]struct{}, len(assigned))
	for _, id := range assigned {
		selected[id] = struct{}{}
	}
	view.Insurances = make([]adminviews.InsuranceAssignOption, 0, len(rows))
	for _, row := range rows {
		_, ok := selected[row.ID]
		view.Insurances = append(view.Insurances, adminviews.InsuranceAssignOption{
			ID:          row.ID,
			Name:        row.Name,
			LogoURL:     row.LogoURL,
			Description: row.Description,
			Selected:    ok,
		})
	}
	return nil
}

// requireUserClinics loads the current user and clinics they may assign insurances to.
func (h *InsuranceHandler) requireUserClinics(c *gin.Context) (*models.AppointmentUser, []models.Clinic, bool) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return nil, nil, false
	}
	allowed, err := h.Scope.AllowedClinics(user)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load clinics")
		return nil, nil, false
	}
	if len(allowed) == 0 {
		c.String(http.StatusForbidden, "هیچ مرکزی برای حساب شما تعریف نشده است.")
		return nil, nil, false
	}
	return user, allowed, true
}

// resolveAssignClinicID picks the clinic for the assignment form (single clinic or query/post id).
func (h *InsuranceHandler) resolveAssignClinicID(c *gin.Context, user *models.AppointmentUser, allowed []models.Clinic) uint {
	if len(allowed) == 1 {
		return allowed[0].ID
	}
	raw := c.Query("clinic_id")
	if raw == "" {
		raw = c.PostForm("clinic_id")
	}
	id, _ := strconv.ParseUint(raw, 10, 64)
	if id == 0 || !h.Scope.CanAccess(user, uint(id)) {
		return 0
	}
	return uint(id)
}

// filterKnownInsuranceIDs keeps unique IDs that exist in the insurance catalog.
func (h *InsuranceHandler) filterKnownInsuranceIDs(ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := h.Insurances.ListAll()
	if err != nil {
		return nil, err
	}
	known := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		known[row.ID] = struct{}{}
	}
	out := make([]uint, 0, len(ids))
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := known[id]; !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

// saveInsuranceLogo stores an uploaded logo or returns fallback; clear_logo=1 removes it.
func (h *InsuranceHandler) saveInsuranceLogo(c *gin.Context, fallback string) (string, error) {
	file, hdr, err := c.Request.FormFile("logo")
	if err != nil || file == nil {
		if c.PostForm("clear_logo") == "1" {
			return "", nil
		}
		return strings.TrimSpace(fallback), nil
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
	default:
		return "", fmt.Errorf("فرمت تصویر مجاز نیست.")
	}
	if hdr.Size > 5<<20 {
		return "", fmt.Errorf("حجم تصویر نباید بیش از ۵ مگابایت باشد.")
	}
	if err := os.MkdirAll(insuranceUploadDir, 0o755); err != nil {
		return "", fmt.Errorf("خطا در ذخیره فایل.")
	}
	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	destPath := filepath.Join(insuranceUploadDir, name)
	out, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("خطا در ذخیره فایل.")
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		return "", fmt.Errorf("خطا در ذخیره فایل.")
	}
	return "/static/uploads/insurances/" + name, nil
}

// renderCatalogWithMessage reloads the catalog list and shows an inline error.
func (h *InsuranceHandler) renderCatalogWithMessage(c *gin.Context, user *models.AppointmentUser, message string) {
	view := h.catalogView(user)
	view.Message = message
	rows, err := h.Insurances.ListAll()
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load insurances")
		return
	}
	view.Items = toInsuranceRows(rows)
	h.renderCatalog(c, view)
}

// renderCatalog writes the catalog HTML page.
func (h *InsuranceHandler) renderCatalog(c *gin.Context, view adminviews.InsurancePageView) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.Insurances(view).Render(c.Request.Context(), c.Writer)
}

// renderAssign writes the clinic assignment HTML page.
func (h *InsuranceHandler) renderAssign(c *gin.Context, view adminviews.InsuranceAssignView) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.InsuranceAssign(view).Render(c.Request.Context(), c.Writer)
}

// parseInsuranceFields reads and truncates name/description from the catalog form.
func parseInsuranceFields(c *gin.Context) (name, description, message string) {
	name = strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		return "", "", "نام بیمه الزامی است."
	}
	if len([]rune(name)) > insuranceNameMaxLen {
		name = string([]rune(name)[:insuranceNameMaxLen])
	}
	description = strings.TrimSpace(c.PostForm("description"))
	if len([]rune(description)) > insuranceDescMaxLen {
		description = string([]rune(description)[:insuranceDescMaxLen])
	}
	return name, description, ""
}

// toInsuranceRows maps catalog models to the admin table view.
func toInsuranceRows(rows []models.Insurance) []adminviews.InsuranceRow {
	out := make([]adminviews.InsuranceRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, adminviews.InsuranceRow{
			ID:          row.ID,
			Name:        row.Name,
			LogoURL:     row.LogoURL,
			Description: row.Description,
		})
	}
	return out
}

// toInsuranceClinicOptions maps clinics to selector options with the selected clinic marked.
func toInsuranceClinicOptions(clinics []models.Clinic, selected uint) []adminviews.ClinicOption {
	opts := make([]adminviews.ClinicOption, 0, len(clinics))
	for _, clinic := range clinics {
		opts = append(opts, adminviews.ClinicOption{
			ID:       clinic.ID,
			Name:     clinic.Name,
			Selected: clinic.ID == selected,
		})
	}
	return opts
}

// insuranceFlashMessage maps redirect query codes to Persian UI messages.
func insuranceFlashMessage(code string) string {
	switch code {
	case "created":
		return "بیمه با موفقیت ایجاد شد."
	case "updated":
		return "بیمه با موفقیت بروزرسانی شد."
	case "deleted":
		return "بیمه حذف شد."
	case "name_required":
		return "نام بیمه الزامی است."
	case "update_failed":
		return "خطا در بروزرسانی بیمه."
	case "delete_failed":
		return "خطا در حذف بیمه."
	case "logo_failed":
		return "خطا در ذخیره لوگوی بیمه."
	case "assigned":
		return "بیمه‌های مرکز ذخیره شد."
	case "assign_failed":
		return "خطا در ذخیره بیمه‌های مرکز."
	case "clinic_required":
		return "انتخاب مرکز الزامی است."
	default:
		return ""
	}
}
