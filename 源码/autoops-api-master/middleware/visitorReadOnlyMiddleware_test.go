package middleware

import (
	"dodevops-api/api/system/model"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestVisitorReadOnlyMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		username   string
		method     string
		websocket  bool
		wantCalled bool
		wantCode   int
	}{
		{name: "visitor can read", username: "test", method: http.MethodGet, wantCalled: true},
		{name: "visitor cannot create", username: "test", method: http.MethodPost, wantCode: int(result.ApiCode.TESTUSERFORBIDDEN)},
		{name: "visitor cannot update", username: "TEST", method: http.MethodPut, wantCode: int(result.ApiCode.TESTUSERFORBIDDEN)},
		{name: "visitor cannot patch", username: "test", method: http.MethodPatch, wantCode: int(result.ApiCode.TESTUSERFORBIDDEN)},
		{name: "visitor cannot delete", username: "test", method: http.MethodDelete, wantCode: int(result.ApiCode.TESTUSERFORBIDDEN)},
		{name: "visitor cannot open websocket", username: "test", method: http.MethodGet, websocket: true, wantCode: int(result.ApiCode.TESTUSERFORBIDDEN)},
		{name: "visitor cannot execute command over get", username: "test", method: http.MethodGet, wantCode: int(result.ApiCode.TESTUSERFORBIDDEN)},
		{name: "admin can write", username: "admin", method: http.MethodPost, wantCalled: true},
		{name: "ordinary user keeps existing permissions", username: "operator", method: http.MethodDelete, wantCalled: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(constant.ContextKeyUserObj, &model.JwtAdmin{ID: 12, Username: tt.username})
				c.Next()
			})
			router.Use(VisitorReadOnlyMiddleware())
			path := "/resource"
			requestPath := path
			if tt.name == "visitor cannot execute command over get" {
				path = "/cmdb/hostssh/command/:id"
				requestPath = "/cmdb/hostssh/command/1"
			}
			router.Handle(tt.method, path, func(c *gin.Context) {
				called = true
				c.Status(http.StatusNoContent)
			})

			req := httptest.NewRequest(tt.method, requestPath, nil)
			if tt.websocket {
				req.Header.Set("Upgrade", "websocket")
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)

			if called != tt.wantCalled {
				t.Fatalf("handler called = %v, want %v", called, tt.wantCalled)
			}
			if tt.wantCode == 0 {
				return
			}

			var body result.Result
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d", body.Code, tt.wantCode)
			}
			if body.Message != "游客无权限！" {
				t.Fatalf("message = %q", body.Message)
			}
		})
	}
}
