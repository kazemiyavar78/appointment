package dashboard

import (
	"testing"
	"time"
)

func TestClassifyChangeInvert(t *testing.T) {
	dir, sent, pct := ClassifyChange(12, 10, true)
	if dir != DirectionUp || sent != SentimentNegative || pct == nil {
		t.Fatalf("invert up: dir=%s sent=%s pct=%v", dir, sent, pct)
	}
	dir, sent, _ = ClassifyChange(8, 10, true)
	if dir != DirectionDown || sent != SentimentPositive {
		t.Fatalf("invert down: dir=%s sent=%s", dir, sent)
	}
}

func TestChangePercentUndefined(t *testing.T) {
	if _, ok := ChangePercent(5, 0); ok {
		t.Fatal("previous zero should be undefined")
	}
	got, ok := ChangePercent(12, 10)
	if !ok || got != 20 {
		t.Fatalf("20%% expected, got %v ok=%v", got, ok)
	}
}

func TestBuildKPIsOrder(t *testing.T) {
	cur := Values{MetricVisits: 118, MetricAppointmentsTotal: 10}
	prev := Values{MetricVisits: 100, MetricAppointmentsTotal: 10}
	period := ResolvePeriod(RangeToday, parseDashTime(t, "2026-09-20"))
	kpis := BuildKPIs(DefaultMetrics, cur, prev, period, 3)
	if len(kpis) != len(DefaultMetrics) {
		t.Fatalf("kpi count=%d", len(kpis))
	}
	if kpis[4].ID != MetricVisits || kpis[4].Direction != DirectionUp || kpis[4].Sentiment != SentimentPositive {
		t.Fatalf("visits kpi %+v", kpis[4])
	}
	if kpis[0].DetailURL == "" {
		t.Fatal("expected detail url")
	}
}

func parseDashTime(t *testing.T, day string) time.Time {
	t.Helper()
	ts, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	return ts.Add(10 * time.Hour)
}
