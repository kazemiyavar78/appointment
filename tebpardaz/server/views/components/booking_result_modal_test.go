package components

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestBookingResultModal_hasTrackingLabel(t *testing.T) {
	var buf bytes.Buffer
	if err := BookingResultModal(BookingResultModalView{}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`id="bk-modal-tracking-row"`,
		"کد رهگیری",
		`id="bk-modal-tracking"`,
		`id="bk-modal-home"`,
		"بازگشت به صفحه اصلی",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("modal missing %q: %s", want, html)
		}
	}
	if strings.Contains(html, "متوجه شدم") {
		t.Fatal("result modal must not offer going back to previous booking steps")
	}
	if strings.Contains(html, "🔖") {
		t.Fatalf("tracking row should use a text label, not a bare emoji")
	}
}
