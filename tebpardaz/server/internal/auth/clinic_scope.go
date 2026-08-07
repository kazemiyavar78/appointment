package auth

import (
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/shared/constants"
)

// ClinicScope resolves which clinics an admin user may access.
type ClinicScope struct {
	Clinics *repository.ClinicRepo
}

// NewClinicScope constructs a ClinicScope.
func NewClinicScope(clinics *repository.ClinicRepo) *ClinicScope {
	return &ClinicScope{Clinics: clinics}
}

// AllowedClinics returns clinics visible to the user based on role assignment.
func (s *ClinicScope) AllowedClinics(user *models.AppointmentUser) ([]models.Clinic, error) {
	if user == nil || s.Clinics == nil {
		return nil, nil
	}
	switch constants.UserRole(user.Role) {
	case constants.UserRoleSuperAdmin:
		return s.Clinics.ListAll()
	case constants.UserRoleAdmin, constants.UserRoleClinicAdmin:
		if user.ClinicID == nil {
			return nil, nil
		}
		c, err := s.Clinics.GetByID(*user.ClinicID)
		if err != nil {
			return nil, err
		}
		return []models.Clinic{*c}, nil
	case constants.UserRoleOrganAdmin:
		if user.OrganizationID == nil {
			return nil, nil
		}
		return s.Clinics.ListByOrganizationID(*user.OrganizationID)
	case constants.UserRoleEditor:
		// Editor may be clinic-scoped or organization-scoped.
		if user.OrganizationID != nil {
			return s.Clinics.ListByOrganizationID(*user.OrganizationID)
		}
		if user.ClinicID != nil {
			c, err := s.Clinics.GetByID(*user.ClinicID)
			if err != nil {
				return nil, err
			}
			return []models.Clinic{*c}, nil
		}
		return nil, nil
	default:
		return nil, nil
	}
}

// CanAccess reports whether the user may view the given clinic.
func (s *ClinicScope) CanAccess(user *models.AppointmentUser, clinicID uint) bool {
	clinics, err := s.AllowedClinics(user)
	if err != nil {
		return false
	}
	for _, c := range clinics {
		if c.ID == clinicID {
			return true
		}
	}
	return false
}
