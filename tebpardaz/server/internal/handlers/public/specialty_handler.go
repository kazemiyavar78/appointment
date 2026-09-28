package public

import (
	"net/http"

	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// SpecialtyHandler صفحه فهرست تخصص‌ها را سرو می‌کند.
type SpecialtyHandler struct {
	Specialties *repository.SpecialtyRepo
}

// NewSpecialtyHandler سازنده SpecialtyHandler است.
func NewSpecialtyHandler(specialties *repository.SpecialtyRepo) *SpecialtyHandler {
	return &SpecialtyHandler{Specialties: specialties}
}

// List صفحه تمام تخصص‌ها را به‌صورت فهرست لینک‌دار رندر می‌کند.
func (h *SpecialtyHandler) List(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	switch tc.Layout {
	case constants.LayoutOrgan, constants.LayoutPrivate, constants.LayoutPlatform:
	default:
		NotFound(c)
		return
	}

	all := loadApprovedSpecialties(h.Specialties)
	view := pages.SpecialtiesView{
		Specialties: all,
		TotalCount:  len(all),
	}
	renderPublicLayout(c, tc, pages.Specialties(view), "specialties")
}
