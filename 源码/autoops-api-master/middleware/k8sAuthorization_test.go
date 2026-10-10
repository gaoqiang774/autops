package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	systemmodel "dodevops-api/api/system/model"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

func TestRequireK8SAction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name        string
		action      K8SAction
		systemAdmin bool
		wantCode    int
		wantCalled  bool
	}{
		{name: "ordinary user can read", action: K8SActionRead, wantCode: http.StatusOK, wantCalled: true},
		{name: "ordinary user can open restricted terminal", action: K8SActionTerminal, wantCode: http.StatusOK, wantCalled: true},
		{name: "ordinary user can browse and download files", action: K8SActionFileRead, wantCode: http.StatusOK, wantCalled: true},
		{name: "ordinary user cannot read sensitive data", action: K8SActionSensitive, wantCode: http.StatusForbidden},
		{name: "ordinary user cannot execute", action: K8SActionExecute, wantCode: http.StatusForbidden},
		{name: "ordinary user cannot write", action: K8SActionWrite, wantCode: http.StatusForbidden},
		{name: "system administrator can write", action: K8SActionWrite, systemAdmin: true, wantCode: http.StatusOK, wantCalled: true},
		{name: "unknown action fails closed", action: K8SAction("unknown"), wantCode: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(constant.ContextKeyUserObj, &systemmodel.JwtAdmin{ID: 42})
				c.Set("aiops.system_admin_role", tt.systemAdmin)
				c.Next()
			})
			router.GET("/test", RequireK8SAction(tt.action), func(c *gin.Context) {
				called = true
				result.Success(c, nil)
			})

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/test", nil))

			var body result.Result
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != tt.wantCode {
				t.Fatalf("response code = %d, want %d; body=%s", body.Code, tt.wantCode, response.Body.String())
			}
			if called != tt.wantCalled {
				t.Fatalf("handler called = %v, want %v", called, tt.wantCalled)
			}
		})
	}
}

func TestRequireK8SActionRequiresAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/test", RequireK8SAction(K8SActionRead), func(c *gin.Context) {
		result.Success(c, nil)
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/test", nil))

	var body result.Result
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != http.StatusInternalServerError {
		t.Fatalf("response code = %d, want %d", body.Code, http.StatusInternalServerError)
	}
}

func TestCapabilityForK8SAction(t *testing.T) {
	tests := []struct {
		action    K8SAction
		namespace string
		want      string
	}{
		{action: K8SActionRead, want: "read"},
		{action: K8SActionTerminal, want: "terminal"},
		{action: K8SActionFileRead, want: "file_read"},
		{action: K8SActionExecute, want: "execute"},
		{action: K8SActionWrite, namespace: "default", want: "write_namespaced"},
		{action: K8SActionWrite, want: "write_cluster"},
	}
	for _, tt := range tests {
		if got := capabilityForK8SAction(tt.action, tt.namespace); got != tt.want {
			t.Errorf("capabilityForK8SAction(%q, %q) = %q, want %q", tt.action, tt.namespace, got, tt.want)
		}
	}
}

func TestK8SAuditDescriptionIncludesNonReadAuthorization(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	appendK8SAuditRecord(c, K8SActionTerminal, "terminal", 7, "dev", "allowed")
	description, ok := k8sAuditDescription(c)
	if !ok || description != "K8S操作: action=terminal capability=terminal cluster=7 namespace=dev authorization=allowed" {
		t.Fatalf("unexpected K8S audit description: %q, ok=%v", description, ok)
	}
	if !hasAuditableK8SAction(c) {
		t.Fatal("terminal action must be audited")
	}
}

func TestK8SReadActionDoesNotForceOperationLog(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	appendK8SAuditRecord(c, K8SActionRead, "read", 7, "dev", "allowed")
	if hasAuditableK8SAction(c) {
		t.Fatal("ordinary K8S read action must not force an operation log")
	}
}
