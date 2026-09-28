package admin

import (
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/text"
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
			view.EditShortDescription = row.ShortDescription
			view.EditDescription = row.Description
			view.EditIcon = row.Icon
			view.EditColor = row.Color
			view.EditSortOrder = row.SortOrder
			view.EditApproved = row.IsApproved
			view.EditShowInBooking = row.ShowInBooking
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

	name, msg := h.prepareSpecialtyName(c.PostForm("name"), 0)
	if msg != "" {
		h.renderWithMessage(c, user, msg)
		return
	}

	row := specialtyFromForm(c)
	row.Name = name
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

	name, msg := h.prepareSpecialtyName(c.PostForm("name"), id)
	if msg != "" {
		c.Redirect(http.StatusFound, "/admin/specialties?edit="+strconv.FormatUint(uint64(id), 10)+"&msg="+specialtyNameFlash(msg))
		return
	}

	applySpecialtyForm(row, c)
	row.Name = name
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

// specialtyNameMaxRunes سقف nvarchar(100) مدل Specialty است.
const specialtyNameMaxRunes = 100

// prepareSpecialtyName نام تخصص را نرمال و در برابر تکرار بررسی می‌کند.
// ورودی: نام خام و شناسه‌ای که در ویرایش نباید تکراری حساب شود. خروجی: نام نهایی یا پیام خطا.
func (h *SpecialtyHandler) prepareSpecialtyName(raw string, exceptID uint) (string, string) {
	name, err := normalizeSpecialtyName(raw)
	if err != nil {
		return "", err.Error()
	}
	if h.Specialties == nil {
		return "", "خطا در بررسی نام تخصص."
	}
	rows, err := h.Specialties.ListAll()
	if err != nil {
		return "", "خطا در بررسی نام تخصص."
	}
	if specialtyNameExists(rows, name, exceptID) {
		return "", "تخصصی با این نام قبلاً ثبت شده است."
	}
	return name, ""
}

// normalizeSpecialtyName نام مستر تخصص را برای ذخیره آماده می‌کند.
// ورودی: نام خام فرم. خروجی: نام نرمال‌شده یا خطا. متن تخصصی اصلاح نمی‌شود.
func normalizeSpecialtyName(raw string) (string, error) {
	name := text.NormalizePersianText(raw)
	if name == "" {
		return "", errSpecialtyNameRequired
	}
	if utf8.RuneCountInString(name) > specialtyNameMaxRunes {
		return "", errSpecialtyNameTooLong
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", errSpecialtyNameInvalid
		}
	}
	return name, nil
}

// specialtyNameExists نام نرمال‌شده را با ردیف‌های موجود مقایسه می‌کند.
// ورودی: ردیف‌ها، نام نرمال و شناسهٔ مستثنی. خروجی: true اگر تکرار متعلق به رکورد دیگری باشد.
func specialtyNameExists(rows []models.Specialty, name string, exceptID uint) bool {
	name = text.NormalizePersianText(name)
	if name == "" {
		return false
	}
	for _, row := range rows {
		if exceptID > 0 && row.ID == exceptID {
			continue
		}
		if text.NormalizePersianText(row.Name) == name {
			return true
		}
	}
	return false
}

var (
	errSpecialtyNameRequired = errString("نام فارسی الزامی است.")
	errSpecialtyNameTooLong  = errString("نام فارسی بلندتر از حد مجاز است.")
	errSpecialtyNameInvalid  = errString("نام فارسی نویسهٔ نامعتبر دارد.")
)

type errString string

func (e errString) Error() string { return string(e) }

// specialtyNameFlash کد پیام redirect را از متن اعتبارسنجی نام برمی‌گرداند.
// ورودی: پیام فارسی. خروجی: کد query.
func specialtyNameFlash(msg string) string {
	switch msg {
	case errSpecialtyNameTooLong.Error():
		return "name_too_long"
	case errSpecialtyNameInvalid.Error():
		return "name_invalid"
	case "تخصصی با این نام قبلاً ثبت شده است.":
		return "name_duplicate"
	case "خطا در بررسی نام تخصص.":
		return "name_check_failed"
	default:
		return "name_required"
	}
}

// specialtyFromForm مدل تخصص جدید را از فیلدهای فرم می‌سازد.
// ورودی: context درخواست. خروجی: اشاره‌گر Specialty بدون Name (توسط Create ست می‌شود).
func specialtyFromForm(c *gin.Context) *models.Specialty {
	row := &models.Specialty{}
	applySpecialtyForm(row, c)
	return row
}

// applySpecialtyForm فیلدهای اختیاری تخصص (شامل ترتیب و نمایش در نوبت‌دهی) را از فرم روی مدل می‌نویسد.
// ورودی: مدل تخصص و context. خروجی: ندارد (مدل به‌روز می‌شود).
func applySpecialtyForm(row *models.Specialty, c *gin.Context) {
	row.NameEN = strings.TrimSpace(c.PostForm("name_en"))
	row.ShortDescription = strings.TrimSpace(c.PostForm("short_description"))
	row.Description = strings.TrimSpace(c.PostForm("description"))
	row.Icon = strings.TrimSpace(c.PostForm("icon"))
	row.Color = strings.TrimSpace(c.PostForm("color"))
	row.SortOrder = parseSpecialtySortOrder(c.PostForm("sort_order"))
	row.IsApproved = c.PostForm("is_approved") == "1"
	row.ShowInBooking = c.PostForm("show_in_booking") == "1"
}

// parseSpecialtySortOrder مقدار ترتیب نمایش را از فرم به عدد معتبر تبدیل می‌کند.
// ورودی: رشته خام فیلد sort_order. خروجی: عدد بین ۰ تا ۹۹۹ (نامعتبر = ۰).
func parseSpecialtySortOrder(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 {
		return 0
	}
	if n > 999 {
		return 999
	}
	return n
}

func (h *SpecialtyHandler) baseView(user *models.AppointmentUser) adminviews.SpecialtyPageView {
	return adminviews.SpecialtyPageView{
		Nav:               adminviews.BuildAdminNav(adminviews.NavSpecialties, user.Role),
		EditShowInBooking: true,
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
			ID:               row.ID,
			Name:             row.Name,
			NameEN:           row.NameEN,
			ShortDescription: row.ShortDescription,
			Icon:             row.Icon,
			SortOrder:        row.SortOrder,
			IsApproved:       row.IsApproved,
			ShowInBooking:    row.ShowInBooking,
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
	case "name_too_long":
		return "نام فارسی بلندتر از حد مجاز است."
	case "name_invalid":
		return "نام فارسی نویسهٔ نامعتبر دارد."
	case "name_duplicate":
		return "تخصصی با این نام قبلاً ثبت شده است."
	case "name_check_failed":
		return "خطا در بررسی نام تخصص."
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
