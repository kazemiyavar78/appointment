package public

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// headBodyDiscarder بدنه پاسخ HEAD را دور می‌ریزد و status و header را نگه می‌دارد.
// Gin برای HEAD بدنه را خودش حذف نمی‌کند؛ http.Server این کار را می‌کند،
// ولی ServeHTTP مستقیم (تست و برخی فراخوانی‌ها) بدنه را می‌نویسد.
type headBodyDiscarder struct {
	gin.ResponseWriter
}

// Write status را می‌فرستد و بدنه HEAD را نمی‌نویسد.
// ورودی: بایت‌های بدنه. خروجی: طول ورودی و خطای nil.
func (w headBodyDiscarder) Write(b []byte) (int, error) {
	w.ResponseWriter.WriteHeaderNow()
	return len(b), nil
}

// WriteString status را می‌فرستد و بدنه متنی HEAD را نمی‌نویسد.
// ورودی: رشته بدنه. خروجی: طول رشته و خطای nil.
func (w headBodyDiscarder) WriteString(s string) (int, error) {
	w.ResponseWriter.WriteHeaderNow()
	return len(s), nil
}

// discardHEADBody نویسنده پاسخ را برای متد HEAD عوض می‌کند تا بدنه ارسال نشود.
// ورودی: کانتکست Gin. خروجی: ندارد.
func discardHEADBody(c *gin.Context) {
	if c.Request != nil && c.Request.Method == http.MethodHead {
		c.Writer = headBodyDiscarder{ResponseWriter: c.Writer}
	}
	c.Next()
}

// GETAndHEAD یک مسیر عمومی را هم برای GET و هم برای HEAD با همان handler ثبت می‌کند.
// ورودی: گروه Gin، مسیر، و handlerها. خروجی: ندارد.
// POST و مسیرهایی که این تابع را صدا نزنند HEAD جداگانه نمی‌گیرند.
func GETAndHEAD(group *gin.RouterGroup, relativePath string, handlers ...gin.HandlerFunc) {
	group.GET(relativePath, handlers...)
	chain := make([]gin.HandlerFunc, 0, len(handlers)+1)
	chain = append(chain, discardHEADBody)
	chain = append(chain, handlers...)
	group.HEAD(relativePath, chain...)
}
