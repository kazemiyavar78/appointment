package public

import (
	"strings"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"

	"github.com/gin-gonic/gin"
)

// organizationClinicJSONLD موجودیت MedicalClinic را روی دامنهٔ سازمان می‌سازد.
// ورودی: درخواست، مستأجر و مرکز همان سازمان. خروجی: JSON-LD یا خالی.
// شناسهٔ مرکز با شناسهٔ سازمان یکی نیست و parentOrganization فقط ارجاع @id است.
func organizationClinicJSONLD(c *gin.Context, tc *tenant.Context, clinic *models.Clinic) string {
	origin := organizationSurfaceOrigin(c, tc)
	if origin == "" || clinic == nil || tc == nil || tc.OrganizationID == nil {
		return ""
	}
	if !tenant.ClinicVisibleOnOrganization(clinic, *tc.OrganizationID) {
		return ""
	}
	id, pageURL := seo.OrganizationClinicRef(origin, viewSlug(clinic))
	if id == "" || strings.TrimSpace(clinic.Name) == "" {
		return ""
	}
	dto := seo.MedicalClinicDTO{
		ID:          id,
		Name:        strings.TrimSpace(clinic.Name),
		URL:         pageURL,
		Description: seo.PlainText(clinic.Description),
		Telephone:   strings.TrimSpace(clinic.Phone),
		LogoURL:     seo.AbsoluteSchemaURL(origin, clinic.LogoURL),
		Address:     clinicPostalAddress(clinic),
		ParentID:    seo.OriginID(origin, "organization"),
	}
	return seo.BuildGraph(seo.BuildMedicalClinicSchema(dto))
}
