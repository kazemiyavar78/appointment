package components

import "strings"

// SpecialtyCardBannerTestProps props کارت بنر تخصص را از داده‌های واقعی می‌سازد.
// ورودی: view تخصص و اندیس برای انیمیشن. خروجی: props نمایشی کارت flip.
func SpecialtyCardBannerTestProps(view SpecialtyCardView, index int) SpecialtyCardProps {
	desc := strings.TrimSpace(view.ShortDescription)
	if desc == "" {
		desc = strings.TrimSpace(view.Description)
	}
	if desc == "" {
		desc = "خدمات تخصصی " + view.Name
	}
	return SpecialtyCardProps{
		ID:            view.ID,
		NameSpecialty: view.Name,
		StaggerIndex:  index,
		Description:   desc,
		TopImage:      view.Icon,
	}
}
