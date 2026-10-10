package model

import "dodevops-api/common/util"

type OperationAudit struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	AssetID      uint       `gorm:"index" json:"assetId"`
	Engine       int        `gorm:"type:integer;index" json:"engine"`
	DatabaseName string     `gorm:"size:128;index" json:"databaseName"`
	SchemaName   string     `gorm:"size:128;index" json:"schemaName"`
	ObjectName   string     `gorm:"size:256;index" json:"objectName"`
	ObjectType   string     `gorm:"size:64;index" json:"objectType"`
	Action       string     `gorm:"size:64;index" json:"action"`
	RequestBody  string     `gorm:"type:text" json:"requestBody"`
	WhereJSON    string     `gorm:"type:text" json:"whereJson"`
	BeforeJSON   string     `gorm:"type:text" json:"beforeJson"`
	AfterJSON    string     `gorm:"type:text" json:"afterJson"`
	RowsAffected int64      `json:"rowsAffected"`
	Success      bool       `gorm:"index" json:"success"`
	ErrorMessage string     `gorm:"type:text" json:"errorMessage"`
	DurationMS   int64      `json:"durationMs"`
	OperatorID   uint       `gorm:"index" json:"operatorId"`
	OperatorName string     `gorm:"size:128;index" json:"operatorName"`
	ClientIP     string     `gorm:"size:64" json:"clientIp"`
	CreatedAt    util.HTime `gorm:"index" json:"createdAt"`
}

func (OperationAudit) TableName() string {
	return "db_operation_audit"
}

type AuditQuery struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"pageSize"`
	AssetID      uint   `form:"assetId"`
	Engine       int    `form:"engine"`
	DatabaseName string `form:"databaseName"`
	ObjectName   string `form:"objectName"`
	Action       string `form:"action"`
	Success      *bool  `form:"success"`
	OperatorName string `form:"operatorName"`
	BeginTime    string `form:"beginTime"`
	EndTime      string `form:"endTime"`
}
