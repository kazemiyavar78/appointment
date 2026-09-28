package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TestParsePostedUintIDsSkipsInvalidValues شناسه‌های نامعتبر و صفر را حذف می‌کند.
func TestParsePostedUintIDsSkipsInvalidValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	form := url.Values{}
	form.Add("service_ids", "3")
	form.Add("service_ids", "0")
	form.Add("service_ids", "abc")
	form.Add("service_ids", "12")

	req := httptest.NewRequest(http.MethodPost, "/admin/services/packages", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	if err := c.Request.ParseForm(); err != nil {
		t.Fatalf("parse form: %v", err)
	}

	got := parsePostedUintIDs(c, "service_ids")
	if len(got) != 2 || got[0] != 3 || got[1] != 12 {
		t.Fatalf("got %#v, want [3 12]", got)
	}
}

// TestToServicePackageRowsCopiesCount تعداد خدمات هر بسته را به ردیف جدول منتقل می‌کند.
func TestToServicePackageRowsCopiesCount(t *testing.T) {
	rows := toServicePackageRows(
		[]models.ServicePackage{
			{Model: gorm.Model{ID: 1}, Name: "قلب", Description: "توضیح"},
			{Model: gorm.Model{ID: 2}, Name: "جراحی"},
		},
		map[uint]int{1: 4},
	)
	if len(rows) != 2 {
		t.Fatalf("len = %d, want 2", len(rows))
	}
	if rows[0].ServiceCount != 4 || rows[1].ServiceCount != 0 {
		t.Fatalf("unexpected counts: %+v", rows)
	}
	if rows[0].Name != "قلب" || rows[0].Description != "توضیح" {
		t.Fatalf("unexpected first row: %+v", rows[0])
	}
}

// TestToServicePackageOptionsUsesMemberIDs شناسه و تعداد خدمات بسته را برای دکمه انتصاب می‌سازد.
func TestToServicePackageOptionsUsesMemberIDs(t *testing.T) {
	got := toServicePackageOptions([]repository.ServicePackageWithIDs{
		{Package: models.ServicePackage{Model: gorm.Model{ID: 8}, Name: "چکاپ"}, ServiceIDs: []uint{1, 2, 3}},
		{Package: models.ServicePackage{Model: gorm.Model{ID: 9}, Name: "خالی"}, ServiceIDs: nil},
	})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != 8 || got[0].Count != 3 || got[0].ServiceIDs[1] != 2 {
		t.Fatalf("unexpected first option: %+v", got[0])
	}
	if got[1].Count != 0 || got[1].ServiceIDs == nil {
		t.Fatalf("empty package should have non-nil empty IDs: %+v", got[1])
	}
}

// TestServiceFlashMessagePackageCodes پیام‌های بسته خدمات را به فارسی برمی‌گرداند.
func TestServiceFlashMessagePackageCodes(t *testing.T) {
	if got := serviceFlashMessage("pkg_created"); got != "بسته خدمات با موفقیت ایجاد شد." {
		t.Fatalf("got %q", got)
	}
	if got := serviceFlashMessage("pkg_name_required"); got != "نام بسته الزامی است." {
		t.Fatalf("got %q", got)
	}
	if got := serviceFlashMessage("section_assigned"); got == "" {
		t.Fatal("section assignment flash should be set")
	}
	if got := serviceFlashMessage("packages_failed"); got == "" {
		t.Fatal("package assignment flash should be set")
	}
}

// TestCatalogQueryFromRequestReadsSearchAndPage عبارت جستجو و شماره صفحه را از کوئری می‌خواند.
func TestCatalogQueryFromRequestReadsSearchAndPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "/admin/services?q=نوار&page=3", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	q, page := catalogQueryFromRequest(c)
	if q != "نوار" || page != 3 {
		t.Fatalf("got q=%q page=%d", q, page)
	}
}

// TestSectionPackageAssignPathKeepsClinicAndSection آدرس انتصاب بسته، مرکز و بخش را نگه می‌دارد.
func TestSectionPackageAssignPathKeepsClinicAndSection(t *testing.T) {
	got := sectionPackageAssignPath(4, 9, "section_assigned")
	for _, want := range []string{"/admin/services/section-packages?", "clinic_id=4", "section_id=9", "msg=section_assigned"} {
		if !strings.Contains(got, want) {
			t.Fatalf("path %q missing %q", got, want)
		}
	}
}
