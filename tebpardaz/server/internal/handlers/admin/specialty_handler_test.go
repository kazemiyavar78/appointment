package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tebpardaz/server/internal/models"

	"github.com/gin-gonic/gin"
)

func TestNormalizeSpecialtyName(t *testing.T) {
	name, err := normalizeSpecialtyName("  داخلي  ")
	if err != nil || name != "داخلی" {
		t.Fatalf("normalize = %q %v", name, err)
	}
	name, err = normalizeSpecialtyName("رئیس")
	if err != nil || name != "رئیس" {
		t.Fatalf("hamza rewritten: %q %v", name, err)
	}
	name, err = normalizeSpecialtyName("فوق گواراش")
	if err != nil || name != "فوق گواراش" {
		t.Fatalf("content was corrected: %q %v", name, err)
	}
	if _, err = normalizeSpecialtyName("   "); err == nil {
		t.Fatal("blank name accepted")
	}
	if _, err = normalizeSpecialtyName(strings.Repeat("ا", specialtyNameMaxRunes+1)); err == nil {
		t.Fatal("long name accepted")
	}
	if _, err = normalizeSpecialtyName("قلب\x00"); err == nil {
		t.Fatal("control character accepted")
	}
}

func TestSpecialtyNameExists(t *testing.T) {
	rows := []models.Specialty{{Name: "داخلي"}}
	rows[0].ID = 7
	if !specialtyNameExists(rows, "  داخلی  ", 0) {
		t.Fatal("duplicate normalized name allowed")
	}
	if specialtyNameExists(rows, "داخلی", 7) {
		t.Fatal("update treated itself as duplicate")
	}
	if specialtyNameExists(rows, "قلب", 0) {
		t.Fatal("different name marked duplicate")
	}
}

func TestApplySpecialtyFormLeavesNameEN(t *testing.T) {
	gin.SetMode(gin.TestMode)
	form := url.Values{}
	form.Set("name_en", "  Cardiology ي  ")
	req := httptest.NewRequest(http.MethodPost, "/admin/specialties", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	if err := c.Request.ParseForm(); err != nil {
		t.Fatal(err)
	}
	row := &models.Specialty{}
	applySpecialtyForm(row, c)
	if row.NameEN != "Cardiology ي" {
		t.Fatalf("NameEN normalized: %q", row.NameEN)
	}
}

// TestParseSpecialtySortOrderClampsInvalidAndOutOfRange مقادیر نامعتبر و خارج از محدوده را محدود می‌کند.
func TestParseSpecialtySortOrderClampsInvalidAndOutOfRange(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"abc", 0},
		{"-3", 0},
		{"12", 12},
		{"999", 999},
		{"1000", 999},
		{" 7 ", 7},
	}
	for _, tc := range cases {
		if got := parseSpecialtySortOrder(tc.in); got != tc.want {
			t.Fatalf("parseSpecialtySortOrder(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestApplySpecialtyFormReadsOrderAndBookingVisibility فیلدهای ترتیب و نمایش نوبت‌دهی را از فرم می‌خواند.
func TestApplySpecialtyFormReadsOrderAndBookingVisibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	form := url.Values{}
	form.Set("name_en", "Cardiology")
	form.Set("sort_order", "4")
	form.Set("is_approved", "1")
	form.Set("show_in_booking", "1")

	req := httptest.NewRequest(http.MethodPost, "/admin/specialties", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	if err := c.Request.ParseForm(); err != nil {
		t.Fatalf("parse form: %v", err)
	}

	row := &models.Specialty{}
	applySpecialtyForm(row, c)
	if row.SortOrder != 4 {
		t.Fatalf("SortOrder = %d, want 4", row.SortOrder)
	}
	if !row.ShowInBooking {
		t.Fatal("ShowInBooking should be true when checkbox is posted")
	}
	if !row.IsApproved {
		t.Fatal("IsApproved should be true when checkbox is posted")
	}
}

// TestApplySpecialtyFormUncheckedBookingHidesSpecialty وقتی چک‌باکس نوبت‌دهی نباشد تخصص مخفی می‌شود.
func TestApplySpecialtyFormUncheckedBookingHidesSpecialty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodPost, "/admin/specialties", strings.NewReader("sort_order=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	if err := c.Request.ParseForm(); err != nil {
		t.Fatalf("parse form: %v", err)
	}

	row := &models.Specialty{ShowInBooking: true}
	applySpecialtyForm(row, c)
	if row.ShowInBooking {
		t.Fatal("ShowInBooking should be false when checkbox is omitted")
	}
}
