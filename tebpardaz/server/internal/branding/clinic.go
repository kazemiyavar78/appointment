package branding

import (
	"fmt"

	"tebpardaz/server/internal/models"
)

const defaultPlatformFavicon = "/static/clinics/logo.jpg"

// ClinicLogoURL returns the public logo URL for a clinic with legacy static fallback.
// Inputs: clinic model pointer (may be nil).
// Output: logo URL or empty string.
func ClinicLogoURL(clinic *models.Clinic) string {
	if clinic == nil {
		return ""
	}
	if clinic.LogoURL != "" {
		return clinic.LogoURL
	}
	if clinic.Code > 0 {
		return fmt.Sprintf("/static/clinics/%d-logo.jpg", clinic.Code)
	}
	return ""
}

// ClinicFaviconURL returns the favicon URL for a clinic with logo and platform fallbacks.
// Inputs: clinic model pointer (may be nil).
// Output: favicon URL or platform default.
func ClinicFaviconURL(clinic *models.Clinic) string {
	if clinic == nil {
		return defaultPlatformFavicon
	}
	if clinic.FaviconURL != "" {
		return clinic.FaviconURL
	}
	if logo := ClinicLogoURL(clinic); logo != "" {
		return logo
	}
	return defaultPlatformFavicon
}
