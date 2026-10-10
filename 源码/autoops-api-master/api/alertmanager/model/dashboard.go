package model

import "dodevops-api/common/util"

type AlertDashboardQuery struct {
	Range          string `form:"range" json:"range"`
	StartTime      string `form:"startTime" json:"startTime"`
	EndTime        string `form:"endTime" json:"endTime"`
	Limit          int    `form:"limit" json:"limit"`
	DatasourceID   uint   `form:"datasourceId" json:"datasourceId"`
	DatasourceType string `form:"datasourceType" json:"datasourceType"`
	Severity       string `form:"severity" json:"severity"`
	BusinessID     string `form:"businessId" json:"businessId"`
}

type AlertDashboardChartsVO struct {
	Range            string                           `json:"range"`
	StartTime        util.HTime                       `json:"startTime"`
	EndTime          util.HTime                       `json:"endTime"`
	AlertNameTop     []AlertDashboardAlertNameTopItem `json:"alertNameTop"`
	NotifyTrend      []AlertDashboardNotifyTrendPoint `json:"notifyTrend"`
	RuleGroupTop     []AlertDashboardHostGroupTopItem `json:"ruleGroupTop"`
	BusinessRouteTop []AlertDashboardHostGroupTopItem `json:"businessRouteTop"`
	// HostGroupTop keeps older dashboard clients working; its values mirror
	// alert rule groups for the "告警群组 Top" chart.
	HostGroupTop []AlertDashboardHostGroupTopItem `json:"hostGroupTop"`
}

type AlertDashboardScreenVO struct {
	SeverityDistribution []AlertDashboardSeverityDistributionItem `json:"severityDistribution"`
	LatestActiveTop5     []AlertEventVO                           `json:"latestActiveTop5"`
	AlertNameTop10       []AlertDashboardAlertNameTopItem         `json:"alertNameTop10"`
	TotalActive          int64                                    `json:"totalActive"`
	Source               string                                   `json:"source"`
}

type AlertDashboardSeverityDistributionItem struct {
	Level   string  `json:"level"`
	Name    string  `json:"name"`
	Count   int64   `json:"count"`
	Percent float64 `json:"percent"`
}

type AlertDashboardAlertNameTopItem struct {
	RuleID  uint    `json:"ruleId"`
	Name    string  `json:"name"`
	Count   int64   `json:"count"`
	Percent float64 `json:"percent"`
}

type AlertDashboardHostGroupTopItem struct {
	GroupID   string  `json:"groupId"`
	GroupName string  `json:"groupName"`
	Count     int64   `json:"count"`
	Percent   float64 `json:"percent"`
}

type AlertDashboardNotifyTrendPoint struct {
	Date              string `json:"date"`
	AlertCount        int64  `json:"alertCount"`
	NotificationCount int64  `json:"notificationCount"`
	SuccessCount      int64  `json:"successCount"`
	FailedCount       int64  `json:"failedCount"`
	Source            string `json:"source"`
}

type AlertDashboardEventRow struct {
	ID               uint64        `json:"id"`
	RuleID           uint          `json:"ruleId"`
	RuleName         string        `json:"ruleName"`
	RuleGroupID      uint          `gorm:"column:rule_group_id" json:"ruleGroupId"`
	RuleGroupName    string        `gorm:"column:rule_group_name" json:"ruleGroupName"`
	NotifyRuleIDs    JSONUintArray `gorm:"column:notify_rule_ids;type:json" json:"notifyRuleIds"`
	DatasourceID     uint          `json:"datasourceId"`
	DatasourceType   string        `json:"datasourceType"`
	Severity         string        `json:"severity"`
	Labels           JSONMap       `gorm:"column:labels;type:json" json:"labels"`
	BusinessID       string        `json:"businessId"`
	FirstTriggeredAt util.HTime    `json:"firstTriggeredAt"`
	LastNotifiedAt   util.HTime    `json:"lastNotifiedAt"`
	NotifyCount      int           `json:"notifyCount"`
	CreatedAt        util.HTime    `json:"createdAt"`
}
