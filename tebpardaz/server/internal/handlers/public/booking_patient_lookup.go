package public

import (
	"net/http"

	"tebpardaz/server/internal/booking"
	"tebpardaz/server/internal/tenant"

	"github.com/gin-gonic/gin"
)

// LookupPatient اطلاعات بیمار ثبت‌شده را با کد ملی برای autofill برمی‌گرداند.
// ورودی: query national_id. خروجی: JSON {ok, patient?}.
func (h *BookingHandler) LookupPatient(c *gin.Context) {
	if _, ok := tenant.FromGin(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "دسترسی نامعتبر"})
		return
	}
	nationalID := digitsOnlyPublic(c.Query("national_id"))
	if !booking.IsValidIranianNationalID(nationalID) {
		c.JSON(http.StatusOK, gin.H{"ok": false, "found": false})
		return
	}
	if h.Bookings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "message": "سرویس در دسترس نیست"})
		return
	}
	dto, found := h.Bookings.LookupPatientByNationalID(nationalID)
	if !found || dto == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "found": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"found":   true,
		"patient": dto,
	})
}
