package ticket

import (
	"dodevops-api/api/ticket/controller"

	"github.com/gin-gonic/gin"
)

// RegisterTicketRoutes 注册工单系统相关路由
func RegisterTicketRoutes(router *gin.RouterGroup) {
	ticketCtrl := controller.NewTicketController()

	// 工单工作台
	router.GET("/ticket/overview", ticketCtrl.Overview)                       // 获取工单统计概览
	router.GET("/ticket/approval-tasks/my", ticketCtrl.MyApprovalTasks)       // 获取我的审批任务
	router.POST("/ticket/approval-tasks/:id/approve", ticketCtrl.ApproveTask) // 审批通过
	router.POST("/ticket/approval-tasks/:id/reject", ticketCtrl.RejectTask)   // 审批驳回

	// 审批流管理，具体路径放在参数路径之前
	router.GET("/ticket/workflows", ticketCtrl.ListWorkflows)                   // 获取审批流列表
	router.POST("/ticket/workflows", ticketCtrl.SaveWorkflow)                   // 创建审批流
	router.GET("/ticket/workflows/instances/:id", ticketCtrl.WorkflowDetail)    // 获取审批实例详情
	router.PUT("/ticket/workflows/:id/status", ticketCtrl.ChangeWorkflowStatus) // 启用或禁用审批流
	router.GET("/ticket/workflows/:id", ticketCtrl.GetWorkflow)                 // 获取审批流详情
	router.PUT("/ticket/workflows/:id", ticketCtrl.SaveWorkflow)                // 更新审批流
	router.DELETE("/ticket/workflows/:id", ticketCtrl.DeleteWorkflow)           // 删除审批流

	// 日常事务工单
	router.GET("/ticket/daily", ticketCtrl.ListDaily)                 // 获取日常工单列表
	router.POST("/ticket/daily", ticketCtrl.CreateDaily)              // 创建日常工单
	router.GET("/ticket/daily/:id", ticketCtrl.DailyDetail)           // 获取日常工单详情
	router.POST("/ticket/daily/:id/assign", ticketCtrl.AssignDaily)   // 分派日常工单
	router.POST("/ticket/daily/:id/start", ticketCtrl.StartDaily)     // 开始处理日常工单
	router.POST("/ticket/daily/:id/resolve", ticketCtrl.ResolveDaily) // 提交解决结果
	router.POST("/ticket/daily/:id/confirm", ticketCtrl.ConfirmDaily) // 确认解决结果
	router.POST("/ticket/daily/:id/close", ticketCtrl.CloseDaily)     // 关闭日常工单
	router.POST("/ticket/daily/:id/cancel", ticketCtrl.CancelDaily)   // 撤销日常工单

	// Jenkins 发布应用配置
	router.GET("/ticket/release-applications", ticketCtrl.ListApplications)         // 获取发布应用列表
	router.POST("/ticket/release-applications", ticketCtrl.SaveApplication)         // 创建发布应用
	router.PUT("/ticket/release-applications/:id", ticketCtrl.SaveApplication)      // 更新发布应用
	router.DELETE("/ticket/release-applications/:id", ticketCtrl.DeleteApplication) // 删除发布应用

	// 应用发布工单
	router.GET("/ticket/releases", ticketCtrl.ListReleases)                                // 获取发布工单列表
	router.POST("/ticket/releases", ticketCtrl.CreateRelease)                              // 创建发布工单
	router.GET("/ticket/releases/:id", ticketCtrl.ReleaseDetail)                           // 获取发布工单详情
	router.POST("/ticket/releases/:id/execute", ticketCtrl.ExecuteRelease)                 // 执行发布工单
	router.POST("/ticket/releases/:id/items/:itemId/retry", ticketCtrl.RetryReleaseItem)   // 重试发布项
	router.POST("/ticket/releases/:id/items/:itemId/cancel", ticketCtrl.CancelReleaseItem) // 中止发布项
	router.GET("/ticket/releases/:id/items/:itemId/log", ticketCtrl.ReleaseItemLog)        // 获取 Jenkins 构建日志
}
