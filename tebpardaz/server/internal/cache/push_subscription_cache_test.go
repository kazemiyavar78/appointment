package cache

import (
	"testing"
	"time"
)

func TestPushSubscriptionCache_NotifiesOnlyOnDecrease(t *testing.T) {
	store := New(time.Hour, time.Minute)
	subs := NewPushSubscriptionCache(store)
	const clinicID uint = 7
	if err := subs.Add(clinicID, 248, "0012345678", "https://push.example/1", "p256", "auth", "/248/0012345678/waiting-queue"); err != nil {
		t.Fatal(err)
	}
	if err := subs.Add(clinicID, 248, "0012345678", "https://push.example/2", "p256b", "authb", "/248/0012345678/waiting-queue"); err != nil {
		t.Fatal(err)
	}
	if !subs.HasClinic(clinicID) {
		t.Fatal("expected live subscription")
	}

	if jobs := subs.CollectDecreases(clinicID, 248, "0012345678", true, 8); len(jobs) != 0 {
		t.Fatalf("first observation must not notify, got %d", len(jobs))
	}
	if jobs := subs.CollectDecreases(clinicID, 248, "0012345678", true, 8); len(jobs) != 0 {
		t.Fatalf("same position must not notify, got %d", len(jobs))
	}
	jobs := subs.CollectDecreases(clinicID, 248, "0012345678", true, 3)
	if len(jobs) != 2 {
		t.Fatalf("decrease must notify both devices, got %d", len(jobs))
	}
	if jobs[0].Strong || jobs[0].Ahead != 3 {
		t.Fatalf("unexpected job %+v", jobs[0])
	}
	subs.MarkNotified(jobs[0])
	subs.MarkNotified(jobs[1])

	again := subs.CollectDecreases(clinicID, 248, "0012345678", true, 3)
	if len(again) != 0 {
		t.Fatal("notified position must not repeat")
	}
	zero := subs.CollectDecreases(clinicID, 248, "0012345678", true, 0)
	if len(zero) != 2 || !zero[0].Strong {
		t.Fatalf("zero must be a strong alert, got %+v", zero)
	}

	subs.RemoveEndpoint(clinicID, 248, "0012345678", "https://push.example/1")
	subs.RemoveEndpoint(clinicID, 248, "0012345678", "https://push.example/2")
	if subs.HasClinic(clinicID) {
		t.Fatal("clinic should have no subscribers after removal")
	}
}
