package tenant

import (
	"errors"
	"testing"
	"time"

	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/models"
	"tebpardaz/shared/constants"

	"gorm.io/gorm"
)

// TestResolveCustomDomainFromCache حل دامنه را فقط از کش، بدون دیتابیس، بررسی می‌کند.
func TestResolveCustomDomainFromCache(t *testing.T) {
	store := cache.New(time.Minute, time.Minute)
	tenantCache := cache.NewTenantCache(store)
	slug := "alpha"
	domain := "clinic.example"
	tenantCache.SetClinicByDomain(domain, &models.Clinic{
		Model:          gorm.Model{ID: 9},
		Name:           "Alpha",
		Slug:           &slug,
		OrganizationID: 4,
		TenantType:     string(constants.TenantPrivateOwnDomain),
	})

	r := NewResolver("tebpardaz.ir", nil, nil)
	r.UseCache(tenantCache)
	got, err := r.Resolve(domain, "/")
	if err != nil {
		t.Fatal(err)
	}
	if got.Layout != constants.LayoutPrivate || got.ClinicID == nil || *got.ClinicID != 9 {
		t.Fatalf("context = %+v", got)
	}
}

// TestResolveDomainMissIsCached ذخیره نشدن دامنه ناشناس در کش را بررسی می‌کند.
func TestResolveDomainMissIsCached(t *testing.T) {
	tenantCache := cache.NewTenantCache(cache.New(time.Minute, time.Minute))
	r := NewResolver("tebpardaz.ir", nil, nil)
	r.UseCache(tenantCache)

	_, err := r.Resolve("missing.example", "/")
	if !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("err = %v", err)
	}
	if _, found, ok := tenantCache.GetClinicByDomain("missing.example"); !ok || found {
		t.Fatal("clinic miss was not cached")
	}
	if _, found, ok := tenantCache.GetOrgByDomain("missing.example"); !ok || found {
		t.Fatal("organization miss was not cached")
	}

	_, err = r.Resolve("missing.example", "/")
	if !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("second err = %v", err)
	}
}
