package model

import "dodevops-api/common/util"

const (
	NotifyChannelFeishu     = "feishu"
	NotifyChannelFeishuCard = "feishu_card"
	NotifyChannelDingtalk   = "dingtalk"
	NotifyChannelWecom      = "wecom"
	NotifyChannelLeliao     = "leliao"

	NotifyTemplateMarkdown = "markdown"
	NotifyTemplateCard     = "card"

	NotifyStatusSuccess = "success"
	NotifyStatusFailed  = "failed"

	NotifyRouteMatchExact    = "exact"
	NotifyRouteMatchContains = "contains"
	NotifyRouteMatchRegex    = "regex"
)

type AlertSilenceRule struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Name       string     `gorm:"size:150;not null;index" json:"name"`
	Matchers   JSONMap    `gorm:"type:json" json:"matchers"`
	StartTime  util.HTime `json:"startTime"`
	EndTime    util.HTime `json:"endTime"`
	RepeatType string     `gorm:"size:32;default:'once'" json:"repeatType"`
	Reason     string     `gorm:"size:500" json:"reason"`
	Enabled    int        `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	CreatedBy  string     `gorm:"size:64" json:"createdBy"`
	UpdatedBy  string     `gorm:"size:64" json:"updatedBy"`
	CreatedAt  util.HTime `json:"createdAt"`
	UpdatedAt  util.HTime `json:"updatedAt"`
}

func (AlertSilenceRule) TableName() string {
	return "alert_silence_rules"
}

type AlertNotifyChannel struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"size:150;not null;index" json:"name"`
	Type        string     `gorm:"size:64;not null;index" json:"type"`
	Description string     `gorm:"size:500" json:"description"`
	Config      JSONMap    `gorm:"type:json" json:"config"`
	Enabled     int        `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	CreatedBy   string     `gorm:"size:64" json:"createdBy"`
	UpdatedBy   string     `gorm:"size:64" json:"updatedBy"`
	CreatedAt   util.HTime `json:"createdAt"`
	UpdatedAt   util.HTime `json:"updatedAt"`
}

func (AlertNotifyChannel) TableName() string {
	return "alert_notify_channels"
}

type AlertNotifyGroup struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	Name        string          `gorm:"size:150;not null;index" json:"name"`
	BusinessID  string          `gorm:"size:128;not null;uniqueIndex" json:"businessId"`
	ChannelType string          `gorm:"size:64;not null;default:'leliao';index" json:"channelType"`
	UserIDs     JSONStringArray `gorm:"column:user_ids;type:json" json:"userIds"`
	Description string          `gorm:"size:500" json:"description"`
	Enabled     int             `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	CreatedBy   string          `gorm:"size:64" json:"createdBy"`
	UpdatedBy   string          `gorm:"size:64" json:"updatedBy"`
	CreatedAt   util.HTime      `json:"createdAt"`
	UpdatedAt   util.HTime      `json:"updatedAt"`
}

func (AlertNotifyGroup) TableName() string {
	return "alert_notify_groups"
}

// AlertNotifyRouteRule maps an event label value to a notification group.
// NotifyRuleID=0 means the route is available to every notification policy.
// OverrideGroup allows a matching route to replace a policy channel's default group.
type AlertNotifyRouteRule struct {
	ID              uint                       `gorm:"primaryKey" json:"id"`
	Name            string                     `gorm:"size:150;not null;uniqueIndex" json:"name"`
	Conditions      AlertNotifyRouteConditions `gorm:"column:conditions;type:json" json:"conditions"`
	NotifyRuleID    uint                       `gorm:"column:notify_rule_id;index" json:"notifyRuleId"`
	NotifyGroupID   uint                       `gorm:"column:notify_group_id;not null;index" json:"notifyGroupId"`
	OverrideGroup   int                        `gorm:"column:override_group;type:tinyint;not null;default:0" json:"overrideGroup"`
	Priority        int                        `gorm:"type:integer;not null;default:0;index" json:"priority"`
	Description     string                     `gorm:"size:500" json:"description"`
	Enabled         int                        `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	CreatedBy       string                     `gorm:"size:64" json:"createdBy"`
	UpdatedBy       string                     `gorm:"size:64" json:"updatedBy"`
	CreatedAt       util.HTime                 `json:"createdAt"`
	UpdatedAt       util.HTime                 `json:"updatedAt"`
	NotifyGroupName string                     `gorm:"-" json:"notifyGroupName,omitempty"`
}

func (AlertNotifyRouteRule) TableName() string {
	return "alert_notify_route_rules"
}

type FeishuEventCallbackRequest struct {
	Type      string  `json:"type"`
	Schema    string  `json:"schema"`
	Header    JSONMap `json:"header"`
	Event     JSONMap `json:"event"`
	Token     string  `json:"token"`
	Challenge string  `json:"challenge"`
	Encrypt   string  `json:"encrypt"`
}

type AlertNotifyTemplate struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Ident        string     `gorm:"size:100;not null;index" json:"ident"`
	Name         string     `gorm:"size:150;not null;index" json:"name"`
	ChannelType  string     `gorm:"size:64;not null;index" json:"channelType"`
	TemplateType string     `gorm:"size:32;not null;default:'markdown';index" json:"templateType"`
	Description  string     `gorm:"size:500" json:"description"`
	Content      JSONMap    `gorm:"type:json" json:"content"`
	Builtin      int        `gorm:"type:tinyint;not null;default:0;index" json:"builtin"`
	Enabled      int        `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	CreatedBy    string     `gorm:"size:64" json:"createdBy"`
	UpdatedBy    string     `gorm:"size:64" json:"updatedBy"`
	CreatedAt    util.HTime `json:"createdAt"`
	UpdatedAt    util.HTime `json:"updatedAt"`
}

func (AlertNotifyTemplate) TableName() string {
	return "alert_notify_templates"
}

type AlertNotifyRule struct {
	ID                       uint       `gorm:"primaryKey" json:"id"`
	Name                     string     `gorm:"size:150;not null;index" json:"name"`
	Matchers                 JSONMap    `gorm:"type:json" json:"matchers"`
	Channels                 JSONMap    `gorm:"type:json" json:"channels"`
	ReceiverGroups           JSONMap    `gorm:"type:json" json:"receiverGroups"`
	RepeatNotify             int        `gorm:"type:tinyint;not null;default:1" json:"repeatNotify"`
	RepeatMode               string     `gorm:"size:32;not null;default:'bySeverity'" json:"repeatMode"`
	RepeatIntervalSeconds    int64      `gorm:"type:bigint;not null;default:3600" json:"repeatIntervalSeconds"`
	RepeatIntervals          JSONMap    `gorm:"type:json" json:"repeatIntervals"`
	MaxRepeatTimes           int        `gorm:"type:integer;not null;default:0" json:"maxRepeatTimes"`
	RecoveryNotify           int        `gorm:"type:tinyint;not null;default:1" json:"recoveryNotify"`
	AggregationEnabled       int        `gorm:"type:tinyint;not null;default:1" json:"aggregationEnabled"`
	GroupWaitSeconds         int64      `gorm:"type:bigint;not null;default:30" json:"groupWaitSeconds"`
	GroupIntervalSeconds     int64      `gorm:"type:bigint;not null;default:300" json:"groupIntervalSeconds"`
	AggregationWindowSeconds int64      `gorm:"type:bigint;not null;default:60" json:"aggregationWindowSeconds"`
	Priority                 int        `gorm:"type:integer;not null;default:0;index" json:"priority"`
	Enabled                  int        `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	CreatedBy                string     `gorm:"size:64" json:"createdBy"`
	UpdatedBy                string     `gorm:"size:64" json:"updatedBy"`
	CreatedAt                util.HTime `json:"createdAt"`
	UpdatedAt                util.HTime `json:"updatedAt"`
}

func (AlertNotifyRule) TableName() string {
	return "alert_notify_rules"
}

type AlertNotifyRecord struct {
	ID              uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	EventID         uint64     `gorm:"column:event_id;index" json:"eventId"`
	EventType       string     `gorm:"size:32;index" json:"eventType"`
	NotifyGroupKey  string     `gorm:"size:200;index" json:"notifyGroupKey"`
	NotifyRuleID    uint       `gorm:"column:notify_rule_id;index" json:"notifyRuleId"`
	RuleID          uint       `gorm:"column:rule_id;index" json:"ruleId"`
	RuleName        string     `gorm:"size:150;index" json:"ruleName"`
	ChannelID       uint       `gorm:"column:channel_id;index" json:"channelId"`
	ChannelName     string     `gorm:"size:150" json:"channelName"`
	ChannelType     string     `gorm:"size:64;index" json:"channelType"`
	NotifyGroupID   uint       `gorm:"column:notify_group_id;index" json:"notifyGroupId"`
	NotifyGroupName string     `gorm:"size:150" json:"notifyGroupName"`
	DatasourceID    uint       `gorm:"column:datasource_id;index" json:"datasourceId"`
	DatasourceType  string     `gorm:"size:64;index" json:"datasourceType"`
	Severity        string     `gorm:"size:32;index" json:"severity"`
	Receiver        string     `gorm:"size:255;index" json:"receiver"`
	Status          string     `gorm:"size:32;index" json:"status"`
	ErrorMessage    string     `gorm:"size:500" json:"errorMessage"`
	EventCount      int        `gorm:"type:integer;default:0" json:"eventCount"`
	ResourceCount   int        `gorm:"type:integer;default:0" json:"resourceCount"`
	RuleNames       JSONMap    `gorm:"type:json" json:"ruleNames"`
	Request         string     `gorm:"type:text" json:"request"`
	Response        string     `gorm:"type:text" json:"response"`
	Content         string     `gorm:"type:text" json:"content"`
	BusinessID      string     `gorm:"size:128;index" json:"businessId"`
	Operator        string     `gorm:"size:64" json:"operator"`
	CreatedAt       util.HTime `gorm:"index" json:"createdAt"`
}

func (AlertNotifyRecord) TableName() string {
	return "alert_notify_records"
}

type AlertSilenceRuleQuery struct {
	PageQuery
	Enabled string `form:"enabled" json:"enabled"`
}

type AlertNotifyChannelQuery struct {
	PageQuery
	Type    string `form:"type" json:"type"`
	Enabled string `form:"enabled" json:"enabled"`
}

type AlertNotifyGroupQuery struct {
	PageQuery
	BusinessID  string `form:"businessId" json:"businessId"`
	ChannelType string `form:"channelType" json:"channelType"`
	Enabled     string `form:"enabled" json:"enabled"`
}

type AlertNotifyRouteRuleQuery struct {
	PageQuery
	MatchMode     string `form:"matchMode" json:"matchMode"`
	NotifyRuleID  uint   `form:"notifyRuleId" json:"notifyRuleId"`
	NotifyGroupID uint   `form:"notifyGroupId" json:"notifyGroupId"`
	Enabled       string `form:"enabled" json:"enabled"`
}

type AlertNotifyTemplateQuery struct {
	PageQuery
	ChannelType  string `form:"channelType" json:"channelType"`
	TemplateType string `form:"templateType" json:"templateType"`
	Builtin      string `form:"builtin" json:"builtin"`
	Enabled      string `form:"enabled" json:"enabled"`
}

type AlertNotifyRuleQuery struct {
	PageQuery
	Enabled string `form:"enabled" json:"enabled"`
}

type AlertNotifyRecordQuery struct {
	PageQuery
	EventID        uint64 `form:"eventId" json:"eventId"`
	RuleID         uint   `form:"ruleId" json:"ruleId"`
	NotifyRuleID   uint   `form:"notifyRuleId" json:"notifyRuleId"`
	ChannelID      uint   `form:"channelId" json:"channelId"`
	ChannelType    string `form:"channelType" json:"channelType"`
	Status         string `form:"status" json:"status"`
	DatasourceID   uint   `form:"datasourceId" json:"datasourceId"`
	DatasourceType string `form:"datasourceType" json:"datasourceType"`
	Severity       string `form:"severity" json:"severity"`
	BusinessID     string `form:"businessId" json:"businessId"`
}

type AlertNotifyRecordStats struct {
	Total        int64 `json:"total"`
	SuccessCount int64 `json:"successCount"`
	FailedCount  int64 `json:"failedCount"`
}

type AlertNotifyChannelTestRequest struct {
	TemplateID  uint     `json:"templateId"`
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	IsRecovered bool     `json:"isRecovered"`
	Event       JSONMap  `json:"event"`
	Receivers   []string `json:"receivers"`
	UserIDs     []string `json:"userIds"`
}

type AlertNotifyChannelTestResponse struct {
	ChannelID    uint       `json:"channelId"`
	ChannelName  string     `json:"channelName"`
	ChannelType  string     `json:"channelType"`
	Target       string     `json:"target"`
	Status       string     `json:"status"`
	Response     string     `json:"response"`
	ErrorMessage string     `json:"errorMessage"`
	DurationMs   int64      `json:"durationMs"`
	SentAt       util.HTime `json:"sentAt"`
}

type AlertNotifyTemplatePreviewRequest struct {
	IsRecovered bool    `json:"isRecovered"`
	Event       JSONMap `json:"event"`
}

type AlertNotifyTemplatePreviewResponse struct {
	Title      string  `json:"title"`
	TitleColor string  `json:"titleColor"`
	Content    string  `json:"content"`
	Footer     string  `json:"footer"`
	Card       JSONMap `json:"card"`
	Data       JSONMap `json:"data"`
}
