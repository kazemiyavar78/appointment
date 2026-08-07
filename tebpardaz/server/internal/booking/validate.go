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

// ValidatePatientForm validates and normalizes public booking form fields.
// Inputs: raw form strings (first/last name, national id, mobile, birth_date YYYY-MM-DD, sex).
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

	bd, err := time.ParseInLocation("2006-01-02", birthDate, time.Local)
	if err != nil {
		return nil, fmt.Errorf("%w: تاریخ تولد نامعتبر است", ErrValidation)
	}
	if bd.After(time.Now()) {
		return nil, fmt.Errorf("%w: تاریخ تولد نمی‌تواند در آینده باشد", ErrValidation)
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
	sexMale   = "مرد"
	sexFemale = "زن"
)

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
