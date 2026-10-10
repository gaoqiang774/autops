package model

const (
	AssistantIntentResourcePlacement  = "resource_placement"
	AssistantIntentResourceReclaim    = "resource_reclaim"
	AssistantIntentCapacityInspection = "capacity_inspection"
	AssistantIntentAssetQuery         = "asset_query"
)

type AssistantQueryRequest struct {
	Question   string                `json:"question" binding:"required"`
	ProviderID uint                  `json:"providerId"`
	Scope      AssistantQueryScope   `json:"scope"`
	Options    AssistantQueryOptions `json:"options"`
}

type AssistantQueryScope struct {
	GroupID       uint   `json:"groupId"`
	HostIDs       []uint `json:"hostIds"`
	Keyword       string `json:"keyword"`
	Env           string `json:"env"`
	TimeRangeDays int    `json:"timeRangeDays"`
	Limit         int    `json:"limit"`
}

type AssistantQueryOptions struct {
	RequiredCPUCores float64 `json:"requiredCpuCores"`
	RequiredMemoryGB float64 `json:"requiredMemoryGb"`
	RequiredDiskGB   float64 `json:"requiredDiskGb"`
	CPUThreshold     float64 `json:"cpuThreshold"`
	MemoryThreshold  float64 `json:"memoryThreshold"`
	DiskThreshold    float64 `json:"diskThreshold"`
	IncludeOffline   bool    `json:"includeOffline"`
}

type AssistantQueryResult struct {
	Intent       string                   `json:"intent"`
	Question     string                   `json:"question"`
	Answer       string                   `json:"answer"`
	Ranking      *AssistantQueryRanking   `json:"ranking,omitempty"`
	Summary      AssistantQuerySummary    `json:"summary"`
	Candidates   []AssistantHostCandidate `json:"candidates"`
	Evidence     []AssistantEvidence      `json:"evidence"`
	MissingInfo  []string                 `json:"missingInfo"`
	UsedLLM      bool                     `json:"usedLlm"`
	ProviderID   uint                     `json:"providerId,omitempty"`
	ProviderName string                   `json:"providerName,omitempty"`
	ProviderType string                   `json:"providerType,omitempty"`
	ModelCode    string                   `json:"modelCode,omitempty"`
	GeneratedAt  string                   `json:"generatedAt"`
}

type AssistantQueryRanking struct {
	Metric    string `json:"metric"`
	Period    string `json:"period"`
	Direction string `json:"direction"`
	Limit     int    `json:"limit"`
}

type AssistantQuerySummary struct {
	TotalHosts                  int     `json:"totalHosts"`
	SelectedHosts               int     `json:"selectedHosts"`
	AnalyzedHosts               int     `json:"analyzedHosts"`
	MetricHosts                 int     `json:"metricHosts"`
	NoMetricHosts               int     `json:"noMetricHosts"`
	AgentInstalledHosts         int     `json:"agentInstalledHosts"`
	AgentOnlineHosts            int     `json:"agentOnlineHosts"`
	AgentOfflineHosts           int     `json:"agentOfflineHosts"`
	SelectedAgentInstalledHosts int     `json:"selectedAgentInstalledHosts"`
	SelectedAgentOnlineHosts    int     `json:"selectedAgentOnlineHosts"`
	SelectedAgentOfflineHosts   int     `json:"selectedAgentOfflineHosts"`
	MatchedHosts                int     `json:"matchedHosts"`
	TimeRangeDays               int     `json:"timeRangeDays"`
	RequiredCPUCores            float64 `json:"requiredCpuCores,omitempty"`
	RequiredMemoryGB            float64 `json:"requiredMemoryGb,omitempty"`
	RequiredDiskGB              float64 `json:"requiredDiskGb,omitempty"`
	CPUThreshold                float64 `json:"cpuThreshold,omitempty"`
	MemoryThreshold             float64 `json:"memoryThreshold,omitempty"`
	DiskThreshold               float64 `json:"diskThreshold,omitempty"`
}

type AssistantHostCandidate struct {
	HostID               uint                    `json:"hostId"`
	HostName             string                  `json:"hostName"`
	Name                 string                  `json:"name,omitempty"`
	GroupID              uint                    `json:"groupId"`
	GroupName            string                  `json:"groupName,omitempty"`
	IP                   string                  `json:"ip"`
	PrivateIP            string                  `json:"privateIp,omitempty"`
	PublicIP             string                  `json:"publicIp,omitempty"`
	SSHIP                string                  `json:"sshIp,omitempty"`
	OS                   string                  `json:"os,omitempty"`
	Status               int                     `json:"status"`
	Vendor               int                     `json:"vendor"`
	Region               string                  `json:"region,omitempty"`
	Tags                 string                  `json:"tags,omitempty"`
	CPUCores             float64                 `json:"cpuCores,omitempty"`
	MemoryGB             float64                 `json:"memoryGb,omitempty"`
	DiskTotalGB          float64                 `json:"diskTotalGb,omitempty"`
	CurrentCPU           float64                 `json:"currentCpu"`
	CurrentMemory        float64                 `json:"currentMemory"`
	CurrentDisk          float64                 `json:"currentDisk"`
	AvgCPU               float64                 `json:"avgCpu"`
	AvgMemory            float64                 `json:"avgMemory"`
	AvgDisk              float64                 `json:"avgDisk"`
	HasMetrics           bool                    `json:"hasMetrics"`
	AgentInstalled       bool                    `json:"agentInstalled"`
	AgentOnline          bool                    `json:"agentOnline"`
	AgentStatus          int                     `json:"agentStatus,omitempty"`
	AgentLastHeartbeat   string                  `json:"agentLastHeartbeat,omitempty"`
	ServiceCount         int                     `json:"serviceCount"`
	BusinessServiceCount int                     `json:"businessServiceCount"`
	Services             []string                `json:"services"`
	ServiceDetails       []AssistantServiceUsage `json:"serviceDetails"`
	Score                float64                 `json:"score"`
	Recommendation       string                  `json:"recommendation"`
	RequirementMatched   bool                    `json:"requirementMatched"`
	RequirementGaps      []string                `json:"requirementGaps,omitempty"`
	Reasons              []string                `json:"reasons"`
	Risks                []string                `json:"risks"`
	NextChecks           []string                `json:"nextChecks"`
}

type AssistantServiceUsage struct {
	Name     string  `json:"name"`
	CPU      float64 `json:"cpu"`
	Memory   float64 `json:"memory"`
	Platform bool    `json:"platform"`
	Reason   string  `json:"reason,omitempty"`
}

type AssistantEvidence struct {
	Type      string      `json:"type"`
	Title     string      `json:"title"`
	Reference string      `json:"reference,omitempty"`
	Content   string      `json:"content"`
	Raw       interface{} `json:"raw,omitempty"`
}
