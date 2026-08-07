package tenant

import (
	"errors"
	
	"strings"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/shared/constants"

	"gorm.io/gorm"
)

// ErrTenantNotFound is returned when host/slug does not map to any tenant.
var ErrTenantNotFound = errors.New("tenant not found")

// Context holds resolved tenant identity for the current request.
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

// Resolver maps hostnames/slugs to a tenant Context and layout.
type Resolver struct {
	BaseDomain string
	Clinics    *repository.ClinicRepo
	Orgs       *repository.OrganizationRepo
}

// NewResolver constructs a tenant Resolver.
// Inputs: baseDomain (platform host), clinics repo, orgs repo.
// Output: pointer to Resolver.
func NewResolver(baseDomain string, clinics *repository.ClinicRepo, orgs *repository.OrganizationRepo) *Resolver {
	return &Resolver{
		BaseDomain: strings.ToLower(strings.TrimSpace(baseDomain)),
		Clinics:    clinics,
		Orgs:       orgs,
	}
}

// Resolve returns the tenant for the given host and URL path.
// Inputs: host (may include port), path (request URL path for optional /{slug} routing).
// Output: Context with Layout set, or ErrTenantNotFound / DB error.
func (r *Resolver) Resolve(host, path string) (*Context, error) {
	normalized := normalizeHost(host)
	
	base := r.BaseDomain

	ctx := &Context{Host: normalized}

	// 1) Platform apex (tebpardaz.ir / www) — path slug or marketing home.
	if base != "" && (normalized == base || normalized == "www."+base) {
		if slug := firstPathSegment(path); slug != "" {
			ctx.Slug = slug
			return r.resolveSlug(ctx, slug)
		}
		ctx.Layout = constants.LayoutPlatform
		ctx.OrganizationID = new(uint)
		*ctx.OrganizationID = 3
		return ctx, nil
	}

	// 2) Subdomain on platform base: {slug}.tebpardaz.ir
	if base != "" && strings.HasSuffix(normalized, "."+base) {
		slug := strings.TrimSuffix(normalized, "."+base)
		slug = strings.TrimSuffix(slug, ".")
		if slug != "" && slug != "www" {
			ctx.Slug = slug
			return r.resolveSlug(ctx, slug)
		}
	}

	// 3) Custom clinic domain.
	if r.Clinics != nil {
		clinic, err := r.Clinics.GetByDomain(normalized)
		if err == nil && clinic != nil {
			return r.contextFromClinic(ctx, clinic), nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	// 4) Custom organization domain.
	if r.Orgs != nil {
		org, err := r.Orgs.GetByDomain(normalized)
		if err == nil && org != nil {
			return r.contextFromOrg(ctx, org), nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	return nil, ErrTenantNotFound
}

// resolveSlug looks up clinic then organization by slug.
func (r *Resolver) resolveSlug(ctx *Context, slug string) (*Context, error) {
	if r.Clinics != nil {
		clinic, err := r.Clinics.GetBySlug(slug)
		if err == nil && clinic != nil {
			return r.contextFromClinic(ctx, clinic), nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	if r.Orgs != nil {
		org, err := r.Orgs.GetBySlug(slug)
		if err == nil && org != nil {
			return r.contextFromOrg(ctx, org), nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return nil, ErrTenantNotFound
}

// contextFromClinic fills Context fields and selects private layout.
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

// contextFromOrg fills Context fields and selects organ layout.
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

// normalizeHost lowercases host and strips port.
func normalizeHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if i := strings.Index(h, ":"); i >= 0 {
		h = h[:i]
	}
	return h
}

// firstPathSegment returns the first non-empty path segment without leading slash.
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
		"doctors", "booking", "news", "clinics", "test-results", "healthz":
		return ""
	}
	return seg
}
