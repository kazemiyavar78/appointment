package services

import (
	"errors"

	"tebpardaz/client/internal/localdb"
	"tebpardaz/shared/protocol"
)

// BookingService persists website bookings/cancellations into the local HIS.
type BookingService struct {
	Store  *localdb.Store
	Sender Sender
}

// NewBookingService constructs a BookingService.
// Inputs: store (local HIS access), sender (WebSocket outbound).
// Output: pointer to BookingService.
func NewBookingService(store *localdb.Store, sender Sender) *BookingService {
	return &BookingService{Store: store, Sender: sender}
}

// HandleCreate saves a new website booking locally and acks the server.
// Inputs: requestID, booking payload from the server.
// Output: error from local save or WebSocket ack send.
func (s *BookingService) HandleCreate(requestID string, booking *protocol.BookingCreate) error {
	ack := protocol.BookingCreateAck{
		IdempotencyKey:    booking.IdempotencyKey,
		SiteAppointmentID: booking.SiteAppointmentID,
		OK:                true,
	}
	externalID, err := s.Store.SaveBooking(booking)
	if err != nil {
		ack.OK = false
		ack.ErrorCode = string(protocol.ErrCodeInternal)
		ack.Message = err.Error()
		var pe *protocol.ProtocolError
		if errors.As(err, &pe) && pe != nil {
			ack.ErrorCode = string(pe.Code)
			ack.Message = pe.Message
		}
	} else {
		ack.ExternalID = externalID
	}
	env, envErr := protocol.NewEnvelope(protocol.TypeBookingCreateAck, requestID, ack)
	if envErr != nil {
		return envErr
	}
	return s.Sender.SendEnvelope(env)
}

// HandleCancel cancels a website booking locally and acks the server.
// Inputs: requestID, cancel payload from the server.
// Output: error from local cancel or WebSocket ack send.
func (s *BookingService) HandleCancel(requestID string, cancel *protocol.BookingCancel) error {
	ack := protocol.BookingCancelAck{
		IdempotencyKey:    cancel.IdempotencyKey,
		SiteAppointmentID: cancel.SiteAppointmentID,
		OK:                true,
	}
	if err := s.Store.CancelBooking(cancel); err != nil {
		ack.OK = false
		ack.ErrorCode = string(protocol.ErrCodeInternal)
		ack.Message = err.Error()
	}
	env, envErr := protocol.NewEnvelope(protocol.TypeBookingCancelAck, requestID, ack)
	if envErr != nil {
		return envErr
	}
	return s.Sender.SendEnvelope(env)
}
