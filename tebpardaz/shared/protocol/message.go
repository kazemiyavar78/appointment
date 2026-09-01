package protocol

import (
	"encoding/json"
	"time"
)

// MessageType identifies the kind of WebSocket protocol frame.
type MessageType string

const (
	TypePing MessageType = "ping"
	TypePong MessageType = "pong"

	// Doctor list: server may request; client pushes local HIS doctors.
	TypeDoctorListRequest MessageType = "doctor.list.request"
	TypeDoctorListPush    MessageType = "doctor.list.push"
	TypeDoctorListAck     MessageType = "doctor.list.ack"

	// Update approved doctor ExternalID/code on the central site.
	TypeDoctorCodeUpdate    MessageType = "doctor.code.update"
	TypeDoctorCodeUpdateAck MessageType = "doctor.code.update.ack"

	// Server notifies client that a doctor was approved/rejected on the site.
	TypeDoctorApprovalNotify MessageType = "doctor.approval.notify"

	// Appointment sync for one doctor or all synced doctors.
	TypeAppointmentListRequest MessageType = "appointment.list.request"
	TypeAppointmentListPush    MessageType = "appointment.list.push"
	TypeAppointmentListAck     MessageType = "appointment.list.ack"

	// Booking created on the website → save into clinic local DB.
	TypeBookingCreate    MessageType = "booking.create"
	TypeBookingCreateAck MessageType = "booking.create.ack"

	// Cancellation from website → reflect on clinic local DB.
	TypeBookingCancel    MessageType = "booking.cancel"
	TypeBookingCancelAck MessageType = "booking.cancel.ack"

	// Lab result lookup via clinic LIS.
	TypeTestResultRequest  MessageType = "test_result.request"
	TypeTestResultResponse MessageType = "test_result.response"

	// لیست نوبت‌دهی هفتگی پزشکان مرکز (بروزرسانی دوره‌ای از HIS).
	TypeWeeklyReserveListRequest MessageType = "weekly_reserve.list.request"
	TypeWeeklyReserveListPush    MessageType = "weekly_reserve.list.push"
	TypeWeeklyReserveListAck     MessageType = "weekly_reserve.list.ack"

	// صف انتظار بیماران در حال پذیرش (مانیتورینگ لابی).
	TypeWaitingQueueListRequest MessageType = "waiting_queue.list.request"
	TypeWaitingQueueListPush    MessageType = "waiting_queue.list.push"
	TypeWaitingQueueListAck     MessageType = "waiting_queue.list.ack"

	// Generic error frame when a request cannot be fulfilled.
	TypeError MessageType = "error"
)

// Envelope is the common wrapper for all client↔server WebSocket messages.
type Envelope struct {
	Type      MessageType     `json:"type"`
	RequestID string          `json:"request_id"`
	ClinicKey string          `json:"clinic_key,omitempty"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Error     *ProtocolError  `json:"error,omitempty"`
}

// NewEnvelope builds a protocol envelope with the given type, request id, and payload.
// Inputs: typ (message type), requestID (correlation id), payload (any JSON-serializable value).
// Output: pointer to Envelope or an error if payload marshaling fails.
func NewEnvelope(typ MessageType, requestID string, payload any) (*Envelope, error) {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	return &Envelope{
		Type:      typ,
		RequestID: requestID,
		Timestamp: time.Now().Unix(),
		Payload:   raw,
	}, nil
}

// DecodePayload unmarshals Envelope.Payload into dest.
// Inputs: dest pointer to the expected payload struct.
// Output: error if payload is empty or JSON does not match dest.
func (e *Envelope) DecodePayload(dest any) error {
	if len(e.Payload) == 0 {
		return ErrEmptyPayload
	}
	return json.Unmarshal(e.Payload, dest)
}

// MustMarshal serializes the envelope to JSON bytes for WebSocket write.
// Inputs: none (receiver).
// Output: JSON bytes or error.
func (e *Envelope) MustMarshal() ([]byte, error) {
	return json.Marshal(e)
}
