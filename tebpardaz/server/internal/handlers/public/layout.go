package public

import (
	"fmt"
	"net/http"

	"tebpardaz/server/internal/branding"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/components"
	"tebpardaz/server/views/layouts"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
)

var (
	publicSectionRepo *repository.SectionRepo
	publicClinicRepo  *repository.ClinicRepo
)

// SetPublicSectionRepo registers the repository used to dynamically load active clinic sections in the public navbar.
// Input: pointer to repository.SectionRepo.
// Output: none.
func SetPublicSectionRepo(repo *repository.SectionRepo) {
	publicSectionRepo = repo
}

// SetPublicClinicRepo ریپازیتوری کلینیک‌ها را برای استفاده در ناوبری لایوت‌های ارگان و پلتفرم ثبت می‌کند.
// ورودی: اشاره‌گر به repository.ClinicRepo. خروجی: ندارد.
func SetPublicClinicRepo(repo *repository.ClinicRepo) {
	publicClinicRepo = repo
}

// resolveTenantClinicIDs returns clinic IDs in scope for the current public layout.
// Inputs: tenant context, clinic repo.
// Output: clinic IDs, whether to show clinic badge/filter (organ + platform), error.
func resolveTenantClinicIDs(tc *tenant.Context, clinics *repository.ClinicRepo) ([]uint, bool, error) {
	if tc == nil {
		return nil, false, nil
	}
	switch tc.Layout {
	case constants.LayoutPrivate:
		if tc.ClinicID == nil {
			return nil, false, nil
		}
		return []uint{*tc.ClinicID}, false, nil
	case constants.LayoutOrgan:
		if tc.OrganizationID == nil || clinics == nil {
			return nil, true, nil
		}
		rows, err := clinics.ListByOrganizationID(*tc.OrganizationID)
		if err != nil {
			return nil, true, err
		}
		ids := make([]uint, 0, len(rows))
		for _, clinic := range rows {
			ids = append(ids, clinic.ID)
		}
		return ids, true, nil
	case constants.LayoutPlatform:
		// Company site lists all clinics, same multi-clinic UX as organ.
		if clinics == nil {
			return nil, true, nil
		}
		rows, err := clinics.ListAll()
		if err != nil {
			return nil, true, err
		}
		ids := make([]uint, 0, len(rows))
		for _, clinic := range rows {
			ids = append(ids, clinic.ID)
		}
		return ids, true, nil
	default:
		return nil, false, nil
	}
}

// renderPublicLayout wraps child content in the layout for the current tenant.
// Platform uses the same shell pattern as organ (multi-clinic company brand).
// Inputs: gin context, tenant context, child templ component, active navigation key.
// Output: none (renders response or sets status code).
func renderPublicLayout(c *gin.Context, tc *tenant.Context, child templ.Component, activeNav string) {
	RenderPublicLayoutWithHead(c, tc, child, activeNav, layouts.PageHead{})
}

// RenderPublicLayoutWithHead wraps child content in the layout for the current tenant with custom SEO PageHead metadata.
// Inputs: gin context, tenant context, child templ component, active navigation key, custom PageHead.
// Output: none (renders response or sets status code).
func RenderPublicLayoutWithHead(c *gin.Context, tc *tenant.Context, child templ.Component, activeNav string, head layouts.PageHead) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	ctx := templ.WithChildren(c.Request.Context(), child)
	var err error
	switch tc.Layout {
	case constants.LayoutPlatform:
		if head.Title == "" {
			head.Title = "طب‌پرداز | پلتفرم جامع نوبت‌دهی آنلاین مراکز درمانی و پزشکان"
		}
		view := layouts.PlatformLayoutView{
			Head:        head,
			CompanyName: "طب پرداز شرق ایرانیان",
			NavLinks:    publicNavLinksForPlatform(tc, activeNav),
			LogoURL:     "/static/clinics/logo.jpg",
		}
		err = layouts.PlatformLayout(view).Render(ctx, c.Writer)
	case constants.LayoutOrgan:
		if head.Title == "" && tc.Organization != nil {
			head.Title = tc.Organization.Name + " | سامانه نوبت‌دهی آنلاین"
		}
		view := layouts.OrganLayoutView{
			Head:     head,
			NavLinks: publicNavLinksForOrgan(tc, activeNav),
		}
		if tc.Organization != nil {
			view.OrganizationName = tc.Organization.Name
			view.LogoURL = tc.Organization.LogoURL
		}
		err = layouts.OrganLayout(view).Render(ctx, c.Writer)
	case constants.LayoutPrivate:
		if head.Title == "" && tc.Clinic != nil {
			head.Title = tc.Clinic.Name + " | رزرو نوبت آنلاین"
		}
		if tc.Clinic != nil {
			// PageHead مقدار است؛ favicon باید قبل از کپی شدن داخل view ست شود.
			head.FaviconURL = branding.ClinicFaviconURL(tc.Clinic)
		}
		navLinks := publicNavLinksForTenant(tc, activeNav)
		view := layouts.PrivateLayoutView{
			Head:     head,
			NavLinks: navLinks,
		}
		if tc.Clinic != nil {
			view.ClinicName = tc.Clinic.Name
			view.LogoURL = branding.ClinicLogoURL(tc.Clinic)
			view.Phone = tc.Clinic.Phone
			view.Address = tc.Clinic.Address
			view.City = tc.Clinic.City.Name
			view.Province = tc.Clinic.City.Province
		}
		err = layouts.PrivateLayout(view).Render(ctx, c.Writer)
	default:
		c.Status(http.StatusNotFound)
		head := layouts.PageHead{
			Title:  "صفحه پیدا نشد | طب‌پرداز",
			Robots: "noindex, nofollow",
		}
		standalone := templ.WithChildren(c.Request.Context(), pages.NotFound(pages.NotFoundView{HomeURL: "/"}))
		_ = layouts.Document(head).Render(standalone, c.Writer)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
	}
}

// publicNavLinks لیست لینک‌های ناوبری هدر عمومی پیش‌فرض را برمی‌گرداند.
// ورودی: رشته کلید صفحه فعال (active). خروجی: آرایه آیتم‌های ناوبری NavLink.
func publicNavLinks(active string) []components.NavLink {
	return []components.NavLink{
		{Label: "صفحه اصلی", Href: "/", Active: active == "home"},
		{Label: "پزشکان", Href: "/doctors", Active: active == "doctors" || active == "booking"},
		{Label: "نوبت‌دهی", Href: "/doctors", Active: active == "booking"},
		{Label: "نوبت هفتگی پزشکان", Href: "/weekly-schedule", Active: active == "weekly-schedule"},
		{Label: "اخبار", Href: "/news", Active: active == "news"},
		{Label: "جواب آزمایش", Href: "/test-results", Active: active == "test-results"},
		{Label: "وضعیت نوبت", Href: "/waiting-queue", Active: active == "waiting-queue"},
	}
}

// publicNavLinksForPlatform لینک‌های ناوبری پلتفرم شامل دراپ‌داون بخش‌ها را برمی‌گرداند.
// ورودی: کانتکست tenant و کلید صفحه فعال. خروجی: آرایه آیتم‌های ناوبری NavLink.
func publicNavLinksForPlatform(tc *tenant.Context, active string) []components.NavLink {
	links := []components.NavLink{
		{Label: "صفحه اصلی", Href: "/", Active: active == "home"},
		{Label: "مراکز", Href: "/#clinics", Active: active == "clinics"},
		{Label: "پزشکان", Href: "/doctors", Active: active == "doctors" || active == "booking"},
		{Label: "نوبت‌دهی", Href: "/doctors", Active: active == "booking"},
		{Label: "نوبت هفتگی پزشکان", Href: "/weekly-schedule", Active: active == "weekly-schedule"},
	}

	if publicClinicRepo != nil && publicSectionRepo != nil {
		if clinics, err := publicClinicRepo.ListAll(); err == nil && len(clinics) > 0 {
			var sectionChildren []components.NavDropdownItem
			isAnyActive := false
			for _, clinic := range clinics {
				if sections, err := publicSectionRepo.ListActiveSectionsByClinic(clinic.ID); err == nil {
					clinicSlug := ""
					if clinic.Slug != nil && *clinic.Slug != "" {
						clinicSlug = *clinic.Slug
					}
					for _, s := range sections {
						targetHref := fmt.Sprintf("/section/%s", s.Slug)
						if clinicSlug != "" {
							targetHref = fmt.Sprintf("/clinics/%s/section/%s", clinicSlug, s.Slug)
						}
						childActive := active == "section_"+s.Slug
						if childActive {
							isAnyActive = true
						}
						sectionChildren = append(sectionChildren, components.NavDropdownItem{
							Label:  fmt.Sprintf("%s - %s", s.Title, clinic.Name),
							Href:   targetHref,
							Active: childActive,
						})
					}
				}
			}
			if len(sectionChildren) > 0 {
				links = append(links, components.NavLink{
					Label:    "بخش‌ها",
					Active:   isAnyActive,
					Children: sectionChildren,
				})
			}
		}
	}

	links = append(links,
		components.NavLink{Label: "اخبار", Href: "/news", Active: active == "news"},
		components.NavLink{Label: "جواب آزمایش", Href: "/test-results", Active: active == "test-results"},
		components.NavLink{Label: "وضعیت نوبت", Href: "/waiting-queue", Active: active == "waiting-queue"},
	)
	return links
}

// publicNavLinksForOrgan لینک‌های ناوبری ارگان شامل دراپ‌داون بخش‌های مراکز تابعه را برمی‌گرداند.
// ورودی: کانتکست tenant و کلید صفحه فعال. خروجی: آرایه آیتم‌های ناوبری NavLink.
func publicNavLinksForOrgan(tc *tenant.Context, active string) []components.NavLink {
	links := []components.NavLink{
		{Label: "صفحه اصلی", Href: "/", Active: active == "home"},
		{Label: "مراکز", Href: "/#clinics", Active: active == "clinics"},
		{Label: "پزشکان", Href: "/doctors", Active: active == "doctors" || active == "booking"},
		{Label: "نوبت‌دهی", Href: "/doctors", Active: active == "booking"},
		{Label: "برنامه هفتگی پزشکان", Href: "/weekly-schedule", Active: active == "weekly-schedule"},
	}

	if tc != nil && tc.OrganizationID != nil && publicClinicRepo != nil && publicSectionRepo != nil {
		if clinics, err := publicClinicRepo.ListByOrganizationID(*tc.OrganizationID); err == nil && len(clinics) > 0 {
			var sectionChildren []components.NavDropdownItem
			isAnyActive := false
			for _, clinic := range clinics {
				if sections, err := publicSectionRepo.ListActiveSectionsByClinic(clinic.ID); err == nil {
					clinicSlug := ""
					if clinic.Slug != nil && *clinic.Slug != "" {
						clinicSlug = *clinic.Slug
					}
					for _, s := range sections {
						targetHref := fmt.Sprintf("/section/%s", s.Slug)
						if clinicSlug != "" {
							targetHref = fmt.Sprintf("/clinics/%s/section/%s", clinicSlug, s.Slug)
						}
						childActive := active == "section_"+s.Slug
						if childActive {
							isAnyActive = true
						}
						sectionChildren = append(sectionChildren, components.NavDropdownItem{
							Label:  fmt.Sprintf("%s - %s", s.Title, clinic.Name),
							Href:   targetHref,
							Active: childActive,
						})
					}
				}
			}
			if len(sectionChildren) > 0 {
				links = append(links, components.NavLink{
					Label:    "بخش‌ها",
					Active:   isAnyActive,
					Children: sectionChildren,
				})
			}
		}
	}

	links = append(links,
		components.NavLink{Label: "اخبار", Href: "/news", Active: active == "news"},
		components.NavLink{Label: "جواب آزمایش", Href: "/test-results", Active: active == "test-results"},
		components.NavLink{Label: "وضعیت نوبت", Href: "/waiting-queue", Active: active == "waiting-queue"},
	)
	return links
}

// publicNavLinksForTenant لیست لینک‌های ناوبری را همراه با بخش‌های فعال کلینیک برای صفحات مرکز برمی‌گرداند.
// ورودی: کانتکست tenant و کلید صفحه فعال. خروجی: آرایه آیتم‌های ناوبری NavLink.
func publicNavLinksForTenant(tc *tenant.Context, active string) []components.NavLink {
	links := []components.NavLink{
		{Label: "صفحه اصلی", Href: "/", Active: active == "home"},
		{Label: "پزشکان", Href: "/doctors", Active: active == "doctors" || active == "booking"},
		{Label: "نوبت‌دهی", Href: "/doctors", Active: active == "booking"},
		{Label: "نوبت هفتگی پزشکان", Href: "/weekly-schedule", Active: active == "weekly-schedule"},
	}

	hasLab := false
	hasDoctorSite := false

	// بخش‌های اختصاصی و فعال هر مرکز در یک dropdown با عنوان «بخش‌ها» نمایش داده می‌شوند
	if tc != nil && tc.Layout == constants.LayoutPrivate && tc.ClinicID != nil && publicSectionRepo != nil {
		clinicID := *tc.ClinicID
		hasLab = publicSectionRepo.HasLabSection(clinicID)
		hasDoctorSite = publicSectionRepo.HasDoctorSiteSection(clinicID)

		if sections, err := publicSectionRepo.ListActiveSectionsByClinic(clinicID); err == nil && len(sections) > 0 {
			var sectionChildren []components.NavDropdownItem
			isAnyActive := false
			for _, s := range sections {
				childActive := active == "section_"+s.Slug
				if childActive {
					isAnyActive = true
				}
				sectionChildren = append(sectionChildren, components.NavDropdownItem{
					Label:  s.Title,
					Href:   fmt.Sprintf("/section/%s", s.Slug),
					Active: childActive,
				})
			}
			if len(sectionChildren) > 0 {
				links = append(links, components.NavLink{
					Label:    "بخش‌ها",
					Active:   isAnyActive,
					Children: sectionChildren,
				})
			}
		}
	}

	links = append(links, components.NavLink{Label: "اخبار", Href: "/news", Active: active == "news"})

	// جواب آزمایش فقط در صورت وجود بخش فعال آزمایشگاه در مرکز نمایش داده می‌شود
	if hasLab {
		links = append(links, components.NavLink{Label: "جواب آزمایش", Href: "/test-results", Active: active == "test-results"})
	}

	// وضعیت نوبت فقط در صورت وجود بخش فعال سایت پزشک در مرکز نمایش داده می‌شود
	if hasDoctorSite {
		links = append(links, components.NavLink{Label: "وضعیت نوبت", Href: "/waiting-queue", Active: active == "waiting-queue"})
	}

	return links
}
