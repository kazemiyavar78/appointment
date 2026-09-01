package seo

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PostalAddressDTO داده‌های آدرس پستی را برای اسکیما نگه‌داری می‌کند.
type PostalAddressDTO struct {
	StreetAddress   string `json:"streetAddress,omitempty"`
	AddressLocality string `json:"addressLocality,omitempty"`
	AddressRegion   string `json:"addressRegion,omitempty"`
	AddressCountry  string `json:"addressCountry,omitempty"`
}

// OpeningHoursDTO داده‌های بازه‌های زمانی کارکرد را نگه می‌دارد.
type OpeningHoursDTO struct {
	DaysOfWeek []string `json:"dayOfWeek"`
	Opens      string   `json:"opens"`
	Closes     string   `json:"closes"`
}

// SubOrgDTO مشخصات مختصر کلینیک یا ارگان زیرمجموعه را نگه می‌دارد.
type SubOrgDTO struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// MedicalClinicDTO مدل ورودی برای تولید اسکیمای MedicalClinic است.
type MedicalClinicDTO struct {
	ID                 string             `json:"@id,omitempty"`
	Name               string             `json:"name"`
	URL                string             `json:"url"`
	LogoURL            string             `json:"logo,omitempty"`
	ImageURL           string             `json:"image,omitempty"`
	Telephone          string             `json:"telephone,omitempty"`
	Specialties        []string           `json:"medicalSpecialty,omitempty"`
	Address            *PostalAddressDTO  `json:"address,omitempty"`
	ParentOrganization *SubOrgDTO         `json:"parentOrganization,omitempty"`
	OpeningHours       []OpeningHoursDTO  `json:"openingHoursSpecification,omitempty"`
}

// PhysicianDTO مدل ورودی برای تولید اسکیمای Physician است.
type PhysicianDTO struct {
	ID             string            `json:"@id,omitempty"`
	Name           string            `json:"name"`
	ImageURL       string            `json:"image,omitempty"`
	Specialty      string            `json:"medicalSpecialty,omitempty"`
	JobTitle       string            `json:"jobTitle,omitempty"`
	MedicalCouncil string            `json:"medicalCouncilID,omitempty"`
	ClinicName     string            `json:"clinicName,omitempty"`
	ClinicURL      string            `json:"clinicURL,omitempty"`
	URL            string            `json:"url,omitempty"`
	HoursAvailable []OpeningHoursDTO `json:"hoursAvailable,omitempty"`
}

// ItemListElementDTO یک آیتم منفرد در فهرست ساختاریافته است.
type ItemListElementDTO struct {
	Position int    `json:"position"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Type     string `json:"type,omitempty"`
}

// ArticleDTO مدل ورودی برای تولید اسکیمای Article پیام یا خبر است.
type ArticleDTO struct {
	Headline      string `json:"headline"`
	DatePublished string `json:"datePublished,omitempty"`
	DateModified  string `json:"dateModified,omitempty"`
	URL           string `json:"url"`
	AuthorName    string `json:"authorName,omitempty"`
	AuthorJob     string `json:"authorJob,omitempty"`
	AuthorCouncil string `json:"authorCouncil,omitempty"`
	PublisherName string `json:"publisherName,omitempty"`
	PublisherLogo string `json:"publisherLogo,omitempty"`
	Description   string `json:"description,omitempty"`
}

// EquipmentItemDTO مدل ورودی برای معرفی یک تجهیز پزشکی است.
type EquipmentItemDTO struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Category     string `json:"category,omitempty"`
	ImageURL     string `json:"image,omitempty"`
}

// BreadcrumbItemDTO آیتم‌های ناوبری خرده‌نانی را نگه می‌دارد.
type BreadcrumbItemDTO struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// MinifyJSONLD یک ساختار داده را به رشته JSON فشرده و بدون فاصله اضافی تبدیل می‌کند.
// ورودی: شیء داده با ساختار استاندارد. خروجی: رشته JSON فشرده یا رشته خالی در صورت خطا.
func MinifyJSONLD(v interface{}) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(bytes)
}

// BuildMedicalClinicSchema اسکیمای استاندارد JSON-LD برای کلینیک یا درمانگاه تولید می‌کند.
// ورودی: شیء MedicalClinicDTO حاوی مشخصات مرکز. خروجی: رشته JSON-LD معتبر Schema.org.
func BuildMedicalClinicSchema(dto MedicalClinicDTO) string {
	data := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "MedicalClinic",
		"name":     dto.Name,
		"url":      dto.URL,
	}
	if dto.ID != "" {
		data["@id"] = dto.ID
	}
	if dto.LogoURL != "" {
		data["logo"] = dto.LogoURL
	}
	if dto.ImageURL != "" {
		data["image"] = dto.ImageURL
	}
	if dto.Telephone != "" {
		data["telephone"] = dto.Telephone
	}
	if len(dto.Specialties) > 0 {
		data["medicalSpecialty"] = dto.Specialties
	}
	if dto.Address != nil {
		addr := map[string]interface{}{
			"@type": "PostalAddress",
		}
		if dto.Address.StreetAddress != "" {
			addr["streetAddress"] = dto.Address.StreetAddress
		}
		if dto.Address.AddressLocality != "" {
			addr["addressLocality"] = dto.Address.AddressLocality
		}
		if dto.Address.AddressRegion != "" {
			addr["addressRegion"] = dto.Address.AddressRegion
		}
		if dto.Address.AddressCountry != "" {
			addr["addressCountry"] = dto.Address.AddressCountry
		}
		data["address"] = addr
	}
	if dto.ParentOrganization != nil && dto.ParentOrganization.Name != "" {
		data["parentOrganization"] = map[string]interface{}{
			"@type": "MedicalOrganization",
			"name":  dto.ParentOrganization.Name,
			"url":   dto.ParentOrganization.URL,
		}
	}
	if len(dto.OpeningHours) > 0 {
		specs := make([]map[string]interface{}, 0, len(dto.OpeningHours))
		for _, oh := range dto.OpeningHours {
			spec := map[string]interface{}{
				"@type":     "OpeningHoursSpecification",
				"dayOfWeek": oh.DaysOfWeek,
				"opens":     oh.Opens,
				"closes":    oh.Closes,
			}
			specs = append(specs, spec)
		}
		data["openingHoursSpecification"] = specs
	}
	return MinifyJSONLD(data)
}

// BuildMedicalOrganizationSchema اسکیمای استاندارد JSON-LD برای ارگان مادر و هلدینگ درمانی تولید می‌کند.
// ورودی: نام ارگان، آدرس URL، لوگو، و لیست مراکز تابعه. خروجی: رشته JSON-LD معتبر Schema.org.
func BuildMedicalOrganizationSchema(name, orgURL, logoURL string, subOrgs []SubOrgDTO) string {
	data := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "MedicalOrganization",
		"name":     name,
		"url":      orgURL,
	}
	if logoURL != "" {
		data["logo"] = logoURL
	}
	if len(subOrgs) > 0 {
		subs := make([]map[string]interface{}, 0, len(subOrgs))
		for _, s := range subOrgs {
			subs = append(subs, map[string]interface{}{
				"@type": "MedicalClinic",
				"name":  s.Name,
				"url":   s.URL,
			})
		}
		data["subOrganization"] = subs
	}
	return MinifyJSONLD(data)
}

// BuildPhysicianSchema اسکیمای استاندارد JSON-LD برای پزشک به همراه شماره نظام و کلینیک کارفرما می‌سازد.
// ورودی: شیء PhysicianDTO حاوی اطلاعات فردی و حرفه‌ای پزشک. خروجی: رشته JSON-LD معتبر Schema.org.
func BuildPhysicianSchema(dto PhysicianDTO) string {
	data := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Physician",
		"name":     dto.Name,
	}
	if dto.ID != "" {
		data["@id"] = dto.ID
	}
	if dto.ImageURL != "" {
		data["image"] = dto.ImageURL
	}
	if dto.Specialty != "" {
		data["medicalSpecialty"] = dto.Specialty
	}
	if dto.JobTitle != "" {
		data["jobTitle"] = dto.JobTitle
	}
	if dto.URL != "" {
		data["url"] = dto.URL
	}
	if dto.MedicalCouncil != "" {
		data["identifier"] = map[string]interface{}{
			"@type":      "PropertyValue",
			"name":       "شماره نظام پزشکی",
			"propertyID": "MedicalCouncilID",
			"value":      dto.MedicalCouncil,
		}
	}
	if dto.ClinicName != "" {
		worksFor := map[string]interface{}{
			"@type": "MedicalClinic",
			"name":  dto.ClinicName,
		}
		if dto.ClinicURL != "" {
			worksFor["url"] = dto.ClinicURL
		}
		data["worksFor"] = worksFor
	}
	if len(dto.HoursAvailable) > 0 {
		hours := make([]map[string]interface{}, 0, len(dto.HoursAvailable))
		for _, ha := range dto.HoursAvailable {
			hours = append(hours, map[string]interface{}{
				"@type":     "OpeningHoursSpecification",
				"dayOfWeek": ha.DaysOfWeek,
				"opens":     ha.Opens,
				"closes":    ha.Closes,
			})
		}
		data["hoursAvailable"] = hours
	}
	return MinifyJSONLD(data)
}

// BuildItemListSchema اسکیمای فهرست ساختاریافته از عناصر (نظیر لیست پزشکان یا تخصص‌ها) تولید می‌کند.
// ورودی: نام لیست و آرایه‌ای از آیتم‌های دارای موقعیت و پیوند. خروجی: رشته JSON-LD معتبر Schema.org.
func BuildItemListSchema(listName string, items []ItemListElementDTO) string {
	elements := make([]map[string]interface{}, 0, len(items))
	for idx, it := range items {
		pos := it.Position
		if pos == 0 {
			pos = idx + 1
		}
		itemType := it.Type
		if itemType == "" {
			itemType = "Thing"
		}
		elements = append(elements, map[string]interface{}{
			"@type":    "ListItem",
			"position": pos,
			"item": map[string]interface{}{
				"@type": itemType,
				"name":  it.Name,
				"url":   it.URL,
			},
		})
	}
	data := map[string]interface{}{
		"@context":        "https://schema.org",
		"@type":           "ItemList",
		"name":            listName,
		"numberOfItems":   len(items),
		"itemListElement": elements,
	}
	return MinifyJSONLD(data)
}

// BuildArticleSchema اسکیمای ساختاریافته برای پیام به مراجعین یا اخبار پزشکی ایجاد می‌کند.
// ورودی: شیء ArticleDTO حاوی تیتر، متن، تاریخ و مؤلف. خروجی: رشته JSON-LD معتبر Schema.org.
func BuildArticleSchema(dto ArticleDTO) string {
	data := map[string]interface{}{
		"@context":         "https://schema.org",
		"@type":            "Article",
		"headline":         dto.Headline,
		"mainEntityOfPage": dto.URL,
	}
	if dto.DatePublished != "" {
		data["datePublished"] = dto.DatePublished
	}
	if dto.DateModified != "" {
		data["dateModified"] = dto.DateModified
	}
	if dto.Description != "" {
		data["description"] = dto.Description
	}
	if dto.AuthorName != "" {
		author := map[string]interface{}{
			"@type": "Person",
			"name":  dto.AuthorName,
		}
		if dto.AuthorJob != "" {
			author["jobTitle"] = dto.AuthorJob
		}
		if dto.AuthorCouncil != "" {
			author["identifier"] = map[string]interface{}{
				"@type": "PropertyValue",
				"name":  "شماره نظام پزشکی",
				"value": dto.AuthorCouncil,
			}
		}
		data["author"] = author
	}
	if dto.PublisherName != "" {
		publisher := map[string]interface{}{
			"@type": "MedicalClinic",
			"name":  dto.PublisherName,
		}
		if dto.PublisherLogo != "" {
			publisher["logo"] = dto.PublisherLogo
		}
		data["publisher"] = publisher
	}
	return MinifyJSONLD(data)
}

// BuildEquipmentListSchema اسکیمای تجهیزات پزشکی و تشخیصی مرکز درمانی را برمی‌گرداند.
// ورودی: نام مرکز، پیوند صفحه تجهیزات و لیست تجهیزات. خروجی: رشته JSON-LD معتبر Schema.org.
func BuildEquipmentListSchema(clinicName, pageURL string, items []EquipmentItemDTO) string {
	devices := make([]map[string]interface{}, 0, len(items))
	for _, it := range items {
		dev := map[string]interface{}{
			"@type":       "MedicalDevice",
			"name":        it.Name,
			"description": it.Description,
		}
		if it.Manufacturer != "" {
			dev["manufacturer"] = it.Manufacturer
		}
		if it.Category != "" {
			dev["category"] = it.Category
		}
		if it.ImageURL != "" {
			dev["image"] = it.ImageURL
		}
		devices = append(devices, dev)
	}
	data := map[string]interface{}{
		"@context":   "https://schema.org",
		"@type":      "MedicalBusiness",
		"name":       fmt.Sprintf("تجهیزات و فناوری‌های درمانی %s", clinicName),
		"url":        pageURL,
		"department": devices,
	}
	return MinifyJSONLD(data)
}

// BuildBreadcrumbSchema اسکیمای سلسله‌مراتب خرده‌نانی صفحات سایت را ایجاد می‌کند.
// ورودی: آرایه‌ای از آیتم‌های مسیر BreadcrumbItemDTO. خروجی: رشته JSON-LD معتبر Schema.org.
func BuildBreadcrumbSchema(items []BreadcrumbItemDTO) string {
	elements := make([]map[string]interface{}, 0, len(items))
	for idx, it := range items {
		elements = append(elements, map[string]interface{}{
			"@type":    "ListItem",
			"position": idx + 1,
			"name":     it.Name,
			"item":     it.URL,
		})
	}
	data := map[string]interface{}{
		"@context":        "https://schema.org",
		"@type":           "BreadcrumbList",
		"itemListElement": elements,
	}
	return MinifyJSONLD(data)
}

// CombineSchemas چندین رشته JSON-LD را داخل یک آرایه مشترک ترکیب می‌کند.
// ورودی: یک یا چند رشته JSON-LD. خروجی: رشته ادغام شده معتبر JSON-LD.
func CombineSchemas(schemas ...string) string {
	var valid []string
	for _, s := range schemas {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			valid = append(valid, trimmed)
		}
	}
	if len(valid) == 0 {
		return ""
	}
	if len(valid) == 1 {
		return valid[0]
	}
	return "[" + strings.Join(valid, ",") + "]"
}
