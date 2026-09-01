package public

import (
	"net/http"
	"strconv"
	"strings"

	"tebpardaz/server/internal/csrf"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/components"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// StaticPageHandler صفحات درباره ما، تماس و قوانین را سرو می‌کند.
type StaticPageHandler struct {
	Clinics *repository.ClinicRepo
	Reviews *repository.ReviewRepo
	CSRF    *csrf.Manager
}

// NewStaticPageHandler سازنده StaticPageHandler است.
func NewStaticPageHandler(clinics *repository.ClinicRepo, reviews *repository.ReviewRepo, csrfMgr *csrf.Manager) *StaticPageHandler {
	return &StaticPageHandler{Clinics: clinics, Reviews: reviews, CSRF: csrfMgr}
}

// About صفحه درباره ما را رندر می‌کند (برای مرکز خصوصی نظرات مرکز هم هست).
func (h *StaticPageHandler) About(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	view := pages.StaticPageView{
		Title: "درباره ما",
		Lead:  "آشنایی با مرکز و خدمات نوبت‌دهی آنلاین.",
	}
	switch tc.Layout {
	case constants.LayoutPrivate:
		if tc.Clinic != nil {
			view.ClinicName = tc.Clinic.Name
			view.Lead = strings.TrimSpace(tc.Clinic.Description)
			if view.Lead == "" {
				view.Lead = "مرکز درمانی " + tc.Clinic.Name
			}
			view.BodyHTML = "<p>" + htmlEscape(tc.Clinic.Description) + "</p>"
			if strings.TrimSpace(tc.Clinic.Description) == "" {
				view.BodyHTML = "<p>این مرکز از سامانه نوبت‌دهی طب‌پرداز استفاده می‌کند.</p>"
			}
			view.ShowReviews = true
			view.Reviews = h.buildClinicReviews(c, tc.Clinic.ID, c.Query("review"))
		}
	case constants.LayoutOrgan:
		name := "سازمان"
		if tc.Organization != nil {
			name = tc.Organization.Name
		}
		view.ClinicName = name
		view.BodyHTML = "<p>سامانه نوبت‌دهی مراکز زیرمجموعه «" + htmlEscape(name) + "».</p>"
	default:
		view.ClinicName = "طب‌پرداز"
		view.BodyHTML = "<p>پلتفرم نوبت‌دهی مراکز درمانی طب‌پرداز.</p>"
	}
	renderPublicLayout(c, tc, pages.AboutPage(view), "about")
}

// Contact صفحه تماس با ما را رندر می‌کند.
func (h *StaticPageHandler) Contact(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	view := pages.StaticPageView{
		Title: "تماس با ما",
		Lead:  "راه‌های ارتباطی مرکز را ببینید.",
	}
	if tc.Layout == constants.LayoutPrivate && tc.Clinic != nil {
		view.ClinicName = tc.Clinic.Name
		view.Phone = tc.Clinic.Phone
		view.Address = tc.Clinic.Address
	} else if tc.Layout == constants.LayoutOrgan && tc.Organization != nil {
		view.ClinicName = tc.Organization.Name
	} else {
		view.ClinicName = "طب‌پرداز"
	}
	renderPublicLayout(c, tc, pages.ContactPage(view), "contact")
}

// Terms صفحه قوانین و مقررات را رندر می‌کند.
func (h *StaticPageHandler) Terms(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	view := pages.StaticPageView{
		Title: "قوانین و مقررات",
		Lead:  "شرایط استفاده از سامانه نوبت‌دهی.",
	}
	renderPublicLayout(c, tc, pages.TermsPage(view), "terms")
}

// PostReview نظر کاربر برای پزشک یا مرکز را ثبت می‌کند.
func (h *StaticPageHandler) PostReview(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	if h.Reviews == nil || h.CSRF == nil || !h.CSRF.Verify(c.Request, c.PostForm("csrf_token")) {
		c.String(http.StatusBadRequest, "درخواست نامعتبر است")
		return
	}
	targetType := strings.TrimSpace(c.PostForm("target_type"))
	targetID := parseUintPost(c.PostForm("target_id"))
	clinicID := parseUintPost(c.PostForm("clinic_id"))
	rating, _ := strconv.Atoi(strings.TrimSpace(c.PostForm("rating")))
	redirect := strings.TrimSpace(c.PostForm("redirect"))
	if redirect == "" || strings.Contains(redirect, "://") || strings.HasPrefix(redirect, "//") || !strings.HasPrefix(redirect, "/") {
		redirect = "/"
	}

	if err := h.Reviews.Create(&models.Review{
		TargetType: targetType,
		TargetID:   targetID,
		ClinicID:   clinicID,
		AuthorName: c.PostForm("author_name"),
		Rating:     rating,
		Body:       c.PostForm("body"),
	}); err != nil {
		sep := "?"
		if strings.Contains(redirect, "?") {
			sep = "&"
		}
		c.Redirect(http.StatusFound, redirect+sep+"review=error")
		_ = tc
		return
	}
	sep := "?"
	if strings.Contains(redirect, "?") {
		sep = "&"
	}
	c.Redirect(http.StatusFound, redirect+sep+"review=ok")
}

func (h *StaticPageHandler) buildClinicReviews(c *gin.Context, clinicID uint, reviewFlash string) components.ReviewSectionView {
	view := components.ReviewSectionView{
		Title:       "نظرات درباره مرکز",
		TargetType:  models.ReviewTargetClinic,
		TargetID:    strconv.FormatUint(uint64(clinicID), 10),
		ClinicID:    strconv.FormatUint(uint64(clinicID), 10),
		FormAction:  "/reviews",
		RedirectURL: "/about",
	}
	if h.CSRF != nil {
		if tok, err := h.CSRF.Issue(c.Writer); err == nil {
			view.CSRFToken = tok
		}
	}
	if reviewFlash == "ok" {
		view.InfoMessage = "نظر شما ثبت شد و پس از تأیید نمایش داده می‌شود."
	}
	if reviewFlash == "error" {
		view.ErrorMessage = "ثبت نظر ناموفق بود. امتیاز ۱ تا ۵ الزامی است."
	}
	if h.Reviews == nil {
		return view
	}
	sum, _ := h.Reviews.Summary(models.ReviewTargetClinic, clinicID)
	view.Count = sum.Count
	view.Average = sum.Average
	rows, _ := h.Reviews.ListApprovedBodies(models.ReviewTargetClinic, clinicID, 20)
	for _, row := range rows {
		name := row.AuthorName
		if name == "" {
			name = "کاربر"
		}
		view.Items = append(view.Items, components.ReviewItemView{
			AuthorName: name,
			Rating:     row.Rating,
			Body:       row.Body,
		})
	}
	return view
}

func parseUintPost(raw string) uint {
	n, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0
	}
	return uint(n)
}

func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		`&`, "&amp;",
		`<`, "&lt;",
		`>`, "&gt;",
		`"`, "&quot;",
	)
	return replacer.Replace(strings.TrimSpace(s))
}

// NotFound صفحه ۴۰۴ مستأجر را رندر می‌کند.
func NotFound(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/admin") {
		c.String(http.StatusNotFound, "صفحه ادمین پیدا نشد")
		return
	}
	tc, ok := tenant.FromGin(c)
	home := "/"
	if !ok {
		c.Redirect(http.StatusFound, home)
		return
	}
	c.Status(http.StatusNotFound)
	renderPublicLayout(c, tc, pages.NotFound(pages.NotFoundView{HomeURL: home}), "")
}
