package components

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestBookingOTPPanel_showsChannelNotice(t *testing.T) {
	var buf bytes.Buffer
	if err := BookingOTPPanel(BookingOTPPanelView{CodeLength: 5}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`id="bk-otp-channel-notice"`,
		"پیامک",
		"بله",
		`<strong>`,
		`id="booking-otp-verify"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("otp panel missing %q: %s", want, html)
		}
	}
}
