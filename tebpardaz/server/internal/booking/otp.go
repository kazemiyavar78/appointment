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

	"gorm.io/gorm"
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
	NationalID string
}

// otpSession نشست تاییدشده پس از وارد کردن صحیح کد.
type otpSession struct {
	ID           string
	Mobile       string
	ExpiresAt    time.Time
	BookingsUsed int
	CreatedAt    time.Time
}

// OTPStore نگهداری کدهای OTP و نشست‌های رزرو.
// کد پیامکی در دیتابیس ذخیره می‌شود؛ نشست تاییدشده تا ری‌استارت سرور در حافظه می‌ماند.
type OTPStore struct {
	mu            sync.Mutex
	db            *gorm.DB
	codes         map[string]otpCodeEntry
	sessions      map[string]otpSession
	tickets       map[string]string
	mobileSession map[string]string
}

// NewOTPStore یک مخزن OTP می‌سازد.
// ورودی: اتصال appointment (nil یعنی فقط حافظه، برای تست). خروجی: اشاره‌گر OTPStore.
func NewOTPStore(db *gorm.DB) *OTPStore {
	return &OTPStore{
		db:            db,
		codes:         make(map[string]otpCodeEntry),
		sessions:      make(map[string]otpSession),
		tickets:       make(map[string]string),
		mobileSession: make(map[string]string),
	}
}

// OTPCreateInput هویت بیمار و مرکز برای صدور کد تایید.
type OTPCreateInput struct {
	Mobile     string
	FirstName  string
	LastName   string
	NationalID string
	ClinicID   uint
	PatientID  uint
	IPAddress  string
}

// SendResult خروجی متد CreateAndStore شامل اطلاعات محدودیت ارسال.
type SendResult struct {
	Code           string
	SendsRemaining int
}

// CreateAndStore کد OTP تولید و هش آن را برای موبایل ذخیره می‌کند.
// ورودی: هویت بیمار، مرکز و آی‌پی. خروجی: کد خام (فقط برای ارسال)، زمان انتظار، خطا.
func (s *OTPStore) CreateAndStore(in OTPCreateInput) (SendResult, time.Duration, error) {
	if s == nil {
		return SendResult{}, 0, fmt.Errorf("otp store nil")
	}
	in.Mobile = digitsOnly(in.Mobile)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	if s.db != nil {
		return s.createAndStoreDBLocked(in, now)
	}
	return s.createAndStoreMemoryLocked(in, now)
}

// createAndStoreMemoryLocked کد را فقط در حافظه نگه می‌دارد. قفل باید گرفته شده باشد.
// ورودی: هویت بیمار و زمان فعلی. خروجی: نتیجه ارسال یا خطای محدودیت.
func (s *OTPStore) createAndStoreMemoryLocked(in OTPCreateInput, now time.Time) (SendResult, time.Duration, error) {
	entry := s.codes[in.Mobile]
	entry, retry, err := prepareOTPSend(entry, now)
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
	entry.NationalID = digitsOnly(in.NationalID)
	s.codes[in.Mobile] = entry
	return SendResult{Code: code, SendsRemaining: sendsRemaining(entry.SendCount)}, 0, nil
}

// PendingCode می‌گوید آیا برای همین موبایل و کد ملی هنوز می‌توان کد قبلی را وارد کرد.
// ورودی: موبایل و کد ملی. خروجی: مدت انتظار تا ارسال مجدد، زمان انقضای کد، و وجود کد قابل ورود.
func (s *OTPStore) PendingCode(mobile, nationalID string) (resendAfter time.Duration, expiresAt time.Time, ok bool) {
	if s == nil {
		return 0, time.Time{}, false
	}
	mobile = digitsOnly(mobile)
	nationalID = digitsOnly(nationalID)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	if s.db != nil {
		return s.pendingCodeDBLocked(mobile, nationalID, now)
	}
	entry, found := s.codes[mobile]
	if !found || !codeStillEnterable(entry, now) || !sameOTPNationalID(entry.NationalID, nationalID) {
		return 0, time.Time{}, false
	}
	return resendWait(entry, now), entry.ExpiresAt, true
}

// codeStillEnterable یعنی هش کد باقی است، منقضی نشده و تلاش اشتباه به سقف نرسیده.
// ورودی: رکورد کد و زمان فعلی. خروجی: true وقتی بیمار هنوز می‌تواند همان کد را وارد کند.
func codeStillEnterable(entry otpCodeEntry, now time.Time) bool {
	return entry.CodeHash != "" && !now.After(entry.ExpiresAt) && entry.Attempts < otpMaxVerifyAttempts
}

// sameOTPNationalID کد ملی ذخیره‌شده را با درخواست فعلی مقایسه می‌کند.
// ورودی: کد ملی رکورد و کد ملی درخواست. خروجی: true وقتی خالی است یا هر دو یکی هستند.
func sameOTPNationalID(stored, got string) bool {
	stored = digitsOnly(stored)
	got = digitsOnly(got)
	return stored == "" || stored == got
}

// resendWait فاصله باقی‌مانده تا ارسال مجدد مجاز را برمی‌گرداند.
// ورودی: رکورد ارسال و زمان فعلی. خروجی: صفر یعنی ارسال مجدد همین حالا مجاز است.
func resendWait(entry otpCodeEntry, now time.Time) time.Duration {
	var cooldown time.Duration
	if !entry.LastSentAt.IsZero() {
		elapsed := now.Sub(entry.LastSentAt)
		if elapsed < otpResendCooldown {
			cooldown = otpResendCooldown - elapsed
		}
	}
	if entry.SendWindow.IsZero() || now.Sub(entry.SendWindow) >= otpSendWindow || entry.SendCount < otpMaxSendsPerWindow {
		return cooldown
	}
	windowLeft := otpSendWindow - now.Sub(entry.SendWindow)
	if windowLeft < time.Second {
		windowLeft = time.Second
	}
	if windowLeft > cooldown {
		return windowLeft
	}
	return cooldown
}

// prepareOTPSend محدودیت فاصله و سقف ارسال را روی رکورد فعلی اعمال می‌کند.
// ورودی: رکورد قبلی و زمان فعلی. خروجی: رکورد آماده‌شده، مدت انتظار، یا ErrOTPRateLimited.
func prepareOTPSend(entry otpCodeEntry, now time.Time) (otpCodeEntry, time.Duration, error) {
	if !entry.LastSentAt.IsZero() {
		elapsed := now.Sub(entry.LastSentAt)
		if elapsed < otpResendCooldown {
			return entry, otpResendCooldown - elapsed, ErrOTPRateLimited
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
		return entry, retry, ErrOTPRateLimited
	}
	return entry, 0, nil
}

// sendsRemaining تعداد ارسال باقی‌مانده در پنجره فعلی را برمی‌گرداند.
// ورودی: تعداد ارسال انجام‌شده. خروجی: عدد صفر یا بیشتر.
func sendsRemaining(sendCount int) int {
	remaining := otpMaxSendsPerWindow - sendCount
	if remaining < 0 {
		return 0
	}
	return remaining
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
	if s.db != nil {
		return s.verifyDBLocked(mobile, code, now)
	}
	return s.verifyMemoryLocked(mobile, code, now)
}

// verifyMemoryLocked کد حافظه‌ای را بررسی و نشست صادر می‌کند. قفل باید گرفته شده باشد.
// ورودی: موبایل، کد و زمان فعلی. خروجی: بلیط نشست یا خطای OTP.
func (s *OTPStore) verifyMemoryLocked(mobile, code string, now time.Time) (VerifyResult, error) {
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
