package dao

import (
	"dodevops-api/api/costcenter/model"
	"time"

	"gorm.io/gorm"
)

type BillCloudAccountDAO struct {
	db *gorm.DB
}

func NewBillCloudAccountDAO(db *gorm.DB) *BillCloudAccountDAO {
	return &BillCloudAccountDAO{db: db}
}

func (d *BillCloudAccountDAO) Create(data *model.BillCloudAccount) error {
	return d.db.Create(data).Error
}

func (d *BillCloudAccountDAO) Update(id uint, updates map[string]interface{}) error {
	return d.db.Model(&model.BillCloudAccount{}).Where("id = ?", id).Updates(updates).Error
}

func (d *BillCloudAccountDAO) Delete(id uint) error {
	return d.db.Where("id = ?", id).Delete(&model.BillCloudAccount{}).Error
}

func (d *BillCloudAccountDAO) GetByID(id uint) (*model.BillCloudAccount, error) {
	var data model.BillCloudAccount
	err := d.db.Where("id = ?", id).First(&data).Error
	return &data, err
}

func (d *BillCloudAccountDAO) GetList(operator, syncStatus string, enabled *int, page, pageSize int) ([]model.BillCloudAccount, int64, error) {
	var list []model.BillCloudAccount
	var total int64

	query := d.db.Model(&model.BillCloudAccount{})
	if operator != "" {
		query = query.Where("operator = ?", operator)
	}
	if syncStatus != "" {
		query = query.Where("sync_status = ?", syncStatus)
	}
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *BillCloudAccountDAO) UpdateSyncStatus(id uint, status string, cost float64) error {
	now := time.Now()
	updates := map[string]interface{}{
		"sync_status":        status,
		"last_sync_time":     &now,
		"current_month_cost": cost,
	}
	return d.db.Model(&model.BillCloudAccount{}).Where("id = ?", id).Updates(updates).Error
}

func (d *BillCloudAccountDAO) ClaimSync(id uint) (bool, error) {
	result := d.db.Model(&model.BillCloudAccount{}).
		Where("id = ? AND enabled = ? AND bill_sync_enabled = ? AND sync_status <> ?", id, 1, 1, "syncing").
		Update("sync_status", "syncing")
	return result.RowsAffected == 1, result.Error
}

func (d *BillCloudAccountDAO) ClaimBalanceRefresh(id uint) (bool, error) {
	result := d.db.Model(&model.BillCloudAccount{}).
		Where("id = ? AND enabled = ? AND balance_sync_enabled = ? AND balance_status <> ?", id, 1, 1, model.BalanceStatusRefreshing).
		Update("balance_status", model.BalanceStatusRefreshing)
	return result.RowsAffected == 1, result.Error
}

func (d *BillCloudAccountDAO) SaveBalance(cloudAccountID uint, balance float64, currency, status string, collectedAt time.Time) error {
	return d.db.Model(&model.BillCloudAccount{}).Where("id = ?", cloudAccountID).Updates(map[string]interface{}{
		"available_balance":  balance,
		"currency":           currency,
		"balance_status":     status,
		"last_balance_time":  &collectedAt,
		"last_balance_error": "",
	}).Error
}

func (d *BillCloudAccountDAO) SaveBalanceError(cloudAccountID uint, message string, checkedAt time.Time) error {
	return d.db.Model(&model.BillCloudAccount{}).Where("id = ?", cloudAccountID).Updates(map[string]interface{}{
		"balance_status":     model.BalanceStatusError,
		"last_balance_time":  &checkedAt,
		"last_balance_error": message,
	}).Error
}

func (d *BillCloudAccountDAO) FinishSync(id uint, status, message string, balance, currentCost float64, currency string) error {
	now := time.Now()
	return d.db.Model(&model.BillCloudAccount{}).Where("id = ?", id).Updates(map[string]interface{}{
		"sync_status": status, "last_sync_time": &now, "last_sync_error": message,
		"available_balance": balance, "current_month_cost": currentCost, "currency": currency,
	}).Error
}

func (d *BillCloudAccountDAO) FinishBillSync(id uint, status, message string, currentCost float64) error {
	now := time.Now()
	return d.db.Model(&model.BillCloudAccount{}).Where("id = ?", id).Updates(map[string]interface{}{
		"sync_status": status, "last_sync_time": &now, "last_sync_error": message,
		"current_month_cost": currentCost,
	}).Error
}

func (d *BillCloudAccountDAO) KeyManageExists(keyManageID, excludeID uint) (bool, error) {
	var count int64
	query := d.db.Model(&model.BillCloudAccount{}).Where("key_manage_id = ?", keyManageID)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

func (d *BillCloudAccountDAO) SummaryCount(cloudAccountID uint) (int64, error) {
	var count int64
	err := d.db.Model(&model.BillMonthlySummary{}).Where("cloud_account_id = ?", cloudAccountID).Count(&count).Error
	return count, err
}

// HasIncompleteIssuedDetails detects summary rows whose detail total is missing
// or materially below the provider total. Those months need a lookback sync.
func (d *BillCloudAccountDAO) HasIncompleteIssuedDetails(cloudAccountID uint, months []string) (bool, error) {
	if len(months) == 0 {
		return false, nil
	}
	var missing bool
	err := d.db.Raw(`
SELECT EXISTS(
    SELECT 1
    FROM bill_monthly_summary summary
    WHERE summary.cloud_account_id = ?
      AND summary.month IN ?
      AND summary.bill_status = 'issued'
      AND COALESCE((
          SELECT SUM(detail.payable_amount)
          FROM bill_detail detail
          WHERE detail.cloud_account_id = summary.cloud_account_id
            AND detail.month = summary.month
      ), 0) + 0.01 < summary.total_cost
)`, cloudAccountID, months).Scan(&missing).Error
	return missing, err
}

func (d *BillCloudAccountDAO) ActiveList() ([]model.BillCloudAccount, error) {
	var rows []model.BillCloudAccount
	err := d.db.Where("enabled = ?", 1).Order("id ASC").Find(&rows).Error
	return rows, err
}

func (d *BillCloudAccountDAO) CheckExists(operator, accountID string, excludeID uint) (bool, error) {
	var count int64
	query := d.db.Model(&model.BillCloudAccount{}).Where("operator = ? AND account_id = ?", operator, accountID)
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}
