package model

import "dodevops-api/common/util"

const (
	EventStatusPending    = "pending"
	EventStatusFiring     = "firing"
	EventStatusResolved   = "resolved"
	EventStatusSilenced   = "silenced"
	EventStatusSuppressed = "suppressed"

	EventDisplayStatusActive   = "active"
	EventDisplayStatusPending  = "pending"
	EventDisplayStatusSilenced = "silenced"
	EventDisplayStatusResolved = "resolved"

	AlertBusinessRouteOperations = "operations"
)

type AlertActiveEvent struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Fingerprint      string     `gorm:"size:128;not null;uniqueIndex" json:"fingerprint"`
	RuleID           uint       `gorm:"column:rule_id;index" json:"ruleId"`
	RuleName         string     `gorm:"size:150;index" json:"ruleName"`
	DatasourceID     uint       `gorm:"column:datasource_id;index" json:"datasourceId"`
	DatasourceType   string     `gorm:"size:64;index" json:"datasourceType"`
	Severity         string     `gorm:"size:32;index" json:"severity"`
	Status           string     `gorm:"size:32;index" json:"status"`
	Labels           JSONMap    `gorm:"type:json" json:"labels"`
	Annotations      JSONMap    `gorm:"type:json" json:"annotations"`
	Value            string     `gorm:"size:128" json:"value"`
	TriggerQuery     string     `gorm:"type:text" json:"triggerQuery"`
	TriggerCondition string     `gorm:"type:text" json:"triggerCondition"`
	EventDetail      string     `gorm:"type:text" json:"eventDetail"`
	AssetID          uint       `gorm:"column:asset_id;index" json:"assetId"`
	BusinessID       string     `gorm:"size:128;index" json:"businessId"`
	FirstTriggeredAt util.HTime `json:"firstTriggeredAt"`
	LastEvaluatedAt  util.HTime `json:"lastEvaluatedAt"`
	LastNotifiedAt   util.HTime `json:"lastNotifiedAt"`
	RecoveredAt      util.HTime `json:"recoveredAt"`
	NotifyCount      int        `gorm:"type:integer;default:0" json:"notifyCount"`
	Suppressed       int        `gorm:"type:tinyint;default:0;index" json:"suppressed"`
	Silenced         int        `gorm:"type:tinyint;default:0;index" json:"silenced"`
	ClaimUser        string     `gorm:"size:64;index" json:"claimUser"`
	ClaimedAt        util.HTime `json:"claimedAt"`
	CreatedAt        util.HTime `json:"createdAt"`
	UpdatedAt        util.HTime `json:"updatedAt"`
}

func (AlertActiveEvent) TableName() string {
	return "alert_active_events"
}

type AlertHistoryEvent struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Fingerprint      string     `gorm:"size:128;not null;index" json:"fingerprint"`
	ActiveEventID    uint64     `gorm:"column:active_event_id;index" json:"activeEventId"`
	RuleID           uint       `gorm:"column:rule_id;index" json:"ruleId"`
	RuleName         string     `gorm:"size:150;index" json:"ruleName"`
	DatasourceID     uint       `gorm:"column:datasource_id;index" json:"datasourceId"`
	DatasourceType   string     `gorm:"size:64;index" json:"datasourceType"`
	EventType        string     `gorm:"size:32;not null;index" json:"eventType"`
	Severity         string     `gorm:"size:32;index" json:"severity"`
	Labels           JSONMap    `gorm:"type:json" json:"labels"`
	Annotations      JSONMap    `gorm:"type:json" json:"annotations"`
	Value            string     `gorm:"size:128" json:"value"`
	TriggerQuery     string     `gorm:"type:text" json:"triggerQuery"`
	TriggerCondition string     `gorm:"type:text" json:"triggerCondition"`
	EventDetail      string     `gorm:"type:text" json:"eventDetail"`
	AssetID          uint       `gorm:"column:asset_id;index" json:"assetId"`
	BusinessID       string     `gorm:"size:128;index" json:"businessId"`
	FirstTriggeredAt util.HTime `json:"firstTriggeredAt"`
	RecoveredAt      util.HTime `json:"recoveredAt"`
	ClaimUser        string     `gorm:"size:64;index" json:"claimUser"`
	OccurredAt       util.HTime `json:"occurredAt"`
	CreatedAt        util.HTime `json:"createdAt"`
}

func (AlertHistoryEvent) TableName() string {
	return "alert_history_events"
}

type AlertEventTimeline struct {
	ID        uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	EventID   uint64     `gorm:"column:event_id;index" json:"eventId"`
	Action    string     `gorm:"size:64;not null;index" json:"action"`
	Message   string     `gorm:"size:500" json:"message"`
	Operator  string     `gorm:"size:64" json:"operator"`
	Metadata  JSONMap    `gorm:"type:json" json:"metadata"`
	CreatedAt util.HTime `json:"createdAt"`
}

func (AlertEventTimeline) TableName() string {
	return "alert_event_timeline"
}

type AlertEventQuery struct {
	PageQuery
	RuleID          uint   `form:"ruleId" json:"ruleId"`
	DatasourceID    uint   `form:"datasourceId" json:"datasourceId"`
	DatasourceType  string `form:"datasourceType" json:"datasourceType"`
	Severity        string `form:"severity" json:"severity"`
	Status          string `form:"status" json:"status"`
	EventType       string `form:"eventType" json:"eventType"`
	BusinessID      string `form:"businessId" json:"businessId"`
	BusinessRouteID string `form:"businessRouteId" json:"businessRouteId"`
	Fingerprint     string `form:"fingerprint" json:"fingerprint"`
	EventStatus     string `form:"eventStatus" json:"eventStatus"`
	Silenced        string `form:"silenced" json:"silenced"`
	IncludeActive   string `form:"includeActive" json:"includeActive"`
	IncludePending  string `form:"includePending" json:"includePending"`
	MemberLimit     int    `form:"memberLimit" json:"memberLimit"`
}

type AlertBusinessRouteOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type AlertEventVO struct {
	ID               uint64     `json:"id"`
	Fingerprint      string     `json:"fingerprint"`
	RuleID           uint       `json:"ruleId"`
	RuleName         string     `json:"ruleName"`
	RuleGroupID      uint       `json:"ruleGroupId"`
	RuleGroupName    string     `json:"ruleGroupName"`
	DatasourceID     uint       `json:"datasourceId"`
	DatasourceType   string     `json:"datasourceType"`
	DatasourceName   string     `json:"datasourceName"`
	Severity         string     `json:"severity"`
	Level            string     `json:"level"`
	EventStatus      string     `json:"eventStatus"`
	EventStatusName  string     `json:"eventStatusName"`
	EvalStatus       string     `json:"evalStatus"`
	EvalStatusName   string     `json:"evalStatusName"`
	Labels           JSONMap    `json:"labels"`
	Annotations      JSONMap    `json:"annotations"`
	Value            string     `json:"value"`
	AssetID          uint       `json:"assetId"`
	BusinessID       string     `json:"businessId"`
	FirstTriggeredAt util.HTime `json:"firstTriggeredAt"`
	LastEvaluatedAt  util.HTime `json:"lastEvaluatedAt"`
	LastNotifiedAt   util.HTime `json:"lastNotifiedAt"`
	RecoveredAt      util.HTime `json:"recoveredAt"`
	OccurredAt       util.HTime `json:"occurredAt"`
	DurationSeconds  int64      `json:"durationSeconds"`
	NotifyCount      int        `json:"notifyCount"`
	Suppressed       int        `json:"suppressed"`
	Silenced         int        `json:"silenced"`
	ClaimUser        string     `json:"claimUser"`
	ClaimedAt        util.HTime `json:"claimedAt"`
	CreatedAt        util.HTime `json:"createdAt"`
	UpdatedAt        util.HTime `json:"updatedAt"`
}

type AlertEventGroupVO struct {
	GroupKey          string         `json:"groupKey"`
	RuleID            uint           `json:"ruleId"`
	RuleName          string         `json:"ruleName"`
	RuleGroupID       uint           `json:"ruleGroupId"`
	RuleGroupName     string         `json:"ruleGroupName"`
	DatasourceID      uint           `json:"datasourceId"`
	DatasourceType    string         `json:"datasourceType"`
	DatasourceName    string         `json:"datasourceName"`
	Severity          string         `json:"severity"`
	Level             string         `json:"level"`
	EventStatus       string         `json:"eventStatus"`
	EventStatusName   string         `json:"eventStatusName"`
	BusinessID        string         `json:"businessId"`
	ResourceCount     int            `json:"resourceCount"`
	FiringCount       int            `json:"firingCount"`
	PendingCount      int            `json:"pendingCount"`
	SilencedCount     int            `json:"silencedCount"`
	SuppressedCount   int            `json:"suppressedCount"`
	AffectedResources []string       `json:"affectedResources"`
	FirstTriggeredAt  util.HTime     `json:"firstTriggeredAt"`
	LastEvaluatedAt   util.HTime     `json:"lastEvaluatedAt"`
	LastNotifiedAt    util.HTime     `json:"lastNotifiedAt"`
	DurationSeconds   int64          `json:"durationSeconds"`
	NotifyCount       int            `json:"notifyCount"`
	MemberTotal       int            `json:"memberTotal"`
	MembersLimited    bool           `json:"membersLimited"`
	Members           []AlertEventVO `json:"members"`
}

type AlertEventDetailVO struct {
	EventType        string                   `json:"eventType"`
	Event            AlertEventVO             `json:"event"`
	Rule             *AlertRule               `json:"rule,omitempty"`
	TriggerQuery     string                   `json:"triggerQuery"`
	TriggerCondition string                   `json:"triggerCondition"`
	TriggerQueries   []AlertEventTriggerQuery `json:"triggerQueries"`
	EventDetail      string                   `json:"eventDetail"`
	SampleLogs       *AlertEventSampleLogs    `json:"sampleLogs,omitempty"`
	Timeline         []AlertEventTimeline     `json:"timeline"`
	NotifyRecords    []AlertNotifyRecordVO    `json:"notifyRecords"`
}

type AlertEventSampleLogs struct {
	Query         string                   `json:"query"`
	Count         int                      `json:"count"`
	Limit         int                      `json:"limit"`
	Start         string                   `json:"start"`
	End           string                   `json:"end"`
	WindowSeconds int64                    `json:"windowSeconds"`
	Logs          []map[string]interface{} `json:"logs"`
	Error         string                   `json:"error,omitempty"`
}

type AlertEventTriggerQuery struct {
	Query             string  `json:"query"`
	Level             string  `json:"level"`
	Severity          string  `json:"severity"`
	ConditionOperator string  `json:"conditionOperator"`
	ConditionValue    float64 `json:"conditionValue"`
	Condition         string  `json:"condition"`
	Matched           bool    `json:"matched"`
}

type AlertNotifyRecordVO struct {
	ID              uint64     `json:"id"`
	EventID         uint64     `json:"eventId"`
	EventType       string     `json:"eventType"`
	NotifyGroupKey  string     `json:"notifyGroupKey"`
	NotifyRuleID    uint       `json:"notifyRuleId"`
	RuleID          uint       `json:"ruleId"`
	RuleName        string     `json:"ruleName"`
	ChannelID       uint       `json:"channelId"`
	ChannelName     string     `json:"channelName"`
	ChannelType     string     `json:"channelType"`
	NotifyGroupID   uint       `json:"notifyGroupId"`
	NotifyGroupName string     `json:"notifyGroupName"`
	Receiver        string     `json:"receiver"`
	Status          string     `json:"status"`
	ErrorMessage    string     `json:"errorMessage"`
	EventCount      int        `json:"eventCount"`
	ResourceCount   int        `json:"resourceCount"`
	Request         string     `json:"request"`
	Response        string     `json:"response"`
	Content         string     `json:"content"`
	Operator        string     `json:"operator"`
	CreatedAt       util.HTime `json:"createdAt"`
	CreatedAtFull   string     `json:"createdAtFull"`
}

type SilenceEventRequest struct {
	DurationMinutes int64      `json:"durationMinutes"`
	EndTime         util.HTime `json:"endTime"`
	Reason          string     `json:"reason"`
	Matchers        JSONMap    `json:"matchers"`
}
