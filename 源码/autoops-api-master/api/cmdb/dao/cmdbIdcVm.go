package dao

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common"
	"strings"

	"gorm.io/gorm"
)

type CmdbIdcVmDao struct {
	db *gorm.DB
}

func NewCmdbIdcVmDao() CmdbIdcVmDao {
	return CmdbIdcVmDao{
		db: common.GetDB(),
	}
}

func (d *CmdbIdcVmDao) CreateVm(vm *model.CmdbIdcVm) error {
	return d.db.Create(vm).Error
}

func (d *CmdbIdcVmDao) UpdateVm(id uint, vm *model.CmdbIdcVm) error {
	return d.db.Model(&model.CmdbIdcVm{}).Where("id = ?", id).Updates(vm).Error
}

func (d *CmdbIdcVmDao) DeleteVm(id uint) error {
	return d.db.Delete(&model.CmdbIdcVm{}, id).Error
}

func (d *CmdbIdcVmDao) GetVmById(id uint) (model.CmdbIdcVm, error) {
	var vm model.CmdbIdcVm
	err := d.db.Preload("PhysicalAsset").Where("id = ?", id).First(&vm).Error
	return vm, err
}

func (d *CmdbIdcVmDao) GetVmByPhysicalAssetAndIP(physicalAssetID uint, vmIP string, excludeID uint) (*model.CmdbIdcVm, error) {
	var vm model.CmdbIdcVm
	db := d.db.Where("physical_asset_id = ? AND vm_ip = ?", physicalAssetID, strings.TrimSpace(vmIP))
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	err := db.First(&vm).Error
	if err != nil {
		return nil, err
	}
	return &vm, nil
}

func (d *CmdbIdcVmDao) GetVmList(query *model.CmdbIdcVmQuery) ([]model.CmdbIdcVm, int64, error) {
	var list []model.CmdbIdcVm
	var total int64

	fieldMap := map[string]string{
		"id":          "cmdb_idc_vm.id",
		"createTime":  "cmdb_idc_vm.create_time",
		"create_time": "cmdb_idc_vm.create_time",
		"updateTime":  "cmdb_idc_vm.update_time",
		"update_time": "cmdb_idc_vm.update_time",
		"vmIp":        "cmdb_idc_vm.vm_ip",
		"vm_ip":       "cmdb_idc_vm.vm_ip",
		"vmName":      "cmdb_idc_vm.vm_name",
		"vm_name":     "cmdb_idc_vm.vm_name",
		"username":    "cmdb_idc_vm.username",
		"physicalIp":  "asset.ip",
		"physical_ip": "asset.ip",
	}

	dbField, exists := fieldMap[query.SortBy]
	if !exists {
		dbField = "cmdb_idc_vm.id"
	}

	sortOrder := strings.ToLower(strings.TrimSpace(query.SortOrder))
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	db := d.db.Model(&model.CmdbIdcVm{}).
		Joins("LEFT JOIN cmdb_idc_asset asset ON asset.id = cmdb_idc_vm.physical_asset_id")

	if query.PhysicalAssetID > 0 {
		db = db.Where("cmdb_idc_vm.physical_asset_id = ?", query.PhysicalAssetID)
	}
	if physicalIP := strings.TrimSpace(query.PhysicalIP); physicalIP != "" {
		db = db.Where("asset.ip = ?", physicalIP)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where(
			"asset.ip LIKE ? OR cmdb_idc_vm.vm_ip LIKE ? OR cmdb_idc_vm.vm_name LIKE ? OR cmdb_idc_vm.username LIKE ? OR cmdb_idc_vm.remark LIKE ?",
			like, like, like, like, like,
		)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Preload("PhysicalAsset").
		Order(dbField + " " + sortOrder).
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

func (d *CmdbIdcVmDao) CountByPhysicalAssetID(physicalAssetID uint) (int64, error) {
	var count int64
	err := d.db.Model(&model.CmdbIdcVm{}).Where("physical_asset_id = ?", physicalAssetID).Count(&count).Error
	return count, err
}

func (d *CmdbIdcVmDao) CountByPhysicalAssetIDs(physicalAssetIDs []uint) (map[uint]int64, error) {
	if len(physicalAssetIDs) == 0 {
		return map[uint]int64{}, nil
	}

	type counter struct {
		PhysicalAssetID uint  `gorm:"column:physical_asset_id"`
		Count           int64 `gorm:"column:count"`
	}

	var counters []counter
	if err := d.db.Model(&model.CmdbIdcVm{}).
		Select("physical_asset_id, count(*) as count").
		Where("physical_asset_id IN ?", physicalAssetIDs).
		Group("physical_asset_id").
		Find(&counters).Error; err != nil {
		return nil, err
	}

	countMap := make(map[uint]int64, len(counters))
	for _, item := range counters {
		countMap[item.PhysicalAssetID] = item.Count
	}
	return countMap, nil
}
