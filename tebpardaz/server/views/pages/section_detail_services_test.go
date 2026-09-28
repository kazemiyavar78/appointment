package pages

import (
	"context"
	"strings"
	"testing"
)

// TestSectionDetailShowsCatalogServices خدمات بسته‌های بخش را بعد از پزشکان نشان می‌دهد.
func TestSectionDetailShowsCatalogServices(t *testing.T) {
	var buf strings.Builder
	view := SectionDetailView{
		Title:   "قلب",
		Doctors: nil,
		CatalogServices: []SectionServiceDisplay{
			{Name: "نوار قلب", Description: "ثبت فعالیت الکتریکی", Packages: []string{"خدمات قلب"}},
			{Name: "اکوکاردیوگرافی", Packages: []string{"خدمات قلب"}},
		},
	}
	if err := SectionDetail(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{"خدمات این بخش", "نوار قلب", "ثبت فعالیت الکتریکی", "خدمات قلب", "اکوکاردیوگرافی"} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered section missing %q", want)
		}
	}
}

// TestSectionDetailHidesEmptyCatalog بخش خدمات را وقتی بسته‌ای منتسب نیست رندر نمی‌کند.
func TestSectionDetailHidesEmptyCatalog(t *testing.T) {
	var buf strings.Builder
	view := SectionDetailView{Title: "آزمایشگاه"}
	if err := SectionDetail(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(buf.String(), "خدمات این بخش") {
		t.Fatal("empty catalog should stay hidden")
	}
}
