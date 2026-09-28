package branding

import (
	"testing"

	"tebpardaz/server/internal/models"
)

func TestClinicLogoURL(t *testing.T) {
	clinic := &models.Clinic{Code: 9, LogoURL: "/static/uploads/clinics/abc.png"}
	if got := ClinicLogoURL(clinic); got != "/static/uploads/clinics/abc.png" {
		t.Fatalf("expected uploaded logo, got %q", got)
	}

	clinic.LogoURL = ""
	if got := ClinicLogoURL(clinic); got != "/static/clinics/9-logo.jpg" {
		t.Fatalf("expected legacy fallback, got %q", got)
	}
}

func TestClinicFaviconURL(t *testing.T) {
	clinic := &models.Clinic{FaviconURL: "/static/uploads/clinics/favicons/x.ico"}
	if got := ClinicFaviconURL(clinic); got != "/static/uploads/clinics/favicons/x.ico" {
		t.Fatalf("expected uploaded favicon, got %q", got)
	}

	clinic.FaviconURL = ""
	clinic.LogoURL = "/static/uploads/clinics/logo.png"
	if got := ClinicFaviconURL(clinic); got != "/static/uploads/clinics/logo.png" {
		t.Fatalf("expected logo fallback, got %q", got)
	}
}
