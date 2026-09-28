package dashboard

import (
	"strings"
	"testing"
)

func TestBuildInsightsEmpty(t *testing.T) {
	got := BuildInsights(Snapshot{KPIs: []KPI{{ID: MetricVisits, Value: 0}}})
	if len(got) != 1 {
		t.Fatalf("empty insights=%v", got)
	}
}

func TestBuildInsightsTopPageAndDoctor(t *testing.T) {
	pct := 18.0
	snap := Snapshot{
		CompareLabel: "دیروز",
		KPIs: []KPI{
			{ID: MetricVisits, Value: 118, ChangePct: &pct},
			{ID: MetricAppointmentsTotal, Value: 10, ValueText: "۱۰"},
			{ID: MetricSuccessRate, Value: 80, ValueText: "۸۰٫۰٪"},
		},
		TopPages:       []NamedCount{{Label: "صفحه پزشکان", Count: 40}},
		TopDoctors:     []NamedCount{{Label: "دکتر احمدی", Count: 12}},
		TopSpecialties: []NamedCount{{Label: "داخلی", Count: 8}},
		BookingHours:   []HourCount{{Hour: 10, Count: 1}, {Hour: 14, Count: 6}},
	}
	got := BuildInsights(snap)
	joined := strings.Join(got, " ")
	for _, want := range []string{"۱۸٫۰٪", "صفحه پزشکان", "دکتر احمدی", "داخلی", "۱۴:۰۰"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %v", want, got)
		}
	}
}
