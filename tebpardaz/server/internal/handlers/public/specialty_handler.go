package public

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type specialtyCatalog interface {
	ListWithPublicDoctors(clinicIDs []uint) ([]repository.SpecialtyPublicCount, error)
	GetBySlug(slug string) (*models.Specialty, error)
}

type specialtyDoctors interface {
	ListPublic(filter repository.DoctorPublicFilter) ([]models.Doctor, error)
}

type specialtyClinics interface {
	ListAll() ([]models.Clinic, error)
	ListByIDs(ids []uint) ([]models.Clinic, error)
}

// SpecialtyHandler صفحهٔ عمومی تخصص‌های پلتفرم را سرو می‌کند.
type SpecialtyHandler struct {
	Specialties specialtyCatalog
	Doctors     specialtyDoctors
	Clinics     specialtyClinics
	Slots       booking.SlotSource
}

// NewSpecialtyHandler سازنده SpecialtyHandler است.
// ورودی: تخصص، پزشک، مرکز و منبع نوبت. خروجی: handler. روی tenant این صفحات ۴۰۴ است.
func NewSpecialtyHandler(specialties *repository.SpecialtyRepo, doctors *repository.DoctorRepo, clinics *repository.ClinicRepo, slots booking.SlotSource) *SpecialtyHandler {
	return &SpecialtyHandler{Specialties: specialties, Doctors: doctors, Clinics: clinics, Slots: slots}
}

// Index فهرست تخصص‌هایی را نشان می‌دهد که پزشک عمومی دارند.
// ورودی: درخواست پلتفرم. خروجی: HTML یا ۴۰۴ روی tenant.
func (h *SpecialtyHandler) Index(c *gin.Context) {
	tc, ok := platformSpecialty(c)
	if !ok {
		return
	}
	clinicIDs, err := platformClinicIDs(h.Clinics)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	var rows []repository.SpecialtyPublicCount
	if h.Specialties != nil {
		rows, err = h.Specialties.ListWithPublicDoctors(clinicIDs)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
	}
	items := make([]pages.SpecialtyIndexItem, 0, len(rows))
	graphItems := make([]seo.ItemListElementDTO, 0, len(rows))
	for _, row := range rows {
		if !seo.SpecialtyIndexable(row.Slug, row.DoctorCount) {
			continue
		}
		path := seo.SpecialtyPath(row.Slug)
		items = append(items, pages.SpecialtyIndexItem{
			Name:        row.Name,
			URL:         path,
			DoctorCount: row.DoctorCount,
		})
		graphItems = append(graphItems, seo.ItemListElementDTO{
			Position: len(graphItems) + 1,
			Name:     row.Name,
			URL:      absolutePublicURL(c, path),
		})
	}
	meta := seo.SpecialtyIndexMeta(publicBaseURL(c))
	head := headFromMeta(meta)
	head.JSONLD = seo.SpecialtyIndexGraph(absolutePublicURL(c, "/"), meta.Canonical, meta.Title, meta.Description, graphItems)
	RenderPublicLayoutWithHead(c, tc, pages.Specialties(pages.SpecialtiesView{Items: items}), "specialties", head)
}

// Detail پزشکان یک تخصص را با slug ذخیره‌شده نشان می‌دهد.
// ورودی: :slug. خروجی: HTML، یا ۴۰۴ اگر slug یا context پلتفرم نباشد.
func (h *SpecialtyHandler) Detail(c *gin.Context) {
	tc, ok := platformSpecialty(c)
	if !ok {
		return
	}
	if h.Specialties == nil {
		NotFound(c)
		return
	}
	row, err := h.Specialties.GetBySlug(c.Param("slug"))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			NotFound(c)
			return
		}
		c.Status(http.StatusInternalServerError)
		return
	}
	if row == nil || !row.IsVisibleInBooking() {
		NotFound(c)
		return
	}
	clinicIDs, err := platformClinicIDs(h.Clinics)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	var doctors []models.Doctor
	if h.Doctors != nil {
		doctors, err = h.Doctors.ListPublic(repository.DoctorPublicFilter{
			ClinicIDs:   clinicIDs,
			SpecialtyID: row.ID,
		})
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
	}
	clinics, err := h.loadClinics(doctors)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	nearest, err := h.nearestSlots(clinicIDs, doctors)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	page, pageOK := parseSpecialtyPage(c.Query("page"), len(doctors), doctorListPageSize)
	if !pageOK {
		NotFound(c)
		return
	}
	cards := booking.PublicDoctorCards(doctors, clinics, nearest, constants.LayoutPlatform, true)
	start, end := pageSlice(len(doctors), page, doctorListPageSize)
	pageDoctors := doctors[start:end]
	pageCards := cards[start:end]
	totalPages := 0
	if len(doctors) > 0 {
		totalPages = (len(doctors) + doctorListPageSize - 1) / doctorListPageSize
	}
	viewCards := toDoctorCards(pageCards, seo.SpecialtyPagePath(row.Slug, page), true, true)
	intro := ""
	if len(doctors) > 0 {
		intro = "فهرست پزشکان " + row.Name + " فعال در مراکز درمانی طب‌پرداز را مشاهده کنید و برای زمان مناسب نوبت بگیرید."
	}
	view := pages.SpecialtyDetailView{
		Name:       row.Name,
		Slug:       row.Slug,
		Intro:      intro,
		Doctors:    viewCards,
		Page:       page,
		TotalPages: totalPages,
		TotalCount: len(doctors),
		Empty:      len(doctors) == 0,
	}
	indexable := seo.SpecialtyIndexable(row.Slug, len(doctors))
	meta := seo.SpecialtyDetailMeta(row.Name, row.Slug, publicBaseURL(c), page, len(doctors), indexable)
	head := headFromMeta(meta)
	head.JSONLD = specialtyDetailJSONLD(c, row.Slug, row.Name, meta, pageDoctors, clinics, page, indexable, len(doctors))
	RenderPublicLayoutWithHead(c, tc, pages.SpecialtyDetail(view), "specialties", head)
}

// platformSpecialty فقط دامنه پلتفرم را به صفحه تخصص راه می‌دهد.
// ورودی: درخواست. خروجی: context یا false بعد از ۴۰۴/۴۰۱.
func platformSpecialty(c *gin.Context) (*tenant.Context, bool) {
	tc, ok := tenant.FromGin(c)
	if !ok || tc == nil {
		c.Status(http.StatusUnauthorized)
		return nil, false
	}
	if tc.Layout != constants.LayoutPlatform {
		NotFound(c)
		return nil, false
	}
	return tc, true
}

// platformClinicIDs همان مراکز فعال وب را که /doctors روی پلتفرم می‌بیند برمی‌گرداند.
// ورودی: منبع مراکز. خروجی: شناسه‌ها.
func platformClinicIDs(clinics specialtyClinics) ([]uint, error) {
	if clinics == nil {
		return nil, nil
	}
	rows, err := clinics.ListAll()
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for _, clinic := range rows {
		ids = append(ids, clinic.ID)
	}
	return ids, nil
}

// loadClinics مراکز پزشکان همین صفحه را یک‌جا می‌خواند.
// ورودی: پزشکان. خروجی: مراکز. به ازای هر پزشک query جدا نمی‌زند.
func (h *SpecialtyHandler) loadClinics(doctors []models.Doctor) ([]models.Clinic, error) {
	if h.Clinics == nil || len(doctors) == 0 {
		return nil, nil
	}
	seen := map[uint]struct{}{}
	ids := make([]uint, 0)
	for _, doctor := range doctors {
		if _, ok := seen[doctor.ClinicID]; ok || doctor.ClinicID == 0 {
			continue
		}
		seen[doctor.ClinicID] = struct{}{}
		ids = append(ids, doctor.ClinicID)
	}
	return h.Clinics.ListByIDs(ids)
}

// nearestSlots نزدیک‌ترین نوبت پزشکان را در یک فراخوانی می‌خواند.
// ورودی: مراکز و پزشکان. خروجی: نقشه اسلات. منبع خالی خطا نیست.
func (h *SpecialtyHandler) nearestSlots(clinicIDs []uint, doctors []models.Doctor) (map[uint]models.DoctorSlot, error) {
	if h.Slots == nil || len(doctors) == 0 {
		return map[uint]models.DoctorSlot{}, nil
	}
	ids := make([]uint, 0, len(doctors))
	for _, doctor := range doctors {
		ids = append(ids, doctor.ID)
	}
	return h.Slots.NearestAvailableByDoctors(clinicIDs, ids, time.Now(), time.Time{})
}

// specialtyDetailJSONLD گراف detail را از پزشکان همین صفحه می‌سازد.
// ورودی: درخواست، نام تخصص، متا، پزشکان صفحه، مراکز، صفحه، مجاز بودن فهرست و تعداد کل.
// خروجی: JSON-LD. کد ملی و شماره نظام نوشته نمی‌شود.
func specialtyDetailJSONLD(c *gin.Context, slug, name string, meta seo.Meta, doctors []models.Doctor, clinics []models.Clinic, page int, includeList bool, total int) string {
	byID := map[uint]models.Clinic{}
	for _, clinic := range clinics {
		byID[clinic.ID] = clinic
	}
	base := publicBaseURL(c)
	physicians := make([]seo.PhysicianDTO, 0, len(doctors))
	positions := make([]int, 0, len(doctors))
	if includeList {
		for i, doctor := range doctors {
			display := strings.TrimSpace(doctor.Name)
			if display == "" {
				display = strings.TrimSpace(doctor.FirstName + " " + doctor.LastName)
			}
			clinic := byID[doctor.ClinicID]
			bookingPath := booking.BuildBookingURL(constants.LayoutPlatform, booking.ClinicPathKey(&clinic), doctor.Slug)
			bookingURL := absolutePublicURL(c, bookingPath)
			if display == "" || strings.TrimSpace(doctor.Slug) == "" || bookingPath == "/doctors" {
				continue
			}
			clinicID := ""
			if seo.ClinicIndexable(clinic.IsActiveOnWebsite, clinic.Slug) {
				clinicID, _ = seo.OrganizationClinicRef(base, viewSlug(&clinic))
			}
			physicians = append(physicians, seo.PhysicianDTO{
				ID:         seo.PageFragmentID(bookingURL, "physician"),
				Name:       display,
				URL:        bookingURL,
				Specialty:  name,
				ImageURL:   seo.PhysicianImage(base, resolveDoctorPhoto(&doctor), doctor.UseClinicLogo),
				ClinicID:   clinicID,
				ClinicName: strings.TrimSpace(clinic.Name),
			})
			positions = append(positions, seo.ListPosition(page, doctorListPageSize, i))
		}
	}
	return seo.SpecialtyDetailGraph(
		absolutePublicURL(c, "/"),
		absolutePublicURL(c, "/specialties"),
		absolutePublicURL(c, seo.SpecialtyPath(slug)),
		meta.Canonical,
		meta.Title,
		meta.Description,
		name,
		physicians,
		positions,
		includeList && len(physicians) > 0,
		total,
	)
}

// parseSpecialtyPage صفحه landing را بدون clamp می‌خواند.
// ورودی: query خام، تعداد پزشک و اندازه صفحه. خروجی: صفحه معتبر، یا false برای 404.
// خالی یعنی صفحه ۱. صفر، منفی، متن و صفحه بعد از آخر 404 هستند.
func parseSpecialtyPage(raw string, total, pageSize int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 1, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, false
	}
	if n == 1 {
		return 1, true
	}
	if pageSize < 1 {
		pageSize = doctorListPageSize
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	if n > totalPages {
		return 0, false
	}
	return n, true
}

// pageSlice بازه پزشکان همان صفحه را بعد از اعتبارسنجی صفحه برمی‌گرداند.
// ورودی: تعداد کل، صفحه و اندازه. خروجی: ابتدا و انتهای نیمه‌باز.
func pageSlice(total, page, pageSize int) (int, int) {
	if total == 0 || page < 1 || pageSize < 1 {
		return 0, 0
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return start, end
}
