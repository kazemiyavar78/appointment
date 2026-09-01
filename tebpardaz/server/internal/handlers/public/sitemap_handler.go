package public

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	scheme := "https"
	if c.Request.TLS == nil && !strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	nowStr := time.Now().UTC().Format(time.RFC3339)

	var urls []SitemapURL

	// ۱. صفحات پایه مشترک
	urls = append(urls,
		SitemapURL{Loc: baseURL + "/", LastMod: nowStr, ChangeFreq: "daily", Priority: "1.0"},
		SitemapURL{Loc: baseURL + "/doctors", LastMod: nowStr, ChangeFreq: "daily", Priority: "0.9"},
		SitemapURL{Loc: baseURL + "/weekly-schedule", LastMod: nowStr, ChangeFreq: "daily", Priority: "0.9"},
		SitemapURL{Loc: baseURL + "/news", LastMod: nowStr, ChangeFreq: "weekly", Priority: "0.7"},
	)

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
			SitemapURL{Loc: baseURL + "/sections", LastMod: nowStr, ChangeFreq: "weekly", Priority: "0.8"},
			SitemapURL{Loc: fmt.Sprintf("%s/%s", baseURL, encodedWorkingHours), LastMod: nowStr, ChangeFreq: "weekly", Priority: "0.8"},
			SitemapURL{Loc: fmt.Sprintf("%s/%s", baseURL, encodedMessages), LastMod: nowStr, ChangeFreq: "monthly", Priority: "0.7"},
			SitemapURL{Loc: fmt.Sprintf("%s/%s", baseURL, encodedEquipment), LastMod: nowStr, ChangeFreq: "monthly", Priority: "0.8"},
			SitemapURL{Loc: fmt.Sprintf("%s/%s", baseURL, encodedSchedule), LastMod: nowStr, ChangeFreq: "daily", Priority: "0.9"},
			SitemapURL{Loc: fmt.Sprintf("%s/%s", baseURL, encodedIntro), LastMod: nowStr, ChangeFreq: "monthly", Priority: "0.7"},
		)

		if h.Sections != nil {
			if secList, err := h.Sections.ListActiveSectionsByClinic(clinicID); err == nil {
				for _, s := range secList {
					encodedSecSlug := url.PathEscape(s.Slug)
					urls = append(urls,
						SitemapURL{Loc: fmt.Sprintf("%s/section/%s", baseURL, encodedSecSlug), LastMod: nowStr, ChangeFreq: "weekly", Priority: "0.8"},
						SitemapURL{Loc: fmt.Sprintf("%s/section/%s/%s", baseURL, encodedSecSlug, encodedWorkingHours), LastMod: nowStr, ChangeFreq: "weekly", Priority: "0.7"},
						SitemapURL{Loc: fmt.Sprintf("%s/section/%s/%s", baseURL, encodedSecSlug, encodedMessages), LastMod: nowStr, ChangeFreq: "monthly", Priority: "0.7"},
						SitemapURL{Loc: fmt.Sprintf("%s/section/%s/%s", baseURL, encodedSecSlug, encodedEquipment), LastMod: nowStr, ChangeFreq: "monthly", Priority: "0.7"},
						SitemapURL{Loc: fmt.Sprintf("%s/section/%s/%s", baseURL, encodedSecSlug, encodedIntro), LastMod: nowStr, ChangeFreq: "monthly", Priority: "0.7"},
					)
				}
			}
		}

		if h.Doctors != nil {
			if docs, err := h.Doctors.ListApprovedByClinic(clinicID); err == nil {
				for _, doc := range docs {
					if doc.IsActive && doc.Slug != "" {
						urls = append(urls, SitemapURL{
							Loc:        fmt.Sprintf("%s/booking/%d/%s", baseURL, clinicID, doc.Slug),
							LastMod:    nowStr,
							ChangeFreq: "weekly",
							Priority:   "0.8",
						})
					}
				}
			}
		}
	} else {
		// پلتفرم یا ارگان
		urls = append(urls,
			SitemapURL{Loc: baseURL + "/clinics", LastMod: nowStr, ChangeFreq: "weekly", Priority: "0.8"},
			SitemapURL{Loc: baseURL + "/sections", LastMod: nowStr, ChangeFreq: "weekly", Priority: "0.8"},
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
					SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/sections", baseURL, cSlug), LastMod: nowStr, ChangeFreq: "weekly", Priority: "0.8"},
					SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/%s", baseURL, cSlug, encodedWorkingHours), LastMod: nowStr, ChangeFreq: "weekly", Priority: "0.8"},
					SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/%s", baseURL, cSlug, encodedMessages), LastMod: nowStr, ChangeFreq: "monthly", Priority: "0.7"},
					SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/%s", baseURL, cSlug, encodedEquipment), LastMod: nowStr, ChangeFreq: "monthly", Priority: "0.8"},
					SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/%s", baseURL, cSlug, encodedSchedule), LastMod: nowStr, ChangeFreq: "daily", Priority: "0.9"},
					SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/%s", baseURL, cSlug, encodedIntro), LastMod: nowStr, ChangeFreq: "monthly", Priority: "0.7"},
				)

				if h.Sections != nil {
					if secList, err := h.Sections.ListActiveSectionsByClinic(cl.ID); err == nil {
						for _, s := range secList {
							encodedSecSlug := url.PathEscape(s.Slug)
							urls = append(urls,
								SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/section/%s", baseURL, cSlug, encodedSecSlug), LastMod: nowStr, ChangeFreq: "weekly", Priority: "0.8"},
								SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/section/%s/%s", baseURL, cSlug, encodedSecSlug, encodedWorkingHours), LastMod: nowStr, ChangeFreq: "weekly", Priority: "0.7"},
								SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/section/%s/%s", baseURL, cSlug, encodedSecSlug, encodedMessages), LastMod: nowStr, ChangeFreq: "monthly", Priority: "0.7"},
								SitemapURL{Loc: fmt.Sprintf("%s/clinics/%s/section/%s/%s", baseURL, cSlug, encodedSecSlug, encodedEquipment), LastMod: nowStr, ChangeFreq: "monthly", Priority: "0.7"},
							)
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
					LastMod:    n.UpdatedAt.UTC().Format(time.RFC3339),
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
