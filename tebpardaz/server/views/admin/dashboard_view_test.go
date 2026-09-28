package admin

import (
	"context"
	"strings"
	"testing"

	"tebpardaz/server/internal/dashboard"
	"tebpardaz/shared/constants"
)

func TestDashboardRendersKPIsAndRanges(t *testing.T) {
	var buf strings.Builder
	view := DashboardView{
		Nav:   BuildAdminNav(NavDashboard, string(constants.UserRoleClinicAdmin)),
		Range: string(dashboard.RangeToday),
		Data: dashboard.Snapshot{
			RangeLabel:   "امروز",
			CompareLabel: "دیروز",
			UpdatedText:  "۱۴۰۵/۰۶/۲۹ ۱۲:۰۰:۰۰",
			KPIs: []dashboard.KPI{
				{
					ID:         dashboard.MetricVisits,
					Label:      "بازدید سایت",
					Icon:       "fa-solid fa-eye",
					ValueText:  "۱۱۸",
					ChangeText: "۱۸٫۰٪ افزایش نسبت به دیروز",
					Sentiment:  dashboard.SentimentPositive,
					Direction:  dashboard.DirectionUp,
					DetailURL:  "/admin/visitors/visits",
				},
			},
			Insights: []string{"بازدید سایت نسبت به دیروز ۱۸٫۰٪ افزایش داشته است."},
			Daily: []dashboard.DayPoint{
				{Date: "2026-09-20", Label: "۰۶/۲۹", Visits: 10, Appointments: 2, VisitsText: "۱۰", ApptText: "۲"},
			},
		},
		DataJSON: `{"range":"today"}`,
	}
	if err := Dashboard(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		"داشبورد مدیریت مرکز",
		"بازدید سایت",
		"۱۱۸",
		"خلاصه امروز",
		"/admin/dashboard/data",
		"admin_dashboard.js",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q", want)
		}
	}
}
