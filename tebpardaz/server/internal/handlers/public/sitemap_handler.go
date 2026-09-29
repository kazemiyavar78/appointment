package public

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// SitemapURL یک ورودی منفرد در فایل sitemap.xml است.
type SitemapURL struct {
	XMLName    xml.Name `xml:"url"`
	Loc        string   `xml:"loc"`
	LastMod    string   `xml:"lastmod,omitempty"`
	ChangeFreq string   `xml:"changefreq,omitempty"`
	Priority   string   `xml:"priority,omitempty"`
}

// SitemapURLSet ریشه ساختار sitemap.xml استاندارد است.
type SitemapURLSet struct {
	XMLName xml.Name     `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	URLs    []SitemapURL `xml:"url"`
}

// SitemapHandler تولیدکننده داینامیک نقشه سایت (sitemap.xml) به تفکیک هر دامنه است.
type SitemapHandler struct {
	Clinics     *repository.ClinicRepo
	Doctors     *repository.DoctorRepo
	Sections    *repository.SectionRepo
	News        *repository.NewsRepo
	Specialties *repository.SpecialtyRepo
}

// buildPlatformAboutSitemapEntry یک ورودی sitemap مخصوص صفحه «درباره ما» دامنه tebpardaz.ir می‌سازد.
// ورودی: baseURL و lastMod.
// خروجی: SitemapURL با مسیر /about، جدا از صفحات درباره کلینیک‌های مشتری.
func buildPlatformAboutSitemapEntry(baseURL, lastMod string) SitemapURL {
	return SitemapURL{
		Loc:        baseURL + "/about",
		LastMod:    lastMod,
		ChangeFreq: "monthly",
		Priority:   "0.8",
	}
}

// shouldIncludePlatformAboutSitemap مشخص می‌کند آیا entry صفحه درباره پلتفرم باید در sitemap باشد.
// ورودی: کانتکست tenant.
// خروجی: true فقط برای دامنه اصلی tebpardaz.ir (LayoutPlatform).
func shouldIncludePlatformAboutSitemap(tc *tenant.Context) bool {
	return tc != nil && tc.Layout == constants.LayoutPlatform
}

// buildBookingSitemapLoc URL مطلق صفحه رزرو پزشک را برای sitemap با segmentهای URL-encoded می‌سازد.
// ورودی: baseURL، نوع layout، مرکز (برای پلتفرم/ارگان)، اسلاگ پزشک.
// خروجی: URL مطلق یا رشته خالی اگر مسیر رزرو معتبر نباشد.
func buildBookingSitemapLoc(baseURL string, layout constants.LayoutKind, clinic *models.Clinic, doctorSlug string) string {
	doctorSlug = strings.TrimSpace(doctorSlug)
	if doctorSlug == "" {
		return ""
	}
	clinicKey := ""
	if layout != constants.LayoutPrivate {
		clinicKey = booking.ClinicPathKey(clinic)
		if clinicKey == "" {
			return ""
		}
	}
	relative := booking.BuildBookingURL(layout, clinicKey, doctorSlug)
	rest := strings.TrimPrefix(relative, "/booking/")
	if rest == "" || rest == relative {
		return ""
	}
	parts := strings.Split(rest, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return baseURL + "/booking/" + strings.Join(parts, "/")
}

// includeDoctorInBookingSitemap لینک رزرو پزشک را فقط برای تخصص‌های قابل‌نمایش در نوبت‌دهی نگه می‌دارد.
// ورودی: مدل پزشک با تخصص پیش‌بارگذاری‌شده. خروجی: true اگر پزشک فعال، دارای اسلاگ و تخصص قابل رزرو باشد.
func includeDoctorInBookingSitemap(doc models.Doctor) bool {
	return doc.IsActive && strings.TrimSpace(doc.Slug) != "" && doc.Specialty.IsVisibleInBooking()
}

// platformAboutLastMod زمان build را از vcs.time برمی‌گرداند.
// ورودی: ندارد. خروجی: RFC3339، یا رشته خالی اگر زمان build معتبر نباشد.
// زمان تولید sitemap جایگزین lastmod نمی‌شود.
func platformAboutLastMod() string {
	info, ok := debug.ReadBuildInfo()
	return lastModFromBuildInfo(info, ok)
}

// lastModFromBuildInfo مقدار vcs.time را به lastmod تبدیل می‌کند.
// ورودی: اطلاعات بیلد و ok حاصل ReadBuildInfo. خروجی: RFC3339 یا رشته خالی.
func lastModFromBuildInfo(info *debug.BuildInfo, ok bool) string {
	if !ok || info == nil {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key != "vcs.time" || s.Value == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339Nano, s.Value); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
		if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

// newsSitemapLastMod زمان واقعی ویرایش خبر را برای lastmod برمی‌گرداند.
// ورودی: زمان UpdatedAt. خروجی: RFC3339، یا خالی اگر زمان صفر باشد.
func newsSitemapLastMod(updatedAt time.Time) string {
	if updatedAt.IsZero() {
		return ""
	}
	return updatedAt.UTC().Format(time.RFC3339)
}

// sectionSitemapItem صفحهٔ اصلی یک بخش را تا مشخص شدن ایندکس‌پذیری نگه می‌دارد.
type sectionSitemapItem struct {
	ClinicSlug string
	Section    models.AppointmentClinicSection
}

// sectionFactsByID سیگنال ایندکس بخش‌ها را با یک بار خواندن دسته‌ای می‌گیرد.
// ورودی: بخش‌های همان sitemap. خروجی: نقشهٔ sectionID. خطا یعنی هیچ صفحهٔ اصلی ایندکس نشود.
// به ازای هر بخش query جدا زده نمی‌شود.
func (h *SitemapHandler) sectionFactsByID(sections []models.AppointmentClinicSection) map[uint]repository.SectionPublicFacts {
	out := map[uint]repository.SectionPublicFacts{}
	if h == nil || h.Sections == nil || len(sections) == 0 {
		return out
	}
	ids := make([]uint, 0, len(sections))
	seen := map[uint]struct{}{}
	for _, sec := range sections {
		if _, ok := seen[sec.ID]; ok {
			continue
		}
		seen[sec.ID] = struct{}{}
		ids = append(ids, sec.ID)
	}
	loaded, err := h.Sections.LoadSectionPublicFacts(ids)
	if err != nil || loaded == nil {
		return out
	}
	return loaded
}

// primarySectionSitemapLoc آدرس صفحهٔ اصلی بخش را فقط وقتی ایندکس‌پذیر است می‌سازد.
// ورودی: مبدأ، سطح دامنهٔ اختصاصی، اسلاگ مرکز، اسلاگ بخش و نتیجهٔ SectionDetailIndexable.
// خروجی: URL مطلق یا خالی. اسلاگ بخش یک‌بار PathEscape می‌شود.
func primarySectionSitemapLoc(baseURL string, private bool, clinicSlug, sectionSlug string, indexable bool) string {
	sectionSlug = strings.TrimSpace(sectionSlug)
	if !indexable || sectionSlug == "" {
		return ""
	}
	encoded := url.PathEscape(sectionSlug)
	if private {
		return baseURL + "/section/" + encoded
	}
	clinicSlug = strings.TrimSpace(clinicSlug)
	if clinicSlug == "" {
		return ""
	}
	return baseURL + "/clinics/" + clinicSlug + "/section/" + encoded
}

// appendPrimarySectionDetail صفحهٔ اصلی ایندکس‌پذیر را به sitemap اضافه می‌کند.
// ورودی: فهرست فعلی، مبدأ، سطح، اسلاگ مرکز، بخش و سیگنال‌ها. خروجی: فهرست با حداکثر یک loc تازه.
// زیرصفحه‌ها را عوض نمی‌کند. سیگنال غایب یعنی noindex و بنابراین loc ساخته نمی‌شود.
func appendPrimarySectionDetail(urls []SitemapURL, baseURL string, private bool, clinicSlug string, sec models.AppointmentClinicSection, facts map[uint]repository.SectionPublicFacts) []SitemapURL {
	loc := primarySectionSitemapLoc(baseURL, private, clinicSlug, sec.Slug, sectionFactsIndexable(sec, facts[sec.ID]))
	if loc == "" {
		return urls
	}
	return append(urls, SitemapURL{Loc: loc, ChangeFreq: "weekly", Priority: "0.8"})
}

// sectionFactsIndexable همان SectionDetailIndexable صفحه را برای یک ردیف sitemap حساب می‌کند.
// ورودی: بخش و سیگنال دسته‌ای آن. خروجی: true فقط با اسلاگ غیرخالی و محتوای عمومی متمایز.
func sectionFactsIndexable(sec models.AppointmentClinicSection, facts repository.SectionPublicFacts) bool {
	if strings.TrimSpace(sec.Slug) == "" {
		return false
	}
	copySec := sec
	copySec.Banner = facts.Banner
	copySec.Equipment = nil
	copySec.Messages = nil
	for _, title := range facts.EquipmentTitles {
		copySec.Equipment = append(copySec.Equipment, models.SectionEquipment{Title: title, IsActive: true})
	}
	for _, body := range facts.MessageBodies {
		copySec.Messages = append(copySec.Messages, models.SectionMessage{Content: body, IsActive: true})
	}
	return seo.SectionDetailIndexable(sectionDetailContent(&copySec, facts.PublicDoctors, facts.CatalogServices))
}

// NewSitemapHandler نمونه جدیدی از SitemapHandler را با وابستگی‌های ریپازیتوری ایجاد می‌کند.
// ورودی: ریپازیتوری‌های کلینیک، پزشک، بخش و اخبار.
// خروجی: اشاره‌گر به SitemapHandler.
func NewSitemapHandler(
	clinics *repository.ClinicRepo,
	doctors *repository.DoctorRepo,
	sections *repository.SectionRepo,
	news *repository.NewsRepo,
	specialties *repository.SpecialtyRepo,
) *SitemapHandler {
	return &SitemapHandler{
		Clinics:     clinics,
		Doctors:     doctors,
		Sections:    sections,
		News:        news,
		Specialties: specialties,
	}
}

// platformSpecialtySitemapEntries آدرس‌های تخصص پلتفرم را برای sitemap می‌سازد.
// ورودی: مبدأ و تخصص‌های دارای پزشک. خروجی: /specialties و detailها. صفحهٔ query وارد نمی‌شود.
func platformSpecialtySitemapEntries(baseURL string, rows []repository.SpecialtyPublicCount) []SitemapURL {
	out := []SitemapURL{{
		Loc:        baseURL + "/specialties",
		ChangeFreq: "weekly",
		Priority:   "0.8",
	}}
	for _, row := range rows {
		if !seo.SpecialtyIndexable(row.Slug, row.DoctorCount) {
			continue
		}
		loc := seo.AbsoluteURL(baseURL, seo.SpecialtyPath(row.Slug))
		if loc == "" || strings.Contains(loc, "?page=") {
			continue
		}
		out = append(out, SitemapURL{
			Loc:        loc,
			LastMod:    newsSitemapLastMod(row.UpdatedAt),
			ChangeFreq: "weekly",
			Priority:   "0.7",
		})
	}
	return out
}

// platformClinicSitemapEntries آدرس‌های مرکز پلتفرم را برای sitemap می‌سازد.
// ورودی: مبدأ و مراکز عمومی. خروجی: /clinics و detailهای دارای slug. صفحهٔ query وارد نمی‌شود.
func platformClinicSitemapEntries(baseURL string, rows []models.Clinic) []SitemapURL {
	out := []SitemapURL{{
		Loc:        baseURL + "/clinics",
		ChangeFreq: "weekly",
		Priority:   "0.8",
	}}
	for i := range rows {
		row := &rows[i]
		if !seo.ClinicIndexable(row.IsActiveOnWebsite, row.Slug) {
			continue
		}
		loc := seo.AbsoluteURL(baseURL, seo.ClinicPath(*row.Slug))
		if loc == "" || strings.Contains(loc, "?page=") {
			continue
		}
		out = append(out, SitemapURL{
			Loc:        loc,
			LastMod:    newsSitemapLastMod(row.UpdatedAt),
			ChangeFreq: "weekly",
			Priority:   "0.7",
		})
	}
	return out
}

// organizationClinicLandingEntries لندینگ مراکز همان سازمان را برای sitemap می‌سازد.
// ورودی: مبدأ، شناسه سازمان و ردیف‌ها. خروجی: URLهای /clinics/{slug} بدون page.
// مرکز سازمان دیگر، مرکز غیرفعال و slug خالی حذف می‌شوند. فهرست /clinics اضافه نمی‌شود.
func organizationClinicLandingEntries(baseURL string, organizationID uint, rows []models.Clinic) []SitemapURL {
	out := make([]SitemapURL, 0)
	for i := range rows {
		row := &rows[i]
		if !tenant.ClinicVisibleOnOrganization(row, organizationID) || !seo.ClinicIndexable(row.IsActiveOnWebsite, row.Slug) {
			continue
		}
		loc := seo.AbsoluteURL(baseURL, seo.ClinicPath(*row.Slug))
		if loc == "" || strings.Contains(loc, "?page=") {
			continue
		}
		out = append(out, SitemapURL{
			Loc:        loc,
			LastMod:    newsSitemapLastMod(row.UpdatedAt),
			ChangeFreq: "weekly",
			Priority:   "0.7",
		})
	}
	return out
}

// platformClinicRows مراکز فعال روی وب را برای sitemap پلتفرم می‌خواند.
// ورودی: handler. خروجی: ردیف‌ها. خطا یعنی فقط فهرست /clinics می‌ماند.
func platformClinicRows(h *SitemapHandler) []models.Clinic {
	if h == nil || h.Clinics == nil {
		return nil
	}
	rows, err := h.Clinics.ListAll()
	if err != nil {
		return nil
	}
	return rows
}

// platformSpecialtyCounts تخصص‌های دارای پزشک عمومی را برای sitemap پلتفرم می‌خواند.
// ورودی: handler و context. خروجی: ردیف‌ها. خطا یعنی detail ساخته نمی‌شود.
func platformSpecialtyCounts(h *SitemapHandler) []repository.SpecialtyPublicCount {
	if h == nil || h.Specialties == nil || h.Clinics == nil {
		return nil
	}
	clinics, err := h.Clinics.ListAll()
	if err != nil {
		return nil
	}
	ids := make([]uint, 0, len(clinics))
	for _, clinic := range clinics {
		ids = append(ids, clinic.ID)
	}
	rows, err := h.Specialties.ListWithPublicDoctors(ids)
	if err != nil {
		return nil
	}
	return rows
}

// ServeSitemap نقشه سایت XML را به صورت داینامیک بر اساس دامنه و مستأجر فعال تولید و ارسال می‌کند.
// ورودی: کانتکست Gin حاوی دامنه و اطلاعات Tenant.
// خروجی: محتوای XML با سرلوحه Content-Type مناسب application/xml.
func (h *SitemapHandler) ServeSitemap(c *gin.Context) {
	tc, _ := tenant.FromGin(c)
	baseURL := publicBaseURL(c)

	var urls []SitemapURL

	// ۱. صفحات پایه مشترک
	urls = append(urls,
		SitemapURL{Loc: baseURL + "/", ChangeFreq: "daily", Priority: "1.0"},
		SitemapURL{Loc: baseURL + "/doctors", ChangeFreq: "daily", Priority: "0.9"},
		SitemapURL{Loc: baseURL + "/weekly-schedule", ChangeFreq: "daily", Priority: "0.9"},
		SitemapURL{Loc: baseURL + "/news", ChangeFreq: "weekly", Priority: "0.7"},
	)

	// صفحه «درباره ما» فقط برای دامنه پلتفرم (tebpardaz.ir) — جدا از about کلینیک‌های مشتری
	if shouldIncludePlatformAboutSitemap(tc) {
		urls = append(urls, buildPlatformAboutSitemapEntry(baseURL, platformAboutLastMod()))
		urls = append(urls, platformSpecialtySitemapEntries(baseURL, platformSpecialtyCounts(h))...)
		urls = append(urls, platformClinicSitemapEntries(baseURL, platformClinicRows(h))...)
	}

	// اسلاگ‌های ۵ صفحه اختصاصی به صورت URL-Encoded
	encodedSchedule := url.PathEscape("برنامه-هفتگی-پزشکان")

	if tc != nil && tc.Layout == constants.LayoutPrivate && tc.ClinicID != nil {
		// دامنه اختصاصی مرکز (Private Clinic Domain)
		clinicID := *tc.ClinicID

		urls = append(urls,
			SitemapURL{Loc: baseURL + "/sections", ChangeFreq: "weekly", Priority: "0.8"},
			SitemapURL{Loc: fmt.Sprintf("%s/%s", baseURL, encodedSchedule), ChangeFreq: "daily", Priority: "0.9"},
		)

		if h.Sections != nil {
			if secList, err := h.Sections.ListActiveSectionsByClinic(clinicID); err == nil {
				facts := h.sectionFactsByID(secList)
				for _, s := range secList {
					urls = appendPrimarySectionDetail(urls, baseURL, true, "", s, facts)
				}
			}
		}

		if h.Doctors != nil {
			if docs, err := h.Doctors.ListPublic(repository.DoctorPublicFilter{ClinicIDs: []uint{clinicID}}); err == nil {
				for _, doc := range docs {
					if includeDoctorInBookingSitemap(doc) {
						if loc := buildBookingSitemapLoc(baseURL, constants.LayoutPrivate, nil, doc.Slug); loc != "" {
							urls = append(urls, SitemapURL{
								Loc:        loc,
								ChangeFreq: "weekly",
								Priority:   "0.8",
							})
						}
					}
				}
			}
		}
	} else {
		// پلتفرم یا ارگان
		urls = append(urls,
			SitemapURL{Loc: baseURL + "/sections", ChangeFreq: "weekly", Priority: "0.8"},
		)

		var targetClinics []models.Clinic
		if tc != nil && tc.Layout == constants.LayoutOrgan && tc.OrganizationID != nil && h.Clinics != nil {
			if list, err := h.Clinics.ListByOrganizationID(*tc.OrganizationID); err == nil {
				targetClinics = list
			}
		} else if h.Clinics != nil {
			if list, err := h.Clinics.ListAll(); err == nil {
				targetClinics = list
			}
		}

		if tc != nil && tc.Layout == constants.LayoutOrgan && tc.OrganizationID != nil {
			urls = append(urls, organizationClinicLandingEntries(baseURL, *tc.OrganizationID, targetClinics)...)
		}

		var pendingSectionDetails []sectionSitemapItem
		for _, cl := range targetClinics {
			if cl.Slug != nil && *cl.Slug != "" {
				cSlug := *cl.Slug
				urls = append(urls,
					SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/sections", baseURL, cSlug), ChangeFreq: "weekly", Priority: "0.8"},
					SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/%s", baseURL, cSlug, encodedSchedule), ChangeFreq: "daily", Priority: "0.9"},
				)

				if h.Sections != nil {
					if secList, err := h.Sections.ListActiveSectionsByClinic(cl.ID); err == nil {
						for _, s := range secList {
							pendingSectionDetails = append(pendingSectionDetails, sectionSitemapItem{ClinicSlug: cSlug, Section: s})
						}
					}
				}

				if h.Doctors != nil {
					layout := constants.LayoutPlatform
					if tc != nil && tc.Layout == constants.LayoutOrgan {
						layout = constants.LayoutOrgan
					}
					if docs, err := h.Doctors.ListPublic(repository.DoctorPublicFilter{ClinicIDs: []uint{cl.ID}}); err == nil {
						for _, doc := range docs {
							if includeDoctorInBookingSitemap(doc) {
								if loc := buildBookingSitemapLoc(baseURL, layout, &cl, doc.Slug); loc != "" {
									urls = append(urls, SitemapURL{
										Loc:        loc,
										ChangeFreq: "weekly",
										Priority:   "0.8",
									})
								}
							}
						}
					}
				}
			}
		}
		if len(pendingSectionDetails) > 0 {
			sections := make([]models.AppointmentClinicSection, len(pendingSectionDetails))
			for i, item := range pendingSectionDetails {
				sections[i] = item.Section
			}
			facts := h.sectionFactsByID(sections)
			for _, item := range pendingSectionDetails {
				urls = appendPrimarySectionDetail(urls, baseURL, false, item.ClinicSlug, item.Section, facts)
			}
		}
	}

	// اخبار منتشرشده
	if h.News != nil {
		var clinicIDs []uint
		if tc != nil && tc.ClinicID != nil {
			clinicIDs = []uint{*tc.ClinicID}
		}
		if newsList, err := h.News.ListPublishedByClinicIDs(clinicIDs, 100); err == nil {
			for _, n := range newsList {
				urls = append(urls, SitemapURL{
					Loc:        fmt.Sprintf("%s/news/%d", baseURL, n.ID),
					LastMod:    newsSitemapLastMod(n.UpdatedAt),
					ChangeFreq: "monthly",
					Priority:   "0.6",
				})
			}
		}
	}

	urlset := SitemapURLSet{URLs: urls}
	output, err := xml.MarshalIndent(urlset, "", "  ")
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.String(http.StatusOK, xml.Header+string(output))
}
