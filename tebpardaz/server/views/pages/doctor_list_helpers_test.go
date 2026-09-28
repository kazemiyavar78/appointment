package pages

import "testing"

// TestDoctorListPageURLUsesClinicSlug puts the clinic slug in the public query, not the numeric id.
func TestDoctorListPageURLUsesClinicSlug(t *testing.T) {
	got := doctorListPageURL(DoctorListView{
		FormAction: "/doctors",
		ClinicID:   9,
		ClinicSlug: "chamran",
	}, 1)
	if got != "/doctors?clinic=chamran" {
		t.Fatalf("got %q, want /doctors?clinic=chamran", got)
	}
}

// TestDoctorListClearURLRemovesClinicSlug drops the clinic filter without leaving clinic_id behind.
func TestDoctorListClearURLRemovesClinicSlug(t *testing.T) {
	got := doctorListClearURL(DoctorListView{
		FormAction:  "/doctors",
		Query:       "رضا",
		ClinicID:    9,
		ClinicSlug:  "chamran",
		SpecialtyID: 3,
	}, "clinic")
	if got != "/doctors?q=%D8%B1%D8%B6%D8%A7&specialty_id=3" {
		t.Fatalf("got %q", got)
	}
}

// TestDoctorListRadioSplitKeepsShortListsClosed hides the disclosure when there are at most five options.
func TestDoctorListRadioSplitKeepsShortListsClosed(t *testing.T) {
	items := []doctorListRadioItem{{Value: "1", Label: "داخلی", Selected: true}}
	visible, extra, expanded := doctorListRadioSplit(items)
	if len(visible) != 1 || extra != nil || expanded {
		t.Fatalf("visible=%d extra=%v expanded=%v", len(visible), extra, expanded)
	}
}

// TestDoctorListRadioSplitExpandsWhenSelectionIsHidden opens the group when the active option is past the first five.
func TestDoctorListRadioSplitExpandsWhenSelectionIsHidden(t *testing.T) {
	items := make([]doctorListRadioItem, 6)
	for i := range items {
		items[i] = doctorListRadioItem{Value: intToString(i + 1), Label: "گزینه"}
	}
	items[5].Selected = true
	visible, extra, expanded := doctorListRadioSplit(items)
	if len(visible) != 5 || len(extra) != 1 || !expanded || !extra[0].Selected {
		t.Fatalf("visible=%d extra=%d expanded=%v", len(visible), len(extra), expanded)
	}
}

// TestDoctorListRadioSplitStaysClosedWhenSelectionIsVisible keeps extra options collapsed when the choice is in the preview.
func TestDoctorListRadioSplitStaysClosedWhenSelectionIsVisible(t *testing.T) {
	items := make([]doctorListRadioItem, 6)
	for i := range items {
		items[i] = doctorListRadioItem{Value: intToString(i + 1), Label: "گزینه"}
	}
	items[0].Selected = true
	_, extra, expanded := doctorListRadioSplit(items)
	if len(extra) != 1 || expanded {
		t.Fatalf("extra=%d expanded=%v", len(extra), expanded)
	}
}

// TestDoctorListClinicRadioGroupSkipsEmptySlug drops clinics that cannot be submitted as a public slug.
func TestDoctorListClinicRadioGroupSkipsEmptySlug(t *testing.T) {
	group := doctorListClinicRadioGroup(DoctorListView{
		ClinicSlug: "chamran",
		Clinics: []DoctorListFilterOption{
			{ID: 1, Name: "بدون اسلاگ", Slug: ""},
			{ID: 9, Name: "چمران", Slug: "chamran"},
		},
	}, "d")
	if group.Name != "clinic" || group.MoreID != "clinic-more-d" || len(group.Items) != 1 {
		t.Fatalf("group=%+v", group)
	}
	if group.Items[0].Value != "chamran" || !group.Items[0].Selected || group.AllSelected {
		t.Fatalf("item=%+v all=%v", group.Items[0], group.AllSelected)
	}
	if group.CountLabel != "۱ مرکز" {
		t.Fatalf("count=%q", group.CountLabel)
	}
}

// TestDoctorListHasFiltersUsesClinicSlug treats an empty slug as no clinic filter.
func TestDoctorListHasFiltersUsesClinicSlug(t *testing.T) {
	if doctorListHasFilters(DoctorListView{ClinicID: 9}) {
		t.Fatal("numeric clinic id alone should not count as an active public filter")
	}
	if !doctorListHasFilters(DoctorListView{ClinicSlug: "chamran"}) {
		t.Fatal("clinic slug should count as an active filter")
	}
}
