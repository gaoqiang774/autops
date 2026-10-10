package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	aiopsdao "dodevops-api/api/aiops/dao"
	aiopsmodel "dodevops-api/api/aiops/model"
	alertdao "dodevops-api/api/alertmanager/dao"
	alertmodel "dodevops-api/api/alertmanager/model"
	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	cmdbservice "dodevops-api/api/cmdb/service"
	systemauthz "dodevops-api/api/system/authz"
	systemdao "dodevops-api/api/system/dao"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
)

const (
	defaultToolGatewayPageSize = 10
	maxToolGatewayPageSize     = 50
	defaultToolMetricMinutes   = 5
	maxToolMetricMinutes       = 24 * 60
	defaultToolLogTailLines    = 500
	maxToolLogTailLines        = 5000
)

type AgentToolGatewayService struct {
	agentDao *aiopsdao.AgentDao
	hostDao  cmdbdao.CmdbHostDao
	agentMon cmdbdao.AgentDao
	alertDao *alertdao.AlertManagerDao
	monSvc   cmdbservice.MonitorServiceInterface
	logSvc   *LogAnalysisService
	tools    map[string]aiopsmodel.AgentToolDefinition
}

func NewAgentToolGatewayService() *AgentToolGatewayService {
	return &AgentToolGatewayService{
		agentDao: aiopsdao.NewAgentDao(),
		hostDao:  cmdbdao.NewCmdbHostDao(),
		agentMon: cmdbdao.NewAgentDao(),
		alertDao: alertdao.NewAlertManagerDao(),
		monSvc:   cmdbservice.NewMonitorService(),
		logSvc:   NewLogAnalysisService(),
		tools:    agentToolDefinitions(),
	}
}

func (s *AgentToolGatewayService) ListTools() []aiopsmodel.AgentToolDefinition {
	names := []string{
		"get_cmdb_host",
		"search_cmdb_hosts",
		"get_agent_status",
		"query_active_alerts",
		"query_alert_detail",
		"query_host_metrics",
		"query_host_services",
		"query_host_logs",
		"aiops_context_hosts",
		"aiops_context_database",
		"aiops_context_k8s",
		"knowledge_search",
	}
	result := make([]aiopsmodel.AgentToolDefinition, 0, len(names))
	for _, name := range names {
		result = append(result, s.tools[name])
	}
	return result
}

func (s *AgentToolGatewayService) ListEnabledTools() ([]aiopsmodel.AgentToolDefinition, error) {
	servers, err := NewMCPCatalogService().ListServers("")
	if err != nil {
		return nil, err
	}
	result := make([]aiopsmodel.AgentToolDefinition, 0)
	for _, tool := range s.ListTools() {
		if _, err := authorizeGatewayMCPTool(servers, tool.Name, ""); err == nil {
			result = append(result, tool)
		}
	}
	return result, nil
}

func (s *AgentToolGatewayService) Invoke(c *gin.Context, req *aiopsmodel.AgentToolInvokeRequest) (*aiopsmodel.AgentToolInvokeResult, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	req.ToolName = strings.TrimSpace(req.ToolName)
	if req.ToolName == "" {
		return nil, errors.New("toolName is required")
	}
	if req.Arguments == nil {
		req.Arguments = map[string]interface{}{}
	}

	tool, ok := s.tools[req.ToolName]
	if !ok {
		return nil, fmt.Errorf("tool %q is not registered", req.ToolName)
	}
	policyTool, policyErr := NewMCPCatalogService().AuthorizeGatewayTool(req.ToolName, req.Context.MCPToolName)
	if policyErr != nil {
		return nil, policyErr
	}
	internalInvocation := strings.TrimSpace(req.Context.MCPToolName) == ""
	validationSchema := policyTool.InputSchema
	if internalInvocation {
		validationSchema = tool.InputSchema
	}
	if err := ValidateMCPArguments(validationSchema, req.Arguments); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if err := validateMCPEnvironment(policyTool.AllowedEnvironments, req.Arguments); err != nil {
		return nil, err
	}
	if internalInvocation {
		admin, err := mcpExecutionAdmin(c)
		if err != nil {
			return nil, err
		}
		if !allowMCPRate(policyTool.Name, admin.ID, policyTool.RateLimitPerMinute, time.Now()) {
			return nil, ErrMCPRateLimited
		}
	}
	if err := authorizeAgentGatewayScope(c, policyTool, req.ToolName, req.Arguments); err != nil {
		return nil, err
	}

	decision := evaluateAgentToolPolicy(tool)
	if decision.Decision != aiopsmodel.AgentToolDecisionAllow {
		return nil, errors.New(decision.Reason)
	}

	now := time.Now()
	runID := req.RunID
	if runID == 0 {
		run := &aiopsmodel.AgentRun{
			RunType:   aiopsmodel.AgentRunTypeToolInvoke,
			Status:    aiopsmodel.AgentRunStatusRunning,
			Summary:   "Agent tool invocation: " + req.ToolName,
			StartedAt: now,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.agentDao.CreateRun(run); err != nil {
			return nil, err
		}
		runID = run.ID
	}

	started := time.Now()
	var requestContext context.Context
	var cancel context.CancelFunc
	var originalRequest = c.Request
	if internalInvocation {
		timeout := time.Duration(policyTool.TimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		requestContext, cancel = context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(requestContext)
	}
	var output interface{}
	var err error
	switch req.ToolName {
	case "get_cmdb_host":
		output, err = s.invokeGetCMDBHost(c, req.Arguments, policyTool.Name)
	case "search_cmdb_hosts":
		output, err = s.invokeSearchCMDBHosts(c, req.Arguments)
	case "get_agent_status":
		output, err = s.invokeGetAgentStatus(req.Arguments)
	case "query_active_alerts":
		output, err = s.invokeQueryActiveAlerts(c, req.Arguments)
	case "query_alert_detail":
		output, err = s.invokeQueryAlertDetail(req.Arguments)
	case "query_host_metrics":
		output, err = s.invokeQueryHostMetrics(req.Arguments, policyTool.Name)
	case "query_host_services":
		output, err = s.invokeQueryHostServices(req.Arguments, policyTool.Name)
	case "query_host_logs":
		output, err = s.invokeQueryHostLogs(c, req.Arguments)
	case "aiops_context_hosts":
		output, err = s.invokeAIOpsHostContext(c, req.Arguments)
	case "aiops_context_database":
		output, err = s.invokeAIOpsDatabaseContext(c, req.Arguments)
	case "aiops_context_k8s":
		output, err = s.invokeAIOpsK8sContext(c, req.Arguments)
	case "knowledge_search":
		output, err = s.invokeKnowledgeSearch(req.Arguments)
	default:
		err = fmt.Errorf("tool %q has no handler", req.ToolName)
	}
	if internalInvocation {
		c.Request = originalRequest
		if err == nil {
			err = requestContext.Err()
		}
	}
	if err == nil {
		output = LimitMCPResultRows(RedactMCPValue(output), policyTool.MaxResultRows)
		payload, marshalErr := json.Marshal(output)
		if marshalErr != nil {
			err = fmt.Errorf("marshal Agent Tool result: %w", marshalErr)
		} else if policyTool.MaxResultBytes > 0 && len(payload) > policyTool.MaxResultBytes {
			err = fmt.Errorf("%w: result is %d bytes, limit is %d", ErrMCPResultTooLarge, len(payload), policyTool.MaxResultBytes)
		}
	}
	finished := time.Now()

	step := aiopsmodel.AgentStep{
		StepName:   "调用工具 " + req.ToolName,
		ToolName:   req.ToolName,
		Status:     aiopsmodel.AgentStepStatusSucceeded,
		InputJSON:  compactToolJSON(req.Arguments),
		StartedAt:  started,
		FinishedAt: &finished,
		DurationMS: finished.Sub(started).Milliseconds(),
	}
	if err != nil {
		step.Status = aiopsmodel.AgentStepStatusFailed
		step.Error = err.Error()
		_ = s.agentDao.AppendRunStep(runID, step)
		_ = s.agentDao.FinishRun(runID, aiopsmodel.AgentRunStatusFailed, err.Error(), finished)
		return nil, err
	}
	step.OutputJSON = compactToolJSON(output)
	_ = s.agentDao.AppendRunStep(runID, step)

	evidence := buildToolEvidence(req.ToolName, output)
	if evidence != nil {
		_ = s.agentDao.CreateEvidence(&aiopsmodel.AgentEvidence{
			RunID:     runID,
			Type:      evidence.Type,
			Title:     evidence.Title,
			Reference: evidence.Reference,
			Content:   evidence.Content,
			RawJSON:   compactToolJSON(output),
		})
	}

	if req.RunID == 0 {
		_ = s.agentDao.FinishRun(runID, aiopsmodel.AgentRunStatusSucceeded, "", finished)
	}

	return &aiopsmodel.AgentToolInvokeResult{
		RunID:      runID,
		ToolName:   req.ToolName,
		Decision:   decision,
		Output:     output,
		Evidence:   evidence,
		StartedAt:  started.Format("2006-01-02 15:04:05"),
		FinishedAt: finished.Format("2006-01-02 15:04:05"),
		DurationMS: finished.Sub(started).Milliseconds(),
	}, nil
}

func agentToolDefinitions() map[string]aiopsmodel.AgentToolDefinition {
	return map[string]aiopsmodel.AgentToolDefinition{
		"get_cmdb_host": {
			Name:        "get_cmdb_host",
			Description: "当用户提供主机名、IP、实例名或 hostId，需要先定位资产时使用。按 hostId、ip、hostname 或 instance 查询 CMDB 主机上下文。",
			RiskLevel:   aiopsmodel.AgentToolRiskReadonly,
			Readonly:    true,
			InputSchema: objectSchema(map[string]interface{}{
				"hostId":   "number",
				"ip":       "string",
				"hostname": "string",
				"instance": "string",
			}, []string{}),
		},
		"search_cmdb_hosts": {
			Name:        "search_cmdb_hosts",
			Description: "当用户只提供 IP、主机名、环境关键词或模糊资产线索，但没有明确 hostId 时使用。按关键词搜索 CMDB 主机。",
			RiskLevel:   aiopsmodel.AgentToolRiskReadonly,
			Readonly:    true,
			InputSchema: objectSchema(map[string]interface{}{
				"keyword": "string",
				"limit":   "number",
			}, []string{"keyword"}),
		},
		"get_agent_status": {
			Name:        "get_agent_status",
			Description: "当用户询问采集 Agent 是否安装、是否在线、监控覆盖是否正常时使用。查询指定主机的采集 Agent 状态。",
			RiskLevel:   aiopsmodel.AgentToolRiskReadonly,
			Readonly:    true,
			InputSchema: objectSchema(map[string]interface{}{
				"hostId": "number",
			}, []string{"hostId"}),
		},
		"query_active_alerts": {
			Name:        "query_active_alerts",
			Description: "当用户询问当前告警、故障、异常、报警数量或某类严重级别告警时使用。分页查询当前活跃告警。",
			RiskLevel:   aiopsmodel.AgentToolRiskReadonly,
			Readonly:    true,
			InputSchema: objectSchema(map[string]interface{}{
				"businessId": "string",
				"severity":   "string",
				"keyword":    "string",
				"page":       "number",
				"pageSize":   "number",
			}, []string{}),
		},
		"query_alert_detail": {
			Name:        "query_alert_detail",
			Description: "当已经知道 alertId 或 fingerprint，并需要查看某条告警上下文时使用。按 alertId 或 fingerprint 查询告警详情。",
			RiskLevel:   aiopsmodel.AgentToolRiskReadonly,
			Readonly:    true,
			InputSchema: objectSchema(map[string]interface{}{
				"alertId":     "number",
				"fingerprint": "string",
			}, []string{}),
		},
		"query_host_metrics": {
			Name:        "query_host_metrics",
			Description: "当用户想分析某台主机的 CPU、内存、磁盘、负载、资源水位或容量压力时使用。可以传 hostId，也可以传 ip、hostname 或 instance，工具会先解析主机再查询指标。",
			RiskLevel:   aiopsmodel.AgentToolRiskReadonly,
			Readonly:    true,
			InputSchema: objectSchema(map[string]interface{}{
				"hostId":   "number",
				"ip":       "string",
				"hostname": "string",
				"instance": "string",
				"minutes":  "number",
				"step":     "string",
			}, []string{}),
		},
		"query_host_services": {
			Name:        "query_host_services",
			Description: "当用户询问某台主机部署了什么服务、运行了哪些服务、Top 进程或服务列表时使用。只读查询主机 Top 进程并推断疑似服务。",
			RiskLevel:   aiopsmodel.AgentToolRiskReadonly,
			Readonly:    true,
			InputSchema: objectSchema(map[string]interface{}{
				"hostId":   "number",
				"ip":       "string",
				"hostname": "string",
				"instance": "string",
			}, []string{}),
		},
		"query_host_logs": {
			Name:        "query_host_logs",
			Description: "当用户要求查看日志、错误、异常堆栈或最近日志内容时使用。只读查询指定主机日志尾部内容，需要 hostId 和日志路径。",
			RiskLevel:   aiopsmodel.AgentToolRiskMedium,
			Readonly:    true,
			InputSchema: objectSchema(map[string]interface{}{
				"hostId":    "number",
				"authId":    "number",
				"path":      "string",
				"tailLines": "number",
				"limit":     "number",
				"keyword":   "string",
				"regex":     "string",
				"levels":    "array",
			}, []string{"hostId", "path"}),
		},
		"aiops_context_hosts": {
			Name:        "aiops_context_hosts",
			Description: "读取平台主机资产摘要，用于全局 AIOps 助手做资产覆盖、认证状态和近期资产上下文判断。",
			RiskLevel:   aiopsmodel.AgentToolRiskReadonly,
			Readonly:    true,
			InputSchema: objectSchema(map[string]interface{}{
				"limit": "number",
			}, []string{}),
		},
		"aiops_context_database": {
			Name:        "aiops_context_database",
			Description: "读取数据库资产上下文，用于数据库总览、实例定位和平台风险分析。只读，不执行 SQL。",
			RiskLevel:   aiopsmodel.AgentToolRiskReadonly,
			Readonly:    true,
			InputSchema: objectSchema(map[string]interface{}{
				"assetId": "number",
				"keyword": "string",
				"env":     "string",
				"scope":   "string",
			}, []string{}),
		},
		"aiops_context_k8s": {
			Name:        "aiops_context_k8s",
			Description: "读取 K8s 集群上下文，用于集群总览和平台风险分析。只读，不修改集群资源。",
			RiskLevel:   aiopsmodel.AgentToolRiskReadonly,
			Readonly:    true,
			InputSchema: objectSchema(map[string]interface{}{
				"clusterId": "number",
				"keyword":   "string",
				"scope":     "string",
			}, []string{}),
		},
		"knowledge_search": {
			Name:        "knowledge_search",
			Description: "检索运维知识库、Runbook、故障复盘和 FAQ，为助手回答补充可追溯文档证据。",
			RiskLevel:   aiopsmodel.AgentToolRiskReadonly,
			Readonly:    true,
			InputSchema: objectSchema(map[string]interface{}{
				"query":       "string",
				"limit":       "number",
				"docType":     "string",
				"status":      "string",
				"groupName":   "string",
				"category":    "string",
				"serviceName": "string",
				"tags":        "array",
			}, []string{"query"}),
		},
	}
}

func evaluateAgentToolPolicy(tool aiopsmodel.AgentToolDefinition) aiopsmodel.AgentToolPolicyDecision {
	decision := aiopsmodel.AgentToolPolicyDecision{
		Decision: aiopsmodel.AgentToolDecisionAllow,
		Reason:   "第一阶段仅允许只读工具调用",
		Risk:     tool.RiskLevel,
		Readonly: tool.Readonly,
		Checks: []string{
			"tool_registered",
			"readonly_required",
			"mutation_tools_disabled",
		},
	}
	if !tool.Readonly {
		decision.Decision = aiopsmodel.AgentToolDecisionDeny
		decision.Reason = "第一阶段不允许变更类工具"
	}
	return decision
}

func authorizeAgentGatewayScope(c *gin.Context, policyTool aiopsmodel.MCPToolView, handlerKey string, args map[string]interface{}) error {
	if err := authorizeMCPResourceScope(c, policyTool.Domain, args); err != nil {
		return err
	}

	switch handlerKey {
	case "get_cmdb_host", "get_agent_status", "query_host_metrics", "query_host_services", "query_host_logs":
		isAdmin, allowed, err := agentGatewayAuthorizedHostScope(c)
		if err != nil || isAdmin {
			return err
		}
		hostID := uintArg(args, "hostId")
		if hostID == 0 {
			identifier := ""
			for _, key := range []string{"ip", "hostname", "instance"} {
				if value := strings.TrimSpace(stringArg(args, key)); value != "" {
					identifier = value
					break
				}
			}
			if identifier == "" {
				return nil // The handler will report missing required identity.
			}
			hostDao := cmdbdao.NewCmdbHostDao()
			host, resolveErr := hostDao.GetCmdbHostByInstanceIdentifier(identifier)
			if resolveErr != nil {
				return resolveErr
			}
			hostID = host.ID
		}
		if _, ok := allowed[hostID]; !ok {
			return ErrMCPPermissionDenied
		}
	}
	return nil
}

func agentGatewayAuthorizedHostScope(c *gin.Context) (bool, map[uint]struct{}, error) {
	isAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return false, nil, fmt.Errorf("resolve Agent Tool host scope: %w", err)
	}
	if isAdmin {
		return true, nil, nil
	}
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil || admin.ID == 0 {
		return false, nil, ErrMCPUnauthorized
	}
	authDao := cmdbdao.NewBastionHostAuthorizationDao()
	ids, err := authDao.GetUserAuthorizedHostIds(admin.ID)
	if err != nil {
		return false, nil, err
	}
	allowed := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		allowed[id] = struct{}{}
	}
	return false, allowed, nil
}

func (s *AgentToolGatewayService) invokeGetCMDBHost(c *gin.Context, args map[string]interface{}, mcpToolName string) (interface{}, error) {
	if hostID := uintArg(args, "hostId"); hostID > 0 {
		host, err := s.hostDao.GetCmdbHostById(hostID)
		if err != nil {
			return nil, err
		}
		return s.authorizedAgentHostContext(c, host, mcpToolName)
	}
	for _, key := range []string{"ip", "hostname", "instance"} {
		value := strings.TrimSpace(stringArg(args, key))
		if value == "" {
			continue
		}
		host, err := s.hostDao.GetCmdbHostByInstanceIdentifier(value)
		if err == nil {
			return s.authorizedAgentHostContext(c, host, mcpToolName)
		}
	}
	return nil, errors.New("one of hostId, ip, hostname or instance is required")
}

func (s *AgentToolGatewayService) authorizedAgentHostContext(c *gin.Context, host cmdbmodel.CmdbHost, mcpToolName string) (interface{}, error) {
	isAdmin, allowed, err := agentGatewayAuthorizedHostScope(c)
	if err != nil {
		return nil, err
	}
	if !isAdmin {
		if _, ok := allowed[host.ID]; !ok {
			return nil, ErrMCPPermissionDenied
		}
	}
	if mcpToolName == "ecs_get_configuration" {
		return toAgentHostConfiguration(host), nil
	}
	return toAgentHostContext(host), nil
}

func (s *AgentToolGatewayService) invokeSearchCMDBHosts(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	keyword := strings.TrimSpace(stringArg(args, "keyword"))
	if keyword == "" {
		return nil, errors.New("keyword is required")
	}
	limit := intArg(args, "limit", defaultToolGatewayPageSize)
	if limit <= 0 || limit > maxToolGatewayPageSize {
		limit = defaultToolGatewayPageSize
	}
	hosts := s.hostDao.SearchCmdbHosts(keyword)
	isAdmin, allowed, err := agentGatewayAuthorizedHostScope(c)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]interface{}, 0, len(hosts))
	matched := 0
	for _, host := range hosts {
		if !isAdmin {
			if _, ok := allowed[host.ID]; !ok {
				continue
			}
		}
		matched++
		if len(items) < limit {
			items = append(items, toAgentHostContext(host))
		}
	}
	return map[string]interface{}{
		"keyword": keyword,
		"total":   matched,
		"limit":   limit,
		"items":   items,
	}, nil
}

func (s *AgentToolGatewayService) invokeGetAgentStatus(args map[string]interface{}) (interface{}, error) {
	hostID := uintArg(args, "hostId")
	if hostID == 0 {
		return nil, errors.New("hostId is required")
	}
	agent, err := s.agentMon.GetByHostID(hostID)
	if err != nil {
		return map[string]interface{}{
			"hostId":       hostID,
			"installed":    false,
			"online":       false,
			"statusText":   "未安装或未登记",
			"errorMessage": err.Error(),
		}, nil
	}
	online := agent.Status == cmdbmodel.AgentStatusRunning && time.Since(agent.LastHeartbeat.Time) <= 5*time.Minute
	return map[string]interface{}{
		"id":              agent.ID,
		"hostId":          agent.HostID,
		"hostName":        agent.HostName,
		"version":         agent.Version,
		"status":          agent.Status,
		"statusText":      agent.GetStatusText(),
		"installed":       true,
		"online":          online,
		"lastHeartbeat":   agent.LastHeartbeat,
		"installPath":     agent.InstallPath,
		"port":            agent.Port,
		"pid":             agent.PID,
		"installProgress": agent.InstallProgress,
		"errorMsg":        agent.ErrorMsg,
	}, nil
}

func (s *AgentToolGatewayService) invokeQueryActiveAlerts(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	page := intArg(args, "page", 1)
	pageSize := intArg(args, "pageSize", defaultToolGatewayPageSize)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > maxToolGatewayPageSize {
		pageSize = defaultToolGatewayPageSize
	}

	query := alertmodel.AlertEventQuery{
		PageQuery: alertmodel.PageQuery{
			Page:     page,
			PageSize: pageSize,
			Keyword:  strings.TrimSpace(stringArg(args, "keyword")),
		},
		Severity:    strings.TrimSpace(stringArg(args, "severity")),
		BusinessID:  strings.TrimSpace(stringArg(args, "businessId")),
		EventStatus: alertmodel.EventDisplayStatusActive,
	}
	isAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return nil, err
	}
	var list []alertmodel.AlertActiveEvent
	var total int64
	if isAdmin {
		list, total, err = s.alertDao.ListActiveEvents(query)
	} else {
		admin, adminErr := jwt.GetAdmin(c)
		if adminErr != nil || admin == nil || admin.ID == 0 {
			return nil, ErrMCPUnauthorized
		}
		scope, scopeErr := systemdao.GetUserBusinessScope(admin.ID)
		if scopeErr != nil {
			return nil, scopeErr
		}
		if query.BusinessID != "" {
			if !agentGatewayScopeAllows(scope, query.BusinessID) {
				return nil, ErrMCPPermissionDenied
			}
			list, total, err = s.alertDao.ListActiveEvents(query)
		} else {
			for _, businessID := range scope {
				scopedQuery := query
				scopedQuery.BusinessID = businessID
				rows, count, queryErr := s.alertDao.ListActiveEvents(scopedQuery)
				if queryErr != nil {
					return nil, fmt.Errorf("查询 Alertmanager 活跃告警失败: %w", queryErr)
				}
				list = append(list, rows...)
				total += count
			}
			if len(list) > pageSize {
				list = list[:pageSize]
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("查询 Alertmanager 活跃告警失败: %w", err)
	}

	items := make([]map[string]interface{}, 0, len(list))
	for _, event := range list {
		items = append(items, map[string]interface{}{
			"id":               event.ID,
			"fingerprint":      event.Fingerprint,
			"ruleId":           event.RuleID,
			"ruleName":         event.RuleName,
			"datasourceId":     event.DatasourceID,
			"datasourceType":   event.DatasourceType,
			"severity":         event.Severity,
			"level":            event.Labels["level"],
			"eventStatus":      alertmodel.EventDisplayStatusActive,
			"evalStatus":       event.Status,
			"labels":           event.Labels,
			"annotations":      event.Annotations,
			"value":            event.Value,
			"assetId":          event.AssetID,
			"businessId":       event.BusinessID,
			"firstTriggeredAt": event.FirstTriggeredAt,
			"lastEvaluatedAt":  event.LastEvaluatedAt,
			"lastNotifiedAt":   event.LastNotifiedAt,
			"notifyCount":      event.NotifyCount,
			"suppressed":       event.Suppressed,
			"silenced":         event.Silenced,
		})
	}
	return map[string]interface{}{
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"list":     items,
	}, nil
}

func agentGatewayScopeAllows(scope []string, value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range scope {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return true
		}
	}
	return false
}

func (s *AgentToolGatewayService) invokeQueryAlertDetail(args map[string]interface{}) (interface{}, error) {
	return nil, errors.New("旧 api/monitor 告警详情已移除，请使用 alertmanager 告警接口")
}

func (s *AgentToolGatewayService) invokeQueryHostMetrics(args map[string]interface{}, mcpToolName string) (interface{}, error) {
	host, err := s.resolveToolHost(args)
	if err != nil {
		return nil, err
	}
	instances := metricInstanceCandidates(host)
	if len(instances) == 0 {
		return nil, errors.New("host has no metric identifier")
	}

	minutes := intArg(args, "minutes", defaultToolMetricMinutes)
	if minutes <= 0 {
		minutes = defaultToolMetricMinutes
	}
	if minutes > maxToolMetricMinutes {
		minutes = maxToolMetricMinutes
	}
	step := strings.TrimSpace(stringArg(args, "step"))
	if step == "" {
		step = "30s"
	}

	summary, summaryErr := s.monSvc.GetHostMetricsData(host.ID, minutes, step)
	if mcpToolName == "ecs_list_processes" {
		topProcesses, topErr := s.monSvc.GetTopProcessesData(host.ID)
		return map[string]interface{}{
			"host": toAgentHostContext(host), "topProcesses": topProcesses,
			"error": errorString(topErr), "updateTime": time.Now().Unix(),
		}, nil
	}
	var topProcesses *cmdbmodel.TopProcessesResult
	var topErr error
	if mcpToolName == "ecs_diagnose_host" {
		topProcesses, topErr = s.monSvc.GetTopProcessesData(host.ID)
	}

	end := time.Now()
	start := end.Add(-time.Duration(minutes) * time.Minute)

	instanceMatcher := prometheusRegexMatcher(instances)
	queries := map[string]string{
		"cpu":    fmt.Sprintf("system_cpu_usage_percent{instance=~%q}", instanceMatcher),
		"memory": fmt.Sprintf("system_memory_usage_percent{instance=~%q}", instanceMatcher),
		"disk":   fmt.Sprintf("system_disk_usage_percent{instance=~%q,mountpoint=%q}", instanceMatcher, "/"),
		"load5":  fmt.Sprintf("system_load_average_5min{instance=~%q}", instanceMatcher),
	}
	hasData := false
	if summaryErr == nil && summary != nil && summary.OnlineStatus == 0 {
		hasData = true
	}

	result := map[string]interface{}{
		"host":               toAgentHostContext(host),
		"instanceCandidates": instances,
		"instanceMatcher":    instanceMatcher,
		"hasData":            hasData,
		"summary":            summary,
		"start":              start.Format("2006-01-02 15:04:05"),
		"end":                end.Format("2006-01-02 15:04:05"),
		"step":               step,
		"summaryError":       errorString(summaryErr),
		"queries":            queries,
	}
	if mcpToolName == "ecs_check_monitoring" {
		return map[string]interface{}{
			"host": result["host"], "hasData": hasData, "summary": summary,
			"summaryError": errorString(summaryErr), "checkedAt": end.Format("2006-01-02 15:04:05"),
		}, nil
	}
	if mcpToolName == "ecs_diagnose_host" {
		result["topProcesses"] = topProcesses
		result["topProcessesError"] = errorString(topErr)
		result["diagnosis"] = summarizeHostPressure(summary, topProcesses, summaryErr, topErr)
	}
	return result, nil
}

func (s *AgentToolGatewayService) invokeQueryHostServices(args map[string]interface{}, mcpToolName string) (interface{}, error) {
	host, err := s.resolveToolHost(args)
	if err != nil {
		return nil, err
	}
	topProcesses, topErr := s.monSvc.GetTopProcessesData(host.ID)
	if topErr != nil {
		return nil, topErr
	}
	services := inferAgentHostServices(topProcesses)
	result := map[string]interface{}{
		"host":         toAgentHostContext(host),
		"services":     services,
		"serviceCount": len(services),
		"updateTime":   time.Now().Unix(),
	}
	if mcpToolName == "ecs_check_service" {
		result["checkStatus"] = "completed"
	}
	return result, nil
}

func (s *AgentToolGatewayService) resolveToolHost(args map[string]interface{}) (cmdbmodel.CmdbHost, error) {
	if hostID := uintArg(args, "hostId"); hostID > 0 {
		return s.hostDao.GetCmdbHostById(hostID)
	}
	for _, key := range []string{"ip", "hostname", "instance"} {
		value := strings.TrimSpace(stringArg(args, key))
		if value == "" {
			continue
		}
		host, err := s.hostDao.GetCmdbHostByInstanceIdentifier(value)
		if err == nil {
			return host, nil
		}
	}
	return cmdbmodel.CmdbHost{}, errors.New("one of hostId, ip, hostname or instance is required")
}

func (s *AgentToolGatewayService) invokeQueryHostLogs(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	hostID := uintArg(args, "hostId")
	path := strings.TrimSpace(stringArg(args, "path"))
	if hostID == 0 || path == "" {
		return nil, errors.New("hostId and path are required")
	}
	tailLines := intArg(args, "tailLines", defaultToolLogTailLines)
	if tailLines <= 0 || tailLines > maxToolLogTailLines {
		tailLines = defaultToolLogTailLines
	}
	req := &aiopsmodel.HostLogQueryRequest{
		SourceType: aiopsmodel.LogSourceTypeHost,
		HostID:     hostID,
		AuthID:     uintArg(args, "authId"),
		Path:       path,
		TailLines:  tailLines,
		Limit:      intArg(args, "limit", tailLines),
		Keyword:    strings.TrimSpace(stringArg(args, "keyword")),
		Regex:      strings.TrimSpace(stringArg(args, "regex")),
		Levels:     stringSliceArg(args, "levels"),
		Format:     strings.TrimSpace(stringArg(args, "format")),
		FormatName: strings.TrimSpace(stringArg(args, "formatName")),
		FormatYAML: strings.TrimSpace(stringArg(args, "formatYaml")),
	}
	return s.logSvc.QueryHostLogs(c, req)
}

func (s *AgentToolGatewayService) invokeAIOpsHostContext(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	output, err := buildAssistantHostInventoryContext(c)
	if err != nil {
		return nil, err
	}
	limit := intArg(args, "limit", 10)
	if limit > 0 {
		items := assistantChatSlice(output["recentInventory"])
		if len(items) > limit {
			output["recentInventory"] = items[:limit]
		}
	}
	return output, nil
}

func (s *AgentToolGatewayService) invokeAIOpsDatabaseContext(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	return buildAssistantDatabaseContext(c, args)
}

func (s *AgentToolGatewayService) invokeAIOpsK8sContext(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	return buildAssistantK8sContext(c, args)
}

func (s *AgentToolGatewayService) invokeKnowledgeSearch(args map[string]interface{}) (interface{}, error) {
	query := strings.TrimSpace(stringArg(args, "query"))
	if query == "" {
		return nil, errors.New("query is required")
	}
	limit := intArg(args, "limit", 5)
	if limit <= 0 {
		limit = 5
	}
	if limit > maxToolGatewayPageSize {
		limit = maxToolGatewayPageSize
	}
	items, err := NewKnowledgeService().SearchDocuments(&aiopsmodel.KnowledgeSearchRequest{
		Query:       query,
		Limit:       limit,
		DocType:     strings.TrimSpace(stringArg(args, "docType")),
		Status:      aiopsmodel.KnowledgeStatusPublished,
		GroupName:   strings.TrimSpace(stringArg(args, "groupName")),
		Category:    strings.TrimSpace(stringArg(args, "category")),
		ServiceName: strings.TrimSpace(stringArg(args, "serviceName")),
		Tags:        stringSliceArg(args, "tags"),
	})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"query": query,
		"limit": limit,
		"total": len(items),
		"items": items,
	}, nil
}

func buildToolEvidence(toolName string, output interface{}) *aiopsmodel.AgentDiagnosisEvidence {
	content := "工具调用完成"
	evidenceType := "tool"
	title := toolName
	reference := toolName
	switch toolName {
	case "get_cmdb_host":
		evidenceType = "cmdb"
		content = "已获取 CMDB 主机上下文"
	case "search_cmdb_hosts":
		evidenceType = "cmdb"
		content = "已搜索 CMDB 主机"
	case "get_agent_status":
		evidenceType = "agent"
		content = "已获取主机 Agent 状态"
	case "query_active_alerts":
		evidenceType = "alert"
		content = "已查询活跃告警"
	case "query_alert_detail":
		evidenceType = "alert"
		content = "已查询告警详情"
	case "query_host_metrics":
		evidenceType = "metric"
		content = "已查询主机监控指标"
	case "query_host_services":
		evidenceType = "process"
		content = "已查询主机 Top 进程并推断服务"
	case "query_host_logs":
		evidenceType = "log"
		content = "已查询主机日志"
	case "aiops_context_hosts":
		evidenceType = "cmdb"
		title = "主机资产上下文"
		reference = "cmdb_host"
		content = truncateAssistantChatText(buildAssistantChatHostInventoryAnswer(assistantChatMap(output)), 1000)
	case "aiops_context_database":
		evidenceType = "database"
		title = "数据库资产上下文"
		reference = "cmdb_db_management"
		content = truncateAssistantChatText(buildAssistantChatDatabaseAnswer(assistantChatMap(output)), 1000)
	case "aiops_context_k8s":
		evidenceType = "k8s"
		title = "K8s 集群上下文"
		reference = "k8s_cluster"
		content = truncateAssistantChatText(buildAssistantChatK8sAnswer(assistantChatMap(output)), 1000)
	case "knowledge_search":
		evidenceType = "knowledge"
		title = "知识库检索结果"
		reference = "aiops_knowledge_document"
		data := assistantChatMap(output)
		content = fmt.Sprintf("已检索运维知识库，命中 %d 篇文档", int(assistantChatNumber(data["total"])))
	}
	return &aiopsmodel.AgentDiagnosisEvidence{
		Type:      evidenceType,
		Title:     title,
		Reference: reference,
		Content:   content,
		Raw:       output,
	}
}

func toAgentHostContext(host cmdbmodel.CmdbHost) map[string]interface{} {
	groupName := ""
	if host.Group.ID != 0 {
		groupName = host.Group.Name
	}
	return map[string]interface{}{
		"id":         host.ID,
		"hostName":   host.HostName,
		"name":       host.Name,
		"groupId":    host.GroupID,
		"groupName":  groupName,
		"privateIp":  host.PrivateIP,
		"publicIp":   host.PublicIP,
		"sshIp":      host.SSHIP,
		"hostType":   host.HostType,
		"os":         host.OS,
		"status":     host.Status,
		"vendor":     host.Vendor,
		"region":     host.Region,
		"instanceId": host.InstanceID,
		"tag":        host.Tag,
		"remark":     host.Remark,
		"expireTime": host.ExpireTime,
	}
}

func toAgentHostConfiguration(host cmdbmodel.CmdbHost) map[string]interface{} {
	return map[string]interface{}{
		"id": host.ID, "hostName": host.HostName, "name": host.Name,
		"hostType": host.HostType, "os": host.OS, "cpu": host.CPU,
		"memory": host.Memory, "disk": host.Disk,
	}
}

func summarizeHostPressure(
	summary *cmdbmodel.HostMetrics,
	topProcesses *cmdbmodel.TopProcessesResult,
	summaryErr error,
	topErr error,
) map[string]interface{} {
	result := map[string]interface{}{
		"status": "unknown",
		"risks":  []string{},
		"notes":  []string{},
	}
	risks := make([]string, 0)
	notes := make([]string, 0)

	if summaryErr != nil {
		notes = append(notes, "监控摘要查询失败："+summaryErr.Error())
	} else if summary == nil {
		notes = append(notes, "监控摘要为空")
	} else if summary.OnlineStatus != 0 {
		risks = append(risks, "监控目标离线或采集不可用")
	} else {
		switch {
		case summary.CPUUsage >= 85:
			risks = append(risks, fmt.Sprintf("CPU 使用率较高：%.2f%%", summary.CPUUsage))
		case summary.CPUUsage >= 70:
			notes = append(notes, fmt.Sprintf("CPU 使用率偏高：%.2f%%", summary.CPUUsage))
		default:
			notes = append(notes, fmt.Sprintf("CPU 使用率正常：%.2f%%", summary.CPUUsage))
		}

		switch {
		case summary.MemoryUsage >= 85:
			risks = append(risks, fmt.Sprintf("内存使用率较高：%.2f%%", summary.MemoryUsage))
		case summary.MemoryUsage >= 70:
			notes = append(notes, fmt.Sprintf("内存使用率偏高：%.2f%%", summary.MemoryUsage))
		default:
			notes = append(notes, fmt.Sprintf("内存使用率正常：%.2f%%", summary.MemoryUsage))
		}

		switch {
		case summary.DiskUsage >= 85:
			risks = append(risks, fmt.Sprintf("磁盘使用率较高：%.2f%%", summary.DiskUsage))
		case summary.DiskUsage >= 70:
			notes = append(notes, fmt.Sprintf("磁盘使用率偏高：%.2f%%", summary.DiskUsage))
		default:
			notes = append(notes, fmt.Sprintf("磁盘使用率正常：%.2f%%", summary.DiskUsage))
		}

		if summary.Load5 >= 4 {
			risks = append(risks, fmt.Sprintf("5 分钟负载偏高：%.2f", summary.Load5))
		} else {
			notes = append(notes, fmt.Sprintf("5 分钟负载正常：%.2f", summary.Load5))
		}
	}

	if topErr != nil {
		notes = append(notes, "Top 进程查询失败："+topErr.Error())
	} else if topProcesses != nil {
		if len(topProcesses.TopCPU) > 0 {
			notes = append(notes, fmt.Sprintf("CPU 最高进程：%s(pid=%d, %.2f%%)", topProcesses.TopCPU[0].Name, topProcesses.TopCPU[0].PID, topProcesses.TopCPU[0].CPUPercent))
		}
		if len(topProcesses.TopMemory) > 0 {
			notes = append(notes, fmt.Sprintf("内存最高进程：%s(pid=%d, %.2f%%)", topProcesses.TopMemory[0].Name, topProcesses.TopMemory[0].PID, topProcesses.TopMemory[0].MemPercent))
		}
	}

	status := "healthy"
	if len(risks) > 0 {
		status = "warning"
	}
	result["status"] = status
	result["risks"] = risks
	result["notes"] = notes
	return result
}

func inferAgentHostServices(topProcesses *cmdbmodel.TopProcessesResult) []map[string]interface{} {
	if topProcesses == nil {
		return []map[string]interface{}{}
	}
	type serviceAgg struct {
		Name       string
		PIDs       map[uint]struct{}
		CPUPercent float64
		MemPercent float64
		Sources    map[string]struct{}
	}
	items := map[string]*serviceAgg{}
	add := func(process cmdbmodel.ProcessInfo, source string) {
		name := normalizeAgentServiceName(process.Name)
		if name == "" || isIgnoredAgentServiceProcess(name) {
			return
		}
		item := items[name]
		if item == nil {
			item = &serviceAgg{
				Name:    name,
				PIDs:    map[uint]struct{}{},
				Sources: map[string]struct{}{},
			}
			items[name] = item
		}
		if process.PID > 0 {
			item.PIDs[process.PID] = struct{}{}
		}
		if process.CPUPercent > item.CPUPercent {
			item.CPUPercent = process.CPUPercent
		}
		if process.MemPercent > item.MemPercent {
			item.MemPercent = process.MemPercent
		}
		item.Sources[source] = struct{}{}
	}
	for _, process := range topProcesses.TopCPU {
		add(process, "topCPU")
	}
	for _, process := range topProcesses.TopMemory {
		add(process, "topMemory")
	}

	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		pids := make([]uint, 0, len(item.PIDs))
		for pid := range item.PIDs {
			pids = append(pids, pid)
		}
		sources := make([]string, 0, len(item.Sources))
		for source := range item.Sources {
			sources = append(sources, source)
		}
		result = append(result, map[string]interface{}{
			"name":       item.Name,
			"pids":       pids,
			"cpuPercent": item.CPUPercent,
			"memPercent": item.MemPercent,
			"sources":    sources,
			"confidence": inferAgentServiceConfidence(item.Name),
		})
	}
	return result
}

func normalizeAgentServiceName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ""
	}
	if idx := strings.Index(name, ":"); idx > 0 && strings.HasPrefix(name, "kworker") {
		return "kworker"
	}
	if idx := strings.Index(name, "/"); idx > 0 {
		name = name[:idx]
	}
	return name
}

func isIgnoredAgentServiceProcess(name string) bool {
	switch name {
	case "", "kworker", "systemd", "bash", "sh", "sshd", "sleep", "top", "ps", "grep", "awk":
		return true
	default:
		return false
	}
}

func inferAgentServiceConfidence(name string) string {
	switch name {
	case "nginx", "httpd", "apache2", "mysqld", "mysql", "redis-server", "redis", "postgres", "postgresql",
		"java", "python", "node", "php-fpm", "tomcat", "docker", "dockerd", "containerd", "dodevops-agent":
		return "high"
	default:
		return "medium"
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func metricInstanceCandidates(host cmdbmodel.CmdbHost) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, 8)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	for _, value := range []string{
		host.PrivateIP,
		host.SSHIP,
		host.PublicIP,
		host.Name,
		host.HostName,
		host.InstanceID,
	} {
		add(value)
		if looksLikeIP(value) {
			add(value + ":9100")
			add(value + ":9182")
			add(value + ":12580")
		}
	}
	return result
}

func prometheusRegexMatcher(values []string) string {
	escaped := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		escaped = append(escaped, regexp.QuoteMeta(value))
	}
	if len(escaped) == 0 {
		return "$^"
	}
	return "^(?:" + strings.Join(escaped, "|") + ")$"
}

func looksLikeIP(value string) bool {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || n > 255 {
			return false
		}
	}
	return true
}

func objectSchema(properties map[string]interface{}, required []string) map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

func compactToolJSON(value interface{}) string {
	data, err := json.Marshal(RedactMCPValue(value))
	if err != nil {
		return ""
	}
	const max = 60000
	if len(data) > max {
		return string(data[:max]) + "...truncated"
	}
	return string(data)
}

func stringArg(args map[string]interface{}, key string) string {
	value, ok := args[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func intArg(args map[string]interface{}, key string, fallback int) int {
	value, ok := args[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		parsed, err := v.Int64()
		if err == nil {
			return int(parsed)
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func uintArg(args map[string]interface{}, key string) uint {
	value := intArg(args, key, 0)
	if value <= 0 {
		return 0
	}
	return uint(value)
}

func stringSliceArg(args map[string]interface{}, key string) []string {
	value, ok := args[key]
	if !ok || value == nil {
		return nil
	}
	switch v := value.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			text := strings.TrimSpace(fmt.Sprintf("%v", item))
			if text != "" {
				result = append(result, text)
			}
		}
		return result
	case string:
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, item := range parts {
			text := strings.TrimSpace(item)
			if text != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}
