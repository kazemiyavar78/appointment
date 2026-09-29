package pages

import "tebpardaz/server/internal/seo"

// clinicDetailPageURL آدرس صفحه‌بندی landing مرکز را می‌سازد.
// ورودی: slug ذخیره‌شده و شماره صفحه. خروجی: مسیر نسبی. صفحه ۱ بدون query است.
func clinicDetailPageURL(slug string, page int) string {
	return seo.ClinicPagePath(slug, page)
}
