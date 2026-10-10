package model

import (
	"time"
)

const (
	BalanceStatusUnknown    = "unknown"
	BalanceStatusRefreshing = "refreshing"
	BalanceStatusNormal     = "normal"
	BalanceStatusLow        = "low"
	BalanceStatusError      = "error"
	BalanceStatusDisabled   = "disabled"
)

// BillCloudAccount is a billing account linked to a credential in key management.
type BillCloudAccount struct {
	ID                          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	Operator                    string     `gorm:"type:varchar(32);not null;index" json:"operator"`
	AccountName                 string     `gorm:"type:varchar(128);not null;default:'';index" json:"accountName"`
	AccountID                   string     `gorm:"type:varchar(128);not null;default:'';index" json:"accountId"`
	KeyManageID                 uint       `gorm:"not null;default:0;index" json:"keyManageId"`
	Enabled                     int        `gorm:"type:tinyint;not null;default:1;index" json:"enabled"`
	BillSyncEnabled             int        `gorm:"type:tinyint;not null;default:1;index" json:"billSyncEnabled"`
	BalanceSyncEnabled          int        `gorm:"type:tinyint;not null;default:1;index" json:"balanceSyncEnabled"`
	BalanceSyncIntervalMinutes  int        `gorm:"not null;default:60" json:"balanceSyncIntervalMinutes"`
	SyncCycle                   string     `gorm:"type:varchar(32);not null;default:'daily'" json:"syncCycle"`
	LookbackMonths              int        `gorm:"not null;default:12" json:"lookbackMonths"`
	SyncStatus                  string     `gorm:"type:varchar(32);not null;default:'pending';index" json:"syncStatus"`
	LastSyncTime                *time.Time `gorm:"index" json:"lastSyncTime"`
	LastSyncError               string     `gorm:"type:varchar(1000)" json:"lastSyncError"`
	CurrentMonthCost            float64    `gorm:"type:decimal(20,6);not null;default:0" json:"currentMonthCost"`
	AvailableBalance            float64    `gorm:"type:decimal(20,6);not null;default:0" json:"availableBalance"`
	Currency                    string     `gorm:"type:char(3);not null;default:'CNY'" json:"currency"`
	BalanceStatus               string     `gorm:"type:varchar(32);not null;default:'unknown';index" json:"balanceStatus"`
	LastBalanceTime             *time.Time `gorm:"index" json:"lastBalanceTime"`
	LastBalanceError            string     `gorm:"type:varchar(1000)" json:"lastBalanceError"`
	BalanceAlertRuleID          uint       `gorm:"not null;default:0;index" json:"balanceAlertRuleId"`
	BalanceAlertEnabled         int        `gorm:"type:tinyint;not null;default:0;index" json:"balanceAlertEnabled"`
	BalanceAlertThreshold       float64    `gorm:"type:decimal(20,6);not null;default:0" json:"balanceAlertThreshold"`
	BalanceAlertThresholdDays   int        `gorm:"not null;default:0" json:"balanceAlertThresholdDays"`
	BalanceAlertOnQueryError    int        `gorm:"type:tinyint;not null;default:1" json:"balanceAlertOnQueryError"`
	BalanceAlertSeverity        string     `gorm:"type:varchar(32);not null;default:'warning'" json:"balanceAlertSeverity"`
	BalanceAlertRepeatMinutes   int        `gorm:"not null;default:1440" json:"balanceAlertRepeatMinutes"`
	BalanceAlertNotifyRecovered int        `gorm:"type:tinyint;not null;default:1" json:"balanceAlertNotifyRecovered"`
	BalanceAlertNotifyRuleIDs   string     `gorm:"type:text" json:"-"`
	CreatedAt                   time.Time  `json:"createdAt"`
	UpdatedAt                   time.Time  `json:"updatedAt"`
	DeletedAt                   *time.Time `gorm:"index" json:"-"`
}

func (BillCloudAccount) TableName() string {
	return "bill_cloud_account"
}
