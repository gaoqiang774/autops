package model

const (
	AssistantChatIntentHostPressure        = "host_pressure"
	AssistantChatIntentHostInventory       = "host_inventory"
	AssistantChatIntentHostLogCleanup      = "host_log_cleanup"
	AssistantChatIntentUnsupportedFileOp   = "unsupported_file_operation"
	AssistantChatIntentAlertOverview       = "alert_overview"
	AssistantChatIntentHostServices        = "host_services"
	AssistantChatIntentPlatformOverview    = "platform_overview"
	AssistantChatIntentDatabaseOverview    = "database_overview"
	AssistantChatIntentK8sOverview         = "k8s_overview"
	AssistantChatIntentK8sRolloutRestart   = "k8s_rollout_restart"
	AssistantChatIntentK8sScaleWorkload    = "k8s_scale_workload"
	AssistantChatIntentK8sRollbackWorkload = "k8s_rollback_workload"
	AssistantChatIntentKnowledgeSearch     = "knowledge_search"
	AssistantChatIntentSkillCatalog        = "skill_catalog"
	AssistantChatIntentMCPCatalog          = "mcp_catalog"
	AssistantChatIntentActionConfirmation  = "action_confirmation"
	AssistantChatIntentResourceGovernance  = "resource_governance"
	AssistantChatIntentOpsGeneral          = "ops_general"
	AssistantChatIntentGeneralQA           = "general_qa"
	AssistantChatIntentUnknown             = "unknown"
	AssistantChatIntentClarify             = "clarify"

	AssistantChatSeverityInfo     = "info"
	AssistantChatSeverityWarning  = "warning"
	AssistantChatSeverityCritical = "critical"
	AssistantChatSeverityUnknown  = "unknown"

	AssistantChatPlanPending   = "pending"
	AssistantChatPlanRunning   = "running"
	AssistantChatPlanSucceeded = "succeeded"
	AssistantChatPlanFailed    = "failed"
	AssistantChatPlanSkipped   = "skipped"
)

type AssistantChatRequest struct {
	SessionID  string                 `json:"sessionId"`
	Question   string                 `json:"question" binding:"required"`
	ProviderID uint                   `json:"providerId"`
	Mode       string                 `json:"mode"`
	Context    map[string]interface{} `json:"context"`
}

type AssistantChatHistoryClearRequest struct {
	SessionID string `json:"sessionId"`
}

type AssistantChatHistoryClearResult struct {
	SessionID       string `json:"sessionId,omitempty"`
	ClearedAll      bool   `json:"clearedAll"`
	DeletedSessions int64  `json:"deletedSessions"`
	DeletedMessages int64  `json:"deletedMessages"`
	DeletedRuns     int64  `json:"deletedRuns"`
	DeletedEvidence int64  `json:"deletedEvidence"`
}

type AssistantChatUsedTool struct {
	Name       string                 `json:"name"`
	Arguments  map[string]interface{} `json:"arguments,omitempty"`
	DurationMS int64                  `json:"durationMs"`
	CallCount  int                    `json:"callCount,omitempty"`
	Success    bool                   `json:"success"`
	Error      string                 `json:"error,omitempty"`
}

type AssistantChatToolPlanStep struct {
	Step      int                    `json:"step"`
	ToolName  string                 `json:"toolName"`
	Purpose   string                 `json:"purpose"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	Status    string                 `json:"status"`
	Error     string                 `json:"error,omitempty"`
}

type AssistantInvestigationPlan struct {
	Goal        string                       `json:"goal"`
	Strategy    string                       `json:"strategy"`
	Steps       []AssistantInvestigationStep `json:"steps"`
	Budget      AssistantInvestigationBudget `json:"budget"`
	Assumptions []string                     `json:"assumptions,omitempty"`
}

type AssistantInvestigationStep struct {
	Step      int      `json:"step"`
	Name      string   `json:"name"`
	Purpose   string   `json:"purpose"`
	Tools     []string `json:"tools,omitempty"`
	Required  bool     `json:"required"`
	Status    string   `json:"status"`
	DependsOn []int    `json:"dependsOn,omitempty"`
}

type AssistantInvestigationBudget struct {
	MaxSteps       int `json:"maxSteps"`
	TimeoutSeconds int `json:"timeoutSeconds"`
	MaxToolCalls   int `json:"maxToolCalls"`
}

type AssistantReflection struct {
	CompletenessScore int      `json:"completenessScore"`
	Confidence        string   `json:"confidence"`
	SupportedBy       []string `json:"supportedBy,omitempty"`
	MissingEvidence   []string `json:"missingEvidence,omitempty"`
	Risks             []string `json:"risks,omitempty"`
	Notes             []string `json:"notes,omitempty"`
}

type AssistantChatProgress struct {
	Percent     int                         `json:"percent"`
	CurrentStep int                         `json:"currentStep"`
	Status      string                      `json:"status"`
	Steps       []AssistantChatProgressStep `json:"steps"`
}

type AssistantChatProgressStep struct {
	Step       int    `json:"step"`
	Key        string `json:"key"`
	Name       string `json:"name"`
	ToolName   string `json:"toolName,omitempty"`
	Purpose    string `json:"purpose,omitempty"`
	Status     string `json:"status"`
	Percent    int    `json:"percent"`
	Required   bool   `json:"required"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	DurationMS int64  `json:"durationMs,omitempty"`
	Error      string `json:"error,omitempty"`
}

type AssistantChatTimelineEvent struct {
	Step       int    `json:"step"`
	ToolName   string `json:"toolName"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	DurationMS int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

// AssistantChatThinkingProcess is an auditable execution trace for UI display.
// It contains observable routing and tool activity, not private model reasoning.
type AssistantChatThinkingProcess struct {
	Status         string                             `json:"status"`
	StartedAt      string                             `json:"startedAt"`
	FinishedAt     string                             `json:"finishedAt,omitempty"`
	DurationMS     int64                              `json:"durationMs"`
	ToolCallCount  int                                `json:"toolCallCount"`
	CompletedSteps int                                `json:"completedSteps"`
	TotalSteps     int                                `json:"totalSteps"`
	Items          []AssistantChatThinkingProcessItem `json:"items"`
}

type AssistantChatThinkingProcessItem struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	Time        string `json:"time,omitempty"`
	StartedAt   string `json:"startedAt,omitempty"`
	FinishedAt  string `json:"finishedAt,omitempty"`
	DurationMS  int64  `json:"durationMs,omitempty"`
	ToolName    string `json:"toolName,omitempty"`
	Error       string `json:"error,omitempty"`
}

type AssistantChatDiagnosisCard struct {
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
}

type AssistantChatMemoryMessage struct {
	Role      string                 `json:"role"`
	Intent    string                 `json:"intent,omitempty"`
	Content   string                 `json:"content,omitempty"`
	Entities  map[string]interface{} `json:"entities,omitempty"`
	CreatedAt string                 `json:"createdAt,omitempty"`
}

type AssistantChatMemoryContext struct {
	Used         bool                         `json:"used"`
	MessageCount int                          `json:"messageCount"`
	Entities     map[string]interface{}       `json:"entities,omitempty"`
	Messages     []AssistantChatMemoryMessage `json:"messages,omitempty"`
}

type AssistantChatClarifyQuestion struct {
	Field       string   `json:"field"`
	Question    string   `json:"question"`
	Suggestions []string `json:"suggestions,omitempty"`
}

type AssistantChatAsyncTask struct {
	Type        string `json:"type"`
	TaskID      string `json:"taskId"`
	Status      string `json:"status"`
	CurrentStep string `json:"currentStep,omitempty"`
	Progress    int    `json:"progress"`
	Message     string `json:"message,omitempty"`
	StreamURL   string `json:"streamUrl,omitempty"`
	ResultURL   string `json:"resultUrl,omitempty"`
}

type AssistantChatWorkloadHistory struct {
	ClusterID       uint                               `json:"clusterId"`
	ClusterName     string                             `json:"clusterName"`
	Namespace       string                             `json:"namespace"`
	WorkloadType    string                             `json:"workloadType"`
	WorkloadName    string                             `json:"workloadName"`
	CurrentRevision int64                              `json:"currentRevision"`
	Total           int                                `json:"total"`
	Limit           int                                `json:"limit"`
	Items           []AssistantChatWorkloadHistoryItem `json:"items"`
}

type AssistantChatWorkloadHistoryItem struct {
	Revision     int64    `json:"revision"`
	CreationTime string   `json:"creationTime"`
	ImageVersion string   `json:"imageVersion"`
	Images       []string `json:"images,omitempty"`
	Status       string   `json:"status"`
	Current      bool     `json:"current"`
}

type AssistantChatWorkloadStatus struct {
	ClusterID         uint                             `json:"clusterId"`
	ClusterName       string                           `json:"clusterName"`
	Namespace         string                           `json:"namespace"`
	WorkloadType      string                           `json:"workloadType"`
	WorkloadName      string                           `json:"workloadName"`
	Status            string                           `json:"status"`
	Healthy           bool                             `json:"healthy"`
	DesiredReplicas   int                              `json:"desiredReplicas"`
	CurrentReplicas   int                              `json:"currentReplicas"`
	ReadyReplicas     int                              `json:"readyReplicas"`
	AvailableReplicas int                              `json:"availableReplicas"`
	Images            []string                         `json:"images,omitempty"`
	Pods              []AssistantChatWorkloadStatusPod `json:"pods,omitempty"`
	WarningEvents     int                              `json:"warningEvents"`
}

type AssistantChatWorkloadStatusPod struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	Phase        string `json:"phase"`
	RestartCount int    `json:"restartCount"`
	NodeName     string `json:"nodeName"`
	PodIP        string `json:"podIP"`
	RunningTime  string `json:"runningTime"`
}

type AssistantChatResult struct {
	SessionID        string                         `json:"sessionId"`
	RunID            uint64                         `json:"runId"`
	Intent           string                         `json:"intent"`
	Question         string                         `json:"question"`
	Answer           string                         `json:"answer"`
	Entities         map[string]interface{}         `json:"entities,omitempty"`
	Memory           *AssistantChatMemoryContext    `json:"memory,omitempty"`
	AssistantPlan    *AssistantInvestigationPlan    `json:"assistantPlan,omitempty"`
	Reflection       *AssistantReflection           `json:"reflection,omitempty"`
	Progress         *AssistantChatProgress         `json:"progress,omitempty"`
	ToolPlan         []AssistantChatToolPlanStep    `json:"toolPlan,omitempty"`
	Timeline         []AssistantChatTimelineEvent   `json:"timeline,omitempty"`
	ThinkingProcess  *AssistantChatThinkingProcess  `json:"thinkingProcess,omitempty"`
	DiagnosisCard    *AssistantChatDiagnosisCard    `json:"diagnosisCard,omitempty"`
	AnalysisTask     *AssistantChatAsyncTask        `json:"analysisTask,omitempty"`
	WorkloadStatus   *AssistantChatWorkloadStatus   `json:"workloadStatus,omitempty"`
	WorkloadHistory  *AssistantChatWorkloadHistory  `json:"workloadHistory,omitempty"`
	PendingAction    map[string]interface{}         `json:"pendingAction,omitempty"`
	ClarifyQuestions []AssistantChatClarifyQuestion `json:"clarifyQuestions,omitempty"`
	UsedTools        []AssistantChatUsedTool        `json:"usedTools"`
	Evidence         []AgentDiagnosisEvidence       `json:"evidence"`
	SuggestedActions []string                       `json:"suggestedActions"`
	MissingInfo      []string                       `json:"missingInfo"`
	Readonly         bool                           `json:"readonly"`
	UsedLLM          bool                           `json:"usedLlm"`
	ProviderID       uint                           `json:"providerId,omitempty"`
	ProviderName     string                         `json:"providerName,omitempty"`
	ProviderType     string                         `json:"providerType,omitempty"`
	ModelCode        string                         `json:"modelCode,omitempty"`
	GeneratedAt      string                         `json:"generatedAt"`
}
