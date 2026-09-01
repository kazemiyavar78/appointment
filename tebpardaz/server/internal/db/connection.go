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

	appointment, err := gorm.Open(sqlserver.Open(appointmentDSN), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Warn),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open appointment db: %w", err)
	}

	c := &Connections{Appointment: appointment}

	if managementDSN != "" {
		management, err := gorm.Open(sqlserver.Open(managementDSN), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
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

// MigrateAppointment runs AutoMigrate for appointment_tapesh models only.
// Clinic / Organization / City live in tp_managment and are intentionally skipped.
// Inputs: none (uses Appointment connection).
// Output: migration error, if any.
func (c *Connections) MigrateAppointment() error {
	if c == nil || c.Appointment == nil {
		return fmt.Errorf("appointment db is not open")
	}

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
		&models.DoctorApprovalRequest{},
		&models.AppointmentUser{},
		&models.News{},
		&models.TestResultCache{},
		&models.Insurance{},
		&models.ClinicInsurance{},
		&models.Service{},
		&models.ClinicInsuranceService{},
		&models.DoctorService{},
		&models.Review{},
		&models.ClinicBehaviorLog{},
		&models.AppointmentClinicSection{},
		&models.ClinicSectionQuota{},
		&models.SectionBanner{},
		&models.SectionSchedule{},
		&models.SectionMessage{},
		&models.SectionEquipment{},
	)
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
