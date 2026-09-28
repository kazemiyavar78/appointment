package seo

import (
	"encoding/json"
	"strings"
)

// PostalAddressDTO داده‌های آدرس پستی را برای اسکیما نگه‌داری می‌کند.
type PostalAddressDTO struct {
	StreetAddress   string `json:"streetAddress,omitempty"`
	AddressLocality string `json:"addressLocality,omitempty"`
	AddressRegion   string `json:"addressRegion,omitempty"`
	AddressCountry  string `json:"addressCountry,omitempty"`
}

// SubOrgDTO مشخصات مختصر کلینیک یا ارگان زیرمجموعه را نگه می‌دارد.
type SubOrgDTO struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// MedicalClinicDTO مدل ورودی برای تولید اسکیمای MedicalClinic است.
type MedicalClinicDTO struct {
	ID                 string            `json:"@id,omitempty"`
	Name               string            `json:"name"`
	URL                string            `json:"url"`
	Description        string            `json:"description,omitempty"`
	LogoURL            string            `json:"logo,omitempty"`
	ImageURL           string            `json:"image,omitempty"`
	Telephone          string            `json:"telephone,omitempty"`
	Specialties        []string          `json:"medicalSpecialty,omitempty"`
	Address            *PostalAddressDTO `json:"address,omitempty"`
	ParentOrganization *SubOrgDTO        `json:"parentOrganization,omitempty"`
}

// PhysicianDTO مدل ورودی برای تولید اسکیمای Physician است.
type PhysicianDTO struct {
	ID         string `json:"@id,omitempty"`
	Name       string `json:"name"`
	ImageURL   string `json:"image,omitempty"`
	Specialty  string `json:"medicalSpecialty,omitempty"`
	JobTitle   string `json:"jobTitle,omitempty"`
	ClinicID   string `json:"clinicID,omitempty"`
	ClinicName string `json:"clinicName,omitempty"`
	ClinicURL  string `json:"clinicURL,omitempty"`
	URL        string `json:"url,omitempty"`
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
	if desc := strings.TrimSpace(dto.Description); desc != "" {
		data["description"] = desc
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
	// ساعات بخش روی درمانگاه نوشته نمی‌شود؛ درمانگاه ساعت سراسری ندارد.
	if dto.ParentOrganization != nil && dto.ParentOrganization.Name != "" {
		data["parentOrganization"] = map[string]interface{}{
			"@type": "MedicalOrganization",
			"name":  dto.ParentOrganization.Name,
			"url":   dto.ParentOrganization.URL,
		}
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

// BuildPhysicianSchema اسکیمای Physician را از داده واقعی می‌سازد.
// ورودی: PhysicianDTO. خروجی: JSON-LD.
// شماره نظام، کد ملی و ساعات نوبت اینجا نوشته نمی‌شوند.
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
	if dto.ClinicID != "" || dto.ClinicName != "" {
		worksFor := map[string]interface{}{
			"@type": "MedicalClinic",
		}
		if dto.ClinicID != "" {
			worksFor["@id"] = dto.ClinicID
		}
		if dto.ClinicName != "" {
			worksFor["name"] = dto.ClinicName
		}
		if dto.ClinicURL != "" {
			worksFor["url"] = dto.ClinicURL
		}
		data["worksFor"] = worksFor
	}
	return MinifyJSONLD(data)
}

// BuildItemListSchema اسکیمای فهرست ساختاریافته از عناصر (نظیر لیست پزشکان یا تخصص‌ها) تولید می‌کند.
// ورودی: نام لیست، آیتم‌های همین صفحه، و تعداد کل listing. خروجی: رشته JSON-LD معتبر Schema.org.
// اگر total مثبت نباشد numberOfItems حذف می‌شود تا تعداد همین صفحه به‌جای کل نتایج ننشیند.
func BuildItemListSchema(listName string, items []ItemListElementDTO, total int) string {
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
		"itemListElement": elements,
	}
	if total > 0 {
		data["numberOfItems"] = total
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

// equipmentBadgePlaceholder مقدار پیش‌فرض مدل تجهیزات است و دستهٔ واقعی دستگاه نیست.
const equipmentBadgePlaceholder = "فناوری روز دنیا"

// BuildEquipmentListSchema فهرست MedicalDevice را از تجهیزات همان صفحه می‌سازد.
// ورودی: URL صفحه و آیتم‌ها. خروجی: ItemList، یا خالی اگر دستگاهی نمانده باشد.
// مرکز را MedicalBusiness معرفی نمی‌کند.
func BuildEquipmentListSchema(pageURL string, items []EquipmentItemDTO) string {
	wrapped := make([]map[string]interface{}, 0, len(items))
	for _, it := range items {
		name := strings.TrimSpace(it.Name)
		if name == "" {
			continue
		}
		dev := map[string]interface{}{
			"@type": "MedicalDevice",
			"name":  name,
		}
		if desc := strings.TrimSpace(it.Description); desc != "" {
			dev["description"] = desc
		}
		if m := strings.TrimSpace(it.Manufacturer); m != "" {
			dev["manufacturer"] = m
		}
		if cat := strings.TrimSpace(it.Category); cat != "" && cat != equipmentBadgePlaceholder {
			dev["category"] = cat
		}
		if img := strings.TrimSpace(it.ImageURL); img != "" {
			dev["image"] = img
		}
		if pageURL != "" {
			dev["url"] = pageURL
		}
		wrapped = append(wrapped, map[string]interface{}{
			"@type":    "ListItem",
			"position": len(wrapped) + 1,
			"item":     dev,
		})
	}
	if len(wrapped) == 0 {
		return ""
	}
	return MinifyJSONLD(map[string]interface{}{
		"@context":        "https://schema.org",
		"@type":           "ItemList",
		"name":            "تجهیزات",
		"numberOfItems":   len(wrapped),
		"itemListElement": wrapped,
	})
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
