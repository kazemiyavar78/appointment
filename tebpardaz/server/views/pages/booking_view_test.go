package pages

import (
	"context"
	"strings"
	"testing"

	"tebpardaz/server/views/components"
)

func TestBooking_exposesSuccessURLAndErrorBanner(t *testing.T) {
	var buf strings.Builder
	view := BookingView{
		DoctorName:   "دکتر تست",
		LongDesc:     "متخصص داخلی با تمرکز بر بیماری‌های گوارش",
		SubmitURL:    "/booking/test-doctor",
		BackURL:      "/doctors",
		ErrorMessage: "ظرفیت این نوبت تکمیل شده است",
		Slots: components.SlotPickerView{
			InputName: "slot_id",
			EmptyText: "نوبتی نیست",
		},
	}
	if err := Booking(view).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`id="booking-root"`,
		`class="bk-profile"`,
		`class="bk-doctor-about"`,
		"متخصص داخلی با تمرکز بر بیماری‌های گوارش",
		`class="bk-booking"`,
		`data-success-url="/booking/test-doctor"`,
		`id="booking-server-error"`,
		"ظرفیت این نوبت تکمیل شده است",
		"کد رهگیری",
		`id="bk-otp-channel-notice"`,
		"بازگشت به صفحه اصلی",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("booking page missing %q", want)
		}
	}
	if strings.Contains(html, "متوجه شدم") {
		t.Fatal("booking result modal must not include a dismiss button that returns to previous steps")
	}
	if strings.Contains(html, `class="bk-alert bk-hidden"`) && strings.Contains(html, "ظرفیت این نوبت تکمیل شده است") {
		if strings.Contains(html, `id="booking-server-error" class="bk-alert bk-hidden"`) {
			t.Fatal("server error banner should be visible when ErrorMessage is set")
		}
	}
}
