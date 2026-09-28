package pages

import (
	"context"
	"strings"
	"testing"
)

func TestRingDashOffsetFillsAsTurnApproaches(t *testing.T) {
	if ringDashOffset(0) != "0.0" {
		t.Fatalf("full ring at zero, got %s", ringDashOffset(0))
	}
	if ringDashOffset(20) != "703.7" {
		t.Fatalf("empty ring at 20, got %s", ringDashOffset(20))
	}
	if waitingQueueState(1) != "urgent" || waitingQueueState(3) != "urgent" {
		t.Fatal("1 to 3 must be urgent")
	}
	if waitingQueueState(4) != "approaching" || waitingQueueState(11) != "calm" || waitingQueueState(0) != "now" {
		t.Fatal("unexpected state bands")
	}
}

func TestWaitingQueueLiveMarkup(t *testing.T) {
	view := WaitingQueueFormView{
		ShowLivePanel:  true,
		ClinicName:     "درمانگاه امید",
		AdmissionNo:    "248",
		CSRFToken:      "csrf-token",
		PushPath:       "/248/001/5/push-subscribe",
		WSPath:         "/248/001/5/ws/waiting-queue",
		VAPIDPublicKey: "test-key",
		Status: WaitingQueueStatusView{
			PatientName: "آقای رضایی",
			DoctorName:  "دکتر احمدی",
			VisitTime:   "۱۰:۳۰",
			AheadCount:  2,
		},
	}
	var buf strings.Builder
	if err := WaitingQueueForm(view).Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	for _, needle := range []string{
		"wq-state-urgent",
		"id=\"wq-ring-fg\"",
		"stroke-dashoffset=\"70.4\"",
		"id=\"wq-confirm\"",
		"id=\"wq-sound-toggle\"",
		"id=\"wq-push-toggle\"",
		"data-vapid-key=\"test-key\"",
		"/static/js/waiting_queue.js",
		"آقای رضایی",
	} {
		if !strings.Contains(html, needle) {
			t.Fatalf("markup missing %s", needle)
		}
	}
}
