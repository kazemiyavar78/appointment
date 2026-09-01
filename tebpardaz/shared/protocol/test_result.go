package protocol

// TestResultRequest asks the clinic client for a lab-result PDF by admission credentials.
// Direction: server → client.
// The client looks up `{admission_no}-{password}.pdf` (no clinic code in the filename).
type TestResultRequest struct {
	ClinicCode  int    `json:"clinic_code"`
	AdmissionNo string `json:"admission_no"`
	Password    string `json:"password"`
}

// TestResultResponse carries the PDF (base64) when found, or Found=false.
// Direction: client → server.
type TestResultResponse struct {
	ClinicCode  int    `json:"clinic_code"`
	AdmissionNo string `json:"admission_no"`
	Found       bool   `json:"found"`
	Filename    string `json:"filename,omitempty"`
	PDFBase64   string `json:"pdf_base64,omitempty"`
	Message     string `json:"message,omitempty"`
}
