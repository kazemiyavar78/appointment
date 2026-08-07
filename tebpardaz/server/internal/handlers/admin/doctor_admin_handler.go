package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DoctorAdminHandler manages doctor admin listing and edits.
type DoctorAdminHandler struct{}

// NewDoctorAdminHandler constructs a DoctorAdminHandler.
func NewDoctorAdminHandler() *DoctorAdminHandler {
	return &DoctorAdminHandler{}
}

// List renders admin doctor management UI.
// TODO: require auth/RBAC, list doctors for tenant
func (h *DoctorAdminHandler) List(c *gin.Context) {
	c.Status(http.StatusNotImplemented)
}
