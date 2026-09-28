package admin

import (
	"fmt"

	"tebpardaz/server/views/layouts"
	"tebpardaz/shared/constants"
)

const (
	NavDashboard               = "dashboard"
	NavApprovals               = "approvals"
	NavAppointments            = "appointments"
	NavClinicLogs              = "clinic_logs"
	NavSpecialties             = "specialties"
	NavInsurances              = "insurances"
	NavInsuranceAssign         = "insurance_assign"
	NavServices                = "services"
	NavServicePackages         = "service_packages"
	NavClinicInsuranceServices = "clinic_insurance_services"
	NavDoctorServices          = "doctor_services"
	NavSectionServicePackages  = "section_service_packages"
	NavNews                    = "news"
	NavUsers                   = "users"
	NavReviews                 = "reviews"
	NavSections                = "sections"
	NavClinicBranding          = "clinic_branding"
	NavVisitorIPs              = "visitor_ips"
	NavVisitorVisits           = "visitor_visits"
	NavRegisteredAppointments  = "registered_appointments"
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

// BuildAdminNavWithSections آیتم‌های سایدبار را همراه با بخش‌های کلینیک و زیرمنوهای بخش برمی‌گرداند.
// ورودی: کلید صفحه فعال، نقش کاربر و لیست خلاصه بخش‌ها. خروجی: AdminLayoutView.
func BuildAdminNavWithSections(active string, role string, sections []SectionNavSummary) layouts.AdminLayoutView {
	items := []layouts.AdminNavItem{
		{
			Label:  "داشبورد",
			Href:   "/admin",
			Active: active == NavDashboard,
		},
		{
			Label:  "لیست پزشکان",
			Href:   "/admin/approvals",
			Active: active == NavApprovals,
		},
		{
			Label:  "لیست نوبت‌ها",
			Href:   "/admin/appointments",
			Active: active == NavAppointments || active == NavRegisteredAppointments,
			Children: []layouts.AdminNavItem{
				{
					Label:  "نوبت‌های زنده پزشکان",
					Href:   "/admin/appointments",
					Active: active == NavAppointments,
				},
				{
					Label:  "نوبت‌های ثبت‌شده",
					Href:   "/admin/bookings",
					Active: active == NavRegisteredAppointments,
				},
			},
		},
		{
			Label:  "لاگ کلاینت‌ها",
			Href:   "/admin/logs",
			Active: active == NavClinicLogs,
		},
		{
			Label:  "بازدید کاربران",
			Href:   "/admin/visitors",
			Active: active == NavVisitorIPs || active == NavVisitorVisits,
			Children: []layouts.AdminNavItem{
				{
					Label:  "لیست آی‌پی‌ها",
					Href:   "/admin/visitors",
					Active: active == NavVisitorIPs,
				},
				{
					Label:  "بازدید صفحات",
					Href:   "/admin/visitors/visits",
					Active: active == NavVisitorVisits,
				},
			},
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
						Label:  "پزشکان",
						Href:   fmt.Sprintf("/admin/sections/%d/doctors", s.ID),
						Active: active == fmt.Sprintf("section_%d_doctors", s.ID),
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
	if canManageClinicBranding(role) {
		items = append(items, layouts.AdminNavItem{
			Label:  "لوگو و آیکون مرکز",
			Href:   "/admin/clinic/branding",
			Active: active == NavClinicBranding,
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
				Active: active == NavServices || active == NavServicePackages || active == NavClinicInsuranceServices || active == NavDoctorServices || active == NavSectionServicePackages,
				Children: []layouts.AdminNavItem{
					{
						Label:  "مدیریت خدمات",
						Href:   "/admin/services",
						Active: active == NavServices,
					},
					{
						Label:  "بسته‌های خدمات",
						Href:   "/admin/services/packages",
						Active: active == NavServicePackages,
					},
					{
						Label:  "انتصاب به بخش‌های مرکز",
						Href:   "/admin/services/section-packages",
						Active: active == NavSectionServicePackages,
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

// canManageClinicBranding مشخص می‌کند نقش مجاز به بارگذاری لوگو و favicon مرکز است یا نه.
// ورودی: رشته نقش کاربر. خروجی: بولین.
func canManageClinicBranding(role string) bool {
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
