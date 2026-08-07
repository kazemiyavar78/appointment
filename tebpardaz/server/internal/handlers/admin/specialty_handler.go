package admin

import (
	"net/http"
	"strconv"
	"strings"

	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	adminviews "tebpardaz/server/views/admin"

	"github.com/gin-gonic/gin"
)

// SpecialtyHandler manages specialty CRUD for super admins.
type SpecialtyHandler struct {
	Specialties *repository.SpecialtyRepo
}

// NewSpecialtyHandler constructs a SpecialtyHandler.
func NewSpecialtyHandler(specialties *repository.SpecialtyRepo) *SpecialtyHandler {
	return &SpecialtyHandler{Specialties: specialties}
}

// List renders the specialty page with optional edit form prefill.
func (h *SpecialtyHandler) List(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	view := h.baseView(user)
	view.Message = specialtyFlashMessage(c.Query("msg"))

	editID, _ := strconv.ParseUint(c.Query("edit"), 10, 64)
	if editID > 0 {
		row, err := h.Specialties.GetByID(uint(editID))
		if err != nil {
			if view.Message == "" {
				view.Message = "تخصص مورد نظر یافت نشد."
			}
		} else {
			view.EditID = row.ID
			view.EditName = row.Name
			view.EditNameEN = row.NameEN
			view.EditApproved = row.IsApproved
		}
	}

	rows, err := h.Specialties.ListAll()
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load specialties")
		return
	}
	view.Items = toSpecialtyRows(rows)
	h.render(c, view)
}

// Create adds a new specialty from form fields.
func (h *SpecialtyHandler) Create(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		h.renderWithMessage(c, user, "نام فارسی الزامی است.")
		return
	}

	row := &models.Specialty{
		Name:       name,
		NameEN:     strings.TrimSpace(c.PostForm("name_en")),
		IsApproved: c.PostForm("is_approved") == "1",
	}
	if err := h.Specialties.Create(row); err != nil {
		h.renderWithMessage(c, user, "خطا در ایجاد تخصص.")
		return
	}
	c.Redirect(http.StatusFound, "/admin/specialties?msg=created")
}

// Update saves changes to an existing specialty.
func (h *SpecialtyHandler) Update(c *gin.Context) {
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

	row, err := h.Specialties.GetByID(id)
	if err != nil {
		h.renderWithMessage(c, user, "تخصص مورد نظر یافت نشد.")
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		c.Redirect(http.StatusFound, "/admin/specialties?edit="+strconv.FormatUint(uint64(id), 10)+"&msg=name_required")
		return
	}

	row.Name = name
	row.NameEN = strings.TrimSpace(c.PostForm("name_en"))
	row.IsApproved = c.PostForm("is_approved") == "1"
	if err := h.Specialties.Update(row); err != nil {
		c.Redirect(http.StatusFound, "/admin/specialties?edit="+strconv.FormatUint(uint64(id), 10)+"&msg=update_failed")
		return
	}
	c.Redirect(http.StatusFound, "/admin/specialties?msg=updated")
}

// Delete removes a specialty by id.
func (h *SpecialtyHandler) Delete(c *gin.Context) {
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

	if err := h.Specialties.Delete(id); err != nil {
		h.renderWithMessage(c, user, "خطا در حذف تخصص. ممکن است به پزشکان مرتبط باشد.")
		return
	}
	c.Redirect(http.StatusFound, "/admin/specialties?msg=deleted")
}

func (h *SpecialtyHandler) baseView(user *models.AppointmentUser) adminviews.SpecialtyPageView {
	return adminviews.SpecialtyPageView{
		Nav: adminviews.BuildAdminNav(adminviews.NavSpecialties, user.Role),
	}
}

func (h *SpecialtyHandler) renderWithMessage(c *gin.Context, user *models.AppointmentUser, message string) {
	view := h.baseView(user)
	view.Message = message
	rows, err := h.Specialties.ListAll()
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load specialties")
		return
	}
	view.Items = toSpecialtyRows(rows)
	h.render(c, view)
}

func (h *SpecialtyHandler) render(c *gin.Context, view adminviews.SpecialtyPageView) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.Specialties(view).Render(c.Request.Context(), c.Writer)
}

func toSpecialtyRows(rows []models.Specialty) []adminviews.SpecialtyRow {
	out := make([]adminviews.SpecialtyRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, adminviews.SpecialtyRow{
			ID:         row.ID,
			Name:       row.Name,
			NameEN:     row.NameEN,
			IsApproved: row.IsApproved,
		})
	}
	return out
}

func specialtyFlashMessage(code string) string {
	switch code {
	case "created":
		return "تخصص با موفقیت ایجاد شد."
	case "updated":
		return "تخصص با موفقیت بروزرسانی شد."
	case "deleted":
		return "تخصص حذف شد."
	case "name_required":
		return "نام فارسی الزامی است."
	case "update_failed":
		return "خطا در بروزرسانی تخصص."
	default:
		return ""
	}
}

func parseUintParam(c *gin.Context, name string) (uint, error) {
	v, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(v), nil
}
