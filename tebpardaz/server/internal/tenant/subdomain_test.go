package tenant

import (
	"errors"
	"testing"

	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/models"
	"tebpardaz/shared/constants"
)

// TestPlatformSubdomainDoesNotResolveSlug ثابت می‌کند زیردامنهٔ طب‌پرداز مستأجر نمی‌سازد.
func TestPlatformSubdomainDoesNotResolveSlug(t *testing.T) {
	slug := "foo"
	orgSlug := "mehr"
	clinicDomain := "chamranclinic.ir"
	orgDomain := "mehrshafaclinics.ir"
	clinicSlug := slug
	clinic := &models.Clinic{
		Name:              "مرکز فو",
		Slug:              &clinicSlug,
		Domain:            &clinicDomain,
		IsActiveOnWebsite: true,
	}
	clinic.ID = 9
	org := &models.Organization{Name: "سازمان مهر", Domain: &orgDomain, Slug: &orgSlug}
	org.ID = 4

	tc := cache.NewTenantCache(nil)
	tc.SetClinicBySlug(slug, clinic)
	tc.SetClinicByDomain(clinicDomain, clinic)
	tc.SetOrgBySlug(slug, org)
	tc.SetOrgBySlug(orgSlug, org)
	tc.SetOrgByDomain(orgDomain, org)

	r := NewResolver("tebpardaz.ir", nil, nil)
	r.UseCache(tc)

	for _, host := range []string{"foo.tebpardaz.ir", "FOO.tebpardaz.ir:443", "random.tebpardaz.ir", "a.b.tebpardaz.ir", "mehr.tebpardaz.ir"} {
		_, err := r.Resolve(host, "/")
		if !errors.Is(err, ErrTenantNotFound) {
			t.Fatalf("Resolve(%q) err = %v, want ErrTenantNotFound", host, err)
		}
		_, err = r.Resolve(host, "/clinics/foo")
		if !errors.Is(err, ErrTenantNotFound) {
			t.Fatalf("Resolve(%q, path) err = %v, want ErrTenantNotFound", host, err)
		}
	}

	got, err := r.Resolve("tebpardaz.ir", "/foo")
	if err != nil || got == nil || got.Layout != constants.LayoutPrivate || got.Clinic == nil || got.Clinic.ID != 9 {
		t.Fatalf("path alias = %+v err=%v", got, err)
	}

	got, err = r.Resolve("www.tebpardaz.ir", "/")
	if err != nil || got == nil || got.Layout != constants.LayoutPlatform {
		t.Fatalf("www = %+v err=%v", got, err)
	}

	got, err = r.Resolve("tebpardaz.ir", "/doctors")
	if err != nil || got == nil || got.Layout != constants.LayoutPlatform {
		t.Fatalf("reserved /doctors = %+v err=%v", got, err)
	}

	got, err = r.Resolve("tebpardaz.ir", "/"+orgSlug)
	if err != nil || got == nil || got.Layout != constants.LayoutOrgan || got.Organization == nil || got.Organization.ID != 4 {
		t.Fatalf("path org slug = %+v err=%v", got, err)
	}

	got, err = r.Resolve(clinicDomain, "/")
	if err != nil || got == nil || got.Layout != constants.LayoutPrivate || got.Clinic == nil || got.Clinic.ID != 9 {
		t.Fatalf("own domain = %+v err=%v", got, err)
	}

	got, err = r.Resolve(orgDomain, "/")
	if err != nil || got == nil || got.Layout != constants.LayoutOrgan || got.Organization == nil || got.Organization.ID != 4 {
		t.Fatalf("org domain = %+v err=%v", got, err)
	}
}
