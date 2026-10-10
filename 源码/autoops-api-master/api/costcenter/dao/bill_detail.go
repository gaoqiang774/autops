package dao

import (
	"strings"

	"dodevops-api/api/costcenter/model"

	"gorm.io/gorm"
)

type BillDetailDAO struct {
	db *gorm.DB
}

func NewBillDetailDAO(db *gorm.DB) *BillDetailDAO {
	return &BillDetailDAO{db: db}
}

func (d *BillDetailDAO) List(month, operator string, cloudAccountID uint, keyword, productName, chargeType, region string, page, pageSize int) ([]model.BillDetail, int64, error) {
	query := d.filteredQuery(month, operator, cloudAccountID, keyword, productName, chargeType, region)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []model.BillDetail
	err := query.Order("deducted_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error
	return rows, total, err
}

func (d *BillDetailDAO) GetByID(id uint) (*model.BillDetail, error) {
	var detail model.BillDetail
	err := d.db.First(&detail, id).Error
	return &detail, err
}

func (d *BillDetailDAO) ExportRows(month, operator string, cloudAccountID uint, keyword, productName, chargeType, region string, limit int) ([]model.BillDetail, int64, error) {
	query := d.filteredQuery(month, operator, cloudAccountID, keyword, productName, chargeType, region)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 || total > int64(limit) {
		return nil, total, nil
	}
	var rows []model.BillDetail
	err := query.Order("deducted_at DESC, id DESC").Limit(limit).Find(&rows).Error
	return rows, total, err
}

func (d *BillDetailDAO) filteredQuery(month, operator string, cloudAccountID uint, keyword, productName, chargeType, region string) *gorm.DB {
	query := d.db.Model(&model.BillDetail{})
	if value := strings.TrimSpace(month); value != "" {
		query = query.Where("month = ?", value)
	}
	if value := strings.TrimSpace(operator); value != "" {
		query = query.Where("operator = ?", value)
	}
	if cloudAccountID > 0 {
		query = query.Where("cloud_account_id = ?", cloudAccountID)
	}
	if value := strings.TrimSpace(keyword); value != "" {
		pattern := "%" + value + "%"
		query = query.Where("resource_id LIKE ? OR resource_name LIKE ? OR order_id LIKE ? OR transaction_id LIKE ?", pattern, pattern, pattern, pattern)
	}
	if value := strings.TrimSpace(productName); value != "" {
		query = query.Where("product_name LIKE ?", "%"+value+"%")
	}
	if value := strings.TrimSpace(chargeType); value != "" {
		query = query.Where("charge_type = ?", value)
	}
	if value := strings.TrimSpace(region); value != "" {
		query = query.Where("region LIKE ?", "%"+value+"%")
	}
	return query
}
