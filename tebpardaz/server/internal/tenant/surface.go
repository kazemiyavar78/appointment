package tenant

import "tebpardaz/server/internal/models"

// PublicSurface لایه‌ای است که یک مرکز فعال می‌تواند همزمان در چندتای آن دیده شود.
type PublicSurface int

const (
	// SurfacePlatform دایرکتوری tebpardaz.ir است و دامنهٔ اختصاصی مرکز را حذف نمی‌کند.
	SurfacePlatform PublicSurface = iota + 1
	// SurfaceOrganization سایت دامنهٔ سازمان است و فقط مراکز همان سازمان را می‌بیند.
	SurfaceOrganization
	// SurfaceClinic دامنهٔ اختصاصی خود مرکز است.
	SurfaceClinic
)

// ClinicOnWebsite می‌گوید مرکز در سایت نوبت‌دهی عمومی هست یا نه.
// ورودی: مرکز. خروجی: true فقط با is_active_on_website.
// عضویت سازمان، tenant_type و دامنه در این تصمیم نیستند. حذف نرم را GORM جدا اعمال می‌کند.
func ClinicOnWebsite(clinic *models.Clinic) bool {
	return clinic != nil && clinic.IsActiveOnWebsite
}

// ClinicVisibleOnPlatform می‌گوید مرکز در دایرکتوری طب‌پرداز بیاید یا نه.
// ورودی: مرکز. خروجی: true برای هر مرکز فعال، حتی با دامنهٔ مرکز یا سازمان.
func ClinicVisibleOnPlatform(clinic *models.Clinic) bool {
	return ClinicOnWebsite(clinic)
}

// ClinicVisibleOnOrganization می‌گوید مرکز روی دامنهٔ همین سازمان دیده شود یا نه.
// ورودی: مرکز و شناسهٔ سازمان resolveشده. خروجی: true فقط با سایت فعال و همان organization_id.
func ClinicVisibleOnOrganization(clinic *models.Clinic, organizationID uint) bool {
	return ClinicOnWebsite(clinic) && organizationID != 0 && clinic.OrganizationID == organizationID
}

// OrganizationDomainSurface می‌گوید این هاست همان دامنهٔ ذخیره‌شدهٔ سازمان است یا نه.
// ورودی: سازمان و هاست. خروجی: true فقط وقتی دامنهٔ غیرخالی با هاست یکی باشد.
// status و slug در این تصمیم نیستند.
func OrganizationDomainSurface(org *models.Organization, host string) bool {
	if org == nil || org.Domain == nil {
		return false
	}
	domain := normalizeHost(*org.Domain)
	host = normalizeHost(host)
	return domain != "" && domain == host
}

// ClinicVisibleOnOwnDomain می‌گوید این هاست دامنهٔ اختصاصی همان مرکز است یا نه.
// ورودی: مرکز و هاست درخواست. خروجی: true فقط با سایت فعال و برابری دامنهٔ ذخیره‌شده با هاست.
func ClinicVisibleOnOwnDomain(clinic *models.Clinic, host string) bool {
	if !ClinicOnWebsite(clinic) || clinic.Domain == nil {
		return false
	}
	domain := normalizeHost(*clinic.Domain)
	host = normalizeHost(host)
	return domain != "" && domain == host
}
