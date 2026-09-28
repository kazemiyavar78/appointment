package sms

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestFormatScheduledAt_convertsUnixUTCToIranOffset checks Unix/UTC slot times
// serialize as RFC3339 with +03:30, matching the messaging API example.
func TestFormatScheduledAt_convertsUnixUTCToIranOffset(t *testing.T) {
	iran := time.FixedZone("IRST", 3*3600+30*60)
	local := time.Date(2026, 9, 15, 10, 30, 0, 0, iran)
	unixUTC := time.Unix(local.Unix(), 0) // same instant, UTC location (slot cache)

	got := formatScheduledAt(unixUTC)
	want := "2026-09-15T10:30:00+03:30"
	if got != want {
		t.Fatalf("formatScheduledAt=%q want %q", got, want)
	}
}

// TestSend_scheduledReminderKeepsClinicPackageFields posts a delayed SMS and checks
// scheduled_at is Iran RFC3339 while clinic_code stays set for package billing.
func TestSend_scheduledReminderKeepsClinicPackageFields(t *testing.T) {
	iran := time.FixedZone("IRST", 3*3600+30*60)
	at := time.Date(2026, 9, 15, 10, 30, 0, 0, iran)

	var raw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, AuthToken: "test-token"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	err = client.Send(context.Background(), SendParams{
		Phone:       "09028151964",
		FirstName:   "علی",
		LastName:    "تست",
		NationalID:  "0012345678",
		ClinicCode:  1001,
		PatientCode: 10,
		MessageText: "یادآوری نوبت",
		Messenger:   "BALEORSMS",
		IP:          "1.2.3.4",
		ScheduledAt: &at,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["message_text"] != "یادآوری نوبت" {
		t.Fatalf("message_text=%v", body["message_text"])
	}
	if body["messenger"] != "BALEORSMS" {
		t.Fatalf("messenger=%v", body["messenger"])
	}
	if body["clinic_code"] != float64(1001) {
		t.Fatalf("clinic_code=%v", body["clinic_code"])
	}
	if body["scheduled_at"] != "2026-09-15T10:30:00+03:30" {
		t.Fatalf("scheduled_at=%v", body["scheduled_at"])
	}
	rels, _ := body["patient_relations"].([]any)
	if len(rels) != 1 {
		t.Fatalf("patient_relations=%v", body["patient_relations"])
	}
	rel, _ := rels[0].(map[string]any)
	if rel["phone"] != "09028151964" {
		t.Fatalf("phone=%v", rel["phone"])
	}
}
