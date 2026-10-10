package model

import "dodevops-api/common/util"

const (
	RuleStatusDisabled = 0
	RuleStatusEnabled  = 1

	TemplateGroupLevelComponent = "component"
	TemplateGroupLevelCollector = "collector"
	TemplateGroupLevelCategory  = "category"
)

type AlertRuleGroup struct {
	ID          uint             `gorm:"primaryKey" json:"id"`
	ParentID    uint             `gorm:"column:parent_id;default:0;index" json:"parentId"`
	Name        string           `gorm:"size:100;not null;index" json:"name"`
	Description string           `gorm:"size:500" json:"description"`
	Sort        int              `gorm:"type:integer;default:0" json:"sort"`
	Enabled     int              `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	CreatedBy   string           `gorm:"size:64" json:"createdBy"`
	UpdatedBy   string           `gorm:"size:64" json:"updatedBy"`
	CreatedAt   util.HTime       `json:"createdAt"`
	UpdatedAt   util.HTime       `json:"updatedAt"`
	RuleCount   int64            `gorm:"-" json:"ruleCount"`
	Children    []AlertRuleGroup `gorm:"-" json:"children,omitempty"`
}

func (AlertRuleGroup) TableName() string {
	return "alert_rule_groups"
}

type AlertRule struct {
	ID                      uint            `gorm:"primaryKey" json:"id"`
	Name                    string          `gorm:"size:150;not null;index" json:"name"`
	Note                    string          `gorm:"type:text" json:"note"`
	GroupID                 uint            `gorm:"column:group_id;index" json:"groupId"`
	DatasourceID            uint            `gorm:"column:datasource_id;index" json:"datasourceId"`
	DatasourceType          string          `gorm:"size:64;not null;index" json:"datasourceType"`
	DatasourceQueries       JSONArray       `gorm:"type:json" json:"datasourceQueries"`
	Query                   string          `gorm:"type:text;not null" json:"query"`
	ConditionOperator       string          `gorm:"size:16" json:"conditionOperator"`
	ConditionValue          float64         `gorm:"type:double" json:"conditionValue"`
	RuleConfig              JSONMap         `gorm:"type:json" json:"ruleConfig"`
	AppendTags              JSONMap         `gorm:"type:json" json:"appendTags"`
	CronPattern             string          `gorm:"size:64;not null;default:'@every 60s'" json:"cronPattern"`
	EvalIntervalSeconds     int64           `gorm:"type:bigint;not null;default:60" json:"evalIntervalSeconds"`
	ForDurationSeconds      int64           `gorm:"type:bigint;not null;default:0" json:"forDurationSeconds"`
	ForDurationCount        int64           `gorm:"-" json:"forDurationCount,omitempty"`
	RecoverDurationSeconds  int64           `gorm:"type:bigint;not null;default:0" json:"recoverDurationSeconds"`
	Severity                string          `gorm:"size:32;not null;default:'warning';index" json:"severity"`
	Enabled                 int             `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	Internal                int             `gorm:"type:tinyint;not null;default:0;index" json:"internal"`
	EnableInBG              int             `gorm:"column:enable_in_bg;type:tinyint;not null;default:0;index" json:"enableInBg"`
	TimeZone                string          `gorm:"size:64;not null;default:'Asia/Shanghai'" json:"timeZone"`
	Labels                  JSONMap         `gorm:"type:json" json:"labels"`
	Annotations             JSONMap         `gorm:"type:json" json:"annotations"`
	AssetScope              JSONMap         `gorm:"type:json" json:"assetScope"`
	BusinessID              string          `gorm:"size:128;index" json:"businessId"`
	NoDataPolicy            string          `gorm:"size:32;default:'ignore'" json:"noDataPolicy"`
	ErrorPolicy             string          `gorm:"size:32;default:'ignore'" json:"errorPolicy"`
	EffectiveTime           JSONMap         `gorm:"type:json" json:"effectiveTime"`
	NotifyRuleIDs           JSONUintArray   `gorm:"column:notify_rule_ids;type:json" json:"notifyRuleIds"`
	NotifyRecovered         int             `gorm:"type:tinyint;not null;default:1" json:"notifyRecovered"`
	NotifyRepeatStepMinutes int64           `gorm:"type:bigint;not null;default:60" json:"notifyRepeatStepMinutes"`
	NotifyMaxNumber         int             `gorm:"type:integer;not null;default:0" json:"notifyMaxNumber"`
	PipelineConfigs         JSONArray       `gorm:"type:json" json:"pipelineConfigs"`
	RunbookURL              string          `gorm:"column:runbook_url;size:500" json:"runbookUrl"`
	Callbacks               JSONStringArray `gorm:"type:json" json:"callbacks"`
	SelfHealConfig          JSONMap         `gorm:"type:json" json:"selfHealConfig"`
	CreatedBy               string          `gorm:"size:64" json:"createdBy"`
	UpdatedBy               string          `gorm:"size:64" json:"updatedBy"`
	CreatedAt               util.HTime      `json:"createdAt"`
	UpdatedAt               util.HTime      `json:"updatedAt"`
}

func (AlertRule) TableName() string {
	return "alert_rules"
}

type AlertBuiltinRuleTemplate struct {
	ID                         uint       `gorm:"primaryKey" json:"id"`
	ResourceType               string     `gorm:"size:64;not null;default:'host';index" json:"resourceType"`
	Component                  string     `gorm:"size:100;not null;default:'';index" json:"component"`
	Collector                  string     `gorm:"size:100;not null;default:'';index" json:"collector"`
	Category                   string     `gorm:"size:64;not null;index" json:"category"`
	Name                       string     `gorm:"size:150;not null;index" json:"name"`
	Description                string     `gorm:"size:500" json:"description"`
	DatasourceType             string     `gorm:"size:64;not null;index" json:"datasourceType"`
	Source                     string     `gorm:"size:64;not null;default:'manual';index" json:"source"`
	SourceUUID                 string     `gorm:"size:128;not null;default:'';index" json:"sourceUuid"`
	QueryTemplate              string     `gorm:"type:text;not null" json:"queryTemplate"`
	DefaultConditionOperator   string     `gorm:"size:16" json:"defaultConditionOperator"`
	DefaultConditionValue      float64    `gorm:"type:double" json:"defaultConditionValue"`
	DefaultEvalIntervalSeconds int64      `gorm:"type:bigint;not null;default:60" json:"defaultEvalIntervalSeconds"`
	DefaultForDurationSeconds  int64      `gorm:"type:bigint;not null;default:0" json:"defaultForDurationSeconds"`
	DefaultSeverity            string     `gorm:"size:32;not null;default:'warning'" json:"defaultSeverity"`
	DefaultEnabled             int        `gorm:"type:tinyint;not null;default:0" json:"defaultEnabled"`
	Labels                     JSONMap    `gorm:"type:json" json:"labels"`
	Annotations                JSONMap    `gorm:"type:json" json:"annotations"`
	Variables                  JSONMap    `gorm:"type:json" json:"variables"`
	Tags                       JSONMap    `gorm:"type:json" json:"tags"`
	RuleConfig                 JSONMap    `gorm:"type:json" json:"ruleConfig"`
	Sort                       int        `gorm:"type:integer;default:0" json:"sort"`
	Enabled                    int        `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	CreatedAt                  util.HTime `json:"createdAt"`
	UpdatedAt                  util.HTime `json:"updatedAt"`
}

func (AlertBuiltinRuleTemplate) TableName() string {
	return "alert_builtin_rule_templates"
}

type AlertBuiltinRuleTemplateGroup struct {
	ID            uint                            `gorm:"primaryKey" json:"id"`
	ParentID      uint                            `gorm:"column:parent_id;default:0;index" json:"parentId"`
	Level         string                          `gorm:"size:32;not null;index" json:"level"`
	Name          string                          `gorm:"size:100;not null;index" json:"name"`
	Value         string                          `gorm:"size:100;not null;index" json:"value"`
	ResourceType  string                          `gorm:"size:64;default:'';index" json:"-" form:"resourceType"`
	Description   string                          `gorm:"size:500" json:"description"`
	Sort          int                             `gorm:"type:integer;default:0" json:"sort"`
	Builtin       int                             `gorm:"type:tinyint;not null;default:0;index" json:"builtin"`
	Enabled       int                             `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	CreatedBy     string                          `gorm:"size:64" json:"createdBy"`
	UpdatedBy     string                          `gorm:"size:64" json:"updatedBy"`
	CreatedAt     util.HTime                      `json:"createdAt"`
	UpdatedAt     util.HTime                      `json:"updatedAt"`
	Key           string                          `gorm:"-" json:"key,omitempty"`
	Children      []AlertBuiltinRuleTemplateGroup `gorm:"-" json:"children,omitempty"`
	TemplateCount int64                           `gorm:"-" json:"templateCount"`
}

func (AlertBuiltinRuleTemplateGroup) TableName() string {
	return "alert_builtin_rule_template_groups"
}

type AlertRuleQuery struct {
	PageQuery
	GroupID        uint   `form:"groupId" json:"groupId"`
	DatasourceID   uint   `form:"datasourceId" json:"datasourceId"`
	DatasourceType string `form:"datasourceType" json:"datasourceType"`
	Severity       string `form:"severity" json:"severity"`
	Enabled        string `form:"enabled" json:"enabled"`
	BusinessID     string `form:"businessId" json:"businessId"`
	EnableInBG     string `form:"enableInBg" json:"enableInBg"`
}

type BatchUpdateRuleDatasourceRequest struct {
	RuleIDs      []uint `json:"ruleIds"`
	DatasourceID uint   `json:"datasourceId"`
}

type BatchUpdateRuleNotifyRulesRequest struct {
	RuleIDs       []uint        `json:"ruleIds"`
	NotifyRuleIDs JSONUintArray `json:"notifyRuleIds"`
}

type PreviewRuleQueryRequest struct {
	DatasourceID   uint   `form:"datasourceId" json:"datasourceId"`
	DatasourceType string `form:"datasourceType" json:"datasourceType"`
	ProjectCode    string `form:"projectCode" json:"projectCode"`
	StoreCode      string `form:"storeCode" json:"storeCode"`
	Query          string `form:"query" json:"query"`
	Time           int64  `form:"time" json:"time"`
	Limit          int    `form:"limit" json:"limit"`
	WindowSeconds  int64  `form:"windowSeconds" json:"windowSeconds"`
}

type PreviewRuleQueryResponse struct {
	DatasourceID   uint        `json:"datasourceId"`
	DatasourceType string      `json:"datasourceType"`
	Query          string      `json:"query"`
	ResultType     string      `json:"resultType"`
	Total          int         `json:"total"`
	Limited        bool        `json:"limited"`
	Result         interface{} `json:"result"`
	Warnings       []string    `json:"warnings"`
	ExecutedAt     int64       `json:"executedAt"`
}

type AlertBuiltinRuleTemplateQuery struct {
	PageQuery
	ResourceType   string `form:"resourceType" json:"resourceType"`
	Component      string `form:"component" json:"component"`
	Collector      string `form:"collector" json:"collector"`
	Category       string `form:"category" json:"category"`
	DatasourceType string `form:"datasourceType" json:"datasourceType"`
	Source         string `form:"source" json:"source"`
	Enabled        string `form:"enabled" json:"enabled"`
}

type AlertBuiltinRuleTemplateGroupQuery struct {
	PageQuery
	ParentID     string `form:"parentId" json:"parentId"`
	Level        string `form:"level" json:"level"`
	Component    string `form:"component" json:"component"`
	Collector    string `form:"collector" json:"collector"`
	ResourceType string `form:"resourceType" json:"resourceType"`
	Enabled      string `form:"enabled" json:"enabled"`
}

type AlertBuiltinRuleTemplateGroupOptionsQuery struct {
	Component string `form:"component" json:"component"`
	Collector string `form:"collector" json:"collector"`
	Level     string `form:"level" json:"level"`
}

type AlertBuiltinRuleTemplateGroupOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Count int64  `json:"count"`
}

type ApplyBuiltinRuleTemplateRequest struct {
	Name                    string          `json:"name"`
	Note                    string          `json:"note"`
	GroupID                 uint            `json:"groupId"`
	DatasourceID            uint            `json:"datasourceId"`
	DatasourceType          string          `json:"datasourceType"`
	DatasourceQueries       JSONArray       `json:"datasourceQueries"`
	VariableValues          JSONMap         `json:"variableValues"`
	AppendTags              JSONMap         `json:"appendTags"`
	Labels                  JSONMap         `json:"labels"`
	Annotations             JSONMap         `json:"annotations"`
	AssetScope              JSONMap         `json:"assetScope"`
	BusinessID              string          `json:"businessId"`
	EffectiveTime           JSONMap         `json:"effectiveTime"`
	NotifyRuleIDs           JSONUintArray   `json:"notifyRuleIds"`
	Enabled                 *int            `json:"enabled"`
	CronPattern             string          `json:"cronPattern"`
	EvalIntervalSeconds     int64           `json:"evalIntervalSeconds"`
	ForDurationSeconds      int64           `json:"forDurationSeconds"`
	ForDurationCount        int64           `json:"forDurationCount"`
	RecoverDurationSeconds  int64           `json:"recoverDurationSeconds"`
	EnableInBG              int             `json:"enableInBg"`
	TimeZone                string          `json:"timeZone"`
	NotifyRecovered         int             `json:"notifyRecovered"`
	NotifyRepeatStepMinutes int64           `json:"notifyRepeatStepMinutes"`
	NotifyMaxNumber         int             `json:"notifyMaxNumber"`
	PipelineConfigs         JSONArray       `json:"pipelineConfigs"`
	RunbookURL              string          `json:"runbookUrl"`
	Callbacks               JSONStringArray `json:"callbacks"`
	SelfHealConfig          JSONMap         `json:"selfHealConfig"`
	NoDataPolicy            string          `json:"noDataPolicy"`
	ErrorPolicy             string          `json:"errorPolicy"`
}

type ImportPrometheusRulesRequest struct {
	GroupID                 uint          `json:"groupId"`
	DatasourceID            uint          `json:"datasourceId"`
	DatasourceType          string        `json:"datasourceType"`
	DatasourceQueries       JSONArray     `json:"datasourceQueries"`
	Content                 string        `json:"content"`
	YAML                    string        `json:"yaml"`
	YAMLContent             string        `json:"yamlContent"`
	DefaultEnabled          *int          `json:"defaultEnabled"`
	EvalIntervalSeconds     int64         `json:"evalIntervalSeconds"`
	CronPattern             string        `json:"cronPattern"`
	BusinessID              string        `json:"businessId"`
	AppendTags              JSONMap       `json:"appendTags"`
	Labels                  JSONMap       `json:"labels"`
	AssetScope              JSONMap       `json:"assetScope"`
	EffectiveTime           JSONMap       `json:"effectiveTime"`
	NotifyRuleIDs           JSONUintArray `json:"notifyRuleIds"`
	NotifyRecovered         int           `json:"notifyRecovered"`
	NotifyRepeatStepMinutes int64         `json:"notifyRepeatStepMinutes"`
	NotifyMaxNumber         int           `json:"notifyMaxNumber"`
	NoDataPolicy            string        `json:"noDataPolicy"`
	ErrorPolicy             string        `json:"errorPolicy"`
}

type ImportPrometheusRuleSkipped struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type ImportPrometheusRulesResponse struct {
	Total        int                           `json:"total"`
	Imported     int                           `json:"imported"`
	Skipped      int                           `json:"skipped"`
	Rules        []AlertRule                   `json:"rules"`
	SkippedItems []ImportPrometheusRuleSkipped `json:"skippedItems"`
}

type PreviewPrometheusTemplateQuery struct {
	Keyword        string `form:"keyword" json:"keyword"`
	Component      string `form:"component" json:"component"`
	Collector      string `form:"collector" json:"collector"`
	Category       string `form:"category" json:"category"`
	DatasourceType string `form:"datasourceType" json:"datasourceType"`
	Limit          int    `form:"limit" json:"limit"`
}

type PrometheusTemplateImportItem struct {
	Name                       string  `json:"name"`
	Description                string  `json:"description"`
	QueryTemplate              string  `json:"queryTemplate"`
	DefaultSeverity            string  `json:"defaultSeverity"`
	DefaultForDurationSeconds  int64   `json:"defaultForDurationSeconds"`
	DefaultEvalIntervalSeconds int64   `json:"defaultEvalIntervalSeconds"`
	DefaultConditionOperator   string  `json:"defaultConditionOperator"`
	DefaultConditionValue      float64 `json:"defaultConditionValue"`
	ResourceType               string  `json:"resourceType"`
	Component                  string  `json:"component"`
	Collector                  string  `json:"collector"`
	Category                   string  `json:"category"`
	DatasourceType             string  `json:"datasourceType"`
	Source                     string  `json:"source"`
	SourceUUID                 string  `json:"sourceUuid"`
	SourceGroup                string  `json:"sourceGroup"`
	SourceService              string  `json:"sourceService"`
	SourceExporter             string  `json:"sourceExporter"`
	Labels                     JSONMap `json:"labels"`
	Annotations                JSONMap `json:"annotations"`
	RuleConfig                 JSONMap `json:"ruleConfig"`
	Exists                     bool    `json:"exists"`
}

type PreviewPrometheusTemplatesResponse struct {
	Total int                            `json:"total"`
	List  []PrometheusTemplateImportItem `json:"list"`
}

type PrometheusTemplateImportTypeOption struct {
	Label          string   `json:"label"`
	Value          string   `json:"value"`
	Keywords       []string `json:"keywords"`
	ResourceType   string   `json:"resourceType"`
	Component      string   `json:"component"`
	Collector      string   `json:"collector"`
	DatasourceType string   `json:"datasourceType"`
}

type ImportPrometheusTemplatesRequest struct {
	Rules []PrometheusTemplateImportItem `json:"rules"`
}

type ImportPrometheusTemplatesResponse struct {
	Total        int                           `json:"total"`
	Imported     int                           `json:"imported"`
	Skipped      int                           `json:"skipped"`
	Templates    []AlertBuiltinRuleTemplate    `json:"templates"`
	SkippedItems []ImportPrometheusRuleSkipped `json:"skippedItems"`
}
