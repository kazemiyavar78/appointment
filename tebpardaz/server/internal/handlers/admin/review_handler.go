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

// ReviewAdminHandler مدیریت تأیید نظرات در پنل ادمین است.
type ReviewAdminHandler struct {
	Reviews *repository.ReviewRepo
	Clinics *repository.ClinicRepo
	Doctors *repository.DoctorRepo
	Scope   *auth.ClinicScope
}

// NewReviewAdminHandler سازنده ReviewAdminHandler است.
// ورودی: مخزن نظرات، مراکز، پزشکان و محدوده دسترسی. خروجی: هندلر آماده.
func NewReviewAdminHandler(reviews *repository.ReviewRepo, clinics *repository.ClinicRepo, doctors *repository.DoctorRepo, scope *auth.ClinicScope) *ReviewAdminHandler {
	return &ReviewAdminHandler{Reviews: reviews, Clinics: clinics, Doctors: doctors, Scope: scope}
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
	clinicNames := map[uint]string{}
	doctorNames := map[uint]string{}
	items := make([]adminviews.ReviewAdminItem, 0, len(rows))
	for _, row := range rows {
		ip := strings.TrimSpace(row.IPAddress)
		if ip == "" {
			ip = "—"
		}
		items = append(items, adminviews.ReviewAdminItem{
			ID:         row.ID,
			TargetType: row.TargetType,
			DoctorName: h.doctorLabel(row, doctorNames),
			ClinicName: h.clinicLabel(row.ClinicID, clinicNames),
			AuthorName: strings.TrimSpace(row.AuthorName),
			Rating:     row.Rating,
			Body:       row.Body,
			IPAddress:  ip,
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

// clinicLabel نام مرکز را با کش برمی‌گرداند.
// ورودی: شناسه مرکز و کش نام‌ها. خروجی: نام مرکز یا «نامشخص».
func (h *ReviewAdminHandler) clinicLabel(id uint, cache map[uint]string) string {
	if name, ok := cache[id]; ok {
		return name
	}
	name := "نامشخص"
	if h != nil && h.Clinics != nil && id != 0 {
		if cl, err := h.Clinics.GetByIDForAdmin(id); err == nil && cl != nil {
			if n := strings.TrimSpace(cl.Name); n != "" {
				name = n
			}
		}
	}
	cache[id] = name
	return name
}

// doctorLabel نام پزشک را فقط برای نظرهای پزشک برمی‌گرداند.
// ورودی: ردیف نظر و کش نام پزشکان. خروجی: نام پزشک، یا خالی اگر هدف مرکز باشد.
func (h *ReviewAdminHandler) doctorLabel(row models.Review, cache map[uint]string) string {
	if row.TargetType != models.ReviewTargetDoctor {
		return ""
	}
	if name, ok := cache[row.TargetID]; ok {
		return name
	}
	name := "نامشخص"
	if h != nil && h.Doctors != nil && row.TargetID != 0 {
		if doc, err := h.Doctors.GetByID(row.TargetID); err == nil && doc != nil {
			if n := strings.TrimSpace(doctorDisplayName(*doc)); n != "" {
				name = n
			}
		}
	}
	cache[row.TargetID] = name
	return name
}
