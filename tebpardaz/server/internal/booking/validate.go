package booking

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	ptime "github.com/yaa110/go-persian-calendar"
)

var (
	// ErrValidation is returned when patient form fields fail server-side checks.
	ErrValidation = errors.New("validation failed")

	persianNameRe = regexp.MustCompile(`^[\x{0600}-\x{06FF}\x{FB50}-\x{FDFF}\x{FE70}-\x{FEFF}\s\x{200c}]+$`)
	mobileRe      = regexp.MustCompile(`^09\d{9}$`)
)

// PatientForm is the validated booking payload from the public form.
type PatientForm struct {
	SlotID     string
	FirstName  string
	LastName   string
	NationalID string
	Mobile     string
	BirthDate  time.Time // Gregorian instant (UTC noon of calendar day)
	BirthJalali string   // yyyy/MM/dd for HIS / protocol
	Sex        string    // مرد | زن
	SexCode    int       // HIS pjens: 1=مرد, 2=زن
}

// ValidatePatientIdentity فیلدهای هویتی را قبل از ارسال OTP بررسی می‌کند.
// ورودی: نام، نام‌خانوادگی، کدملی، موبایل. خروجی: خطای فارسی یا nil.
func ValidatePatientIdentity(firstName, lastName, nationalID, mobile string) error {
	firstName = normalizePersianName(firstName)
	lastName = normalizePersianName(lastName)
	nationalID = digitsOnly(nationalID)
	mobile = digitsOnly(mobile)
	if firstName == "" || !persianNameRe.MatchString(firstName) {
		return fmt.Errorf("%w: نام باید فارسی باشد", ErrValidation)
	}
	if lastName == "" || !persianNameRe.MatchString(lastName) {
		return fmt.Errorf("%w: نام خانوادگی باید فارسی باشد", ErrValidation)
	}
	if !IsValidIranianNationalID(nationalID) {
		return fmt.Errorf("%w: کد ملی معتبر نیست", ErrValidation)
	}
	if !mobileRe.MatchString(mobile) {
		return fmt.Errorf("%w: موبایل باید ۱۱ رقم و با ۰۹ شروع شود", ErrValidation)
	}
	return nil
}

// ValidatePatientForm validates and normalizes public booking form fields.
// Inputs: raw form strings (first/last name, national id, mobile, birth_date Shamsi or Gregorian, sex).
// Output: normalized PatientForm or a Persian error message wrapping ErrValidation.
func ValidatePatientForm(firstName, lastName, nationalID, mobile, birthDate, sex string) (*PatientForm, error) {
	firstName = normalizePersianName(firstName)
	lastName = normalizePersianName(lastName)
	nationalID = digitsOnly(nationalID)
	mobile = digitsOnly(mobile)
	birthDate = strings.TrimSpace(birthDate)
	sex = strings.TrimSpace(sex)

	if firstName == "" || !persianNameRe.MatchString(firstName) {
		return nil, fmt.Errorf("%w: نام باید فارسی باشد", ErrValidation)
	}
	if lastName == "" || !persianNameRe.MatchString(lastName) {
		return nil, fmt.Errorf("%w: نام خانوادگی باید فارسی باشد", ErrValidation)
	}
	if !IsValidIranianNationalID(nationalID) {
		return nil, fmt.Errorf("%w: کد ملی معتبر نیست", ErrValidation)
	}
	if !mobileRe.MatchString(mobile) {
		return nil, fmt.Errorf("%w: موبایل باید ۱۱ رقم و با ۰۹ شروع شود", ErrValidation)
	}
	if sex != string(sexMale) && sex != string(sexFemale) {
		return nil, fmt.Errorf("%w: جنسیت باید مرد یا زن باشد", ErrValidation)
	}

	bd, err := parseBirthDate(birthDate)
	if err != nil {
		return nil, fmt.Errorf("%w: تاریخ تولد نامعتبر است", ErrValidation)
	}
	minAgeDay := time.Now().AddDate(-minPatientAgeYears, 0, 0)
	if bd.After(minAgeDay) {
		return nil, fmt.Errorf("%w: حداقل سن بیمار باید ۱ سال باشد", ErrValidation)
	}
	maxAgeDay := time.Now().AddDate(-maxPatientAgeYears, 0, 0)
	if bd.Before(maxAgeDay) {
		return nil, fmt.Errorf("%w: تاریخ تولد نامعتبر است", ErrValidation)
	}
	jalali := ptime.New(bd).Format("yyyy/MM/dd")

	sexCode := 1
	if sex == string(sexFemale) {
		sexCode = 2
	}

	return &PatientForm{
		FirstName:   firstName,
		LastName:    lastName,
		NationalID:  nationalID,
		Mobile:      mobile,
		BirthDate:   bd,
		BirthJalali: jalali,
		Sex:         sex,
		SexCode:     sexCode,
	}, nil
}

const (
	sexMale            = "مرد"
	sexFemale          = "زن"
	minPatientAgeYears = 1
	maxPatientAgeYears = 120
)

// parseBirthDate converts a Shamsi YYYY/MM/DD (from persian-datepicker) or Gregorian date to time.Time.
// Input: raw birth date string. Output: local midnight Time or error.
func parseBirthDate(raw string) (time.Time, error) {
	raw = normalizeBirthDigits(strings.TrimSpace(raw))
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	rawSlash := strings.ReplaceAll(raw, "-", "/")
	parts := strings.Split(rawSlash, "/")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("format")
	}
	y, errY := strconv.Atoi(parts[0])
	m, errM := strconv.Atoi(parts[1])
	d, errD := strconv.Atoi(parts[2])
	if errY != nil || errM != nil || errD != nil {
		return time.Time{}, fmt.Errorf("digits")
	}
	// Shamsi from persian-datepicker: convert to Gregorian via go-persian-calendar.
	if y >= 1200 && y <= 1600 {
		pt := ptime.Date(y, ptime.Month(m), d, 0, 0, 0, 0, time.Local)
		t := pt.Time() // Gregorian time.Time
		if t.IsZero() {
			return time.Time{}, fmt.Errorf("jalali")
		}
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local), nil
	}
	t, err := time.ParseInLocation("2006/01/02", rawSlash, time.Local)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

// normalizeBirthDigits maps Persian/Arabic-Indic digits to ASCII.
func normalizeBirthDigits(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= '۰' && r <= '۹':
			b.WriteRune('0' + (r - '۰'))
		case r >= '٠' && r <= '٩':
			b.WriteRune('0' + (r - '٠'))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// IsValidIranianNationalID validates a 10-digit Iranian national ID checksum.
// Inputs: nationalID digits (may include separators; stripped internally when called after digitsOnly).
// Output: true when length and control digit are valid.
func IsValidIranianNationalID(nationalID string) bool {
	nationalID = digitsOnly(nationalID)
	if len(nationalID) != 10 {
		return false
	}
	if allSameDigits(nationalID) {
		return false
	}
	sum := 0
	for i := 0; i < 9; i++ {
		d, err := strconv.Atoi(string(nationalID[i]))
		if err != nil {
			return false
		}
		sum += d * (10 - i)
	}
	r := sum % 11
	check, err := strconv.Atoi(string(nationalID[9]))
	if err != nil {
		return false
	}
	if r < 2 {
		return check == r
	}
	return check == 11-r
}

func normalizePersianName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "ي", "ی")
	s = strings.ReplaceAll(s, "ك", "ک")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsDigit(r) {
			// Persian/Arabic digits → ASCII
			switch {
			case r >= '۰' && r <= '۹':
				b.WriteByte(byte('0' + (r - '۰')))
			case r >= '٠' && r <= '٩':
				b.WriteByte(byte('0' + (r - '٠')))
			default:
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func allSameDigits(s string) bool {
	if s == "" {
		return true
	}
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}

// MobileLooksValid reports whether mobile matches the public booking mobile format.
// Inputs: digits-only or raw mobile string.
// Output: true when it matches ^09\d{9}$.
func MobileLooksValid(mobile string) bool {
	return mobileRe.MatchString(digitsOnly(mobile))
}
