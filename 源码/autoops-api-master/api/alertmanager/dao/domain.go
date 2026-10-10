package dao

import (
	"strings"
	"time"

	"dodevops-api/api/alertmanager/model"
	"dodevops-api/common/util"

	"gorm.io/gorm"
)

func (d *AlertManagerDao) DomainExists(domain string, excludeID uint) bool {
	var count int64
	db := d.db.Model(&model.AlertDomain{}).Where("domain = ?", strings.TrimSpace(domain))
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	return db.Count(&count).Error == nil && count > 0
}

func (d *AlertManagerDao) GetDomain(id uint) (*model.AlertDomain, error) {
	var item model.AlertDomain
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *AlertManagerDao) ListDomains(query model.AlertDomainQuery) ([]model.AlertDomain, int64, error) {
	var list []model.AlertDomain
	var total int64
	db := d.db.Model(&model.AlertDomain{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("domain LIKE ? OR remarks LIKE ? OR CAST(tags AS CHAR) LIKE ?", like, like, like)
	}
	if tag := strings.TrimSpace(query.Tag); tag != "" {
		db = db.Where("CAST(tags AS CHAR) LIKE ?", "%\""+tag+"\"%")
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.Enabled != "" {
		db = db.Where("enabled = ?", query.Enabled)
	}
	if query.ExpireWithinDays > 0 {
		deadline := time.Now().Add(time.Duration(query.ExpireWithinDays) * 24 * time.Hour)
		db = db.Where("certificate_expires_at IS NOT NULL AND certificate_expires_at <= ?", deadline)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("status = 'abnormal' DESC, updated_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *AlertManagerDao) DomainSummary(warnDays int) (model.AlertDomainSummary, error) {
	if warnDays <= 0 {
		warnDays = 30
	}
	var summary model.AlertDomainSummary
	queries := []struct {
		where string
		args  []interface{}
		value *int64
	}{
		{value: &summary.Total},
		{where: "status = ?", args: []interface{}{model.DomainStatusNormal}, value: &summary.Normal},
		{where: "status = ?", args: []interface{}{model.DomainStatusAbnormal}, value: &summary.Abnormal},
		{where: "enabled = ?", args: []interface{}{model.RuleStatusEnabled}, value: &summary.Enabled},
		{where: "certificate_expires_at IS NOT NULL AND certificate_expires_at <= ?", args: []interface{}{time.Now().Add(time.Duration(warnDays) * 24 * time.Hour)}, value: &summary.Expiring},
	}
	for _, item := range queries {
		db := d.db.Model(&model.AlertDomain{})
		if item.where != "" {
			db = db.Where(item.where, item.args...)
		}
		if err := db.Count(item.value).Error; err != nil {
			return summary, err
		}
	}
	return summary, nil
}

func (d *AlertManagerDao) GetBuiltinAlertDatasource() (*model.AlertDatasource, error) {
	var item model.AlertDatasource
	err := d.db.Where("datasource_type = ? AND enabled = ?", model.AlertDatasourceBuiltin, model.RuleStatusEnabled).
		Order("is_default DESC, builtin DESC, id ASC").First(&item).Error
	return &item, err
}

func (d *AlertManagerDao) CreateDomainWithRule(item *model.AlertDomain, rule *model.AlertRule) error {
	now := nowHTime()
	return d.db.Transaction(func(tx *gorm.DB) error {
		rule.CreatedAt = now
		rule.UpdatedAt = now
		if err := tx.Create(rule).Error; err != nil {
			return err
		}
		item.AlertRuleID = rule.ID
		item.CreatedAt = now
		item.UpdatedAt = now
		return tx.Create(item).Error
	})
}

func (d *AlertManagerDao) UpdateDomainWithRule(item *model.AlertDomain, rule *model.AlertRule) error {
	now := nowHTime()
	return d.db.Transaction(func(tx *gorm.DB) error {
		rule.UpdatedAt = now
		if err := tx.Model(&model.AlertRule{}).Where("id = ?", item.AlertRuleID).Updates(map[string]interface{}{
			"name":                       rule.Name,
			"note":                       rule.Note,
			"notify_rule_ids":            rule.NotifyRuleIDs,
			"notify_recovered":           rule.NotifyRecovered,
			"notify_repeat_step_minutes": rule.NotifyRepeatStepMinutes,
			"notify_max_number":          rule.NotifyMaxNumber,
			"labels":                     rule.Labels,
			"annotations":                rule.Annotations,
			"updated_by":                 rule.UpdatedBy,
			"updated_at":                 now,
		}).Error; err != nil {
			return err
		}
		item.UpdatedAt = now
		return tx.Model(&model.AlertDomain{}).Where("id = ?", item.ID).Updates(map[string]interface{}{
			"domain":                 item.Domain,
			"protocol":               item.Protocol,
			"port":                   item.Port,
			"path":                   item.Path,
			"tags":                   item.Tags,
			"expected_status":        item.ExpectedStatus,
			"check_interval_seconds": item.CheckIntervalSeconds,
			"timeout_seconds":        item.TimeoutSeconds,
			"ssl_expire_warn_days":   item.SSLExpireWarnDays,
			"enabled":                item.Enabled,
			"remarks":                item.Remarks,
			"notify_rule_ids":        item.NotifyRuleIDs,
			"next_check_at":          item.NextCheckAt,
			"updated_by":             item.UpdatedBy,
			"updated_at":             item.UpdatedAt,
		}).Error
	})
}

func (d *AlertManagerDao) DeleteDomainWithRule(id, ruleID uint) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.AlertDomain{}, id).Error; err != nil {
			return err
		}
		if ruleID > 0 {
			return tx.Delete(&model.AlertRule{}, ruleID).Error
		}
		return nil
	})
}

func (d *AlertManagerDao) UpdateDomainEnabled(id uint, enabled int, nextCheckAt *util.HTime, operator string) error {
	return d.db.Model(&model.AlertDomain{}).Where("id = ?", id).Updates(map[string]interface{}{
		"enabled":       enabled,
		"next_check_at": nextCheckAt,
		"updated_by":    operator,
		"updated_at":    nowHTime(),
	}).Error
}

func (d *AlertManagerDao) UpdateDomainCheckResult(item model.AlertDomain) error {
	return d.db.Model(&model.AlertDomain{}).Where("id = ?", item.ID).Updates(map[string]interface{}{
		"status":                     item.Status,
		"status_code":                item.StatusCode,
		"response_time_ms":           item.ResponseTimeMS,
		"certificate_expires_at":     item.CertificateExpiresAt,
		"certificate_remaining_days": item.CertificateRemainingDays,
		"certificate_issuer":         item.CertificateIssuer,
		"failure_reason":             item.FailureReason,
		"last_checked_at":            item.LastCheckedAt,
		"next_check_at":              item.NextCheckAt,
		"updated_at":                 item.UpdatedAt,
	}).Error
}

func (d *AlertManagerDao) ListDueDomains(now time.Time, limit int) ([]model.AlertDomain, error) {
	if limit <= 0 {
		limit = 100
	}
	var list []model.AlertDomain
	err := d.db.Where("enabled = ? AND (next_check_at IS NULL OR next_check_at <= ?)", model.RuleStatusEnabled, now).
		Order("next_check_at ASC, id ASC").Limit(limit).Find(&list).Error
	return list, err
}

func (d *AlertManagerDao) ListDomainsByIDs(ids []uint, enabledOnly bool) ([]model.AlertDomain, error) {
	var list []model.AlertDomain
	db := d.db.Model(&model.AlertDomain{})
	if len(ids) > 0 {
		db = db.Where("id IN ?", ids)
	}
	if enabledOnly {
		db = db.Where("enabled = ?", model.RuleStatusEnabled)
	}
	err := db.Order("id ASC").Find(&list).Error
	return list, err
}
