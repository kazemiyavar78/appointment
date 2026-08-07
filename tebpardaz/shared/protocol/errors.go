package protocol

// ErrorCode is a stable protocol error identifier.
type ErrorCode string

const (
	ErrCodeUnknown       ErrorCode = "unknown"
	ErrCodeUnauthorized  ErrorCode = "unauthorized"
	ErrCodeNotFound      ErrorCode = "not_found"
	ErrCodeConflict      ErrorCode = "conflict"
	ErrCodeValidation    ErrorCode = "validation"
	ErrCodeRateLimited   ErrorCode = "rate_limited"
	ErrCodeUnavailable   ErrorCode = "unavailable"
	ErrCodeInternal      ErrorCode = "internal"
	ErrCodeAlreadyBooked ErrorCode = "already_booked"
	ErrCodeNoCapacity    ErrorCode = "no_capacity"
)

// ProtocolError is returned inside Envelope on failure.
type ProtocolError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
}

// Error implements the error interface.
// Inputs: none (receiver).
// Output: human-readable error string.
func (e *ProtocolError) Error() string {
	if e == nil {
		return "protocol error"
	}
	if e.Message == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Message
}

// ErrEmptyPayload is returned when DecodePayload is called on an empty payload.
var ErrEmptyPayload = &ProtocolError{Code: ErrCodeValidation, Message: "empty payload"}
