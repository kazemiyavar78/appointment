package pages

import "tebpardaz/server/internal/seo"

// specialtyDetailPageURL آدرس صفحه‌بندی landing تخصص را می‌سازد.
// ورودی: slug ذخیره‌شده و شماره صفحه. خروجی: مسیر نسبی. صفحه ۱ بدون query است.
func specialtyDetailPageURL(slug string, page int) string {
	return seo.SpecialtyPagePath(slug, page)
}
