package public

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestBuildPlatformAboutSitemapEntry(t *testing.T) {
	entry := buildPlatformAboutSitemapEntry("https://tebpardaz.ir", "2026-03-10T06:00:00Z")

	if entry.Loc != "https://tebpardaz.ir/about" {
		t.Fatalf("Loc = %q, want https://tebpardaz.ir/about", entry.Loc)
	}
	if entry.Priority != "0.8" {
		t.Fatalf("Priority = %q, want 0.8", entry.Priority)
	}
	if entry.ChangeFreq != "monthly" {
		t.Fatalf("ChangeFreq = %q, want monthly", entry.ChangeFreq)
	}
	if entry.LastMod != "2026-03-10T06:00:00Z" {
		t.Fatalf("LastMod = %q, want 2026-03-10T06:00:00Z", entry.LastMod)
	}
}

func TestShouldIncludePlatformAboutSitemap(t *testing.T) {
	tests := []struct {
		name string
		tc   *tenant.Context
		want bool
	}{
		{
			name: "platform domain includes about",
			tc:   &tenant.Context{Layout: constants.LayoutPlatform},
			want: true,
		},
		{
			name: "private clinic excludes about",
			tc:   &tenant.Context{Layout: constants.LayoutPrivate},
			want: false,
		},
		{
			name: "organ domain excludes about",
			tc:   &tenant.Context{Layout: constants.LayoutOrgan},
			want: false,
		},
		{
			name: "nil tenant excludes about",
			tc:   nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldIncludePlatformAboutSitemap(tt.tc); got != tt.want {
				t.Fatalf("shouldIncludePlatformAboutSitemap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildBookingSitemapLoc(t *testing.T) {
	doctorSlug := "دکتر-سيد-حسين-هژبرالساداتي"
	clinicSlug := "chamran-clinic"

	tests := []struct {
		name       string
		layout     constants.LayoutKind
		clinic     *models.Clinic
		doctorSlug string
		wantSuffix string
		wantEmpty  bool
	}{
		{
			name:       "private domain uses doctor slug only",
			layout:     constants.LayoutPrivate,
			clinic:     nil,
			doctorSlug: doctorSlug,
			wantSuffix: "/booking/" + url.PathEscape(doctorSlug),
		},
		{
			name:   "platform uses clinic slug not id",
			layout: constants.LayoutPlatform,
			clinic: &models.Clinic{
				Model: gorm.Model{ID: 9},
				Slug:  strPtr(clinicSlug),
			},
			doctorSlug: doctorSlug,
			wantSuffix: "/booking/" + url.PathEscape(clinicSlug) + "/" + url.PathEscape(doctorSlug),
		},
		{
			name:       "empty doctor slug",
			layout:     constants.LayoutPrivate,
			doctorSlug: "  ",
			wantEmpty:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildBookingSitemapLoc("https://example.test", tt.layout, tt.clinic, tt.doctorSlug)
			if tt.wantEmpty {
				if got != "" {
					t.Fatalf("buildBookingSitemapLoc() = %q, want empty", got)
				}
				return
			}
			if !strings.HasSuffix(got, tt.wantSuffix) {
				t.Fatalf("buildBookingSitemapLoc() = %q, want suffix %q", got, tt.wantSuffix)
			}
			if strings.Contains(got, "/booking/9/") {
				t.Fatalf("buildBookingSitemapLoc() must not contain clinic id: %q", got)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func TestLastModFromBuildInfoOmitsUnknownTime(t *testing.T) {
	if got := lastModFromBuildInfo(nil, false); got != "" {
		t.Fatalf("missing build info lastmod = %q", got)
	}
	invalid := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.time", Value: "not-a-time"}}}
	if got := lastModFromBuildInfo(invalid, true); got != "" {
		t.Fatalf("invalid vcs.time lastmod = %q", got)
	}
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.time", Value: "2026-03-10T06:00:00Z"}}}
	if got := lastModFromBuildInfo(info, true); got != "2026-03-10T06:00:00Z" {
		t.Fatalf("vcs.time lastmod = %q", got)
	}
}

func TestNewsSitemapLastModUsesUpdatedAtOnly(t *testing.T) {
	if got := newsSitemapLastMod(time.Time{}); got != "" {
		t.Fatalf("zero UpdatedAt lastmod = %q", got)
	}
	updated := time.Date(2026, 3, 10, 6, 0, 0, 0, time.UTC)
	if got := newsSitemapLastMod(updated); got != "2026-03-10T06:00:00Z" {
		t.Fatalf("UpdatedAt lastmod = %q", got)
	}
}

// TestIncludeDoctorInBookingSitemap پزشک تخصص مخفی از نوبت‌دهی را از نقشه سایت حذف می‌کند.
func TestPlatformSpecialtySitemapEntries(t *testing.T) {
	updated := time.Date(2026, 3, 10, 6, 0, 0, 0, time.UTC)
	entries := platformSpecialtySitemapEntries("https://tebpardaz.ir", []repository.SpecialtyPublicCount{
		{Name: "قلب و عروق", Slug: "قلب-و-عروق", DoctorCount: 2, UpdatedAt: updated},
		{Name: "خالی", Slug: "خالی", DoctorCount: 0, UpdatedAt: updated},
		{Name: "بدون اسلاگ", Slug: " ", DoctorCount: 4, UpdatedAt: updated},
	})
	if len(entries) != 2 {
		t.Fatalf("entries = %#v", entries)
	}
	if entries[0].Loc != "https://tebpardaz.ir/specialties" || entries[0].LastMod != "" {
		t.Fatalf("index = %#v", entries[0])
	}
	want := seo.AbsoluteURL("https://tebpardaz.ir", seo.SpecialtyPath("قلب-و-عروق"))
	if entries[1].Loc != want || strings.Contains(entries[1].Loc, "?page=") || !strings.HasPrefix(entries[1].Loc, "https://") {
		t.Fatalf("detail = %#v", entries[1])
	}
	if entries[1].LastMod != "2026-03-10T06:00:00Z" {
		t.Fatalf("lastmod = %q", entries[1].LastMod)
	}
}

func TestPlatformClinicSitemapEntries(t *testing.T) {
	updated := time.Date(2026, 4, 2, 8, 0, 0, 0, time.UTC)
	slug := "چمران-مشهد"
	blank := " "
	inactive := false
	entries := platformClinicSitemapEntries("https://tebpardaz.ir", []models.Clinic{
		{Name: "چمران", Slug: &slug, IsActiveOnWebsite: true, Model: gorm.Model{UpdatedAt: updated}},
		{Name: "بدون اسلاگ", IsActiveOnWebsite: true},
		{Name: "خالی", Slug: &blank, IsActiveOnWebsite: true},
		{Name: "خاموش", Slug: &slug, IsActiveOnWebsite: inactive},
	})
	if len(entries) != 2 {
		t.Fatalf("entries = %#v", entries)
	}
	if entries[0].Loc != "https://tebpardaz.ir/clinics" || entries[0].LastMod != "" {
		t.Fatalf("index = %#v", entries[0])
	}
	want := seo.AbsoluteURL("https://tebpardaz.ir", seo.ClinicPath(slug))
	if entries[1].Loc != want || strings.Contains(entries[1].Loc, "%25") || strings.Contains(entries[1].Loc, "?page=") {
		t.Fatalf("detail = %#v", entries[1])
	}
	if entries[1].LastMod != "2026-04-02T08:00:00Z" {
		t.Fatalf("lastmod = %q", entries[1].LastMod)
	}
}

func TestIncludeDoctorInBookingSitemap(t *testing.T) {
	bookable := models.Specialty{Model: gorm.Model{ID: 1}, IsApproved: true, ShowInBooking: true}
	hidden := models.Specialty{Model: gorm.Model{ID: 2}, IsApproved: true, ShowInBooking: false}
	if !includeDoctorInBookingSitemap(models.Doctor{IsActive: true, Slug: "a", Specialty: bookable}) {
		t.Fatal("bookable specialty should be included")
	}
	if includeDoctorInBookingSitemap(models.Doctor{IsActive: true, Slug: "a", Specialty: hidden}) {
		t.Fatal("hidden specialty should be excluded")
	}
	if includeDoctorInBookingSitemap(models.Doctor{IsActive: false, Slug: "a", Specialty: bookable}) {
		t.Fatal("inactive doctor should be excluded")
	}
}

func TestServeSitemapPublicHostsAndLastMod(t *testing.T) {
	trust, err := seo.NewProxyTrust([]string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	SetPublicProxyTrust(trust)
	t.Cleanup(func() { SetPublicProxyTrust(nil) })

	h := NewSitemapHandler(nil, nil, nil, nil, nil)
	gin.SetMode(gin.TestMode)

	platform := serveSitemap(t, h, "tebpardaz.ir", constants.LayoutPlatform, nil)
	assertSitemapHTTPS(t, platform, "tebpardaz.ir")
	if !strings.Contains(platform, "https://tebpardaz.ir/clinics</loc>") {
		t.Fatalf("platform sitemap missing clinic index: %s", platform)
	}
	if strings.Contains(platform, "/clinics?") {
		t.Fatalf("platform sitemap included clinic page query: %s", platform)
	}
	assertNoGeneratedLastMod(t, platform, "https://tebpardaz.ir/about")

	clinicID := uint(9)
	tenantBody := serveSitemap(t, h, "chamranclinic.ir", constants.LayoutPrivate, &clinicID)
	assertSitemapHTTPS(t, tenantBody, "chamranclinic.ir")
	if strings.Contains(platform, "https://tebpardaz.ir/specialties</loc>") == false {
		t.Fatalf("platform sitemap missing specialty index: %s", platform)
	}
	if strings.Contains(platform, "/specialties?") {
		t.Fatalf("platform sitemap included specialty page query: %s", platform)
	}
	if strings.Contains(tenantBody, "/specialties") {
		t.Fatalf("tenant sitemap listed platform specialties: %s", tenantBody)
	}
	if strings.Contains(tenantBody, "tebpardaz.ir") {
		t.Fatalf("tenant sitemap used platform host: %s", tenantBody)
	}
	if strings.Contains(tenantBody, "/clinics</loc>") {
		t.Fatalf("tenant sitemap listed platform clinics: %s", tenantBody)
	}
	organBody := serveSitemap(t, h, "mehrshafaclinics.ir", constants.LayoutOrgan, nil)
	if strings.Contains(organBody, "https://mehrshafaclinics.ir/clinics</loc>") || strings.Contains(organBody, "https://tebpardaz.ir/clinics") {
		t.Fatalf("organ sitemap listed platform clinic landing: %s", organBody)
	}
	if strings.Contains(tenantBody, "<lastmod>") {
		t.Fatalf("tenant sitemap without news must not emit lastmod: %s", tenantBody)
	}
	for _, banned := range []string{"ساعات-کاری", "پیام-به-مراجعین", "تجهیزات", "معرفی", "working-hours", "message-to-visitors", "/equipment", "%25"} {
		if strings.Contains(tenantBody, banned) || strings.Contains(platform, banned) || strings.Contains(organBody, banned) {
			t.Fatalf("sitemap still lists supporting section path %s", banned)
		}
	}
	if !strings.Contains(tenantBody, "https://chamranclinic.ir/sections") || !strings.Contains(tenantBody, url.PathEscape("برنامه-هفتگی-پزشکان")) {
		t.Fatalf("tenant sitemap lost index or weekly schedule: %s", tenantBody)
	}
}

func serveSitemap(t *testing.T, h *SitemapHandler, host string, layout constants.LayoutKind, clinicID *uint) string {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	req.Host = host
	req.RemoteAddr = "127.0.0.1:443"
	req.Header.Set("X-Forwarded-Proto", "https")
	c.Request = req
	c.Set(tenant.ContextKey, &tenant.Context{Layout: layout, Host: host, ClinicID: clinicID})
	h.ServeSitemap(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

func assertSitemapHTTPS(t *testing.T, body, host string) {
	t.Helper()
	if strings.Contains(body, "<loc>http://") {
		t.Fatalf("sitemap loc uses http: %s", body)
	}
	if !strings.Contains(body, "https://"+host+"/") {
		t.Fatalf("sitemap missing https host %s: %s", host, body)
	}
}

func assertNoGeneratedLastMod(t *testing.T, body, allowedLoc string) {
	t.Helper()
	payload := body
	if i := strings.Index(body, "<urlset"); i >= 0 {
		payload = body[i:]
	}
	var set SitemapURLSet
	if err := xml.Unmarshal([]byte(payload), &set); err != nil {
		t.Fatalf("unmarshal sitemap: %v\n%s", err, body)
	}
	allowed := platformAboutLastMod()
	for _, entry := range set.URLs {
		if strings.HasSuffix(entry.Loc, "/clinics") {
			if entry.LastMod != "" {
				t.Fatalf("clinic index lastmod = %q", entry.LastMod)
			}
			continue
		}
		if entry.LastMod == "" {
			continue
		}
		if entry.Loc == allowedLoc && entry.LastMod == allowed {
			continue
		}
		t.Fatalf("unexpected lastmod on %s = %s", entry.Loc, entry.LastMod)
	}
}

func TestOrganizationClinicLandingSitemapIsolation(t *testing.T) {
	slugA := "مشهد-عبدالمطلب"
	slugB := "clinic-b"
	empty := ""
	rows := []models.Clinic{
		{Model: gorm.Model{ID: 1}, Name: "الف", Slug: &slugA, OrganizationID: 11, IsActiveOnWebsite: true},
		{Model: gorm.Model{ID: 2}, Name: "ب", Slug: &slugB, OrganizationID: 22, IsActiveOnWebsite: true},
		{Model: gorm.Model{ID: 3}, Name: "خاموش", Slug: &slugA, OrganizationID: 11, IsActiveOnWebsite: false},
		{Model: gorm.Model{ID: 4}, Name: "بی‌اسلاگ", Slug: &empty, OrganizationID: 11, IsActiveOnWebsite: true},
	}
	got := organizationClinicLandingEntries("https://org-a.example", 11, rows)
	if len(got) != 1 {
		t.Fatalf("entries = %#v", got)
	}
	want := seo.AbsoluteURL("https://org-a.example", seo.ClinicPath(slugA))
	if got[0].Loc != want || strings.Contains(got[0].Loc, "%25") || strings.Contains(got[0].Loc, "?page=") || strings.Contains(got[0].Loc, "clinic-b") {
		t.Fatalf("loc = %s", got[0].Loc)
	}
	if len(organizationClinicLandingEntries("https://tebpardaz.ir", 11, rows)) != 1 {
		t.Fatal("helper changed eligibility by host name")
	}
	platform := platformClinicSitemapEntries("https://tebpardaz.ir", rows)
	foundIndex, foundA, foundB := false, false, false
	for _, entry := range platform {
		switch entry.Loc {
		case "https://tebpardaz.ir/clinics":
			foundIndex = true
		case seo.AbsoluteURL("https://tebpardaz.ir", seo.ClinicPath(slugA)):
			foundA = true
		case seo.AbsoluteURL("https://tebpardaz.ir", seo.ClinicPath(slugB)):
			foundB = true
		}
		if strings.Contains(entry.Loc, "?page=") || entry.Loc == "https://tebpardaz.ir/"+slugA || entry.Loc == "https://tebpardaz.ir/"+slugB {
			t.Fatalf("platform sitemap legacy or page query: %s", entry.Loc)
		}
	}
	if !foundIndex || !foundA || !foundB {
		t.Fatalf("platform sitemap index=%v a=%v b=%v entries=%#v", foundIndex, foundA, foundB, platform)
	}
}

func TestIncludeDoctorInBookingSitemapRejectsNonPublic(t *testing.T) {
	doc := models.Doctor{
		IsActive:  true,
		Slug:      "reza",
		Specialty: models.Specialty{Model: gorm.Model{ID: 1}, IsApproved: true, ShowInBooking: true},
	}
	if !includeDoctorInBookingSitemap(doc) {
		t.Fatal("public doctor excluded")
	}
	doc.IsActive = false
	if includeDoctorInBookingSitemap(doc) {
		t.Fatal("inactive doctor included")
	}
	doc.IsActive = true
	doc.Specialty.ShowInBooking = false
	if includeDoctorInBookingSitemap(doc) {
		t.Fatal("hidden specialty included")
	}
}
