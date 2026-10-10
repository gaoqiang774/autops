package dao

import (
	dbmodel "dodevops-api/api/db/model"
	"dodevops-api/common"
	"strings"
)

type AuditDao struct{}

func NewAuditDao() *AuditDao {
	return &AuditDao{}
}

func (d *AuditDao) Create(audit *dbmodel.OperationAudit) error {
	return common.GetDB().Create(audit).Error
}

func (d *AuditDao) GetByID(id uint) (*dbmodel.OperationAudit, error) {
	var audit dbmodel.OperationAudit
	err := common.GetDB().First(&audit, id).Error
	return &audit, err
}

func (d *AuditDao) List(query dbmodel.AuditQuery) ([]dbmodel.OperationAudit, int64, error) {
	var list []dbmodel.OperationAudit
	var total int64

	db := common.GetDB().Model(&dbmodel.OperationAudit{})
	if query.AssetID > 0 {
		db = db.Where("asset_id = ?", query.AssetID)
	}
	if query.Engine > 0 {
		db = db.Where("engine = ?", query.Engine)
	}
	if value := strings.TrimSpace(query.DatabaseName); value != "" {
		db = db.Where("database_name = ?", value)
	}
	if value := strings.TrimSpace(query.ObjectName); value != "" {
		db = db.Where("object_name LIKE ?", "%"+value+"%")
	}
	if value := strings.TrimSpace(query.Action); value != "" {
		db = db.Where("action = ?", value)
	}
	if query.Success != nil {
		db = db.Where("success = ?", *query.Success)
	}
	if value := strings.TrimSpace(query.OperatorName); value != "" {
		db = db.Where("operator_name LIKE ?", "%"+value+"%")
	}
	if value := strings.TrimSpace(query.BeginTime); value != "" {
		db = db.Where("created_at >= ?", value)
	}
	if value := strings.TrimSpace(query.EndTime); value != "" {
		db = db.Where("created_at <= ?", value)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := dbmodel.NormalizePage(query.Page, query.PageSize)
	err := db.Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&list).Error
	return list, total, err
}
