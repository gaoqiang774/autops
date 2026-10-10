package task

import (
	appdb "dodevops-api/pkg/db"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestTaskRoutesRegistered(t *testing.T) {
	originalDB := appdb.Db
	appdb.Db = &gorm.DB{}
	t.Cleanup(func() { appdb.Db = originalDB })
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	api := engine.Group("/api/v1")
	RegisterTaskRoutes(api)
	RegisterWebSocketRoutes(api)

	want := map[string]bool{
		http.MethodPost + " /api/v1/template/add":                    false,
		http.MethodPost + " /api/v1/task/add":                        false,
		http.MethodPost + " /api/v1/taskjob/start":                   false,
		http.MethodPost + " /api/v1/task/ansible":                    false,
		http.MethodPost + " /api/v1/task/ansible/:id/stop/:work_id":  false,
		http.MethodGet + " /api/v1/ws/task/ansible/:id/log/:work_id": false,
	}
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Errorf("route not registered: %s", route)
		}
	}
}
