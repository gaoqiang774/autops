package cmdb

import (
	"dodevops-api/api/cmdb/controller"
	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/service"
	"dodevops-api/common"

	"github.com/gin-gonic/gin"
)

// RegisterCmdbRoutes 注册 CMDB 相关路由
func RegisterCmdbModelRoutes(api *gin.RouterGroup) {
	// 初始化依赖
	db := common.GetDB()
	cmdbDao := dao.NewCmdbDao(db)
	cmdbService := service.NewCmdbService(cmdbDao)
	cmdbController := controller.NewCmdbController(cmdbService)

	// 业务模型：业务健康视图、资产关系图谱、健康快照写入
	api.PUT("/resources/:id", cmdbController.UpdateBusinessModelResource)
	api.DELETE("/relations/:id", cmdbController.DeleteBusinessModelRelation)

	businessModels := api.Group("/cmdb/business-models")
	{
		businessModels.POST("", cmdbController.CreateBusinessModel)
		businessModels.GET("", cmdbController.ListBusinessModels)
		businessModels.DELETE("/:id", cmdbController.DeleteBusinessModel)
		businessModels.POST("/health-snapshots", cmdbController.UpsertHealthSnapshot)
		businessModels.POST("/:id/bindings", cmdbController.BindBusinessModelResources)
		businessModels.GET("/:id/overview", cmdbController.GetBusinessModelOverview)
		businessModels.GET("/:id/topology", cmdbController.GetBusinessModelTopology)
		businessModels.GET("/:id/observability", cmdbController.GetBusinessModelObservability)
		businessModels.GET("/:id/log-analysis", cmdbController.GetBusinessModelLogAnalysis)
		businessModels.GET("/:id/monitor-alerts/config", cmdbController.GetBusinessMonitorAlertConfig)
		businessModels.PUT("/:id/monitor-alerts/config", cmdbController.SaveBusinessMonitorAlertConfig)
		businessModels.GET("/:id/monitor-alerts", cmdbController.GetBusinessMonitorAlerts)
		businessModels.POST("/:id/health/refresh", cmdbController.RefreshBusinessModelHealth)
	}
}
