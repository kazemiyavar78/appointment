package public

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TestResultHandler serves lab result lookup form and results.
type TestResultHandler struct{}

// NewTestResultHandler constructs a TestResultHandler.
func NewTestResultHandler() *TestResultHandler {
	return &TestResultHandler{}
}

// GetForm renders the test result lookup form.
// TODO: render test_result_form.templ
func (h *TestResultHandler) GetForm(c *gin.Context) {
	c.Status(http.StatusNotImplemented)
}

// PostLookup handles result lookup submission.
// TODO: apply rate limit, call testresult.Service.Lookup
func (h *TestResultHandler) PostLookup(c *gin.Context) {
	c.Status(http.StatusNotImplemented)
}
