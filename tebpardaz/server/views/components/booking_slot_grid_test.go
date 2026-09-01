package components

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestBookingSlotGrid_rendersCards(t *testing.T) {
	view := BookingSlotCardView{
		InputName:  "slot_id",
		DoctorName: "دکتر تست",
		Options: []SlotOption{
			{
				Value:     "slot-1",
				Date:      "1405/06/04",
				Time:      "10:00",
				Label:     "1405/06/04 10:00",
				DayNum:    "4",
				MonthAbbr: "شهر",
			},
		},
	}
	var buf bytes.Buffer
	if err := BookingSlotGrid(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "bk-slot-card") {
		t.Fatalf("expected slot card in HTML: %s", html)
	}
	if strings.Contains(html, "bk-slots__empty") {
		t.Fatalf("unexpected empty state")
	}
}
