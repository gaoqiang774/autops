package task

import (
	"dodevops-api/api/task/controller"
	"dodevops-api/api/task/service"
	"dodevops-api/common"

	"github.com/gin-gonic/gin"
)

// RegisterTaskRoutes registers authenticated task-center routes.
func RegisterTaskRoutes(router *gin.RouterGroup) {
	router.POST("/template/add", controller.CreateTemplate)
	router.GET("/template/list", controller.GetAllTemplates)
	router.PUT("/template/update", controller.UpdateTemplate)
	router.DELETE("/template/delete", controller.DeleteTemplate)
	router.GET("/template/info/:id", controller.GetTemplateByID)
	router.GET("/template/content/:id", controller.GetTemplateContent)
	router.GET("/template/query/name", controller.GetTemplatesByName)
	router.GET("/template/query/type", controller.GetTemplatesByType)

	router.POST("/task/add", controller.CreateTask)
	router.GET("/task/get", controller.GetTaskByID)
	router.PUT("/task/update", controller.UpdateTask)
	router.DELETE("/task/delete", controller.DeleteTask)
	router.GET("/task/list", controller.ListTasks)
	router.GET("/task/list-with-details", controller.ListTasksWithDetails)
	router.GET("/task/query/name", controller.GetTasksByName)
	router.GET("/task/query/type", controller.GetTasksByType)
	router.GET("/task/query/status", controller.GetTasksByStatus)
	router.GET("/task/next-execution", controller.GetNextExecutionTime)
	router.GET("/task/execution-info", controller.GetTaskExecutionInfo)
	router.GET("/task/templates", controller.GetTaskTemplatesWithStatus)

	work := controller.TaskWork()
	router.POST("/taskjob/start", work.StartJob)
	router.GET("/taskjob/log", work.GetJobLog)
	router.POST("/taskjob/stop", work.StopJob)
	router.GET("/taskjob/status", work.GetJobStatus)

	monitor := controller.NewTaskMonitorController()
	router.GET("/task/monitor/queue/metrics", monitor.GetQueueMetrics)
	router.GET("/task/monitor/scheduler/stats", monitor.GetSchedulerStats)
	router.GET("/task/monitor/system/status", monitor.GetSystemStatus)
	router.GET("/task/monitor/queue/details", monitor.GetQueueDetails)
	router.POST("/task/monitor/queue/clear-failed", monitor.ClearFailedQueue)
	router.POST("/task/monitor/queue/retry-failed", monitor.RetryFailedTasks)
	router.POST("/task/monitor/scheduled/pause", monitor.PauseScheduledTask)
	router.POST("/task/monitor/scheduled/resume", monitor.ResumeScheduledTask)
	router.POST("/task/monitor/scheduled/reset", monitor.ResetScheduledTaskStatus)
	router.GET("/task/monitor/task/status", monitor.GetTaskStatus)

	ansible := controller.NewTaskAnsibleController(service.NewTaskAnsibleService(common.GetDB()))
	router.GET("/task/ansiblelist", ansible.List)
	router.POST("/task/ansible", ansible.CreateTask)
	router.POST("/task/k8s", ansible.CreateK8sTask)
	router.GET("/task/ansible/query/name", ansible.GetTasksByName)
	router.GET("/task/ansible/query/type", ansible.GetTasksByType)
	router.GET("/task/ansible/:id", ansible.GetTask)
	router.POST("/task/ansible/:id/start", ansible.StartTask)
	router.POST("/task/ansible/:id/stop/:work_id", ansible.StopTask)
	router.DELETE("/task/ansible/:id", ansible.DeleteTask)
	router.GET("/task/ansible/:id/log/:work_id", ansible.GetJobLog)
}

// RegisterWebSocketRoutes registers the token-authenticated real-time log route.
func RegisterWebSocketRoutes(router *gin.RouterGroup) {
	ws := controller.NewWebSocketController(service.NewTaskAnsibleService(common.GetDB()))
	router.GET("/ws/task/ansible/:id/log/:work_id", ws.GetJobLogWS)
}
