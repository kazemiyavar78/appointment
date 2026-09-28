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

// ListPackages صفحه مدیریت بسته‌های خدمات را همراه با فرم ایجاد یا ویرایش رندر می‌کند.
// ورودی: c کانتکست Gin (حاوی پارامترهای اختیاری edit و msg).
// خروجی: صفحه HTML بسته‌های خدمات.
func (h *ServiceHandler) ListPackages(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	view := h.packageView(user)
	view.Message = serviceFlashMessage(c.Query("msg"))

	editID, _ := strconv.ParseUint(c.Query("edit"), 10, 64)
	var assignedMap map[uint]struct{}
	if editID > 0 {
		row, err := h.Services.GetPackageByID(uint(editID))
		if err != nil {
			if view.Message == "" {
				view.Message = "بسته مورد نظر یافت نشد."
			}
		} else {
			view.EditID = row.ID
			view.EditName = row.Name
			view.EditDescription = row.Description
			assignedIDs, _ := h.Services.ListServiceIDsByPackageID(row.ID)
			assignedMap = make(map[uint]struct{}, len(assignedIDs))
			for _, id := range assignedIDs {
				assignedMap[id] = struct{}{}
			}
		}
	}

	allServices, err := h.Services.ListAll()
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در بارگذاری لیست خدمات")
		return
	}
	view.Services, err = h.serviceAssignOptions(allServices, assignedMap)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در بارگذاری بسته‌های خدمات")
		return
	}

	if !h.loadPackageRows(c, &view) {
		return
	}
	h.renderPackages(c, view)
}

// CreatePackage یک بسته خدمات جدید ایجاد و خدمات انتخاب‌شده را به آن منتسب می‌کند.
// ورودی: c کانتکست Gin (حاوی name، description و آرایه service_ids).
// خروجی: ریدایرکت به صفحه بسته‌ها با پیام موفقیت یا خطا.
func (h *ServiceHandler) CreatePackage(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	name, description, msg := parseServiceFields(c)
	if msg != "" {
		h.renderPackagesWithMessage(c, user, "نام بسته الزامی است.")
		return
	}

	row := &models.ServicePackage{Name: name, Description: description}
	if err := h.Services.CreatePackage(row); err != nil {
		h.renderPackagesWithMessage(c, user, "خطا در ایجاد بسته خدمات.")
		return
	}
	if err := h.Services.ReplacePackageServices(row.ID, parsePostedUintIDs(c, "service_ids")); err != nil {
		c.Redirect(http.StatusFound, "/admin/services/packages?edit="+strconv.FormatUint(uint64(row.ID), 10)+"&msg=pkg_update_failed")
		return
	}
	c.Redirect(http.StatusFound, "/admin/services/packages?msg=pkg_created")
}

// UpdatePackage تغییرات یک بسته موجود و انتصاب خدمات آن را ذخیره می‌کند.
// ورودی: c کانتکست Gin (حاوی شناسه id در URL و فیلدهای فرم).
// خروجی: ریدایرکت به صفحه بسته‌ها.
func (h *ServiceHandler) UpdatePackage(c *gin.Context) {
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
	row, err := h.Services.GetPackageByID(id)
	if err != nil {
		h.renderPackagesWithMessage(c, user, "بسته مورد نظر یافت نشد.")
		return
	}

	name, description, msg := parseServiceFields(c)
	if msg != "" {
		c.Redirect(http.StatusFound, "/admin/services/packages?edit="+strconv.FormatUint(uint64(id), 10)+"&msg=pkg_name_required")
		return
	}

	row.Name = name
	row.Description = description
	if err := h.Services.UpdatePackage(row); err != nil {
		c.Redirect(http.StatusFound, "/admin/services/packages?edit="+strconv.FormatUint(uint64(id), 10)+"&msg=pkg_update_failed")
		return
	}
	if err := h.Services.ReplacePackageServices(row.ID, parsePostedUintIDs(c, "service_ids")); err != nil {
		c.Redirect(http.StatusFound, "/admin/services/packages?edit="+strconv.FormatUint(uint64(id), 10)+"&msg=pkg_update_failed")
		return
	}
	c.Redirect(http.StatusFound, "/admin/services/packages?msg=pkg_updated")
}

// DeletePackage یک بسته خدمات را حذف می‌کند؛ خدمات کاتالوگ باقی می‌مانند.
// ورودی: c کانتکست Gin (حاوی شناسه id در URL).
// خروجی: ریدایرکت به صفحه بسته‌ها با پیام حذف.
func (h *ServiceHandler) DeletePackage(c *gin.Context) {
	if _, ok := auth.UserFromGin(c); !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err := h.Services.DeletePackage(id); err != nil {
		c.Redirect(http.StatusFound, "/admin/services/packages?msg=pkg_delete_failed")
		return
	}
	c.Redirect(http.StatusFound, "/admin/services/packages?msg=pkg_deleted")
}

// packageView مدل خالی صفحه بسته‌های خدمات را برای کاربر جاری می‌سازد.
// ورودی: user کاربر لاگین شده.
// خروجی: ساختار ServicePackagePageView با سایدبار متناسب با نقش کاربر.
func (h *ServiceHandler) packageView(user *models.AppointmentUser) adminviews.ServicePackagePageView {
	return adminviews.ServicePackagePageView{
		Nav: adminviews.BuildAdminNav(adminviews.NavServicePackages, user.Role),
	}
}

// loadPackageRows ردیف‌های جدول بسته‌ها را در مدل صفحه بارگذاری می‌کند.
// ورودی: c کانتکست Gin، view اشاره‌گر به مدل صفحه.
// خروجی: false اگر بارگذاری با خطا مواجه شود.
func (h *ServiceHandler) loadPackageRows(c *gin.Context, view *adminviews.ServicePackagePageView) bool {
	rows, err := h.Services.ListPackages()
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در بارگذاری لیست بسته‌ها")
		return false
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	counts, err := h.Services.CountServicesByPackageIDs(ids)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در بارگذاری تعداد خدمات بسته‌ها")
		return false
	}
	view.Items = toServicePackageRows(rows, counts)
	return true
}

// renderPackagesWithMessage صفحه بسته‌ها را با یک پیام خطا دوباره رندر می‌کند.
// ورودی: c کانتکست Gin، user کاربر جاری، message پیام مورد نظر.
// خروجی: ندارد (رندر پاسخ HTML در کانتکست).
func (h *ServiceHandler) renderPackagesWithMessage(c *gin.Context, user *models.AppointmentUser, message string) {
	view := h.packageView(user)
	view.Message = message
	allServices, err := h.Services.ListAll()
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در بارگذاری لیست خدمات")
		return
	}
	view.Services, err = h.serviceAssignOptions(allServices, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در بارگذاری بسته‌های خدمات")
		return
	}
	if !h.loadPackageRows(c, &view) {
		return
	}
	h.renderPackages(c, view)
}

// renderPackages صفحه HTML بسته‌های خدمات را خروجی می‌دهد.
// ورودی: c کانتکست Gin، view مدل داده صفحه.
// خروجی: ندارد (رندر کامپوننت templ).
func (h *ServiceHandler) renderPackages(c *gin.Context, view adminviews.ServicePackagePageView) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.ServicePackages(view).Render(c.Request.Context(), c.Writer)
}

// parsePostedUintIDs شناسه‌های عددی یک فیلد آرایه‌ای فرم را استخراج می‌کند.
// ورودی: c کانتکست Gin، key نام فیلد فرم.
// خروجی: اسلایس شناسه‌های معتبر بزرگ‌تر از صفر.
func parsePostedUintIDs(c *gin.Context, key string) []uint {
	rawIDs := c.PostFormArray(key)
	ids := make([]uint, 0, len(rawIDs))
	for _, raw := range rawIDs {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			continue
		}
		ids = append(ids, uint(id))
	}
	return ids
}

// serviceAssignOptions گزینه‌های چک‌باکس خدمت را همراه با نام بسته‌های هر خدمت می‌سازد.
// ورودی: rows خدمات، assignedMap مجموعه شناسه‌های انتخاب‌شده (می‌تواند nil باشد).
// خروجی: گزینه‌های انتصاب یا خطای دیتابیس.
func (h *ServiceHandler) serviceAssignOptions(rows []models.Service, assignedMap map[uint]struct{}) ([]adminviews.ServiceAssignOption, error) {
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	packages, err := h.Services.ListPackageNamesByServiceIDs(ids)
	if err != nil {
		return nil, err
	}
	return toServiceAssignOptions(rows, assignedMap, packages), nil
}

// toServiceAssignOptions رکوردهای خدمت را به گزینه‌های چک‌باکس تبدیل می‌کند.
// ورودی: rows خدمات، assignedMap مجموعه انتخاب‌شده، packages نگاشت شناسه خدمت به نام بسته‌ها.
// خروجی: اسلایس ServiceAssignOption.
func toServiceAssignOptions(rows []models.Service, assignedMap map[uint]struct{}, packages map[uint][]string) []adminviews.ServiceAssignOption {
	out := make([]adminviews.ServiceAssignOption, 0, len(rows))
	for _, s := range rows {
		var selected bool
		if assignedMap != nil {
			_, selected = assignedMap[s.ID]
		}
		names := packages[s.ID]
		if names == nil {
			names = []string{}
		}
		out = append(out, adminviews.ServiceAssignOption{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Packages:    names,
			Selected:    selected,
		})
	}
	return out
}

// toServicePackageRows بسته‌ها را به ردیف‌های جدول ادمین تبدیل می‌کند.
// ورودی: rows بسته‌ها، counts نگاشت شناسه بسته به تعداد خدمات.
// خروجی: اسلایس ServicePackageRow.
func toServicePackageRows(rows []models.ServicePackage, counts map[uint]int) []adminviews.ServicePackageRow {
	out := make([]adminviews.ServicePackageRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, adminviews.ServicePackageRow{
			ID:           row.ID,
			Name:         row.Name,
			Description:  row.Description,
			ServiceCount: counts[row.ID],
		})
	}
	return out
}

// toServicePackageOptions بسته‌های مخزن را به گزینه‌های انتخاب گروهی صفحه انتصاب پزشک تبدیل می‌کند.
// ورودی: rows بسته‌ها همراه با شناسه خدمات.
// خروجی: اسلایس ServicePackageOption.
func toServicePackageOptions(rows []repository.ServicePackageWithIDs) []adminviews.ServicePackageOption {
	out := make([]adminviews.ServicePackageOption, 0, len(rows))
	for _, row := range rows {
		ids := row.ServiceIDs
		if ids == nil {
			ids = []uint{}
		}
		out = append(out, adminviews.ServicePackageOption{
			ID:         row.Package.ID,
			Name:       row.Package.Name,
			ServiceIDs: ids,
			Count:      len(ids),
		})
	}
	return out
}
