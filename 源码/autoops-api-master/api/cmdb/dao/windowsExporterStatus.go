package dao

import (
	"errors"
	"time"

	"dodevops-api/api/cmdb/model"
	"dodevops-api/common"

	"gorm.io/gorm"
)

type CmdbWindowsExporterStatusDao struct {
	db *gorm.DB
}

func NewCmdbWindowsExporterStatusDao() *CmdbWindowsExporterStatusDao {
	return &CmdbWindowsExporterStatusDao{db: common.GetDB()}
}

func (d *CmdbWindowsExporterStatusDao) GetByHostID(hostID uint) (*model.CmdbWindowsExporterStatus, error) {
	var item model.CmdbWindowsExporterStatus
	err := d.db.Where("host_id = ?", hostID).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (d *CmdbWindowsExporterStatusDao) Save(hostID uint, fields map[string]interface{}) error {
	if hostID == 0 {
		return errors.New("host id is required")
	}
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["updated_at"] = time.Now()
	item := model.CmdbWindowsExporterStatus{HostID: hostID}
	return d.db.Where("host_id = ?", hostID).Assign(fields).FirstOrCreate(&item).Error
}
