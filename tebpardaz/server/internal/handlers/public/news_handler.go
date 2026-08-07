package public

import (
	"net/http"
	"strconv"

	"tebpardaz/server/internal/news"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/components"
	"tebpardaz/server/views/pages"

	"github.com/gin-gonic/gin"
)

// NewsHandler serves public news list and detail pages.
type NewsHandler struct {
	News    *news.Service
	Clinics *repository.ClinicRepo
}

// NewNewsHandler constructs a NewsHandler.
// Inputs: news service, clinic repo (for organ/platform clinic ID resolution).
// Output: pointer to NewsHandler.
func NewNewsHandler(svc *news.Service, clinics *repository.ClinicRepo) *NewsHandler {
	return &NewsHandler{News: svc, Clinics: clinics}
}

// List renders published news for the current tenant.
func (h *NewsHandler) List(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	ids, showClinic, err := resolveTenantClinicIDs(tc, h.Clinics)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	items, err := h.News.ListPublished(ids, 0)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	view := pages.NewsListView{
		Items:           toPublicCards(items, showClinic),
		ShowClinicBadge: showClinic,
		EmptyMessage:    "خبری برای نمایش وجود ندارد.",
	}
	renderPublicLayout(c, tc, pages.NewsList(view), "news")
}

// GetDetail renders a single published news article.
func (h *NewsHandler) GetDetail(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Status(http.StatusBadRequest)
		return
	}
	ids, showClinic, err := resolveTenantClinicIDs(tc, h.Clinics)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	item, err := h.News.GetPublished(uint(id), ids)
	if err != nil {
		if err == news.ErrForbidden || err == news.ErrNotFound {
			c.Status(http.StatusNotFound)
			return
		}
		c.Status(http.StatusInternalServerError)
		return
	}
	view := pages.NewsDetailView{
		TitleHTML:       item.Title,
		BodyHTML:        item.Body,
		CoverURL:        item.CoverURL,
		PublishedAt:     item.PublishedAt,
		ClinicName:      item.ClinicName,
		ShowClinicBadge: showClinic,
		BackURL:         "/news",
	}
	renderPublicLayout(c, tc, pages.NewsDetail(view), "news")
}

func toPublicCards(items []news.Item, showClinic bool) []components.NewsCardView {
	out := make([]components.NewsCardView, 0, len(items))
	for _, item := range items {
		card := components.NewsCardView{
			TitleHTML:       item.Title,
			ExcerptHTML:     item.Excerpt,
			CoverURL:        item.CoverURL,
			PublishedAt:     item.PublishedAt,
			DetailURL:       "/news/" + strconv.FormatUint(uint64(item.ID), 10),
			ClinicName:      item.ClinicName,
			ShowClinicBadge: showClinic,
		}
		out = append(out, card)
	}
	return out
}
