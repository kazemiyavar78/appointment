package pages

import (
	"context"
	"strings"
	"testing"

	"tebpardaz/server/views/components"
)

// TestSectionDetailRendersDoctorsBelowSchedule پزشکان بخش را زیر ساعات کاری رندر می‌کند.
func TestSectionDetailRendersDoctorsBelowSchedule(t *testing.T) {
	var buf strings.Builder
	view := SectionDetailView{
		Title: "آزمایشگاه",
		Slug:  "lab",
		Schedules: []SectionScheduleDisplay{
			{DayOfWeek: 0, DayName: "شنبه", IsOpen: true, HoursText: "08:00 – 14:00"},
		},
		Doctors: []components.DoctorCardView{
			{Name: "دکتر احمدی", SpecialtyName: "داخلی", BookingURL: "/booking/ahmadi"},
		},
	}
	if err := SectionDetail(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	scheduleAt := strings.Index(html, "ساعات کاری")
	doctorsAt := strings.Index(html, "پزشکان این بخش")
	if scheduleAt < 0 || doctorsAt < 0 {
		t.Fatalf("missing schedule or doctors heading")
	}
	if doctorsAt < scheduleAt {
		t.Fatalf("doctors section should appear after working hours")
	}
	if !strings.Contains(html, "دکتر احمدی") || !strings.Contains(html, "/booking/ahmadi") {
		t.Fatalf("doctor card missing from section page")
	}
}
