package public

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ClinicListHandler serves clinic listing pages (organ tenants).
type ClinicListHandler struct{}

// NewClinicListHandler constructs a ClinicListHandler.
func NewClinicListHandler() *ClinicListHandler {
	return &ClinicListHandler{}
}

// Get renders the clinic list.
// TODO: filter by city, load ClinicRepo, render clinic_list.templ
func (h *ClinicListHandler) Get(c *gin.Context) {
	c.Status(http.StatusNotImplemented)
}
