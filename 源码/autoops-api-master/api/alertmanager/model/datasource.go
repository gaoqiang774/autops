package model

import "dodevops-api/common/util"

const (
	AlertDatasourceBuiltin         = "builtin"
	AlertDatasourceVictoriaMetrics = "victoriametrics"
	AlertDatasourcePrometheus      = "prometheus"
	AlertDatasourceVictoriaLogs    = "victorialogs"
)

type AlertDatasource struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Name           string     `gorm:"size:150;not null;index" json:"name"`
	DatasourceType string     `gorm:"size:64;not null;index" json:"datasourceType"`
	URL            string     `gorm:"size:500" json:"url"`
	AuthType       string     `gorm:"size:32;not null;default:'none'" json:"authType"`
	Username       string     `gorm:"size:128" json:"username"`
	Password       string     `gorm:"size:512" json:"password,omitempty"`
	Token          string     `gorm:"type:text" json:"token,omitempty"`
	Config         JSONMap    `gorm:"type:json" json:"config"`
	Labels         JSONMap    `gorm:"type:json" json:"labels"`
	Description    string     `gorm:"size:500" json:"description"`
	Builtin        int        `gorm:"type:tinyint;not null;default:0;index" json:"builtin"`
	Default        int        `gorm:"column:is_default;type:tinyint;not null;default:0;index" json:"default"`
	Enabled        int        `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	CreatedBy      string     `gorm:"size:64" json:"createdBy"`
	UpdatedBy      string     `gorm:"size:64" json:"updatedBy"`
	CreatedAt      util.HTime `json:"createdAt"`
	UpdatedAt      util.HTime `json:"updatedAt"`
}

func (AlertDatasource) TableName() string {
	return "alert_datasources"
}

type AlertDatasourceQuery struct {
	PageQuery
	DatasourceType string `form:"datasourceType" json:"datasourceType"`
	Enabled        string `form:"enabled" json:"enabled"`
	Builtin        string `form:"builtin" json:"builtin"`
}

type AlertDatasourceTypeOption struct {
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	QueryLang   string   `json:"queryLang"`
	AuthTypes   []string `json:"authTypes"`
	Builtin     int      `json:"builtin"`
	Enabled     int      `json:"enabled"`
}

type AlertMetricsQueryRequest struct {
	DatasourceID   uint   `form:"datasourceId" json:"datasourceId"`
	DatasourceType string `form:"datasourceType" json:"datasourceType"`
	Query          string `form:"query" json:"query"`
	Time           int64  `form:"time" json:"time"`
}

type AlertMetricsQueryRangeRequest struct {
	DatasourceID   uint   `form:"datasourceId" json:"datasourceId"`
	DatasourceType string `form:"datasourceType" json:"datasourceType"`
	Query          string `form:"query" json:"query"`
	Start          int64  `form:"start" json:"start"`
	End            int64  `form:"end" json:"end"`
	Step           string `form:"step" json:"step"`
}

type AlertMetricTarget struct {
	Instance string `json:"instance"`
	IP       string `json:"ip"`
	Job      string `json:"job"`
	Hostname string `json:"hostname,omitempty"`
}
