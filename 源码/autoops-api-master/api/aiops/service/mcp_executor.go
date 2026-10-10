package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	aiopsdao "dodevops-api/api/aiops/dao"
	aiopsmodel "dodevops-api/api/aiops/model"
	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	systemauthz "dodevops-api/api/system/authz"
	systemdao "dodevops-api/api/system/dao"
	systemmodel "dodevops-api/api/system/model"
	"dodevops-api/common/config"
	"dodevops-api/common/constant"

	"github.com/gin-gonic/gin"
)

const mcpDatabaseAssetType = "database"

var (
	ErrMCPUnauthorized     = errors.New("MCP_UNAUTHORIZED")
	ErrMCPPermissionDenied = errors.New("MCP_PERMISSION_DENIED")
	ErrMCPRateLimited      = errors.New("MCP_RATE_LIMITED")
	ErrMCPResultTooLarge   = errors.New("MCP_RESULT_TOO_LARGE")
)

type MCPReadHandler func(c *gin.Context, args map[string]interface{}, runID uint64) (interface{}, error)

type MCPToolExecutor struct {
	agentDao *aiopsdao.AgentDao
}

func NewMCPToolExecutor() *MCPToolExecutor {
	return &MCPToolExecutor{agentDao: aiopsdao.NewAgentDao()}
}

func (e *MCPToolExecutor) Execute(c *gin.Context, tool aiopsmodel.MCPToolView, args map[string]interface{}, handler MCPReadHandler) (interface{}, uint64, error) {
	return e.execute(c, tool, args, 0, false, handler)
}

// ExecuteAction applies the MCP policy and audit controls to a constrained
// mutation handler. The handler remains responsible for confirmation checks.
func (e *MCPToolExecutor) ExecuteAction(c *gin.Context, tool aiopsmodel.MCPToolView, args map[string]interface{}, handler MCPReadHandler) (interface{}, uint64, error) {
	return e.execute(c, tool, args, 0, true, handler)
}

// ExecuteInRun applies the same MCP policy and audit controls while appending
// steps and evidence to an existing assistant run.
func (e *MCPToolExecutor) ExecuteInRun(c *gin.Context, tool aiopsmodel.MCPToolView, args map[string]interface{}, runID uint64, handler MCPReadHandler) (interface{}, error) {
	if runID == 0 {
		return nil, errors.New("existing runId is required")
	}
	output, _, err := e.execute(c, tool, args, runID, false, handler)
	return output, err
}

func (e *MCPToolExecutor) execute(c *gin.Context, tool aiopsmodel.MCPToolView, args map[string]interface{}, runID uint64, allowMutation bool, handler MCPReadHandler) (interface{}, uint64, error) {
	if c == nil || handler == nil {
		return nil, 0, errors.New("invalid MCP execution context")
	}
	if !tool.Readonly && !allowMutation {
		return nil, 0, errors.New("mutation tools must be executed through Action Center")
	}
	if tool.Readonly && allowMutation {
		return nil, 0, errors.New("read-only tools cannot use the MCP action executor")
	}
	admin, err := mcpExecutionAdmin(c)
	if err != nil {
		return nil, 0, err
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	if err := ValidateMCPArguments(tool.InputSchema, args); err != nil {
		return nil, 0, fmt.Errorf("invalid arguments: %w", err)
	}
	if err := validateMCPEnvironment(tool.AllowedEnvironments, args); err != nil {
		return nil, 0, err
	}
	if !allowMCPRate(tool.Name, admin.ID, tool.RateLimitPerMinute, time.Now()) {
		return nil, 0, ErrMCPRateLimited
	}
	if err := authorizeMCPResourceScope(c, tool.Domain, args); err != nil {
		return nil, 0, err
	}

	ownedRun := runID == 0
	if ownedRun {
		now := time.Now()
		run := &aiopsmodel.AgentRun{
			RunType: aiopsmodel.AgentRunTypeMCPToolInvoke, Status: aiopsmodel.AgentRunStatusRunning,
			Summary:   fmt.Sprintf("MCP tool invocation: %s by %s(%d)", tool.Name, admin.Username, admin.ID),
			StartedAt: now, CreatedAt: now, UpdatedAt: now,
		}
		if err := e.agentDao.CreateRun(run); err != nil {
			return nil, 0, fmt.Errorf("create MCP audit run: %w", err)
		}
		runID = run.ID
	}

	started := time.Now()
	timeout := time.Duration(tool.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	requestContext, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()
	originalRequest := c.Request
	c.Request = c.Request.WithContext(requestContext)
	output, invokeErr := handler(c, args, runID)
	c.Request = originalRequest
	if invokeErr == nil {
		if contextErr := requestContext.Err(); contextErr != nil {
			invokeErr = contextErr
		}
	}
	finished := time.Now()

	step := aiopsmodel.AgentStep{
		StepName: "MCP 调用 " + tool.Name, ToolName: tool.Name,
		Status: aiopsmodel.AgentStepStatusSucceeded, InputJSON: marshalMCPAuditValue(redactMCPAuditInput(args)),
		StartedAt: started, FinishedAt: &finished, DurationMS: finished.Sub(started).Milliseconds(),
	}
	if invokeErr != nil {
		step.Status = aiopsmodel.AgentStepStatusFailed
		step.Error = invokeErr.Error()
		_ = e.agentDao.AppendRunStep(runID, step)
		if ownedRun {
			_ = e.agentDao.FinishRun(runID, aiopsmodel.AgentRunStatusFailed, invokeErr.Error(), finished)
		}
		return nil, runID, invokeErr
	}

	output = LimitMCPResultRows(RedactMCPValue(output), tool.MaxResultRows)
	payload, err := json.Marshal(output)
	if err != nil {
		err = fmt.Errorf("marshal MCP result: %w", err)
	} else if tool.MaxResultBytes > 0 && len(payload) > tool.MaxResultBytes {
		err = fmt.Errorf("%w: result is %d bytes, limit is %d", ErrMCPResultTooLarge, len(payload), tool.MaxResultBytes)
	}
	if err != nil {
		step.Status = aiopsmodel.AgentStepStatusFailed
		step.Error = err.Error()
		_ = e.agentDao.AppendRunStep(runID, step)
		if ownedRun {
			_ = e.agentDao.FinishRun(runID, aiopsmodel.AgentRunStatusFailed, err.Error(), finished)
		}
		return nil, runID, err
	}

	step.OutputJSON = string(payload)
	_ = e.agentDao.AppendRunStep(runID, step)
	evidenceContent := "MCP 只读工具调用成功"
	if !tool.Readonly {
		evidenceContent = "MCP 受控操作调用成功"
	}
	_ = e.agentDao.CreateEvidence(&aiopsmodel.AgentEvidence{
		RunID: runID, Type: tool.Domain, Title: tool.DisplayName,
		Reference: tool.Name, Content: evidenceContent, RawJSON: string(payload),
	})
	if ownedRun {
		_ = e.agentDao.FinishRun(runID, aiopsmodel.AgentRunStatusSucceeded, "", finished)
	}
	return output, runID, nil
}

func mcpExecutionAdmin(c *gin.Context) (*systemmodel.JwtAdmin, error) {
	value, ok := c.Get(constant.ContextKeyUserObj)
	admin, valid := value.(*systemmodel.JwtAdmin)
	if !ok || !valid || admin == nil || admin.ID == 0 {
		return nil, ErrMCPUnauthorized
	}
	return admin, nil
}

// InjectMCPServiceAccount attaches the configured MCP service account to an
// internal request context. Channel adapters use the same identity and RBAC
// scope as the public MCP endpoint.
func InjectMCPServiceAccount(c *gin.Context) error {
	if c == nil {
		return errors.New("MCP request context is required")
	}
	if config.Config == nil {
		return errors.New("MCP configuration is not loaded")
	}
	serviceUserID := config.Config.MCP.UserID
	if serviceUserID == 0 {
		defaultAdmin, err := systemdao.GetDefaultSystemAdmin()
		if err != nil {
			return fmt.Errorf("resolve default MCP service account: %w", err)
		}
		serviceUserID = defaultAdmin.ID
	}
	admin, _, err := systemdao.GetSysAdminAuthzState(serviceUserID)
	if err != nil {
		return fmt.Errorf("load MCP service account: %w", err)
	}
	if admin.Status != 1 {
		return errors.New("MCP service account is disabled")
	}
	c.Set(constant.ContextKeyUserObj, &systemmodel.JwtAdmin{
		ID: admin.ID, Username: admin.Username, Nickname: admin.Nickname,
		Icon: admin.Icon, Email: admin.Email, Note: admin.Note,
	})
	return nil
}

func authorizeMCPResourceScope(c *gin.Context, domain string, args map[string]interface{}) error {
	isAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return fmt.Errorf("resolve MCP service account scope: %w", err)
	}
	if isAdmin {
		return nil
	}
	admin, err := mcpExecutionAdmin(c)
	if err != nil {
		return err
	}
	assetID := mcpUintArgument(args, "hostId", "clusterId", "assetId", "projectId")
	if assetID == 0 {
		return nil // List/search handlers must filter their result set by the same identity.
	}

	switch domain {
	case "ecs":
		authDao := cmdbdao.NewBastionHostAuthorizationDao()
		ids, err := authDao.GetUserAuthorizedHostIds(admin.ID)
		if err != nil {
			return err
		}
		if !containsMCPUint(ids, assetID) {
			return ErrMCPPermissionDenied
		}
	case "k8s":
		authDao := cmdbdao.NewBastionAssetAuthorizationDao()
		allowed, err := authDao.CheckUserAssetAccess(admin.ID, cmdbmodel.AssetTypeK8SCluster, assetID)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrMCPPermissionDenied
		}
	case "database":
		authDao := cmdbdao.NewBastionAssetAuthorizationDao()
		allowed, err := authDao.CheckUserAssetAccess(admin.ID, mcpDatabaseAssetType, assetID)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrMCPPermissionDenied
		}
	}
	return nil
}

func validateMCPEnvironment(allowed []string, args map[string]interface{}) error {
	environment := strings.ToLower(strings.TrimSpace(mcpStringArgument(args, "environment", "env")))
	if environment == "" || len(allowed) == 0 {
		return nil
	}
	for _, item := range allowed {
		if strings.EqualFold(strings.TrimSpace(item), environment) {
			return nil
		}
	}
	return fmt.Errorf("environment %q is outside the configured MCP scope", environment)
}

func ValidateMCPArguments(schema map[string]interface{}, args map[string]interface{}) error {
	if schema == nil {
		return nil
	}
	if schemaType, _ := schema["type"].(string); schemaType != "" && schemaType != "object" {
		return fmt.Errorf("root schema must be object")
	}
	if required, ok := schema["required"].([]string); ok {
		for _, key := range required {
			value, exists := args[key]
			if !exists || value == nil || (reflect.ValueOf(value).Kind() == reflect.String && strings.TrimSpace(fmt.Sprint(value)) == "") {
				return fmt.Errorf("%s is required", key)
			}
		}
	} else if required, ok := schema["required"].([]interface{}); ok {
		for _, raw := range required {
			key := fmt.Sprint(raw)
			if value, exists := args[key]; !exists || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
				return fmt.Errorf("%s is required", key)
			}
		}
	}
	properties, _ := schema["properties"].(map[string]interface{})
	for key, value := range args {
		property, exists := properties[key]
		if !exists {
			if additional, ok := schema["additionalProperties"].(bool); ok && !additional {
				return fmt.Errorf("unknown argument %s", key)
			}
			continue
		}
		definition, _ := property.(map[string]interface{})
		if err := validateMCPArgumentType(key, value, definition); err != nil {
			return err
		}
	}
	return nil
}

func validateMCPArgumentType(name string, value interface{}, schema map[string]interface{}) error {
	if value == nil || schema == nil {
		return nil
	}
	typeName, _ := schema["type"].(string)
	valid := true
	switch typeName {
	case "string":
		_, valid = value.(string)
	case "number", "integer":
		switch value.(type) {
		case float64, float32, int, int32, int64, uint, uint32, uint64, json.Number:
			valid = true
		default:
			valid = false
		}
	case "boolean":
		_, valid = value.(bool)
	case "array":
		kind := reflect.ValueOf(value).Kind()
		valid = kind == reflect.Array || kind == reflect.Slice
	case "object":
		_, valid = value.(map[string]interface{})
	}
	if !valid {
		return fmt.Errorf("%s must be %s", name, typeName)
	}
	return nil
}

var mcpSensitiveFields = map[string]struct{}{
	"password": {}, "passwd": {}, "pwd": {}, "secret": {}, "token": {}, "authorization": {},
	"apikey": {}, "api_key": {}, "accesskey": {}, "access_key": {}, "privatekey": {}, "private_key": {},
	"credential": {}, "credentials": {}, "connectionstring": {}, "connection_string": {},
}

func redactMCPAuditInput(value interface{}) interface{} {
	return redactMCPConfirmationFields(RedactMCPValue(value))
}

func redactMCPConfirmationFields(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, item := range typed {
			normalizedKey := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
			if normalizedKey == "confirmationid" || normalizedKey == "confirmation_id" {
				typed[key] = "***REDACTED***"
				continue
			}
			typed[key] = redactMCPConfirmationFields(item)
		}
	case []interface{}:
		for index, item := range typed {
			typed[index] = redactMCPConfirmationFields(item)
		}
	}
	return value
}

func RedactMCPValue(value interface{}) interface{} {
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var normalized interface{}
	if err := json.Unmarshal(data, &normalized); err != nil {
		return value
	}
	return redactMCPNormalized(normalized)
}

func redactMCPNormalized(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			normalizedKey := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
			if _, sensitive := mcpSensitiveFields[normalizedKey]; sensitive || strings.Contains(normalizedKey, "password") || strings.Contains(normalizedKey, "secret") {
				result[key] = "***REDACTED***"
				continue
			}
			result[key] = redactMCPNormalized(item)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(typed))
		for index, item := range typed {
			result[index] = redactMCPNormalized(item)
		}
		return result
	default:
		return value
	}
}

func LimitMCPResultRows(value interface{}, maxRows int) interface{} {
	if maxRows <= 0 {
		return value
	}
	remaining := maxRows
	var visit func(interface{}) interface{}
	visit = func(current interface{}) interface{} {
		switch typed := current.(type) {
		case []interface{}:
			limit := len(typed)
			if limit > remaining {
				limit = remaining
			}
			remaining -= limit
			result := make([]interface{}, 0, limit)
			for index := 0; index < limit; index++ {
				result = append(result, visit(typed[index]))
			}
			return result
		case map[string]interface{}:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			result := make(map[string]interface{}, len(typed))
			for _, key := range keys {
				result[key] = visit(typed[key])
			}
			return result
		default:
			return current
		}
	}
	return visit(value)
}

type mcpRateWindow struct {
	started time.Time
	count   int
}

var mcpRateState = struct {
	sync.Mutex
	items map[string]mcpRateWindow
}{items: map[string]mcpRateWindow{}}

func allowMCPRate(toolName string, userID uint, limit int, now time.Time) bool {
	if limit <= 0 {
		return true
	}
	key := fmt.Sprintf("%d:%s", userID, toolName)
	mcpRateState.Lock()
	defer mcpRateState.Unlock()
	window := mcpRateState.items[key]
	if window.started.IsZero() || now.Sub(window.started) >= time.Minute {
		mcpRateState.items[key] = mcpRateWindow{started: now, count: 1}
		return true
	}
	if window.count >= limit {
		return false
	}
	window.count++
	mcpRateState.items[key] = window
	return true
}

func mcpUintArgument(args map[string]interface{}, keys ...string) uint {
	for _, key := range keys {
		value, ok := args[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			if typed > 0 {
				return uint(typed)
			}
		case int:
			if typed > 0 {
				return uint(typed)
			}
		case uint:
			return typed
		case json.Number:
			if parsed, err := typed.Int64(); err == nil && parsed > 0 {
				return uint(parsed)
			}
		}
	}
	return 0
}

func mcpStringArgument(args map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := args[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func containsMCPUint(items []uint, target uint) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func marshalMCPAuditValue(value interface{}) string {
	data, _ := json.Marshal(value)
	return string(data)
}
