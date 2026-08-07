package protocol

// TestResultRequest asks the clinic client/LIS for a patient's lab result.
// Direction: server → client.
type TestResultRequest struct {
	NationalID string `json:"national_id"`
	Barcode    string `json:"barcode,omitempty"`
}

// TestResultResponse carries lab result payload (or a not-found status).
// Direction: client → server.
type TestResultResponse struct {
	NationalID  string `json:"national_id"`
	Barcode     string `json:"barcode,omitempty"`
	Found       bool   `json:"found"`
	PayloadJSON string `json:"payload_json,omitempty"`
	Message     string `json:"message,omitempty"`
}
