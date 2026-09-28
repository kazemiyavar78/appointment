package public

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/tenant"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

type otpSendBody struct {
	CSRFToken  string `json:"csrf_token"`
	Mobile     string `json:"mobile"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	NationalID string `json:"national_id"`
	ClinicPath string `json:"clinic_path"`
	// Resend فقط وقتی true است که بیمار دکمه «ارسال مجدد کد» را زده باشد.
	Resend bool `json:"resend"`
}

type otpVerifyBody struct {
	CSRFToken string `json:"csrf_token"`
	Mobile    string `json:"mobile"`
	Code      string `json:"code"`
}

// SendOTP اعتبارسنجی موبایل، تولید OTP و ارسال پیامک.
// ورودی: JSON با csrf_token، موبایل، نام، کدملی. خروجی: JSON {ok, message, ...}.
func (h *BookingHandler) SendOTP(c *gin.Context) {
	tc, ok := tenant.FromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "دسترسی نامعتبر"})
		return
	}
	var body otpSendBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "درخواست نامعتبر است"})
		return
	}
	if h.CSRF == nil || !h.CSRF.Verify(c.Request, body.CSRFToken) {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "توکن امنیتی نامعتبر است؛ صفحه را تازه کنید"})
		return
	}
	if h.OTP == nil || h.SMS == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "message": "سرویس تایید موبایل در دسترس نیست"})
		return
	}

	mobile := digitsOnlyPublic(body.Mobile)
	if !booking.MobileLooksValid(mobile) {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "موبایل باید ۱۱ رقم و با ۰۹ شروع شود"})
		return
	}
	firstName := strings.TrimSpace(body.FirstName)
	lastName := strings.TrimSpace(body.LastName)
	nationalID := digitsOnlyPublic(body.NationalID)

	if err := booking.ValidatePatientIdentity(firstName, lastName, nationalID, mobile); err != nil {
		msg := err.Error()
		if i := strings.Index(msg, ": "); i >= 0 {
			msg = msg[i+2:]
		}
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": msg})
		return
	}

	clinicID, clinicCode, clinicName, err := h.resolveOTPClinic(tc, body.ClinicPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "مرکز یافت نشد"})
		return
	}

	// اگر نشست فعال وجود دارد، بلیط جدید صادر می‌شود بدون ارسال پیامک.
	if remaining, expiresAt, hasSession := h.OTP.ActiveSessionForMobile(mobile); hasSession && remaining > 0 {
		ticketResult, err := h.OTP.IssueSessionTicket(mobile)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "خطا در صدور نشست"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":                 true,
			"message":            "موبایل قبلاً تایید شده؛ می‌توانید نوبت ثبت کنید",
			"session_active":     true,
			"otp_ticket":         ticketResult.Ticket,
			"bookings_remaining": remaining,
			"expires_at":         expiresAt.Unix(),
		})
		return
	}

	patientID, err := h.persistOTPPatient(nationalID, firstName, lastName, mobile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "خطا در ذخیره اطلاعات بیمار"})
		return
	}

	// بازگشت به فرم نباید کد جدید بسازد؛ فقط درخواست صریح «ارسال مجدد» پیامک تازه می‌فرستد.
	if !body.Resend {
		if wait, _, pending := h.OTP.PendingCode(mobile, nationalID); pending {
			sec := int(wait.Seconds())
			if sec < 0 {
				sec = 0
			}
			c.JSON(http.StatusOK, gin.H{
				"ok":              true,
				"code_pending":    true,
				"message":         "کد تأیید قبلاً ارسال شده؛ همان کد را وارد کنید",
				"retry_after_sec": sec,
			})
			return
		}
	}

	result, retryAfter, err := h.OTP.CreateAndStore(booking.OTPCreateInput{
		Mobile:     mobile,
		FirstName:  firstName,
		LastName:   lastName,
		NationalID: nationalID,
		ClinicID:   clinicID,
		PatientID:  patientID,
		IPAddress:  c.ClientIP(),
	})
	if err != nil {
		if errors.Is(err, booking.ErrOTPRateLimited) {
			sec := int(retryAfter.Seconds())
			if sec < 1 {
				sec = 1
			}
			c.JSON(http.StatusTooManyRequests, gin.H{
				"ok":              false,
				"message":         "حداکثر ۲ بار در ۲ ساعت می‌توانید کد دریافت کنید؛ بعداً تلاش کنید",
				"retry_after_sec": sec,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "خطا در ایجاد کد تایید"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	if err := booking.SendOTPMessage(ctx, h.SMS, mobile, firstName, lastName, nationalID, clinicCode, result.Code, c.ClientIP()); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "message": "ارسال پیامک ناموفق بود؛ دوباره تلاش کنید"})
		return
	}
	if err := h.OTP.MarkDelivered(mobile, patientID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "کد ارسال شد اما ثبت پیگیری بیمار ناموفق بود؛ چند دقیقه بعد دوباره تلاش کنید"})
		return
	}
	if h.Status != nil {
		h.Status.NotifyOTPRequested(clinicID, clinicCode, clinicName, firstName, lastName, nationalID, mobile, c.ClientIP())
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":              true,
		"message":         "کد تأیید از طریق پیامک یا پیام‌رسان بله ارسال شد؛ لطفاً بله یا پیامک‌هایتان را بررسی کنید",
		"sends_remaining": result.SendsRemaining,
	})
}

// VerifyOTP کد OTP را بررسی و بلیط نشست رزرو برمی‌گرداند.
// ورودی: JSON با csrf_token، موبایل، code. خروجی: JSON {ok, otp_ticket?, ...}.
func (h *BookingHandler) VerifyOTP(c *gin.Context) {
	if _, ok := tenant.FromGin(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "دسترسی نامعتبر"})
		return
	}
	var body otpVerifyBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "درخواست نامعتبر است"})
		return
	}
	if h.CSRF == nil || !h.CSRF.Verify(c.Request, body.CSRFToken) {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "توکن امنیتی نامعتبر است؛ صفحه را تازه کنید"})
		return
	}
	if h.OTP == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "message": "سرویس تایید موبایل در دسترس نیست"})
		return
	}

	mobile := digitsOnlyPublic(body.Mobile)
	code := digitsOnlyPublic(body.Code)
	if !booking.MobileLooksValid(mobile) {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "موبایل نامعتبر است"})
		return
	}
	if len(code) < 4 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "کد تایید را وارد کنید"})
		return
	}

	verifyResult, err := h.OTP.Verify(mobile, code)
	if err != nil {
		msg := "کد تایید نادرست است"
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, booking.ErrOTPNotFound), errors.Is(err, booking.ErrOTPExpired):
			msg = "کد تایید منقضی شده؛ دوباره درخواست کنید"
		case errors.Is(err, booking.ErrOTPTooManyAttempts):
			msg = "تعداد تلاش‌ها بیش از حد مجاز است؛ دوباره کد بگیرید"
			status = http.StatusTooManyRequests
		case errors.Is(err, booking.ErrOTPInvalid):
			msg = "کد تایید نادرست است"
		}
		c.JSON(status, gin.H{"ok": false, "message": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":                 true,
		"message":            "شماره موبایل تایید شد",
		"otp_ticket":         verifyResult.Ticket,
		"bookings_remaining": verifyResult.BookingsRemaining,
		"expires_at":         verifyResult.ExpiresAt.Unix(),
	})
}

// persistOTPPatient بیمار را با همان هویت فرم OTP ذخیره می‌کند تا بدون نوبت هم قابل پیگیری باشد.
// ورودی: کد ملی، نام، نام خانوادگی، موبایل. خروجی: شناسه بیمار یا خطا.
func (h *BookingHandler) persistOTPPatient(nationalID, firstName, lastName, mobile string) (uint, error) {
	if h.Bookings == nil || h.Bookings.Appointments == nil {
		return 0, errors.New("appointment repo missing")
	}
	saved, err := h.Bookings.Appointments.UpsertPatientIdentity(nationalID, firstName, lastName, mobile)
	if err != nil || saved == nil {
		if err == nil {
			err = errors.New("patient not saved")
		}
		return 0, err
	}
	return saved.ID, nil
}

// resolveOTPClinic شناسه، کد HIS و نام مرکز را برای tenant فعلی resolve می‌کند.
func (h *BookingHandler) resolveOTPClinic(tc *tenant.Context, clinicPath string) (clinicID uint, clinicCode int, clinicName string, err error) {
	clinicPath = strings.TrimSpace(clinicPath)
	switch {
	case tc.Layout == constants.LayoutPrivate && tc.ClinicID != nil:
		clinic, e := h.Clinics.GetByID(*tc.ClinicID)
		if e != nil || clinic == nil {
			return 0, 0, "", e
		}
		return clinic.ID, clinic.Code, clinic.Name, nil
	case tc.Layout == constants.LayoutOrgan && clinicPath != "":
		clinic, e := h.resolveOrganClinic(tc, clinicPath)
		if e != nil || clinic == nil {
			return 0, 0, "", e
		}
		return clinic.ID, clinic.Code, clinic.Name, nil
	case tc.Layout == constants.LayoutPlatform && clinicPath != "":
		clinic, e := h.Clinics.GetBySlug(clinicPath)
		if e != nil || clinic == nil {
			return 0, 0, "", e
		}
		return clinic.ID, clinic.Code, clinic.Name, nil
	default:
		if tc.ClinicID != nil {
			clinic, e := h.Clinics.GetByID(*tc.ClinicID)
			if e != nil || clinic == nil {
				return 0, 0, "", e
			}
			return clinic.ID, clinic.Code, clinic.Name, nil
		}
		return 0, 0, "", errors.New("clinic unresolved")
	}
}

// digitsOnlyPublic ارقام فارسی/عربی را به ASCII تبدیل می‌کند.
func digitsOnlyPublic(s string) string {
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
