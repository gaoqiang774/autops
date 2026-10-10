package model

import (
	"time"

	"dodevops-api/common/util"
)

const (
	LogStatusEnabled  = 1
	LogStatusDisabled = 2

	LogCollectTypeHost        = "host"
	LogCollectTypeLogtailHost = "logtail_host"
	LogCollectTypeKubernetes  = "kubernetes"

	LogK8SLogSourceStdout        = "stdout"
	LogK8SLogSourceNodeFile      = "node_file"
	LogK8SLogSourceContainerFile = "container_file"
	LogK8SLogSourceBoth          = "both"
)

type LogProject struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"size:128;not null" json:"name"`
	Code        string     `gorm:"size:128;not null;uniqueIndex:uk_log_project_code" json:"code"`
	Description string     `gorm:"type:text" json:"description"`
	OwnerType   string     `gorm:"size:64" json:"ownerType"`
	OwnerID     uint       `gorm:"index" json:"ownerId"`
	OwnerName   string     `gorm:"size:128" json:"ownerName"`
	Env         string     `gorm:"size:64;index" json:"env"`
	Status      int        `gorm:"not null;default:1;index" json:"status"`
	ExtraJSON   string     `gorm:"column:extra_json;type:text" json:"extraJson"`
	CreatedBy   string     `gorm:"size:64" json:"createdBy"`
	UpdatedBy   string     `gorm:"size:64" json:"updatedBy"`
	CreatedAt   util.HTime `json:"createdAt"`
	UpdatedAt   util.HTime `json:"updatedAt"`

	StoreCount        int64 `gorm:"-" json:"storeCount"`
	EnabledStoreCount int64 `gorm:"-" json:"enabledStoreCount"`
}

func (LogProject) TableName() string {
	return "log_project"
}

type LogStore struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	ProjectID     uint       `gorm:"not null;index:idx_log_store_project_id" json:"projectId"`
	Name          string     `gorm:"size:128;not null" json:"name"`
	Code          string     `gorm:"size:128;not null;uniqueIndex:uk_log_store_code" json:"code"`
	Description   string     `gorm:"type:text" json:"description"`
	StoreType     string     `gorm:"size:64" json:"storeType"`
	RetentionDays int        `gorm:"not null;default:7" json:"retentionDays"`
	Status        int        `gorm:"not null;default:1;index:idx_log_store_status" json:"status"`
	Sort          int        `gorm:"not null;default:0" json:"sort"`
	VlogsTenant   string     `gorm:"size:128" json:"vlogsTenant"`
	ExtraJSON     string     `gorm:"column:extra_json;type:text" json:"extraJson"`
	CreatedBy     string     `gorm:"size:64" json:"createdBy"`
	UpdatedBy     string     `gorm:"size:64" json:"updatedBy"`
	CreatedAt     util.HTime `json:"createdAt"`
	UpdatedAt     util.HTime `json:"updatedAt"`
}

type LogProjectStoreCount struct {
	ProjectID         uint  `json:"projectId"`
	StoreCount        int64 `json:"storeCount"`
	EnabledStoreCount int64 `json:"enabledStoreCount"`
}

func (LogStore) TableName() string {
	return "log_store"
}

type LogParseTemplate struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	ProjectID         uint       `gorm:"not null;index:idx_log_parse_template_project_id" json:"projectId"`
	Name              string     `gorm:"size:128;not null" json:"name"`
	Code              string     `gorm:"size:128;not null;uniqueIndex:uk_log_parse_template_code" json:"code"`
	ParseMode         string     `gorm:"size:64;not null" json:"parseMode"`
	InputFormat       string     `gorm:"size:64" json:"inputFormat"`
	MultilineEnabled  bool       `gorm:"not null;default:false" json:"multilineEnabled"`
	MultilinePattern  string     `gorm:"size:512" json:"multilinePattern"`
	TimeKey           string     `gorm:"size:128" json:"timeKey"`
	TimeFormat        string     `gorm:"size:128" json:"timeFormat"`
	LevelKey          string     `gorm:"size:128" json:"levelKey"`
	MessageKey        string     `gorm:"size:128" json:"messageKey"`
	TraceIDKey        string     `gorm:"size:128" json:"traceIdKey"`
	SpanIDKey         string     `gorm:"size:128" json:"spanIdKey"`
	FieldsMappingJSON string     `gorm:"column:fields_mapping_json;type:text" json:"fieldsMappingJson"`
	LabelsMappingJSON string     `gorm:"column:labels_mapping_json;type:text" json:"labelsMappingJson"`
	VRLScript         string     `gorm:"column:vrl_script;type:text" json:"vrlScript"`
	Status            int        `gorm:"not null;default:1;index:idx_log_parse_template_status" json:"status"`
	Version           int        `gorm:"not null;default:1" json:"version"`
	Remark            string     `gorm:"type:text" json:"remark"`
	CreatedBy         string     `gorm:"size:64" json:"createdBy"`
	UpdatedBy         string     `gorm:"size:64" json:"updatedBy"`
	CreatedAt         util.HTime `json:"createdAt"`
	UpdatedAt         util.HTime `json:"updatedAt"`
}

func (LogParseTemplate) TableName() string {
	return "log_parse_template"
}

type LogCollectTask struct {
	ID                       uint       `gorm:"primaryKey" json:"id"`
	ProjectID                uint       `gorm:"not null;index:idx_log_collect_task_project_id" json:"projectId"`
	StoreID                  uint       `gorm:"not null;index:idx_log_collect_task_store_id" json:"storeId"`
	Name                     string     `gorm:"size:128;not null" json:"name"`
	Code                     string     `gorm:"size:128;not null;uniqueIndex:uk_log_collect_task_code" json:"code"`
	NameKey                  *string    `gorm:"column:name_key;size:64;uniqueIndex:uk_log_collect_task_name_key" json:"-"`
	ConfigKey                *string    `gorm:"column:config_key;size:64;uniqueIndex:uk_log_collect_task_config_key" json:"-"`
	CollectType              string     `gorm:"size:64;not null" json:"collectType"`
	ExecuteMode              string     `gorm:"size:64;not null" json:"executeMode"`
	ExecuteTargetsJSON       string     `gorm:"column:execute_targets_json;type:text" json:"executeTargetsJson"`
	PathPattern              string     `gorm:"type:text;not null" json:"pathPattern"`
	PathPatternsJSON         string     `gorm:"column:path_patterns_json;type:text" json:"pathPatternsJson"`
	K8SLogSource             string     `gorm:"column:k8s_log_source;size:64" json:"k8sLogSource"`
	ParseTemplateID          uint       `gorm:"index" json:"parseTemplateId"`
	ParseModeSnapshot        string     `gorm:"size:64" json:"parseModeSnapshot"`
	SampleLog                string     `gorm:"column:sample_log;type:text" json:"sampleLog"`
	RegexPattern             string     `gorm:"column:regex_pattern;type:text" json:"regexPattern"`
	MultilineEnabledSnapshot *bool      `gorm:"column:multiline_enabled_snapshot" json:"multilineEnabledSnapshot"`
	MultilinePatternSnapshot string     `gorm:"size:512" json:"multilinePatternSnapshot"`
	LabelsJSON               string     `gorm:"column:labels_json;type:text" json:"labelsJson"`
	FiltersJSON              string     `gorm:"column:filters_json;type:text" json:"filtersJson"`
	ConfigVersion            int        `gorm:"not null;default:1" json:"configVersion"`
	Status                   int        `gorm:"not null;default:1;index:idx_log_collect_task_status" json:"status"`
	LastError                string     `gorm:"type:text" json:"lastError"`
	Remark                   string     `gorm:"type:text" json:"remark"`
	CreatedBy                string     `gorm:"size:64" json:"createdBy"`
	UpdatedBy                string     `gorm:"size:64" json:"updatedBy"`
	CreatedAt                util.HTime `json:"createdAt"`
	UpdatedAt                util.HTime `json:"updatedAt"`

	AutoApplyTriggered bool                        `gorm:"-" json:"autoApplyTriggered,omitempty"`
	AutoApplyStatus    string                      `gorm:"-" json:"autoApplyStatus,omitempty"`
	AutoApplyMessage   string                      `gorm:"-" json:"autoApplyMessage,omitempty"`
	AutoApplySummary   *LogCollectTaskApplySummary `gorm:"-" json:"autoApplySummary,omitempty"`
	PathPatterns       []string                    `gorm:"-" json:"pathPatterns,omitempty"`
	LogType            string                      `gorm:"-" json:"logType,omitempty"`
}

func (LogCollectTask) TableName() string {
	return "log_collect_task"
}

type LogAlertRule struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	ProjectID           uint       `gorm:"not null;index:idx_log_alert_rule_project_id" json:"projectId"`
	StoreID             uint       `gorm:"not null;index:idx_log_alert_rule_store_id" json:"storeId"`
	Name                string     `gorm:"size:128;not null" json:"name"`
	Code                string     `gorm:"size:128;not null;uniqueIndex:uk_log_alert_rule_code" json:"code"`
	RuleType            string     `gorm:"size:64;not null" json:"ruleType"`
	Severity            string     `gorm:"size:32;not null" json:"severity"`
	ScanIntervalMinutes int        `gorm:"not null;default:5" json:"scanIntervalMinutes"`
	WindowMinutes       int        `gorm:"not null;default:5" json:"windowMinutes"`
	Threshold           int        `gorm:"not null;default:1" json:"threshold"`
	SilenceMinutes      int        `gorm:"not null;default:30" json:"silenceMinutes"`
	KeywordsJSON        string     `gorm:"column:keywords_json;type:text" json:"keywordsJson"`
	FieldConditionsJSON string     `gorm:"column:field_conditions_json;type:text" json:"fieldConditionsJson"`
	QueryJSON           string     `gorm:"column:query_json;type:text" json:"queryJson"`
	NotifyChannelsJSON  string     `gorm:"column:notify_channels_json;type:text" json:"notifyChannelsJson"`
	LabelsJSON          string     `gorm:"column:labels_json;type:text" json:"labelsJson"`
	AnnotationsJSON     string     `gorm:"column:annotations_json;type:text" json:"annotationsJson"`
	Status              int        `gorm:"not null;default:1;index:idx_log_alert_rule_status" json:"status"`
	LastEvalAt          *time.Time `json:"lastEvalAt"`
	Remark              string     `gorm:"type:text" json:"remark"`
	CreatedBy           string     `gorm:"size:64" json:"createdBy"`
	UpdatedBy           string     `gorm:"size:64" json:"updatedBy"`
	CreatedAt           util.HTime `json:"createdAt"`
	UpdatedAt           util.HTime `json:"updatedAt"`
}

func (LogAlertRule) TableName() string {
	return "log_alert_rule"
}

type LogAlertEvent struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	RuleID            uint       `gorm:"not null;index:idx_log_alert_event_rule_id" json:"ruleId"`
	ProjectID         uint       `gorm:"not null;index:idx_log_alert_event_project_id" json:"projectId"`
	StoreID           uint       `gorm:"not null;index:idx_log_alert_event_store_id" json:"storeId"`
	SourceName        string     `gorm:"size:128" json:"sourceName"`
	Severity          string     `gorm:"size:32;not null" json:"severity"`
	Status            string     `gorm:"size:32;not null;index:idx_log_alert_event_status" json:"status"`
	TriggerReason     string     `gorm:"type:text" json:"triggerReason"`
	Summary           string     `gorm:"type:text" json:"summary"`
	MatchCount        int        `gorm:"not null;default:0" json:"matchCount"`
	FirstMatchTime    *time.Time `json:"firstMatchTime"`
	LastMatchTime     *time.Time `json:"lastMatchTime"`
	FiredAt           *time.Time `gorm:"index:idx_log_alert_event_fired_at" json:"firedAt"`
	RecoveredAt       *time.Time `json:"recoveredAt"`
	SampleQueryJSON   string     `gorm:"column:sample_query_json;type:text" json:"sampleQueryJson"`
	SampleEventIDs    string     `gorm:"column:sample_event_ids_json;type:text" json:"sampleEventIdsJson"`
	DiagnosisReportID uint       `json:"diagnosisReportId"`
	NotifyStatus      string     `gorm:"size:32" json:"notifyStatus"`
	NotifyResult      string     `gorm:"type:text" json:"notifyResult"`
	CreatedAt         util.HTime `json:"createdAt"`
	UpdatedAt         util.HTime `json:"updatedAt"`
}

func (LogAlertEvent) TableName() string {
	return "log_alert_event"
}

type LogProjectQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Keyword  string `form:"keyword"`
	Status   int    `form:"status"`
}

type LogStoreQuery struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"pageSize"`
	ProjectID uint   `form:"projectId"`
	Keyword   string `form:"keyword"`
	Status    int    `form:"status"`
}

type LogParseTemplateQuery struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"pageSize"`
	ProjectID uint   `form:"projectId"`
	Keyword   string `form:"keyword"`
	ParseMode string `form:"parseMode"`
	Status    int    `form:"status"`
}

type LogCollectTaskQuery struct {
	Page        int    `form:"page"`
	PageSize    int    `form:"pageSize"`
	ProjectID   uint   `form:"projectId"`
	StoreID     uint   `form:"storeId"`
	Keyword     string `form:"keyword"`
	CollectType string `form:"collectType"`
	Status      int    `form:"status"`
}

type LogAlertRuleQuery struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"pageSize"`
	ProjectID uint   `form:"projectId"`
	StoreID   uint   `form:"storeId"`
	Keyword   string `form:"keyword"`
	Severity  string `form:"severity"`
	Status    int    `form:"status"`
}

type LogAlertEventQuery struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"pageSize"`
	ProjectID uint   `form:"projectId"`
	StoreID   uint   `form:"storeId"`
	Keyword   string `form:"keyword"`
	Severity  string `form:"severity"`
	Status    string `form:"status"`
}

type LogQueryBaseRequest struct {
	StoreID        uint                   `json:"storeId" form:"storeId"`
	StoreCode      string                 `json:"storeCode" form:"storeCode"`
	Direct         bool                   `json:"direct" form:"direct"`
	Query          string                 `json:"query" form:"query"`
	Keyword        string                 `json:"keyword" form:"keyword"`
	Start          string                 `json:"start" form:"start"`
	End            string                 `json:"end" form:"end"`
	Timeout        string                 `json:"timeout" form:"timeout"`
	ExtraFilters   map[string]interface{} `json:"extraFilters"`
	VLogsAccountID string                 `json:"vlogsAccountId" form:"vlogsAccountId"`
	VLogsProjectID string                 `json:"vlogsProjectId" form:"vlogsProjectId"`
}

type LogQueryLogsRequest struct {
	LogQueryBaseRequest
	Limit        int      `json:"limit" form:"limit"`
	Offset       int      `json:"offset" form:"offset"`
	Page         int      `json:"page" form:"page"`
	PageSize     int      `json:"pageSize" form:"pageSize"`
	Fields       []string `json:"fields"`
	IncludeTotal bool     `json:"includeTotal" form:"includeTotal"`
}

type LogQueryDownloadRequest struct {
	LogQueryBaseRequest
	Limit  int      `json:"limit" form:"limit"`
	Offset int      `json:"offset" form:"offset"`
	Fields []string `json:"fields"`
	Format string   `json:"format" form:"format"` // csv/jsonl/txt
}

type LogQueryHitsRequest struct {
	LogQueryBaseRequest
	Step        string   `json:"step" form:"step"`
	Offset      string   `json:"offset" form:"offset"`
	Fields      []string `json:"fields"`
	FieldsLimit int      `json:"fieldsLimit" form:"fieldsLimit"`
}

type LogQueryFieldsRequest struct {
	LogQueryBaseRequest
	Filter string `json:"filter" form:"filter"`
}

type LogQueryFieldValuesRequest struct {
	LogQueryBaseRequest
	Field  string `json:"field" form:"field"`
	Filter string `json:"filter" form:"filter"`
	Limit  int    `json:"limit" form:"limit"`
}

type LogParseTemplateAIGenerateRequest struct {
	ProviderID uint   `json:"providerId"`
	ProjectID  uint   `json:"projectId"`
	StoreID    uint   `json:"storeId"`
	Name       string `json:"name"`
	LogType    string `json:"logType"`
	SampleLog  string `json:"sampleLog"`
	Save       bool   `json:"save"`
}

type LogParseTemplateAIGenerateResponse struct {
	UsedLLM       bool              `json:"usedLlm"`
	Saved         bool              `json:"saved"`
	Template      LogParseTemplate  `json:"template"`
	Fields        []string          `json:"fields,omitempty"`
	FieldMappings map[string]string `json:"fieldMappings,omitempty"`
	Warnings      []string          `json:"warnings,omitempty"`
	RawAnswer     string            `json:"rawAnswer,omitempty"`
}

type LogQueryStatsRangeRequest struct {
	LogQueryBaseRequest
	Step   string `json:"step" form:"step"`
	Offset string `json:"offset" form:"offset"`
}

// LogTraceQueryRequest describes a project-scoped log correlation query.
// It correlates logs by trace ID and does not claim parent/child span ordering.
type LogTraceQueryRequest struct {
	ProjectID uint   `json:"projectId"`
	TraceID   string `json:"traceId"`
	Start     string `json:"start"`
	End       string `json:"end"`
	Limit     int    `json:"limit"`
}

type LogTraceListQueryRequest struct {
	ProjectID uint   `json:"projectId"`
	TraceID   string `json:"traceId"`
	Start     string `json:"start"`
	End       string `json:"end"`
	Status    string `json:"status"`
	Service   string `json:"service"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
}

type LogTraceProjectRef struct {
	ID   uint   `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type LogTraceEvent struct {
	ID              string  `json:"id"`
	Timestamp       string  `json:"timestamp"`
	StartedAt       string  `json:"startedAt,omitempty"`
	EndedAt         string  `json:"endedAt,omitempty"`
	StoreID         uint    `json:"storeId"`
	StoreCode       string  `json:"storeCode"`
	StoreName       string  `json:"storeName"`
	Service         string  `json:"service"`
	App             string  `json:"app,omitempty"`
	Event           string  `json:"event,omitempty"`
	EventType       string  `json:"eventType"`
	IsKeyEvent      bool    `json:"isKeyEvent"`
	DisplayName     string  `json:"displayName"`
	Operation       string  `json:"operation,omitempty"`
	Status          string  `json:"status"`
	Level           string  `json:"level,omitempty"`
	HTTPStatus      int     `json:"httpStatus,omitempty"`
	DurationMs      float64 `json:"durationMs,omitempty"`
	RequestMethod   string  `json:"requestMethod,omitempty"`
	RequestPath     string  `json:"requestPath,omitempty"`
	RequestID       string  `json:"requestId,omitempty"`
	TraceID         string  `json:"traceId"`
	SpanID          string  `json:"spanId,omitempty"`
	ParentSpanID    string  `json:"parentSpanId,omitempty"`
	LogSource       string  `json:"logSource,omitempty"`
	HostName        string  `json:"hostName,omitempty"`
	FilePath        string  `json:"filePath,omitempty"`
	Message         string  `json:"message,omitempty"`
	CollectTaskCode string  `json:"collectTaskCode,omitempty"`
}

type LogTraceServiceSummary struct {
	Service       string   `json:"service"`
	App           string   `json:"app,omitempty"`
	Status        string   `json:"status"`
	FirstSeen     string   `json:"firstSeen"`
	LastSeen      string   `json:"lastSeen"`
	MaxDurationMs float64  `json:"maxDurationMs,omitempty"`
	EventCount    int      `json:"eventCount"`
	ErrorCount    int      `json:"errorCount"`
	StoreCodes    []string `json:"storeCodes"`
	Operations    []string `json:"operations,omitempty"`
}

type LogTraceAnalysisResponse struct {
	Project            LogTraceProjectRef       `json:"project"`
	TraceID            string                   `json:"traceId"`
	Mode               string                   `json:"mode"`
	HasCallOrder       bool                     `json:"hasCallOrder"`
	Status             string                   `json:"status"`
	Start              string                   `json:"start"`
	End                string                   `json:"end"`
	FirstSeen          string                   `json:"firstSeen,omitempty"`
	LastSeen           string                   `json:"lastSeen,omitempty"`
	ObservedDurationMs float64                  `json:"observedDurationMs,omitempty"`
	ServiceCount       int                      `json:"serviceCount"`
	EventCount         int                      `json:"eventCount"`
	ErrorCount         int                      `json:"errorCount"`
	Services           []LogTraceServiceSummary `json:"services"`
	Events             []LogTraceEvent          `json:"events"`
	Partial            bool                     `json:"partial"`
	Truncated          bool                     `json:"truncated"`
	Warnings           []string                 `json:"warnings,omitempty"`
	QueryDurationMs    int64                    `json:"queryDurationMs"`
}

type LogTraceListItem struct {
	TraceID            string   `json:"traceId"`
	PrimaryOperation   string   `json:"primaryOperation"`
	Status             string   `json:"status"`
	FirstSeen          string   `json:"firstSeen"`
	LastSeen           string   `json:"lastSeen"`
	ObservedDurationMs float64  `json:"observedDurationMs"`
	MaxDurationMs      float64  `json:"maxDurationMs,omitempty"`
	ServiceCount       int      `json:"serviceCount"`
	EventCount         int      `json:"eventCount"`
	ErrorCount         int      `json:"errorCount"`
	WarningCount       int      `json:"warningCount"`
	Services           []string `json:"services"`
	StoreCodes         []string `json:"storeCodes"`
}

type LogTraceListResponse struct {
	Project         LogTraceProjectRef `json:"project"`
	Start           string             `json:"start"`
	End             string             `json:"end"`
	Page            int                `json:"page"`
	PageSize        int                `json:"pageSize"`
	Count           int                `json:"count"`
	HasMore         bool               `json:"hasMore"`
	Items           []LogTraceListItem `json:"items"`
	Partial         bool               `json:"partial"`
	Warnings        []string           `json:"warnings,omitempty"`
	QueryDurationMs int64              `json:"queryDurationMs"`
}

type LogQueryStoreRef struct {
	ID   uint   `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type LogQueryResponseMeta struct {
	Store                   LogQueryStoreRef `json:"store"`
	Query                   string           `json:"query"`
	Start                   string           `json:"start"`
	End                     string           `json:"end"`
	DurationMs              int64            `json:"durationMs"`
	VictoriaLogsDurationSec string           `json:"victoriaLogsDurationSec,omitempty"`
}

type LogQueryLogsResponse struct {
	LogQueryResponseMeta
	Limit           int                      `json:"limit"`
	Offset          int                      `json:"offset"`
	Count           int                      `json:"count"`
	Total           int64                    `json:"total"`
	HasMore         bool                     `json:"hasMore"`
	Fields          []string                 `json:"fields"`
	Logs            []map[string]interface{} `json:"logs"`
	DisplayMode     string                   `json:"displayMode"`
	LogType         string                   `json:"logType,omitempty"`
	TableColumns    []LogQueryTableColumn    `json:"tableColumns,omitempty"`
	AnalysisPresets []LogQueryAnalysisPreset `json:"analysisPresets,omitempty"`
}

type LogQueryTableColumn struct {
	Field    string `json:"field"`
	Title    string `json:"title"`
	Type     string `json:"type,omitempty"`
	Width    int    `json:"width,omitempty"`
	Sortable bool   `json:"sortable,omitempty"`
	Ellipsis bool   `json:"ellipsis,omitempty"`
	Primary  bool   `json:"primary,omitempty"`
}

type LogQueryAnalysisPreset struct {
	Key         string                `json:"key"`
	Title       string                `json:"title"`
	Description string                `json:"description,omitempty"`
	LogType     string                `json:"logType,omitempty"`
	Query       string                `json:"query"`
	ResultType  string                `json:"resultType"`
	Columns     []LogQueryTableColumn `json:"columns,omitempty"`
}

type LogQueryQuickFilter struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	LogType     string `json:"logType,omitempty"`
	Query       string `json:"query"`
}

type LogQueryHitSeries struct {
	Fields     map[string]string `json:"fields"`
	Timestamps []string          `json:"timestamps"`
	Values     []int64           `json:"values"`
	Total      int64             `json:"total"`
}

type LogQueryHitsResponse struct {
	LogQueryResponseMeta
	Step string              `json:"step"`
	Hits []LogQueryHitSeries `json:"hits"`
}

type LogQueryFieldItem struct {
	Value   string  `json:"value"`
	Hits    int64   `json:"hits"`
	Total   int64   `json:"total,omitempty"`
	Ratio   float64 `json:"ratio,omitempty"`
	Percent float64 `json:"percent,omitempty"`
	IsOther bool    `json:"isOther,omitempty"`
}

type LogQueryFieldsResponse struct {
	LogQueryResponseMeta
	Values          []LogQueryFieldItem      `json:"values"`
	NormalFields    []LogQueryFieldItem      `json:"normalFields,omitempty"`
	DisplayFields   []LogQueryFieldItem      `json:"displayFields,omitempty"`
	IndexFields     []LogQueryFieldItem      `json:"indexFields,omitempty"`
	OtherFields     []LogQueryFieldItem      `json:"otherFields,omitempty"`
	QuickFilters    []LogQueryQuickFilter    `json:"quickFilters,omitempty"`
	AnalysisPresets []LogQueryAnalysisPreset `json:"analysisPresets,omitempty"`
}

type LogQueryStatsRangeResponse struct {
	LogQueryResponseMeta
	Step string      `json:"step"`
	Raw  interface{} `json:"raw"`
}

// LogSLAStoreQuery describes the bounded log query used by the business SLA dashboard.
type LogSLAStoreQuery struct {
	StoreID              uint                   `json:"storeId"`
	Start                string                 `json:"start"`
	End                  string                 `json:"end"`
	Step                 string                 `json:"step"`
	SlowRequestThreshold float64                `json:"slowRequestThresholdSeconds"`
	Filter               string                 `json:"filter,omitempty"`
	RecentFilter         string                 `json:"recentFilter,omitempty"`
	ExtraFilters         map[string]interface{} `json:"extraFilters,omitempty"`
	TopAPISort           string                 `json:"topApiSort,omitempty"`
}

type LogSLAMetrics struct {
	RequestTotal     int64   `json:"requestTotal"`
	HTTP4xx          int64   `json:"http4xx"`
	HTTP5xx          int64   `json:"http5xx"`
	SlowRequestCount int64   `json:"slowRequestCount"`
	SuccessRate      float64 `json:"successRate"`
	AvailabilityRate float64 `json:"availabilityRate"`
	ErrorRate        float64 `json:"errorRate"`
	HTTP4xxRate      float64 `json:"http4xxRate"`
	HTTP5xxRate      float64 `json:"http5xxRate"`
	SlowRequestRate  float64 `json:"slowRequestRate"`
	AvgLatencyMs     float64 `json:"avgLatencyMs"`
	P95LatencyMs     float64 `json:"p95LatencyMs"`
	ResponseBytes    int64   `json:"responseBytes"`
}

type LogSLATrendPoint struct {
	Timestamp    string `json:"timestamp"`
	RequestTotal int64  `json:"requestTotal"`
	HTTP200      int64  `json:"http200"`
	HTTP4xx      int64  `json:"http4xx"`
	HTTP5xx      int64  `json:"http5xx"`
}

type LogSLATopAPI struct {
	RequestMethod string  `json:"requestMethod"`
	RequestPath   string  `json:"requestPath"`
	RequestTotal  int64   `json:"requestTotal"`
	HTTP4xx       int64   `json:"http4xx"`
	HTTP5xx       int64   `json:"http5xx"`
	ErrorCount    int64   `json:"errorCount"`
	ErrorRate     float64 `json:"errorRate"`
	AvgLatencyMs  float64 `json:"avgLatencyMs"`
	P95LatencyMs  float64 `json:"p95LatencyMs"`
	ResponseBytes int64   `json:"responseBytes"`
}

type LogSLARecentError struct {
	Timestamp     string  `json:"timestamp"`
	RequestMethod string  `json:"requestMethod"`
	RequestPath   string  `json:"requestPath"`
	Domain        string  `json:"domain"`
	Status        int     `json:"status"`
	LatencyMs     float64 `json:"latencyMs"`
	UpstreamAddr  string  `json:"upstreamAddr"`
	ClientIP      string  `json:"clientIp"`
	RequestID     string  `json:"requestId"`
	TraceID       string  `json:"traceId"`
}

type LogSLAStoreSnapshot struct {
	Store         LogQueryStoreRef    `json:"store"`
	Metrics       LogSLAMetrics       `json:"metrics"`
	Trend         []LogSLATrendPoint  `json:"trend"`
	TopAPIs       []LogSLATopAPI      `json:"topApis"`
	RecentErrors  []LogSLARecentError `json:"recentErrors"`
	Partial       bool                `json:"partial"`
	Warnings      []string            `json:"warnings,omitempty"`
	QueryDuration int64               `json:"queryDurationMs"`
}
