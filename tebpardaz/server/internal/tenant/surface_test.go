package tenant

import (
	"testing"

	"tebpardaz/server/internal/models"
	"tebpardaz/shared/constants"
)

func strPtr(s string) *string { return &s }

func surfaceClinic(id, org uint, domain string, active bool, tenantType string) *models.Clinic {
	c := &models.Clinic{
		OrganizationID:    org,
		IsActiveOnWebsite: active,
		TenantType:        tenantType,
	}
	c.ID = id
	if domain != "" {
		c.Domain = strPtr(domain)
	}
	return c
}

func TestClinicSurfacesAreAdditive(t *testing.T) {
	platformOnly := surfaceClinic(1, 3, "", true, string(constants.TenantPrivateTebpardaz))
	orgOnly := surfaceClinic(4, 2, "", true, string(constants.TenantOrganSubsidiary))
	clinicDomain := surfaceClinic(9, 3, "chamranclinic.ir", true, string(constants.TenantPrivateOwnDomain))
	both := surfaceClinic(20, 2, "clinic.example", true, string(constants.TenantOrganSubsidiary))
	inactive := surfaceClinic(9, 3, "chamranclinic.ir", false, string(constants.TenantPrivateOwnDomain))

	if !ClinicVisibleOnPlatform(platformOnly) || !ClinicVisibleOnPlatform(orgOnly) || !ClinicVisibleOnPlatform(clinicDomain) || !ClinicVisibleOnPlatform(both) {
		t.Fatal("platform must keep every active clinic")
	}
	if ClinicVisibleOnPlatform(inactive) || ClinicVisibleOnOrganization(inactive, 3) || ClinicVisibleOnOwnDomain(inactive, "chamranclinic.ir") {
		t.Fatal("inactive clinic leaked")
	}
	orgDomain := "mehrshafaclinics.ir"
	org := &models.Organization{Name: "موسسه", Domain: strPtr(orgDomain)}
	if !OrganizationDomainSurface(org, "mehrshafaclinics.ir") || !OrganizationDomainSurface(org, "MEHRSHAFACLINICS.IR:443") {
		t.Fatal("organization domain surface")
	}
	if OrganizationDomainSurface(org, "tebpardaz.ir") || OrganizationDomainSurface(&models.Organization{Name: "موسسه"}, "mehrshafaclinics.ir") {
		t.Fatal("organization domain must match the stored domain")
	}
	if !ClinicVisibleOnOrganization(orgOnly, 2) || ClinicVisibleOnOrganization(orgOnly, 3) || ClinicVisibleOnOrganization(clinicDomain, 2) {
		t.Fatal("organization scope")
	}
	if !ClinicVisibleOnOrganization(both, 2) || !ClinicVisibleOnOwnDomain(both, "clinic.example") || !ClinicVisibleOnPlatform(both) {
		t.Fatal("both domains should remain additive")
	}
	if !ClinicVisibleOnOwnDomain(clinicDomain, "chamranclinic.ir") || ClinicVisibleOnOwnDomain(clinicDomain, "merajclinic.ir") {
		t.Fatal("own domain")
	}
	if ClinicVisibleOnOwnDomain(orgOnly, "mehrshafaclinics.ir") {
		t.Fatal("organization domain is not the clinic domain")
	}
	// tenant_type نباید visibility را عوض کند.
	flipped := surfaceClinic(4, 2, "", true, string(constants.TenantPrivateOwnDomain))
	if ClinicVisibleOnOrganization(flipped, 2) != ClinicVisibleOnOrganization(orgOnly, 2) {
		t.Fatal("tenant_type changed organization visibility")
	}
}
