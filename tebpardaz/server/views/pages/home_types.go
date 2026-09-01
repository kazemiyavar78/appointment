package pages

import (
	"fmt"
	"time"

	"tebpardaz/server/views/components"
)

// HomeNewsItem یک کارت خبر در صفحه اصلی است.
type HomeNewsItem struct {
	TitleHTML       string
	ExcerptHTML     string
	CoverURL        string
	PublishedAt     time.Time
	DetailURL       string
	ClinicName      string
	ShowClinicBadge bool
}

// HomeView محتوای صفحه اصلی tenant را راهبری می‌کند.
type HomeView struct {
	LatestNews           []HomeNewsItem
	Specialties          []components.SpecialtyCardView
	SpecialtiesListURL   string
	ShowSpecialtiesLink  bool
	Insurances           []components.InsuranceItemView
	Clinics              []components.ClinicCardView
	Doctors              []components.DoctorCardView
	ShowClinicCards     bool
	ShowClinicBadge     bool
	NewsListURL         string
}

// homeNewsToCard آیتم خبر خانه را به NewsCardView نگاشت می‌کند.
func homeNewsToCard(item HomeNewsItem) components.NewsCardView {
	return components.NewsCardView{
		TitleHTML:       item.TitleHTML,
		ExcerptHTML:     item.ExcerptHTML,
		CoverURL:        item.CoverURL,
		PublishedAt:     item.PublishedAt,
		DetailURL:       item.DetailURL,
		ClinicName:      item.ClinicName,
		ShowClinicBadge: item.ShowClinicBadge,
	}
}

// FormatRatingAverage میانگین امتیاز را برای نمایش قالب می‌کند.
func FormatRatingAverage(avg float64) string {
	return fmt.Sprintf("%.1f", avg)
}
