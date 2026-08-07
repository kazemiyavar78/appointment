package testresult

// Service fetches lab results via clinic WebSocket/LIS and serves cached copies.
// TODO: check cache, dispatch TestResultRequest, store in TestResultRepo.
type Service struct{}

// NewService constructs a test-result Service.
func NewService() *Service {
	return &Service{}
}

// Lookup returns a cached or freshly fetched test result for a patient.
// TODO: implement lookup + rate limiting.
func (s *Service) Lookup(_ string, _ string) error {
	return nil
}
