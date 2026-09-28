package admin

import (
	"context"
	"strings"
	"testing"
	"time"

	"tebpardaz/shared/constants"
)

// TestVisitorIPsRendersFiltersAndRows checks the IP list filter form and table.
func TestVisitorIPsRendersFiltersAndRows(t *testing.T) {
	var buf strings.Builder
	view := VisitorIPsView{
		Nav:         BuildAdminNav(NavVisitorIPs, string(constants.UserRoleSuperAdmin)),
		SearchQuery: "203.0.113.10",
		Rows: []VisitorIPRow{
			{ID: 4, IPAddress: "203.0.113.10", VisitCount: 9, LastVisitAt: time.Date(2026, 9, 15, 10, 0, 0, 0, time.Local)},
		},
		Total:          1,
		Page:           1,
		TotalPages:     1,
		CanSeePlatform: true,
	}
	if err := VisitorIPs(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		"لیست آی‌پی بازدیدکنندگان",
		`name="q"`,
		`name="clinic_id"`,
		"بدون مرکز (پلتفرم/ارگان)",
		"203.0.113.10",
		"/admin/visitors/visits?visitor_ip_id=4",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered IP page missing %q", want)
		}
	}
}

// TestVisitorVisitsRendersFiltersAndRows checks the page-visit filter form and table.
func TestVisitorVisitsRendersFiltersAndRows(t *testing.T) {
	var buf strings.Builder
	view := VisitorVisitsView{
		Nav:              BuildAdminNav(NavVisitorVisits, string(constants.UserRoleSuperAdmin)),
		Browsers:         []string{"Chrome", "Firefox"},
		OperatingSystems: []string{"Windows", "Android"},
		Browser:          "Chrome",
		OS:               "Windows",
		GoogleFilter:     "1",
		Rows: []VisitorVisitRow{
			{
				ID: 11, VisitorIPID: 4, IPAddress: "203.0.113.10",
				Host: "tebpardaz.ir", Path: "/doctors", FullURL: "/doctors?q=1",
				Browser: "Chrome", OS: "Windows", IsFromGoogle: true,
			},
		},
		Total:      1,
		Page:       1,
		TotalPages: 1,
	}
	if err := VisitorVisits(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		"بازدید صفحات",
		`name="browser"`,
		`name="os"`,
		`name="from_google"`,
		"/doctors",
		"Chrome",
		"Windows",
		"بله",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered visits page missing %q", want)
		}
	}
}
