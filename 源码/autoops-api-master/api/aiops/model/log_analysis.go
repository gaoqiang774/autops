package model

type LogSourceType string

const (
	LogSourceTypeHost LogSourceType = "host"
	LogSourceTypeK8s  LogSourceType = "k8s"
)

type HostLogFilesQuery struct {
	HostID   uint   `form:"hostId" binding:"required"`
	AuthID   uint   `form:"authId"`
	Dir      string `form:"dir"`
	MaxDepth int    `form:"maxDepth"`
	Limit    int    `form:"limit"`
	Keyword  string `form:"keyword"`
}

type HostLogFileInfo struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

type HostLogFilesResult struct {
	HostID uint              `json:"hostId"`
	Dir    string            `json:"dir"`
	Files  []HostLogFileInfo `json:"files"`
	Total  int               `json:"total"`
}

type HostLogQueryRequest struct {
	SourceType LogSourceType `json:"sourceType" form:"sourceType"`
	SessionID  string        `json:"sessionId" form:"sessionId"`
	HostID     uint          `json:"hostId" form:"hostId" binding:"required"`
	AuthID     uint          `json:"authId" form:"authId"`
	Path       string        `json:"path" form:"path" binding:"required"`
	TailLines  int           `json:"tailLines" form:"tailLines"`
	Limit      int           `json:"limit" form:"limit"`
	Keyword    string        `json:"keyword" form:"keyword"`
	Regex      string        `json:"regex" form:"regex"`
	Levels     []string      `json:"levels" form:"levels"`
	Format     string        `json:"format" form:"format"`
	FormatName string        `json:"formatName" form:"formatName"`
	FormatYAML string        `json:"formatYaml" form:"formatYaml"`
}

type HostLogStreamQuery struct {
	HostLogQueryRequest
	Follow bool `json:"follow" form:"follow"`
}

type K8sLogQueryRequest struct {
	SourceType LogSourceType `json:"sourceType" form:"sourceType"`
	SessionID  string        `json:"sessionId" form:"sessionId"`
	ClusterID  uint          `json:"clusterId" form:"clusterId" binding:"required"`
	Namespace  string        `json:"namespace" form:"namespace" binding:"required"`
	Pod        string        `json:"pod" form:"pod" binding:"required"`
	Container  string        `json:"container" form:"container"`
	TailLines  int           `json:"tailLines" form:"tailLines"`
	Limit      int           `json:"limit" form:"limit"`
	Keyword    string        `json:"keyword" form:"keyword"`
	Regex      string        `json:"regex" form:"regex"`
	Levels     []string      `json:"levels" form:"levels"`
	Format     string        `json:"format" form:"format"`
	FormatName string        `json:"formatName" form:"formatName"`
	FormatYAML string        `json:"formatYaml" form:"formatYaml"`
	Previous   bool          `json:"previous" form:"previous"`
	Timestamps bool          `json:"timestamps" form:"timestamps"`
}

type K8sLogStreamQuery struct {
	K8sLogQueryRequest
	Follow bool `json:"follow" form:"follow"`
}

type LogSource struct {
	Type      LogSourceType `json:"type"`
	HostID    uint          `json:"hostId,omitempty"`
	HostName  string        `json:"hostName,omitempty"`
	Path      string        `json:"path,omitempty"`
	ClusterID uint          `json:"clusterId,omitempty"`
	Namespace string        `json:"namespace,omitempty"`
	Pod       string        `json:"pod,omitempty"`
	Container string        `json:"container,omitempty"`
}

type LogEntry struct {
	LineNo        int                    `json:"lineNo"`
	Timestamp     string                 `json:"timestamp"`
	TimestampUnix int64                  `json:"timestampUnix,omitempty"`
	Level         string                 `json:"level"`
	Message       string                 `json:"message"`
	Raw           string                 `json:"raw"`
	Fields        map[string]interface{} `json:"fields,omitempty"`
	Attributes    map[string]string      `json:"attributes,omitempty"`
	Source        LogSource              `json:"source"`
}

type CountItem struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type PatternClusterItem struct {
	Pattern    string  `json:"pattern"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
	Sample     string  `json:"sample,omitempty"`
}

type LogSummary struct {
	Total        int                  `json:"total"`
	Matched      int                  `json:"matched"`
	LevelCounts  map[string]int       `json:"levelCounts"`
	TopKeywords  []CountItem          `json:"topKeywords"`
	TopPatterns  []PatternClusterItem `json:"topPatterns"`
	ErrorSamples []string             `json:"errorSamples"`
}

type LogQueryResult struct {
	SessionID string            `json:"sessionId,omitempty"`
	Source    LogSource         `json:"source"`
	Entries   []LogEntry        `json:"entries"`
	Summary   LogSummary        `json:"summary"`
	Status    *LogSessionStatus `json:"status,omitempty"`
	Truncated bool              `json:"truncated"`
}

type LogAnalyzeRequest struct {
	SessionID  string               `json:"sessionId"`
	ProviderID uint                 `json:"providerId"`
	Prompt     string               `json:"prompt"`
	Query      *HostLogQueryRequest `json:"query"`
	K8sQuery   *K8sLogQueryRequest  `json:"k8sQuery"`
	Entries    []LogEntry           `json:"entries"`
	RawLines   []string             `json:"rawLines"`
	Summary    *LogSummary          `json:"summary"`
}

type LogAnalyzeResult struct {
	ProviderID   uint   `json:"providerId"`
	ProviderName string `json:"providerName"`
	ModelCode    string `json:"modelCode"`
	Analysis     string `json:"analysis"`
	UsedLogLines int    `json:"usedLogLines"`
	GeneratedAt  string `json:"generatedAt"`
}

type K8sPodAnalyzeRequest struct {
	ProviderID      uint   `json:"providerId"`
	Prompt          string `json:"prompt"`
	ClusterID       uint   `json:"clusterId" binding:"required"`
	Namespace       string `json:"namespace" binding:"required"`
	Pod             string `json:"pod"`
	WorkloadType    string `json:"workloadType"`
	WorkloadName    string `json:"workloadName"`
	Container       string `json:"container"`
	TailLines       int    `json:"tailLines"`
	Limit           int    `json:"limit"`
	EventLimit      int    `json:"eventLimit"`
	ContextLines    int    `json:"contextLines"`
	IncludePrevious *bool  `json:"includePrevious"`
	IncludeEvents   *bool  `json:"includeEvents"`
	IncludeMetrics  *bool  `json:"includeMetrics"`
	IncludeNode     *bool  `json:"includeNode"`
	IncludeQuota    *bool  `json:"includeQuota"`
	IncludeStorage  *bool  `json:"includeStorage"`
	IncludeProbe    *bool  `json:"includeProbe"`
	AllContainers   bool   `json:"allContainers"`
	FormatName      string `json:"formatName"`
	FormatYAML      string `json:"formatYaml"`
}

type K8sPodAnalyzePodInfo struct {
	ClusterID      uint                             `json:"clusterId"`
	Namespace      string                           `json:"namespace"`
	Pod            string                           `json:"pod"`
	Node           string                           `json:"node,omitempty"`
	Phase          string                           `json:"phase"`
	StartTime      string                           `json:"startTime,omitempty"`
	QoSClass       string                           `json:"qosClass,omitempty"`
	Container      string                           `json:"container,omitempty"`
	RestartCount   int32                            `json:"restartCount"`
	Resources      K8sPodAnalyzeResourceInfo        `json:"resources"`
	Containers     []K8sPodAnalyzeContainerInfo     `json:"containers"`
	ContainerPorts []K8sPodAnalyzeContainerPortInfo `json:"containerPorts,omitempty"`
	Conditions     []K8sPodAnalyzeCondition         `json:"conditions"`
}

type K8sPodAnalyzeContainerPortInfo struct {
	Container string `json:"container"`
	Name      string `json:"name,omitempty"`
	Port      int32  `json:"port"`
	Protocol  string `json:"protocol,omitempty"`
}

type K8sPodAnalyzeWorkloadInfo struct {
	Type              string `json:"type"`
	Name              string `json:"name"`
	Namespace         string `json:"namespace"`
	DesiredReplicas   int32  `json:"desiredReplicas"`
	CurrentReplicas   int32  `json:"currentReplicas"`
	ReadyReplicas     int32  `json:"readyReplicas"`
	AvailableReplicas int32  `json:"availableReplicas"`
	SelectedPod       string `json:"selectedPod,omitempty"`
	SelectedReason    string `json:"selectedReason,omitempty"`
	PodCount          int    `json:"podCount"`
	AbnormalPodCount  int    `json:"abnormalPodCount"`
}

type K8sPodAnalyzeContainerInfo struct {
	Name         string                    `json:"name"`
	Ready        bool                      `json:"ready"`
	RestartCount int32                     `json:"restartCount"`
	State        string                    `json:"state"`
	Reason       string                    `json:"reason,omitempty"`
	Message      string                    `json:"message,omitempty"`
	LastState    string                    `json:"lastState,omitempty"`
	LastReason   string                    `json:"lastReason,omitempty"`
	LastMessage  string                    `json:"lastMessage,omitempty"`
	Resources    K8sPodAnalyzeResourceInfo `json:"resources"`
}

type K8sPodAnalyzeResourceInfo struct {
	CPURequest             string  `json:"cpuRequest,omitempty"`
	CPULimit               string  `json:"cpuLimit,omitempty"`
	MemoryRequest          string  `json:"memoryRequest,omitempty"`
	MemoryLimit            string  `json:"memoryLimit,omitempty"`
	CPUUsage               string  `json:"cpuUsage,omitempty"`
	MemoryUsage            string  `json:"memoryUsage,omitempty"`
	CPUUsageMilli          int64   `json:"cpuUsageMilli,omitempty"`
	MemoryUsageBytes       int64   `json:"memoryUsageBytes,omitempty"`
	CPURequestUsageRate    float64 `json:"cpuRequestUsageRate,omitempty"`
	CPULimitUsageRate      float64 `json:"cpuLimitUsageRate,omitempty"`
	MemoryRequestUsageRate float64 `json:"memoryRequestUsageRate,omitempty"`
	MemoryLimitUsageRate   float64 `json:"memoryLimitUsageRate,omitempty"`
}

type K8sPodAnalyzeCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type K8sPodAnalyzeProbeInfo struct {
	Container              string   `json:"container"`
	Type                   string   `json:"type"`
	Handler                string   `json:"handler"`
	Path                   string   `json:"path,omitempty"`
	Port                   string   `json:"port,omitempty"`
	Command                []string `json:"command,omitempty"`
	InitialDelaySeconds    int32    `json:"initialDelaySeconds,omitempty"`
	PeriodSeconds          int32    `json:"periodSeconds,omitempty"`
	TimeoutSeconds         int32    `json:"timeoutSeconds,omitempty"`
	FailureThreshold       int32    `json:"failureThreshold,omitempty"`
	SuccessThreshold       int32    `json:"successThreshold,omitempty"`
	TerminationGracePeriod *int64   `json:"terminationGracePeriodSeconds,omitempty"`
}

type K8sPodAnalyzeNodeInfo struct {
	Name              string                   `json:"name,omitempty"`
	Ready             string                   `json:"ready,omitempty"`
	Conditions        []K8sPodAnalyzeCondition `json:"conditions,omitempty"`
	Taints            []string                 `json:"taints,omitempty"`
	Capacity          map[string]string        `json:"capacity,omitempty"`
	Allocatable       map[string]string        `json:"allocatable,omitempty"`
	SuspectedReasons  []string                 `json:"suspectedReasons,omitempty"`
	CollectionSkipped bool                     `json:"collectionSkipped,omitempty"`
}

type K8sPodAnalyzeQuotaInfo struct {
	Name             string            `json:"name"`
	Hard             map[string]string `json:"hard,omitempty"`
	Used             map[string]string `json:"used,omitempty"`
	SuspectedReasons []string          `json:"suspectedReasons,omitempty"`
}

type K8sPodAnalyzePVCInfo struct {
	Name             string            `json:"name"`
	Phase            string            `json:"phase,omitempty"`
	StorageClass     string            `json:"storageClass,omitempty"`
	VolumeName       string            `json:"volumeName,omitempty"`
	AccessModes      []string          `json:"accessModes,omitempty"`
	Requests         map[string]string `json:"requests,omitempty"`
	PVPhase          string            `json:"pvPhase,omitempty"`
	PVNodeAffinity   []string          `json:"pvNodeAffinity,omitempty"`
	SuspectedReasons []string          `json:"suspectedReasons,omitempty"`
	Events           []string          `json:"events,omitempty"`
}

type K8sPodAnalyzeMetricTrendStat struct {
	Unit        string  `json:"unit,omitempty"`
	Current     float64 `json:"current"`
	Peak        float64 `json:"peak"`
	Average     float64 `json:"average"`
	Min         float64 `json:"min"`
	Trend       string  `json:"trend,omitempty"`
	Rising      bool    `json:"rising"`
	NearLimit   bool    `json:"nearLimit"`
	SampleCount int     `json:"sampleCount"`
}

type K8sPodAnalyzeMetricsTrendInfo struct {
	Available            bool                          `json:"available"`
	Source               string                        `json:"source,omitempty"`
	DatasourceID         uint                          `json:"datasourceId,omitempty"`
	DatasourceType       string                        `json:"datasourceType,omitempty"`
	DatasourceName       string                        `json:"datasourceName,omitempty"`
	Range                string                        `json:"range,omitempty"`
	Step                 string                        `json:"step,omitempty"`
	Scope                string                        `json:"scope,omitempty"`
	PodCount             int                           `json:"podCount"`
	Pods                 []string                      `json:"pods,omitempty"`
	CPUUsageRate         *K8sPodAnalyzeMetricTrendStat `json:"cpuUsageRate,omitempty"`
	MemoryWSSUsageRate   *K8sPodAnalyzeMetricTrendStat `json:"memoryWssUsageRate,omitempty"`
	NetworkReceiveMiBps  *K8sPodAnalyzeMetricTrendStat `json:"networkReceiveMiBps,omitempty"`
	NetworkTransmitMiBps *K8sPodAnalyzeMetricTrendStat `json:"networkTransmitMiBps,omitempty"`
	Warnings             []string                      `json:"warnings,omitempty"`
}

type K8sPodAnalyzeEvidenceSummary struct {
	Workload         *K8sPodAnalyzeWorkloadInfo      `json:"workload,omitempty"`
	Pod              K8sPodAnalyzePodInfo            `json:"pod"`
	LogLines         int                             `json:"logLines"`
	MatchedLogLines  int                             `json:"matchedLogLines"`
	RecentLogLines   int                             `json:"recentLogLines"`
	PreviousLogLines int                             `json:"previousLogLines"`
	EventCount       int                             `json:"eventCount"`
	WarningEvents    int                             `json:"warningEvents"`
	NormalEvents     int                             `json:"normalEvents"`
	MatchedKeywords  []string                        `json:"matchedKeywords"`
	SuspectedReasons []string                        `json:"suspectedReasons"`
	LogFindings      []K8sPodAnalyzeLogFinding       `json:"logFindings,omitempty"`
	ResourceHotspots []string                        `json:"resourceHotspots,omitempty"`
	Node             *K8sPodAnalyzeNodeInfo          `json:"node,omitempty"`
	Quotas           []K8sPodAnalyzeQuotaInfo        `json:"quotas,omitempty"`
	PVCs             []K8sPodAnalyzePVCInfo          `json:"pvcs,omitempty"`
	Probes           []K8sPodAnalyzeProbeInfo        `json:"probes,omitempty"`
	Services         []K8sPodAnalyzeServiceInfo      `json:"services,omitempty"`
	IngressRoutes    []K8sPodAnalyzeIngressRouteInfo `json:"ingressRoutes,omitempty"`
	Manifests        []K8sPodAnalyzeManifestInfo     `json:"manifests,omitempty"`
	MetricsTrend     *K8sPodAnalyzeMetricsTrendInfo  `json:"metricsTrend,omitempty"`
	CollectionErrors []string                        `json:"collectionErrors,omitempty"`
}

type K8sPodAnalyzeServicePortInfo struct {
	Name       string `json:"name,omitempty"`
	Port       int32  `json:"port"`
	TargetPort string `json:"targetPort,omitempty"`
	NodePort   int32  `json:"nodePort,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
}

type K8sPodAnalyzeServiceInfo struct {
	Name     string                         `json:"name"`
	Type     string                         `json:"type,omitempty"`
	Selector map[string]string              `json:"selector,omitempty"`
	Ports    []K8sPodAnalyzeServicePortInfo `json:"ports,omitempty"`
}

type K8sPodAnalyzeIngressRouteInfo struct {
	Ingress     string `json:"ingress"`
	Host        string `json:"host,omitempty"`
	Path        string `json:"path,omitempty"`
	ServiceName string `json:"serviceName"`
	ServicePort string `json:"servicePort"`
}

type K8sPodAnalyzeManifestInfo struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	YAML      string `json:"yaml"`
}

type K8sPodAnalyzeLogFinding struct {
	LineNo    int      `json:"lineNo"`
	Timestamp string   `json:"timestamp,omitempty"`
	Level     string   `json:"level,omitempty"`
	Kind      string   `json:"kind"`
	Message   string   `json:"message"`
	Context   []string `json:"context,omitempty"`
}

type K8sPodAnalyzeResult struct {
	ProviderID     uint                         `json:"providerId"`
	ProviderName   string                       `json:"providerName"`
	ModelCode      string                       `json:"modelCode"`
	Analysis       string                       `json:"analysis"`
	Status         string                       `json:"status"`
	Severity       string                       `json:"severity"`
	Summary        string                       `json:"summary,omitempty"`
	Remediation    []string                     `json:"remediation,omitempty"`
	Conclusion     string                       `json:"conclusion,omitempty"`
	Evidence       K8sPodAnalyzeEvidenceSummary `json:"evidence"`
	UsedLogLines   int                          `json:"usedLogLines"`
	UsedEventCount int                          `json:"usedEventCount"`
	GeneratedAt    string                       `json:"generatedAt"`
}

type K8sPodAnalyzeTaskCreateResult struct {
	TaskID    string `json:"taskId"`
	Status    string `json:"status"`
	StreamURL string `json:"streamUrl"`
	ResultURL string `json:"resultUrl"`
}

type K8sPodAnalyzeTaskStatus struct {
	TaskID      string                       `json:"taskId"`
	Status      string                       `json:"status"`
	PodStatus   string                       `json:"podStatus"`
	CurrentStep string                       `json:"currentStep"`
	Progress    int                          `json:"progress"`
	Message     string                       `json:"message,omitempty"`
	Result      *K8sPodAnalyzeResult         `json:"result,omitempty"`
	Error       string                       `json:"error,omitempty"`
	Events      []K8sPodAnalyzeProgressEvent `json:"events,omitempty"`
	CreatedAt   string                       `json:"createdAt"`
	UpdatedAt   string                       `json:"updatedAt"`
}

type K8sPodAnalyzeProgressEvent struct {
	TaskID     string               `json:"taskId,omitempty"`
	Type       string               `json:"type"`
	Step       string               `json:"step,omitempty"`
	Status     string               `json:"status,omitempty"`
	StepStatus string               `json:"stepStatus,omitempty"`
	Progress   int                  `json:"progress"`
	Message    string               `json:"message,omitempty"`
	PodStatus  string               `json:"podStatus,omitempty"`
	Result     *K8sPodAnalyzeResult `json:"result,omitempty"`
	Error      string               `json:"error,omitempty"`
	DurationMs int64                `json:"durationMs,omitempty"`
	Timestamp  string               `json:"timestamp"`
}

type K8sIngressAnalyzeRequest struct {
	ProviderID       uint   `json:"providerId" form:"providerId"`
	Prompt           string `json:"prompt" form:"prompt"`
	ClusterID        uint   `json:"clusterId" form:"clusterId" binding:"required"`
	Namespace        string `json:"namespace" form:"namespace" binding:"required"`
	Ingress          string `json:"ingress" form:"ingress"`
	Host             string `json:"host" form:"host"`
	Path             string `json:"path" form:"path"`
	StatusCode       int    `json:"statusCode" form:"statusCode"`
	ObservedError    string `json:"observedError" form:"observedError"`
	TailLines        int    `json:"tailLines" form:"tailLines"`
	Limit            int    `json:"limit" form:"limit"`
	EventLimit       int    `json:"eventLimit" form:"eventLimit"`
	IncludeLogs      *bool  `json:"includeLogs" form:"includeLogs"`
	IncludeEvents    *bool  `json:"includeEvents" form:"includeEvents"`
	IncludeBackend   *bool  `json:"includeBackend" form:"includeBackend"`
	IncludeHTTPProbe *bool  `json:"includeHttpProbe" form:"includeHttpProbe"`
	ProbeScheme      string `json:"probeScheme" form:"probeScheme"`
	ProbeTimeout     int    `json:"probeTimeout" form:"probeTimeout"`
	FormatName       string `json:"formatName" form:"formatName"`
	FormatYAML       string `json:"formatYaml" form:"formatYaml"`
}

type K8sIngressAnalyzeRuleInfo struct {
	Host        string `json:"host,omitempty"`
	Path        string `json:"path,omitempty"`
	PathType    string `json:"pathType,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
	ServicePort string `json:"servicePort,omitempty"`
	Matched     bool   `json:"matched"`
}

type K8sIngressAnalyzeBackendInfo struct {
	ServiceName      string                 `json:"serviceName"`
	ServiceType      string                 `json:"serviceType,omitempty"`
	ServicePort      string                 `json:"servicePort,omitempty"`
	TargetPort       string                 `json:"targetPort,omitempty"`
	Selector         map[string]string      `json:"selector,omitempty"`
	EndpointReady    int                    `json:"endpointReady"`
	EndpointNotReady int                    `json:"endpointNotReady"`
	Pods             []K8sPodAnalyzePodInfo `json:"pods,omitempty"`
	Events           []string               `json:"events,omitempty"`
	SuspectedReasons []string               `json:"suspectedReasons,omitempty"`
	CollectionErrors []string               `json:"collectionErrors,omitempty"`
}

type K8sIngressAnalyzeHTTPProbeInfo struct {
	URL          string            `json:"url,omitempty"`
	Method       string            `json:"method,omitempty"`
	StatusCode   int               `json:"statusCode,omitempty"`
	Status       string            `json:"status,omitempty"`
	ResponseTime int64             `json:"responseTimeMs,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Error        string            `json:"error,omitempty"`
	MatchedInput bool              `json:"matchedInput"`
}

type K8sIngressAnalyzeEvidenceSummary struct {
	ClusterID        uint                            `json:"clusterId"`
	Namespace        string                          `json:"namespace"`
	Ingress          string                          `json:"ingress,omitempty"`
	Host             string                          `json:"host,omitempty"`
	Path             string                          `json:"path,omitempty"`
	StatusCode       int                             `json:"statusCode,omitempty"`
	ObservedError    string                          `json:"observedError,omitempty"`
	IngressClass     string                          `json:"ingressClass,omitempty"`
	LoadBalancers    []string                        `json:"loadBalancers,omitempty"`
	HTTPProbe        *K8sIngressAnalyzeHTTPProbeInfo `json:"httpProbe,omitempty"`
	Rules            []K8sIngressAnalyzeRuleInfo     `json:"rules,omitempty"`
	MatchedBackends  []K8sIngressAnalyzeBackendInfo  `json:"matchedBackends,omitempty"`
	EventCount       int                             `json:"eventCount"`
	WarningEvents    int                             `json:"warningEvents"`
	LogLines         int                             `json:"logLines"`
	MatchedLogLines  int                             `json:"matchedLogLines"`
	SuspectedReasons []string                        `json:"suspectedReasons,omitempty"`
	CollectionErrors []string                        `json:"collectionErrors,omitempty"`
}

type K8sIngressAnalyzeResult struct {
	ProviderID     uint                             `json:"providerId"`
	ProviderName   string                           `json:"providerName"`
	ModelCode      string                           `json:"modelCode"`
	Analysis       string                           `json:"analysis"`
	Status         string                           `json:"status"`
	Severity       string                           `json:"severity"`
	Summary        string                           `json:"summary,omitempty"`
	Remediation    []string                         `json:"remediation,omitempty"`
	Conclusion     string                           `json:"conclusion,omitempty"`
	Evidence       K8sIngressAnalyzeEvidenceSummary `json:"evidence"`
	UsedLogLines   int                              `json:"usedLogLines"`
	UsedEventCount int                              `json:"usedEventCount"`
	GeneratedAt    string                           `json:"generatedAt"`
}

type K8sIngressAnalyzeProgressEvent struct {
	Type      string                   `json:"type"`
	Step      string                   `json:"step,omitempty"`
	Status    string                   `json:"status,omitempty"`
	Progress  int                      `json:"progress"`
	Message   string                   `json:"message,omitempty"`
	Result    *K8sIngressAnalyzeResult `json:"result,omitempty"`
	Error     string                   `json:"error,omitempty"`
	Timestamp string                   `json:"timestamp"`
}

type LogStreamEvent struct {
	Type    string      `json:"type"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type LogSessionsQuery struct {
	ActiveOnly bool `form:"activeOnly"`
}

type LogSessionStatusQuery struct {
	SessionID string `form:"sessionId" binding:"required"`
}

type LogStreamStatus struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	Active   bool   `json:"active"`
	LogCount int    `json:"logCount"`
	LastSeen string `json:"lastSeen"`
}

type LogSessionStatus struct {
	SessionID       string               `json:"sessionId"`
	Source          LogSource            `json:"source"`
	Format          string               `json:"format"`
	HasCustomFormat bool                 `json:"hasCustomFormat"`
	StartedAt       string               `json:"startedAt"`
	LastSeen        string               `json:"lastSeen"`
	TotalLogs       int                  `json:"totalLogs"`
	TotalBytes      int64                `json:"totalBytes"`
	BufferSize      int                  `json:"bufferSize"`
	BufferUsed      int                  `json:"bufferUsed"`
	LevelCounts     map[string]int       `json:"levelCounts"`
	TopKeywords     []CountItem          `json:"topKeywords"`
	TopPatterns     []PatternClusterItem `json:"topPatterns"`
	Streams         []LogStreamStatus    `json:"streams"`
}

type LogSessionListItem struct {
	SessionID   string    `json:"sessionId"`
	Source      LogSource `json:"source"`
	Format      string    `json:"format"`
	StartedAt   string    `json:"startedAt"`
	LastSeen    string    `json:"lastSeen"`
	TotalLogs   int       `json:"totalLogs"`
	BufferUsed  int       `json:"bufferUsed"`
	Active      bool      `json:"active"`
	StreamCount int       `json:"streamCount"`
}
