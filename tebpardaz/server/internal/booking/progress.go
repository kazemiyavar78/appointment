package booking

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
	Step       int    `json:"step"`
	Status     string `json:"status"` // loading | done | error
	Label      string `json:"label"`
	Message    string `json:"message,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
	OK         *bool  `json:"ok,omitempty"`
	Done       bool   `json:"done,omitempty"`
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
func NewFinal(ok bool, message, externalID string) ProgressEvent {
	status := "error"
	if ok {
		status = "done"
	}
	return ProgressEvent{
		Status:     status,
		Message:    message,
		ExternalID: externalID,
		OK:         &ok,
		Done:       true,
	}
}
