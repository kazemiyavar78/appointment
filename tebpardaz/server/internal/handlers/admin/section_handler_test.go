package admin

import (
	"testing"

	"tebpardaz/server/internal/models"
	adminviews "tebpardaz/server/views/admin"
)

// TestSectionFlashMessageDoctorCodes پیام‌های فارسی انتصاب پزشک به بخش را بررسی می‌کند.
func TestSectionFlashMessageDoctorCodes(t *testing.T) {
	cases := map[string]string{
		"doctor_assigned":     "پزشک تأییدشده با موفقیت به این بخش اضافه شد.",
		"doctor_removed":      "پزشک از این بخش حذف شد.",
		"doctor_already":      "این پزشک از قبل به بخش اضافه شده است.",
		"doctor_not_approved": "فقط پزشکان تأییدشده همین مرکز را می‌توان به بخش افزود.",
		"doctor_invalid":      "لطفاً یک پزشک معتبر انتخاب کنید.",
	}
	for code, want := range cases {
		if got := sectionFlashMessage(code); got != want {
			t.Errorf("sectionFlashMessage(%q) = %q; want %q", code, got, want)
		}
	}
}

// TestSectionDoctorDisplayName نام نمایشی پزشک را از فیلدهای مختلف می‌سازد.
func TestSectionDoctorDisplayName(t *testing.T) {
	if got := sectionDoctorDisplayName(models.Doctor{Name: " دکتر رضایی "}); got != "دکتر رضایی" {
		t.Fatalf("got %q", got)
	}
	if got := sectionDoctorDisplayName(models.Doctor{FirstName: "علی", LastName: "احمدی"}); got != "علی احمدی" {
		t.Fatalf("got %q", got)
	}
}

// TestToSectionDoctorRows ردیف‌های انتصاب را به مدل نما تبدیل می‌کند.
func TestToSectionDoctorRows(t *testing.T) {
	rows := toSectionDoctorRows([]models.SectionDoctor{
		{
			DoctorID:  7,
			SortOrder: 2,
			Doctor: models.Doctor{
				Name:           "دکتر نوری",
				DoctorSystemID: 123,
				IsApproved:     true,
				IsActive:       true,
				Specialty:      models.Specialty{Name: "قلب"},
			},
		},
	})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	want := adminviews.SectionDoctorRow{
		DoctorID:      7,
		Name:          "دکتر نوری",
		SpecialtyName: "قلب",
		SystemID:      123,
		IsApproved:    true,
		IsActive:      true,
		SortOrder:     2,
	}
	if rows[0] != want {
		t.Fatalf("got %+v; want %+v", rows[0], want)
	}
}

// TestSanitizeSlug tests URL slug formatting.
func TestSanitizeSlug(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Lab", "lab"},
		{"  Medical Imaging  ", "medical-imaging"},
		{"CT-Scan", "ct-scan"},
		{"", ""},
	}

	for _, c := range cases {
		got := sanitizeSlug(c.input)
		if got != c.expected {
			t.Errorf("sanitizeSlug(%q) = %q; want %q", c.input, got, c.expected)
		}
	}
}

// TestParseServicesLines tests extracting up to max services from raw string.
func TestParseServicesLines(t *testing.T) {
	raw := `
		Service 1
		Service 2
		Service 3
		Service 4
		Service 5
		Service 6
	`
	services := parseServicesLines(raw, 5)
	if len(services) != 5 {
		t.Fatalf("expected 5 services, got %d", len(services))
	}
	if services[0] != "Service 1" {
		t.Errorf("expected 'Service 1', got %q", services[0])
	}
	if services[4] != "Service 5" {
		t.Errorf("expected 'Service 5', got %q", services[4])
	}
}
