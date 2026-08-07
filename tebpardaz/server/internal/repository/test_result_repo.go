package repository

// TestResultRepo provides persistence for cached lab results.
// TODO: inject *gorm.DB; implement GetCache, UpsertCache, PurgeExpired.
type TestResultRepo struct{}

// NewTestResultRepo constructs a TestResultRepo.
func NewTestResultRepo() *TestResultRepo {
	return &TestResultRepo{}
}
