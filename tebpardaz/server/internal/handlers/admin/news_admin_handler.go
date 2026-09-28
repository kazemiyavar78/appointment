package admin

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/news"
	adminviews "tebpardaz/server/views/admin"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

const newsUploadDir = "static/uploads/news"

var newsStripTags = regexp.MustCompile(`(?i)<[^>]+>`)

// NewsAdminHandler manages news list/create/edit in the back-office.
type NewsAdminHandler struct {
	News  *news.Service
	Scope *auth.ClinicScope
}

// NewNewsAdminHandler constructs a NewsAdminHandler.
// Inputs: news service, clinic scope.
// Output: pointer to NewsAdminHandler.
func NewNewsAdminHandler(svc *news.Service, scope *auth.ClinicScope) *NewsAdminHandler {
	return &NewsAdminHandler{News: svc, Scope: scope}
}

// List renders the admin news list.
func (h *NewsAdminHandler) List(c *gin.Context) {
	user, allowed, ok := h.requireUserClinics(c)
	if !ok {
		return
	}

	ids := clinicIDs(allowed)
	items, err := h.News.ListAdmin(ids)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load news")
		return
	}

	view := adminviews.NewsListView{
		Nav:     adminviews.BuildAdminNav(adminviews.NavNews, user.Role),
		Message: newsFlashMessage(c.Query("msg")),
		Items:   toNewsListRows(items),
	}
	h.renderList(c, view)
}

// Editor renders the create/edit news form.
func (h *NewsAdminHandler) Editor(c *gin.Context) {
	user, allowed, ok := h.requireUserClinics(c)
	if !ok {
		return
	}

	view := adminviews.NewsEditorView{
		Nav:            adminviews.BuildAdminNav(adminviews.NavNews, user.Role),
		Message:        newsFlashMessage(c.Query("msg")),
		FormAction:     "/admin/news",
		Clinics:        toNewsClinicOptions(allowed, 0),
		ShowClinicPick: h.showClinicPicker(user, allowed),
		IsPublished:    true,
	}
	if len(allowed) == 1 {
		view.ClinicID = allowed[0].ID
	}

	editID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if editID > 0 {
		row, err := h.News.GetForAdmin(uint(editID), clinicIDs(allowed))
		if err != nil {
			c.Redirect(http.StatusFound, "/admin/news?msg=not_found")
			return
		}
		view.EditID = row.ID
		view.ClinicID = row.ClinicID
		view.TitleHTML = row.Title
		view.ExcerptHTML = row.Excerpt
		view.CoverURL = row.CoverURL
		view.BodyHTML = row.Body
		view.IsPublished = row.IsPublished
		view.FormAction = "/admin/news/" + strconv.FormatUint(uint64(row.ID), 10) + "/update"
		view.Clinics = toNewsClinicOptions(allowed, row.ClinicID)
	}

	h.renderEditor(c, view)
}

// Create persists a new news article from the editor form.
func (h *NewsAdminHandler) Create(c *gin.Context) {
	user, allowed, ok := h.requireUserClinics(c)
	if !ok {
		return
	}
	in, errMsg := h.parseSaveInput(c, user, allowed)
	if errMsg != "" {
		h.renderEditor(c, h.editorFromForm(c, user, allowed, 0, "/admin/news", errMsg))
		return
	}
	if _, err := h.News.Create(in, clinicIDs(allowed)); err != nil {
		fmt.Println(err)
		msg := "خطا در ایجاد خبر."
		if err == news.ErrInvalidInput {
			msg = "عنوان الزامی است و خلاصه نباید بیش از ۱۲۰ کاراکتر باشد."
		} else if err == news.ErrForbidden {
			msg = "اجازه انتشار برای این مرکز را ندارید."
		}
		h.renderEditor(c, h.editorFromForm(c, user, allowed, 0, "/admin/news", msg))
		return
	}
	c.Redirect(http.StatusFound, "/admin/news?msg=created")
}

// Update saves changes to an existing news article.
func (h *NewsAdminHandler) Update(c *gin.Context) {
	user, allowed, ok := h.requireUserClinics(c)
	if !ok {
		return
	}
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	action := "/admin/news/" + strconv.FormatUint(uint64(id), 10) + "/update"
	in, errMsg := h.parseSaveInput(c, user, allowed)
	if errMsg != "" {
		h.renderEditor(c, h.editorFromForm(c, user, allowed, id, action, errMsg))
		return
	}
	if _, err := h.News.Update(id, in, clinicIDs(allowed)); err != nil {
		msg := "خطا در بروزرسانی خبر."
		if err == news.ErrInvalidInput {
			msg = "عنوان الزامی است و خلاصه نباید بیش از ۱۲۰ کاراکتر باشد."
		} else if err == news.ErrForbidden || err == news.ErrNotFound {
			c.Redirect(http.StatusFound, "/admin/news?msg=not_found")
			return
		}
		h.renderEditor(c, h.editorFromForm(c, user, allowed, id, action, msg))
		return
	}
	c.Redirect(http.StatusFound, "/admin/news?msg=updated")
}

// Delete removes a news article.
func (h *NewsAdminHandler) Delete(c *gin.Context) {
	_, allowed, ok := h.requireUserClinics(c)
	if !ok {
		return
	}
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err := h.News.Delete(id, clinicIDs(allowed)); err != nil {
		c.Redirect(http.StatusFound, "/admin/news?msg=delete_failed")
		return
	}
	c.Redirect(http.StatusFound, "/admin/news?msg=deleted")
}

// UploadImage accepts a multipart image and returns its public URL as JSON.
func (h *NewsAdminHandler) UploadImage(c *gin.Context) {
	if _, _, ok := h.requireUserClinics(c); !ok {
		return
	}
	file, hdr, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "فایل تصویر الزامی است."})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "فرمت تصویر مجاز نیست."})
		return
	}
	if hdr.Size > 5<<20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "حجم تصویر نباید بیش از ۵ مگابایت باشد."})
		return
	}

	if err := os.MkdirAll(newsUploadDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در ذخیره فایل."})
		return
	}
	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	destPath := filepath.Join(newsUploadDir, name)
	out, err := os.Create(destPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در ذخیره فایل."})
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "خطا در ذخیره فایل."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": "/static/uploads/news/" + name})
}

func (h *NewsAdminHandler) requireUserClinics(c *gin.Context) (*models.AppointmentUser, []models.Clinic, bool) {
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

func (h *NewsAdminHandler) showClinicPicker(user *models.AppointmentUser, allowed []models.Clinic) bool {
	if len(allowed) <= 1 {
		return false
	}
	role := constants.UserRole(user.Role)
	return role == constants.UserRoleOrganAdmin ||
		role == constants.UserRoleSuperAdmin ||
		(role == constants.UserRoleEditor && user.OrganizationID != nil)
}

func (h *NewsAdminHandler) parseSaveInput(c *gin.Context, user *models.AppointmentUser, allowed []models.Clinic) (news.SaveInput, string) {
	clinicID := uint(0)
	if h.showClinicPicker(user, allowed) {
		id, _ := strconv.ParseUint(c.PostForm("clinic_id"), 10, 64)
		clinicID = uint(id)
	} else {
		clinicID = allowed[0].ID
	}
	if clinicID == 0 {
		return news.SaveInput{}, "انتخاب مرکز الزامی است."
	}
	coverURL := strings.TrimSpace(c.PostForm("cover_url"))
	return news.SaveInput{
		ClinicID:    clinicID,
		Title:       c.PostForm("title"),
		Excerpt:     c.PostForm("excerpt"),
		CoverURL:    coverURL,
		Body:        c.PostForm("body"),
		IsPublished: c.PostForm("is_published") == "1",
		PublishedAt: time.Now(),
	}, ""
}

func (h *NewsAdminHandler) editorFromForm(c *gin.Context, user *models.AppointmentUser, allowed []models.Clinic, editID uint, action, message string) adminviews.NewsEditorView {
	clinicID, _ := strconv.ParseUint(c.PostForm("clinic_id"), 10, 64)
	if clinicID == 0 && len(allowed) > 0 {
		clinicID = uint64(allowed[0].ID)
	}
	return adminviews.NewsEditorView{
		Nav:            adminviews.BuildAdminNav(adminviews.NavNews, user.Role),
		Message:        message,
		EditID:         editID,
		ClinicID:       uint(clinicID),
		TitleHTML:      c.PostForm("title"),
		ExcerptHTML:    c.PostForm("excerpt"),
		CoverURL:       c.PostForm("cover_url"),
		BodyHTML:       c.PostForm("body"),
		IsPublished:    c.PostForm("is_published") == "1",
		FormAction:     action,
		Clinics:        toNewsClinicOptions(allowed, uint(clinicID)),
		ShowClinicPick: h.showClinicPicker(user, allowed),
	}
}

func (h *NewsAdminHandler) renderList(c *gin.Context, view adminviews.NewsListView) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.NewsList(view).Render(c.Request.Context(), c.Writer)
}

func (h *NewsAdminHandler) renderEditor(c *gin.Context, view adminviews.NewsEditorView) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.NewsEditor(view).Render(c.Request.Context(), c.Writer)
}

func clinicIDs(clinics []models.Clinic) []uint {
	ids := make([]uint, 0, len(clinics))
	for _, c := range clinics {
		ids = append(ids, c.ID)
	}
	return ids
}

func toNewsClinicOptions(clinics []models.Clinic, selected uint) []adminviews.ClinicOption {
	opts := make([]adminviews.ClinicOption, 0, len(clinics))
	for _, c := range clinics {
		opts = append(opts, adminviews.ClinicOption{
			ID:       c.ID,
			Name:     c.Name,
			Selected: c.ID == selected,
		})
	}
	return opts
}

func toNewsListRows(items []news.Item) []adminviews.NewsListRow {
	out := make([]adminviews.NewsListRow, 0, len(items))
	for _, item := range items {
		out = append(out, adminviews.NewsListRow{
			ID:          item.ID,
			TitlePlain:  strings.TrimSpace(newsStripTags.ReplaceAllString(item.Title, "")),
			ClinicName:  item.ClinicName,
			CoverURL:    item.CoverURL,
			IsPublished: item.IsPublished,
			PublishedAt: item.PublishedAt,
		})
	}
	return out
}

func newsFlashMessage(code string) string {
	switch code {
	case "created":
		return "خبر با موفقیت ایجاد شد."
	case "updated":
		return "خبر با موفقیت بروزرسانی شد."
	case "deleted":
		return "خبر حذف شد."
	case "not_found":
		return "خبر مورد نظر یافت نشد."
	case "delete_failed":
		return "خطا در حذف خبر."
	default:
		return ""
	}
}
