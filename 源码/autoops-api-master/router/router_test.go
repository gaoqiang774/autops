package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLoginUsesCanonicalAPIPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerAuthenticationRoutes(router.Group(apiRoutePrefix))

	var canonicalFound bool
	for _, route := range router.Routes() {
		if route.Method != http.MethodPost {
			continue
		}
		switch route.Path {
		case "/api/v1/login":
			canonicalFound = true
		case "/aiops/api/v1/login":
			t.Fatal("legacy global /aiops login route must not be registered")
		}
	}
	if !canonicalFound {
		t.Fatal("POST /api/v1/login route is not registered")
	}
}
