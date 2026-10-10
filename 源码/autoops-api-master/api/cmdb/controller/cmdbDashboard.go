package controller

import (
	"net/http"
	"strconv"

	"dodevops-api/api/cmdb/service"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

type CmdbDashboardController struct {
	service service.CmdbDashboardServiceInterface
}

func NewCmdbDashboardController() *CmdbDashboardController {
	return &CmdbDashboardController{
		service: service.GetCmdbDashboardService(),
	}
}

// GetCards 获取资产管理大盘卡片数据
// @Summary 获取资产管理大盘卡片数据
// @Description 聚合物理机/虚拟机/交换机、机房机柜、监控覆盖、维保到期、云主机和认证率
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param warrantyDays query int false "维保到期窗口天数，默认90，最大365"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/dashboard/cards [get]
// @Security ApiKeyAuth
func (c *CmdbDashboardController) GetCards(ctx *gin.Context) {
	warrantyDays := parseDashboardIntQuery(ctx, "warrantyDays", 90)
	data, err := c.service.GetCards(warrantyDays)
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	result.Success(ctx, data)
}

// GetCharts 获取资产管理大盘图表数据
// @Summary 获取资产管理大盘图表数据
// @Description 聚合品牌分布、机房物理机数量和用途分布
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/dashboard/charts [get]
// @Security ApiKeyAuth
func (c *CmdbDashboardController) GetCharts(ctx *gin.Context) {
	data, err := c.service.GetCharts()
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	result.Success(ctx, data)
}

// GetTables 获取资产管理大盘表格数据
// @Summary 获取资产管理大盘表格数据
// @Description 聚合CPU/内存/磁盘最高和最低TOP，以及硬件异常列表
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param limit query int false "CPU/内存/磁盘排行数量，默认5，最大10"
// @Param hardwareLimit query int false "硬件异常返回数量，默认20，最大100"
// @Param lookbackMinutes query int false "硬件指标回看分钟数，默认5，最大15"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/dashboard/tables [get]
// @Security ApiKeyAuth
func (c *CmdbDashboardController) GetTables(ctx *gin.Context) {
	limit := parseDashboardIntQuery(ctx, "limit", 5)
	hardwareLimit := parseDashboardIntQuery(ctx, "hardwareLimit", 20)
	lookbackMinutes := parseDashboardIntQuery(ctx, "lookbackMinutes", 5)

	data, err := c.service.GetTables(limit, hardwareLimit, lookbackMinutes)
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	result.Success(ctx, data)
}

func parseDashboardIntQuery(ctx *gin.Context, key string, fallback int) int {
	value := ctx.Query(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
