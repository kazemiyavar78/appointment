package public

import (
	"strings"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// clinicVisibleForTenant می‌گوید این مرکز روی سطح عمومی همین درخواست مجاز است یا نه.
// ورودی: مستأجر resolveشده و مرکز. خروجی: true فقط با قرارداد سطح.
// دامنهٔ اختصاصی یا سازمان، مرکز فعال را از پلتفرم حذف نمی‌کند.
func clinicVisibleForTenant(tc *tenant.Context, clinic *models.Clinic) bool {
	if tc == nil || !tenant.ClinicOnWebsite(clinic) {
		return false
	}
	switch tc.Layout {
	case constants.LayoutPlatform:
		return tenant.ClinicVisibleOnPlatform(clinic)
	case constants.LayoutOrgan:
		if tc.OrganizationID == nil {
			return false
		}
		return tenant.ClinicVisibleOnOrganization(clinic, *tc.OrganizationID)
	case constants.LayoutPrivate:
		return tc.ClinicID != nil && clinic.ID == *tc.ClinicID
	default:
		return false
	}
}

// clinicSlugMatches اسلاگ ذخیره‌شده را با بخش مسیر مقایسه می‌کند.
// ورودی: مرکز و اسلاگ URL. خروجی: true وقتی هر دو پس از trim یکی باشند.
func clinicSlugMatches(clinic *models.Clinic, slug string) bool {
	if clinic == nil || clinic.Slug == nil {
		return false
	}
	return strings.TrimSpace(*clinic.Slug) == strings.TrimSpace(slug)
}

// organizationDomainSurface می‌گوید layout واقعاً از دامنهٔ سازمان resolve شده یا نه.
// ورودی: مستأجر. خروجی: true فقط برای LayoutOrgan وقتی هاست با Organization.Domain یکی است.
func organizationDomainSurface(tc *tenant.Context) bool {
	if tc == nil || tc.Layout != constants.LayoutOrgan {
		return false
	}
	return tenant.OrganizationDomainSurface(tc.Organization, tc.Host)
}

// organizationSurfaceOrigin مبدأ HTTPS سطح سازمان را برمی‌گرداند.
// ورودی: درخواست و مستأجر. خروجی: origin بدون اسلش، یا خالی اگر هاست درخواست با دامنهٔ سازمان یکی نباشد.
func organizationSurfaceOrigin(c *gin.Context, tc *tenant.Context) string {
	if !organizationDomainSurface(tc) {
		return ""
	}
	base := strings.TrimRight(publicBaseURL(c), "/")
	if base == "" || !tenant.OrganizationDomainSurface(tc.Organization, hostOfOrigin(base)) {
		return ""
	}
	return base
}

// hostOfOrigin هاست را از مبدأ scheme://host جدا می‌کند.
// ورودی: مبدأ. خروجی: هاست بدون scheme.
func hostOfOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	origin = strings.TrimPrefix(origin, "https://")
	origin = strings.TrimPrefix(origin, "http://")
	return strings.Trim(origin, "/")
}
