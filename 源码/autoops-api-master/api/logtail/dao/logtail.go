package dao

import (
	"fmt"
	"log"
	"strings"
	"sync"

	logtailmodel "dodevops-api/api/logtail/model"
	"dodevops-api/common"

	"gorm.io/gorm"
)

type LogProjectDAO struct{}
type LogStoreDAO struct{}
type LogParseTemplateDAO struct{}
type LogCollectTaskDAO struct{}
type LogAlertRuleDAO struct{}
type LogAlertEventDAO struct{}

var (
	logCollectTaskSchemaOnce sync.Once
	logCollectTaskSchemaErr  error
)

func NewLogProjectDAO() *LogProjectDAO {
	return &LogProjectDAO{}
}

func NewLogStoreDAO() *LogStoreDAO {
	return &LogStoreDAO{}
}

func NewLogParseTemplateDAO() *LogParseTemplateDAO {
	return &LogParseTemplateDAO{}
}

func NewLogCollectTaskDAO() *LogCollectTaskDAO {
	return &LogCollectTaskDAO{}
}

func NewLogAlertRuleDAO() *LogAlertRuleDAO {
	return &LogAlertRuleDAO{}
}

func NewLogAlertEventDAO() *LogAlertEventDAO {
	return &LogAlertEventDAO{}
}

func (d *LogProjectDAO) Create(item *logtailmodel.LogProject) error {
	return common.GetDB().Create(item).Error
}

func (d *LogProjectDAO) Update(item *logtailmodel.LogProject) error {
	return common.GetDB().Save(item).Error
}

func (d *LogProjectDAO) Delete(id uint) error {
	return common.GetDB().Delete(&logtailmodel.LogProject{}, id).Error
}

func (d *LogProjectDAO) GetByID(id uint) (*logtailmodel.LogProject, error) {
	var item logtailmodel.LogProject
	err := common.GetDB().First(&item, id).Error
	return &item, err
}

func (d *LogProjectDAO) List(query logtailmodel.LogProjectQuery) ([]logtailmodel.LogProject, int64, error) {
	var items []logtailmodel.LogProject
	var total int64
	db := applyProjectFilters(common.GetDB().Model(&logtailmodel.LogProject{}), query)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("updated_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&items).Error
	return items, total, err
}

func (d *LogProjectDAO) ListByIDs(query logtailmodel.LogProjectQuery, ids []uint) ([]logtailmodel.LogProject, int64, error) {
	var items []logtailmodel.LogProject
	var total int64
	if len(ids) == 0 {
		return items, 0, nil
	}

	db := applyProjectFilters(common.GetDB().Model(&logtailmodel.LogProject{}), query).Where("id IN ?", ids)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("updated_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&items).Error
	return items, total, err
}

func (d *LogStoreDAO) Create(item *logtailmodel.LogStore) error {
	return common.GetDB().Create(item).Error
}

func (d *LogStoreDAO) Update(item *logtailmodel.LogStore) error {
	return common.GetDB().Save(item).Error
}

func (d *LogStoreDAO) Delete(id uint) error {
	return common.GetDB().Delete(&logtailmodel.LogStore{}, id).Error
}

func (d *LogStoreDAO) GetByID(id uint) (*logtailmodel.LogStore, error) {
	var item logtailmodel.LogStore
	err := common.GetDB().First(&item, id).Error
	return &item, err
}

func (d *LogStoreDAO) List(query logtailmodel.LogStoreQuery) ([]logtailmodel.LogStore, int64, error) {
	var items []logtailmodel.LogStore
	var total int64
	db := applyStoreFilters(common.GetDB().Model(&logtailmodel.LogStore{}), query)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("sort ASC, updated_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&items).Error
	return items, total, err
}

func (d *LogStoreDAO) ListByIDs(query logtailmodel.LogStoreQuery, ids []uint) ([]logtailmodel.LogStore, int64, error) {
	var items []logtailmodel.LogStore
	var total int64
	if len(ids) == 0 {
		return items, 0, nil
	}

	db := applyStoreFilters(common.GetDB().Model(&logtailmodel.LogStore{}), query).Where("id IN ?", ids)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("sort ASC, updated_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&items).Error
	return items, total, err
}

func (d *LogStoreDAO) ListIDsByProjectIDs(projectIDs []uint) ([]uint, error) {
	var ids []uint
	if len(projectIDs) == 0 {
		return ids, nil
	}
	err := common.GetDB().Model(&logtailmodel.LogStore{}).
		Where("project_id IN ?", projectIDs).
		Pluck("id", &ids).Error
	return ids, err
}

func (d *LogStoreDAO) CountByProject(projectIDs []uint) ([]logtailmodel.LogProjectStoreCount, error) {
	var items []logtailmodel.LogProjectStoreCount
	db := common.GetDB().Model(&logtailmodel.LogStore{})
	if len(projectIDs) > 0 {
		db = db.Where("project_id IN ?", projectIDs)
	}
	err := db.Select(
		"project_id, COUNT(*) AS store_count, SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS enabled_store_count",
		logtailmodel.LogStatusEnabled,
	).Group("project_id").Scan(&items).Error
	return items, err
}

func (d *LogParseTemplateDAO) Create(item *logtailmodel.LogParseTemplate) error {
	return common.GetDB().Create(item).Error
}

func (d *LogParseTemplateDAO) Update(item *logtailmodel.LogParseTemplate) error {
	return common.GetDB().Save(item).Error
}

func (d *LogParseTemplateDAO) Delete(id uint) error {
	return common.GetDB().Delete(&logtailmodel.LogParseTemplate{}, id).Error
}

func (d *LogParseTemplateDAO) GetByID(id uint) (*logtailmodel.LogParseTemplate, error) {
	var item logtailmodel.LogParseTemplate
	err := common.GetDB().First(&item, id).Error
	return &item, err
}

func (d *LogParseTemplateDAO) List(query logtailmodel.LogParseTemplateQuery) ([]logtailmodel.LogParseTemplate, int64, error) {
	var items []logtailmodel.LogParseTemplate
	var total int64
	db := applyTemplateFilters(common.GetDB().Model(&logtailmodel.LogParseTemplate{}), query)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("updated_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&items).Error
	return items, total, err
}

func (d *LogCollectTaskDAO) Create(item *logtailmodel.LogCollectTask) error {
	if err := ensureLogCollectTaskSchema(); err != nil {
		return err
	}
	return common.GetDB().Create(item).Error
}

func (d *LogCollectTaskDAO) Update(item *logtailmodel.LogCollectTask) error {
	if err := ensureLogCollectTaskSchema(); err != nil {
		return err
	}
	return common.GetDB().Save(item).Error
}

func (d *LogCollectTaskDAO) Delete(id uint) error {
	if err := ensureLogCollectTaskSchema(); err != nil {
		return err
	}
	return common.GetDB().Delete(&logtailmodel.LogCollectTask{}, id).Error
}

func (d *LogCollectTaskDAO) GetByID(id uint) (*logtailmodel.LogCollectTask, error) {
	if err := ensureLogCollectTaskSchema(); err != nil {
		return nil, err
	}
	var item logtailmodel.LogCollectTask
	err := common.GetDB().First(&item, id).Error
	return &item, err
}

func (d *LogCollectTaskDAO) ListForDedup(projectID, excludeID uint) ([]logtailmodel.LogCollectTask, error) {
	if err := ensureLogCollectTaskSchema(); err != nil {
		return nil, err
	}
	var items []logtailmodel.LogCollectTask
	db := common.GetDB().Where("project_id = ?", projectID)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	err := db.Order("id ASC").Find(&items).Error
	return items, err
}

func (d *LogCollectTaskDAO) List(query logtailmodel.LogCollectTaskQuery) ([]logtailmodel.LogCollectTask, int64, error) {
	if err := ensureLogCollectTaskSchema(); err != nil {
		return nil, 0, err
	}
	var items []logtailmodel.LogCollectTask
	var total int64
	db := applyCollectTaskFilters(common.GetDB().Model(&logtailmodel.LogCollectTask{}), query)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("updated_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&items).Error
	return items, total, err
}

func (d *LogCollectTaskDAO) ListKubernetesCollectors(query logtailmodel.LogK8SCollectorQuery) ([]logtailmodel.LogCollectTask, error) {
	if err := ensureLogCollectTaskSchema(); err != nil {
		return nil, err
	}
	var items []logtailmodel.LogCollectTask
	db := common.GetDB().Model(&logtailmodel.LogCollectTask{}).
		Where("collect_type IN ?", []string{"kubernetes", "k8s", "kubernetes_workload", "k8s_workload"})
	if query.ProjectID > 0 {
		db = db.Where("project_id = ?", query.ProjectID)
	}
	if query.StoreID > 0 {
		db = db.Where("store_id = ?", query.StoreID)
	}
	if query.TaskID > 0 {
		db = db.Where("id = ?", query.TaskID)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR code LIKE ? OR execute_targets_json LIKE ?", like, like, like)
	}
	if source := strings.TrimSpace(query.K8SLogSource); source != "" {
		switch source {
		case logtailmodel.LogK8SLogSourceStdout, logtailmodel.LogK8SLogSourceContainerFile:
			db = db.Where("k8s_log_source IN ?", []string{source, logtailmodel.LogK8SLogSourceBoth})
		default:
			db = db.Where("k8s_log_source = ?", source)
		}
	}
	err := db.Order("updated_at DESC, id DESC").Find(&items).Error
	return items, err
}

func ensureLogCollectTaskSchema() error {
	logCollectTaskSchemaOnce.Do(func() {
		db := common.GetDB()
		if db == nil {
			logCollectTaskSchemaErr = fmt.Errorf("database not initialized")
			return
		}
		if !db.Migrator().HasTable("log_collect_task") {
			if err := db.AutoMigrate(&logtailmodel.LogCollectTask{}); err != nil {
				logCollectTaskSchemaErr = err
				return
			}
		}
		if err := ensureLogCollectTaskColumn(db, "PathPatternsJSON"); err != nil {
			logCollectTaskSchemaErr = err
			return
		}
		if err := ensureLogCollectTaskColumn(db, "SampleLog"); err != nil {
			logCollectTaskSchemaErr = err
			return
		}
		if err := ensureLogCollectTaskColumn(db, "RegexPattern"); err != nil {
			logCollectTaskSchemaErr = err
			return
		}
		if err := ensureLogCollectTaskColumn(db, "NameKey"); err != nil {
			logCollectTaskSchemaErr = err
			return
		}
		if err := ensureLogCollectTaskColumn(db, "ConfigKey"); err != nil {
			logCollectTaskSchemaErr = err
			return
		}
		if err := ensureLogCollectTaskColumn(db, "K8SLogSource"); err != nil {
			logCollectTaskSchemaErr = err
			return
		}
		for _, indexName := range []string{"uk_log_collect_task_name_key", "uk_log_collect_task_config_key"} {
			if db.Migrator().HasIndex(&logtailmodel.LogCollectTask{}, indexName) {
				continue
			}
			if err := db.Migrator().CreateIndex(&logtailmodel.LogCollectTask{}, indexName); err != nil {
				logCollectTaskSchemaErr = fmt.Errorf("create log collect task index %s: %w", indexName, err)
				return
			}
		}
	})
	return logCollectTaskSchemaErr
}

func ensureLogCollectTaskColumn(db *gorm.DB, columnName string) error {
	if db.Migrator().HasColumn(&logtailmodel.LogCollectTask{}, columnName) {
		return nil
	}
	log.Printf("Logtail collect task schema adding missing column: log_collect_task.%s", columnName)
	return db.Migrator().AddColumn(&logtailmodel.LogCollectTask{}, columnName)
}

func (d *LogAlertRuleDAO) Create(item *logtailmodel.LogAlertRule) error {
	return common.GetDB().Create(item).Error
}

func (d *LogAlertRuleDAO) Update(item *logtailmodel.LogAlertRule) error {
	return common.GetDB().Save(item).Error
}

func (d *LogAlertRuleDAO) Delete(id uint) error {
	return common.GetDB().Delete(&logtailmodel.LogAlertRule{}, id).Error
}

func (d *LogAlertRuleDAO) GetByID(id uint) (*logtailmodel.LogAlertRule, error) {
	var item logtailmodel.LogAlertRule
	err := common.GetDB().First(&item, id).Error
	return &item, err
}

func (d *LogAlertRuleDAO) List(query logtailmodel.LogAlertRuleQuery) ([]logtailmodel.LogAlertRule, int64, error) {
	var items []logtailmodel.LogAlertRule
	var total int64
	db := applyAlertRuleFilters(common.GetDB().Model(&logtailmodel.LogAlertRule{}), query)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("updated_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&items).Error
	return items, total, err
}

func (d *LogAlertEventDAO) Create(item *logtailmodel.LogAlertEvent) error {
	return common.GetDB().Create(item).Error
}

func (d *LogAlertEventDAO) Update(item *logtailmodel.LogAlertEvent) error {
	return common.GetDB().Save(item).Error
}

func (d *LogAlertEventDAO) Delete(id uint) error {
	return common.GetDB().Delete(&logtailmodel.LogAlertEvent{}, id).Error
}

func (d *LogAlertEventDAO) GetByID(id uint) (*logtailmodel.LogAlertEvent, error) {
	var item logtailmodel.LogAlertEvent
	err := common.GetDB().First(&item, id).Error
	return &item, err
}

func (d *LogAlertEventDAO) List(query logtailmodel.LogAlertEventQuery) ([]logtailmodel.LogAlertEvent, int64, error) {
	var items []logtailmodel.LogAlertEvent
	var total int64
	db := applyAlertEventFilters(common.GetDB().Model(&logtailmodel.LogAlertEvent{}), query)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("fired_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&items).Error
	return items, total, err
}

func applyProjectFilters(db *gorm.DB, query logtailmodel.LogProjectQuery) *gorm.DB {
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR code LIKE ? OR owner_name LIKE ?", like, like, like)
	}
	if query.Status > 0 {
		db = db.Where("status = ?", query.Status)
	}
	return db
}

func applyStoreFilters(db *gorm.DB, query logtailmodel.LogStoreQuery) *gorm.DB {
	if query.ProjectID > 0 {
		db = db.Where("project_id = ?", query.ProjectID)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR code LIKE ? OR description LIKE ?", like, like, like)
	}
	if query.Status > 0 {
		db = db.Where("status = ?", query.Status)
	}
	return db
}

func applyTemplateFilters(db *gorm.DB, query logtailmodel.LogParseTemplateQuery) *gorm.DB {
	if query.ProjectID > 0 {
		db = db.Where("project_id = ?", query.ProjectID)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR code LIKE ? OR remark LIKE ?", like, like, like)
	}
	if mode := strings.TrimSpace(query.ParseMode); mode != "" {
		db = db.Where("parse_mode = ?", mode)
	}
	if query.Status > 0 {
		db = db.Where("status = ?", query.Status)
	}
	return db
}

func applyCollectTaskFilters(db *gorm.DB, query logtailmodel.LogCollectTaskQuery) *gorm.DB {
	if query.ProjectID > 0 {
		db = db.Where("project_id = ?", query.ProjectID)
	}
	if query.StoreID > 0 {
		db = db.Where("store_id = ?", query.StoreID)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR code LIKE ? OR path_pattern LIKE ?", like, like, like)
	}
	if collectType := strings.TrimSpace(query.CollectType); collectType != "" {
		db = db.Where("collect_type = ?", collectType)
	}
	if query.Status > 0 {
		db = db.Where("status = ?", query.Status)
	}
	return db
}

func applyAlertRuleFilters(db *gorm.DB, query logtailmodel.LogAlertRuleQuery) *gorm.DB {
	if query.ProjectID > 0 {
		db = db.Where("project_id = ?", query.ProjectID)
	}
	if query.StoreID > 0 {
		db = db.Where("store_id = ?", query.StoreID)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR code LIKE ? OR remark LIKE ?", like, like, like)
	}
	if severity := strings.TrimSpace(query.Severity); severity != "" {
		db = db.Where("severity = ?", severity)
	}
	if query.Status > 0 {
		db = db.Where("status = ?", query.Status)
	}
	return db
}

func applyAlertEventFilters(db *gorm.DB, query logtailmodel.LogAlertEventQuery) *gorm.DB {
	if query.ProjectID > 0 {
		db = db.Where("project_id = ?", query.ProjectID)
	}
	if query.StoreID > 0 {
		db = db.Where("store_id = ?", query.StoreID)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("source_name LIKE ? OR trigger_reason LIKE ? OR summary LIKE ?", like, like, like)
	}
	if severity := strings.TrimSpace(query.Severity); severity != "" {
		db = db.Where("severity = ?", severity)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		db = db.Where("status = ?", status)
	}
	return db
}
