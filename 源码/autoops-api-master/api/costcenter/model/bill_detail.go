package model

import "time"

// BillDetail is a provider-neutral resource charge item.
type BillDetail struct {
	ID              uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CloudAccountID  uint       `gorm:"not null;uniqueIndex:uk_bill_detail_account_month_record,priority:1;index:idx_bill_detail_account_month,priority:1" json:"cloudAccountId"`
	Operator        string     `gorm:"type:varchar(32);not null;index" json:"operator"`
	Month           string     `gorm:"type:char(7);not null;uniqueIndex:uk_bill_detail_account_month_record,priority:2;index:idx_bill_detail_account_month,priority:2;index" json:"month"`
	RecordID        string     `gorm:"type:varchar(191);not null;uniqueIndex:uk_bill_detail_account_month_record,priority:3" json:"recordId"`
	ResourceID      string     `gorm:"type:varchar(191);not null;default:'';index" json:"resourceId"`
	ResourceName    string     `gorm:"type:varchar(255);not null;default:''" json:"resourceName"`
	ProductCode     string     `gorm:"type:varchar(128);not null;default:'';index" json:"productCode"`
	ProductName     string     `gorm:"type:varchar(255);not null;default:'';index" json:"productName"`
	SubProductName  string     `gorm:"type:varchar(255);not null;default:''" json:"subProductName"`
	BillingItemCode string     `gorm:"type:varchar(128);not null;default:''" json:"billingItemCode"`
	BillingItemName string     `gorm:"type:varchar(255);not null;default:''" json:"billingItemName"`
	UsageQuantity   float64    `gorm:"type:decimal(24,8);not null;default:0" json:"usageQuantity"`
	UsageUnit       string     `gorm:"type:varchar(64);not null;default:''" json:"usageUnit"`
	OriginalAmount  float64    `gorm:"type:decimal(20,6);not null;default:0" json:"originalAmount"`
	DiscountAmount  float64    `gorm:"type:decimal(20,6);not null;default:0" json:"discountAmount"`
	PayableAmount   float64    `gorm:"type:decimal(20,6);not null;default:0" json:"payableAmount"`
	Currency        string     `gorm:"type:char(3);not null;default:'CNY'" json:"currency"`
	ChargeType      string     `gorm:"type:varchar(64);not null;default:'';index" json:"chargeType"`
	TransactionType string     `gorm:"type:varchar(64);not null;default:''" json:"transactionType"`
	Region          string     `gorm:"type:varchar(128);not null;default:'';index" json:"region"`
	Zone            string     `gorm:"type:varchar(128);not null;default:''" json:"zone"`
	ProjectName     string     `gorm:"type:varchar(255);not null;default:''" json:"projectName"`
	OrderID         string     `gorm:"type:varchar(191);not null;default:'';index" json:"orderId"`
	TransactionID   string     `gorm:"type:varchar(191);not null;default:'';index" json:"transactionId"`
	DeductedAt      *time.Time `gorm:"index" json:"deductedAt"`
	BillingStartAt  *time.Time `json:"billingStartAt"`
	BillingEndAt    *time.Time `json:"billingEndAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (BillDetail) TableName() string { return "bill_detail" }
