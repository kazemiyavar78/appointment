package dashboard

import "testing"

func TestPageLabelHomeAndDoctors(t *testing.T) {
	if PageLabel("/") != "صفحه اصلی" {
		t.Fatalf("home: %q", PageLabel("/"))
	}
	if PageLabel("/doctors?specialty_id=2") != "پزشکان" {
		t.Fatalf("doctors: %q", PageLabel("/doctors?specialty_id=2"))
	}
}

func TestDoctorSlugFromPath(t *testing.T) {
	if got := DoctorSlugFromPath("/booking/clinic-a/dr-sara"); got != "dr-sara" {
		t.Fatalf("organ slug=%q", got)
	}
	if got := DoctorSlugFromPath("/booking/dr-sara"); got != "dr-sara" {
		t.Fatalf("private slug=%q", got)
	}
	if got := DoctorSlugFromPath("/doctors"); got != "" {
		t.Fatalf("non-booking=%q", got)
	}
}

func TestSpecialtyIDFromURL(t *testing.T) {
	if got := SpecialtyIDFromURL("/doctors?specialty_id=9"); got != 9 {
		t.Fatalf("id=%d", got)
	}
	if got := SpecialtyIDFromURL("/doctors"); got != 0 {
		t.Fatalf("missing id=%d", got)
	}
}

func TestRankedNamedCounts(t *testing.T) {
	got := RankedNamedCounts(map[string]NamedCount{
		"a": {Key: "a", Label: "A", Count: 2},
		"b": {Key: "b", Label: "B", Count: 9},
		"c": {Key: "c", Label: "C", Count: 0},
	}, 1)
	if len(got) != 1 || got[0].Key != "b" {
		t.Fatalf("ranked=%+v", got)
	}
}
