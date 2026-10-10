package dao

import (
	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"
	"dodevops-api/common"
	"strings"

	"gorm.io/gorm"
)

type AssetDao struct{}

func NewAssetDao() *AssetDao {
	return &AssetDao{}
}

func (d *AssetDao) GetByID(id uint) (*cmdbmodel.CmdbSQL, error) {
	var asset cmdbmodel.CmdbSQL
	err := common.GetDB().First(&asset, id).Error
	return &asset, err
}

func (d *AssetDao) List(query dbmodel.AssetQuery) ([]cmdbmodel.CmdbSQL, int64, error) {
	var assets []cmdbmodel.CmdbSQL
	var total int64

	db := common.GetDB().Model(&cmdbmodel.CmdbSQL{})
	db = applyAssetFilters(db, query)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := dbmodel.NormalizePage(query.Page, query.PageSize)
	err := db.Order("COALESCE(updated_at, created_at) DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&assets).Error
	return assets, total, err
}

func applyAssetFilters(db *gorm.DB, query dbmodel.AssetQuery) *gorm.DB {
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where(
			"name LIKE ? OR instance_name LIKE ? OR host_ip LIKE ? OR endpoint LIKE ? OR business_system LIKE ? OR owner LIKE ?",
			like, like, like, like, like, like,
		)
	}
	if query.Type > 0 {
		db = db.Where("type = ?", query.Type)
	}
	if environment := strings.TrimSpace(query.Environment); environment != "" {
		db = db.Where("environment = ?", environment)
	}
	if query.Status > 0 {
		db = db.Where("status = ?", query.Status)
	}
	if query.AuthStatus > 0 {
		db = db.Where("auth_status = ?", query.AuthStatus)
	}
	if businessSystem := strings.TrimSpace(query.BusinessSystem); businessSystem != "" {
		db = db.Where("business_system = ?", businessSystem)
	}
	if owner := strings.TrimSpace(query.Owner); owner != "" {
		db = db.Where("owner = ?", owner)
	}
	return db
}
