package seo

import (
	"html"
	"regexp"
	"strconv"
	"strings"
)

const (
	// PlatformBrand نام برند دامنه پلتفرم است و برای tenant استفاده نمی‌شود.
	PlatformBrand = "طب‌پرداز"
	// RobotsIndexFollow صفحات پایدار و قابل ایندکس.
	RobotsIndexFollow = "index,follow"
	// RobotsNoindexFollow نتیجه جستجو یا فیلتر موقت؛ لینک‌ها دنبال می‌شوند.
	RobotsNoindexFollow = "noindex,follow"
)

// SiteKind لایه عمومی سایت است: پلتفرم، مرکز خصوصی، یا سازمان.
type SiteKind int

const (
	// SitePlatform دامنه شرکت (tebpardaz.ir) است.
	SitePlatform SiteKind = iota
	// SiteClinic سایت اختصاصی یک مرکز است.
	SiteClinic
	// SiteOrgan سایت سازمان با چند مرکز است.
	SiteOrgan
)

// SectionKind زیرصفحه بخش درمانی است. عنوان هر کدام باید یکتا بماند.
type SectionKind int

const (
	// SectionOverview صفحه اصلی بخش است.
	SectionOverview SectionKind = iota
	// SectionHours ساعات کاری است.
	SectionHours
	// SectionMessages پیام به مراجعین است.
	SectionMessages
	// SectionEquipment تجهیزات است.
	SectionEquipment
	// SectionIntro معرفی بخش است.
	SectionIntro
)

// Meta عنوان، توضیح، canonical و robots یک صفحه عمومی است.
type Meta struct {
	Title       string
	Description string
	Canonical   string
	Robots      string
}

// DoctorListQuery پارامترهای خام لیست پزشکان است. منطق ایندکس اینجاست نه در handler.
type DoctorListQuery struct {
	Q           string
	Date        string
	SpecialtyID string
	Clinic      string
	Page        int
}

// WeeklyQuery پارامترهای خام برنامه هفتگی است.
type WeeklyQuery struct {
	Q        string
	Date     string
	Shift    string
	ClinicID string
}

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

// BrandName نام نمایشی سایت را برمی‌گرداند.
// ورودی: نوع سایت و نام مرکز یا سازمان. خروجی: برای پلتفرم همیشه طب‌پرداز؛ برای بقیه همان نام، یا «مرکز درمانی» اگر نام خالی باشد.
func BrandName(kind SiteKind, name string) string {
	if kind == SitePlatform {
		return PlatformBrand
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "مرکز درمانی"
	}
	return name
}

// PlainText تگ HTML را برمی‌دارد و متن را یک‌خطی می‌کند.
// ورودی: رشته ممکن است HTML باشد. خروجی: متن ساده بدون تگ.
func PlainText(s string) string {
	s = htmlTagPattern.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

// HomeMeta متادیتای صفحه اصلی را می‌سازد.
// ورودی: نوع سایت، نام مرکز یا سازمان، canonical مطلق همان میزبان. خروجی: Meta.
func HomeMeta(kind SiteKind, placeName, canonical string) Meta {
	if kind == SitePlatform {
		return finish(Meta{
			Title:       "نوبت‌دهی آنلاین پزشکان و مراکز درمانی | " + PlatformBrand,
			Description: "جستجوی پزشکان و مراکز درمانی و دریافت نوبت آنلاین از طریق طب‌پرداز.",
			Canonical:   canonical,
			Robots:      RobotsIndexFollow,
		})
	}
	brand := BrandName(kind, placeName)
	return finish(Meta{
		Title:       brand + " | نوبت‌دهی آنلاین پزشکان",
		Description: "نوبت‌دهی آنلاین پزشکان " + brand + ". مشاهده پزشکان، تخصص‌ها، برنامه حضور و رزرو نوبت اینترنتی.",
		Canonical:   canonical,
		Robots:      RobotsIndexFollow,
	})
}

// DoctorsMeta متادیتای لیست پزشکان را با سیاست فیلتر و صفحه‌بندی می‌سازد.
// ورودی: نوع سایت، نام مرکز یا سازمان، مبدأ scheme://host، query خام. خروجی: Meta.
func DoctorsMeta(kind SiteKind, placeName, baseURL string, q DoctorListQuery) Meta {
	brand := BrandName(kind, placeName)
	title := "لیست پزشکان و نوبت‌دهی آنلاین | " + PlatformBrand
	desc := "لیست پزشکان را ببینید، پزشک موردنظر را پیدا کنید و نوبت آنلاین بگیرید."
	if kind != SitePlatform {
		title = "پزشکان " + brand + " | نوبت‌دهی آنلاین"
		desc = "لیست پزشکان " + brand + " را مشاهده کنید، پزشک موردنظر را پیدا کنید و نوبت خود را آنلاین رزرو کنید."
	}
	return finish(Meta{
		Title:       title,
		Description: desc,
		Canonical:   DoctorListCanonical(baseURL, q),
		Robots:      DoctorListRobots(q),
	})
}

// DoctorListRobots ایندکس لیست پزشکان را تعیین می‌کند.
// ورودی: query خام. خروجی: index,follow فقط وقتی فیلتر جستجو نباشد؛ page به‌تنهایی ایندکس می‌ماند.
func DoctorListRobots(q DoctorListQuery) string {
	if q.filtered() {
		return RobotsNoindexFollow
	}
	return RobotsIndexFollow
}

// DoctorListCanonical آدرس کانونیکال لیست پزشکان را می‌سازد.
// ورودی: مبدأ و query. خروجی: URL مطلق. فیلتر به مسیر تمیز برمی‌گردد؛ page بزرگ‌تر از ۱ خودش را نگه می‌دارد.
func DoctorListCanonical(baseURL string, q DoctorListQuery) string {
	clean := AbsoluteURL(baseURL, "/doctors")
	if q.filtered() {
		return clean
	}
	if q.Page > 1 {
		return clean + "?page=" + strconv.Itoa(q.Page)
	}
	return clean
}

// BookingMeta متادیتای صفحه نوبت پزشک را از داده واقعی می‌سازد.
// ورودی: نام نمایشی پزشک، نام تخصص، نام مرکز، canonical مطلق همان مسیر booking.
// خروجی: Meta. تخصص یا مرکز خالی جداکننده اضافه نمی‌گذارد. شماره نظام پزشکی اینجا استفاده نمی‌شود.
func BookingMeta(doctorName, specialty, clinicName, canonical string) Meta {
	name := strings.TrimSpace(doctorName)
	specialty = strings.TrimSpace(specialty)
	clinicName = strings.TrimSpace(clinicName)
	if name == "" {
		name = "پزشک"
	}
	title := joinParts(" | ", "نوبت "+bookingTitleName(name), specialty, clinicName)
	return finish(Meta{
		Title:       title,
		Description: bookingDescription(name, specialty, clinicName),
		Canonical:   canonical,
		Robots:      RobotsIndexFollow,
	})
}

// SectionListMeta متادیتای فهرست بخش‌ها را می‌سازد.
// ورودی: نوع سایت، نام مرکز یا سازمان اگر صفحه به یک مجموعه محدود است، canonical. خروجی: Meta.
func SectionListMeta(kind SiteKind, placeName, canonical string) Meta {
	placeName = strings.TrimSpace(placeName)
	if placeName != "" {
		return finish(Meta{
			Title:       "بخش‌های درمانی " + placeName,
			Description: "بخش‌های درمانی " + placeName + "، ساعات پذیرش و نوبت‌دهی.",
			Canonical:   canonical,
			Robots:      RobotsIndexFollow,
		})
	}
	brand := BrandName(kind, "")
	return finish(Meta{
		Title:       "بخش‌های درمانی | " + brand,
		Description: "بخش‌های درمانی و نوبت‌دهی آنلاین در " + brand + ".",
		Canonical:   canonical,
		Robots:      RobotsIndexFollow,
	})
}

// SectionDetailMeta متادیتای یک بخش یا زیرصفحه آن را می‌سازد.
// ورودی: نوع زیرصفحه، نام بخش، نام مرکز، نشانی فقط برای ساعات کاری، canonical مطلق.
// خروجی: Meta. فیلد خالی در عنوان یا توضیح نمی‌آید.
func SectionDetailMeta(kind SectionKind, sectionName, clinicName, address, canonical string) Meta {
	sectionName = strings.TrimSpace(sectionName)
	clinicName = strings.TrimSpace(clinicName)
	address = strings.TrimSpace(address)
	head := strings.TrimSpace(sectionName + " " + clinicName)
	if head == "" {
		head = "بخش درمانی"
	}
	label := sectionName
	if label == "" {
		label = "درمانی"
	}
	place := ""
	if clinicName != "" {
		place = " در " + clinicName
	}
	titleSuffix := "خدمات و نوبت‌دهی"
	desc := "اطلاعات بخش " + label + place + " و نوبت‌دهی آنلاین."
	switch kind {
	case SectionHours:
		titleSuffix = "ساعات کاری"
		desc = "ساعات کاری بخش " + label + place + "."
		if address != "" {
			desc += " نشانی: " + address + "."
		}
	case SectionMessages:
		titleSuffix = "پیام به مراجعین"
		desc = "پیام‌های بخش " + label + place + " برای مراجعین."
	case SectionEquipment:
		titleSuffix = "تجهیزات"
		desc = "تجهیزات بخش " + label + place + "."
	case SectionIntro:
		titleSuffix = "معرفی"
		desc = "معرفی بخش " + label + place + "."
	}
	return finish(Meta{
		Title:       head + " | " + titleSuffix,
		Description: desc,
		Canonical:   canonical,
		Robots:      RobotsIndexFollow,
	})
}

// NewsListMeta متادیتای فهرست اخبار را می‌سازد.
// ورودی: نوع سایت، نام مرکز یا سازمان، canonical. خروجی: Meta.
func NewsListMeta(kind SiteKind, placeName, canonical string) Meta {
	if kind == SitePlatform {
		return finish(Meta{
			Title:       "اخبار و مطالب پزشکی | " + PlatformBrand,
			Description: "اخبار و مطالب پزشکی مراکز درمانی را در طب‌پرداز بخوانید.",
			Canonical:   canonical,
			Robots:      RobotsIndexFollow,
		})
	}
	brand := BrandName(kind, placeName)
	return finish(Meta{
		Title:       "اخبار و اطلاعیه‌های " + brand,
		Description: "اخبار و اطلاعیه‌های " + brand + " را بخوانید.",
		Canonical:   canonical,
		Robots:      RobotsIndexFollow,
	})
}

// NewsDetailMeta متادیتای یک خبر را از عنوان و خلاصه واقعی می‌سازد.
// ورودی: عنوان و خلاصه (ممکن است HTML باشند)، نام برند یا مرکز، canonical.
// خروجی: Meta. متن کامل خبر داخل description ریخته نمی‌شود.
func NewsDetailMeta(titleHTML, excerptHTML, brand, canonical string) Meta {
	title := PlainText(titleHTML)
	if title == "" {
		title = "خبر"
	}
	brand = strings.TrimSpace(brand)
	if brand == "" {
		brand = PlatformBrand
	}
	desc := PlainText(excerptHTML)
	if desc == "" || desc == title {
		desc = "خبر «" + title + "» در " + brand + "."
	} else {
		desc = truncateRunes(desc, 160)
	}
	return finish(Meta{
		Title:       title + " | " + brand,
		Description: desc,
		Canonical:   canonical,
		Robots:      RobotsIndexFollow,
	})
}

// WeeklyMeta متادیتای برنامه هفتگی را می‌سازد.
// ورودی: نوع سایت، نام مرکز اگر مشخص است، canonical مسیر تمیز، و اینکه query فیلتر دارد یا نه.
// خروجی: Meta. فیلتر q/date/shift/clinic_id باعث noindex می‌شود.
func WeeklyMeta(kind SiteKind, placeName, canonical string, filtered bool) Meta {
	placeName = strings.TrimSpace(placeName)
	title := "برنامه هفتگی پزشکان | " + PlatformBrand
	desc := "برنامه حضور پزشکان و نوبت‌های هفته را در طب‌پرداز ببینید."
	if placeName != "" {
		title = "برنامه هفتگی پزشکان " + placeName
		desc = "برنامه حضور پزشکان و نوبت‌های هفته در " + placeName + " را ببینید."
	} else if kind != SitePlatform {
		brand := BrandName(kind, "")
		title = "برنامه هفتگی پزشکان " + brand
		desc = "برنامه حضور پزشکان و نوبت‌های هفته در " + brand + " را ببینید."
	}
	robots := RobotsIndexFollow
	if filtered {
		robots = RobotsNoindexFollow
	}
	return finish(Meta{
		Title:       title,
		Description: desc,
		Canonical:   canonical,
		Robots:      robots,
	})
}

// WeeklyFiltered گزارش می‌دهد برنامه هفتگی فیلتر موقت دارد یا نه.
// ورودی: query خام. خروجی: true اگر q، date، shift یا clinic_id غیرخالی باشد.
func WeeklyFiltered(q WeeklyQuery) bool {
	return q.filtered()
}

// AboutMeta متادیتای درباره ما را می‌سازد.
// ورودی: نوع سایت، نام، canonical، توضیح واقعی مرکز اگر موجود باشد. خروجی: Meta.
// توضیح پلتفرم متن ثابت صفحه محصول است؛ اگر توضیح مرکز خالی باشد حدس زده نمی‌شود.
func AboutMeta(kind SiteKind, placeName, canonical, aboutText string) Meta {
	brand := BrandName(kind, placeName)
	desc := PlainText(aboutText)
	if kind == SitePlatform {
		desc = "نرم‌افزار جامع کلینیک طب‌پرداز؛ مدیریت یکپارچه پذیرش، نوبت‌دهی، آزمایشگاه، طب تصویری، حسابداری و حقوق مراکز درمانی."
	} else if desc == "" {
		desc = "آشنایی با " + brand + " و نوبت‌دهی آنلاین پزشکان."
	} else {
		desc = truncateRunes(desc, 160)
	}
	return finish(Meta{
		Title:       "درباره ما | " + brand,
		Description: desc,
		Canonical:   canonical,
		Robots:      RobotsIndexFollow,
	})
}

// ContactMeta متادیتای تماس را از تلفن و نشانی واقعی می‌سازد.
// ورودی: نوع سایت، نام، canonical، تلفن و نشانی. خروجی: Meta. فیلد خالی حذف می‌شود.
func ContactMeta(kind SiteKind, placeName, canonical, phone, address string) Meta {
	brand := BrandName(kind, placeName)
	desc := "راه‌های تماس با " + brand + "."
	phone = strings.TrimSpace(phone)
	address = strings.TrimSpace(address)
	if phone != "" {
		desc += " تلفن: " + phone + "."
	}
	if address != "" {
		desc += " نشانی: " + address + "."
	}
	return finish(Meta{
		Title:       "تماس با ما | " + brand,
		Description: truncateRunes(desc, 180),
		Canonical:   canonical,
		Robots:      RobotsIndexFollow,
	})
}

// TermsMeta متادیتای قوانین را می‌سازد.
// ورودی: نوع سایت، نام، canonical. خروجی: Meta.
func TermsMeta(kind SiteKind, placeName, canonical string) Meta {
	brand := BrandName(kind, placeName)
	return finish(Meta{
		Title:       "قوانین و مقررات | " + brand,
		Description: "شرایط استفاده از نوبت‌دهی آنلاین " + brand + ".",
		Canonical:   canonical,
		Robots:      RobotsIndexFollow,
	})
}

// TestResultMeta متادیتای فرم پیگیری جواب آزمایش را می‌سازد.
// ورودی: نوع سایت، نام، canonical مسیر فرم. خروجی: Meta.
func TestResultMeta(kind SiteKind, placeName, canonical string) Meta {
	brand := BrandName(kind, placeName)
	return finish(Meta{
		Title:       "جواب آزمایش | " + brand,
		Description: "پیگیری جواب آزمایش در " + brand + ".",
		Canonical:   canonical,
		Robots:      RobotsIndexFollow,
	})
}

func (q DoctorListQuery) filtered() bool {
	if strings.TrimSpace(q.Q) != "" || strings.TrimSpace(q.Date) != "" || strings.TrimSpace(q.Clinic) != "" {
		return true
	}
	spec := strings.TrimSpace(q.SpecialtyID)
	return spec != "" && spec != "0"
}

func (q WeeklyQuery) filtered() bool {
	if strings.TrimSpace(q.Q) != "" || strings.TrimSpace(q.Date) != "" || strings.TrimSpace(q.Shift) != "" {
		return true
	}
	id := strings.TrimSpace(q.ClinicID)
	return id != "" && id != "0"
}

// bookingTitleName پیشوند دکتر را فقط وقتی نام خودش آن را ندارد اضافه می‌کند.
func bookingTitleName(name string) string {
	if strings.Contains(name, "دکتر") || strings.Contains(name, "دكتر") {
		return name
	}
	if name == "پزشک" {
		return name
	}
	return "دکتر " + name
}

func bookingDescription(name, specialty, clinic string) string {
	switch {
	case specialty != "" && clinic != "":
		return "مشاهده نوبت‌های " + name + "، " + specialty + " در " + clinic + " و رزرو نوبت آنلاین."
	case specialty != "":
		return "مشاهده نوبت‌های " + name + "، " + specialty + " و رزرو نوبت آنلاین."
	case clinic != "":
		return "مشاهده نوبت‌های " + name + " در " + clinic + " و رزرو نوبت آنلاین."
	default:
		return "مشاهده نوبت‌های " + name + " و رزرو نوبت آنلاین."
	}
}

func joinParts(sep string, parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, sep)
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if max < 1 || len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max])) + "…"
}

func finish(m Meta) Meta {
	m.Title = strings.TrimSpace(m.Title)
	m.Description = strings.TrimSpace(m.Description)
	m.Canonical = strings.TrimSpace(m.Canonical)
	if m.Robots == "" {
		m.Robots = RobotsIndexFollow
	}
	if m.Description == "" || m.Description == m.Title {
		m.Description = m.Title + "."
	}
	return m
}
