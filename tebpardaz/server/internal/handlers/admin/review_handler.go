package admin

import (
	"net/http"
	"strconv"

	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	adminviews "tebpardaz/server/views/admin"

	"github.com/gin-gonic/gin"
)

// ReviewAdminHandler مدیریت تأیید نظرات در پنل ادمین است.
type ReviewAdminHandler struct {
	Reviews *repository.ReviewRepo
	Clinics *repository.ClinicRepo
	Scope   *auth.ClinicScope
}

// NewReviewAdminHandler سازنده ReviewAdminHandler است.
func NewReviewAdminHandler(reviews *repository.ReviewRepo, clinics *repository.ClinicRepo, scope *auth.ClinicScope) *ReviewAdminHandler {
	return &ReviewAdminHandler{Reviews: reviews, Clinics: clinics, Scope: scope}
}

// List نظرات در انتظار تأیید را برای مراکز مجاز کاربر نشان می‌دهد.
func (h *ReviewAdminHandler) List(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	ids := h.allowedClinicIDs(user)
	rows, err := h.Reviews.ListPending(ids)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت نظرات")
		return
	}
	items := make([]adminviews.ReviewAdminItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, adminviews.ReviewAdminItem{
			ID:         row.ID,
			TargetType: row.TargetType,
			TargetID:   row.TargetID,
			ClinicID:   row.ClinicID,
			AuthorName: row.AuthorName,
			Rating:     row.Rating,
			Body:       row.Body,
		})
	}
	view := adminviews.ReviewAdminView{
		Nav:     adminviews.BuildAdminNav(adminviews.NavReviews, user.Role),
		Items:   items,
		Message: c.Query("msg"),
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.ReviewAdminPage(view).Render(c.Request.Context(), c.Writer)
}

// Decide نظر را تأیید یا حذف می‌کند.
func (h *ReviewAdminHandler) Decide(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	id, _ := strconv.ParseUint(c.PostForm("id"), 10, 64)
	row, err := h.Reviews.GetByID(uint(id))
	if err != nil || row == nil {
		c.Redirect(http.StatusFound, "/admin/reviews?msg=notfound")
		return
	}
	if !h.canManageClinic(user, row.ClinicID) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	action := c.PostForm("action")
	if action == "approve" {
		_ = h.Reviews.SetApproved(row.ID, true)
		c.Redirect(http.StatusFound, "/admin/reviews?msg=approved")
		return
	}
	_ = h.Reviews.Delete(row.ID)
	c.Redirect(http.StatusFound, "/admin/reviews?msg=deleted")
}

func (h *ReviewAdminHandler) allowedClinicIDs(user *models.AppointmentUser) []uint {
	if h.Scope == nil {
		return nil
	}
	rows, err := h.Scope.AllowedClinics(user)
	if err != nil || len(rows) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(rows))
	for _, cl := range rows {
		ids = append(ids, cl.ID)
	}
	return ids
}

func (h *ReviewAdminHandler) canManageClinic(user *models.AppointmentUser, clinicID uint) bool {
	if h.Scope == nil {
		return false
	}
	return h.Scope.CanAccess(user, clinicID)
}
