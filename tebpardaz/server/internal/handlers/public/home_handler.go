package public

import (
	"net/http"
	"strconv"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/news"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/components"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// HomeHandler serves the tenant home page.
type HomeHandler struct {
	News        *news.Service
	Clinics     *repository.ClinicRepo
	Specialties *repository.SpecialtyRepo
}

// NewHomeHandler constructs a HomeHandler.
// Inputs: news service, clinic repo, specialty repo.
// Output: pointer to HomeHandler.
func NewHomeHandler(svc *news.Service, clinics *repository.ClinicRepo, specialties *repository.SpecialtyRepo) *HomeHandler {
	return &HomeHandler{News: svc, Clinics: clinics, Specialties: specialties}
}

// Get renders the home page with specialties, clinics (organ/platform), and latest news.
func (h *HomeHandler) Get(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}

	var latest []pages.HomeNewsItem
	showClinic := false
	var clinicRows []models.Clinic
	switch tc.Layout {
	case constants.LayoutOrgan, constants.LayoutPrivate, constants.LayoutPlatform:
		ids, clinicBadge, err := resolveTenantClinicIDs(tc, h.Clinics)
		showClinic = clinicBadge
		if err == nil && len(ids) > 0 {
			items, listErr := h.News.ListPublished(ids, 6)
			if listErr == nil {
				for _, item := range items {
					latest = append(latest, pages.HomeNewsItem{
						TitleHTML:       item.Title,
						ExcerptHTML:     item.Excerpt,
						CoverURL:        item.CoverURL,
						PublishedAt:     item.PublishedAt,
						DetailURL:       "/news/" + strconv.FormatUint(uint64(item.ID), 10),
						ClinicName:      item.ClinicName,
						ShowClinicBadge: showClinic,
					})
				}
			}
		}
		if showClinic {
			clinicRows = loadHomeClinics(tc, h.Clinics)
		}
	default:
		c.Status(http.StatusNotFound)
		return
	}

	homeView := pages.HomeView{
		LatestNews:      latest,
		Specialties:     loadHomeSpecialties(h.Specialties),
		Clinics:         toHomeClinicCards(clinicRows),
		ShowClinicCards: showClinic,
		ShowClinicBadge: showClinic,
		NewsListURL:     "/news",
	}
	renderPublicLayout(c, tc, pages.Home(homeView), "home")
}

// loadHomeSpecialties returns approved specialty cards for the home page.
// Inputs: specialty repo (may be nil).
// Output: specialty card views ordered by name.
func loadHomeSpecialties(repo *repository.SpecialtyRepo) []components.SpecialtyCardView {
	if repo == nil {
		return nil
	}
	rows, err := repo.ListApproved()
	if err != nil || len(rows) == 0 {
		return nil
	}
	out := make([]components.SpecialtyCardView, 0, len(rows))
	for _, row := range rows {
		out = append(out, components.SpecialtyCardView{ID: row.ID, Name: row.Name})
	}
	return out
}

// loadHomeClinics returns subsidiary clinics for organ/platform home cards.
// Inputs: tenant context, clinic repo.
// Output: clinic rows with City preloaded.
func loadHomeClinics(tc *tenant.Context, clinics *repository.ClinicRepo) []models.Clinic {
	if tc == nil || clinics == nil {
		return nil
	}
	switch tc.Layout {
	case constants.LayoutOrgan:
		if tc.OrganizationID == nil {
			return nil
		}
		rows, err := clinics.ListByOrganizationID(*tc.OrganizationID)
		if err != nil {
			return nil
		}
		return rows
	case constants.LayoutPlatform:
		rows, err := clinics.ListAll()
		if err != nil {
			return nil
		}
		return rows
	default:
		return nil
	}
}

// toHomeClinicCards maps clinic models to home clinic card views.
// Inputs: clinic rows with optional City.
// Output: ClinicCardView slice.
func toHomeClinicCards(rows []models.Clinic) []components.ClinicCardView {
	if len(rows) == 0 {
		return nil
	}
	out := make([]components.ClinicCardView, 0, len(rows))
	for _, row := range rows {
		out = append(out, components.ClinicCardView{
			ID:       row.ID,
			Name:     row.Name,
			Address:  row.Address,
			Phone:    row.Phone,
			Province: row.City.Province,
			City:     row.City.Name,
		})
	}
	return out
}
