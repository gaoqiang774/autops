package db

import (
	"log"
	"time"

	cmdbmodel "dodevops-api/api/cmdb/model"
	"dodevops-api/common/util"

	"gorm.io/gorm"
)

// ensureDefaultCmdbGroup reconciles the default root group on every startup.
func ensureDefaultCmdbGroup(db *gorm.DB) error {
	if !db.Migrator().HasTable("cmdb_group") {
		return nil
	}

	now := util.HTime{Time: time.Now()}
	group := cmdbmodel.CmdbGroup{
		ParentID: 0,
		Name:     cmdbmodel.DefaultCmdbGroupName,
	}
	result := db.Where("parent_id = ? AND name = ?", 0, cmdbmodel.DefaultCmdbGroupName).
		Attrs(cmdbmodel.CmdbGroup{
			CreateTime: now,
			UpdateTime: now,
		}).FirstOrCreate(&group)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		log.Printf("AutoMigrate created default CMDB group: %s (ID: %d)", group.Name, group.ID)
	}
	return nil
}
