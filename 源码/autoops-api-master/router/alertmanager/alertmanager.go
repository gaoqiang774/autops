package alertmanager

import (
	"dodevops-api/api/alertmanager/controller"

	"github.com/gin-gonic/gin"
)

// RegisterAlertManagerRoutes registers alert engine management APIs.
func RegisterAlertManagerRoutes(router *gin.RouterGroup) {
	group := router.Group("/alertmanager")
	{
		ruleGroup := group.Group("/rule-groups")
		{
			ruleGroup.POST("", controller.CreateRuleGroup)       // 创建告警规则分组
			ruleGroup.GET("/all/tree", controller.AllRuleGroups) // 获取告警规则分组树
			ruleGroup.PUT("/:id", controller.UpdateRuleGroup)    // 更新告警规则分组
			ruleGroup.DELETE("/:id", controller.DeleteRuleGroup) // 删除告警规则分组
			ruleGroup.GET("/:id", controller.GetRuleGroup)       // 获取告警规则分组详情
			ruleGroup.GET("", controller.ListRuleGroups)         // 获取告警规则分组列表
		}

		rules := group.Group("/rules")
		{
			rules.POST("", controller.CreateRule)                                    // 创建告警规则
			rules.POST("/preview-query", controller.PreviewRuleQuery)                // 预览 PromQL 查询结果
			rules.POST("/import-prometheus", controller.ImportPrometheusRules)       // 导入 Prometheus Rule YAML
			rules.POST("/batch-datasource", controller.BatchUpdateRuleDatasource)    // 批量修改规则数据源
			rules.POST("/batch-notify-rules", controller.BatchUpdateRuleNotifyRules) // 批量修改规则告警策略
			rules.PUT("/:id", controller.UpdateRule)                                 // 更新告警规则
			rules.DELETE("/:id", controller.DeleteRule)                              // 删除告警规则
			rules.GET("/:id", controller.GetRule)                                    // 获取告警规则详情
			rules.GET("", controller.ListRules)                                      // 获取告警规则列表
			rules.PUT("/:id/status", controller.ChangeRuleStatus)                    // 启用或禁用告警规则
		}

		datasources := group.Group("/datasources")
		{
			datasources.POST("", controller.CreateDatasource)                 // 创建告警数据源
			datasources.PUT("/:id", controller.UpdateDatasource)              // 更新告警数据源
			datasources.DELETE("/:id", controller.DeleteDatasource)           // 删除告警数据源
			datasources.GET("/types", controller.ListDatasourceTypes)         // 获取告警数据源类型
			datasources.GET("/options", controller.ListDatasourceOptions)     // 获取告警规则可选数据源
			datasources.GET("/:id", controller.GetDatasource)                 // 获取告警数据源详情
			datasources.GET("", controller.ListDatasources)                   // 获取告警数据源列表
			datasources.PUT("/:id/status", controller.ChangeDatasourceStatus) // 启用或禁用告警数据源
		}

		metrics := group.Group("/metrics")
		{
			metrics.GET("/query_range", controller.QueryMetricsRange) // Prometheus 范围查询
			metrics.GET("/query", controller.QueryMetrics)            // Prometheus 即时查询
			metrics.GET("/targets", controller.GetMetricTargets)      // 查询交换机/网络设备指标目标
		}

		domains := group.Group("/domains")
		{
			domains.POST("", controller.CreateDomain)                 // 新增域名监控
			domains.POST("/check", controller.CheckDomains)           // 批量检查域名
			domains.POST("/:id/check", controller.CheckDomain)        // 立即检查域名
			domains.PUT("/:id/status", controller.ChangeDomainStatus) // 启用或禁用域名监控
			domains.PUT("/:id", controller.UpdateDomain)              // 更新域名监控
			domains.DELETE("/:id", controller.DeleteDomain)           // 删除域名监控
			domains.GET("/:id", controller.GetDomain)                 // 获取域名监控详情
			domains.GET("", controller.ListDomains)                   // 获取域名监控列表
		}

		sslCertificates := group.Group("/ssl-certificates")
		{
			sslCertificates.POST("/apply", controller.ApplySSLCertificate)                        // 申请 ACME SSL 证书
			sslCertificates.POST("/:id/deploy", controller.DeploySSLCertificate)                  // 更新目标主机 SSL 证书
			sslCertificates.GET("/:id/deploy-logs/:logId", controller.GetSSLCertificateDeployLog) // 获取部署日志详情
			sslCertificates.GET("/:id/deploy-logs", controller.ListSSLCertificateDeployLogs)      // 查询部署日志
			sslCertificates.GET("/:id/download", controller.DownloadSSLCertificate)               // 下载 SSL 证书
			sslCertificates.GET("/:id", controller.GetSSLCertificate)                             // 获取 SSL 证书详情
			sslCertificates.DELETE("/:id", controller.DeleteSSLCertificate)                       // 删除 SSL 证书记录
			sslCertificates.GET("", controller.ListSSLCertificates)                               // 查询 SSL 证书申请记录
		}

		builtin := group.Group("/builtin-rules")
		{
			builtin.POST("", controller.CreateBuiltinRuleTemplate)                                // 创建内置告警规则模板
			builtin.GET("/import-prometheus/preview", controller.PreviewPrometheusTemplates)      // 预览远程 Prometheus 告警模板
			builtin.GET("/import-prometheus/types", controller.ListPrometheusTemplateImportTypes) // 获取 Prometheus 模板导入类型
			builtin.POST("/import-prometheus", controller.ImportPrometheusTemplates)              // 导入远程 Prometheus 告警模板
			builtin.PUT("/:id", controller.UpdateBuiltinRuleTemplate)                             // 更新内置告警规则模板
			builtin.POST("/:id/apply", controller.ApplyBuiltinRuleTemplate)                       // 应用模板创建告警规则
			builtin.GET("/:id", controller.GetBuiltinRuleTemplate)                                // 获取内置告警规则模板详情
			builtin.GET("", controller.ListBuiltinRuleTemplates)                                  // 获取内置告警规则模板列表
		}

		builtinGroups := group.Group("/builtin-rule-groups")
		{
			builtinGroups.GET("/tree", controller.AllBuiltinRuleTemplateGroupTree)        // 获取告警模板三级分组树
			builtinGroups.GET("/options", controller.ListBuiltinRuleTemplateGroupOptions) // 获取告警模板分组级联选项
			builtinGroups.POST("", controller.CreateBuiltinRuleTemplateGroup)             // 创建告警模板分组
			builtinGroups.PUT("/:id", controller.UpdateBuiltinRuleTemplateGroup)          // 更新告警模板分组
			builtinGroups.DELETE("/:id", controller.DeleteBuiltinRuleTemplateGroup)       // 删除告警模板分组
			builtinGroups.GET("/:id", controller.GetBuiltinRuleTemplateGroup)             // 获取告警模板分组详情
			builtinGroups.GET("", controller.ListBuiltinRuleTemplateGroups)               // 获取告警模板分组列表
		}

		activeEvents := group.Group("/active-events")
		{
			activeEvents.GET("", controller.ListActiveEvents)                                // 获取活跃告警事件列表
			activeEvents.GET("/business-options", controller.ListActiveEventBusinessOptions) // 获取活跃告警业务选项
			activeEvents.GET("/groups", controller.ListActiveEventGroups)                    // 获取活跃告警聚合列表
			activeEvents.POST("/:eventId/claim", controller.ClaimActiveEvent)                // 认领活跃告警事件
			activeEvents.POST("/:eventId/silence", controller.SilenceActiveEvent)            // 静默活跃告警事件
			activeEvents.DELETE("/:eventId/silence", controller.UnsilenceActiveEvent)        // 取消活跃告警事件静默
			activeEvents.GET("/:eventId/timeline", controller.ListTimeline)                  // 获取告警事件时间线
		}

		dashboard := group.Group("/dashboard")
		{
			dashboard.GET("/charts", controller.GetDashboardCharts)                        // 获取告警仪表盘图表数据
			dashboard.GET("/screen", controller.GetDashboardScreen)                        // 获取告警大屏当前活跃态数据
			dashboard.GET("/top-alert-names", controller.GetDashboardAlertNameTop)         // 获取告警名称 Top
			dashboard.GET("/notify-trend", controller.GetDashboardNotifyTrend)             // 获取告警通知趋势
			dashboard.GET("/top-business-routes", controller.GetDashboardBusinessRouteTop) // 获取告警业务路由 Top
			dashboard.GET("/top-rule-groups", controller.GetDashboardRuleGroupTop)         // 兼容旧路径，返回告警业务路由 Top
			dashboard.GET("/top-host-groups", controller.GetDashboardHostGroupTop)         // 兼容旧路径，返回告警业务路由 Top
		}

		historyEvents := group.Group("/history-events")
		{
			historyEvents.GET("", controller.ListHistoryEvents) // 获取历史告警事件列表
		}

		events := group.Group("/events")
		{
			events.GET("/:eventType/:eventId", controller.GetEventDetail) // 获取告警事件统一详情
		}

		silenceRules := group.Group("/silence-rules")
		{
			silenceRules.POST("", controller.CreateSilenceRule)       // 创建告警屏蔽规则
			silenceRules.PUT("/:id", controller.UpdateSilenceRule)    // 更新告警屏蔽规则
			silenceRules.DELETE("/:id", controller.DeleteSilenceRule) // 删除告警屏蔽规则
			silenceRules.GET("/:id", controller.GetSilenceRule)       // 获取告警屏蔽规则详情
			silenceRules.GET("", controller.ListSilenceRules)         // 获取告警屏蔽规则列表
		}

		notifyChannels := group.Group("/notify-channels")
		{
			notifyChannels.POST("", controller.CreateNotifyChannel)        // 创建告警通知媒介
			notifyChannels.PUT("/:id", controller.UpdateNotifyChannel)     // 更新告警通知媒介
			notifyChannels.DELETE("/:id", controller.DeleteNotifyChannel)  // 删除告警通知媒介
			notifyChannels.POST("/:id/test", controller.TestNotifyChannel) // 测试告警通知媒介
			notifyChannels.GET("/:id", controller.GetNotifyChannel)        // 获取告警通知媒介详情
			notifyChannels.GET("", controller.ListNotifyChannels)          // 获取告警通知媒介列表
		}

		notifyGroups := group.Group("/notify-groups")
		{
			notifyGroups.POST("", controller.CreateNotifyGroup)       // 创建告警通知群组
			notifyGroups.PUT("/:id", controller.UpdateNotifyGroup)    // 更新告警通知群组
			notifyGroups.DELETE("/:id", controller.DeleteNotifyGroup) // 删除告警通知群组
			notifyGroups.GET("/:id", controller.GetNotifyGroup)       // 获取告警通知群组详情
			notifyGroups.GET("", controller.ListNotifyGroups)         // 获取告警通知群组列表
		}

		notifyRouteRules := group.Group("/notify-route-rules")
		{
			notifyRouteRules.POST("", controller.CreateNotifyRouteRule)
			notifyRouteRules.PUT("/:id", controller.UpdateNotifyRouteRule)
			notifyRouteRules.DELETE("/:id", controller.DeleteNotifyRouteRule)
			notifyRouteRules.GET("/:id", controller.GetNotifyRouteRule)
			notifyRouteRules.GET("", controller.ListNotifyRouteRules)
		}

		notifyTemplates := group.Group("/notify-templates")
		{
			notifyTemplates.POST("", controller.CreateNotifyTemplate)              // 创建告警通知模板
			notifyTemplates.PUT("/:id", controller.UpdateNotifyTemplate)           // 更新告警通知模板
			notifyTemplates.DELETE("/:id", controller.DeleteNotifyTemplate)        // 删除告警通知模板
			notifyTemplates.POST("/:id/preview", controller.PreviewNotifyTemplate) // 预览告警通知模板
			notifyTemplates.GET("/:id", controller.GetNotifyTemplate)              // 获取告警通知模板详情
			notifyTemplates.GET("", controller.ListNotifyTemplates)                // 获取告警通知模板列表
		}

		notifyRules := group.Group("/notify-rules")
		{
			notifyRules.POST("", controller.CreateNotifyRule)       // 创建告警通知规则
			notifyRules.PUT("/:id", controller.UpdateNotifyRule)    // 更新告警通知规则
			notifyRules.DELETE("/:id", controller.DeleteNotifyRule) // 删除告警通知规则
			notifyRules.GET("/:id", controller.GetNotifyRule)       // 获取告警通知规则详情
			notifyRules.GET("", controller.ListNotifyRules)         // 获取告警通知规则列表
		}

		notifyRecords := group.Group("/notify-records")
		{
			notifyRecords.GET("/stats", controller.GetNotifyRecordStats) // 获取告警通知记录统计
			notifyRecords.GET("", controller.ListNotifyRecords)          // 获取告警通知记录列表
		}
	}
}
