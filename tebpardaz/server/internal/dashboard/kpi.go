package dashboard

import (
	"math"
	"net/url"
	"strconv"

	"tebpardaz/server/internal/text"
)

// MetricDef describes one KPI card. Append to DefaultMetrics to add a new card.
type MetricDef struct {
	ID     MetricID
	Label  string
	Icon   string
	Format string
	Invert bool
	Hint   string
}

// DefaultMetrics is the ordered set of clinic-home KPI cards.
var DefaultMetrics = []MetricDef{
	{ID: MetricAppointmentsTotal, Label: "نوبت‌های گرفته‌شده", Icon: "fa-solid fa-calendar-plus", Format: FormatCount, Hint: "همه نوبت‌های ثبت‌شده در بازه"},
	{ID: MetricAppointmentsConfirmed, Label: "نوبت‌های موفق", Icon: "fa-solid fa-circle-check", Format: FormatCount, Hint: "نوبت‌های تأییدشده"},
	{ID: MetricAppointmentsFailed, Label: "نوبت‌های ناموفق", Icon: "fa-solid fa-circle-xmark", Format: FormatCount, Invert: true, Hint: "نوبت‌هایی که ثبت آن‌ها شکست خورده"},
	{ID: MetricAppointmentsCancelled, Label: "نوبت‌های لغوشده", Icon: "fa-solid fa-ban", Format: FormatCount, Invert: true, Hint: "نوبت‌های لغوشده در بازه"},
	{ID: MetricVisits, Label: "بازدید سایت", Icon: "fa-solid fa-eye", Format: FormatCount, Hint: "تعداد بازدید صفحات"},
	{ID: MetricSuccessRate, Label: "نرخ موفقیت نوبت‌دهی", Icon: "fa-solid fa-percent", Format: FormatPercent, Hint: "نوبت موفق نسبت به کل نوبت‌ها"},
	{ID: MetricConversionRate, Label: "نرخ تبدیل بازدید به نوبت", Icon: "fa-solid fa-arrow-trend-up", Format: FormatPercent, Hint: "نوبت ثبت‌شده نسبت به بازدید"},
	{ID: MetricActiveDoctors, Label: "پزشکان فعال", Icon: "fa-solid fa-user-doctor", Format: FormatCount, Hint: "پزشکانی که در این بازه نوبت گرفته‌اند"},
}

// Values holds computed metric values for current and previous windows.
type Values map[MetricID]float64

// ChangePercent returns the percent change from previous to current.
// Inputs: current and previous numeric values.
// Output: percent and ok=false when previous is 0 (undefined ratio).
func ChangePercent(current, previous float64) (float64, bool) {
	if previous == 0 {
		return 0, false
	}
	return ((current - previous) / math.Abs(previous)) * 100, true
}

// ClassifyChange maps a delta onto direction and traffic-light sentiment.
// Inputs: current, previous, invert (true when an increase is bad).
// Output: direction, sentiment, optional change percent.
func ClassifyChange(current, previous float64, invert bool) (direction, sentiment string, pct *float64) {
	diff := current - previous
	switch {
	case diff > 0:
		direction = DirectionUp
	case diff < 0:
		direction = DirectionDown
	default:
		direction = DirectionFlat
	}
	if p, ok := ChangePercent(current, previous); ok {
		pct = &p
	}
	switch direction {
	case DirectionFlat:
		sentiment = SentimentNeutral
	case DirectionUp:
		if invert {
			sentiment = SentimentNegative
		} else {
			sentiment = SentimentPositive
		}
	default:
		if invert {
			sentiment = SentimentPositive
		} else {
			sentiment = SentimentNegative
		}
	}
	return direction, sentiment, pct
}

// FormatKPIValue renders a metric for display in Persian digits.
// Inputs: value and format (count or percent).
// Output: formatted Persian string.
func FormatKPIValue(value float64, format string) string {
	if format == FormatPercent {
		return text.FormatPersianPercent(value)
	}
	return text.FormatPersianInt(int64(math.Round(value)))
}

// BuildKPIs turns metric definitions and two value maps into dashboard cards.
// Inputs: defs (usually DefaultMetrics), current/previous values, period, clinic filter.
// Output: KPI slice in definition order.
func BuildKPIs(defs []MetricDef, current, previous Values, period Period, clinicID uint) []KPI {
	out := make([]KPI, 0, len(defs))
	for _, def := range defs {
		cur := current[def.ID]
		prev := previous[def.ID]
		dir, sent, pct := ClassifyChange(cur, prev, def.Invert)
		card := KPI{
			ID:           def.ID,
			Label:        def.Label,
			Icon:         def.Icon,
			Format:       def.Format,
			Value:        cur,
			Previous:     prev,
			ValueText:    FormatKPIValue(cur, def.Format),
			PreviousText: FormatKPIValue(prev, def.Format),
			ChangePct:    pct,
			Direction:    dir,
			Sentiment:    sent,
			Invert:       def.Invert,
			DetailURL:    metricDetailURL(def.ID, period, clinicID),
			Hint:         def.Hint,
		}
		card.ChangeText = changeCaption(pct, dir, period.CompareLabel)
		out = append(out, card)
	}
	return out
}

// changeCaption builds the small comparison line under a KPI value.
func changeCaption(pct *float64, direction, compareLabel string) string {
	if pct == nil {
		switch direction {
		case DirectionUp:
			return "دادهٔ دوره قبل صفر بوده است"
		case DirectionDown:
			return "در این بازه مقداری ثبت نشده"
		default:
			return "بدون تغییر نسبت به " + compareLabel
		}
	}
	abs := math.Abs(*pct)
	textPct := text.FormatPersianPercent(abs)
	switch direction {
	case DirectionUp:
		return textPct + " افزایش نسبت به " + compareLabel
	case DirectionDown:
		return textPct + " کاهش نسبت به " + compareLabel
	default:
		return "بدون تغییر نسبت به " + compareLabel
	}
}

// metricDetailURL links a KPI card onto the existing filtered admin list.
func metricDetailURL(id MetricID, period Period, clinicID uint) string {
	from := DateKey(period.From)
	to := DateKey(InclusiveEndDate(period))
	v := url.Values{}
	v.Set("from", from)
	v.Set("to", to)
	if clinicID > 0 {
		v.Set("clinic_id", strconv.FormatUint(uint64(clinicID), 10))
	}
	switch id {
	case MetricAppointmentsConfirmed:
		v.Set("status", "confirmed")
		return "/admin/bookings?" + v.Encode()
	case MetricAppointmentsFailed:
		v.Set("status", "failed")
		return "/admin/bookings?" + v.Encode()
	case MetricAppointmentsCancelled:
		v.Set("status", "cancelled")
		return "/admin/bookings?" + v.Encode()
	case MetricAppointmentsTotal, MetricSuccessRate, MetricConversionRate, MetricActiveDoctors:
		return "/admin/bookings?" + v.Encode()
	case MetricVisits:
		return "/admin/visitors/visits?" + v.Encode()
	default:
		return "/admin/bookings?" + v.Encode()
	}
}

// RatioPercent returns (num/den)*100, or 0 when den is 0.
func RatioPercent(num, den float64) float64 {
	if den <= 0 {
		return 0
	}
	return (num / den) * 100
}
