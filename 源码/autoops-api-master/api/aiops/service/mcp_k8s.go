package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	aiopsmodel "dodevops-api/api/aiops/model"
	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	k8sdao "dodevops-api/api/k8s/dao"
	k8smodel "dodevops-api/api/k8s/model"
	k8sservice "dodevops-api/api/k8s/service"
	systemauthz "dodevops-api/api/system/authz"
	"dodevops-api/common"
	"dodevops-api/common/config"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
)

const mcpK8sActionVersion = 1

type mcpK8sActionPayload struct {
	Version          int    `json:"v"`
	UserID           uint   `json:"uid"`
	Tool             string `json:"tool"`
	ClusterID        uint   `json:"clusterId"`
	Namespace        string `json:"namespace"`
	WorkloadType     string `json:"workloadType"`
	WorkloadName     string `json:"workloadName"`
	Container        string `json:"container,omitempty"`
	Image            string `json:"image,omitempty"`
	Replicas         int32  `json:"replicas,omitempty"`
	ExpectedReplicas *int32 `json:"expectedReplicas,omitempty"`
	Revision         int64  `json:"revision,omitempty"`
	IssuedAt         int64  `json:"iat"`
	ExpiresAt        int64  `json:"exp"`
	Nonce            string `json:"nonce"`
}

var mcpK8sUsedActions = struct {
	sync.Mutex
	items map[string]int64
}{items: map[string]int64{}}

var mcpK8sCaptureEngine = &gin.Engine{}

// ConfirmMCPAction executes only the operation encoded in a signed confirmation
// token. No target or command fields are accepted from the caller.
func ConfirmMCPAction(c *gin.Context, confirmationID string) (interface{}, error) {
	if _, err := decodeMCPK8sAction(confirmationID); err == nil {
		return NewMCPK8sActionService().Confirm(c, confirmationID)
	}
	payload, err := decodeMCPHostConfirmation(confirmationID)
	if err != nil {
		return nil, errors.New("confirmationId is invalid, expired or unsupported")
	}
	if payload.Tool == "ecs_request_cleanup_logs" {
		return NewMCPHostOperationService().ConfirmLogCleanup(c, confirmationID)
	}
	if payload.Tool != "ecs_request_restart_service" && payload.Tool != "ecs_request_start_service" && payload.Tool != "ecs_request_stop_service" {
		return nil, fmt.Errorf("confirmation tool %s is unsupported", payload.Tool)
	}
	parts := strings.SplitN(payload.Scope, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return nil, errors.New("confirmationId does not contain a valid service target")
	}
	return NewMCPHostOperationService().Invoke(c, payload.Tool, map[string]interface{}{
		"hostId": payload.HostID, "authId": payload.AuthID, "service": parts[1],
		"confirmed": true, "confirmationId": confirmationID,
	})
}

// CancelMCPAction invalidates a signed pending action without executing it.
func CancelMCPAction(c *gin.Context, confirmationID string) error {
	admin, err := mcpExecutionAdmin(c)
	if err != nil {
		return err
	}
	if payload, decodeErr := decodeMCPK8sAction(confirmationID); decodeErr == nil {
		if payload.UserID != admin.ID {
			return errors.New("confirmationId does not match the current user")
		}
		return consumeMCPK8sAction(confirmationID, payload.ExpiresAt)
	}
	payload, err := decodeMCPHostConfirmation(confirmationID)
	if err != nil {
		return errors.New("confirmationId is invalid, expired or unsupported")
	}
	if payload.UserID != admin.ID {
		return errors.New("confirmationId does not match the current user")
	}
	return consumeMCPHostConfirmation(confirmationID, payload.ExpiresAt)
}

// InvokeMCPK8sTool is the shared K8s adapter used by both the MCP endpoint
// and the internal assistant runtime.
func InvokeMCPK8sTool(c *gin.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	if args == nil {
		args = map[string]interface{}{}
	}
	switch toolName {
	case "k8s_list_clusters":
		return buildAssistantK8sContext(c, args)
	case "k8s_query_logs":
		var req aiopsmodel.K8sLogQueryRequest
		if err := decodeServiceMCPArguments(args, &req); err != nil {
			return nil, err
		}
		return NewLogAnalysisService().QueryK8sLogs(c, &req)
	case "k8s_diagnose_workload", "k8s_diagnose_pod":
		var req aiopsmodel.K8sPodAnalyzeRequest
		if err := decodeServiceMCPArguments(args, &req); err != nil {
			return nil, err
		}
		return NewLogAnalysisService().AnalyzeK8sPod(c, &req)
	case "k8s_diagnose_ingress":
		var req aiopsmodel.K8sIngressAnalyzeRequest
		if err := decodeServiceMCPArguments(args, &req); err != nil {
			return nil, err
		}
		return NewLogAnalysisService().AnalyzeK8sIngress(c, &req)
	default:
		return invokeMCPK8sAPI(c, toolName, args)
	}
}

func invokeMCPK8sAPI(c *gin.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	clusterID := mcpUintArgument(args, "clusterId")
	namespace := strings.TrimSpace(mcpStringArgument(args, "namespace"))
	page := intArg(args, "page", 1)
	pageSize := intArg(args, "pageSize", 50)
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	db := common.GetDB()
	return captureMCPK8sGinResult(c, func(inner *gin.Context) {
		switch toolName {
		case "k8s_check_cluster":
			k8sservice.NewKubeClusterService(db).GetClusterDetail(inner, clusterID)
		case "k8s_list_namespaces":
			k8sservice.NewK8sNamespaceService(db).GetNamespaces(inner, clusterID)
		case "k8s_list_nodes":
			k8sservice.NewK8sNodesService(db).GetNodes(inner, clusterID)
		case "k8s_get_node_status":
			query := inner.Request.URL.Query()
			query.Set("metrics", strconv.FormatBool(mcpK8sBoolArg(args, "includeMetrics", true)))
			inner.Request.URL.RawQuery = query.Encode()
			k8sservice.NewK8sNodesService(db).GetNodeDetail(inner, clusterID, mcpStringArgument(args, "node"))
		case "k8s_search_resources":
			query := inner.Request.URL.Query()
			query.Set("exactName", strconv.FormatBool(mcpK8sBoolArg(args, "exactName", false)))
			inner.Request.URL.RawQuery = query.Encode()
			rawKind := strings.ToLower(strings.TrimSpace(mcpStringArgument(args, "kind")))
			kind := normalizeMCPK8sKind(rawKind)
			if rawKind == "workload" || rawKind == "workloads" || rawKind == "all" {
				kind = "all"
			}
			if kind == "" {
				failMCPK8sGinResult(inner, "kind must be Workload, Deployment, StatefulSet, DaemonSet, Job, CronJob or Pod")
				return
			}
			k8sservice.NewK8sWorkloadService(db).SearchWorkloads(inner, clusterID, namespace, kind, mcpStringArgument(args, "keyword"), page, pageSize)
		case "k8s_get_resource":
			kind := strings.ToLower(strings.TrimSpace(mcpStringArgument(args, "kind")))
			name := mcpStringArgument(args, "name")
			switch kind {
			case "pod", "pods":
				k8sservice.NewK8sWorkloadService(db).GetPodDetail(inner, clusterID, namespace, name)
			case "service", "services", "svc":
				k8sservice.NewK8sServiceService(db).GetServiceDetail(inner, clusterID, namespace, name)
			default:
				workloadType := normalizeMCPK8sKind(kind)
				if workloadType == "" {
					failMCPK8sGinResult(inner, "unsupported read-only resource kind")
					return
				}
				k8sservice.NewK8sWorkloadService(db).GetWorkloadDetail(inner, clusterID, namespace, workloadType, name)
			}
		case "k8s_get_workload_status":
			k8sservice.NewK8sWorkloadService(db).GetWorkloadDetail(inner, clusterID, namespace, normalizeMCPK8sKind(mcpStringArgument(args, "workloadType")), mcpStringArgument(args, "workloadName"))
		case "k8s_get_workload_history":
			k8sservice.NewK8sWorkloadService(db).GetDeploymentHistory(inner, clusterID, namespace, mcpStringArgument(args, "workloadName"))
		case "k8s_get_pod_status":
			k8sservice.NewK8sWorkloadService(db).GetPodDetail(inner, clusterID, namespace, mcpStringArgument(args, "pod"))
		case "k8s_get_service_endpoints":
			k8sservice.NewK8sServiceService(db).GetServiceDetail(inner, clusterID, namespace, mcpStringArgument(args, "service"))
		case "k8s_query_events":
			eventService := k8sservice.NewK8sEventsService(db)
			if namespace == "" {
				eventService.GetClusterEvents(inner, clusterID, intArg(args, "limit", 100))
			} else {
				eventService.GetEvents(inner, clusterID, namespace, mcpStringArgument(args, "kind"), mcpStringArgument(args, "name"), intArg(args, "limit", 100))
			}
		case "k8s_query_metrics":
			workloadService := k8sservice.NewK8sWorkloadService(db)
			if pod := mcpStringArgument(args, "pod"); pod != "" {
				var query k8smodel.PodPrometheusMetricsQuery
				_ = decodeServiceMCPArguments(args, &query)
				workloadService.GetPodPrometheusMetrics(inner, clusterID, namespace, pod, query)
			} else if node := mcpStringArgument(args, "node"); node != "" {
				workloadService.GetNodeMetrics(inner, clusterID, node)
			} else if namespace != "" {
				workloadService.GetNamespaceMetrics(inner, clusterID, namespace)
			} else {
				failMCPK8sGinResult(inner, "one of pod, node or namespace is required")
			}
		default:
			failMCPK8sGinResult(inner, "unsupported K8s MCP tool: "+toolName)
		}
	})
}

func captureMCPK8sGinResult(parent *gin.Context, invoke func(*gin.Context)) (interface{}, error) {
	if parent == nil || parent.Request == nil {
		return nil, ErrMCPUnauthorized
	}
	recorder := httptest.NewRecorder()
	inner := gin.CreateTestContextOnly(recorder, mcpK8sCaptureEngine)
	inner.Request = parent.Request.Clone(parent.Request.Context())
	for key, value := range parent.Keys {
		inner.Set(key, value)
	}
	invoke(inner)
	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		return nil, fmt.Errorf("decode K8s service response: %w", err)
	}
	if envelope.Code != 200 {
		if strings.TrimSpace(envelope.Message) == "" {
			envelope.Message = "K8s service request failed"
		}
		return nil, errors.New(envelope.Message)
	}
	var data interface{}
	if len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func failMCPK8sGinResult(c *gin.Context, message string) {
	c.JSON(200, gin.H{"code": 400, "message": message, "data": gin.H{}})
}

func normalizeMCPK8sKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "deployment", "deployments":
		return "deployment"
	case "statefulset", "statefulsets":
		return "statefulset"
	case "daemonset", "daemonsets":
		return "daemonset"
	case "job", "jobs":
		return "job"
	case "cronjob", "cronjobs":
		return "cronjob"
	case "pod", "pods":
		return "pod"
	default:
		return ""
	}
}

func mcpK8sBoolArg(args map[string]interface{}, key string, fallback bool) bool {
	value, ok := args[key].(bool)
	if !ok {
		return fallback
	}
	return value
}

func decodeServiceMCPArguments(args map[string]interface{}, target interface{}) error {
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// MCPK8sActionService creates immutable previews and executes them only after
// the current JWT user confirms the signed confirmation id.
type MCPK8sActionService struct{}

func NewMCPK8sActionService() *MCPK8sActionService { return &MCPK8sActionService{} }

func (s *MCPK8sActionService) Preview(c *gin.Context, tool string, args map[string]interface{}) (interface{}, error) {
	admin, err := mcpExecutionAdmin(c)
	if err != nil {
		return nil, err
	}
	clusterID := mcpUintArgument(args, "clusterId")
	namespace := strings.TrimSpace(mcpStringArgument(args, "namespace"))
	workloadType := normalizeMCPK8sKind(mcpStringArgument(args, "workloadType"))
	workloadName := strings.TrimSpace(mcpStringArgument(args, "workloadName"))
	if clusterID == 0 || namespace == "" || workloadName == "" {
		return nil, errors.New("clusterId, namespace and workloadName are required")
	}
	if err := authorizeMCPK8sWrite(c, clusterID, namespace); err != nil {
		return nil, err
	}
	payload := mcpK8sActionPayload{Version: mcpK8sActionVersion, UserID: admin.ID, Tool: tool, ClusterID: clusterID, Namespace: namespace, WorkloadType: workloadType, WorkloadName: workloadName, IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(10 * time.Minute).Unix(), Nonce: fmt.Sprintf("%d-%d", time.Now().UnixNano(), admin.ID)}
	summary := ""
	switch tool {
	case "k8s_request_rollout_restart":
		if workloadType != "deployment" {
			return nil, errors.New("当前仅支持 Deployment 滚动重启")
		}
		summary = fmt.Sprintf("滚动重启 %s/%s", workloadType, workloadName)
	case "k8s_request_scale_workload":
		if workloadType != "deployment" {
			return nil, errors.New("当前仅支持 Deployment 扩缩容")
		}
		payload.Replicas = int32(mcpK8sActionIntArg(args, "replicas", -1))
		if payload.Replicas < 0 {
			return nil, errors.New("replicas must be zero or greater")
		}
		if _, exists := args["expectedReplicas"]; exists {
			expected := int32(mcpK8sActionIntArg(args, "expectedReplicas", -1))
			if expected < 0 {
				return nil, errors.New("expectedReplicas must be zero or greater")
			}
			payload.ExpectedReplicas = &expected
		}
		summary = fmt.Sprintf("将 Deployment/%s 副本数调整为 %d", workloadName, payload.Replicas)
	case "k8s_request_rollback_workload":
		payload.WorkloadType = "deployment"
		payload.Image = strings.TrimSpace(mcpStringArgument(args, "imageVersion"))
		history, historyErr := InvokeMCPK8sTool(c, "k8s_get_workload_history", map[string]interface{}{
			"clusterId": clusterID, "namespace": namespace, "workloadName": workloadName,
		})
		if historyErr != nil {
			return nil, fmt.Errorf("读取 Deployment 历史版本失败: %w", historyErr)
		}
		payload.Revision, payload.Image, err = mcpK8sResolveRollbackTarget(history, int64(mcpK8sActionIntArg(args, "revision", 0)), payload.Image)
		if err != nil {
			return nil, err
		}
		summary = fmt.Sprintf("回滚 Deployment/%s 到 revision %d", workloadName, payload.Revision)
		if payload.Image != "" {
			summary += "（镜像版本 " + payload.Image + "）"
		}
	case "k8s_request_update_image":
		if workloadType != "deployment" {
			return nil, errors.New("当前仅支持 Deployment 更新镜像")
		}
		payload.Container = strings.TrimSpace(mcpStringArgument(args, "container"))
		payload.Image = strings.TrimSpace(mcpStringArgument(args, "image"))
		if payload.Container == "" || payload.Image == "" {
			return nil, errors.New("container and image are required")
		}
		summary = fmt.Sprintf("更新 Deployment/%s 容器 %s 镜像为 %s", workloadName, payload.Container, payload.Image)
	default:
		return nil, ErrMCPToolUnsupported
	}
	confirmationID, err := signMCPK8sAction(payload)
	if err != nil {
		return nil, err
	}
	clusterName := mcpK8sClusterName(clusterID)
	expiresAt := time.Unix(payload.ExpiresAt, 0).Format(time.RFC3339)
	target := mcpK8sActionTarget(clusterID, clusterName, namespace, payload.WorkloadType, workloadName)
	change := gin.H{"summary": summary}
	switch tool {
	case "k8s_request_scale_workload":
		change["field"] = "replicas"
		change["toReplicas"] = payload.Replicas
		if payload.ExpectedReplicas != nil {
			change["fromReplicas"] = *payload.ExpectedReplicas
		}
	case "k8s_request_rollback_workload":
		change["field"] = "revision"
		change["toRevision"] = payload.Revision
		if payload.Image != "" {
			change["imageVersion"] = payload.Image
		}
	case "k8s_request_update_image":
		change["field"] = "image"
		change["container"] = payload.Container
		change["image"] = payload.Image
	}
	return gin.H{
		"action":               mcpK8sActionName(tool),
		"status":               "awaiting_confirmation",
		"phase":                "awaiting_confirmation",
		"message":              "等待用户确认，当前尚未执行",
		"target":               target,
		"change":               change,
		"confirmation":         gin.H{"id": confirmationID, "expiresAt": expiresAt, "endpoint": "/api/v1/aiops/mcp/actions/k8s/confirm"},
		"requiresConfirmation": true,
		"confirmed":            false,
		"executed":             false,
		"tool":                 tool,
		"clusterId":            clusterID,
		"clusterName":          clusterName,
		"namespace":            namespace,
		"workloadType":         payload.WorkloadType,
		"workloadName":         workloadName,
		"replicas":             payload.Replicas,
		"summary":              summary,
		"confirmationId":       confirmationID,
		"expiresAt":            payload.ExpiresAt,
		"confirmationEndpoint": "/api/v1/aiops/mcp/actions/k8s/confirm",
	}, nil
}

func mcpK8sResolveRollbackRevision(history interface{}, requestedRevision int64, imageVersion string) (int64, error) {
	revision, _, err := mcpK8sResolveRollbackTarget(history, requestedRevision, imageVersion)
	return revision, err
}

func mcpK8sResolveRollbackTarget(history interface{}, requestedRevision int64, imageVersion string) (int64, string, error) {
	values := assistantChatMap(history)
	revisions := assistantChatSlice(values["revisions"])
	if requestedRevision > 0 {
		for _, raw := range revisions {
			revision := assistantChatMap(raw)
			if int64(assistantChatNumber(revision["revision"])) == requestedRevision {
				return requestedRevision, assistantK8sImageVersions(assistantChatStringSlice(revision["images"])), nil
			}
		}
		return 0, "", fmt.Errorf("Deployment 历史中不存在 revision %d", requestedRevision)
	}

	imageVersion = strings.TrimSpace(imageVersion)
	if imageVersion == "" {
		return 0, "", errors.New("revision or imageVersion is required")
	}
	matches := make([]int64, 0, 1)
	for _, raw := range revisions {
		revision := assistantChatMap(raw)
		if !mcpK8sRevisionMatchesImage(revision, imageVersion) {
			continue
		}
		if value := int64(assistantChatNumber(revision["revision"])); value > 0 {
			matches = append(matches, value)
		}
	}
	if len(matches) == 0 {
		return 0, "", fmt.Errorf("Deployment 历史中没有找到镜像版本 %s", imageVersion)
	}
	if len(matches) > 1 {
		sort.Slice(matches, func(i, j int) bool { return matches[i] > matches[j] })
		return 0, "", fmt.Errorf("镜像版本 %s 匹配多个 revision：%v，请指定版本号", imageVersion, matches)
	}
	return matches[0], imageVersion, nil
}

func mcpK8sRevisionMatchesImage(revision map[string]interface{}, requested string) bool {
	requested = strings.TrimSpace(requested)
	for _, image := range assistantChatStringSlice(revision["images"]) {
		image = strings.TrimSpace(image)
		if image == requested || assistantK8sImageVersions([]string{image}) == requested {
			return true
		}
	}
	return false
}

func (s *MCPK8sActionService) Confirm(c *gin.Context, confirmationID string) (interface{}, error) {
	payload, err := decodeMCPK8sAction(confirmationID)
	if err != nil {
		return nil, err
	}
	admin, err := mcpExecutionAdmin(c)
	serviceUserID := uint(0)
	if config.Config != nil {
		serviceUserID = config.Config.MCP.UserID
	}
	if err != nil || (admin.ID != payload.UserID && (serviceUserID == 0 || payload.UserID != serviceUserID)) {
		return nil, errors.New("confirmationId does not match the current user")
	}
	if err := authorizeMCPK8sWrite(c, payload.ClusterID, payload.Namespace); err != nil {
		return nil, err
	}
	if err := consumeMCPK8sAction(confirmationID, payload.ExpiresAt); err != nil {
		return nil, err
	}
	data, err := s.execute(c, payload)
	if err != nil {
		return nil, err
	}
	triggeredAt := time.Now().Format(time.RFC3339)
	clusterName := mcpK8sClusterName(payload.ClusterID)
	target := mcpK8sActionTarget(payload.ClusterID, clusterName, payload.Namespace, payload.WorkloadType, payload.WorkloadName)
	response := gin.H{
		"action":       mcpK8sActionName(payload.Tool),
		"status":       "succeeded",
		"phase":        "succeeded",
		"message":      mcpK8sActionResultMessage(data, "变更已执行"),
		"target":       target,
		"triggeredAt":  triggeredAt,
		"confirmed":    true,
		"executed":     true,
		"tool":         payload.Tool,
		"clusterId":    payload.ClusterID,
		"clusterName":  clusterName,
		"namespace":    payload.Namespace,
		"workloadType": payload.WorkloadType,
		"workloadName": payload.WorkloadName,
		"data":         data,
	}
	if payload.Tool == "k8s_request_rollout_restart" {
		response["status"] = "triggered"
		response["phase"] = "progressing"
		response["message"] = mcpK8sActionResultMessage(data, "滚动重启已触发")
		response["rollout"] = mcpK8sInitialRollout(data)
		response["polling"] = mcpK8sPollingPolicy("progressing", assistantChatMap(data))
		response["statusEndpoint"] = fmt.Sprintf("/api/v1/aiops/mcp/actions/k8s/rollout-status?clusterId=%d&namespace=%s&workloadType=%s&workloadName=%s&generation=%v", payload.ClusterID, payload.Namespace, payload.WorkloadType, payload.WorkloadName, mcpK8sNumber(assistantChatMap(data), "generation"))
	} else if payload.Tool == "k8s_request_scale_workload" {
		rollout := mcpK8sInitialScaleProgress(data, payload.Replicas)
		phase, _ := rollout["phase"].(string)
		response["status"] = "triggered"
		if phase == "succeeded" {
			response["status"] = phase
		}
		response["phase"] = phase
		response["message"] = mcpK8sActionResultMessage(data, "副本数调整已触发")
		response["rollout"] = rollout
		response["polling"] = mcpK8sPollingPolicy(phase, assistantChatMap(data))
		response["statusEndpoint"] = fmt.Sprintf("/api/v1/aiops/mcp/actions/k8s/rollout-status?action=scale_workload&clusterId=%d&namespace=%s&workloadType=%s&workloadName=%s&generation=%v&targetReplicas=%d", payload.ClusterID, payload.Namespace, payload.WorkloadType, payload.WorkloadName, mcpK8sNumber(assistantChatMap(data), "generation"), payload.Replicas)
	} else if payload.Tool == "k8s_request_rollback_workload" || payload.Tool == "k8s_request_update_image" {
		action := mcpK8sActionName(payload.Tool)
		response["status"] = "triggered"
		response["phase"] = "progressing"
		response["message"] = mcpK8sActionTriggeredMessage(action, data)
		response["rollout"] = mcpK8sInitialRollout(data)
		response["polling"] = mcpK8sPollingPolicy("progressing", assistantChatMap(data))
		response["statusEndpoint"] = fmt.Sprintf("/api/v1/aiops/mcp/actions/k8s/rollout-status?action=%s&clusterId=%d&namespace=%s&workloadType=%s&workloadName=%s&generation=%v", action, payload.ClusterID, payload.Namespace, payload.WorkloadType, payload.WorkloadName, mcpK8sNumber(assistantChatMap(data), "generation"))
	}
	return response, nil
}

// RolloutStatus returns the current state of a previously triggered rollout.
// It is intentionally read-only and does not reuse confirmation credentials.
func (s *MCPK8sActionService) RolloutStatus(c *gin.Context, action string, clusterID uint, namespace, workloadType, workloadName string, operationGeneration int64, targetReplicas int32) (interface{}, error) {
	action = strings.TrimSpace(action)
	if action == "" {
		action = "rollout_restart"
	}
	if action != "rollout_restart" && action != "scale_workload" && action != "rollback_workload" && action != "update_image" && action != "configuration_update" {
		return nil, errors.New("不支持的 K8s 动作状态类型")
	}
	workloadType = normalizeMCPK8sKind(workloadType)
	namespace = strings.TrimSpace(namespace)
	workloadName = strings.TrimSpace(workloadName)
	if clusterID == 0 || namespace == "" || workloadName == "" {
		return nil, errors.New("clusterId, namespace and workloadName are required")
	}
	if workloadType != "deployment" {
		return nil, errors.New("当前仅支持查询 Deployment 动作状态")
	}
	if err := authorizeMCPK8sRead(c, clusterID, namespace); err != nil {
		return nil, err
	}
	data, err := captureMCPK8sGinResult(c, func(inner *gin.Context) {
		k8sservice.NewK8sWorkloadService(common.GetDB()).GetDeploymentRolloutStatus(inner, clusterID, namespace, workloadName)
	})
	if err != nil {
		return nil, err
	}
	rollout := mcpK8sRolloutFromStatus(data)
	phase, _ := rollout["phase"].(string)
	currentGeneration := mcpK8sInt64(assistantChatMap(data), "generation")
	if operationGeneration > 0 && currentGeneration > operationGeneration {
		phase = "superseded"
		rollout["phase"] = phase
		rollout["complete"] = false
	}
	if action == "scale_workload" && targetReplicas >= 0 {
		rollout["targetReplicas"] = targetReplicas
		if mcpK8sInt64(assistantChatMap(data), "desiredReplicas") != int64(targetReplicas) {
			phase = "superseded"
			rollout["phase"] = phase
			rollout["complete"] = false
		}
	}
	message := mcpK8sActionStatusMessage(action, phase)
	if message == "" {
		message = "已获取工作负载状态"
	}
	if phase == "progressing" {
		if progress, ok := rollout["progress"].(gin.H); ok {
			if label, ok := progress["label"].(string); ok && label != "" {
				message += "（" + label + "）"
			}
		}
	}
	response := gin.H{
		"action":     action,
		"status":     phase,
		"phase":      phase,
		"message":    message,
		"target":     mcpK8sActionTarget(clusterID, mcpK8sClusterName(clusterID), namespace, workloadType, workloadName),
		"rollout":    rollout,
		"observedAt": time.Now().Format(time.RFC3339),
		"polling":    mcpK8sPollingPolicy(phase, assistantChatMap(data)),
	}
	if phase == "failed" {
		if failure := mcpK8sRolloutFailure(data); failure != nil {
			response["failure"] = failure
			if reason, _ := failure["reason"].(string); reason != "" {
				message += "（" + reason + "）"
				response["message"] = message
			}
		}
	}
	if operationGeneration > 0 {
		response["operationGeneration"] = operationGeneration
	}
	return response, nil
}

func (s *MCPK8sActionService) execute(c *gin.Context, p *mcpK8sActionPayload) (interface{}, error) {
	return captureMCPK8sGinResult(c, func(inner *gin.Context) {
		workload := k8sservice.NewK8sWorkloadService(common.GetDB())
		switch p.Tool {
		case "k8s_request_rollout_restart":
			switch p.WorkloadType {
			case "deployment":
				workload.RestartDeployment(inner, p.ClusterID, p.Namespace, p.WorkloadName)
			case "statefulset":
				workload.RestartStatefulSet(inner, p.ClusterID, p.Namespace, p.WorkloadName)
			case "daemonset":
				workload.RestartDaemonSet(inner, p.ClusterID, p.Namespace, p.WorkloadName)
			default:
				failMCPK8sGinResult(inner, "unsupported workload type")
			}
		case "k8s_request_scale_workload":
			workload.ScaleDeployment(inner, p.ClusterID, p.Namespace, p.WorkloadName, &k8smodel.ScaleWorkloadRequest{Replicas: p.Replicas, ExpectedReplicas: p.ExpectedReplicas})
		case "k8s_request_rollback_workload":
			workload.RollbackDeployment(inner, p.ClusterID, p.Namespace, p.WorkloadName, &k8smodel.RollbackDeploymentRequest{ToRevision: p.Revision})
		case "k8s_request_update_image":
			workload.UpdateDeploymentImage(inner, p.ClusterID, p.Namespace, p.WorkloadName, p.Container, p.Image)
		default:
			failMCPK8sGinResult(inner, "unsupported K8s action")
		}
	})
}

func mcpK8sActionName(tool string) string {
	switch tool {
	case "k8s_request_rollout_restart":
		return "rollout_restart"
	case "k8s_request_scale_workload":
		return "scale_workload"
	case "k8s_request_rollback_workload":
		return "rollback_workload"
	case "k8s_request_update_image":
		return "update_image"
	default:
		return tool
	}
}

func mcpK8sClusterName(clusterID uint) string {
	cluster, err := k8sdao.NewKubeClusterDao(common.GetDB()).GetByID(clusterID)
	if err != nil || cluster == nil {
		return ""
	}
	return cluster.Name
}

func mcpK8sActionTarget(clusterID uint, clusterName, namespace, workloadType, workloadName string) gin.H {
	kind := workloadType
	if kind != "" {
		kind = strings.ToUpper(kind[:1]) + kind[1:]
	}
	target := gin.H{
		"clusterId": clusterID,
		"namespace": namespace,
		"kind":      kind,
		"name":      workloadName,
	}
	if strings.TrimSpace(clusterName) != "" {
		target["clusterName"] = clusterName
	}
	return target
}

func mcpK8sActionResultMessage(data interface{}, fallback string) string {
	if values, ok := data.(map[string]interface{}); ok {
		if message, ok := values["message"].(string); ok && strings.TrimSpace(message) != "" {
			return message
		}
	}
	return fallback
}

func mcpK8sInitialRollout(data interface{}) gin.H {
	values, _ := data.(map[string]interface{})
	return gin.H{
		"phase":                   "progressing",
		"desiredReplicas":         mcpK8sNumber(values, "desiredReplicas"),
		"currentReplicas":         mcpK8sNumber(values, "currentReplicas"),
		"updatedReplicas":         mcpK8sNumber(values, "updatedReplicas"),
		"readyReplicas":           mcpK8sNumber(values, "readyReplicas"),
		"availableReplicas":       mcpK8sNumber(values, "availableReplicas"),
		"unavailableReplicas":     mcpK8sNumber(values, "unavailableReplicas"),
		"generation":              mcpK8sNumber(values, "generation"),
		"observedGeneration":      mcpK8sNumber(values, "observedGeneration"),
		"progressDeadlineSeconds": mcpK8sProgressDeadlineSeconds(values),
		"progress":                mcpK8sReadinessProgress(values),
	}
}

func mcpK8sInitialScaleProgress(data interface{}, targetReplicas int32) gin.H {
	values := map[string]interface{}{}
	for key, value := range assistantChatMap(data) {
		values[key] = value
	}
	values["desiredReplicas"] = targetReplicas
	rollout := gin.H{
		"phase":                   "progressing",
		"complete":                false,
		"desiredReplicas":         targetReplicas,
		"targetReplicas":          targetReplicas,
		"currentReplicas":         mcpK8sNumber(values, "currentReplicas"),
		"updatedReplicas":         mcpK8sNumber(values, "updatedReplicas"),
		"readyReplicas":           mcpK8sNumber(values, "readyReplicas"),
		"availableReplicas":       mcpK8sNumber(values, "availableReplicas"),
		"unavailableReplicas":     mcpK8sNumber(values, "unavailableReplicas"),
		"generation":              mcpK8sNumber(values, "generation"),
		"observedGeneration":      mcpK8sNumber(values, "observedGeneration"),
		"progressDeadlineSeconds": mcpK8sProgressDeadlineSeconds(values),
		"progress":                mcpK8sReadinessProgress(values),
	}
	if mcpK8sInt64(values, "currentReplicas") == int64(targetReplicas) &&
		mcpK8sInt64(values, "readyReplicas") == int64(targetReplicas) &&
		mcpK8sInt64(values, "availableReplicas") == int64(targetReplicas) {
		rollout["phase"] = "succeeded"
		rollout["complete"] = true
	}
	return rollout
}

func mcpK8sActionStatusMessage(action, phase string) string {
	messages := map[string]map[string]string{
		"scale_workload": {
			"progressing": "工作负载正在调整副本",
			"succeeded":   "工作负载副本调整完成",
			"failed":      "工作负载副本调整异常",
			"paused":      "工作负载发布已暂停",
			"superseded":  "本次副本调整已被更新的工作负载变更覆盖",
		},
		"rollback_workload": {
			"progressing": "工作负载正在回滚",
			"succeeded":   "工作负载回滚完成",
			"failed":      "工作负载回滚异常",
			"paused":      "工作负载回滚已暂停",
			"superseded":  "本次回滚已被更新的工作负载变更覆盖",
		},
		"update_image": {
			"progressing": "工作负载正在更新镜像",
			"succeeded":   "工作负载镜像更新完成",
			"failed":      "工作负载镜像更新异常",
			"paused":      "工作负载发布已暂停",
			"superseded":  "本次镜像更新已被更新的工作负载变更覆盖",
		},
		"configuration_update": {
			"progressing": "工作负载正在应用配置变更",
			"succeeded":   "工作负载配置变更完成",
			"failed":      "工作负载配置变更异常",
			"paused":      "工作负载发布已暂停",
			"superseded":  "本次配置变更已被更新的工作负载变更覆盖",
		},
		"rollout_restart": {
			"progressing": "工作负载正在滚动重启",
			"succeeded":   "工作负载滚动重启完成",
			"failed":      "工作负载滚动重启异常",
			"paused":      "工作负载发布已暂停",
			"superseded":  "本次滚动重启已被更新的工作负载变更覆盖",
		},
	}
	return messages[action][phase]
}

func mcpK8sActionTriggeredMessage(action string, data interface{}) string {
	fallback := map[string]string{
		"rollback_workload": "工作负载回滚已触发",
		"update_image":      "工作负载镜像更新已触发",
	}[action]
	return mcpK8sActionResultMessage(data, fallback)
}

func mcpK8sRolloutFromStatus(data interface{}) gin.H {
	values, _ := data.(map[string]interface{})
	phase := "progressing"
	switch strings.ToLower(mcpK8sString(values, "status")) {
	case "complete":
		phase = "succeeded"
	case "failed":
		phase = "failed"
	case "paused":
		phase = "paused"
	}
	rollout := gin.H{
		"phase":                   phase,
		"complete":                phase == "succeeded",
		"desiredReplicas":         mcpK8sNumber(values, "desiredReplicas"),
		"currentReplicas":         mcpK8sNumber(values, "currentReplicas"),
		"updatedReplicas":         mcpK8sNumber(values, "updatedReplicas"),
		"readyReplicas":           mcpK8sNumber(values, "readyReplicas"),
		"availableReplicas":       mcpK8sNumber(values, "availableReplicas"),
		"unavailableReplicas":     mcpK8sNumber(values, "unavailableReplicas"),
		"generation":              mcpK8sNumber(values, "generation"),
		"observedGeneration":      mcpK8sNumber(values, "observedGeneration"),
		"currentRevision":         mcpK8sNumber(values, "currentRevision"),
		"progressDeadlineSeconds": mcpK8sProgressDeadlineSeconds(values),
		"progress":                mcpK8sReadinessProgress(values),
	}
	if conditions, ok := values["conditions"]; ok && conditions != nil {
		rollout["conditions"] = conditions
	}
	return rollout
}

func mcpK8sPollingPolicy(phase string, values map[string]interface{}) gin.H {
	phase = strings.ToLower(strings.TrimSpace(phase))
	shouldPoll := phase == "progressing"
	progressDeadlineSeconds := mcpK8sProgressDeadlineSeconds(values)
	autoObserveSeconds := progressDeadlineSeconds + 300
	if autoObserveSeconds < 600 {
		autoObserveSeconds = 600
	}
	if autoObserveSeconds > 900 {
		autoObserveSeconds = 900
	}
	return gin.H{
		"shouldPoll":                 shouldPoll,
		"terminal":                   !shouldPoll,
		"monitoringState":            map[bool]string{true: "active", false: "stopped"}[shouldPoll],
		"progressDeadlineSeconds":    progressDeadlineSeconds,
		"autoObserveLimitSeconds":    autoObserveSeconds,
		"stalledWarningAfterSeconds": 120,
		"progressToken":              mcpK8sProgressToken(values),
		"schedule": []gin.H{
			{"untilElapsedSeconds": 30, "intervalSeconds": 2},
			{"untilElapsedSeconds": 120, "intervalSeconds": 5},
			{"untilElapsedSeconds": autoObserveSeconds, "intervalSeconds": 10},
		},
		"observationLimitState":   "monitoring_paused",
		"observationLimitMessage": "自动监控已暂停，工作负载可能仍在更新。可重新检查或查看工作负载详情。",
		"stalledWarningMessage":   "滚动更新暂未推进，可能存在镜像拉取、调度或健康检查问题。系统仍在监控中。",
	}
}

func mcpK8sProgressDeadlineSeconds(values map[string]interface{}) int64 {
	deadline := mcpK8sInt64(values, "progressDeadline")
	if deadline <= 0 {
		deadline = mcpK8sInt64(values, "progressDeadlineSeconds")
	}
	if deadline <= 0 {
		deadline = 600
	}
	return deadline
}

func mcpK8sProgressToken(values map[string]interface{}) string {
	return fmt.Sprintf("%d:%d:%d:%d:%d:%d",
		mcpK8sInt64(values, "generation"),
		mcpK8sInt64(values, "observedGeneration"),
		mcpK8sInt64(values, "updatedReplicas"),
		mcpK8sInt64(values, "readyReplicas"),
		mcpK8sInt64(values, "availableReplicas"),
		mcpK8sInt64(values, "unavailableReplicas"),
	)
}

func mcpK8sRolloutFailure(data interface{}) gin.H {
	values := assistantChatMap(data)
	for _, raw := range assistantChatSlice(values["conditions"]) {
		condition := assistantChatMap(raw)
		if !strings.EqualFold(mcpK8sString(condition, "type"), "Progressing") || !strings.EqualFold(mcpK8sString(condition, "status"), "False") {
			continue
		}
		return gin.H{
			"reason":             mcpK8sString(condition, "reason"),
			"message":            mcpK8sString(condition, "message"),
			"lastTransitionTime": mcpK8sString(condition, "lastTransitionTime"),
		}
	}
	return nil
}

func mcpK8sNumber(values map[string]interface{}, key string) interface{} {
	if values == nil {
		return 0
	}
	if value, ok := values[key]; ok {
		return value
	}
	return 0
}

func mcpK8sString(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return value
}

func mcpK8sInt64(values map[string]interface{}, key string) int64 {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case float64:
		return int64(value)
	case float32:
		return int64(value)
	case int:
		return int64(value)
	case int32:
		return int64(value)
	case int64:
		return value
	case uint:
		return int64(value)
	case uint32:
		return int64(value)
	case uint64:
		return int64(value)
	default:
		return 0
	}
}

func mcpK8sReadinessProgress(values map[string]interface{}) gin.H {
	desired := mcpK8sInt64(values, "desiredReplicas")
	current := mcpK8sInt64(values, "currentReplicas")
	ready := mcpK8sInt64(values, "readyReplicas")
	total := current
	if desired > total {
		total = desired
	}
	percent := 100
	if total > 0 {
		percent = int(ready * 100 / total)
		if percent > 100 {
			percent = 100
		}
	}
	return gin.H{
		"current": ready,
		"total":   total,
		"percent": percent,
		"label":   fmt.Sprintf("%d / %d 就绪", ready, total),
		"mode":    "readiness",
	}
}

func authorizeMCPK8sWrite(c *gin.Context, clusterID uint, namespace string) error {
	isAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return err
	}
	if isAdmin {
		return nil
	}
	admin, err := mcpExecutionAdmin(c)
	if err != nil {
		return err
	}
	auth := cmdbdao.NewBastionAssetAuthorizationDao()
	ok, err := auth.CheckUserAssetAccess(admin.ID, cmdbmodel.AssetTypeK8SCluster, clusterID)
	if err != nil || !ok {
		return ErrMCPPermissionDenied
	}
	return nil
}

func authorizeMCPK8sRead(c *gin.Context, clusterID uint, namespace string) error {
	isAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return err
	}
	if isAdmin {
		return nil
	}
	admin, err := mcpExecutionAdmin(c)
	if err != nil {
		return err
	}
	auth := cmdbdao.NewBastionAssetAuthorizationDao()
	ok, err := auth.CheckUserAssetAccess(admin.ID, cmdbmodel.AssetTypeK8SCluster, clusterID)
	if err != nil || !ok {
		return ErrMCPPermissionDenied
	}
	return nil
}

func mcpK8sActionSecret() ([]byte, error) {
	if strings.TrimSpace(string(jwt.Secret)) == "" {
		return nil, errors.New("JWT secret is required for action confirmation signing")
	}
	return append([]byte("mcp-k8s-action-v1:"), jwt.Secret...), nil
}

func signMCPK8sAction(p mcpK8sActionPayload) (string, error) {
	b, _ := json.Marshal(p)
	enc := base64.RawURLEncoding.EncodeToString(b)
	secret, err := mcpK8sActionSecret()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(enc))
	return enc + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func decodeMCPK8sAction(id string) (*mcpK8sActionPayload, error) {
	parts := strings.Split(strings.TrimSpace(id), ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid confirmationId")
	}
	secret, err := mcpK8sActionSecret()
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid confirmationId signature")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, errors.New("confirmationId signature mismatch")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid confirmationId payload")
	}
	var p mcpK8sActionPayload
	if json.Unmarshal(raw, &p) != nil || p.Version != mcpK8sActionVersion || p.UserID == 0 || p.ClusterID == 0 || p.WorkloadName == "" || p.ExpiresAt < time.Now().Unix() {
		return nil, errors.New("confirmationId has expired or is invalid; request a new preview")
	}
	return &p, nil
}

func consumeMCPK8sAction(id string, expires int64) error {
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(id)))
	now := time.Now().Unix()
	mcpK8sUsedActions.Lock()
	defer mcpK8sUsedActions.Unlock()
	for k, v := range mcpK8sUsedActions.items {
		if v < now {
			delete(mcpK8sUsedActions.items, k)
		}
	}
	if _, ok := mcpK8sUsedActions.items[key]; ok {
		return errors.New("confirmationId has already been used")
	}
	mcpK8sUsedActions.items[key] = expires
	return nil
}

func mcpK8sActionIntArg(args map[string]interface{}, key string, fallback int) int {
	value, ok := args[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
	}
	return fallback
}
