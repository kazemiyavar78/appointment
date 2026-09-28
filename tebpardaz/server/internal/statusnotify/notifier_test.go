package statusnotify

import (
	"testing"

	"tebpardaz/server/internal/models"
)

func TestMergeRecipients_dedupesByID(t *testing.T) {
	admins := []models.ManagementUser{{ID: 1, Username: "sa"}, {ID: 2, Username: "sa2"}}
	clinic := []models.ManagementUser{{ID: 2, Username: "dup"}, {ID: 3, Username: "clinic"}}
	got := mergeRecipients(admins, clinic)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	if got[0].Username != "sa" || got[1].Username != "sa2" || got[2].Username != "clinic" {
		t.Fatalf("unexpected order/names: %+v", got)
	}
}

func TestFormatOTPRequested_includesPatient(t *testing.T) {
	msg := formatOTPRequested("نور", "علی", "محمدی", "001", "0912")
	if !containsAll(msg, "درخواست کد تایید نوبت", "نور", "علی محمدی", "001", "0912") {
		t.Fatalf("message missing fields: %q", msg)
	}
}

func TestFormatBookingSuccess_includesTracking(t *testing.T) {
	msg := formatBookingSuccess("نور", "علی", "محمدی", "001", "0912", "دکتر کیان", "A1")
	if !containsAll(msg, "رزرو موفق نوبت", "دکتر کیان", "A1") {
		t.Fatalf("message missing fields: %q", msg)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if p == "" || !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()))
}
