package admin

import (
	"context"
	"strings"
	"testing"

	"tebpardaz/shared/constants"
)

// TestServiceCatalogHrefKeepsSearchAndPage پارامترهای جستجو، صفحه و ویرایش را در آدرس نگه می‌دارد.
func TestServiceCatalogHrefKeepsSearchAndPage(t *testing.T) {
	got := ServiceCatalogHref("نوار", 2, 9, "updated")
	for _, want := range []string{"/admin/services?", "q=", "page=2", "edit=9", "msg=updated"} {
		if !strings.Contains(got, want) {
			t.Fatalf("href %q missing %q", got, want)
		}
	}
	if ServiceCatalogHref("", 1, 0, "") != "/admin/services" {
		t.Fatalf("empty href = %q", ServiceCatalogHref("", 1, 0, ""))
	}
}

// TestServicesPageRendersPackagesSearchAndPager ردیف بسته، جستجو و صفحه‌بندی را نشان می‌دهد.
func TestServicesPageRendersPackagesSearchAndPager(t *testing.T) {
	var buf strings.Builder
	view := ServicePageView{
		Nav:         BuildAdminNav(NavServices, string(constants.UserRoleSuperAdmin)),
		SearchQuery: "قلب",
		Page:        1,
		PageSize:    15,
		Total:       20,
		TotalPages:  2,
		Items: []ServiceRow{
			{ID: 4, Name: "نوار قلب", Description: "ثبت نوار", Packages: []string{"چکاپ قلب"}},
		},
		Packages: []ServiceAssignOption{
			{ID: 3, Name: "چکاپ قلب", Selected: true},
		},
	}
	if err := Services(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		"چکاپ قلب",
		`name="q"`,
		`name="package_ids"`,
		`value="3"`,
		"صفحه 1 از 2",
		"/admin/services/section-packages",
		"checked",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered services page missing %q", want)
		}
	}
}

// TestSectionPackageAssignPageRendersSelectedPackage بسته انتخاب‌شده بخش را در فرم نشان می‌دهد.
func TestSectionPackageAssignPageRendersSelectedPackage(t *testing.T) {
	var buf strings.Builder
	view := SectionPackageAssignView{
		Nav:               BuildAdminNav(NavSectionServicePackages, string(constants.UserRoleSuperAdmin)),
		SelectedClinicID:  2,
		SelectedSectionID: 8,
		Clinics:           []ClinicOption{{ID: 2, Name: "مرکز نمونه", Selected: true}},
		Sections:          []ClinicSectionOption{{ID: 8, Title: "قلب", Selected: true}},
		Packages: []ServicePackageOption{
			{ID: 3, Name: "چکاپ قلب", Count: 2, Selected: true},
			{ID: 4, Name: "بسته خالی", Count: 0},
		},
	}
	if err := SectionServicePackages(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		"انتصاب بسته خدمات به بخش‌ها",
		"چکاپ قلب",
		"بسته خالی",
		"بدون خدمت",
		`name="package_ids"`,
		`name="section_id"`,
		`value="8"`,
		"checked",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered assign page missing %q", want)
		}
	}
}
