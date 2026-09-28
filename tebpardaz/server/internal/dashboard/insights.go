package dashboard

import (
	"fmt"
	"math"

	"tebpardaz/server/internal/text"
)

// BuildInsights writes a short Persian briefing from real snapshot values.
// Inputs: assembled dashboard snapshot (KPIs, tops, hours).
// Output: up to a handful of sentences; empty data yields a single empty-state line.
func BuildInsights(snap Snapshot) []string {
	out := make([]string, 0, 8)
	hasActivity := false
	if k := snap.KPIByID(MetricVisits); k != nil && k.Value > 0 {
		hasActivity = true
	}
	if k := snap.KPIByID(MetricAppointmentsTotal); k != nil && k.Value > 0 {
		hasActivity = true
	}
	if !hasActivity {
		return []string{"در این بازه هنوز بازدید یا نوبتی ثبت نشده است."}
	}

	if line := visitChangeInsight(snap); line != "" {
		out = append(out, line)
	}
	if line := bookingChangeInsight(snap); line != "" {
		out = append(out, line)
	}
	if len(snap.TopPages) > 0 && snap.TopPages[0].Count > 0 {
		out = append(out, "بیشترین بازدید مربوط به "+snap.TopPages[0].Label+" بوده است.")
	}
	if len(snap.TopDoctors) > 0 && snap.TopDoctors[0].Count > 0 {
		out = append(out, "بیشترین پزشک مشاهده‌شده "+snap.TopDoctors[0].Label+" بوده است.")
	}
	if len(snap.TopSpecialties) > 0 && snap.TopSpecialties[0].Count > 0 {
		out = append(out, "بیشترین تقاضا مربوط به تخصص "+snap.TopSpecialties[0].Label+" بوده است.")
	}
	if hour, ok := peakHour(snap.BookingHours); ok {
		next := hour.Hour + 1
		if next > 23 {
			next = 0
		}
		out = append(out, "بیشترین نوبت‌ها در بازه زمانی "+
			text.ToPersianDigits(fmt.Sprintf("%02d", hour.Hour))+":۰۰ تا "+
			text.ToPersianDigits(fmt.Sprintf("%02d", next))+":۰۰ ثبت شده‌اند.")
	}
	if k := snap.KPIByID(MetricAppointmentsFailed); k != nil && k.Value > 0 {
		out = append(out, "در این بازه "+k.ValueText+" نوبت ناموفق ثبت شده است.")
	}
	if k := snap.KPIByID(MetricSuccessRate); k != nil && snap.KPIByID(MetricAppointmentsTotal) != nil && snap.KPIByID(MetricAppointmentsTotal).Value > 0 {
		out = append(out, "نرخ موفقیت نوبت‌دهی "+k.ValueText+" بوده است.")
	}
	return out
}

func visitChangeInsight(snap Snapshot) string {
	k := snap.KPIByID(MetricVisits)
	if k == nil {
		return ""
	}
	return changeInsight("بازدید سایت نسبت به "+snap.CompareLabel, k)
}

func bookingChangeInsight(snap Snapshot) string {
	k := snap.KPIByID(MetricAppointmentsTotal)
	if k == nil {
		return ""
	}
	return changeInsight("نوبت‌های ثبت‌شده نسبت به "+snap.CompareLabel, k)
}

func changeInsight(prefix string, k *KPI) string {
	if k.ChangePct == nil {
		if k.Direction == DirectionFlat {
			return prefix + " تغییری نداشته است."
		}
		return ""
	}
	abs := math.Abs(*k.ChangePct)
	if abs < 0.05 {
		return prefix + " تغییری نداشته است."
	}
	verb := "افزایش"
	if *k.ChangePct < 0 {
		verb = "کاهش"
	}
	return prefix + " " + text.FormatPersianPercent(abs) + " " + verb + " داشته است."
}

func peakHour(hours []HourCount) (HourCount, bool) {
	var best HourCount
	found := false
	for _, h := range hours {
		if !found || h.Count > best.Count {
			best = h
			found = h.Count > 0
		}
	}
	return best, found
}
