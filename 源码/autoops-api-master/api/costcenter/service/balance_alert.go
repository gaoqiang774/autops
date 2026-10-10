package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	alertengine "dodevops-api/api/alertmanager/engine"
	alertmodel "dodevops-api/api/alertmanager/model"
	"dodevops-api/api/costcenter/model"

	"gorm.io/gorm"
)

const (
	balanceEventLowAmount = "low_balance"
	balanceEventLowDays   = "low_available_days"
	balanceEventQueryErr  = "balance_query_error"
)

type BalanceAlertService struct {
	db *gorm.DB
}

func NewBalanceAlertService(db *gorm.DB) *BalanceAlertService {
	return &BalanceAlertService{db: db}
}

type SaveBalanceAlertRuleReq struct {
	Enabled               int     `json:"enabled"`
	ThresholdAmount       float64 `json:"thresholdAmount"`
	ThresholdDays         int     `json:"thresholdDays"`
	AlertOnQueryError     int     `json:"alertOnQueryError"`
	Severity              string  `json:"severity"`
	RepeatIntervalMinutes int     `json:"repeatIntervalMinutes"`
	NotifyRecovered       int     `json:"notifyRecovered"`
	NotifyRuleIDs         []uint  `json:"notifyRuleIds"`
}

type BalanceAlertRuleResp struct {
	CloudAccountID        uint    `json:"cloudAccountId"`
	AlertRuleID           uint    `json:"alertRuleId"`
	Enabled               int     `json:"enabled"`
	ThresholdAmount       float64 `json:"thresholdAmount"`
	ThresholdDays         int     `json:"thresholdDays"`
	AlertOnQueryError     int     `json:"alertOnQueryError"`
	Severity              string  `json:"severity"`
	RepeatIntervalMinutes int     `json:"repeatIntervalMinutes"`
	NotifyRecovered       int     `json:"notifyRecovered"`
	NotifyRuleIDs         []uint  `json:"notifyRuleIds"`
}

type BalanceCurrencyTotal struct {
	Currency string  `json:"currency"`
	Balance  float64 `json:"balance"`
}

type BalanceOverview struct {
	AccountCount       int64                  `json:"accountCount"`
	MonitoringCount    int64                  `json:"monitoringCount"`
	AlertEnabledCount  int64                  `json:"alertEnabledCount"`
	LowBalanceCount    int64                  `json:"lowBalanceCount"`
	QueryErrorCount    int64                  `json:"queryErrorCount"`
	CurrencyTotals     []BalanceCurrencyTotal `json:"currencyTotals"`
	LastBalanceRefresh string                 `json:"lastBalanceRefresh"`
}

func (s *BalanceAlertService) GetRule(cloudAccountID uint) (*BalanceAlertRuleResp, error) {
	account, err := s.account(cloudAccountID)
	if err != nil {
		return nil, err
	}
	return balanceAlertRuleResp(account), nil
}

func (s *BalanceAlertService) SaveRule(cloudAccountID uint, req SaveBalanceAlertRuleReq) (*BalanceAlertRuleResp, error) {
	account, err := s.account(cloudAccountID)
	if err != nil {
		return nil, err
	}
	req.Enabled = normalizeSwitch(req.Enabled)
	req.AlertOnQueryError = normalizeSwitch(req.AlertOnQueryError)
	req.NotifyRecovered = normalizeSwitch(req.NotifyRecovered)
	if req.ThresholdAmount < 0 {
		return nil, errors.New("余额告警阈值不能小于 0")
	}
	if req.ThresholdDays < 0 || req.ThresholdDays > 365 {
		return nil, errors.New("可用天数阈值应在 0 到 365 之间")
	}
	if req.Enabled == 1 && req.ThresholdAmount == 0 && req.ThresholdDays == 0 && req.AlertOnQueryError == 0 {
		return nil, errors.New("开启告警时至少配置一种告警条件")
	}
	severity, err := normalizeBalanceSeverity(req.Severity)
	if err != nil {
		return nil, err
	}
	repeat := req.RepeatIntervalMinutes
	if repeat == 0 {
		repeat = 1440
	}
	if repeat < 5 || repeat > 43200 {
		return nil, errors.New("重复通知间隔应在 5 到 43200 分钟之间")
	}
	notifyRuleIDs := uniqueUintValues(req.NotifyRuleIDs)
	if err := s.validateNotifyRuleIDs(notifyRuleIDs); err != nil {
		return nil, err
	}
	notifyJSON, _ := json.Marshal(notifyRuleIDs)
	updates := map[string]interface{}{
		"balance_alert_enabled":          req.Enabled,
		"balance_alert_threshold":        req.ThresholdAmount,
		"balance_alert_threshold_days":   req.ThresholdDays,
		"balance_alert_on_query_error":   req.AlertOnQueryError,
		"balance_alert_severity":         severity,
		"balance_alert_repeat_minutes":   repeat,
		"balance_alert_notify_recovered": req.NotifyRecovered,
		"balance_alert_notify_rule_ids":  string(notifyJSON),
	}
	if err := s.db.Model(account).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.db.First(account, cloudAccountID).Error; err != nil {
		return nil, err
	}
	if err := s.syncAlertManagerRule(account, notifyRuleIDs); err != nil {
		return nil, err
	}
	if req.Enabled == 0 {
		if err := s.resolveAll(account, time.Now()); err != nil {
			return nil, err
		}
	} else if account.LastBalanceTime != nil && account.BalanceStatus != model.BalanceStatusError {
		if err := s.EvaluateBalance(account.ID, account.AvailableBalance, account.Currency, time.Now()); err != nil {
			return nil, err
		}
	}
	return balanceAlertRuleResp(account), nil
}

func (s *BalanceAlertService) EvaluateBalance(cloudAccountID uint, balance float64, currency string, now time.Time) error {
	account, err := s.activeAccount(cloudAccountID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.Model(&model.BillCloudAccount{}).Where("id = ?", cloudAccountID).Update("balance_status", model.BalanceStatusNormal).Error
	}
	if err != nil {
		return err
	}
	estimatedDays := s.EstimatedAvailableDays(cloudAccountID, balance, now)
	amountFiring, daysFiring := balanceAlertConditions(*account, balance, estimatedDays)
	currency = normalizedCurrency(currency)
	if err := s.publish(account, balanceEventLowAmount, amountFiring, balance, currency,
		fmt.Sprintf("当前余额 %.2f %s，低于告警阈值 %.2f %s", balance, currency, account.BalanceAlertThreshold, currency), now); err != nil {
		return err
	}
	if err := s.publish(account, balanceEventLowDays, daysFiring, balance, currency,
		fmt.Sprintf("预计余额可用 %.1f 天，低于告警阈值 %d 天", valueOrZero(estimatedDays), account.BalanceAlertThresholdDays), now); err != nil {
		return err
	}
	if err := s.publish(account, balanceEventQueryErr, false, balance, currency, "余额查询已恢复", now); err != nil {
		return err
	}
	status := model.BalanceStatusNormal
	if amountFiring || daysFiring {
		status = model.BalanceStatusLow
	}
	return s.db.Model(account).Update("balance_status", status).Error
}

func (s *BalanceAlertService) EvaluateQueryError(cloudAccountID uint, message string, now time.Time) error {
	account, err := s.activeAccount(cloudAccountID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if account.BalanceAlertOnQueryError != 1 {
		return nil
	}
	return s.publish(account, balanceEventQueryErr, true, 0, normalizedCurrency(account.Currency), "余额查询失败："+message, now)
}

func (s *BalanceAlertService) EstimatedAvailableDays(cloudAccountID uint, balance float64, now time.Time) *float64 {
	if balance < 0 {
		value := 0.0
		return &value
	}
	months := []string{
		now.AddDate(0, -1, 0).Format("2006-01"),
		now.AddDate(0, -2, 0).Format("2006-01"),
		now.AddDate(0, -3, 0).Format("2006-01"),
	}
	var completed []model.BillMonthlySummary
	if err := s.db.Where("cloud_account_id = ? AND month IN ?", cloudAccountID, months).Find(&completed).Error; err != nil {
		return nil
	}
	totalCost := 0.0
	totalDays := 0
	for _, item := range completed {
		if item.TotalCost <= 0 {
			continue
		}
		parsed, err := time.ParseInLocation("2006-01", item.Month, now.Location())
		if err != nil {
			continue
		}
		totalCost += item.TotalCost
		totalDays += parsed.AddDate(0, 1, -1).Day()
	}
	if totalCost <= 0 || totalDays == 0 {
		var current model.BillMonthlySummary
		err := s.db.Where("cloud_account_id = ? AND month = ?", cloudAccountID, now.Format("2006-01")).First(&current).Error
		if err != nil || current.TotalCost <= 0 {
			return nil
		}
		totalCost = current.TotalCost
		totalDays = now.Day()
	}
	dailyCost := totalCost / float64(totalDays)
	if dailyCost <= 0 {
		return nil
	}
	value := balance / dailyCost
	return &value
}

func (s *BalanceAlertService) Overview() (*BalanceOverview, error) {
	var accounts []model.BillCloudAccount
	if err := s.db.Where("enabled = ?", 1).Find(&accounts).Error; err != nil {
		return nil, err
	}
	result := &BalanceOverview{AccountCount: int64(len(accounts)), CurrencyTotals: []BalanceCurrencyTotal{}}
	totals := map[string]float64{}
	for _, account := range accounts {
		if account.BalanceSyncEnabled == 1 {
			result.MonitoringCount++
		}
		if account.BalanceAlertEnabled == 1 {
			result.AlertEnabledCount++
		}
		switch account.BalanceStatus {
		case model.BalanceStatusLow:
			result.LowBalanceCount++
		case model.BalanceStatusError:
			result.QueryErrorCount++
		}
		currency := normalizedCurrency(account.Currency)
		totals[currency] += account.AvailableBalance
		formatted := formatTimePointer(account.LastBalanceTime)
		if formatted > result.LastBalanceRefresh {
			result.LastBalanceRefresh = formatted
		}
	}
	for currency, balance := range totals {
		result.CurrencyTotals = append(result.CurrencyTotals, BalanceCurrencyTotal{Currency: currency, Balance: balance})
	}
	sort.Slice(result.CurrencyTotals, func(i, j int) bool { return result.CurrencyTotals[i].Currency < result.CurrencyTotals[j].Currency })
	return result, nil
}

func (s *BalanceAlertService) account(id uint) (*model.BillCloudAccount, error) {
	var account model.BillCloudAccount
	if err := s.db.First(&account, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("云账号不存在")
		}
		return nil, err
	}
	return &account, nil
}

func (s *BalanceAlertService) activeAccount(id uint) (*model.BillCloudAccount, error) {
	var account model.BillCloudAccount
	err := s.db.Where("id = ? AND balance_alert_enabled = ?", id, 1).First(&account).Error
	return &account, err
}

func (s *BalanceAlertService) validateNotifyRuleIDs(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	var count int64
	if err := s.db.Model(&alertmodel.AlertNotifyRule{}).Where("id IN ? AND enabled = ?", ids, 1).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(ids)) {
		return errors.New("通知策略不存在或已停用")
	}
	return nil
}

func (s *BalanceAlertService) syncAlertManagerRule(account *model.BillCloudAccount, notifyRuleIDs []uint) error {
	var datasource alertmodel.AlertDatasource
	if err := s.db.Where("datasource_type = ? AND enabled = ?", alertmodel.AlertDatasourceBuiltin, 1).
		Order("is_default DESC, id ASC").First(&datasource).Error; err != nil {
		return errors.New("告警中心内置数据源未初始化")
	}
	alertRule := alertmodel.AlertRule{}
	if account.BalanceAlertRuleID > 0 {
		_ = s.db.First(&alertRule, account.BalanceAlertRuleID).Error
	}
	accountName := firstNonBlank(account.AccountName, account.AccountID, fmt.Sprintf("账号%d", account.ID))
	alertRule.Name = "云账号余额告警 - " + accountName
	alertRule.Note = "账单中心余额监控产生的外部告警"
	alertRule.DatasourceID = datasource.ID
	alertRule.DatasourceType = datasource.DatasourceType
	alertRule.Query = fmt.Sprintf("external://costcenter/balance/%d", account.ID)
	alertRule.ConditionOperator = "<"
	alertRule.ConditionValue = account.BalanceAlertThreshold
	alertRule.CronPattern = "@every 1h"
	alertRule.EvalIntervalSeconds = int64(normalizeBalanceInterval(account.BalanceSyncIntervalMinutes) * 60)
	alertRule.Severity = account.BalanceAlertSeverity
	alertRule.Enabled = 0
	alertRule.TimeZone = "Asia/Shanghai"
	alertRule.Labels = alertmodel.JSONMap{"service": "costcenter", "provider": normalizeOperator(account.Operator), "account": accountName}
	alertRule.Annotations = alertmodel.JSONMap{"summary": accountName + " 云账户余额告警"}
	alertRule.BusinessID = alertmodel.AlertBusinessRouteOperations
	alertRule.NoDataPolicy = "ignore"
	alertRule.ErrorPolicy = "ignore"
	alertRule.NotifyRuleIDs = alertmodel.JSONUintArray(notifyRuleIDs)
	alertRule.NotifyRecovered = account.BalanceAlertNotifyRecovered
	alertRule.NotifyRepeatStepMinutes = int64(account.BalanceAlertRepeatMinutes)
	if alertRule.ID == 0 {
		if err := s.db.Create(&alertRule).Error; err != nil {
			return err
		}
	} else if err := s.db.Save(&alertRule).Error; err != nil {
		return err
	}
	account.BalanceAlertRuleID = alertRule.ID
	return s.db.Model(account).Update("balance_alert_rule_id", alertRule.ID).Error
}

func (s *BalanceAlertService) publish(account *model.BillCloudAccount, eventType string, firing bool, balance float64, currency, message string, now time.Time) error {
	if account.BalanceAlertRuleID == 0 {
		return nil
	}
	var rule alertmodel.AlertRule
	if err := s.db.First(&rule, account.BalanceAlertRuleID).Error; err != nil {
		return err
	}
	accountName := firstNonBlank(account.AccountName, account.AccountID, fmt.Sprintf("账号%d", account.ID))
	_, err := alertengine.PublishExternalEvent(context.Background(), s.db, alertengine.ExternalEventInput{
		Rule:        rule,
		Fingerprint: fmt.Sprintf("costcenter:balance:%d:%s", account.ID, eventType),
		Labels: alertmodel.JSONMap{
			"service": "costcenter", "provider": normalizeOperator(account.Operator), "account": accountName,
			"cloudAccountId": strconv.FormatUint(uint64(account.ID), 10), "eventType": eventType, "currency": currency,
		},
		Annotations:      alertmodel.JSONMap{"summary": message, "description": message},
		Value:            strconv.FormatFloat(balance, 'f', 6, 64),
		Severity:         account.BalanceAlertSeverity,
		TriggerCondition: message,
		EventDetail:      message,
		Firing:           firing,
		Now:              now,
	})
	return err
}

func (s *BalanceAlertService) resolveAll(account *model.BillCloudAccount, now time.Time) error {
	for _, eventType := range []string{balanceEventLowAmount, balanceEventLowDays, balanceEventQueryErr} {
		if err := s.publish(account, eventType, false, account.AvailableBalance, normalizedCurrency(account.Currency), "余额告警规则已停用", now); err != nil {
			return err
		}
	}
	return nil
}

func balanceAlertRuleResp(account *model.BillCloudAccount) *BalanceAlertRuleResp {
	ids := []uint{}
	_ = json.Unmarshal([]byte(account.BalanceAlertNotifyRuleIDs), &ids)
	severity := account.BalanceAlertSeverity
	if severity == "" {
		severity = "warning"
	}
	repeat := account.BalanceAlertRepeatMinutes
	if repeat == 0 {
		repeat = 1440
	}
	return &BalanceAlertRuleResp{
		CloudAccountID: account.ID, AlertRuleID: account.BalanceAlertRuleID,
		Enabled: account.BalanceAlertEnabled, ThresholdAmount: account.BalanceAlertThreshold,
		ThresholdDays: account.BalanceAlertThresholdDays, AlertOnQueryError: account.BalanceAlertOnQueryError,
		Severity: severity, RepeatIntervalMinutes: repeat, NotifyRecovered: account.BalanceAlertNotifyRecovered,
		NotifyRuleIDs: ids,
	}
}

func normalizeBalanceSeverity(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "warning", nil
	}
	switch value {
	case "info", "warning", "critical":
		return value, nil
	default:
		return "", errors.New("告警级别仅支持 info、warning、critical")
	}
}

func uniqueUintValues(values []uint) []uint {
	seen := map[uint]struct{}{}
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func balanceAlertConditions(account model.BillCloudAccount, balance float64, estimatedDays *float64) (bool, bool) {
	amountFiring := account.BalanceAlertThreshold > 0 && balance < account.BalanceAlertThreshold
	daysFiring := account.BalanceAlertThresholdDays > 0 && estimatedDays != nil && *estimatedDays < float64(account.BalanceAlertThresholdDays)
	return amountFiring, daysFiring
}
