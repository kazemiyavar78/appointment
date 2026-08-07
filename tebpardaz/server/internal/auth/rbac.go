package auth

import "tebpardaz/shared/constants"

// RBAC checks whether a user role may perform an action.
type RBAC struct{}

// NewRBAC constructs an RBAC checker.
func NewRBAC() *RBAC {
	return &RBAC{}
}

// Can reports whether role is allowed to perform action.
func (r *RBAC) Can(role constants.UserRole, action string) bool {
	switch role {
	case constants.UserRoleSuperAdmin:
		return true
	case constants.UserRoleAdmin, constants.UserRoleClinicAdmin, constants.UserRoleOrganAdmin:
		switch action {
		case "doctor.approve", "doctor.list", "news.edit":
			return true
		}
	case constants.UserRoleEditor:
		switch action {
		case "doctor.approve", "news.edit", "doctor.list":
			return true
		}
	}
	return false
}
