package public

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/components"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type memCatalog struct {
	rows    []repository.SpecialtyPublicCount
	row     *models.Specialty
	gotSlug string
}

func (m *memCatalog) ListWithPublicDoctors([]uint) ([]repository.SpecialtyPublicCount, error) {
	return m.rows, nil
}

func (m *memCatalog) GetBySlug(slug string) (*models.Specialty, error) {
	m.gotSlug = slug
	if m.row == nil || m.row.Slug != slug {
		return nil, gorm.ErrRecordNotFound
	}
	return m.row, nil
}

type memDoctors struct {
	rows []models.Doctor
}

func (m *memDoctors) ListPublic(filter repository.DoctorPublicFilter) ([]models.Doctor, error) {
	out := make([]models.Doctor, 0)
	for _, row := range m.rows {
		if filter.SpecialtyID == 0 || row.SpecialtyID == filter.SpecialtyID {
			out = append(out, row)
		}
	}
	return out, nil
}

type memClinics struct {
	rows []models.Clinic
}

func (m *memClinics) ListAll() ([]models.Clinic, error) { return m.rows, nil }

func (m *memClinics) GetBySlug(slug string) (*models.Clinic, error) {
	for i := range m.rows {
		row := &m.rows[i]
		if row.IsActiveOnWebsite && row.Slug != nil && *row.Slug == slug {
			return row, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *memClinics) GetByID(id uint) (*models.Clinic, error) {
	for i := range m.rows {
		row := &m.rows[i]
		if row.ID == id && row.IsActiveOnWebsite {
			return row, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *memClinics) ListByOrganizationID(orgID uint) ([]models.Clinic, error) {
	out := make([]models.Clinic, 0)
	for _, row := range m.rows {
		if row.IsActiveOnWebsite && row.OrganizationID == orgID {
			out = append(out, row)
		}
	}
	return out, nil
}

func (m *memClinics) ListByIDs(ids []uint) ([]models.Clinic, error) {
	allow := map[uint]bool{}
	for _, id := range ids {
		allow[id] = true
	}
	out := make([]models.Clinic, 0)
	for _, row := range m.rows {
		if allow[row.ID] {
			out = append(out, row)
		}
	}
	return out, nil
}

func specialtyEngine(h *SpecialtyHandler, layout constants.LayoutKind) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(tenant.ContextKey, &tenant.Context{Layout: layout, Host: c.Request.Host})
		c.Next()
	})
	pub := r.Group("/")
	GETAndHEAD(pub, "/specialties", h.Index)
	GETAndHEAD(pub, "/specialties/:slug", h.Detail)
	return r
}

func doSpecialty(t *testing.T, r http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Host = "tebpardaz.ir"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSpecialtyRoutesPlatformAndTenant(t *testing.T) {
	slug := "قلب-و-عروق"
	clinicSlug := "chamran"
	catalog := &memCatalog{
		rows: []repository.SpecialtyPublicCount{{ID: 9, Name: "قلب و عروق", Slug: slug, DoctorCount: 1}},
		row: &models.Specialty{
			Model:         gorm.Model{ID: 9},
			Name:          "قلب و عروق",
			Slug:          slug,
			IsApproved:    true,
			ShowInBooking: true,
		},
	}
	doctors := &memDoctors{rows: []models.Doctor{{
		Model:       gorm.Model{ID: 4},
		Name:        "رضا احمدی",
		Slug:        "reza",
		SpecialtyID: 9,
		ClinicID:    3,
		Specialty:   models.Specialty{Name: "قلب و عروق", Slug: slug},
	}}}
	clinics := &memClinics{rows: []models.Clinic{{
		Model: gorm.Model{ID: 3},
		Name:  "درمانگاه چمران",
		Slug:  &clinicSlug,
	}}}
	h := &SpecialtyHandler{Specialties: catalog, Doctors: doctors, Clinics: clinics}
	platform := specialtyEngine(h, constants.LayoutPlatform)

	index := doSpecialty(t, platform, http.MethodGet, "/specialties")
	if index.Code != http.StatusOK {
		t.Fatalf("index status = %d", index.Code)
	}
	body := index.Body.String()
	if !strings.Contains(body, "تخصص‌های پزشکی و نوبت‌دهی آنلاین | طب‌پرداز") ||
		!strings.Contains(body, `<h1 class="ui-page-title">تخصص‌های پزشکی</h1>`) ||
		!strings.Contains(body, `rel="canonical" href="https://tebpardaz.ir/specialties"`) ||
		!strings.Contains(body, `content="index,follow"`) ||
		!strings.Contains(body, `href="`+seo.SpecialtyPath(slug)+`"`) {
		t.Fatalf("index html missing seo fields")
	}

	head := doSpecialty(t, platform, http.MethodHead, "/specialties")
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("index head status=%d body=%d", head.Code, head.Body.Len())
	}

	detail := doSpecialty(t, platform, http.MethodGet, "/specialties/"+url.PathEscape(slug))
	if detail.Code != http.StatusOK || catalog.gotSlug != slug {
		t.Fatalf("detail status=%d slug=%q", detail.Code, catalog.gotSlug)
	}
	detailBody := detail.Body.String()
	canon := seo.AbsoluteURL("https://tebpardaz.ir", seo.SpecialtyPath(slug))
	if !strings.Contains(detailBody, "پزشکان قلب و عروق و نوبت‌دهی آنلاین | طب‌پرداز") ||
		!strings.Contains(detailBody, `<h1 class="ui-page-title">پزشکان قلب و عروق</h1>`) ||
		!strings.Contains(detailBody, `href="`+canon+`"`) ||
		!strings.Contains(detailBody, `content="index,follow"`) ||
		!strings.Contains(detailBody, `href="/booking/chamran/reza"`) ||
		strings.Contains(detailBody, "%25") ||
		strings.Contains(detailBody, "0012345678") {
		t.Fatalf("detail html missing fields")
	}
	detailHead := doSpecialty(t, platform, http.MethodHead, "/specialties/"+url.PathEscape(slug))
	if detailHead.Code != http.StatusOK || detailHead.Body.Len() != 0 {
		t.Fatalf("detail head status=%d body=%d", detailHead.Code, detailHead.Body.Len())
	}

	missing := doSpecialty(t, platform, http.MethodGet, "/specialties/not-a-specialty")
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), "noindex, nofollow") {
		t.Fatalf("missing status=%d", missing.Code)
	}

	tenant := specialtyEngine(h, constants.LayoutPrivate)
	for _, path := range []string{"/specialties", "/specialties/" + url.PathEscape(slug)} {
		res := doSpecialty(t, tenant, http.MethodGet, path)
		if res.Code != http.StatusNotFound {
			t.Fatalf("%s tenant status=%d", path, res.Code)
		}
	}
}

func TestSpecialtyDetailPageAndEmpty(t *testing.T) {
	slug := "داخلی"
	catalog := &memCatalog{row: &models.Specialty{
		Model: gorm.Model{ID: 2}, Name: "داخلی", Slug: slug, IsApproved: true, ShowInBooking: true,
	}}
	clinicSlug := "center"
	doctors := make([]models.Doctor, 0, 13)
	for i := 0; i < 13; i++ {
		doctors = append(doctors, models.Doctor{
			Model:       gorm.Model{ID: uint(i + 1)},
			Name:        "پزشک",
			Slug:        "dr",
			SpecialtyID: 2,
			ClinicID:    1,
			NationalID:  "0099887766",
		})
	}
	h := &SpecialtyHandler{
		Specialties: catalog,
		Doctors:     &memDoctors{rows: doctors},
		Clinics:     &memClinics{rows: []models.Clinic{{Model: gorm.Model{ID: 1}, Name: "مرکز", Slug: &clinicSlug}}},
	}
	platform := specialtyEngine(h, constants.LayoutPlatform)
	page2 := doSpecialty(t, platform, http.MethodGet, "/specialties/"+url.PathEscape(slug)+"?page=2")
	if page2.Code != http.StatusOK {
		t.Fatalf("page2 status=%d", page2.Code)
	}
	body := page2.Body.String()
	if !strings.Contains(body, "page=2") || !strings.Contains(body, `"position":13`) || strings.Contains(body, "0099887766") {
		t.Fatalf("page2 json missing position")
	}

	emptyH := &SpecialtyHandler{
		Specialties: &memCatalog{row: catalog.row},
		Doctors:     &memDoctors{},
		Clinics:     &memClinics{rows: []models.Clinic{{Model: gorm.Model{ID: 1}, Slug: &clinicSlug}}},
	}
	empty := doSpecialty(t, specialtyEngine(emptyH, constants.LayoutPlatform), http.MethodGet, "/specialties/"+url.PathEscape(slug))
	emptyBody := empty.Body.String()
	if empty.Code != http.StatusOK || !strings.Contains(emptyBody, "noindex,follow") || strings.Contains(emptyBody, `"@type":"Physician"`) || strings.Contains(emptyBody, `"@type":"ItemList"`) {
		t.Fatalf("empty status=%d", empty.Code)
	}
}

func TestToDoctorCardsSpecialtyLinkOnlyOnPlatform(t *testing.T) {
	cards := sampleDoctorCards()
	linked := toDoctorCards(cards, "/doctors", true, false)
	plain := toDoctorCards(cards, "/doctors", false, false)
	if linked[0].SpecialtyURL != seo.SpecialtyPath("داخلی") || plain[0].SpecialtyURL != "" {
		t.Fatalf("linked=%q plain=%q", linked[0].SpecialtyURL, plain[0].SpecialtyURL)
	}
}

func TestSpecialtyNotFoundWhenMissing(t *testing.T) {
	h := &SpecialtyHandler{Specialties: &memCatalog{}, Doctors: &memDoctors{}, Clinics: &memClinics{}}
	res := doSpecialty(t, specialtyEngine(h, constants.LayoutPlatform), http.MethodGet, "/specialties/missing")
	if res.Code != http.StatusNotFound || !strings.Contains(res.Body.String(), "noindex, nofollow") {
		t.Fatalf("status=%d", res.Code)
	}
}

func TestSpecialtyNotFoundWhenNotPublic(t *testing.T) {
	slug := "داخلی"
	cases := []models.Specialty{
		{Model: gorm.Model{ID: 2}, Name: "داخلی", Slug: slug, IsApproved: false, ShowInBooking: true},
		{Model: gorm.Model{ID: 2}, Name: "داخلی", Slug: slug, IsApproved: true, ShowInBooking: false},
	}
	for _, row := range cases {
		row := row
		h := &SpecialtyHandler{
			Specialties: &memCatalog{row: &row},
			Doctors: &memDoctors{rows: []models.Doctor{{
				Model: gorm.Model{ID: 1}, Name: "پزشک", Slug: "dr", SpecialtyID: 2, ClinicID: 1,
			}}},
			Clinics: &memClinics{rows: []models.Clinic{{Model: gorm.Model{ID: 1}}}},
		}
		res := doSpecialty(t, specialtyEngine(h, constants.LayoutPlatform), http.MethodGet, "/specialties/"+url.PathEscape(slug))
		if res.Code != http.StatusNotFound || !strings.Contains(res.Body.String(), "noindex, nofollow") {
			t.Fatalf("approved=%v booking=%v status=%d", row.IsApproved, row.ShowInBooking, res.Code)
		}
	}
}

func TestSpecialtyWithoutPublicDoctorIsNoindex(t *testing.T) {
	slug := "داخلی"
	catalog := &memCatalog{
		rows: []repository.SpecialtyPublicCount{{ID: 2, Name: "داخلی", Slug: slug, DoctorCount: 0}},
		row:  &models.Specialty{Model: gorm.Model{ID: 2}, Name: "داخلی", Slug: slug, IsApproved: true, ShowInBooking: true},
	}
	h := &SpecialtyHandler{Specialties: catalog, Doctors: &memDoctors{}, Clinics: &memClinics{rows: []models.Clinic{{Model: gorm.Model{ID: 1}}}}}
	platform := specialtyEngine(h, constants.LayoutPlatform)
	detail := doSpecialty(t, platform, http.MethodGet, "/specialties/"+url.PathEscape(slug))
	body := detail.Body.String()
	canon := seo.AbsoluteURL("https://tebpardaz.ir", seo.SpecialtyPath(slug))
	if detail.Code != http.StatusOK ||
		!strings.Contains(body, "noindex,follow") ||
		!strings.Contains(body, `href="`+canon+`"`) ||
		!strings.Contains(body, `<h1 class="ui-page-title">پزشکان داخلی</h1>`) ||
		!strings.Contains(body, "در حال حاضر پزشک فعالی برای این تخصص موجود نیست.") ||
		strings.Contains(body, `"@type":"Physician"`) ||
		strings.Contains(body, `"@type":"ItemList"`) {
		t.Fatalf("empty detail status=%d", detail.Code)
	}
	index := doSpecialty(t, platform, http.MethodGet, "/specialties")
	if index.Code != http.StatusOK || strings.Contains(index.Body.String(), seo.SpecialtyPath(slug)) {
		t.Fatal("empty specialty leaked into index")
	}
}

func TestSpecialtyWithPublicDoctorIsIndex(t *testing.T) {
	slug := "داخلی"
	catalog := &memCatalog{
		rows: []repository.SpecialtyPublicCount{{ID: 2, Name: "داخلی", Slug: slug, DoctorCount: 1}},
		row:  &models.Specialty{Model: gorm.Model{ID: 2}, Name: "داخلی", Slug: slug, IsApproved: true, ShowInBooking: true},
	}
	clinicSlug := "center"
	h := &SpecialtyHandler{
		Specialties: catalog,
		Doctors: &memDoctors{rows: []models.Doctor{{
			Model: gorm.Model{ID: 1}, Name: "پزشک", Slug: "dr", SpecialtyID: 2, ClinicID: 1,
		}}},
		Clinics: &memClinics{rows: []models.Clinic{{Model: gorm.Model{ID: 1}, Name: "مرکز", Slug: &clinicSlug}}},
	}
	platform := specialtyEngine(h, constants.LayoutPlatform)
	detail := doSpecialty(t, platform, http.MethodGet, "/specialties/"+url.PathEscape(slug))
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `content="index,follow"`) {
		t.Fatalf("detail status=%d", detail.Code)
	}
	index := doSpecialty(t, platform, http.MethodGet, "/specialties")
	if !strings.Contains(index.Body.String(), `href="`+seo.SpecialtyPath(slug)+`"`) {
		t.Fatal("indexable specialty missing from index")
	}
}

func TestSpecialtyPageBounds(t *testing.T) {
	slug := "داخلی"
	doctors := make([]models.Doctor, 0, 13)
	for i := 0; i < 13; i++ {
		doctors = append(doctors, models.Doctor{
			Model: gorm.Model{ID: uint(i + 1)}, Name: "پزشک", Slug: "dr", SpecialtyID: 2, ClinicID: 1,
		})
	}
	h := &SpecialtyHandler{
		Specialties: &memCatalog{row: &models.Specialty{
			Model: gorm.Model{ID: 2}, Name: "داخلی", Slug: slug, IsApproved: true, ShowInBooking: true,
		}},
		Doctors: &memDoctors{rows: doctors},
		Clinics: &memClinics{rows: []models.Clinic{{Model: gorm.Model{ID: 1}}}},
	}
	platform := specialtyEngine(h, constants.LayoutPlatform)
	base := "/specialties/" + url.PathEscape(slug)
	page1 := doSpecialty(t, platform, http.MethodGet, base+"?page=1")
	canon := seo.AbsoluteURL("https://tebpardaz.ir", seo.SpecialtyPath(slug))
	if page1.Code != http.StatusOK || !strings.Contains(page1.Body.String(), `href="`+canon+`"`) || strings.Contains(page1.Body.String(), canon+"?page=1") {
		t.Fatalf("page1 status=%d", page1.Code)
	}
	over := doSpecialty(t, platform, http.MethodGet, base+"?page=999")
	if over.Code != http.StatusNotFound || !strings.Contains(over.Body.String(), "noindex, nofollow") {
		t.Fatalf("oversize status=%d", over.Code)
	}
	for _, raw := range []string{"0", "-1", "abc"} {
		res := doSpecialty(t, platform, http.MethodGet, base+"?page="+raw)
		if res.Code != http.StatusNotFound {
			t.Fatalf("page %s status=%d", raw, res.Code)
		}
	}
	empty := &SpecialtyHandler{
		Specialties: h.Specialties,
		Doctors:     &memDoctors{},
		Clinics:     h.Clinics,
	}
	beyondEmpty := doSpecialty(t, specialtyEngine(empty, constants.LayoutPlatform), http.MethodGet, base+"?page=2")
	if beyondEmpty.Code != http.StatusNotFound {
		t.Fatalf("empty page2 status=%d", beyondEmpty.Code)
	}
	mixed := doSpecialty(t, platform, http.MethodGet, base+"?page=2&foo=bar")
	pageCanon := seo.AbsoluteURL("https://tebpardaz.ir", seo.SpecialtyPagePath(slug, 2))
	if mixed.Code != http.StatusOK || strings.Contains(mixed.Body.String(), "foo=bar") || !strings.Contains(mixed.Body.String(), `href="`+pageCanon+`"`) {
		t.Fatalf("mixed query status=%d", mixed.Code)
	}
}

func TestSpecialtyUnknownQueryCanonical(t *testing.T) {
	slug := "داخلی"
	h := &SpecialtyHandler{
		Specialties: &memCatalog{
			rows: []repository.SpecialtyPublicCount{{ID: 2, Name: "داخلی", Slug: slug, DoctorCount: 1}},
			row:  &models.Specialty{Model: gorm.Model{ID: 2}, Name: "داخلی", Slug: slug, IsApproved: true, ShowInBooking: true},
		},
		Doctors: &memDoctors{rows: []models.Doctor{{
			Model: gorm.Model{ID: 1}, Name: "پزشک", Slug: "dr", SpecialtyID: 2, ClinicID: 1,
		}}},
		Clinics: &memClinics{rows: []models.Clinic{{Model: gorm.Model{ID: 1}}}},
	}
	platform := specialtyEngine(h, constants.LayoutPlatform)
	base := "/specialties/" + url.PathEscape(slug)
	res := doSpecialty(t, platform, http.MethodGet, base+"?foo=bar")
	body := res.Body.String()
	want := seo.AbsoluteURL("https://tebpardaz.ir", seo.SpecialtyPagePath(slug, 1))
	if res.Code != http.StatusOK || strings.Contains(body, "foo=bar") || !strings.Contains(body, `href="`+want+`"`) {
		t.Fatalf("unknown query status=%d", res.Code)
	}
	index := doSpecialty(t, platform, http.MethodGet, "/specialties?foo=bar")
	if !strings.Contains(index.Body.String(), `rel="canonical" href="https://tebpardaz.ir/specialties"`) || strings.Contains(index.Body.String(), "foo=bar") {
		t.Fatal("index kept unknown query")
	}
}

func TestSpecialtyPersianHTMLPaths(t *testing.T) {
	slug := "داخلی"
	catalog := &memCatalog{
		rows: []repository.SpecialtyPublicCount{{ID: 2, Name: "داخلی", Slug: slug, DoctorCount: 1}},
		row:  &models.Specialty{Model: gorm.Model{ID: 2}, Name: "داخلی", Slug: slug, IsApproved: true, ShowInBooking: true},
	}
	h := &SpecialtyHandler{
		Specialties: catalog,
		Doctors: &memDoctors{rows: []models.Doctor{{
			Model: gorm.Model{ID: 1}, Name: "پزشک", Slug: "dr", SpecialtyID: 2, ClinicID: 1,
		}}},
		Clinics: &memClinics{rows: []models.Clinic{{Model: gorm.Model{ID: 1}}}},
	}
	platform := specialtyEngine(h, constants.LayoutPlatform)
	path := seo.SpecialtyPath(slug)
	index := doSpecialty(t, platform, http.MethodGet, "/specialties")
	detail := doSpecialty(t, platform, http.MethodGet, "/specialties/"+url.PathEscape(slug))
	canon := seo.AbsoluteURL("https://tebpardaz.ir", path)
	for _, body := range []string{index.Body.String(), detail.Body.String()} {
		if strings.Contains(body, "%25") || strings.Count(body, path) < 1 {
			t.Fatal("persian path missing or double-encoded")
		}
	}
	if !strings.Contains(detail.Body.String(), `href="`+canon+`"`) || catalog.gotSlug != slug {
		t.Fatalf("canonical slug=%q", catalog.gotSlug)
	}
}

func TestApplyPlatformSpecialtyHrefs(t *testing.T) {
	items := []components.SpecialtyCardView{{ID: 2, Name: "داخلی"}, {ID: 3, Name: "خالی"}}
	applyPlatformSpecialtyHrefs(items, []repository.SpecialtyPublicCount{
		{ID: 2, Slug: "داخلی", DoctorCount: 2},
		{ID: 3, Slug: "خالی", DoctorCount: 0},
	})
	if items[0].Href != seo.SpecialtyPath("داخلی") || strings.Contains(items[0].Href, "%25") || items[1].Href != "" {
		t.Fatalf("hrefs = %#v", items)
	}
}

func TestParseSpecialtyPage(t *testing.T) {
	if page, ok := parseSpecialtyPage("", 0, 12); !ok || page != 1 {
		t.Fatalf("empty = %d %v", page, ok)
	}
	if page, ok := parseSpecialtyPage("2", 13, 12); !ok || page != 2 {
		t.Fatalf("page2 = %d %v", page, ok)
	}
	for _, raw := range []string{"0", "-4", "x", "999"} {
		if _, ok := parseSpecialtyPage(raw, 13, 12); ok {
			t.Fatalf("%s should 404", raw)
		}
	}
	if _, ok := parseSpecialtyPage("2", 0, 12); ok {
		t.Fatal("empty listing page 2 should 404")
	}
}

func sampleDoctorCards() []booking.DoctorCard {
	return []booking.DoctorCard{{
		Name:          "رضا",
		SpecialtyName: "داخلی",
		SpecialtySlug: "داخلی",
		BookingURL:    "/booking/center/reza",
	}}
}
