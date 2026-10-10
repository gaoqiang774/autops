package dao

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common"
	"strings"

	"gorm.io/gorm"
)

type CmdbIdcAssetDao struct {
	db *gorm.DB
}

func NewCmdbIdcAssetDao() CmdbIdcAssetDao {
	return CmdbIdcAssetDao{
		db: common.GetDB(),
	}
}

func (d *CmdbIdcAssetDao) CreateAsset(asset *model.CmdbIdcAsset) error {
	return d.db.Create(asset).Error
}

func (d *CmdbIdcAssetDao) UpdateAsset(id uint, asset *model.CmdbIdcAsset) error {
	return d.db.Model(&model.CmdbIdcAsset{}).Where("id = ?", id).Updates(asset).Error
}

func (d *CmdbIdcAssetDao) DeleteAsset(id uint) error {
	return d.db.Delete(&model.CmdbIdcAsset{}, id).Error
}

func (d *CmdbIdcAssetDao) GetAssetById(id uint) (model.CmdbIdcAsset, error) {
	var asset model.CmdbIdcAsset
	err := d.db.Preload("Room").Preload("Cabinet").Where("id = ?", id).First(&asset).Error
	return asset, err
}

func (d *CmdbIdcAssetDao) GetAssetByIP(ip string) (*model.CmdbIdcAsset, error) {
	var asset model.CmdbIdcAsset
	err := d.db.Where("ip = ?", strings.TrimSpace(ip)).First(&asset).Error
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

func (d *CmdbIdcAssetDao) GetAssetByRemoteManagementCard(card string) (*model.CmdbIdcAsset, error) {
	var asset model.CmdbIdcAsset
	err := d.db.Preload("Room").Preload("Cabinet").Where("remote_management_card = ?", strings.TrimSpace(card)).First(&asset).Error
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

func (d *CmdbIdcAssetDao) ListAssetsByCabinetID(cabinetID uint) ([]model.CmdbIdcAsset, error) {
	var list []model.CmdbIdcAsset
	err := d.db.Preload("Room").
		Preload("Cabinet").
		Where("cabinet_id = ?", cabinetID).
		Order("cabinet_position asc, id asc").
		Find(&list).Error
	return list, err
}

func (d *CmdbIdcAssetDao) ListAssetsByRoomID(roomID uint) ([]model.CmdbIdcAsset, error) {
	var list []model.CmdbIdcAsset
	err := d.db.Preload("Room").
		Preload("Cabinet").
		Where("room_id = ?", roomID).
		Order("cabinet_id asc, cabinet_position asc, id asc").
		Find(&list).Error
	return list, err
}

func (d *CmdbIdcAssetDao) HasCabinetPositionConflict(cabinetID uint, startU, endU int, excludeID uint) (bool, error) {
	if cabinetID == 0 || startU <= 0 || endU < startU {
		return false, nil
	}

	var count int64
	db := d.db.Model(&model.CmdbIdcAsset{}).
		Where("cabinet_id = ?", cabinetID).
		Where("cabinet_position > 0").
		Where("cabinet_position <= ?", endU).
		Where("(cabinet_position + IF(cabinet_u_size <= 0, 1, cabinet_u_size) - 1) >= ?", startU)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}

	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *CmdbIdcAssetDao) GetAssetList(query *model.CmdbIdcAssetQuery) ([]model.CmdbIdcAsset, int64, error) {
	var list []model.CmdbIdcAsset
	var total int64

	fieldMap := map[string]string{
		"id":               "id",
		"createTime":       "create_time",
		"create_time":      "create_time",
		"updateTime":       "update_time",
		"update_time":      "update_time",
		"purchaseDate":     "purchase_date",
		"purchase_date":    "purchase_date",
		"usageYears":       "usage_years",
		"usage_years":      "usage_years",
		"cabinetPosition":  "cabinet_position",
		"cabinet_position": "cabinet_position",
		"ip":               "ip",
	}

	dbField, exists := fieldMap[query.SortBy]
	if !exists {
		dbField = "id"
	}
	sortOrder := strings.ToLower(strings.TrimSpace(query.SortOrder))
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	db := d.db.Model(&model.CmdbIdcAsset{})
	if query.RoomID > 0 {
		db = db.Where("room_id = ?", query.RoomID)
	}
	if query.CabinetID > 0 {
		db = db.Where("cabinet_id = ?", query.CabinetID)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where(
			"ip LIKE ? OR brand LIKE ? OR model LIKE ? OR purpose LIKE ? OR remote_management_card LIKE ? OR remark LIKE ?",
			like, like, like, like, like, like,
		)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Preload("Room").Preload("Cabinet").
		Order(dbField + " " + sortOrder).
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (d *CmdbIdcAssetDao) CountAssetsByRoomID(roomID uint) (int64, error) {
	var count int64
	err := d.db.Model(&model.CmdbIdcAsset{}).Where("room_id = ?", roomID).Count(&count).Error
	return count, err
}

func (d *CmdbIdcAssetDao) CountAssetsByCabinetID(cabinetID uint) (int64, error) {
	var count int64
	err := d.db.Model(&model.CmdbIdcAsset{}).Where("cabinet_id = ?", cabinetID).Count(&count).Error
	return count, err
}

func (d *CmdbIdcAssetDao) GetRoomAndCabinetAssetCount() (map[uint]int64, map[uint]int64, error) {
	type roomCounter struct {
		RoomID uint  `gorm:"column:room_id"`
		Count  int64 `gorm:"column:count"`
	}
	type cabinetCounter struct {
		CabinetID uint  `gorm:"column:cabinet_id"`
		Count     int64 `gorm:"column:count"`
	}

	var roomCounters []roomCounter
	var cabinetCounters []cabinetCounter

	if err := d.db.Model(&model.CmdbIdcAsset{}).
		Select("room_id, count(*) as count").
		Group("room_id").
		Find(&roomCounters).Error; err != nil {
		return nil, nil, err
	}

	if err := d.db.Model(&model.CmdbIdcAsset{}).
		Select("cabinet_id, count(*) as count").
		Where("cabinet_id > 0").
		Group("cabinet_id").
		Find(&cabinetCounters).Error; err != nil {
		return nil, nil, err
	}

	roomCountMap := make(map[uint]int64, len(roomCounters))
	cabinetCountMap := make(map[uint]int64, len(cabinetCounters))

	for _, item := range roomCounters {
		roomCountMap[item.RoomID] = item.Count
	}
	for _, item := range cabinetCounters {
		cabinetCountMap[item.CabinetID] = item.Count
	}
	return roomCountMap, cabinetCountMap, nil
}
