package booking

import (
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// createAndStoreDBLocked هش کد را در booking_otps می‌نویسد. قفل OTPStore باید گرفته شده باشد.
// ورودی: هویت بیمار و زمان فعلی. خروجی: کد خام برای پیامک، مدت انتظار، یا خطا.
func (s *OTPStore) createAndStoreDBLocked(in OTPCreateInput, now time.Time) (SendResult, time.Duration, error) {
	row, found, err := s.loadOTPByMobile(in.Mobile)
	if err != nil {
		return SendResult{}, 0, err
	}
	entry, retry, err := prepareOTPSend(entryFromRow(row), now)
	if err != nil {
		return SendResult{}, retry, err
	}
	code, err := generateOTPCode(otpLength)
	if err != nil {
		return SendResult{}, 0, err
	}
	entry.CodeHash = hashOTP(code)
	entry.ExpiresAt = now.Add(otpCodeTTL)
	entry.Attempts = 0
	entry.LastSentAt = now
	entry.SendCount++

	row.Mobile = in.Mobile
	row.ClinicID = in.ClinicID
	row.PatientID = in.PatientID
	row.NationalID = strings.TrimSpace(in.NationalID)
	row.FirstName = strings.TrimSpace(in.FirstName)
	row.LastName = strings.TrimSpace(in.LastName)
	row.IPAddress = trimStoredIP(in.IPAddress)
	row.CodeHash = entry.CodeHash
	row.ExpiresAt = entry.ExpiresAt
	row.Attempts = entry.Attempts
	row.LastSentAt = entry.LastSentAt
	row.SendCount = entry.SendCount
	row.SendWindow = entry.SendWindow
	if !found {
		if err := s.db.Create(&row).Error; err != nil {
			return SendResult{}, 0, err
		}
		return SendResult{Code: code, SendsRemaining: sendsRemaining(entry.SendCount)}, 0, nil
	}
	if err := s.db.Save(&row).Error; err != nil {
		return SendResult{}, 0, err
	}
	return SendResult{Code: code, SendsRemaining: sendsRemaining(entry.SendCount)}, 0, nil
}

// verifyDBLocked کد ذخیره‌شده در دیتابیس را بررسی می‌کند. قفل OTPStore باید گرفته شده باشد.
// ورودی: موبایل، کد و زمان فعلی. خروجی: بلیط نشست یا خطای OTP. ردیف بیمار حذف نمی‌شود.
func (s *OTPStore) verifyDBLocked(mobile, code string, now time.Time) (VerifyResult, error) {
	row, found, err := s.loadOTPByMobile(mobile)
	if err != nil {
		return VerifyResult{}, err
	}
	if !found || row.CodeHash == "" {
		return VerifyResult{}, ErrOTPNotFound
	}
	if now.After(row.ExpiresAt) {
		return VerifyResult{}, ErrOTPExpired
	}
	if row.Attempts >= otpMaxVerifyAttempts {
		row.CodeHash = ""
		if err := s.db.Save(&row).Error; err != nil {
			return VerifyResult{}, err
		}
		return VerifyResult{}, ErrOTPTooManyAttempts
	}
	if subtleConstantTimeMismatch(row.CodeHash, hashOTP(code)) {
		row.Attempts++
		if row.Attempts >= otpMaxVerifyAttempts {
			row.CodeHash = ""
		}
		if err := s.db.Save(&row).Error; err != nil {
			return VerifyResult{}, err
		}
		if row.CodeHash == "" {
			return VerifyResult{}, ErrOTPTooManyAttempts
		}
		return VerifyResult{}, ErrOTPInvalid
	}

	verifiedAt := now
	row.CodeHash = ""
	row.Attempts = 0
	row.VerifiedAt = &verifiedAt
	if err := s.db.Save(&row).Error; err != nil {
		return VerifyResult{}, err
	}
	sessionID, err := randomToken(16)
	if err != nil {
		return VerifyResult{}, err
	}
	sess := otpSession{
		ID:           sessionID,
		Mobile:       mobile,
		ExpiresAt:    now.Add(otpSessionTTL),
		BookingsUsed: 0,
		CreatedAt:    now,
	}
	s.sessions[sess.ID] = sess
	s.mobileSession[mobile] = sess.ID
	return s.issueTicketLocked(sess)
}

// MarkDelivered ارسال موفق پیامک را ثبت می‌کند و وضعیت «نوبت ثبت شد» قبلی را برمی‌دارد.
// ورودی: موبایل و شناسه بیمار (صفر یعنی بیمار از قبل روی ردیف بماند). خروجی: خطای دیتابیس.
func (s *OTPStore) MarkDelivered(mobile string, patientID uint) error {
	if s == nil || s.db == nil {
		return nil
	}
	mobile = digitsOnly(mobile)
	now := time.Now()
	// booked_at و verified_at باید NULL شوند تا تلاش جدیدِ بدون نوبت دوباره در لیست پیگیری دیده شود.
	res := s.db.Exec(`
		UPDATE booking_otps
		SET sms_sent_at = ?,
		    patient_id = CASE WHEN ? > 0 THEN ? ELSE patient_id END,
		    booked_at = NULL,
		    verified_at = NULL,
		    updated_at = ?
		WHERE mobile = ? AND deleted_at IS NULL
	`, now, patientID, patientID, now, mobile)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// MarkBooked زمان ثبت نوبت را روی چالش OTP همان موبایل می‌نویسد.
// ورودی: موبایل بیمار. خروجی: خطای دیتابیس. اگر ردیفی نباشد خطا برنمی‌گرداند.
func (s *OTPStore) MarkBooked(mobile string) error {
	if s == nil || s.db == nil {
		return nil
	}
	mobile = digitsOnly(mobile)
	now := time.Now()
	return s.db.Model(&models.BookingOTP{}).Where("mobile = ?", mobile).Update("booked_at", now).Error
}

// loadOTPByMobile ردیف فعال OTP یک موبایل را می‌خواند.
// ورودی: موبایل نرمال‌شده. خروجی: ردیف، وجود داشتن، خطا.
func (s *OTPStore) loadOTPByMobile(mobile string) (models.BookingOTP, bool, error) {
	var row models.BookingOTP
	err := s.db.Where("mobile = ?", mobile).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.BookingOTP{}, false, nil
	}
	if err != nil {
		return models.BookingOTP{}, false, err
	}
	return row, true, nil
}

// pendingCodeDBLocked کد ذخیره‌شده و هنوز قابل ورود را برای همان موبایل و کد ملی برمی‌گرداند.
// ورودی: موبایل، کد ملی و زمان فعلی. قفل OTPStore باید گرفته شده باشد.
// خروجی: مدت انتظار ارسال مجدد، انقضا، و true وقتی کد قبلی را می‌توان وارد کرد.
func (s *OTPStore) pendingCodeDBLocked(mobile, nationalID string, now time.Time) (time.Duration, time.Time, bool) {
	row, found, err := s.loadOTPByMobile(mobile)
	if err != nil || !found {
		return 0, time.Time{}, false
	}
	entry := entryFromRow(row)
	if !codeStillEnterable(entry, now) || !sameOTPNationalID(entry.NationalID, nationalID) {
		return 0, time.Time{}, false
	}
	return resendWait(entry, now), entry.ExpiresAt, true
}

// entryFromRow فیلدهای محدودیت ارسال را از ردیف دیتابیس برمی‌دارد.
// ورودی: ردیف BookingOTP. خروجی: otpCodeEntry.
func entryFromRow(row models.BookingOTP) otpCodeEntry {
	return otpCodeEntry{
		CodeHash:   row.CodeHash,
		ExpiresAt:  row.ExpiresAt,
		Attempts:   row.Attempts,
		LastSentAt: row.LastSentAt,
		SendCount:  row.SendCount,
		SendWindow: row.SendWindow,
		NationalID: row.NationalID,
	}
}

// subtleConstantTimeMismatch مقایسه زمان-ثابت هش کد را برمی‌گرداند.
// ورودی: هش ذخیره‌شده و هش کد واردشده. خروجی: true وقتی برابر نیستند.
func subtleConstantTimeMismatch(stored, got string) bool {
	return subtle.ConstantTimeCompare([]byte(stored), []byte(got)) != 1
}

// trimStoredIP آی‌پی را به طول ستون booking_otps.ip_address محدود می‌کند.
// ورودی: آی‌پی خام. خروجی: رشته حداکثر ۴۵ نویسه‌ای.
func trimStoredIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if len(ip) > 45 {
		return ip[:45]
	}
	return ip
}
