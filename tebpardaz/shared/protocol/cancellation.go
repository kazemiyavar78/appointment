package protocol

// BookingCancel asks the clinic client to cancel a local appointment.
// Direction: server → client.
type BookingCancel struct {
	IdempotencyKey    string `json:"idempotency_key"`
	SiteAppointmentID uint   `json:"site_appointment_id"`
	ExternalID        string `json:"external_id,omitempty"`
	DoctorExternalID  string `json:"doctor_external_id,omitempty"`
	Reason            string `json:"reason,omitempty"`
}

// BookingCancelAck reports the result of a local cancellation.
// Direction: client → server.
type BookingCancelAck struct {
	IdempotencyKey    string `json:"idempotency_key"`
	SiteAppointmentID uint   `json:"site_appointment_id"`
	OK                bool   `json:"ok"`
	ErrorCode         string `json:"error_code,omitempty"`
	Message           string `json:"message,omitempty"`
}

// CancellationRequest is an alias for BookingCancel.
type CancellationRequest = BookingCancel

// CancellationResponse is an alias for BookingCancelAck.
type CancellationResponse = BookingCancelAck
