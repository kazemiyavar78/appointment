package websocket

import (
	"encoding/json"
	"time"

	"tebpardaz/shared/protocol"
)

// BookingDispatcher sends booking requests to the owning clinic client and awaits ack.
type BookingDispatcher struct {
	Hub *Hub
}

// NewBookingDispatcher constructs a BookingDispatcher.
// Inputs: hub of clinic WebSocket connections.
// Output: pointer to BookingDispatcher.
func NewBookingDispatcher(hub *Hub) *BookingDispatcher {
	return &BookingDispatcher{Hub: hub}
}

// DispatchBookingCreate sends BookingCreate and waits for BookingCreateAck.
// Inputs: clinicID, timeout, payload.
// Output: ack from clinic or hub error.
func (d *BookingDispatcher) DispatchBookingCreate(clinicID uint, timeout time.Duration, req protocol.BookingCreate) (*protocol.BookingCreateAck, error) {
	if d == nil || d.Hub == nil {
		return nil, ErrClinicOffline
	}
	return d.Hub.RequestBookingCreate(clinicID, timeout, req)
}

// DecodeBookingCreateAck unmarshals a booking.create.ack payload for tests/helpers.
func DecodeBookingCreateAck(raw json.RawMessage) (*protocol.BookingCreateAck, error) {
	var ack protocol.BookingCreateAck
	if err := json.Unmarshal(raw, &ack); err != nil {
		return nil, err
	}
	return &ack, nil
}
