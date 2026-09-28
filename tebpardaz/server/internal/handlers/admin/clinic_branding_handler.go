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
	clinicLogoUploadDir    = "static/uploads/clinics"
	clinicFaviconUploadDir = "static/uploads/clinics/favicons"
)

// ClinicBrandingHandler manages per-clinic logo and favicon uploads in admin.
type ClinicBrandingHandler struct {
	Clinics *repository.ClinicRepo
	Scope   *auth.ClinicScope
}

// NewClinicBrandingHandler constructs a ClinicBrandingHandler.
// Inputs: clinic repo, clinic scope.
// Output: pointer to ClinicBrandingHandler.
func NewClinicBrandingHandler(clinics *repository.ClinicRepo, scope *auth.ClinicScope) *ClinicBrandingHandler {
	return &ClinicBrandingHandler{Clinics: clinics, Scope: scope}
}

// Form renders the clinic branding upload page for the current admin scope.
// Inputs: gin context with optional clinic_id query.
// Output: HTML response or HTTP error status.
func (h *ClinicBrandingHandler) Form(c *gin.Context) {
	user, allowed, ok := h.requireUserClinics(c)
	if !ok {
		return
	}

	selectedID := h.resolveClinicID(c, user, allowed)
	view := h.brandingView(user, allowed, selectedID)
	view.Message = clinicBrandingFlashMessage(c.Query("msg"))

	if selectedID > 0 {
		clinic, err := h.Clinics.GetByIDForAdmin(selectedID)
		if err != nil {
			view.Message = "مرکز مورد نظر یافت نشد."
		} else {
			view.LogoURL = clinic.LogoURL
			view.FaviconURL = clinic.FaviconURL
		}
	}

	h.renderForm(c, view)
}

// Save stores uploaded logo and favicon files for one scoped clinic.
// Inputs: gin context with multipart form (clinic_id, logo, favicon, clear flags).
// Output: redirect with flash message.
func (h *ClinicBrandingHandler) Save(c *gin.Context) {
	user, _, ok := h.requireUserClinics(c)
	if !ok {
		return
	}

	clinicID, err := strconv.ParseUint(c.PostForm("clinic_id"), 10, 64)
	if err != nil || clinicID == 0 {
		c.Redirect(http.StatusFound, "/admin/clinic/branding?msg=clinic_required")
		return
	}
	if !h.Scope.CanAccess(user, uint(clinicID)) {
		c.Redirect(http.StatusFound, "/admin/clinic/branding?msg=forbidden")
		return
	}

	clinic, err := h.Clinics.GetByIDForAdmin(uint(clinicID))
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/clinic/branding?msg=not_found")
		return
	}

	logoURL, err := h.saveUploadedImage(c, "logo", "clear_logo", clinicLogoUploadDir, "/static/uploads/clinics/", clinic.LogoURL, false)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/clinic/branding?clinic_id="+strconv.FormatUint(clinicID, 10)+"&msg=logo_failed")
		return
	}
	faviconURL, err := h.saveUploadedImage(c, "favicon", "clear_favicon", clinicFaviconUploadDir, "/static/uploads/clinics/favicons/", clinic.FaviconURL, true)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/clinic/branding?clinic_id="+strconv.FormatUint(clinicID, 10)+"&msg=favicon_failed")
		return
	}

	if err := h.Clinics.UpdateBranding(uint(clinicID), logoURL, faviconURL); err != nil {
		c.Redirect(http.StatusFound, "/admin/clinic/branding?clinic_id="+strconv.FormatUint(clinicID, 10)+"&msg=save_failed")
		return
	}

	c.Redirect(http.StatusFound, "/admin/clinic/branding?clinic_id="+strconv.FormatUint(clinicID, 10)+"&msg=saved")
}

// brandingView builds the admin page model for clinic branding.
// Inputs: user, allowed clinics, selected clinic id.
// Output: ClinicBrandingView for templ rendering.
func (h *ClinicBrandingHandler) brandingView(user *models.AppointmentUser, allowed []models.Clinic, selected uint) adminviews.ClinicBrandingView {
	return adminviews.ClinicBrandingView{
		Nav:            adminviews.BuildAdminNav(adminviews.NavClinicBranding, user.Role),
		Clinics:        toInsuranceClinicOptions(allowed, selected),
		ShowClinicPick: len(allowed) > 1,
		ClinicID:       selected,
	}
}

// requireUserClinics loads the current user and clinics they may manage branding for.
// Inputs: gin context.
// Output: user, allowed clinics, ok flag.
func (h *ClinicBrandingHandler) requireUserClinics(c *gin.Context) (*models.AppointmentUser, []models.Clinic, bool) {
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

// resolveClinicID picks the clinic for the branding form (single clinic or query/post id).
// Inputs: gin context, user, allowed clinics.
// Output: selected clinic id or zero.
func (h *ClinicBrandingHandler) resolveClinicID(c *gin.Context, user *models.AppointmentUser, allowed []models.Clinic) uint {
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

// saveUploadedImage stores one uploaded image or returns fallback; clearField=1 removes it.
// Inputs: gin context, form field names, upload dir, URL prefix, fallback URL, allowICO for favicon.
// Output: public URL or error.
func (h *ClinicBrandingHandler) saveUploadedImage(c *gin.Context, fieldName, clearField, uploadDir, urlPrefix, fallback string, allowICO bool) (string, error) {
	file, hdr, err := c.Request.FormFile(fieldName)
	if err != nil || file == nil {
		if c.PostForm(clearField) == "1" {
			return "", nil
		}
		return strings.TrimSpace(fallback), nil
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
	case ".ico":
		if !allowICO {
			return "", fmt.Errorf("فرمت تصویر مجاز نیست.")
		}
	default:
		return "", fmt.Errorf("فرمت تصویر مجاز نیست.")
	}
	if hdr.Size > 5<<20 {
		return "", fmt.Errorf("حجم تصویر نباید بیش از ۵ مگابایت باشد.")
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", fmt.Errorf("خطا در ذخیره فایل.")
	}
	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	destPath := filepath.Join(uploadDir, name)
	out, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("خطا در ذخیره فایل.")
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		return "", fmt.Errorf("خطا در ذخیره فایل.")
	}
	return urlPrefix + name, nil
}

// renderForm writes the clinic branding admin page HTML response.
// Inputs: gin context, view model.
// Output: none.
func (h *ClinicBrandingHandler) renderForm(c *gin.Context, view adminviews.ClinicBrandingView) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.ClinicBranding(view).Render(c.Request.Context(), c.Writer)
}

// clinicBrandingFlashMessage maps redirect query codes to Persian UI messages.
// Inputs: flash code from query string.
// Output: localized message or empty string.
func clinicBrandingFlashMessage(code string) string {
	switch code {
	case "saved":
		return "لوگو و آیکون مرکز با موفقیت ذخیره شد."
	case "logo_failed":
		return "خطا در ذخیره لوگوی مرکز."
	case "favicon_failed":
		return "خطا در ذخیره آیکون (favicon) مرکز."
	case "save_failed":
		return "خطا در ذخیره تنظیمات برندینگ مرکز."
	case "clinic_required":
		return "انتخاب مرکز الزامی است."
	case "forbidden":
		return "دسترسی به این مرکز مجاز نیست."
	case "not_found":
		return "مرکز مورد نظر یافت نشد."
	default:
		return ""
	}
}
