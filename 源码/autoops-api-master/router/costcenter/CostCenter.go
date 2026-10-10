package costcenter

import (
	"context"

	"dodevops-api/api/costcenter/controller"
	"dodevops-api/api/costcenter/service"
	"dodevops-api/common"

	"github.com/gin-gonic/gin"
)

// RegisterCostCenterRoutes registers the redesigned cloud billing APIs.
func RegisterCostCenterRoutes(api *gin.RouterGroup) {
	db := common.GetDB()
	service.StartCloudBillScheduler(context.Background(), service.NewBillCloudAccountService(db))

	billDetailController := controller.NewBillDetailController(db)
	accountController := controller.NewBillCloudAccountController(db)
	dashboardController := controller.NewCloudBillDashboardController(db)
	balanceController := controller.NewBalanceAlertController(db)

	billGroup := api.Group("/cost-center")
	{
		billGroup.GET("/overview", dashboardController.Overview)
		billGroup.GET("/providers/:operator", dashboardController.ProviderDetail)
		billGroup.GET("/details", billDetailController.GetList)
		billGroup.GET("/details/export", billDetailController.Export)
		billGroup.GET("/details/:id", billDetailController.GetByID)
	}

	accountGroup := api.Group("/cost-center/accounts")
	{
		accountGroup.GET("", accountController.GetList)
		accountGroup.POST("", accountController.Create)
		accountGroup.PUT("/:id", accountController.Update)
		accountGroup.DELETE("/:id", accountController.Delete)
		accountGroup.POST("/:id/sync-bills", accountController.ManualSync)
		accountGroup.POST("/:id/refresh-balance", accountController.RefreshBalance)
		accountGroup.GET("/:id/balance-alert", balanceController.GetRule)
		accountGroup.PUT("/:id/balance-alert", balanceController.SaveRule)
		accountGroup.POST("/sync-all", accountController.SyncAll)
		accountGroup.POST("/refresh-all-balances", accountController.RefreshAllBalances)
	}

	balanceGroup := api.Group("/cost-center/balance")
	{
		balanceGroup.GET("/overview", balanceController.Overview)
	}
}
