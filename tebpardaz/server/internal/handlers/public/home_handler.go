package public

import (
	"net/http"
	"strconv"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/news"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/components"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// HomeHandler صفحه اصلی مستأجر را سرو می‌کند.
type HomeHandler struct {
	News        *news.Service
	Clinics     *repository.ClinicRepo
	Specialties *repository.SpecialtyRepo
	Insurances  *repository.InsuranceRepo
	Listing     *booking.ListingService
}

// NewHomeHandler سازنده HomeHandler است.
func NewHomeHandler(
	svc *news.Service,
	clinics *repository.ClinicRepo,
	specialties *repository.SpecialtyRepo,
	insurances *repository.InsuranceRepo,
	listing *booking.ListingService,
) *HomeHandler {
	return &HomeHandler{
		News:        svc,
		Clinics:     clinics,
		Specialties: specialties,
		Insurances:  insurances,
		Listing:     listing,
	}
}

// Get صفحه اصلی را با پزشکان، تمام تخصص‌ها، بیمه، مراکز و اخبار رندر می‌کند.
// ورودی: context درخواست gin حاوی اطلاعات tenant. خروجی: ندارد (صفحه اصلی HTML رندر می‌شود).
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

	allSpecialties := loadApprovedSpecialties(h.Specialties)

	homeView := pages.HomeView{
		LatestNews:          latest,
		Specialties:         allSpecialties,
		SpecialtiesListURL:  "",
		ShowSpecialtiesLink: false,
		Insurances:          loadHomeInsurances(tc, h.Clinics, h.Insurances),
		Clinics:             toHomeClinicCards(clinicRows),
		Doctors:             loadHomeDoctors(h.Listing, tc, h.Clinics, showClinic),
		ShowClinicCards:     showClinic,
		ShowClinicBadge:     showClinic,
		NewsListURL:         "/news",
	}
	renderPublicLayout(c, tc, pages.Home(homeView), "home")
}

// loadHomeDoctors حداکثر ۱۰ پزشک برای صفحه اول را بارگذاری می‌کند.
func loadHomeDoctors(
	listing *booking.ListingService,
	tc *tenant.Context,
	clinics *repository.ClinicRepo,
	showClinicBadge bool,
) []components.DoctorCardView {
	if listing == nil || tc == nil {
		return nil
	}
	ids, _, err := resolveTenantClinicIDs(tc, clinics)
	if err != nil || len(ids) == 0 {
		return nil
	}
	diversify := tc.Layout == constants.LayoutOrgan || tc.Layout == constants.LayoutPlatform
	cards, err := listing.ListHomeDoctors(booking.ListFilter{
		ClinicIDs: ids,
		Layout:    tc.Layout,
	}, showClinicBadge, 10, diversify)
	if err != nil || len(cards) == 0 {
		return nil
	}
	out := make([]components.DoctorCardView, 0, len(cards))
	for _, item := range cards {
		out = append(out, components.DoctorCardView{
			Name:            item.Name,
			SpecialtyName:   item.SpecialtyName,
			DoctorSystemID:  item.DoctorSystemID,
			PhotoURL:        item.PhotoURL,
			Photo300:        item.Photo300,
			Photo600:        item.Photo600,
			Photo900:        item.Photo900,
			Photo1200:       item.Photo1200,
			ShortDesc:       item.ShortDesc,
			ClinicName:      item.ClinicName,
			ShowClinicBadge: item.ShowClinicBadge,
			HasSlot:         item.HasSlot,
			NearestStartsAt: item.NearestStartsAt,
			BookingURL:      item.BookingURL,
		})
	}
	return out
}

// loadHomeInsurances بیمه‌های مرتبط با مستأجر فعلی را برمی‌گرداند.
func loadHomeInsurances(tc *tenant.Context, clinics *repository.ClinicRepo, insurances *repository.InsuranceRepo) []components.InsuranceItemView {
	if tc == nil || insurances == nil {
		return nil
	}
	ids, _, err := resolveTenantClinicIDs(tc, clinics)
	if err != nil || len(ids) == 0 {
		return nil
	}
	rows, err := insurances.ListByClinicIDs(ids)
	if err != nil || len(rows) == 0 {
		return nil
	}
	out := make([]components.InsuranceItemView, 0, len(rows))
	for _, row := range rows {
		out = append(out, components.InsuranceItemView{
			Name:        row.Name,
			LogoURL:     row.LogoURL,
			Description: row.Description,
		})
	}
	return out
}

// loadHomeClinics مراکز تابعه ارگان/پلتفرم را برمی‌گرداند.
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

// toHomeClinicCards مدل مرکز را به کارت خانه نگاشت می‌کند.
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
