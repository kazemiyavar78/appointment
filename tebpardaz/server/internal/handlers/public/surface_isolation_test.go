package public

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestClinicVisibleForTenantKeepsDomainClinicsOnPlatform(t *testing.T) {
	domain := "chamranclinic.ir"
	orgDomainClinic := &models.Clinic{OrganizationID: 2, IsActiveOnWebsite: true, TenantType: string(constants.TenantOrganSubsidiary)}
	orgDomainClinic.ID = 4
	own := &models.Clinic{OrganizationID: 3, IsActiveOnWebsite: true, Domain: &domain, TenantType: string(constants.TenantPrivateOwnDomain)}
	own.ID = 9
	bothDomain := "clinic.example"
	both := &models.Clinic{OrganizationID: 2, IsActiveOnWebsite: true, Domain: &bothDomain}
	both.ID = 20
	platform := &tenant.Context{Layout: constants.LayoutPlatform, Host: "tebpardaz.ir"}
	if !clinicVisibleForTenant(platform, orgDomainClinic) || !clinicVisibleForTenant(platform, own) || !clinicVisibleForTenant(platform, both) {
		t.Fatal("platform dropped a clinic that has a domain")
	}
	org := &tenant.Context{Layout: constants.LayoutOrgan, Host: "mehrshafaclinics.ir", OrganizationID: uintPtr(2)}
	if !clinicVisibleForTenant(org, orgDomainClinic) || !clinicVisibleForTenant(org, both) || clinicVisibleForTenant(org, own) {
		t.Fatal("organization scope")
	}
	ownHost := &tenant.Context{Layout: constants.LayoutPrivate, Host: "chamranclinic.ir", ClinicID: uintPtr(9), Clinic: own}
	if !clinicVisibleForTenant(ownHost, own) || clinicVisibleForTenant(ownHost, orgDomainClinic) {
		t.Fatal("own domain isolation")
	}
	legacy := &tenant.Context{Layout: constants.LayoutPrivate, Host: "tebpardaz.ir", ClinicID: uintPtr(9), Clinic: own}
	if !clinicVisibleForTenant(legacy, own) {
		t.Fatal("legacy slug host lost its resolved clinic")
	}
	own.IsActiveOnWebsite = false
	if clinicVisibleForTenant(platform, own) || clinicVisibleForTenant(ownHost, own) {
		t.Fatal("inactive clinic stayed visible")
	}
}

func uintPtr(v uint) *uint { return &v }

func TestSectionRoutesStayInsideSurface(t *testing.T) {
	slugA := "abdolmotaleb"
	slugB := "چمران-مشهد"
	domain := "chamranclinic.ir"
	rows := []models.Clinic{
		{Model: gorm.Model{ID: 4}, Name: "عبدالمطلب", Slug: &slugA, OrganizationID: 2, IsActiveOnWebsite: true},
		{Model: gorm.Model{ID: 9}, Name: "چمران", Slug: &slugB, OrganizationID: 3, IsActiveOnWebsite: true, Domain: &domain},
	}
	h := &SectionPublicHandler{Clinics: &memClinics{rows: rows}}
	org := gin.New()
	org.Use(func(c *gin.Context) {
		id := uint(2)
		c.Set(tenant.ContextKey, &tenant.Context{Layout: constants.LayoutOrgan, Host: "mehrshafaclinics.ir", OrganizationID: &id})
		c.Next()
	})
	GETAndHEAD(org.Group("/"), "/clinics/:clinic_slug/sections", h.ListSections)
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/clinics/"+url.PathEscape(slugB)+"/sections", nil)
	req.Host = "mehrshafaclinics.ir"
	org.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound || !strings.Contains(res.Body.String(), "noindex, nofollow") {
		t.Fatalf("org cross clinic status=%d", res.Code)
	}

	own := gin.New()
	own.Use(func(c *gin.Context) {
		id := uint(9)
		c.Set(tenant.ContextKey, &tenant.Context{
			Layout: constants.LayoutPrivate, Host: "chamranclinic.ir", ClinicID: &id, Clinic: &rows[1],
		})
		c.Next()
	})
	GETAndHEAD(own.Group("/"), "/clinics/:clinic_slug/sections", h.ListSections)
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/clinics/"+slugA+"/sections", nil)
	req.Host = "chamranclinic.ir"
	own.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound || !strings.Contains(res.Body.String(), "noindex, nofollow") {
		t.Fatalf("own-domain cross clinic status=%d", res.Code)
	}
}

func TestBookingStaysInsideSurface(t *testing.T) {
	slugA := "abdolmotaleb"
	slugB := "chamran"
	domain := "chamranclinic.ir"
	rows := &memClinics{rows: []models.Clinic{
		{Model: gorm.Model{ID: 4}, Name: "عبدالمطلب", Slug: &slugA, OrganizationID: 2, IsActiveOnWebsite: true},
		{Model: gorm.Model{ID: 9}, Name: "چمران", Slug: &slugB, OrganizationID: 3, IsActiveOnWebsite: true, Domain: &domain},
	}}
	h := &BookingHandler{Clinics: rows}
	orgID := uint(2)
	org := &tenant.Context{Layout: constants.LayoutOrgan, OrganizationID: &orgID, Host: "mehrshafaclinics.ir"}
	if clinic, err := h.resolveOrganClinic(org, slugB); err == nil || clinic != nil {
		t.Fatal("organization booked another organization's clinic")
	}
	if clinic, err := h.resolveOrganClinic(org, slugA); err != nil || clinic == nil || clinic.ID != 4 {
		t.Fatalf("same organization clinic = %v %v", clinic, err)
	}
	platform := &tenant.Context{Layout: constants.LayoutPlatform, Host: "tebpardaz.ir"}
	id, doctor, err := h.resolveBookingTarget(platform, slugB, "reza")
	if err != nil || id != 9 || doctor != "reza" {
		t.Fatalf("platform booking = %d %s %v", id, doctor, err)
	}
	hostID := uint(9)
	private := &tenant.Context{Layout: constants.LayoutPrivate, Host: "chamranclinic.ir", ClinicID: &hostID, Clinic: &rows.rows[1]}
	if _, _, err := h.resolveBookingTarget(private, slugA, "reza"); err == nil {
		t.Fatal("own domain accepted another clinic booking path")
	}
	gotID, gotDoctor, err := h.resolveBookingTarget(private, "reza", "")
	if err != nil || gotID != 9 || gotDoctor != "reza" {
		t.Fatalf("own domain doctor = %d %s %v", gotID, gotDoctor, err)
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(tenant.ContextKey, org)
		c.Next()
	})
	GETAndHEAD(r.Group("/"), "/booking/*path", h.Get)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/booking/"+slugB+"/reza", nil)
	req.Host = "mehrshafaclinics.ir"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("http booking status=%d", w.Code)
	}
}
