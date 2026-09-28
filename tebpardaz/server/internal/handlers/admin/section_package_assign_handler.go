package admin

import (
	"net/http"
	"net/url"
	"strconv"

	"tebpardaz/server/internal/models"
	adminviews "tebpardaz/server/views/admin"

	"github.com/gin-gonic/gin"
)

// SectionPackagesForm فرم انتخاب مرکز، بخش و انتصاب بسته‌های خدمات را رندر می‌کند.
// ورودی: c کانتکست Gin (حاوی clinic_id و section_id اختیاری).
// خروجی: صفحه HTML انتصاب بسته به بخش.
func (h *ServiceHandler) SectionPackagesForm(c *gin.Context) {
	user, allowed, ok := h.requireUserClinics(c)
	if !ok {
		return
	}
	if h.Sections == nil {
		c.String(http.StatusInternalServerError, "خطا در بارگذاری بخش‌ها")
		return
	}

	clinicID := h.resolveClinicID(c, user, allowed)
	sectionID, _ := strconv.ParseUint(c.Query("section_id"), 10, 64)
	view := adminviews.SectionPackageAssignView{
		Nav:              adminviews.BuildAdminNav(adminviews.NavSectionServicePackages, user.Role),
		Message:          serviceFlashMessage(c.Query("msg")),
		Clinics:          toInsuranceClinicOptions(allowed, clinicID),
		SelectedClinicID: clinicID,
	}

	if clinicID > 0 {
		sections, err := h.Sections.ListSectionsByClinic(clinicID)
		if err != nil {
			c.String(http.StatusInternalServerError, "خطا در بارگذاری بخش‌های مرکز")
			return
		}
		view.Sections = toClinicSectionOptions(sections, uint(sectionID))
		if sec, found := h.sectionInClinic(sections, uint(sectionID)); found {
			view.SelectedSectionID = sec.ID
		}
	}

	if view.SelectedSectionID > 0 {
		packages, err := h.Services.ListPackagesWithServiceIDs()
		if err != nil {
			c.String(http.StatusInternalServerError, "خطا در بارگذاری بسته‌های خدمات")
			return
		}
		assignedIDs, err := h.Services.ListPackageIDsBySectionID(view.SelectedSectionID)
		if err != nil {
			c.String(http.StatusInternalServerError, "خطا در بارگذاری بسته‌های بخش")
			return
		}
		assigned := uintSet(assignedIDs)
		view.Packages = toServicePackageOptions(packages)
		for i := range view.Packages {
			_, view.Packages[i].Selected = assigned[view.Packages[i].ID]
		}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.SectionServicePackages(view).Render(c.Request.Context(), c.Writer)
}

// SaveSectionPackages بسته‌های انتخاب‌شده را به بخش یک مرکز جایگزین می‌کند.
// ورودی: c کانتکست Gin (حاوی clinic_id، section_id و آرایه package_ids).
// خروجی: ریدایرکت به همان صفحه با پیام وضعیت.
func (h *ServiceHandler) SaveSectionPackages(c *gin.Context) {
	user, _, ok := h.requireUserClinics(c)
	if !ok {
		return
	}
	if h.Sections == nil {
		c.String(http.StatusInternalServerError, "خطا در بارگذاری بخش‌ها")
		return
	}

	clinicID64, _ := strconv.ParseUint(c.PostForm("clinic_id"), 10, 64)
	sectionID64, _ := strconv.ParseUint(c.PostForm("section_id"), 10, 64)
	clinicID := uint(clinicID64)
	sectionID := uint(sectionID64)
	if clinicID == 0 || !h.Scope.CanAccess(user, clinicID) {
		c.Redirect(http.StatusFound, sectionPackageAssignPath(0, 0, "clinic_required"))
		return
	}

	sections, err := h.Sections.ListSectionsByClinic(clinicID)
	if err != nil {
		c.Redirect(http.StatusFound, sectionPackageAssignPath(clinicID, 0, "section_assign_failed"))
		return
	}
	if _, found := h.sectionInClinic(sections, sectionID); !found {
		c.Redirect(http.StatusFound, sectionPackageAssignPath(clinicID, 0, "section_required"))
		return
	}

	if err := h.Services.ReplaceSectionPackages(sectionID, parsePostedUintIDs(c, "package_ids")); err != nil {
		c.Redirect(http.StatusFound, sectionPackageAssignPath(clinicID, sectionID, "section_assign_failed"))
		return
	}
	c.Redirect(http.StatusFound, sectionPackageAssignPath(clinicID, sectionID, "section_assigned"))
}

// sectionInClinic بخش را فقط وقتی برمی‌گرداند که متعلق به لیست همان مرکز باشد.
// ورودی: sections بخش‌های مرکز، sectionID شناسه درخواستی.
// خروجی: مدل بخش و بولین پیدا شدن.
func (h *ServiceHandler) sectionInClinic(sections []models.AppointmentClinicSection, sectionID uint) (models.AppointmentClinicSection, bool) {
	if sectionID == 0 {
		return models.AppointmentClinicSection{}, false
	}
	for _, sec := range sections {
		if sec.ID == sectionID {
			return sec, true
		}
	}
	return models.AppointmentClinicSection{}, false
}

// toClinicSectionOptions بخش‌های مرکز را به گزینه‌های کشویی تبدیل می‌کند.
// ورودی: sections بخش‌ها، selected شناسه بخش انتخاب‌شده.
// خروجی: اسلایس ClinicSectionOption. بخش غیرفعال با پسوند مشخص می‌شود.
func toClinicSectionOptions(sections []models.AppointmentClinicSection, selected uint) []adminviews.ClinicSectionOption {
	out := make([]adminviews.ClinicSectionOption, 0, len(sections))
	for _, sec := range sections {
		title := sec.Title
		if !sec.IsActive {
			title += " (غیرفعال)"
		}
		out = append(out, adminviews.ClinicSectionOption{
			ID:       sec.ID,
			Title:    title,
			Selected: sec.ID == selected,
		})
	}
	return out
}

// sectionPackageAssignPath آدرس صفحه انتصاب بسته به بخش را با فیلتر و پیام می‌سازد.
// ورودی: شناسه مرکز، شناسه بخش و کد پیام. خروجی: مسیر نسبی.
func sectionPackageAssignPath(clinicID, sectionID uint, msg string) string {
	values := url.Values{}
	if clinicID > 0 {
		values.Set("clinic_id", strconv.FormatUint(uint64(clinicID), 10))
	}
	if sectionID > 0 {
		values.Set("section_id", strconv.FormatUint(uint64(sectionID), 10))
	}
	if msg != "" {
		values.Set("msg", msg)
	}
	encoded := values.Encode()
	if encoded == "" {
		return "/admin/services/section-packages"
	}
	return "/admin/services/section-packages?" + encoded
}
