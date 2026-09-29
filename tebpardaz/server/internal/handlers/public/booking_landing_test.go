package public

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// stubBookingDoctors نتیجه GetPublicByClinicAndSlug را بدون دیتابیس برمی‌گرداند.
type stubBookingDoctors struct {
	doc  *models.Doctor
	miss bool
}

func (s stubBookingDoctors) GetPublicByClinicAndSlug(clinicID uint, slug string) (*models.Doctor, error) {
	if s.miss || s.doc == nil || clinicID == 0 || s.doc.Slug != slug {
		return nil, gorm.ErrRecordNotFound
	}
	return s.doc, nil
}

func (s stubBookingDoctors) EnsureSlug(doctor *models.Doctor) error { return nil }

// TestBookingLandingSSR محتوای اولیه، متادیتا و ۴۰۴ صفحه رزرو را بدون جاوااسکریپت می‌خواند.
func TestBookingLandingSSR(t *testing.T) {
	gin.SetMode(gin.TestMode)
	slug := "center"
	domain := "chamranclinic.ir"
	clinic := organClinic(9, 4, slug, domain)
	spec := "dakheli"
	doctor := &models.Doctor{
		Name:           "آقای جواد منزه",
		Slug:           "آقاي-جواد-منزه",
		ExternalID:     "ext-secret",
		NationalID:     "0011223344",
		Mobile:         "09121112233",
		DoctorSystemID: 445566,
		IsApproved:     true,
		IsActive:       true,
		Photo1200:      "/uploads/javad.jpg",
		Specialty:      models.Specialty{Name: "داخلی", Slug: spec, IsApproved: true, ShowInBooking: true},
	}
	doctor.ID = 15
	escaped := url.PathEscape(doctor.Slug)
	canonPath := "https://tebpardaz.ir/booking/center/" + doctor.Slug

	h := &BookingHandler{Doctors: stubBookingDoctors{doc: doctor}, Clinics: &memClinics{rows: []models.Clinic{*clinic}}}
	body := serveBooking(t, h, platformTenant(), http.MethodGet, "/booking/center/"+escaped+"?return=/doctors&utm_source=ads")
	if strings.Count(body, "<h1") != 1 || !strings.Contains(body, "<h1 class=\"bk-doctor-card__name\">آقای جواد منزه</h1>") {
		t.Fatalf("h1:\n%s", body)
	}
	if !strings.Contains(body, "داخلی") || !strings.Contains(body, "درمانگاه چمران") {
		t.Fatalf("identity text missing")
	}
	if !strings.Contains(body, `href="/specialties/dakheli"`) || !strings.Contains(body, `href="/clinics/center"`) {
		t.Fatalf("links:\n%s", body)
	}
	if !strings.Contains(body, `alt="آقای جواد منزه"`) || !strings.Contains(body, `src="/uploads/javad.jpg"`) {
		t.Fatalf("image:\n%s", body)
	}
	if !strings.Contains(body, `<title>آقای جواد منزه | داخلی | نوبت‌دهی آنلاین</title>`) {
		t.Fatalf("title:\n%s", body)
	}
	if !strings.Contains(body, "مشاهده اطلاعات و نوبت‌های آقای جواد منزه، داخلی در درمانگاه چمران") {
		t.Fatalf("description:\n%s", body)
	}
	if href := canonicalHref(t, body); href != canonPath || strings.Contains(href, "?") || strings.Contains(href, "%") {
		t.Fatalf("canonical = %q", href)
	}
	if strings.Contains(body, "%25") {
		t.Fatalf("double encoding:\n%s", body)
	}
	if !strings.Contains(body, "index,follow") {
		t.Fatalf("robots")
	}
	script := jsonLDScript(t, body)
	if !strings.Contains(script, canonPath+"#physician") || !strings.Contains(script, canonPath+"#webpage") || !strings.Contains(script, "https://tebpardaz.ir/clinics/center#clinic") || strings.Contains(script, "chamranclinic.ir") {
		t.Fatalf("graph:\n%s", script)
	}
	assertBookingPrivateAbsent(t, body)

	orgBody := serveBooking(t, h, organTenant(4, ""), http.MethodGet, "/booking/center/"+escaped)
	if strings.Contains(orgBody, "/specialties/") || !strings.Contains(orgBody, `href="/clinics/center"`) {
		t.Fatalf("org links:\n%s", orgBody)
	}
	orgScript := jsonLDScript(t, orgBody)
	orgCanon := "https://mehrshafaclinics.ir/booking/center/" + doctor.Slug
	if !strings.Contains(orgScript, orgCanon+"#physician") || !strings.Contains(orgScript, "https://mehrshafaclinics.ir/clinics/center#clinic") || strings.Contains(orgScript, "chamranclinic.ir") {
		t.Fatalf("org graph:\n%s", orgScript)
	}

	own := &tenant.Context{Layout: constants.LayoutPrivate, Host: domain, ClinicID: &clinic.ID, Clinic: clinic}
	ownBody := serveBooking(t, h, own, http.MethodGet, "/booking/"+escaped)
	if strings.Contains(ownBody, `href="/specialties/`) || strings.Contains(ownBody, `href="/clinics/`) || !strings.Contains(ownBody, `class="bk-doctor-card__clinic" href="/"`) {
		t.Fatalf("own links:\n%s", ownBody)
	}
	ownCanon := "https://chamranclinic.ir/booking/" + doctor.Slug
	ownScript := jsonLDScript(t, ownBody)
	if !strings.Contains(ownScript, ownCanon+"#physician") || !strings.Contains(ownScript, "https://chamranclinic.ir/#clinic") || strings.Contains(ownScript, "tebpardaz.ir/booking") {
		t.Fatalf("own graph:\n%s", ownScript)
	}

	head := serveBookingRaw(t, h, platformTenant(), http.MethodHead, "/booking/center/"+escaped)
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Location") != "" {
		t.Fatalf("head status=%d len=%d", head.Code, head.Body.Len())
	}

	for _, name := range []string{"unknown", "inactive", "unapproved", "empty external_id", "hidden specialty"} {
		t.Run(name, func(t *testing.T) {
			miss := &BookingHandler{Doctors: stubBookingDoctors{miss: true}, Clinics: h.Clinics}
			res := serveBookingRaw(t, miss, platformTenant(), http.MethodGet, "/booking/center/"+escaped)
			assertBookingNotFound(t, res)
		})
	}

	inactive := organClinic(9, 4, slug, domain)
	inactive.IsActiveOnWebsite = false
	inactiveH := &BookingHandler{Doctors: h.Doctors, Clinics: &memClinics{rows: []models.Clinic{*inactive}}}
	assertBookingNotFound(t, serveBookingRaw(t, inactiveH, platformTenant(), http.MethodGet, "/booking/center/"+escaped))

	off := *clinic
	off.IsActiveOnWebsite = false
	ownOff := &tenant.Context{Layout: constants.LayoutPrivate, Host: domain, ClinicID: &off.ID, Clinic: &off}
	assertBookingNotFound(t, serveBookingRaw(t, h, ownOff, http.MethodGet, "/booking/"+escaped))
}

func platformTenant() *tenant.Context {
	return &tenant.Context{Layout: constants.LayoutPlatform, Host: "tebpardaz.ir"}
}

func serveBooking(t *testing.T, h *BookingHandler, tc *tenant.Context, method, path string) string {
	t.Helper()
	res := serveBookingRaw(t, h, tc, method, path)
	if res.Code != http.StatusOK {
		t.Fatalf("%s %s status=%d\n%s", method, path, res.Code, res.Body.String())
	}
	return res.Body.String()
}

func serveBookingRaw(t *testing.T, h *BookingHandler, tc *tenant.Context, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(tenant.ContextKey, tc)
		c.Next()
	})
	GETAndHEAD(r.Group("/"), "/booking/*path", h.Get)
	res := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	req.Host = tc.Host
	r.ServeHTTP(res, req)
	return res
}

func assertBookingNotFound(t *testing.T, res *httptest.ResponseRecorder) {
	t.Helper()
	body := res.Body.String()
	if res.Code != http.StatusNotFound || !strings.Contains(body, "noindex, nofollow") || strings.Contains(body, "rel=\"canonical\"") || strings.Contains(body, "application/ld+json") || strings.Contains(body, "#physician") {
		t.Fatalf("status=%d body=%s", res.Code, body)
	}
}

func canonicalHref(t *testing.T, body string) string {
	t.Helper()
	const key = `rel="canonical" href="`
	start := strings.Index(body, key)
	if start < 0 {
		t.Fatalf("canonical missing")
	}
	start += len(key)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		t.Fatal("canonical unclosed")
	}
	return body[start : start+end]
}

func assertBookingPrivateAbsent(t *testing.T, body string) {
	t.Helper()
	for _, banned := range []string{"0011223344", "09121112233", "445566", "ext-secret"} {
		if strings.Contains(body, banned) {
			t.Fatalf("private %s leaked", banned)
		}
	}
}
