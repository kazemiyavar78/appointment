package pages

import "strconv"

const (
	ringCircumference = 703.7
	ringMaxAhead      = 20
)

// waitingQueueLiveClass کلاس ریشه پنل زنده را با حالت رنگی فعلی می‌سازد.
// ورودی: تعداد نفرات جلوتر. خروجی: کلاس‌های CSS.
func waitingQueueLiveClass(ahead int) string {
	return "wq-live wq-state-" + waitingQueueState(ahead)
}

// ورودی: تعداد نفرات. خروجی: calm، approaching، urgent یا now.
func waitingQueueState(ahead int) string {
	if ahead <= 0 {
		return "now"
	}
	if ahead <= 3 {
		return "urgent"
	}
	if ahead <= 10 {
		return "approaching"
	}
	return "calm"
}

// waitingQueueBanner جمله وضعیت را برای همان حالت برمی‌گرداند.
// ورودی: تعداد نفرات جلوتر. خروجی: متن فارسی بنر.
func waitingQueueBanner(ahead int) string {
	switch waitingQueueState(ahead) {
	case "now":
		return "نوبت شماست"
	case "urgent":
		return "نوبت شما خیلی نزدیک است، لطفاً به مطب مراجعه کنید"
	case "approaching":
		return "نوبت شما نزدیک می‌شود — بهتر است در دسترس باشید"
	default:
		return "می‌توانید در محیط دیگری منتظر بمانید، به‌موقع خبر می‌دهیم"
	}
}

// ringDashOffset مقدار stroke-dashoffset حلقه را طوری می‌دهد که با نزدیک شدن نوبت حلقه پر شود.
// ورودی: تعداد نفرات جلوتر. خروجی: عدد اعشاری برای ویژگی SVG.
func ringDashOffset(ahead int) string {
	if ahead < 0 {
		ahead = 0
	}
	if ahead > ringMaxAhead {
		ahead = ringMaxAhead
	}
	offset := ringCircumference * float64(ahead) / float64(ringMaxAhead)
	return strconv.FormatFloat(offset, 'f', 1, 64)
}

// faQueueNumber عدد وسط حلقه را فارسی می‌کند و در نوبت خود بیمار «حالا» نشان می‌دهد.
// ورودی: تعداد نفرات. خروجی: متن وسط حلقه.
func faQueueNumber(ahead int) string {
	if ahead <= 0 {
		return "حالا"
	}
	return faDigits(ahead)
}

// ringCaption برچسب زیر عدد حلقه است و وقتی نوبت رسیده خالی می‌ماند.
// ورودی: تعداد نفرات. خروجی: برچسب یا رشته خالی.
func ringCaption(ahead int) string {
	if ahead <= 0 {
		return ""
	}
	return "نفر جلوتر از شما"
}

// faDigits رقم‌های انگلیسی را به فارسی تبدیل می‌کند.
// ورودی: عدد. خروجی: رشته با ارقام فارسی.
func faDigits(n int) string {
	raw := strconv.Itoa(n)
	digits := []string{"۰", "۱", "۲", "۳", "۴", "۵", "۶", "۷", "۸", "۹"}
	out := make([]rune, 0, len(raw))
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			out = append(out, []rune(digits[r-'0'])...)
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
