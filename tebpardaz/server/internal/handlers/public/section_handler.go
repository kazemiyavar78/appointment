package public

import (
	"fmt"
	"net/http"
	"strings"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/components"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// SectionPublicHandler مدیریت و نمایش صفحات عمومی بخش‌ها و زیرصفحات تخصصی آن‌ها را بر عهده دارد.
type SectionPublicHandler struct {
	Sections *repository.SectionRepo
	Clinics  *repository.ClinicRepo
	Listing  *booking.ListingService
	Services *repository.ServiceRepo
}

// NewSectionPublicHandler نمونه جدیدی از SectionPublicHandler ایجاد می‌کند.
// ورودی: ریپازیتوری بخش‌ها، ریپازیتوری مراکز، سرویس لیست پزشکان و ریپازیتوری خدمات.
// خروجی: اشاره‌گر به SectionPublicHandler.
func NewSectionPublicHandler(sections *repository.SectionRepo, clinics *repository.ClinicRepo, listing *booking.ListingService, services *repository.ServiceRepo) *SectionPublicHandler {
	return &SectionPublicHandler{
		Sections: sections,
		Clinics:  clinics,
		Listing:  listing,
		Services: services,
	}
}

// resolveClinicForRequest مرکز مربوط به درخواست را بر اساس مستأجر فعال یا اسلاگ مرکز در URL استخراج می‌کند.
// ورودی: کانتکست Gin و کانتکست مستأجر.
// خروجی: اشاره‌گر به مدل کلینیک، شناسه کلینیک یا خطا در صورت عدم وجود.
func (h *SectionPublicHandler) resolveClinicForRequest(c *gin.Context, tc *tenant.Context) (*models.Clinic, uint, error) {
	if tc != nil && tc.Layout == constants.LayoutPrivate && tc.ClinicID != nil && tc.Clinic != nil {
		return tc.Clinic, *tc.ClinicID, nil
	}
	clinicSlug := strings.TrimSpace(c.Param("clinic_slug"))
	if clinicSlug != "" && h.Clinics != nil {
		clinic, err := h.Clinics.GetBySlug(clinicSlug)
		if err == nil && clinic != nil {
			return clinic, clinic.ID, nil
		}
	}
	if tc != nil && tc.ClinicID != nil && tc.Clinic != nil {
		return tc.Clinic, *tc.ClinicID, nil
	}
	return nil, 0, fmt.Errorf("clinic not found")
}

// resolveSectionForRequest بخش مورد نظر را از دیتابیس با تطبیق کلینیک و اسلاگ بخش پیدا می‌کند.
// ورودی: کانتکست Gin و کانتکست مستأجر.
// خروجی: مدل بخش، مدل کلینیک، پیشوند مسیر و خطا.
func (h *SectionPublicHandler) resolveSectionForRequest(c *gin.Context, tc *tenant.Context) (*models.AppointmentClinicSection, *models.Clinic, string, error) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		return nil, nil, "", fmt.Errorf("slug is empty")
	}

	clinic, clinicID, err := h.resolveClinicForRequest(c, tc)
	if err == nil && clinic != nil && clinicID > 0 {
		sec, secErr := h.Sections.GetSectionByClinicAndSlug(clinicID, slug)
		if secErr == nil && sec != nil && sec.IsActive {
			prefix := "/section/" + sec.Slug
			if tc != nil && (tc.Layout == constants.LayoutOrgan || tc.Layout == constants.LayoutPlatform) && clinic.Slug != nil {
				prefix = fmt.Sprintf("/clinics/%s/section/%s", *clinic.Slug, sec.Slug)
			}
			return sec, clinic, prefix, nil
		}
	}

	// در حالت ارگان یا پلتفرم، ممکن است تمام مراکز را برای پیدا کردن بخش با این اسلاگ جستجو کنیم
	if h.Clinics != nil && h.Sections != nil {
		var candidateClinics []models.Clinic
		if tc != nil && tc.Layout == constants.LayoutOrgan && tc.OrganizationID != nil {
			candidateClinics, _ = h.Clinics.ListByOrganizationID(*tc.OrganizationID)
		} else {
			candidateClinics, _ = h.Clinics.ListAll()
		}

		for _, cl := range candidateClinics {
			sec, secErr := h.Sections.GetSectionByClinicAndSlug(cl.ID, slug)
			if secErr == nil && sec != nil && sec.IsActive {
				prefix := "/section/" + sec.Slug
				if cl.Slug != nil {
					prefix = fmt.Sprintf("/clinics/%s/section/%s", *cl.Slug, sec.Slug)
				}
				return sec, &cl, prefix, nil
			}
		}
	}

	return nil, nil, "", fmt.Errorf("section not found")
}

// sectionListPath مسیر واقعی فهرست بخش‌ها را از پیشوند صفحهٔ بخش برمی‌گرداند.
// ورودی: basePath مثل /section/{slug} یا /clinics/{slug}/section/{slug}. خروجی: مسیر فهرست.
func sectionListPath(basePath string) string {
	parts := strings.Split(strings.Trim(basePath, "/"), "/")
	if len(parts) >= 2 && parts[0] == "clinics" && parts[1] != "" {
		return "/clinics/" + parts[1] + "/sections"
	}
	return "/sections"
}

// sectionBreadcrumb مسیر خانه، فهرست واقعی بخش‌ها و خود بخش را می‌سازد.
// ورودی: درخواست، پیشوند بخش، نام بخش، و در صورت زیرصفحه نام و مسیر برگ. خروجی: JSON-LD خرده‌نان.
func sectionBreadcrumb(c *gin.Context, basePath, sectionTitle, leafName, leafPath string) string {
	items := []seo.BreadcrumbItemDTO{
		{Name: "خانه", URL: absolutePublicURL(c, "/")},
		{Name: "بخش‌ها", URL: absolutePublicURL(c, sectionListPath(basePath))},
		{Name: sectionTitle, URL: absolutePublicURL(c, basePath)},
	}
	if strings.TrimSpace(leafName) != "" && strings.TrimSpace(leafPath) != "" {
		items = append(items, seo.BreadcrumbItemDTO{
			Name: leafName,
			URL:  absolutePublicURL(c, leafPath),
		})
	}
	return seo.BuildBreadcrumbSchema(items)
}

// ListSections لیست تمام بخش‌های فعال را به صورت کارت‌های بنری نمایش می‌دهد.
// ورودی: کانتکست Gin.
// خروجی: صفحه HTML حاوی کارت‌های بخش با نشان مرکز در لایه ارگان/پلتفرم.
func (h *SectionPublicHandler) ListSections(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok || tc == nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	var targetClinicIDs []uint
	showClinicBadge := false
	title := "بخش‌های تخصصی و پاراکلینیکی"
	subtitle := "معرفی دپارتمان‌ها، تجهیزات پیشرفته و ساعات کاری بخش‌های درمانی"
	seoPlace := ""

	clinicSlug := strings.TrimSpace(c.Param("clinic_slug"))
	if clinicSlug != "" && h.Clinics != nil {
		cl, err := h.Clinics.GetBySlug(clinicSlug)
		if err != nil || cl == nil {
			NotFound(c)
			return
		}
		targetClinicIDs = []uint{cl.ID}
		title = fmt.Sprintf("بخش‌های درمانی %s", cl.Name)
		subtitle = fmt.Sprintf("لیست بخش‌ها و امکانات تخصصی فعال در %s", cl.Name)
		seoPlace = cl.Name
	} else if tc.Layout == constants.LayoutPrivate && tc.ClinicID != nil {
		targetClinicIDs = []uint{*tc.ClinicID}
		if tc.Clinic != nil {
			title = fmt.Sprintf("بخش‌های درمانی %s", tc.Clinic.Name)
			subtitle = fmt.Sprintf("آشنایی با بخش‌ها، تجهیزات و ساعات پذیرش %s", tc.Clinic.Name)
			seoPlace = tc.Clinic.Name
		}
	} else if tc.Layout == constants.LayoutOrgan && tc.OrganizationID != nil && h.Clinics != nil {
		showClinicBadge = true
		if tc.Organization != nil {
			seoPlace = tc.Organization.Name
		}
		if orgClinics, err := h.Clinics.ListByOrganizationID(*tc.OrganizationID); err == nil {
			for _, cl := range orgClinics {
				targetClinicIDs = append(targetClinicIDs, cl.ID)
			}
		}
	} else if h.Clinics != nil {
		showClinicBadge = true
		if allClinics, err := h.Clinics.ListAll(); err == nil {
			for _, cl := range allClinics {
				targetClinicIDs = append(targetClinicIDs, cl.ID)
			}
		}
	}

	var cards []components.SectionCardView
	for _, cid := range targetClinicIDs {
		sections, err := h.Sections.ListActiveSectionsWithBannerByClinic(cid)
		if err != nil || len(sections) == 0 {
			continue
		}

		var clinicName string
		var clinicSlugStr string
		if cl, err := h.Clinics.GetByID(cid); err == nil && cl != nil {
			clinicName = cl.Name
			if cl.Slug != nil {
				clinicSlugStr = *cl.Slug
			}
		}

		for _, s := range sections {
			banner := s.Banner
			slogan := "ارائه خدمات تخصصی با بالاترین کیفیت"
			desc := ""
			var services []string
			imgURL := ""
			bgColor := "#0a2e2e"
			bgStyle := "background-color: #0a2e2e;"

			if banner != nil {
				if banner.Slogan != "" {
					slogan = banner.Slogan
				}
				desc = banner.Description
				imgURL = banner.ImageURL
				if banner.BackgroundColor != "" {
					bgColor = banner.BackgroundColor
				}
				bgStyle = banner.BackgroundCSS()
				if banner.Services != "" {
					for _, line := range strings.Split(banner.Services, "\n") {
						if tr := strings.TrimSpace(line); tr != "" {
							services = append(services, tr)
						}
					}
				}
			}

			targetURL := fmt.Sprintf("/section/%s", s.Slug)
			whURL := fmt.Sprintf("/section/%s/ساعات-کاری", s.Slug)
			eqURL := fmt.Sprintf("/section/%s/تجهیزات", s.Slug)
			msgURL := fmt.Sprintf("/section/%s/پیام-به-مراجعین", s.Slug)

			if (tc.Layout == constants.LayoutOrgan || tc.Layout == constants.LayoutPlatform) && clinicSlugStr != "" {
				targetURL = fmt.Sprintf("/clinics/%s/section/%s", clinicSlugStr, s.Slug)
				whURL = fmt.Sprintf("/clinics/%s/section/%s/ساعات-کاری", clinicSlugStr, s.Slug)
				eqURL = fmt.Sprintf("/clinics/%s/section/%s/تجهیزات", clinicSlugStr, s.Slug)
				msgURL = fmt.Sprintf("/clinics/%s/section/%s/پیام-به-مراجعین", clinicSlugStr, s.Slug)
			}

			cards = append(cards, components.SectionCardView{
				Title:           s.Title,
				Slug:            s.Slug,
				ClinicName:      clinicName,
				ShowClinicBadge: showClinicBadge,
				Slogan:          slogan,
				Description:     desc,
				Services:        services,
				ImageURL:        imgURL,
				BackgroundColor: bgColor,
				BackgroundStyle: bgStyle,
				URL:             targetURL,
				WorkingHoursURL: whURL,
				EquipmentURL:    eqURL,
				MessagesURL:     msgURL,
				HasSchedules:    len(s.Schedules) > 0,
				HasEquipment:    len(s.Equipment) > 0,
				HasMessages:     len(s.Messages) > 0,
			})
		}
	}

	pageView := pages.SectionsListView{
		Title:            title,
		Subtitle:         subtitle,
		Sections:         cards,
		ShowClinicFilter: showClinicBadge,
	}

	pageURL := absolutePublicURL(c, c.Request.URL.Path)
	kind, _ := publicSite(tc)
	head := headFromMeta(seo.SectionListMeta(kind, seoPlace, pageURL))

	RenderPublicLayoutWithHead(c, tc, pages.SectionsList(pageView), "sections", head)
}

// Get صفحه اختصاصی کامل یک بخش (بنر، ساعات کاری، پیام‌ها و تجهیزات) را رندر می‌کند.
// ورودی: کانتکست Gin حاوی اسلاگ بخش.
// خروجی: صفحه HTML بخش به همراه متادیتا و اسکیماهای سئو.
func (h *SectionPublicHandler) Get(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok || tc == nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	sec, clinic, basePath, err := h.resolveSectionForRequest(c, tc)
	if err != nil || sec == nil {
		NotFound(c)
		return
	}

	clinicName := ""
	if clinic != nil {
		clinicName = clinic.Name
	}

	bannerDisplay := h.toBannerDisplay(sec.Banner, sec.Title)
	schedulesDisplay := h.toSchedulesDisplay(sec.Schedules)
	doctorsDisplay := h.loadSectionDoctorCards(sec.ID, tc)
	catalogDisplay := h.loadSectionCatalog(sec.ID)
	messagesDisplay := h.toMessagesDisplay(sec.Messages)
	equipmentDisplay := h.toEquipmentDisplay(sec.Equipment)

	view := pages.SectionDetailView{
		Title:           sec.Title,
		Slug:            sec.Slug,
		ClinicName:      clinicName,
		Banner:          bannerDisplay,
		Schedules:       schedulesDisplay,
		Doctors:         doctorsDisplay,
		CatalogServices: catalogDisplay,
		Messages:        messagesDisplay,
		Equipment:       equipmentDisplay,
	}

	pageURL := absolutePublicURL(c, basePath)
	breadSchema := sectionBreadcrumb(c, basePath, sec.Title, "", "")

	meta := seo.SectionDetailMeta(seo.SectionOverview, sec.Title, clinicName, "", pageURL)
	head := headFromMeta(meta)
	head.JSONLD = seo.BuildGraph(breadSchema)

	RenderPublicLayoutWithHead(c, tc, pages.SectionDetail(view), "section_"+sec.Slug, head)
}

// GetWorkingHours ساعات کاری مشخص یک بخش را رندر می‌کند.
// ساعات بخش ساعت درمانگاه نیست و MedicalClinic برای آن ساخته نمی‌شود.
// ورودی: کانتکست Gin.
// خروجی: صفحه HTML ساعات کاری بخش.
func (h *SectionPublicHandler) GetWorkingHours(c *gin.Context) {
	tc, _ := tenant.FromGin(c)
	sec, clinic, basePath, err := h.resolveSectionForRequest(c, tc)
	if err != nil || sec == nil || clinic == nil {
		if strings.TrimSpace(c.Param("slug")) != "" {
			NotFound(c)
			return
		}
		h.ListSections(c)
		return
	}

	schedulesDisplay := h.toSchedulesDisplay(sec.Schedules)
	view := pages.SectionDetailView{
		Title:      fmt.Sprintf("ساعات کاری بخش %s", sec.Title),
		Slug:       sec.Slug,
		ClinicName: clinic.Name,
		Schedules:  schedulesDisplay,
	}

	pageURL := absolutePublicURL(c, basePath+"/ساعات-کاری")
	breadSchema := sectionBreadcrumb(c, basePath, sec.Title, "ساعات کاری", basePath+"/ساعات-کاری")

	meta := seo.SectionDetailMeta(seo.SectionHours, sec.Title, clinic.Name, clinic.Address, pageURL)
	head := headFromMeta(meta)
	head.JSONLD = seo.BuildGraph(breadSchema)

	RenderPublicLayoutWithHead(c, tc, pages.SectionDetail(view), "section_"+sec.Slug, head)
}

// GetMessages پیام‌های مربوط به بیماران یک بخش را با اسکیمای Article رندر می‌کند.
// ورودی: کانتکست Gin.
// خروجی: صفحه HTML پیام‌های بخش.
func (h *SectionPublicHandler) GetMessages(c *gin.Context) {
	tc, _ := tenant.FromGin(c)
	sec, clinic, basePath, err := h.resolveSectionForRequest(c, tc)
	if err != nil || sec == nil || clinic == nil {
		if strings.TrimSpace(c.Param("slug")) != "" {
			NotFound(c)
			return
		}
		h.ListSections(c)
		return
	}

	messagesDisplay := h.toMessagesDisplay(sec.Messages)
	view := pages.SectionDetailView{
		Title:      fmt.Sprintf("پیام‌های بخش %s به مراجعین", sec.Title),
		Slug:       sec.Slug,
		ClinicName: clinic.Name,
		Messages:   messagesDisplay,
	}

	pageURL := absolutePublicURL(c, basePath+"/پیام-به-مراجعین")
	headline := fmt.Sprintf("پیام کادر درمانی بخش %s (%s) به مراجعین گرامی", sec.Title, clinic.Name)
	authorName := ""
	if len(messagesDisplay) > 0 {
		authorName = strings.TrimSpace(messagesDisplay[0].SenderTitle)
	}

	artSchema := seo.BuildArticleSchema(seo.ArticleDTO{
		Headline:      headline,
		URL:           pageURL,
		AuthorName:    authorName,
		PublisherName: clinic.Name,
		Description:   fmt.Sprintf("پیام رسمی و توصیه‌های پذیرش بخش %s در %s برای مراجعین محترم.", sec.Title, clinic.Name),
	})
	breadSchema := sectionBreadcrumb(c, basePath, sec.Title, "پیام به مراجعین", basePath+"/پیام-به-مراجعین")

	meta := seo.SectionDetailMeta(seo.SectionMessages, sec.Title, clinic.Name, "", pageURL)
	head := headFromMeta(meta)
	head.JSONLD = seo.BuildGraph(artSchema, breadSchema)

	RenderPublicLayoutWithHead(c, tc, pages.SectionDetail(view), "section_"+sec.Slug, head)
}

// GetEquipment لیست تجهیزات پزشکی یک بخش را با اسکیمای MedicalDevice رندر می‌کند.
// ورودی: کانتکست Gin.
// خروجی: صفحه HTML معرفی تجهیزات بخش.
func (h *SectionPublicHandler) GetEquipment(c *gin.Context) {
	tc, _ := tenant.FromGin(c)
	sec, clinic, basePath, err := h.resolveSectionForRequest(c, tc)
	if err != nil || sec == nil || clinic == nil {
		if strings.TrimSpace(c.Param("slug")) != "" {
			NotFound(c)
			return
		}
		h.ListSections(c)
		return
	}

	equipmentDisplay := h.toEquipmentDisplay(sec.Equipment)
	view := pages.SectionDetailView{
		Title:      fmt.Sprintf("تجهیزات و امکانات بخش %s", sec.Title),
		Slug:       sec.Slug,
		ClinicName: clinic.Name,
		Equipment:  equipmentDisplay,
	}

	pageURL := absolutePublicURL(c, basePath+"/تجهیزات")
	var eqDTOs []seo.EquipmentItemDTO
	for _, eq := range equipmentDisplay {
		eqDTOs = append(eqDTOs, seo.EquipmentItemDTO{
			Name:        eq.Title,
			Description: eq.Description,
			ImageURL:    eq.ImageURL,
			Category:    eq.Badge,
		})
	}

	for i := range eqDTOs {
		eqDTOs[i].ImageURL = seo.AbsoluteSchemaURL(publicBaseURL(c), eqDTOs[i].ImageURL)
	}
	eqSchema := seo.BuildEquipmentListSchema(pageURL, eqDTOs)
	breadSchema := sectionBreadcrumb(c, basePath, sec.Title, "تجهیزات", basePath+"/تجهیزات")

	meta := seo.SectionDetailMeta(seo.SectionEquipment, sec.Title, clinic.Name, "", pageURL)
	head := headFromMeta(meta)
	head.JSONLD = seo.BuildGraph(eqSchema, breadSchema)

	RenderPublicLayoutWithHead(c, tc, pages.SectionDetail(view), "section_"+sec.Slug, head)
}

// GetBannerIntro صفحه بنر و معرفی بصری یک بخش را رندر می‌کند.
// ورودی: کانتکست Gin.
// خروجی: صفحه HTML بنر بخش.
func (h *SectionPublicHandler) GetBannerIntro(c *gin.Context) {
	tc, _ := tenant.FromGin(c)
	sec, clinic, basePath, err := h.resolveSectionForRequest(c, tc)
	if err != nil || sec == nil || clinic == nil {
		if strings.TrimSpace(c.Param("slug")) != "" {
			NotFound(c)
			return
		}
		h.ListSections(c)
		return
	}

	bannerDisplay := h.toBannerDisplay(sec.Banner, sec.Title)
	view := pages.SectionDetailView{
		Title:      sec.Title,
		Slug:       sec.Slug,
		ClinicName: clinic.Name,
		Banner:     bannerDisplay,
	}

	pageURL := absolutePublicURL(c, basePath+"/معرفی")
	breadSchema := sectionBreadcrumb(c, basePath, sec.Title, "معرفی", basePath+"/معرفی")

	meta := seo.SectionDetailMeta(seo.SectionIntro, sec.Title, clinic.Name, "", pageURL)
	head := headFromMeta(meta)
	head.JSONLD = seo.BuildGraph(breadSchema)

	RenderPublicLayoutWithHead(c, tc, pages.SectionDetail(view), "section_"+sec.Slug, head)
}

// loadSectionCatalog خدمات یکتای بسته‌های غیرخالی منتسب به بخش را برای صفحه عمومی می‌سازد.
// ورودی: شناسه بخش. خروجی: کارت‌های خدمت، یا nil اگر بسته‌ای منتسب نباشد یا همه بسته‌ها خالی باشند.
func (h *SectionPublicHandler) loadSectionCatalog(sectionID uint) []pages.SectionServiceDisplay {
	if h == nil || h.Services == nil || sectionID == 0 {
		return nil
	}
	rows, err := h.Services.ListPublicServicesBySection(sectionID)
	if err != nil || len(rows) == 0 {
		return nil
	}
	out := make([]pages.SectionServiceDisplay, 0, len(rows))
	for _, row := range rows {
		names := row.PackageNames
		if names == nil {
			names = []string{}
		}
		out = append(out, pages.SectionServiceDisplay{
			Name:        row.Service.Name,
			Description: row.Service.Description,
			Packages:    names,
		})
	}
	return out
}

// loadSectionDoctorCards پزشکان منتسب به بخش را به کارت‌های رزرو عمومی تبدیل می‌کند.
// ورودی: شناسه بخش و کانتکست مستأجر. خروجی: اسلایس DoctorCardView برای صفحه بخش.
func (h *SectionPublicHandler) loadSectionDoctorCards(sectionID uint, tc *tenant.Context) []components.DoctorCardView {
	if h == nil || h.Sections == nil || sectionID == 0 {
		return nil
	}
	doctors, err := h.Sections.ListAssignedPublicDoctors(sectionID)
	if err != nil || len(doctors) == 0 {
		return nil
	}

	layout := constants.LayoutPrivate
	showClinicBadge := false
	if tc != nil {
		layout = tc.Layout
		showClinicBadge = tc.Layout == constants.LayoutOrgan || tc.Layout == constants.LayoutPlatform
	}

	if h.Listing != nil {
		cards, listErr := h.Listing.CardsFromDoctors(doctors, layout, showClinicBadge)
		if listErr == nil && len(cards) > 0 {
			return toSectionDoctorCardViews(cards)
		}
	}
	return toSectionDoctorCardViews(fallbackSectionDoctorCards(doctors, layout, showClinicBadge, clinicNameFromTenant(tc)))
}

// clinicNameFromTenant نام مرکز را از کانتکست مستأجر برمی‌گرداند.
// ورودی: کانتکست مستأجر. خروجی: نام مرکز یا رشته خالی.
func clinicNameFromTenant(tc *tenant.Context) string {
	if tc != nil && tc.Clinic != nil {
		return tc.Clinic.Name
	}
	return ""
}

// fallbackSectionDoctorCards کارت پزشک را بدون سرویس لیست/اسلات می‌سازد.
// ورودی: پزشکان، چیدمان، نشان مرکز و نام مرکز. خروجی: کارت‌های رزرو ساده.
func fallbackSectionDoctorCards(doctors []models.Doctor, layout constants.LayoutKind, showClinicBadge bool, clinicName string) []booking.DoctorCard {
	out := make([]booking.DoctorCard, 0, len(doctors))
	for _, d := range doctors {
		name := strings.TrimSpace(d.Name)
		if name == "" {
			name = strings.TrimSpace(d.FirstName + " " + d.LastName)
		}
		out = append(out, booking.DoctorCard{
			ID:              d.ID,
			Name:            name,
			SpecialtyName:   d.Specialty.Name,
			DoctorSystemID:  d.DoctorSystemID,
			PhotoURL:        d.PhotoURL,
			Photo300:        d.Photo300,
			Photo600:        d.Photo600,
			Photo900:        d.Photo900,
			Photo1200:       d.Photo1200,
			ShortDesc:       strings.TrimSpace(d.ShortDesc),
			ClinicID:        d.ClinicID,
			ClinicName:      clinicName,
			DoctorSlug:      d.Slug,
			ShowClinicBadge: showClinicBadge,
			BookingURL:      booking.BuildBookingURL(layout, "", d.Slug),
		})
	}
	return out
}

// toSectionDoctorCardViews کارت‌های سرویس رزرو را به مدل نمای DoctorCard تبدیل می‌کند.
// ورودی: اسلایس booking.DoctorCard. خروجی: اسلایس components.DoctorCardView.
func toSectionDoctorCardViews(items []booking.DoctorCard) []components.DoctorCardView {
	out := make([]components.DoctorCardView, 0, len(items))
	for _, item := range items {
		out = append(out, components.DoctorCardView{
			Name:            item.Name,
			SpecialtyName:   item.SpecialtyName,
			DoctorSystemID:  item.DoctorSystemID,
			PhotoURL:        item.PhotoURL,
			Photo300:        item.Photo300,
			Photo600:        item.Photo600,
			Photo900:        item.Photo900,
			Photo1200:       item.Photo1200,
			ShortDesc:       item.ShortDesc,
			ClinicName:      item.ClinicName,
			ShowClinicBadge: item.ShowClinicBadge,
			HasSlot:         item.HasSlot,
			NearestStartsAt: item.NearestStartsAt,
			BookingURL:      item.BookingURL,
		})
	}
	return out
}

// toBannerDisplay مدل بنر را به نمای قابل رندر تبدیل می‌کند.
// ورودی: اشاره‌گر به مدل بنر و عنوان بخش.
// خروجی: ساختار SectionBannerDisplay.
func (h *SectionPublicHandler) toBannerDisplay(b *models.SectionBanner, sectionTitle string) pages.SectionBannerDisplay {
	if b == nil {
		fallback := models.DefaultSectionBanner(0, sectionTitle)
		return pages.SectionBannerDisplay{
			Slogan:             fallback.Slogan,
			Description:        fallback.Description,
			Services:           []string{"پوشش کامل بیمه‌ها", "نوبت‌دهی آنلاین", "کادر تخصصی", "پاسخگویی سریع"},
			BackgroundColor:    fallback.BackgroundColor,
			BackgroundStyle:    fallback.BackgroundCSS(),
			UseOverlayGradient: fallback.OverlayEnabled(),
			OverlayStyle:       fallback.OverlayCSS(),
		}
	}

	var services []string
	if b.Services != "" {
		lines := strings.Split(b.Services, "\n")
		for _, l := range lines {
			trimmed := strings.TrimSpace(l)
			if trimmed != "" {
				services = append(services, trimmed)
			}
		}
	}

	bgColor := b.BackgroundColor
	if bgColor == "" {
		bgColor = "#0a2e2e"
	}

	return pages.SectionBannerDisplay{
		Slogan:             b.Slogan,
		Description:        b.Description,
		Services:           services,
		BackgroundColor:    bgColor,
		BackgroundStyle:    b.BackgroundCSS(),
		ImageURL:           b.ImageURL,
		UseOverlayGradient: b.OverlayEnabled(),
		OverlayStyle:       b.OverlayCSS(),
	}
}

// toSchedulesDisplay مدل‌های ساعات کاری را به ساختار نمای عمومی تبدیل می‌کند.
// ورودی: لیست مدل‌های SectionSchedule.
// خروجی: لیست ساختارهای SectionScheduleDisplay.
func (h *SectionPublicHandler) toSchedulesDisplay(schedules []models.SectionSchedule) []pages.SectionScheduleDisplay {
	out := make([]pages.SectionScheduleDisplay, 0, len(schedules))
	for _, s := range schedules {
		hoursText := ""
		if s.IsOpen {
			if s.HasShift2 && s.Shift2Start != "" && s.Shift2End != "" {
				hoursText = fmt.Sprintf("%s – %s\n%s – %s", s.Shift1Start, s.Shift1End, s.Shift2Start, s.Shift2End)
			} else {
				hoursText = fmt.Sprintf("%s – %s", s.Shift1Start, s.Shift1End)
			}
		} else {
			hoursText = "تعطیل"
		}

		out = append(out, pages.SectionScheduleDisplay{
			DayOfWeek: s.DayOfWeek,
			DayName:   s.DayName,
			IsOpen:    s.IsOpen,
			HoursText: hoursText,
		})
	}
	return out
}

// toMessagesDisplay مدل‌های پیام‌های بخش را به ساختار نمای عمومی تبدیل می‌کند.
// ورودی: لیست مدل‌های SectionMessage.
// خروجی: لیست ساختارهای SectionMessageDisplay.
func (h *SectionPublicHandler) toMessagesDisplay(messages []models.SectionMessage) []pages.SectionMessageDisplay {
	out := make([]pages.SectionMessageDisplay, 0, len(messages))
	for _, m := range messages {
		if !m.IsActive {
			continue
		}
		out = append(out, pages.SectionMessageDisplay{
			ID:          m.ID,
			Title:       m.Title,
			Content:     m.Content,
			SenderTitle: m.SenderTitle,
		})
	}
	return out
}

// toEquipmentDisplay مدل‌های تجهیزات بخش را به ساختار نمای عمومی تبدیل می‌کند.
// ورودی: لیست مدل‌های SectionEquipment.
// خروجی: لیست ساختارهای SectionEquipmentDisplay.
func (h *SectionPublicHandler) toEquipmentDisplay(equipment []models.SectionEquipment) []pages.SectionEquipmentDisplay {
	out := make([]pages.SectionEquipmentDisplay, 0, len(equipment))
	for _, eq := range equipment {
		if !eq.IsActive {
			continue
		}

		var tags []string
		if eq.Tags != "" {
			parts := strings.FieldsFunc(eq.Tags, func(r rune) bool {
				return r == ',' || r == '،' || r == '\n'
			})
			for _, p := range parts {
				trimmed := strings.TrimSpace(p)
				if trimmed != "" {
					tags = append(tags, trimmed)
				}
			}
		}

		btnURL := eq.ButtonURL
		if btnURL == "" {
			btnURL = "/doctors"
		}

		out = append(out, pages.SectionEquipmentDisplay{
			ID:          eq.ID,
			QuoteTitle:  eq.QuoteTitle,
			QuoteText:   eq.QuoteText,
			Badge:       eq.Badge,
			ImageURL:    eq.ImageURL,
			Title:       eq.Title,
			Subtitle:    eq.Subtitle,
			Description: eq.Description,
			Tags:        tags,
			ButtonText:  eq.ButtonText,
			ButtonURL:   btnURL,
		})
	}
	return out
}
