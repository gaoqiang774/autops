package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	CmdbResourceTypeBusinessSystem  = "business_system"
	CmdbResourceTypeBusinessService = "business_service"
	CmdbResourceTypeApplication     = "application"
	CmdbResourceTypeNginx           = "nginx"
	CmdbResourceTypeGateway         = "gateway"
	CmdbResourceTypePhysicalServer  = "physical_server"
	CmdbResourceTypeHost            = "host"
	CmdbResourceTypeVM              = "vm"
	CmdbResourceTypeCloudHost       = "cloud_host"
	CmdbResourceTypeK8sCluster      = "k8s_cluster"
	CmdbResourceTypeK8sNamespace    = "k8s_namespace"
	CmdbResourceTypeK8sWorkload     = "k8s_workload"
	CmdbResourceTypeDatabase        = "database"
	CmdbResourceTypeMiddleware      = "middleware"
	CmdbResourceTypeLogProject      = "log_project"
	CmdbResourceTypeLogStore        = "log_store"
)

const (
	CmdbRelationContains   = "contains"
	CmdbRelationRunsOn     = "runs_on"
	CmdbRelationConnectsTo = "connects_to"
	CmdbRelationDependsOn  = "depends_on"
	CmdbRelationDeployedOn = "deployed_on"
	CmdbRelationUses       = "uses"
	CmdbRelationEmitsLogTo = "emits_log_to"
)

const (
	CmdbHealthDimensionOverall    = "overall"
	CmdbHealthDimensionInfra      = "infra"
	CmdbHealthDimensionMiddleware = "middleware"
	CmdbHealthDimensionContainer  = "container"
	CmdbHealthDimensionBusiness   = "business"
	CmdbHealthDimensionLog        = "log"
)

const (
	CmdbHealthStatusHealthy  = "healthy"
	CmdbHealthStatusWarning  = "warning"
	CmdbHealthStatusCritical = "critical"
	CmdbHealthStatusUnknown  = "unknown"
)

type CmdbBusinessModel struct {
	gorm.Model
	Name        string         `gorm:"column:name;type:varchar(200);not null;uniqueIndex;comment:业务模型名称" json:"name"`
	DisplayName string         `gorm:"column:display_name;type:varchar(200);index;comment:展示名称" json:"displayName"`
	Env         string         `gorm:"column:env;type:varchar(50);index;comment:环境" json:"env"`
	Owner       string         `gorm:"column:owner;type:varchar(64);index;comment:负责人" json:"owner"`
	BizCode     string         `gorm:"column:biz_code;type:varchar(128);index;comment:业务编码" json:"bizCode"`
	Properties  datatypes.JSON `gorm:"column:properties;type:json;comment:业务属性" json:"properties"`
	Status      string         `gorm:"column:status;type:varchar(20);default:active;index;comment:状态" json:"status"`
}

func (CmdbBusinessModel) TableName() string {
	return "cmdb_business_model"
}

type CmdbBusinessModelBinding struct {
	gorm.Model
	BusinessID   uint           `gorm:"column:business_id;not null;index;comment:业务模型ID" json:"businessId"`
	ParentID     uint           `gorm:"column:parent_id;index;comment:父绑定节点ID" json:"parentId"`
	Name         string         `gorm:"column:name;type:varchar(200);not null;index;comment:节点名称" json:"name"`
	DisplayName  string         `gorm:"column:display_name;type:varchar(200);index;comment:展示名称" json:"displayName"`
	ResourceType string         `gorm:"column:resource_type;type:varchar(50);not null;index;comment:资源类型" json:"resourceType"`
	SourceType   string         `gorm:"column:source_type;type:varchar(50);index;comment:来源模块类型" json:"sourceType"`
	SourceID     uint           `gorm:"column:source_id;index;comment:来源模块资源ID" json:"sourceId"`
	SourceKey    string         `gorm:"column:source_key;type:varchar(255);index;comment:来源模块资源唯一键" json:"sourceKey"`
	Env          string         `gorm:"column:env;type:varchar(50);index;comment:环境" json:"env"`
	Owner        string         `gorm:"column:owner;type:varchar(64);index;comment:负责人" json:"owner"`
	BizCode      string         `gorm:"column:biz_code;type:varchar(128);index;comment:业务编码" json:"bizCode"`
	Properties   datatypes.JSON `gorm:"column:properties;type:json;comment:节点属性" json:"properties"`
	InstanceId   string         `gorm:"column:instance_id;type:varchar(128);index;comment:实例唯一ID" json:"instance_id"`
	Status       string         `gorm:"column:status;type:varchar(20);default:active;index;comment:状态" json:"status"`
	Tags         datatypes.JSON `gorm:"column:tags;type:json;comment:标签" json:"tags"`

	Business *CmdbBusinessModel `gorm:"foreignKey:BusinessID" json:"business,omitempty"`
}

func (CmdbBusinessModelBinding) TableName() string {
	return "cmdb_business_model_binding"
}

type CmdbBusinessModelRelation struct {
	gorm.Model
	BusinessID   uint                      `gorm:"column:business_id;not null;index;uniqueIndex:uk_cmdb_business_relation;comment:业务模型ID" json:"businessId"`
	SrcID        uint                      `gorm:"column:src_id;type:int unsigned;not null;index;uniqueIndex:uk_cmdb_business_relation;comment:源绑定节点ID" json:"srcId"`
	DstID        uint                      `gorm:"column:dst_id;type:int unsigned;not null;index;uniqueIndex:uk_cmdb_business_relation;comment:目标绑定节点ID" json:"dstId"`
	RelationType string                    `gorm:"column:relation_type;type:varchar(50);not null;index;uniqueIndex:uk_cmdb_business_relation;comment:关系类型" json:"relationType"`
	Properties   datatypes.JSON            `gorm:"column:properties;type:json;comment:关系属性" json:"properties"`
	Status       int8                      `gorm:"column:status;type:tinyint;default:1;comment:状态 1-有效 0-无效" json:"status"`
	Src          *CmdbBusinessModelBinding `gorm:"foreignKey:SrcID;comment:上游" json:"src,omitempty"`
	Dst          *CmdbBusinessModelBinding `gorm:"foreignKey:DstID;comment:下游" json:"dst,omitempty"`
}

func (CmdbBusinessModelRelation) TableName() string {
	return "cmdb_business_model_relation"
}

type CmdbBusinessModelHealthSnapshot struct {
	ID         uint           `gorm:"column:id;primaryKey;comment:主键ID" json:"id"`
	BusinessID uint           `gorm:"column:business_id;not null;uniqueIndex:uk_cmdb_business_health_resource_dimension;index;comment:业务模型ID" json:"businessId"`
	ResourceID uint           `gorm:"column:resource_id;not null;uniqueIndex:uk_cmdb_business_health_resource_dimension;index;comment:绑定节点ID" json:"resourceId"`
	Dimension  string         `gorm:"column:dimension;type:varchar(50);not null;uniqueIndex:uk_cmdb_business_health_resource_dimension;index;comment:健康维度" json:"dimension"`
	Score      int            `gorm:"column:score;type:int;not null;default:100;comment:健康评分" json:"score"`
	Status     string         `gorm:"column:status;type:varchar(20);not null;default:healthy;index;comment:健康状态" json:"status"`
	Source     string         `gorm:"column:source;type:varchar(50);comment:数据来源" json:"source"`
	Message    string         `gorm:"column:message;type:varchar(500);comment:摘要信息" json:"message"`
	Evidence   datatypes.JSON `gorm:"column:evidence;type:json;comment:健康证据" json:"evidence"`
	CreatedAt  time.Time      `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`

	Resource *CmdbBusinessModelBinding `gorm:"foreignKey:ResourceID" json:"resource,omitempty"`
}

func (CmdbBusinessModelHealthSnapshot) TableName() string {
	return "cmdb_business_model_health_snapshot"
}

type CreateBusinessModelDto struct {
	Name        string                 `json:"name" binding:"required"`
	DisplayName string                 `json:"displayName"`
	Env         string                 `json:"env"`
	Owner       string                 `json:"owner"`
	BizCode     string                 `json:"bizCode"`
	Properties  map[string]interface{} `json:"properties"`
}

type BusinessModelBindingDto struct {
	ParentID  uint                              `json:"parentId"`
	Resources []BusinessModelBindingResourceDto `json:"resources" binding:"required"`
}

type BusinessModelBindingResourceDto struct {
	ResourceID   uint                   `json:"resourceId"`
	Name         string                 `json:"name"`
	Label        string                 `json:"label"`
	DisplayName  string                 `json:"displayName"`
	ResourceType string                 `json:"resourceType"`
	Type         string                 `json:"type"`
	SourceType   string                 `json:"sourceType"`
	SourceID     uint                   `json:"sourceId"`
	SourceKey    string                 `json:"sourceKey"`
	Env          string                 `json:"env"`
	Owner        string                 `json:"owner"`
	BizCode      string                 `json:"bizCode"`
	RelationType string                 `json:"relationType"`
	Properties   map[string]interface{} `json:"properties"`
	Status       string                 `json:"status"`
}

type UpsertHealthSnapshotDto struct {
	ResourceID uint                   `json:"resourceId" binding:"required"`
	Dimension  string                 `json:"dimension"`
	Score      int                    `json:"score"`
	Status     string                 `json:"status"`
	Source     string                 `json:"source"`
	Message    string                 `json:"message"`
	Evidence   map[string]interface{} `json:"evidence"`
}

type BusinessModelQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Keyword  string `form:"keyword"`
	Env      string `form:"env"`
	Status   string `form:"status"`
}

type HealthDimensionVo struct {
	Dimension string                 `json:"dimension"`
	Label     string                 `json:"label"`
	Score     int                    `json:"score"`
	Status    string                 `json:"status"`
	Color     string                 `json:"color"`
	Message   string                 `json:"message"`
	Evidence  map[string]interface{} `json:"evidence,omitempty"`
	UpdatedAt *time.Time             `json:"updatedAt,omitempty"`
}

type BusinessModelSummaryVo struct {
	ID             uint                `json:"id"`
	Name           string              `json:"name"`
	DisplayName    string              `json:"displayName"`
	ResourceType   string              `json:"resourceType"`
	Env            string              `json:"env"`
	Owner          string              `json:"owner"`
	BizCode        string              `json:"bizCode"`
	HealthScore    int                 `json:"healthScore"`
	HealthStatus   string              `json:"healthStatus"`
	HealthColor    string              `json:"healthColor"`
	ProblemSummary string              `json:"problemSummary"`
	Dimensions     []HealthDimensionVo `json:"dimensions,omitempty"`
	UpdatedAt      time.Time           `json:"updatedAt"`
}

type BusinessObservabilityQuery struct {
	Window string `form:"window" json:"window"`
	Start  string `form:"start" json:"start"`
	End    string `form:"end" json:"end"`
}

type BusinessLogAnalysisQuery struct {
	BusinessObservabilityQuery
	SourceType  string `form:"sourceType" json:"sourceType"`
	StoreID     uint   `form:"storeId" json:"storeId"`
	Domain      string `form:"domain" json:"domain"`
	StatusClass string `form:"statusClass" json:"statusClass"`
	RequestPath string `form:"requestPath" json:"requestPath"`
	ClientIP    string `form:"clientIp" json:"clientIp"`
}

type BusinessMonitorAlertQuery struct {
	Page         int    `form:"page" json:"page"`
	PageSize     int    `form:"pageSize" json:"pageSize"`
	Severity     string `form:"severity" json:"severity"`
	Status       string `form:"status" json:"status"`
	DatasourceID uint   `form:"datasourceId" json:"datasourceId"`
}

type BusinessMonitorAlertMatchRuleDto struct {
	Name        string `json:"name"`
	Selector    string `json:"selector" binding:"required"`
	Enabled     *bool  `json:"enabled"`
	Description string `json:"description"`
}

type BusinessMonitorAlertConfigDto struct {
	Enabled        *bool                              `json:"enabled"`
	DatasourceID   uint                               `json:"datasourceId"`
	DatasourceType string                             `json:"datasourceType"`
	MatchRules     []BusinessMonitorAlertMatchRuleDto `json:"matchRules"`
}

type BusinessSLAMetricsVo struct {
	RequestTotal                int64   `json:"requestTotal"`
	HTTP4xx                     int64   `json:"http4xx"`
	HTTP5xx                     int64   `json:"http5xx"`
	SlowRequestCount            int64   `json:"slowRequestCount"`
	ActiveAlertCount            int64   `json:"activeAlertCount"`
	SuccessRate                 float64 `json:"successRate"`
	AvailabilityRate            float64 `json:"availabilityRate"`
	ErrorRate                   float64 `json:"errorRate"`
	HTTP4xxRate                 float64 `json:"http4xxRate"`
	HTTP5xxRate                 float64 `json:"http5xxRate"`
	RequestErrorRate            float64 `json:"requestErrorRate"`
	SystemErrorRate             float64 `json:"systemErrorRate"`
	SlowRequestRate             float64 `json:"slowRequestRate"`
	AvgLatencyMs                float64 `json:"avgLatencyMs"`
	P95LatencyMs                float64 `json:"p95LatencyMs"`
	ResponseBytes               int64   `json:"responseBytes"`
	UniqueClientCount           int64   `json:"uniqueClientCount"`
	SLOTarget                   float64 `json:"sloTarget"`
	SLOMet                      bool    `json:"sloMet"`
	SLOStatus                   string  `json:"sloStatus"`
	ErrorBudgetConsumedPercent  float64 `json:"errorBudgetConsumedPercent"`
	SlowRequestThresholdSeconds float64 `json:"slowRequestThresholdSeconds"`
	HasTraffic                  bool    `json:"hasTraffic"`
	AggregationMode             string  `json:"aggregationMode"`
}

type BusinessLogSourceVo struct {
	StoreID        uint   `json:"storeId"`
	StoreCode      string `json:"storeCode"`
	StoreName      string `json:"storeName"`
	ProjectID      uint   `json:"projectId"`
	Status         int    `json:"status"`
	QueryStatus    string `json:"queryStatus"`
	QueryDuration  int64  `json:"queryDurationMs"`
	BoundNodeIDs   []uint `json:"boundNodeIds"`
	DirectNodeIDs  []uint `json:"directNodeIds"`
	WarningMessage string `json:"warningMessage,omitempty"`
}

type BusinessNodeSignalVo struct {
	ResourceID       uint    `json:"resourceId"`
	RequestTotal     int64   `json:"requestTotal"`
	HTTP4xx          int64   `json:"http4xx"`
	HTTP5xx          int64   `json:"http5xx"`
	ErrorRate        float64 `json:"errorRate"`
	HTTP5xxRate      float64 `json:"http5xxRate"`
	AvgLatencyMs     float64 `json:"avgLatencyMs"`
	P95LatencyMs     float64 `json:"p95LatencyMs"`
	HealthStatus     string  `json:"healthStatus"`
	HealthColor      string  `json:"healthColor"`
	Message          string  `json:"message"`
	BoundLogStoreIDs []uint  `json:"boundLogStoreIds"`
}

type BusinessSLATopAPIVo struct {
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
	UniqueClients int64   `json:"uniqueClients"`
	StoreIDs      []uint  `json:"storeIds"`
}

type BusinessSLARecentErrorVo struct {
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
	StoreID       uint    `json:"storeId"`
	StoreCode     string  `json:"storeCode"`
}
