package admin

import (
	"testing"
)

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
