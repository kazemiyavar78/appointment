package seo

import (
	"strings"

	"tebpardaz/server/internal/models"
)

// SectionDetailContent سیگنال‌های محتوای عمومی صفحهٔ اصلی بخش است، نه زیرصفحه‌ها.
// ساعات پیش‌فرض، نام بخش، و متن قالب به‌تنهایی ایندکس نمی‌سازند.
type SectionDetailContent struct {
	Title           string
	HasBanner       bool
	Slogan          string
	Description     string
	ServiceLines    []string
	PublicDoctors   int
	CatalogServices int
	EquipmentTitles []string
	MessageBodies   []string
}

// ApplySupportingSectionPolicy زیرصفحهٔ پشتیبان بخش را از ایندکس خارج می‌کند.
// ورودی: متادیتای ساخته‌شده با canonical خودش. خروجی: همان title و canonical با robots برابر noindex,follow.
// صفحهٔ اصلی بخش از این تابع استفاده نمی‌کند و همچنان SectionDetailIndexable است.
func ApplySupportingSectionPolicy(meta Meta) Meta {
	meta.Robots = RobotsNoindexFollow
	return meta
}

// SectionDetailIndexable می‌گوید صفحهٔ اصلی بخش محتوای عمومی متمایز دارد یا نه.
// ورودی: عنوان بخش و سیگنال‌های از قبل بارگذاری‌شده. خروجی: true فقط با پزشک، خدمت، تجهیز، پیام، یا متن بنر غیرقالب.
// شعار، توضیح و خدمات پیش‌فرض DefaultSectionBanner، و ساعات کاری، کافی نیستند.
func SectionDetailIndexable(in SectionDetailContent) bool {
	if in.PublicDoctors > 0 || in.CatalogServices > 0 {
		return true
	}
	for _, title := range in.EquipmentTitles {
		if strings.TrimSpace(title) != "" {
			return true
		}
	}
	for _, body := range in.MessageBodies {
		if strings.TrimSpace(body) != "" {
			return true
		}
	}
	if !in.HasBanner {
		return false
	}
	def := models.DefaultSectionBanner(0, in.Title)
	if distinctText(in.Description, def.Description) || distinctText(in.Slogan, def.Slogan) {
		return true
	}
	return hasCustomServiceLine(in.ServiceLines, def.Services)
}

// distinctText متن ذخیره‌شده را با مقدار قالب مقایسه می‌کند.
// ورودی: متن صفحه و متن پیش‌فرض. خروجی: true وقتی متن غیرخالی و با پیش‌فرض یکی نیست.
func distinctText(value, fallback string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return value != strings.TrimSpace(fallback)
}

// hasCustomServiceLine می‌گوید حداقل یک خط خدمت خارج از فهرست قالب وجود دارد یا نه.
// ورودی: خطوط ذخیره‌شده و متن خدمات پیش‌فرض. خروجی: true فقط برای خطی که در مجموعهٔ پیش‌فرض نیست.
func hasCustomServiceLine(lines []string, fallback string) bool {
	defaults := serviceLineSet(fallback)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := defaults[line]; !ok {
			return true
		}
	}
	return false
}

// serviceLineSet خطوط غیرخالی را به مجموعه تبدیل می‌کند.
// ورودی: متن چندخطی. خروجی: مجموعهٔ خطوط trimشده.
func serviceLineSet(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out[line] = struct{}{}
	}
	return out
}
