package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"dodevops-api/common/util"
)

const (
	WorkflowStatusRunning  = "running"
	WorkflowStatusApproved = "approved"
	WorkflowStatusRejected = "rejected"
	WorkflowStatusCanceled = "canceled"
	ApprovalTaskPending    = "pending"
	ApprovalTaskApproved   = "approved"
	ApprovalTaskRejected   = "rejected"
	ApprovalTaskCanceled   = "canceled"
	ApproverTypeUser       = "user"
	ApproverTypeRole       = "role"
	ApprovalModeAny        = "any"
	ApprovalModeAll        = "all"
	BusinessTypeDaily      = "daily"
	BusinessTypeRelease    = "release"
	BusinessTypeSQLReview  = "sql_review"
	DailyStatusSubmitted   = "submitted"
	DailyStatusPending     = "pending_approval"
	DailyStatusApproved    = "approved"
	DailyStatusRejected    = "rejected"
	DailyStatusAssigned    = "assigned"
	DailyStatusProcessing  = "processing"
	DailyStatusResolved    = "resolved"
	DailyStatusConfirmed   = "confirmed"
	DailyStatusClosed      = "closed"
	DailyStatusCanceled    = "canceled"
	ReleaseStatusPending   = "pending_approval"
	ReleaseStatusApproved  = "approved"
	ReleaseStatusRejected  = "rejected"
	ReleaseStatusRunning   = "running"
	ReleaseStatusSucceeded = "succeeded"
	ReleaseStatusFailed    = "failed"
	ReleaseStatusCanceled  = "canceled"
	ReleaseItemPending     = "pending"
	ReleaseItemQueued      = "queued"
	ReleaseItemRunning     = "running"
	ReleaseItemSucceeded   = "succeeded"
	ReleaseItemFailed      = "failed"
	ReleaseItemCanceled    = "canceled"
)

type JSONMap map[string]interface{}
type JSONUintArray []uint
type JSONStringArray []string
type JSONWorkflowNodes []WorkflowNode

func (j JSONMap) Value() (driver.Value, error)            { return json.Marshal(j) }
func (j JSONUintArray) Value() (driver.Value, error)      { return json.Marshal(j) }
func (j JSONStringArray) Value() (driver.Value, error)    { return json.Marshal(j) }
func (j JSONWorkflowNodes) Value() (driver.Value, error)  { return json.Marshal(j) }
func (j *JSONMap) Scan(value interface{}) error           { return scanJSON(value, j) }
func (j *JSONUintArray) Scan(value interface{}) error     { return scanJSON(value, j) }
func (j *JSONStringArray) Scan(value interface{}) error   { return scanJSON(value, j) }
func (j *JSONWorkflowNodes) Scan(value interface{}) error { return scanJSON(value, j) }

func scanJSON(value interface{}, target interface{}) error {
	if value == nil {
		return nil
	}
	var data []byte
	switch typed := value.(type) {
	case []byte:
		data = typed
	case string:
		data = []byte(typed)
	default:
		return fmt.Errorf("unsupported JSON value %T", value)
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

type WorkflowDefinition struct {
	ID                  uint              `gorm:"primaryKey" json:"id"`
	Name                string            `gorm:"size:128;not null;index" json:"name"`
	Code                string            `gorm:"size:64;not null;uniqueIndex" json:"-"`
	BusinessTypes       JSONStringArray   `gorm:"type:json" json:"businessTypes"`
	Environments        JSONStringArray   `gorm:"type:json" json:"environments"`
	Description         string            `gorm:"size:500" json:"description"`
	Enabled             int               `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	Version             int               `gorm:"not null;default:1" json:"version"`
	PreventSelfApproval int               `gorm:"type:tinyint;not null;default:1" json:"preventSelfApproval"`
	MaintainerID        uint              `gorm:"index" json:"maintainerId"`
	MaintainerName      string            `gorm:"size:128" json:"maintainerName"`
	CreatedAt           util.HTime        `gorm:"index" json:"createdAt"`
	UpdatedAt           util.HTime        `json:"updatedAt"`
	Nodes               JSONWorkflowNodes `gorm:"type:json" json:"nodes"`
}

func (WorkflowDefinition) TableName() string { return "ticket_workflow_definitions" }

type WorkflowNode struct {
	ID           uint          `json:"id"`
	DefinitionID uint          `json:"definitionId,omitempty"`
	NodeOrder    int           `json:"nodeOrder"`
	Name         string        `json:"name"`
	ApproverType string        `gorm:"size:16;not null" json:"approverType"`
	ApproverIDs  JSONUintArray `json:"approverIds"`
	ApprovalMode string        `json:"approvalMode"`
	Description  string        `json:"description"`
}

type WorkflowInstance struct {
	ID                uint        `gorm:"primaryKey" json:"id"`
	DefinitionID      uint        `gorm:"not null;index" json:"definitionId"`
	DefinitionName    string      `gorm:"size:128;not null" json:"definitionName"`
	DefinitionVersion int         `gorm:"not null" json:"definitionVersion"`
	BusinessType      string      `gorm:"size:32;not null;uniqueIndex:idx_ticket_flow_business" json:"businessType"`
	BusinessID        uint        `gorm:"not null;uniqueIndex:idx_ticket_flow_business" json:"businessId"`
	BusinessNo        string      `gorm:"size:64;index" json:"businessNo"`
	ApplicantID       uint        `gorm:"not null;index" json:"applicantId"`
	ApplicantName     string      `gorm:"size:128;not null" json:"applicantName"`
	Status            string      `gorm:"size:32;not null;index" json:"status"`
	CurrentNodeOrder  int         `gorm:"not null;default:0" json:"currentNodeOrder"`
	CurrentNodeName   string      `gorm:"size:128" json:"currentNodeName"`
	NodeSnapshot      string      `gorm:"type:longtext" json:"nodeSnapshot"`
	StartedAt         util.HTime  `gorm:"index" json:"startedAt"`
	FinishedAt        *util.HTime `json:"finishedAt"`
	UpdatedAt         util.HTime  `json:"updatedAt"`
}

func (WorkflowInstance) TableName() string { return "ticket_workflow_instances" }

type ApprovalTask struct {
	ID             uint        `gorm:"primaryKey" json:"id"`
	InstanceID     uint        `gorm:"not null;index:idx_ticket_task_instance_status" json:"instanceId"`
	NodeID         uint        `gorm:"not null;index" json:"nodeId"`
	NodeOrder      int         `gorm:"not null" json:"nodeOrder"`
	NodeName       string      `gorm:"size:128;not null" json:"nodeName"`
	AssigneeID     uint        `gorm:"not null;index:idx_ticket_task_assignee_status" json:"assigneeId"`
	AssigneeName   string      `gorm:"size:128;not null" json:"assigneeName"`
	Status         string      `gorm:"size:32;not null;index:idx_ticket_task_instance_status;index:idx_ticket_task_assignee_status" json:"status"`
	Comment        string      `gorm:"size:1000" json:"comment"`
	OperatorID     uint        `gorm:"index" json:"operatorId"`
	OperatorName   string      `gorm:"size:128" json:"operatorName"`
	ActedAt        *util.HTime `json:"actedAt"`
	CreatedAt      util.HTime  `gorm:"index" json:"createdAt"`
	BusinessType   string      `gorm:"-" json:"businessType"`
	BusinessID     uint        `gorm:"-" json:"businessId"`
	BusinessNo     string      `gorm:"-" json:"businessNo"`
	DefinitionName string      `gorm:"-" json:"definitionName"`
}

func (ApprovalTask) TableName() string { return "ticket_approval_tasks" }

type Ticket struct {
	ID                 uint            `gorm:"primaryKey" json:"id"`
	TicketNo           string          `gorm:"size:64;not null;uniqueIndex" json:"ticketNo"`
	Title              string          `gorm:"size:256;not null;index" json:"title"`
	BusinessType       string          `gorm:"size:32;not null;index:idx_ticket_type_status" json:"businessType"`
	Category           string          `gorm:"size:32;not null;index" json:"category"`
	Environment        string          `gorm:"size:32;not null;index" json:"environment"`
	Priority           string          `gorm:"size:16;not null;index" json:"priority"`
	ImpactScope        string          `gorm:"size:1000" json:"impactScope"`
	Description        string          `gorm:"type:text" json:"description"`
	FormData           JSONMap         `gorm:"type:json" json:"formData"`
	Attachments        JSONStringArray `gorm:"type:json" json:"attachments"`
	Status             string          `gorm:"size:32;not null;index" json:"status"`
	WorkflowInstanceID uint            `gorm:"index" json:"workflowInstanceId"`
	ApplicantID        uint            `gorm:"not null;index" json:"applicantId"`
	ApplicantName      string          `gorm:"size:128;not null;index" json:"applicantName"`
	HandlerID          uint            `gorm:"index" json:"handlerId"`
	HandlerName        string          `gorm:"size:128;index" json:"handlerName"`
	Strategy           string          `gorm:"size:16" json:"strategy"`
	RollbackPlan       string          `gorm:"type:text" json:"rollbackPlan"`
	ExecutorID         uint            `gorm:"index" json:"executorId"`
	ExecutorName       string          `gorm:"size:128" json:"executorName"`
	ExpectedAt         *util.HTime     `json:"expectedAt"`
	ResolvedAt         *util.HTime     `json:"resolvedAt"`
	StartedAt          *util.HTime     `json:"startedAt"`
	FinishedAt         *util.HTime     `json:"finishedAt"`
	ClosedAt           *util.HTime     `json:"closedAt"`
	CreatedAt          util.HTime      `gorm:"index" json:"createdAt"`
	UpdatedAt          util.HTime      `json:"updatedAt"`
	Items              []ReleaseItem   `gorm:"foreignKey:ReleaseID" json:"items,omitempty"`
}

func (Ticket) TableName() string { return "ticket_orders" }

type DailyTicket = Ticket
type ReleaseTicket = Ticket

type TicketAction struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	TicketID     uint       `gorm:"not null;index" json:"ticketId"`
	OperatorID   uint       `gorm:"not null;index" json:"operatorId"`
	OperatorName string     `gorm:"size:128;not null" json:"operatorName"`
	Action       string     `gorm:"size:32;not null;index" json:"action"`
	FromStatus   string     `gorm:"size:32" json:"fromStatus"`
	ToStatus     string     `gorm:"size:32" json:"toStatus"`
	Comment      string     `gorm:"size:2000" json:"comment"`
	CreatedAt    util.HTime `gorm:"index" json:"createdAt"`
}

func (TicketAction) TableName() string { return "ticket_actions" }

type DailyAction = TicketAction

type ReleaseApplication struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	Name              string     `gorm:"size:128;not null;index" json:"name"`
	Code              string     `gorm:"size:64;not null;uniqueIndex" json:"code"`
	SystemName        string     `gorm:"size:128;index" json:"systemName"`
	Environment       string     `gorm:"size:32;not null;index" json:"environment"`
	JenkinsAccountID  uint       `gorm:"not null;index" json:"jenkinsAccountId"`
	JenkinsJobName    string     `gorm:"size:256;not null" json:"jenkinsJobName"`
	DefaultParameters JSONMap    `gorm:"type:json" json:"defaultParameters"`
	Description       string     `gorm:"size:500" json:"description"`
	Enabled           int        `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	CreatedAt         util.HTime `json:"createdAt"`
	UpdatedAt         util.HTime `json:"updatedAt"`
}

func (ReleaseApplication) TableName() string { return "ticket_release_applications" }

type ReleaseItem struct {
	ID               uint        `gorm:"primaryKey" json:"id"`
	ReleaseID        uint        `gorm:"not null;index" json:"releaseId"`
	ApplicationID    uint        `gorm:"not null;index" json:"applicationId"`
	ApplicationName  string      `gorm:"size:128;not null" json:"applicationName"`
	Version          string      `gorm:"size:128;not null" json:"version"`
	Target           string      `gorm:"size:256" json:"target"`
	JenkinsAccountID uint        `gorm:"not null;index" json:"jenkinsAccountId"`
	JenkinsJobName   string      `gorm:"size:256;not null" json:"jenkinsJobName"`
	Parameters       JSONMap     `gorm:"type:json" json:"parameters"`
	ExecuteOrder     int         `gorm:"not null;default:0" json:"executeOrder"`
	Status           string      `gorm:"size:32;not null;index" json:"status"`
	QueueID          int64       `gorm:"index" json:"queueId"`
	BuildNumber      int64       `gorm:"index" json:"buildNumber"`
	BuildURL         string      `gorm:"size:500" json:"buildUrl"`
	ErrorMessage     string      `gorm:"type:text" json:"errorMessage"`
	StartedAt        *util.HTime `json:"startedAt"`
	FinishedAt       *util.HTime `json:"finishedAt"`
	CreatedAt        util.HTime  `json:"createdAt"`
	UpdatedAt        util.HTime  `json:"updatedAt"`
}

func (ReleaseItem) TableName() string { return "ticket_release_items" }

type PageQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Keyword  string `form:"keyword"`
}

func NormalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
