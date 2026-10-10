package dao

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common"
	"dodevops-api/common/util"
	"time"

	"gorm.io/gorm"
)

type CmdbHostCloudDao struct {
	db *gorm.DB
}

func NewCmdbHostCloudDao() CmdbHostCloudDao {
	return CmdbHostCloudDao{
		db: common.GetDB(),
	}
}

// 创建云主机
func (d *CmdbHostCloudDao) CreateCmdbHost(host *model.CmdbHost) error {
	now := util.HTime{Time: time.Now()}
	if host.CreateTime.Time.IsZero() {
		host.CreateTime = now
	}
	if host.UpdateTime.Time.IsZero() {
		host.UpdateTime = now
	}
	return d.db.Create(host).Error
}

// 根据ID获取云主机
func (d *CmdbHostCloudDao) GetCmdbHostById(id uint) (model.CmdbHost, error) {
	var host model.CmdbHost
	err := d.db.Where("id = ?", id).First(&host).Error
	return host, err
}
