package service

import (
	"context"
	"crypto/sha1"
	"dodevops-api/api/alertmanager/dao"
	alertengine "dodevops-api/api/alertmanager/engine"
	"dodevops-api/api/alertmanager/model"
	alertnotify "dodevops-api/api/alertmanager/notify"
	"dodevops-api/common/result"
	"dodevops-api/common/util"
	"dodevops-api/pkg/jwt"
	"dodevops-api/pkg/log"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prommodel "github.com/prometheus/common/model"
	yamlv2 "gopkg.in/yaml.v2"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

const defaultVictoriaLogsPreviewWindowSeconds int64 = 300

type AlertManagerServiceInterface interface {
	CreateRuleGroup(c *gin.Context, item model.AlertRuleGroup)
	UpdateRuleGroup(c *gin.Context, item model.AlertRuleGroup)
	DeleteRuleGroup(c *gin.Context, id uint)
	GetRuleGroup(c *gin.Context, id uint)
	ListRuleGroups(c *gin.Context, query model.PageQuery)
	AllRuleGroups(c *gin.Context)

	CreateRule(c *gin.Context, item model.AlertRule)
	UpdateRule(c *gin.Context, item model.AlertRule)
	DeleteRule(c *gin.Context, id uint)
	GetRule(c *gin.Context, id uint)
	ListRules(c *gin.Context, query model.AlertRuleQuery)
	ChangeRuleStatus(c *gin.Context, id uint, enabled int)
	BatchUpdateRuleDatasource(c *gin.Context, req model.BatchUpdateRuleDatasourceRequest)
	BatchUpdateRuleNotifyRules(c *gin.Context, req model.BatchUpdateRuleNotifyRulesRequest)
	ImportPrometheusRules(c *gin.Context, req model.ImportPrometheusRulesRequest)
	PreviewRuleQuery(c *gin.Context, req model.PreviewRuleQueryRequest)
	QueryMetricsRange(c *gin.Context, req model.AlertMetricsQueryRangeRequest)
	QueryMetrics(c *gin.Context, req model.AlertMetricsQueryRequest)
	GetMetricTargets(c *gin.Context, req model.AlertMetricsQueryRequest)

	CreateDatasource(c *gin.Context, item model.AlertDatasource)
	UpdateDatasource(c *gin.Context, item model.AlertDatasource)
	DeleteDatasource(c *gin.Context, id uint)
	GetDatasource(c *gin.Context, id uint)
	ListDatasources(c *gin.Context, query model.AlertDatasourceQuery)
	ListDatasourceTypes(c *gin.Context)
	ListDatasourceOptions(c *gin.Context)
	ChangeDatasourceStatus(c *gin.Context, id uint, enabled int)

	CreateBuiltinRuleTemplate(c *gin.Context, item model.AlertBuiltinRuleTemplate)
	UpdateBuiltinRuleTemplate(c *gin.Context, item model.AlertBuiltinRuleTemplate)
	DeleteBuiltinRuleTemplate(c *gin.Context, id uint)
	GetBuiltinRuleTemplate(c *gin.Context, id uint)
	ListBuiltinRuleTemplates(c *gin.Context, query model.AlertBuiltinRuleTemplateQuery)
	PreviewPrometheusTemplates(c *gin.Context, query model.PreviewPrometheusTemplateQuery)
	ListPrometheusTemplateImportTypes(c *gin.Context)
	ImportPrometheusTemplates(c *gin.Context, req model.ImportPrometheusTemplatesRequest)
	CreateBuiltinRuleTemplateGroup(c *gin.Context, item model.AlertBuiltinRuleTemplateGroup)
	UpdateBuiltinRuleTemplateGroup(c *gin.Context, item model.AlertBuiltinRuleTemplateGroup)
	DeleteBuiltinRuleTemplateGroup(c *gin.Context, id uint)
	GetBuiltinRuleTemplateGroup(c *gin.Context, id uint)
	ListBuiltinRuleTemplateGroups(c *gin.Context, query model.AlertBuiltinRuleTemplateGroupQuery)
	AllBuiltinRuleTemplateGroupTree(c *gin.Context)
	ListBuiltinRuleTemplateGroupOptions(c *gin.Context, query model.AlertBuiltinRuleTemplateGroupOptionsQuery)
	ApplyBuiltinRuleTemplate(c *gin.Context, id uint, req model.ApplyBuiltinRuleTemplateRequest)

	ListActiveEvents(c *gin.Context, query model.AlertEventQuery)
	ListActiveEventBusinessOptions(c *gin.Context)
	ListActiveEventGroups(c *gin.Context, query model.AlertEventQuery)
	ListHistoryEvents(c *gin.Context, query model.AlertEventQuery)
	GetEventDetail(c *gin.Context, eventType string, eventID uint64)
	ListTimeline(c *gin.Context, eventID uint64)
	ClaimActiveEvent(c *gin.Context, eventID uint64)
	SilenceActiveEvent(c *gin.Context, eventID uint64, req model.SilenceEventRequest)
	UnsilenceActiveEvent(c *gin.Context, eventID uint64)
	GetDashboardCharts(c *gin.Context, query model.AlertDashboardQuery)
	GetDashboardScreen(c *gin.Context, query model.AlertDashboardQuery)
	GetDashboardAlertNameTop(c *gin.Context, query model.AlertDashboardQuery)
	GetDashboardNotifyTrend(c *gin.Context, query model.AlertDashboardQuery)
	GetDashboardBusinessRouteTop(c *gin.Context, query model.AlertDashboardQuery)

	CreateSilenceRule(c *gin.Context, item model.AlertSilenceRule)
	UpdateSilenceRule(c *gin.Context, item model.AlertSilenceRule)
	DeleteSilenceRule(c *gin.Context, id uint)
	GetSilenceRule(c *gin.Context, id uint)
	ListSilenceRules(c *gin.Context, query model.AlertSilenceRuleQuery)

	CreateNotifyChannel(c *gin.Context, item model.AlertNotifyChannel)
	UpdateNotifyChannel(c *gin.Context, item model.AlertNotifyChannel)
	DeleteNotifyChannel(c *gin.Context, id uint)
	GetNotifyChannel(c *gin.Context, id uint)
	ListNotifyChannels(c *gin.Context, query model.AlertNotifyChannelQuery)
	TestNotifyChannel(c *gin.Context, id uint, req model.AlertNotifyChannelTestRequest)
	HandleFeishuCallback(c *gin.Context)

	CreateNotifyGroup(c *gin.Context, item model.AlertNotifyGroup)
	UpdateNotifyGroup(c *gin.Context, item model.AlertNotifyGroup)
	DeleteNotifyGroup(c *gin.Context, id uint)
	GetNotifyGroup(c *gin.Context, id uint)
	ListNotifyGroups(c *gin.Context, query model.AlertNotifyGroupQuery)

	CreateNotifyRouteRule(c *gin.Context, item model.AlertNotifyRouteRule)
	UpdateNotifyRouteRule(c *gin.Context, item model.AlertNotifyRouteRule)
	DeleteNotifyRouteRule(c *gin.Context, id uint)
	GetNotifyRouteRule(c *gin.Context, id uint)
	ListNotifyRouteRules(c *gin.Context, query model.AlertNotifyRouteRuleQuery)

	CreateNotifyTemplate(c *gin.Context, item model.AlertNotifyTemplate)
	UpdateNotifyTemplate(c *gin.Context, item model.AlertNotifyTemplate)
	DeleteNotifyTemplate(c *gin.Context, id uint)
	GetNotifyTemplate(c *gin.Context, id uint)
	ListNotifyTemplates(c *gin.Context, query model.AlertNotifyTemplateQuery)
	PreviewNotifyTemplate(c *gin.Context, id uint, req model.AlertNotifyTemplatePreviewRequest)

	CreateNotifyRule(c *gin.Context, item model.AlertNotifyRule)
	UpdateNotifyRule(c *gin.Context, item model.AlertNotifyRule)
	DeleteNotifyRule(c *gin.Context, id uint)
	GetNotifyRule(c *gin.Context, id uint)
	ListNotifyRules(c *gin.Context, query model.AlertNotifyRuleQuery)
	ListNotifyRecords(c *gin.Context, query model.AlertNotifyRecordQuery)
	GetNotifyRecordStats(c *gin.Context, query model.AlertNotifyRecordQuery)
}

type AlertManagerServiceImpl struct {
	dao *dao.AlertManagerDao
}

func GetAlertManagerService() AlertManagerServiceInterface {
	return &AlertManagerServiceImpl{dao: dao.NewAlertManagerDao()}
}

func newAlertManagerService() *AlertManagerServiceImpl {
	return &AlertManagerServiceImpl{dao: dao.NewAlertManagerDao()}
}

func (s *AlertManagerServiceImpl) CreateRuleGroup(c *gin.Context, item model.AlertRuleGroup) {
	operator := currentOperator(c)
	item.CreatedBy = operator
	item.UpdatedBy = operator
	if err := validateRuleGroup(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if s.dao.RuleGroupNameExists(item.Name, item.ParentID, 0) {
		result.Failed(c, 400, "规则分组名称已存在")
		return
	}
	if item.Enabled == 0 {
		item.Enabled = model.RuleStatusEnabled
	}
	if err := s.dao.CreateRuleGroup(&item); err != nil {
		result.Failed(c, 500, "创建规则分组失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) UpdateRuleGroup(c *gin.Context, item model.AlertRuleGroup) {
	if item.ID == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item.CreatedBy = ""
	item.UpdatedBy = currentOperator(c)
	if err := validateRuleGroup(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if s.dao.RuleGroupNameExists(item.Name, item.ParentID, item.ID) {
		result.Failed(c, 400, "规则分组名称已存在")
		return
	}
	if err := s.dao.UpdateRuleGroup(&item); err != nil {
		result.Failed(c, 500, "更新规则分组失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) DeleteRuleGroup(c *gin.Context, id uint) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	groups, err := s.dao.AllRuleGroups()
	if err != nil {
		result.Failed(c, 500, "查询规则分组失败: "+err.Error())
		return
	}
	groupIDs := descendantRuleGroupIDs(groups, id)
	if len(groupIDs) == 0 {
		result.Failed(c, 404, "规则分组不存在")
		return
	}
	ruleIDs, err := s.dao.RuleIDsByGroupIDs(groupIDs)
	if err != nil {
		result.Failed(c, 500, "查询分组下告警规则失败: "+err.Error())
		return
	}
	deletedRules, deletedGroups, err := s.dao.DeleteRuleGroupsWithRules(groupIDs)
	if err != nil {
		result.Failed(c, 500, "删除规则分组失败: "+err.Error())
		return
	}
	for _, ruleID := range ruleIDs {
		alertengine.StopRule(ruleID)
	}
	result.Success(c, gin.H{
		"deletedGroupCount": deletedGroups,
		"deletedRuleCount":  deletedRules,
	})
}

func (s *AlertManagerServiceImpl) GetRuleGroup(c *gin.Context, id uint) {
	item, err := s.dao.GetRuleGroup(id)
	if err != nil {
		result.Failed(c, 500, "查询规则分组失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) ListRuleGroups(c *gin.Context, query model.PageQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	list, total, err := s.dao.ListRuleGroups(query)
	if err != nil {
		result.Failed(c, 500, "查询规则分组失败: "+err.Error())
		return
	}
	if err := s.fillRuleGroupCounts(list); err != nil {
		result.Failed(c, 500, "统计规则分组数量失败: "+err.Error())
		return
	}
	result.SuccessWithPage(c, list, total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) AllRuleGroups(c *gin.Context) {
	list, err := s.dao.AllRuleGroups()
	if err != nil {
		result.Failed(c, 500, "查询规则分组失败: "+err.Error())
		return
	}
	if err := s.fillRuleGroupCounts(list); err != nil {
		result.Failed(c, 500, "统计规则分组数量失败: "+err.Error())
		return
	}
	result.Success(c, buildRuleGroupTree(list))
}

func (s *AlertManagerServiceImpl) fillRuleGroupCounts(list []model.AlertRuleGroup) error {
	groupIDs := make([]uint, 0, len(list))
	for _, item := range list {
		if item.ID > 0 {
			groupIDs = append(groupIDs, item.ID)
		}
	}
	counts, err := s.dao.CountRulesByGroupIDs(groupIDs)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].RuleCount = counts[list[i].ID]
	}
	return nil
}

func (s *AlertManagerServiceImpl) CreateRule(c *gin.Context, item model.AlertRule) {
	operator := currentOperator(c)
	item.CreatedBy = operator
	item.UpdatedBy = operator
	applyRuleDefaults(&item)
	if err := s.normalizeLogAlertAssociation(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if err := validateRule(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if err := s.validateRuleNotifyRuleIDs(item.NotifyRuleIDs); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if s.dao.RuleNameExists(item.Name, 0) {
		result.Failed(c, 400, "告警规则名称已存在")
		return
	}
	if err := s.dao.CreateRule(&item); err != nil {
		result.Failed(c, 500, "创建告警规则失败: "+err.Error())
		return
	}
	if item.Enabled == model.RuleStatusEnabled {
		alertengine.ResumeRule(item.ID)
	}
	fillRuleDurationCount(&item)
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) UpdateRule(c *gin.Context, item model.AlertRule) {
	if item.ID == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item.CreatedBy = ""
	item.UpdatedBy = currentOperator(c)
	applyRuleDefaults(&item)
	if err := s.normalizeLogAlertAssociation(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if err := validateRule(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if err := s.validateRuleNotifyRuleIDs(item.NotifyRuleIDs); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if s.dao.RuleNameExists(item.Name, item.ID) {
		result.Failed(c, 400, "告警规则名称已存在")
		return
	}
	if err := s.dao.UpdateRule(&item); err != nil {
		result.Failed(c, 500, "更新告警规则失败: "+err.Error())
		return
	}
	if item.Enabled == model.RuleStatusEnabled {
		alertengine.ResumeRule(item.ID)
	} else {
		alertengine.StopRule(item.ID)
	}
	fillRuleDurationCount(&item)
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) DeleteRule(c *gin.Context, id uint) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	if err := s.dao.DeleteRule(id); err != nil {
		result.Failed(c, 500, "删除告警规则失败: "+err.Error())
		return
	}
	alertengine.StopRule(id)
	result.Success(c, nil)
}

func (s *AlertManagerServiceImpl) GetRule(c *gin.Context, id uint) {
	item, err := s.dao.GetRule(id)
	if err != nil {
		result.Failed(c, 500, "查询告警规则失败: "+err.Error())
		return
	}
	fillRuleDurationCount(item)
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) ListRules(c *gin.Context, query model.AlertRuleQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	list, total, err := s.dao.ListRules(query)
	if err != nil {
		result.Failed(c, 500, "查询告警规则失败: "+err.Error())
		return
	}
	fillRuleDurationCounts(list)
	result.SuccessWithPage(c, list, total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) ChangeRuleStatus(c *gin.Context, id uint, enabled int) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item, err := s.dao.GetRule(id)
	if err != nil {
		result.Failed(c, 500, "查询告警规则失败: "+err.Error())
		return
	}
	if enabled != model.RuleStatusEnabled {
		enabled = model.RuleStatusDisabled
	}
	wasEnabled := item.Enabled == model.RuleStatusEnabled
	item.Enabled = enabled
	item.UpdatedBy = currentOperator(c)
	if err := s.dao.UpdateRuleStatus(item.ID, enabled, item.UpdatedBy); err != nil {
		result.Failed(c, 500, "更新告警规则状态失败: "+err.Error())
		return
	}
	if enabled == model.RuleStatusDisabled {
		alertengine.StopRule(item.ID)
	} else {
		alertengine.ResumeRule(item.ID)
	}
	if wasEnabled && enabled == model.RuleStatusDisabled {
		if _, err := s.dao.ResolveActiveEventsByRuleID(item.ID, item.UpdatedBy, "规则已禁用，告警自动恢复"); err != nil {
			result.Failed(c, 500, "收敛活跃告警失败: "+err.Error())
			return
		}
	}
	fillRuleDurationCount(item)
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) BatchUpdateRuleDatasource(c *gin.Context, req model.BatchUpdateRuleDatasourceRequest) {
	ruleIDs := normalizeUintIDs(req.RuleIDs)
	if len(ruleIDs) == 0 {
		result.Failed(c, 400, "ruleIds is required")
		return
	}
	if req.DatasourceID == 0 {
		result.Failed(c, 400, "datasourceId is required")
		return
	}
	ds, err := s.dao.GetDatasource(req.DatasourceID)
	if err != nil {
		result.Failed(c, 500, "查询告警数据源失败: "+err.Error())
		return
	}
	if ds.Enabled != model.RuleStatusEnabled {
		result.Failed(c, 400, "目标数据源已禁用")
		return
	}
	operator := currentOperator(c)
	if err := s.dao.BatchUpdateRuleDatasource(ruleIDs, ds.ID, ds.DatasourceType, operator); err != nil {
		result.Failed(c, 500, "批量修改规则数据源失败: "+err.Error())
		return
	}
	alertengine.TriggerReload()
	result.Success(c, gin.H{
		"updated":        len(ruleIDs),
		"ruleIds":        ruleIDs,
		"datasourceId":   ds.ID,
		"datasourceType": ds.DatasourceType,
	})
}

func (s *AlertManagerServiceImpl) BatchUpdateRuleNotifyRules(c *gin.Context, req model.BatchUpdateRuleNotifyRulesRequest) {
	ruleIDs := normalizeUintIDs(req.RuleIDs)
	if len(ruleIDs) == 0 {
		result.Failed(c, 400, "ruleIds is required")
		return
	}
	notifyRuleIDs := model.JSONUintArray(normalizeUintIDs([]uint(req.NotifyRuleIDs)))
	if notifyRuleIDs == nil {
		notifyRuleIDs = model.JSONUintArray{}
	}
	if err := s.validateRuleNotifyRuleIDs(notifyRuleIDs); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	operator := currentOperator(c)
	if err := s.dao.BatchUpdateRuleNotifyRules(ruleIDs, notifyRuleIDs, operator); err != nil {
		result.Failed(c, 500, "批量修改规则告警策略失败: "+err.Error())
		return
	}
	alertengine.TriggerReload()
	result.Success(c, gin.H{
		"updated":       len(ruleIDs),
		"ruleIds":       ruleIDs,
		"notifyRuleIds": notifyRuleIDs,
	})
}

func (s *AlertManagerServiceImpl) PreviewRuleQuery(c *gin.Context, req model.PreviewRuleQueryRequest) {
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		result.Failed(c, 400, "query is required")
		return
	}

	ds, err := s.resolveQueryDatasource(req)
	if err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if ds.Enabled != model.RuleStatusEnabled {
		result.Failed(c, 400, "告警数据源已禁用")
		return
	}
	if ds.DatasourceType == model.AlertDatasourceVictoriaLogs {
		s.previewVictoriaLogsRuleQuery(c, *ds, req)
		return
	}
	if !isPromQLDatasourceType(ds.DatasourceType) {
		result.Failed(c, 400, "仅支持 Prometheus/VictoriaMetrics 查询预览")
		return
	}
	if strings.TrimSpace(ds.URL) == "" {
		result.Failed(c, 400, "告警数据源 URL 为空")
		return
	}

	promAPI, err := newAlertDatasourcePromAPI(*ds)
	if err != nil {
		result.Failed(c, 500, "创建查询客户端失败: "+err.Error())
		return
	}

	queryTime := time.Now()
	if req.Time > 0 {
		queryTime = time.UnixMilli(req.Time)
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	value, warnings, err := promAPI.Query(ctx, req.Query, queryTime)
	if err != nil {
		result.Failed(c, 500, "查询失败: "+err.Error())
		return
	}

	limit := normalizePreviewLimit(req.Limit)
	previewValue, total, limited := limitPrometheusValue(value, limit)
	result.Success(c, model.PreviewRuleQueryResponse{
		DatasourceID:   ds.ID,
		DatasourceType: ds.DatasourceType,
		Query:          req.Query,
		ResultType:     prometheusValueTypeString(value.Type()),
		Total:          total,
		Limited:        limited,
		Result:         previewValue,
		Warnings:       prometheusWarningsToStrings(warnings),
		ExecutedAt:     queryTime.UnixMilli(),
	})
}

func (s *AlertManagerServiceImpl) previewVictoriaLogsRuleQuery(c *gin.Context, ds model.AlertDatasource, req model.PreviewRuleQueryRequest) {
	if strings.TrimSpace(ds.URL) == "" {
		result.Failed(c, 400, "告警数据源 URL 为空")
		return
	}

	queryTime := time.Now()
	if req.Time > 0 {
		queryTime = time.UnixMilli(req.Time)
	}
	windowSeconds := req.WindowSeconds
	if windowSeconds <= 0 {
		windowSeconds = defaultVictoriaLogsPreviewWindowSeconds
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	ruleConfig := model.JSONMap{}
	if strings.TrimSpace(req.ProjectCode) != "" {
		ruleConfig["projectCode"] = strings.TrimSpace(req.ProjectCode)
	}
	if strings.TrimSpace(req.StoreCode) != "" {
		ruleConfig["storeCode"] = strings.TrimSpace(req.StoreCode)
	}
	countResult, err := alertengine.QueryVictoriaLogsCount(ctx, ds, req.Query, queryTime, windowSeconds, ruleConfig)
	if err != nil {
		result.Failed(c, 500, "查询失败: "+err.Error())
		return
	}
	result.Success(c, model.PreviewRuleQueryResponse{
		DatasourceID:   ds.ID,
		DatasourceType: ds.DatasourceType,
		Query:          req.Query,
		ResultType:     "logs_count",
		Total:          1,
		Limited:        false,
		Result:         countResult,
		ExecutedAt:     queryTime.UnixMilli(),
	})
}

func (s *AlertManagerServiceImpl) QueryMetricsRange(c *gin.Context, req model.AlertMetricsQueryRangeRequest) {
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		result.Failed(c, 400, "promql 不能为空")
		return
	}
	if req.Start <= 0 {
		result.Failed(c, 400, "start 不能为空")
		return
	}
	if req.End <= 0 {
		result.Failed(c, 400, "end 不能为空")
		return
	}

	ds, promAPI, err := s.metricPromAPI(req.DatasourceID, req.DatasourceType)
	if err != nil {
		result.Failed(c, 400, err.Error())
		return
	}

	step := strings.TrimSpace(req.Step)
	if step == "" {
		step = "5m"
	}
	stepDuration, err := time.ParseDuration(step)
	if err != nil {
		stepDuration = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	value, warnings, err := promAPI.QueryRange(ctx, req.Query, v1.Range{
		Start: time.UnixMilli(req.Start),
		End:   time.UnixMilli(req.End),
		Step:  stepDuration,
	})
	if err != nil {
		result.Failed(c, 500, "查询失败: "+err.Error())
		return
	}
	if len(warnings) > 0 {
		log.Log().Warnf("alertmanager metrics query_range warnings: datasource=%d query=%s warnings=%v", ds.ID, req.Query, warnings)
	}
	result.Success(c, value)
}

func (s *AlertManagerServiceImpl) QueryMetrics(c *gin.Context, req model.AlertMetricsQueryRequest) {
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		result.Failed(c, 400, "promql 不能为空")
		return
	}

	ds, promAPI, err := s.metricPromAPI(req.DatasourceID, req.DatasourceType)
	if err != nil {
		result.Failed(c, 400, err.Error())
		return
	}

	queryTime := time.Now()
	if req.Time > 0 {
		queryTime = time.UnixMilli(req.Time)
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	value, warnings, err := promAPI.Query(ctx, req.Query, queryTime)
	if err != nil {
		result.Failed(c, 500, "即时查询失败: "+err.Error())
		return
	}
	if len(warnings) > 0 {
		log.Log().Warnf("alertmanager metrics query warnings: datasource=%d query=%s warnings=%v", ds.ID, req.Query, warnings)
	}
	result.Success(c, value)
}

func (s *AlertManagerServiceImpl) GetMetricTargets(c *gin.Context, req model.AlertMetricsQueryRequest) {
	ds, promAPI, err := s.metricPromAPI(req.DatasourceID, req.DatasourceType)
	if err != nil {
		result.Failed(c, 400, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	value, warnings, err := promAPI.Query(ctx, `count by (instance, job) (ifHCInOctets)`, time.Now())
	if err != nil {
		result.Failed(c, 500, "查询设备列表失败: "+err.Error())
		return
	}
	if len(warnings) > 0 {
		log.Log().Warnf("alertmanager metrics targets warnings: datasource=%d warnings=%v", ds.ID, warnings)
	}

	vector, ok := value.(prommodel.Vector)
	if !ok {
		result.Failed(c, 500, "查询结果格式异常")
		return
	}

	targets := make([]model.AlertMetricTarget, 0, len(vector))
	seen := make(map[string]struct{}, len(vector))
	for _, sample := range vector {
		instance := string(sample.Metric["instance"])
		if instance == "" {
			continue
		}
		if _, ok := seen[instance]; ok {
			continue
		}
		seen[instance] = struct{}{}
		ip := instance
		if idx := strings.IndexByte(ip, ':'); idx >= 0 {
			ip = ip[:idx]
		}
		targets = append(targets, model.AlertMetricTarget{
			Instance: instance,
			IP:       ip,
			Job:      string(sample.Metric["job"]),
		})
	}
	result.Success(c, targets)
}

func (s *AlertManagerServiceImpl) ImportPrometheusRules(c *gin.Context, req model.ImportPrometheusRulesRequest) {
	content := firstNonEmpty(req.Content, req.YAML, req.YAMLContent)
	if strings.TrimSpace(content) == "" {
		result.Failed(c, 400, "content is required")
		return
	}
	if strings.TrimSpace(req.DatasourceType) == "" {
		req.DatasourceType = model.AlertDatasourcePrometheus
	}
	operator := currentOperator(c)
	parsed, err := parsePrometheusRuleContent(content)
	if err != nil {
		result.Failed(c, 400, "解析 Prometheus Rule YAML 失败: "+err.Error())
		return
	}
	if len(parsed) == 0 {
		result.Failed(c, 400, "未发现可导入的 alert 规则")
		return
	}

	enabled := model.RuleStatusDisabled
	if req.DefaultEnabled != nil && *req.DefaultEnabled == model.RuleStatusEnabled {
		enabled = model.RuleStatusEnabled
	}

	imported := make([]model.AlertRule, 0)
	skipped := make([]model.ImportPrometheusRuleSkipped, 0)
	seen := make(map[string]bool)
	for _, item := range parsed {
		rule, skipReason := buildRuleFromPrometheusRule(item, req)
		if skipReason != "" {
			skipped = append(skipped, model.ImportPrometheusRuleSkipped{Name: item.Alert, Reason: skipReason})
			continue
		}
		if seen[rule.Name] {
			skipped = append(skipped, model.ImportPrometheusRuleSkipped{Name: rule.Name, Reason: "导入文件中规则名称重复"})
			continue
		}
		seen[rule.Name] = true
		if s.dao.RuleNameExists(rule.Name, 0) {
			skipped = append(skipped, model.ImportPrometheusRuleSkipped{Name: rule.Name, Reason: "平台中已存在同名规则"})
			continue
		}
		applyRuleDefaults(&rule)
		rule.Enabled = enabled
		rule.CreatedBy = operator
		rule.UpdatedBy = operator
		if err := validateRule(&rule); err != nil {
			skipped = append(skipped, model.ImportPrometheusRuleSkipped{Name: rule.Name, Reason: err.Error()})
			continue
		}
		if err := s.validateRuleNotifyRuleIDs(rule.NotifyRuleIDs); err != nil {
			skipped = append(skipped, model.ImportPrometheusRuleSkipped{Name: rule.Name, Reason: err.Error()})
			continue
		}
		if err := s.dao.CreateRule(&rule); err != nil {
			skipped = append(skipped, model.ImportPrometheusRuleSkipped{Name: rule.Name, Reason: err.Error()})
			continue
		}
		imported = append(imported, rule)
	}
	if enabled == model.RuleStatusEnabled && len(imported) > 0 {
		ruleIDs := make([]uint, 0, len(imported))
		for _, rule := range imported {
			ruleIDs = append(ruleIDs, rule.ID)
		}
		alertengine.ResumeRules(ruleIDs)
	}

	result.Success(c, model.ImportPrometheusRulesResponse{
		Total:        len(parsed),
		Imported:     len(imported),
		Skipped:      len(skipped),
		Rules:        imported,
		SkippedItems: skipped,
	})
}

func (s *AlertManagerServiceImpl) CreateDatasource(c *gin.Context, item model.AlertDatasource) {
	operator := currentOperator(c)
	item.CreatedBy = operator
	item.UpdatedBy = operator
	if err := validateDatasource(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	applyDatasourceDefaults(&item)
	if s.dao.DatasourceNameExists(item.Name, 0) {
		result.Failed(c, 400, "告警数据源名称已存在")
		return
	}
	if err := s.dao.CreateDatasource(&item); err != nil {
		result.Failed(c, 500, "创建告警数据源失败: "+err.Error())
		return
	}
	alertengine.InvalidateDatasourceCache()
	alertengine.TriggerReload()
	result.Success(c, sanitizeDatasource(item))
}

func (s *AlertManagerServiceImpl) UpdateDatasource(c *gin.Context, item model.AlertDatasource) {
	if item.ID == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item.CreatedBy = ""
	item.UpdatedBy = currentOperator(c)
	existing, err := s.dao.GetDatasource(item.ID)
	if err != nil {
		result.Failed(c, 500, "查询告警数据源失败: "+err.Error())
		return
	}
	if item.Password == "" {
		item.Password = existing.Password
	}
	if item.Token == "" {
		item.Token = existing.Token
	}
	item.Builtin = existing.Builtin
	if item.Enabled == 0 {
		item.Enabled = existing.Enabled
	}
	if err := validateDatasource(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if item.AuthType == "" {
		item.AuthType = "none"
	}
	if s.dao.DatasourceNameExists(item.Name, item.ID) {
		result.Failed(c, 400, "告警数据源名称已存在")
		return
	}
	if err := s.dao.UpdateDatasource(&item); err != nil {
		result.Failed(c, 500, "更新告警数据源失败: "+err.Error())
		return
	}
	alertengine.InvalidateDatasourceCache()
	alertengine.TriggerReload()
	result.Success(c, sanitizeDatasource(item))
}

func (s *AlertManagerServiceImpl) DeleteDatasource(c *gin.Context, id uint) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item, err := s.dao.GetDatasource(id)
	if err != nil {
		result.Failed(c, 500, "查询告警数据源失败: "+err.Error())
		return
	}
	if item.Builtin == 1 {
		result.Failed(c, 400, "内置告警数据源不允许删除")
		return
	}
	count, err := s.dao.CountRulesByDatasourceID(id)
	if err != nil {
		result.Failed(c, 500, "检查告警数据源失败: "+err.Error())
		return
	}
	if count > 0 {
		result.Failed(c, 400, "告警数据源已被规则引用，无法删除")
		return
	}
	if err := s.dao.DeleteDatasource(id); err != nil {
		result.Failed(c, 500, "删除告警数据源失败: "+err.Error())
		return
	}
	alertengine.InvalidateDatasourceCache()
	alertengine.TriggerReload()
	result.Success(c, nil)
}

func (s *AlertManagerServiceImpl) GetDatasource(c *gin.Context, id uint) {
	item, err := s.dao.GetDatasource(id)
	if err != nil {
		result.Failed(c, 500, "查询告警数据源失败: "+err.Error())
		return
	}
	result.Success(c, sanitizeDatasource(*item))
}

func (s *AlertManagerServiceImpl) ListDatasources(c *gin.Context, query model.AlertDatasourceQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	list, total, err := s.dao.ListDatasources(query)
	if err != nil {
		result.Failed(c, 500, "查询告警数据源失败: "+err.Error())
		return
	}
	result.SuccessWithPage(c, sanitizeDatasourceList(list), total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) ListDatasourceTypes(c *gin.Context) {
	result.Success(c, datasourceTypeOptions())
}

func (s *AlertManagerServiceImpl) ListDatasourceOptions(c *gin.Context) {
	list, err := s.dao.AllEnabledDatasources()
	if err != nil {
		result.Failed(c, 500, "查询告警数据源选项失败: "+err.Error())
		return
	}
	result.Success(c, sanitizeDatasourceList(list))
}

func (s *AlertManagerServiceImpl) ChangeDatasourceStatus(c *gin.Context, id uint, enabled int) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item, err := s.dao.GetDatasource(id)
	if err != nil {
		result.Failed(c, 500, "查询告警数据源失败: "+err.Error())
		return
	}
	if enabled != model.RuleStatusEnabled {
		enabled = model.RuleStatusDisabled
	}
	if err := s.dao.UpdateDatasourceStatus(id, enabled, currentOperator(c)); err != nil {
		result.Failed(c, 500, "更新告警数据源状态失败: "+err.Error())
		return
	}
	item.Enabled = enabled
	alertengine.InvalidateDatasourceCache()
	alertengine.TriggerReload()
	result.Success(c, sanitizeDatasource(*item))
}

func (s *AlertManagerServiceImpl) CreateBuiltinRuleTemplate(c *gin.Context, item model.AlertBuiltinRuleTemplate) {
	if isPlatformBuiltinRuleTemplate(&item) {
		result.Failed(c, 400, "platform 来源仅允许系统初始化创建")
		return
	}
	if err := validateBuiltinRuleTemplate(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	applyBuiltinDefaults(&item)
	if err := s.dao.CreateBuiltinRuleTemplate(&item); err != nil {
		result.Failed(c, 500, "创建内置规则模板失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) UpdateBuiltinRuleTemplate(c *gin.Context, item model.AlertBuiltinRuleTemplate) {
	if item.ID == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	existing, err := s.dao.GetBuiltinRuleTemplate(item.ID)
	if err != nil {
		result.Failed(c, 500, "查询内置规则模板失败: "+err.Error())
		return
	}
	if isPlatformBuiltinRuleTemplate(existing) {
		result.Failed(c, 400, "系统内置告警模板不允许修改")
		return
	}
	if isPlatformBuiltinRuleTemplate(&item) {
		result.Failed(c, 400, "platform 来源仅允许系统初始化设置")
		return
	}
	if err := validateBuiltinRuleTemplate(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	applyBuiltinDefaults(&item)
	if err := s.dao.UpdateBuiltinRuleTemplate(&item); err != nil {
		result.Failed(c, 500, "更新内置规则模板失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) DeleteBuiltinRuleTemplate(c *gin.Context, id uint) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item, err := s.dao.GetBuiltinRuleTemplate(id)
	if err != nil {
		result.Failed(c, 500, "查询内置规则模板失败: "+err.Error())
		return
	}
	if isPlatformBuiltinRuleTemplate(item) {
		result.Failed(c, 400, "系统内置告警模板不允许删除")
		return
	}
	if err := s.dao.DeleteBuiltinRuleTemplate(id); err != nil {
		result.Failed(c, 500, "删除内置规则模板失败: "+err.Error())
		return
	}
	result.Success(c, nil)
}

func (s *AlertManagerServiceImpl) GetBuiltinRuleTemplate(c *gin.Context, id uint) {
	item, err := s.dao.GetBuiltinRuleTemplate(id)
	if err != nil {
		result.Failed(c, 500, "查询内置规则模板失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) ListBuiltinRuleTemplates(c *gin.Context, query model.AlertBuiltinRuleTemplateQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	list, total, err := s.dao.ListBuiltinRuleTemplates(query)
	if err != nil {
		result.Failed(c, 500, "查询内置规则模板失败: "+err.Error())
		return
	}
	result.SuccessWithPage(c, list, total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) PreviewPrometheusTemplates(c *gin.Context, query model.PreviewPrometheusTemplateQuery) {
	list, err := loadAwesomePrometheusTemplates(query)
	if err != nil {
		result.Failed(c, 500, "拉取 Prometheus 模板失败: "+err.Error())
		return
	}
	for i := range list {
		list[i].Exists = s.dao.BuiltinRuleTemplateSourceExists(list[i].Source, list[i].SourceUUID)
	}
	result.Success(c, model.PreviewPrometheusTemplatesResponse{
		Total: len(list),
		List:  list,
	})
}

func (s *AlertManagerServiceImpl) ListPrometheusTemplateImportTypes(c *gin.Context) {
	result.Success(c, prometheusTemplateImportTypeOptions())
}

func (s *AlertManagerServiceImpl) ImportPrometheusTemplates(c *gin.Context, req model.ImportPrometheusTemplatesRequest) {
	if len(req.Rules) == 0 {
		result.Failed(c, 400, "rules is required")
		return
	}
	imported := make([]model.AlertBuiltinRuleTemplate, 0)
	skipped := make([]model.ImportPrometheusRuleSkipped, 0)
	for _, item := range req.Rules {
		template := buildTemplateFromPrometheusImportItem(item)
		applyBuiltinDefaults(&template)
		if err := validateBuiltinRuleTemplate(&template); err != nil {
			skipped = append(skipped, model.ImportPrometheusRuleSkipped{Name: template.Name, Reason: err.Error()})
			continue
		}
		if s.dao.BuiltinRuleTemplateSourceExists(template.Source, template.SourceUUID) {
			skipped = append(skipped, model.ImportPrometheusRuleSkipped{Name: template.Name, Reason: "模板已存在"})
			continue
		}
		if err := s.ensureBuiltinTemplateGroupPath(&template); err != nil {
			skipped = append(skipped, model.ImportPrometheusRuleSkipped{Name: template.Name, Reason: "创建模板分组失败: " + err.Error()})
			continue
		}
		if err := s.dao.CreateBuiltinRuleTemplate(&template); err != nil {
			skipped = append(skipped, model.ImportPrometheusRuleSkipped{Name: template.Name, Reason: err.Error()})
			continue
		}
		imported = append(imported, template)
	}
	result.Success(c, model.ImportPrometheusTemplatesResponse{
		Total:        len(req.Rules),
		Imported:     len(imported),
		Skipped:      len(skipped),
		Templates:    imported,
		SkippedItems: skipped,
	})
}

func (s *AlertManagerServiceImpl) CreateBuiltinRuleTemplateGroup(c *gin.Context, item model.AlertBuiltinRuleTemplateGroup) {
	operator := currentOperator(c)
	item.CreatedBy = operator
	item.UpdatedBy = operator
	applyBuiltinTemplateGroupDefaults(&item)
	if err := validateBuiltinTemplateGroup(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if s.dao.BuiltinRuleTemplateGroupExists(item.ParentID, item.Level, item.Value, 0) {
		result.Failed(c, 400, "模板分组已存在")
		return
	}
	if err := s.dao.CreateBuiltinRuleTemplateGroup(&item); err != nil {
		result.Failed(c, 500, "创建模板分组失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) ensureBuiltinTemplateGroupPath(template *model.AlertBuiltinRuleTemplate) error {
	operator := "system"
	component, err := s.ensureBuiltinTemplateGroup(0, model.TemplateGroupLevelComponent, template.Component, template.ResourceType, operator)
	if err != nil {
		return err
	}
	collector, err := s.ensureBuiltinTemplateGroup(component.ID, model.TemplateGroupLevelCollector, template.Collector, template.ResourceType, operator)
	if err != nil {
		return err
	}
	_, err = s.ensureBuiltinTemplateGroup(collector.ID, model.TemplateGroupLevelCategory, template.Category, template.ResourceType, operator)
	return err
}

func (s *AlertManagerServiceImpl) ensureBuiltinTemplateGroup(parentID uint, level, value, resourceType, operator string) (*model.AlertBuiltinRuleTemplateGroup, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "other"
	}
	existing, err := s.dao.GetBuiltinRuleTemplateGroupByPath(parentID, level, value)
	if err == nil {
		return existing, nil
	}
	item := model.AlertBuiltinRuleTemplateGroup{
		ParentID:     parentID,
		Level:        level,
		Name:         value,
		Value:        value,
		ResourceType: resourceType,
		Builtin:      model.RuleStatusEnabled,
		Enabled:      model.RuleStatusEnabled,
		CreatedBy:    operator,
		UpdatedBy:    operator,
	}
	applyBuiltinTemplateGroupDefaults(&item)
	if err := validateBuiltinTemplateGroup(&item); err != nil {
		return nil, err
	}
	if err := s.dao.CreateBuiltinRuleTemplateGroup(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AlertManagerServiceImpl) UpdateBuiltinRuleTemplateGroup(c *gin.Context, item model.AlertBuiltinRuleTemplateGroup) {
	if item.ID == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item.CreatedBy = ""
	item.UpdatedBy = currentOperator(c)
	existing, err := s.dao.GetBuiltinRuleTemplateGroup(item.ID)
	if err != nil {
		result.Failed(c, 500, "查询模板分组失败: "+err.Error())
		return
	}
	platformCount, err := s.countPlatformTemplatesByTemplateGroup(existing)
	if err != nil {
		result.Failed(c, 500, "检查系统内置模板失败: "+err.Error())
		return
	}
	if platformCount > 0 {
		result.Failed(c, 400, "包含系统内置告警模板的分组不允许修改")
		return
	}
	item.Builtin = existing.Builtin
	applyBuiltinTemplateGroupDefaults(&item)
	if err := validateBuiltinTemplateGroup(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if s.dao.BuiltinRuleTemplateGroupExists(item.ParentID, item.Level, item.Value, item.ID) {
		result.Failed(c, 400, "模板分组已存在")
		return
	}
	if err := s.dao.UpdateBuiltinRuleTemplateGroup(&item); err != nil {
		result.Failed(c, 500, "更新模板分组失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) DeleteBuiltinRuleTemplateGroup(c *gin.Context, id uint) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	group, err := s.dao.GetBuiltinRuleTemplateGroup(id)
	if err != nil {
		result.Failed(c, 500, "查询模板分组失败: "+err.Error())
		return
	}
	component, collector, category, err := s.templateGroupPathByGroup(group)
	if err != nil {
		result.Failed(c, 500, "解析模板分组失败: "+err.Error())
		return
	}
	platformCount, err := s.dao.CountPlatformBuiltinRuleTemplatesByPath(component, collector, category)
	if err != nil {
		result.Failed(c, 500, "检查系统内置模板失败: "+err.Error())
		return
	}
	if platformCount > 0 {
		result.Failed(c, 400, "包含系统内置告警模板的分组不允许删除")
		return
	}
	deletedTemplates, err := s.dao.DeleteBuiltinRuleTemplatesByPath(component, collector, category)
	if err != nil {
		result.Failed(c, 500, "删除模板分组下模板失败: "+err.Error())
		return
	}
	groupIDs, err := s.descendantBuiltinTemplateGroupIDs(id)
	if err != nil {
		result.Failed(c, 500, "查询模板子分组失败: "+err.Error())
		return
	}
	if err := s.dao.DeleteBuiltinRuleTemplateGroupsByIDs(groupIDs); err != nil {
		result.Failed(c, 500, "删除模板分组失败: "+err.Error())
		return
	}
	result.Success(c, model.JSONMap{
		"deletedTemplates": deletedTemplates,
		"deletedGroups":    len(groupIDs),
	})
}

func (s *AlertManagerServiceImpl) GetBuiltinRuleTemplateGroup(c *gin.Context, id uint) {
	item, err := s.dao.GetBuiltinRuleTemplateGroup(id)
	if err != nil {
		result.Failed(c, 500, "查询模板分组失败: "+err.Error())
		return
	}
	count, err := s.countTemplatesByTemplateGroup(item)
	if err != nil {
		result.Failed(c, 500, "统计模板分组数量失败: "+err.Error())
		return
	}
	item.TemplateCount = count
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) ListBuiltinRuleTemplateGroups(c *gin.Context, query model.AlertBuiltinRuleTemplateGroupQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	list, total, err := s.dao.ListBuiltinRuleTemplateGroups(query)
	if err != nil {
		result.Failed(c, 500, "查询模板分组失败: "+err.Error())
		return
	}
	if err := s.fillBuiltinTemplateGroupCounts(list); err != nil {
		result.Failed(c, 500, "统计模板分组数量失败: "+err.Error())
		return
	}
	result.SuccessWithPage(c, list, total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) fillBuiltinTemplateGroupCounts(list []model.AlertBuiltinRuleTemplateGroup) error {
	for i := range list {
		count, err := s.countTemplatesByTemplateGroup(&list[i])
		if err != nil {
			return err
		}
		list[i].TemplateCount = count
	}
	return nil
}

func (s *AlertManagerServiceImpl) AllBuiltinRuleTemplateGroupTree(c *gin.Context) {
	rows, err := s.dao.ListBuiltinRuleTemplatePathRows()
	if err != nil {
		result.Failed(c, 500, "查询模板分组失败: "+err.Error())
		return
	}
	manualGroups, err := s.dao.AllBuiltinRuleTemplateGroups()
	if err != nil {
		result.Failed(c, 500, "查询模板分组失败: "+err.Error())
		return
	}
	result.Success(c, buildTemplateGroupTree(rows, manualGroups))
}

func (s *AlertManagerServiceImpl) ListBuiltinRuleTemplateGroupOptions(c *gin.Context, query model.AlertBuiltinRuleTemplateGroupOptionsQuery) {
	rows, err := s.dao.ListBuiltinRuleTemplatePathRows()
	if err != nil {
		result.Failed(c, 500, "查询模板分组选项失败: "+err.Error())
		return
	}
	manualGroups, err := s.dao.AllBuiltinRuleTemplateGroups()
	if err != nil {
		result.Failed(c, 500, "查询模板分组选项失败: "+err.Error())
		return
	}
	result.Success(c, buildTemplateGroupOptions(rows, manualGroups, query))
}

func (s *AlertManagerServiceImpl) ApplyBuiltinRuleTemplate(c *gin.Context, id uint, req model.ApplyBuiltinRuleTemplateRequest) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	template, err := s.dao.GetBuiltinRuleTemplate(id)
	if err != nil {
		result.Failed(c, 500, "查询内置规则模板失败: "+err.Error())
		return
	}
	rule := buildRuleFromBuiltinTemplate(template, req)
	applyRuleDefaults(&rule)
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	} else {
		rule.Enabled = template.DefaultEnabled
	}
	if err := validateRule(&rule); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if err := s.validateRuleNotifyRuleIDs(rule.NotifyRuleIDs); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if s.dao.RuleNameExists(rule.Name, 0) {
		result.Failed(c, 400, "告警规则名称已存在")
		return
	}
	if err := s.dao.CreateRule(&rule); err != nil {
		result.Failed(c, 500, "应用内置规则模板失败: "+err.Error())
		return
	}
	if rule.Enabled == model.RuleStatusEnabled {
		alertengine.ResumeRule(rule.ID)
	}
	fillRuleDurationCount(&rule)
	result.Success(c, rule)
}

func (s *AlertManagerServiceImpl) ListActiveEvents(c *gin.Context, query model.AlertEventQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	if strings.TrimSpace(query.BusinessRouteID) != "" {
		s.listActiveEventsByBusinessRoute(c, query)
		return
	}
	if list, total, ok := s.listActiveEventsFromState(query); ok {
		result.SuccessWithPage(c, s.activeEventVOs(list), total, query.Page, query.PageSize)
		return
	}
	list, total, err := s.dao.ListActiveEvents(query)
	if err != nil {
		result.Failed(c, 500, "查询活跃告警失败: "+err.Error())
		return
	}
	result.SuccessWithPage(c, s.activeEventVOs(list), total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) ListActiveEventBusinessOptions(c *gin.Context) {
	routes, err := s.dao.ListEnabledNotifyRouteRules()
	if err != nil {
		result.Failed(c, 500, "查询告警业务选项失败: "+err.Error())
		return
	}
	result.Success(c, buildActiveEventBusinessOptions(routes))
}

func buildActiveEventBusinessOptions(routes []model.AlertNotifyRouteRule) []model.AlertBusinessRouteOption {
	options := make([]model.AlertBusinessRouteOption, 0, len(routes)+1)
	for _, route := range routes {
		if route.Enabled != model.RuleStatusEnabled || route.NotifyGroupID == 0 {
			continue
		}
		label := dashboardBusinessGroupName(route.Name)
		if label == "" {
			label = fmt.Sprint(route.ID)
		}
		options = append(options, model.AlertBusinessRouteOption{
			Label: label,
			Value: fmt.Sprint(route.ID),
		})
	}
	options = append(options, model.AlertBusinessRouteOption{
		Label: "运维组",
		Value: model.AlertBusinessRouteOperations,
	})
	return options
}

func (s *AlertManagerServiceImpl) listActiveEventsByBusinessRoute(c *gin.Context, query model.AlertEventQuery) {
	list, ok := s.listActiveEventsForGroupsFromState(query)
	if !ok {
		var err error
		list, err = s.dao.ListActiveEventsForGroups(query)
		if err != nil {
			result.Failed(c, 500, "查询活跃告警失败: "+err.Error())
			return
		}
	}
	filtered, err := s.filterActiveEventsByBusinessRoute(list, query.BusinessRouteID)
	if err != nil {
		result.Failed(c, 500, "匹配告警业务失败: "+err.Error())
		return
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if !filtered[i].LastEvaluatedAt.Time.Equal(filtered[j].LastEvaluatedAt.Time) {
			return filtered[i].LastEvaluatedAt.Time.After(filtered[j].LastEvaluatedAt.Time)
		}
		return filtered[i].ID > filtered[j].ID
	})
	total := int64(len(filtered))
	start := (query.Page - 1) * query.PageSize
	if start >= len(filtered) {
		result.SuccessWithPage(c, []model.AlertEventVO{}, total, query.Page, query.PageSize)
		return
	}
	end := start + query.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	result.SuccessWithPage(c, s.activeEventVOs(filtered[start:end]), total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) filterActiveEventsByBusinessRoute(list []model.AlertActiveEvent, businessRouteID string) ([]model.AlertActiveEvent, error) {
	target := strings.TrimSpace(businessRouteID)
	if target == "" {
		return list, nil
	}
	routes, err := s.dao.ListEnabledNotifyRouteRules()
	if err != nil {
		return nil, err
	}
	ruleIDs := make([]uint, 0, len(list))
	for _, event := range list {
		ruleIDs = append(ruleIDs, event.RuleID)
	}
	notifyRuleIDsByRuleID, err := s.dao.RuleNotifyRuleIDsByIDs(normalizeUintIDs(ruleIDs))
	if err != nil {
		return nil, err
	}
	return filterActiveEventsByMatchedBusinessRoute(list, routes, notifyRuleIDsByRuleID, target), nil
}

func filterActiveEventsByMatchedBusinessRoute(list []model.AlertActiveEvent, routes []model.AlertNotifyRouteRule, notifyRuleIDsByRuleID map[uint]model.JSONUintArray, target string) []model.AlertActiveEvent {
	filtered := make([]model.AlertActiveEvent, 0, len(list))
	for _, event := range list {
		route, matched := matchBusinessRoute(routes, notifyRuleIDsByRuleID[event.RuleID], event.Labels)
		if target == model.AlertBusinessRouteOperations {
			if !matched {
				filtered = append(filtered, event)
			}
			continue
		}
		if matched && fmt.Sprint(route.ID) == target {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func (s *AlertManagerServiceImpl) ListActiveEventGroups(c *gin.Context, query model.AlertEventQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	list, ok := s.listActiveEventsForGroupsFromState(query)
	if !ok {
		var err error
		list, err = s.dao.ListActiveEventsForGroups(query)
		if err != nil {
			result.Failed(c, 500, "查询活跃告警聚合失败: "+err.Error())
			return
		}
	}
	groups := s.activeEventGroupVOs(list, query.MemberLimit)
	total := int64(len(groups))
	start := (query.Page - 1) * query.PageSize
	if start >= len(groups) {
		result.SuccessWithPage(c, []model.AlertEventGroupVO{}, total, query.Page, query.PageSize)
		return
	}
	end := start + query.PageSize
	if end > len(groups) {
		end = len(groups)
	}
	result.SuccessWithPage(c, groups[start:end], total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) listActiveEventsFromState(query model.AlertEventQuery) ([]model.AlertActiveEvent, int64, bool) {
	list, ok := s.listActiveEventsForGroupsFromState(query)
	if !ok {
		return nil, 0, false
	}
	sort.SliceStable(list, func(i, j int) bool {
		if !list[i].LastEvaluatedAt.Time.Equal(list[j].LastEvaluatedAt.Time) {
			return list[i].LastEvaluatedAt.Time.After(list[j].LastEvaluatedAt.Time)
		}
		return list[i].ID > list[j].ID
	})
	total := int64(len(list))
	start := (query.Page - 1) * query.PageSize
	if start >= len(list) {
		return []model.AlertActiveEvent{}, total, true
	}
	end := start + query.PageSize
	if end > len(list) {
		end = len(list)
	}
	return list[start:end], total, true
}

func (s *AlertManagerServiceImpl) listActiveEventsForGroupsFromState(query model.AlertEventQuery) ([]model.AlertActiveEvent, bool) {
	list, ok := alertengine.ListActiveEventsFromState()
	if !ok {
		return nil, false
	}
	filtered := make([]model.AlertActiveEvent, 0, len(list))
	for _, event := range list {
		if activeEventMatchesQuery(event, query) {
			filtered = append(filtered, event)
		}
	}
	return filtered, true
}

func activeEventMatchesQuery(event model.AlertActiveEvent, query model.AlertEventQuery) bool {
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		haystack := strings.ToLower(event.RuleName + " " + event.Fingerprint)
		if !strings.Contains(haystack, strings.ToLower(keyword)) {
			return false
		}
	}
	if query.Fingerprint != "" && event.Fingerprint != query.Fingerprint {
		return false
	}
	if query.RuleID > 0 && event.RuleID != query.RuleID {
		return false
	}
	if query.DatasourceID > 0 && event.DatasourceID != query.DatasourceID {
		return false
	}
	if query.DatasourceType != "" && event.DatasourceType != query.DatasourceType {
		return false
	}
	if query.Severity != "" && event.Severity != query.Severity {
		return false
	}
	if query.BusinessID != "" && event.BusinessID != query.BusinessID {
		return false
	}
	if query.Status != "" && event.Status != query.Status {
		return false
	}
	if query.Status == "" && !truthyServiceQuery(query.IncludePending) && query.EventStatus != model.EventDisplayStatusPending && event.Status == model.EventStatusPending {
		return false
	}
	switch query.EventStatus {
	case model.EventDisplayStatusActive:
		if event.Silenced == 1 {
			return false
		}
		if !truthyServiceQuery(query.IncludePending) && event.Status == model.EventStatusPending {
			return false
		}
	case model.EventDisplayStatusPending:
		if event.Status != model.EventStatusPending {
			return false
		}
	case model.EventDisplayStatusSilenced:
		if event.Silenced != 1 {
			return false
		}
	}
	if query.Silenced != "" && fmt.Sprint(event.Silenced) != query.Silenced {
		return false
	}
	return true
}

func truthyServiceQuery(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}

func normalizeUintIDs(values []uint) []uint {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (s *AlertManagerServiceImpl) ListHistoryEvents(c *gin.Context, query model.AlertEventQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	list, total, err := s.dao.ListHistoryEvents(query)
	if err != nil {
		result.Failed(c, 500, "查询历史告警失败: "+err.Error())
		return
	}
	result.SuccessWithPage(c, s.historyEventVOs(list), total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) GetEventDetail(c *gin.Context, eventType string, eventID uint64) {
	if eventID == 0 {
		result.Failed(c, 400, "eventId is required")
		return
	}
	eventType = strings.ToLower(strings.TrimSpace(eventType))
	switch eventType {
	case "", model.EventDisplayStatusActive:
		event, err := s.dao.GetActiveEvent(eventID)
		if err != nil {
			result.Failed(c, 500, "查询活跃告警详情失败: "+err.Error())
			return
		}
		detail, err := s.activeEventDetail(*event)
		if err != nil {
			result.Failed(c, 500, "组装活跃告警详情失败: "+err.Error())
			return
		}
		result.Success(c, detail)
	case "history", model.EventDisplayStatusResolved:
		event, err := s.dao.GetHistoryEvent(eventID)
		if err != nil {
			result.Failed(c, 500, "查询历史告警详情失败: "+err.Error())
			return
		}
		detail, err := s.historyEventDetail(*event)
		if err != nil {
			result.Failed(c, 500, "组装历史告警详情失败: "+err.Error())
			return
		}
		result.Success(c, detail)
	default:
		result.Failed(c, 400, "eventType must be active or history")
	}
}

func (s *AlertManagerServiceImpl) ListTimeline(c *gin.Context, eventID uint64) {
	if eventID == 0 {
		result.Failed(c, 400, "eventId is required")
		return
	}
	list, err := s.dao.ListTimeline(eventID)
	if err != nil {
		result.Failed(c, 500, "查询告警时间线失败: "+err.Error())
		return
	}
	result.Success(c, list)
}

func (s *AlertManagerServiceImpl) ClaimActiveEvent(c *gin.Context, eventID uint64) {
	if eventID == 0 {
		result.Failed(c, 400, "eventId is required")
		return
	}
	operator := currentOperator(c)
	if err := s.claimActiveEventInternal(eventID, operator); err != nil {
		result.Failed(c, 500, "认领告警失败: "+err.Error())
		return
	}
	result.Success(c, gin.H{"claimUser": operator})
}

func (s *AlertManagerServiceImpl) SilenceActiveEvent(c *gin.Context, eventID uint64, req model.SilenceEventRequest) {
	if eventID == 0 {
		result.Failed(c, 400, "eventId is required")
		return
	}
	event, err := s.dao.GetActiveEvent(eventID)
	if err != nil {
		result.Failed(c, 500, "查询告警事件失败: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		result.Failed(c, 400, "静默原因不能为空")
		return
	}
	now := time.Now()
	endTime := req.EndTime.Time
	if endTime.IsZero() {
		if req.DurationMinutes <= 0 {
			result.Failed(c, 400, "durationMinutes or endTime is required")
			return
		}
		endTime = now.Add(time.Duration(req.DurationMinutes) * time.Minute)
	}
	if !endTime.After(now) {
		result.Failed(c, 400, "静默结束时间必须晚于当前时间")
		return
	}
	operator := currentOperator(c)
	item, err := s.silenceActiveEventInternal(event, endTime, req.Reason, operator, req.Matchers)
	if err != nil {
		result.Failed(c, 500, "静默告警失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

// UnsilenceActiveEvent cancels event-created silence rules and makes the event
// visible to the active list immediately. The next evaluation will refine the
// status to pending or firing according to the rule's hold duration.
func (s *AlertManagerServiceImpl) UnsilenceActiveEvent(c *gin.Context, eventID uint64) {
	if eventID == 0 {
		result.Failed(c, 400, "eventId is required")
		return
	}
	if _, err := s.dao.GetActiveEvent(eventID); err != nil {
		result.Failed(c, 404, "告警事件不存在或已恢复")
		return
	}
	operator := currentOperator(c)
	cancellation, err := s.dao.CancelEventSilences(eventID, operator)
	if err != nil {
		result.Failed(c, 500, "取消静默失败: "+err.Error())
		return
	}
	alertengine.InvalidateSilenceCache()
	alertengine.CacheActiveEventState(cancellation.Event)
	result.Success(c, gin.H{
		"eventId":        cancellation.Event.ID,
		"silenced":       cancellation.Event.Silenced == 1,
		"disabledRules":  cancellation.DisabledRules,
		"remainingRules": cancellation.RemainingRules,
	})
}

func (s *AlertManagerServiceImpl) activeEventVOs(list []model.AlertActiveEvent) []model.AlertEventVO {
	result := make([]model.AlertEventVO, 0, len(list))
	for _, event := range list {
		result = append(result, s.activeEventVO(event))
	}
	return result
}

func (s *AlertManagerServiceImpl) activeEventGroupVOs(list []model.AlertActiveEvent, memberLimit int) []model.AlertEventGroupVO {
	if memberLimit <= 0 {
		memberLimit = 20
	}
	if memberLimit > 100 {
		memberLimit = 100
	}
	groupMap := make(map[string]*activeEventGroupBuilder)
	order := make([]string, 0)
	for _, event := range list {
		key := activeEventGroupKey(event)
		builder, ok := groupMap[key]
		if !ok {
			ruleGroupID, ruleGroupName := s.eventRuleGroup(event.RuleID)
			builder = &activeEventGroupBuilder{
				vo: model.AlertEventGroupVO{
					GroupKey:       key,
					RuleID:         event.RuleID,
					RuleName:       event.RuleName,
					RuleGroupID:    ruleGroupID,
					RuleGroupName:  ruleGroupName,
					DatasourceID:   event.DatasourceID,
					DatasourceType: event.DatasourceType,
					DatasourceName: s.eventDatasourceName(event.DatasourceID),
					Severity:       event.Severity,
					Level:          eventLabelString(event.Labels, "level"),
					BusinessID:     event.BusinessID,
				},
				resources: make(map[string]bool),
			}
			groupMap[key] = builder
			order = append(order, key)
		}
		builder.add(event, s.activeEventVO(event), memberLimit)
	}

	groups := make([]model.AlertEventGroupVO, 0, len(order))
	now := time.Now()
	for _, key := range order {
		builder := groupMap[key]
		group := builder.vo
		group.ResourceCount = len(builder.resources)
		group.MemberTotal = builder.total
		group.MembersLimited = builder.total > len(group.Members)
		group.AffectedResources = builder.resourceList
		group.EventStatus, group.EventStatusName = groupedEventDisplayStatus(group)
		group.DurationSeconds = durationSeconds(group.FirstTriggeredAt.Time, now)
		groups = append(groups, group)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if !groups[i].LastEvaluatedAt.Time.Equal(groups[j].LastEvaluatedAt.Time) {
			return groups[i].LastEvaluatedAt.Time.After(groups[j].LastEvaluatedAt.Time)
		}
		return groups[i].GroupKey > groups[j].GroupKey
	})
	return groups
}

func (s *AlertManagerServiceImpl) historyEventVOs(list []model.AlertHistoryEvent) []model.AlertEventVO {
	result := make([]model.AlertEventVO, 0, len(list))
	for _, event := range list {
		result = append(result, s.historyEventVO(event))
	}
	return result
}

func (s *AlertManagerServiceImpl) activeEventVO(event model.AlertActiveEvent) model.AlertEventVO {
	ruleGroupID, ruleGroupName := s.eventRuleGroup(event.RuleID)
	datasourceName := s.eventDatasourceName(event.DatasourceID)
	eventStatus, eventStatusName := activeEventDisplayStatus(event)
	now := time.Now()
	return model.AlertEventVO{
		ID:               event.ID,
		Fingerprint:      event.Fingerprint,
		RuleID:           event.RuleID,
		RuleName:         event.RuleName,
		RuleGroupID:      ruleGroupID,
		RuleGroupName:    ruleGroupName,
		DatasourceID:     event.DatasourceID,
		DatasourceType:   event.DatasourceType,
		DatasourceName:   datasourceName,
		Severity:         event.Severity,
		Level:            eventLabelString(event.Labels, "level"),
		EventStatus:      eventStatus,
		EventStatusName:  eventStatusName,
		EvalStatus:       event.Status,
		EvalStatusName:   evalStatusName(event.Status),
		Labels:           event.Labels,
		Annotations:      event.Annotations,
		Value:            event.Value,
		AssetID:          event.AssetID,
		BusinessID:       event.BusinessID,
		FirstTriggeredAt: event.FirstTriggeredAt,
		LastEvaluatedAt:  event.LastEvaluatedAt,
		LastNotifiedAt:   event.LastNotifiedAt,
		RecoveredAt:      event.RecoveredAt,
		DurationSeconds:  durationSeconds(event.FirstTriggeredAt.Time, now),
		NotifyCount:      event.NotifyCount,
		Suppressed:       event.Suppressed,
		Silenced:         event.Silenced,
		ClaimUser:        event.ClaimUser,
		ClaimedAt:        event.ClaimedAt,
		CreatedAt:        event.CreatedAt,
		UpdatedAt:        event.UpdatedAt,
	}
}

func (s *AlertManagerServiceImpl) historyEventVO(event model.AlertHistoryEvent) model.AlertEventVO {
	ruleGroupID, ruleGroupName := s.eventRuleGroup(event.RuleID)
	datasourceName := s.eventDatasourceName(event.DatasourceID)
	endTime := event.RecoveredAt.Time
	if endTime.IsZero() {
		endTime = event.OccurredAt.Time
	}
	return model.AlertEventVO{
		ID:               event.ID,
		Fingerprint:      event.Fingerprint,
		RuleID:           event.RuleID,
		RuleName:         event.RuleName,
		RuleGroupID:      ruleGroupID,
		RuleGroupName:    ruleGroupName,
		DatasourceID:     event.DatasourceID,
		DatasourceType:   event.DatasourceType,
		DatasourceName:   datasourceName,
		Severity:         event.Severity,
		Level:            eventLabelString(event.Labels, "level"),
		EventStatus:      model.EventDisplayStatusResolved,
		EventStatusName:  "已恢复",
		EvalStatus:       event.EventType,
		EvalStatusName:   evalStatusName(event.EventType),
		Labels:           event.Labels,
		Annotations:      event.Annotations,
		Value:            event.Value,
		AssetID:          event.AssetID,
		BusinessID:       event.BusinessID,
		FirstTriggeredAt: event.FirstTriggeredAt,
		RecoveredAt:      event.RecoveredAt,
		OccurredAt:       event.OccurredAt,
		DurationSeconds:  durationSeconds(event.FirstTriggeredAt.Time, endTime),
		ClaimUser:        event.ClaimUser,
		CreatedAt:        event.CreatedAt,
	}
}

func (s *AlertManagerServiceImpl) activeEventDetail(event model.AlertActiveEvent) (model.AlertEventDetailVO, error) {
	rule := s.detailRule(event.RuleID)
	timeline, err := s.dao.ListTimeline(event.ID)
	if err != nil {
		return model.AlertEventDetailVO{}, err
	}
	triggerQueries := eventTriggerQueries(rule, event.Labels)
	timeline = ensureTriggerTimeline(timeline, event.ID, event.FirstTriggeredAt, event.CreatedAt, event.TriggerCondition, event.TriggerQuery, event.Value, event.Severity)
	notifyRecords, err := s.dao.ListNotifyRecordsByEvent(event.ID, model.EventDisplayStatusActive)
	if err != nil {
		return model.AlertEventDetailVO{}, err
	}
	triggerQuery := firstNonEmpty(event.TriggerQuery, primaryTriggerQuery(triggerQueries, rule))
	triggerCondition := firstNonEmpty(event.TriggerCondition, primaryTriggerCondition(triggerQueries, rule))
	return model.AlertEventDetailVO{
		EventType:        model.EventDisplayStatusActive,
		Event:            s.activeEventVO(event),
		Rule:             rule,
		TriggerQuery:     triggerQuery,
		TriggerCondition: triggerCondition,
		TriggerQueries:   triggerQueries,
		EventDetail:      firstNonEmpty(event.EventDetail, eventDetailText(event.Annotations, rule)),
		SampleLogs:       s.eventSampleLogs(rule, event.DatasourceID, event.DatasourceType, triggerQuery, event.LastEvaluatedAt.Time, event.FirstTriggeredAt.Time, event.CreatedAt.Time),
		Timeline:         timeline,
		NotifyRecords:    notifyRecordVOs(notifyRecords),
	}, nil
}

func (s *AlertManagerServiceImpl) historyEventDetail(event model.AlertHistoryEvent) (model.AlertEventDetailVO, error) {
	rule := s.detailRule(event.RuleID)
	timelineEventID := event.ActiveEventID
	if timelineEventID == 0 {
		timelineEventID = event.ID
	}
	timeline, err := s.dao.ListTimeline(timelineEventID)
	if err != nil {
		return model.AlertEventDetailVO{}, err
	}
	triggerQueries := eventTriggerQueries(rule, event.Labels)
	timeline = ensureTriggerTimeline(timeline, timelineEventID, event.FirstTriggeredAt, event.CreatedAt, event.TriggerCondition, event.TriggerQuery, event.Value, event.Severity)
	notifyRecords, err := s.dao.ListNotifyRecordsByEvent(timelineEventID, "")
	if err != nil {
		return model.AlertEventDetailVO{}, err
	}
	triggerQuery := firstNonEmpty(event.TriggerQuery, primaryTriggerQuery(triggerQueries, rule))
	triggerCondition := firstNonEmpty(event.TriggerCondition, primaryTriggerCondition(triggerQueries, rule))
	return model.AlertEventDetailVO{
		EventType:        "history",
		Event:            s.historyEventVO(event),
		Rule:             rule,
		TriggerQuery:     triggerQuery,
		TriggerCondition: triggerCondition,
		TriggerQueries:   triggerQueries,
		EventDetail:      firstNonEmpty(event.EventDetail, eventDetailText(event.Annotations, rule)),
		SampleLogs:       s.eventSampleLogs(rule, event.DatasourceID, event.DatasourceType, triggerQuery, event.FirstTriggeredAt.Time, event.OccurredAt.Time, event.CreatedAt.Time),
		Timeline:         timeline,
		NotifyRecords:    notifyRecordVOs(notifyRecords),
	}, nil
}

func (s *AlertManagerServiceImpl) eventSampleLogs(rule *model.AlertRule, datasourceID uint, datasourceType, query string, candidateTimes ...time.Time) *model.AlertEventSampleLogs {
	if datasourceType != model.AlertDatasourceVictoriaLogs || strings.TrimSpace(query) == "" {
		return nil
	}
	ds, err := s.eventDatasource(datasourceID, datasourceType)
	if err != nil {
		return &model.AlertEventSampleLogs{
			Query:         strings.TrimSpace(query),
			WindowSeconds: eventSampleLogWindowSeconds(rule),
			Limit:         eventSampleLogLimit(rule),
			Error:         err.Error(),
		}
	}
	end := firstNonZeroTime(candidateTimes...)
	if end.IsZero() {
		end = time.Now()
	}
	windowSeconds := eventSampleLogWindowSeconds(rule)
	limit := eventSampleLogLimit(rule)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var ruleConfig model.JSONMap
	if rule != nil {
		ruleConfig = rule.RuleConfig
	}
	sample, err := alertengine.QueryVictoriaLogsSampleLogs(ctx, *ds, query, end, windowSeconds, limit, ruleConfig)
	if err != nil {
		return &model.AlertEventSampleLogs{
			Query:         strings.TrimSpace(query),
			WindowSeconds: windowSeconds,
			Limit:         limit,
			End:           end.UTC().Format(time.RFC3339Nano),
			Start:         end.Add(-time.Duration(windowSeconds) * time.Second).UTC().Format(time.RFC3339Nano),
			Error:         err.Error(),
		}
	}
	return &model.AlertEventSampleLogs{
		Query:         sample.Query,
		Count:         sample.Count,
		Limit:         sample.Limit,
		Start:         sample.Start,
		End:           sample.End,
		WindowSeconds: sample.WindowSeconds,
		Logs:          sample.Logs,
	}
}

func (s *AlertManagerServiceImpl) eventDatasource(datasourceID uint, datasourceType string) (*model.AlertDatasource, error) {
	if datasourceID > 0 {
		return s.dao.GetDatasource(datasourceID)
	}
	return s.resolveQueryDatasource(model.PreviewRuleQueryRequest{DatasourceType: datasourceType})
}

func eventSampleLogWindowSeconds(rule *model.AlertRule) int64 {
	if rule != nil && rule.RuleConfig != nil {
		if seconds := durationSecondsFromAnyMap(rule.RuleConfig, "sampleWindowSeconds", "windowSeconds", "lookbackSeconds", "rangeSeconds", "durationSeconds", "window", "lookback", "range"); seconds > 0 {
			return seconds
		}
		if query, ok := firstRuleConfigQuery(rule.RuleConfig); ok {
			if seconds := durationSecondsFromAnyMap(query, "sampleWindowSeconds", "windowSeconds", "lookbackSeconds", "rangeSeconds", "durationSeconds", "window", "lookback", "range"); seconds > 0 {
				return seconds
			}
		}
	}
	return defaultVictoriaLogsPreviewWindowSeconds
}

func eventSampleLogLimit(rule *model.AlertRule) int {
	limit := int64(5)
	if rule != nil && rule.RuleConfig != nil {
		if value := durationSecondsFromAnyMap(rule.RuleConfig, "sampleLogLimit", "sampleLogsLimit", "logSampleLimit"); value > 0 {
			limit = value
		}
	}
	if limit <= 0 {
		return 5
	}
	if limit > 50 {
		return 50
	}
	return int(limit)
}

func durationSecondsFromAnyMap(values map[string]interface{}, keys ...string) int64 {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok || raw == nil {
			continue
		}
		switch typed := raw.(type) {
		case int:
			return int64(typed)
		case int64:
			return typed
		case int32:
			return int64(typed)
		case float64:
			return int64(typed)
		case float32:
			return int64(typed)
		case string:
			text := strings.TrimSpace(typed)
			if text == "" {
				continue
			}
			if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
				return parsed
			}
			if duration, err := time.ParseDuration(text); err == nil {
				return int64(duration.Seconds())
			}
		default:
			text := strings.TrimSpace(fmt.Sprint(raw))
			if text == "" || text == "<nil>" {
				continue
			}
			if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func (s *AlertManagerServiceImpl) detailRule(ruleID uint) *model.AlertRule {
	if ruleID == 0 {
		return nil
	}
	rule, err := s.dao.GetRule(ruleID)
	if err != nil || rule == nil || rule.ID == 0 {
		return nil
	}
	return rule
}

func (s *AlertManagerServiceImpl) eventRuleGroup(ruleID uint) (uint, string) {
	if ruleID == 0 || s == nil || s.dao == nil {
		return 0, ""
	}
	rule, err := s.dao.GetRule(ruleID)
	if err != nil || rule == nil || rule.GroupID == 0 {
		return 0, ""
	}
	group, err := s.dao.GetRuleGroup(rule.GroupID)
	if err != nil || group == nil {
		return rule.GroupID, ""
	}
	return group.ID, group.Name
}

func (s *AlertManagerServiceImpl) eventDatasourceName(datasourceID uint) string {
	if datasourceID == 0 || s == nil || s.dao == nil {
		return ""
	}
	datasource, err := s.dao.GetDatasource(datasourceID)
	if err != nil || datasource == nil {
		return ""
	}
	return datasource.Name
}

type activeEventGroupBuilder struct {
	vo           model.AlertEventGroupVO
	resources    map[string]bool
	resourceList []string
	total        int
}

func (b *activeEventGroupBuilder) add(event model.AlertActiveEvent, member model.AlertEventVO, memberLimit int) {
	b.total++
	b.vo.NotifyCount += event.NotifyCount
	resource := activeEventResourceName(event)
	if resource != "" && !b.resources[resource] {
		b.resources[resource] = true
		b.resourceList = append(b.resourceList, resource)
	}
	switch event.Status {
	case model.EventStatusPending:
		b.vo.PendingCount++
	case model.EventStatusFiring:
		b.vo.FiringCount++
	case model.EventStatusSilenced:
		b.vo.SilencedCount++
	case model.EventStatusSuppressed:
		b.vo.SuppressedCount++
	}
	if event.Silenced == 1 && event.Status != model.EventStatusSilenced {
		b.vo.SilencedCount++
	}
	if b.vo.FirstTriggeredAt.IsZero() || (!event.FirstTriggeredAt.IsZero() && event.FirstTriggeredAt.Time.Before(b.vo.FirstTriggeredAt.Time)) {
		b.vo.FirstTriggeredAt = event.FirstTriggeredAt
	}
	if b.vo.LastEvaluatedAt.IsZero() || event.LastEvaluatedAt.Time.After(b.vo.LastEvaluatedAt.Time) {
		b.vo.LastEvaluatedAt = event.LastEvaluatedAt
	}
	if !event.LastNotifiedAt.IsZero() && (b.vo.LastNotifiedAt.IsZero() || event.LastNotifiedAt.Time.After(b.vo.LastNotifiedAt.Time)) {
		b.vo.LastNotifiedAt = event.LastNotifiedAt
	}
	if len(b.vo.Members) < memberLimit {
		b.vo.Members = append(b.vo.Members, member)
	}
}

func activeEventGroupKey(event model.AlertActiveEvent) string {
	return strings.Join([]string{
		fmt.Sprintf("rule:%d", event.RuleID),
		fmt.Sprintf("datasource:%d", event.DatasourceID),
		"severity:" + strings.TrimSpace(event.Severity),
		"business:" + strings.TrimSpace(event.BusinessID),
	}, "|")
}

func activeEventResourceName(event model.AlertActiveEvent) string {
	for _, key := range []string{"hostname", "instance", "pod", "node", "container", "job", "name"} {
		if value := eventLabelString(event.Labels, key); value != "" {
			return value
		}
	}
	if event.AssetID > 0 {
		return fmt.Sprintf("asset:%d", event.AssetID)
	}
	return event.Fingerprint
}

func groupedEventDisplayStatus(group model.AlertEventGroupVO) (string, string) {
	switch {
	case group.FiringCount > 0:
		return model.EventDisplayStatusActive, "活跃"
	case group.SuppressedCount > 0:
		return model.EventStatusSuppressed, "抑制"
	case group.SilencedCount > 0:
		return model.EventDisplayStatusSilenced, "静默"
	case group.PendingCount > 0:
		return model.EventDisplayStatusPending, "观察中"
	default:
		return model.EventDisplayStatusActive, "活跃"
	}
}

func activeEventDisplayStatus(event model.AlertActiveEvent) (string, string) {
	if event.Silenced == 1 || event.Status == model.EventStatusSilenced {
		return model.EventDisplayStatusSilenced, "静默"
	}
	if event.Status == model.EventStatusPending {
		return model.EventDisplayStatusPending, "观察中"
	}
	return model.EventDisplayStatusActive, "活跃"
}

func evalStatusName(status string) string {
	switch status {
	case model.EventStatusPending:
		return "观察中"
	case model.EventStatusFiring:
		return "告警中"
	case model.EventStatusResolved:
		return "已恢复"
	case model.EventStatusSilenced:
		return "静默"
	case model.EventStatusSuppressed:
		return "抑制"
	default:
		return status
	}
}

func durationSeconds(start, end time.Time) int64 {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0
	}
	return int64(end.Sub(start).Seconds())
}

func eventLabelString(labels model.JSONMap, key string) string {
	if labels == nil {
		return ""
	}
	value, ok := labels[key]
	if !ok || value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func ensureTriggerTimeline(timeline []model.AlertEventTimeline, eventID uint64, firstTriggeredAt, createdAt util.HTime, triggerCondition, triggerQuery, value, severity string) []model.AlertEventTimeline {
	for _, item := range timeline {
		if item.Action == "triggered" {
			return timeline
		}
	}
	triggeredAt := firstTriggeredAt
	if triggeredAt.IsZero() {
		triggeredAt = createdAt
	}
	if triggeredAt.IsZero() {
		return timeline
	}
	message := firstNonEmpty(triggerCondition, triggerQuery, "告警已触发")
	item := model.AlertEventTimeline{
		EventID:   eventID,
		Action:    "triggered",
		Message:   message,
		Operator:  "alert-engine",
		Metadata:  model.JSONMap{"value": value, "severity": severity},
		CreatedAt: triggeredAt,
	}
	return append([]model.AlertEventTimeline{item}, timeline...)
}

func eventTriggerQueries(rule *model.AlertRule, labels model.JSONMap) []model.AlertEventTriggerQuery {
	if rule == nil {
		return nil
	}
	queries := make([]model.AlertEventTriggerQuery, 0)
	if rule.RuleConfig != nil {
		for _, item := range ruleConfigQueries(rule.RuleConfig) {
			query := model.AlertEventTriggerQuery{
				Query:             firstStringFromAnyMap(item, "prom_ql", "promQl", "promql", "log_ql", "logQl", "logql", "logs_ql", "logsQl", "logsql", "query"),
				Level:             firstStringFromAnyMap(item, "level", "priority"),
				Severity:          firstStringFromAnyMap(item, "severity"),
				ConditionOperator: firstStringFromAnyMap(item, "conditionOperator", "operator"),
			}
			if value, ok := firstFloatFromMap(item, "conditionValue", "threshold", "value"); ok {
				query.ConditionValue = value
			}
			query.Condition = formatTriggerCondition(query)
			query.Matched = triggerQueryMatched(query, labels)
			if query.Query != "" {
				queries = append(queries, query)
			}
		}
	}
	if len(queries) == 0 && strings.TrimSpace(rule.Query) != "" {
		query := model.AlertEventTriggerQuery{
			Query:             strings.TrimSpace(rule.Query),
			Severity:          rule.Severity,
			ConditionOperator: rule.ConditionOperator,
			ConditionValue:    rule.ConditionValue,
			Matched:           true,
		}
		query.Condition = formatTriggerCondition(query)
		queries = append(queries, query)
	}
	if len(queries) > 0 && !hasMatchedTriggerQuery(queries) {
		queries[0].Matched = true
	}
	return queries
}

func ruleConfigQueries(ruleConfig model.JSONMap) []map[string]interface{} {
	if ruleConfig == nil {
		return nil
	}
	rawQueries, ok := ruleConfig["queries"]
	if !ok || rawQueries == nil {
		return nil
	}
	switch typed := rawQueries.(type) {
	case []interface{}:
		result := make([]map[string]interface{}, 0, len(typed))
		for _, raw := range typed {
			if item, ok := raw.(map[string]interface{}); ok {
				result = append(result, item)
			}
		}
		return result
	case []map[string]interface{}:
		return typed
	}
	return nil
}

func firstStringFromAnyMap(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func firstFloatFromMap(values map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok {
			continue
		}
		switch value := raw.(type) {
		case float64:
			return value, true
		case float32:
			return float64(value), true
		case int:
			return float64(value), true
		case int64:
			return float64(value), true
		case uint:
			return float64(value), true
		case uint64:
			return float64(value), true
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			return parsed, err == nil
		default:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
			return parsed, err == nil
		}
	}
	return 0, false
}

func triggerQueryMatched(query model.AlertEventTriggerQuery, labels model.JSONMap) bool {
	if labels == nil {
		return false
	}
	level := strings.TrimSpace(fmt.Sprint(labels["level"]))
	severity := strings.TrimSpace(fmt.Sprint(labels["severity"]))
	if query.Level != "" && strings.EqualFold(query.Level, level) {
		return true
	}
	if query.Severity != "" && strings.EqualFold(query.Severity, severity) {
		return true
	}
	return false
}

func hasMatchedTriggerQuery(queries []model.AlertEventTriggerQuery) bool {
	for _, query := range queries {
		if query.Matched {
			return true
		}
	}
	return false
}

func primaryTriggerQuery(queries []model.AlertEventTriggerQuery, rule *model.AlertRule) string {
	for _, query := range queries {
		if query.Matched && strings.TrimSpace(query.Query) != "" {
			return strings.TrimSpace(query.Query)
		}
	}
	if len(queries) > 0 {
		return strings.TrimSpace(queries[0].Query)
	}
	if rule != nil {
		return strings.TrimSpace(rule.Query)
	}
	return ""
}

func primaryTriggerCondition(queries []model.AlertEventTriggerQuery, rule *model.AlertRule) string {
	for _, query := range queries {
		if query.Matched && strings.TrimSpace(query.Condition) != "" {
			return query.Condition
		}
	}
	if len(queries) > 0 {
		return queries[0].Condition
	}
	if rule != nil {
		return formatTriggerCondition(model.AlertEventTriggerQuery{
			Query:             rule.Query,
			ConditionOperator: rule.ConditionOperator,
			ConditionValue:    rule.ConditionValue,
		})
	}
	return ""
}

func formatTriggerCondition(query model.AlertEventTriggerQuery) string {
	promQL := strings.TrimSpace(query.Query)
	operator := strings.TrimSpace(query.ConditionOperator)
	if operator == "" {
		return promQL
	}
	return strings.TrimSpace(fmt.Sprintf("%s %s %s", promQL, operator, formatFloatValue(query.ConditionValue)))
}

func formatFloatValue(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func eventDetailText(annotations model.JSONMap, rule *model.AlertRule) string {
	if annotations != nil {
		for _, key := range []string{"description", "summary", "message", "detail", "comments"} {
			if value := strings.TrimSpace(fmt.Sprint(annotations[key])); value != "" && value != "<nil>" {
				return value
			}
		}
	}
	if rule == nil {
		return ""
	}
	if rule.Annotations != nil {
		for _, key := range []string{"description", "summary", "message", "detail", "comments"} {
			if value := strings.TrimSpace(fmt.Sprint(rule.Annotations[key])); value != "" && value != "<nil>" {
				return value
			}
		}
	}
	return strings.TrimSpace(rule.Note)
}

func notifyRecordVOs(list []model.AlertNotifyRecord) []model.AlertNotifyRecordVO {
	result := make([]model.AlertNotifyRecordVO, 0, len(list))
	for _, item := range list {
		result = append(result, model.AlertNotifyRecordVO{
			ID:              item.ID,
			EventID:         item.EventID,
			EventType:       item.EventType,
			NotifyGroupKey:  item.NotifyGroupKey,
			NotifyRuleID:    item.NotifyRuleID,
			RuleID:          item.RuleID,
			RuleName:        item.RuleName,
			ChannelID:       item.ChannelID,
			ChannelName:     item.ChannelName,
			ChannelType:     item.ChannelType,
			NotifyGroupID:   item.NotifyGroupID,
			NotifyGroupName: item.NotifyGroupName,
			Receiver:        item.Receiver,
			Status:          item.Status,
			ErrorMessage:    item.ErrorMessage,
			EventCount:      item.EventCount,
			ResourceCount:   item.ResourceCount,
			Request:         item.Request,
			Response:        item.Response,
			Content:         item.Content,
			Operator:        item.Operator,
			CreatedAt:       item.CreatedAt,
			CreatedAtFull:   item.CreatedAt.Time.Format("2006-01-02 15:04:05"),
		})
	}
	return result
}

func (s *AlertManagerServiceImpl) CreateSilenceRule(c *gin.Context, item model.AlertSilenceRule) {
	operator := currentOperator(c)
	item.CreatedBy = operator
	item.UpdatedBy = operator
	if err := validateSilenceRule(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if item.Enabled == 0 {
		item.Enabled = model.RuleStatusEnabled
	}
	events, err := s.dao.CreateSilenceRuleAndReconcile(&item, operator)
	if err != nil {
		result.Failed(c, 500, "创建屏蔽规则失败: "+err.Error())
		return
	}
	alertengine.InvalidateSilenceCache()
	for _, event := range events {
		alertengine.CacheActiveEventState(event)
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) UpdateSilenceRule(c *gin.Context, item model.AlertSilenceRule) {
	if item.ID == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item.CreatedBy = ""
	item.UpdatedBy = currentOperator(c)
	if err := validateSilenceRule(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	events, err := s.dao.UpdateSilenceRuleAndReconcile(&item, item.UpdatedBy)
	if err != nil {
		result.Failed(c, 500, "更新屏蔽规则失败: "+err.Error())
		return
	}
	alertengine.InvalidateSilenceCache()
	for _, event := range events {
		alertengine.CacheActiveEventState(event)
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) DeleteSilenceRule(c *gin.Context, id uint) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	events, err := s.dao.CancelSilenceRuleAndReconcile(id, currentOperator(c))
	if err != nil {
		result.Failed(c, 500, "取消静默规则失败: "+err.Error())
		return
	}
	alertengine.InvalidateSilenceCache()
	for _, event := range events {
		alertengine.CacheActiveEventState(event)
	}
	result.Success(c, nil)
}

func (s *AlertManagerServiceImpl) GetSilenceRule(c *gin.Context, id uint) {
	item, err := s.dao.GetSilenceRule(id)
	if err != nil {
		result.Failed(c, 500, "查询屏蔽规则失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) ListSilenceRules(c *gin.Context, query model.AlertSilenceRuleQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	list, total, err := s.dao.ListSilenceRules(query)
	if err != nil {
		result.Failed(c, 500, "查询屏蔽规则失败: "+err.Error())
		return
	}
	result.SuccessWithPage(c, list, total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) CreateNotifyChannel(c *gin.Context, item model.AlertNotifyChannel) {
	operator := currentOperator(c)
	item.CreatedBy = operator
	item.UpdatedBy = operator
	alertnotify.EnsureFeishuEventMode(&item)
	if err := validateNotifyChannel(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if item.Enabled == 0 {
		item.Enabled = model.RuleStatusEnabled
	}
	if err := s.dao.CreateNotifyChannel(&item); err != nil {
		result.Failed(c, 500, "创建通知媒介失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	refreshFeishuEventTransports()
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) UpdateNotifyChannel(c *gin.Context, item model.AlertNotifyChannel) {
	if item.ID == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item.CreatedBy = ""
	item.UpdatedBy = currentOperator(c)
	if strings.TrimSpace(item.Type) == model.NotifyChannelFeishuCard {
		existing, err := s.dao.GetNotifyChannel(item.ID)
		if err != nil {
			result.Failed(c, 500, "查询通知媒介失败: "+err.Error())
			return
		}
		alertnotify.PreserveFeishuServerConfig(&item, existing.Config)
	}
	alertnotify.EnsureFeishuEventMode(&item)
	if err := validateNotifyChannel(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if err := s.dao.UpdateNotifyChannel(&item); err != nil {
		result.Failed(c, 500, "更新通知媒介失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	refreshFeishuEventTransports()
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) DeleteNotifyChannel(c *gin.Context, id uint) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	if err := s.dao.DeleteNotifyChannel(id); err != nil {
		result.Failed(c, 500, "删除通知媒介失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	refreshFeishuEventTransports()
	result.Success(c, nil)
}

func (s *AlertManagerServiceImpl) GetNotifyChannel(c *gin.Context, id uint) {
	item, err := s.dao.GetNotifyChannel(id)
	if err != nil {
		result.Failed(c, 500, "查询通知媒介失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) ListNotifyChannels(c *gin.Context, query model.AlertNotifyChannelQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	list, total, err := s.dao.ListNotifyChannels(query)
	if err != nil {
		result.Failed(c, 500, "查询通知媒介失败: "+err.Error())
		return
	}
	result.SuccessWithPage(c, list, total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) TestNotifyChannel(c *gin.Context, id uint, req model.AlertNotifyChannelTestRequest) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	channel, err := s.dao.GetNotifyChannel(id)
	if err != nil {
		result.Failed(c, 500, "查询通知媒介失败: "+err.Error())
		return
	}
	if channel.Enabled != model.RuleStatusEnabled {
		result.Failed(c, 400, "通知媒介已禁用")
		return
	}
	if err := alertnotify.ValidateChannel(*channel); err != nil {
		result.Failed(c, 400, "通知媒介配置错误: "+err.Error())
		return
	}
	if channel.Type == model.NotifyChannelFeishuCard {
		cfg, cfgErr := alertnotify.ParseFeishuCardAppConfig(channel.Config)
		if cfgErr != nil {
			log.Log().Warnf("feishu card test parse config failed: channel=%d name=%s err=%v", channel.ID, channel.Name, cfgErr)
		} else {
			log.Log().Infof("feishu card test begin: channel=%d name=%s appId=%s receiveIdType=%s defaultReceiveId=%s lastChatId=%s userIds=%v receivers=%v templateId=%d",
				channel.ID, channel.Name, cfg.AppID, cfg.ReceiveIDType, cfg.DefaultReceiveID, cfg.LastChatID, req.UserIDs, req.Receivers, req.TemplateID)
		}
	}
	title := req.Title
	content := req.Content
	var card model.JSONMap
	tplReq := model.AlertNotifyTemplatePreviewRequest{
		IsRecovered: req.IsRecovered,
		Event:       req.Event,
	}
	if req.TemplateID > 0 || strings.TrimSpace(channel.Type) != "" {
		var tpl *model.AlertNotifyTemplate
		if req.TemplateID > 0 {
			tpl, err = s.dao.GetNotifyTemplate(req.TemplateID)
		} else {
			tpl, err = s.dao.GetDefaultNotifyTemplateByChannelType(channel.Type)
		}
		if err != nil {
			if req.TemplateID > 0 {
				result.Failed(c, 500, "查询通知模板失败: "+err.Error())
				return
			}
		}
		if tpl != nil && tpl.ID > 0 {
			if strings.TrimSpace(tpl.ChannelType) != strings.TrimSpace(channel.Type) {
				result.Failed(c, 400, "通知模板与通知媒介类型不匹配")
				return
			}
			title, content, card, err = alertnotify.BuildChannelTemplatePayload(channel.Type, *tpl, tplReq)
			if err != nil {
				result.Failed(c, 400, "通知模板渲染失败: "+err.Error())
				return
			}
		}
	}
	sendResult := alertnotify.Send(c.Request.Context(), alertnotify.SendRequest{
		Channel:   *channel,
		Title:     title,
		Content:   content,
		Card:      card,
		Receivers: req.Receivers,
		UserIDs:   req.UserIDs,
	})
	if channel.Type == model.NotifyChannelFeishuCard {
		log.Log().Infof("feishu card test result: channel=%d target=%s status=%s err=%s", channel.ID, sendResult.Target, sendResult.Status, sendResult.ErrorMessage)
	}
	resp := model.AlertNotifyChannelTestResponse{
		ChannelID:    channel.ID,
		ChannelName:  channel.Name,
		ChannelType:  channel.Type,
		Target:       sendResult.Target,
		Status:       sendResult.Status,
		Response:     sendResult.Response,
		ErrorMessage: sendResult.ErrorMessage,
		DurationMs:   sendResult.DurationMs,
		SentAt:       util.HTime{Time: time.Now()},
	}
	if sendResult.Status == model.NotifyStatusFailed {
		if channel.Type == model.NotifyChannelFeishuCard && strings.Contains(sendResult.ErrorMessage, "defaultReceiveId or lastChatId or userIds is required") {
			result.Failed(c, 400, "测试发送失败: 飞书应用卡片需要指定接收对象，请先通过飞书事件回调自动记录群 chat_id，或在媒介配置中填写 defaultReceiveId，或在测试时传 userIds（如 chat_id/open_id/user_id/email）")
			return
		}
		result.Failed(c, 500, "测试发送失败: "+sendResult.ErrorMessage)
		return
	}
	result.Success(c, resp)
}

type feishuHTTPCallbackMatch struct {
	channel  model.AlertNotifyChannel
	config   alertnotify.FeishuCardAppConfig
	envelope *alertnotify.FeishuEventEnvelope
	payload  []byte
}

func matchFeishuHTTPCallback(headers http.Header, raw []byte, channels []model.AlertNotifyChannel) (*feishuHTTPCallbackMatch, error) {
	encrypted, err := alertnotify.FeishuEventPayloadEncrypted(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid callback JSON: %w", err)
	}
	candidates := 0
	encryptionPolicyMismatch := 0
	signatureFailed := 0
	decodeFailed := 0
	tokenMismatch := 0
	appIDMismatch := 0
	for i := range channels {
		channel := channels[i]
		if strings.TrimSpace(channel.Type) != model.NotifyChannelFeishuCard || channel.Enabled != model.RuleStatusEnabled {
			continue
		}
		cfg, cfgErr := alertnotify.ParseFeishuCardAppConfig(channel.Config)
		if cfgErr != nil {
			continue
		}
		candidates++
		configuredEncryption := strings.TrimSpace(cfg.EncryptKey) != ""
		if configuredEncryption != encrypted {
			encryptionPolicyMismatch++
			continue
		}
		if encrypted {
			if sigErr := alertnotify.VerifyFeishuEventSignature(headers, raw, cfg.EncryptKey); sigErr != nil {
				signatureFailed++
				continue
			}
		}
		payload, decodeErr := alertnotify.DecodeFeishuEventPayload(raw, cfg.EncryptKey)
		if decodeErr != nil {
			decodeFailed++
			continue
		}
		envelope, parseErr := alertnotify.ParseFeishuEventPayload(payload, "")
		if parseErr != nil {
			decodeFailed++
			continue
		}
		callbackToken := strings.TrimSpace(envelope.Header.Token)
		if callbackToken == "" {
			callbackToken = strings.TrimSpace(envelope.Token)
		}
		if callbackToken == "" || callbackToken != strings.TrimSpace(cfg.VerificationToken) {
			tokenMismatch++
			continue
		}
		callbackAppID := strings.TrimSpace(envelope.Header.AppID)
		if callbackAppID != "" && callbackAppID != strings.TrimSpace(cfg.AppID) {
			appIDMismatch++
			continue
		}
		if strings.TrimSpace(envelope.Type) != "url_verification" && callbackAppID == "" {
			appIDMismatch++
			continue
		}
		return &feishuHTTPCallbackMatch{channel: channel, config: cfg, envelope: envelope, payload: payload}, nil
	}
	return nil, fmt.Errorf("no enabled callback channel matched: encrypted=%t candidates=%d encryptionPolicyMismatch=%d signatureFailed=%d decodeFailed=%d tokenMismatch=%d appIdMismatch=%d",
		encrypted, candidates, encryptionPolicyMismatch, signatureFailed, decodeFailed, tokenMismatch, appIDMismatch)
}

func (s *AlertManagerServiceImpl) HandleFeishuCallback(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}
	log.Log().Infof("feishu http callback received: bytes=%d signed=%t", len(raw), strings.TrimSpace(c.GetHeader("X-Lark-Signature")) != "")
	channels, err := s.dao.ListAllNotifyChannels()
	if err != nil {
		log.Log().Warnf("feishu http callback list channels failed: %v", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "internal server error"})
		return
	}
	matched, err := matchFeishuHTTPCallback(c.Request.Header, raw, channels)
	if err != nil {
		log.Log().Warnf("feishu http callback rejected: %v", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "callback verification failed"})
		return
	}

	watcher := sharedFeishuWatcher()
	appID := strings.TrimSpace(matched.config.AppID)
	if !matched.config.UsesHTTPCallbacks() {
		newConfig := cloneJSONMap(matched.channel.Config)
		newConfig["eventMode"] = alertnotify.FeishuEventModeHTTP
		if updateErr := s.dao.UpdateNotifyChannelConfig(matched.channel.ID, newConfig, "system"); updateErr != nil {
			log.Log().Warnf("feishu http callback enable HTTP mode failed: channel=%d appId=%s err=%v", matched.channel.ID, appID, updateErr)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "failed to enable HTTP callback mode"})
			return
		}
		matched.channel.Config = newConfig
		matched.config.EventMode = alertnotify.FeishuEventModeHTTP
		alertengine.InvalidateNotifyConfigCache()
		log.Log().Infof("feishu callback transport automatically switched to HTTP: channel=%d appId=%s", matched.channel.ID, appID)
	}
	watcher.stopWebSocketApp(appID)
	if strings.TrimSpace(matched.envelope.Type) == "url_verification" {
		log.Log().Infof("feishu http callback url verification succeeded: channel=%d appId=%s encrypted=%t", matched.channel.ID, appID, strings.TrimSpace(matched.config.EncryptKey) != "")
		c.JSON(http.StatusOK, gin.H{"challenge": matched.envelope.Challenge})
		return
	}

	eventType := strings.TrimSpace(matched.envelope.Header.EventType)
	if eventType != "im.message.receive_v1" && eventType != "card.action.trigger" {
		log.Log().Infof("feishu http callback ignored event: channel=%d appId=%s eventType=%s", matched.channel.ID, appID, eventType)
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	eventID := strings.TrimSpace(matched.envelope.Header.EventID)
	if eventID == "" {
		log.Log().Warnf("feishu http callback rejected: channel=%d appId=%s eventType=%s reason=missing_event_id", matched.channel.ID, appID, eventType)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "event_id is required"})
		return
	}
	claimed, claim := watcher.claimHTTPEvent(c.Request.Context(), appID, eventID)
	if !claimed {
		log.Log().Infof("feishu http callback duplicate ignored: channel=%d appId=%s eventType=%s eventId=%s", matched.channel.ID, appID, eventType, eventID)
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	response, dispatchErr := watcher.dispatchHTTPEvent(c.Request.Context(), matched.channel.ID, matched.payload)
	if dispatchErr != nil {
		watcher.releaseHTTPEventClaim(claim)
		log.Log().Warnf("feishu http callback dispatch failed: channel=%d appId=%s eventType=%s eventId=%s err=%v", matched.channel.ID, appID, eventType, eventID, dispatchErr)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "event dispatch failed"})
		return
	}
	log.Log().Infof("feishu http callback completed: channel=%d appId=%s eventType=%s eventId=%s", matched.channel.ID, appID, eventType, eventID)
	if response != nil {
		c.JSON(http.StatusOK, response)
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}

func (s *AlertManagerServiceImpl) CreateNotifyGroup(c *gin.Context, item model.AlertNotifyGroup) {
	operator := currentOperator(c)
	item.CreatedBy = operator
	item.UpdatedBy = operator
	applyNotifyGroupDefaults(&item)
	if item.Enabled == 0 {
		item.Enabled = model.RuleStatusEnabled
	}
	if err := validateNotifyGroup(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if s.dao.NotifyGroupBusinessExists(item.BusinessID, 0) {
		result.Failed(c, 400, "业务组已存在通知群组")
		return
	}
	if err := s.dao.CreateNotifyGroup(&item); err != nil {
		result.Failed(c, 500, "创建通知群组失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) UpdateNotifyGroup(c *gin.Context, item model.AlertNotifyGroup) {
	if item.ID == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item.CreatedBy = ""
	item.UpdatedBy = currentOperator(c)
	applyNotifyGroupDefaults(&item)
	if err := validateNotifyGroup(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if s.dao.NotifyGroupBusinessExists(item.BusinessID, item.ID) {
		result.Failed(c, 400, "业务组已存在通知群组")
		return
	}
	if err := s.dao.UpdateNotifyGroup(&item); err != nil {
		result.Failed(c, 500, "更新通知群组失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) DeleteNotifyGroup(c *gin.Context, id uint) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	if references, err := s.dao.NotifyGroupReferenceCount(id); err != nil {
		result.Failed(c, 500, "校验通知群组引用失败: "+err.Error())
		return
	} else if references > 0 {
		result.Failed(c, 409, "通知群组仍被通知策略或业务路由引用，请先解除关联")
		return
	}
	if err := s.dao.DeleteNotifyGroup(id); err != nil {
		result.Failed(c, 500, "删除通知群组失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	result.Success(c, nil)
}

func (s *AlertManagerServiceImpl) GetNotifyGroup(c *gin.Context, id uint) {
	item, err := s.dao.GetNotifyGroup(id)
	if err != nil {
		result.Failed(c, 500, "查询通知群组失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) ListNotifyGroups(c *gin.Context, query model.AlertNotifyGroupQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	list, total, err := s.dao.ListNotifyGroups(query)
	if err != nil {
		result.Failed(c, 500, "查询通知群组失败: "+err.Error())
		return
	}
	result.SuccessWithPage(c, list, total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) CreateNotifyRouteRule(c *gin.Context, item model.AlertNotifyRouteRule) {
	operator := currentOperator(c)
	item.CreatedBy = operator
	item.UpdatedBy = operator
	normalizeNotifyRouteRule(&item)
	if item.Enabled == 0 {
		item.Enabled = model.RuleStatusEnabled
	}
	if err := validateNotifyRouteRule(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if item.NotifyRuleID > 0 {
		if _, err := s.dao.GetNotifyRule(item.NotifyRuleID); err != nil {
			result.Failed(c, 400, "通知策略不存在")
			return
		}
	}
	if s.dao.NotifyRouteRuleNameExists(item.Name, 0) {
		result.Failed(c, 400, "业务路由规则名称已存在")
		return
	}
	if _, err := s.dao.GetEnabledNotifyGroupByID(item.NotifyGroupID); err != nil {
		result.Failed(c, 400, "目标通知群组不存在或已禁用")
		return
	}
	if err := s.dao.CreateNotifyRouteRule(&item); err != nil {
		result.Failed(c, 500, "创建业务路由规则失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) UpdateNotifyRouteRule(c *gin.Context, item model.AlertNotifyRouteRule) {
	if item.ID == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item.UpdatedBy = currentOperator(c)
	normalizeNotifyRouteRule(&item)
	if err := validateNotifyRouteRule(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if item.NotifyRuleID > 0 {
		if _, err := s.dao.GetNotifyRule(item.NotifyRuleID); err != nil {
			result.Failed(c, 400, "通知策略不存在")
			return
		}
	}
	if s.dao.NotifyRouteRuleNameExists(item.Name, item.ID) {
		result.Failed(c, 400, "业务路由规则名称已存在")
		return
	}
	if _, err := s.dao.GetEnabledNotifyGroupByID(item.NotifyGroupID); err != nil && item.Enabled == model.RuleStatusEnabled {
		result.Failed(c, 400, "目标通知群组不存在或已禁用")
		return
	}
	if err := s.dao.UpdateNotifyRouteRule(&item); err != nil {
		result.Failed(c, 500, "更新业务路由规则失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) DeleteNotifyRouteRule(c *gin.Context, id uint) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	if err := s.dao.DeleteNotifyRouteRule(id); err != nil {
		result.Failed(c, 500, "删除业务路由规则失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	result.Success(c, nil)
}

func (s *AlertManagerServiceImpl) GetNotifyRouteRule(c *gin.Context, id uint) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item, err := s.dao.GetNotifyRouteRule(id)
	if err != nil {
		result.Failed(c, 500, "查询业务路由规则失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) ListNotifyRouteRules(c *gin.Context, query model.AlertNotifyRouteRuleQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	list, total, err := s.dao.ListNotifyRouteRules(query)
	if err != nil {
		result.Failed(c, 500, "查询业务路由规则失败: "+err.Error())
		return
	}
	result.SuccessWithPage(c, list, total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) CreateNotifyTemplate(c *gin.Context, item model.AlertNotifyTemplate) {
	operator := currentOperator(c)
	item.CreatedBy = operator
	item.UpdatedBy = operator
	applyNotifyTemplateDefaults(&item)
	if err := validateNotifyTemplate(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if err := s.dao.CreateNotifyTemplate(&item); err != nil {
		result.Failed(c, 500, "创建通知模板失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) UpdateNotifyTemplate(c *gin.Context, item model.AlertNotifyTemplate) {
	if item.ID == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	existing, err := s.dao.GetNotifyTemplate(item.ID)
	if err != nil {
		result.Failed(c, 500, "查询通知模板失败: "+err.Error())
		return
	}
	item.CreatedBy = ""
	item.UpdatedBy = currentOperator(c)
	if strings.TrimSpace(item.Ident) == "" {
		item.Ident = existing.Ident
	}
	if existing.Builtin == 1 {
		item.Builtin = 1
	}
	applyNotifyTemplateDefaults(&item)
	if err := validateNotifyTemplate(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if err := s.dao.UpdateNotifyTemplate(&item); err != nil {
		result.Failed(c, 500, "更新通知模板失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) DeleteNotifyTemplate(c *gin.Context, id uint) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item, err := s.dao.GetNotifyTemplate(id)
	if err != nil {
		result.Failed(c, 500, "查询通知模板失败: "+err.Error())
		return
	}
	if item.Builtin == 1 {
		result.Failed(c, 400, "内置通知模板不允许删除")
		return
	}
	if err := s.dao.DeleteNotifyTemplate(id); err != nil {
		result.Failed(c, 500, "删除通知模板失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	result.Success(c, nil)
}

func (s *AlertManagerServiceImpl) GetNotifyTemplate(c *gin.Context, id uint) {
	item, err := s.dao.GetNotifyTemplate(id)
	if err != nil {
		result.Failed(c, 500, "查询通知模板失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) ListNotifyTemplates(c *gin.Context, query model.AlertNotifyTemplateQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	list, total, err := s.dao.ListNotifyTemplates(query)
	if err != nil {
		result.Failed(c, 500, "查询通知模板失败: "+err.Error())
		return
	}
	result.SuccessWithPage(c, list, total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) PreviewNotifyTemplate(c *gin.Context, id uint, req model.AlertNotifyTemplatePreviewRequest) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item, err := s.dao.GetNotifyTemplate(id)
	if err != nil {
		result.Failed(c, 500, "查询通知模板失败: "+err.Error())
		return
	}
	resp, err := alertnotify.PreviewTemplate(*item, req)
	if err != nil {
		result.Failed(c, 400, "通知模板渲染失败: "+err.Error())
		return
	}
	result.Success(c, resp)
}

func (s *AlertManagerServiceImpl) CreateNotifyRule(c *gin.Context, item model.AlertNotifyRule) {
	operator := currentOperator(c)
	item.CreatedBy = operator
	item.UpdatedBy = operator
	applyNotifyRuleCreateDefaults(&item)
	if err := validateNotifyRule(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if err := s.validateNotifyRuleGroupIDs(item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if err := s.dao.CreateNotifyRule(&item); err != nil {
		result.Failed(c, 500, "创建通知规则失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) UpdateNotifyRule(c *gin.Context, item model.AlertNotifyRule) {
	if item.ID == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	item.CreatedBy = ""
	item.UpdatedBy = currentOperator(c)
	normalizeNotifyRuleForSave(&item)
	if err := validateNotifyRule(&item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if err := s.validateNotifyRuleGroupIDs(item); err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	if err := s.dao.UpdateNotifyRule(&item); err != nil {
		result.Failed(c, 500, "更新通知规则失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) DeleteNotifyRule(c *gin.Context, id uint) {
	if id == 0 {
		result.Failed(c, 400, "id is required")
		return
	}
	if references, err := s.dao.NotifyRuleReferenceCount(id); err != nil {
		result.Failed(c, 500, "校验通知策略引用失败: "+err.Error())
		return
	} else if references > 0 {
		result.Failed(c, 409, "通知策略仍被告警规则或业务路由引用，请先解除关联")
		return
	}
	if err := s.dao.DeleteNotifyRule(id); err != nil {
		result.Failed(c, 500, "删除通知规则失败: "+err.Error())
		return
	}
	alertengine.InvalidateNotifyConfigCache()
	result.Success(c, nil)
}

func (s *AlertManagerServiceImpl) GetNotifyRule(c *gin.Context, id uint) {
	item, err := s.dao.GetNotifyRule(id)
	if err != nil {
		result.Failed(c, 500, "查询通知规则失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func (s *AlertManagerServiceImpl) ListNotifyRules(c *gin.Context, query model.AlertNotifyRuleQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	list, total, err := s.dao.ListNotifyRules(query)
	if err != nil {
		result.Failed(c, 500, "查询通知规则失败: "+err.Error())
		return
	}
	result.SuccessWithPage(c, list, total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) ListNotifyRecords(c *gin.Context, query model.AlertNotifyRecordQuery) {
	query.Page, query.PageSize = model.NormalizePage(query.Page, query.PageSize)
	list, total, err := s.dao.ListNotifyRecords(query)
	if err != nil {
		result.Failed(c, 500, "查询通知记录失败: "+err.Error())
		return
	}
	result.SuccessWithPage(c, notifyRecordVOs(list), total, query.Page, query.PageSize)
}

func (s *AlertManagerServiceImpl) GetNotifyRecordStats(c *gin.Context, query model.AlertNotifyRecordQuery) {
	stats, err := s.dao.NotifyRecordStats(query)
	if err != nil {
		result.Failed(c, 500, "查询通知记录统计失败: "+err.Error())
		return
	}
	result.Success(c, stats)
}

func validateRuleGroup(item *model.AlertRuleGroup) error {
	return validateNamed(item.Name)
}

func (s *AlertManagerServiceImpl) validateRuleNotifyRuleIDs(ids model.JSONUintArray) error {
	normalized := normalizeUintIDs([]uint(ids))
	if len(normalized) == 0 {
		return nil
	}
	count, err := s.dao.CountEnabledNotifyRulesByIDs(normalized)
	if err != nil {
		return fmt.Errorf("校验通知策略失败: %w", err)
	}
	if count != int64(len(normalized)) {
		return errors.New("关联的通知策略不存在或已禁用")
	}
	return nil
}

func (s *AlertManagerServiceImpl) validateNotifyRuleGroupIDs(item model.AlertNotifyRule) error {
	policy, err := alertnotify.ParseNotifyPolicy(item)
	if err != nil {
		return err
	}
	ids := make([]uint, 0)
	seen := make(map[uint]struct{})
	for _, channel := range policy.Channels {
		if channel.ReceiverGroupID == 0 {
			continue
		}
		if _, ok := seen[channel.ReceiverGroupID]; ok {
			continue
		}
		seen[channel.ReceiverGroupID] = struct{}{}
		ids = append(ids, channel.ReceiverGroupID)
	}
	if len(ids) == 0 {
		return nil
	}
	count, err := s.dao.CountEnabledNotifyGroupsByIDs(ids)
	if err != nil {
		return fmt.Errorf("校验通知群组失败: %w", err)
	}
	if count != int64(len(ids)) {
		return errors.New("通知策略关联的通知群组不存在或已禁用")
	}
	return nil
}

func validateRule(item *model.AlertRule) error {
	if err := validateNamed(item.Name); err != nil {
		return err
	}
	if strings.TrimSpace(item.DatasourceType) == "" {
		return errors.New("datasourceType is required")
	}
	if strings.TrimSpace(item.Query) == "" && extractRuleConfigQuery(item.RuleConfig) == "" {
		return errors.New("query or ruleConfig.queries is required")
	}
	if item.EvalIntervalSeconds < 1 {
		return errors.New("evalIntervalSeconds must be greater than 0")
	}
	if item.ForDurationCount < 0 {
		return errors.New("forDurationCount must be greater than or equal to 0")
	}
	if item.ForDurationSeconds < 0 {
		return errors.New("forDurationSeconds must be greater than or equal to 0")
	}
	return nil
}

func validateDatasource(item *model.AlertDatasource) error {
	if err := validateNamed(item.Name); err != nil {
		return err
	}
	if strings.TrimSpace(item.DatasourceType) == "" {
		return errors.New("datasourceType is required")
	}
	return nil
}

func validateBuiltinRuleTemplate(item *model.AlertBuiltinRuleTemplate) error {
	if err := validateNamed(item.Name); err != nil {
		return err
	}
	if strings.TrimSpace(item.Category) == "" {
		return errors.New("category is required")
	}
	if strings.TrimSpace(item.DatasourceType) == "" {
		return errors.New("datasourceType is required")
	}
	if strings.TrimSpace(item.QueryTemplate) == "" {
		return errors.New("queryTemplate is required")
	}
	return nil
}

func isPlatformBuiltinRuleTemplate(item *model.AlertBuiltinRuleTemplate) bool {
	return item != nil && strings.EqualFold(strings.TrimSpace(item.Source), "platform")
}

func (s *AlertManagerServiceImpl) normalizeLogAlertAssociation(item *model.AlertRule) error {
	if item == nil || !isLogAlertRule(*item) {
		return nil
	}
	var datasource model.AlertDatasource
	if item.DatasourceID > 0 {
		if err := s.dao.DB().Where("id = ? AND datasource_type = ? AND enabled = ?", item.DatasourceID, model.AlertDatasourceVictoriaLogs, model.RuleStatusEnabled).First(&datasource).Error; err == nil {
			item.DatasourceType = model.AlertDatasourceVictoriaLogs
		} else {
			item.DatasourceID = 0
		}
	}
	if item.DatasourceID == 0 {
		if err := s.dao.DB().Where("datasource_type = ? AND enabled = ?", model.AlertDatasourceVictoriaLogs, model.RuleStatusEnabled).
			Order("is_default DESC, builtin DESC, id ASC").First(&datasource).Error; err != nil {
			return errors.New("未找到启用的 VictoriaLogs 日志数据源")
		}
		item.DatasourceID = datasource.ID
		item.DatasourceType = model.AlertDatasourceVictoriaLogs
	}
	var group model.AlertRuleGroup
	if err := s.dao.DB().Where("name = ? AND parent_id = ?", "日志告警", 0).First(&group).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("查询日志告警分组失败: %w", err)
		}
		group = model.AlertRuleGroup{Name: "日志告警", Description: "VictoriaLogs LogsQL 日志告警规则", Enabled: model.RuleStatusEnabled, CreatedBy: item.UpdatedBy, UpdatedBy: item.UpdatedBy}
		if err := s.dao.CreateRuleGroup(&group); err != nil {
			return fmt.Errorf("创建日志告警分组失败: %w", err)
		}
	}
	item.GroupID = group.ID
	return nil
}

func isLogAlertRule(item model.AlertRule) bool {
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(item.Labels["alert_type"])), "log") {
		return true
	}
	return item.DatasourceType == model.AlertDatasourceVictoriaLogs && item.RuleConfig != nil && (item.RuleConfig["projectCode"] != nil || item.RuleConfig["storeCode"] != nil)
}

func validateBuiltinTemplateGroup(item *model.AlertBuiltinRuleTemplateGroup) error {
	if err := validateNamed(item.Name); err != nil {
		return err
	}
	if strings.TrimSpace(item.Value) == "" {
		return errors.New("value is required")
	}
	switch item.Level {
	case model.TemplateGroupLevelComponent:
		if item.ParentID != 0 {
			return errors.New("component group parentId must be 0")
		}
	case model.TemplateGroupLevelCollector, model.TemplateGroupLevelCategory:
		if item.ParentID == 0 {
			return errors.New("parentId is required")
		}
	default:
		return errors.New("level must be component, collector or category")
	}
	return nil
}

func validateNotifyChannel(item *model.AlertNotifyChannel) error {
	if err := validateNamed(item.Name); err != nil {
		return err
	}
	return alertnotify.ValidateChannel(*item)
}

func applyNotifyGroupDefaults(item *model.AlertNotifyGroup) {
	item.Name = strings.TrimSpace(item.Name)
	item.BusinessID = strings.TrimSpace(item.BusinessID)
	item.ChannelType = strings.TrimSpace(item.ChannelType)
	if item.ChannelType == "" {
		item.ChannelType = model.NotifyChannelLeliao
	}
	item.UserIDs = normalizeStringArray(item.UserIDs)
}

func validateNotifyGroup(item *model.AlertNotifyGroup) error {
	if err := validateNamed(item.Name); err != nil {
		return err
	}
	if strings.TrimSpace(item.BusinessID) == "" {
		return errors.New("businessId is required")
	}
	if item.ChannelType != model.NotifyChannelLeliao {
		return errors.New("channelType currently only supports leliao")
	}
	if len(item.UserIDs) == 0 {
		return errors.New("userIds is required")
	}
	return nil
}

func normalizeNotifyRouteRule(item *model.AlertNotifyRouteRule) {
	item.Name = strings.TrimSpace(item.Name)
	item.Conditions = normalizeNotifyRouteConditions(item.Conditions)
	item.Description = strings.TrimSpace(item.Description)
}

func validateNotifyRouteRule(item *model.AlertNotifyRouteRule) error {
	if err := validateNamed(item.Name); err != nil {
		return err
	}
	if len(item.Conditions) == 0 {
		return errors.New("conditions is required")
	}
	for index, condition := range item.Conditions {
		if strings.TrimSpace(condition.LabelKey) == "" {
			return fmt.Errorf("conditions[%d].labelKey is required", index)
		}
		if strings.TrimSpace(condition.MatchValue) == "" {
			return fmt.Errorf("conditions[%d].matchValue is required", index)
		}
		switch condition.MatchMode {
		case model.NotifyRouteMatchExact, model.NotifyRouteMatchContains, model.NotifyRouteMatchRegex:
		default:
			return fmt.Errorf("conditions[%d].matchMode must be exact, contains or regex", index)
		}
		if condition.MatchMode == model.NotifyRouteMatchRegex {
			if _, err := regexp.Compile(condition.MatchValue); err != nil {
				return fmt.Errorf("invalid conditions[%d].matchValue regex %q: %w", index, condition.MatchValue, err)
			}
		}
	}
	if item.NotifyGroupID == 0 {
		return errors.New("notifyGroupId is required")
	}
	if item.OverrideGroup != 0 && item.OverrideGroup != 1 {
		return errors.New("overrideGroup must be 0 or 1")
	}
	if item.Priority < 0 {
		return errors.New("priority cannot be negative")
	}
	return nil
}

func normalizeNotifyRouteConditions(conditions model.AlertNotifyRouteConditions) model.AlertNotifyRouteConditions {
	result := make(model.AlertNotifyRouteConditions, 0, len(conditions))
	for _, condition := range conditions {
		condition.LabelKey = strings.TrimSpace(condition.LabelKey)
		condition.MatchMode = strings.ToLower(strings.TrimSpace(condition.MatchMode))
		if condition.MatchMode == "" {
			condition.MatchMode = model.NotifyRouteMatchExact
		}
		condition.MatchValue = strings.TrimSpace(condition.MatchValue)
		result = append(result, condition)
	}
	return result
}

func normalizeStringArray(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text == "" {
			continue
		}
		if _, ok := seen[text]; ok {
			continue
		}
		seen[text] = struct{}{}
		result = append(result, text)
	}
	return result
}

func validateNotifyTemplate(item *model.AlertNotifyTemplate) error {
	if err := validateNamed(item.Name); err != nil {
		return err
	}
	if !alertnotify.IsSupportedChannelType(item.ChannelType) {
		return errors.New("channelType must be feishu, feishu_card, dingtalk, wecom or leliao")
	}
	switch strings.TrimSpace(item.TemplateType) {
	case model.NotifyTemplateMarkdown, model.NotifyTemplateCard:
	default:
		return errors.New("templateType must be markdown or card")
	}
	if len(item.Content) == 0 {
		return errors.New("content is required")
	}
	if !notifyTemplateContentHasValue(item.Content, "title") &&
		!notifyTemplateContentHasValue(item.Content, "content") &&
		!notifyTemplateContentHasValue(item.Content, "raw") {
		return errors.New("content.title, content.content or content.raw is required")
	}
	return nil
}

func applyNotifyTemplateDefaults(item *model.AlertNotifyTemplate) {
	item.Ident = strings.TrimSpace(item.Ident)
	item.Name = strings.TrimSpace(item.Name)
	item.ChannelType = strings.TrimSpace(item.ChannelType)
	item.TemplateType = strings.TrimSpace(item.TemplateType)
	if item.TemplateType == "" {
		if item.ChannelType == model.NotifyChannelFeishuCard {
			item.TemplateType = model.NotifyTemplateCard
		} else {
			item.TemplateType = model.NotifyTemplateMarkdown
		}
	}
	if item.Ident == "" {
		item.Ident = fmt.Sprintf("custom_%s_%d", strings.ReplaceAll(item.ChannelType, "-", "_"), time.Now().UnixNano())
	}
	if item.Enabled == 0 {
		item.Enabled = model.RuleStatusEnabled
	}
}

func notifyTemplateContentHasValue(content model.JSONMap, key string) bool {
	value, ok := content[key]
	if !ok || value == nil {
		return false
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	return text != "" && text != "<nil>"
}

func applyNotifyRuleDefaults(item *model.AlertNotifyRule) {
	applyNotifyRuleCreateDefaults(item)
}

func applyNotifyRuleCreateDefaults(item *model.AlertNotifyRule) {
	normalizeNotifyRuleForSave(item)
	if item.Enabled == 0 {
		item.Enabled = model.RuleStatusEnabled
	}
	if item.RepeatNotify == 0 {
		item.RepeatNotify = 1
	}
	if item.RepeatIntervalSeconds == 0 {
		item.RepeatIntervalSeconds = 3600
	}
	if item.AggregationEnabled == 0 {
		item.AggregationEnabled = 1
	}
	if item.GroupWaitSeconds == 0 {
		item.GroupWaitSeconds = 30
	}
	if item.GroupIntervalSeconds == 0 {
		item.GroupIntervalSeconds = 300
	}
	if item.RecoveryNotify == 0 {
		item.RecoveryNotify = 1
	}
	if item.AggregationWindowSeconds == 0 {
		item.AggregationWindowSeconds = item.GroupWaitSeconds
	}
	item.RepeatIntervals = withDefaultNotifyRepeatIntervals(item.RepeatIntervals)
}

func normalizeNotifyRuleForSave(item *model.AlertNotifyRule) {
	item.Name = strings.TrimSpace(item.Name)
	item.RepeatMode = strings.TrimSpace(item.RepeatMode)
	if item.RepeatMode == "" {
		item.RepeatMode = "bySeverity"
	}
	if strings.EqualFold(item.RepeatMode, "severity") {
		item.RepeatMode = "bySeverity"
	}
	item.RepeatIntervals = normalizeNotifyRepeatIntervals(item.RepeatIntervals)
	if item.AggregationWindowSeconds == 0 && item.AggregationEnabled == 1 {
		item.AggregationWindowSeconds = item.GroupWaitSeconds
	}
	if item.Matchers == nil {
		item.Matchers = model.JSONMap{}
	}
	if item.Channels == nil {
		item.Channels = model.JSONMap{"items": []interface{}{}}
	}
	if item.ReceiverGroups == nil {
		item.ReceiverGroups = model.JSONMap{}
	}
}

func validateNotifyRule(item *model.AlertNotifyRule) error {
	if err := validateNamed(item.Name); err != nil {
		return err
	}
	policy, err := alertnotify.ParseNotifyPolicy(*item)
	if err != nil {
		return err
	}
	return alertnotify.ValidateNotifyPolicy(policy)
}

func normalizeNotifyRepeatIntervals(raw model.JSONMap) model.JSONMap {
	if raw == nil {
		return model.JSONMap{}
	}
	result := make(model.JSONMap, len(raw))
	for key, value := range raw {
		severity := strings.ToUpper(strings.TrimSpace(key))
		if severity == "" {
			continue
		}
		result[severity] = value
	}
	return result
}

func withDefaultNotifyRepeatIntervals(raw model.JSONMap) model.JSONMap {
	defaults := model.JSONMap{
		"P0": int64(3600),
		"P1": int64(7200),
		"P2": int64(14400),
	}
	if raw == nil {
		return defaults
	}
	for key, value := range normalizeNotifyRepeatIntervals(raw) {
		severity := strings.ToUpper(strings.TrimSpace(key))
		if severity == "" {
			continue
		}
		defaults[severity] = value
	}
	return defaults
}

func cloneJSONMap(src model.JSONMap) model.JSONMap {
	if src == nil {
		return model.JSONMap{}
	}
	dst := make(model.JSONMap, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func truncateForLog(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + "...(truncated)"
}

func (s *AlertManagerServiceImpl) claimActiveEventInternal(eventID uint64, operator string) error {
	if err := s.dao.ClaimActiveEvent(eventID, operator); err != nil {
		return err
	}
	if event, err := s.dao.GetActiveEvent(eventID); err == nil {
		alertengine.CacheActiveEventState(*event)
	}
	_ = s.dao.AppendTimeline(eventID, "claim", "告警已被认领", operator, model.JSONMap{"claimUser": operator})
	return nil
}

func (s *AlertManagerServiceImpl) silenceActiveEventInternal(event *model.AlertActiveEvent, endTime time.Time, reason, operator string, requestedMatchers model.JSONMap) (*model.AlertSilenceRule, error) {
	if event == nil {
		return nil, errors.New("event is required")
	}
	now := time.Now()
	activeRules, err := s.dao.ListActiveSilenceRulesForEvent(*event, now)
	if err != nil {
		return nil, err
	}
	if len(activeRules) > 0 {
		return nil, errors.New("告警已处于静默状态")
	}
	name := "事件静默-" + event.Fingerprint
	matchers := cloneJSONMap(requestedMatchers)
	if len(matchers) == 0 {
		matchers = model.JSONMap{
			"fingerprint": event.Fingerprint,
			"ruleId":      event.RuleID,
		}
	} else {
		name = "标签静默-" + event.RuleName
	}
	matchers["eventId"] = event.ID
	item := &model.AlertSilenceRule{
		Name:       name,
		Matchers:   matchers,
		StartTime:  util.HTime{Time: now},
		EndTime:    util.HTime{Time: endTime},
		RepeatType: "once",
		Reason:     reason,
		Enabled:    model.RuleStatusEnabled,
		CreatedBy:  operator,
		UpdatedBy:  operator,
	}
	if err := validateSilenceRule(item); err != nil {
		return nil, err
	}
	updatedEvents, err := s.dao.CreateEventSilence(item, event.ID, operator)
	if err != nil {
		return nil, err
	}
	alertengine.InvalidateSilenceCache()
	for _, updatedEvent := range updatedEvents {
		alertengine.CacheActiveEventState(updatedEvent)
	}
	return item, nil
}

func validateNamed(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	return nil
}

func validateSilenceRule(item *model.AlertSilenceRule) error {
	if item == nil {
		return errors.New("silence rule is required")
	}
	item.Name = strings.TrimSpace(item.Name)
	item.Reason = strings.TrimSpace(item.Reason)
	if err := validateNamed(item.Name); err != nil {
		return err
	}
	if len(item.Matchers) == 0 {
		return errors.New("matchers is required")
	}
	if rawItems, exists := item.Matchers["items"]; exists {
		items, ok := rawItems.([]interface{})
		if !ok || len(items) == 0 {
			return errors.New("at least one matcher is required")
		}
		for _, raw := range items {
			matcher, ok := raw.(map[string]interface{})
			if !ok {
				return errors.New("matcher must be an object")
			}
			label := silenceMatcherField(matcher, "name", "label")
			if label == "" {
				return errors.New("matcher label is required")
			}
			op := silenceMatcherField(matcher, "op", "operator")
			if op == "" {
				op = "="
			}
			switch op {
			case "=", "==", "!=":
			case "=~", "!~":
				if _, err := regexp.Compile("^(?:" + silenceMatcherField(matcher, "value") + ")$"); err != nil {
					return fmt.Errorf("invalid matcher regex for %s: %w", label, err)
				}
			default:
				return fmt.Errorf("unsupported matcher operator %q", op)
			}
		}
	}
	if item.StartTime.IsZero() {
		return errors.New("startTime is required")
	}
	if item.EndTime.IsZero() {
		return errors.New("endTime is required")
	}
	if !item.EndTime.Time.After(item.StartTime.Time) {
		return errors.New("endTime must be later than startTime")
	}
	if item.Reason == "" {
		return errors.New("reason is required")
	}
	item.RepeatType = strings.ToLower(strings.TrimSpace(item.RepeatType))
	if item.RepeatType == "" {
		item.RepeatType = "once"
	}
	if item.RepeatType != "once" {
		return errors.New("repeatType must be once")
	}
	return nil
}

func silenceMatcherField(matcher map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, exists := matcher[key]
		if !exists || value == nil {
			continue
		}
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return ""
}

func applyRuleDefaults(item *model.AlertRule) {
	if item.Query == "" {
		item.Query = extractRuleConfigQuery(item.RuleConfig)
	}
	if item.EvalIntervalSeconds == 0 {
		item.EvalIntervalSeconds = 60
	}
	applyRuleDurationCount(item)
	if item.CronPattern == "" {
		item.CronPattern = "@every 60s"
	}
	if item.Severity == "" {
		item.Severity = extractRuleConfigSeverity(item.RuleConfig)
		if item.Severity == "" {
			item.Severity = "warning"
		}
	}
	if item.Enabled == 0 {
		item.Enabled = model.RuleStatusEnabled
	}
	if item.TimeZone == "" {
		item.TimeZone = "Asia/Shanghai"
	}
	if item.NoDataPolicy == "" {
		item.NoDataPolicy = "ignore"
	}
	if item.ErrorPolicy == "" {
		item.ErrorPolicy = "ignore"
	}
}

func applyRuleDurationCount(item *model.AlertRule) {
	if item == nil || item.ForDurationCount <= 0 {
		return
	}
	if item.EvalIntervalSeconds <= 0 {
		item.EvalIntervalSeconds = 60
	}
	item.ForDurationSeconds = item.EvalIntervalSeconds * item.ForDurationCount
	applyRuleConfigForDuration(item.RuleConfig, item.ForDurationSeconds)
}

func applyRuleConfigForDuration(ruleConfig model.JSONMap, forDurationSeconds int64) {
	if ruleConfig == nil {
		return
	}
	for _, query := range ruleConfigQueries(ruleConfig) {
		query["forDurationSeconds"] = forDurationSeconds
	}
}

func fillRuleDurationCounts(list []model.AlertRule) {
	for i := range list {
		fillRuleDurationCount(&list[i])
	}
}

func fillRuleDurationCount(item *model.AlertRule) {
	if item == nil {
		return
	}
	item.ForDurationCount = durationCountFromSeconds(item.ForDurationSeconds, item.EvalIntervalSeconds)
}

func durationCountFromSeconds(forDurationSeconds, evalIntervalSeconds int64) int64 {
	if forDurationSeconds <= 0 {
		return 0
	}
	if evalIntervalSeconds <= 0 {
		evalIntervalSeconds = 60
	}
	return (forDurationSeconds + evalIntervalSeconds - 1) / evalIntervalSeconds
}

func applyDatasourceDefaults(item *model.AlertDatasource) {
	if item.AuthType == "" {
		item.AuthType = "none"
	}
	if item.Enabled == 0 {
		item.Enabled = model.RuleStatusEnabled
	}
}

func applyBuiltinTemplateGroupDefaults(item *model.AlertBuiltinRuleTemplateGroup) {
	item.Level = strings.TrimSpace(item.Level)
	item.Name = strings.TrimSpace(item.Name)
	item.Value = strings.TrimSpace(item.Value)
	item.ResourceType = ""
	if item.Value == "" {
		item.Value = item.Name
	}
	if item.Name == "" {
		item.Name = item.Value
	}
	if item.Enabled == 0 {
		item.Enabled = model.RuleStatusEnabled
	}
}

func sanitizeDatasource(item model.AlertDatasource) model.AlertDatasource {
	item.Password = ""
	item.Token = ""
	return item
}

func sanitizeDatasourceList(list []model.AlertDatasource) []model.AlertDatasource {
	result := make([]model.AlertDatasource, 0, len(list))
	for _, item := range list {
		result = append(result, sanitizeDatasource(item))
	}
	return result
}

func datasourceTypeOptions() []model.AlertDatasourceTypeOption {
	return []model.AlertDatasourceTypeOption{
		{
			Type:        model.AlertDatasourceBuiltin,
			Name:        "平台内置告警源",
			Description: "平台内置 VictoriaMetrics 查询源，系统默认创建，不允许手动新增",
			QueryLang:   "promql",
			AuthTypes:   []string{"none"},
			Builtin:     1,
			Enabled:     1,
		},
		{
			Type:        model.AlertDatasourcePrometheus,
			Name:        "Prometheus",
			Description: "原生 Prometheus 数据源，用于兼容已有 Prometheus 监控",
			QueryLang:   "promql",
			AuthTypes:   []string{"none", "basic", "bearer"},
			Builtin:     0,
			Enabled:     1,
		},
		{
			Type:        model.AlertDatasourceVictoriaMetrics,
			Name:        "VictoriaMetrics",
			Description: "独立 VictoriaMetrics 数据源，适合接入额外 VM 集群",
			QueryLang:   "promql",
			AuthTypes:   []string{"none", "basic", "bearer"},
			Builtin:     0,
			Enabled:     1,
		},
		{
			Type:        model.AlertDatasourceVictoriaLogs,
			Name:        "VictoriaLogs",
			Description: "VictoriaLogs 日志查询数据源，用于日志告警规则",
			QueryLang:   "logsql",
			AuthTypes:   []string{"none", "basic", "bearer"},
			Builtin:     0,
			Enabled:     1,
		},
	}
}

func (s *AlertManagerServiceImpl) resolveQueryDatasource(req model.PreviewRuleQueryRequest) (*model.AlertDatasource, error) {
	if req.DatasourceID > 0 {
		item, err := s.dao.GetDatasource(req.DatasourceID)
		if err != nil {
			return nil, fmt.Errorf("查询告警数据源失败: %w", err)
		}
		return item, nil
	}

	datasourceType := strings.TrimSpace(req.DatasourceType)
	if datasourceType == "" {
		datasourceType = model.AlertDatasourcePrometheus
	}
	list, err := s.dao.AllEnabledDatasources()
	if err != nil {
		return nil, fmt.Errorf("查询告警数据源失败: %w", err)
	}
	for i := range list {
		if list[i].DatasourceType == datasourceType {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("未找到可用的 %s 告警数据源", datasourceType)
}

func (s *AlertManagerServiceImpl) metricPromAPI(datasourceID uint, datasourceType string) (*model.AlertDatasource, v1.API, error) {
	ds, err := s.resolveQueryDatasource(model.PreviewRuleQueryRequest{
		DatasourceID:   datasourceID,
		DatasourceType: datasourceType,
	})
	if err != nil {
		return nil, nil, err
	}
	if ds.Enabled != model.RuleStatusEnabled {
		return nil, nil, fmt.Errorf("告警数据源已禁用")
	}
	if !isPromQLDatasourceType(ds.DatasourceType) {
		return nil, nil, fmt.Errorf("仅支持 Prometheus/VictoriaMetrics 查询")
	}
	if strings.TrimSpace(ds.URL) == "" {
		return nil, nil, fmt.Errorf("告警数据源 URL 为空")
	}
	promAPI, err := newAlertDatasourcePromAPI(*ds)
	if err != nil {
		return nil, nil, fmt.Errorf("创建查询客户端失败: %w", err)
	}
	return ds, promAPI, nil
}

func isPromQLDatasourceType(datasourceType string) bool {
	switch datasourceType {
	case model.AlertDatasourcePrometheus, model.AlertDatasourceVictoriaMetrics, model.AlertDatasourceBuiltin:
		return true
	default:
		return false
	}
}

func newAlertDatasourcePromAPI(ds model.AlertDatasource) (v1.API, error) {
	roundTripper := http.DefaultTransport
	switch strings.ToLower(strings.TrimSpace(ds.AuthType)) {
	case "", "none":
	case "basic":
		roundTripper = &alertDatasourceBasicAuthRoundTripper{
			username: ds.Username,
			password: ds.Password,
			next:     roundTripper,
		}
	case "bearer":
		roundTripper = &alertDatasourceBearerAuthRoundTripper{
			token: ds.Token,
			next:  roundTripper,
		}
	default:
		return nil, fmt.Errorf("不支持的认证方式: %s", ds.AuthType)
	}

	client, err := api.NewClient(api.Config{
		Address:      normalizeAlertDatasourceQueryURL(ds.URL),
		RoundTripper: roundTripper,
	})
	if err != nil {
		return nil, err
	}
	return v1.NewAPI(client), nil
}

func normalizeAlertDatasourceQueryURL(rawURL string) string {
	url := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	for _, suffix := range []string{"/api/v1/import/prometheus", "/api/v1/write", "/write"} {
		if strings.HasSuffix(url, suffix) {
			return strings.TrimRight(strings.TrimSuffix(url, suffix), "/")
		}
	}
	return url
}

type alertDatasourceBasicAuthRoundTripper struct {
	username string
	password string
	next     http.RoundTripper
}

func (rt *alertDatasourceBasicAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(rt.username, rt.password)
	return rt.next.RoundTrip(req)
}

type alertDatasourceBearerAuthRoundTripper struct {
	token string
	next  http.RoundTripper
}

func (rt *alertDatasourceBearerAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+rt.token)
	return rt.next.RoundTrip(req)
}

func normalizePreviewLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func limitPrometheusValue(value prommodel.Value, limit int) (interface{}, int, bool) {
	switch typed := value.(type) {
	case prommodel.Vector:
		total := len(typed)
		if total > limit {
			return typed[:limit], total, true
		}
		return typed, total, false
	case prommodel.Matrix:
		total := len(typed)
		if total > limit {
			return typed[:limit], total, true
		}
		return typed, total, false
	default:
		return value, 1, false
	}
}

func prometheusWarningsToStrings(warnings v1.Warnings) []string {
	if len(warnings) == 0 {
		return nil
	}
	result := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		result = append(result, string(warning))
	}
	return result
}

func prometheusValueTypeString(valueType prommodel.ValueType) string {
	switch valueType {
	case prommodel.ValScalar:
		return "scalar"
	case prommodel.ValVector:
		return "vector"
	case prommodel.ValMatrix:
		return "matrix"
	case prommodel.ValString:
		return "string"
	default:
		return "unknown"
	}
}

func extractRuleConfigPromQL(ruleConfig model.JSONMap) string {
	return extractRuleConfigQuery(ruleConfig)
}

func extractRuleConfigQuery(ruleConfig model.JSONMap) string {
	if ruleConfig == nil {
		return ""
	}
	for _, key := range []string{"prom_ql", "promQl", "promql", "log_ql", "logQl", "logql", "logs_ql", "logsQl", "logsql", "query"} {
		if value, ok := stringFromMap(ruleConfig, key); ok {
			return value
		}
	}
	query, ok := firstRuleConfigQuery(ruleConfig)
	if !ok {
		return ""
	}
	for _, key := range []string{"prom_ql", "promQl", "promql", "log_ql", "logQl", "logql", "logs_ql", "logsQl", "logsql", "query"} {
		if value, ok := stringFromAnyMap(query, key); ok {
			return value
		}
	}
	return ""
}

func extractRuleConfigSeverity(ruleConfig model.JSONMap) string {
	query, ok := firstRuleConfigQuery(ruleConfig)
	if !ok {
		return ""
	}
	if severity, ok := stringFromAnyMap(query, "severity"); ok {
		return severity
	}
	return ""
}

func firstRuleConfigQuery(ruleConfig model.JSONMap) (map[string]interface{}, bool) {
	queriesValue, ok := ruleConfig["queries"]
	if !ok || queriesValue == nil {
		return nil, false
	}
	switch queries := queriesValue.(type) {
	case []interface{}:
		if len(queries) == 0 {
			return nil, false
		}
		query, ok := queries[0].(map[string]interface{})
		return query, ok
	case []map[string]interface{}:
		if len(queries) == 0 {
			return nil, false
		}
		return queries[0], true
	}
	return nil, false
}

func stringFromMap(values model.JSONMap, key string) (string, bool) {
	value, ok := values[key]
	if !ok {
		return "", false
	}
	return stringFromValue(value)
}

func stringFromAnyMap(values map[string]interface{}, key string) (string, bool) {
	value, ok := values[key]
	if !ok {
		return "", false
	}
	return stringFromValue(value)
}

func stringFromValue(value interface{}) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	return text, text != ""
}

func applyBuiltinDefaults(item *model.AlertBuiltinRuleTemplate) {
	if item.ResourceType == "" {
		item.ResourceType = "host"
	}
	if item.Component == "" {
		item.Component = item.ResourceType
	}
	if item.Source == "" {
		item.Source = "manual"
	}
	if item.DefaultEvalIntervalSeconds == 0 {
		item.DefaultEvalIntervalSeconds = 60
	}
	if item.DefaultSeverity == "" {
		item.DefaultSeverity = "warning"
	}
	if item.Enabled == 0 {
		item.Enabled = model.RuleStatusEnabled
	}
}

func buildRuleFromBuiltinTemplate(template *model.AlertBuiltinRuleTemplate, req model.ApplyBuiltinRuleTemplateRequest) model.AlertRule {
	vars := builtinVariableValues(template.Variables, req.VariableValues)
	query := applyTemplateVariables(template.QueryTemplate, vars)
	ruleConfig := applyTemplateVariablesToJSONMap(template.RuleConfig, vars)
	labels := mergeJSONMap(template.Labels, req.Labels)
	annotations := mergeJSONMap(template.Annotations, req.Annotations)
	appendTags := mergeJSONMap(template.Tags, req.AppendTags)

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = template.Name
	}
	note := strings.TrimSpace(req.Note)
	if note == "" {
		note = template.Description
	}
	datasourceType := strings.TrimSpace(req.DatasourceType)
	if datasourceType == "" {
		datasourceType = template.DatasourceType
	}

	rule := model.AlertRule{
		Name:                    name,
		Note:                    note,
		GroupID:                 req.GroupID,
		DatasourceID:            req.DatasourceID,
		DatasourceType:          datasourceType,
		DatasourceQueries:       req.DatasourceQueries,
		Query:                   query,
		ConditionOperator:       template.DefaultConditionOperator,
		ConditionValue:          template.DefaultConditionValue,
		RuleConfig:              ruleConfig,
		AppendTags:              appendTags,
		CronPattern:             req.CronPattern,
		EvalIntervalSeconds:     firstNonZeroInt64(req.EvalIntervalSeconds, template.DefaultEvalIntervalSeconds),
		ForDurationSeconds:      firstNonZeroInt64(req.ForDurationSeconds, template.DefaultForDurationSeconds),
		ForDurationCount:        req.ForDurationCount,
		RecoverDurationSeconds:  req.RecoverDurationSeconds,
		Severity:                template.DefaultSeverity,
		EnableInBG:              req.EnableInBG,
		TimeZone:                req.TimeZone,
		Labels:                  labels,
		Annotations:             annotations,
		AssetScope:              req.AssetScope,
		BusinessID:              req.BusinessID,
		NoDataPolicy:            req.NoDataPolicy,
		ErrorPolicy:             req.ErrorPolicy,
		EffectiveTime:           req.EffectiveTime,
		NotifyRuleIDs:           req.NotifyRuleIDs,
		NotifyRecovered:         req.NotifyRecovered,
		NotifyRepeatStepMinutes: req.NotifyRepeatStepMinutes,
		NotifyMaxNumber:         req.NotifyMaxNumber,
		PipelineConfigs:         req.PipelineConfigs,
		RunbookURL:              req.RunbookURL,
		Callbacks:               req.Callbacks,
		SelfHealConfig:          req.SelfHealConfig,
	}
	if rule.RuleConfig == nil {
		rule.RuleConfig = model.JSONMap{
			"inhibit": false,
			"queries": []interface{}{
				map[string]interface{}{
					"prom_ql":  query,
					"severity": template.DefaultSeverity,
				},
			},
		}
	}
	return rule
}

type prometheusRuleFile struct {
	Groups []prometheusRuleGroup `yaml:"groups"`
	Rules  []prometheusRuleItem  `yaml:"rules"`
}

type prometheusRuleGroup struct {
	Name  string               `yaml:"name"`
	Rules []prometheusRuleItem `yaml:"rules"`
}

type prometheusRuleItem struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

type awesomePrometheusRulesFile struct {
	Groups []awesomePrometheusGroup `yaml:"groups"`
}

type awesomePrometheusGroup struct {
	Name     string                     `yaml:"name"`
	Services []awesomePrometheusService `yaml:"services"`
}

type awesomePrometheusService struct {
	Name      string                      `yaml:"name"`
	Exporters []awesomePrometheusExporter `yaml:"exporters"`
}

type awesomePrometheusExporter struct {
	Name  string                      `yaml:"name"`
	Slug  string                      `yaml:"slug"`
	Rules []awesomePrometheusRuleItem `yaml:"rules"`
}

type awesomePrometheusRuleRef struct {
	Name     string
	Exporter string
}

type awesomePrometheusRuleItem struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Query       string `yaml:"query"`
	Severity    string `yaml:"severity"`
	For         string `yaml:"for"`
	Comments    string `yaml:"comments"`
}

const awesomePrometheusRulesURL = "https://raw.githubusercontent.com/samber/awesome-prometheus-alerts/master/_data/rules.yml"

func parsePrometheusRuleContent(content string) ([]prometheusRuleItem, error) {
	var file prometheusRuleFile
	if err := yaml.Unmarshal([]byte(content), &file); err != nil {
		return nil, err
	}
	rules := make([]prometheusRuleItem, 0)
	for _, item := range file.Rules {
		if strings.TrimSpace(item.Alert) != "" {
			rules = append(rules, item)
		}
	}
	for _, group := range file.Groups {
		for _, item := range group.Rules {
			if strings.TrimSpace(item.Alert) != "" {
				rules = append(rules, item)
			}
		}
	}
	return rules, nil
}

func loadAwesomePrometheusTemplates(query model.PreviewPrometheusTemplateQuery) ([]model.PrometheusTemplateImportItem, error) {
	content, err := fetchRemoteText(awesomePrometheusRulesURL)
	if err != nil {
		return nil, err
	}
	var source awesomePrometheusRulesFile
	if err := yamlv2.Unmarshal([]byte(content), &source); err != nil {
		return nil, err
	}
	if len(source.Groups) == 0 {
		return nil, errors.New("awesome-prometheus-alerts 规则源格式异常：未读取到 groups")
	}
	result := make([]model.PrometheusTemplateImportItem, 0)
	limit := query.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	for _, group := range source.Groups {
		for _, service := range group.Services {
			for _, exporter := range service.Exporters {
				ref := awesomePrometheusRuleRef{
					Name:     service.Name,
					Exporter: firstNonEmpty(exporter.Slug, exporter.Name, "embedded-exporter"),
				}
				for _, sourceRule := range exporter.Rules {
					rule := sourceRule.toPrometheusRuleItem()
					if strings.TrimSpace(rule.Alert) == "" || strings.TrimSpace(rule.Expr) == "" {
						continue
					}
					item := buildPrometheusTemplateImportItem(group.Name, ref, rule, query)
					if !matchesPrometheusTemplateQuery(item, query) {
						continue
					}
					result = append(result, item)
					if len(result) >= limit {
						return result, nil
					}
				}
			}
		}
	}
	return result, nil
}

func (item awesomePrometheusRuleItem) toPrometheusRuleItem() prometheusRuleItem {
	labels := make(map[string]string)
	if strings.TrimSpace(item.Severity) != "" {
		labels["severity"] = strings.TrimSpace(item.Severity)
	}
	annotations := make(map[string]string)
	if strings.TrimSpace(item.Description) != "" {
		annotations["description"] = strings.TrimSpace(item.Description)
		annotations["summary"] = strings.TrimSpace(item.Description)
	}
	if strings.TrimSpace(item.Comments) != "" {
		annotations["comments"] = strings.TrimSpace(item.Comments)
	}
	return prometheusRuleItem{
		Alert:       strings.TrimSpace(item.Name),
		Expr:        strings.TrimSpace(item.Query),
		For:         strings.TrimSpace(item.For),
		Labels:      labels,
		Annotations: annotations,
	}
}

func fetchRemoteText(url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GET %s status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func resolveAwesomePrometheusRuleURL(path string) string {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	path = strings.TrimPrefix(path, "/")
	return "https://raw.githubusercontent.com/samber/awesome-prometheus-alerts/master/" + path
}

func buildPrometheusTemplateImportItem(groupName string, ref awesomePrometheusRuleRef, rule prometheusRuleItem, query model.PreviewPrometheusTemplateQuery) model.PrometheusTemplateImportItem {
	classification := classifyPrometheusTemplate(groupName, ref, rule)
	datasourceType := firstNonEmpty(query.DatasourceType, model.AlertDatasourcePrometheus)
	severity := normalizePrometheusSeverity(firstNonEmpty(mapStringValue(rule.Labels, "severity"), mapStringValue(rule.Labels, "level"), "warning"))
	forDurationSeconds, _ := parsePrometheusDurationSeconds(rule.For)
	sourceUUID := prometheusTemplateSourceUUID(groupName, ref, rule)
	annotations := stringMapToJSONMap(rule.Annotations)
	description := firstNonEmpty(mapStringValue(rule.Annotations, "description"), mapStringValue(rule.Annotations, "summary"), rule.Alert)
	labels := stringMapToJSONMap(rule.Labels)
	templateLabels := mergeJSONMap(labels, model.JSONMap{
		"resourceType": classification.ResourceType,
		"component":    classification.Component,
		"collector":    classification.Collector,
		"category":     classification.Category,
	})
	return model.PrometheusTemplateImportItem{
		Name:                       rule.Alert,
		Description:                description,
		QueryTemplate:              rule.Expr,
		DefaultSeverity:            severity,
		DefaultForDurationSeconds:  forDurationSeconds,
		DefaultEvalIntervalSeconds: 60,
		DefaultConditionOperator:   "",
		DefaultConditionValue:      0,
		ResourceType:               classification.ResourceType,
		Component:                  classification.Component,
		Collector:                  classification.Collector,
		Category:                   classification.Category,
		DatasourceType:             datasourceType,
		Source:                     "awesome-prometheus-alerts",
		SourceUUID:                 sourceUUID,
		SourceGroup:                groupName,
		SourceService:              ref.Name,
		SourceExporter:             ref.Exporter,
		Labels:                     templateLabels,
		Annotations:                annotations,
		RuleConfig:                 prometheusRuleConfig(rule.Expr, severity, forDurationSeconds),
	}
}

func buildTemplateFromPrometheusImportItem(item model.PrometheusTemplateImportItem) model.AlertBuiltinRuleTemplate {
	return model.AlertBuiltinRuleTemplate{
		ResourceType:               firstNonEmpty(item.ResourceType, "other"),
		Component:                  firstNonEmpty(item.Component, "Other"),
		Collector:                  firstNonEmpty(item.Collector, "prometheus"),
		Category:                   firstNonEmpty(item.Category, "other"),
		Name:                       item.Name,
		Description:                item.Description,
		DatasourceType:             firstNonEmpty(item.DatasourceType, model.AlertDatasourcePrometheus),
		Source:                     firstNonEmpty(item.Source, "awesome-prometheus-alerts"),
		SourceUUID:                 firstNonEmpty(item.SourceUUID, prometheusTemplateSourceUUIDFromItem(item)),
		QueryTemplate:              item.QueryTemplate,
		DefaultConditionOperator:   item.DefaultConditionOperator,
		DefaultConditionValue:      item.DefaultConditionValue,
		DefaultEvalIntervalSeconds: firstNonZeroInt64(item.DefaultEvalIntervalSeconds, 60),
		DefaultForDurationSeconds:  item.DefaultForDurationSeconds,
		DefaultSeverity:            normalizePrometheusSeverity(firstNonEmpty(item.DefaultSeverity, "warning")),
		DefaultEnabled:             model.RuleStatusDisabled,
		Labels:                     item.Labels,
		Annotations:                item.Annotations,
		RuleConfig:                 item.RuleConfig,
		Enabled:                    model.RuleStatusEnabled,
	}
}

func prometheusTemplateImportTypeOptions() []model.PrometheusTemplateImportTypeOption {
	return []model.PrometheusTemplateImportTypeOption{
		prometheusTemplateImportTypeOption("Linux / node_exporter", "linux", "host", "Linux", "node_exporter", []string{"linux", "host", "node_exporter", "node exporter"}),
		prometheusTemplateImportTypeOption("Kubernetes / kube-state-metrics", "k8s", "k8s", "Kubernetes", "kube-state-metrics", []string{"k8s", "kubernetes", "kube", "kube-state-metrics"}),
		prometheusTemplateImportTypeOption("MySQL / mysqld_exporter", "mysql", "mysql", "MySQL", "mysqld_exporter", []string{"mysql", "mysqld", "mysqld_exporter"}),
		prometheusTemplateImportTypeOption("Redis / redis_exporter", "redis", "redis", "Redis", "redis_exporter", []string{"redis", "redis_exporter"}),
		prometheusTemplateImportTypeOption("Nginx / nginx_exporter", "nginx", "nginx", "Nginx", "nginx_exporter", []string{"nginx", "nginx_exporter"}),
		prometheusTemplateImportTypeOption("PostgreSQL / postgres_exporter", "postgres", "postgresql", "PostgreSQL", "postgres_exporter", []string{"postgres", "postgresql", "postgres_exporter"}),
		prometheusTemplateImportTypeOption("MongoDB / mongodb_exporter", "mongodb", "mongodb", "MongoDB", "mongodb_exporter", []string{"mongodb", "mongo", "mongodb_exporter"}),
		prometheusTemplateImportTypeOption("Elasticsearch / elasticsearch_exporter", "elasticsearch", "elasticsearch", "Elasticsearch", "elasticsearch_exporter", []string{"elasticsearch", "elastic", "es", "elasticsearch_exporter"}),
		prometheusTemplateImportTypeOption("Kafka / kafka_exporter", "kafka", "kafka", "Kafka", "kafka_exporter", []string{"kafka", "kafka_exporter", "jmx"}),
		prometheusTemplateImportTypeOption("RabbitMQ / rabbitmq_exporter", "rabbitmq", "rabbitmq", "RabbitMQ", "rabbitmq_exporter", []string{"rabbitmq", "rabbitmq_exporter"}),
		prometheusTemplateImportTypeOption("Windows / windows_exporter", "windows", "windows", "Windows", "windows_exporter", []string{"windows", "windows_exporter", "wmi"}),
	}
}

func prometheusTemplateImportTypeOption(label, value, resourceType, component, collector string, keywords []string) model.PrometheusTemplateImportTypeOption {
	return model.PrometheusTemplateImportTypeOption{
		Label:          label,
		Value:          value,
		Keywords:       keywords,
		ResourceType:   resourceType,
		Component:      component,
		Collector:      collector,
		DatasourceType: model.AlertDatasourcePrometheus,
	}
}

type prometheusTemplateClassification struct {
	ResourceType string
	Component    string
	Collector    string
	Category     string
}

func classifyPrometheusTemplate(groupName string, ref awesomePrometheusRuleRef, rule prometheusRuleItem) prometheusTemplateClassification {
	sourceExporter := normalizePrometheusSourceValue(ref.Exporter)
	sourceService := normalizePrometheusSourceValue(ref.Name)
	text := strings.ToLower(groupName + " " + ref.Name + " " + ref.Exporter + " " + rule.Alert + " " + rule.Expr)
	switch {
	case sourceExporter == "node_exporter":
		return prometheusNodeExporterClassification(text)
	case sourceService == "kubernetes" || sourceExporter == "kubestate_exporter" || sourceExporter == "kube_state_metrics":
		return prometheusKubernetesClassification(text)
	case sourceService == "docker_containers" && strings.Contains(sourceExporter, "cadvisor"):
		return prometheusKubernetesClassification(text)
	case sourceService == "mysql" || strings.Contains(sourceExporter, "mysqld"):
		return prometheusSimpleClassification("mysql", "MySQL", "mysqld_exporter", prometheusMysqlCategory(text))
	case sourceService == "redis" || strings.Contains(sourceExporter, "redis"):
		return prometheusSimpleClassification("redis", "Redis", "redis_exporter", prometheusRedisCategory(text))
	case sourceService == "postgresql" || sourceService == "postgres" || strings.Contains(sourceExporter, "postgres"):
		return prometheusSimpleClassification("postgresql", "PostgreSQL", "postgres_exporter", prometheusPostgresCategory(text))
	case sourceService == "mongodb" || sourceService == "mongo" || strings.Contains(sourceExporter, "mongodb"):
		return prometheusSimpleClassification("mongodb", "MongoDB", "mongodb_exporter", prometheusMongoDBCategory(text))
	case sourceService == "nginx" || strings.Contains(sourceExporter, "nginx"):
		return prometheusSimpleClassification("nginx", "Nginx", "nginx_exporter", "availability")
	case sourceService == "elasticsearch" || strings.Contains(sourceExporter, "elasticsearch"):
		return prometheusSimpleClassification("elasticsearch", "Elasticsearch", "elasticsearch_exporter", "cluster")
	case sourceService == "kafka" || strings.Contains(sourceExporter, "kafka"):
		return prometheusSimpleClassification("kafka", "Kafka", "kafka_exporter", "broker")
	case sourceService == "rabbitmq" || strings.Contains(sourceExporter, "rabbitmq"):
		return prometheusSimpleClassification("rabbitmq", "RabbitMQ", "rabbitmq_exporter", "queue")
	case sourceService == "blackbox" || strings.Contains(sourceExporter, "blackbox"):
		return prometheusSimpleClassification("blackbox", "Blackbox", "blackbox_exporter", "availability")
	case sourceService == "prometheus_self_monitoring" || strings.Contains(sourceService, "prometheus") || strings.Contains(sourceExporter, "prometheus"):
		return prometheusSimpleClassification("prometheus", "Prometheus", "prometheus", "target")
	case sourceService == "windows" || strings.Contains(sourceExporter, "windows") || strings.Contains(sourceExporter, "wmi"):
		return prometheusSimpleClassification("windows", "Windows", "windows_exporter", prometheusWindowsCategory(text))
	case hasNodeExporterMetric(rule.Expr) && !hasKnownNonNodeExporterMetric(rule.Expr):
		return prometheusNodeExporterClassification(text)
	}
	return prometheusSimpleClassification("other", firstNonEmpty(ref.Name, "Other"), firstNonEmpty(ref.Exporter, "prometheus"), "other")
}

func normalizePrometheusSourceValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.Trim(value, "_")
	return value
}

func hasNodeExporterMetric(expr string) bool {
	return containsAny(strings.ToLower(expr), "node_cpu", "node_memory", "node_filesystem", "node_disk", "node_network", "node_load", "node_systemd", "node_time", "node_hwmon", "node_md", "node_vmstat", "node_edac", "node_zfs")
}

func hasKnownNonNodeExporterMetric(expr string) bool {
	return containsAny(strings.ToLower(expr), "kube_", "container_", "consul_", "jenkins_", "cilium_", "patroni_", "pgbouncer_", "mysql_", "redis_", "nginx_", "prometheus_", "alertmanager_")
}

func prometheusNodeExporterClassification(text string) prometheusTemplateClassification {
	category := "other"
	switch {
	case containsAny(text, "cpu", "node_cpu"):
		category = "cpu"
	case containsAny(text, "memory", "mem", "node_memory"):
		category = "memory"
	case containsAny(text, "filesystem", "disk", "node_disk", "node_filesystem"):
		category = "disk"
	case containsAny(text, "network", "node_network"):
		category = "network"
	case containsAny(text, "load", "node_load"):
		category = "load"
	case containsAny(text, "systemd", "service"):
		category = "service"
	case containsAny(text, "time", "clock"):
		category = "time"
	case containsAny(text, "up == 0", "down", "unavailable"):
		category = "availability"
	}
	return prometheusSimpleClassification("host", "Linux", "node_exporter", category)
}

func prometheusKubernetesClassification(text string) prometheusTemplateClassification {
	category := "other"
	switch {
	case containsAny(text, "kube_node", "node"):
		category = "node"
	case containsAny(text, "kube_pod", "pod"):
		category = "pod"
	case containsAny(text, "deployment"):
		category = "deployment"
	case containsAny(text, "statefulset"):
		category = "statefulset"
	case containsAny(text, "daemonset"):
		category = "daemonset"
	case containsAny(text, "persistentvolume", "pvc", "volume"):
		category = "storage"
	case containsAny(text, "container_cpu"):
		return prometheusSimpleClassification("k8s", "Kubernetes", "cadvisor", "cpu")
	case containsAny(text, "container_memory"):
		return prometheusSimpleClassification("k8s", "Kubernetes", "cadvisor", "memory")
	}
	return prometheusSimpleClassification("k8s", "Kubernetes", "kube-state-metrics", category)
}

func prometheusMysqlCategory(text string) string {
	switch {
	case containsAny(text, "slave", "replica", "replication"):
		return "replication"
	case containsAny(text, "connection", "threads_connected", "threads_running", "max_connections"):
		return "connection"
	case containsAny(text, "slow"):
		return "slow_query"
	case containsAny(text, "innodb"):
		return "innodb"
	}
	return "other"
}

func prometheusRedisCategory(text string) string {
	switch {
	case containsAny(text, "memory"):
		return "memory"
	case containsAny(text, "connected_clients", "connection", "rejected_connections"):
		return "connection"
	case containsAny(text, "keyspace"):
		return "keyspace"
	case containsAny(text, "command"):
		return "command"
	case containsAny(text, "up == 0", "down"):
		return "availability"
	}
	return "other"
}

func prometheusPostgresCategory(text string) string {
	switch {
	case containsAny(text, "connection", "conn"):
		return "connection"
	case containsAny(text, "replication"):
		return "replication"
	case containsAny(text, "deadlock", "lock"):
		return "lock"
	}
	return "other"
}

func prometheusMongoDBCategory(text string) string {
	switch {
	case containsAny(text, "connection", "conn"):
		return "connection"
	case containsAny(text, "replication", "replset", "replica"):
		return "replication"
	case containsAny(text, "memory", "mem"):
		return "memory"
	case containsAny(text, "disk", "storage"):
		return "storage"
	}
	return "other"
}

func prometheusWindowsCategory(text string) string {
	switch {
	case containsAny(text, "cpu", "processor"):
		return "cpu"
	case containsAny(text, "memory", "mem"):
		return "memory"
	case containsAny(text, "disk", "logical_disk", "physical_disk"):
		return "disk"
	case containsAny(text, "network", "net"):
		return "network"
	case containsAny(text, "service"):
		return "service"
	}
	return "other"
}

func prometheusSimpleClassification(resourceType, component, collector, category string) prometheusTemplateClassification {
	return prometheusTemplateClassification{
		ResourceType: resourceType,
		Component:    component,
		Collector:    collector,
		Category:     category,
	}
}

func matchesPrometheusTemplateQuery(item model.PrometheusTemplateImportItem, query model.PreviewPrometheusTemplateQuery) bool {
	if query.Component != "" && item.Component != query.Component {
		return false
	}
	if query.Collector != "" && item.Collector != query.Collector {
		return false
	}
	if query.Category != "" && item.Category != query.Category {
		return false
	}
	keywords := expandPrometheusTemplateKeywords(query.Keyword)
	if len(keywords) == 0 {
		return true
	}
	if isLinuxNodeExporterKeyword(query.Keyword) {
		return item.Component == "Linux" && item.Collector == "node_exporter"
	}
	if isKubernetesKeyword(query.Keyword) {
		return item.Component == "Kubernetes" && (item.Collector == "kube-state-metrics" || item.Collector == "cadvisor")
	}
	haystack := strings.ToLower(item.Name + " " + item.Description + " " + item.QueryTemplate + " " + item.Component + " " + item.Collector + " " + item.Category)
	for _, keyword := range keywords {
		if strings.Contains(haystack, keyword) {
			return true
		}
	}
	return false
}

func isLinuxNodeExporterKeyword(keyword string) bool {
	switch normalizePrometheusSourceValue(keyword) {
	case "linux", "host", "node_exporter":
		return true
	default:
		return false
	}
}

func isKubernetesKeyword(keyword string) bool {
	switch normalizePrometheusSourceValue(keyword) {
	case "k8s", "kubernetes", "kube", "kube_state_metrics", "cadvisor":
		return true
	default:
		return false
	}
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, strings.ToLower(value)) {
			return true
		}
	}
	return false
}

func expandPrometheusTemplateKeywords(keyword string) []string {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return nil
	}
	keywords := []string{keyword}
	synonyms := map[string][]string{
		"k8s":        {"kubernetes", "kube", "kube-state-metrics", "kube_pod", "kube_node"},
		"kubernetes": {"k8s", "kube", "kube-state-metrics", "kube_pod", "kube_node"},
		"mysql":      {"mysqld", "mysqld_exporter", "mysql_global_status", "mysql_slave_status"},
		"redis":      {"redis_exporter", "redis_memory", "redis_connected_clients"},
		"linux":      {"node_exporter", "node_", "node exporter"},
		"host":       {"node_exporter", "node_", "node exporter"},
		"node":       {"node_exporter", "node_", "node exporter"},
		"nginx":      {"nginx_exporter", "nginx_vts", "nginx_ingress"},
		"postgres":   {"postgresql", "postgres_exporter", "pg_"},
		"postgresql": {"postgres", "postgres_exporter", "pg_"},
		"mongo":      {"mongodb", "mongodb_exporter"},
		"mongodb":    {"mongo", "mongodb_exporter"},
		"es":         {"elasticsearch", "elasticsearch_exporter"},
		"elastic":    {"elasticsearch", "elasticsearch_exporter"},
		"kafka":      {"kafka_exporter", "jmx"},
		"rabbitmq":   {"rabbitmq_exporter"},
		"blackbox":   {"blackbox_exporter", "probe_"},
		"windows":    {"windows_exporter", "wmi_"},
	}
	if extra, ok := synonyms[keyword]; ok {
		keywords = append(keywords, extra...)
	}
	return uniqueStrings(keywords)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func normalizePrometheusSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical", "p0", "p1", "serious", "fatal", "error":
		return "critical"
	case "warning", "warn", "p2":
		return "warning"
	case "info", "notice", "p3":
		return "info"
	default:
		return strings.TrimSpace(value)
	}
}

func prometheusTemplateSourceUUID(groupName string, ref awesomePrometheusRuleRef, rule prometheusRuleItem) string {
	return "awesome-prometheus-alerts:" + sha1Hex(groupName+"|"+ref.Name+"|"+ref.Exporter+"|"+rule.Alert+"|"+rule.Expr)
}

func prometheusTemplateSourceUUIDFromItem(item model.PrometheusTemplateImportItem) string {
	return "awesome-prometheus-alerts:" + sha1Hex(item.SourceGroup+"|"+item.SourceService+"|"+item.SourceExporter+"|"+item.Name+"|"+item.QueryTemplate)
}

func sha1Hex(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func buildRuleFromPrometheusRule(item prometheusRuleItem, req model.ImportPrometheusRulesRequest) (model.AlertRule, string) {
	name := strings.TrimSpace(item.Alert)
	if name == "" {
		return model.AlertRule{}, "alert is required"
	}
	query := strings.TrimSpace(item.Expr)
	if query == "" {
		return model.AlertRule{}, "expr is required"
	}

	labels := stringMapToJSONMap(item.Labels)
	annotations := stringMapToJSONMap(item.Annotations)
	labels = mergeJSONMap(labels, req.Labels)
	severity := firstNonEmpty(mapStringValue(item.Labels, "severity"), mapStringValue(item.Labels, "level"), "warning")
	forDurationSeconds, err := parsePrometheusDurationSeconds(item.For)
	if err != nil {
		return model.AlertRule{}, "invalid for duration: " + err.Error()
	}

	rule := model.AlertRule{
		Name:                    name,
		Note:                    mapStringValue(item.Annotations, "description"),
		GroupID:                 req.GroupID,
		DatasourceID:            req.DatasourceID,
		DatasourceType:          req.DatasourceType,
		DatasourceQueries:       req.DatasourceQueries,
		Query:                   query,
		RuleConfig:              prometheusRuleConfig(query, severity, forDurationSeconds),
		AppendTags:              req.AppendTags,
		CronPattern:             req.CronPattern,
		EvalIntervalSeconds:     req.EvalIntervalSeconds,
		ForDurationSeconds:      forDurationSeconds,
		Severity:                severity,
		Labels:                  labels,
		Annotations:             annotations,
		AssetScope:              req.AssetScope,
		BusinessID:              req.BusinessID,
		EffectiveTime:           req.EffectiveTime,
		NotifyRuleIDs:           req.NotifyRuleIDs,
		NotifyRecovered:         req.NotifyRecovered,
		NotifyRepeatStepMinutes: req.NotifyRepeatStepMinutes,
		NotifyMaxNumber:         req.NotifyMaxNumber,
		NoDataPolicy:            req.NoDataPolicy,
		ErrorPolicy:             req.ErrorPolicy,
	}
	return rule, ""
}

func parsePrometheusDurationSeconds(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	if duration, err := time.ParseDuration(value); err == nil {
		return int64(duration.Seconds()), nil
	}
	duration, err := prommodel.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	return int64(time.Duration(duration).Seconds()), nil
}

func prometheusRuleConfig(query, severity string, forDurationSeconds int64) model.JSONMap {
	return model.JSONMap{
		"inhibit": false,
		"queries": []interface{}{
			map[string]interface{}{
				"prom_ql":            query,
				"severity":           severity,
				"forDurationSeconds": forDurationSeconds,
			},
		},
	}
}

func stringMapToJSONMap(values map[string]string) model.JSONMap {
	if len(values) == 0 {
		return nil
	}
	result := make(model.JSONMap, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func mapStringValue(values map[string]string, key string) string {
	if values == nil {
		return ""
	}
	return strings.TrimSpace(values[key])
}

func currentOperator(c *gin.Context) string {
	if name, err := jwt.GetAdminName(c); err == nil && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	if username, exists := c.Get("username"); exists {
		if name, ok := username.(string); ok && strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name)
		}
	}
	return "system"
}

func builtinVariableValues(defaults, overrides model.JSONMap) map[string]string {
	values := make(map[string]string)
	for key, raw := range defaults {
		if config, ok := raw.(map[string]interface{}); ok {
			if value, exists := config["default"]; exists {
				values[key] = fmt.Sprint(value)
			}
		}
	}
	for key, value := range overrides {
		values[key] = fmt.Sprint(value)
	}
	return values
}

func applyTemplateVariables(text string, vars map[string]string) string {
	for key, value := range vars {
		text = strings.ReplaceAll(text, "{{"+key+"}}", value)
		text = strings.ReplaceAll(text, "{{ "+key+" }}", value)
	}
	return text
}

func applyTemplateVariablesToJSONMap(values model.JSONMap, vars map[string]string) model.JSONMap {
	if values == nil {
		return nil
	}
	result := make(model.JSONMap, len(values))
	for key, value := range values {
		result[key] = applyTemplateVariablesToValue(value, vars)
	}
	return result
}

func applyTemplateVariablesToValue(value interface{}, vars map[string]string) interface{} {
	switch typed := value.(type) {
	case string:
		return applyTemplateVariables(typed, vars)
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			result[key] = applyTemplateVariablesToValue(item, vars)
		}
		return result
	case []interface{}:
		result := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			result = append(result, applyTemplateVariablesToValue(item, vars))
		}
		return result
	default:
		return value
	}
}

func mergeJSONMap(base, override model.JSONMap) model.JSONMap {
	if base == nil && override == nil {
		return nil
	}
	merged := make(model.JSONMap)
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func (s *AlertManagerServiceImpl) countTemplatesByTemplateGroup(group *model.AlertBuiltinRuleTemplateGroup) (int64, error) {
	component, collector, category, err := s.templateGroupPathByGroup(group)
	if err != nil {
		return 0, err
	}
	return s.dao.CountBuiltinRuleTemplatesByPath(component, collector, category)
}

func (s *AlertManagerServiceImpl) countPlatformTemplatesByTemplateGroup(group *model.AlertBuiltinRuleTemplateGroup) (int64, error) {
	component, collector, category, err := s.templateGroupPathByGroup(group)
	if err != nil {
		return 0, err
	}
	return s.dao.CountPlatformBuiltinRuleTemplatesByPath(component, collector, category)
}

func (s *AlertManagerServiceImpl) templateGroupPathByGroup(group *model.AlertBuiltinRuleTemplateGroup) (string, string, string, error) {
	component, collector, category := "", "", ""
	switch group.Level {
	case model.TemplateGroupLevelComponent:
		component = group.Value
	case model.TemplateGroupLevelCollector:
		parent, err := s.dao.GetBuiltinRuleTemplateGroup(group.ParentID)
		if err != nil {
			return "", "", "", err
		}
		component = parent.Value
		collector = group.Value
	case model.TemplateGroupLevelCategory:
		collectorGroup, err := s.dao.GetBuiltinRuleTemplateGroup(group.ParentID)
		if err != nil {
			return "", "", "", err
		}
		componentGroup, err := s.dao.GetBuiltinRuleTemplateGroup(collectorGroup.ParentID)
		if err != nil {
			return "", "", "", err
		}
		component = componentGroup.Value
		collector = collectorGroup.Value
		category = group.Value
	default:
		return "", "", "", errors.New("invalid template group level")
	}
	return component, collector, category, nil
}

func (s *AlertManagerServiceImpl) descendantBuiltinTemplateGroupIDs(id uint) ([]uint, error) {
	groups, err := s.dao.AllBuiltinRuleTemplateGroupsForDelete()
	if err != nil {
		return nil, err
	}
	childrenMap := make(map[uint][]uint)
	for _, group := range groups {
		childrenMap[group.ParentID] = append(childrenMap[group.ParentID], group.ID)
	}
	ids := make([]uint, 0)
	var walk func(uint)
	walk = func(groupID uint) {
		ids = append(ids, groupID)
		for _, childID := range childrenMap[groupID] {
			walk(childID)
		}
	}
	walk(id)
	return ids, nil
}

func buildTemplateGroupTree(rows []model.AlertBuiltinRuleTemplate, manualGroups []model.AlertBuiltinRuleTemplateGroup) []model.AlertBuiltinRuleTemplateGroup {
	nodeMap := make(map[string]*model.AlertBuiltinRuleTemplateGroup)
	manualMap := make(map[uint]model.AlertBuiltinRuleTemplateGroup, len(manualGroups))
	for _, group := range manualGroups {
		manualMap[group.ID] = group
	}

	for _, row := range rows {
		component := strings.TrimSpace(row.Component)
		collector := strings.TrimSpace(row.Collector)
		category := strings.TrimSpace(row.Category)
		if component == "" || collector == "" || category == "" {
			continue
		}
		componentNode := ensureTemplateGroupNode(nodeMap, model.TemplateGroupLevelComponent, component, "", "", "")
		collectorNode := ensureTemplateGroupNode(nodeMap, model.TemplateGroupLevelCollector, component, collector, "", "")
		categoryNode := ensureTemplateGroupNode(nodeMap, model.TemplateGroupLevelCategory, component, collector, category, "")
		componentNode.TemplateCount++
		collectorNode.TemplateCount++
		categoryNode.TemplateCount++
	}

	for _, group := range manualGroups {
		component, collector, category, ok := manualGroupPath(group, manualMap)
		if !ok {
			continue
		}
		node := ensureTemplateGroupNode(nodeMap, group.Level, component, collector, category, "")
		node.ID = group.ID
		node.ParentID = group.ParentID
		node.Name = firstNonEmpty(group.Name, node.Name)
		node.Value = firstNonEmpty(group.Value, node.Value)
		node.Description = group.Description
		node.Sort = group.Sort
		node.Builtin = group.Builtin
		node.Enabled = group.Enabled
		node.CreatedBy = group.CreatedBy
		node.UpdatedBy = group.UpdatedBy
		node.CreatedAt = group.CreatedAt
		node.UpdatedAt = group.UpdatedAt
	}

	roots := make([]model.AlertBuiltinRuleTemplateGroup, 0)
	for _, node := range nodeMap {
		if node.Level != model.TemplateGroupLevelComponent {
			continue
		}
		node.Children = templateGroupChildren(nodeMap, node.Value, "", "")
		roots = append(roots, *node)
	}
	sortTemplateGroupNodes(roots)
	return roots
}

func buildTemplateGroupOptions(rows []model.AlertBuiltinRuleTemplate, manualGroups []model.AlertBuiltinRuleTemplateGroup, query model.AlertBuiltinRuleTemplateGroupOptionsQuery) []model.AlertBuiltinRuleTemplateGroupOption {
	level := query.Level
	if level == "" {
		switch {
		case query.Component == "":
			level = model.TemplateGroupLevelComponent
		case query.Collector == "":
			level = model.TemplateGroupLevelCollector
		default:
			level = model.TemplateGroupLevelCategory
		}
	}

	tree := buildTemplateGroupTree(rows, manualGroups)
	options := make([]model.AlertBuiltinRuleTemplateGroupOption, 0)
	seen := make(map[string]bool)

	for _, component := range tree {
		if level == model.TemplateGroupLevelComponent {
			appendTemplateGroupOption(&options, seen, component)
			continue
		}
		if query.Component != "" && component.Value != query.Component {
			continue
		}
		for _, collector := range component.Children {
			if level == model.TemplateGroupLevelCollector {
				appendTemplateGroupOption(&options, seen, collector)
				continue
			}
			if query.Collector != "" && collector.Value != query.Collector {
				continue
			}
			for _, category := range collector.Children {
				appendTemplateGroupOption(&options, seen, category)
			}
		}
	}

	sort.SliceStable(options, func(i, j int) bool {
		return options[i].Label < options[j].Label
	})
	return options
}

func ensureTemplateGroupNode(nodeMap map[string]*model.AlertBuiltinRuleTemplateGroup, level, component, collector, category, resourceType string) *model.AlertBuiltinRuleTemplateGroup {
	key := templateGroupKey(level, component, collector, category)
	if node, ok := nodeMap[key]; ok {
		if node.ResourceType == "" {
			node.ResourceType = resourceType
		}
		return node
	}
	value := component
	name := component
	parentKey := ""
	switch level {
	case model.TemplateGroupLevelCollector:
		value = collector
		name = collector
		parentKey = templateGroupKey(model.TemplateGroupLevelComponent, component, "", "")
	case model.TemplateGroupLevelCategory:
		value = category
		name = category
		parentKey = templateGroupKey(model.TemplateGroupLevelCollector, component, collector, "")
	}
	node := &model.AlertBuiltinRuleTemplateGroup{
		Level:        level,
		Name:         name,
		Value:        value,
		ResourceType: resourceType,
		Enabled:      model.RuleStatusEnabled,
		Key:          key,
	}
	nodeMap[key] = node
	if parentKey != "" {
		ensureTemplateGroupParentNode(nodeMap, parentKey, component, collector)
	}
	return node
}

func ensureTemplateGroupParentNode(nodeMap map[string]*model.AlertBuiltinRuleTemplateGroup, parentKey, component, collector string) {
	if _, ok := nodeMap[parentKey]; ok {
		return
	}
	if strings.HasPrefix(parentKey, model.TemplateGroupLevelComponent+":") {
		ensureTemplateGroupNode(nodeMap, model.TemplateGroupLevelComponent, component, "", "", "")
		return
	}
	ensureTemplateGroupNode(nodeMap, model.TemplateGroupLevelCollector, component, collector, "", "")
}

func templateGroupChildren(nodeMap map[string]*model.AlertBuiltinRuleTemplateGroup, component, collector, category string) []model.AlertBuiltinRuleTemplateGroup {
	children := make([]model.AlertBuiltinRuleTemplateGroup, 0)
	expectedLevel := model.TemplateGroupLevelCollector
	if collector != "" {
		expectedLevel = model.TemplateGroupLevelCategory
	}
	for _, child := range nodeMap {
		if child.Level != expectedLevel {
			continue
		}
		switch expectedLevel {
		case model.TemplateGroupLevelCollector:
			if strings.HasPrefix(child.Key, templateGroupKey(model.TemplateGroupLevelCollector, component, "", "")) {
				child.Children = templateGroupChildren(nodeMap, component, child.Value, "")
				children = append(children, *child)
			}
		case model.TemplateGroupLevelCategory:
			if strings.HasPrefix(child.Key, templateGroupKey(model.TemplateGroupLevelCategory, component, collector, "")) {
				children = append(children, *child)
			}
		}
	}
	sortTemplateGroupNodes(children)
	return children
}

func sortTemplateGroupNodes(nodes []model.AlertBuiltinRuleTemplateGroup) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Sort != nodes[j].Sort {
			return nodes[i].Sort > nodes[j].Sort
		}
		return nodes[i].Name < nodes[j].Name
	})
	for i := range nodes {
		sortTemplateGroupNodes(nodes[i].Children)
	}
}

func templateGroupKey(level, component, collector, category string) string {
	switch level {
	case model.TemplateGroupLevelCollector:
		return level + ":" + component + "/" + collector
	case model.TemplateGroupLevelCategory:
		return level + ":" + component + "/" + collector + "/" + category
	default:
		return model.TemplateGroupLevelComponent + ":" + component
	}
}

func manualGroupPath(group model.AlertBuiltinRuleTemplateGroup, groupMap map[uint]model.AlertBuiltinRuleTemplateGroup) (string, string, string, bool) {
	switch group.Level {
	case model.TemplateGroupLevelComponent:
		return group.Value, "", "", group.Value != ""
	case model.TemplateGroupLevelCollector:
		componentGroup, ok := groupMap[group.ParentID]
		if !ok || componentGroup.Value == "" || group.Value == "" {
			return "", "", "", false
		}
		return componentGroup.Value, group.Value, "", true
	case model.TemplateGroupLevelCategory:
		collectorGroup, ok := groupMap[group.ParentID]
		if !ok || collectorGroup.Value == "" || group.Value == "" {
			return "", "", "", false
		}
		componentGroup, ok := groupMap[collectorGroup.ParentID]
		if !ok || componentGroup.Value == "" {
			return "", "", "", false
		}
		return componentGroup.Value, collectorGroup.Value, group.Value, true
	}
	return "", "", "", false
}

func appendTemplateGroupOption(options *[]model.AlertBuiltinRuleTemplateGroupOption, seen map[string]bool, group model.AlertBuiltinRuleTemplateGroup) {
	if group.Value == "" || seen[group.Value] {
		return
	}
	seen[group.Value] = true
	*options = append(*options, model.AlertBuiltinRuleTemplateGroupOption{
		Label: group.Name,
		Value: group.Value,
		Count: group.TemplateCount,
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func buildRuleGroupTree(groups []model.AlertRuleGroup) []model.AlertRuleGroup {
	groupMap := make(map[uint]model.AlertRuleGroup, len(groups))
	for i := range groups {
		groups[i].Children = nil
		groupMap[groups[i].ID] = groups[i]
	}

	childrenMap := make(map[uint][]model.AlertRuleGroup)
	for _, group := range groups {
		if group.ParentID == 0 {
			continue
		}
		if _, ok := groupMap[group.ParentID]; ok {
			childrenMap[group.ParentID] = append(childrenMap[group.ParentID], group)
		}
	}

	tree := make([]model.AlertRuleGroup, 0)
	for _, group := range groups {
		if group.ParentID == 0 {
			tree = append(tree, buildRuleGroupNode(group, childrenMap))
		}
	}
	return tree
}

func descendantRuleGroupIDs(groups []model.AlertRuleGroup, rootID uint) []uint {
	if rootID == 0 {
		return nil
	}
	exists := false
	childrenMap := make(map[uint][]uint)
	for _, group := range groups {
		if group.ID == rootID {
			exists = true
		}
		childrenMap[group.ParentID] = append(childrenMap[group.ParentID], group.ID)
	}
	if !exists {
		return nil
	}
	ids := make([]uint, 0)
	var walk func(uint)
	walk = func(id uint) {
		ids = append(ids, id)
		for _, childID := range childrenMap[id] {
			walk(childID)
		}
	}
	walk(rootID)
	return ids
}

func buildRuleGroupNode(group model.AlertRuleGroup, childrenMap map[uint][]model.AlertRuleGroup) model.AlertRuleGroup {
	children := childrenMap[group.ID]
	group.Children = make([]model.AlertRuleGroup, 0, len(children))
	for _, child := range children {
		group.Children = append(group.Children, buildRuleGroupNode(child, childrenMap))
	}
	return group
}
