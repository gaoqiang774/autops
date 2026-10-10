package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dodevops-api/pkg/license"

	"github.com/gin-gonic/gin"
)

type staticLicenseStatus struct {
	status license.Status
}

func (s staticLicenseStatus) Status() license.Status { return s.status }

func TestRequireLicense(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		status     license.Status
		wantBody   string
		wantCalled bool
	}{
		{name: "licensed", status: license.Status{Licensed: true, State: license.StateValid}, wantCalled: true},
		{name: "missing", status: license.Status{State: license.StateMissing}, wantBody: "系统未授权"},
		{name: "expired", status: license.Status{State: license.StateExpired}, wantBody: "授权已过期"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			router := gin.New()
			router.GET("/restricted", requireLicense(staticLicenseStatus{status: tt.status}), func(c *gin.Context) {
				called = true
				c.Status(http.StatusNoContent)
			})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/restricted", nil))
			if called != tt.wantCalled {
				t.Fatalf("handler called = %t, want %t", called, tt.wantCalled)
			}
			if tt.wantBody != "" && !strings.Contains(response.Body.String(), tt.wantBody) {
				t.Fatalf("body = %q, want %q", response.Body.String(), tt.wantBody)
			}
		})
	}
}
