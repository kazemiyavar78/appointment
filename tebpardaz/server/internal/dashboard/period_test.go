package dashboard

import (
	"testing"
	"time"
)

func TestResolvePeriodToday(t *testing.T) {
	now := time.Date(2026, 9, 20, 14, 30, 0, 0, time.Local)
	p := ResolvePeriod(RangeToday, now)
	if p.Key != RangeToday || p.CompareLabel != "دیروز" {
		t.Fatalf("today labels: %+v", p)
	}
	if p.From.Hour() != 0 || p.To.Sub(p.From) != 24*time.Hour {
		t.Fatalf("today window from=%v to=%v", p.From, p.To)
	}
	if p.ChartDays != 7 {
		t.Fatalf("chart days=%d", p.ChartDays)
	}
}

func TestResolvePeriodLast30(t *testing.T) {
	now := time.Date(2026, 9, 20, 8, 0, 0, 0, time.Local)
	p := ResolvePeriod(RangeLast30, now)
	if p.ChartDays != 30 {
		t.Fatalf("chart days=%d", p.ChartDays)
	}
	if int(p.To.Sub(p.From).Hours()/24) != 30 {
		t.Fatalf("span hours=%v", p.To.Sub(p.From))
	}
}

func TestParseRangeKeyUnknown(t *testing.T) {
	if ParseRangeKey("nope") != RangeToday {
		t.Fatal("unknown range should default to today")
	}
}
