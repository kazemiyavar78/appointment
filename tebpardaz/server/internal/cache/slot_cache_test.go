package cache

import (
	"testing"
	"time"

	"tebpardaz/server/internal/models"
)

func TestAvailableForDoctor_filtersPastEnd(t *testing.T) {
	loc := time.FixedZone("IRST", 3*3600+30*60)
	start := time.Date(2026, 8, 26, 1, 0, 0, 0, loc)
	end := start.Add(10 * time.Minute)
	c := NewSlotCache(New(SlotTTL, slotCleanupInterval))
	bag := clinicSlotBag{
		ByDoctor: map[uint][]models.DoctorSlot{
			1217: {{
				DoctorID:       1217,
				ClinicID:       1,
				StartsAt:       start,
				EndsAt:         end,
				Capacity:       30,
				BookedCount:    28,
				IsAvailable:    true,
				ExternalSlotID: "test-slot",
			}},
		},
	}
	c.store.SetWithTTL(clinicKey(1), bag, SlotTTL)

	from := time.Date(2026, 8, 26, 1, 35, 0, 0, loc)
	out := c.AvailableForDoctor(1, 1217, from, 0)
	if len(out) != 0 {
		t.Fatalf("expected ended slot filtered, got %d", len(out))
	}

	from = time.Date(2026, 8, 26, 1, 5, 0, 0, loc)
	out = c.AvailableForDoctor(1, 1217, from, 0)
	if len(out) != 0 {
		t.Fatalf("expected in-progress today slot hidden after presence start, got %d", len(out))
	}
}

func TestAvailableForDoctor_hidesTodayWithinOneHour(t *testing.T) {
	loc := time.FixedZone("IRST", 3*3600+30*60)
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, loc)
	later := time.Date(2026, 9, 22, 12, 0, 0, 0, loc)
	soon := time.Date(2026, 9, 22, 10, 30, 0, 0, loc)
	exact := time.Date(2026, 9, 22, 11, 0, 0, 0, loc)
	tomorrowSoon := time.Date(2026, 9, 23, 0, 20, 0, 0, loc)
	c := NewSlotCache(New(SlotTTL, slotCleanupInterval))
	c.store.SetWithTTL(clinicKey(1), clinicSlotBag{
		ByDoctor: map[uint][]models.DoctorSlot{
			7: {
				slotAt(7, soon, "soon"),
				slotAt(7, exact, "exact"),
				slotAt(7, later, "later"),
				slotAt(7, tomorrowSoon, "tomorrow"),
			},
		},
	}, SlotTTL)

	out := c.AvailableForDoctor(1, 7, now, 0)
	got := map[string]bool{}
	for _, slot := range out {
		got[slot.ExternalSlotID] = true
	}
	if got["soon"] {
		t.Fatal("today slot under 1 hour must be hidden")
	}
	if !got["exact"] || !got["later"] || !got["tomorrow"] {
		t.Fatalf("expected exact-hour, later today, and next-day slots, got %#v", got)
	}
}

func TestTodayPresencePassed(t *testing.T) {
	loc := time.FixedZone("IRST", 3*3600+30*60)
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, loc)
	started := models.DoctorSlot{StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour)}
	upcoming := models.DoctorSlot{StartsAt: now.Add(2 * time.Hour), EndsAt: now.Add(3 * time.Hour)}
	tomorrow := models.DoctorSlot{StartsAt: now.Add(24 * time.Hour), EndsAt: now.Add(25 * time.Hour)}
	if !TodayPresencePassed(started, now) {
		t.Fatal("expected today's started slot to be past presence")
	}
	if TodayPresencePassed(upcoming, now) || TodayPresencePassed(tomorrow, now) {
		t.Fatal("future slots must stay on the appointment list")
	}
}

func TestDropSlot_removesFullSlot(t *testing.T) {
	loc := time.FixedZone("IRST", 3*3600+30*60)
	start := time.Date(2026, 9, 22, 14, 0, 0, 0, loc)
	c := NewSlotCache(New(SlotTTL, slotCleanupInterval))
	c.store.SetWithTTL(clinicKey(1), clinicSlotBag{
		ByDoctor: map[uint][]models.DoctorSlot{
			7: {
				slotAt(7, start, "full"),
				slotAt(7, start.Add(2*time.Hour), "open"),
			},
		},
	}, SlotTTL)

	if !c.DropSlot(1, 7, "full", start) {
		t.Fatal("expected slot removal")
	}
	left := c.AvailableForDoctor(1, 7, start.Add(-3*time.Hour), 0)
	if len(left) != 1 || left[0].ExternalSlotID != "open" {
		t.Fatalf("expected only open slot, got %+v", left)
	}
}

func slotAt(doctorID uint, start time.Time, id string) models.DoctorSlot {
	return models.DoctorSlot{
		DoctorID:       doctorID,
		ClinicID:       1,
		StartsAt:       start,
		EndsAt:         start.Add(30 * time.Minute),
		Capacity:       10,
		BookedCount:    1,
		IsAvailable:    true,
		ExternalSlotID: id,
	}
}

func TestAvailableForDoctor_wrongDoctorID(t *testing.T) {
	loc := time.FixedZone("IRST", 3*3600+30*60)
	start := time.Now().In(loc).Add(2 * time.Hour)
	c := NewSlotCache(New(SlotTTL, slotCleanupInterval))
	bag := clinicSlotBag{
		ByDoctor: map[uint][]models.DoctorSlot{
			1217: {{
				DoctorID:    1217,
				ClinicID:    1,
				StartsAt:    start,
				EndsAt:      start.Add(10 * time.Minute),
				IsAvailable: true,
			}},
		},
	}
	c.store.SetWithTTL(clinicKey(1), bag, SlotTTL)

	out := c.AvailableForDoctor(1, 999, time.Now(), 0)
	if len(out) != 0 {
		t.Fatalf("expected empty for wrong doctor, got %d", len(out))
	}
}
