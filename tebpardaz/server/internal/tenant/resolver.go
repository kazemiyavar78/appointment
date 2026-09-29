package tenant

import (
	"errors"

	"strings"

	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/shared/constants"

	"gorm.io/gorm"
)

// ErrTenantNotFound وقتی هاست یا اسلاگ به هیچ مستأجری نخورد برمی‌گردد.
var ErrTenantNotFound = errors.New("tenant not found")

// Context هویت مستأجر حل‌شده برای درخواست جاری را نگه می‌دارد.
type Context struct {
	Host           string
	Slug           string
	Layout         constants.LayoutKind
	TenantType     constants.TenantType
	ClinicID       *uint
	OrganizationID *uint
	Clinic         *models.Clinic
	Organization   *models.Organization
}

// Resolver هاست و اسلاگ را به Context و چیدمان مستأجر نگاشت می‌کند.
type Resolver struct {
	BaseDomain string
	Clinics    *repository.ClinicRepo
	Orgs       *repository.OrganizationRepo
	Lookup     *LookupService
}

// NewResolver یک Resolver مستأجر می‌سازد.
// ورودی: baseDomain (هاست پلتفرم)، مخزن کلینیک، مخزن سازمان.
// خروجی: اشاره‌گر به Resolver.
func NewResolver(baseDomain string, clinics *repository.ClinicRepo, orgs *repository.OrganizationRepo) *Resolver {
	return &Resolver{
		BaseDomain: strings.ToLower(strings.TrimSpace(baseDomain)),
		Clinics:    clinics,
		Orgs:       orgs,
		Lookup:     NewLookupService(clinics, orgs, nil),
	}
}

// UseCache کش مستأجر را وصل می‌کند تا جستجوی دامنه و اسلاگ از حافظه خوانده شود.
// ورودی: tenantCache. اگر nil باشد هر Resolve مستقیم به دیتابیس می‌رود.
// خروجی: ندارد.
func (r *Resolver) UseCache(tenantCache *cache.TenantCache) {
	if r == nil {
		return
	}
	if r.Lookup == nil {
		r.Lookup = NewLookupService(r.Clinics, r.Orgs, tenantCache)
		return
	}
	r.Lookup.Cache = tenantCache
}

// Resolve مستأجر هاست و مسیر را برمی‌گرداند.
// ورودی: host (ممکن است پورت داشته باشد)، path برای مسیر اختیاری /{slug}.
// خروجی: Context با Layout، یا ErrTenantNotFound / خطای دیتابیس.
func (r *Resolver) Resolve(host, path string) (*Context, error) {
	normalized := normalizeHost(host)

	base := r.BaseDomain

	ctx := &Context{Host: normalized}

	// 1) Platform apex (tebpardaz.ir / www) — path slug or marketing home.
	if base != "" && (normalized == base || normalized == "www."+base) {
		// لینک‌های عمیق صف انتظار (QR قبض) نباید به عنوان اسلاگ مستأجر تفسیر شوند.
		if !isWaitingQueueDeepLink(path) {
			if slug := firstPathSegment(path); slug != "" {
				ctx.Slug = slug
				return r.resolveSlug(ctx, slug)
			}
		}
		ctx.Layout = constants.LayoutPlatform
		ctx.OrganizationID = new(uint)
		*ctx.OrganizationID = 3
		return ctx, nil
	}

	// زیردامنهٔ پلتفرم ({slug}.tebpardaz.ir) سطح عمومی نیست.
	// نه کلینیک و نه سازمان از روی اسلاگ این هاست ساخته نمی‌شوند و به دامنهٔ سفارشی هم نمی‌افتند.
	if platformSubdomain(normalized, base) {
		return nil, ErrTenantNotFound
	}

	// 3) Custom clinic domain.
	clinic, err := r.lookupClinicByDomain(normalized)
	if err == nil && clinic != nil {
		return r.contextFromClinic(ctx, clinic), nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	// 4) Custom organization domain.
	org, err := r.lookupOrgByDomain(normalized)
	if err == nil && org != nil {
		return r.contextFromOrg(ctx, org), nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return nil, ErrTenantNotFound
}

// platformSubdomain می‌گوید هاست زیردامنهٔ دامنهٔ پایه است (غیر از خود دامنه و www).
// ورودی: host نرمال‌شده و base. خروجی: true برای شکل‌هایی مثل foo.tebpardaz.ir.
func platformSubdomain(host, base string) bool {
	if base == "" || host == "" || host == base || host == "www."+base {
		return false
	}
	return strings.HasSuffix(host, "."+base)
}

// resolveSlug اول کلینیک و بعد سازمان را با اسلاگ مسیر روی هاست اصلی پیدا می‌کند.
// ورودی: ctx و slug. خروجی: Context یا خطا.
func (r *Resolver) resolveSlug(ctx *Context, slug string) (*Context, error) {
	clinic, err := r.lookupClinicBySlug(slug)
	if err == nil && clinic != nil {
		return r.contextFromClinic(ctx, clinic), nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	org, err := r.lookupOrgBySlug(slug)
	if err == nil && org != nil {
		return r.contextFromOrg(ctx, org), nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return nil, ErrTenantNotFound
}

// lookupClinicByDomain کلینیک را از کش می‌خواند و در صورت نبود، کش را از دیتابیس پر می‌کند.
// ورودی: domain. خروجی: کلینیک یا خطای نبودن / دیتابیس.
func (r *Resolver) lookupClinicByDomain(domain string) (*models.Clinic, error) {
	if r != nil && r.Lookup != nil {
		return r.Lookup.ClinicByDomain(domain)
	}
	if r == nil || r.Clinics == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.Clinics.GetByDomain(domain)
}

// lookupClinicBySlug کلینیک را از کش می‌خواند و در صورت نبود، کش را از دیتابیس پر می‌کند.
// ورودی: slug. خروجی: کلینیک یا خطای نبودن / دیتابیس.
func (r *Resolver) lookupClinicBySlug(slug string) (*models.Clinic, error) {
	if r != nil && r.Lookup != nil {
		return r.Lookup.ClinicBySlug(slug)
	}
	if r == nil || r.Clinics == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.Clinics.GetBySlug(slug)
}

// lookupOrgByDomain سازمان را از کش می‌خواند و در صورت نبود، کش را از دیتابیس پر می‌کند.
// ورودی: domain. خروجی: سازمان یا خطای نبودن / دیتابیس.
func (r *Resolver) lookupOrgByDomain(domain string) (*models.Organization, error) {
	if r != nil && r.Lookup != nil {
		return r.Lookup.OrgByDomain(domain)
	}
	if r == nil || r.Orgs == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.Orgs.GetByDomain(domain)
}

// lookupOrgBySlug سازمان را از کش می‌خواند و در صورت نبود، کش را از دیتابیس پر می‌کند.
// ورودی: slug. خروجی: سازمان یا خطای نبودن / دیتابیس.
func (r *Resolver) lookupOrgBySlug(slug string) (*models.Organization, error) {
	if r != nil && r.Lookup != nil {
		return r.Lookup.OrgBySlug(slug)
	}
	if r == nil || r.Orgs == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.Orgs.GetBySlug(slug)
}

// contextFromClinic فیلدهای Context را پر می‌کند و چیدمان خصوصی را انتخاب می‌کند.
// ورودی: ctx و clinic. خروجی: همان Context.
func (r *Resolver) contextFromClinic(ctx *Context, clinic *models.Clinic) *Context {
	id := clinic.ID
	orgID := clinic.OrganizationID
	ctx.Clinic = clinic
	ctx.ClinicID = &id
	ctx.OrganizationID = &orgID
	ctx.TenantType = constants.TenantType(clinic.TenantType)
	ctx.Layout = constants.LayoutPrivate
	if clinic.Slug != nil {
		ctx.Slug = *clinic.Slug
	}
	// Subsidiary clinics are normally reached via the organ site; if hit directly, still private page.
	if ctx.TenantType == constants.TenantOrganSubsidiary {
		ctx.Layout = constants.LayoutPrivate
	}
	return ctx
}

// contextFromOrg فیلدهای Context را پر می‌کند و چیدمان سازمان را انتخاب می‌کند.
// ورودی: ctx و org. خروجی: همان Context.
func (r *Resolver) contextFromOrg(ctx *Context, org *models.Organization) *Context {
	id := org.ID
	ctx.Organization = org
	ctx.OrganizationID = &id
	ctx.Layout = constants.LayoutOrgan
	if org.Slug != nil {
		ctx.Slug = *org.Slug
	}
	return ctx
}

// normalizeHost هاست را کوچک می‌کند و پورت را حذف می‌کند.
// ورودی: host. خروجی: هاست نرمال‌شده.
func normalizeHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if i := strings.Index(h, ":"); i >= 0 {
		h = h[:i]
	}
	return h
}

// firstPathSegment اولین بخش غیرخالی مسیر را بدون اسلش اول برمی‌گرداند.
// مسیرهای ثابت سامانه را اسلاگ مستأجر حساب نمی‌کند.
// ورودی: path. خروجی: اسلاگ یا رشته خالی.
func firstPathSegment(path string) string {
	p := strings.Trim(path, "/")
	if p == "" {
		return ""
	}
	parts := strings.Split(p, "/")
	if len(parts) == 0 {
		return ""
	}
	seg := strings.ToLower(parts[0])
	switch seg {
	case "static", "api", "ws", "admin", "favicon.ico",
		"doctors", "booking", "news", "clinics", "sections", "specialties",
		"about", "contact", "terms", "reviews", "otp", "patient",
		"test-results", "weekly-schedule", "waiting-queue", "healthz",
		"working-hours", "message-to-visitors", "equipment", "section",
		"ساعات-کاری", "پیام-به-مراجعین", "تجهیزات", "برنامه-هفتگی-پزشکان", "معرفی",
		"sitemap.xml", "robots.txt", "sw.js", "manifest.webmanifest":
		return ""
	}
	return seg
}

// isWaitingQueueDeepLink تشخیص می‌دهد مسیر مربوط به لینک QR صف انتظار است
// (مثلاً /123/۰۰۱۲۳۴۵۶۷۸/waiting-queue یا /123/-/5/ws/waiting-queue).
func isWaitingQueueDeepLink(path string) bool {
	p := strings.ToLower(strings.Trim(path, "/"))
	if p == "waiting-queue" {
		return true
	}
	return strings.HasSuffix(p, "/waiting-queue") || strings.HasSuffix(p, "/ws/waiting-queue") || strings.HasSuffix(p, "/push-subscribe")
}
