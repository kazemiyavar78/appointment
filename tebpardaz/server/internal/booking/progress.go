package booking

import "strconv"

// Progress step identifiers for the public booking WebSocket UI.
const (
	StepReceived   = 1 // اطلاعات دریافت شد
	StepValidated  = 2 // اعتبار سنجی انجام شد
	StepSentClinic = 3 // برای کلینیک ارسال شد
	StepClinicAck  = 4 // جواب از کلینیک دریافت شد
)

// StepLabel returns the Persian label for a progress step.
// Inputs: step number (1–4).
// Output: display label.
func StepLabel(step int) string {
	switch step {
	case StepReceived:
		return "اطلاعات دریافت شد"
	case StepValidated:
		return "اعتبار سنجی انجام شد"
	case StepSentClinic:
		return "برای کلینیک ارسال شد"
	case StepClinicAck:
		return "جواب از کلینیک دریافت شد"
	default:
		return ""
	}
}

// ProgressEvent is pushed to the browser over the booking WebSocket.
type ProgressEvent struct {
	Step         int    `json:"step"`
	Status       string `json:"status"` // loading | done | error
	Label        string `json:"label"`
	Message      string `json:"message,omitempty"`
	TrackingCode string `json:"tracking_code,omitempty"`
	OK           *bool  `json:"ok"`
	Done         bool   `json:"done"`
}

// NewStepLoading builds a loading progress event for the given step.
func NewStepLoading(step int) ProgressEvent {
	return ProgressEvent{Step: step, Status: "loading", Label: StepLabel(step)}
}

// NewStepDone builds a completed progress event for the given step.
func NewStepDone(step int) ProgressEvent {
	return ProgressEvent{Step: step, Status: "done", Label: StepLabel(step)}
}

// NewFinal builds the terminal progress event with the booking result message.
// Inputs: ok (success flag), message (Persian user text), trackingCode (public follow-up code; empty on failure).
// Output: event with Done=true and OK always serialized so the browser can show errors.
func NewFinal(ok bool, message, trackingCode string) ProgressEvent {
	status := "error"
	if ok {
		status = "done"
	}
	return ProgressEvent{
		Status:       status,
		Message:      message,
		TrackingCode: trackingCode,
		OK:           &ok,
		Done:         true,
	}
}

// FormatTrackingCode builds the public follow-up code from the site appointment ID.
// Inputs: appointmentID (PatientAppointment.ID). Output: decimal code, or empty when id is 0.
func FormatTrackingCode(appointmentID uint) string {
	if appointmentID == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(appointmentID), 10)
}
