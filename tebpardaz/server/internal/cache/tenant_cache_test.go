package cache

import (
	"testing"
	"time"

	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// TestTenantCacheClinicDomainRoundTrip ذخیره و خواندن کلینیک از کش و مستقل بودن کپی را بررسی می‌کند.
func TestTenantCacheClinicDomainRoundTrip(t *testing.T) {
	c := NewTenantCache(New(time.Minute, time.Minute))
	slug := "alpha"
	domain := "Alpha.Example"
	c.SetClinicByDomain(domain, &models.Clinic{
		Model: gorm.Model{ID: 7},
		Name:  "Alpha",
		Slug:  &slug,
	})

	got, found, ok := c.GetClinicByDomain("alpha.example")
	if !ok || !found || got == nil || got.ID != 7 || got.Slug == nil || *got.Slug != slug {
		t.Fatalf("cache hit = %+v found=%v ok=%v", got, found, ok)
	}
	*got.Slug = "changed"
	again, _, _ := c.GetClinicByDomain(domain)
	if again == nil || again.Slug == nil || *again.Slug != slug {
		t.Fatal("cached clinic was aliased to the caller")
	}
}

// TestTenantCacheStoresMiss ذخیره نبودن سازمان را به‌عنوان miss بررسی می‌کند.
func TestTenantCacheStoresMiss(t *testing.T) {
	c := NewTenantCache(New(time.Minute, time.Minute))
	c.SetOrgByDomain("missing.example", nil)
	org, found, ok := c.GetOrgByDomain("missing.example")
	if !ok || found || org != nil {
		t.Fatalf("miss = org:%v found:%v ok:%v", org, found, ok)
	}
}
