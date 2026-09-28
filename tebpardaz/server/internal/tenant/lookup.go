package tenant

import (
	"errors"

	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"

	"gorm.io/gorm"
)

// LookupService ردیف کلینیک و سازمان را برای resolver بارگذاری می‌کند.
// اگر در کش نباشد از دیتابیس می‌خواند، در کش می‌نویسد، و صداکننده دوباره کش را می‌خواند.
type LookupService struct {
	Clinics *repository.ClinicRepo
	Orgs    *repository.OrganizationRepo
	Cache   *cache.TenantCache
}

// NewLookupService یک LookupService می‌سازد.
// ورودی: مخزن کلینیک، مخزن سازمان، کش. کش nil یعنی هر بار مستقیم از دیتابیس خوانده شود.
// خروجی: اشاره‌گر به LookupService.
func NewLookupService(clinics *repository.ClinicRepo, orgs *repository.OrganizationRepo, tenantCache *cache.TenantCache) *LookupService {
	return &LookupService{
		Clinics: clinics,
		Orgs:    orgs,
		Cache:   tenantCache,
	}
}

// ClinicByDomain کلینیک دامنه اختصاصی را برمی‌گرداند.
// ورودی: domain بدون پورت. خروجی: کلینیک، ErrRecordNotFound، یا خطای دیگر دیتابیس.
func (s *LookupService) ClinicByDomain(domain string) (*models.Clinic, error) {
	if s == nil {
		return nil, gorm.ErrRecordNotFound
	}
	if s.Cache == nil {
		return clinicFromRepo(s.Clinics, func(r *repository.ClinicRepo) (*models.Clinic, error) {
			return r.GetByDomain(domain)
		})
	}
	if clinic, found, ok := s.Cache.GetClinicByDomain(domain); ok {
		return clinicFromCache(clinic, found)
	}
	if err := s.loadClinicByDomain(domain); err != nil {
		return nil, err
	}
	if clinic, found, ok := s.Cache.GetClinicByDomain(domain); ok {
		return clinicFromCache(clinic, found)
	}
	return nil, ErrTenantNotFound
}

// ClinicBySlug کلینیک اسلاگ پلتفرم را برمی‌گرداند.
// ورودی: slug. خروجی: کلینیک، ErrRecordNotFound، یا خطای دیگر دیتابیس.
func (s *LookupService) ClinicBySlug(slug string) (*models.Clinic, error) {
	if s == nil {
		return nil, gorm.ErrRecordNotFound
	}
	if s.Cache == nil {
		return clinicFromRepo(s.Clinics, func(r *repository.ClinicRepo) (*models.Clinic, error) {
			return r.GetBySlug(slug)
		})
	}
	if clinic, found, ok := s.Cache.GetClinicBySlug(slug); ok {
		return clinicFromCache(clinic, found)
	}
	if err := s.loadClinicBySlug(slug); err != nil {
		return nil, err
	}
	if clinic, found, ok := s.Cache.GetClinicBySlug(slug); ok {
		return clinicFromCache(clinic, found)
	}
	return nil, ErrTenantNotFound
}

// OrgByDomain سازمان دامنه اختصاصی را برمی‌گرداند.
// ورودی: domain بدون پورت. خروجی: سازمان، ErrRecordNotFound، یا خطای دیگر دیتابیس.
func (s *LookupService) OrgByDomain(domain string) (*models.Organization, error) {
	if s == nil {
		return nil, gorm.ErrRecordNotFound
	}
	if s.Cache == nil {
		return orgFromRepo(s.Orgs, func(r *repository.OrganizationRepo) (*models.Organization, error) {
			return r.GetByDomain(domain)
		})
	}
	if org, found, ok := s.Cache.GetOrgByDomain(domain); ok {
		return orgFromCache(org, found)
	}
	if err := s.loadOrgByDomain(domain); err != nil {
		return nil, err
	}
	if org, found, ok := s.Cache.GetOrgByDomain(domain); ok {
		return orgFromCache(org, found)
	}
	return nil, ErrTenantNotFound
}

// OrgBySlug سازمان اسلاگ پلتفرم را برمی‌گرداند.
// ورودی: slug. خروجی: سازمان، ErrRecordNotFound، یا خطای دیگر دیتابیس.
func (s *LookupService) OrgBySlug(slug string) (*models.Organization, error) {
	if s == nil {
		return nil, gorm.ErrRecordNotFound
	}
	if s.Cache == nil {
		return orgFromRepo(s.Orgs, func(r *repository.OrganizationRepo) (*models.Organization, error) {
			return r.GetBySlug(slug)
		})
	}
	if org, found, ok := s.Cache.GetOrgBySlug(slug); ok {
		return orgFromCache(org, found)
	}
	if err := s.loadOrgBySlug(slug); err != nil {
		return nil, err
	}
	if org, found, ok := s.Cache.GetOrgBySlug(slug); ok {
		return orgFromCache(org, found)
	}
	return nil, ErrTenantNotFound
}

// loadClinicByDomain کلینیک را از دیتابیس می‌خواند و در کش می‌نویسد.
// ورودی: domain. خروجی: خطای دیتابیس. نبودن رکورد به‌صورت miss کش می‌شود و nil برمی‌گردد.
func (s *LookupService) loadClinicByDomain(domain string) error {
	clinic, err := clinicFromRepo(s.Clinics, func(r *repository.ClinicRepo) (*models.Clinic, error) {
		return r.GetByDomain(domain)
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if s.Cache != nil {
		s.Cache.SetClinicByDomain(domain, clinic)
	}
	return nil
}

// loadClinicBySlug کلینیک را از دیتابیس می‌خواند و در کش می‌نویسد.
// ورودی: slug. خروجی: خطای دیتابیس. نبودن رکورد به‌صورت miss کش می‌شود و nil برمی‌گردد.
func (s *LookupService) loadClinicBySlug(slug string) error {
	clinic, err := clinicFromRepo(s.Clinics, func(r *repository.ClinicRepo) (*models.Clinic, error) {
		return r.GetBySlug(slug)
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if s.Cache != nil {
		s.Cache.SetClinicBySlug(slug, clinic)
	}
	return nil
}

// loadOrgByDomain سازمان را از دیتابیس می‌خواند و در کش می‌نویسد.
// ورودی: domain. خروجی: خطای دیتابیس. نبودن رکورد به‌صورت miss کش می‌شود و nil برمی‌گردد.
func (s *LookupService) loadOrgByDomain(domain string) error {
	org, err := orgFromRepo(s.Orgs, func(r *repository.OrganizationRepo) (*models.Organization, error) {
		return r.GetByDomain(domain)
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if s.Cache != nil {
		s.Cache.SetOrgByDomain(domain, org)
	}
	return nil
}

// loadOrgBySlug سازمان را از دیتابیس می‌خواند و در کش می‌نویسد.
// ورودی: slug. خروجی: خطای دیتابیس. نبودن رکورد به‌صورت miss کش می‌شود و nil برمی‌گردد.
func (s *LookupService) loadOrgBySlug(slug string) error {
	org, err := orgFromRepo(s.Orgs, func(r *repository.OrganizationRepo) (*models.Organization, error) {
		return r.GetBySlug(slug)
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if s.Cache != nil {
		s.Cache.SetOrgBySlug(slug, org)
	}
	return nil
}

// clinicFromCache نتیجه کش کلینیک را به خروجی مخزن تبدیل می‌کند.
// ورودی: clinic و found. خروجی: کلینیک یا ErrRecordNotFound.
func clinicFromCache(clinic *models.Clinic, found bool) (*models.Clinic, error) {
	if !found || clinic == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return clinic, nil
}

// orgFromCache نتیجه کش سازمان را به خروجی مخزن تبدیل می‌کند.
// ورودی: org و found. خروجی: سازمان یا ErrRecordNotFound.
func orgFromCache(org *models.Organization, found bool) (*models.Organization, error) {
	if !found || org == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return org, nil
}

// clinicFromRepo اگر مخزن nil باشد ErrRecordNotFound می‌دهد وگرنه load را صدا می‌زند.
// ورودی: repo و load. خروجی: کلینیک یا خطا.
func clinicFromRepo(repo *repository.ClinicRepo, load func(*repository.ClinicRepo) (*models.Clinic, error)) (*models.Clinic, error) {
	if repo == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return load(repo)
}

// orgFromRepo اگر مخزن nil باشد ErrRecordNotFound می‌دهد وگرنه load را صدا می‌زند.
// ورودی: repo و load. خروجی: سازمان یا خطا.
func orgFromRepo(repo *repository.OrganizationRepo, load func(*repository.OrganizationRepo) (*models.Organization, error)) (*models.Organization, error) {
	if repo == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return load(repo)
}
