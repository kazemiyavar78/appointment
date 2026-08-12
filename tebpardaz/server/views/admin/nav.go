package admin

import (
	"tebpardaz/server/views/layouts"
	"tebpardaz/shared/constants"
)

const (
	NavApprovals    = "approvals"
	NavAppointments = "appointments"
	NavSpecialties  = "specialties"
	NavNews         = "news"
	NavUsers        = "users"
)

// BuildAdminNav returns sidebar items for the given active page and user role.
func BuildAdminNav(active string, role string) layouts.AdminLayoutView {
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
	}
	if constants.UserRole(role) == constants.UserRoleSuperAdmin {
		items = append(items,
			layouts.AdminNavItem{
				Label:  "تخصص‌ها",
				Href:   "/admin/specialties",
				Active: active == NavSpecialties,
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
