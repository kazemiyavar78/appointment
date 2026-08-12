package admin

import (
	"net/http"
	"strconv"
	"strings"

	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	adminviews "tebpardaz/server/views/admin"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UserAdminHandler manages appointment users for super admins.
type UserAdminHandler struct {
	Users         *repository.UserRepo
	Clinics       *repository.ClinicRepo
	Organizations *repository.OrganizationRepo
}

// NewUserAdminHandler constructs a UserAdminHandler.
// Inputs: users, clinics, and organizations repositories.
// Output: pointer to UserAdminHandler.
func NewUserAdminHandler(
	users *repository.UserRepo,
	clinics *repository.ClinicRepo,
	organizations *repository.OrganizationRepo,
) *UserAdminHandler {
	return &UserAdminHandler{
		Users:         users,
		Clinics:       clinics,
		Organizations: organizations,
	}
}

// List renders the user management page with optional edit form prefill.
func (h *UserAdminHandler) List(c *gin.Context) {
	actor, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	view, err := h.buildPageView(actor, c.Query("msg"), "")
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load users")
		return
	}

	editID, _ := strconv.ParseUint(c.Query("edit"), 10, 64)
	if editID > 0 {
		row, err := h.Users.FindByID(uint(editID))
		if err != nil {
			if view.Message == "" {
				view.Message = "کاربر مورد نظر یافت نشد."
			}
		} else {
			view.EditID = row.ID
			view.EditUsername = row.Username
			view.EditRole = row.Role
			view.EditIsActive = row.IsActive
			if row.ClinicID != nil {
				view.EditClinicID = *row.ClinicID
			}
			if row.OrganizationID != nil {
				view.EditOrganizationID = *row.OrganizationID
			}
		}
	}

	h.render(c, view)
}

// Create adds a new appointment user from form fields.
func (h *UserAdminHandler) Create(c *gin.Context) {
	actor, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	role := strings.TrimSpace(c.PostForm("role"))
	clinicID, orgID, scopeErr := h.parseScope(role, c.PostForm("clinic_id"), c.PostForm("organization_id"))
	if username == "" {
		h.renderWithMessage(c, actor, "نام کاربری الزامی است.")
		return
	}
	if password == "" {
		h.renderWithMessage(c, actor, "رمز عبور الزامی است.")
		return
	}
	if !isManagedRole(role) {
		h.renderWithMessage(c, actor, "نقش انتخاب‌شده معتبر نیست.")
		return
	}
	if scopeErr != "" {
		h.renderWithMessage(c, actor, scopeErr)
		return
	}

	user := &models.AppointmentUser{
		Username:       username,
		Role:           role,
		ClinicID:       clinicID,
		OrganizationID: orgID,
		IsActive:       true,
	}
	if err := h.Users.Create(user, password); err != nil {
		if err == repository.ErrUsernameTaken {
			h.renderWithMessage(c, actor, "این نام کاربری قبلاً ثبت شده است.")
			return
		}
		h.renderWithMessage(c, actor, "خطا در ایجاد کاربر.")
		return
	}
	c.Redirect(http.StatusFound, "/admin/users?msg=created")
}

// Update saves changes to an existing appointment user.
func (h *UserAdminHandler) Update(c *gin.Context) {
	actor, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	user, err := h.Users.FindByID(id)
	if err != nil {
		h.renderWithMessage(c, actor, "کاربر مورد نظر یافت نشد.")
		return
	}

	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	role := strings.TrimSpace(c.PostForm("role"))
	clinicID, orgID, scopeErr := h.parseScope(role, c.PostForm("clinic_id"), c.PostForm("organization_id"))
	if username == "" {
		c.Redirect(http.StatusFound, "/admin/users?edit="+strconv.FormatUint(uint64(id), 10)+"&msg=username_required")
		return
	}
	if !isManagedRole(role) {
		c.Redirect(http.StatusFound, "/admin/users?edit="+strconv.FormatUint(uint64(id), 10)+"&msg=invalid_role")
		return
	}
	if scopeErr != "" {
		view, err := h.buildPageView(actor, "", scopeErr)
		if err != nil {
			c.String(http.StatusInternalServerError, "failed to load users")
			return
		}
		view.EditID = user.ID
		view.EditUsername = username
		view.EditRole = role
		view.EditIsActive = user.IsActive
		if clinicID != nil {
			view.EditClinicID = *clinicID
		}
		if orgID != nil {
			view.EditOrganizationID = *orgID
		}
		h.render(c, view)
		return
	}

	user.Username = username
	user.Role = role
	user.ClinicID = clinicID
	user.OrganizationID = orgID
	if err := h.Users.Update(user, password); err != nil {
		if err == repository.ErrUsernameTaken {
			c.Redirect(http.StatusFound, "/admin/users?edit="+strconv.FormatUint(uint64(id), 10)+"&msg=username_taken")
			return
		}
		c.Redirect(http.StatusFound, "/admin/users?edit="+strconv.FormatUint(uint64(id), 10)+"&msg=update_failed")
		return
	}
	c.Redirect(http.StatusFound, "/admin/users?msg=updated")
}

// Deactivate sets IsActive=false for the target user.
func (h *UserAdminHandler) Deactivate(c *gin.Context) {
	h.setActive(c, false)
}

// Activate sets IsActive=true for the target user.
func (h *UserAdminHandler) Activate(c *gin.Context) {
	h.setActive(c, true)
}

func (h *UserAdminHandler) setActive(c *gin.Context, active bool) {
	actor, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if !active && id == actor.ID {
		h.renderWithMessage(c, actor, "نمی‌توانید حساب خود را غیرفعال کنید.")
		return
	}
	if _, err := h.Users.FindByID(id); err != nil {
		h.renderWithMessage(c, actor, "کاربر مورد نظر یافت نشد.")
		return
	}
	if err := h.Users.SetActive(id, active); err != nil {
		h.renderWithMessage(c, actor, "خطا در تغییر وضعیت کاربر.")
		return
	}
	if active {
		c.Redirect(http.StatusFound, "/admin/users?msg=activated")
		return
	}
	c.Redirect(http.StatusFound, "/admin/users?msg=deactivated")
}

func (h *UserAdminHandler) buildPageView(actor *models.AppointmentUser, msgCode, message string) (adminviews.UsersPageView, error) {
	view := adminviews.UsersPageView{
		Nav: adminviews.BuildAdminNav(adminviews.NavUsers, actor.Role),
	}
	if message != "" {
		view.Message = message
	} else {
		view.Message = userFlashMessage(msgCode)
	}

	users, err := h.Users.ListAll()
	if err != nil {
		return view, err
	}
	clinics, err := h.Clinics.ListAllForAdmin()
	if err != nil && err != gorm.ErrRecordNotFound {
		return view, err
	}
	orgs, err := h.Organizations.ListAll()
	if err != nil && err != gorm.ErrRecordNotFound {
		return view, err
	}

	clinicNames := make(map[uint]string, len(clinics))
	view.Clinics = make([]adminviews.UserSelectOption, 0, len(clinics))
	for _, clinic := range clinics {
		clinicNames[clinic.ID] = clinic.Name
		view.Clinics = append(view.Clinics, adminviews.UserSelectOption{
			ID:   clinic.ID,
			Name: clinic.Name,
		})
	}

	orgNames := make(map[uint]string, len(orgs))
	view.Organizations = make([]adminviews.UserSelectOption, 0, len(orgs))
	for _, org := range orgs {
		orgNames[org.ID] = org.Name
		view.Organizations = append(view.Organizations, adminviews.UserSelectOption{
			ID:   org.ID,
			Name: org.Name,
		})
	}

	view.Items = make([]adminviews.UserRow, 0, len(users))
	for _, u := range users {
		row := adminviews.UserRow{
			ID:       u.ID,
			Username: u.Username,
			Role:     u.Role,
			RoleLabel: roleLabel(u.Role),
			IsActive: u.IsActive,
			IsSelf:   u.ID == actor.ID,
		}
		if u.ClinicID != nil {
			row.ScopeLabel = clinicNames[*u.ClinicID]
			if row.ScopeLabel == "" {
				row.ScopeLabel = "کلینیک #" + strconv.FormatUint(uint64(*u.ClinicID), 10)
			}
		} else if u.OrganizationID != nil {
			row.ScopeLabel = orgNames[*u.OrganizationID]
			if row.ScopeLabel == "" {
				row.ScopeLabel = "ارگان #" + strconv.FormatUint(uint64(*u.OrganizationID), 10)
			}
		} else {
			row.ScopeLabel = "همه"
		}
		view.Items = append(view.Items, row)
	}
	return view, nil
}

func (h *UserAdminHandler) renderWithMessage(c *gin.Context, actor *models.AppointmentUser, message string) {
	view, err := h.buildPageView(actor, "", message)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load users")
		return
	}
	h.render(c, view)
}

func (h *UserAdminHandler) render(c *gin.Context, view adminviews.UsersPageView) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.Users(view).Render(c.Request.Context(), c.Writer)
}

// parseScope validates role-specific clinic/organization assignment.
// Inputs: role string and raw clinic_id / organization_id form values.
// Output: clinicID, organizationID pointers (may be nil), or a Persian error message.
func (h *UserAdminHandler) parseScope(role, clinicRaw, orgRaw string) (*uint, *uint, string) {
	switch constants.UserRole(role) {
	case constants.UserRoleSuperAdmin:
		return nil, nil, ""
	case constants.UserRoleClinicAdmin:
		id, err := strconv.ParseUint(strings.TrimSpace(clinicRaw), 10, 64)
		if err != nil || id == 0 {
			return nil, nil, "انتخاب کلینیک برای ادمین کلینیک الزامی است."
		}
		if _, err := h.Clinics.GetByIDForAdmin(uint(id)); err != nil {
			return nil, nil, "کلینیک انتخاب‌شده یافت نشد."
		}
		clinicID := uint(id)
		return &clinicID, nil, ""
	case constants.UserRoleOrganAdmin:
		id, err := strconv.ParseUint(strings.TrimSpace(orgRaw), 10, 64)
		if err != nil || id == 0 {
			return nil, nil, "انتخاب ارگان برای ادمین ارگان الزامی است."
		}
		if _, err := h.Organizations.GetByID(uint(id)); err != nil {
			return nil, nil, "ارگان انتخاب‌شده یافت نشد."
		}
		orgID := uint(id)
		return nil, &orgID, ""
	default:
		return nil, nil, "نقش انتخاب‌شده معتبر نیست."
	}
}

func isManagedRole(role string) bool {
	switch constants.UserRole(role) {
	case constants.UserRoleSuperAdmin, constants.UserRoleClinicAdmin, constants.UserRoleOrganAdmin:
		return true
	default:
		return false
	}
}

func roleLabel(role string) string {
	switch constants.UserRole(role) {
	case constants.UserRoleSuperAdmin:
		return "سوپر ادمین"
	case constants.UserRoleClinicAdmin:
		return "ادمین کلینیک"
	case constants.UserRoleOrganAdmin:
		return "ادمین ارگان"
	case constants.UserRoleAdmin:
		return "ادمین"
	case constants.UserRoleEditor:
		return "ویراستار"
	default:
		return role
	}
}

func userFlashMessage(code string) string {
	switch code {
	case "created":
		return "کاربر با موفقیت ایجاد شد."
	case "updated":
		return "کاربر با موفقیت بروزرسانی شد."
	case "deactivated":
		return "کاربر غیرفعال شد."
	case "activated":
		return "کاربر فعال شد."
	case "username_required":
		return "نام کاربری الزامی است."
	case "username_taken":
		return "این نام کاربری قبلاً ثبت شده است."
	case "invalid_role":
		return "نقش انتخاب‌شده معتبر نیست."
	case "update_failed":
		return "خطا در بروزرسانی کاربر."
	default:
		return ""
	}
}
