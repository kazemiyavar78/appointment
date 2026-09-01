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
	if len(out) != 1 {
		t.Fatalf("expected in-progress slot visible, got %d", len(out))
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
