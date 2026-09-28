package booking

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"tebpardaz/server/internal/config"
	"tebpardaz/server/internal/sms"
)

func TestPlanReminders_bothWhenFar(t *testing.T) {
	// نوبت 1405/06/21 14:00 ، اکنون 1405/06/20 08:00 → بیش از ۳۰ ساعت
	startsAt := time.Date(2026, 9, 12, 14, 0, 0, 0, time.Local) // approx Gregorian for logic
	now := startsAt.Add(-30*time.Hour - time.Minute)

	plan := PlanReminders(now, startsAt)
	if plan.DayBeforeAt == nil {
		t.Fatal("expected day-before reminder")
	}
	if plan.FourHoursAt == nil {
		t.Fatal("expected 4h reminder")
	}
	if !plan.DayBeforeAt.Equal(startsAt.Add(-24 * time.Hour)) {
		t.Fatalf("day-before at %v want %v", plan.DayBeforeAt, startsAt.Add(-24*time.Hour))
	}
	if !plan.FourHoursAt.Equal(startsAt.Add(-4 * time.Hour)) {
		t.Fatalf("4h at %v want %v", plan.FourHoursAt, startsAt.Add(-4*time.Hour))
	}
}

func TestPlanReminders_only4hWhenBetween24And30(t *testing.T) {
	// بیش از ۲۴ و حداکثر ۳۰ ساعت → فقط یادآوری ۴ ساعته
	startsAt := time.Date(2026, 9, 12, 14, 0, 0, 0, time.Local)
	now := startsAt.Add(-25 * time.Hour)

	plan := PlanReminders(now, startsAt)
	if plan.DayBeforeAt != nil {
		t.Fatalf("day-before should be skipped, got %v", plan.DayBeforeAt)
	}
	if plan.FourHoursAt == nil {
		t.Fatal("expected 4h reminder")
	}
}

func TestPlanReminders_noneWhenUnder24h(t *testing.T) {
	startsAt := time.Date(2026, 9, 12, 14, 0, 0, 0, time.Local)
	now := startsAt.Add(-10 * time.Hour)

	plan := PlanReminders(now, startsAt)
	if plan.DayBeforeAt != nil || plan.FourHoursAt != nil {
		t.Fatalf("expected no reminders, got day=%v 4h=%v", plan.DayBeforeAt, plan.FourHoursAt)
	}
}

func TestBuildConfirmationMessage_usesTrackingCode(t *testing.T) {
	msg := buildConfirmationMessage(BookingNotifyParams{
		DoctorName:   "دکتر تست",
		ClinicName:   "مرکز آزمایش",
		TrackingCode: "128",
		StartsAt:     time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC),
	})
	for _, want := range []string{"کد رهگیری: 128", "دکتر تست", "مرکز آزمایش"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("confirmation SMS missing %q in %s", want, msg)
		}
	}
}

func TestPlanReminders_exactBoundaryExcluded(t *testing.T) {
	startsAt := time.Date(2026, 9, 12, 14, 0, 0, 0, time.Local)

	plan24 := PlanReminders(startsAt.Add(-24*time.Hour), startsAt)
	if plan24.FourHoursAt != nil {
		t.Fatal("at exactly 24h, 4h reminder must not schedule")
	}

	plan30 := PlanReminders(startsAt.Add(-30*time.Hour), startsAt)
	if plan30.DayBeforeAt != nil {
		t.Fatal("at exactly 30h, day-before reminder must not schedule")
	}
	if plan30.FourHoursAt == nil {
		t.Fatal("at exactly 30h, 4h reminder should still schedule (>24h)")
	}
}

// liveSMSLogTransport logs outbound messaging requests/responses during live tests.
type liveSMSLogTransport struct {
	t    *testing.T
	base http.RoundTripper
}

// RoundTrip dumps the JSON body and HTTP status, then forwards the request.
func (tr *liveSMSLogTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	tr.t.Logf("messaging %s %s\n%s", req.Method, req.URL, body)

	base := tr.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	tr.t.Logf("messaging status=%d body=%s", resp.StatusCode, bytes.TrimSpace(respBody))
	return resp, nil
}

// TestNotifyBookingConfirmed_schedules4HourReminder queues a real 4-hour reminder
// against the configured messaging API (confirmation/OTP is skipped).
func TestNotifyBookingConfirmed_schedules4HourReminder(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	baseURL := strings.Replace(cfg.MessagingBaseURL, "localhost", "192.168.1.100", 1)
	client, err := sms.NewClient(sms.Config{
		BaseURL:    baseURL,
		AuthToken:  cfg.MessagingAuthToken,
		AESKeyB64:  cfg.MessagingAESKey,
		HMACKeyB64: cfg.MessagingHMACKey,
		UserCode:   cfg.MessagingUserCode,
		Messenger:  cfg.MessagingMessenger,
		Operator:   cfg.MessagingOperator,
		HTTPClient: &http.Client{
			Timeout: 20 * time.Second,
			Transport: &liveSMSLogTransport{
				t:    t,
				base: http.DefaultTransport,
			},
		},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if !client.Enabled() {
		t.Fatal("messaging client is not configured")
	}

	phone := strings.TrimSpace(os.Getenv("LIVE_SMS_PHONE"))
	if phone == "" {
		phone = "09028151964"
	}
	clinicCode := 1001
	if raw := strings.TrimSpace(os.Getenv("LIVE_SMS_CLINIC_CODE")); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil {
			t.Fatalf("LIVE_SMS_CLINIC_CODE: %v", convErr)
		}
		clinicCode = n
	}

	now := time.Now()
	startsAt := now.Add(25*time.Hour + time.Minute)
	plan := PlanReminders(now, startsAt)
	if plan.FourHoursAt == nil {
		t.Fatal("expected 4h reminder for a visit 25h ahead")
	}
	if plan.DayBeforeAt != nil {
		t.Fatal("did not expect day-before reminder for a visit 25h ahead")
	}

	n := NewNotifier(client)
	n.now = func() time.Time { return now }
	params := BookingNotifyParams{
		Phone:        phone,
		FirstName:    "علی",
		LastName:     "تست",
		NationalID:   "0012345678",
		ClinicCode:   clinicCode,
		ClinicName:   "مرکز آزمایش",
		DoctorName:   "دکتر تست",
		StartsAt:     startsAt,
		TrackingCode: "LIVE4H",
		ClientIP:     "127.0.0.1",
	}
	if err := n.sendScheduledReminder(context.Background(), params, buildReminderMessage(params), *plan.FourHoursAt); err != nil {
		t.Fatalf("sendScheduledReminder: %v", err)
	}
	t.Logf("4h reminder queued for %s to %s (clinic %d)", plan.FourHoursAt.In(time.FixedZone("IRST", 3*3600+30*60)).Format(time.RFC3339), phone, clinicCode)
}
