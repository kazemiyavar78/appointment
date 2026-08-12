package testresult

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultPDFDir is the on-disk folder of lab result PDFs served to the public site.
	DefaultPDFDir = "static/uploads/pdffile"
	// publicPDFPrefix is the URL path served by gin Static("/static", ...).
	publicPDFPrefix = "/static/uploads/pdffile"
)

// StoreResult is the outcome of persisting a clinic PDF for public download.
type StoreResult struct {
	Found       bool
	Filename    string
	DownloadURL string
}

// Service persists lab-result PDFs received from clinic clients for HTTP download.
type Service struct {
	PDFDir string
}

// NewService constructs a test-result Service.
// Inputs: optional absolute or relative PDF directory (empty uses DefaultPDFDir).
// Output: pointer to Service.
func NewService(pdfDir string) *Service {
	if strings.TrimSpace(pdfDir) == "" {
		pdfDir = DefaultPDFDir
	}
	return &Service{PDFDir: pdfDir}
}

// StoreFromBase64 writes `{clinicCode}-{admission}-{password}.pdf` under PDFDir.
// Inputs: clinic code (for unique public filename), admission, password, base64 PDF body.
// Output: StoreResult with download URL, or error on I/O / decode failure.
// Note: clinic code is only used for the server-side public filename; the clinic client
// looks up `{admission}-{password}.pdf` without the code.
func (s *Service) StoreFromBase64(clinicCode int, admissionNo, password, pdfBase64 string) (StoreResult, error) {
	admission := sanitizeToken(admissionNo)
	pwd := sanitizeToken(password)
	if clinicCode <= 0 || admission == "" || pwd == "" || strings.TrimSpace(pdfBase64) == "" {
		return StoreResult{Found: false}, nil
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pdfBase64))
	if err != nil {
		return StoreResult{}, fmt.Errorf("decode pdf base64: %w", err)
	}
	if len(raw) == 0 {
		return StoreResult{Found: false}, nil
	}

	if err := os.MkdirAll(s.PDFDir, 0o755); err != nil {
		return StoreResult{}, err
	}

	filename := fmt.Sprintf("%d-%s-%s.pdf", clinicCode, admission, pwd)
	path := filepath.Join(s.PDFDir, filename)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return StoreResult{}, err
	}

	return StoreResult{
		Found:       true,
		Filename:    filename,
		DownloadURL: publicPDFPrefix + "/" + filename,
	}, nil
}

// sanitizeToken trims and rejects path-traversal / separator characters in admission or password.
// Inputs: raw form value.
// Output: cleaned token or empty string when unsafe/empty.
func sanitizeToken(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if strings.ContainsAny(s, `/\:`) || strings.Contains(s, "..") {
		return ""
	}
	return s
}
