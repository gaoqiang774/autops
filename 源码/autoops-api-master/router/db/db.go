package db

import (
	"dodevops-api/api/db/controller"

	"github.com/gin-gonic/gin"
)

func RegisterDbRoutes(router *gin.RouterGroup) {
	dbCtrl := controller.NewDatabaseController()
	reviewCtrl := controller.NewReviewController()

	router.GET("/db/assets", dbCtrl.ListAssets)                                             // 获取数据库资产列表
	router.POST("/db/assets/:id/ping", dbCtrl.Ping)                                         // 测试数据库连接
	router.GET("/db/assets/:id/monitor/mysql", dbCtrl.MySQLMonitor)                         // 获取MySQL监控大盘
	router.GET("/db/assets/:id/monitor/postgresql", dbCtrl.PostgreSQLMonitor)               // 获取PostgreSQL监控大盘
	router.GET("/db/assets/:id/monitor/redis", dbCtrl.RedisMonitor)                         // 获取Redis监控大盘
	router.GET("/db/assets/:id/slow-log", dbCtrl.SlowLogAnalysis)                           // 获取慢日志聚合分析
	router.POST("/db/assets/:id/slow-log/tasks", dbCtrl.CreateSlowLogTask)                  // 提交慢SQL异步分析任务
	router.POST("/db/assets/:id/slow-log/upload", dbCtrl.UploadSlowLogFile)                 // 上传慢日志文件并异步分析
	router.GET("/db/assets/:id/slow-log/tasks", dbCtrl.ListSlowLogTasks)                    // 查询慢SQL历史分析任务
	router.DELETE("/db/assets/:id/slow-log/tasks", dbCtrl.ClearSlowLogTasks)                // 清空慢SQL历史分析任务
	router.GET("/db/assets/:id/slow-log/tasks/:taskId", dbCtrl.GetSlowLogTask)              // 查询慢SQL异步分析任务
	router.DELETE("/db/assets/:id/slow-log/tasks/:taskId", dbCtrl.DeleteSlowLogTask)        // 删除慢SQL历史分析任务
	router.GET("/db/assets/:id/slow-log/tasks/:taskId/result", dbCtrl.GetSlowLogTaskResult) // 查询慢SQL异步任务结果并排序
	router.GET("/db/assets/:id/slow-log/tasks/:taskId/export", dbCtrl.ExportSlowLogTaskCSV) // 导出慢SQL异步分析任务Top100
	router.GET("/db/assets/:id/slow-log/export", dbCtrl.ExportSlowLogCSV)                   // 导出慢日志聚合分析CSV
	router.GET("/db/tree", dbCtrl.Tree)                                                     // 获取数据库资源树
	router.GET("/db/objects/columns", dbCtrl.Columns)                                       // 获取数据库对象字段
	router.GET("/db/objects/indexes", dbCtrl.Indexes)                                       // 获取数据库对象索引
	router.GET("/db/objects/ddl", dbCtrl.DDL)                                               // 获取数据库对象DDL
	router.POST("/db/data/query", dbCtrl.Query)                                             // 查询表数据
	router.POST("/db/data/export", dbCtrl.ExportCSV)                                        // 导出表数据CSV
	router.POST("/db/data/create", dbCtrl.Create)                                           // 新增表数据
	router.POST("/db/data/update", dbCtrl.Update)                                           // 更新表数据
	router.POST("/db/data/delete", dbCtrl.Delete)                                           // 删除表数据
	router.POST("/db/sql/execute", dbCtrl.ExecuteSQL)                                       // 执行只读SQL
	router.GET("/db/sql/completion", dbCtrl.Completion)                                     // 获取SQL自动补全元数据
	router.GET("/db/sql/history", dbCtrl.SQLHistory)                                        // 获取SQL执行历史
	router.POST("/db/ai/sql-assistant", dbCtrl.SQLAssistant)                                // AI SQL助手
	router.GET("/db/ai/tools", dbCtrl.ListAITools)                                          // 获取数据库AI工具清单
	router.POST("/db/ai/tools/run", dbCtrl.RunAITool)                                       // 执行数据库AI工具
	router.POST("/db/ai/chat", dbCtrl.DBAIChat)                                             // 数据库运维AI-Agent助手
	router.GET("/db/audit/list", dbCtrl.ListAudits)                                         // 获取数据库操作审计列表
	router.GET("/db/audit/info", dbCtrl.AuditInfo)                                          // 获取数据库操作审计详情
	router.GET("/db/audit/:id", dbCtrl.AuditInfo)                                           // 获取数据库操作审计详情
	router.GET("/db/tools/status", reviewCtrl.ToolboxStatus)                                // 按数据库类型获取工具箱状态
	router.GET("/db/tools/my2sql/status", reviewCtrl.My2SQLStatus)                          // 获取my2sql工具状态
	router.GET("/db/tools/gh-ost/status", reviewCtrl.GhOstStatus)                           // 获取gh-ost工具状态
	router.GET("/db/tools/pgroll/status", reviewCtrl.PgRollStatus)                          // 获取pgroll工具状态
	router.GET("/db/tools/walminer/status", reviewCtrl.WalMinerStatus)                      // 获取WalMiner工具状态

	router.POST("/db/review/dml/check", reviewCtrl.CheckDML)                                       // 自动检测DML SQL
	router.POST("/db/review/dml/tickets", reviewCtrl.SubmitDML)                                    // 提交DML工单
	router.GET("/db/review/dml/tickets", reviewCtrl.ListDML)                                       // 获取DML工单列表
	router.POST("/db/review/ddl/check", reviewCtrl.CheckDDL)                                       // 自动检测DDL SQL
	router.POST("/db/review/ddl/tickets", reviewCtrl.SubmitDDL)                                    // 提交DDL工单
	router.GET("/db/review/ddl/tickets", reviewCtrl.ListDDL)                                       // 获取DDL工单列表
	router.POST("/db/review/query/check", reviewCtrl.CheckQuery)                                   // 自动检测查询SQL
	router.POST("/db/review/query/tickets", reviewCtrl.SubmitQuery)                                // 提交查询工单
	router.GET("/db/review/query/tickets", reviewCtrl.ListQuery)                                   // 获取查询工单列表
	router.GET("/db/review/tickets", reviewCtrl.ListTickets)                                       // 获取SQL审核工单列表
	router.GET("/db/review/tickets/:id", reviewCtrl.Detail)                                        // 获取SQL审核工单详情
	router.POST("/db/review/tickets/:id/approve", reviewCtrl.Approve)                              // 审批通过SQL审核工单
	router.POST("/db/review/tickets/:id/reject", reviewCtrl.Reject)                                // 驳回SQL审核工单
	router.POST("/db/review/tickets/:id/execute", reviewCtrl.Execute)                              // 执行SQL审核工单
	router.POST("/db/review/tickets/:id/cancel", reviewCtrl.Cancel)                                // 撤销SQL审核工单
	router.GET("/db/review/tickets/:id/logs", reviewCtrl.Logs)                                     // 获取SQL审核工单执行日志
	router.GET("/db/review/tickets/:id/rollback", reviewCtrl.Rollback)                             // 获取SQL审核工单回滚SQL
	router.POST("/db/review/tickets/:id/rollback/flashback", reviewCtrl.GenerateFlashbackRollback) // 生成binlog闪回回滚SQL
	router.POST("/db/review/tickets/:id/rollback/tickets", reviewCtrl.CreateRollbackTicket)        // 基于回滚SQL生成回滚工单
	router.POST("/db/review/tickets/:id/backup/cleanup", reviewCtrl.CleanupBackupTables)           // 一键清理工单备份表
	router.GET("/db/review/tickets/:id/result", reviewCtrl.QueryResult)                            // 获取查询工单结果
	router.GET("/db/review/tickets/:id/result/export", reviewCtrl.ExportResult)                    // 导出查询工单结果CSV
}
