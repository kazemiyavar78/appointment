package models

import "gorm.io/gorm"

// ManagementUser is a read-only mapping of tp_managment.users (clinic backend).
// Appointment server must not AutoMigrate this table.
type ManagementUser struct {
	ID                    uint           `gorm:"primaryKey"`
	Username              string         `gorm:"column:username"`
	FullName              string         `gorm:"column:full_name"`
	PhoneNumber           string         `gorm:"column:phone_number"`
	Role                  string         `gorm:"column:role"`
	Active                bool           `gorm:"column:active"`
	ClinicID              uint           `gorm:"column:clinic_id"`
	SendAppointmentStatus bool           `gorm:"column:send_appointment_status"`
	DeletedAt             gorm.DeletedAt `gorm:"index"`
}

// TableName returns the clinic-backend users table name.
func (ManagementUser) TableName() string {
	return "users"
}
