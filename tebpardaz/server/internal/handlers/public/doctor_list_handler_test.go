package public

import "testing"

// TestClinicFilterIDAllowedReportsMembership checks tenant-scope membership for the clinic filter.
func TestClinicFilterIDAllowedReportsMembership(t *testing.T) {
	if clinicFilterIDAllowed(nil, 9) {
		t.Fatal("empty allow-list should reject")
	}
	if !clinicFilterIDAllowed([]uint{3, 9}, 9) {
		t.Fatal("expected 9 to be allowed")
	}
	if clinicFilterIDAllowed([]uint{3, 9}, 4) {
		t.Fatal("expected 4 to be rejected")
	}
}

// TestResolveClinicFilterEmptyQueryReturnsZero keeps an empty clinic query unfiltered.
func TestResolveClinicFilterEmptyQueryReturnsZero(t *testing.T) {
	id, slug := resolveClinicFilter(nil, "  ", []uint{9})
	if id != 0 || slug != "" {
		t.Fatalf("got id=%d slug=%q, want empty", id, slug)
	}
}
