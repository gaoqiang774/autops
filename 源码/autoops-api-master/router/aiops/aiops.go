package aiops

import (
	"dodevops-api/api/aiops/controller"
	"dodevops-api/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterAiopsRoutes registers AIOps APIs.
func RegisterAiopsRoutes(router *gin.RouterGroup) {
	// MCP capability catalog and controlled action management.
	mcpManage := middleware.RequirePermission("aiops:mcp:manage")
	router.GET("/aiops/mcp/admin/servers", mcpManage, controller.ListMCPServers)
	router.PUT("/aiops/mcp/admin/servers/:code/status", mcpManage, controller.UpdateMCPServerStatus)
	router.PUT("/aiops/mcp/admin/servers/:code/tools/:toolName/config", mcpManage, controller.UpdateMCPToolConfig)
	router.POST("/aiops/mcp/actions/cancel", controller.CancelMCPAction)
	router.POST("/aiops/mcp/actions/cleanup-logs/confirm", controller.ConfirmMCPLogCleanup)
	router.POST("/aiops/mcp/actions/k8s/confirm", middleware.RequireK8SAction(middleware.K8SActionSensitive), controller.ConfirmMCPK8sAction)
	router.GET("/aiops/mcp/actions/k8s/rollout-status", middleware.RequireK8SAction(middleware.K8SActionSensitive), controller.GetMCPK8sRolloutStatus)

	// LLM Provider 管理
	router.GET("/aiops/llm/providers/types", controller.GetLLMProviderTypes)       // 获取 LLM 供应商类型
	router.GET("/aiops/llm/providers/enabled", controller.GetEnabledLLMProviders)  // 获取已启用的 LLM 供应商
	router.GET("/aiops/llm/providers/list", controller.ListLLMProviders)           // 获取 LLM 供应商列表
	router.GET("/aiops/llm/providers", controller.GetLLMProviderByID)              // 获取 LLM 供应商详情
	router.POST("/aiops/llm/providers", controller.CreateLLMProvider)              // 创建 LLM 供应商
	router.PUT("/aiops/llm/providers", controller.UpdateLLMProvider)               // 更新 LLM 供应商
	router.DELETE("/aiops/llm/providers", controller.DeleteLLMProvider)            // 删除 LLM 供应商
	router.POST("/aiops/llm/providers/default", controller.SetDefaultLLMProvider)  // 设置默认 LLM 供应商
	router.POST("/aiops/llm/providers/test", controller.TestLLMProviderConnection) // 测试 LLM 供应商连接
	router.POST("/aiops/llm/providers/secret", controller.GetLLMProviderSecret)    // 获取 LLM 供应商密钥

	// 日志分析
	router.GET("/aiops/log/host/files", controller.ListHostLogFiles)                                  // 获取主机日志文件列表
	router.GET("/aiops/log/sessions", controller.ListLogSessions)                                     // 获取日志分析会话列表
	router.GET("/aiops/log/session/status", controller.GetLogSessionStatus)                           // 获取日志分析会话状态
	router.POST("/aiops/log/host/query", controller.QueryHostLogs)                                    // 查询主机日志
	router.GET("/aiops/log/host/stream", controller.StreamHostLogs)                                   // 实时追踪主机日志
	router.POST("/aiops/log/k8s/query", controller.QueryK8sLogs)                                      // 查询 K8s Pod 日志
	router.GET("/aiops/log/k8s/stream", controller.StreamK8sLogs)                                     // 实时追踪 K8s Pod 日志
	router.POST("/aiops/log/k8s/pod/analyze", controller.AnalyzeK8sPod)                               // AI 分析 K8s Pod 状态、事件和日志
	router.POST("/aiops/k8s/ingress/analyze", controller.AnalyzeK8sIngress)                           // AI 分析 K8s Ingress 域名、Service、Endpoint 和后端 Pod
	router.GET("/aiops/k8s/ingress/analyze", controller.AnalyzeK8sIngress)                            // GET 方式 AI 分析 K8s Ingress 域名链路
	router.GET("/aiops/k8s/ingress/analyze/stream", controller.StreamK8sIngressAnalyze)               // SSE 返回 K8s Ingress AI 分析进度
	router.POST("/aiops/log/k8s/pod/analyze/tasks", controller.StartK8sPodAnalyzeTask)                // 创建 K8s Pod 异步分析任务
	router.GET("/aiops/log/k8s/pod/analyze/tasks/:taskId", controller.GetK8sPodAnalyzeTask)           // 查询 K8s Pod 异步分析任务
	router.GET("/aiops/log/k8s/pod/analyze/tasks/:taskId/stream", controller.StreamK8sPodAnalyzeTask) // SSE 返回 K8s Pod 分析进度
	router.POST("/aiops/log/analyze", controller.AnalyzeLogs)                                         // AI 分析日志
	router.POST("/aiops/database/mysql/analyze", controller.AnalyzeMySQL)                             // MySQL 故障证据采集与诊断分析

	// 平台内部 AI 助手
	router.POST("/aiops/assistant/query", controller.QueryAssistant)                     // 自然语言查询资源选址、资源回收、容量巡检、资产信息
	router.POST("/aiops/assistant/chat", controller.ChatAssistant)                       // 通用内部 AI 助手统一入口，自动编排只读工具
	router.DELETE("/aiops/assistant/chat/history", controller.ClearAssistantChatHistory) // 清理通用助手历史数据

	// 声明式 Skill 目录
	router.GET("/aiops/skills/categories", controller.ListSkillCategories) // 获取 Skill 分类统计
	router.GET("/aiops/skills", controller.ListSkills)                     // 获取已加载 Skill 列表
	router.GET("/aiops/skills/detail/:name", controller.GetSkill)          // 获取 Skill 详情
	router.POST("/aiops/skills/reload", controller.ReloadSkills)           // 重新加载 skills 目录
	router.POST("/aiops/skills/validate", controller.ValidateSkill)        // 校验 SKILL.md 内容
	router.POST("/aiops/skills/execute", controller.ExecuteSkill)          // 通过 Skill Runtime 执行声明式 Skill

	// 运维知识库
	router.GET("/aiops/knowledge/groups/tree", controller.ListKnowledgeGroupTree)                                    // 查询独立知识分组树
	router.POST("/aiops/knowledge/groups", controller.CreateKnowledgeGroup)                                          // 创建知识分组
	router.PUT("/aiops/knowledge/groups/:id", controller.UpdateKnowledgeGroup)                                       // 重命名或移动知识分组
	router.DELETE("/aiops/knowledge/groups/:id", controller.DeleteKnowledgeGroup)                                    // 级联删除知识分组、子分组及文档
	router.POST("/aiops/knowledge/documents", controller.CreateKnowledgeDocument)                                    // 创建 Markdown 运维知识库文档
	router.POST("/aiops/knowledge/documents/import", controller.ImportKnowledgeDocument)                             // 上传导入普通文档并转换为 Markdown
	router.GET("/aiops/knowledge/documents", controller.ListKnowledgeDocuments)                                      // 查询运维知识库文档列表
	router.GET("/aiops/knowledge/documents/:id", controller.GetKnowledgeDocument)                                    // 查询运维知识库文档详情
	router.PUT("/aiops/knowledge/documents/:id", controller.UpdateKnowledgeDocument)                                 // 编辑运维知识库 Markdown 文档
	router.DELETE("/aiops/knowledge/documents/:id", controller.DeleteKnowledgeDocument)                              // 删除运维知识库文档
	router.POST("/aiops/knowledge/documents/:id/summarize", controller.SummarizeKnowledgeDocument)                   // AI 总结 Markdown 知识库文档
	router.POST("/aiops/knowledge/documents/:id/search-data/refresh", controller.RefreshKnowledgeDocumentSearchData) // 刷新 MySQL 检索分块和关键词
	router.POST("/aiops/knowledge/search", controller.SearchKnowledgeDocuments)                                      // 检索运维知识库文档

	// 告警诊断与 Agent Tool Gateway
	router.POST("/aiops/agent/alerts/analyze", controller.AnalyzeAlert)   // 根据告警补齐证据并生成诊断报告
	router.POST("/aiops/agent/feedback", controller.CreateAgentFeedback)  // 提交 Agent 回答反馈，沉淀 Bad Case
	router.GET("/aiops/agent/runs/:id", controller.GetAgentRun)           // 查询 Agent 执行详情
	router.GET("/aiops/agent/runs/:id/stream", controller.StreamAgentRun) // SSE 返回 Agent 执行轨迹
	router.GET("/aiops/agent/tools", controller.ListAgentTools)           // 获取 Agent 只读工具白名单
	router.POST("/aiops/agent/tools/invoke", controller.InvokeAgentTool)  // 通过 Tool Gateway 调用受控工具
}
