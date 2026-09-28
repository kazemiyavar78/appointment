package slug

import (
	"errors"
	"strings"
	"unicode"

	"tebpardaz/server/internal/text"
)

// ErrEmpty یعنی بعد از پاکسازی، segment قابل استفاده نمانده است.
var ErrEmpty = errors.New("empty slug")

// Make builds a URL-safe path segment from a display name.
// Inputs: raw name (Persian or Latin).
// Output: hyphenated slug; "doctor" when empty after cleanup.
func Make(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "doctor"
	}
	var b strings.Builder
	lastHyphen := false
	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastHyphen = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '/':
			if !lastHyphen && b.Len() > 0 {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "doctor"
	}
	return out
}

// MakePersian یک segment برای URLهای SEO آینده می‌سازد.
// ورودی: نام انسانی. خروجی: slug بدون / و بدون خط‌تیرهٔ تکراری، یا ErrEmpty.
// Doctor.Slug ذخیره‌شده از این تابع ساخته نمی‌شود.
func MakePersian(raw string) (string, error) {
	raw = text.NormalizePersianText(raw)
	var b strings.Builder
	lastHyphen := false
	for _, r := range raw {
		if isPersianSlugSeparator(r) {
			if !lastHyphen && b.Len() > 0 {
				b.WriteByte('-')
				lastHyphen = true
			}
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen && b.Len() > 0 {
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "", ErrEmpty
	}
	return out, nil
}

// isPersianSlugSeparator فاصله، نیم‌فاصله و نویسه‌های شکاف مسیر را جداکننده می‌داند.
// ورودی: یک نویسه. خروجی: true اگر باید به یک خط‌تیره تبدیل شود.
func isPersianSlugSeparator(r rune) bool {
	return unicode.IsSpace(r) || r == '\u200c' || r == '-' || r == '_' || r == '/' || r == '\\'
}
