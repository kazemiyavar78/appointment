package protocol

import "time"

// BookingCreate is sent from server to clinic client to persist a website booking locally.
// Direction: server → client.
type BookingCreate struct {
	IdempotencyKey    string    `json:"idempotency_key"`
	SiteAppointmentID uint      `json:"site_appointment_id"`
	DoctorExternalID  string    `json:"doctor_external_id"`
	ExternalSlotID    string    `json:"external_slot_id,omitempty"`
	StartsAtUnix      int64     `json:"starts_at_unix"`
	EndsAtUnix        int64     `json:"ends_at_unix"`
	StartDateTime     time.Time `json:"start_date_time"`

	NationalID string `json:"national_id"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Mobile     string `json:"mobile"`
	BirthDate  string `json:"birth_date,omitempty"`
	Sex        int `json:"sex,omitempty"`
}

// BookingCreateAck reports whether the clinic client saved the booking.
// Direction: client → server.
type BookingCreateAck struct {
	IdempotencyKey    string `json:"idempotency_key"`
	SiteAppointmentID uint   `json:"site_appointment_id"`
	ExternalID        string `json:"external_id,omitempty"`
	OK                bool   `json:"ok"`
	ErrorCode         string `json:"error_code,omitempty"`
	Message           string `json:"message,omitempty"`
}

// BookingRequest is kept as an alias name for older docs; prefer BookingCreate.
type BookingRequest = BookingCreate

// BookingResponse is kept as an alias name for older docs; prefer BookingCreateAck.
type BookingResponse = BookingCreateAck
