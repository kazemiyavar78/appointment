package public

import (
	"fmt"
	"net/http"
	"strings"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/seo"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/server/views/components"
	"tebpardaz/server/views/layouts"
	"tebpardaz/server/views/pages"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

// SectionPublicHandler مدیریت و نمایش صفحات عمومی بخش‌ها و زیرصفحات تخصصی آن‌ها را بر عهده دارد.
type SectionPublicHandler struct {
	Sections *repository.SectionRepo
	Clinics  *repository.ClinicRepo
}

// NewSectionPublicHandler نمونه جدیدی از SectionPublicHandler ایجاد می‌کند.
// ورودی: ریپازیتوری بخش‌ها و ریپازیتوری مراکز.
// خروجی: اشاره‌گر به SectionPublicHandler.
func NewSectionPublicHandler(sections *repository.SectionRepo, clinics *repository.ClinicRepo) *SectionPublicHandler {
	return &SectionPublicHandler{
		Sections: sections,
		Clinics:  clinics,
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

	clinicSlug := strings.TrimSpace(c.Param("clinic_slug"))
	if clinicSlug != "" && h.Clinics != nil {
		if cl, err := h.Clinics.GetBySlug(clinicSlug); err == nil && cl != nil {
			targetClinicIDs = []uint{cl.ID}
			title = fmt.Sprintf("بخش‌های درمانی %s", cl.Name)
			subtitle = fmt.Sprintf("لیست بخش‌ها و امکانات تخصصی فعال در %s", cl.Name)
		}
	} else if tc.Layout == constants.LayoutPrivate && tc.ClinicID != nil {
		targetClinicIDs = []uint{*tc.ClinicID}
		if tc.Clinic != nil {
			title = fmt.Sprintf("بخش‌های درمانی %s", tc.Clinic.Name)
			subtitle = fmt.Sprintf("آشنایی با بخش‌ها، تجهیزات و ساعات پذیرش %s", tc.Clinic.Name)
		}
	} else if tc.Layout == constants.LayoutOrgan && tc.OrganizationID != nil && h.Clinics != nil {
		showClinicBadge = true
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
		sections, err := h.Sections.ListActiveSectionsByClinic(cid)
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

			if banner != nil {
				if banner.Slogan != "" {
					slogan = banner.Slogan
				}
				desc = banner.Description
				imgURL = banner.ImageURL
				if banner.BackgroundColor != "" {
					bgColor = banner.BackgroundColor
				}
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

	pageURL := fmt.Sprintf("https://%s%s", c.Request.Host, c.Request.URL.Path)
	head := layouts.PageHead{
		Title:           title + " | طب‌پرداز",
		MetaDescription: subtitle,
		CanonicalURL:    pageURL,
		Robots:          "index, follow",
	}

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
		c.Status(http.StatusNotFound)
		return
	}

	clinicName := ""
	if clinic != nil {
		clinicName = clinic.Name
	}

	bannerDisplay := h.toBannerDisplay(sec.Banner, sec.Title)
	schedulesDisplay := h.toSchedulesDisplay(sec.Schedules)
	messagesDisplay := h.toMessagesDisplay(sec.Messages)
	equipmentDisplay := h.toEquipmentDisplay(sec.Equipment)

	view := pages.SectionDetailView{
		Title:      sec.Title,
		Slug:       sec.Slug,
		ClinicName: clinicName,
		Banner:     bannerDisplay,
		Schedules:  schedulesDisplay,
		Messages:   messagesDisplay,
		Equipment:  equipmentDisplay,
	}

	pageURL := fmt.Sprintf("https://%s%s", c.Request.Host, basePath)
	breadSchema := seo.BuildBreadcrumbSchema([]seo.BreadcrumbItemDTO{
		{Name: clinicName, URL: fmt.Sprintf("https://%s", c.Request.Host)},
		{Name: "بخش‌ها", URL: fmt.Sprintf("https://%s/sections", c.Request.Host)},
		{Name: sec.Title, URL: pageURL},
	})

	head := layouts.PageHead{
		Title:           fmt.Sprintf("بخش %s - %s | نوبت‌دهی، ساعات کاری و تجهیزات", sec.Title, clinicName),
		MetaDescription: fmt.Sprintf("اطلاعات کامل، ساعات کاری، معرفی تجهیزات پیشرفته و پیام‌های بخش %s در %s.", sec.Title, clinicName),
		CanonicalURL:    pageURL,
		Robots:          "index, follow",
		JSONLD:          breadSchema,
	}

	RenderPublicLayoutWithHead(c, tc, pages.SectionDetail(view), "section_"+sec.Slug, head)
}

// GetWorkingHours ساعات کاری مشخص یک بخش را به همراه اسکیمای OpeningHoursSpecification رندر می‌کند.
// ورودی: کانتکست Gin.
// خروجی: صفحه HTML ساعات کاری بخش.
func (h *SectionPublicHandler) GetWorkingHours(c *gin.Context) {
	tc, _ := tenant.FromGin(c)
	sec, clinic, basePath, err := h.resolveSectionForRequest(c, tc)
	if err != nil || sec == nil || clinic == nil {
		// اگر بخش در مسیر تعیین نشده باشد، کاربر را به صفحه لیست بخش‌ها هدایت می‌کنیم
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

	var openingHours []seo.OpeningHoursDTO
	dayMap := map[int]string{
		0: "Saturday", 1: "Sunday", 2: "Monday", 3: "Tuesday", 4: "Wednesday", 5: "Thursday", 6: "Friday",
	}
	for _, s := range sec.Schedules {
		if s.IsOpen && s.Shift1Start != "" && s.Shift1End != "" {
			dName := dayMap[s.DayOfWeek]
			if dName != "" {
				openingHours = append(openingHours, seo.OpeningHoursDTO{
					DaysOfWeek: []string{dName},
					Opens:      s.Shift1Start,
					Closes:     s.Shift1End,
				})
			}
		}
	}

	pageURL := fmt.Sprintf("https://%s%s/ساعات-کاری", c.Request.Host, basePath)
	clinicSchema := seo.BuildMedicalClinicSchema(seo.MedicalClinicDTO{
		Name:         fmt.Sprintf("%s - بخش %s", clinic.Name, sec.Title),
		URL:          fmt.Sprintf("https://%s%s", c.Request.Host, basePath),
		Telephone:    clinic.Phone,
		OpeningHours: openingHours,
		Address: &seo.PostalAddressDTO{
			StreetAddress:   clinic.Address,
			AddressLocality: clinic.City.Name,
			AddressRegion:   clinic.City.Province,
			AddressCountry:  "IR",
		},
	})
	breadSchema := seo.BuildBreadcrumbSchema([]seo.BreadcrumbItemDTO{
		{Name: clinic.Name, URL: fmt.Sprintf("https://%s", c.Request.Host)},
		{Name: sec.Title, URL: fmt.Sprintf("https://%s%s", c.Request.Host, basePath)},
		{Name: "ساعات کاری", URL: pageURL},
	})

	head := layouts.PageHead{
		Title:           fmt.Sprintf("ساعات کاری بخش %s %s | %s", sec.Title, clinic.Name, clinic.City.Name),
		MetaDescription: fmt.Sprintf("برنامه دقیق ساعات فعالیت و پذیرش بخش %s در %s واقع در %s. مشاهده شیفت‌های کاری.", sec.Title, clinic.Name, clinic.Address),
		CanonicalURL:    pageURL,
		Robots:          "index, follow",
		JSONLD:          seo.CombineSchemas(clinicSchema, breadSchema),
	}

	RenderPublicLayoutWithHead(c, tc, pages.SectionDetail(view), "section_"+sec.Slug, head)
}

// GetMessages پیام‌های مربوط به بیماران یک بخش را با اسکیمای Article رندر می‌کند.
// ورودی: کانتکست Gin.
// خروجی: صفحه HTML پیام‌های بخش.
func (h *SectionPublicHandler) GetMessages(c *gin.Context) {
	tc, _ := tenant.FromGin(c)
	sec, clinic, basePath, err := h.resolveSectionForRequest(c, tc)
	if err != nil || sec == nil || clinic == nil {
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

	pageURL := fmt.Sprintf("https://%s%s/پیام-به-مراجعین", c.Request.Host, basePath)
	headline := fmt.Sprintf("پیام کادر درمانی بخش %s (%s) به مراجعین گرامی", sec.Title, clinic.Name)
	authorTitle := fmt.Sprintf("مسئول بخش %s - %s", sec.Title, clinic.Name)
	if len(messagesDisplay) > 0 && messagesDisplay[0].SenderTitle != "" {
		authorTitle = messagesDisplay[0].SenderTitle
	}

	artSchema := seo.BuildArticleSchema(seo.ArticleDTO{
		Headline:      headline,
		URL:           pageURL,
		AuthorName:    authorTitle,
		PublisherName: clinic.Name,
		Description:   fmt.Sprintf("پیام رسمی و توصیه‌های پذیرش بخش %s در %s برای مراجعین محترم.", sec.Title, clinic.Name),
	})
	breadSchema := seo.BuildBreadcrumbSchema([]seo.BreadcrumbItemDTO{
		{Name: clinic.Name, URL: fmt.Sprintf("https://%s", c.Request.Host)},
		{Name: sec.Title, URL: fmt.Sprintf("https://%s%s", c.Request.Host, basePath)},
		{Name: "پیام به مراجعین", URL: pageURL},
	})

	head := layouts.PageHead{
		Title:           fmt.Sprintf("پیام به مراجعین بخش %s | %s", sec.Title, clinic.Name),
		MetaDescription: fmt.Sprintf("پیام، توصیه‌های درمانی و راهنمای مراجعین بخش %s در %s.", sec.Title, clinic.Name),
		CanonicalURL:    pageURL,
		Robots:          "index, follow",
		JSONLD:          seo.CombineSchemas(artSchema, breadSchema),
	}

	RenderPublicLayoutWithHead(c, tc, pages.SectionDetail(view), "section_"+sec.Slug, head)
}

// GetEquipment لیست تجهیزات پزشکی یک بخش را با اسکیمای MedicalDevice رندر می‌کند.
// ورودی: کانتکست Gin.
// خروجی: صفحه HTML معرفی تجهیزات بخش.
func (h *SectionPublicHandler) GetEquipment(c *gin.Context) {
	tc, _ := tenant.FromGin(c)
	sec, clinic, basePath, err := h.resolveSectionForRequest(c, tc)
	if err != nil || sec == nil || clinic == nil {
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

	pageURL := fmt.Sprintf("https://%s%s/تجهیزات", c.Request.Host, basePath)
	var eqDTOs []seo.EquipmentItemDTO
	for _, eq := range equipmentDisplay {
		eqDTOs = append(eqDTOs, seo.EquipmentItemDTO{
			Name:        eq.Title,
			Description: eq.Description,
			ImageURL:    eq.ImageURL,
			Category:    eq.Badge,
		})
	}

	eqSchema := seo.BuildEquipmentListSchema(fmt.Sprintf("%s (بخش %s)", clinic.Name, sec.Title), pageURL, eqDTOs)
	breadSchema := seo.BuildBreadcrumbSchema([]seo.BreadcrumbItemDTO{
		{Name: clinic.Name, URL: fmt.Sprintf("https://%s", c.Request.Host)},
		{Name: sec.Title, URL: fmt.Sprintf("https://%s%s", c.Request.Host, basePath)},
		{Name: "تجهیزات پزشکی", URL: pageURL},
	})

	head := layouts.PageHead{
		Title:           fmt.Sprintf("تجهیزات و امکانات بخش %s | %s", sec.Title, clinic.Name),
		MetaDescription: fmt.Sprintf("معرفی فناوری‌ها و تجهیزات پیشرفته تشخیصی درمانی بخش %s در %s.", sec.Title, clinic.Name),
		CanonicalURL:    pageURL,
		Robots:          "index, follow",
		JSONLD:          seo.CombineSchemas(eqSchema, breadSchema),
	}

	RenderPublicLayoutWithHead(c, tc, pages.SectionDetail(view), "section_"+sec.Slug, head)
}

// GetBannerIntro صفحه بنر و معرفی بصری یک بخش را رندر می‌کند.
// ورودی: کانتکست Gin.
// خروجی: صفحه HTML بنر بخش.
func (h *SectionPublicHandler) GetBannerIntro(c *gin.Context) {
	tc, _ := tenant.FromGin(c)
	sec, clinic, basePath, err := h.resolveSectionForRequest(c, tc)
	if err != nil || sec == nil || clinic == nil {
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

	pageURL := fmt.Sprintf("https://%s%s/معرفی", c.Request.Host, basePath)
	breadSchema := seo.BuildBreadcrumbSchema([]seo.BreadcrumbItemDTO{
		{Name: clinic.Name, URL: fmt.Sprintf("https://%s", c.Request.Host)},
		{Name: sec.Title, URL: fmt.Sprintf("https://%s%s", c.Request.Host, basePath)},
		{Name: "معرفی", URL: pageURL},
	})

	head := layouts.PageHead{
		Title:           fmt.Sprintf("معرفی بخش %s | %s", sec.Title, clinic.Name),
		MetaDescription: fmt.Sprintf("آشنایی با خدمات، امکانات و اهداف بخش %s در %s.", sec.Title, clinic.Name),
		CanonicalURL:    pageURL,
		Robots:          "index, follow",
		JSONLD:          breadSchema,
	}

	RenderPublicLayoutWithHead(c, tc, pages.SectionDetail(view), "section_"+sec.Slug, head)
}

// toBannerDisplay مدل بنر را به نمای قابل رندر تبدیل می‌کند.
// ورودی: اشاره‌گر به مدل بنر و عنوان بخش.
// خروجی: ساختار SectionBannerDisplay.
func (h *SectionPublicHandler) toBannerDisplay(b *models.SectionBanner, sectionTitle string) pages.SectionBannerDisplay {
	if b == nil {
		return pages.SectionBannerDisplay{
			Slogan:          "ارائه خدمات تخصصی با بالاترین کیفیت",
			Description:     fmt.Sprintf("بخش %s با بهره‌گیری از کادر مجرب و پیشرفته‌ترین امکانات آماده خدمت‌رسانی به شما عزیزان است.", sectionTitle),
			Services:        []string{"پوشش کامل بیمه‌ها", "نوبت‌دهی آنلاین", "کادر تخصصی", "پاسخگویی سریع"},
			BackgroundColor: "#0a2e2e",
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
		Slogan:          b.Slogan,
		Description:     b.Description,
		Services:        services,
		BackgroundColor: bgColor,
		ImageURL:        b.ImageURL,
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
