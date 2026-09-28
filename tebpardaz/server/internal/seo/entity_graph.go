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
	BaseURL       string
	Canonical     string
	Title         string
	Description   string
	DoctorName    string
	DoctorSlug    string
	Specialty     string
	PhotoURL      string
	UseClinicLogo bool
	ClinicName    string
	ClinicOrigin  string
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
	clinicID := ""
	if origin := strings.TrimRight(strings.TrimSpace(in.ClinicOrigin), "/"); origin != "" {
		clinicID = OriginID(origin, "clinic")
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
	return BuildGraph(page, physician, crumb)
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
	return BuildGraph(crumb, BuildItemListSchema("پزشکان", items, total))
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
