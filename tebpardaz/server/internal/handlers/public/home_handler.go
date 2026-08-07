package public

import (
	"net/http"
	"strconv"

	"tebpardaz/server/internal/news"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// HomeHandler serves the tenant home page.
type HomeHandler struct {
	News    *news.Service
	Clinics *repository.ClinicRepo
}

// NewHomeHandler constructs a HomeHandler.
// Inputs: news service, clinic repo.
// Output: pointer to HomeHandler.
func NewHomeHandler(svc *news.Service, clinics *repository.ClinicRepo) *HomeHandler {
	return &HomeHandler{News: svc, Clinics: clinics}
}

// Get renders the home page with latest published news for the tenant.
func (h *HomeHandler) Get(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}

	var latest []pages.HomeNewsItem
	showClinic := false
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
	default:
		c.Status(http.StatusNotFound)
		return
	}

	homeView := pages.HomeView{
		LatestNews:      latest,
		ShowClinicBadge: showClinic,
		NewsListURL:     "/news",
	}
	renderPublicLayout(c, tc, pages.Home(homeView), "home")
}
