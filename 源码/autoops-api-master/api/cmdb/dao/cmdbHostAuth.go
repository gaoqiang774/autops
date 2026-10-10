package dao

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common"
	"dodevops-api/common/util"
	"time"

	"gorm.io/gorm"
)

type CmdbHostAuthDao struct {
	db *gorm.DB
}

func NewCmdbHostAuthDao() CmdbHostAuthDao {
	return CmdbHostAuthDao{db: common.GetDB()}
}

func (d CmdbHostAuthDao) ReplaceHostAuths(hostID uint, authIDs []uint, defaultAuthID uint) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("host_id = ?", hostID).Delete(&model.CmdbHostAuth{}).Error; err != nil {
			return err
		}
		if len(authIDs) == 0 {
			return nil
		}

		now := util.HTime{Time: time.Now()}
		rows := make([]model.CmdbHostAuth, 0, len(authIDs))
		for _, authID := range authIDs {
			row := model.CmdbHostAuth{
				HostID:     hostID,
				AuthID:     authID,
				CreateTime: now,
				UpdateTime: now,
			}
			if authID == defaultAuthID {
				row.IsDefault = 1
			}
			rows = append(rows, row)
		}
		return tx.Create(&rows).Error
	})
}

func (d CmdbHostAuthDao) GetHostAuths(hostID uint) ([]model.CmdbHostAuth, error) {
	var rows []model.CmdbHostAuth
	err := d.db.Where("host_id = ?", hostID).
		Order("is_default DESC, id ASC").
		Find(&rows).Error
	return rows, err
}

func (d CmdbHostAuthDao) DeleteByHostID(hostID uint) error {
	return d.db.Where("host_id = ?", hostID).Delete(&model.CmdbHostAuth{}).Error
}
