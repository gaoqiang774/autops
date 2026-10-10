package model

import "time"

const (
	AgentRunTypeAlertDiagnosis = "alert_diagnosis"
	AgentRunTypeAssistantChat  = "assistant_chat"
	AgentRunTypeSkillRuntime   = "skill_runtime"
	AgentRunTypeMCPToolInvoke  = "mcp_tool_invoke"

	AgentRunStatusRunning   = "running"
	AgentRunStatusSucceeded = "succeeded"
	AgentRunStatusFailed    = "failed"
	AgentRunStatusPartial   = "partial"

	AgentStepStatusRunning   = "running"
	AgentStepStatusSucceeded = "succeeded"
	AgentStepStatusFailed    = "failed"

	AgentMessageRoleUser      = "user"
	AgentMessageRoleAssistant = "assistant"
	AgentMessageRoleSystem    = "system"

	AgentFeedbackUseful       = "useful"
	AgentFeedbackNotUseful    = "not_useful"
	AgentFeedbackWrong        = "wrong"
	AgentFeedbackInsufficient = "insufficient_evidence"
	AgentFeedbackToolError    = "tool_error"
)

type AgentSession struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	SessionID string    `gorm:"type:varchar(128);not null;uniqueIndex;column:session_id" json:"sessionId"`
	Title     string    `gorm:"type:varchar(255);column:title" json:"title,omitempty"`
	Channel   string    `gorm:"type:varchar(64);column:channel;index" json:"channel,omitempty"`
	UserID    uint      `gorm:"column:user_id;index" json:"userId,omitempty"`
	Username  string    `gorm:"type:varchar(128);column:username;index" json:"username,omitempty"`
	Status    string    `gorm:"type:varchar(32);not null;index;column:status" json:"status"`
	LastRunID uint64    `gorm:"column:last_run_id;index" json:"lastRunId,omitempty"`
	CreatedAt time.Time `gorm:"type:datetime;not null;column:created_at;index" json:"createdAt"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;column:updated_at" json:"updatedAt"`
}

func (AgentSession) TableName() string {
	return "aiops_agent_session"
}

type AgentMessage struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	SessionID     string    `gorm:"type:varchar(128);not null;index;column:session_id" json:"sessionId"`
	RunID         uint64    `gorm:"column:run_id;index" json:"runId,omitempty"`
	Role          string    `gorm:"type:varchar(32);not null;index;column:role" json:"role"`
	Content       string    `gorm:"type:longtext;column:content" json:"content"`
	Intent        string    `gorm:"type:varchar(64);column:intent;index" json:"intent,omitempty"`
	EntitiesJSON  string    `gorm:"type:longtext;column:entities_json" json:"entitiesJson,omitempty"`
	ToolPlanJSON  string    `gorm:"type:longtext;column:tool_plan_json" json:"toolPlanJson,omitempty"`
	TimelineJSON  string    `gorm:"type:longtext;column:timeline_json" json:"timelineJson,omitempty"`
	DiagnosisJSON string    `gorm:"type:longtext;column:diagnosis_json" json:"diagnosisJson,omitempty"`
	RawJSON       string    `gorm:"type:longtext;column:raw_json" json:"rawJson,omitempty"`
	CreatedAt     time.Time `gorm:"type:datetime;not null;column:created_at;index" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"type:datetime;not null;column:updated_at" json:"updatedAt"`
}

func (AgentMessage) TableName() string {
	return "aiops_agent_message"
}

type AgentRun struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	RunType      string     `gorm:"type:varchar(64);not null;index;column:run_type" json:"runType"`
	Status       string     `gorm:"type:varchar(32);not null;index;column:status" json:"status"`
	AlertID      uint64     `gorm:"column:alert_id;index" json:"alertId,omitempty"`
	Fingerprint  string     `gorm:"type:varchar(64);column:fingerprint;index" json:"fingerprint,omitempty"`
	ProviderID   uint       `gorm:"column:provider_id;index" json:"providerId,omitempty"`
	ProviderName string     `gorm:"type:varchar(128);column:provider_name" json:"providerName,omitempty"`
	ProviderType string     `gorm:"type:varchar(64);column:provider_type" json:"providerType,omitempty"`
	ModelCode    string     `gorm:"type:varchar(128);column:model_code" json:"modelCode,omitempty"`
	Summary      string     `gorm:"type:text;column:summary" json:"summary,omitempty"`
	Severity     string     `gorm:"type:varchar(32);column:severity" json:"severity,omitempty"`
	StepsJSON    string     `gorm:"type:longtext;column:steps_json" json:"stepsJson,omitempty"`
	ResultJSON   string     `gorm:"type:longtext;column:result_json" json:"resultJson,omitempty"`
	UsedLLM      bool       `gorm:"column:used_llm" json:"usedLlm"`
	Error        string     `gorm:"type:text;column:error" json:"error,omitempty"`
	StartedAt    time.Time  `gorm:"type:datetime;not null;column:started_at;index" json:"startedAt"`
	FinishedAt   *time.Time `gorm:"type:datetime;column:finished_at" json:"finishedAt,omitempty"`
	DurationMS   int64      `gorm:"column:duration_ms" json:"durationMs"`
	GeneratedAt  *time.Time `gorm:"type:datetime;column:generated_at;index" json:"generatedAt,omitempty"`
	CreatedAt    time.Time  `gorm:"type:datetime;not null;column:created_at;index" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"type:datetime;not null;column:updated_at" json:"updatedAt"`
}

func (AgentRun) TableName() string {
	return "aiops_agent_run"
}

type AgentStep struct {
	StepName   string     `json:"stepName"`
	ToolName   string     `json:"toolName"`
	Status     string     `json:"status"`
	InputJSON  string     `json:"inputJson,omitempty"`
	OutputJSON string     `json:"outputJson,omitempty"`
	Error      string     `json:"error,omitempty"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	DurationMS int64      `json:"durationMs"`
}

type AgentEvidence struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	RunID     uint64    `gorm:"not null;column:run_id;index" json:"runId"`
	Type      string    `gorm:"type:varchar(32);not null;index;column:type" json:"type"`
	Title     string    `gorm:"type:varchar(255);not null;column:title" json:"title"`
	Reference string    `gorm:"type:varchar(255);column:reference" json:"reference,omitempty"`
	Content   string    `gorm:"type:text;column:content" json:"content"`
	RawJSON   string    `gorm:"type:longtext;column:raw_json" json:"rawJson,omitempty"`
	CreatedAt time.Time `gorm:"type:datetime;not null;column:created_at;index" json:"createdAt"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;column:updated_at" json:"updatedAt"`
}

func (AgentEvidence) TableName() string {
	return "aiops_evidence"
}

type AgentFeedback struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	RunID     uint64    `gorm:"not null;column:run_id;index" json:"runId"`
	SessionID string    `gorm:"type:varchar(128);column:session_id;index" json:"sessionId,omitempty"`
	Rating    string    `gorm:"type:varchar(64);not null;column:rating;index" json:"rating"`
	Reason    string    `gorm:"type:varchar(255);column:reason" json:"reason,omitempty"`
	Comment   string    `gorm:"type:text;column:comment" json:"comment,omitempty"`
	Intent    string    `gorm:"type:varchar(64);column:intent;index" json:"intent,omitempty"`
	UserID    uint      `gorm:"column:user_id;index" json:"userId,omitempty"`
	Username  string    `gorm:"type:varchar(128);column:username;index" json:"username,omitempty"`
	RawJSON   string    `gorm:"type:longtext;column:raw_json" json:"rawJson,omitempty"`
	CreatedAt time.Time `gorm:"type:datetime;not null;column:created_at;index" json:"createdAt"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;column:updated_at" json:"updatedAt"`
}

func (AgentFeedback) TableName() string {
	return "aiops_agent_feedback"
}

type AgentFeedbackRequest struct {
	RunID     uint64                 `json:"runId" binding:"required"`
	SessionID string                 `json:"sessionId"`
	Rating    string                 `json:"rating" binding:"required"`
	Reason    string                 `json:"reason"`
	Comment   string                 `json:"comment"`
	Intent    string                 `json:"intent"`
	Context   map[string]interface{} `json:"context"`
}

type AlertAnalyzeRequest struct {
	AlertID            uint64   `json:"alertId"`
	Fingerprint        string   `json:"fingerprint"`
	ProviderID         uint     `json:"providerId"`
	HostID             uint     `json:"hostId"`
	AuthID             uint     `json:"authId"`
	TimeRangeMinutes   int      `json:"timeRangeMinutes"`
	IncludeLogs        bool     `json:"includeLogs"`
	IncludeRawEvidence bool     `json:"includeRawEvidence"`
	LogPaths           []string `json:"logPaths"`
	LogTailLines       int      `json:"logTailLines"`
}

type AgentDiagnosisEvidence struct {
	Type      string      `json:"type"`
	Title     string      `json:"title"`
	Reference string      `json:"reference,omitempty"`
	Content   string      `json:"content"`
	Raw       interface{} `json:"raw,omitempty"`
}

type AgentDiagnosisHypothesis struct {
	Cause              string   `json:"cause"`
	Confidence         string   `json:"confidence"`
	SupportingEvidence []string `json:"supportingEvidence"`
}

type AlertDiagnosisResult struct {
	RunID              uint64                     `json:"runId"`
	Summary            string                     `json:"summary"`
	Severity           string                     `json:"severity"`
	ImpactedResources  []string                   `json:"impactedResources"`
	Evidence           []AgentDiagnosisEvidence   `json:"evidence"`
	Hypotheses         []AgentDiagnosisHypothesis `json:"hypotheses"`
	NextChecks         []string                   `json:"nextChecks"`
	Remediation        []string                   `json:"remediation"`
	RunbookSuggestions []string                   `json:"runbookSuggestions"`
	HandoffNote        string                     `json:"handoffNote"`
	MissingData        []string                   `json:"missingData"`
	Readonly           bool                       `json:"readonly"`
	UsedLLM            bool                       `json:"usedLlm"`
	ProviderID         uint                       `json:"providerId,omitempty"`
	ProviderName       string                     `json:"providerName,omitempty"`
	ProviderType       string                     `json:"providerType,omitempty"`
	ModelCode          string                     `json:"modelCode,omitempty"`
	GeneratedAt        string                     `json:"generatedAt"`
}

type AgentRunDetail struct {
	Run             AgentRun                 `json:"run"`
	Steps           []AgentStep              `json:"steps"`
	Evidence        []AgentDiagnosisEvidence `json:"evidence"`
	Result          *AlertDiagnosisResult    `json:"result,omitempty"`
	AssistantResult *AssistantChatResult     `json:"assistantResult,omitempty"`
}

type AgentRunStreamEvent struct {
	Type      string      `json:"type"`
	RunID     uint64      `json:"runId"`
	Status    string      `json:"status,omitempty"`
	StepIndex int         `json:"stepIndex,omitempty"`
	Step      *AgentStep  `json:"step,omitempty"`
	Result    interface{} `json:"result,omitempty"`
	Message   string      `json:"message,omitempty"`
	Error     string      `json:"error,omitempty"`
	Time      string      `json:"time"`
}
