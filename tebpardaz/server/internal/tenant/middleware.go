package tenant

import (
	"net/http"

	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

const (
	// ContextKey is the gin.Context key for *tenant.Context.
	ContextKey = "tenant"
	// LayoutKey is the gin.Context key for constants.LayoutKind.
	LayoutKey = "layout"
)

// Middleware resolves the request host/path into a tenant Context and layout.
// Unknown hosts abort with 404 except when layout is explicitly platform.
// Inputs: resolver (domain/slug mapper).
// Output: gin.HandlerFunc that sets tenant + layout on the request context.
func Middleware(resolver *Resolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		if resolver == nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		host := c.Request.Host
		if xf := c.GetHeader("X-Forwarded-Host"); xf != "" {
			host = xf
		}

		tc, err := resolver.Resolve(host, c.Request.URL.Path)
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		
		c.Set(ContextKey, tc)
		c.Set(LayoutKey, tc.Layout)
		c.Next()
	}
}

// FromGin returns the resolved tenant Context from a gin request.
// Inputs: gin context.
// Output: tenant Context and true when present.
func FromGin(c *gin.Context) (*Context, bool) {
	v, ok := c.Get(ContextKey)
	if !ok {
		return nil, false
	}
	tc, ok := v.(*Context)
	return tc, ok
}

// LayoutFromGin returns which templ layout should be rendered.
// Inputs: gin context.
// Output: LayoutKind (platform/organ/private) and true when present.
func LayoutFromGin(c *gin.Context) (constants.LayoutKind, bool) {
	v, ok := c.Get(LayoutKey)
	if !ok {
		return "", false
	}
	layout, ok := v.(constants.LayoutKind)
	return layout, ok
}
