package service

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMCPK8sInitialScaleProgress(t *testing.T) {
	tests := []struct {
		name  string
		data  map[string]interface{}
		phase string
		label string
	}{
		{
			name: "progressing",
			data: map[string]interface{}{
				"currentReplicas": float64(2), "readyReplicas": float64(1), "availableReplicas": float64(1),
			},
			phase: "progressing", label: "1 / 2 就绪",
		},
		{
			name: "succeeded",
			data: map[string]interface{}{
				"currentReplicas": float64(2), "readyReplicas": float64(2), "availableReplicas": float64(2),
			},
			phase: "succeeded", label: "2 / 2 就绪",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rollout := mcpK8sInitialScaleProgress(tt.data, 2)
			if rollout["phase"] != tt.phase {
				t.Fatalf("phase = %v, want %s", rollout["phase"], tt.phase)
			}
			progress := assistantChatMap(rollout["progress"])
			if progress["label"] != tt.label {
				t.Fatalf("progress label = %v, want %s", progress["label"], tt.label)
			}
		})
	}
}

func TestMCPK8sActionStatusMessageUsesScaleWording(t *testing.T) {
	if got := mcpK8sActionStatusMessage("scale_workload", "progressing"); got != "工作负载正在调整副本" {
		t.Fatalf("scale status message = %q", got)
	}
	if got := mcpK8sActionStatusMessage("rollout_restart", "progressing"); got != "工作负载正在滚动重启" {
		t.Fatalf("restart status message = %q", got)
	}
	if got := mcpK8sActionStatusMessage("rollback_workload", "progressing"); got != "工作负载正在回滚" {
		t.Fatalf("rollback status message = %q", got)
	}
	if got := mcpK8sActionStatusMessage("update_image", "progressing"); got != "工作负载正在更新镜像" {
		t.Fatalf("image update status message = %q", got)
	}
	if got := mcpK8sActionStatusMessage("configuration_update", "progressing"); got != "工作负载正在应用配置变更" {
		t.Fatalf("configuration update status message = %q", got)
	}
}

func TestMCPK8sPollingPolicySeparatesObservationLimitFromFailure(t *testing.T) {
	policy := mcpK8sPollingPolicy("progressing", map[string]interface{}{
		"progressDeadline": float64(600), "generation": float64(8), "observedGeneration": float64(8),
		"updatedReplicas": float64(73), "readyReplicas": float64(70), "availableReplicas": float64(70), "unavailableReplicas": float64(5),
	})
	if policy["shouldPoll"] != true || policy["terminal"] != false {
		t.Fatalf("progressing polling policy = %#v", policy)
	}
	if policy["autoObserveLimitSeconds"] != int64(900) {
		t.Fatalf("autoObserveLimitSeconds = %v, want 900", policy["autoObserveLimitSeconds"])
	}
	if policy["observationLimitState"] != "monitoring_paused" {
		t.Fatalf("observation limit must pause monitoring without changing rollout phase: %#v", policy)
	}
	stages, ok := policy["schedule"].([]gin.H)
	if !ok || len(stages) != 3 || stages[0]["intervalSeconds"] != 2 || stages[2]["intervalSeconds"] != 10 {
		t.Fatalf("unexpected polling schedule: %#v", policy["schedule"])
	}
}

func TestMCPK8sRolloutFailureReturnsKubernetesReason(t *testing.T) {
	failure := mcpK8sRolloutFailure(map[string]interface{}{
		"conditions": []interface{}{map[string]interface{}{
			"type": "Progressing", "status": "False", "reason": "ProgressDeadlineExceeded", "message": "ReplicaSet has timed out progressing.",
		}},
	})
	if failure["reason"] != "ProgressDeadlineExceeded" {
		t.Fatalf("failure = %#v", failure)
	}
}

func TestMCPK8sResolveRollbackRevision(t *testing.T) {
	history := map[string]interface{}{
		"revisions": []interface{}{
			map[string]interface{}{"revision": float64(16), "images": []interface{}{"registry/zero-ai-kos:v2026-0812-000932"}},
			map[string]interface{}{"revision": float64(14), "images": []interface{}{"registry/zero-ai-kos:v2026-0805-165529"}},
		},
	}
	if got, err := mcpK8sResolveRollbackRevision(history, 14, ""); err != nil || got != 14 {
		t.Fatalf("resolve revision = %d, %v; want 14", got, err)
	}
	if got, err := mcpK8sResolveRollbackRevision(history, 0, "v2026-0805-165529"); err != nil || got != 14 {
		t.Fatalf("resolve image version = %d, %v; want 14", got, err)
	}
	if revision, imageVersion, err := mcpK8sResolveRollbackTarget(history, 14, ""); err != nil || revision != 14 || imageVersion != "v2026-0805-165529" {
		t.Fatalf("resolve target = revision %d, image %q, %v", revision, imageVersion, err)
	}
}

func TestCaptureMCPK8sGinResultDoesNotCreateDebugEngine(t *testing.T) {
	previousMode := gin.Mode()
	previousWriter := gin.DefaultWriter
	defer func() {
		gin.SetMode(previousMode)
		gin.DefaultWriter = previousWriter
	}()

	gin.SetMode(gin.DebugMode)
	var debugOutput bytes.Buffer
	gin.DefaultWriter = &debugOutput

	parent := gin.CreateTestContextOnly(httptest.NewRecorder(), &gin.Engine{})
	parent.Request = httptest.NewRequest("GET", "/internal-k8s-capture", nil)
	output, err := captureMCPK8sGinResult(parent, func(inner *gin.Context) {
		inner.JSON(200, gin.H{"code": 200, "message": "ok", "data": gin.H{"ready": true}})
	})
	if err != nil {
		t.Fatalf("captureMCPK8sGinResult() error = %v", err)
	}
	if ready, _ := assistantChatMap(output)["ready"].(bool); !ready {
		t.Fatalf("unexpected capture output: %#v", output)
	}
	if strings.Contains(debugOutput.String(), "Running in \"debug\" mode") {
		t.Fatalf("internal K8s capture emitted Gin debug warning: %s", debugOutput.String())
	}
}
