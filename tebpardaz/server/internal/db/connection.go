package db

import (
	"fmt"
	"log"

	"tebpardaz/server/internal/models"

	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connections holds the two SQL Server databases used by the server.
// Management (tp_managment) owns Clinic/Organization/City and is never AutoMigrated.
// Appointment (appointment_tapesh) owns the rest and is migrated at startup.
type Connections struct {
	Management  *gorm.DB
	Appointment *gorm.DB
}

// Open establishes GORM connections to management and appointment databases.
// Inputs: managementDSN (tp_managment), appointmentDSN (appointment_tapesh).
// Output: Connections pointer or error if either DSN fails to open.
func Open(managementDSN, appointmentDSN string) (*Connections, error) {
	if appointmentDSN == "" {
		return nil, fmt.Errorf("appointment DSN is required")
	}

	// لاگ SQL گورم غیرفعال است تا خروجی کنسول شلوغ نشود.
	appointment, err := gorm.Open(sqlserver.Open(appointmentDSN), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open appointment db: %w", err)
	}

	c := &Connections{Appointment: appointment}

	if managementDSN != "" {
		management, err := gorm.Open(sqlserver.Open(managementDSN), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			return nil, fmt.Errorf("open management db: %w", err)
		}
		c.Management = management
	} else {
		log.Println("db: MANAGEMENT_DSN empty — clinic lookups against tp_managment are disabled")
	}

	return c, nil
}

// MigrateManagementClinicBranding ensures logo_url and favicon_url exist on tp_managment.clinics.
// Inputs: none (uses Management connection).
// Output: migration error, if any.
func (c *Connections) MigrateManagementClinicBranding() error {
	if c == nil || c.Management == nil {
		return nil
	}
	brandingCols := []string{"logo_url", "favicon_url"}
	for _, col := range brandingCols {
		query := fmt.Sprintf(`
			IF NOT EXISTS (
				SELECT 1 FROM sys.columns
				WHERE Name = N'%s' AND Object_ID = Object_ID(N'clinics')
			)
			BEGIN
				ALTER TABLE clinics ADD %s NVARCHAR(500) NOT NULL DEFAULT '';
			END`, col, col)
		if err := c.Management.Exec(query).Error; err != nil {
			return fmt.Errorf("migrate clinics.%s: %w", col, err)
		}
	}
	return nil
}

// MigrateAppointment runs AutoMigrate for appointment_tapesh models only.
// Clinic / Organization / City live in tp_managment and are intentionally skipped.
// Inputs: none (uses Appointment connection).
// Output: migration error, if any.
func (c *Connections) MigrateAppointment() error {
	if c == nil || c.Appointment == nil {
		return fmt.Errorf("appointment db is not open")
	}

	ensureSpecialtyDisplayColumns(c.Appointment)
	ensureSectionBannerGradientColumns(c.Appointment)

	ensureDoctorUseClinicLogoColumn(c.Appointment)

	// اطمینان از وجود ستون‌های عکس در جدول پزشکان (برای سازگاری با SQL Server)
	photoCols := []string{"photo_300", "photo_600", "photo_900", "photo_1200"}
	for _, col := range photoCols {
		query := fmt.Sprintf(`
			IF NOT EXISTS (
				SELECT 1 FROM sys.columns 
				WHERE Name = N'%s' AND Object_ID = Object_ID(N'doctors')
			)
			BEGIN
				ALTER TABLE doctors ADD %s NVARCHAR(255) NOT NULL DEFAULT '';
			END`, col, col)
		_ = c.Appointment.Exec(query).Error
	}

	// همگام‌سازی ستون‌های بدون خط تیره به ستون‌های استاندارد در صورت وجود
	syncQuery := `
		IF EXISTS (SELECT 1 FROM sys.columns WHERE Name = N'photo300' AND Object_ID = Object_ID(N'doctors'))
		BEGIN
			UPDATE doctors SET photo_300 = photo300 WHERE (photo_300 IS NULL OR photo_300 = '') AND photo300 IS NOT NULL AND photo300 <> '';
			UPDATE doctors SET photo_600 = photo600 WHERE (photo_600 IS NULL OR photo_600 = '') AND photo600 IS NOT NULL AND photo600 <> '';
			UPDATE doctors SET photo_900 = photo900 WHERE (photo_900 IS NULL OR photo_900 = '') AND photo900 IS NOT NULL AND photo900 <> '';
			UPDATE doctors SET photo_1200 = photo1200 WHERE (photo_1200 IS NULL OR photo_1200 = '') AND photo1200 IS NOT NULL AND photo1200 <> '';
		END`
	_ = c.Appointment.Exec(syncQuery).Error

	return c.Appointment.AutoMigrate(
		&models.Specialty{},
		&models.Doctor{},
		&models.DoctorSlot{},
		&models.Patient{},
		&models.PatientAppointment{},
		&models.BookingOTP{},
		&models.DoctorApprovalRequest{},
		&models.AppointmentUser{},
		&models.News{},
		&models.TestResultCache{},
		&models.Insurance{},
		&models.ClinicInsurance{},
		&models.Service{},
		&models.ClinicInsuranceService{},
		&models.DoctorService{},
		&models.ServicePackage{},
		&models.ServicePackageItem{},
		&models.SectionServicePackage{},
		&models.Review{},
		&models.ClinicBehaviorLog{},
		&models.AppointmentClinicSection{},
		&models.ClinicSectionQuota{},
		&models.SectionBanner{},
		&models.SectionSchedule{},
		&models.SectionMessage{},
		&models.SectionEquipment{},
		&models.SectionDoctor{},
		&models.VisitorIP{},
		&models.VisitDetail{},
		&models.IPRestriction{}, // جدول بلاک و rate limit آی‌پی
		&models.WaitingQueueSubmission{},
	)
}

// ensureDoctorUseClinicLogoColumn ستون انتخاب لوگوی مرکز به‌جای عکس پزشک را اضافه می‌کند.
// ورودی: اتصال appointment. خروجی: ندارد (خطای SQL نادیده گرفته می‌شود تا AutoMigrate ادامه یابد).
func ensureDoctorUseClinicLogoColumn(db *gorm.DB) {
	if db == nil {
		return
	}
	_ = db.Exec(`
		IF NOT EXISTS (
			SELECT 1 FROM sys.columns
			WHERE Name = N'use_clinic_logo' AND Object_ID = Object_ID(N'doctors')
		)
		BEGIN
			ALTER TABLE doctors ADD use_clinic_logo BIT NOT NULL CONSTRAINT DF_doctors_use_clinic_logo DEFAULT 0;
		END`).Error
}

// ensureSpecialtyDisplayColumns ستون‌های ترتیب نمایش و نمایش در نوبت‌دهی را با پیش‌فرض امن اضافه می‌کند.
// ورودی: اتصال appointment. خروجی: ندارد (خطای SQL نادیده گرفته می‌شود تا AutoMigrate ادامه یابد).
func ensureSpecialtyDisplayColumns(db *gorm.DB) {
	if db == nil {
		return
	}
	queries := []string{
		`
		IF NOT EXISTS (
			SELECT 1 FROM sys.columns
			WHERE Name = N'sort_order' AND Object_ID = Object_ID(N'specialties')
		)
		BEGIN
			ALTER TABLE specialties ADD sort_order INT NOT NULL CONSTRAINT DF_specialties_sort_order DEFAULT 0;
		END`,
		`
		IF NOT EXISTS (
			SELECT 1 FROM sys.columns
			WHERE Name = N'show_in_booking' AND Object_ID = Object_ID(N'specialties')
		)
		BEGIN
			ALTER TABLE specialties ADD show_in_booking BIT NOT NULL CONSTRAINT DF_specialties_show_in_booking DEFAULT 1;
		END`,
	}
	for _, query := range queries {
		_ = db.Exec(query).Error
	}
}

// ensureSectionBannerGradientColumns ستون‌های گرادیان پس‌زمینه و پوشش تصویر بنر بخش را اضافه می‌کند.
// ورودی: اتصال appointment. خروجی: ندارد (خطای SQL نادیده گرفته می‌شود تا AutoMigrate ادامه یابد).
func ensureSectionBannerGradientColumns(db *gorm.DB) {
	if db == nil {
		return
	}
	queries := []string{
		`
		IF NOT EXISTS (
			SELECT 1 FROM sys.columns
			WHERE Name = N'background_color_end' AND Object_ID = Object_ID(N'section_banners')
		)
		BEGIN
			ALTER TABLE section_banners ADD background_color_end NVARCHAR(50) NOT NULL CONSTRAINT DF_section_banners_background_color_end DEFAULT '';
		END`,
		`
		IF NOT EXISTS (
			SELECT 1 FROM sys.columns
			WHERE Name = N'use_background_gradient' AND Object_ID = Object_ID(N'section_banners')
		)
		BEGIN
			ALTER TABLE section_banners ADD use_background_gradient BIT NOT NULL CONSTRAINT DF_section_banners_use_background_gradient DEFAULT 0;
		END`,
		`
		IF NOT EXISTS (
			SELECT 1 FROM sys.columns
			WHERE Name = N'background_gradient_dir' AND Object_ID = Object_ID(N'section_banners')
		)
		BEGIN
			ALTER TABLE section_banners ADD background_gradient_dir NVARCHAR(40) NOT NULL CONSTRAINT DF_section_banners_background_gradient_dir DEFAULT 'to left';
		END`,
		`
		IF NOT EXISTS (
			SELECT 1 FROM sys.columns
			WHERE Name = N'overlay_color' AND Object_ID = Object_ID(N'section_banners')
		)
		BEGIN
			ALTER TABLE section_banners ADD overlay_color NVARCHAR(50) NOT NULL CONSTRAINT DF_section_banners_overlay_color DEFAULT '#0a2e2e';
		END`,
		`
		IF NOT EXISTS (
			SELECT 1 FROM sys.columns
			WHERE Name = N'use_overlay_gradient' AND Object_ID = Object_ID(N'section_banners')
		)
		BEGIN
			ALTER TABLE section_banners ADD use_overlay_gradient BIT NOT NULL CONSTRAINT DF_section_banners_use_overlay_gradient DEFAULT 1;
		END`,
		`
		IF NOT EXISTS (
			SELECT 1 FROM sys.columns
			WHERE Name = N'overlay_opacity_left' AND Object_ID = Object_ID(N'section_banners')
		)
		BEGIN
			ALTER TABLE section_banners ADD overlay_opacity_left INT NOT NULL CONSTRAINT DF_section_banners_overlay_opacity_left DEFAULT 85;
		END`,
		`
		IF NOT EXISTS (
			SELECT 1 FROM sys.columns
			WHERE Name = N'overlay_opacity_bottom' AND Object_ID = Object_ID(N'section_banners')
		)
		BEGIN
			ALTER TABLE section_banners ADD overlay_opacity_bottom INT NOT NULL CONSTRAINT DF_section_banners_overlay_opacity_bottom DEFAULT 60;
		END`,
	}
	for _, query := range queries {
		_ = db.Exec(query).Error
	}
}

// Close releases both underlying database connections.
// Inputs: none (receiver).
// Output: first close error encountered, if any.
func (c *Connections) Close() error {
	if c == nil {
		return nil
	}
	var firstErr error
	for _, gdb := range []*gorm.DB{c.Appointment, c.Management} {
		if gdb == nil {
			continue
		}
		sqlDB, err := gdb.DB()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := sqlDB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
