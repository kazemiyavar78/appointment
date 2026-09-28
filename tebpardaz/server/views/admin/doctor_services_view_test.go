package admin

import (
	"context"
	"strings"
	"testing"

	"tebpardaz/shared/constants"
)

// TestDoctorServicesPageRendersPackageButtons دکمه‌های اعمال بسته را در ستون خدمات نشان می‌دهد.
func TestDoctorServicesPageRendersPackageButtons(t *testing.T) {
	var buf strings.Builder
	view := DoctorServicesView{
		Nav:              BuildAdminNav(NavDoctorServices, string(constants.UserRoleSuperAdmin)),
		SelectedClinicID: 2,
		Doctors: []DoctorOption{
			{ID: 5, Name: "دکتر رضایی"},
		},
		Services: []ServiceAssignOption{
			{ID: 11, Name: "نوار قلب", Packages: []string{"بسته قلب"}},
			{ID: 12, Name: "اکو"},
		},
		Packages: []ServicePackageOption{
			{ID: 3, Name: "چکاپ قلب", ServiceIDs: []uint{11, 12}, Count: 2},
		},
	}
	if err := DoctorServices(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		"اعمال بسته خدمات",
		"چکاپ قلب",
		"بسته قلب",
		`data-ids="11,12"`,
		"applyServicePackage(this)",
		`name="service_ids"`,
		`value="11"`,
		"/admin/services/packages",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered page missing %q", want)
		}
	}
}
