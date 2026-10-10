package model

import "dodevops-api/common/util"

const (
	DomainStatusUnknown  = "unknown"
	DomainStatusNormal   = "normal"
	DomainStatusAbnormal = "abnormal"
)

// AlertDomain stores both domain probe settings and the latest probe result.
type AlertDomain struct {
	ID                       uint            `gorm:"primaryKey" json:"id"`
	Domain                   string          `gorm:"size:253;not null;uniqueIndex" json:"domain"`
	Protocol                 string          `gorm:"size:16;not null;default:'https'" json:"protocol"`
	Port                     int             `gorm:"type:integer;not null;default:0" json:"port"`
	Path                     string          `gorm:"size:500;not null;default:'/'" json:"path"`
	Tags                     JSONStringArray `gorm:"type:json" json:"tags"`
	ExpectedStatus           int             `gorm:"type:integer;not null;default:0" json:"expectedStatus"`
	CheckIntervalSeconds     int64           `gorm:"type:bigint;not null;default:300" json:"checkIntervalSeconds"`
	TimeoutSeconds           int64           `gorm:"type:bigint;not null;default:10" json:"timeoutSeconds"`
	SSLExpireWarnDays        int             `gorm:"type:integer;not null;default:30" json:"sslExpireWarnDays"`
	Enabled                  int             `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	Remarks                  string          `gorm:"size:500" json:"remarks"`
	AlertRuleID              uint            `gorm:"column:alert_rule_id;not null;default:0;index" json:"alertRuleId"`
	NotifyRuleIDs            JSONUintArray   `gorm:"column:notify_rule_ids;type:json" json:"notifyRuleIds"`
	Status                   string          `gorm:"size:32;not null;default:'unknown';index" json:"status"`
	StatusCode               int             `gorm:"type:integer;not null;default:0" json:"statusCode"`
	ResponseTimeMS           int64           `gorm:"column:response_time_ms;type:bigint;not null;default:0" json:"responseTimeMs"`
	CertificateExpiresAt     *util.HTime     `json:"certificateExpiresAt"`
	CertificateRemainingDays int             `gorm:"type:integer;not null;default:0" json:"certificateRemainingDays"`
	CertificateIssuer        string          `gorm:"size:500" json:"certificateIssuer"`
	FailureReason            string          `gorm:"size:1000" json:"failureReason"`
	LastCheckedAt            *util.HTime     `gorm:"index" json:"lastCheckedAt"`
	NextCheckAt              *util.HTime     `gorm:"index" json:"nextCheckAt"`
	CreatedBy                string          `gorm:"size:64" json:"createdBy"`
	UpdatedBy                string          `gorm:"size:64" json:"updatedBy"`
	CreatedAt                util.HTime      `json:"createdAt"`
	UpdatedAt                util.HTime      `json:"updatedAt"`
}

func (AlertDomain) TableName() string {
	return "alert_domains"
}

type AlertDomainQuery struct {
	PageQuery
	Tag              string `form:"tag" json:"tag"`
	Status           string `form:"status" json:"status"`
	Enabled          string `form:"enabled" json:"enabled"`
	ExpireWithinDays int    `form:"expireWithinDays" json:"expireWithinDays"`
}

type AlertDomainInput struct {
	Domain               string        `json:"domain"`
	Protocol             string        `json:"protocol"`
	Port                 int           `json:"port"`
	Path                 string        `json:"path"`
	Tags                 []string      `json:"tags"`
	ExpectedStatus       int           `json:"expectedStatus"`
	CheckIntervalSeconds int64         `json:"checkIntervalSeconds"`
	TimeoutSeconds       int64         `json:"timeoutSeconds"`
	SSLExpireWarnDays    int           `json:"sslExpireWarnDays"`
	Enabled              *int          `json:"enabled"`
	Remarks              string        `json:"remarks"`
	NotifyRuleIDs        JSONUintArray `json:"notifyRuleIds"`
}

type AlertDomainCheckRequest struct {
	IDs []uint `json:"ids"`
}

type AlertDomainSummary struct {
	Total    int64 `json:"total"`
	Normal   int64 `json:"normal"`
	Abnormal int64 `json:"abnormal"`
	Enabled  int64 `json:"enabled"`
	Expiring int64 `json:"expiring"`
}
