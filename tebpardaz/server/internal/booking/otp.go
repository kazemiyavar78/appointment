package booking

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

var (
	// ErrOTPNotFound وقتی برای این موبایل کد فعالی وجود ندارد.
	ErrOTPNotFound = errors.New("otp not found")
	// ErrOTPExpired وقتی کد یا نشست منقضی شده است.
	ErrOTPExpired = errors.New("otp expired")
	// ErrOTPInvalid وقتی کد واردشده نادرست است.
	ErrOTPInvalid = errors.New("otp invalid")
	// ErrOTPTooManyAttempts وقتی تلاش‌های اشتباه از حد مجاز گذشته.
	ErrOTPTooManyAttempts = errors.New("otp too many attempts")
	// ErrOTPRateLimited وقتی ارسال مجدد زودتر از موعد مجاز است.
	ErrOTPRateLimited = errors.New("otp rate limited")
	// ErrOTPNotVerified وقتی رزرو بدون نشست معتبر انجام شود.
	ErrOTPNotVerified = errors.New("otp not verified")
	// ErrOTPSessionExhausted وقتی سقف ۵ نوبت با یک نشست پر شده.
	ErrOTPSessionExhausted = errors.New("otp session exhausted")
)

// otpCodeEntry نگهداری کد پیامکی فعال برای یک شماره موبایل.
type otpCodeEntry struct {
	CodeHash   string
	ExpiresAt  time.Time
	Attempts   int
	LastSentAt time.Time
	SendCount  int
	SendWindow time.Time
}

// otpSession نشست تاییدشده پس از وارد کردن صحیح کد.
type otpSession struct {
	ID           string
	Mobile       string
	ExpiresAt    time.Time
	BookingsUsed int
	CreatedAt    time.Time
}

// OTPStore نگهداری درحافظه کدهای OTP و نشست‌های رزرو.
type OTPStore struct {
	mu            sync.Mutex
	codes         map[string]otpCodeEntry
	sessions      map[string]otpSession
	tickets       map[string]string
	mobileSession map[string]string
}

// NewOTPStore یک مخزن OTP خالی می‌سازد.
// ورودی: ندارد. خروجی: اشاره‌گر OTPStore.
func NewOTPStore() *OTPStore {
	return &OTPStore{
		codes:         make(map[string]otpCodeEntry),
		sessions:      make(map[string]otpSession),
		tickets:       make(map[string]string),
		mobileSession: make(map[string]string),
	}
}

// SendResult خروجی متد CreateAndStore شامل اطلاعات محدودیت ارسال.
type SendResult struct {
	Code           string
	SendsRemaining int
}

// CreateAndStore کد OTP تولید و برای موبایل ذخیره می‌کند.
// ورودی: موبایل و فیلدهای بیمار (برای پیامک). خروجی: نتیجه، زمان انتظار، خطا.
func (s *OTPStore) CreateAndStore(mobile, firstName, lastName, nationalID string) (SendResult, time.Duration, error) {
	_ = firstName
	_ = lastName
	_ = nationalID

	if s == nil {
		return SendResult{}, 0, fmt.Errorf("otp store nil")
	}
	mobile = digitsOnly(mobile)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)

	entry := s.codes[mobile]
	if !entry.LastSentAt.IsZero() {
		elapsed := now.Sub(entry.LastSentAt)
		if elapsed < otpResendCooldown {
			return SendResult{}, otpResendCooldown - elapsed, ErrOTPRateLimited
		}
	}
	if entry.SendWindow.IsZero() || now.Sub(entry.SendWindow) >= otpSendWindow {
		entry.SendWindow = now
		entry.SendCount = 0
	}
	if entry.SendCount >= otpMaxSendsPerWindow {
		retry := otpSendWindow - now.Sub(entry.SendWindow)
		if retry < time.Second {
			retry = time.Second
		}
		return SendResult{}, retry, ErrOTPRateLimited
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
	s.codes[mobile] = entry

	remaining := otpMaxSendsPerWindow - entry.SendCount
	if remaining < 0 {
		remaining = 0
	}
	return SendResult{Code: code, SendsRemaining: remaining}, 0, nil
}

// VerifyResult خروجی تایید OTP شامل بلیط نشست.
type VerifyResult struct {
	Ticket            string
	BookingsRemaining int
	ExpiresAt         time.Time
}

// Verify کد OTP را بررسی و نشست رزرو صادر می‌کند.
// ورودی: موبایل و کد. خروجی: بلیط نشست یا خطا.
func (s *OTPStore) Verify(mobile, code string) (VerifyResult, error) {
	if s == nil {
		return VerifyResult{}, fmt.Errorf("otp store nil")
	}
	mobile = digitsOnly(mobile)
	code = digitsOnly(code)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)

	if sess, ok := s.activeSessionLocked(mobile, now); ok {
		return s.issueTicketLocked(sess)
	}

	entry, ok := s.codes[mobile]
	if !ok {
		return VerifyResult{}, ErrOTPNotFound
	}
	if now.After(entry.ExpiresAt) {
		delete(s.codes, mobile)
		return VerifyResult{}, ErrOTPExpired
	}
	if entry.Attempts >= otpMaxVerifyAttempts {
		delete(s.codes, mobile)
		return VerifyResult{}, ErrOTPTooManyAttempts
	}
	if subtle.ConstantTimeCompare([]byte(entry.CodeHash), []byte(hashOTP(code))) != 1 {
		entry.Attempts++
		s.codes[mobile] = entry
		if entry.Attempts >= otpMaxVerifyAttempts {
			delete(s.codes, mobile)
			return VerifyResult{}, ErrOTPTooManyAttempts
		}
		return VerifyResult{}, ErrOTPInvalid
	}

	delete(s.codes, mobile)
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

// ConsumeTicket یک نوبت از سهمیه نشست مصرف می‌کند.
// ورودی: بلیط، موبایل. خروجی: خطا در صورت نامعتبر بودن.
func (s *OTPStore) ConsumeTicket(ticket, mobile, nationalID string) error {
	_ = nationalID
	if s == nil {
		return ErrOTPNotVerified
	}
	ticket = strings.TrimSpace(ticket)
	mobile = digitsOnly(mobile)
	if ticket == "" || mobile == "" {
		return ErrOTPNotVerified
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)

	sessionID, ok := s.tickets[ticket]
	if !ok {
		return ErrOTPNotVerified
	}
	sess, ok := s.sessions[sessionID]
	if !ok {
		return ErrOTPNotVerified
	}
	if now.After(sess.ExpiresAt) {
		s.dropSessionLocked(sessionID, mobile)
		return ErrOTPExpired
	}
	if sess.Mobile != mobile {
		return ErrOTPNotVerified
	}
	if sess.BookingsUsed >= otpMaxBookingsPerSession {
		return ErrOTPSessionExhausted
	}
	sess.BookingsUsed++
	s.sessions[sessionID] = sess
	return nil
}

// ActiveSessionForMobile وضعیت نشست فعال موبایل را برمی‌گرداند.
// ورودی: موبایل. خروجی: باقیمانده نوبت، انقضا، وجود داشتن.
func (s *OTPStore) ActiveSessionForMobile(mobile string) (bookingsRemaining int, expiresAt time.Time, ok bool) {
	if s == nil {
		return 0, time.Time{}, false
	}
	mobile = digitsOnly(mobile)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	sess, found := s.activeSessionLocked(mobile, now)
	if !found {
		return 0, time.Time{}, false
	}
	return otpMaxBookingsPerSession - sess.BookingsUsed, sess.ExpiresAt, true
}

// IssueSessionTicket برای نشست فعال موبایل یک بلیط جدید صادر می‌کند.
// ورودی: موبایل. خروجی: VerifyResult یا خطا.
func (s *OTPStore) IssueSessionTicket(mobile string) (VerifyResult, error) {
	if s == nil {
		return VerifyResult{}, fmt.Errorf("otp store nil")
	}
	mobile = digitsOnly(mobile)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)

	sess, ok := s.activeSessionLocked(mobile, now)
	if !ok {
		return VerifyResult{}, ErrOTPNotVerified
	}
	return s.issueTicketLocked(sess)
}

// issueTicketLocked بلیط جدید برای نشست موجود صادر می‌کند (قفل باید گرفته شده باشد).
func (s *OTPStore) issueTicketLocked(sess otpSession) (VerifyResult, error) {
	ticket, err := randomToken(32)
	if err != nil {
		return VerifyResult{}, err
	}
	s.tickets[ticket] = sess.ID
	remaining := otpMaxBookingsPerSession - sess.BookingsUsed
	return VerifyResult{
		Ticket:            ticket,
		BookingsRemaining: remaining,
		ExpiresAt:         sess.ExpiresAt,
	}, nil
}

// activeSessionLocked نشست فعال موبایل را برمی‌گرداند.
func (s *OTPStore) activeSessionLocked(mobile string, now time.Time) (otpSession, bool) {
	sessionID, ok := s.mobileSession[mobile]
	if !ok {
		return otpSession{}, false
	}
	sess, ok := s.sessions[sessionID]
	if !ok {
		delete(s.mobileSession, mobile)
		return otpSession{}, false
	}
	if now.After(sess.ExpiresAt) || sess.BookingsUsed >= otpMaxBookingsPerSession {
		s.dropSessionLocked(sessionID, mobile)
		return otpSession{}, false
	}
	return sess, true
}

// dropSessionLocked نشست و بلیط‌های مرتبط را حذف می‌کند.
func (s *OTPStore) dropSessionLocked(sessionID, mobile string) {
	delete(s.sessions, sessionID)
	delete(s.mobileSession, mobile)
	for ticket, sid := range s.tickets {
		if sid == sessionID {
			delete(s.tickets, ticket)
		}
	}
}

// cleanupLocked ورودی‌های منقضی را از حافظه حذف می‌کند.
func (s *OTPStore) cleanupLocked(now time.Time) {
	for k, e := range s.codes {
		if now.After(e.ExpiresAt.Add(time.Hour)) {
			delete(s.codes, k)
		}
	}
	for id, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			s.dropSessionLocked(id, sess.Mobile)
		}
	}
}

// generateOTPCode یک کد عددی با طول مشخص تولید می‌کند.
func generateOTPCode(length int) (string, error) {
	if length <= 0 {
		length = otpLength
	}
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", length, n.Int64()), nil
}

// hashOTP هش SHA-256 کد را برای ذخیره امن برمی‌گرداند.
func hashOTP(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// randomToken توکن تصادفی hex با n بایت تولید می‌کند.
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
