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

const (
	serviceNameMaxLen = 150
	serviceDescMaxLen = 1000
	servicePageSize   = 15
	serviceQueryMax   = 100
)

// ServiceHandler عملیات مدیریت کاتالوگ خدمات و انتصاب آن‌ها به بیمه‌های مرکز، پزشکان و بخش‌ها را بر عهده دارد.
type ServiceHandler struct {
	Services   *repository.ServiceRepo
	Insurances *repository.InsuranceRepo
	Doctors    *repository.DoctorRepo
	Clinics    *repository.ClinicRepo
	Sections   *repository.SectionRepo
	Scope      *auth.ClinicScope
}

// NewServiceHandler یک نمونه جدید از ServiceHandler می‌سازد.
// ورودی: مخازن خدمات، بیمه‌ها، پزشکان، کلینیک‌ها، بخش‌ها و دامنه دسترسی کلینیک.
// خروجی: اشاره‌گر به ServiceHandler.
func NewServiceHandler(
	services *repository.ServiceRepo,
	insurances *repository.InsuranceRepo,
	doctors *repository.DoctorRepo,
	clinics *repository.ClinicRepo,
	scope *auth.ClinicScope,
	sections *repository.SectionRepo,
) *ServiceHandler {
	return &ServiceHandler{
		Services:   services,
		Insurances: insurances,
		Doctors:    doctors,
		Clinics:    clinics,
		Sections:   sections,
		Scope:      scope,
	}
}

// List صفحه کاتالوگ خدمات را همراه با فرم ایجاد یا ویرایش، جستجو و صفحه‌بندی رندر می‌کند.
// ورودی: c کانتکست Gin (حاوی پارامترهای اختیاری edit، q، page و msg).
// خروجی: صفحه HTML کاتالوگ خدمات.
func (h *ServiceHandler) List(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	view := h.catalogView(user)
	view.Message = serviceFlashMessage(c.Query("msg"))

	editID, _ := strconv.ParseUint(c.Query("edit"), 10, 64)
	if editID > 0 {
		row, err := h.Services.GetByID(uint(editID))
		if err != nil {
			if view.Message == "" {
				view.Message = "خدمت مورد نظر یافت نشد."
			}
		} else {
			view.EditID = row.ID
			view.EditName = row.Name
			view.EditDescription = row.Description
		}
	}

	if err := h.loadServiceCatalog(c, &view, nil); err != nil {
		c.String(http.StatusInternalServerError, "خطا در بارگذاری لیست خدمات")
		return
	}
	h.renderCatalog(c, view)
}

// Create یک رکورد خدمت جدید در کاتالوگ ایجاد می‌کند.
// ورودی: c کانتکست Gin (حاوی داده‌های فرم name و description).
// خروجی: ریدایرکت به صفحه لیست خدمات با پیام موفقیت یا خطا.
func (h *ServiceHandler) Create(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	name, description, msg := parseServiceFields(c)
	if msg != "" {
		h.renderCatalogWithMessage(c, user, msg)
		return
	}

	row := &models.Service{Name: name, Description: description}
	if err := h.Services.Create(row); err != nil {
		h.renderCatalogWithMessage(c, user, "خطا در ایجاد خدمت.")
		return
	}
	if err := h.Services.ReplaceServicePackages(row.ID, parsePostedUintIDs(c, "package_ids")); err != nil {
		c.Redirect(http.StatusFound, adminviews.ServiceCatalogHref("", 1, row.ID, "packages_failed"))
		return
	}
	c.Redirect(http.StatusFound, adminviews.ServiceCatalogHref("", 1, 0, "created"))
}

// Update تغییرات یک خدمت موجود در کاتالوگ را ذخیره می‌کند.
// ورودی: c کانتکست Gin (حاوی شناسه id در URL و فیلدهای name و description در فرم).
// خروجی: ریدایرکت به صفحه لیست خدمات.
func (h *ServiceHandler) Update(c *gin.Context) {
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
	row, err := h.Services.GetByID(id)
	if err != nil {
		h.renderCatalogWithMessage(c, user, "خدمت مورد نظر یافت نشد.")
		return
	}

	q, page := catalogQueryFromRequest(c)
	name, description, msg := parseServiceFields(c)
	if msg != "" {
		c.Redirect(http.StatusFound, adminviews.ServiceCatalogHref(q, page, id, "name_required"))
		return
	}

	row.Name = name
	row.Description = description
	if err := h.Services.Update(row); err != nil {
		c.Redirect(http.StatusFound, adminviews.ServiceCatalogHref(q, page, id, "update_failed"))
		return
	}
	if err := h.Services.ReplaceServicePackages(row.ID, parsePostedUintIDs(c, "package_ids")); err != nil {
		c.Redirect(http.StatusFound, adminviews.ServiceCatalogHref(q, page, id, "packages_failed"))
		return
	}
	c.Redirect(http.StatusFound, adminviews.ServiceCatalogHref(q, page, 0, "updated"))
}

// Delete یک خدمت را از کاتالوگ حذف کرده و انتصاب‌های آن را پاک می‌کند.
// ورودی: c کانتکست Gin (حاوی شناسه id در URL).
// خروجی: ریدایرکت به صفحه لیست خدمات با پیام حذف.
func (h *ServiceHandler) Delete(c *gin.Context) {
	if _, ok := auth.UserFromGin(c); !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	q, page := catalogQueryFromRequest(c)
	if err := h.Services.Delete(id); err != nil {
		c.Redirect(http.StatusFound, adminviews.ServiceCatalogHref(q, page, 0, "delete_failed"))
		return
	}
	c.Redirect(http.StatusFound, adminviews.ServiceCatalogHref(q, page, 0, "deleted"))
}

// ClinicInsuranceServicesForm فرم انتخاب مرکز، بیمه و انتصاب خدمات به آن بیمه را رندر می‌کند.
// ورودی: c کانتکست Gin (حاوی پارامترهای query نظیر clinic_id و insurance_id).
// خروجی: صفحه HTML انتصاب خدمات به بیمه‌های مرکز.
func (h *ServiceHandler) ClinicInsuranceServicesForm(c *gin.Context) {
	user, allowed, ok := h.requireUserClinics(c)
	if !ok {
		return
	}

	clinicID := h.resolveClinicID(c, user, allowed)
	insID, _ := strconv.ParseUint(c.Query("insurance_id"), 10, 64)

	view := adminviews.ClinicInsuranceServicesView{
		Nav:                 adminviews.BuildAdminNav(adminviews.NavClinicInsuranceServices, user.Role),
		Message:             serviceFlashMessage(c.Query("msg")),
		Clinics:             toInsuranceClinicOptions(allowed, clinicID),
		SelectedClinicID:    clinicID,
		SelectedInsuranceID: uint(insID),
	}

	if clinicID > 0 {
		// بیمه‌های منتسب به این مرکز
		clinicInsurances, err := h.Insurances.ListByClinicID(clinicID)
		if err == nil {
			view.Insurances = make([]adminviews.InsuranceOption, 0, len(clinicInsurances))
			for _, ins := range clinicInsurances {
				view.Insurances = append(view.Insurances, adminviews.InsuranceOption{
					ID:       ins.ID,
					Name:     ins.Name,
					Selected: ins.ID == uint(insID),
				})
			}
		}

		if insID > 0 {
			// چک‌باکس‌های تمام خدمات با علامت‌گذاری خدمات منتسب‌شده
			allServices, _ := h.Services.ListAll()
			assignedIDs, _ := h.Services.ListServiceIDsByClinicAndInsurance(clinicID, uint(insID))
			assignedMap := make(map[uint]struct{}, len(assignedIDs))
			for _, id := range assignedIDs {
				assignedMap[id] = struct{}{}
			}
			options, optErr := h.serviceAssignOptions(allServices, assignedMap)
			if optErr != nil {
				options = toServiceAssignOptions(allServices, assignedMap, nil)
			}
			view.Services = options
		}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.ClinicInsuranceServices(view).Render(c.Request.Context(), c.Writer)
}

// SaveClinicInsuranceServices انتصاب خدمات به بیمه‌های یک مرکز درمانی را ذخیره می‌کند.
// ورودی: c کانتکست Gin (حاوی clinic_id, insurance_ids یا insurance_id و آرایه service_ids در فرم POST).
// خروجی: ریدایرکت با پارامترهای فیلتر و پیام وضعیت.
func (h *ServiceHandler) SaveClinicInsuranceServices(c *gin.Context) {
	user, _, ok := h.requireUserClinics(c)
	if !ok {
		return
	}

	clinicID, _ := strconv.ParseUint(c.PostForm("clinic_id"), 10, 64)
	if clinicID == 0 || !h.Scope.CanAccess(user, uint(clinicID)) {
		c.Redirect(http.StatusFound, "/admin/services/clinic-insurances?msg=clinic_required")
		return
	}

	rawInsIDs := c.PostFormArray("insurance_ids")
	if len(rawInsIDs) == 0 {
		if singleIns := c.PostForm("insurance_id"); singleIns != "" {
			rawInsIDs = []string{singleIns}
		}
	}

	insuranceIDs := make([]uint, 0, len(rawInsIDs))
	for _, raw := range rawInsIDs {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			continue
		}
		insuranceIDs = append(insuranceIDs, uint(id))
	}

	if len(insuranceIDs) == 0 {
		c.Redirect(http.StatusFound, "/admin/services/clinic-insurances?clinic_id="+strconv.FormatUint(clinicID, 10)+"&msg=insurance_required")
		return
	}

	rawIDs := c.PostFormArray("service_ids")
	serviceIDs := make([]uint, 0, len(rawIDs))
	for _, raw := range rawIDs {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			continue
		}
		serviceIDs = append(serviceIDs, uint(id))
	}

	if err := h.Services.ReplaceClinicInsurancesServices(uint(clinicID), insuranceIDs, serviceIDs); err != nil {
		c.Redirect(http.StatusFound, "/admin/services/clinic-insurances?clinic_id="+strconv.FormatUint(clinicID, 10)+"&msg=assign_failed")
		return
	}

	c.Redirect(http.StatusFound, "/admin/services/clinic-insurances?clinic_id="+strconv.FormatUint(clinicID, 10)+"&msg=assigned")
}

// DoctorServicesForm فرم انتخاب مرکز، پزشک و انتصاب خدمات به آن پزشک را رندر می‌کند.
// ورودی: c کانتکست Gin (حاوی پارامترهای query نظیر clinic_id و doctor_id).
// خروجی: صفحه HTML انتصاب خدمات به پزشک.
func (h *ServiceHandler) DoctorServicesForm(c *gin.Context) {
	user, allowed, ok := h.requireUserClinics(c)
	if !ok {
		return
	}

	clinicID := h.resolveClinicID(c, user, allowed)
	docID, _ := strconv.ParseUint(c.Query("doctor_id"), 10, 64)

	view := adminviews.DoctorServicesView{
		Nav:              adminviews.BuildAdminNav(adminviews.NavDoctorServices, user.Role),
		Message:          serviceFlashMessage(c.Query("msg")),
		Clinics:          toInsuranceClinicOptions(allowed, clinicID),
		SelectedClinicID: clinicID,
		SelectedDoctorID: uint(docID),
	}

	if clinicID > 0 {
		// پزشکان تأیید شده این مرکز
		approvedDocs, err := h.Doctors.ListApprovedByClinic(clinicID)
		if err == nil {
			view.Doctors = make([]adminviews.DoctorOption, 0, len(approvedDocs))
			for _, doc := range approvedDocs {
				docName := strings.TrimSpace(doc.Name)
				if docName == "" {
					docName = strings.TrimSpace(doc.FirstName + " " + doc.LastName)
				}
				view.Doctors = append(view.Doctors, adminviews.DoctorOption{
					ID:       doc.ID,
					Name:     docName,
					Selected: doc.ID == uint(docID),
				})
			}
		}

		allServices, _ := h.Services.ListAll()
		var assignedMap map[uint]struct{}
		if docID > 0 {
			assignedIDs, _ := h.Services.ListServiceIDsByDoctorID(uint(docID))
			assignedMap = make(map[uint]struct{}, len(assignedIDs))
			for _, id := range assignedIDs {
				assignedMap[id] = struct{}{}
			}
		}
		options, optErr := h.serviceAssignOptions(allServices, assignedMap)
		if optErr != nil {
			options = toServiceAssignOptions(allServices, assignedMap, nil)
		}
		view.Services = options

		packages, _ := h.Services.ListPackagesWithServiceIDs()
		view.Packages = toServicePackageOptions(packages)
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.DoctorServices(view).Render(c.Request.Context(), c.Writer)
}

// SaveDoctorServices انتصاب خدمات به پزشکان انتخاب‌شده را ذخیره می‌کند.
// ورودی: c کانتکست Gin (حاوی clinic_id, doctor_ids یا doctor_id و آرایه service_ids در فرم POST).
// خروجی: ریدایرکت با پارامترهای فیلتر و پیام وضعیت.
func (h *ServiceHandler) SaveDoctorServices(c *gin.Context) {
	user, _, ok := h.requireUserClinics(c)
	if !ok {
		return
	}

	clinicID, _ := strconv.ParseUint(c.PostForm("clinic_id"), 10, 64)
	if clinicID == 0 || !h.Scope.CanAccess(user, uint(clinicID)) {
		c.Redirect(http.StatusFound, "/admin/services/doctor-services?msg=clinic_required")
		return
	}

	rawDocIDs := c.PostFormArray("doctor_ids")
	if len(rawDocIDs) == 0 {
		if singleDoc := c.PostForm("doctor_id"); singleDoc != "" {
			rawDocIDs = []string{singleDoc}
		}
	}

	doctorIDs := make([]uint, 0, len(rawDocIDs))
	for _, raw := range rawDocIDs {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			continue
		}
		doctorIDs = append(doctorIDs, uint(id))
	}

	if len(doctorIDs) == 0 {
		c.Redirect(http.StatusFound, "/admin/services/doctor-services?clinic_id="+strconv.FormatUint(clinicID, 10)+"&msg=doctor_required")
		return
	}

	rawIDs := c.PostFormArray("service_ids")
	serviceIDs := make([]uint, 0, len(rawIDs))
	for _, raw := range rawIDs {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			continue
		}
		serviceIDs = append(serviceIDs, uint(id))
	}

	if err := h.Services.ReplaceDoctorsServices(doctorIDs, serviceIDs); err != nil {
		c.Redirect(http.StatusFound, "/admin/services/doctor-services?clinic_id="+strconv.FormatUint(clinicID, 10)+"&msg=assign_failed")
		return
	}

	c.Redirect(http.StatusFound, "/admin/services/doctor-services?clinic_id="+strconv.FormatUint(clinicID, 10)+"&msg=assigned")
}

// catalogView مدل خالی صفحه کاتالوگ خدمات را برای کاربر جاری می‌سازد.
// ورودی: user کاربر لاگین شده.
// خروجی: ساختار ServicePageView با سایدبار متناسب با نقش کاربر.
func (h *ServiceHandler) catalogView(user *models.AppointmentUser) adminviews.ServicePageView {
	return adminviews.ServicePageView{
		Nav: adminviews.BuildAdminNav(adminviews.NavServices, user.Role),
	}
}

// requireUserClinics کاربر جاری و مراکز درمانی مجاز برای وی را استخراج و بررسی می‌کند.
// ورودی: c کانتکست Gin.
// خروجی: کاربر جاری، لیست کلینیک‌های مجاز، و بولین موفقیت.
func (h *ServiceHandler) requireUserClinics(c *gin.Context) (*models.AppointmentUser, []models.Clinic, bool) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return nil, nil, false
	}
	allowed, err := h.Scope.AllowedClinics(user)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در بارگذاری مراکز درمانی")
		return nil, nil, false
	}
	if len(allowed) == 0 {
		c.String(http.StatusForbidden, "هیچ مرکزی برای حساب شما تعریف نشده است.")
		return nil, nil, false
	}
	return user, allowed, true
}

// resolveClinicID شناسه کلینیک انتخابی را از کوئری یا فرم یا تک کلینیک مجاز تعیین می‌کند.
// ورودی: c کانتکست Gin، user کاربر، allowed لیست مراکز مجاز.
// خروجی: شناسه عددی کلینیک یا ۰.
func (h *ServiceHandler) resolveClinicID(c *gin.Context, user *models.AppointmentUser, allowed []models.Clinic) uint {
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

// renderCatalogWithMessage صفحه کاتالوگ خدمات را با یک پیام خطا یا هشدار دوباره رندر می‌کند.
// ورودی: c کانتکست Gin، user کاربر جاری، message پیام مورد نظر.
// خروجی: ندارد (رندر پاسخ HTML در کانتکست).
func (h *ServiceHandler) renderCatalogWithMessage(c *gin.Context, user *models.AppointmentUser, message string) {
	view := h.catalogView(user)
	view.Message = message
	view.EditName = strings.TrimSpace(c.PostForm("name"))
	view.EditDescription = strings.TrimSpace(c.PostForm("description"))
	selected := parsePostedUintIDs(c, "package_ids")
	if err := h.loadServiceCatalog(c, &view, &selected); err != nil {
		c.String(http.StatusInternalServerError, "خطا در بارگذاری لیست خدمات")
		return
	}
	h.renderCatalog(c, view)
}

// loadServiceCatalog لیست صفحه‌بندی‌شده خدمات، نام بسته‌ها و گزینه‌های فرم را پر می‌کند.
// ورودی: c درخواست جاری، view مدل صفحه، explicitPackages انتخاب صریح بسته‌ها (nil یعنی خواندن از خدمت در حال ویرایش).
// خروجی: خطای دیتابیس در صورت وقوع.
func (h *ServiceHandler) loadServiceCatalog(c *gin.Context, view *adminviews.ServicePageView, explicitPackages *[]uint) error {
	q, page := catalogQueryFromRequest(c)
	view.SearchQuery = q
	view.PageSize = servicePageSize

	rows, total, err := h.Services.SearchServices(q, page, servicePageSize)
	if err != nil {
		return err
	}
	totalPages := 1
	if total > 0 {
		totalPages = int((total + int64(servicePageSize) - 1) / int64(servicePageSize))
	}
	if page > totalPages {
		page = totalPages
		rows, total, err = h.Services.SearchServices(q, page, servicePageSize)
		if err != nil {
			return err
		}
	}
	view.Page = page
	view.TotalPages = totalPages
	view.Total = int(total)

	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	names, err := h.Services.ListPackageNamesByServiceIDs(ids)
	if err != nil {
		return err
	}
	view.Items = toServiceRows(rows, names)

	var selected map[uint]struct{}
	switch {
	case explicitPackages != nil:
		selected = uintSet(*explicitPackages)
	case view.EditID > 0:
		packageIDs, pkgErr := h.Services.ListPackageIDsByServiceID(view.EditID)
		if pkgErr != nil {
			return pkgErr
		}
		selected = uintSet(packageIDs)
	}
	packages, err := h.Services.ListPackages()
	if err != nil {
		return err
	}
	view.Packages = toPackageCheckOptions(packages, selected)
	return nil
}

// catalogQueryFromRequest عبارت جستجو و شماره صفحه را از کوئری یا فرم می‌خواند.
// ورودی: c کانتکست Gin. خروجی: عبارت جستجو و شماره صفحه حداقل ۱.
func catalogQueryFromRequest(c *gin.Context) (string, int) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		q = strings.TrimSpace(c.PostForm("q"))
	}
	if len([]rune(q)) > serviceQueryMax {
		q = string([]rune(q)[:serviceQueryMax])
	}
	raw := c.Query("page")
	if raw == "" {
		raw = c.PostForm("page")
	}
	page, _ := strconv.Atoi(raw)
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		page = 10000
	}
	return q, page
}

// uintSet شناسه‌های مثبت را به یک مجموعه تبدیل می‌کند.
// ورودی: ids اسلایس شناسه. خروجی: نگاشت شناسه به مجموعه خالی.
func uintSet(ids []uint) map[uint]struct{} {
	out := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id > 0 {
			out[id] = struct{}{}
		}
	}
	return out
}

// renderCatalog صفحه HTML کاتالوگ خدمات را خروجی می‌دهد.
// ورودی: c کانتکست Gin، view مدل داده صفحه.
// خروجی: ندارد (رندر کامپوننت templ).
func (h *ServiceHandler) renderCatalog(c *gin.Context, view adminviews.ServicePageView) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.Services(view).Render(c.Request.Context(), c.Writer)
}

// parseServiceFields نام و توضیحات خدمت را از فرم دریافت و اعتبارسنجی می‌کند.
// ورودی: c کانتکست Gin.
// خروجی: name نام خدمت، description توضیحات، message پیام خطا در صورت نامعتبر بودن فیلدها.
func parseServiceFields(c *gin.Context) (name, description, message string) {
	name = strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		return "", "", "نام خدمت الزامی است."
	}
	if len([]rune(name)) > serviceNameMaxLen {
		name = string([]rune(name)[:serviceNameMaxLen])
	}
	description = strings.TrimSpace(c.PostForm("description"))
	if len([]rune(description)) > serviceDescMaxLen {
		description = string([]rune(description)[:serviceDescMaxLen])
	}
	return name, description, ""
}

// toServiceRows رکوردهای مدل Service را به ردیف‌های نمایشی در جدول ادمین تبدیل می‌کند.
// ورودی: rows اسلایس مدل‌های Service، packages نگاشت شناسه خدمت به نام بسته‌ها.
// خروجی: اسلایس ServiceRow مناسب برای تمپلیت.
func toServiceRows(rows []models.Service, packages map[uint][]string) []adminviews.ServiceRow {
	out := make([]adminviews.ServiceRow, 0, len(rows))
	for _, row := range rows {
		names := packages[row.ID]
		if names == nil {
			names = []string{}
		}
		out = append(out, adminviews.ServiceRow{
			ID:          row.ID,
			Name:        row.Name,
			Description: row.Description,
			Packages:    names,
		})
	}
	return out
}

// toPackageCheckOptions بسته‌ها را به گزینه‌های چک‌باکس فرم خدمت تبدیل می‌کند.
// ورودی: rows بسته‌ها، selected مجموعه شناسه‌های انتخاب‌شده (می‌تواند nil باشد).
// خروجی: اسلایس ServiceAssignOption.
func toPackageCheckOptions(rows []models.ServicePackage, selected map[uint]struct{}) []adminviews.ServiceAssignOption {
	out := make([]adminviews.ServiceAssignOption, 0, len(rows))
	for _, row := range rows {
		_, isSelected := selected[row.ID]
		out = append(out, adminviews.ServiceAssignOption{
			ID:          row.ID,
			Name:        row.Name,
			Description: row.Description,
			Selected:    isSelected,
		})
	}
	return out
}

// serviceFlashMessage کدهای وضعیت ریدایرکت را به پیام‌های فارسی خوانا تبدیل می‌کند.
// ورودی: code کد رشته‌ای پیام.
// خروجی: متن فارسی پیام.
func serviceFlashMessage(code string) string {
	switch code {
	case "created":
		return "خدمت با موفقیت ایجاد شد."
	case "updated":
		return "خدمت با موفقیت بروزرسانی شد."
	case "deleted":
		return "خدمت و تمام انتصاب‌های آن با موفقیت حذف شد."
	case "name_required":
		return "نام خدمت الزامی است."
	case "update_failed":
		return "خطا در بروزرسانی خدمت."
	case "delete_failed":
		return "خطا در حذف خدمت."
	case "assigned":
		return "انتصاب خدمات با موفقیت ذخیره شد."
	case "assign_failed":
		return "خطا در ذخیره انتصاب خدمات."
	case "clinic_required":
		return "انتخاب مرکز الزامی است."
	case "insurance_required":
		return "انتخاب بیمه الزامی است."
	case "doctor_required":
		return "انتخاب پزشک الزامی است."
	case "packages_failed":
		return "خدمت ذخیره شد اما انتصاب بسته‌ها انجام نشد."
	case "section_assigned":
		return "بسته‌های خدمات این بخش با موفقیت ذخیره شد."
	case "section_required":
		return "انتخاب بخش الزامی است."
	case "section_assign_failed":
		return "خطا در ذخیره انتصاب بسته‌ها به بخش."
	case "pkg_created":
		return "بسته خدمات با موفقیت ایجاد شد."
	case "pkg_updated":
		return "بسته خدمات با موفقیت بروزرسانی شد."
	case "pkg_deleted":
		return "بسته خدمات با موفقیت حذف شد."
	case "pkg_name_required":
		return "نام بسته الزامی است."
	case "pkg_update_failed":
		return "خطا در بروزرسانی بسته خدمات."
	case "pkg_delete_failed":
		return "خطا در حذف بسته خدمات."
	case "pkg_create_failed":
		return "خطا در ایجاد بسته خدمات."
	default:
		return ""
	}
}
