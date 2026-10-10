package model

import (
	"bytes"
	"encoding/json"
	"time"

	"dodevops-api/common/util"
)

const (
	ObjectTypeAsset      = "asset"
	ObjectTypeDatabase   = "database"
	ObjectTypeSchema     = "schema"
	ObjectTypeTable      = "table"
	ObjectTypeView       = "view"
	ObjectTypeColumn     = "column"
	ObjectTypeIndex      = "index"
	ObjectTypeCollection = "collection"
	ObjectTypeRedisDB    = "redis_db"
	ObjectTypeRedisKey   = "redis_key"
	ObjectTypeESIndex    = "es_index"
)

const (
	ActionMetadata = "metadata"
	ActionQuery    = "query"
	ActionSQLExec  = "sql_execute"
	ActionAI       = "ai_sql_assistant"
	ActionAIChat   = "ai_db_chat"
	ActionAITool   = "ai_db_tool"
	ActionSlowLog  = "slow_log_analysis"
	ActionExport   = "export"
	ActionCreate   = "create"
	ActionUpdate   = "update"
	ActionDelete   = "delete"
	ActionPing     = "ping"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 200
)

type AssetQuery struct {
	Page           int    `form:"page"`
	PageSize       int    `form:"pageSize"`
	Keyword        string `form:"keyword"`
	Type           int    `form:"type"`
	Environment    string `form:"environment"`
	Status         int    `form:"status"`
	AuthStatus     int    `form:"authStatus"`
	BusinessSystem string `form:"businessSystem"`
	Owner          string `form:"owner"`
}

type AssetVO struct {
	ID                      uint   `json:"id"`
	Name                    string `json:"name"`
	InstanceName            string `json:"instanceName"`
	Type                    int    `json:"type"`
	TypeName                string `json:"typeName"`
	Version                 string `json:"version"`
	Environment             string `json:"environment"`
	Status                  int    `json:"status"`
	AuthStatus              int    `json:"authStatus"`
	AuthStatusName          string `json:"authStatusName"`
	HostIP                  string `json:"hostIp"`
	Endpoint                string `json:"endpoint"`
	Port                    int    `json:"port"`
	BusinessSystem          string `json:"businessSystem"`
	Owner                   string `json:"owner"`
	DBAOwner                string `json:"dbaOwner"`
	OpsOwner                string `json:"opsOwner"`
	SlowLogPath             string `json:"slowLogPath"`
	PrometheusAccountID     uint   `json:"prometheusAccountId"`
	PrometheusAccountAlias  string `json:"prometheusAccountAlias"`
	PrometheusEndpoint      string `json:"prometheusEndpoint"`
	PrometheusLabelSelector string `json:"prometheusLabelSelector"`
}

type AssetIDRequest struct {
	AssetID uint `json:"assetId" form:"assetId"`
}

type PingResult struct {
	AssetID uint   `json:"assetId"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type MySQLMonitorQuery struct {
	Range string `form:"range"`
	Step  string `form:"step"`
	Start int64  `form:"start"`
	End   int64  `form:"end"`
}

type PostgreSQLMonitorQuery = MySQLMonitorQuery
type RedisMonitorQuery = MySQLMonitorQuery

type SlowLogQuery struct {
	AuthID    uint   `form:"authId" json:"authId"`
	Database  string `form:"database" json:"database"`
	User      string `form:"user" json:"user"`
	Keyword   string `form:"keyword" json:"keyword"`
	BeginTime string `form:"beginTime" json:"beginTime"`
	EndTime   string `form:"endTime" json:"endTime"`
	SortBy    string `form:"sortBy" json:"sortBy"`
	SortOrder string `form:"sortOrder" json:"sortOrder"`
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"pageSize" json:"pageSize"`
	TailLines int    `form:"tailLines" json:"tailLines"`
}

type SlowLogTaskQuery struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"pageSize"`
	Status    string `form:"status"`
	Keyword   string `form:"keyword"`
	BeginTime string `form:"beginTime"`
	EndTime   string `form:"endTime"`
}

type SlowLogTaskCreateResult struct {
	TaskID    string `json:"taskId"`
	Status    string `json:"status"`
	ResultURL string `json:"resultUrl"`
}

type SlowLogTaskStatus struct {
	TaskID        string                `json:"taskId"`
	AssetID       uint                  `json:"assetId"`
	Status        string                `json:"status"`
	Progress      int                   `json:"progress"`
	Message       string                `json:"message"`
	Error         string                `json:"error,omitempty"`
	Source        SlowLogSource         `json:"source"`
	Summary       SlowLogSummary        `json:"summary"`
	Result        *SlowLogAnalyzeResult `json:"result,omitempty"`
	CreatedBy     uint                  `json:"createdBy"`
	CreatedByName string                `json:"createdByName"`
	CreatedAt     string                `json:"createdAt"`
	StartedAt     string                `json:"startedAt,omitempty"`
	CompletedAt   string                `json:"completedAt,omitempty"`
}

const (
	SlowLogTaskStatusPending = "pending"
	SlowLogTaskStatusRunning = "running"
	SlowLogTaskStatusSuccess = "success"
	SlowLogTaskStatusFailed  = "failed"
)

type SlowLogAnalyzeTask struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	TaskID         string     `gorm:"size:64;uniqueIndex;not null" json:"taskId"`
	AssetID        uint       `gorm:"index;not null" json:"assetId"`
	Engine         int        `gorm:"type:integer;index" json:"engine"`
	Status         string     `gorm:"size:32;index" json:"status"`
	Progress       int        `gorm:"type:integer" json:"progress"`
	Message        string     `gorm:"size:255" json:"message"`
	Error          string     `gorm:"type:text" json:"error"`
	AuthID         uint       `gorm:"index" json:"authId"`
	RequestJSON    string     `gorm:"type:text" json:"requestJson"`
	ResultJSON     string     `gorm:"type:longtext" json:"resultJson"`
	SlowLogPath    string     `gorm:"size:512" json:"slowLogPath"`
	HostID         uint       `gorm:"index" json:"hostId"`
	HostIP         string     `gorm:"size:64" json:"hostIp"`
	HostName       string     `gorm:"size:128" json:"hostName"`
	TotalBytes     int64      `json:"totalBytes"`
	ReadBytes      int64      `json:"readBytes"`
	TotalEntries   int        `json:"totalEntries"`
	MatchedEntries int        `json:"matchedEntries"`
	UniqueSQL      int        `json:"uniqueSql"`
	CreatedBy      uint       `gorm:"index" json:"createdBy"`
	CreatedByName  string     `gorm:"size:128" json:"createdByName"`
	CreatedAt      util.HTime `gorm:"index" json:"createdAt"`
	UpdatedAt      util.HTime `json:"updatedAt"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
}

func (SlowLogAnalyzeTask) TableName() string {
	return "db_slow_log_tasks"
}

type SlowLogAnalyzeResult struct {
	Asset       AssetVO        `json:"asset"`
	Source      SlowLogSource  `json:"source"`
	Summary     SlowLogSummary `json:"summary"`
	Items       []SlowLogItem  `json:"items"`
	Page        int            `json:"page"`
	PageSize    int            `json:"pageSize"`
	Total       int64          `json:"total"`
	Warnings    []string       `json:"warnings"`
	GeneratedAt string         `json:"generatedAt"`
}

type SlowLogSource struct {
	Configured bool   `json:"configured"`
	Type       string `json:"type"`
	Path       string `json:"path"`
	HostID     uint   `json:"hostId"`
	HostIP     string `json:"hostIp"`
	HostName   string `json:"hostName"`
	TailLines  int    `json:"tailLines"`
	ReadLines  int    `json:"readLines"`
}

type SlowLogSummary struct {
	TotalEntries         int     `json:"totalEntries"`
	MatchedEntries       int     `json:"matchedEntries"`
	UniqueSQL            int     `json:"uniqueSql"`
	TotalQueryTime       float64 `json:"totalQueryTime"`
	MaxQueryTime         float64 `json:"maxQueryTime"`
	TotalLockTime        float64 `json:"totalLockTime"`
	TotalRowsExamined    int64   `json:"totalRowsExamined"`
	TotalRowsSent        int64   `json:"totalRowsSent"`
	LargeScanCount       int     `json:"largeScanCount"`
	SlowCommitCount      int     `json:"slowCommitCount"`
	ConnectionProbeCount int     `json:"connectionProbeCount"`
}

type SlowLogItem struct {
	Digest            string              `json:"digest"`
	Fingerprint       string              `json:"fingerprint"`
	SampleSQL         string              `json:"sampleSql"`
	Database          string              `json:"database"`
	User              string              `json:"user"`
	Host              string              `json:"host"`
	FirstSeen         string              `json:"firstSeen"`
	LastSeen          string              `json:"lastSeen"`
	Count             int                 `json:"count"`
	QueryTimeAvg      float64             `json:"queryTimeAvg"`
	QueryTimeP95      float64             `json:"queryTimeP95"`
	QueryTimeMax      float64             `json:"queryTimeMax"`
	QueryTimeTotal    float64             `json:"queryTimeTotal"`
	LockTimeTotal     float64             `json:"lockTimeTotal"`
	RowsExaminedTotal int64               `json:"rowsExaminedTotal"`
	RowsExaminedMax   int64               `json:"rowsExaminedMax"`
	RowsSentTotal     int64               `json:"rowsSentTotal"`
	RowsSentMax       int64               `json:"rowsSentMax"`
	AISuggestion      SlowLogAISuggestion `json:"aiSuggestion"`
	LastError         string              `json:"lastError,omitempty"`
	Examples          []string            `json:"examples,omitempty"`
}

type SlowLogAISuggestion struct {
	SlowReason         string `json:"slowReason"`
	SchemaOptimization string `json:"schemaOptimization"`
	IndexOptimization  string `json:"indexOptimization"`
	SQLOptimization    string `json:"sqlOptimization"`
	ExpectedEffect     string `json:"expectedEffect"`
}

func (s *SlowLogAISuggestion) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*s = SlowLogAISuggestion{}
		return nil
	}
	var legacy string
	if err := json.Unmarshal(data, &legacy); err == nil {
		*s = SlowLogAISuggestion{SlowReason: legacy}
		return nil
	}
	type alias SlowLogAISuggestion
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*s = SlowLogAISuggestion(value)
	return nil
}

type MySQLMonitorDashboard struct {
	Asset     AssetVO             `json:"asset"`
	Target    MySQLMonitorTarget  `json:"target"`
	Overview  []MonitorMetricCard `json:"overview"`
	Trends    []MonitorTrendPanel `json:"trends"`
	Breakdown []MonitorTablePanel `json:"breakdown"`
	Range     MonitorQueryRange   `json:"range"`
	Warnings  []string            `json:"warnings"`
}

type PostgreSQLMonitorDashboard = MySQLMonitorDashboard
type RedisMonitorDashboard = MySQLMonitorDashboard

type MonitorQueryRange struct {
	Start int64  `json:"start"`
	End   int64  `json:"end"`
	Step  string `json:"step"`
}

type MySQLMonitorTarget struct {
	Matched       bool              `json:"matched"`
	Health        string            `json:"health"`
	ScrapeURL     string            `json:"scrapeUrl"`
	MetricsURL    string            `json:"metricsUrl"`
	MetricsType   string            `json:"metricsType"`
	TargetAddress string            `json:"targetAddress"`
	TargetHost    string            `json:"targetHost"`
	HostScrapeURL string            `json:"hostScrapeUrl"`
	HostSelector  string            `json:"hostSelector"`
	LastError     string            `json:"lastError"`
	Selector      string            `json:"selector"`
	Labels        map[string]string `json:"labels"`
	MatchBy       string            `json:"matchBy"`
	Prometheus    string            `json:"prometheus"`
}

type MonitorMetricCard struct {
	Key         string  `json:"key"`
	Name        string  `json:"name"`
	Value       float64 `json:"value"`
	Unit        string  `json:"unit"`
	Status      string  `json:"status"`
	Description string  `json:"description"`
	PromQL      string  `json:"promql"`
	Required    bool    `json:"required"`
	Error       string  `json:"error,omitempty"`
}

type MonitorTrendPanel struct {
	Key    string                `json:"key"`
	Title  string                `json:"title"`
	Unit   string                `json:"unit"`
	Series []MonitorMetricSeries `json:"series"`
}

type MonitorMetricSeries struct {
	Name     string               `json:"name"`
	PromQL   string               `json:"promql"`
	Points   []MonitorMetricPoint `json:"points"`
	Required bool                 `json:"required"`
	Error    string               `json:"error,omitempty"`
}

type MonitorMetricPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

type MonitorTablePanel struct {
	Key     string                   `json:"key"`
	Title   string                   `json:"title"`
	Columns []MonitorTableColumn     `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
}

type MonitorTableColumn struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Unit  string `json:"unit,omitempty"`
}

type TreeQuery struct {
	AssetID    uint   `form:"assetId"`
	Level      string `form:"level"`
	Database   string `form:"database"`
	Schema     string `form:"schema"`
	ObjectName string `form:"objectName"`
	ObjectType string `form:"objectType"`
	Keyword    string `form:"keyword"`
}

type TreeNode struct {
	Key        string                 `json:"key"`
	Label      string                 `json:"label"`
	Type       string                 `json:"type"`
	AssetID    uint                   `json:"assetId"`
	Engine     int                    `json:"engine"`
	Database   string                 `json:"database"`
	Schema     string                 `json:"schema"`
	ObjectName string                 `json:"objectName"`
	Leaf       bool                   `json:"leaf"`
	Extra      map[string]interface{} `json:"extra,omitempty"`
}

type ObjectRequest struct {
	AssetID    uint   `form:"assetId" json:"assetId"`
	Database   string `form:"database" json:"database"`
	Schema     string `form:"schema" json:"schema"`
	ObjectName string `form:"objectName" json:"objectName"`
	ObjectType string `form:"objectType" json:"objectType"`
}

type ColumnInfo struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Nullable      bool   `json:"nullable"`
	DefaultValue  string `json:"defaultValue"`
	Comment       string `json:"comment"`
	PrimaryKey    bool   `json:"primaryKey"`
	AutoIncrement bool   `json:"autoIncrement"`
}

type IndexInfo struct {
	Name      string   `json:"name"`
	Columns   []string `json:"columns"`
	Unique    bool     `json:"unique"`
	Primary   bool     `json:"primary"`
	IndexType string   `json:"indexType"`
	Comment   string   `json:"comment"`
}

type DDLResponse struct {
	DDL string `json:"ddl"`
}

type DataFilter struct {
	Column   string      `json:"column"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

type DataSort struct {
	Column string `json:"column"`
	Order  string `json:"order"`
}

type DataQueryRequest struct {
	AssetID    uint         `json:"assetId"`
	Database   string       `json:"database"`
	Schema     string       `json:"schema"`
	ObjectName string       `json:"objectName"`
	ObjectType string       `json:"objectType"`
	Page       int          `json:"page"`
	PageSize   int          `json:"pageSize"`
	ExportAll  bool         `json:"exportAll"`
	Columns    []string     `json:"columns"`
	Filters    []DataFilter `json:"filters"`
	Sorts      []DataSort   `json:"sorts"`
}

type DataCreateRequest struct {
	AssetID    uint                   `json:"assetId"`
	Database   string                 `json:"database"`
	Schema     string                 `json:"schema"`
	ObjectName string                 `json:"objectName"`
	ObjectType string                 `json:"objectType"`
	Values     map[string]interface{} `json:"values"`
}

type DataUpdateRequest struct {
	AssetID    uint                   `json:"assetId"`
	Database   string                 `json:"database"`
	Schema     string                 `json:"schema"`
	ObjectName string                 `json:"objectName"`
	ObjectType string                 `json:"objectType"`
	Keys       map[string]interface{} `json:"keys"`
	Values     map[string]interface{} `json:"values"`
}

type DataDeleteRequest struct {
	AssetID    uint                   `json:"assetId"`
	Database   string                 `json:"database"`
	Schema     string                 `json:"schema"`
	ObjectName string                 `json:"objectName"`
	ObjectType string                 `json:"objectType"`
	Keys       map[string]interface{} `json:"keys"`
}

type DataResult struct {
	Columns     []ColumnInfo             `json:"columns"`
	Rows        []map[string]interface{} `json:"rows"`
	PrimaryKeys []string                 `json:"primaryKeys"`
	Page        int                      `json:"page"`
	PageSize    int                      `json:"pageSize"`
	Total       int64                    `json:"total"`
	Editable    bool                     `json:"editable"`
}

type MutationResult struct {
	RowsAffected int64                  `json:"rowsAffected"`
	LastInsertID int64                  `json:"lastInsertId,omitempty"`
	Keys         map[string]interface{} `json:"keys,omitempty"`
}

type SQLExecuteRequest struct {
	AssetID   uint       `json:"assetId"`
	Database  string     `json:"database"`
	Schema    string     `json:"schema"`
	SQL       string     `json:"sql"`
	Page      int        `json:"page"`
	PageSize  int        `json:"pageSize"`
	Confirmed SQLConfirm `json:"confirmed"`
}

type SQLConfirm bool

func (v *SQLConfirm) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*v = false
		return nil
	}

	var value bool
	if err := json.Unmarshal(data, &value); err == nil {
		*v = SQLConfirm(value)
		return nil
	}

	// Some frontends may accidentally pass the click event object as confirmed.
	// Treat non-boolean values as unconfirmed instead of failing the whole request.
	*v = false
	return nil
}

func (v SQLConfirm) Bool() bool {
	return bool(v)
}

type SQLExecuteResult struct {
	Columns        []ColumnInfo             `json:"columns"`
	Rows           []map[string]interface{} `json:"rows"`
	Page           int                      `json:"page"`
	PageSize       int                      `json:"pageSize"`
	Total          int64                    `json:"total"`
	DurationMS     int64                    `json:"durationMs"`
	SQL            string                   `json:"sql"`
	ReadOnly       bool                     `json:"readOnly"`
	RequireConfirm bool                     `json:"requireConfirm"`
	Estimate       SQLExecutionEstimate     `json:"estimate"`
}

type SQLExecutionEstimate struct {
	EstimatedRows    int64  `json:"estimatedRows"`
	EstimatedSeconds int    `json:"estimatedSeconds"`
	RiskLevel        string `json:"riskLevel"`
	Message          string `json:"message"`
}

type SQLCompletionQuery struct {
	AssetID    uint   `form:"assetId"`
	Database   string `form:"database"`
	Schema     string `form:"schema"`
	ObjectName string `form:"objectName"`
	Keyword    string `form:"keyword"`
}

type SQLCompletionResponse struct {
	Keywords  []string              `json:"keywords"`
	Databases []SQLCompletionItem   `json:"databases"`
	Tables    []SQLCompletionItem   `json:"tables"`
	Columns   []SQLCompletionColumn `json:"columns"`
}

type SQLCompletionItem struct {
	Label      string `json:"label"`
	Value      string `json:"value"`
	Type       string `json:"type"`
	Database   string `json:"database,omitempty"`
	Schema     string `json:"schema,omitempty"`
	ObjectName string `json:"objectName,omitempty"`
}

type SQLCompletionColumn struct {
	Label      string `json:"label"`
	Value      string `json:"value"`
	Type       string `json:"type"`
	DataType   string `json:"dataType"`
	Database   string `json:"database,omitempty"`
	Schema     string `json:"schema,omitempty"`
	ObjectName string `json:"objectName,omitempty"`
	PrimaryKey bool   `json:"primaryKey"`
}

const (
	SQLAssistantModeTextToSQL = "text_to_sql"
	SQLAssistantModeOptimize  = "optimize"
	SQLAssistantModeExplain   = "explain"
)

const (
	DBAIToolSearchObjects      = "db_search_objects"
	DBAIToolGetTableSchema     = "db_get_table_schema"
	DBAIToolExecuteReadonlySQL = "db_execute_readonly_sql"
	DBAIToolExplainSQL         = "db_explain_sql"
	DBAIToolGetMonitorSnapshot = "db_get_monitor_snapshot"
	DBAIToolGetSlowSQLTop      = "db_get_slow_sql_top"
	DBAIToolGetInstanceHealth  = "db_get_instance_health"
)

type SQLAssistantRequest struct {
	AssetID      uint   `json:"assetId"`
	Database     string `json:"database"`
	Schema       string `json:"schema"`
	ObjectName   string `json:"objectName"`
	ObjectType   string `json:"objectType"`
	Mode         string `json:"mode"`
	Input        string `json:"input"`
	SQL          string `json:"sql"`
	ProviderID   uint   `json:"providerId"`
	CustomPrompt string `json:"customPrompt"`
}

type SQLAssistantResult struct {
	Mode         string   `json:"mode"`
	SQL          string   `json:"sql"`
	Explanation  string   `json:"explanation"`
	Warnings     []string `json:"warnings"`
	Indexes      []string `json:"indexes"`
	Confidence   string   `json:"confidence"`
	RawAnswer    string   `json:"rawAnswer"`
	UsedLLM      bool     `json:"usedLlm"`
	ProviderID   uint     `json:"providerId"`
	ProviderName string   `json:"providerName"`
	ModelCode    string   `json:"modelCode"`
}

type DBAIToolDefinition struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	ReadOnly       bool     `json:"readOnly"`
	RequiredFields []string `json:"requiredFields"`
	OptionalFields []string `json:"optionalFields"`
}

type DBAIToolRequest struct {
	Tool       string     `json:"tool"`
	AssetID    uint       `json:"assetId"`
	AuthID     uint       `json:"authId"`
	Database   string     `json:"database"`
	Schema     string     `json:"schema"`
	ObjectName string     `json:"objectName"`
	ObjectType string     `json:"objectType"`
	Keyword    string     `json:"keyword"`
	SQL        string     `json:"sql"`
	Range      string     `json:"range"`
	Step       string     `json:"step"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
	TailLines  int        `json:"tailLines"`
	Confirmed  SQLConfirm `json:"confirmed"`
}

type DBAIChatRequest struct {
	AssetID            uint     `json:"assetId"`
	Database           string   `json:"database"`
	Schema             string   `json:"schema"`
	ObjectName         string   `json:"objectName"`
	ObjectType         string   `json:"objectType"`
	Question           string   `json:"question"`
	SQL                string   `json:"sql"`
	Range              string   `json:"range"`
	Step               string   `json:"step"`
	ProviderID         uint     `json:"providerId"`
	Tools              []string `json:"tools"`
	IncludeRawEvidence bool     `json:"includeRawEvidence"`
	CustomPrompt       string   `json:"customPrompt"`
}

type DBAIChatResult struct {
	Answer          string             `json:"answer"`
	Status          string             `json:"status"`
	Summary         string             `json:"summary"`
	Intent          DBAIIntent         `json:"intent"`
	ToolPlan        []DBAIToolPlanItem `json:"toolPlan"`
	Suggestions     []string           `json:"suggestions"`
	Warnings        []string           `json:"warnings"`
	MissingEvidence []string           `json:"missingEvidence"`
	Evidence        []DBAIToolEvidence `json:"evidence"`
	ToolCalls       []string           `json:"toolCalls"`
	RawAnswer       string             `json:"rawAnswer"`
	UsedLLM         bool               `json:"usedLlm"`
	ProviderID      uint               `json:"providerId"`
	ProviderName    string             `json:"providerName"`
	ModelCode       string             `json:"modelCode"`
}

type DBAIIntent struct {
	Category     string `json:"category"`
	Description  string `json:"description"`
	ReadOnly     bool   `json:"readOnly"`
	RiskLevel    string `json:"riskLevel"`
	Reason       string `json:"reason"`
	NeedsSQL     bool   `json:"needsSql"`
	NeedsMonitor bool   `json:"needsMonitor"`
	NeedsSchema  bool   `json:"needsSchema"`
}

type DBAIToolPlanItem struct {
	Tool       string `json:"tool"`
	Reason     string `json:"reason"`
	Required   bool   `json:"required"`
	Skipped    bool   `json:"skipped"`
	SkipReason string `json:"skipReason,omitempty"`
}

type DBAIToolEvidence struct {
	Tool      string      `json:"tool"`
	Supported bool        `json:"supported"`
	Summary   string      `json:"summary"`
	Warnings  []string    `json:"warnings"`
	Data      interface{} `json:"data,omitempty"`
}

type DBAIToolResult struct {
	Tool      string      `json:"tool"`
	Supported bool        `json:"supported"`
	Summary   string      `json:"summary"`
	Data      interface{} `json:"data,omitempty"`
	Warnings  []string    `json:"warnings"`
}

type DBTableSchemaResult struct {
	Columns []ColumnInfo `json:"columns"`
	Indexes []IndexInfo  `json:"indexes"`
	DDL     string       `json:"ddl"`
}

type DBExplainSQLResult struct {
	SQL      string                   `json:"sql"`
	Engine   int                      `json:"engine"`
	Plan     []map[string]interface{} `json:"plan"`
	Summary  DBExplainSummary         `json:"summary"`
	Estimate SQLExecutionEstimate     `json:"estimate"`
}

type DBExplainSummary struct {
	RiskLevel       string   `json:"riskLevel"`
	FullScan        bool     `json:"fullScan"`
	UsesIndex       bool     `json:"usesIndex"`
	EstimatedRows   int64    `json:"estimatedRows"`
	ProblemTables   []string `json:"problemTables"`
	UsedIndexes     []string `json:"usedIndexes"`
	Suggestions     []string `json:"suggestions"`
	RawPlanAnalyzed bool     `json:"rawPlanAnalyzed"`
}

type DBMonitorSnapshot struct {
	Asset    AssetVO             `json:"asset"`
	Target   MySQLMonitorTarget  `json:"target"`
	Overview []MonitorMetricCard `json:"overview"`
	Range    MonitorQueryRange   `json:"range"`
}

type DBInstanceHealthResult struct {
	Asset       AssetVO             `json:"asset"`
	Target      MySQLMonitorTarget  `json:"target"`
	Health      string              `json:"health"`
	Status      string              `json:"status"`
	Abnormal    []MonitorMetricCard `json:"abnormal"`
	Warnings    []string            `json:"warnings"`
	Prometheus  string              `json:"prometheus"`
	MetricsURL  string              `json:"metricsUrl"`
	HostMetrics string              `json:"hostMetricsUrl"`
}

type DBSlowSQLTopResult struct {
	SourceConfigured bool                     `json:"sourceConfigured"`
	Source           string                   `json:"source"`
	Items            []map[string]interface{} `json:"items"`
	Message          string                   `json:"message"`
}

func NormalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = DefaultPage
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return page, pageSize
}
