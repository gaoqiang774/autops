package model

const (
	AgentRunTypeToolInvoke = "tool_invoke"

	AgentToolRiskReadonly = "readonly"
	AgentToolRiskLow      = "low"
	AgentToolRiskMedium   = "medium"
	AgentToolRiskHigh     = "high"

	AgentToolDecisionAllow = "allow"
	AgentToolDecisionDeny  = "deny"
)

type AgentToolDefinition struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	RiskLevel    string                 `json:"riskLevel"`
	Readonly     bool                   `json:"readonly"`
	InputSchema  map[string]interface{} `json:"inputSchema,omitempty"`
	OutputSchema map[string]interface{} `json:"outputSchema,omitempty"`
}

type AgentToolInvokeRequest struct {
	RunID     uint64                 `json:"runId"`
	ToolName  string                 `json:"toolName" binding:"required"`
	Arguments map[string]interface{} `json:"arguments"`
	Context   AgentToolInvokeContext `json:"context"`
}

type AgentToolInvokeContext struct {
	AgentID      string `json:"agentId,omitempty"`
	AgentRuntime string `json:"agentRuntime,omitempty"`
	MCPToolName  string `json:"-"`
	TaskID       string `json:"taskId,omitempty"`
	UserPrompt   string `json:"userPrompt,omitempty"`
	TraceID      string `json:"traceId,omitempty"`
}

type AgentToolPolicyDecision struct {
	Decision string   `json:"decision"`
	Reason   string   `json:"reason"`
	Risk     string   `json:"risk"`
	Readonly bool     `json:"readonly"`
	Checks   []string `json:"checks,omitempty"`
}

type AgentToolInvokeResult struct {
	RunID      uint64                  `json:"runId"`
	ToolName   string                  `json:"toolName"`
	Decision   AgentToolPolicyDecision `json:"decision"`
	Output     interface{}             `json:"output,omitempty"`
	Evidence   *AgentDiagnosisEvidence `json:"evidence,omitempty"`
	StartedAt  string                  `json:"startedAt"`
	FinishedAt string                  `json:"finishedAt"`
	DurationMS int64                   `json:"durationMs"`
}
