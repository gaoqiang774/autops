package dao

import (
	"dodevops-api/api/alertmanager/model"
	"dodevops-api/common"
	"dodevops-api/common/util"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AlertManagerDao struct {
	db *gorm.DB
}

func (d *AlertManagerDao) DB() *gorm.DB {
	return d.db
}

type EventSilenceCancellation struct {
	Event          model.AlertActiveEvent
	DisabledRules  int64
	RemainingRules []model.AlertSilenceRule
}

func NewAlertManagerDao() *AlertManagerDao {
	return &AlertManagerDao{db: common.GetDB()}
}

func NewAlertManagerDaoWithDB(db *gorm.DB) *AlertManagerDao {
	return &AlertManagerDao{db: db}
}

func nowHTime() util.HTime {
	return util.HTime{Time: time.Now()}
}

func (d *AlertManagerDao) CreateRuleGroup(item *model.AlertRuleGroup) error {
	now := nowHTime()
	item.CreatedAt = now
	item.UpdatedAt = now
	return d.db.Create(item).Error
}

func (d *AlertManagerDao) UpdateRuleGroup(item *model.AlertRuleGroup) error {
	item.UpdatedAt = nowHTime()
	updates := map[string]interface{}{
		"parent_id":   item.ParentID,
		"name":        item.Name,
		"description": item.Description,
		"sort":        item.Sort,
		"enabled":     item.Enabled,
		"updated_by":  item.UpdatedBy,
		"updated_at":  item.UpdatedAt,
	}
	return d.db.Model(&model.AlertRuleGroup{}).Where("id = ?", item.ID).Updates(updates).Error
}

func (d *AlertManagerDao) DeleteRuleGroup(id uint) error {
	return d.db.Delete(&model.AlertRuleGroup{}, id).Error
}

func (d *AlertManagerDao) DeleteRuleGroupsWithRules(groupIDs []uint) (int64, int64, error) {
	if len(groupIDs) == 0 {
		return 0, 0, nil
	}
	var deletedRules int64
	var deletedGroups int64
	err := d.db.Transaction(func(tx *gorm.DB) error {
		ruleResult := tx.Where("group_id IN ?", groupIDs).Delete(&model.AlertRule{})
		if ruleResult.Error != nil {
			return ruleResult.Error
		}
		deletedRules = ruleResult.RowsAffected

		groupResult := tx.Where("id IN ?", groupIDs).Delete(&model.AlertRuleGroup{})
		if groupResult.Error != nil {
			return groupResult.Error
		}
		deletedGroups = groupResult.RowsAffected
		return nil
	})
	return deletedRules, deletedGroups, err
}

func (d *AlertManagerDao) RuleIDsByGroupIDs(groupIDs []uint) ([]uint, error) {
	var ids []uint
	if len(groupIDs) == 0 {
		return ids, nil
	}
	err := d.db.Model(&model.AlertRule{}).
		Where("group_id IN ?", groupIDs).
		Pluck("id", &ids).Error
	return ids, err
}

func (d *AlertManagerDao) GetRuleGroup(id uint) (*model.AlertRuleGroup, error) {
	var item model.AlertRuleGroup
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *AlertManagerDao) ListRuleGroups(query model.PageQuery) ([]model.AlertRuleGroup, int64, error) {
	var list []model.AlertRuleGroup
	var total int64
	db := d.db.Model(&model.AlertRuleGroup{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("sort DESC, updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *AlertManagerDao) AllRuleGroups() ([]model.AlertRuleGroup, error) {
	var list []model.AlertRuleGroup
	err := d.db.Order("sort DESC, updated_at DESC, id DESC").Find(&list).Error
	return list, err
}

func (d *AlertManagerDao) RuleGroupNameExists(name string, parentID, excludeID uint) bool {
	var count int64
	db := d.db.Model(&model.AlertRuleGroup{}).Where("name = ? AND parent_id = ?", name, parentID)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	db.Count(&count)
	return count > 0
}

func (d *AlertManagerDao) CountRulesByGroupID(groupID uint) (int64, error) {
	var count int64
	err := d.db.Model(&model.AlertRule{}).Where("group_id = ?", groupID).Count(&count).Error
	return count, err
}

func (d *AlertManagerDao) CountRulesByGroupIDs(groupIDs []uint) (map[uint]int64, error) {
	result := make(map[uint]int64, len(groupIDs))
	if len(groupIDs) == 0 {
		return result, nil
	}
	type row struct {
		GroupID uint
		Count   int64
	}
	var rows []row
	err := d.db.Model(&model.AlertRule{}).
		Select("group_id, count(*) as count").
		Where("group_id IN ?", groupIDs).
		Group("group_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, item := range rows {
		result[item.GroupID] = item.Count
	}
	return result, nil
}

func (d *AlertManagerDao) CountRulesByDatasourceID(datasourceID uint) (int64, error) {
	var count int64
	err := d.db.Model(&model.AlertRule{}).Where("datasource_id = ?", datasourceID).Count(&count).Error
	return count, err
}

func (d *AlertManagerDao) CreateRule(item *model.AlertRule) error {
	now := nowHTime()
	item.CreatedAt = now
	item.UpdatedAt = now
	return d.db.Create(item).Error
}

func (d *AlertManagerDao) UpdateRule(item *model.AlertRule) error {
	item.UpdatedAt = nowHTime()
	updates := map[string]interface{}{
		"name":                       item.Name,
		"note":                       item.Note,
		"group_id":                   item.GroupID,
		"datasource_id":              item.DatasourceID,
		"datasource_type":            item.DatasourceType,
		"datasource_queries":         item.DatasourceQueries,
		"query":                      item.Query,
		"condition_operator":         item.ConditionOperator,
		"condition_value":            item.ConditionValue,
		"rule_config":                item.RuleConfig,
		"append_tags":                item.AppendTags,
		"cron_pattern":               item.CronPattern,
		"eval_interval_seconds":      item.EvalIntervalSeconds,
		"for_duration_seconds":       item.ForDurationSeconds,
		"recover_duration_seconds":   item.RecoverDurationSeconds,
		"severity":                   item.Severity,
		"enabled":                    item.Enabled,
		"enable_in_bg":               item.EnableInBG,
		"time_zone":                  item.TimeZone,
		"labels":                     item.Labels,
		"annotations":                item.Annotations,
		"asset_scope":                item.AssetScope,
		"business_id":                item.BusinessID,
		"no_data_policy":             item.NoDataPolicy,
		"error_policy":               item.ErrorPolicy,
		"effective_time":             item.EffectiveTime,
		"notify_rule_ids":            item.NotifyRuleIDs,
		"notify_recovered":           item.NotifyRecovered,
		"notify_repeat_step_minutes": item.NotifyRepeatStepMinutes,
		"notify_max_number":          item.NotifyMaxNumber,
		"pipeline_configs":           item.PipelineConfigs,
		"runbook_url":                item.RunbookURL,
		"callbacks":                  item.Callbacks,
		"self_heal_config":           item.SelfHealConfig,
		"updated_by":                 item.UpdatedBy,
		"updated_at":                 item.UpdatedAt,
	}
	return d.db.Model(&model.AlertRule{}).Where("id = ?", item.ID).Updates(updates).Error
}

func (d *AlertManagerDao) UpdateRuleStatus(id uint, enabled int, updatedBy string) error {
	return d.db.Model(&model.AlertRule{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"enabled":    enabled,
			"updated_by": updatedBy,
			"updated_at": nowHTime(),
		}).Error
}

func (d *AlertManagerDao) BatchUpdateRuleDatasource(ruleIDs []uint, datasourceID uint, datasourceType, updatedBy string) error {
	if len(ruleIDs) == 0 {
		return nil
	}
	return d.db.Model(&model.AlertRule{}).
		Where("id IN ?", ruleIDs).
		Updates(map[string]interface{}{
			"datasource_id":   datasourceID,
			"datasource_type": datasourceType,
			"updated_by":      updatedBy,
			"updated_at":      nowHTime(),
		}).Error
}

func (d *AlertManagerDao) BatchUpdateRuleNotifyRules(ruleIDs []uint, notifyRuleIDs model.JSONUintArray, updatedBy string) error {
	if len(ruleIDs) == 0 {
		return nil
	}
	return d.db.Model(&model.AlertRule{}).
		Where("id IN ?", ruleIDs).
		Updates(map[string]interface{}{
			"notify_rule_ids": notifyRuleIDs,
			"updated_by":      updatedBy,
			"updated_at":      nowHTime(),
		}).Error
}

func (d *AlertManagerDao) CountEnabledNotifyRulesByIDs(ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var count int64
	err := d.db.Model(&model.AlertNotifyRule{}).
		Where("id IN ? AND enabled = ?", ids, model.RuleStatusEnabled).
		Count(&count).Error
	return count, err
}

func (d *AlertManagerDao) CountEnabledNotifyGroupsByIDs(ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var count int64
	err := d.db.Model(&model.AlertNotifyGroup{}).
		Where("id IN ? AND enabled = ?", ids, model.RuleStatusEnabled).
		Count(&count).Error
	return count, err
}

func (d *AlertManagerDao) DeleteRule(id uint) error {
	return d.db.Delete(&model.AlertRule{}, id).Error
}

func (d *AlertManagerDao) GetRule(id uint) (*model.AlertRule, error) {
	var item model.AlertRule
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *AlertManagerDao) RuleNotifyRuleIDsByIDs(ids []uint) (map[uint]model.JSONUintArray, error) {
	result := make(map[uint]model.JSONUintArray, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rules []model.AlertRule
	if err := d.db.Select("id, notify_rule_ids").Where("id IN ?", ids).Find(&rules).Error; err != nil {
		return nil, err
	}
	for _, rule := range rules {
		result[rule.ID] = rule.NotifyRuleIDs
	}
	return result, nil
}

func (d *AlertManagerDao) ListRules(query model.AlertRuleQuery) ([]model.AlertRule, int64, error) {
	var list []model.AlertRule
	var total int64
	db := d.db.Model(&model.AlertRule{}).Where("internal = ?", 0)
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR note LIKE ? OR query LIKE ?", like, like, like)
	}
	if query.GroupID > 0 {
		db = db.Where("group_id = ?", query.GroupID)
	}
	if query.DatasourceID > 0 {
		db = db.Where("datasource_id = ?", query.DatasourceID)
	}
	if query.DatasourceType != "" {
		db = db.Where("datasource_type = ?", query.DatasourceType)
	}
	if query.Severity != "" {
		db = db.Where("severity = ?", query.Severity)
	}
	if query.Enabled != "" {
		db = db.Where("enabled = ?", query.Enabled)
	}
	if query.BusinessID != "" {
		db = db.Where("business_id = ?", query.BusinessID)
	}
	if query.EnableInBG != "" {
		db = db.Where("enable_in_bg = ?", query.EnableInBG)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *AlertManagerDao) RuleNameExists(name string, excludeID uint) bool {
	var count int64
	db := d.db.Model(&model.AlertRule{}).Where("name = ?", name)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	db.Count(&count)
	return count > 0
}

func (d *AlertManagerDao) CreateDatasource(item *model.AlertDatasource) error {
	now := nowHTime()
	item.CreatedAt = now
	item.UpdatedAt = now
	return d.db.Create(item).Error
}

func (d *AlertManagerDao) UpdateDatasource(item *model.AlertDatasource) error {
	item.UpdatedAt = nowHTime()
	updates := map[string]interface{}{
		"name":            item.Name,
		"datasource_type": item.DatasourceType,
		"url":             item.URL,
		"auth_type":       item.AuthType,
		"username":        item.Username,
		"password":        item.Password,
		"token":           item.Token,
		"config":          item.Config,
		"labels":          item.Labels,
		"description":     item.Description,
		"builtin":         item.Builtin,
		"is_default":      item.Default,
		"enabled":         item.Enabled,
		"updated_by":      item.UpdatedBy,
		"updated_at":      item.UpdatedAt,
	}
	return d.db.Model(&model.AlertDatasource{}).Where("id = ?", item.ID).Updates(updates).Error
}

func (d *AlertManagerDao) UpdateDatasourceStatus(id uint, enabled int, updatedBy string) error {
	return d.db.Model(&model.AlertDatasource{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"enabled":    enabled,
			"updated_by": updatedBy,
			"updated_at": nowHTime(),
		}).Error
}

func (d *AlertManagerDao) DeleteDatasource(id uint) error {
	return d.db.Delete(&model.AlertDatasource{}, id).Error
}

func (d *AlertManagerDao) GetDatasource(id uint) (*model.AlertDatasource, error) {
	var item model.AlertDatasource
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *AlertManagerDao) ListDatasources(query model.AlertDatasourceQuery) ([]model.AlertDatasource, int64, error) {
	var list []model.AlertDatasource
	var total int64
	db := d.db.Model(&model.AlertDatasource{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR url LIKE ? OR description LIKE ?", like, like, like)
	}
	if query.DatasourceType != "" {
		db = db.Where("datasource_type = ?", query.DatasourceType)
	}
	if query.Enabled != "" {
		db = db.Where("enabled = ?", query.Enabled)
	}
	if query.Builtin != "" {
		db = db.Where("builtin = ?", query.Builtin)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("is_default DESC, builtin DESC, updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *AlertManagerDao) AllEnabledDatasources() ([]model.AlertDatasource, error) {
	var list []model.AlertDatasource
	err := d.db.Where("enabled = ?", model.RuleStatusEnabled).
		Order("is_default DESC, builtin DESC, datasource_type ASC, name ASC, id DESC").
		Find(&list).Error
	return list, err
}

func (d *AlertManagerDao) DatasourceNameExists(name string, excludeID uint) bool {
	var count int64
	db := d.db.Model(&model.AlertDatasource{}).Where("name = ?", name)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	db.Count(&count)
	return count > 0
}

func (d *AlertManagerDao) CreateBuiltinRuleTemplate(item *model.AlertBuiltinRuleTemplate) error {
	now := nowHTime()
	item.CreatedAt = now
	item.UpdatedAt = now
	return d.db.Create(item).Error
}

func (d *AlertManagerDao) UpdateBuiltinRuleTemplate(item *model.AlertBuiltinRuleTemplate) error {
	item.UpdatedAt = nowHTime()
	updates := map[string]interface{}{
		"resource_type":                 item.ResourceType,
		"component":                     item.Component,
		"collector":                     item.Collector,
		"category":                      item.Category,
		"name":                          item.Name,
		"description":                   item.Description,
		"datasource_type":               item.DatasourceType,
		"source":                        item.Source,
		"source_uuid":                   item.SourceUUID,
		"query_template":                item.QueryTemplate,
		"default_condition_operator":    item.DefaultConditionOperator,
		"default_condition_value":       item.DefaultConditionValue,
		"default_eval_interval_seconds": item.DefaultEvalIntervalSeconds,
		"default_for_duration_seconds":  item.DefaultForDurationSeconds,
		"default_severity":              item.DefaultSeverity,
		"default_enabled":               item.DefaultEnabled,
		"labels":                        item.Labels,
		"annotations":                   item.Annotations,
		"variables":                     item.Variables,
		"tags":                          item.Tags,
		"rule_config":                   item.RuleConfig,
		"sort":                          item.Sort,
		"enabled":                       item.Enabled,
		"updated_at":                    item.UpdatedAt,
	}
	return d.db.Model(&model.AlertBuiltinRuleTemplate{}).Where("id = ?", item.ID).Updates(updates).Error
}

func (d *AlertManagerDao) DeleteBuiltinRuleTemplate(id uint) error {
	return d.db.Delete(&model.AlertBuiltinRuleTemplate{}, id).Error
}

func (d *AlertManagerDao) GetBuiltinRuleTemplate(id uint) (*model.AlertBuiltinRuleTemplate, error) {
	var item model.AlertBuiltinRuleTemplate
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *AlertManagerDao) ListBuiltinRuleTemplates(query model.AlertBuiltinRuleTemplateQuery) ([]model.AlertBuiltinRuleTemplate, int64, error) {
	var list []model.AlertBuiltinRuleTemplate
	var total int64
	db := d.db.Model(&model.AlertBuiltinRuleTemplate{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR description LIKE ? OR query_template LIKE ?", like, like, like)
	}
	if query.Category != "" {
		db = db.Where("category = ?", query.Category)
	}
	if query.ResourceType != "" {
		db = db.Where("resource_type = ?", query.ResourceType)
	}
	if query.Component != "" {
		db = db.Where("component = ?", query.Component)
	}
	if query.Collector != "" {
		db = db.Where("collector = ?", query.Collector)
	}
	if query.DatasourceType != "" {
		db = db.Where("datasource_type = ?", query.DatasourceType)
	}
	if query.Source != "" {
		db = db.Where("source = ?", query.Source)
	}
	if query.Enabled != "" {
		db = db.Where("enabled = ?", query.Enabled)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("resource_type ASC, component ASC, category ASC, sort DESC, updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *AlertManagerDao) BuiltinRuleTemplateSourceExists(source, sourceUUID string) bool {
	var count int64
	d.db.Model(&model.AlertBuiltinRuleTemplate{}).
		Where("source = ? AND source_uuid = ?", source, sourceUUID).
		Count(&count)
	return count > 0
}

func (d *AlertManagerDao) CreateBuiltinRuleTemplateGroup(item *model.AlertBuiltinRuleTemplateGroup) error {
	now := nowHTime()
	item.CreatedAt = now
	item.UpdatedAt = now
	return d.db.Create(item).Error
}

func (d *AlertManagerDao) UpdateBuiltinRuleTemplateGroup(item *model.AlertBuiltinRuleTemplateGroup) error {
	item.UpdatedAt = nowHTime()
	updates := map[string]interface{}{
		"parent_id":     item.ParentID,
		"level":         item.Level,
		"name":          item.Name,
		"value":         item.Value,
		"resource_type": item.ResourceType,
		"description":   item.Description,
		"sort":          item.Sort,
		"builtin":       item.Builtin,
		"enabled":       item.Enabled,
		"updated_by":    item.UpdatedBy,
		"updated_at":    item.UpdatedAt,
	}
	return d.db.Model(&model.AlertBuiltinRuleTemplateGroup{}).Where("id = ?", item.ID).Updates(updates).Error
}

func (d *AlertManagerDao) DeleteBuiltinRuleTemplateGroup(id uint) error {
	return d.db.Delete(&model.AlertBuiltinRuleTemplateGroup{}, id).Error
}

func (d *AlertManagerDao) DeleteBuiltinRuleTemplateGroupsByIDs(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return d.db.Where("id IN ?", ids).Delete(&model.AlertBuiltinRuleTemplateGroup{}).Error
}

func (d *AlertManagerDao) DeleteBuiltinRuleTemplatesByPath(component, collector, category string) (int64, error) {
	db := d.db.Model(&model.AlertBuiltinRuleTemplate{})
	if component != "" {
		db = db.Where("component = ?", component)
	}
	if collector != "" {
		db = db.Where("collector = ?", collector)
	}
	if category != "" {
		db = db.Where("category = ?", category)
	}
	result := db.Delete(&model.AlertBuiltinRuleTemplate{})
	return result.RowsAffected, result.Error
}

func (d *AlertManagerDao) GetBuiltinRuleTemplateGroup(id uint) (*model.AlertBuiltinRuleTemplateGroup, error) {
	var item model.AlertBuiltinRuleTemplateGroup
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *AlertManagerDao) ListBuiltinRuleTemplateGroups(query model.AlertBuiltinRuleTemplateGroupQuery) ([]model.AlertBuiltinRuleTemplateGroup, int64, error) {
	var list []model.AlertBuiltinRuleTemplateGroup
	var total int64
	db := d.db.Model(&model.AlertBuiltinRuleTemplateGroup{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR value LIKE ? OR description LIKE ?", like, like, like)
	}
	if query.ParentID != "" {
		db = db.Where("parent_id = ?", query.ParentID)
	}
	if query.Level != "" {
		db = db.Where("level = ?", query.Level)
	}
	if query.ResourceType != "" {
		db = db.Where("resource_type = ?", query.ResourceType)
	}
	if query.Enabled != "" {
		db = db.Where("enabled = ?", query.Enabled)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("sort DESC, updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *AlertManagerDao) AllBuiltinRuleTemplateGroups() ([]model.AlertBuiltinRuleTemplateGroup, error) {
	var list []model.AlertBuiltinRuleTemplateGroup
	err := d.db.Where("enabled = ?", model.RuleStatusEnabled).
		Order("sort DESC, id ASC").
		Find(&list).Error
	return list, err
}

func (d *AlertManagerDao) AllBuiltinRuleTemplateGroupsForDelete() ([]model.AlertBuiltinRuleTemplateGroup, error) {
	var list []model.AlertBuiltinRuleTemplateGroup
	err := d.db.Order("sort DESC, id ASC").Find(&list).Error
	return list, err
}

func (d *AlertManagerDao) BuiltinRuleTemplateGroupExists(parentID uint, level, value string, excludeID uint) bool {
	var count int64
	db := d.db.Model(&model.AlertBuiltinRuleTemplateGroup{}).
		Where("parent_id = ? AND level = ? AND value = ?", parentID, level, value)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	db.Count(&count)
	return count > 0
}

func (d *AlertManagerDao) GetBuiltinRuleTemplateGroupByPath(parentID uint, level, value string) (*model.AlertBuiltinRuleTemplateGroup, error) {
	var item model.AlertBuiltinRuleTemplateGroup
	err := d.db.Where("parent_id = ? AND level = ? AND value = ?", parentID, level, value).First(&item).Error
	return &item, err
}

func (d *AlertManagerDao) CountBuiltinRuleTemplateGroupChildren(parentID uint) (int64, error) {
	var count int64
	err := d.db.Model(&model.AlertBuiltinRuleTemplateGroup{}).Where("parent_id = ?", parentID).Count(&count).Error
	return count, err
}

func (d *AlertManagerDao) CountBuiltinRuleTemplatesByPath(component, collector, category string) (int64, error) {
	var count int64
	db := d.db.Model(&model.AlertBuiltinRuleTemplate{})
	if component != "" {
		db = db.Where("component = ?", component)
	}
	if collector != "" {
		db = db.Where("collector = ?", collector)
	}
	if category != "" {
		db = db.Where("category = ?", category)
	}
	err := db.Count(&count).Error
	return count, err
}

func (d *AlertManagerDao) CountPlatformBuiltinRuleTemplatesByPath(component, collector, category string) (int64, error) {
	var count int64
	db := d.db.Model(&model.AlertBuiltinRuleTemplate{}).Where("source = ?", "platform")
	if component != "" {
		db = db.Where("component = ?", component)
	}
	if collector != "" {
		db = db.Where("collector = ?", collector)
	}
	if category != "" {
		db = db.Where("category = ?", category)
	}
	err := db.Count(&count).Error
	return count, err
}

func (d *AlertManagerDao) ListBuiltinRuleTemplatePathRows() ([]model.AlertBuiltinRuleTemplate, error) {
	var list []model.AlertBuiltinRuleTemplate
	err := d.db.Select("component, collector, category, resource_type").
		Where("component <> '' AND collector <> '' AND category <> ''").
		Find(&list).Error
	return list, err
}

func (d *AlertManagerDao) ListActiveEvents(query model.AlertEventQuery) ([]model.AlertActiveEvent, int64, error) {
	var list []model.AlertActiveEvent
	var total int64
	db := d.applyEventFilters(d.db.Model(&model.AlertActiveEvent{}), query, false)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("last_evaluated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *AlertManagerDao) ListActiveEventsForGroups(query model.AlertEventQuery) ([]model.AlertActiveEvent, error) {
	var list []model.AlertActiveEvent
	db := d.applyEventFilters(d.db.Model(&model.AlertActiveEvent{}), query, false)
	err := db.Order("rule_id ASC, datasource_id ASC, severity DESC, last_evaluated_at DESC, id DESC").Find(&list).Error
	return list, err
}

func (d *AlertManagerDao) ListDashboardActiveEvents(query model.AlertDashboardQuery) ([]model.AlertActiveEvent, error) {
	var list []model.AlertActiveEvent
	db := d.db.Model(&model.AlertActiveEvent{}).Where("status <> ?", model.EventStatusResolved)
	if query.DatasourceID > 0 {
		db = db.Where("datasource_id = ?", query.DatasourceID)
	}
	if query.DatasourceType != "" {
		db = db.Where("datasource_type = ?", query.DatasourceType)
	}
	if query.Severity != "" {
		db = db.Where("severity = ?", query.Severity)
	}
	if query.BusinessID != "" {
		db = db.Where("business_id = ?", query.BusinessID)
	}
	err := db.Order("last_evaluated_at DESC, id DESC").Find(&list).Error
	return list, err
}

func (d *AlertManagerDao) ListHistoryEvents(query model.AlertEventQuery) ([]model.AlertHistoryEvent, int64, error) {
	var list []model.AlertHistoryEvent
	var total int64
	db := d.applyEventFilters(d.db.Model(&model.AlertHistoryEvent{}), query, true)
	if query.IncludeActive != "1" && strings.ToLower(query.IncludeActive) != "true" {
		db = db.Where("NOT EXISTS (?)",
			d.db.Model(&model.AlertActiveEvent{}).
				Select("1").
				Where("alert_active_events.fingerprint = alert_history_events.fingerprint"),
		)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("occurred_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *AlertManagerDao) ListDashboardEvents(query model.AlertDashboardQuery, start, end time.Time) ([]model.AlertDashboardEventRow, error) {
	rows := make([]model.AlertDashboardEventRow, 0)
	if err := d.dashboardEventQuery(d.db.Model(&model.AlertActiveEvent{}), query, start, end, false).Scan(&rows).Error; err != nil {
		return nil, err
	}
	var historyRows []model.AlertDashboardEventRow
	if err := d.dashboardEventQuery(d.db.Model(&model.AlertHistoryEvent{}), query, start, end, true).Scan(&historyRows).Error; err != nil {
		return nil, err
	}
	rows = append(rows, historyRows...)
	return rows, nil
}

func (d *AlertManagerDao) DashboardNotifyTrendFromRecords(query model.AlertDashboardQuery, start, end time.Time) ([]model.AlertDashboardNotifyTrendPoint, bool, error) {
	if !d.db.Migrator().HasTable("alert_notify_records") {
		return nil, false, nil
	}
	rows := make([]model.AlertDashboardNotifyTrendPoint, 0)
	dateExpr := dashboardTrendSelectExpr(d.db, "created_at", end.Sub(start) <= 48*time.Hour)
	db := d.db.Table("alert_notify_records").
		Select(dateExpr+" AS date, COUNT(*) AS notification_count, COALESCE(SUM(event_count), 0) AS alert_count, SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) AS success_count, SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failed_count, 'notify_records' AS source").
		Where("created_at >= ? AND created_at < ?", start, end).
		Group(dateExpr).
		Order("date ASC")
	if query.DatasourceID > 0 && d.db.Migrator().HasColumn("alert_notify_records", "datasource_id") {
		db = db.Where("datasource_id = ?", query.DatasourceID)
	}
	if query.DatasourceType != "" && d.db.Migrator().HasColumn("alert_notify_records", "datasource_type") {
		db = db.Where("datasource_type = ?", query.DatasourceType)
	}
	if query.Severity != "" && d.db.Migrator().HasColumn("alert_notify_records", "severity") {
		db = db.Where("severity = ?", query.Severity)
	}
	if query.BusinessID != "" && d.db.Migrator().HasColumn("alert_notify_records", "business_id") {
		db = db.Where("business_id = ?", query.BusinessID)
	}
	err := db.Scan(&rows).Error
	return rows, len(rows) > 0, err
}

func (d *AlertManagerDao) applyEventFilters(db *gorm.DB, query model.AlertEventQuery, history bool) *gorm.DB {
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("rule_name LIKE ? OR fingerprint LIKE ?", like, like)
	}
	if query.Fingerprint != "" {
		db = db.Where("fingerprint = ?", query.Fingerprint)
	}
	if query.RuleID > 0 {
		db = db.Where("rule_id = ?", query.RuleID)
	}
	if query.DatasourceID > 0 {
		db = db.Where("datasource_id = ?", query.DatasourceID)
	}
	if query.DatasourceType != "" {
		db = db.Where("datasource_type = ?", query.DatasourceType)
	}
	if query.Severity != "" {
		db = db.Where("severity = ?", query.Severity)
	}
	if query.Status != "" && !history {
		db = db.Where("status = ?", query.Status)
	} else if !history && !truthyQuery(query.IncludePending) && query.EventStatus != model.EventDisplayStatusPending {
		db = db.Where("status <> ?", model.EventStatusPending)
	}
	if query.EventType != "" && history {
		db = db.Where("event_type = ?", query.EventType)
	}
	if query.BusinessID != "" {
		db = db.Where("business_id = ?", query.BusinessID)
	}
	if !history {
		switch query.EventStatus {
		case model.EventDisplayStatusActive:
			db = db.Where("silenced = ?", 0)
			if !truthyQuery(query.IncludePending) {
				db = db.Where("status <> ?", model.EventStatusPending)
			}
		case model.EventDisplayStatusPending:
			db = db.Where("status = ?", model.EventStatusPending)
		case model.EventDisplayStatusSilenced:
			db = db.Where("silenced = ?", 1)
		}
		if query.Silenced != "" {
			db = db.Where("silenced = ?", query.Silenced)
		}
	}
	return db
}

func (d *AlertManagerDao) dashboardEventQuery(db *gorm.DB, query model.AlertDashboardQuery, start, end time.Time, history bool) *gorm.DB {
	createdColumn := "created_at"
	tableName := "alert_active_events"
	if history {
		tableName = "alert_history_events"
	}
	db = db.Joins("LEFT JOIN alert_rules ON alert_rules.id = " + tableName + ".rule_id")
	db = db.Joins("LEFT JOIN alert_rule_groups ON alert_rule_groups.id = alert_rules.group_id")
	if history {
		db = db.Select(tableName + ".id, " + tableName + ".rule_id, " + tableName + ".rule_name, alert_rules.group_id AS rule_group_id, alert_rule_groups.name AS rule_group_name, alert_rules.notify_rule_ids AS notify_rule_ids, " + tableName + ".datasource_id, " + tableName + ".datasource_type, " + tableName + ".severity, " + tableName + ".labels, " + tableName + ".business_id, " + tableName + ".first_triggered_at, " + tableName + ".created_at")
	} else {
		db = db.Select(tableName + ".id, " + tableName + ".rule_id, " + tableName + ".rule_name, alert_rules.group_id AS rule_group_id, alert_rule_groups.name AS rule_group_name, alert_rules.notify_rule_ids AS notify_rule_ids, " + tableName + ".datasource_id, " + tableName + ".datasource_type, " + tableName + ".severity, " + tableName + ".labels, " + tableName + ".business_id, " + tableName + ".first_triggered_at, " + tableName + ".last_notified_at, " + tableName + ".notify_count, " + tableName + ".created_at")
	}
	db = db.Where(tableName+"."+createdColumn+" >= ? AND "+tableName+"."+createdColumn+" < ?", start, end)
	if query.DatasourceID > 0 {
		db = db.Where(tableName+".datasource_id = ?", query.DatasourceID)
	}
	if query.DatasourceType != "" {
		db = db.Where(tableName+".datasource_type = ?", query.DatasourceType)
	}
	if query.Severity != "" {
		db = db.Where(tableName+".severity = ?", query.Severity)
	}
	if query.BusinessID != "" {
		db = db.Where(tableName+".business_id = ?", query.BusinessID)
	}
	return db
}

func dashboardTrendSelectExpr(db *gorm.DB, column string, hourly bool) string {
	switch db.Dialector.Name() {
	case "mysql":
		if hourly {
			return "DATE_FORMAT(" + column + ", '%Y-%m-%d %H:00')"
		}
		return "DATE_FORMAT(" + column + ", '%Y-%m-%d')"
	case "sqlite":
		if hourly {
			return "strftime('%Y-%m-%d %H:00', " + column + ")"
		}
		return "strftime('%Y-%m-%d', " + column + ")"
	case "postgres":
		if hourly {
			return "TO_CHAR(" + column + ", 'YYYY-MM-DD HH24:00')"
		}
		return "TO_CHAR(" + column + ", 'YYYY-MM-DD')"
	default:
		return "DATE(" + column + ")"
	}
}

func truthyQuery(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func (d *AlertManagerDao) ListTimeline(eventID uint64) ([]model.AlertEventTimeline, error) {
	var list []model.AlertEventTimeline
	err := d.db.Where("event_id = ?", eventID).Order("created_at ASC, id ASC").Find(&list).Error
	return list, err
}

func (d *AlertManagerDao) CreateNotifyRecord(item *model.AlertNotifyRecord) error {
	if item.CreatedAt.IsZero() {
		item.CreatedAt = nowHTime()
	}
	return d.db.Create(item).Error
}

func (d *AlertManagerDao) ListNotifyRecordsByEvent(eventID uint64, eventType string) ([]model.AlertNotifyRecord, error) {
	var list []model.AlertNotifyRecord
	if eventID == 0 {
		return list, nil
	}
	db := d.db.Where("event_id = ?", eventID)
	if strings.TrimSpace(eventType) != "" {
		db = db.Where("(event_type = ? OR event_type = '')", eventType)
	}
	err := db.Order("created_at DESC, id DESC").Find(&list).Error
	if err == nil {
		d.hydrateNotifyRecordGroups(list)
	}
	return list, err
}

func (d *AlertManagerDao) ListNotifyRecords(query model.AlertNotifyRecordQuery) ([]model.AlertNotifyRecord, int64, error) {
	var list []model.AlertNotifyRecord
	var total int64
	db := d.applyNotifyRecordFilters(d.db.Model(&model.AlertNotifyRecord{}), query)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	if err == nil {
		d.hydrateNotifyRecordGroups(list)
	}
	return list, total, err
}

func (d *AlertManagerDao) hydrateNotifyRecordGroups(list []model.AlertNotifyRecord) {
	groupIDs := make([]uint, 0)
	seen := make(map[uint]struct{})
	for index := range list {
		if request := parseNotifyRecordRequest(list[index].Request); request.ReceiverGroupID > 0 {
			if list[index].NotifyGroupID == 0 {
				list[index].NotifyGroupID = request.ReceiverGroupID
			}
			if strings.TrimSpace(list[index].NotifyGroupName) == "" {
				list[index].NotifyGroupName = request.ReceiverGroupName
			}
		}
		if list[index].NotifyGroupID == 0 || strings.TrimSpace(list[index].NotifyGroupName) != "" {
			continue
		}
		if _, ok := seen[list[index].NotifyGroupID]; ok {
			continue
		}
		seen[list[index].NotifyGroupID] = struct{}{}
		groupIDs = append(groupIDs, list[index].NotifyGroupID)
	}
	if len(groupIDs) == 0 {
		return
	}

	var groups []model.AlertNotifyGroup
	if err := d.db.Select("id, name").Where("id IN ?", groupIDs).Find(&groups).Error; err != nil {
		return
	}
	nameByID := make(map[uint]string, len(groups))
	for _, group := range groups {
		nameByID[group.ID] = group.Name
	}
	for index := range list {
		if strings.TrimSpace(list[index].NotifyGroupName) == "" {
			list[index].NotifyGroupName = nameByID[list[index].NotifyGroupID]
		}
	}
}

type notifyRecordRequest struct {
	ReceiverGroupID   uint   `json:"receiverGroupId"`
	ReceiverGroupName string `json:"receiverGroupName"`
}

func parseNotifyRecordRequest(value string) notifyRecordRequest {
	var request notifyRecordRequest
	if strings.TrimSpace(value) != "" {
		_ = json.Unmarshal([]byte(value), &request)
	}
	return request
}

func (d *AlertManagerDao) NotifyRecordStats(query model.AlertNotifyRecordQuery) (model.AlertNotifyRecordStats, error) {
	var stats model.AlertNotifyRecordStats
	db := d.applyNotifyRecordFilters(d.db.Model(&model.AlertNotifyRecord{}), query)
	err := db.Select("COUNT(*) AS total, SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS success_count, SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS failed_count", model.NotifyStatusSuccess, model.NotifyStatusFailed).Scan(&stats).Error
	return stats, err
}

func (d *AlertManagerDao) applyNotifyRecordFilters(db *gorm.DB, query model.AlertNotifyRecordQuery) *gorm.DB {
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("rule_name LIKE ? OR channel_name LIKE ? OR receiver LIKE ? OR error_message LIKE ?", like, like, like, like)
	}
	if query.EventID > 0 {
		db = db.Where("event_id = ?", query.EventID)
	}
	if query.RuleID > 0 {
		db = db.Where("rule_id = ?", query.RuleID)
	}
	if query.NotifyRuleID > 0 {
		db = db.Where("notify_rule_id = ?", query.NotifyRuleID)
	}
	if query.ChannelID > 0 {
		db = db.Where("channel_id = ?", query.ChannelID)
	}
	if query.ChannelType != "" {
		db = db.Where("channel_type = ?", query.ChannelType)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.DatasourceID > 0 {
		db = db.Where("datasource_id = ?", query.DatasourceID)
	}
	if query.DatasourceType != "" {
		db = db.Where("datasource_type = ?", query.DatasourceType)
	}
	if query.Severity != "" {
		db = db.Where("severity = ?", query.Severity)
	}
	if query.BusinessID != "" {
		db = db.Where("business_id = ?", query.BusinessID)
	}
	return db
}

func (d *AlertManagerDao) GetActiveEvent(id uint64) (*model.AlertActiveEvent, error) {
	var item model.AlertActiveEvent
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *AlertManagerDao) ListActiveEventsByIDs(ids []uint64) ([]model.AlertActiveEvent, error) {
	var list []model.AlertActiveEvent
	if len(ids) == 0 {
		return list, nil
	}
	err := d.db.Where("id IN ?", ids).Order("id ASC").Find(&list).Error
	return list, err
}

func (d *AlertManagerDao) ResolveActiveEventsByRuleID(ruleID uint, operator, reason string) (int64, error) {
	if ruleID == 0 {
		return 0, nil
	}
	now := nowHTime()
	var resolvedCount int64
	err := d.db.Transaction(func(tx *gorm.DB) error {
		var events []model.AlertActiveEvent
		if err := tx.Where("rule_id = ?", ruleID).Find(&events).Error; err != nil {
			return err
		}
		if len(events) == 0 {
			return nil
		}
		historyEvents := make([]model.AlertHistoryEvent, 0, len(events))
		timelines := make([]model.AlertEventTimeline, 0, len(events))
		eventIDs := make([]uint64, 0, len(events))
		for _, event := range events {
			eventIDs = append(eventIDs, event.ID)
			historyEvents = append(historyEvents, activeEventToHistoryEvent(event, now))
			timelines = append(timelines, model.AlertEventTimeline{
				EventID:   event.ID,
				Action:    "resolved",
				Message:   firstNonEmptyText(reason, "规则已禁用，告警自动恢复"),
				Operator:  firstNonEmptyText(operator, "system"),
				Metadata:  model.JSONMap{"reason": "rule_disabled", "ruleId": ruleID},
				CreatedAt: now,
			})
		}
		if err := tx.Create(&historyEvents).Error; err != nil {
			return err
		}
		if err := tx.Create(&timelines).Error; err != nil {
			return err
		}
		result := tx.Where("id IN ?", eventIDs).Delete(&model.AlertActiveEvent{})
		if result.Error != nil {
			return result.Error
		}
		resolvedCount = result.RowsAffected
		return nil
	})
	return resolvedCount, err
}

func activeEventToHistoryEvent(event model.AlertActiveEvent, now util.HTime) model.AlertHistoryEvent {
	return model.AlertHistoryEvent{
		Fingerprint:      event.Fingerprint,
		ActiveEventID:    event.ID,
		RuleID:           event.RuleID,
		RuleName:         event.RuleName,
		DatasourceID:     event.DatasourceID,
		DatasourceType:   event.DatasourceType,
		EventType:        model.EventStatusResolved,
		Severity:         event.Severity,
		Labels:           event.Labels,
		Annotations:      event.Annotations,
		Value:            event.Value,
		TriggerQuery:     event.TriggerQuery,
		TriggerCondition: event.TriggerCondition,
		EventDetail:      event.EventDetail,
		AssetID:          event.AssetID,
		BusinessID:       event.BusinessID,
		FirstTriggeredAt: event.FirstTriggeredAt,
		RecoveredAt:      now,
		ClaimUser:        event.ClaimUser,
		OccurredAt:       now,
		CreatedAt:        now,
	}
}

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (d *AlertManagerDao) GetHistoryEvent(id uint64) (*model.AlertHistoryEvent, error) {
	var item model.AlertHistoryEvent
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *AlertManagerDao) ClaimActiveEvent(id uint64, username string) error {
	now := nowHTime()
	return d.db.Model(&model.AlertActiveEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"claim_user": username,
			"claimed_at": now,
			"updated_at": now,
		}).Error
}

func (d *AlertManagerDao) CreateEventSilence(item *model.AlertSilenceRule, eventID uint64, operator string) ([]model.AlertActiveEvent, error) {
	now := time.Now()
	updatedEvents := make([]model.AlertActiveEvent, 0)
	err := d.db.Transaction(func(tx *gorm.DB) error {
		var event model.AlertActiveEvent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&event, eventID).Error; err != nil {
			return err
		}
		if event.Silenced == 1 {
			return fmt.Errorf("event %d is already silenced", eventID)
		}
		item.CreatedAt = util.HTime{Time: now}
		item.UpdatedAt = util.HTime{Time: now}
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		var err error
		updatedEvents, err = reconcileEventSilenceStates(tx, now, "静默规则已更新", operator)
		return err
	})
	return updatedEvents, err
}

func (d *AlertManagerDao) CancelEventSilences(eventID uint64, operator string) (*EventSilenceCancellation, error) {
	now := time.Now()
	result := &EventSilenceCancellation{}
	err := d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&result.Event, eventID).Error; err != nil {
			return err
		}
		var rules []model.AlertSilenceRule
		if err := tx.Where("enabled = ?", model.RuleStatusEnabled).Find(&rules).Error; err != nil {
			return err
		}
		ids := eventSilenceRuleIDs(rules, result.Event)
		if len(ids) > 0 {
			updated := tx.Model(&model.AlertSilenceRule{}).Where("id IN ?", ids).Updates(map[string]interface{}{
				"enabled":    0,
				"end_time":   util.HTime{Time: now},
				"updated_by": operator,
				"updated_at": util.HTime{Time: now},
			})
			if updated.Error != nil {
				return updated.Error
			}
			result.DisabledRules = updated.RowsAffected
		}
		activeRules, err := activeSilenceRulesForEvent(tx, result.Event, now)
		if err != nil {
			return err
		}
		result.RemainingRules = activeRules
		message := "事件静默规则已取消"
		if len(activeRules) == 0 {
			if err := setEventUnsilenced(tx, &result.Event, now); err != nil {
				return err
			}
			message = "告警已取消静默"
		} else {
			message = "事件仍受其他静默规则影响"
		}
		return tx.Create(&model.AlertEventTimeline{
			EventID:   eventID,
			Action:    "unsilence",
			Message:   message,
			Operator:  operator,
			Metadata:  model.JSONMap{"disabledRules": result.DisabledRules, "remainingRuleIds": silenceRuleIDs(activeRules)},
			CreatedAt: util.HTime{Time: now},
		}).Error
	})
	return result, err
}

func (d *AlertManagerDao) CancelSilenceRuleAndReconcile(id uint, operator string) ([]model.AlertActiveEvent, error) {
	now := time.Now()
	updatedEvents := make([]model.AlertActiveEvent, 0)
	err := d.db.Transaction(func(tx *gorm.DB) error {
		var item model.AlertSilenceRule
		if err := tx.First(&item, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.AlertSilenceRule{}).Where("id = ?", id).Updates(map[string]interface{}{
			"enabled":    0,
			"end_time":   util.HTime{Time: now},
			"updated_by": operator,
			"updated_at": util.HTime{Time: now},
		}).Error; err != nil {
			return err
		}
		events, err := reconcileEventSilenceStates(tx, now, "静默规则已取消", operator)
		if err != nil {
			return err
		}
		updatedEvents = events
		return nil
	})
	return updatedEvents, err
}

func (d *AlertManagerDao) ReconcileEventSilenceStates(now time.Time, message, operator string) ([]model.AlertActiveEvent, error) {
	var updated []model.AlertActiveEvent
	err := d.db.Transaction(func(tx *gorm.DB) error {
		var err error
		updated, err = reconcileEventSilenceStates(tx, now, message, operator)
		return err
	})
	return updated, err
}

func (d *AlertManagerDao) ListActiveSilenceRulesForEvent(event model.AlertActiveEvent, now time.Time) ([]model.AlertSilenceRule, error) {
	return activeSilenceRulesForEvent(d.db, event, now)
}

func reconcileEventSilenceStates(tx *gorm.DB, now time.Time, unsilenceMessage, operator string) ([]model.AlertActiveEvent, error) {
	var events []model.AlertActiveEvent
	if err := tx.Where("status <> ?", model.EventStatusResolved).Find(&events).Error; err != nil {
		return nil, err
	}
	var rules []model.AlertSilenceRule
	if err := tx.Where("enabled = ? AND start_time <= ? AND end_time >= ?", model.RuleStatusEnabled, now, now).Find(&rules).Error; err != nil {
		return nil, err
	}
	updated := make([]model.AlertActiveEvent, 0)
	for index := range events {
		matched := matchingSilenceRules(rules, events[index])
		if len(matched) > 0 && events[index].Silenced == 0 {
			if err := setEventSilenced(tx, &events[index], now); err != nil {
				return nil, err
			}
			if err := tx.Create(&model.AlertEventTimeline{
				EventID:   events[index].ID,
				Action:    "silence",
				Message:   "告警已被静默规则匹配",
				Operator:  operator,
				Metadata:  model.JSONMap{"silenceRuleIds": silenceRuleIDs(matched)},
				CreatedAt: util.HTime{Time: now},
			}).Error; err != nil {
				return nil, err
			}
			updated = append(updated, events[index])
			continue
		}
		if len(matched) > 0 || events[index].Silenced == 0 {
			continue
		}
		if err := setEventUnsilenced(tx, &events[index], now); err != nil {
			return nil, err
		}
		if err := tx.Create(&model.AlertEventTimeline{
			EventID:   events[index].ID,
			Action:    "unsilence",
			Message:   unsilenceMessage,
			Operator:  operator,
			Metadata:  model.JSONMap{"reason": "no_active_silence_rules"},
			CreatedAt: util.HTime{Time: now},
		}).Error; err != nil {
			return nil, err
		}
		updated = append(updated, events[index])
	}
	return updated, nil
}

func activeSilenceRulesForEvent(tx *gorm.DB, event model.AlertActiveEvent, now time.Time) ([]model.AlertSilenceRule, error) {
	var rules []model.AlertSilenceRule
	if err := tx.Where("enabled = ? AND start_time <= ? AND end_time >= ?", model.RuleStatusEnabled, now, now).Find(&rules).Error; err != nil {
		return nil, err
	}
	return matchingSilenceRules(rules, event), nil
}

func matchingSilenceRules(rules []model.AlertSilenceRule, event model.AlertActiveEvent) []model.AlertSilenceRule {
	context := model.SilenceContextFromEvent(event)
	matched := make([]model.AlertSilenceRule, 0)
	for _, rule := range rules {
		if model.SilenceRuleMatches(rule, context) {
			matched = append(matched, rule)
		}
	}
	return matched
}

func eventSilenceRuleIDs(rules []model.AlertSilenceRule, event model.AlertActiveEvent) []uint {
	eventID := strconv.FormatUint(event.ID, 10)
	ids := make([]uint, 0)
	for _, rule := range rules {
		matcherEventID := strings.TrimSpace(fmt.Sprint(rule.Matchers["eventId"]))
		legacy := (matcherEventID == "" || matcherEventID == "<nil>") && strings.HasPrefix(rule.Name, "事件静默-") && fmt.Sprint(rule.Matchers["fingerprint"]) == event.Fingerprint
		if matcherEventID == eventID || legacy {
			ids = append(ids, rule.ID)
		}
	}
	return ids
}

func setEventUnsilenced(tx *gorm.DB, event *model.AlertActiveEvent, now time.Time) error {
	var rule model.AlertRule
	_ = tx.First(&rule, event.RuleID).Error
	event.Silenced = 0
	event.Status = model.EventStatusAfterSilence(*event, rule, now)
	event.UpdatedAt = util.HTime{Time: now}
	return tx.Model(&model.AlertActiveEvent{}).Where("id = ?", event.ID).Updates(map[string]interface{}{
		"silenced":   event.Silenced,
		"status":     event.Status,
		"updated_at": event.UpdatedAt,
	}).Error
}

func setEventSilenced(tx *gorm.DB, event *model.AlertActiveEvent, now time.Time) error {
	event.Silenced = 1
	event.Status = model.EventStatusSilenced
	event.UpdatedAt = util.HTime{Time: now}
	return tx.Model(&model.AlertActiveEvent{}).Where("id = ?", event.ID).Updates(map[string]interface{}{
		"silenced":   event.Silenced,
		"status":     event.Status,
		"updated_at": event.UpdatedAt,
	}).Error
}

func silenceRuleIDs(rules []model.AlertSilenceRule) []uint {
	ids := make([]uint, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}
	return ids
}

func (d *AlertManagerDao) AppendTimeline(eventID uint64, action, message, operator string, metadata model.JSONMap) error {
	return d.db.Create(&model.AlertEventTimeline{
		EventID:   eventID,
		Action:    action,
		Message:   message,
		Operator:  operator,
		Metadata:  metadata,
		CreatedAt: nowHTime(),
	}).Error
}

func (d *AlertManagerDao) CreateSilenceRuleAndReconcile(item *model.AlertSilenceRule, operator string) ([]model.AlertActiveEvent, error) {
	now := time.Now()
	updatedEvents := make([]model.AlertActiveEvent, 0)
	err := d.db.Transaction(func(tx *gorm.DB) error {
		item.CreatedAt = util.HTime{Time: now}
		item.UpdatedAt = util.HTime{Time: now}
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		var err error
		updatedEvents, err = reconcileEventSilenceStates(tx, now, "静默规则已更新", operator)
		return err
	})
	return updatedEvents, err
}

func (d *AlertManagerDao) UpdateSilenceRuleAndReconcile(item *model.AlertSilenceRule, operator string) ([]model.AlertActiveEvent, error) {
	now := time.Now()
	updatedEvents := make([]model.AlertActiveEvent, 0)
	err := d.db.Transaction(func(tx *gorm.DB) error {
		item.UpdatedAt = util.HTime{Time: now}
		updates := map[string]interface{}{
			"name":        item.Name,
			"matchers":    item.Matchers,
			"start_time":  item.StartTime,
			"end_time":    item.EndTime,
			"repeat_type": item.RepeatType,
			"reason":      item.Reason,
			"enabled":     item.Enabled,
			"updated_by":  item.UpdatedBy,
			"updated_at":  item.UpdatedAt,
		}
		if err := tx.Model(&model.AlertSilenceRule{}).Where("id = ?", item.ID).Updates(updates).Error; err != nil {
			return err
		}
		var err error
		updatedEvents, err = reconcileEventSilenceStates(tx, now, "静默规则已更新", operator)
		return err
	})
	return updatedEvents, err
}

func (d *AlertManagerDao) GetSilenceRule(id uint) (*model.AlertSilenceRule, error) {
	var item model.AlertSilenceRule
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *AlertManagerDao) ListSilenceRules(query model.AlertSilenceRuleQuery) ([]model.AlertSilenceRule, int64, error) {
	var list []model.AlertSilenceRule
	var total int64
	db := d.db.Model(&model.AlertSilenceRule{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR reason LIKE ?", like, like)
	}
	if query.Enabled != "" {
		db = db.Where("enabled = ?", query.Enabled)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *AlertManagerDao) CreateNotifyChannel(item *model.AlertNotifyChannel) error {
	now := nowHTime()
	item.CreatedAt = now
	item.UpdatedAt = now
	return d.db.Create(item).Error
}

func (d *AlertManagerDao) UpdateNotifyChannel(item *model.AlertNotifyChannel) error {
	item.UpdatedAt = nowHTime()
	updates := map[string]interface{}{
		"name":        item.Name,
		"type":        item.Type,
		"description": item.Description,
		"config":      item.Config,
		"enabled":     item.Enabled,
		"updated_by":  item.UpdatedBy,
		"updated_at":  item.UpdatedAt,
	}
	return d.db.Model(&model.AlertNotifyChannel{}).Where("id = ?", item.ID).Updates(updates).Error
}

func (d *AlertManagerDao) DeleteNotifyChannel(id uint) error {
	return d.db.Delete(&model.AlertNotifyChannel{}, id).Error
}

func (d *AlertManagerDao) GetNotifyChannel(id uint) (*model.AlertNotifyChannel, error) {
	var item model.AlertNotifyChannel
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *AlertManagerDao) ListNotifyChannels(query model.AlertNotifyChannelQuery) ([]model.AlertNotifyChannel, int64, error) {
	var list []model.AlertNotifyChannel
	var total int64
	db := d.db.Model(&model.AlertNotifyChannel{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if query.Type != "" {
		db = db.Where("type = ?", query.Type)
	}
	if query.Enabled != "" {
		db = db.Where("enabled = ?", query.Enabled)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *AlertManagerDao) ListAllNotifyChannels() ([]model.AlertNotifyChannel, error) {
	var list []model.AlertNotifyChannel
	err := d.db.Order("updated_at DESC, id DESC").Find(&list).Error
	return list, err
}

func (d *AlertManagerDao) CreateNotifyGroup(item *model.AlertNotifyGroup) error {
	now := nowHTime()
	item.CreatedAt = now
	item.UpdatedAt = now
	return d.db.Create(item).Error
}

func (d *AlertManagerDao) UpdateNotifyGroup(item *model.AlertNotifyGroup) error {
	item.UpdatedAt = nowHTime()
	updates := map[string]interface{}{
		"name":         item.Name,
		"business_id":  item.BusinessID,
		"channel_type": item.ChannelType,
		"user_ids":     item.UserIDs,
		"description":  item.Description,
		"enabled":      item.Enabled,
		"updated_by":   item.UpdatedBy,
		"updated_at":   item.UpdatedAt,
	}
	return d.db.Model(&model.AlertNotifyGroup{}).Where("id = ?", item.ID).Updates(updates).Error
}

func (d *AlertManagerDao) DeleteNotifyGroup(id uint) error {
	return d.db.Delete(&model.AlertNotifyGroup{}, id).Error
}

func (d *AlertManagerDao) NotifyGroupReferenceCount(id uint) (int64, error) {
	if id == 0 {
		return 0, nil
	}
	var routeCount int64
	if err := d.db.Model(&model.AlertNotifyRouteRule{}).Where("notify_group_id = ?", id).Count(&routeCount).Error; err != nil {
		return 0, err
	}
	var policies []model.AlertNotifyRule
	if err := d.db.Select("channels, receiver_groups").Find(&policies).Error; err != nil {
		return 0, err
	}
	var policyCount int64
	for _, policy := range policies {
		if jsonValueContainsUint(policy.Channels, []string{"receiverGroupId", "receiver_group_id"}, id) ||
			jsonValueContainsUint(policy.ReceiverGroups, []string{"receiverGroupId", "receiver_group_id"}, id) {
			policyCount++
		}
	}
	return routeCount + policyCount, nil
}

func (d *AlertManagerDao) GetNotifyGroup(id uint) (*model.AlertNotifyGroup, error) {
	var item model.AlertNotifyGroup
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *AlertManagerDao) GetEnabledNotifyGroupByID(id uint) (*model.AlertNotifyGroup, error) {
	var item model.AlertNotifyGroup
	err := d.db.Where("id = ? AND enabled = ?", id, model.RuleStatusEnabled).First(&item).Error
	return &item, err
}

func (d *AlertManagerDao) GetEnabledNotifyGroupByBusinessID(businessID, channelType string) (*model.AlertNotifyGroup, error) {
	var item model.AlertNotifyGroup
	db := d.db.Where("business_id = ? AND enabled = ?", businessID, model.RuleStatusEnabled)
	if strings.TrimSpace(channelType) != "" {
		db = db.Where("channel_type = ?", strings.TrimSpace(channelType))
	}
	err := db.Order("updated_at DESC, id DESC").First(&item).Error
	return &item, err
}

func (d *AlertManagerDao) ListNotifyGroups(query model.AlertNotifyGroupQuery) ([]model.AlertNotifyGroup, int64, error) {
	var list []model.AlertNotifyGroup
	var total int64
	db := d.db.Model(&model.AlertNotifyGroup{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR business_id LIKE ? OR description LIKE ?", like, like, like)
	}
	if query.BusinessID != "" {
		db = db.Where("business_id = ?", query.BusinessID)
	}
	if query.ChannelType != "" {
		db = db.Where("channel_type = ?", query.ChannelType)
	}
	if query.Enabled != "" {
		db = db.Where("enabled = ?", query.Enabled)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *AlertManagerDao) NotifyGroupBusinessExists(businessID string, excludeID uint) bool {
	var count int64
	db := d.db.Model(&model.AlertNotifyGroup{}).Where("business_id = ?", strings.TrimSpace(businessID))
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	db.Count(&count)
	return count > 0
}

func (d *AlertManagerDao) CreateNotifyRouteRule(item *model.AlertNotifyRouteRule) error {
	now := nowHTime()
	item.CreatedAt = now
	item.UpdatedAt = now
	return d.db.Create(item).Error
}

func (d *AlertManagerDao) ListEnabledNotifyRouteRules() ([]model.AlertNotifyRouteRule, error) {
	var list []model.AlertNotifyRouteRule
	err := d.db.Where("enabled = ?", model.RuleStatusEnabled).
		Order("priority DESC, id ASC").Find(&list).Error
	return list, err
}

func (d *AlertManagerDao) UpdateNotifyRouteRule(item *model.AlertNotifyRouteRule) error {
	item.UpdatedAt = nowHTime()
	updates := map[string]interface{}{
		"name":            item.Name,
		"conditions":      item.Conditions,
		"notify_rule_id":  item.NotifyRuleID,
		"notify_group_id": item.NotifyGroupID,
		"override_group":  item.OverrideGroup,
		"priority":        item.Priority,
		"description":     item.Description,
		"enabled":         item.Enabled,
		"updated_by":      item.UpdatedBy,
		"updated_at":      item.UpdatedAt,
	}
	return d.db.Model(&model.AlertNotifyRouteRule{}).Where("id = ?", item.ID).Updates(updates).Error
}

func (d *AlertManagerDao) DeleteNotifyRouteRule(id uint) error {
	return d.db.Delete(&model.AlertNotifyRouteRule{}, id).Error
}

func (d *AlertManagerDao) GetNotifyRouteRule(id uint) (*model.AlertNotifyRouteRule, error) {
	var item model.AlertNotifyRouteRule
	if err := d.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	var group model.AlertNotifyGroup
	if err := d.db.Select("id, name").First(&group, item.NotifyGroupID).Error; err == nil {
		item.NotifyGroupName = group.Name
	}
	return &item, nil
}

func (d *AlertManagerDao) ListNotifyRouteRules(query model.AlertNotifyRouteRuleQuery) ([]model.AlertNotifyRouteRule, int64, error) {
	var list []model.AlertNotifyRouteRule
	var total int64
	db := d.db.Model(&model.AlertNotifyRouteRule{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR description LIKE ? OR JSON_SEARCH(conditions, 'one', ?, NULL, '$[*].labelKey', '$[*].matchValue') IS NOT NULL", like, like, like)
	}
	if query.MatchMode != "" {
		db = db.Where("JSON_SEARCH(conditions, 'one', ?, NULL, '$[*].matchMode') IS NOT NULL", query.MatchMode)
	}
	if query.NotifyRuleID > 0 {
		db = db.Where("notify_rule_id = ?", query.NotifyRuleID)
	}
	if query.NotifyGroupID > 0 {
		db = db.Where("notify_group_id = ?", query.NotifyGroupID)
	}
	if query.Enabled != "" {
		db = db.Where("enabled = ?", query.Enabled)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	if err := db.Order("priority DESC, updated_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	d.fillNotifyRouteGroupNames(list)
	return list, total, nil
}

func (d *AlertManagerDao) NotifyRouteRuleNameExists(name string, excludeID uint) bool {
	var count int64
	db := d.db.Model(&model.AlertNotifyRouteRule{}).Where("name = ?", strings.TrimSpace(name))
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	return db.Count(&count).Error == nil && count > 0
}

func (d *AlertManagerDao) fillNotifyRouteGroupNames(items []model.AlertNotifyRouteRule) {
	if len(items) == 0 {
		return
	}
	ids := make([]uint, 0, len(items))
	seen := make(map[uint]struct{}, len(items))
	for _, item := range items {
		if item.NotifyGroupID == 0 {
			continue
		}
		if _, ok := seen[item.NotifyGroupID]; ok {
			continue
		}
		seen[item.NotifyGroupID] = struct{}{}
		ids = append(ids, item.NotifyGroupID)
	}
	if len(ids) == 0 {
		return
	}
	var groups []model.AlertNotifyGroup
	if err := d.db.Where("id IN ?", ids).Find(&groups).Error; err != nil {
		return
	}
	nameByID := make(map[uint]string, len(groups))
	for _, group := range groups {
		nameByID[group.ID] = group.Name
	}
	for index := range items {
		items[index].NotifyGroupName = nameByID[items[index].NotifyGroupID]
	}
}

func (d *AlertManagerDao) CreateNotifyTemplate(item *model.AlertNotifyTemplate) error {
	now := nowHTime()
	item.CreatedAt = now
	item.UpdatedAt = now
	return d.db.Create(item).Error
}

func (d *AlertManagerDao) UpdateNotifyChannelConfig(id uint, config model.JSONMap, updatedBy string) error {
	updates := map[string]interface{}{
		"config":     config,
		"updated_by": updatedBy,
		"updated_at": nowHTime(),
	}
	return d.db.Model(&model.AlertNotifyChannel{}).Where("id = ?", id).Updates(updates).Error
}

func (d *AlertManagerDao) UpdateNotifyTemplate(item *model.AlertNotifyTemplate) error {
	item.UpdatedAt = nowHTime()
	updates := map[string]interface{}{
		"ident":         item.Ident,
		"name":          item.Name,
		"channel_type":  item.ChannelType,
		"template_type": item.TemplateType,
		"description":   item.Description,
		"content":       item.Content,
		"builtin":       item.Builtin,
		"enabled":       item.Enabled,
		"updated_by":    item.UpdatedBy,
		"updated_at":    item.UpdatedAt,
	}
	return d.db.Model(&model.AlertNotifyTemplate{}).Where("id = ?", item.ID).Updates(updates).Error
}

func (d *AlertManagerDao) DeleteNotifyTemplate(id uint) error {
	return d.db.Delete(&model.AlertNotifyTemplate{}, id).Error
}

func (d *AlertManagerDao) GetNotifyTemplate(id uint) (*model.AlertNotifyTemplate, error) {
	var item model.AlertNotifyTemplate
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *AlertManagerDao) GetDefaultNotifyTemplateByChannelType(channelType string) (*model.AlertNotifyTemplate, error) {
	var item model.AlertNotifyTemplate
	err := d.db.Model(&model.AlertNotifyTemplate{}).
		Where("channel_type = ? AND enabled = ?", channelType, 1).
		Order("builtin DESC, updated_at DESC, id DESC").
		First(&item).Error
	return &item, err
}

func (d *AlertManagerDao) ListNotifyTemplates(query model.AlertNotifyTemplateQuery) ([]model.AlertNotifyTemplate, int64, error) {
	var list []model.AlertNotifyTemplate
	var total int64
	db := d.db.Model(&model.AlertNotifyTemplate{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR ident LIKE ? OR description LIKE ?", like, like, like)
	}
	if query.ChannelType != "" {
		db = db.Where("channel_type = ?", query.ChannelType)
	}
	if query.TemplateType != "" {
		db = db.Where("template_type = ?", query.TemplateType)
	}
	if query.Builtin != "" {
		db = db.Where("builtin = ?", query.Builtin)
	}
	if query.Enabled != "" {
		db = db.Where("enabled = ?", query.Enabled)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("builtin DESC, updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *AlertManagerDao) CreateNotifyRule(item *model.AlertNotifyRule) error {
	now := nowHTime()
	item.CreatedAt = now
	item.UpdatedAt = now
	return d.db.Create(item).Error
}

func (d *AlertManagerDao) UpdateNotifyRule(item *model.AlertNotifyRule) error {
	item.UpdatedAt = nowHTime()
	updates := map[string]interface{}{
		"name":                       item.Name,
		"matchers":                   item.Matchers,
		"channels":                   item.Channels,
		"receiver_groups":            item.ReceiverGroups,
		"repeat_notify":              item.RepeatNotify,
		"repeat_mode":                item.RepeatMode,
		"repeat_interval_seconds":    item.RepeatIntervalSeconds,
		"repeat_intervals":           item.RepeatIntervals,
		"max_repeat_times":           item.MaxRepeatTimes,
		"recovery_notify":            item.RecoveryNotify,
		"aggregation_enabled":        item.AggregationEnabled,
		"group_wait_seconds":         item.GroupWaitSeconds,
		"group_interval_seconds":     item.GroupIntervalSeconds,
		"aggregation_window_seconds": item.AggregationWindowSeconds,
		"priority":                   item.Priority,
		"enabled":                    item.Enabled,
		"updated_by":                 item.UpdatedBy,
		"updated_at":                 item.UpdatedAt,
	}
	return d.db.Model(&model.AlertNotifyRule{}).Where("id = ?", item.ID).Updates(updates).Error
}

func (d *AlertManagerDao) DeleteNotifyRule(id uint) error {
	return d.db.Delete(&model.AlertNotifyRule{}, id).Error
}

func (d *AlertManagerDao) NotifyRuleReferenceCount(id uint) (int64, error) {
	if id == 0 {
		return 0, nil
	}
	var routeCount int64
	if err := d.db.Model(&model.AlertNotifyRouteRule{}).Where("notify_rule_id = ?", id).Count(&routeCount).Error; err != nil {
		return 0, err
	}
	var rules []model.AlertRule
	if err := d.db.Select("notify_rule_ids").Find(&rules).Error; err != nil {
		return 0, err
	}
	var alertRuleCount int64
	for _, rule := range rules {
		for _, notifyRuleID := range rule.NotifyRuleIDs {
			if notifyRuleID == id {
				alertRuleCount++
				break
			}
		}
	}
	return routeCount + alertRuleCount, nil
}

func jsonValueContainsUint(value interface{}, keys []string, target uint) bool {
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}
	var visit func(interface{}) bool
	visit = func(current interface{}) bool {
		switch typed := current.(type) {
		case model.JSONMap:
			return visit(map[string]interface{}(typed))
		case map[string]interface{}:
			for key, child := range typed {
				if _, ok := keySet[key]; ok && jsonNumberEquals(child, target) {
					return true
				}
				if visit(child) {
					return true
				}
			}
		case []interface{}:
			for _, child := range typed {
				if visit(child) {
					return true
				}
			}
		}
		return false
	}
	return visit(value)
}

func jsonNumberEquals(value interface{}, target uint) bool {
	if value == nil {
		return false
	}
	var number uint64
	switch typed := value.(type) {
	case float64:
		number = uint64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil || parsed < 0 {
			return false
		}
		number = uint64(parsed)
	case string:
		parsed, err := strconv.ParseUint(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return false
		}
		number = parsed
	case uint:
		number = uint64(typed)
	case uint64:
		number = typed
	case int:
		if typed < 0 {
			return false
		}
		number = uint64(typed)
	}
	return number == uint64(target)
}

func (d *AlertManagerDao) GetNotifyRule(id uint) (*model.AlertNotifyRule, error) {
	var item model.AlertNotifyRule
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *AlertManagerDao) ListNotifyRules(query model.AlertNotifyRuleQuery) ([]model.AlertNotifyRule, int64, error) {
	var list []model.AlertNotifyRule
	var total int64
	db := d.db.Model(&model.AlertNotifyRule{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ?", like)
	}
	if query.Enabled != "" {
		db = db.Where("enabled = ?", query.Enabled)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("priority DESC, updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}
