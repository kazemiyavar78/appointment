package iprestrict

import (
	"log"
	"net/http"
	"strings"
	"time"

	"tebpardaz/server/internal/models"

	"github.com/gin-gonic/gin"
)

// Evaluator قانون ذخیره‌شده بلاک یا rate limit را اعمال می‌کند.
type Evaluator interface {
	Evaluate(ip, scope string, now time.Time) (allowed bool, kind string, err error)
}

// Middleware درخواست آی‌پی بلاک‌شده یا بالاتر از سقف rate limit را رد می‌کند.
// خطای دیتابیس لاگ می‌شود و درخواست ادامه پیدا می‌کند.
// ورودی: evaluator؛ nil یعنی بررسی خاموش است. خروجی: میان‌افزار Gin.
func Middleware(evaluator Evaluator) gin.HandlerFunc {
	return func(c *gin.Context) {
		if evaluator == nil || c.Request.Method == http.MethodOptions || skipPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		allowed, kind, err := evaluator.Evaluate(c.ClientIP(), models.IPRestrictionScopeGlobal, time.Now())
		if err != nil {
			log.Printf("ip restriction: %v", err)
			c.Next()
			return
		}
		if allowed {
			c.Next()
			return
		}
		status := http.StatusForbidden
		msg := "دسترسی این IP مسدود شده است"
		if kind == models.IPRestrictionKindRateLimit {
			status = http.StatusTooManyRequests
			msg = "تعداد درخواست‌ها بیش از حد مجاز است"
		}
		c.AbortWithStatusJSON(status, gin.H{"error": msg})
	}
}

// skipPath مسیرهای ثابت مثل فایل استاتیک و سلامت سرویس را از بررسی آی‌پی کنار می‌گذارد.
// ورودی: path. خروجی: true اگر نباید بررسی شود.
func skipPath(path string) bool {
	p := strings.ToLower(path)
	switch {
	case p == "/healthz" || p == "/favicon.ico" || p == "/robots.txt":
		return true
	case p == "/static" || strings.HasPrefix(p, "/static/"):
		return true
	default:
		return false
	}
}
