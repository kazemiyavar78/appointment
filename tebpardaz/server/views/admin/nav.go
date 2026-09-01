package admin

import (
	"fmt"

	"tebpardaz/server/views/layouts"
	"tebpardaz/shared/constants"
)

const (
	NavApprovals               = "approvals"
	NavAppointments            = "appointments"
	NavSpecialties             = "specialties"
	NavInsurances              = "insurances"
	NavInsuranceAssign         = "insurance_assign"
	NavServices                = "services"
	NavClinicInsuranceServices = "clinic_insurance_services"
	NavDoctorServices          = "doctor_services"
	NavNews                    = "news"
	NavUsers                   = "users"
	NavReviews                 = "reviews"
	NavSections                = "sections"
)

// SectionNavSummary holds minimal metadata of a section for admin sidebar navigation.
type SectionNavSummary struct {
	ID    uint
	Title string
}

// BuildAdminNav آیتم‌های سایدبار را برای صفحه فعال و نقش کاربر برمی‌گرداند.
// ورودی: کلید صفحه فعال و نقش کاربر. خروجی: AdminLayoutView با لیست لینک‌ها.
func BuildAdminNav(active string, role string) layouts.AdminLayoutView {
	return BuildAdminNavWithSections(active, role, nil)
}

// BuildAdminNavWithSections آیتم‌های سایدبار را همراه با بخش‌های کلینیک و زیرمنوهای ۴ گانه برمی‌گرداند.
// ورودی: کلید صفحه فعال، نقش کاربر و لیست خلاصه بخش‌ها. خروجی: AdminLayoutView.
func BuildAdminNavWithSections(active string, role string, sections []SectionNavSummary) layouts.AdminLayoutView {
	items := []layouts.AdminNavItem{
		{
			Label:  "لیست پزشکان",
			Href:   "/admin/approvals",
			Active: active == NavApprovals,
		},
		{
			Label:  "لیست نوبت‌ها",
			Href:   "/admin/appointments",
			Active: active == NavAppointments,
		},
		{
			Label:  "اخبار",
			Href:   "/admin/news",
			Active: active == NavNews,
		},
		{
			Label:  "نظرات",
			Href:   "/admin/reviews",
			Active: active == NavReviews,
		},
	}

	if canManageSections(role) {
		items = append(items, layouts.AdminNavItem{
			Label:  "مدیریت بخش‌ها",
			Href:   "/admin/sections",
			Active: active == NavSections,
		})

		for _, s := range sections {
			sectionItem := layouts.AdminNavItem{
				Label: s.Title,
				Href:  fmt.Sprintf("/admin/sections?edit=%d", s.ID),
				Children: []layouts.AdminNavItem{
					{
						Label:  "بنر",
						Href:   fmt.Sprintf("/admin/sections/%d/banner", s.ID),
						Active: active == fmt.Sprintf("section_%d_banner", s.ID),
					},
					{
						Label:  "ساعات کاری",
						Href:   fmt.Sprintf("/admin/sections/%d/schedule", s.ID),
						Active: active == fmt.Sprintf("section_%d_schedule", s.ID),
					},
					{
						Label:  "پیام به بیماران",
						Href:   fmt.Sprintf("/admin/sections/%d/messages", s.ID),
						Active: active == fmt.Sprintf("section_%d_messages", s.ID),
					},
					{
						Label:  "معرفی تجهیزات",
						Href:   fmt.Sprintf("/admin/sections/%d/equipment", s.ID),
						Active: active == fmt.Sprintf("section_%d_equipment", s.ID),
					},
				},
			}
			items = append(items, sectionItem)
		}
	}

	if canAssignInsurance(role) {
		items = append(items, layouts.AdminNavItem{
			Label:  "بیمه‌های مرکز",
			Href:   "/admin/insurances/assign",
			Active: active == NavInsuranceAssign,
		})
	}
	if constants.UserRole(role) == constants.UserRoleSuperAdmin {
		items = append(items,
			layouts.AdminNavItem{
				Label:  "تخصص‌ها",
				Href:   "/admin/specialties",
				Active: active == NavSpecialties,
			},
			layouts.AdminNavItem{
				Label:  "بیمه‌ها",
				Href:   "/admin/insurances",
				Active: active == NavInsurances,
			},
			layouts.AdminNavItem{
				Label:  "خدمات",
				Href:   "/admin/services",
				Active: active == NavServices || active == NavClinicInsuranceServices || active == NavDoctorServices,
				Children: []layouts.AdminNavItem{
					{
						Label:  "مدیریت خدمات",
						Href:   "/admin/services",
						Active: active == NavServices,
					},
					{
						Label:  "انتصاب به بیمه‌های مرکز",
						Href:   "/admin/services/clinic-insurances",
						Active: active == NavClinicInsuranceServices,
					},
					{
						Label:  "انتصاب به پزشکان",
						Href:   "/admin/services/doctor-services",
						Active: active == NavDoctorServices,
					},
				},
			},
			layouts.AdminNavItem{
				Label:  "کاربران",
				Href:   "/admin/users",
				Active: active == NavUsers,
			},
		)
	}
	return layouts.AdminLayoutView{NavItems: items}
}

// canManageSections مشخص می‌کند نقش مجاز به مدیریت بخش‌های مرکز است یا نه.
// ورودی: رشته نقش کاربر. خروجی: بولین.
func canManageSections(role string) bool {
	switch constants.UserRole(role) {
	case constants.UserRoleSuperAdmin, constants.UserRoleAdmin, constants.UserRoleClinicAdmin, constants.UserRoleOrganAdmin:
		return true
	default:
		return false
	}
}

// canAssignInsurance مشخص می‌کند نقش مجاز به انتخاب بیمه نمایشی مراکز است یا نه.
// ورودی: رشته نقش کاربر. خروجی: بولین.
func canAssignInsurance(role string) bool {
	switch constants.UserRole(role) {
	case constants.UserRoleSuperAdmin, constants.UserRoleAdmin, constants.UserRoleClinicAdmin, constants.UserRoleOrganAdmin:
		return true
	default:
		return false
	}
}
