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

// Notifier sends booking confirmations to patients via the messaging API.
type Notifier struct {
	SMS *sms.Client
}

// NewNotifier constructs a Notifier.
// Inputs: smsClient may be nil or disabled (notifications become no-ops).
// Output: pointer to Notifier.
func NewNotifier(smsClient *sms.Client) *Notifier {
	return &Notifier{SMS: smsClient}
}

// BookingNotifyParams carries patient and appointment details for confirmation SMS.
type BookingNotifyParams struct {
	Phone       string
	FirstName   string
	LastName    string
	NationalID  string
	ClinicCode  int
	ClinicName  string
	DoctorName  string
	StartsAt    time.Time
	ExternalID  string
	ClientIP    string
}

// NotifyBookingConfirmed sends an SMS confirming a successful booking.
// Inputs: ctx and params with patient phone and appointment details.
// Output: error when messaging fails; nil when disabled or sent successfully.
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
	return nil
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
	when := ""
	if !p.StartsAt.IsZero() {
		when = ptime.New(p.StartsAt).Format("yyyy/MM/dd HH:mm")
	}
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
	if p.ExternalID != "" {
		b.WriteString("\nکد پیگیری: ")
		b.WriteString(p.ExternalID)
	}
	return b.String()
}
