package public

import (
	"net/http"
	"strings"
	"time"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/branding"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type clinicDirectory interface {
	ListAll() ([]models.Clinic, error)
	GetBySlug(slug string) (*models.Clinic, error)
}

// ClinicHandler صفحهٔ عمومی مراکز پلتفرم را سرو می‌کند.
type ClinicHandler struct {
	Clinics clinicDirectory
	Doctors specialtyDoctors
	Slots   booking.SlotSource
}

// NewClinicHandler سازنده ClinicHandler است.
// ورودی: مرکز، پزشک و منبع نوبت. خروجی: handler.
// فهرست فقط پلتفرم است. detail روی پلتفرم و دامنهٔ سازمان است و روی دامنهٔ اختصاصی ۴۰۴ می‌ماند.
func NewClinicHandler(clinics *repository.ClinicRepo, doctors *repository.DoctorRepo, slots booking.SlotSource) *ClinicHandler {
	return &ClinicHandler{Clinics: clinics, Doctors: doctors, Slots: slots}
}

// Index فهرست مراکز فعال روی وب را با slug ذخیره‌شده نشان می‌دهد.
// ورودی: درخواست پلتفرم. خروجی: HTML یا ۴۰۴ بیرون از پلتفرم.
func (h *ClinicHandler) Index(c *gin.Context) {
	tc, ok := platformSpecialty(c)
	if !ok {
		return
	}
	var rows []models.Clinic
	if h.Clinics != nil {
		var err error
		rows, err = h.Clinics.ListAll()
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
	}
	items := make([]pages.ClinicIndexItem, 0, len(rows))
	graphItems := make([]seo.ItemListElementDTO, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		if !seo.ClinicIndexable(row.IsActiveOnWebsite, row.Slug) {
			continue
		}
		path := seo.ClinicPath(*row.Slug)
		items = append(items, pages.ClinicIndexItem{
			Name:        row.Name,
			URL:         path,
			City:        strings.TrimSpace(row.City.Name),
			Province:    strings.TrimSpace(row.City.Province),
			Address:     strings.TrimSpace(row.Address),
			Phone:       strings.TrimSpace(row.Phone),
			Description: strings.TrimSpace(row.Description),
			LogoURL:     branding.ClinicLogoURL(row),
		})
		graphItems = append(graphItems, seo.ItemListElementDTO{
			Position: len(graphItems) + 1,
			Name:     row.Name,
			URL:      absolutePublicURL(c, path),
			Type:     "MedicalClinic",
		})
	}
	meta := seo.ClinicIndexMeta(publicBaseURL(c))
	head := headFromMeta(meta)
	head.JSONLD = seo.ClinicIndexGraph(absolutePublicURL(c, "/"), meta.Canonical, meta.Title, meta.Description, graphItems)
	RenderPublicLayoutWithHead(c, tc, pages.Clinics(pages.ClinicsView{Items: items}), "clinics", head)
}

// clinicDetailTenant سطح‌هایی را که landing مرکز دارند برمی‌گرداند.
// ورودی: درخواست. خروجی: مستأجر، یا false بعد از ۴۰۴/۴۰۱.
// پلتفرم و دامنهٔ سازمان مجازند. دامنهٔ اختصاصی و هاست قدیمی سازمان ۴۰۴ می‌مانند.
func clinicDetailTenant(c *gin.Context) (*tenant.Context, bool) {
	tc, ok := tenant.FromGin(c)
	if !ok || tc == nil {
		c.Status(http.StatusUnauthorized)
		return nil, false
	}
	if tc.Layout == constants.LayoutPlatform || organizationDomainSurface(tc) {
		return tc, true
	}
	NotFound(c)
	return nil, false
}

// clinicVisibleOnLanding می‌گوید این مرکز روی landing همین سطح مجاز است یا نه.
// ورودی: مستأجر و مرکز. خروجی: true با slug عمومی و قرارداد همان سطح.
func clinicVisibleOnLanding(tc *tenant.Context, row *models.Clinic) bool {
	if row == nil || tc == nil || !seo.ClinicIndexable(row.IsActiveOnWebsite, row.Slug) {
		return false
	}
	if tc.Layout == constants.LayoutPlatform {
		return tenant.ClinicVisibleOnPlatform(row)
	}
	return clinicVisibleForTenant(tc, row)
}

// Detail پروفایل یک مرکز را با slug ذخیره‌شده نشان می‌دهد.
// ورودی: :clinic_slug، هم‌نام با مسیرهای بخش. خروجی: HTML، یا ۴۰۴ اگر مرکز همین سطح نباشد.
func (h *ClinicHandler) Detail(c *gin.Context) {
	tc, ok := clinicDetailTenant(c)
	if !ok {
		return
	}
	if h.Clinics == nil {
		NotFound(c)
		return
	}
	slug := strings.TrimSpace(c.Param("clinic_slug"))
	if seo.ClinicPath(slug) == "" {
		NotFound(c)
		return
	}
	row, err := h.Clinics.GetBySlug(slug)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			NotFound(c)
			return
		}
		c.Status(http.StatusInternalServerError)
		return
	}
	if !clinicVisibleOnLanding(tc, row) {
		NotFound(c)
		return
	}
	var doctors []models.Doctor
	if h.Doctors != nil {
		doctors, err = h.Doctors.ListPublic(repository.DoctorPublicFilter{ClinicIDs: []uint{row.ID}})
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
	}
	page, pageOK := parseSpecialtyPage(c.Query("page"), len(doctors), doctorListPageSize)
	if !pageOK {
		NotFound(c)
		return
	}
	nearest, err := h.nearestSlots([]uint{row.ID}, doctors)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	cards := booking.PublicDoctorCards(doctors, []models.Clinic{*row}, nearest, tc.Layout, false)
	start, end := pageSlice(len(doctors), page, doctorListPageSize)
	pageDoctors := doctors[start:end]
	pageCards := cards[start:end]
	totalPages := 0
	if len(doctors) > 0 {
		totalPages = (len(doctors) + doctorListPageSize - 1) / doctorListPageSize
	}
	view := pages.ClinicDetailView{
		Name:          row.Name,
		Slug:          strings.TrimSpace(*row.Slug),
		Description:   strings.TrimSpace(row.Description),
		Address:       strings.TrimSpace(row.Address),
		Phone:         strings.TrimSpace(row.Phone),
		City:          strings.TrimSpace(row.City.Name),
		Province:      strings.TrimSpace(row.City.Province),
		LogoURL:       branding.ClinicLogoURL(row),
		Doctors:       toDoctorCards(pageCards, seo.ClinicPagePath(viewSlug(row), page), tc.Layout == constants.LayoutPlatform, false),
		Page:          page,
		TotalPages:    totalPages,
		TotalCount:    len(doctors),
		Empty:         len(doctors) == 0,
		ShowDirectory: tc.Layout == constants.LayoutPlatform,
	}
	meta := seo.ClinicDetailMeta(row.Name, view.City, row.Description, view.Slug, publicBaseURL(c), page)
	head := headFromMeta(meta)
	head.JSONLD = clinicDetailJSONLD(c, tc, row, meta, pageDoctors, page, len(doctors) > 0, len(doctors))
	RenderPublicLayoutWithHead(c, tc, pages.ClinicDetail(view), "clinics", head)
}

// viewSlug اسلاگ ذخیره‌شدهٔ مرکز را برای URL برمی‌گرداند.
// ورودی: ردیف مرکز. خروجی: متن trimشده یا خالی.
func viewSlug(row *models.Clinic) string {
	if row == nil || row.Slug == nil {
		return ""
	}
	return strings.TrimSpace(*row.Slug)
}

// nearestSlots نزدیک‌ترین نوبت پزشکان همین مرکز را در یک فراخوانی می‌خواند.
// ورودی: شناسه مرکز و پزشکان. خروجی: نقشه اسلات. منبع خالی خطا نیست.
func (h *ClinicHandler) nearestSlots(clinicIDs []uint, doctors []models.Doctor) (map[uint]models.DoctorSlot, error) {
	if h.Slots == nil || len(doctors) == 0 {
		return map[uint]models.DoctorSlot{}, nil
	}
	ids := make([]uint, 0, len(doctors))
	for _, doctor := range doctors {
		ids = append(ids, doctor.ID)
	}
	return h.Slots.NearestAvailableByDoctors(clinicIDs, ids, time.Now(), time.Time{})
}

// clinicDetailJSONLD گراف detail را از دادهٔ واقعی مرکز و پزشکان همین صفحه می‌سازد.
// ورودی: درخواست، مستأجر، مرکز، متا، پزشکان صفحه، صفحه، مجاز بودن فهرست و تعداد کل.
// خروجی: JSON-LD. parentOrganization فقط روی دامنهٔ همان سازمان نوشته می‌شود.
func clinicDetailJSONLD(c *gin.Context, tc *tenant.Context, row *models.Clinic, meta seo.Meta, doctors []models.Doctor, page int, includeList bool, total int) string {
	slug := viewSlug(row)
	entityURL := absolutePublicURL(c, seo.ClinicPath(slug))
	base := publicBaseURL(c)
	clinicID := seo.PageFragmentID(entityURL, "clinic")
	addr := clinicPostalAddress(row)
	dto := seo.MedicalClinicDTO{
		Name:        strings.TrimSpace(row.Name),
		URL:         entityURL,
		Description: seo.PlainText(row.Description),
		LogoURL:     seo.AbsoluteSchemaURL(base, branding.ClinicLogoURL(row)),
		Telephone:   strings.TrimSpace(row.Phone),
		Address:     addr,
	}
	if origin := organizationSurfaceOrigin(c, tc); origin != "" && tc.OrganizationID != nil && tenant.ClinicVisibleOnOrganization(row, *tc.OrganizationID) {
		dto.ParentID = seo.OriginID(origin, "organization")
	}
	physicians := make([]seo.PhysicianDTO, 0, len(doctors))
	positions := make([]int, 0, len(doctors))
	if includeList {
		for i, doctor := range doctors {
			display := strings.TrimSpace(doctor.Name)
			if display == "" {
				display = strings.TrimSpace(doctor.FirstName + " " + doctor.LastName)
			}
			bookingPath := booking.BuildBookingURL(tc.Layout, slug, doctor.Slug)
			bookingURL := absolutePublicURL(c, bookingPath)
			if display == "" || strings.TrimSpace(doctor.Slug) == "" || bookingPath == "/doctors" {
				continue
			}
			physicians = append(physicians, seo.PhysicianDTO{
				ID:         seo.PageFragmentID(bookingURL, "physician"),
				Name:       display,
				URL:        bookingURL,
				Specialty:  strings.TrimSpace(doctor.Specialty.Name),
				ImageURL:   seo.PhysicianImage(base, resolveDoctorPhoto(&doctor), doctor.UseClinicLogo),
				ClinicID:   clinicID,
				ClinicName: dto.Name,
			})
			positions = append(positions, seo.ListPosition(page, doctorListPageSize, i))
		}
	}
	indexURL := ""
	if tc != nil && tc.Layout == constants.LayoutPlatform {
		indexURL = absolutePublicURL(c, "/clinics")
	}
	return seo.ClinicDetailGraph(
		absolutePublicURL(c, "/"),
		indexURL,
		entityURL,
		meta.Canonical,
		meta.Title,
		meta.Description,
		dto,
		physicians,
		positions,
		includeList && len(physicians) > 0,
		total,
	)
}

// clinicPostalAddress آدرس واقعی مرکز را برای اسکیما برمی‌گرداند.
// ورودی: مرکز با شهر preloadشده. خروجی: آدرس یا nil اگر هیچ بخشی نباشد.
func clinicPostalAddress(row *models.Clinic) *seo.PostalAddressDTO {
	if row == nil {
		return nil
	}
	street := strings.TrimSpace(row.Address)
	city := strings.TrimSpace(row.City.Name)
	province := strings.TrimSpace(row.City.Province)
	if street == "" && city == "" && province == "" {
		return nil
	}
	return &seo.PostalAddressDTO{
		StreetAddress:   street,
		AddressLocality: city,
		AddressRegion:   province,
		AddressCountry:  "IR",
	}
}
