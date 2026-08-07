package public

import (
	"net/http"

	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/components"
	"tebpardaz/server/views/layouts"
	"tebpardaz/shared/constants"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
)

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
func renderPublicLayout(c *gin.Context, tc *tenant.Context, child templ.Component, activeNav string) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	ctx := templ.WithChildren(c.Request.Context(), child)
	var err error
	switch tc.Layout {
	case constants.LayoutPlatform:
		view := layouts.PlatformLayoutView{
			CompanyName: "طب‌پرداز",
			NavLinks:    publicNavLinks(activeNav),
		}
		err = layouts.PlatformLayout(view).Render(ctx, c.Writer)
	case constants.LayoutOrgan:
		view := layouts.OrganLayoutView{NavLinks: publicNavLinks(activeNav)}
		if tc.Organization != nil {
			view.OrganizationName = tc.Organization.Name
			view.LogoURL = tc.Organization.LogoURL
		}
		err = layouts.OrganLayout(view).Render(ctx, c.Writer)
	case constants.LayoutPrivate:
		view := layouts.PrivateLayoutView{NavLinks: publicNavLinks(activeNav)}
		if tc.Clinic != nil {
			view.ClinicName = tc.Clinic.Name
		}
		err = layouts.PrivateLayout(view).Render(ctx, c.Writer)
	default:
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
	}
}

func publicNavLinks(active string) []components.NavLink {
	return []components.NavLink{
		{Label: "صفحه اصلی", Href: "/", Active: active == "home"},
		{Label: "پزشکان", Href: "/doctors", Active: active == "doctors" || active == "booking"},
		{Label: "نوبت‌دهی", Href: "/doctors", Active: active == "booking"},
		{Label: "اخبار", Href: "/news", Active: active == "news"},
		{Label: "جواب آزمایش", Href: "/test-results", Active: active == "test-results"},
	}
}
