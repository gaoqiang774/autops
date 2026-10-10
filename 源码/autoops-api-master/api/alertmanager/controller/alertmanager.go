package controller

import (
	"dodevops-api/api/alertmanager/model"
	"dodevops-api/api/alertmanager/service"
	"dodevops-api/common/result"
	"dodevops-api/pkg/jwt"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ApplySSLCertificate submits an asynchronous ACME DNS-01 certificate order.
// @Tags AlertManager SSL证书
// @Summary 申请SSL证书
// @Accept json
// @Produce json
// @Param data body model.ApplySSLCertificateInput true "证书申请信息"
// @Success 200 {object} result.Result
// @Router /api/v1/alertmanager/ssl-certificates/apply [post]
// @Security ApiKeyAuth
func ApplySSLCertificate(c *gin.Context) {
	var input model.ApplySSLCertificateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	item, err := service.GetSSLCertificateService().Apply(input, currentDomainOperator(c))
	if err != nil {
		result.Failed(c, 400, "申请SSL证书失败: "+err.Error())
		return
	}
	result.Success(c, gin.H{
		"message":         "证书申请已提交，正在后台处理",
		"certificateId":   item.ID,
		"domain":          item.Domain,
		"status":          item.Status,
		"certificateInfo": item,
	})
}

// ListSSLCertificates returns certificate order metadata without certificate private keys.
// @Tags AlertManager SSL证书
// @Summary 查询SSL证书申请记录
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "域名"
// @Param status query int false "申请状态"
// @Success 200 {object} result.Result
// @Router /api/v1/alertmanager/ssl-certificates [get]
// @Security ApiKeyAuth
func ListSSLCertificates(c *gin.Context) {
	var query model.SSLCertificateQuery
	_ = c.ShouldBindQuery(&query)
	list, total, err := service.GetSSLCertificateService().List(query)
	if err != nil {
		result.Failed(c, 500, "查询SSL证书申请记录失败: "+err.Error())
		return
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	result.Success(c, gin.H{"list": list, "total": total, "page": page, "pageSize": pageSize})
}

// GetSSLCertificate returns certificate details without the private key.
// @Tags AlertManager SSL证书
// @Summary 获取SSL证书详情
// @Produce json
// @Param id path int true "证书ID"
// @Success 200 {object} result.Result
// @Router /api/v1/alertmanager/ssl-certificates/{id} [get]
// @Security ApiKeyAuth
func GetSSLCertificate(c *gin.Context) {
	item, err := service.GetSSLCertificateService().Get(parseID(c))
	if err != nil {
		if errors.Is(err, service.ErrSSLCertificateNotFound) {
			result.Failed(c, 404, err.Error())
			return
		}
		result.Failed(c, 500, "查询SSL证书详情失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

// DownloadSSLCertificate downloads the certificate chain and private key as a ZIP archive.
// @Tags AlertManager SSL证书
// @Summary 下载SSL证书
// @Produce application/zip
// @Param id path int true "证书ID"
// @Success 200 {file} binary
// @Router /api/v1/alertmanager/ssl-certificates/{id}/download [get]
// @Security ApiKeyAuth
func DownloadSSLCertificate(c *gin.Context) {
	bundle, err := service.GetSSLCertificateService().Download(parseID(c))
	if err != nil {
		code := 500
		if errors.Is(err, service.ErrSSLCertificateNotFound) {
			code = 404
		} else if errors.Is(err, service.ErrSSLCertificateNotIssued) {
			code = 400
		}
		result.Failed(c, code, "下载SSL证书失败: "+err.Error())
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, bundle.FileName))
	c.Header("Content-Transfer-Encoding", "binary")
	c.Data(http.StatusOK, "application/zip", bundle.Content)
}

// DeleteSSLCertificate deletes a completed or failed certificate record.
// @Tags AlertManager SSL证书
// @Summary 删除SSL证书记录
// @Produce json
// @Param id path int true "证书ID"
// @Success 200 {object} result.Result
// @Router /api/v1/alertmanager/ssl-certificates/{id} [delete]
// @Security ApiKeyAuth
func DeleteSSLCertificate(c *gin.Context) {
	err := service.GetSSLCertificateService().Delete(parseID(c))
	if err != nil {
		code := 500
		if errors.Is(err, service.ErrSSLCertificateNotFound) {
			code = 404
		} else if errors.Is(err, service.ErrSSLCertificateApplying) || errors.Is(err, service.ErrSSLCertificateDeploying) {
			code = 400
		}
		result.Failed(c, code, "删除SSL证书失败: "+err.Error())
		return
	}
	result.Success(c, true)
}

// DeploySSLCertificate starts an asynchronous deployment to a CMDB host.
// @Tags AlertManager SSL证书
// @Summary 更新主机SSL证书
// @Accept json
// @Produce json
// @Param id path int true "证书ID"
// @Param data body model.DeploySSLCertificateInput true "部署信息"
// @Success 200 {object} result.Result
// @Router /api/v1/alertmanager/ssl-certificates/{id}/deploy [post]
// @Security ApiKeyAuth
func DeploySSLCertificate(c *gin.Context) {
	var input model.DeploySSLCertificateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	item, err := service.GetSSLCertificateService().Deploy(c, parseID(c), input, currentDomainOperator(c))
	if err != nil {
		code := 400
		if errors.Is(err, service.ErrSSLCertificateNotFound) {
			code = 404
		}
		result.Failed(c, code, "更新SSL证书失败: "+err.Error())
		return
	}
	result.Success(c, gin.H{
		"message":     "证书更新任务已创建，正在后台执行",
		"deployLogId": item.ID,
		"deployLog":   item,
	})
}

// ListSSLCertificateDeployLogs returns deployment history for one certificate.
// @Tags AlertManager SSL证书
// @Summary 查询SSL证书更新日志
// @Produce json
// @Param id path int true "证书ID"
// @Success 200 {object} result.Result
// @Router /api/v1/alertmanager/ssl-certificates/{id}/deploy-logs [get]
// @Security ApiKeyAuth
func ListSSLCertificateDeployLogs(c *gin.Context) {
	var query model.SSLCertificateDeployLogQuery
	_ = c.ShouldBindQuery(&query)
	list, total, err := service.GetSSLCertificateService().ListDeployLogs(parseID(c), query)
	if err != nil {
		code := 500
		if errors.Is(err, service.ErrSSLCertificateNotFound) {
			code = 404
		}
		result.Failed(c, code, err.Error())
		return
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	result.Success(c, gin.H{"list": list, "total": total, "page": page, "pageSize": pageSize})
}

// GetSSLCertificateDeployLog returns one deployment log for polling.
// @Tags AlertManager SSL证书
// @Summary 获取SSL证书更新日志详情
// @Produce json
// @Param id path int true "证书ID"
// @Param logId path int true "部署日志ID"
// @Success 200 {object} result.Result
// @Router /api/v1/alertmanager/ssl-certificates/{id}/deploy-logs/{logId} [get]
// @Security ApiKeyAuth
func GetSSLCertificateDeployLog(c *gin.Context) {
	item, err := service.GetSSLCertificateService().GetDeployLog(parseID(c), parseUintParam(c, "logId"))
	if err != nil {
		code := 500
		if errors.Is(err, service.ErrSSLCertificateNotFound) {
			code = 404
		}
		result.Failed(c, code, err.Error())
		return
	}
	result.Success(c, item)
}

func CreateDomain(c *gin.Context) {
	var input model.AlertDomainInput
	if err := c.ShouldBindJSON(&input); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	item, err := service.GetDomainMonitorService().Create(input, currentDomainOperator(c))
	if err != nil {
		result.Failed(c, 400, "创建域名监控失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func UpdateDomain(c *gin.Context) {
	var input model.AlertDomainInput
	if err := c.ShouldBindJSON(&input); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	item, err := service.GetDomainMonitorService().Update(parseID(c), input, currentDomainOperator(c))
	if err != nil {
		result.Failed(c, 400, "更新域名监控失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func DeleteDomain(c *gin.Context) {
	if err := service.GetDomainMonitorService().Delete(c.Request.Context(), parseID(c)); err != nil {
		result.Failed(c, 500, "删除域名监控失败: "+err.Error())
		return
	}
	result.Success(c, true)
}

func GetDomain(c *gin.Context) {
	item, err := service.GetDomainMonitorService().Get(parseID(c))
	if err != nil {
		result.Failed(c, 404, "域名监控不存在")
		return
	}
	result.Success(c, item)
}

func ListDomains(c *gin.Context) {
	var query model.AlertDomainQuery
	_ = c.ShouldBindQuery(&query)
	list, total, summary, err := service.GetDomainMonitorService().List(query)
	if err != nil {
		result.Failed(c, 500, "查询域名监控失败: "+err.Error())
		return
	}
	result.Success(c, gin.H{"list": list, "total": total, "summary": summary})
}

func CheckDomain(c *gin.Context) {
	item, err := service.GetDomainMonitorService().Check(c.Request.Context(), parseID(c))
	if err != nil {
		result.Failed(c, 500, "检查域名失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func CheckDomains(c *gin.Context) {
	var req model.AlertDomainCheckRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			result.Failed(c, 400, "invalid request body")
			return
		}
	}
	list, err := service.GetDomainMonitorService().CheckBatch(c.Request.Context(), req.IDs)
	if err != nil {
		result.Failed(c, 500, "批量检查域名失败: "+err.Error())
		return
	}
	result.Success(c, gin.H{"list": list, "total": len(list)})
}

func ChangeDomainStatus(c *gin.Context) {
	enabled, _ := strconv.Atoi(c.DefaultQuery("enabled", "1"))
	item, err := service.GetDomainMonitorService().ChangeEnabled(c.Request.Context(), parseID(c), enabled, currentDomainOperator(c))
	if err != nil {
		result.Failed(c, 500, "更新域名状态失败: "+err.Error())
		return
	}
	result.Success(c, item)
}

func currentDomainOperator(c *gin.Context) string {
	if name, err := jwt.GetAdminName(c); err == nil && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return "system"
}

func CreateRuleGroup(c *gin.Context) {
	var item model.AlertRuleGroup
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().CreateRuleGroup(c, item)
}

func UpdateRuleGroup(c *gin.Context) {
	var item model.AlertRuleGroup
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	if item.ID == 0 {
		item.ID = parseUintParam(c, "id")
	}
	service.GetAlertManagerService().UpdateRuleGroup(c, item)
}

func DeleteRuleGroup(c *gin.Context) {
	service.GetAlertManagerService().DeleteRuleGroup(c, parseID(c))
}

func GetRuleGroup(c *gin.Context) {
	service.GetAlertManagerService().GetRuleGroup(c, parseID(c))
}

func ListRuleGroups(c *gin.Context) {
	var query model.PageQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListRuleGroups(c, query)
}

func AllRuleGroups(c *gin.Context) {
	service.GetAlertManagerService().AllRuleGroups(c)
}

func CreateRule(c *gin.Context) {
	var item model.AlertRule
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().CreateRule(c, item)
}

func UpdateRule(c *gin.Context) {
	var item model.AlertRule
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	if item.ID == 0 {
		item.ID = parseUintParam(c, "id")
	}
	service.GetAlertManagerService().UpdateRule(c, item)
}

func DeleteRule(c *gin.Context) {
	service.GetAlertManagerService().DeleteRule(c, parseID(c))
}

func GetRule(c *gin.Context) {
	service.GetAlertManagerService().GetRule(c, parseID(c))
}

func ListRules(c *gin.Context) {
	var query model.AlertRuleQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListRules(c, query)
}

func ChangeRuleStatus(c *gin.Context) {
	enabled, _ := strconv.Atoi(c.DefaultQuery("enabled", "1"))
	service.GetAlertManagerService().ChangeRuleStatus(c, parseID(c), enabled)
}

func BatchUpdateRuleDatasource(c *gin.Context) {
	var req model.BatchUpdateRuleDatasourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().BatchUpdateRuleDatasource(c, req)
}

func BatchUpdateRuleNotifyRules(c *gin.Context) {
	var req model.BatchUpdateRuleNotifyRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().BatchUpdateRuleNotifyRules(c, req)
}

func PreviewRuleQuery(c *gin.Context) {
	var req model.PreviewRuleQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().PreviewRuleQuery(c, req)
}

func QueryMetricsRange(c *gin.Context) {
	var req model.AlertMetricsQueryRangeRequest
	_ = c.ShouldBindQuery(&req)
	req.DatasourceID = parseDatasourceIDQuery(c)
	req.DatasourceType = firstQuery(c, "datasourceType", "datasource_type")
	req.Query = firstQuery(c, "promql", "query")
	service.GetAlertManagerService().QueryMetricsRange(c, req)
}

func QueryMetrics(c *gin.Context) {
	var req model.AlertMetricsQueryRequest
	_ = c.ShouldBindQuery(&req)
	req.DatasourceID = parseDatasourceIDQuery(c)
	req.DatasourceType = firstQuery(c, "datasourceType", "datasource_type")
	req.Query = firstQuery(c, "promql", "query")
	service.GetAlertManagerService().QueryMetrics(c, req)
}

func GetMetricTargets(c *gin.Context) {
	var req model.AlertMetricsQueryRequest
	_ = c.ShouldBindQuery(&req)
	req.DatasourceID = parseDatasourceIDQuery(c)
	req.DatasourceType = firstQuery(c, "datasourceType", "datasource_type")
	service.GetAlertManagerService().GetMetricTargets(c, req)
}

func ImportPrometheusRules(c *gin.Context) {
	var req model.ImportPrometheusRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().ImportPrometheusRules(c, req)
}

func CreateDatasource(c *gin.Context) {
	var item model.AlertDatasource
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().CreateDatasource(c, item)
}

func UpdateDatasource(c *gin.Context) {
	var item model.AlertDatasource
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	if item.ID == 0 {
		item.ID = parseUintParam(c, "id")
	}
	service.GetAlertManagerService().UpdateDatasource(c, item)
}

func DeleteDatasource(c *gin.Context) {
	service.GetAlertManagerService().DeleteDatasource(c, parseID(c))
}

func GetDatasource(c *gin.Context) {
	service.GetAlertManagerService().GetDatasource(c, parseID(c))
}

func ListDatasources(c *gin.Context) {
	var query model.AlertDatasourceQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListDatasources(c, query)
}

func ListDatasourceTypes(c *gin.Context) {
	service.GetAlertManagerService().ListDatasourceTypes(c)
}

func ListDatasourceOptions(c *gin.Context) {
	service.GetAlertManagerService().ListDatasourceOptions(c)
}

func ChangeDatasourceStatus(c *gin.Context) {
	enabled, _ := strconv.Atoi(c.DefaultQuery("enabled", "1"))
	service.GetAlertManagerService().ChangeDatasourceStatus(c, parseID(c), enabled)
}

func CreateBuiltinRuleTemplate(c *gin.Context) {
	var item model.AlertBuiltinRuleTemplate
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().CreateBuiltinRuleTemplate(c, item)
}

func UpdateBuiltinRuleTemplate(c *gin.Context) {
	var item model.AlertBuiltinRuleTemplate
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	if item.ID == 0 {
		item.ID = parseUintParam(c, "id")
	}
	service.GetAlertManagerService().UpdateBuiltinRuleTemplate(c, item)
}

func DeleteBuiltinRuleTemplate(c *gin.Context) {
	service.GetAlertManagerService().DeleteBuiltinRuleTemplate(c, parseID(c))
}

func ApplyBuiltinRuleTemplate(c *gin.Context) {
	var req model.ApplyBuiltinRuleTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().ApplyBuiltinRuleTemplate(c, parseID(c), req)
}

func GetBuiltinRuleTemplate(c *gin.Context) {
	service.GetAlertManagerService().GetBuiltinRuleTemplate(c, parseID(c))
}

func ListBuiltinRuleTemplates(c *gin.Context) {
	var query model.AlertBuiltinRuleTemplateQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListBuiltinRuleTemplates(c, query)
}

func PreviewPrometheusTemplates(c *gin.Context) {
	var query model.PreviewPrometheusTemplateQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().PreviewPrometheusTemplates(c, query)
}

func ListPrometheusTemplateImportTypes(c *gin.Context) {
	service.GetAlertManagerService().ListPrometheusTemplateImportTypes(c)
}

func ImportPrometheusTemplates(c *gin.Context) {
	var req model.ImportPrometheusTemplatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().ImportPrometheusTemplates(c, req)
}

func CreateBuiltinRuleTemplateGroup(c *gin.Context) {
	var item model.AlertBuiltinRuleTemplateGroup
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().CreateBuiltinRuleTemplateGroup(c, item)
}

func UpdateBuiltinRuleTemplateGroup(c *gin.Context) {
	var item model.AlertBuiltinRuleTemplateGroup
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	if item.ID == 0 {
		item.ID = parseUintParam(c, "id")
	}
	service.GetAlertManagerService().UpdateBuiltinRuleTemplateGroup(c, item)
}

func DeleteBuiltinRuleTemplateGroup(c *gin.Context) {
	service.GetAlertManagerService().DeleteBuiltinRuleTemplateGroup(c, parseID(c))
}

func GetBuiltinRuleTemplateGroup(c *gin.Context) {
	service.GetAlertManagerService().GetBuiltinRuleTemplateGroup(c, parseID(c))
}

func ListBuiltinRuleTemplateGroups(c *gin.Context) {
	var query model.AlertBuiltinRuleTemplateGroupQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListBuiltinRuleTemplateGroups(c, query)
}

func AllBuiltinRuleTemplateGroupTree(c *gin.Context) {
	service.GetAlertManagerService().AllBuiltinRuleTemplateGroupTree(c)
}

func ListBuiltinRuleTemplateGroupOptions(c *gin.Context) {
	var query model.AlertBuiltinRuleTemplateGroupOptionsQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListBuiltinRuleTemplateGroupOptions(c, query)
}

func ListActiveEvents(c *gin.Context) {
	var query model.AlertEventQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListActiveEvents(c, query)
}

func ListActiveEventBusinessOptions(c *gin.Context) {
	service.GetAlertManagerService().ListActiveEventBusinessOptions(c)
}

func GetDashboardCharts(c *gin.Context) {
	var query model.AlertDashboardQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().GetDashboardCharts(c, query)
}

func GetDashboardScreen(c *gin.Context) {
	var query model.AlertDashboardQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().GetDashboardScreen(c, query)
}

func GetDashboardAlertNameTop(c *gin.Context) {
	var query model.AlertDashboardQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().GetDashboardAlertNameTop(c, query)
}

func GetDashboardNotifyTrend(c *gin.Context) {
	var query model.AlertDashboardQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().GetDashboardNotifyTrend(c, query)
}

func GetDashboardHostGroupTop(c *gin.Context) {
	var query model.AlertDashboardQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().GetDashboardBusinessRouteTop(c, query)
}

func GetDashboardRuleGroupTop(c *gin.Context) {
	var query model.AlertDashboardQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().GetDashboardBusinessRouteTop(c, query)
}

func GetDashboardBusinessRouteTop(c *gin.Context) {
	var query model.AlertDashboardQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().GetDashboardBusinessRouteTop(c, query)
}

func ListActiveEventGroups(c *gin.Context) {
	var query model.AlertEventQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListActiveEventGroups(c, query)
}

func ListHistoryEvents(c *gin.Context) {
	var query model.AlertEventQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListHistoryEvents(c, query)
}

func GetEventDetail(c *gin.Context) {
	service.GetAlertManagerService().GetEventDetail(c, c.Param("eventType"), parseUint64Param(c, "eventId"))
}

func ListTimeline(c *gin.Context) {
	eventID, _ := strconv.ParseUint(c.Query("eventId"), 10, 64)
	if eventID == 0 {
		eventID, _ = strconv.ParseUint(c.Param("eventId"), 10, 64)
	}
	service.GetAlertManagerService().ListTimeline(c, eventID)
}

func ClaimActiveEvent(c *gin.Context) {
	service.GetAlertManagerService().ClaimActiveEvent(c, parseUint64Param(c, "eventId"))
}

func SilenceActiveEvent(c *gin.Context) {
	var req model.SilenceEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().SilenceActiveEvent(c, parseUint64Param(c, "eventId"), req)
}

func UnsilenceActiveEvent(c *gin.Context) {
	service.GetAlertManagerService().UnsilenceActiveEvent(c, parseUint64Param(c, "eventId"))
}

func CreateSilenceRule(c *gin.Context) {
	var item model.AlertSilenceRule
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().CreateSilenceRule(c, item)
}

func UpdateSilenceRule(c *gin.Context) {
	var item model.AlertSilenceRule
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	if item.ID == 0 {
		item.ID = parseUintParam(c, "id")
	}
	service.GetAlertManagerService().UpdateSilenceRule(c, item)
}

func DeleteSilenceRule(c *gin.Context) {
	service.GetAlertManagerService().DeleteSilenceRule(c, parseID(c))
}

func GetSilenceRule(c *gin.Context) {
	service.GetAlertManagerService().GetSilenceRule(c, parseID(c))
}

func ListSilenceRules(c *gin.Context) {
	var query model.AlertSilenceRuleQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListSilenceRules(c, query)
}

func CreateNotifyChannel(c *gin.Context) {
	var item model.AlertNotifyChannel
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().CreateNotifyChannel(c, item)
}

func UpdateNotifyChannel(c *gin.Context) {
	var item model.AlertNotifyChannel
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	if item.ID == 0 {
		item.ID = parseUintParam(c, "id")
	}
	service.GetAlertManagerService().UpdateNotifyChannel(c, item)
}

func DeleteNotifyChannel(c *gin.Context) {
	service.GetAlertManagerService().DeleteNotifyChannel(c, parseID(c))
}

func GetNotifyChannel(c *gin.Context) {
	service.GetAlertManagerService().GetNotifyChannel(c, parseID(c))
}

func ListNotifyChannels(c *gin.Context) {
	var query model.AlertNotifyChannelQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListNotifyChannels(c, query)
}

func TestNotifyChannel(c *gin.Context) {
	var req model.AlertNotifyChannelTestRequest
	_ = c.ShouldBindJSON(&req)
	service.GetAlertManagerService().TestNotifyChannel(c, parseID(c), req)
}

func HandleFeishuCallback(c *gin.Context) {
	service.GetAlertManagerService().HandleFeishuCallback(c)
}

func CreateNotifyGroup(c *gin.Context) {
	var item model.AlertNotifyGroup
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().CreateNotifyGroup(c, item)
}

func UpdateNotifyGroup(c *gin.Context) {
	var item model.AlertNotifyGroup
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	if item.ID == 0 {
		item.ID = parseUintParam(c, "id")
	}
	service.GetAlertManagerService().UpdateNotifyGroup(c, item)
}

func DeleteNotifyGroup(c *gin.Context) {
	service.GetAlertManagerService().DeleteNotifyGroup(c, parseID(c))
}

func GetNotifyGroup(c *gin.Context) {
	service.GetAlertManagerService().GetNotifyGroup(c, parseID(c))
}

func ListNotifyGroups(c *gin.Context) {
	var query model.AlertNotifyGroupQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListNotifyGroups(c, query)
}

func CreateNotifyRouteRule(c *gin.Context) {
	var item model.AlertNotifyRouteRule
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().CreateNotifyRouteRule(c, item)
}

func UpdateNotifyRouteRule(c *gin.Context) {
	var item model.AlertNotifyRouteRule
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	if item.ID == 0 {
		item.ID = parseUintParam(c, "id")
	}
	service.GetAlertManagerService().UpdateNotifyRouteRule(c, item)
}

func DeleteNotifyRouteRule(c *gin.Context) {
	service.GetAlertManagerService().DeleteNotifyRouteRule(c, parseID(c))
}

func GetNotifyRouteRule(c *gin.Context) {
	service.GetAlertManagerService().GetNotifyRouteRule(c, parseID(c))
}

func ListNotifyRouteRules(c *gin.Context) {
	var query model.AlertNotifyRouteRuleQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListNotifyRouteRules(c, query)
}

func CreateNotifyTemplate(c *gin.Context) {
	var item model.AlertNotifyTemplate
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().CreateNotifyTemplate(c, item)
}

func UpdateNotifyTemplate(c *gin.Context) {
	var item model.AlertNotifyTemplate
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	if item.ID == 0 {
		item.ID = parseUintParam(c, "id")
	}
	service.GetAlertManagerService().UpdateNotifyTemplate(c, item)
}

func DeleteNotifyTemplate(c *gin.Context) {
	service.GetAlertManagerService().DeleteNotifyTemplate(c, parseID(c))
}

func GetNotifyTemplate(c *gin.Context) {
	service.GetAlertManagerService().GetNotifyTemplate(c, parseID(c))
}

func ListNotifyTemplates(c *gin.Context) {
	var query model.AlertNotifyTemplateQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListNotifyTemplates(c, query)
}

func PreviewNotifyTemplate(c *gin.Context) {
	var req model.AlertNotifyTemplatePreviewRequest
	_ = c.ShouldBindJSON(&req)
	service.GetAlertManagerService().PreviewNotifyTemplate(c, parseID(c), req)
}

func CreateNotifyRule(c *gin.Context) {
	var item model.AlertNotifyRule
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	service.GetAlertManagerService().CreateNotifyRule(c, item)
}

func UpdateNotifyRule(c *gin.Context) {
	var item model.AlertNotifyRule
	if err := c.ShouldBindJSON(&item); err != nil {
		result.Failed(c, 400, "invalid request body")
		return
	}
	if item.ID == 0 {
		item.ID = parseUintParam(c, "id")
	}
	service.GetAlertManagerService().UpdateNotifyRule(c, item)
}

func DeleteNotifyRule(c *gin.Context) {
	service.GetAlertManagerService().DeleteNotifyRule(c, parseID(c))
}

func GetNotifyRule(c *gin.Context) {
	service.GetAlertManagerService().GetNotifyRule(c, parseID(c))
}

func ListNotifyRules(c *gin.Context) {
	var query model.AlertNotifyRuleQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListNotifyRules(c, query)
}

func ListNotifyRecords(c *gin.Context) {
	var query model.AlertNotifyRecordQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().ListNotifyRecords(c, query)
}

func GetNotifyRecordStats(c *gin.Context) {
	var query model.AlertNotifyRecordQuery
	_ = c.ShouldBindQuery(&query)
	service.GetAlertManagerService().GetNotifyRecordStats(c, query)
}

func parseID(c *gin.Context) uint {
	id := parseUintParam(c, "id")
	if id > 0 {
		return id
	}
	var req model.IDDto
	if err := c.ShouldBindJSON(&req); err == nil {
		return req.ID
	}
	return 0
}

func parseUintParam(c *gin.Context, key string) uint {
	value := c.Param(key)
	if value == "" {
		value = c.Query(key)
	}
	id, _ := strconv.ParseUint(value, 10, 64)
	return uint(id)
}

func parseUint64Param(c *gin.Context, key string) uint64 {
	value := c.Param(key)
	if value == "" {
		value = c.Query(key)
	}
	id, _ := strconv.ParseUint(value, 10, 64)
	return id
}

func parseDatasourceIDQuery(c *gin.Context) uint {
	value := firstQuery(c, "datasourceId", "datasource_id")
	id, _ := strconv.ParseUint(value, 10, 64)
	return uint(id)
}

func firstQuery(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		if value := c.Query(key); value != "" {
			return value
		}
	}
	return ""
}
