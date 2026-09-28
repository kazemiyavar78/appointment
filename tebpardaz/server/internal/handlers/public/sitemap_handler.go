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
	Clinics  *repository.ClinicRepo
	Doctors  *repository.DoctorRepo
	Sections *repository.SectionRepo
	News     *repository.NewsRepo
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

// NewSitemapHandler نمونه جدیدی از SitemapHandler را با وابستگی‌های ریپازیتوری ایجاد می‌کند.
// ورودی: ریپازیتوری‌های کلینیک، پزشک، بخش و اخبار.
// خروجی: اشاره‌گر به SitemapHandler.
func NewSitemapHandler(
	clinics *repository.ClinicRepo,
	doctors *repository.DoctorRepo,
	sections *repository.SectionRepo,
	news *repository.NewsRepo,
) *SitemapHandler {
	return &SitemapHandler{
		Clinics:  clinics,
		Doctors:  doctors,
		Sections: sections,
		News:     news,
	}
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
	}

	// اسلاگ‌های ۵ صفحه اختصاصی به صورت URL-Encoded
	encodedWorkingHours := url.PathEscape("ساعات-کاری")
	encodedMessages := url.PathEscape("پیام-به-مراجعین")
	encodedEquipment := url.PathEscape("تجهیزات")
	encodedSchedule := url.PathEscape("برنامه-هفتگی-پزشکان")
	encodedIntro := url.PathEscape("معرفی")

	if tc != nil && tc.Layout == constants.LayoutPrivate && tc.ClinicID != nil {
		// دامنه اختصاصی مرکز (Private Clinic Domain)
		clinicID := *tc.ClinicID

		urls = append(urls,
			SitemapURL{Loc: baseURL + "/sections", ChangeFreq: "weekly", Priority: "0.8"},
			SitemapURL{Loc: fmt.Sprintf("%s/%s", baseURL, encodedWorkingHours), ChangeFreq: "weekly", Priority: "0.8"},
			SitemapURL{Loc: fmt.Sprintf("%s/%s", baseURL, encodedMessages), ChangeFreq: "monthly", Priority: "0.7"},
			SitemapURL{Loc: fmt.Sprintf("%s/%s", baseURL, encodedEquipment), ChangeFreq: "monthly", Priority: "0.8"},
			SitemapURL{Loc: fmt.Sprintf("%s/%s", baseURL, encodedSchedule), ChangeFreq: "daily", Priority: "0.9"},
			SitemapURL{Loc: fmt.Sprintf("%s/%s", baseURL, encodedIntro), ChangeFreq: "monthly", Priority: "0.7"},
		)

		if h.Sections != nil {
			if secList, err := h.Sections.ListActiveSectionsByClinic(clinicID); err == nil {
				for _, s := range secList {
					encodedSecSlug := url.PathEscape(s.Slug)
					urls = append(urls,
						SitemapURL{Loc: fmt.Sprintf("%s/section/%s", baseURL, encodedSecSlug), ChangeFreq: "weekly", Priority: "0.8"},
						SitemapURL{Loc: fmt.Sprintf("%s/section/%s/%s", baseURL, encodedSecSlug, encodedWorkingHours), ChangeFreq: "weekly", Priority: "0.7"},
						SitemapURL{Loc: fmt.Sprintf("%s/section/%s/%s", baseURL, encodedSecSlug, encodedMessages), ChangeFreq: "monthly", Priority: "0.7"},
						SitemapURL{Loc: fmt.Sprintf("%s/section/%s/%s", baseURL, encodedSecSlug, encodedEquipment), ChangeFreq: "monthly", Priority: "0.7"},
						SitemapURL{Loc: fmt.Sprintf("%s/section/%s/%s", baseURL, encodedSecSlug, encodedIntro), ChangeFreq: "monthly", Priority: "0.7"},
					)
				}
			}
		}

		if h.Doctors != nil {
			if docs, err := h.Doctors.ListApprovedByClinic(clinicID); err == nil {
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

		for _, cl := range targetClinics {
			if cl.Slug != nil && *cl.Slug != "" {
				cSlug := *cl.Slug
				urls = append(urls,
					SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/sections", baseURL, cSlug), ChangeFreq: "weekly", Priority: "0.8"},
					SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/%s", baseURL, cSlug, encodedWorkingHours), ChangeFreq: "weekly", Priority: "0.8"},
					SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/%s", baseURL, cSlug, encodedMessages), ChangeFreq: "monthly", Priority: "0.7"},
					SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/%s", baseURL, cSlug, encodedEquipment), ChangeFreq: "monthly", Priority: "0.8"},
					SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/%s", baseURL, cSlug, encodedSchedule), ChangeFreq: "daily", Priority: "0.9"},
					SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/%s", baseURL, cSlug, encodedIntro), ChangeFreq: "monthly", Priority: "0.7"},
				)

				if h.Sections != nil {
					if secList, err := h.Sections.ListActiveSectionsByClinic(cl.ID); err == nil {
						for _, s := range secList {
							encodedSecSlug := url.PathEscape(s.Slug)
							urls = append(urls,
								SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/section/%s", baseURL, cSlug, encodedSecSlug), ChangeFreq: "weekly", Priority: "0.8"},
								SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/section/%s/%s", baseURL, cSlug, encodedSecSlug, encodedWorkingHours), ChangeFreq: "weekly", Priority: "0.7"},
								SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/section/%s/%s", baseURL, cSlug, encodedSecSlug, encodedMessages), ChangeFreq: "monthly", Priority: "0.7"},
								SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/section/%s/%s", baseURL, cSlug, encodedSecSlug, encodedEquipment), ChangeFreq: "monthly", Priority: "0.7"},
							)
						}
					}
				}

				if h.Doctors != nil {
					layout := constants.LayoutPlatform
					if tc != nil && tc.Layout == constants.LayoutOrgan {
						layout = constants.LayoutOrgan
					}
					if docs, err := h.Doctors.ListApprovedByClinic(cl.ID); err == nil {
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
