package booking

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"tebpardaz/server/internal/sms"

	ptime "github.com/yaa110/go-persian-calendar"
)

const (
	// reminder4hLead is how long before the visit the 4-hour reminder fires.
	reminder4hLead = 4 * time.Hour
	// reminder4hMinLead requires the visit to be more than 24h from now.
	reminder4hMinLead = 24 * time.Hour
	// reminderDayLead is how long before the visit the day-before reminder fires.
	reminderDayLead = 24 * time.Hour
	// reminderDayMinLead requires the visit to be more than 30h from now.
	reminderDayMinLead = 30 * time.Hour
)

// Notifier sends booking confirmations to patients via the messaging API.
type Notifier struct {
	SMS *sms.Client
	now func() time.Time
}

// NewNotifier constructs a Notifier.
// Inputs: smsClient may be nil or disabled (notifications become no-ops).
// Output: pointer to Notifier.
func NewNotifier(smsClient *sms.Client) *Notifier {
	return &Notifier{
		SMS: smsClient,
		now: time.Now,
	}
}

// BookingNotifyParams carries patient and appointment details for confirmation SMS.
type BookingNotifyParams struct {
	Phone        string
	FirstName    string
	LastName     string
	NationalID   string
	ClinicCode   int
	ClinicName   string
	DoctorName   string
	StartsAt     time.Time
	TrackingCode string
	ClientIP     string
}

// NotifyBookingConfirmed sends the confirmation SMS and schedules eligible reminders.
// Inputs: ctx and params with patient phone and appointment details.
// Output: error when confirmation or a reminder schedule request fails.
func (n *Notifier) NotifyBookingConfirmed(ctx context.Context, params BookingNotifyParams) error {
	if n == nil || n.SMS == nil || !n.SMS.Enabled() {
		return nil
	}

	msg := buildConfirmationMessage(params)
	err := n.SMS.Send(ctx, sms.SendParams{
		Phone:       params.Phone,
		FirstName:   params.FirstName,
		LastName:    params.LastName,
		NationalID:  params.NationalID,
		ClinicCode:  params.ClinicCode,
		PatientCode: 10,
		MessageText: msg,
		Messenger:   "BALEORSMS",
		IP:          params.ClientIP,
	})
	if err != nil {
		log.Printf("booking confirmation SMS failed: %v", err)
		return err
	}

	if err := n.scheduleReminders(ctx, params); err != nil {
		return err
	}
	return nil
}

// scheduleReminders enqueues day-before and 4-hour reminders when lead-time rules match.
// Inputs: ctx and booking notify params.
// Output: first reminder send error, if any; failures are also logged.
func (n *Notifier) scheduleReminders(ctx context.Context, params BookingNotifyParams) error {
	now := time.Now()
	if n.now != nil {
		now = n.now()
	}
	plan := PlanReminders(now, params.StartsAt)
	reminderBody := buildReminderMessage(params)

	var first error
	if plan.DayBeforeAt != nil {
		if err := n.sendScheduledReminder(ctx, params, reminderBody, *plan.DayBeforeAt); err != nil {
			log.Printf("booking day-before reminder schedule failed: %v", err)
			first = err
		}
	}
	if plan.FourHoursAt != nil {
		if err := n.sendScheduledReminder(ctx, params, reminderBody, *plan.FourHoursAt); err != nil {
			log.Printf("booking 4h reminder schedule failed: %v", err)
			if first == nil {
				first = err
			}
		}
	}
	return first
}

// sendScheduledReminder posts one delayed SMS via the messaging API.
// Inputs: ctx, booking params, message body, delivery time.
// Output: error when send fails.
func (n *Notifier) sendScheduledReminder(ctx context.Context, params BookingNotifyParams, message string, at time.Time) error {
	scheduled := at
	return n.SMS.Send(ctx, sms.SendParams{
		Phone:       params.Phone,
		FirstName:   params.FirstName,
		LastName:    params.LastName,
		NationalID:  params.NationalID,
		ClinicCode:  params.ClinicCode,
		PatientCode: 10,
		MessageText: message,
		Messenger:   "BALEORSMS",
		IP:          params.ClientIP,
		ScheduledAt: &scheduled,
	})
}

// SendOTPMessage sends a phone-verification OTP SMS.
// Inputs: ctx, sms client, recipient fields, clinic code, OTP code, client IP.
// Output: error when send fails or client disabled.
func SendOTPMessage(ctx context.Context, client *sms.Client, phone, firstName, lastName, nationalID string, clinicCode int, code, clientIP string) error {
	if client == nil || !client.Enabled() {
		return fmt.Errorf("سرویس پیامک پیکربندی نشده است")
	}
	msg := fmt.Sprintf("کد تایید نوبت‌دهی شما: %s\nاین کد تا ۲ ساعت معتبر است.", code)
	return client.Send(ctx, sms.SendParams{
		Phone:       phone,
		FirstName:   firstName,
		LastName:    lastName,
		NationalID:  nationalID,
		ClinicCode:  clinicCode,
		PatientCode: 10,
		MessageText: msg,
		Messenger:   "BALEORSMS",
		IP:          clientIP,
	})
}

// buildConfirmationMessage formats the post-booking SMS body in Persian.
func buildConfirmationMessage(p BookingNotifyParams) string {
	when := formatVisitWhen(p.StartsAt)
	var b strings.Builder
	b.WriteString("نوبت شما با موفقیت ثبت شد.")
	if p.DoctorName != "" {
		b.WriteString("\nپزشک: ")
		b.WriteString(p.DoctorName)
	}
	if when != "" {
		b.WriteString("\nزمان: ")
		b.WriteString(when)
	}
	if p.ClinicName != "" {
		b.WriteString("\nمرکز: ")
		b.WriteString(p.ClinicName)
	}
	if p.TrackingCode != "" {
		b.WriteString("\nکد رهگیری: ")
		b.WriteString(p.TrackingCode)
	}
	return b.String()
}

// buildReminderMessage formats the appointment reminder SMS body in Persian.
func buildReminderMessage(p BookingNotifyParams) string {
	when := formatVisitWhen(p.StartsAt)
	var b strings.Builder
	b.WriteString("یادآوری نوبت")
	if p.DoctorName != "" {
		b.WriteString("\nپزشک: ")
		b.WriteString(p.DoctorName)
	}
	if when != "" {
		b.WriteString("\nزمان: ")
		b.WriteString(when)
	}
	if p.ClinicName != "" {
		b.WriteString("\nمرکز: ")
		b.WriteString(p.ClinicName)
	}
	return b.String()
}

// formatVisitWhen returns a Jalali yyyy/MM/dd HH:mm label for StartsAt.
func formatVisitWhen(startsAt time.Time) string {
	if startsAt.IsZero() {
		return ""
	}
	return ptime.New(startsAt).Format("yyyy/MM/dd HH:mm")
}

// ReminderSchedule describes which delayed reminders should be queued for a visit.
type ReminderSchedule struct {
	DayBeforeAt *time.Time
	FourHoursAt *time.Time
}

// PlanReminders decides reminder send times from now and the visit start.
// Inputs: now (reference instant), startsAt (appointment time).
// Output: schedule with pointers set only when each rule matches.
func PlanReminders(now, startsAt time.Time) ReminderSchedule {
	var out ReminderSchedule
	if startsAt.IsZero() || !startsAt.After(now) {
		return out
	}
	until := startsAt.Sub(now)
	if until > reminderDayMinLead {
		at := startsAt.Add(-reminderDayLead)
		out.DayBeforeAt = &at
	}
	if until > reminder4hMinLead {
		at := startsAt.Add(-reminder4hLead)
		out.FourHoursAt = &at
	}
	return out
}
