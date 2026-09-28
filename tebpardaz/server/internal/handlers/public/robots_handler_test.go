package public

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tebpardaz/server/internal/seo"

	"github.com/gin-gonic/gin"
)

func TestServeRobotsUsesRequestHostAndHTTPS(t *testing.T) {
	trust, err := seo.NewProxyTrust([]string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	SetPublicProxyTrust(trust)
	t.Cleanup(func() { SetPublicProxyTrust(nil) })

	h := NewRobotsHandler()
	gin.SetMode(gin.TestMode)

	for _, host := range []string{"tebpardaz.ir", "chamranclinic.ir"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
		req.Host = host
		req.RemoteAddr = "127.0.0.1:443"
		req.Header.Set("X-Forwarded-Proto", "https")
		c.Request = req
		h.ServeRobots(c)
		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d", host, w.Code)
		}
		body := w.Body.String()
		want := "Sitemap: https://" + host + "/sitemap.xml\n"
		if !strings.Contains(body, want) {
			t.Fatalf("%s robots missing %q\n%s", host, want, body)
		}
		if strings.Contains(body, "http://") {
			t.Fatalf("%s robots contains http URL:\n%s", host, body)
		}
	}
}
