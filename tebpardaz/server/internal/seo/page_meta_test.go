package seo

import (
	"strings"
	"testing"
)

func TestHomeMetaPlatform(t *testing.T) {
	meta := HomeMeta(SitePlatform, "درمانگاه چمران", "https://tebpardaz.ir/")
	if meta.Title != "نوبت‌دهی آنلاین پزشکان و مراکز درمانی | طب‌پرداز" {
		t.Fatalf("title = %q", meta.Title)
	}
	if meta.Description == "" || meta.Description == meta.Title {
		t.Fatalf("description = %q", meta.Description)
	}
	if meta.Canonical != "https://tebpardaz.ir/" {
		t.Fatalf("canonical = %q", meta.Canonical)
	}
	if meta.Robots != RobotsIndexFollow {
		t.Fatalf("robots = %q", meta.Robots)
	}
	if strings.Contains(meta.Title, "چمران") || strings.Contains(meta.Canonical, "chamran") {
		t.Fatalf("platform home picked up tenant data: %+v", meta)
	}
}

func TestHomeMetaTenantIsolation(t *testing.T) {
	meta := HomeMeta(SiteClinic, "درمانگاه چمران مشهد", "https://chamranclinic.ir/")
	if !strings.Contains(meta.Title, "درمانگاه چمران مشهد") {
		t.Fatalf("title = %q", meta.Title)
	}
	if strings.Contains(meta.Title, "طب‌پرداز") || strings.Contains(meta.Canonical, "tebpardaz.ir") {
		t.Fatalf("tenant home leaked platform brand: %+v", meta)
	}
	if meta.Robots != RobotsIndexFollow {
		t.Fatalf("robots = %q", meta.Robots)
	}
	other := HomeMeta(SiteClinic, "بیمارستان رضوی", "https://razavi.example/")
	if strings.Contains(meta.Title, "رضوی") || strings.Contains(other.Canonical, "chamranclinic.ir") {
		t.Fatalf("tenants mixed: %q vs %q", meta.Title, other.Canonical)
	}
}

func TestDoctorsMetaIndexAndPagination(t *testing.T) {
	base := "https://chamranclinic.ir"
	clean := DoctorsMeta(SiteClinic, "درمانگاه چمران مشهد", base, DoctorListQuery{})
	if clean.Robots != RobotsIndexFollow {
		t.Fatalf("robots = %q", clean.Robots)
	}
	if clean.Canonical != "https://chamranclinic.ir/doctors" {
		t.Fatalf("canonical = %q", clean.Canonical)
	}
	if !strings.Contains(clean.Title, "درمانگاه چمران مشهد") || strings.Contains(clean.Title, "طب‌پرداز") {
		t.Fatalf("title = %q", clean.Title)
	}

	page := DoctorsMeta(SiteClinic, "درمانگاه چمران مشهد", base, DoctorListQuery{Page: 2})
	if page.Robots != RobotsIndexFollow {
		t.Fatalf("page robots = %q", page.Robots)
	}
	if page.Canonical != "https://chamranclinic.ir/doctors?page=2" {
		t.Fatalf("page canonical = %q", page.Canonical)
	}

	platform := DoctorsMeta(SitePlatform, "چمران", "https://tebpardaz.ir", DoctorListQuery{})
	if platform.Title != "لیست پزشکان و نوبت‌دهی آنلاین | طب‌پرداز" {
		t.Fatalf("platform title = %q", platform.Title)
	}
	if strings.Contains(platform.Canonical, "chamran") {
		t.Fatalf("platform canonical = %q", platform.Canonical)
	}
}

func TestDoctorsMetaFiltersAreNoindex(t *testing.T) {
	base := "https://tebpardaz.ir"
	cases := []DoctorListQuery{
		{Q: "علی"},
		{Date: "1404/01/01"},
		{SpecialtyID: "12"},
		{Clinic: "chamran"},
		{Q: "علی", Page: 2},
		{SpecialtyID: "4", Page: 3},
	}
	for _, q := range cases {
		meta := DoctorsMeta(SitePlatform, "", base, q)
		if meta.Robots != RobotsNoindexFollow {
			t.Fatalf("query %+v robots = %q", q, meta.Robots)
		}
		if meta.Canonical != "https://tebpardaz.ir/doctors" {
			t.Fatalf("query %+v canonical = %q", q, meta.Canonical)
		}
	}
	if DoctorsMeta(SitePlatform, "", base, DoctorListQuery{SpecialtyID: "0"}).Robots != RobotsIndexFollow {
		t.Fatal("specialty_id=0 should stay indexable")
	}
}

func TestBookingMetaUsesRealFields(t *testing.T) {
	meta := BookingMeta("رضا احمدی", "قلب و عروق", "درمانگاه چمران مشهد", "https://chamranclinic.ir/booking/reza-ahmadi")
	if meta.Title != "نوبت دکتر رضا احمدی | قلب و عروق | درمانگاه چمران مشهد" {
		t.Fatalf("title = %q", meta.Title)
	}
	if meta.Description == "" || meta.Description == meta.Title {
		t.Fatalf("description = %q", meta.Description)
	}
	if !strings.Contains(meta.Description, "رضا احمدی") || !strings.Contains(meta.Description, "قلب و عروق") || !strings.Contains(meta.Description, "چمران") {
		t.Fatalf("description = %q", meta.Description)
	}
	if meta.Canonical != "https://chamranclinic.ir/booking/reza-ahmadi" {
		t.Fatalf("canonical = %q", meta.Canonical)
	}
	if meta.Robots != RobotsIndexFollow {
		t.Fatalf("robots = %q", meta.Robots)
	}
	if strings.Contains(meta.Canonical, "tebpardaz.ir") {
		t.Fatalf("canonical crossed domain: %q", meta.Canonical)
	}
}

func TestBookingMetaEmptySpecialtyHasNoBrokenSeparator(t *testing.T) {
	meta := BookingMeta("دکتر سارا رضایی", "", "درمانگاه چمران", "https://chamranclinic.ir/booking/sara")
	if meta.Title != "نوبت دکتر سارا رضایی | درمانگاه چمران" {
		t.Fatalf("title = %q", meta.Title)
	}
	if strings.Contains(meta.Title, "| |") || strings.HasSuffix(meta.Title, "|") || strings.Contains(meta.Title, "دکتر دکتر") {
		t.Fatalf("broken title = %q", meta.Title)
	}
	if strings.Contains(meta.Description, "،  ") {
		t.Fatalf("description = %q", meta.Description)
	}

	nameOnly := BookingMeta("احمدی", "", "", "https://tebpardaz.ir/booking/chamran/ahmadi")
	if nameOnly.Title != "نوبت دکتر احمدی" {
		t.Fatalf("title = %q", nameOnly.Title)
	}
	if strings.Contains(nameOnly.Title, "|") {
		t.Fatalf("separator without data: %q", nameOnly.Title)
	}
}

func TestNewsDetailMeta(t *testing.T) {
	meta := NewsDetailMeta("<p>افتتاح بخش تصویربرداری</p>", "<p>خلاصه واقعی خبر افتتاح.</p>", "درمانگاه چمران مشهد", "https://chamranclinic.ir/news/15")
	if meta.Title != "افتتاح بخش تصویربرداری | درمانگاه چمران مشهد" {
		t.Fatalf("title = %q", meta.Title)
	}
	if !strings.Contains(meta.Description, "خلاصه واقعی") {
		t.Fatalf("description = %q", meta.Description)
	}
	if meta.Canonical != "https://chamranclinic.ir/news/15" || meta.Robots != RobotsIndexFollow {
		t.Fatalf("meta = %+v", meta)
	}
	if strings.Contains(meta.Title, "طب‌پرداز") || strings.Contains(meta.Canonical, "tebpardaz.ir") {
		t.Fatalf("news leaked platform: %+v", meta)
	}

	fallback := NewsDetailMeta("اطلاعیه", "", "طب‌پرداز", "https://tebpardaz.ir/news/2")
	if fallback.Title != "اطلاعیه | طب‌پرداز" {
		t.Fatalf("title = %q", fallback.Title)
	}
	if strings.Contains(fallback.Description, "<") || fallback.Description == fallback.Title {
		t.Fatalf("description = %q", fallback.Description)
	}
	if len([]rune(fallback.Description)) > 200 {
		t.Fatalf("fallback dumped long text: %q", fallback.Description)
	}
}

func TestNewsListAndWeeklyAndStatic(t *testing.T) {
	news := NewsListMeta(SiteClinic, "درمانگاه چمران مشهد", "https://chamranclinic.ir/news")
	if news.Title != "اخبار و اطلاعیه‌های درمانگاه چمران مشهد" {
		t.Fatalf("news title = %q", news.Title)
	}
	platformNews := NewsListMeta(SitePlatform, "چمران", "https://tebpardaz.ir/news")
	if platformNews.Title != "اخبار و مطالب پزشکی | طب‌پرداز" || strings.Contains(platformNews.Title, "چمران") {
		t.Fatalf("platform news = %q", platformNews.Title)
	}

	weekly := WeeklyMeta(SiteClinic, "درمانگاه چمران مشهد", "https://chamranclinic.ir/برنامه-هفتگی-پزشکان", false)
	if weekly.Title != "برنامه هفتگی پزشکان درمانگاه چمران مشهد" || weekly.Robots != RobotsIndexFollow {
		t.Fatalf("weekly = %+v", weekly)
	}
	filtered := WeeklyMeta(SitePlatform, "", "https://tebpardaz.ir/weekly-schedule", WeeklyFiltered(WeeklyQuery{Q: "علی"}))
	if filtered.Robots != RobotsNoindexFollow || filtered.Canonical != "https://tebpardaz.ir/weekly-schedule" {
		t.Fatalf("filtered weekly = %+v", filtered)
	}
	if WeeklyFiltered(WeeklyQuery{Date: "1404/01/01"}) != true || WeeklyFiltered(WeeklyQuery{ClinicID: "0"}) != false {
		t.Fatal("weekly filter policy mismatch")
	}

	about := AboutMeta(SiteClinic, "درمانگاه چمران مشهد", "https://chamranclinic.ir/about", "")
	contact := ContactMeta(SiteClinic, "درمانگاه چمران مشهد", "https://chamranclinic.ir/contact", "0513000", "")
	terms := TermsMeta(SitePlatform, "", "https://tebpardaz.ir/terms")
	if about.Title != "درباره ما | درمانگاه چمران مشهد" || strings.Contains(about.Title, "طب‌پرداز") {
		t.Fatalf("about = %q", about.Title)
	}
	if !strings.Contains(contact.Description, "0513000") || contact.Title == contact.Description {
		t.Fatalf("contact = %+v", contact)
	}
	if terms.Title != "قوانین و مقررات | طب‌پرداز" {
		t.Fatalf("terms = %q", terms.Title)
	}
}

func TestSectionMetaOmitsEmptyClinic(t *testing.T) {
	meta := SectionDetailMeta(SectionOverview, "تصویربرداری", "درمانگاه چمران", "", "https://chamranclinic.ir/section/imaging")
	if meta.Title != "تصویربرداری درمانگاه چمران | خدمات و نوبت‌دهی" {
		t.Fatalf("title = %q", meta.Title)
	}
	hours := SectionDetailMeta(SectionHours, "تصویربرداری", "", "", "https://chamranclinic.ir/section/imaging/ساعات-کاری")
	if strings.Contains(hours.Title, "| |") || strings.Contains(hours.Title, " |  |") {
		t.Fatalf("hours title = %q", hours.Title)
	}
	if !strings.Contains(hours.Title, "ساعات کاری") {
		t.Fatalf("hours title = %q", hours.Title)
	}
}
