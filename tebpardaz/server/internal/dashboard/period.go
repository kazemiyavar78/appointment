package dashboard

import "time"

// RangeKey is a named dashboard time window selected by the clinic admin.
type RangeKey string

const (
	RangeToday     RangeKey = "today"
	RangeYesterday RangeKey = "yesterday"
	RangeLast7     RangeKey = "last7"
	RangeLast30    RangeKey = "last30"
)

// Period is an inclusive-start / exclusive-end window plus the matching previous window.
type Period struct {
	Key          RangeKey
	Label        string
	CompareLabel string
	From         time.Time
	To           time.Time
	PrevFrom     time.Time
	PrevTo       time.Time
	ChartFrom    time.Time
	ChartDays    int
}

// ParseRangeKey maps a query value onto a known range; unknown values become today.
// Inputs: raw query string.
// Output: RangeKey used by ResolvePeriod.
func ParseRangeKey(raw string) RangeKey {
	switch RangeKey(raw) {
	case RangeYesterday, RangeLast7, RangeLast30:
		return RangeKey(raw)
	default:
		return RangeToday
	}
}

// ResolvePeriod builds the selected window, previous comparable window, and chart span.
// Inputs: key (today/yesterday/last7/last30), now (usually time.Now in local TZ).
// Output: Period with exclusive To timestamps.
func ResolvePeriod(key RangeKey, now time.Time) Period {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(time.Local)
	today := startOfDay(now)
	tomorrow := today.AddDate(0, 0, 1)

	p := Period{Key: ParseRangeKey(string(key))}
	switch p.Key {
	case RangeYesterday:
		p.Label = "دیروز"
		p.CompareLabel = "روز قبل"
		p.From = today.AddDate(0, 0, -1)
		p.To = today
		p.PrevFrom = today.AddDate(0, 0, -2)
		p.PrevTo = today.AddDate(0, 0, -1)
		p.ChartFrom = today.AddDate(0, 0, -6)
		p.ChartDays = 7
	case RangeLast7:
		p.Label = "۷ روز اخیر"
		p.CompareLabel = "۷ روز قبل"
		p.From = today.AddDate(0, 0, -6)
		p.To = tomorrow
		p.PrevFrom = today.AddDate(0, 0, -13)
		p.PrevTo = today.AddDate(0, 0, -6)
		p.ChartFrom = p.From
		p.ChartDays = 7
	case RangeLast30:
		p.Label = "۳۰ روز اخیر"
		p.CompareLabel = "۳۰ روز قبل"
		p.From = today.AddDate(0, 0, -29)
		p.To = tomorrow
		p.PrevFrom = today.AddDate(0, 0, -59)
		p.PrevTo = today.AddDate(0, 0, -29)
		p.ChartFrom = p.From
		p.ChartDays = 30
	default:
		p.Key = RangeToday
		p.Label = "امروز"
		p.CompareLabel = "دیروز"
		p.From = today
		p.To = tomorrow
		p.PrevFrom = today.AddDate(0, 0, -1)
		p.PrevTo = today
		p.ChartFrom = today.AddDate(0, 0, -6)
		p.ChartDays = 7
	}
	return p
}

// RangeOptions returns the four range pills shown in the dashboard toolbar.
// Inputs: none.
// Output: ordered range keys and Persian labels.
func RangeOptions() []struct {
	Key   RangeKey
	Label string
} {
	return []struct {
		Key   RangeKey
		Label string
	}{
		{RangeToday, "امروز"},
		{RangeYesterday, "دیروز"},
		{RangeLast7, "۷ روز"},
		{RangeLast30, "۳۰ روز"},
	}
}

// startOfDay returns local midnight for t.
func startOfDay(t time.Time) time.Time {
	t = t.In(time.Local)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// DateKey formats a day as YYYY-MM-DD in the local zone.
func DateKey(t time.Time) string {
	return startOfDay(t).Format("2006-01-02")
}

// InclusiveEndDate is the last civil day contained in [From, To).
func InclusiveEndDate(p Period) time.Time {
	if !p.To.After(p.From) {
		return startOfDay(p.From)
	}
	return startOfDay(p.To.Add(-time.Second))
}
