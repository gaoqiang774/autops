package model

import "time"

const (
	MCPRiskR0 = "R0"
	MCPRiskR1 = "R1"
	MCPRiskR2 = "R2"
	MCPRiskR3 = "R3"
	MCPRiskR4 = "R4"
)

// MCPServerConfig stores operational overrides for a code-owned MCP domain.
type MCPServerConfig struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Code          string    `gorm:"size:32;uniqueIndex;not null" json:"code"`
	Enabled       bool      `gorm:"not null" json:"enabled"`
	ConfigVersion uint64    `gorm:"not null;default:1" json:"configVersion"`
	CreatedBy     uint      `json:"createdBy"`
	UpdatedBy     uint      `json:"updatedBy"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (MCPServerConfig) TableName() string { return "aiops_mcp_server" }

// MCPToolConfig stores only policy overrides. Handler code and schemas are not configurable.
type MCPToolConfig struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	ServerCode           string    `gorm:"size:32;uniqueIndex:uk_aiops_mcp_tool,priority:1;not null" json:"serverCode"`
	ToolName             string    `gorm:"size:128;uniqueIndex:uk_aiops_mcp_tool,priority:2;not null" json:"toolName"`
	Enabled              bool      `gorm:"not null;default:false" json:"enabled"`
	RiskLevel            string    `gorm:"size:8" json:"riskLevel"`
	RequiredConfirmation bool      `gorm:"not null;default:false" json:"requiredConfirmation"`
	TimeoutSeconds       int       `gorm:"not null;default:30" json:"timeoutSeconds"`
	RateLimitPerMinute   int       `gorm:"not null;default:60" json:"rateLimitPerMinute"`
	MaxResultRows        int       `gorm:"not null;default:1000" json:"maxResultRows"`
	MaxResultBytes       int       `gorm:"not null;default:1048576" json:"maxResultBytes"`
	AllowedEnvironments  string    `gorm:"type:text" json:"-"`
	ConfigVersion        uint64    `gorm:"not null;default:1" json:"configVersion"`
	CreatedBy            uint      `json:"createdBy"`
	UpdatedBy            uint      `json:"updatedBy"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

func (MCPToolConfig) TableName() string { return "aiops_mcp_tool_config" }

type MCPToolDefinition struct {
	Domain             string                 `json:"domain"`
	Name               string                 `json:"name"`
	DisplayName        string                 `json:"displayName"`
	Description        string                 `json:"description"`
	Category           string                 `json:"category"`
	HandlerKey         string                 `json:"-"`
	InputSchema        map[string]interface{} `json:"inputSchema"`
	Readonly           bool                   `json:"readonly"`
	RiskFloor          string                 `json:"riskFloor"`
	RequiredPermission string                 `json:"requiredPermission,omitempty"`
	Supported          bool                   `json:"supported"`
	Version            string                 `json:"version"`
}

type MCPToolView struct {
	MCPToolDefinition
	Enabled              bool     `json:"enabled"`
	EffectiveRisk        string   `json:"effectiveRisk"`
	RequiredConfirmation bool     `json:"requiredConfirmation"`
	TimeoutSeconds       int      `json:"timeoutSeconds"`
	RateLimitPerMinute   int      `json:"rateLimitPerMinute"`
	MaxResultRows        int      `json:"maxResultRows"`
	MaxResultBytes       int      `json:"maxResultBytes"`
	AllowedEnvironments  []string `json:"allowedEnvironments"`
	ConfigVersion        uint64   `json:"configVersion"`
}

type MCPServerView struct {
	Code           string        `json:"code"`
	DisplayName    string        `json:"displayName"`
	Description    string        `json:"description"`
	Icon           string        `json:"icon"`
	Enabled        bool          `json:"enabled"`
	Version        string        `json:"version"`
	ConfigVersion  uint64        `json:"configVersion"`
	Tools          []MCPToolView `json:"tools"`
	EnabledTools   int           `json:"enabledTools"`
	SupportedTools int           `json:"supportedTools"`
	TotalTools     int           `json:"totalTools"`
}

type MCPServerStatusRequest struct {
	Enabled bool `json:"enabled"`
}

type MCPToolConfigRequest struct {
	Enabled              *bool     `json:"enabled"`
	RiskLevel            *string   `json:"riskLevel"`
	RequiredConfirmation *bool     `json:"requiredConfirmation"`
	TimeoutSeconds       *int      `json:"timeoutSeconds"`
	RateLimitPerMinute   *int      `json:"rateLimitPerMinute"`
	MaxResultRows        *int      `json:"maxResultRows"`
	MaxResultBytes       *int      `json:"maxResultBytes"`
	AllowedEnvironments  *[]string `json:"allowedEnvironments"`
}
