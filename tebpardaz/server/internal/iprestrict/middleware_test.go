package iprestrict

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tebpardaz/server/internal/models"

	"github.com/gin-gonic/gin"
)

// stubEval نتیجه ثابت بررسی آی‌پی را برای تست میان‌افزار برمی‌گرداند.
type stubEval struct {
	allowed bool
	kind    string
}

// Evaluate نتیجه از پیش تعیین‌شده را برای scope سراسری برمی‌گرداند.
// ورودی: ip، scope، now. خروجی: allowed، kind، خطای nil.
func (s stubEval) Evaluate(ip, scope string, now time.Time) (bool, string, error) {
	if scope != models.IPRestrictionScopeGlobal {
		return true, "", nil
	}
	return s.allowed, s.kind, nil
}

// TestMiddlewareBlocksIP پاسخ ۴۰۳ برای آی‌پی بلاک‌شده را بررسی می‌کند.
func TestMiddlewareBlocksIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(stubEval{allowed: false, kind: models.IPRestrictionKindBlock}))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d", w.Code)
	}
}

// TestMiddlewareRateLimit پاسخ ۴۲۹ برای عبور از سقف را بررسی می‌کند.
func TestMiddlewareRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(stubEval{allowed: false, kind: models.IPRestrictionKindRateLimit}))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", w.Code)
	}
}

// TestMiddlewareSkipsStatic رد نشدن فایل استاتیک را بررسی می‌کند.
func TestMiddlewareSkipsStatic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(stubEval{allowed: false, kind: models.IPRestrictionKindBlock}))
	r.GET("/static/app.js", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/static/app.js", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}
