package db

import (
	"log"
	"time"

	aiopsmodel "dodevops-api/api/aiops/model"
	"dodevops-api/common/util"

	"gorm.io/gorm"
)

const (
	defaultKnowledgeGroupSeedKey = "default_knowledge_groups_v1"
	defaultKnowledgeRootName     = "运维知识库"
)

type defaultKnowledgeGroupSpec struct {
	Name        string
	Description string
	SortOrder   int
}

func defaultKnowledgeGroupSpecs() []defaultKnowledgeGroupSpec {
	return []defaultKnowledgeGroupSpec{
		{Name: "01-运维管理制度", Description: "运维管理制度、规范和流程", SortOrder: 10},
		{Name: "02-部署维护记录", Description: "系统部署、变更和日常维护记录", SortOrder: 20},
		{Name: "03-运维故障记录", Description: "故障现象、排查过程、根因和复盘记录", SortOrder: 30},
		{Name: "04-中间件维护记录", Description: "中间件配置、部署、升级和故障处理记录", SortOrder: 40},
		{Name: "05-数据库管理", Description: "数据库部署、配置、备份、恢复和优化记录", SortOrder: 50},
		{Name: "06-运维开发", Description: "运维工具、脚本、平台和自动化开发记录", SortOrder: 60},
		{Name: "07-运维监控", Description: "监控指标、告警规则和可观测性建设记录", SortOrder: 70},
		{Name: "08-运维体系建设", Description: "运维流程、标准化和体系建设记录", SortOrder: 80},
		{Name: "09-运维安全管理", Description: "账号权限、审计、漏洞和安全管理记录", SortOrder: 90},
		{Name: "10-总结记录", Description: "阶段总结、经验沉淀和改进记录", SortOrder: 100},
	}
}

func ensureDefaultKnowledgeGroups(db *gorm.DB) error {
	if !db.Migrator().HasTable("aiops_knowledge_group") {
		return nil
	}

	now := util.HTime{Time: time.Now()}
	return db.Transaction(func(tx *gorm.DB) error {
		// Reconcile on every startup. A restored database may contain the seed
		// marker while the actual default group rows are missing.
		root := aiopsmodel.KnowledgeGroup{ParentID: 0, Name: defaultKnowledgeRootName}
		result := tx.Where("parent_id = ? AND name = ?", 0, defaultKnowledgeRootName).
			Attrs(aiopsmodel.KnowledgeGroup{
				Description: "运维制度、维护记录、故障经验和技术文档的统一知识库",
				SortOrder:   10,
				CreatedAt:   now,
				UpdatedAt:   now,
			}).FirstOrCreate(&root)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			log.Printf("AutoMigrate created default knowledge root group: %s", root.Name)
		}

		for _, spec := range defaultKnowledgeGroupSpecs() {
			group := aiopsmodel.KnowledgeGroup{ParentID: root.ID, Name: spec.Name}
			result := tx.Where("parent_id = ? AND name = ?", root.ID, spec.Name).
				Attrs(aiopsmodel.KnowledgeGroup{
					Description: spec.Description,
					SortOrder:   spec.SortOrder,
					CreatedAt:   now,
					UpdatedAt:   now,
				}).FirstOrCreate(&group)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				log.Printf("AutoMigrate created default knowledge group: %s", group.Name)
			}
		}

		return setStoredMigrationSignature(tx, defaultKnowledgeGroupSeedKey, "completed")
	})
}
