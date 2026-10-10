package model

import "dodevops-api/common/util"

const (
	SQLTicketTypeDML   = "dml"
	SQLTicketTypeDDL   = "ddl"
	SQLTicketTypeQuery = "query"
)

const (
	SQLTicketStatusDraft         = "draft"
	SQLTicketStatusPendingReview = "pending_review"
	SQLTicketStatusRejected      = "rejected"
	SQLTicketStatusApproved      = "approved"
	SQLTicketStatusRunning       = "running"
	SQLTicketStatusSucceeded     = "succeeded"
	SQLTicketStatusFailed        = "failed"
	SQLTicketStatusCanceled      = "canceled"
)

const (
	SQLRiskLow      = "low"
	SQLRiskMedium   = "medium"
	SQLRiskHigh     = "high"
	SQLRiskCritical = "critical"
)

const (
	SQLReviewLevelInfo  = "info"
	SQLReviewLevelWarn  = "warn"
	SQLReviewLevelError = "error"
)

const (
	SQLReviewStatusPass  = "pass"
	SQLReviewStatusWarn  = "warn"
	SQLReviewStatusError = "error"
)

const (
	SQLApprovalActionApprove = "approve"
	SQLApprovalActionReject  = "reject"
	SQLApprovalActionCancel  = "cancel"
	SQLApprovalActionComment = "comment"
)

const (
	SQLExecutionStatusPending   = "pending"
	SQLExecutionStatusRunning   = "running"
	SQLExecutionStatusSucceeded = "succeeded"
	SQLExecutionStatusFailed    = "failed"
)

type SQLTicket struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	TicketNo           string     `gorm:"size:64;not null;uniqueIndex" json:"ticketNo"`
	Title              string     `gorm:"size:256;not null;index" json:"title"`
	TicketType         string     `gorm:"size:16;not null;index" json:"ticketType"`
	AssetID            uint       `gorm:"not null;index" json:"assetId"`
	AssetName          string     `gorm:"-" json:"assetName"`
	AssetTypeName      string     `gorm:"-" json:"assetTypeName"`
	AssetInstance      string     `gorm:"-" json:"assetInstance"`
	LastError          string     `gorm:"-" json:"lastError"`
	UseGhOst           bool       `gorm:"not null;default:false" json:"useGhOst"`
	GhOstScript        string     `gorm:"type:text" json:"ghOstScript"`
	Engine             int        `gorm:"type:integer;index" json:"engine"`
	DatabaseName       string     `gorm:"size:128;index" json:"databaseName"`
	SchemaName         string     `gorm:"size:128;index" json:"schemaName"`
	Environment        string     `gorm:"size:64;index" json:"environment"`
	SQLContent         string     `gorm:"type:text;not null" json:"sqlContent"`
	SQLFingerprint     string     `gorm:"size:128;index" json:"sqlFingerprint"`
	Status             string     `gorm:"size:32;not null;index" json:"status"`
	WorkflowInstanceID uint       `gorm:"index" json:"workflowInstanceId"`
	RiskLevel          string     `gorm:"size:32;not null;index" json:"riskLevel"`
	BackupRequired     bool       `gorm:"not null;default:false" json:"backupRequired"`
	RollbackRequired   bool       `gorm:"not null;default:false" json:"rollbackRequired"`
	ReviewPassed       bool       `gorm:"not null;default:false;index" json:"reviewPassed"`
	Remark             string     `gorm:"type:text" json:"remark"`
	SubmitterID        uint       `gorm:"index" json:"submitterId"`
	SubmitterName      string     `gorm:"size:128;index" json:"submitterName"`
	SubmittedAt        util.HTime `json:"submittedAt"`
	ApproverID         uint       `gorm:"index" json:"approverId"`
	ApproverName       string     `gorm:"size:128;index" json:"approverName"`
	ReviewedAt         util.HTime `json:"reviewedAt"`
	ReviewComment      string     `gorm:"type:text" json:"reviewComment"`
	ExecutorID         uint       `gorm:"index" json:"executorId"`
	ExecutorName       string     `gorm:"size:128;index" json:"executorName"`
	ExecutedAt         util.HTime `json:"executedAt"`
	ExecuteComment     string     `gorm:"type:text" json:"executeComment"`
	ScheduledAt        util.HTime `json:"scheduledAt"`
	CreatedAt          util.HTime `gorm:"index" json:"createdAt"`
	UpdatedAt          util.HTime `json:"updatedAt"`
}

func (SQLTicket) TableName() string {
	return "db_sql_ticket"
}

type SQLStatement struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	TicketID       uint       `gorm:"not null;index" json:"ticketId"`
	StatementIndex int        `gorm:"not null" json:"statementIndex"`
	SQLText        string     `gorm:"type:text;not null" json:"sqlText"`
	StatementType  string     `gorm:"size:64;index" json:"statementType"`
	ObjectName     string     `gorm:"size:256;index" json:"objectName"`
	ObjectType     string     `gorm:"size:64;index" json:"objectType"`
	DatabaseName   string     `gorm:"size:128;index" json:"databaseName"`
	SchemaName     string     `gorm:"size:128;index" json:"schemaName"`
	RiskLevel      string     `gorm:"size:32;index" json:"riskLevel"`
	EstimatedRows  int64      `json:"estimatedRows"`
	ReviewStatus   string     `gorm:"size:32;index" json:"reviewStatus"`
	Message        string     `gorm:"type:text" json:"message"`
	CreatedAt      util.HTime `gorm:"index" json:"createdAt"`
}

func (SQLStatement) TableName() string {
	return "db_sql_statement"
}

type SQLReviewResult struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	TicketID    uint       `gorm:"not null;index" json:"ticketId"`
	StatementID uint       `gorm:"index" json:"statementId"`
	RuleCode    string     `gorm:"size:128;not null;index" json:"ruleCode"`
	RuleName    string     `gorm:"size:128" json:"ruleName"`
	Level       string     `gorm:"size:16;not null;index" json:"level"`
	Message     string     `gorm:"type:text" json:"message"`
	Suggestion  string     `gorm:"type:text" json:"suggestion"`
	StartLine   int        `json:"startLine"`
	EndLine     int        `json:"endLine"`
	CreatedAt   util.HTime `gorm:"index" json:"createdAt"`
}

func (SQLReviewResult) TableName() string {
	return "db_sql_review_result"
}

type SQLApprovalRecord struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	TicketID     uint       `gorm:"not null;index" json:"ticketId"`
	OperatorID   uint       `gorm:"index" json:"operatorId"`
	OperatorName string     `gorm:"size:128;index" json:"operatorName"`
	Action       string     `gorm:"size:32;not null;index" json:"action"`
	Comment      string     `gorm:"type:text" json:"comment"`
	CreatedAt    util.HTime `gorm:"index" json:"createdAt"`
}

func (SQLApprovalRecord) TableName() string {
	return "db_sql_approval_record"
}

type SQLExecutionRecord struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	TicketID      uint       `gorm:"not null;index" json:"ticketId"`
	StatementID   uint       `gorm:"index" json:"statementId"`
	SQLText       string     `gorm:"type:text;not null" json:"sqlText"`
	Status        string     `gorm:"size:32;not null;index" json:"status"`
	RowsAffected  int64      `json:"rowsAffected"`
	DurationMS    int64      `json:"durationMs"`
	ErrorMessage  string     `gorm:"type:text" json:"errorMessage"`
	ExecLog       string     `gorm:"type:longtext" json:"execLog"`
	ResultColumns string     `gorm:"type:text" json:"resultColumns"`
	ResultRows    string     `gorm:"type:longtext" json:"resultRows"`
	StartedAt     util.HTime `json:"startedAt"`
	FinishedAt    util.HTime `json:"finishedAt"`
	ExecutorID    uint       `gorm:"index" json:"executorId"`
	ExecutorName  string     `gorm:"size:128;index" json:"executorName"`
	CreatedAt     util.HTime `gorm:"index" json:"createdAt"`
}

func (SQLExecutionRecord) TableName() string {
	return "db_sql_execution_record"
}

type SQLRollbackRecord struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	TicketID         uint       `gorm:"not null;index" json:"ticketId"`
	StatementID      uint       `gorm:"index" json:"statementId"`
	StatementType    string     `gorm:"size:64;index" json:"statementType"`
	OriginalDatabase string     `gorm:"size:128;index" json:"originalDatabase"`
	OriginalTable    string     `gorm:"size:128;index" json:"originalTable"`
	BackupDatabase   string     `gorm:"size:128;index" json:"backupDatabase"`
	BackupTable      string     `gorm:"size:128;index" json:"backupTable"`
	BackupRows       int64      `json:"backupRows"`
	RollbackSQL      string     `gorm:"type:text" json:"rollbackSql"`
	RollbackType     string     `gorm:"size:32;not null;default:manual" json:"rollbackType"`
	BinlogStartFile  string     `gorm:"size:128" json:"binlogStartFile"`
	BinlogStartPos   uint64     `json:"binlogStartPos"`
	BinlogEndFile    string     `gorm:"size:128" json:"binlogEndFile"`
	BinlogEndPos     uint64     `json:"binlogEndPos"`
	FlashbackTool    string     `gorm:"size:128" json:"flashbackTool"`
	GeneratedAt      util.HTime `json:"generatedAt"`
	Available        bool       `gorm:"not null;default:false" json:"available"`
	Message          string     `gorm:"type:text" json:"message"`
	CreatedAt        util.HTime `gorm:"index" json:"createdAt"`
}

func (SQLRollbackRecord) TableName() string {
	return "db_sql_rollback_record"
}

type SQLRuleGroup struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"size:128;not null;index" json:"name"`
	Description string     `gorm:"type:text" json:"description"`
	Engine      int        `gorm:"type:integer;index" json:"engine"`
	Environment string     `gorm:"size:64;index" json:"environment"`
	Enabled     bool       `gorm:"not null;default:true;index" json:"enabled"`
	RulesJSON   string     `gorm:"type:text" json:"rulesJson"`
	CreatedAt   util.HTime `gorm:"index" json:"createdAt"`
	UpdatedAt   util.HTime `json:"updatedAt"`
}

func (SQLRuleGroup) TableName() string {
	return "db_sql_rule_group"
}

type AssetReviewPolicy struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	AssetID             uint       `gorm:"not null;uniqueIndex" json:"assetId"`
	Environment         string     `gorm:"size:64;index" json:"environment"`
	DMLRuleGroupID      uint       `json:"dmlRuleGroupId"`
	DDLRuleGroupID      uint       `json:"ddlRuleGroupId"`
	DefaultApproverID   uint       `json:"defaultApproverId"`
	DefaultApproverName string     `gorm:"size:128" json:"defaultApproverName"`
	DefaultExecutorID   uint       `json:"defaultExecutorId"`
	DefaultExecutorName string     `gorm:"size:128" json:"defaultExecutorName"`
	RequireBackup       bool       `gorm:"not null;default:false" json:"requireBackup"`
	RequireRollback     bool       `gorm:"not null;default:false" json:"requireRollback"`
	Enabled             bool       `gorm:"not null;default:true;index" json:"enabled"`
	CreatedAt           util.HTime `gorm:"index" json:"createdAt"`
	UpdatedAt           util.HTime `json:"updatedAt"`
}

func (AssetReviewPolicy) TableName() string {
	return "db_asset_review_policy"
}

type SQLReviewCheckRequest struct {
	AssetID        uint   `json:"assetId"`
	Database       string `json:"database"`
	Schema         string `json:"schema"`
	SQL            string `json:"sql"`
	BackupRequired bool   `json:"backupRequired"`
	UseGhOst       bool   `json:"useGhOst"`
}

type SQLReviewSubmitRequest struct {
	Title          string `json:"title"`
	AssetID        uint   `json:"assetId"`
	Database       string `json:"database"`
	Schema         string `json:"schema"`
	SQL            string `json:"sql"`
	BackupRequired bool   `json:"backupRequired"`
	UseGhOst       bool   `json:"useGhOst"`
	RollbackSQL    string `json:"rollbackSql"`
	ApproverID     uint   `json:"approverId"`
	WorkflowID     uint   `json:"workflowId"`
	ApproverName   string `json:"approverName"`
	ExecutorID     uint   `json:"executorId"`
	ExecutorName   string `json:"executorName"`
	Remark         string `json:"remark"`
}

type SQLTicketQuery struct {
	Page          int    `form:"page"`
	PageSize      int    `form:"pageSize"`
	Keyword       string `form:"keyword"`
	TicketType    string `form:"ticketType"`
	Status        string `form:"status"`
	AssetID       uint   `form:"assetId"`
	Database      string `form:"database"`
	Environment   string `form:"environment"`
	SubmitterName string `form:"submitterName"`
	ApproverName  string `form:"approverName"`
	ExecutorName  string `form:"executorName"`
	BeginTime     string `form:"beginTime"`
	EndTime       string `form:"endTime"`
}

type SQLTicketActionRequest struct {
	Comment   string     `json:"comment"`
	Confirmed SQLConfirm `json:"confirmed"`
}

type SQLRollbackTicketRequest struct {
	Title            string `json:"title"`
	RollbackRecordID uint   `json:"rollbackRecordId"`
	ApproverID       uint   `json:"approverId"`
	ExecutorID       uint   `json:"executorId"`
	Remark           string `json:"remark"`
}

type SQLBackupCleanupRequest struct {
	Confirmed bool `json:"confirmed"`
}

type SQLBackupCleanupResult struct {
	DeletedTables []string `json:"deletedTables"`
	SkippedTables []string `json:"skippedTables"`
	DeletedCount  int      `json:"deletedCount"`
}

type SQLFlashbackGenerateRequest struct {
	RollbackRecordID uint `json:"rollbackRecordId"`
	Confirmed        bool `json:"confirmed"`
}

type DBToolStatus struct {
	ToolName            string `json:"toolName"`
	DatabaseType        int    `json:"databaseType"`
	DatabaseTypeName    string `json:"databaseTypeName"`
	Capability          string `json:"capability"`
	Status              string `json:"status"`
	StatusText          string `json:"statusText"`
	InstallPath         string `json:"installPath"`
	ResolvedPath        string `json:"resolvedPath"`
	EnvPath             string `json:"envPath"`
	UsageScenario       string `json:"usageScenario"`
	CheckButtonText     string `json:"checkButtonText"`
	InstallCommand      string `json:"installCommand"`
	InstallInstruction  string `json:"installInstruction"`
	URL                 string `json:"url"`
	DownloadURL         string `json:"downloadUrl"`
	CheckCommand        string `json:"checkCommand"`
	LastCheckMessage    string `json:"lastCheckMessage"`
	CanGenerateRollback bool   `json:"canGenerateRollback"`
}

type DBToolboxStatus struct {
	DatabaseType     int            `json:"databaseType"`
	DatabaseTypeName string         `json:"databaseTypeName"`
	Tools            []DBToolStatus `json:"tools"`
}

type SQLReviewCheckResult struct {
	TicketType    string            `json:"ticketType"`
	ReviewPassed  bool              `json:"reviewPassed"`
	RiskLevel     string            `json:"riskLevel"`
	Statements    []SQLStatement    `json:"statements"`
	ReviewResults []SQLReviewResult `json:"reviewResults"`
	EstimatedRows int64             `json:"estimatedRows"`
	RollbackSQL   string            `json:"rollbackSql"`
}

type SQLTicketDetail struct {
	Ticket           SQLTicket            `json:"ticket"`
	Statements       []SQLStatement       `json:"statements"`
	ReviewResults    []SQLReviewResult    `json:"reviewResults"`
	ApprovalRecords  []SQLApprovalRecord  `json:"approvalRecords"`
	ExecutionRecords []SQLExecutionRecord `json:"executionRecords"`
	RollbackRecords  []SQLRollbackRecord  `json:"rollbackRecords"`
	FlowEnded        bool                 `json:"flowEnded"`
	FlowEndReason    string               `json:"flowEndReason"`
	FlowSteps        []SQLTicketFlowStep  `json:"flowSteps"`
}

type SQLTicketFlowStep struct {
	Key          string     `json:"key"`
	Name         string     `json:"name"`
	Status       string     `json:"status"`
	OperatorID   uint       `json:"operatorId"`
	OperatorName string     `json:"operatorName"`
	Comment      string     `json:"comment"`
	FinishedAt   util.HTime `json:"finishedAt"`
}

type SQLTicketQueryResult struct {
	TicketID          uint                     `json:"ticketId"`
	ExecutionRecordID uint                     `json:"executionRecordId"`
	Columns           []ColumnInfo             `json:"columns"`
	Rows              []map[string]interface{} `json:"rows"`
	Total             int                      `json:"total"`
	DurationMS        int64                    `json:"durationMs"`
	SQL               string                   `json:"sql"`
}
