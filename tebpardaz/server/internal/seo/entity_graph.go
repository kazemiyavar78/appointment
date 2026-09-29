package seo

import (
	"encoding/json"
	"strings"
)

const (
	// PlatformLegalName نام رسمی شرکت در UI پلتفرم است.
	PlatformLegalName = "طب پرداز شرق ایرانیان"
	// PlatformPhone شمارهٔ تماس صفحهٔ دربارهٔ پلتفرم است.
	PlatformPhone = "02191691978"
	// PlatformLogoPath لوگوی layout پلتفرم است.
	PlatformLogoPath = "/static/clinics/logo.jpg"
	// platformAboutLead همان متن lead قابل مشاهده در صفحهٔ دربارهٔ پلتفرم است.
	platformAboutLead = "طب‌پرداز، سیستم مدیریت کلینیک، مطب و درمانگاه، پرونده هر بیمار را از لحظه ورود تا پایان درمان دنبال می‌کند و کلینیک، دندان‌پزشکی، تصویربرداری، آزمایشگاه، حقوق و انبار مرکز شما را در یک سامانه واحد کنار هم می‌گذارد."
)

// TenantHomeInput دادهٔ واقعی صفحهٔ اصلی دامنهٔ اختصاصی مرکز است.
type TenantHomeInput struct {
	Origin      string
	PageURL     string
	Name        string
	Description string
	Phone       string
	LogoURL     string
	Street      string
	City        string
	Province    string
}

// BookingPageInput دادهٔ صفحهٔ رزرو برای WebPage، Physician و Breadcrumb است.
type BookingPageInput struct {
	BaseURL              string
	Canonical            string
	Title                string
	Description          string
	DoctorName           string
	DoctorSlug           string
	Specialty            string
	PhotoURL             string
	UseClinicLogo        bool
	ClinicName           string
	ClinicOrigin         string
	ClinicEntityID       string
	ClinicEntityURL      string
	ParentOrganizationID string
	ClinicDescription    string
	ClinicPhone          string
	ClinicLogo           string
	ClinicAddress        *PostalAddressDTO
}

// AbsoluteSchemaURL مسیر نسبی را با مبدأ درخواست مطلق می‌کند.
// ورودی: مبدأ scheme://host و URL خام. خروجی: URL با https، یا خالی اگر http مطلق باشد.
func AbsoluteSchemaURL(baseURL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "https://") {
		return raw
	}
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(raw, "//") || strings.Contains(raw, "://") {
		return ""
	}
	return AbsoluteURL(baseURL, raw)
}

// OriginID شناسهٔ پایدار موجودیت روی مبدأ را می‌سازد.
// ورودی: مبدأ و نام قطعه مثل clinic. خروجی: https://host/#name
func OriginID(baseURL, name string) string {
	origin := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	name = strings.TrimSpace(name)
	if origin == "" || name == "" {
		return ""
	}
	return origin + "/#" + name
}

// PageFragmentID شناسهٔ موجودیت روی همان URL صفحه را می‌سازد.
// ورودی: URL صفحه و نام قطعه. خروجی: URL بدون اسلش انتهایی به‌علاوه #name
func PageFragmentID(pageURL, name string) string {
	pageURL = strings.TrimRight(strings.TrimSpace(pageURL), "/")
	name = strings.TrimSpace(name)
	if pageURL == "" || name == "" {
		return ""
	}
	return pageURL + "#" + name
}

// HTTPSOrigin دامنهٔ ذخیره‌شده را به مبدأ HTTPS تبدیل می‌کند.
// ورودی: host یا URL. خروجی: https://host یا خالی.
func HTTPSOrigin(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.Trim(host, "/")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	if host == "" || strings.ContainsAny(host, " \t") {
		return ""
	}
	return "https://" + host
}

// OfficialClinicOrigin مبدأ رسمی مرکز است، فقط وقتی دامنه ذخیره شده و سایت فعال است.
// ورودی: دامنهٔ nullable و پرچم فعال بودن روی وب. خروجی: https://domain یا خالی.
func OfficialClinicOrigin(domain *string, activeOnWebsite bool) string {
	if !activeOnWebsite || domain == nil {
		return ""
	}
	return HTTPSOrigin(*domain)
}

// ClinicSchemaOrigin مبدأ @id مرکز را انتخاب می‌کند.
// ورودی: مبدأ درخواست، دامنه و پرچم فعال. خروجی: دامنهٔ رسمی اگر باشد، وگرنه مبدأ درخواست.
func ClinicSchemaOrigin(requestBase string, domain *string, active bool) string {
	if official := OfficialClinicOrigin(domain, active); official != "" {
		return official
	}
	return strings.TrimRight(strings.TrimSpace(requestBase), "/")
}

// ListPosition جایگاه آیتم در فهرست صفحه‌بندی‌شده را برمی‌گرداند.
// ورودی: صفحهٔ ۱مبنا، اندازهٔ صفحه، اندیس صفرمبنا داخل همان صفحه. خروجی: position یک‌مبنا.
func ListPosition(page, pageSize, index int) int {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 1
	}
	if index < 0 {
		index = 0
	}
	return (page-1)*pageSize + index + 1
}

// PhysicianImage عکس پزشک را فقط وقتی لوگوی مرکز جای عکس نشسته باشد حذف می‌کند.
// ورودی: مبدأ، URL عکس، و پرچم UseClinicLogo. خروجی: URL مطلق https یا خالی.
func PhysicianImage(baseURL, photoURL string, useClinicLogo bool) string {
	if useClinicLogo {
		return ""
	}
	return AbsoluteSchemaURL(baseURL, photoURL)
}

// BuildGraph چند شیء JSON-LD را در یک @graph با یک @context جمع می‌کند.
// ورودی: رشته‌های JSON شیء. خروجی: یک سند، یا خالی اگر گره‌ای نمانده باشد.
func BuildGraph(nodes ...string) string {
	graph := make([]json.RawMessage, 0, len(nodes))
	for _, node := range nodes {
		node = strings.TrimSpace(node)
		if node == "" {
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(node), &obj); err != nil {
			continue
		}
		delete(obj, "@context")
		cleaned, err := json.Marshal(obj)
		if err != nil {
			continue
		}
		graph = append(graph, cleaned)
	}
	if len(graph) == 0 {
		return ""
	}
	return MinifyJSONLD(map[string]interface{}{
		"@context": "https://schema.org",
		"@graph":   graph,
	})
}

// BuildWebPageSchema صفحه را به موجودیت اصلی‌اش وصل می‌کند.
// ورودی: @id، url، نام، توضیح، @id موجودیت اصلی. خروجی: JSON-LD.
func BuildWebPageSchema(id, pageURL, name, description, mainEntityID string) string {
	data := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "WebPage",
		"url":      pageURL,
		"name":     name,
	}
	if id != "" {
		data["@id"] = id
	}
	if description != "" {
		data["description"] = description
	}
	if mainEntityID != "" {
		data["mainEntity"] = map[string]interface{}{"@id": mainEntityID}
	}
	return MinifyJSONLD(data)
}

// BuildWebSiteSchema وب‌سایت را با ارجاع publisher می‌سازد.
// ورودی: @id، url، نام، توضیح اختیاری، @id ناشر. خروجی: JSON-LD.
func BuildWebSiteSchema(id, pageURL, name, description, publisherID string) string {
	data := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "WebSite",
		"url":      pageURL,
		"name":     name,
	}
	if id != "" {
		data["@id"] = id
	}
	if description != "" {
		data["description"] = description
	}
	if publisherID != "" {
		data["publisher"] = map[string]interface{}{"@id": publisherID}
	}
	return MinifyJSONLD(data)
}

// PlatformHomeGraph سازمان و وب‌سایت پلتفرم را می‌سازد.
// ورودی: مبدأ درخواست و URL صفحهٔ اصلی. خروجی: @graph یا خالی.
func PlatformHomeGraph(baseURL, pageURL string) string {
	origin := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if origin == "" || strings.TrimSpace(pageURL) == "" {
		return ""
	}
	orgID := OriginID(origin, "organization")
	siteID := OriginID(origin, "website")
	logo := AbsoluteSchemaURL(origin, PlatformLogoPath)
	org := map[string]interface{}{
		"@context":    "https://schema.org",
		"@type":       "Organization",
		"@id":         orgID,
		"name":        PlatformBrand,
		"legalName":   PlatformLegalName,
		"url":         pageURL,
		"telephone":   PlatformPhone,
		"description": platformAboutLead,
	}
	if logo != "" {
		org["logo"] = logo
	}
	return BuildGraph(
		MinifyJSONLD(org),
		BuildWebSiteSchema(siteID, pageURL, PlatformBrand, "", orgID),
	)
}

// TenantHomeGraph درمانگاه و وب‌سایت دامنهٔ اختصاصی را می‌سازد.
// ورودی: دادهٔ مرکز. خروجی: @graph یا خالی. ساعت کاری، geo و sameAs ساخته نمی‌شود.
func TenantHomeGraph(in TenantHomeInput) string {
	name := strings.TrimSpace(in.Name)
	origin := strings.TrimRight(strings.TrimSpace(in.Origin), "/")
	pageURL := strings.TrimSpace(in.PageURL)
	if name == "" || origin == "" || pageURL == "" {
		return ""
	}
	clinicID := OriginID(origin, "clinic")
	var addr *PostalAddressDTO
	street := strings.TrimSpace(in.Street)
	city := strings.TrimSpace(in.City)
	province := strings.TrimSpace(in.Province)
	if street != "" || city != "" || province != "" {
		addr = &PostalAddressDTO{
			StreetAddress:   street,
			AddressLocality: city,
			AddressRegion:   province,
			AddressCountry:  "IR",
		}
	}
	clinic := BuildMedicalClinicSchema(MedicalClinicDTO{
		ID:          clinicID,
		Name:        name,
		URL:         pageURL,
		Description: PlainText(in.Description),
		LogoURL:     strings.TrimSpace(in.LogoURL),
		Telephone:   strings.TrimSpace(in.Phone),
		Address:     addr,
	})
	site := BuildWebSiteSchema(OriginID(origin, "website"), pageURL, name, "", clinicID)
	return BuildGraph(clinic, site)
}

// BookingPageGraph صفحهٔ رزرو، پزشک و مسیر را می‌سازد.
// ورودی: BookingPageInput. خروجی: @graph. canonical صفحه همان URL ورودی می‌ماند.
func BookingPageGraph(in BookingPageInput) string {
	name := strings.TrimSpace(in.DoctorName)
	canonical := strings.TrimSpace(in.Canonical)
	if name == "" || canonical == "" {
		return ""
	}
	physicianID := PageFragmentID(canonical, "physician")
	clinicID := strings.TrimSpace(in.ClinicEntityID)
	if clinicID == "" {
		if origin := strings.TrimRight(strings.TrimSpace(in.ClinicOrigin), "/"); origin != "" {
			clinicID = OriginID(origin, "clinic")
		}
	}
	home := AbsoluteURL(in.BaseURL, "/")
	doctors := AbsoluteURL(in.BaseURL, "/doctors")
	physician := BuildPhysicianSchema(PhysicianDTO{
		ID:         physicianID,
		Name:       name,
		URL:        canonical,
		Specialty:  strings.TrimSpace(in.Specialty),
		ImageURL:   PhysicianImage(in.BaseURL, in.PhotoURL, in.UseClinicLogo),
		ClinicID:   clinicID,
		ClinicName: strings.TrimSpace(in.ClinicName),
	})
	page := BuildWebPageSchema(PageFragmentID(canonical, "webpage"), canonical, in.Title, in.Description, physicianID)
	crumb := BuildBreadcrumbSchema([]BreadcrumbItemDTO{
		{Name: "خانه", URL: home},
		{Name: "پزشکان", URL: doctors},
		{Name: name, URL: canonical},
	})
	nodes := []string{page, physician, crumb}
	if entityID := strings.TrimSpace(in.ClinicEntityID); entityID != "" && strings.TrimSpace(in.ClinicEntityURL) != "" && strings.TrimSpace(in.ClinicName) != "" {
		nodes = append(nodes, BuildMedicalClinicSchema(MedicalClinicDTO{
			ID:          entityID,
			Name:        strings.TrimSpace(in.ClinicName),
			URL:         strings.TrimSpace(in.ClinicEntityURL),
			Description: strings.TrimSpace(in.ClinicDescription),
			Telephone:   strings.TrimSpace(in.ClinicPhone),
			LogoURL:     strings.TrimSpace(in.ClinicLogo),
			Address:     in.ClinicAddress,
			ParentID:    strings.TrimSpace(in.ParentOrganizationID),
		}))
	}
	return BuildGraph(nodes...)
}

// DoctorsPageGraph مسیر پزشکان و در صورت ایندکس بودن ItemList همان صفحه را می‌سازد.
// ورودی: URL خانه و فهرست، آیتم‌های همین صفحه، مجاز بودن فهرست، و تعداد کل listing. خروجی: @graph.
// total صفر یعنی تعداد کل معلوم نیست و numberOfItems نوشته نمی‌شود.
func DoctorsPageGraph(homeURL, doctorsURL string, items []ItemListElementDTO, includeList bool, total int) string {
	crumb := BuildBreadcrumbSchema([]BreadcrumbItemDTO{
		{Name: "خانه", URL: homeURL},
		{Name: "پزشکان", URL: doctorsURL},
	})
	if !includeList || len(items) == 0 {
		return BuildGraph(crumb)
	}
	for i := range items {
		if items[i].Type == "" {
			items[i].Type = "Physician"
		}
	}
	nodes := []string{crumb, BuildItemListSchema("پزشکان", items, total)}
	seen := map[string]struct{}{}
	for _, it := range items {
		id := strings.TrimSpace(it.WorksForID)
		if id == "" || strings.Contains(id, "?") {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		pageURL := strings.TrimSuffix(id, "#clinic")
		if pageURL == id || strings.TrimSpace(it.WorksForName) == "" {
			continue
		}
		nodes = append(nodes, BuildMedicalClinicSchema(MedicalClinicDTO{
			ID:       id,
			Name:     strings.TrimSpace(it.WorksForName),
			URL:      pageURL,
			ParentID: strings.TrimSpace(it.ParentOrganizationID),
		}))
	}
	return BuildGraph(nodes...)
}

// SpecialtyIndexGraph صفحهٔ فهرست تخصص‌ها، مسیر و ItemList لندینگ‌ها را می‌سازد.
// ورودی: URL خانه و صفحه، عنوان، توضیح، و آیتم‌های WebPage. خروجی: @graph.
func SpecialtyIndexGraph(homeURL, pageURL, title, description string, items []ItemListElementDTO) string {
	page := BuildWebPageSchema(PageFragmentID(pageURL, "webpage"), pageURL, title, description, "")
	crumb := BuildBreadcrumbSchema([]BreadcrumbItemDTO{
		{Name: "خانه", URL: homeURL},
		{Name: "تخصص‌های پزشکی", URL: pageURL},
	})
	if len(items) == 0 {
		return BuildGraph(page, crumb)
	}
	for i := range items {
		items[i].Type = "WebPage"
	}
	return BuildGraph(page, crumb, BuildItemListSchema("تخصص‌های پزشکی", items, len(items)))
}

// SpecialtyDetailGraph صفحهٔ تخصص، مسیر و ItemList پزشکان همین صفحه را می‌سازد.
// ورودی: URLها، عنوان صفحه، توضیح، نام تخصص برای breadcrumb، پزشکان با position، مجاز بودن فهرست، و تعداد کل.
// خروجی: @graph. شماره صفحه breadcrumb جدا نمی‌شود. total صفر یعنی numberOfItems نوشته نمی‌شود.
func SpecialtyDetailGraph(homeURL, indexURL, entityURL, pageURL, title, description, specialtyName string, doctors []PhysicianDTO, positions []int, includeList bool, total int) string {
	page := BuildWebPageSchema(PageFragmentID(pageURL, "webpage"), pageURL, title, description, "")
	crumb := BuildBreadcrumbSchema([]BreadcrumbItemDTO{
		{Name: "خانه", URL: homeURL},
		{Name: "تخصص‌های پزشکی", URL: indexURL},
		{Name: strings.TrimSpace(specialtyName), URL: entityURL},
	})
	if !includeList || len(doctors) == 0 {
		return BuildGraph(page, crumb)
	}
	return BuildGraph(page, crumb, buildPhysicianItemList(doctors, positions, total))
}

// ClinicIndexGraph صفحهٔ فهرست مراکز، مسیر و ItemList لندینگ‌ها را می‌سازد.
// ورودی: URL خانه و صفحه، عنوان، توضیح، و آیتم‌های MedicalClinic. خروجی: @graph.
func ClinicIndexGraph(homeURL, pageURL, title, description string, items []ItemListElementDTO) string {
	page := BuildWebPageSchema(PageFragmentID(pageURL, "webpage"), pageURL, title, description, "")
	crumb := BuildBreadcrumbSchema([]BreadcrumbItemDTO{
		{Name: "خانه", URL: homeURL},
		{Name: "مراکز درمانی", URL: pageURL},
	})
	if len(items) == 0 {
		return BuildGraph(page, crumb)
	}
	for i := range items {
		if items[i].Type == "" {
			items[i].Type = "MedicalClinic"
		}
	}
	return BuildGraph(page, crumb, BuildItemListSchema("مراکز درمانی", items, len(items)))
}

// ClinicDetailGraph صفحهٔ مرکز، موجودیت محلی و در صورت وجود پزشکان ItemList را می‌سازد.
// ورودی: URLها، عنوان، توضیح، دادهٔ مرکز، پزشکان صفحه، مجاز بودن فهرست و تعداد کل.
// خروجی: @graph. WebPage از pageURL است؛ @id و url مرکز همیشه entityURL بدون page است.
// parentOrganization فقط اگر caller آن را در clinic گذاشته باشد نوشته می‌شود. sameAs نوشته نمی‌شود.
// indexURL خالی یعنی فهرست /clinics روی این سطح نیست و از breadcrumb حذف می‌شود.
func ClinicDetailGraph(homeURL, indexURL, entityURL, pageURL, title, description string, clinic MedicalClinicDTO, doctors []PhysicianDTO, positions []int, includeList bool, total int) string {
	clinicID := PageFragmentID(entityURL, "clinic")
	clinic.ID = clinicID
	clinic.URL = entityURL
	for i := range doctors {
		doctors[i].ClinicID = clinicID
	}
	page := BuildWebPageSchema(PageFragmentID(pageURL, "webpage"), pageURL, title, description, clinicID)
	crumbs := []BreadcrumbItemDTO{{Name: "خانه", URL: homeURL}}
	if strings.TrimSpace(indexURL) != "" {
		crumbs = append(crumbs, BreadcrumbItemDTO{Name: "مراکز درمانی", URL: indexURL})
	}
	crumbs = append(crumbs, BreadcrumbItemDTO{Name: strings.TrimSpace(clinic.Name), URL: entityURL})
	crumb := BuildBreadcrumbSchema(crumbs)
	nodes := []string{page, BuildMedicalClinicSchema(clinic), crumb}
	if includeList && len(doctors) > 0 {
		nodes = append(nodes, buildPhysicianItemList(doctors, positions, total))
	}
	return BuildGraph(nodes...)
}

// buildPhysicianItemList پزشکان را با جایگاه صفحه‌بندی‌شده در ItemList می‌گذارد.
// ورودی: پزشکان، position هر کدام، و تعداد کل. خروجی: JSON-LD. sameAs و کد ملی نوشته نمی‌شود.
func buildPhysicianItemList(doctors []PhysicianDTO, positions []int, total int) string {
	elements := make([]map[string]interface{}, 0, len(doctors))
	for i, doctor := range doctors {
		if strings.TrimSpace(doctor.Name) == "" || strings.TrimSpace(doctor.URL) == "" {
			continue
		}
		pos := i + 1
		if i < len(positions) && positions[i] > 0 {
			pos = positions[i]
		}
		item := map[string]interface{}{
			"@type": "Physician",
			"name":  doctor.Name,
			"url":   doctor.URL,
		}
		if doctor.ID != "" {
			item["@id"] = doctor.ID
		}
		if doctor.Specialty != "" {
			item["medicalSpecialty"] = doctor.Specialty
		}
		if doctor.ImageURL != "" {
			item["image"] = doctor.ImageURL
		}
		if doctor.ClinicID != "" || doctor.ClinicName != "" {
			worksFor := map[string]interface{}{"@type": "MedicalClinic"}
			if doctor.ClinicID != "" {
				worksFor["@id"] = doctor.ClinicID
			}
			if doctor.ClinicName != "" {
				worksFor["name"] = doctor.ClinicName
			}
			item["worksFor"] = worksFor
		}
		elements = append(elements, map[string]interface{}{
			"@type":    "ListItem",
			"position": pos,
			"item":     item,
		})
	}
	if len(elements) == 0 {
		return ""
	}
	data := map[string]interface{}{
		"@context":        "https://schema.org",
		"@type":           "ItemList",
		"name":            "پزشکان",
		"itemListElement": elements,
	}
	if total > 0 {
		data["numberOfItems"] = total
	}
	return MinifyJSONLD(data)
}

// NewsBreadcrumbGraph مسیر اخبار را بدون Article می‌سازد.
// ورودی: URL خانه، فهرست، و در صورت جزئیات عنوان و URL خبر. خروجی: @graph.
func NewsBreadcrumbGraph(homeURL, listURL, title, detailURL string) string {
	items := []BreadcrumbItemDTO{
		{Name: "خانه", URL: homeURL},
		{Name: "اخبار", URL: listURL},
	}
	title = strings.TrimSpace(title)
	detailURL = strings.TrimSpace(detailURL)
	if title != "" && detailURL != "" {
		items = append(items, BreadcrumbItemDTO{Name: title, URL: detailURL})
	}
	return BuildGraph(BuildBreadcrumbSchema(items))
}

// OrganizationClinicRef شناسه و URL مرکز را روی دامنهٔ سازمان می‌سازد.
// ورودی: مبدأ و اسلاگ ذخیره‌شده. خروجی: @id بدون page و URL لندینگ، یا خالی اگر اسلاگ عمومی نباشد.
func OrganizationClinicRef(baseURL, slug string) (id, pageURL string) {
	path := ClinicPath(slug)
	if path == "" {
		return "", ""
	}
	pageURL = AbsoluteURL(baseURL, path)
	if pageURL == "" {
		return "", ""
	}
	return PageFragmentID(pageURL, "clinic"), pageURL
}

// AttachOrganizationSurface هویت سازمان و وب‌سایت را به گراف صفحه اضافه می‌کند.
// ورودی: JSON-LD موجود، مبدأ، نام، توضیح و لوگوی خود سازمان. خروجی: همان گراف به‌همراه گره‌های ثابت.
// دادهٔ مرکز، تلفن، آدرس و legalName ساخته نمی‌شود. شناسه‌ها روی ریشهٔ هاست می‌مانند.
func AttachOrganizationSurface(existing, origin, name, description, logo string) string {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	name = strings.TrimSpace(name)
	if origin == "" || name == "" {
		return existing
	}
	orgID := OriginID(origin, "organization")
	siteID := OriginID(origin, "website")
	org := map[string]interface{}{
		"@type": "Organization",
		"@id":   orgID,
		"name":  name,
		"url":   origin + "/",
	}
	if text := strings.TrimSpace(description); text != "" {
		org["description"] = text
	}
	if strings.TrimSpace(logo) != "" {
		org["logo"] = strings.TrimSpace(logo)
	}
	site := map[string]interface{}{
		"@type":     "WebSite",
		"@id":       siteID,
		"url":       origin + "/",
		"name":      name,
		"publisher": map[string]interface{}{"@id": orgID},
	}
	nodes, ok := organizationGraphNodes(existing)
	if !ok {
		return existing
	}
	if !graphHasID(nodes, orgID) {
		nodes = append(nodes, org)
	}
	if !graphHasID(nodes, siteID) {
		nodes = append(nodes, site)
	}
	for i := range nodes {
		if graphType(nodes[i]) != "WebPage" {
			continue
		}
		if _, exists := nodes[i]["publisher"]; !exists {
			nodes[i]["publisher"] = map[string]interface{}{"@id": orgID}
		}
	}
	return MinifyJSONLD(map[string]interface{}{
		"@context": "https://schema.org",
		"@graph":   nodes,
	})
}

// organizationGraphNodes گره‌های @graph موجود را برمی‌گرداند.
// ورودی: JSON-LD یا رشتهٔ خالی. خروجی: گره‌ها و true. JSON نامعتبر false است تا گراف صفحه دور ریخته نشود.
func organizationGraphNodes(existing string) ([]map[string]interface{}, bool) {
	existing = strings.TrimSpace(existing)
	if existing == "" {
		return nil, true
	}
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(existing), &doc); err != nil {
		return nil, false
	}
	raw, ok := doc["@graph"].([]interface{})
	if !ok {
		return nil, false
	}
	nodes := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		node, ok := item.(map[string]interface{})
		if !ok {
			return nil, false
		}
		nodes = append(nodes, node)
	}
	return nodes, true
}

// graphHasID می‌گوید گرهٔ با این @id از قبل در گراف هست یا نه.
// ورودی: گره‌ها و شناسه. خروجی: true در صورت وجود.
func graphHasID(nodes []map[string]interface{}, id string) bool {
	for _, node := range nodes {
		if value, ok := node["@id"].(string); ok && value == id {
			return true
		}
	}
	return false
}

// graphType نوع Schema.org گره را برمی‌گرداند.
// ورودی: گره. خروجی: @type رشته‌ای یا خالی.
func graphType(node map[string]interface{}) string {
	value, _ := node["@type"].(string)
	return value
}
