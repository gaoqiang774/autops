package dao

import (
	"dodevops-api/api/k8s/model"

	"gorm.io/gorm"
)

type InspectionDao struct {
	DB *gorm.DB
}

func NewInspectionDao(db *gorm.DB) *InspectionDao {
	return &InspectionDao{DB: db}
}

func (d *InspectionDao) Create(inspection *model.ClusterInspection) error {
	return d.DB.Create(inspection).Error
}

func (d *InspectionDao) GetByID(id uint) (*model.ClusterInspection, error) {
	var inspection model.ClusterInspection
	err := d.DB.First(&inspection, id).Error
	return &inspection, err
}

func (d *InspectionDao) Save(inspection *model.ClusterInspection) error {
	return d.DB.Save(inspection).Error
}

func (d *InspectionDao) UpdateProgress(id uint, progress int, step string) error {
	return d.DB.Model(&model.ClusterInspection{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"progress":     progress,
			"current_step": step,
		}).Error
}

func (d *InspectionDao) List(clusterID uint, page, pageSize int) ([]model.ClusterInspection, int64, error) {
	var total int64
	query := d.DB.Model(&model.ClusterInspection{})
	if clusterID > 0 {
		query = query.Where("cluster_id = ?", clusterID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var inspections []model.ClusterInspection
	err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&inspections).Error
	return inspections, total, err
}

func (d *InspectionDao) Delete(id uint) error {
	return d.DB.Delete(&model.ClusterInspection{}, id).Error
}
