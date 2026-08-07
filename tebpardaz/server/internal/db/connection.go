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
	Management *gorm.DB
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
