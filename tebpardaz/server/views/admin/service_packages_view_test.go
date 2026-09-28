package admin

import (
	"context"
	"strings"
	"testing"

	"tebpardaz/shared/constants"
)

// TestJoinUintIDsFormatsCommaSeparatedList شناسه‌ها را با ویرگول به رشته تبدیل می‌کند.
func TestJoinUintIDsFormatsCommaSeparatedList(t *testing.T) {
	if got := joinUintIDs(nil); got != "" {
		t.Fatalf("nil = %q, want empty", got)
	}
	if got := joinUintIDs([]uint{}); got != "" {
		t.Fatalf("empty = %q, want empty", got)
	}
	if got := joinUintIDs([]uint{4, 8, 15}); got != "4,8,15" {
		t.Fatalf("got %q, want 4,8,15", got)
	}
}

// TestServicePackagesPageRendersFormAndTable فرم ایجاد بسته و جدول را رندر می‌کند.
func TestServicePackagesPageRendersFormAndTable(t *testing.T) {
	var buf strings.Builder
	view := ServicePackagePageView{
		Nav: BuildAdminNav(NavServicePackages, string(constants.UserRoleSuperAdmin)),
		Items: []ServicePackageRow{
			{ID: 7, Name: "چکاپ قلب", Description: "نوار و اکو", ServiceCount: 2},
		},
		Services: []ServiceAssignOption{
			{ID: 3, Name: "نوار قلب", Description: "ECG", Packages: []string{"بسته تشخیص"}, Selected: true},
			{ID: 4, Name: "اکو", Selected: false},
		},
	}
	if err := ServicePackages(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		"بسته‌های خدمات",
		`action="/admin/services/packages"`,
		`name="service_ids"`,
		"نوار قلب",
		"بسته تشخیص",
		"چکاپ قلب",
		"/admin/services/packages?edit=7",
		"/admin/services/packages/7/delete",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered page missing %q", want)
		}
	}
}

// TestServicePackagesEditFormPostsToUpdate مسیر ویرایش بسته را به فرم بروزرسانی می‌برد.
func TestServicePackagesEditFormPostsToUpdate(t *testing.T) {
	var buf strings.Builder
	view := ServicePackagePageView{
		Nav:     BuildAdminNav(NavServicePackages, string(constants.UserRoleSuperAdmin)),
		EditID:  9,
		EditName: "بسته جراحی",
	}
	if err := ServicePackages(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `/admin/services/packages/9/update`) {
		t.Fatalf("edit form missing update action: %s", html)
	}
	if !strings.Contains(html, "بروزرسانی بسته") {
		t.Fatal("edit heading missing")
	}
}
