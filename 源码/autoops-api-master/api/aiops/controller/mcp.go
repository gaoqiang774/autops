package controller

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"dodevops-api/api/aiops/model"
	"dodevops-api/api/aiops/service"
	"dodevops-api/common/config"
	agentskill "dodevops-api/pkg/agent/skill"

	"github.com/gin-gonic/gin"
)

const (
	mcpTokenHeader       = "X-AIOPS-MCP-TOKEN"
	mcpToolAIOpsChat     = "aiops_chat"
	mcpToolAIOpsConfirm  = "aiops_confirm_action"
	mcpToolAIOpsSkillCat = "aiops_skill_catalog"
	mcpToolAIOpsSkillRun = "aiops_skill_execute"
)

// @Summary AIOps MCP Server
// @Description 通过 MCP Streamable HTTP 暴露 AIOps 运维智能体工具。
// @Accept json
// @Produce json
// @Tags AIOps MCP
// @Param X-AIOPS-MCP-TOKEN header string false "MCP API Token"
// @Success 200 {object} model.MCPResponse
// @router /api/v1/aiops/mcp [post]
func AIOpsMCP(c *gin.Context) {
	if !validateMCPToken(c) {
		c.JSON(http.StatusOK, model.MCPResponse{
			JSONRPC: "2.0",
			Error:   &model.MCPError{Code: -32001, Message: "invalid MCP token"},
		})
		return
	}
	if err := injectMCPServiceAccount(c); err != nil {
		c.JSON(http.StatusOK, model.MCPResponse{
			JSONRPC: "2.0",
			Error:   &model.MCPError{Code: -32002, Message: err.Error()},
		})
		return
	}

	var raw json.RawMessage
	if err := c.BindJSON(&raw); err != nil {
		c.JSON(http.StatusOK, model.MCPResponse{
			JSONRPC: "2.0",
			Error:   &model.MCPError{Code: -32700, Message: "parse error"},
		})
		return
	}

	if len(raw) > 0 && raw[0] == '[' {
		var reqs []model.MCPRequest
		if err := json.Unmarshal(raw, &reqs); err != nil {
			c.JSON(http.StatusOK, model.MCPResponse{
				JSONRPC: "2.0",
				Error:   &model.MCPError{Code: -32600, Message: "invalid request"},
			})
			return
		}
		responses := make([]model.MCPResponse, 0, len(reqs))
		for _, req := range reqs {
			if isMCPNotification(req) {
				_, _ = handleMCPRequest(c, req)
				continue
			}
			resp, _ := handleMCPRequest(c, req)
			responses = append(responses, resp)
		}
		if len(responses) == 0 {
			c.Status(http.StatusAccepted)
			return
		}
		c.JSON(http.StatusOK, responses)
		return
	}

	var req model.MCPRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		c.JSON(http.StatusOK, model.MCPResponse{
			JSONRPC: "2.0",
			Error:   &model.MCPError{Code: -32600, Message: "invalid request"},
		})
		return
	}
	if isMCPNotification(req) {
		_, _ = handleMCPRequest(c, req)
		c.Status(http.StatusAccepted)
		return
	}
	resp, _ := handleMCPRequest(c, req)
	c.JSON(http.StatusOK, resp)
}

func validateMCPToken(c *gin.Context) bool {
	if config.Config == nil {
		return false
	}
	expected := strings.TrimSpace(config.Config.MCP.Token)
	if expected == "" {
		return false
	}
	token := strings.TrimSpace(c.GetHeader(mcpTokenHeader))
	if token == "" {
		token = strings.TrimSpace(c.GetHeader("Authorization"))
		token = strings.TrimPrefix(token, "Bearer ")
	}
	return len(token) == len(expected) && subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

func injectMCPServiceAccount(c *gin.Context) error {
	return service.InjectMCPServiceAccount(c)
}

func handleMCPRequest(c *gin.Context, req model.MCPRequest) (model.MCPResponse, bool) {
	base := model.MCPResponse{
		JSONRPC: "2.0",
		ID:      decodeMCPID(req.ID),
	}
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		base.Error = &model.MCPError{Code: -32600, Message: "invalid jsonrpc version"}
		return base, false
	}

	switch req.Method {
	case "initialize":
		base.Result = gin.H{
			"protocolVersion": "2025-03-26",
			"capabilities": gin.H{
				"tools": gin.H{},
			},
			"serverInfo": gin.H{
				"name":    "autoops-aiops",
				"version": "1.0.0",
			},
		}
	case "notifications/initialized":
		base.Result = gin.H{}
	case "ping":
		base.Result = gin.H{}
	case "tools/list":
		tools, err := service.NewMCPCatalogService().ListCallableTools(c.Param("domain"))
		if err != nil {
			base.Error = &model.MCPError{Code: -32603, Message: "load tool catalog failed: " + err.Error()}
			return base, false
		}
		items := make([]gin.H, 0, len(tools))
		for _, tool := range tools {
			items = append(items, gin.H{
				"name": tool.Name, "description": tool.Description, "inputSchema": tool.InputSchema,
				"annotations": gin.H{"readOnlyHint": tool.Readonly},
				"_meta": gin.H{
					"domain": tool.Domain, "riskLevel": tool.EffectiveRisk,
					"requiredConfirmation": tool.RequiredConfirmation, "version": tool.Version,
				},
			})
		}
		base.Result = gin.H{"tools": items}
	case "tools/call":
		resultData, err := callMCPAIOpsTool(c, req.Params, c.Param("domain"))
		if err != nil {
			base.Result = model.MCPToolCallResult{
				Content: []model.MCPToolContent{{Type: "text", Text: err.Error()}},
				IsError: true,
			}
			return base, false
		}
		base.Result = resultData
	default:
		base.Error = &model.MCPError{Code: -32601, Message: "method not found"}
	}
	return base, true
}

func callMCPAIOpsTool(c *gin.Context, paramsJSON json.RawMessage, domain string) (model.MCPToolCallResult, error) {
	var params model.MCPToolCallParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return model.MCPToolCallResult{}, fmt.Errorf("invalid tools/call params: %w", err)
	}
	tool, err := service.NewMCPCatalogService().AuthorizeTool(domain, params.Name)
	if err != nil {
		return model.MCPToolCallResult{}, err
	}
	switch tool.HandlerKey {
	case mcpToolAIOpsChat:
		return callMCPAIOpsChatTool(c, params.Arguments)
	case mcpToolAIOpsConfirm:
		confirmationID := mcpStringArg(params.Arguments, "confirmationId", "confirmation_id")
		if confirmationID == "" {
			return model.MCPToolCallResult{}, fmt.Errorf("confirmationId is required")
		}
		executor := service.NewMCPToolExecutor()
		data, runID, err := executor.ExecuteAction(c, tool, params.Arguments, func(ctx *gin.Context, _ map[string]interface{}, _ uint64) (interface{}, error) {
			return service.ConfirmMCPAction(ctx, confirmationID)
		})
		if err != nil {
			if runID > 0 {
				return model.MCPToolCallResult{}, fmt.Errorf("confirm action failed (runId=%d): %w", runID, err)
			}
			return model.MCPToolCallResult{}, fmt.Errorf("confirm action failed: %w", err)
		}
		payload, _ := json.Marshal(gin.H{"runId": runID, "tool": tool.Name, "data": data})
		return model.MCPToolCallResult{Content: []model.MCPToolContent{{Type: "text", Text: string(payload)}}}, nil
	case mcpToolAIOpsSkillCat:
		return callMCPSkillCatalogTool(params.Arguments)
	case mcpToolAIOpsSkillRun:
		return callMCPSkillExecuteTool(c, params.Arguments)
	default:
		executor := service.NewMCPToolExecutor()
		handler := func(ctx *gin.Context, args map[string]interface{}, auditRunID uint64) (interface{}, error) {
			return invokeMCPReadTool(ctx, tool, args, auditRunID)
		}
		var data interface{}
		var runID uint64
		if tool.Readonly {
			data, runID, err = executor.Execute(c, tool, params.Arguments, handler)
		} else {
			data, runID, err = executor.ExecuteAction(c, tool, params.Arguments, handler)
		}
		if err != nil {
			if runID > 0 {
				return model.MCPToolCallResult{}, fmt.Errorf("tool %s failed (runId=%d): %w", params.Name, runID, err)
			}
			return model.MCPToolCallResult{}, fmt.Errorf("tool %s failed: %w", params.Name, err)
		}
		payload, _ := json.Marshal(gin.H{"runId": runID, "tool": params.Name, "data": data})
		return model.MCPToolCallResult{Content: []model.MCPToolContent{{Type: "text", Text: string(payload)}}}, nil
	}
}

func callMCPAIOpsChatTool(c *gin.Context, args map[string]interface{}) (model.MCPToolCallResult, error) {
	question := mcpStringArg(args, "question", "query", "content", "text")
	if question == "" {
		return model.MCPToolCallResult{}, fmt.Errorf("question is required")
	}
	sessionID := mcpStringArg(args, "sessionId", "session_id")
	hasStableSession := sessionID != "" || mcpStringArg(args, "conversationId", "conversation_id", "chatId", "chat_id", "userId", "user_id") != ""
	if service.IsAssistantActionConfirmation(question) && !hasStableSession && mcpStringArg(args, "confirmationId", "confirmation_id") == "" {
		return model.MCPToolCallResult{}, fmt.Errorf("confirmation requires the same sessionId/conversationId or an explicit confirmationId")
	}
	if sessionID == "" {
		sessionID = buildMCPSessionID(args)
	}

	assistantReq := &model.AssistantChatRequest{
		SessionID: sessionID,
		Question:  question,
		Mode:      "mcp",
		Context:   buildMCPContext(args),
	}

	data, err := service.NewAssistantChatService().Chat(c, assistantReq)
	if err != nil {
		return model.MCPToolCallResult{}, fmt.Errorf("aiops chat failed: %w", err)
	}

	payload, _ := json.Marshal(gin.H{
		"answer":           data.Answer,
		"sessionId":        data.SessionID,
		"runId":            data.RunID,
		"intent":           data.Intent,
		"suggestedActions": data.SuggestedActions,
		"missingInfo":      data.MissingInfo,
		"evidence":         data.Evidence,
		"analysisTask":     data.AnalysisTask,
		"workloadHistory":  data.WorkloadHistory,
		"pendingAction":    data.PendingAction,
		"thinkingProcess":  data.ThinkingProcess,
		"progress":         data.Progress,
	})
	return model.MCPToolCallResult{
		Content: []model.MCPToolContent{
			{Type: "text", Text: data.Answer},
			{Type: "text", Text: string(payload)},
		},
	}, nil
}

func callMCPSkillCatalogTool(args map[string]interface{}) (model.MCPToolCallResult, error) {
	snapshot, err := service.NewSkillCatalogService().Snapshot()
	if err != nil {
		return model.MCPToolCallResult{}, fmt.Errorf("skill catalog failed: %w", err)
	}
	detail := true
	if value, ok := args["detail"]; ok {
		if text := strings.ToLower(strings.TrimSpace(fmt.Sprint(value))); text == "false" || text == "0" {
			detail = false
		}
	}
	payload := gin.H{
		"loaded":   snapshot.Loaded,
		"errors":   snapshot.Errors,
		"loadedAt": snapshot.LoadedAt,
		"root":     snapshot.Root,
		"summary":  mcpSkillCatalogSummary(snapshot),
	}
	if detail {
		payload["manifests"] = snapshot.Manifests
	}
	data, _ := json.Marshal(payload)
	return model.MCPToolCallResult{
		Content: []model.MCPToolContent{
			{Type: "text", Text: mcpSkillCatalogSummary(snapshot)},
			{Type: "text", Text: string(data)},
		},
	}, nil
}

func callMCPSkillExecuteTool(c *gin.Context, args map[string]interface{}) (model.MCPToolCallResult, error) {
	skillName := mcpStringArg(args, "skillName", "skill_name", "name")
	if skillName == "" {
		return model.MCPToolCallResult{}, fmt.Errorf("skillName is required")
	}
	skillArgs := map[string]interface{}{}
	if raw, ok := args["arguments"]; ok {
		if typed, ok := raw.(map[string]interface{}); ok {
			skillArgs = typed
		}
	}
	if len(skillArgs) == 0 {
		for key, value := range args {
			switch key {
			case "skillName", "skill_name", "name", "userPrompt", "traceId", "trace_id":
				continue
			default:
				skillArgs[key] = value
			}
		}
	}
	data, err := service.NewSkillRuntimeService().Execute(c, &service.SkillExecuteRequest{
		SkillName: skillName,
		Arguments: skillArgs,
		Context: service.SkillExecuteContext{
			UserPrompt: mcpStringArg(args, "userPrompt", "question", "query", "content", "text"),
			TraceID:    mcpStringArg(args, "traceId", "trace_id"),
			Source:     "mcp",
		},
	})
	if err != nil {
		return model.MCPToolCallResult{}, fmt.Errorf("skill execute failed: %w", err)
	}
	payload, _ := json.Marshal(data)
	return model.MCPToolCallResult{
		Content: []model.MCPToolContent{
			{Type: "text", Text: data.Summary},
			{Type: "text", Text: string(payload)},
		},
	}, nil
}

func mcpSkillCatalogSummary(snapshot *agentskill.CatalogSnapshot) string {
	if snapshot == nil {
		return "AutoOps Skill Catalog 尚未加载。"
	}
	groupCount := map[string]int{}
	for _, manifest := range snapshot.Manifests {
		group := strings.TrimSpace(manifest.SkillGroup)
		if group == "" {
			group = "unknown"
		}
		groupCount[group]++
	}
	parts := make([]string, 0, len(groupCount))
	for group, count := range groupCount {
		parts = append(parts, fmt.Sprintf("%s=%d", group, count))
	}
	return fmt.Sprintf("AutoOps 后端当前加载了 %d 个声明式 Skill，加载错误 %d 个，分组：%s。", snapshot.Loaded, len(snapshot.Errors), strings.Join(parts, "，"))
}

func isMCPNotification(req model.MCPRequest) bool {
	return len(req.ID) == 0 || string(req.ID) == "null"
}

func decodeMCPID(raw json.RawMessage) interface{} {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var id interface{}
	if err := json.Unmarshal(raw, &id); err != nil {
		return string(raw)
	}
	return id
}

func mcpStringArg(args map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := args[key]; ok {
			if text := strings.TrimSpace(fmt.Sprint(v)); text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func buildMCPSessionID(args map[string]interface{}) string {
	chatType := mcpStringArg(args, "chatType", "chat_type")
	if chatType == "" {
		chatType = "private"
	}
	conversationID := mcpStringArg(args, "conversationId", "conversation_id", "chatId", "chat_id", "userId", "user_id")
	if conversationID == "" {
		conversationID = "default"
	}
	userID := mcpStringArg(args, "userId", "user_id")
	parts := []string{"mcp", chatType, conversationID}
	if chatType == "group" && userID != "" {
		parts = append(parts, userID)
	}
	return strings.Join(parts, ":")
}

func buildMCPContext(args map[string]interface{}) map[string]interface{} {
	ctx := map[string]interface{}{
		"channel": "mcp",
	}
	for key, value := range args {
		ctx[key] = value
	}
	return ctx
}
