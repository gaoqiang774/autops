package dao

import (
	"dodevops-api/api/cmdb/model"

	"gorm.io/gorm"
)

type MonitorDatasourceDAO struct {
	DB *gorm.DB
}

func NewMonitorDatasourceDAO(db *gorm.DB) *MonitorDatasourceDAO {
	return &MonitorDatasourceDAO{DB: db}
}

func (d *MonitorDatasourceDAO) Create(res *model.CmdbBusinessModelBinding) error {
	return d.DB.Create(res).Error
}

func (d *MonitorDatasourceDAO) Update(id uint, updates map[string]interface{}) error {
	return d.DB.Model(&model.CmdbBusinessModelBinding{}).Where("id = ?", id).Updates(updates).Error
}

func (d *MonitorDatasourceDAO) Delete(id uint) error {
	return d.DB.Where("id = ?", id).Delete(&model.CmdbBusinessModelBinding{}).Error
}

func (d *MonitorDatasourceDAO) GetByID(id uint) (*model.CmdbBusinessModelBinding, error) {
	var res model.CmdbBusinessModelBinding
	err := d.DB.Where("id = ?", id).First(&res).Error
	return &res, err
}

func (d *MonitorDatasourceDAO) List(resourceType, status string, page, pageSize int) ([]model.CmdbBusinessModelBinding, int64, error) {
	var list []model.CmdbBusinessModelBinding
	var total int64

	query := d.DB.Model(&model.CmdbBusinessModelBinding{})
	if resourceType != "" {
		query = query.Where("resource_type = ?", resourceType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *MonitorDatasourceDAO) GetActivePrometheusResources() ([]model.CmdbBusinessModelBinding, error) {
	var list []model.CmdbBusinessModelBinding
	err := d.DB.Where("resource_type = ? AND status = ?", model.ResourceTypePrometheus, "active").Find(&list).Error
	return list, err
}
