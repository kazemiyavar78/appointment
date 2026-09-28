package statusnotify

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/sms"
)

const (
	roleSuperAdmin    = "super_admin"
	staffMessenger    = "BALEORSMS"
	staffPatientCode  = 10
	notifySendTimeout = 15 * time.Second
)

// Event carries one appointment-site status notification.
type Event struct {
	ClinicID   uint
	ClinicCode int
	ClinicName string
	Message    string
	ClientIP   string
}

// Notifier resolves staff recipients and sends status SMS via SmsService.
type Notifier struct {
	Users   *repository.ManagementUserRepo
	Clinics *repository.ClinicRepo
	SMS     *sms.Client
}

// New constructs a status Notifier.
// Input: users — ریپو کاربران مدیریت؛ clinics — ریپو مراکز؛ smsClient — کلاینت پیامک.
// Output: اشاره‌گر Notifier.
func New(users *repository.ManagementUserRepo, clinics *repository.ClinicRepo, smsClient *sms.Client) *Notifier {
	return &Notifier{Users: users, Clinics: clinics, SMS: smsClient}
}

// NotifyOTPRequested informs staff that a patient requested a booking OTP.
func (n *Notifier) NotifyOTPRequested(clinicID uint, clinicCode int, clinicName, firstName, lastName, nationalID, mobile, clientIP string) {
	n.notifyAsync(Event{
		ClinicID:   clinicID,
		ClinicCode: clinicCode,
		ClinicName: clinicName,
		ClientIP:   clientIP,
		Message:    formatOTPRequested(clinicName, firstName, lastName, nationalID, mobile),
	})
}

// NotifyBookingSuccess informs staff that a patient booked successfully.
func (n *Notifier) NotifyBookingSuccess(clinicID uint, clinicCode int, clinicName, firstName, lastName, nationalID, mobile, doctorName, trackingCode, clientIP string) {
	n.notifyAsync(Event{
		ClinicID:   clinicID,
		ClinicCode: clinicCode,
		ClinicName: clinicName,
		ClientIP:   clientIP,
		Message:    formatBookingSuccess(clinicName, firstName, lastName, nationalID, mobile, doctorName, trackingCode),
	})
}

// OnClinicPresence implements websocket clinic connect/disconnect notifications.
func (n *Notifier) OnClinicPresence(clinicID uint, connected bool) {
	if n == nil || clinicID == 0 {
		return
	}
	clinicName, clinicCode := n.clinicMeta(clinicID)
	msg := formatClinicOffline(clinicName)
	if connected {
		msg = formatClinicOnline(clinicName)
	}
	n.notifyAsync(Event{
		ClinicID:   clinicID,
		ClinicCode: clinicCode,
		ClinicName: clinicName,
		Message:    msg,
	})
}

// NotifyAdminLogin informs staff that an admin completed login, and is also used as the OTP body.
func (n *Notifier) NotifyAdminLogin(clinicID uint, username, roleLabel, extra string) {
	n.notifyAsync(Event{
		ClinicID: clinicID,
		Message:  formatAdminLogin(username, roleLabel, extra),
	})
}

// SendToRecipients resolves staff phones for a clinic and sends one SMS body to each.
// Input: clinicID — مرکز رویداد (۰ یعنی فقط سوپرادمین‌ها)؛ message — متن؛ clinicCode و clientIP برای توکن پیامک.
// Output: خطا اگر گیرنده‌ای نباشد یا همه ارسال‌ها شکست بخورند.
func (n *Notifier) SendToRecipients(ctx context.Context, clinicID uint, clinicCode int, clientIP, message string) error {
	if n == nil || n.SMS == nil || !n.SMS.Enabled() {
		return fmt.Errorf("سرویس پیامک پیکربندی نشده است")
	}
	recipients, err := n.Recipients(clinicID)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		return fmt.Errorf("گیرنده‌ای برای ارسال پیام یافت نشد")
	}
	var first error
	sent := 0
	seen := make(map[string]struct{}, len(recipients))
	for _, user := range recipients {
		phone := digitsOnly(user.PhoneNumber)
		if phone == "" {
			continue
		}
		if _, ok := seen[phone]; ok {
			continue
		}
		seen[phone] = struct{}{}
		code := n.clinicCodeForSend(clinicCode, user.ClinicID)
		if err := n.SMS.Send(ctx, sms.SendParams{
			Phone:       phone,
			FirstName:   user.FullName,
			LastName:    user.Username,
			ClinicCode:  code,
			PatientCode: staffPatientCode,
			MessageText: message,
			Messenger:   staffMessenger,
			IP:          clientIP,
		}); err != nil {
			log.Printf("status SMS to %s failed: %v", phone, err)
			if first == nil {
				first = err
			}
			continue
		}
		sent++
	}
	if sent == 0 {
		if first != nil {
			return first
		}
		return fmt.Errorf("شماره معتبری برای ارسال پیام یافت نشد")
	}
	return nil
}

// Recipients returns super_admins plus clinic users with appointment-status SMS enabled.
// Input: clinicID — مرکز رویداد؛ صفر یعنی فقط سوپرادمین.
// Output: لیست بدون تکرار یا خطای کوئری.
func (n *Notifier) Recipients(clinicID uint) ([]models.ManagementUser, error) {
	if n == nil || n.Users == nil {
		return nil, nil
	}
	admins, err := n.Users.GetActiveByRole(roleSuperAdmin)
	if err != nil {
		return nil, err
	}
	var clinicUsers []models.ManagementUser
	if clinicID != 0 {
		clinicUsers, err = n.Users.GetByClinicAndAppointmentStatus(clinicID)
		if err != nil {
			return nil, err
		}
	}
	return mergeRecipients(admins, clinicUsers), nil
}

func (n *Notifier) notifyAsync(ev Event) {
	if n == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), notifySendTimeout)
		defer cancel()
		if err := n.SendToRecipients(ctx, ev.ClinicID, ev.ClinicCode, ev.ClientIP, ev.Message); err != nil {
			log.Printf("status notify failed clinic=%d: %v", ev.ClinicID, err)
		}
	}()
}

func (n *Notifier) clinicMeta(clinicID uint) (name string, code int) {
	if n == nil || n.Clinics == nil || clinicID == 0 {
		return fmt.Sprintf("مرکز #%d", clinicID), 0
	}
	clinic, err := n.Clinics.GetByIDForAdmin(clinicID)
	if err != nil || clinic == nil {
		return fmt.Sprintf("مرکز #%d", clinicID), 0
	}
	return clinic.Name, clinic.Code
}

func (n *Notifier) clinicCodeForSend(eventCode int, recipientClinicID uint) int {
	if eventCode > 0 {
		return eventCode
	}
	_, code := n.clinicMeta(recipientClinicID)
	return code
}

// mergeRecipients concatenates admin and clinic users and drops duplicate IDs.
func mergeRecipients(admins, clinicUsers []models.ManagementUser) []models.ManagementUser {
	out := make([]models.ManagementUser, 0, len(admins)+len(clinicUsers))
	seen := make(map[uint]struct{}, len(admins)+len(clinicUsers))
	for _, u := range append(admins, clinicUsers...) {
		if u.ID == 0 {
			continue
		}
		if _, ok := seen[u.ID]; ok {
			continue
		}
		seen[u.ID] = struct{}{}
		out = append(out, u)
	}
	return out
}

func formatOTPRequested(clinicName, firstName, lastName, nationalID, mobile string) string {
	var b strings.Builder
	b.WriteString("درخواست کد تایید نوبت")
	appendKV(&b, "مرکز", clinicName)
	appendKV(&b, "بیمار", strings.TrimSpace(firstName+" "+lastName))
	appendKV(&b, "کد ملی", nationalID)
	appendKV(&b, "موبایل", mobile)
	return b.String()
}

func formatBookingSuccess(clinicName, firstName, lastName, nationalID, mobile, doctorName, trackingCode string) string {
	var b strings.Builder
	b.WriteString("رزرو موفق نوبت")
	appendKV(&b, "مرکز", clinicName)
	appendKV(&b, "بیمار", strings.TrimSpace(firstName+" "+lastName))
	appendKV(&b, "کد ملی", nationalID)
	appendKV(&b, "موبایل", mobile)
	appendKV(&b, "پزشک", doctorName)
	appendKV(&b, "کد رهگیری", trackingCode)
	return b.String()
}

func formatClinicOnline(clinicName string) string {
	if clinicName == "" {
		return "یک مرکز به سامانه نوبت‌دهی متصل شد"
	}
	return "مرکز " + clinicName + " به سامانه نوبت‌دهی متصل شد"
}

func formatClinicOffline(clinicName string) string {
	if clinicName == "" {
		return "یک مرکز از سامانه نوبت‌دهی قطع شد"
	}
	return "مرکز " + clinicName + " از سامانه نوبت‌دهی قطع شد"
}

func formatAdminLogin(username, roleLabel, extra string) string {
	var b strings.Builder
	b.WriteString("ورود به پنل مدیریت")
	appendKV(&b, "کاربر", username)
	appendKV(&b, "نقش", roleLabel)
	if extra != "" {
		b.WriteString("\n")
		b.WriteString(extra)
	}
	return b.String()
}

func appendKV(b *strings.Builder, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	b.WriteString("\n")
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(value)
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= '۰' && r <= '۹':
			b.WriteByte(byte('0' + (r - '۰')))
		case r >= '٠' && r <= '٩':
			b.WriteByte(byte('0' + (r - '٠')))
		}
	}
	return b.String()
}
