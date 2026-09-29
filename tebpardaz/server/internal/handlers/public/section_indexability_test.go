package public

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type memSections struct {
	rows    map[uint]map[string]*models.AppointmentClinicSection
	doctors map[uint][]models.Doctor
}

func (m *memSections) GetSectionByClinicAndSlug(clinicID uint, slug string) (*models.AppointmentClinicSection, error) {
	if m == nil || m.rows[clinicID] == nil || m.rows[clinicID][slug] == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return m.rows[clinicID][slug], nil
}

func (m *memSections) ListActiveSectionsWithBannerByClinic(clinicID uint) ([]models.AppointmentClinicSection, error) {
	return nil, nil
}

func (m *memSections) ListAssignedPublicDoctors(sectionID uint) ([]models.Doctor, error) {
	if m == nil {
		return nil, nil
	}
	return m.doctors[sectionID], nil
}

func sectionTestRouter(tc *tenant.Context, h *SectionPublicHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(tenant.ContextKey, tc)
		c.Next()
	})
	g := r.Group("/")
	GETAndHEAD(g, "/sections", h.ListSections)
	GETAndHEAD(g, "/section/:slug", h.Get)
	GETAndHEAD(g, "/section/:slug/ساعات-کاری", h.GetWorkingHours)
	GETAndHEAD(g, "/section/:slug/working-hours", h.GetWorkingHours)
	GETAndHEAD(g, "/section/:slug/پیام-به-مراجعین", h.GetMessages)
	GETAndHEAD(g, "/section/:slug/تجهیزات", h.GetEquipment)
	GETAndHEAD(g, "/section/:slug/معرفی", h.GetBannerIntro)
	GETAndHEAD(g, "/clinics/:clinic_slug/section/:slug", h.Get)
	GETAndHEAD(g, "/clinics/:clinic_slug/section/:slug/ساعات-کاری", h.GetWorkingHours)
	GETAndHEAD(g, "/clinics/:clinic_slug/section/:slug/working-hours", h.GetWorkingHours)
	GETAndHEAD(g, "/clinics/:clinic_slug/section/:slug/پیام-به-مراجعین", h.GetMessages)
	GETAndHEAD(g, "/clinics/:clinic_slug/section/:slug/message-to-visitors", h.GetMessages)
	GETAndHEAD(g, "/clinics/:clinic_slug/section/:slug/تجهیزات", h.GetEquipment)
	GETAndHEAD(g, "/clinics/:clinic_slug/section/:slug/equipment", h.GetEquipment)
	GETAndHEAD(g, "/clinics/:clinic_slug/section/:slug/معرفی", h.GetBannerIntro)
	GETAndHEAD(g, "/ساعات-کاری", h.GetWorkingHours)
	GETAndHEAD(g, "/clinics/:clinic_slug/معرفی", h.GetBannerIntro)
	return r
}

func defaultSection(id, clinicID uint, title, slug string) *models.AppointmentClinicSection {
	banner := models.DefaultSectionBanner(id, title)
	sec := &models.AppointmentClinicSection{
		ClinicID: clinicID,
		Title:    title,
		Slug:     slug,
		IsActive: true,
		Banner:   &banner,
	}
	sec.ID = id
	return sec
}

func TestPrimarySectionIndexabilitySSR(t *testing.T) {
	clinicSlug := "چمران-مشهد"
	sectionSlug := "تصویربرداری"
	thinSlug := "آزمایشگاه"
	domain := "chamranclinic.ir"
	orgClinic := models.Clinic{Model: gorm.Model{ID: 4}, Name: "درمانگاه چمران", Slug: &clinicSlug, OrganizationID: 2, IsActiveOnWebsite: true}
	otherSlug := "مرکز-دیگر"
	other := models.Clinic{Model: gorm.Model{ID: 9}, Name: "مرکز دیگر", Slug: &otherSlug, OrganizationID: 3, IsActiveOnWebsite: true, Domain: &domain}
	inactiveSlug := "مرکز-غیرفعال"
	inactive := models.Clinic{Model: gorm.Model{ID: 11}, Name: "غیرفعال", Slug: &inactiveSlug, OrganizationID: 2, IsActiveOnWebsite: false}

	meaningful := defaultSection(21, orgClinic.ID, "تصویربرداری", sectionSlug)
	meaningful.Banner.Description = "ام‌آرآی و سی‌تی در بخش تصویربرداری انجام می‌شود."
	thin := defaultSection(22, orgClinic.ID, "آزمایشگاه", thinSlug)
	ownSection := defaultSection(23, other.ID, "تصویربرداری", sectionSlug)
	ownSection.Banner.Description = "خدمات تصویربرداری مخصوص دامنه مرکز."

	sections := &memSections{
		rows: map[uint]map[string]*models.AppointmentClinicSection{
			orgClinic.ID: {sectionSlug: meaningful, thinSlug: thin},
			other.ID:     {sectionSlug: ownSection},
		},
		doctors: map[uint][]models.Doctor{
			meaningful.ID: {{Name: "دکتر آزمایش", Slug: "دکتر-آزمایش", IsApproved: true, IsActive: true}},
		},
	}
	h := &SectionPublicHandler{Clinics: &memClinics{rows: []models.Clinic{orgClinic, other, inactive}}, Sections: sections}

	platform := &tenant.Context{Layout: constants.LayoutPlatform, Host: "tebpardaz.ir"}
	body := serveSection(t, platform, h, "tebpardaz.ir", "/clinics/"+url.PathEscape(clinicSlug)+"/section/"+url.PathEscape(sectionSlug)+"?utm_source=ads")
	if !strings.Contains(body, `content="index,follow"`) {
		t.Fatalf("platform robots:\n%s", body)
	}
	if strings.Count(body, "<h1") != 1 || !strings.Contains(body, ">تصویربرداری</h1>") {
		t.Fatalf("platform h1 count=%d", strings.Count(body, "<h1"))
	}
	wantCanonical := "https://tebpardaz.ir/clinics/" + clinicSlug + "/section/" + sectionSlug
	if !strings.Contains(body, `rel="canonical" href="`+wantCanonical+`"`) || strings.Contains(body, "%25") || strings.Contains(body, "utm_source") {
		t.Fatalf("platform canonical:\n%s", body)
	}
	if strings.Contains(body, `"@type":"Physician"`) || strings.Contains(body, `#clinic"`) {
		t.Fatalf("platform json-ld identity changed:\n%s", body)
	}

	thinBody := serveSection(t, platform, h, "tebpardaz.ir", "/clinics/"+url.PathEscape(clinicSlug)+"/section/"+url.PathEscape(thinSlug))
	if !strings.Contains(thinBody, `content="noindex,follow"`) || strings.Count(thinBody, "<h1") != 1 || !strings.Contains(thinBody, ">آزمایشگاه</h1>") {
		t.Fatalf("thin section:\n%s", thinBody)
	}
	if !strings.Contains(thinBody, `href="https://tebpardaz.ir/clinics/`+clinicSlug+`/section/`+thinSlug+`"`) {
		t.Fatalf("thin canonical missing")
	}

	orgID := uint(2)
	org := &tenant.Context{Layout: constants.LayoutOrgan, Host: "mehrshafaclinics.ir", OrganizationID: &orgID, Organization: &models.Organization{Name: "مهرشفا"}}
	org.Organization.ID = 2
	orgBody := serveSection(t, org, h, "mehrshafaclinics.ir", "/clinics/"+url.PathEscape(clinicSlug)+"/section/"+url.PathEscape(sectionSlug))
	if !strings.Contains(orgBody, `content="index,follow"`) || strings.Contains(orgBody, "https://tebpardaz.ir/clinics/") || !strings.Contains(orgBody, `https://mehrshafaclinics.ir/clinics/`+clinicSlug+`/section/`+sectionSlug) {
		t.Fatalf("organization canonical:\n%s", orgBody)
	}
	foreign := serveSectionStatus(t, org, h, "mehrshafaclinics.ir", "/clinics/"+url.PathEscape(otherSlug)+"/section/"+url.PathEscape(sectionSlug))
	if foreign.code != http.StatusNotFound || !strings.Contains(foreign.body, "noindex, nofollow") {
		t.Fatalf("foreign org status=%d", foreign.code)
	}

	ownID := other.ID
	own := &tenant.Context{Layout: constants.LayoutPrivate, Host: domain, ClinicID: &ownID, Clinic: &other}
	ownBody := serveSection(t, own, h, domain, "/section/"+url.PathEscape(sectionSlug))
	if !strings.Contains(ownBody, `content="index,follow"`) || !strings.Contains(ownBody, `https://chamranclinic.ir/section/`+sectionSlug) || strings.Contains(ownBody, "/clinics/"+clinicSlug+"/section/") {
		t.Fatalf("own-domain canonical:\n%s", ownBody)
	}
	leaked := serveSectionStatus(t, own, h, domain, "/clinics/"+url.PathEscape(clinicSlug)+"/section/"+url.PathEscape(sectionSlug))
	if leaked.code != http.StatusNotFound || !strings.Contains(leaked.body, "noindex, nofollow") {
		t.Fatalf("own-domain leak status=%d", leaked.code)
	}
	missing := serveSectionStatus(t, platform, h, "tebpardaz.ir", "/clinics/"+url.PathEscape(clinicSlug)+"/section/"+url.PathEscape("ناشناخته"))
	if missing.code != http.StatusNotFound || !strings.Contains(missing.body, "noindex, nofollow") {
		t.Fatalf("unknown section status=%d", missing.code)
	}
	inactiveRes := serveSectionStatus(t, platform, h, "tebpardaz.ir", "/clinics/"+url.PathEscape(inactiveSlug)+"/section/"+url.PathEscape(sectionSlug))
	if inactiveRes.code != http.StatusNotFound || !strings.Contains(inactiveRes.body, "noindex, nofollow") {
		t.Fatalf("inactive clinic status=%d", inactiveRes.code)
	}
}

func TestSupportingSectionSubpagesStayNoindex(t *testing.T) {
	clinicSlug := "چمران-مشهد"
	sectionSlug := "تصویربرداری"
	otherSlug := "مرکز-دیگر"
	domain := "chamranclinic.ir"
	orgClinic := models.Clinic{Model: gorm.Model{ID: 4}, Name: "درمانگاه چمران", Slug: &clinicSlug, OrganizationID: 2, IsActiveOnWebsite: true}
	other := models.Clinic{Model: gorm.Model{ID: 9}, Name: "مرکز دیگر", Slug: &otherSlug, OrganizationID: 3, IsActiveOnWebsite: true, Domain: &domain}
	sec := defaultSection(21, orgClinic.ID, "تصویربرداری", sectionSlug)
	sec.Banner.Description = "ام‌آرآی و سی‌تی در بخش تصویربرداری انجام می‌شود."
	own := defaultSection(23, other.ID, "تصویربرداری", sectionSlug)
	own.Banner.Description = "خدمات تصویربرداری مخصوص دامنه مرکز."
	h := &SectionPublicHandler{
		Clinics: &memClinics{rows: []models.Clinic{orgClinic, other}},
		Sections: &memSections{rows: map[uint]map[string]*models.AppointmentClinicSection{
			orgClinic.ID: {sectionSlug: sec},
			other.ID:     {sectionSlug: own},
		}},
	}
	platform := &tenant.Context{Layout: constants.LayoutPlatform, Host: "tebpardaz.ir"}
	base := "/clinics/" + url.PathEscape(clinicSlug) + "/section/" + url.PathEscape(sectionSlug)
	canonicalBase := "https://tebpardaz.ir/clinics/" + clinicSlug + "/section/" + sectionSlug
	leaves := []struct {
		path string
		leaf string
	}{
		{base + "/" + url.PathEscape("ساعات-کاری"), "/ساعات-کاری"},
		{base + "/working-hours", "/ساعات-کاری"},
		{base + "/" + url.PathEscape("پیام-به-مراجعین"), "/پیام-به-مراجعین"},
		{base + "/message-to-visitors", "/پیام-به-مراجعین"},
		{base + "/" + url.PathEscape("تجهیزات"), "/تجهیزات"},
		{base + "/equipment", "/تجهیزات"},
		{base + "/" + url.PathEscape("معرفی"), "/معرفی"},
	}
	for _, leaf := range leaves {
		body := serveSection(t, platform, h, "tebpardaz.ir", leaf.path+"?utm_source=ads")
		if !strings.Contains(body, `content="noindex,follow"`) || strings.Contains(body, `content="index,follow"`) {
			t.Fatalf("%s robots:\n%s", leaf.path, body)
		}
		if strings.Count(body, "<h1") > 1 {
			t.Fatalf("%s has multiple h1", leaf.path)
		}
		want := canonicalBase + leaf.leaf
		if !strings.Contains(body, `rel="canonical" href="`+want+`"`) || strings.Contains(body, "%25") || strings.Contains(body, "utm_source") || strings.Contains(body, "chamranclinic.ir") {
			t.Fatalf("%s canonical:\n%s", leaf.path, body)
		}
	}

	primary := serveSection(t, platform, h, "tebpardaz.ir", base)
	if !strings.Contains(primary, `content="index,follow"`) || !strings.Contains(primary, ">تصویربرداری</h1>") {
		t.Fatal("primary section robots changed")
	}
	listAlias := serveSection(t, platform, h, "tebpardaz.ir", "/"+url.PathEscape("ساعات-کاری"))
	if !strings.Contains(listAlias, `content="noindex,follow"`) || !strings.Contains(listAlias, `href="https://tebpardaz.ir/ساعات-کاری"`) {
		t.Fatalf("bare hours alias:\n%s", listAlias)
	}
	index := serveSection(t, platform, h, "tebpardaz.ir", "/sections")
	if !strings.Contains(index, `content="index,follow"`) {
		t.Fatal("section index became noindex")
	}

	orgID := uint(2)
	org := &tenant.Context{Layout: constants.LayoutOrgan, Host: "mehrshafaclinics.ir", OrganizationID: &orgID, Organization: &models.Organization{Name: "مهرشفا"}}
	org.Organization.ID = 2
	orgHours := serveSection(t, org, h, "mehrshafaclinics.ir", base+"/"+url.PathEscape("ساعات-کاری"))
	if !strings.Contains(orgHours, `content="noindex,follow"`) || !strings.Contains(orgHours, "https://mehrshafaclinics.ir/clinics/"+clinicSlug+"/section/"+sectionSlug+"/ساعات-کاری") || strings.Contains(orgHours, "https://tebpardaz.ir/clinics/") {
		t.Fatalf("org hours:\n%s", orgHours)
	}
	foreign := serveSectionStatus(t, org, h, "mehrshafaclinics.ir", "/clinics/"+url.PathEscape(otherSlug)+"/section/"+url.PathEscape(sectionSlug)+"/"+url.PathEscape("تجهیزات"))
	if foreign.code != http.StatusNotFound || !strings.Contains(foreign.body, "noindex, nofollow") {
		t.Fatalf("foreign equipment status=%d", foreign.code)
	}
	ownID := other.ID
	ownTC := &tenant.Context{Layout: constants.LayoutPrivate, Host: domain, ClinicID: &ownID, Clinic: &other}
	ownHours := serveSection(t, ownTC, h, domain, "/section/"+url.PathEscape(sectionSlug)+"/"+url.PathEscape("ساعات-کاری"))
	if !strings.Contains(ownHours, `content="noindex,follow"`) || !strings.Contains(ownHours, "https://chamranclinic.ir/section/"+sectionSlug+"/ساعات-کاری") || strings.Contains(ownHours, `href="/clinics/`) {
		t.Fatalf("own hours:\n%s", ownHours)
	}
	leaked := serveSectionStatus(t, ownTC, h, domain, "/clinics/"+url.PathEscape(clinicSlug)+"/section/"+url.PathEscape(sectionSlug)+"/معرفی")
	if leaked.code != http.StatusNotFound || !strings.Contains(leaked.body, "noindex, nofollow") {
		t.Fatalf("own intro leak status=%d", leaked.code)
	}
	missing := serveSectionStatus(t, platform, h, "tebpardaz.ir", base+"-missing/"+url.PathEscape("ساعات-کاری"))
	if missing.code != http.StatusNotFound || !strings.Contains(missing.body, "noindex, nofollow") || strings.Contains(missing.body, "rel=\"canonical\"") {
		t.Fatalf("unknown hours status=%d", missing.code)
	}
}

func TestPrimarySectionSitemapLocFollowsIndexability(t *testing.T) {
	clinicSlug := "چمران-مشهد"
	sectionSlug := "تصویربرداری"
	basePlatform := "https://tebpardaz.ir"
	indexable := primarySectionSitemapLoc(basePlatform, false, clinicSlug, sectionSlug, true)
	want := basePlatform + "/clinics/" + clinicSlug + "/section/" + url.PathEscape(sectionSlug)
	if indexable != want || strings.Contains(indexable, "%25") {
		t.Fatalf("platform loc=%s", indexable)
	}
	if primarySectionSitemapLoc(basePlatform, false, clinicSlug, sectionSlug, false) != "" {
		t.Fatal("thin section stayed in sitemap")
	}
	org := primarySectionSitemapLoc("https://mehrshafaclinics.ir", false, clinicSlug, sectionSlug, true)
	if strings.Contains(org, "tebpardaz.ir") || !strings.HasPrefix(org, "https://mehrshafaclinics.ir/clinics/") {
		t.Fatalf("org loc=%s", org)
	}
	own := primarySectionSitemapLoc("https://chamranclinic.ir", true, clinicSlug, sectionSlug, true)
	if own != "https://chamranclinic.ir/section/"+url.PathEscape(sectionSlug) || strings.Contains(own, "/clinics/") || strings.Contains(own, "%25") {
		t.Fatalf("own loc=%s", own)
	}

	def := models.DefaultSectionBanner(1, "آزمایشگاه")
	thin := models.AppointmentClinicSection{Title: "آزمایشگاه", Slug: "آزمایشگاه", IsActive: true}
	thin.ID = 8
	if sectionFactsIndexable(thin, repository.SectionPublicFacts{Banner: &def}) {
		t.Fatal("default facts were indexable")
	}
	rich := thin
	rich.Slug = sectionSlug
	if !sectionFactsIndexable(rich, repository.SectionPublicFacts{Banner: &def, PublicDoctors: 1}) {
		t.Fatal("doctor facts were not indexable")
	}
	urls := appendPrimarySectionDetail(nil, basePlatform, false, clinicSlug, thin, map[uint]repository.SectionPublicFacts{
		thin.ID: {Banner: &def},
	})
	urls = appendPrimarySectionDetail(urls, basePlatform, false, clinicSlug, rich, map[uint]repository.SectionPublicFacts{
		rich.ID: {Banner: &def, PublicDoctors: 1},
	})
	if len(urls) != 1 || urls[0].Loc != want || strings.Contains(urls[0].Loc, "%25") {
		t.Fatalf("sitemap urls=%#v", urls)
	}
}

type sectionHTTPResult struct {
	code int
	body string
}

func serveSection(t *testing.T, tc *tenant.Context, h *SectionPublicHandler, host, path string) string {
	t.Helper()
	res := serveSectionStatus(t, tc, h, host, path)
	if res.code != http.StatusOK {
		t.Fatalf("%s status=%d body=%s", path, res.code, res.body)
	}
	return res.body
}

func serveSectionStatus(t *testing.T, tc *tenant.Context, h *SectionPublicHandler, host, path string) sectionHTTPResult {
	t.Helper()
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = host
	sectionTestRouter(tc, h).ServeHTTP(res, req)
	return sectionHTTPResult{code: res.Code, body: res.Body.String()}
}
