package service

import (
	"encoding/json"
	"errors"
	"testing"

	aiopsmodel "dodevops-api/api/aiops/model"
	cmdbmodel "dodevops-api/api/cmdb/model"
)

func newMCPCatalogTestService(t *testing.T) *MCPCatalogService {
	t.Helper()
	return NewMCPCatalogServiceWithDB(nil)
}

func TestMCPCatalogDefaultsExposeOnlySupportedTools(t *testing.T) {
	service := newMCPCatalogTestService(t)
	tools, err := service.ListCallableTools("ecs")
	if err != nil {
		t.Fatalf("list callable tools: %v", err)
	}
	if !containsMCPTool(tools, "ecs_get_host") {
		t.Fatal("supported ECS tool should be callable by default")
	}
	if containsMCPTool(tools, "ecs_request_reboot") {
		t.Fatal("unsupported mutation tool must not be callable")
	}
	servers, err := service.ListServers("")
	if err != nil {
		t.Fatalf("list servers: %v", err)
	}
	if len(servers) != 7 {
		t.Fatalf("got %d MCP domains, want 7", len(servers))
	}
}

func TestMCPCatalogDisabledToolIsHiddenAndRejected(t *testing.T) {
	servers := buildMCPServerViews("ecs", nil, map[string]aiopsmodel.MCPToolConfig{
		mcpToolConfigKey("ecs", "ecs_get_host"): {ServerCode: "ecs", ToolName: "ecs_get_host", Enabled: false, RiskLevel: "R0"},
	})
	tools := callableToolsFromServers(servers)
	if containsMCPTool(tools, "ecs_get_host") {
		t.Fatal("disabled tool must be absent from tools/list")
	}
	tool, server, ok := findMCPToolView(servers, "ecs_get_host")
	if !ok || !server.Enabled || tool.Enabled {
		t.Fatal("disabled tool policy was not applied")
	}
	if _, err := authorizeMCPTool(servers, "ecs_get_host"); !errors.Is(err, ErrMCPToolDisabled) {
		t.Fatalf("authorize disabled tool error = %v, want TOOL_DISABLED", err)
	}
}

func TestMCPCatalogMasterSwitchBlocksDomain(t *testing.T) {
	servers := buildMCPServerViews("ecs", map[string]aiopsmodel.MCPServerConfig{
		"ecs": {Code: "ecs", Enabled: false, ConfigVersion: 1},
	}, nil)
	if servers[0].Enabled {
		t.Fatal("server should be disabled on its first persisted override")
	}
	tools := callableToolsFromServers(servers)
	if len(tools) != 0 {
		t.Fatalf("disabled server exposed %d tools", len(tools))
	}
	if _, err := authorizeMCPTool(servers, "ecs_get_host"); !errors.Is(err, ErrMCPServerDisabled) {
		t.Fatalf("authorize tool error = %v, want MCP_SERVER_DISABLED", err)
	}
}

func TestGatewayToolUsesCanonicalMCPPolicy(t *testing.T) {
	servers := buildMCPServerViews("ecs", nil, map[string]aiopsmodel.MCPToolConfig{
		mcpToolConfigKey("ecs", "ecs_get_agent_status"): {
			ServerCode: "ecs", ToolName: "ecs_get_agent_status", Enabled: false, RiskLevel: "R0",
		},
	})
	if _, err := authorizeGatewayMCPTool(servers, "get_agent_status", ""); !errors.Is(err, ErrMCPToolDisabled) {
		t.Fatalf("authorize legacy Agent Tool error = %v, want TOOL_DISABLED", err)
	}
}

func TestGatewayToolHonorsCanonicalMCPMasterSwitch(t *testing.T) {
	servers := buildMCPServerViews("ecs", map[string]aiopsmodel.MCPServerConfig{
		"ecs": {Code: "ecs", Enabled: false, ConfigVersion: 1},
	}, nil)
	if _, err := authorizeGatewayMCPTool(servers, "get_agent_status", ""); !errors.Is(err, ErrMCPServerDisabled) {
		t.Fatalf("authorize legacy Agent Tool error = %v, want MCP_SERVER_DISABLED", err)
	}
}

func TestGatewayExplicitMCPToolCannotChangeHandler(t *testing.T) {
	servers := buildMCPServerViews("ecs", nil, nil)
	if _, err := authorizeGatewayMCPTool(servers, "get_agent_status", "ecs_get_host"); err == nil {
		t.Fatal("explicit MCP Tool backed by another handler must be rejected")
	}
	if tool, err := authorizeGatewayMCPTool(servers, "get_cmdb_host", "ecs_get_configuration"); err != nil || tool.Name != "ecs_get_configuration" {
		t.Fatalf("shared handler's exact MCP policy was not resolved: tool=%s err=%v", tool.Name, err)
	}
}

func TestGatewayRejectsUnmappedLegacyTool(t *testing.T) {
	servers := buildMCPServerViews("", nil, nil)
	if _, err := authorizeGatewayMCPTool(servers, "unmapped_tool", ""); !errors.Is(err, ErrMCPToolUnsupported) {
		t.Fatalf("authorize unmapped Agent Tool error = %v, want TOOL_UNSUPPORTED", err)
	}
}

func TestGatewayMCPToolNameCannotBeSpoofedByPublicJSON(t *testing.T) {
	var req aiopsmodel.AgentToolInvokeRequest
	if err := json.Unmarshal([]byte(`{"toolName":"get_cmdb_host","context":{"mcpToolName":"ecs_get_configuration"}}`), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.Context.MCPToolName != "" {
		t.Fatalf("public JSON set trusted MCP tool name to %q", req.Context.MCPToolName)
	}
}

func TestAgentHostViewsDoNotCrossConfigurationBoundary(t *testing.T) {
	host := cmdbmodel.CmdbHost{ID: 7, HostName: "app-01", CPU: "8", Memory: "16GB", Disk: "200GB"}
	detail := toAgentHostContext(host)
	for _, field := range []string{"cpu", "memory", "disk"} {
		if _, ok := detail[field]; ok {
			t.Fatalf("host detail exposed configuration field %s", field)
		}
	}
	configuration := toAgentHostConfiguration(host)
	for _, field := range []string{"cpu", "memory", "disk"} {
		if _, ok := configuration[field]; !ok {
			t.Fatalf("configuration view omitted field %s", field)
		}
	}
}

func TestMCPCatalogRejectsRiskDowngrade(t *testing.T) {
	service := newMCPCatalogTestService(t)
	risk := "R0"
	if _, err := service.UpdateTool("log", "log_search", aiopsmodel.MCPToolConfigRequest{RiskLevel: &risk}, 1); err == nil {
		t.Fatal("risk downgrade below code floor should fail")
	}
}

func TestMCPCatalogEveryDomainHasCallableReadonlyTool(t *testing.T) {
	service := newMCPCatalogTestService(t)
	expectedCounts := map[string]int{"ecs": 10, "k8s": 21, "database": 8, "log": 6, "alert": 6, "knowledge": 9, "asset": 2}
	for _, domain := range []string{"ecs", "k8s", "database", "log", "alert", "knowledge", "asset"} {
		tools, err := service.ListCallableTools(domain)
		if err != nil {
			t.Fatalf("list %s tools: %v", domain, err)
		}
		if len(tools) != expectedCounts[domain] {
			t.Fatalf("domain %s has %d callable tools, want %d", domain, len(tools), expectedCounts[domain])
		}
		for _, tool := range tools {
			if !tool.Readonly {
				t.Fatalf("domain %s exposed mutation tool %s", domain, tool.Name)
			}
		}
	}
}

func TestMCPReadToolSchemasRequireResourceIdentity(t *testing.T) {
	tools := map[string]string{
		"db_search_objects": "database", "db_get_table_schema": "database",
		"db_execute_readonly_sql": "database", "log_search": "log",
		"alert_get_detail": "alert", "knowledge_get_document": "knowledge",
		"k8s_get_workload_history": "k8s",
	}
	for name, domain := range tools {
		definition, ok := findMCPToolDefinition(domain, name)
		if !ok {
			t.Fatalf("missing definition for %s", name)
		}
		if err := ValidateMCPArguments(definition.InputSchema, map[string]interface{}{}); err == nil {
			t.Fatalf("%s accepted empty arguments", name)
		}
	}
}

func containsMCPTool(tools []aiopsmodel.MCPToolView, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func callableToolsFromServers(servers []aiopsmodel.MCPServerView) []aiopsmodel.MCPToolView {
	result := make([]aiopsmodel.MCPToolView, 0)
	for _, server := range servers {
		if !server.Enabled {
			continue
		}
		for _, tool := range server.Tools {
			if tool.Enabled && tool.Supported {
				result = append(result, tool)
			}
		}
	}
	return result
}

func findMCPToolView(servers []aiopsmodel.MCPServerView, name string) (aiopsmodel.MCPToolView, aiopsmodel.MCPServerView, bool) {
	for _, server := range servers {
		for _, tool := range server.Tools {
			if tool.Name == name {
				return tool, server, true
			}
		}
	}
	return aiopsmodel.MCPToolView{}, aiopsmodel.MCPServerView{}, false
}
