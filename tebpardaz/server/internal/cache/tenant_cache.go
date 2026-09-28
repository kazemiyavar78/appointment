package cache

import (
	"strings"
	"time"

	"tebpardaz/server/internal/models"
)

const (
	// TenantTTL مدت ماندن کلینیک یا سازمان حل‌شده در حافظه است.
	TenantTTL = 10 * time.Minute
)

// clinicRecord نتیجه کش‌شده جستجوی کلینیک است. Found=false یعنی رکورد پیدا نشده.
type clinicRecord struct {
	Found  bool
	Clinic models.Clinic
}

// orgRecord نتیجه کش‌شده جستجوی سازمان است. Found=false یعنی رکورد پیدا نشده.
type orgRecord struct {
	Found bool
	Org   models.Organization
}

// TenantCache نتیجه جستجوی کلینیک و سازمان را برای resolver نگه می‌دارد.
type TenantCache struct {
	store *Store
}

// NewTenantCache کش مستأجر را روی Store مشترک می‌سازد.
// ورودی: store؛ اگر nil باشد Store با TenantTTL ساخته می‌شود.
// خروجی: TenantCache آماده.
func NewTenantCache(store *Store) *TenantCache {
	if store == nil {
		store = New(TenantTTL, CleanupInterval)
	}
	return &TenantCache{store: store}
}

// GetClinicByDomain کلینیک ذخیره‌شده برای این هاست را می‌خواند.
// ورودی: domain بدون پورت. خروجی: کپی کلینیک، found (رکورد وجود دارد)، ok (در کش بوده). found=false یعنی نبودن رکورد کش شده است.
func (c *TenantCache) GetClinicByDomain(domain string) (*models.Clinic, bool, bool) {
	rec, ok := getTyped[clinicRecord](c, clinicDomainKey(domain))
	if !ok {
		return nil, false, false
	}
	if !rec.Found {
		return nil, false, true
	}
	return cloneClinic(rec.Clinic), true, true
}

// SetClinicByDomain کلینیک را ذخیره می‌کند. clinic برابر nil یعنی پیدا نشد.
// ورودی: domain و clinic. خروجی: ندارد.
func (c *TenantCache) SetClinicByDomain(domain string, clinic *models.Clinic) {
	setClinic(c, clinicDomainKey(domain), clinic)
}

// GetClinicBySlug کلینیک ذخیره‌شده برای این اسلاگ را می‌خواند.
// ورودی: slug. خروجی: کپی کلینیک، found، ok.
func (c *TenantCache) GetClinicBySlug(slug string) (*models.Clinic, bool, bool) {
	rec, ok := getTyped[clinicRecord](c, clinicSlugKey(slug))
	if !ok {
		return nil, false, false
	}
	if !rec.Found {
		return nil, false, true
	}
	return cloneClinic(rec.Clinic), true, true
}

// SetClinicBySlug کلینیک را ذخیره می‌کند. clinic برابر nil یعنی پیدا نشد.
// ورودی: slug و clinic. خروجی: ندارد.
func (c *TenantCache) SetClinicBySlug(slug string, clinic *models.Clinic) {
	setClinic(c, clinicSlugKey(slug), clinic)
}

// GetOrgByDomain سازمان ذخیره‌شده برای این هاست را می‌خواند.
// ورودی: domain. خروجی: کپی سازمان، found، ok.
func (c *TenantCache) GetOrgByDomain(domain string) (*models.Organization, bool, bool) {
	rec, ok := getTyped[orgRecord](c, orgDomainKey(domain))
	if !ok {
		return nil, false, false
	}
	if !rec.Found {
		return nil, false, true
	}
	return cloneOrg(rec.Org), true, true
}

// SetOrgByDomain سازمان را ذخیره می‌کند. org برابر nil یعنی پیدا نشد.
// ورودی: domain و org. خروجی: ندارد.
func (c *TenantCache) SetOrgByDomain(domain string, org *models.Organization) {
	setOrg(c, orgDomainKey(domain), org)
}

// GetOrgBySlug سازمان ذخیره‌شده برای این اسلاگ را می‌خواند.
// ورودی: slug. خروجی: کپی سازمان، found، ok.
func (c *TenantCache) GetOrgBySlug(slug string) (*models.Organization, bool, bool) {
	rec, ok := getTyped[orgRecord](c, orgSlugKey(slug))
	if !ok {
		return nil, false, false
	}
	if !rec.Found {
		return nil, false, true
	}
	return cloneOrg(rec.Org), true, true
}

// SetOrgBySlug سازمان را ذخیره می‌کند. org برابر nil یعنی پیدا نشد.
// ورودی: slug و org. خروجی: ندارد.
func (c *TenantCache) SetOrgBySlug(slug string, org *models.Organization) {
	setOrg(c, orgSlugKey(slug), org)
}

// getTyped مقدار تایپ‌شده را از کش می‌خواند.
// ورودی: کش و کلید. خروجی: مقدار و true اگر موجود و از نوع T باشد.
func getTyped[T any](c *TenantCache, key string) (T, bool) {
	var zero T
	if c == nil || c.store == nil || key == "" {
		return zero, false
	}
	return GetTyped[T](c.store, key)
}

// setClinic کلینیک یا نبودن آن را با TTL مستأجر ذخیره می‌کند.
// ورودی: کش، کلید، clinic. خروجی: ندارد.
func setClinic(c *TenantCache, key string, clinic *models.Clinic) {
	if c == nil || c.store == nil || key == "" {
		return
	}
	rec := clinicRecord{}
	if clinic != nil {
		rec.Found = true
		rec.Clinic = *cloneClinic(*clinic)
	}
	c.store.SetWithTTL(key, rec, TenantTTL)
}

// setOrg سازمان یا نبودن آن را با TTL مستأجر ذخیره می‌کند.
// ورودی: کش، کلید، org. خروجی: ندارد.
func setOrg(c *TenantCache, key string, org *models.Organization) {
	if c == nil || c.store == nil || key == "" {
		return
	}
	rec := orgRecord{}
	if org != nil {
		rec.Found = true
		rec.Org = *cloneOrg(*org)
	}
	c.store.SetWithTTL(key, rec, TenantTTL)
}

// clinicDomainKey کلید کش کلینیک بر اساس دامنه را می‌سازد.
func clinicDomainKey(domain string) string {
	return tenantKey("clinic", "domain", domain)
}

// clinicSlugKey کلید کش کلینیک بر اساس اسلاگ را می‌سازد.
func clinicSlugKey(slug string) string {
	return tenantKey("clinic", "slug", slug)
}

// orgDomainKey کلید کش سازمان بر اساس دامنه را می‌سازد.
func orgDomainKey(domain string) string {
	return tenantKey("org", "domain", domain)
}

// orgSlugKey کلید کش سازمان بر اساس اسلاگ را می‌سازد.
func orgSlugKey(slug string) string {
	return tenantKey("org", "slug", slug)
}

// tenantKey کلید یکدست کش را از نوع، فیلد و مقدار می‌سازد.
// ورودی: kind، field، value. خروجی: کلید یا رشته خالی اگر مقدار خالی باشد.
func tenantKey(kind, field, value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	return "tenant:" + kind + ":" + field + ":" + value
}

// cloneClinic یک کپی مستقل از کلینیک برمی‌گرداند تا تغییر caller کش را عوض نکند.
// ورودی: in. خروجی: اشاره‌گر به کپی.
func cloneClinic(in models.Clinic) *models.Clinic {
	out := in
	if in.Domain != nil {
		d := *in.Domain
		out.Domain = &d
	}
	if in.Slug != nil {
		s := *in.Slug
		out.Slug = &s
	}
	if in.LastSyncAt != nil {
		t := *in.LastSyncAt
		out.LastSyncAt = &t
	}
	return &out
}

// cloneOrg یک کپی مستقل از سازمان برمی‌گرداند تا تغییر caller کش را عوض نکند.
// ورودی: in. خروجی: اشاره‌گر به کپی.
func cloneOrg(in models.Organization) *models.Organization {
	out := in
	if in.Domain != nil {
		d := *in.Domain
		out.Domain = &d
	}
	if in.Slug != nil {
		s := *in.Slug
		out.Slug = &s
	}
	return &out
}
