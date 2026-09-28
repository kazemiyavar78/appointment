package text

import (
	"strconv"
	"strings"
)

// persianLetterMap فقط نگاشت امن ی/ک است. ئ و ة اصلاح املایی نیستند.
var persianLetterMap = map[rune]rune{
	'\u064A': '\u06CC', // ي → ی
	'\u0649': '\u06CC', // ى → ی
	'\u0643': '\u06A9', // ك → ک
}

// legacyLetterMap همان نگاشت قدیمی ذخیرهٔ پزشک است و برای دادهٔ جدید استفاده نمی‌شود.
var legacyLetterMap = map[rune]rune{
	'\u064A': '\u06CC',
	'\u0649': '\u06CC',
	'\u0626': '\u06CC', // ئ → ی
	'\u0643': '\u06A9',
	'\u0629': '\u0647', // ة → ه
}

// NormalizePersianText حروف و فاصلهٔ متن انسانی را محافظه‌کارانه یکدست می‌کند.
// ورودی: نام یا عبارت. خروجی: متن trim‌شده. رقم، نیم‌فاصله، ئ و ة عوض نمی‌شوند.
func NormalizePersianText(s string) string {
	return collapseSpaces(applyLetterMap(s, persianLetterMap))
}

// NormalizeArabicToPersian رفتار قدیمی حروف، شامل ئ و ة، به‌علاوه رقم فارسی را حفظ می‌کند.
// ورودی: رشتهٔ legacy. خروجی: متن قدیمی. caller جدید نباید از این تابع استفاده کند.
func NormalizeArabicToPersian(s string) string {
	return ToPersianDigits(collapseSpaces(applyLetterMap(s, legacyLetterMap)))
}

// SearchLegacyForms شکل محافظه‌کارانه و در صورت تفاوت، شکل legacy جستجو را برمی‌گرداند.
// ورودی: query خام. خروجی: یک یا دو عبارت. رقم عوض نمی‌شود و برای ذخیره استفاده نمی‌شود.
func SearchLegacyForms(raw string) []string {
	base := NormalizePersianText(raw)
	if base == "" {
		return nil
	}
	folded := applyLetterMap(base, legacyLetterMap)
	if folded == base {
		return []string{base}
	}
	return []string{base, folded}
}

// applyLetterMap نویسه‌های جدول را جایگزین می‌کند و بقیه را نگه می‌دارد.
// ورودی: رشته و جدول. خروجی: رشتهٔ جایگزین‌شده، یا خالی بعد از trim.
func applyLetterMap(s string, table map[rune]rune) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if mapped, ok := table[r]; ok {
			b.WriteRune(mapped)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// collapseSpaces فاصله‌های یونیکد IsSpace را یکی می‌کند و نیم‌فاصله را نگه می‌دارد.
// ورودی: رشته. خروجی: رشته با یک فاصله بین واژه‌ها.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// HasArabicChars وجود ی/ک عربیِ نگاشت امن را گزارش می‌کند.
// ورودی: رشته. خروجی: true اگر حرفی از persianLetterMap باشد. ئ و رقم را شامل نمی‌شود.
func HasArabicChars(s string) bool {
	for _, r := range s {
		if _, ok := persianLetterMap[r]; ok {
			return true
		}
	}
	return false
}

// ToPersianDigits ارقام لاتین و عربی-هندی را به ارقام فارسی تبدیل می‌کند و بقیه نویسهها را نگه می‌دارد.
// ورودی: رشته‌ای که ممکن است عدد داشته باشد.
// خروجی: همان رشته با ارقام فارسی.
func ToPersianDigits(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune('\u06F0' + (r - '0'))
		case r >= '\u0660' && r <= '\u0669':
			b.WriteRune('\u06F0' + (r - '\u0660'))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FormatPersianInt عدد صحیح را با جداکننده هزارگان و ارقام فارسی برمی‌گرداند.
// ورودی: n مقدار صحیح (منفی هم مجاز است).
// خروجی: رشته نمایشی فارسی مثل ۱٬۲۳۴.
func FormatPersianInt(n int64) string {
	sign := ""
	if n < 0 {
		sign = "−"
		n = -n
	}
	raw := strconv.FormatInt(n, 10)
	var b strings.Builder
	b.Grow(len(raw) + len(raw)/3 + len(sign))
	lead := len(raw) % 3
	if lead == 0 {
		lead = 3
	}
	b.WriteString(raw[:lead])
	for i := lead; i < len(raw); i += 3 {
		b.WriteRune('٬')
		b.WriteString(raw[i : i+3])
	}
	return sign + ToPersianDigits(b.String())
}

// FormatPersianPercent درصد را با یک رقم اعشار، جداکننده فارسی و علامت ٪ برمی‌گرداند.
// ورودی: v مقدار درصد.
// خروجی: رشته‌ای مثل ۱۲٫۵٪.
func FormatPersianPercent(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	s = strings.ReplaceAll(s, ".", "٫")
	if strings.HasPrefix(s, "-") {
		s = "−" + s[1:]
	}
	return ToPersianDigits(s) + "٪"
}
