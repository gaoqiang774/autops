package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	aiopsdao "dodevops-api/api/aiops/dao"
	aiopsmodel "dodevops-api/api/aiops/model"
	agentskill "dodevops-api/pkg/agent/skill"

	"github.com/gin-gonic/gin"
)

type SkillExecuteRequest struct {
	SkillName string                 `json:"skillName" binding:"required"`
	Arguments map[string]interface{} `json:"arguments"`
	Context   SkillExecuteContext    `json:"context"`
}

type SkillExecuteContext struct {
	UserPrompt string `json:"userPrompt,omitempty"`
	TraceID    string `json:"traceId,omitempty"`
	Source     string `json:"source,omitempty"`
}

type SkillExecuteResult struct {
	ExecutionMode string                   `json:"executionMode"`
	RunID         uint64                   `json:"runId"`
	SkillName     string                   `json:"skillName"`
	DisplayName   string                   `json:"displayName,omitempty"`
	SkillGroup    string                   `json:"skillGroup"`
	Runtime       string                   `json:"runtime"`
	Status        string                   `json:"status"`
	RiskLevel     string                   `json:"riskLevel"`
	Readonly      bool                     `json:"readonly"`
	UsedTools     []string                 `json:"usedTools"`
	ToolResults   []SkillToolExecuteResult `json:"toolResults"`
	Summary       string                   `json:"summary"`
	StartedAt     string                   `json:"startedAt"`
	FinishedAt    string                   `json:"finishedAt"`
	DurationMS    int64                    `json:"durationMs"`
}

type SkillToolExecuteResult struct {
	ToolName   string      `json:"toolName"`
	Output     interface{} `json:"output,omitempty"`
	Error      string      `json:"error,omitempty"`
	DurationMS int64       `json:"durationMs"`
}

type SkillRuntimeService struct {
	catalog     *SkillCatalogService
	toolGateway *AgentToolGatewayService
	agentDao    *aiopsdao.AgentDao
}

func NewSkillRuntimeService() *SkillRuntimeService {
	return &SkillRuntimeService{
		catalog:     NewSkillCatalogService(),
		toolGateway: NewAgentToolGatewayService(),
		agentDao:    aiopsdao.NewAgentDao(),
	}
}

func (s *SkillRuntimeService) Execute(c *gin.Context, req *SkillExecuteRequest) (*SkillExecuteResult, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	req.SkillName = strings.TrimSpace(req.SkillName)
	if req.SkillName == "" {
		return nil, errors.New("skillName is required")
	}
	if req.Arguments == nil {
		req.Arguments = map[string]interface{}{}
	}

	doc, ok, err := s.catalog.Get(req.SkillName)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("skill %q is not registered", req.SkillName)
	}
	manifest := doc.Manifest
	if manifest.Status != agentskill.SkillStatusEnabled {
		return nil, fmt.Errorf("skill %q status is %q, only enabled skill can execute", manifest.Name, manifest.Status)
	}
	if manifest.Runtime != agentskill.SkillRuntimeDeclarative {
		return nil, fmt.Errorf("skill %q runtime is %q, skill runtime currently supports declarative skills only", manifest.Name, manifest.Runtime)
	}
	if !manifest.Readonly || manifest.RiskLevel != agentskill.SkillRiskReadonly {
		return nil, fmt.Errorf("skill %q is not readonly, blocked by skill runtime policy", manifest.Name)
	}
	if len(manifest.Tools) == 0 {
		return nil, fmt.Errorf("skill %q has no tools declared", manifest.Name)
	}

	started := time.Now()
	run := &aiopsmodel.AgentRun{
		RunType:   aiopsmodel.AgentRunTypeSkillRuntime,
		Status:    aiopsmodel.AgentRunStatusRunning,
		Summary:   "Skill runtime: " + manifest.Name,
		StartedAt: started,
		CreatedAt: started,
		UpdatedAt: started,
	}
	if err := s.agentDao.CreateRun(run); err != nil {
		return nil, err
	}

	executionArgs := copySkillArguments(req.Arguments)
	toolResults := make([]SkillToolExecuteResult, 0, len(manifest.Tools))
	usedTools := make([]string, 0, len(manifest.Tools))
	status := aiopsmodel.AgentRunStatusSucceeded
	var firstErr error
	for _, toolName := range manifest.Tools {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" {
			continue
		}
		toolArgs := buildSkillToolArguments(manifest, toolName, executionArgs)
		toolReq := &aiopsmodel.AgentToolInvokeRequest{
			RunID:     run.ID,
			ToolName:  toolName,
			Arguments: toolArgs,
			Context: aiopsmodel.AgentToolInvokeContext{
				AgentRuntime: aiopsmodel.AgentRunTypeSkillRuntime,
				UserPrompt:   req.Context.UserPrompt,
				TraceID:      req.Context.TraceID,
			},
		}
		toolStarted := time.Now()
		toolResult, toolErr := s.toolGateway.Invoke(c, toolReq)
		toolFinished := time.Now()
		item := SkillToolExecuteResult{
			ToolName:   toolName,
			DurationMS: toolFinished.Sub(toolStarted).Milliseconds(),
		}
		if toolErr != nil {
			item.Error = toolErr.Error()
			toolResults = append(toolResults, item)
			status = aiopsmodel.AgentRunStatusPartial
			firstErr = toolErr
			break
		}
		item.Output = toolResult.Output
		mergeSkillRuntimeOutput(executionArgs, toolName, toolResult.Output)
		toolResults = append(toolResults, item)
		usedTools = append(usedTools, toolName)
	}

	finished := time.Now()
	summary := buildSkillRuntimeSummary(manifest, usedTools, firstErr)
	finishMessage := ""
	if firstErr != nil {
		finishMessage = firstErr.Error()
	}
	_ = s.agentDao.FinishRun(run.ID, status, finishMessage, finished)

	return &SkillExecuteResult{
		ExecutionMode: aiopsmodel.AgentRunTypeSkillRuntime,
		RunID:         run.ID,
		SkillName:     manifest.Name,
		DisplayName:   manifest.DisplayName,
		SkillGroup:    manifest.SkillGroup,
		Runtime:       manifest.Runtime,
		Status:        manifest.Status,
		RiskLevel:     manifest.RiskLevel,
		Readonly:      manifest.Readonly,
		UsedTools:     usedTools,
		ToolResults:   toolResults,
		Summary:       summary,
		StartedAt:     started.Format("2006-01-02 15:04:05"),
		FinishedAt:    finished.Format("2006-01-02 15:04:05"),
		DurationMS:    finished.Sub(started).Milliseconds(),
	}, nil
}

func copySkillArguments(args map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for key, value := range args {
		result[key] = value
	}
	return result
}

func buildSkillToolArguments(manifest agentskill.Manifest, toolName string, args map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for key, value := range args {
		result[key] = value
	}

	target := strings.TrimSpace(fmt.Sprint(args["target"]))
	if target != "" && target != "<nil>" {
		switch toolName {
		case "get_cmdb_host", "query_host_metrics", "query_host_services":
			applySkillTarget(result, target)
		case "search_cmdb_hosts":
			if _, ok := result["keyword"]; !ok {
				result["keyword"] = target
			}
		case "knowledge_search":
			if _, ok := result["query"]; !ok {
				result["query"] = target
			}
		}
	}

	switch toolName {
	case "aiops_context_hosts", "aiops_context_database", "aiops_context_k8s":
		if _, ok := result["limit"]; !ok && manifest.Name != "k8s-context" {
			result["limit"] = 10
		}
	case "query_active_alerts":
		if _, ok := result["page"]; !ok {
			result["page"] = 1
		}
		if _, ok := result["pageSize"]; !ok {
			result["pageSize"] = 10
		}
	case "knowledge_search":
		if _, ok := result["query"]; !ok {
			if prompt := strings.TrimSpace(fmt.Sprint(args["question"])); prompt != "" && prompt != "<nil>" {
				result["query"] = prompt
			}
		}
		if _, ok := result["limit"]; !ok {
			result["limit"] = 5
		}
	}
	delete(result, "target")
	return result
}

func mergeSkillRuntimeOutput(args map[string]interface{}, toolName string, output interface{}) {
	data, ok := output.(map[string]interface{})
	if !ok {
		return
	}
	switch toolName {
	case "get_cmdb_host":
		mergeSkillHostContext(args, data)
	case "search_cmdb_hosts":
		items, ok := data["items"].([]map[string]interface{})
		if ok && len(items) == 1 {
			mergeSkillHostContext(args, items[0])
			return
		}
		rawItems, ok := data["items"].([]interface{})
		if ok && len(rawItems) == 1 {
			if item, ok := rawItems[0].(map[string]interface{}); ok {
				mergeSkillHostContext(args, item)
			}
		}
	}
}

func mergeSkillHostContext(args map[string]interface{}, host map[string]interface{}) {
	if _, ok := args["hostId"]; !ok {
		if id, ok := host["id"]; ok {
			args["hostId"] = id
		}
	}
	if _, ok := args["ip"]; !ok {
		for _, key := range []string{"privateIp", "sshIp", "publicIp"} {
			if value := strings.TrimSpace(fmt.Sprint(host[key])); value != "" && value != "<nil>" {
				args["ip"] = value
				break
			}
		}
	}
	if _, ok := args["hostname"]; !ok {
		for _, key := range []string{"hostName", "name"} {
			if value := strings.TrimSpace(fmt.Sprint(host[key])); value != "" && value != "<nil>" {
				args["hostname"] = value
				break
			}
		}
	}
}

func applySkillTarget(args map[string]interface{}, target string) {
	if _, ok := args["hostId"]; ok {
		return
	}
	if _, ok := args["ip"]; ok {
		return
	}
	if _, ok := args["hostname"]; ok {
		return
	}
	if _, ok := args["instance"]; ok {
		return
	}
	if id, ok := parsePositiveUintText(target); ok {
		args["hostId"] = id
		return
	}
	if looksLikeIP(target) {
		args["ip"] = target
		return
	}
	args["hostname"] = target
}

func parsePositiveUintText(value string) (uint, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	var parsed uint64
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		parsed = parsed*10 + uint64(ch-'0')
		if parsed > uint64(^uint(0)) {
			return 0, false
		}
	}
	if parsed == 0 {
		return 0, false
	}
	return uint(parsed), true
}

func buildSkillRuntimeSummary(manifest agentskill.Manifest, usedTools []string, err error) string {
	name := manifest.DisplayName
	if name == "" {
		name = manifest.Name
	}
	if err != nil {
		return fmt.Sprintf("Skill %s 通过 skill_runtime 执行，已调用 %d 个工具后停止：%s", name, len(usedTools), err.Error())
	}
	return fmt.Sprintf("Skill %s 通过 skill_runtime 执行完成，调用工具：%s", name, strings.Join(usedTools, ", "))
}
