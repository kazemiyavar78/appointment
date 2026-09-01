package services

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"tebpardaz/shared/protocol"
)

// TestResultService looks up local lab-result PDFs and replies over WebSocket.
type TestResultService struct {
	PDFDir string
	Sender Sender
}

// NewTestResultService constructs a TestResultService.
// Inputs: pdfDir (folder of `{admission}-{password}.pdf` files), sender (WebSocket outbound).
// Output: pointer to TestResultService.
func NewTestResultService(pdfDir string, sender Sender) *TestResultService {
	return &TestResultService{PDFDir: strings.TrimSpace(pdfDir), Sender: sender}
}

// HandleRequest finds `{admission}-{password}.pdf` locally and sends test_result.response.
// Inputs: requestID (correlation), req with admission number and password (no clinic code).
// Output: error from filesystem read or WebSocket send.
func (s *TestResultService) HandleRequest(requestID string, req *protocol.TestResultRequest) error {
	resp := protocol.TestResultResponse{
		ClinicCode: req.ClinicCode,
		AdmissionNo: strings.TrimSpace(req.AdmissionNo),
		Found:       false,
	}

	admission := sanitizePDFToken(req.AdmissionNo)
	password := sanitizePDFToken(req.Password)
	if s.PDFDir == "" || admission == "" || password == "" {
		return s.sendResponse(requestID, resp)
	}

	filename := fmt.Sprintf("%d-%s-%s.pdf", req.ClinicCode, admission, password)
	path := filepath.Join(s.PDFDir, filename)
	info, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("test result stat %s: %v", path, err)
			resp.Message = "خطا در خواندن فایل جواب آزمایش"
		}
		return s.sendResponse(requestID, resp)
	}
	if info.IsDir() {
		return s.sendResponse(requestID, resp)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		log.Printf("test result read %s: %v", path, err)
		resp.Message = "خطا در خواندن فایل جواب آزمایش"
		return s.sendResponse(requestID, resp)
	}

	resp.Found = true
	resp.Filename = filename
	resp.PDFBase64 = base64.StdEncoding.EncodeToString(raw)
	return s.sendResponse(requestID, resp)
}

// sendResponse marshals and sends a test_result.response envelope.
func (s *TestResultService) sendResponse(requestID string, resp protocol.TestResultResponse) error {
	if s.Sender == nil {
		return fmt.Errorf("sender not ready")
	}
	env, err := protocol.NewEnvelope(protocol.TypeTestResultResponse, requestID, resp)
	if err != nil {
		return err
	}
	return s.Sender.SendEnvelope(env)
}

// sanitizePDFToken trims and rejects path-traversal / separator characters.
// Inputs: raw admission or password value.
// Output: cleaned token or empty string when unsafe/empty.
func sanitizePDFToken(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if strings.ContainsAny(s, `/\:`) || strings.Contains(s, "..") {
		return ""
	}
	return s
}
