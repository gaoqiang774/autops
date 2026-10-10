// router/rdp_router.go
package cmdb

import (
	"github.com/gin-gonic/gin"

	"dodevops-api/api/cmdb/controller"
	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/service"
)

func RegisteRDPRoutes(api *gin.RouterGroup) {
	rdpDAO := dao.NewRDPTargetDAO()
	rdpService := service.NewRDPService(rdpDAO)
	rdpController := controller.NewRDPController(rdpService)

	// WebSocket 路由 - 使用原生处理
	api.GET("/rdp/ws", func(ctx *gin.Context) {
		rdpController.ConnectRDPNative(ctx)
		ctx.Abort()
	})

	// 管理接口
	rdpGroup := api.Group("/rdp")
	{
		rdpGroup.POST("/session/open", rdpController.OpenSession)
		rdpGroup.POST("/upload/:hostId", rdpController.UploadHostFile)
		rdpGroup.GET("/upload/task/:taskId", rdpController.GetUploadTaskStatus)
		rdpGroup.POST("/disconnect", rdpController.DisconnectRDP)
		rdpGroup.GET("/sessions", rdpController.GetActiveSessions)
	}
}
