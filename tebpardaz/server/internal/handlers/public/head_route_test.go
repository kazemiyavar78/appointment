package public

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGETAndHEADHomeStatusAndEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	pub := r.Group("/")
	GETAndHEAD(pub, "/", func(c *gin.Context) {
		c.String(http.StatusOK, "home-body")
	})
	pub.POST("/otp/send", func(c *gin.Context) {
		c.String(http.StatusOK, "otp")
	})

	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, httptest.NewRequest(http.MethodGet, "/", nil))
	if getW.Code != http.StatusOK {
		t.Fatalf("GET status = %d", getW.Code)
	}
	if getW.Body.String() != "home-body" {
		t.Fatalf("GET body = %q", getW.Body.String())
	}

	headW := httptest.NewRecorder()
	r.ServeHTTP(headW, httptest.NewRequest(http.MethodHead, "/", nil))
	if headW.Code != getW.Code {
		t.Fatalf("HEAD status = %d, want %d", headW.Code, getW.Code)
	}
	if headW.Body.Len() != 0 {
		t.Fatalf("HEAD body = %q", headW.Body.String())
	}

	headPost := httptest.NewRecorder()
	r.ServeHTTP(headPost, httptest.NewRequest(http.MethodHead, "/otp/send", nil))
	if headPost.Code != http.StatusNotFound {
		t.Fatalf("HEAD on POST route status = %d, want 404", headPost.Code)
	}
}
