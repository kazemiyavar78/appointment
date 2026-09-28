package auth

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

const (
	adminOTPLength   = 5
	adminOTPTTL      = 5 * time.Minute
	adminOTPMaxTries = 5
)

var (
	// ErrAdminOTPNotFound وقتی کد فعالی برای این کاربر نیست.
	ErrAdminOTPNotFound = errors.New("admin otp not found")
	// ErrAdminOTPExpired وقتی کد ورود منقضی شده است.
	ErrAdminOTPExpired = errors.New("admin otp expired")
	// ErrAdminOTPInvalid وقتی کد نادرست است.
	ErrAdminOTPInvalid = errors.New("admin otp invalid")
	// ErrAdminOTPTooManyAttempts وقتی تلاش‌ها از حد مجاز گذشته.
	ErrAdminOTPTooManyAttempts = errors.New("admin otp too many attempts")
)

type adminOTPEntry struct {
	CodeHash  string
	ExpiresAt time.Time
	Attempts  int
}

// AdminOTPStore نگهداری درحافظه کد ورود پنل ادمین.
type AdminOTPStore struct {
	mu      sync.Mutex
	pending map[uint]adminOTPEntry
}

// NewAdminOTPStore یک مخزن خالی OTP ادمین می‌سازد.
// Input: ندارد.
// Output: اشاره‌گر AdminOTPStore.
func NewAdminOTPStore() *AdminOTPStore {
	return &AdminOTPStore{pending: make(map[uint]adminOTPEntry)}
}

// Create کد ۵ رقمی برای کاربر می‌سازد و ذخیره می‌کند.
// Input: userID — شناسه کاربر ادمین.
// Output: کد خام یا خطا.
func (s *AdminOTPStore) Create(userID uint) (string, error) {
	if s == nil || userID == 0 {
		return "", fmt.Errorf("otp store unavailable")
	}
	code, err := generateAdminOTPCode(adminOTPLength)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	s.pending[userID] = adminOTPEntry{
		CodeHash:  hashAdminOTP(code),
		ExpiresAt: time.Now().Add(adminOTPTTL),
	}
	return code, nil
}

// Verify کد ورود را با رکورد ذخیره‌شده می‌سنجد.
// Input: userID و code.
// Output: خطا در صورت نامعتبر بودن.
func (s *AdminOTPStore) Verify(userID uint, code string) error {
	if s == nil || userID == 0 {
		return ErrAdminOTPNotFound
	}
	code = digitsOnlyAdmin(code)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)

	entry, ok := s.pending[userID]
	if !ok {
		return ErrAdminOTPNotFound
	}
	if now.After(entry.ExpiresAt) {
		delete(s.pending, userID)
		return ErrAdminOTPExpired
	}
	if entry.Attempts >= adminOTPMaxTries {
		delete(s.pending, userID)
		return ErrAdminOTPTooManyAttempts
	}
	if subtle.ConstantTimeCompare([]byte(entry.CodeHash), []byte(hashAdminOTP(code))) != 1 {
		entry.Attempts++
		if entry.Attempts >= adminOTPMaxTries {
			delete(s.pending, userID)
			return ErrAdminOTPTooManyAttempts
		}
		s.pending[userID] = entry
		return ErrAdminOTPInvalid
	}
	delete(s.pending, userID)
	return nil
}

func (s *AdminOTPStore) cleanupLocked(now time.Time) {
	for id, entry := range s.pending {
		if now.After(entry.ExpiresAt) {
			delete(s.pending, id)
		}
	}
}

func generateAdminOTPCode(length int) (string, error) {
	if length <= 0 {
		length = adminOTPLength
	}
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", length, n.Int64()), nil
}

func hashAdminOTP(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func digitsOnlyAdmin(s string) string {
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
