package model

import "time"

// BillMonthlySummary stores the provider total used by dashboards and comparisons.
type BillMonthlySummary struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CloudAccountID uint      `gorm:"not null;uniqueIndex:uk_bill_monthly_account,priority:1;index" json:"cloudAccountId"`
	Operator       string    `gorm:"type:varchar(32);not null;index" json:"operator"`
	Month          string    `gorm:"type:char(7);not null;uniqueIndex:uk_bill_monthly_account,priority:2;index" json:"month"`
	TotalCost      float64   `gorm:"type:decimal(20,4);not null;default:0" json:"totalCost"`
	Currency       string    `gorm:"type:char(3);not null;default:CNY" json:"currency"`
	BillStatus     string    `gorm:"type:varchar(16);not null;default:estimating" json:"billStatus"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (BillMonthlySummary) TableName() string { return "bill_monthly_summary" }
