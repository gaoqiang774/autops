package controller

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/api/cmdb/service"
	logtailservice "dodevops-api/api/logtail/service"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CmdbController struct {
	service        *service.CmdbService
	logtailService *logtailservice.LogtailService
}

func NewCmdbController(service *service.CmdbService) *CmdbController {
	return &CmdbController{
		service:        service,
		logtailService: logtailservice.NewLogtailService(),
	}
}

// CreateBusinessModel 创建业务模型
// @Summary 创建业务模型
// @Tags CMDB业务模型
// @Param data body model.CreateBusinessModelDto true "业务模型数据"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cmdb/business-models [post]
func (c *CmdbController) CreateBusinessModel(ctx *gin.Context) {
	var req model.CreateBusinessModelDto
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	result, err := c.service.CreateBusinessModel(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建业务模型失败: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    result,
	})
}

// ListBusinessModels 获取业务模型列表
// @Summary 获取业务模型列表
// @Tags CMDB业务模型
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "关键词"
// @Param env query string false "环境"
// @Param status query string false "状态"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cmdb/business-models [get]
func (c *CmdbController) ListBusinessModels(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))

	result, err := c.service.ListBusinessModels(model.BusinessModelQuery{
		Page:     page,
		PageSize: pageSize,
		Keyword:  ctx.Query("keyword"),
		Env:      ctx.Query("env"),
		Status:   ctx.Query("status"),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取业务模型列表失败: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// DeleteBusinessModel 删除业务模型，并清理该业务的诊断快照和拓扑关系
// @Summary 删除业务模型
// @Tags CMDB业务模型
// @Param id path int true "业务模型ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cmdb/business-models/{id} [delete]
func (c *CmdbController) DeleteBusinessModel(ctx *gin.Context) {
	businessID, ok := parseBusinessModelUintParam(ctx, "id", "无效的业务模型ID")
	if !ok {
		return
	}

	result, err := c.service.DeleteBusinessModel(businessID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除业务模型失败: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
		"data":    result,
	})
}

// BindBusinessModelResources 绑定业务模型资源
// @Summary 绑定业务模型资源
// @Tags CMDB业务模型
// @Param id path int true "业务模型ID"
// @Param data body model.BusinessModelBindingDto true "绑定数据"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cmdb/business-models/{id}/bindings [post]
func (c *CmdbController) BindBusinessModelResources(ctx *gin.Context) {
	businessID, ok := parseBusinessModelUintParam(ctx, "id", "无效的业务模型ID")
	if !ok {
		return
	}

	var req model.BusinessModelBindingDto
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	result, err := c.service.BindBusinessModelResources(businessID, req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "绑定业务资源失败: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "绑定成功",
		"data":    result,
	})
}

// UpdateBusinessModelResource 更新业务图谱资源节点
// @Summary 更新业务图谱资源节点
// @Tags CMDB业务模型
// @Param id path int true "资源节点ID"
// @Param data body model.BusinessModelBindingResourceDto true "资源节点数据"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/resources/{id} [put]
func (c *CmdbController) UpdateBusinessModelResource(ctx *gin.Context) {
	resourceID, ok := parseBusinessModelUintParam(ctx, "id", "无效的资源ID")
	if !ok {
		return
	}

	var req model.BusinessModelBindingResourceDto
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	result, err := c.service.UpdateBusinessModelResource(resourceID, req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		ctx.JSON(status, gin.H{
			"code":    status,
			"message": "保存资源失败: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "保存成功",
		"data":    result,
	})
}

// DeleteBusinessModelRelation 删除业务图谱关系
// @Summary 删除业务图谱关系
// @Tags CMDB业务模型
// @Param id path int true "关系ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/relations/{id} [delete]
func (c *CmdbController) DeleteBusinessModelRelation(ctx *gin.Context) {
	relationID, ok := parseBusinessModelUintParam(ctx, "id", "无效的关系ID")
	if !ok {
		return
	}

	result, err := c.service.DeleteBusinessModelRelation(relationID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		ctx.JSON(status, gin.H{
			"code":    status,
			"message": "删除关系失败: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
		"data":    result,
	})
}

// GetBusinessModelOverview 获取业务健康总览
// @Summary 获取业务健康总览
// @Tags CMDB业务模型
// @Param id path int true "业务模型ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cmdb/business-models/{id}/overview [get]
func (c *CmdbController) GetBusinessModelOverview(ctx *gin.Context) {
	businessID, ok := parseBusinessModelUintParam(ctx, "id", "无效的业务模型ID")
	if !ok {
		return
	}

	result, err := c.service.GetBusinessModelOverview(businessID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取业务健康总览失败: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// GetBusinessModelTopology 获取业务关系拓扑
// @Summary 获取业务关系拓扑
// @Tags CMDB业务模型
// @Param id path int true "业务模型ID"
// @Param depth query int false "查询深度"
// @Param direction query string false "查询方向"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cmdb/business-models/{id}/topology [get]
func (c *CmdbController) GetBusinessModelTopology(ctx *gin.Context) {
	businessID, ok := parseBusinessModelUintParam(ctx, "id", "无效的业务模型ID")
	if !ok {
		return
	}

	depth, _ := strconv.Atoi(ctx.DefaultQuery("depth", "4"))
	direction := ctx.DefaultQuery("direction", "downstream")

	result, err := c.service.GetBusinessModelTopology(businessID, depth, direction)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取业务拓扑失败: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// GetBusinessModelObservability 获取业务 SLA 可观测大屏数据
// @Summary 获取业务 SLA 可观测大屏数据
// @Tags CMDB业务模型
// @Param id path int true "业务模型ID"
// @Param window query string false "相对时间窗口，例如 15m、1h、24h、7d"
// @Param start query string false "自定义开始时间，RFC3339，需与 end 同时提供"
// @Param end query string false "自定义结束时间，RFC3339，需与 start 同时提供"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cmdb/business-models/{id}/observability [get]
func (c *CmdbController) GetBusinessModelObservability(ctx *gin.Context) {
	businessID, ok := parseBusinessModelUintParam(ctx, "id", "无效的业务模型ID")
	if !ok {
		return
	}

	query := model.BusinessObservabilityQuery{
		Window: ctx.Query("window"),
		Start:  ctx.Query("start"),
		End:    ctx.Query("end"),
	}
	data, err := c.service.GetBusinessModelObservability(ctx, c.logtailService, businessID, query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidObservabilityWindow) {
			status = http.StatusBadRequest
		}
		ctx.JSON(status, gin.H{
			"code":    status,
			"message": "获取业务 SLA 可观测数据失败: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

// GetBusinessModelLogAnalysis 获取业务入口 Nginx/Gateway 日志分析数据。
// @Summary 获取业务 HTTP 日志分析数据
// @Tags CMDB业务模型
// @Param id path int true "业务模型ID"
// @Param window query string false "相对时间窗口"
// @Param sourceType query string false "入口类型: all/nginx/gateway"
// @Param storeId query int false "日志库ID"
// @Param domain query string false "域名"
// @Param statusClass query string false "状态段或状态码"
// @Param requestPath query string false "接口路径"
// @Param clientIp query string false "客户端IP"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cmdb/business-models/{id}/log-analysis [get]
func (c *CmdbController) GetBusinessModelLogAnalysis(ctx *gin.Context) {
	businessID, ok := parseBusinessModelUintParam(ctx, "id", "无效的业务模型ID")
	if !ok {
		return
	}
	storeID, ok := parseOptionalBusinessModelUintQuery(ctx, "storeId", "无效的日志库ID")
	if !ok {
		return
	}
	query := model.BusinessLogAnalysisQuery{
		BusinessObservabilityQuery: model.BusinessObservabilityQuery{
			Window: ctx.Query("window"),
			Start:  ctx.Query("start"),
			End:    ctx.Query("end"),
		},
		SourceType:  ctx.Query("sourceType"),
		StoreID:     storeID,
		Domain:      ctx.Query("domain"),
		StatusClass: ctx.Query("statusClass"),
		RequestPath: ctx.Query("requestPath"),
		ClientIP:    ctx.Query("clientIp"),
	}
	data, err := c.service.GetBusinessModelLogAnalysis(ctx, c.logtailService, businessID, query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidObservabilityWindow) || errors.Is(err, service.ErrInvalidBusinessLogAnalysisFilter) {
			status = http.StatusBadRequest
		}
		ctx.JSON(status, gin.H{
			"code":    status,
			"message": "获取业务日志分析数据失败: " + err.Error(),
		})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

// GetBusinessMonitorAlertConfig 获取业务监控告警接入配置。
// @Summary 获取业务监控告警接入配置
// @Tags CMDB业务模型
// @Param id path int true "业务模型ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cmdb/business-models/{id}/monitor-alerts/config [get]
func (c *CmdbController) GetBusinessMonitorAlertConfig(ctx *gin.Context) {
	businessID, ok := parseBusinessModelUintParam(ctx, "id", "无效的业务模型ID")
	if !ok {
		return
	}

	data, err := c.service.GetBusinessMonitorAlertConfig(businessID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取业务监控告警配置失败: " + err.Error(),
		})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

// SaveBusinessMonitorAlertConfig 保存业务监控告警接入配置。
// @Summary 保存业务监控告警接入配置
// @Tags CMDB业务模型
// @Param id path int true "业务模型ID"
// @Param data body model.BusinessMonitorAlertConfigDto true "告警源和匹配规则"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cmdb/business-models/{id}/monitor-alerts/config [put]
func (c *CmdbController) SaveBusinessMonitorAlertConfig(ctx *gin.Context) {
	businessID, ok := parseBusinessModelUintParam(ctx, "id", "无效的业务模型ID")
	if !ok {
		return
	}

	var req model.BusinessMonitorAlertConfigDto
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	data, err := c.service.SaveBusinessMonitorAlertConfig(businessID, req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidBusinessMonitorAlertConfig) {
			status = http.StatusBadRequest
		}
		ctx.JSON(status, gin.H{
			"code":    status,
			"message": "保存业务监控告警配置失败: " + err.Error(),
		})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "保存成功",
		"data":    data,
	})
}

// GetBusinessMonitorAlerts 获取业务关联的活跃监控告警。
// @Summary 获取业务关联的活跃监控告警
// @Tags CMDB业务模型
// @Param id path int true "业务模型ID"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param severity query string false "告警级别"
// @Param status query string false "告警状态"
// @Param datasourceId query int false "临时覆盖告警源ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cmdb/business-models/{id}/monitor-alerts [get]
func (c *CmdbController) GetBusinessMonitorAlerts(ctx *gin.Context) {
	businessID, ok := parseBusinessModelUintParam(ctx, "id", "无效的业务模型ID")
	if !ok {
		return
	}
	datasourceID, ok := parseOptionalBusinessModelUintQuery(ctx, "datasourceId", "无效的告警源ID")
	if !ok {
		return
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))

	data, err := c.service.GetBusinessMonitorAlerts(businessID, model.BusinessMonitorAlertQuery{
		Page:         page,
		PageSize:     pageSize,
		Severity:     ctx.Query("severity"),
		Status:       ctx.Query("status"),
		DatasourceID: datasourceID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取业务监控告警失败: " + err.Error(),
		})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

// RefreshBusinessModelHealth 刷新业务健康评分
// @Summary 刷新业务健康评分
// @Tags CMDB业务模型
// @Param id path int true "业务模型ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cmdb/business-models/{id}/health/refresh [post]
func (c *CmdbController) RefreshBusinessModelHealth(ctx *gin.Context) {
	businessID, ok := parseBusinessModelUintParam(ctx, "id", "无效的业务模型ID")
	if !ok {
		return
	}

	result, err := c.service.RefreshBusinessModelHealth(businessID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "刷新业务健康失败: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "刷新成功",
		"data":    result,
	})
}

// UpsertHealthSnapshot 写入资源健康快照
// @Summary 写入资源健康快照
// @Tags CMDB业务模型
// @Param data body model.UpsertHealthSnapshotDto true "健康快照"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cmdb/business-models/health-snapshots [post]
func (c *CmdbController) UpsertHealthSnapshot(ctx *gin.Context) {
	var req model.UpsertHealthSnapshotDto
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	result, err := c.service.UpsertHealthSnapshot(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "写入健康快照失败: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "写入成功",
		"data":    result,
	})
}

func parseBusinessModelUintParam(ctx *gin.Context, name, message string) (uint, bool) {
	value := ctx.Param(name)
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil || id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": message,
		})
		return 0, false
	}
	return uint(id), true
}

func parseOptionalBusinessModelUintQuery(ctx *gin.Context, name, message string) (uint, bool) {
	value := ctx.Query(name)
	if value == "" {
		return 0, true
	}
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil || id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": message,
		})
		return 0, false
	}
	return uint(id), true
}
