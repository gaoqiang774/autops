package logtail

import (
	logtailcontroller "dodevops-api/api/logtail/controller"

	"github.com/gin-gonic/gin"
)

// RegisterLogtailRoutes 注册日志采集与日志管理模块路由。
//
// 基础路径: /api/v1/logtail
// 当前 MVP 先覆盖 6 张核心表:
//  1. project: 日志项目，作为日志库、采集任务、告警规则的业务归属
//  2. store: 日志库，对应 VictoriaLogs 中的一类日志数据
//  3. parse-template: 解析模板，用于生成 Vector 采集端解析配置
//  4. collect-task: 采集任务，用于下发主机/K8s 日志路径与解析规则
//  5. alert-rule: 日志告警规则，用于配置关键字、阈值、窗口、通知渠道
//  6. alert-event: 告警事件，用于记录规则触发后的历史告警
func RegisterLogtailRoutes(router *gin.RouterGroup) {
	ctrl := logtailcontroller.NewLogtailController()

	group := router.Group("/logtail")

	// VictoriaLogs 连通性检测
	group.GET("/victorialogs/health", ctrl.VictoriaLogsHealth)                              // 检测 VictoriaLogs 地址是否可访问
	group.GET("/vector/package", ctrl.GetVectorPackageInfo)                                 // 获取当前默认 Vector 包信息
	group.POST("/vector/package", ctrl.UploadVectorPackage)                                 // 上传默认 Vector 包
	group.GET("/vector/package/upload/task/:taskId", ctrl.GetVectorPackageUploadTaskStatus) // 获取默认 Vector 包上传任务状态
	group.DELETE("/vector/package", ctrl.DeleteVectorPackage)                               // 删除当前默认 Vector 包
	group.GET("/vector/agents", ctrl.ListVectorAgents)                                      // Vector 主机总览，包含已安装和未安装
	group.GET("/vector/aggregator/config-preview", ctrl.PreviewVectorAggregatorConfig)      // 预览 Kafka 到 VictoriaLogs 的 Aggregator 配置

	// 日志查询与聚合分析
	query := group.Group("/query")
	{
		query.POST("/logs", ctrl.QueryLogs)                // 查询日志明细列表
		query.POST("/traces", ctrl.QueryTraceList)         // 查询 trace 摘要列表
		query.POST("/trace", ctrl.QueryTraceAnalysis)      // 按 trace_id 跨日志库关联日志
		query.POST("/logs/download", ctrl.DownloadLogs)    // 下载日志查询结果
		query.POST("/hits", ctrl.QueryHits)                // 查询日志数量趋势
		query.POST("/fields", ctrl.QueryFieldNames)        // 查询当前条件下的可用字段
		query.POST("/field-values", ctrl.QueryFieldValues) // 查询指定字段的可选值
		query.POST("/stats-range", ctrl.QueryStatsRange)   // 查询 LogsQL stats 聚合趋势
	}

	// 日志项目管理
	group.GET("/projects", ctrl.ListProjects)                        // 分页查询日志项目
	group.GET("/projects/store-counts", ctrl.ListProjectStoreCounts) // 查询项目下日志库数量
	group.GET("/projects/:id", ctrl.GetProject)                      // 获取日志项目详情
	group.POST("/projects", ctrl.CreateProject)                      // 创建日志项目
	group.PUT("/projects/:id", ctrl.UpdateProject)                   // 更新日志项目
	group.DELETE("/projects/:id", ctrl.DeleteProject)                // 删除日志项目

	// 日志库管理
	group.GET("/stores", ctrl.ListStores)         // 分页查询日志库
	group.GET("/stores/:id", ctrl.GetStore)       // 获取日志库详情
	group.POST("/stores", ctrl.CreateStore)       // 创建日志库
	group.PUT("/stores/:id", ctrl.UpdateStore)    // 更新日志库
	group.DELETE("/stores/:id", ctrl.DeleteStore) // 删除日志库

	// 解析模板管理
	group.GET("/parse-templates", ctrl.ListParseTemplates)                       // 分页查询解析模板
	group.POST("/parse-templates/ai-generate", ctrl.GenerateParseTemplateWithAI) // AI 根据样例日志生成解析模板
	group.GET("/parse-templates/:id", ctrl.GetParseTemplate)                     // 获取解析模板详情
	group.POST("/parse-templates", ctrl.CreateParseTemplate)                     // 创建解析模板
	group.PUT("/parse-templates/:id", ctrl.UpdateParseTemplate)                  // 更新解析模板
	group.DELETE("/parse-templates/:id", ctrl.DeleteParseTemplate)               // 删除解析模板

	// 日志采集任务管理
	group.GET("/collect-tasks", ctrl.ListCollectTasks)                 // 分页查询采集任务
	group.GET("/collect-tasks/:id/agents", ctrl.ListCollectTaskAgents) // 查询采集任务命中的 Agent
	group.POST("/collect-tasks/:id/apply", ctrl.ApplyCollectTask)      // 一键应用采集任务到命中 Agent
	group.POST("/collect-tasks/:id/stop", ctrl.StopCollectTask)        // 停止主机采集任务并重载命中 Agent
	group.GET("/collect-tasks/:id", ctrl.GetCollectTask)               // 获取采集任务详情
	group.POST("/collect-tasks", ctrl.CreateCollectTask)               // 创建采集任务
	group.PUT("/collect-tasks/:id", ctrl.UpdateCollectTask)            // 更新采集任务
	group.DELETE("/collect-tasks/:id", ctrl.DeleteCollectTask)         // 删除采集任务

	// K8S 集群级 Collector 生命周期管理
	group.GET("/k8s-cluster-collectors", ctrl.ListKubernetesClusterCollectors)                         // 查询 K8S 集群级 Collector 列表与实时状态
	group.GET("/k8s-cluster-collectors/:id/status", ctrl.GetKubernetesClusterCollectorStatus)          // 查询单个 K8S 集群级 Collector 实时状态
	group.POST("/k8s-cluster-collectors/:id/install", ctrl.InstallKubernetesClusterCollector)          // 安装 K8S 集群级 Collector
	group.POST("/k8s-cluster-collectors/:id/update", ctrl.UpdateKubernetesClusterCollector)            // 更新 K8S 集群级 Collector
	group.POST("/k8s-cluster-collectors/:id/uninstall", ctrl.UninstallKubernetesClusterCollector)      // 卸载 K8S 集群级 Collector
	group.POST("/k8s-cluster-collectors/:id/reinstall", ctrl.ReinstallKubernetesClusterCollector)      // 重装 K8S 集群级 Collector
	group.GET("/k8s-cluster-collectors/:id/operations", ctrl.ListKubernetesClusterCollectorOperations) // 查询 K8S 集群级 Collector 操作记录

	// K8S 任务目标级 Collector 状态
	group.GET("/k8s-collectors", ctrl.ListKubernetesCollectors)                         // 查询 K8S Collector 列表与实时状态
	group.GET("/k8s-collectors/:id/status", ctrl.GetKubernetesCollectorStatus)          // 查询单个 K8S Collector 实时状态
	group.POST("/k8s-collectors/:id/install", ctrl.InstallKubernetesCollector)          // 安装 K8S Collector
	group.POST("/k8s-collectors/:id/update", ctrl.UpdateKubernetesCollector)            // 更新 K8S Collector
	group.POST("/k8s-collectors/:id/uninstall", ctrl.UninstallKubernetesCollector)      // 卸载 K8S Collector
	group.POST("/k8s-collectors/:id/reinstall", ctrl.ReinstallKubernetesCollector)      // 重装 K8S Collector
	group.GET("/k8s-collectors/:id/operations", ctrl.ListKubernetesCollectorOperations) // 查询 K8S Collector 操作记录

	// 日志采集 Agent / Vector 管理
	group.GET("/agents", ctrl.ListAgents)                              // 分页查询日志采集 Agent
	group.GET("/agents/:id", ctrl.GetAgent)                            // 获取日志采集 Agent 详情
	group.GET("/agents/:id/config-preview", ctrl.PreviewAgentConfig)   // 预览 Agent 的 Vector 配置
	group.GET("/agents/:id/collect-tasks", ctrl.ListAgentCollectTasks) // 查询 Agent 命中的采集任务
	group.GET("/agents/:id/operations", ctrl.ListAgentOperations)      // 查询 Agent 操作记录
	group.POST("/agents/:id/operations", ctrl.CreateAgentOperation)    // 创建 Agent 操作记录

	// 日志告警规则管理
	group.GET("/alert-rules", ctrl.ListAlertRules)         // 分页查询告警规则
	group.GET("/alert-rules/:id", ctrl.GetAlertRule)       // 获取告警规则详情
	group.POST("/alert-rules", ctrl.CreateAlertRule)       // 创建告警规则
	group.PUT("/alert-rules/:id", ctrl.UpdateAlertRule)    // 更新告警规则
	group.DELETE("/alert-rules/:id", ctrl.DeleteAlertRule) // 删除告警规则

	// 日志告警事件管理
	group.GET("/alert-events", ctrl.ListAlertEvents)         // 分页查询告警事件
	group.GET("/alert-events/:id", ctrl.GetAlertEvent)       // 获取告警事件详情
	group.POST("/alert-events", ctrl.CreateAlertEvent)       // 创建告警事件
	group.PUT("/alert-events/:id", ctrl.UpdateAlertEvent)    // 更新告警事件
	group.DELETE("/alert-events/:id", ctrl.DeleteAlertEvent) // 删除告警事件
}

// RegisterLogtailAgentPublicRoutes 注册自研 Agent 调用的日志采集执行面路由。
//
// 这些接口不走网页登录态，使用现有 Agent heartbeat token 鉴权。
func RegisterLogtailAgentPublicRoutes(router *gin.RouterGroup) {
	ctrl := logtailcontroller.NewLogtailController()

	group := router.Group("/logtail/agent")
	group.GET("/config", ctrl.GetRuntimeConfig)                                // Agent 拉取 Vector 配置
	group.GET("/operations", ctrl.ClaimRuntimeOperations)                      // Agent 拉取待执行操作
	group.POST("/status", ctrl.ReportRuntimeStatus)                            // Agent 上报 Vector 状态
	group.POST("/operations/:operationId/report", ctrl.ReportRuntimeOperation) // Agent 上报操作结果
}
