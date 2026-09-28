package booking

import "testing"

// TestNormalizeAppointmentIP trims and caps IPs to the appointment column length.
func TestNormalizeAppointmentIP(t *testing.T) {
	if got := normalizeAppointmentIP("  203.0.113.10  "); got != "203.0.113.10" {
		t.Fatalf("trim: %q", got)
	}
	if got := normalizeAppointmentIP(""); got != "" {
		t.Fatalf("empty: %q", got)
	}
	long := make([]byte, 60)
	for i := range long {
		long[i] = 'a'
	}
	if got := normalizeAppointmentIP(string(long)); len(got) != maxAppointmentIPLen {
		t.Fatalf("capped length=%d want %d", len(got), maxAppointmentIPLen)
	}
}
