package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	aiopsmodel "dodevops-api/api/aiops/model"
	"dodevops-api/common"

	"gorm.io/gorm"
)

var (
	ErrMCPServerDisabled  = errors.New("MCP_SERVER_DISABLED")
	ErrMCPToolDisabled    = errors.New("TOOL_DISABLED")
	ErrMCPToolUnsupported = errors.New("TOOL_UNSUPPORTED")
)

type mcpDomainDefinition struct {
	Code        string
	DisplayName string
	Description string
	Icon        string
}

type mcpToolSpec struct {
	Name        string
	DisplayName string
	Risk        string
	Description string
}

var mcpDomains = []mcpDomainDefinition{
	{Code: "ecs", DisplayName: "主机管理 MCP", Description: "主机资产、监控、服务、日志与故障诊断", Icon: "Monitor"},
	{Code: "k8s", DisplayName: "Kubernetes MCP", Description: "集群、节点、工作负载、Pod 与流量链路诊断", Icon: "Grid"},
	{Code: "database", DisplayName: "数据库 MCP", Description: "数据库资产、结构、数据、监控与性能诊断", Icon: "Coin"},
	{Code: "log", DisplayName: "日志分析 MCP", Description: "日志采集链路、检索、聚合与异常分析", Icon: "Document"},
	{Code: "alert", DisplayName: "告警分析 MCP", Description: "告警查询、关联、影响分析与处置申请", Icon: "Bell"},
	{Code: "knowledge", DisplayName: "运维知识库 MCP", Description: "知识检索、Runbook、已知问题与引用管理", Icon: "Collection"},
	{Code: "asset", DisplayName: "综合资产 MCP", Description: "跨域资产搜索、归属、拓扑、健康与生命周期", Icon: "Connection"},
}

var mcpToolSpecs = map[string][]mcpToolSpec{
	"ecs": {
		{"ecs_list_hosts", "主机列表与统计", "R0", "查询当前授权范围内的主机总数和主机列表；回答有多少主机、主机数量、主机资产概览时必须使用"},
		{"ecs_search_hosts", "主机搜索", "R0", "按 ID、IP、主机名、环境等条件搜索主机资产"},
		{"ecs_get_host", "主机详情", "R0", "查询主机资产、网络、系统、区域、分组和状态"},
		{"ecs_get_configuration", "配置详情", "R0", "查询 CPU、内存、磁盘、网卡、挂载点和系统版本"},
		{"ecs_get_agent_status", "Agent 状态", "R0", "检查 Agent 安装、版本、心跳、在线状态和采集能力"},
		{"ecs_check_monitoring", "监控检测", "R0", "检查监控目标、指标新鲜度、缺失指标和采集异常"},
		{"ecs_query_metrics", "指标查询", "R0", "查询 CPU、负载、内存、磁盘、网络和文件系统指标"},
		{"ecs_list_filesystems", "文件系统", "R0", "查询挂载点、容量、可用空间、使用率和文件系统类型"},
		{"ecs_inspect_path", "路径检查", "R1", "列出目录直接子项；普通文本文件最多返回最后 500 行并限制内容字节数"},
		{"ecs_analyze_disk_usage", "磁盘占用分析", "R1", "分析指定路径下占用空间最大的目录和文件"},
		{"ecs_list_processes", "进程列表", "R1", "查询 Top CPU、Top 内存、PID、启动时间和命令摘要"},
		{"ecs_list_services", "服务列表", "R1", "查询服务、运行状态、启动类型和应用线索"},
		{"ecs_list_ports", "监听端口", "R1", "查询端口、协议、监听地址、PID 和关联服务"},
		{"ecs_list_events", "主机事件", "R0", "查询宕机、重启、Agent 离线、监控中断和配置变化"},
		{"ecs_list_log_files", "日志文件", "R1", "查询允许目录中的 .log/.txt 文件、大小和修改时间"},
		{"ecs_check_service", "服务检测", "R1", "检查服务、端口、进程、健康接口和近期错误"},
		{"ecs_diagnose_host", "主机诊断", "R1", "聚合资产、Agent、指标、进程、服务和事件证据"},
		{"ecs_request_install_service", "安装服务", "R3", "申请通过已审核软件包或 Runbook 安装服务"},
		{"ecs_request_restart_service", "重启服务", "R3", "申请预检查并重启主机服务"},
		{"ecs_request_start_service", "启动服务", "R2", "申请启动主机服务"},
		{"ecs_request_stop_service", "停止服务", "R3", "申请停止主机服务"},
		{"ecs_request_cleanup_logs", "清空日志预览", "R3", "仅预览受控日志清空并返回目标文件、固定命令和确认凭证；保留文件且 MCP 不执行清空，必须由平台登录的系统管理员确认"},
		{"ecs_request_reboot", "主机重启", "R3", "申请检查业务冗余和维护窗口后重启主机"},
		{"ecs_request_resize", "主机扩容", "R3", "申请评估费用和停机影响后调整主机规格"},
		{"ecs_request_run_runbook", "执行 Runbook", "R3", "申请执行已发布且版本固定的 Runbook"},
	},
	"k8s": {
		{"k8s_list_clusters", "集群列表", "R0", "查询当前授权范围内的集群"},
		{"k8s_check_cluster", "集群检查", "R0", "检查 API、版本、节点、核心组件和资源压力"},
		{"k8s_list_namespaces", "Namespace 列表", "R0", "查询授权范围内的 Namespace"},
		{"k8s_list_nodes", "节点列表", "R0", "查询节点 Ready、压力、污点、容量和 Pod 分布"},
		{"k8s_get_node_status", "节点状态", "R0", "查询指定节点状态和资源压力"},
		{"k8s_search_resources", "资源搜索", "R0", "按 kind、名称、标签和 Namespace 搜索资源"},
		{"k8s_get_resource", "资源详情", "R1", "返回脱敏后的 spec、status、conditions 和 owner"},
		{"k8s_get_workload_status", "工作负载检查", "R0", "检查副本、发布状态、镜像、条件和异常 Pod"},
		{"k8s_get_workload_history", "工作负载历史版本", "R0", "查询 Deployment 历史 revision、镜像、创建时间和副本状态"},
		{"k8s_get_pod_status", "Pod 检查", "R0", "检查容器、重启、探针、退出码、调度和 OOM"},
		{"k8s_get_service_endpoints", "Service 链路", "R0", "查询 Service、Endpoint 和后端 Pod 端口映射"},
		{"k8s_query_events", "事件查询", "R0", "按对象和时间窗查询集群事件"},
		{"k8s_query_logs", "容器日志", "R1", "受限查询当前或 previous 容器日志"},
		{"k8s_query_metrics", "资源指标", "R0", "查询节点、Pod、容器及 Request/Limit 使用率"},
		{"k8s_diagnose_workload", "工作负载诊断", "R1", "聚合副本、事件、Pod、日志、指标和发布状态"},
		{"k8s_diagnose_pod", "Pod 诊断", "R1", "诊断 Pending、CrashLoop、ImagePull、探针和 OOM"},
		{"k8s_diagnose_ingress", "Ingress 诊断", "R1", "串联 Ingress、Service、Endpoint 和后端 Pod"},
		{"k8s_request_rollout_restart", "滚动重启", "R3", "申请检查副本、PDB 和发布策略后滚动重启"},
		{"k8s_request_scale_workload", "工作负载扩缩容", "R3", "申请在策略上下限内调整副本数"},
		{"k8s_request_rollback_workload", "发布回滚", "R3", "按 revision 或镜像版本匹配历史 revision，申请回滚工作负载"},
		{"k8s_request_recreate_pod", "重建 Pod", "R3", "申请重建控制器管理的 Pod"},
		{"k8s_request_rerun_job", "Job 重跑", "R2", "申请复制受控参数创建新 Job"},
		{"k8s_request_update_image", "更新镜像", "R3", "申请校验镜像和差异后更新工作负载"},
		{"k8s_request_apply_yaml", "Apply YAML", "R3", "申请通过 dry-run、策略校验和差异审批后应用 YAML"},
		{"k8s_request_cordon_node", "封锁节点", "R3", "申请将节点设为不可调度"},
		{"k8s_request_drain_node", "驱逐节点", "R4", "高危节点驱逐，默认禁止"},
	},
	"database": {
		{"db_search_assets", "资产搜索", "R0", "按类型、环境、地址、实例、业务和负责人搜索数据库"},
		{"db_get_instance", "实例详情", "R0", "查询实例属性、认证引用和监控配置，不返回密码"},
		{"db_test_connection", "连通性检测", "R1", "使用平台凭证引用执行受控连接测试"},
		{"db_list_databases", "数据库列表", "R1", "查询授权范围内的数据库"},
		{"db_search_objects", "对象搜索", "R1", "搜索库、表、视图、字段和索引"},
		{"db_get_table_schema", "表结构", "R1", "查询字段、主键、索引、大小和统计信息"},
		{"db_get_table_indexes", "索引信息", "R1", "查询索引字段、唯一性、基数和使用线索"},
		{"db_get_table_stats", "表统计", "R1", "查询行数估算、容量、更新时间和膨胀信息"},
		{"db_query_table", "表数据", "R1", "按结构化过滤、排序和分页查询脱敏数据"},
		{"db_execute_readonly_sql", "只读 SQL", "R1", "经 AST 校验、限行和超时后执行只读 SQL"},
		{"db_explain_sql", "执行计划", "R1", "分析扫描、索引、排序、临时表和估算行数风险"},
		{"db_get_instance_health", "实例健康", "R0", "查询连接、QPS/TPS、缓存、延迟和容量"},
		{"db_get_monitor_snapshot", "监控快照", "R0", "查询指定时间窗的核心指标和异常阈值"},
		{"db_get_slow_sql_top", "慢 SQL", "R1", "查询慢 SQL 频次、耗时、扫描行数和指纹"},
		{"db_get_sessions", "会话查询", "R1", "查询活跃会话、执行时间、等待状态和 SQL 指纹"},
		{"db_get_lock_waits", "锁等待", "R1", "查询阻塞链、锁类型、持续时间和相关会话"},
		{"db_get_replication_status", "复制状态", "R0", "查询复制延迟、错误和节点角色"},
		{"db_get_capacity", "容量分析", "R0", "查询库表容量、增长趋势和剩余空间"},
		{"db_diagnose_instance", "实例诊断", "R1", "聚合监控、慢 SQL、会话、锁和复制证据"},
		{"db_request_kill_session", "Kill 会话", "R3", "申请终止固定数据库会话"},
		{"db_request_create_index", "创建索引", "R3", "申请评估容量和锁风险后创建索引"},
		{"db_request_backup", "发起备份", "R3", "申请检查存储和备份窗口后发起备份"},
		{"db_request_execute_change", "SQL 变更", "R4", "仅允许创建 SQL 审核工单，禁止直接执行"},
		{"db_request_failover", "主从切换", "R4", "灾备专用高危流程，默认禁止"},
	},
	"log": {
		{"log_list_business_groups", "业务组", "R0", "查询日志业务组及归属信息"},
		{"log_list_projects", "项目列表", "R0", "查询日志平台项目"},
		{"log_list_stores", "日志库", "R0", "查询业务组下日志库、保留期、规模和状态"},
		{"log_list_sources", "日志源列表", "R0", "查询主机、K8s、服务等采集源"},
		{"log_get_source", "日志源详情", "R0", "查询采集源配置和字段"},
		{"log_check_collector", "采集检测", "R0", "检查 Agent、采集任务、心跳、积压和最近上报"},
		{"log_check_pipeline", "链路检测", "R0", "检查采集、解析、写入、存储和查询链路"},
		{"log_search", "日志检索", "R1", "按时间、服务、主机、Pod、级别、关键词和字段检索"},
		{"log_get_context", "日志上下文", "R1", "获取指定日志前后行或前后时间窗口"},
		{"log_aggregate", "字段聚合", "R1", "按级别、服务、主机、Pod 或错误码聚合"},
		{"log_search_by_trace", "Trace 检索", "R1", "按 traceId 或 requestId 串联日志"},
		{"log_detect_patterns", "异常模式", "R1", "聚类和去重错误日志，识别新增与突增模式"},
		{"log_compare_windows", "窗口对比", "R1", "比较当前和基线窗口的错误量与模式"},
		{"log_analyze", "日志分析", "R1", "输出异常摘要、代表样本、时间分布和相关资产"},
		{"log_get_retention", "保留策略", "R0", "查询保留期、冷热策略和预计过期时间"},
		{"log_request_restart_collector", "重启采集器", "R3", "申请评估日志源和高可用状态后重启采集器"},
		{"log_request_replay_collection", "重放采集", "R2", "申请在限制范围内重放采集数据"},
		{"log_request_update_collection", "修改采集", "R3", "申请审核采集路径、解析规则和目标库差异"},
		{"log_request_create_alert_rule", "创建告警规则", "R2", "申请预览查询、阈值和通知对象后创建规则"},
		{"log_request_export", "日志导出", "R3", "申请通过敏感扫描和审批后导出日志"},
	},
	"alert": {
		{"alert_list_active", "活跃告警", "R0", "按严重级别、来源、资产、业务和时间查询活跃告警"},
		{"alert_get_detail", "告警详情", "R0", "查询规则、标签、阈值、指纹和关联资产"},
		{"alert_get_timeline", "事件时间线", "R0", "查询发生、重复、恢复、通知和状态变化"},
		{"alert_get_history", "历史告警", "R0", "查询指定规则或资产的历史事件"},
		{"alert_find_similar", "相似告警", "R0", "按指纹、标签、资产和历史特征检索"},
		{"alert_correlate", "告警关联", "R0", "聚合同资产、业务、集群或时间窗告警"},
		{"alert_get_noise_summary", "降噪摘要", "R0", "查询重复、抑制、静默和聚合情况"},
		{"alert_analyze_impact", "影响分析", "R0", "基于资产关系分析受影响服务和负责人"},
		{"alert_recommend_runbook", "Runbook 推荐", "R0", "根据告警和资源上下文检索 Runbook"},
		{"alert_get_notification_status", "通知状态", "R0", "查询通知渠道、发送状态和失败原因"},
		{"alert_list_silences", "静默查询", "R0", "查询当前静默和维护窗口"},
		{"alert_request_acknowledge", "告警确认", "R2", "申请记录确认人、理由和时间"},
		{"alert_request_assign", "告警指派", "R2", "申请校验业务范围后指派处理人"},
		{"alert_request_comment", "添加评论", "R2", "申请以真实操作者身份添加告警评论"},
		{"alert_request_close", "关闭告警", "R3", "申请验证恢复状态和原因后关闭告警"},
		{"alert_request_silence", "临时静默", "R3", "申请按匹配条件、理由和过期时间静默"},
		{"alert_request_maintenance_window", "维护窗口", "R3", "申请设置资产范围、持续时间和通知策略"},
		{"alert_request_create_ticket", "创建工单", "R2", "申请创建并关联告警与诊断证据的工单"},
	},
	"knowledge": {
		{"knowledge_list_groups", "分组查询", "R0", "查询知识分组树和文档数量"},
		{"knowledge_list_categories", "分类查询", "R0", "查询文档类型、标签、服务和资产类型分类"},
		{"knowledge_search", "混合检索", "R0", "按权限执行关键词、全文和语义混合检索"},
		{"knowledge_get_document", "文档详情", "R1", "查询正文、摘要、版本、状态和适用范围"},
		{"knowledge_get_section", "章节读取", "R1", "返回命中章节和必要上下文"},
		{"knowledge_find_runbooks", "Runbook 查询", "R0", "按告警、服务、资产类型和故障现象检索"},
		{"knowledge_find_similar_incidents", "相似故障", "R0", "查询历史故障根因、处置和验证结果"},
		{"knowledge_find_known_issues", "已知问题", "R0", "按组件、版本和错误码检索已知问题"},
		{"knowledge_validate_applicability", "适用性校验", "R0", "校验环境、版本、前置条件和过期状态"},
		{"knowledge_get_citation", "引用校验", "R0", "返回固定文档 ID、版本、更新时间和位置"},
		{"knowledge_get_version", "版本查询", "R0", "查询知识文档版本历史和变更摘要"},
		{"knowledge_request_create_draft", "创建草稿", "R2", "申请保留来源证据后创建知识草稿"},
		{"knowledge_request_update_draft", "更新草稿", "R2", "申请通过乐观锁和差异检查更新草稿"},
		{"knowledge_request_publish", "发布文档", "R3", "申请通过内容审核和敏感扫描后发布"},
		{"knowledge_request_unpublish", "下线文档", "R3", "申请检查引用关系后下线文档"},
		{"knowledge_request_rebuild_index", "重建索引", "R2", "申请异步重建可回退的知识索引"},
		{"knowledge_submit_feedback", "提交反馈", "R1", "记录检索结果是否有用和实际处置结果"},
		{"aiops_skill_catalog", "Skill Catalog", "R0", "查询当前加载的声明式 Skill 目录"},
		{"aiops_skill_execute", "执行只读 Skill", "R1", "执行已启用且只读的声明式 Skill"},
	},
	"asset": {
		{"asset_search", "全局搜索", "R0", "跨主机、云、K8s、数据库、中间件和应用搜索"},
		{"asset_get", "资产详情", "R0", "查询统一 ID、类型、来源、状态和更新时间"},
		{"asset_get_ownership", "资产归属", "R0", "查询业务、应用、项目、环境、团队和负责人"},
		{"asset_get_relations", "资产关系", "R0", "按方向、深度和关系类型查询资产关系"},
		{"asset_get_topology", "业务拓扑", "R0", "查询受节点数和深度限制的业务拓扑"},
		{"asset_analyze_impact", "影响范围", "R0", "分析潜在受影响的服务、业务和负责人"},
		{"asset_get_health", "健康摘要", "R0", "查询各领域健康引用和数据时间"},
		{"asset_get_changes", "变更记录", "R0", "查询属性、关系和同步来源变化"},
		{"asset_get_data_quality", "数据质量", "R0", "查询缺失、重复、失联、陈旧和关系缺口"},
		{"asset_get_lifecycle", "生命周期", "R0", "查询创建、上线、到期、回收和归档状态"},
		{"asset_get_cost_summary", "成本摘要", "R1", "查询授权范围内成本和资源利用率摘要"},
		{"asset_request_update_ownership", "修改负责人", "R2", "申请校验业务权限后修改资产负责人"},
		{"asset_request_update_tags", "修改标签", "R2", "申请按受控标签字典修改资产标签"},
		{"asset_request_update_lifecycle", "修改生命周期", "R3", "申请通过审批修改资产生命周期"},
		{"asset_request_bind_relation", "绑定关系", "R3", "申请校验循环依赖和影响后绑定关系"},
		{"asset_request_merge_duplicates", "合并重复资产", "R3", "申请处理字段冲突和引用后合并资产"},
		{"asset_request_sync", "资产同步", "R3", "申请按指定数据源和范围同步资产"},
		{"aiops_chat", "AIOps 智能助手", "R1", "调用已接入 CMDB、K8s、数据库、告警和监控的内部智能助手完成实时查询与诊断；变更预览后，用户在同一 sessionId 或 conversationId 回复确认、OK 可执行待确认操作"},
		{"aiops_confirm_action", "确认待执行操作", "R3", "使用变更预览返回的签名 confirmationId 执行用户已确认的操作；目标和命令完全取自签名凭证，不能由调用方修改"},
	},
}

var mcpHandlerMap = map[string]string{
	"ecs_list_hosts":                "mcp_read",
	"ecs_search_hosts":              "search_cmdb_hosts",
	"ecs_get_host":                  "get_cmdb_host",
	"ecs_get_configuration":         "get_cmdb_host",
	"ecs_get_agent_status":          "get_agent_status",
	"ecs_check_monitoring":          "query_host_metrics",
	"ecs_query_metrics":             "query_host_metrics",
	"ecs_list_filesystems":          "mcp_host_read",
	"ecs_inspect_path":              "mcp_host_read",
	"ecs_analyze_disk_usage":        "mcp_host_read",
	"ecs_list_processes":            "query_host_metrics",
	"ecs_list_services":             "query_host_services",
	"ecs_list_log_files":            "mcp_host_read",
	"ecs_check_service":             "query_host_services",
	"ecs_diagnose_host":             "query_host_metrics",
	"ecs_request_restart_service":   "mcp_host_action",
	"ecs_request_start_service":     "mcp_host_action",
	"ecs_request_stop_service":      "mcp_host_action",
	"ecs_request_cleanup_logs":      "mcp_host_action",
	"k8s_list_clusters":             "aiops_context_k8s",
	"k8s_check_cluster":             "mcp_read",
	"k8s_list_namespaces":           "mcp_read",
	"k8s_list_nodes":                "mcp_read",
	"k8s_get_node_status":           "mcp_read",
	"k8s_search_resources":          "mcp_read",
	"k8s_get_resource":              "mcp_read",
	"k8s_get_workload_status":       "mcp_read",
	"k8s_get_workload_history":      "mcp_read",
	"k8s_get_pod_status":            "mcp_read",
	"k8s_get_service_endpoints":     "mcp_read",
	"k8s_query_events":              "mcp_read",
	"k8s_query_logs":                "mcp_read",
	"k8s_query_metrics":             "mcp_read",
	"k8s_diagnose_workload":         "mcp_read",
	"k8s_diagnose_pod":              "mcp_read",
	"k8s_diagnose_ingress":          "mcp_read",
	"k8s_request_rollout_restart":   "mcp_k8s_action",
	"k8s_request_scale_workload":    "mcp_k8s_action",
	"k8s_request_rollback_workload": "mcp_k8s_action",
	"k8s_request_update_image":      "mcp_k8s_action",
	"db_search_assets":              "mcp_read",
	"db_search_objects":             "mcp_read",
	"db_get_table_schema":           "mcp_read",
	"db_execute_readonly_sql":       "mcp_read",
	"db_explain_sql":                "mcp_read",
	"db_get_monitor_snapshot":       "mcp_read",
	"db_get_slow_sql_top":           "mcp_read",
	"db_get_instance_health":        "mcp_read",
	"log_list_business_groups":      "mcp_read",
	"log_list_projects":             "mcp_read",
	"log_list_stores":               "mcp_read",
	"log_check_pipeline":            "mcp_read",
	"log_search":                    "mcp_read",
	"log_aggregate":                 "mcp_read",
	"alert_list_active":             "mcp_read",
	"alert_get_detail":              "mcp_read",
	"alert_get_timeline":            "mcp_read",
	"alert_get_history":             "mcp_read",
	"alert_get_notification_status": "mcp_read",
	"alert_list_silences":           "mcp_read",
	"knowledge_list_groups":         "mcp_read",
	"knowledge_list_categories":     "mcp_read",
	"knowledge_search":              "knowledge_search",
	"knowledge_get_document":        "mcp_read",
	"knowledge_find_runbooks":       "mcp_read",
	"knowledge_find_known_issues":   "mcp_read",
	"knowledge_get_citation":        "mcp_read",
	"asset_search":                  "mcp_read",
	"aiops_chat":                    "aiops_chat",
	"aiops_confirm_action":          "aiops_confirm_action",
	"aiops_skill_catalog":           "aiops_skill_catalog",
	"aiops_skill_execute":           "aiops_skill_execute",
}

// agentGatewayMCPToolMap gives every legacy Agent Tool one canonical MCP
// policy. Internal callers may use the legacy handler name, but never bypass
// the MCP enablement policy attached to this canonical Tool.
var agentGatewayMCPToolMap = map[string]string{
	"get_cmdb_host":          "ecs_get_host",
	"search_cmdb_hosts":      "ecs_search_hosts",
	"get_agent_status":       "ecs_get_agent_status",
	"query_host_metrics":     "ecs_query_metrics",
	"query_host_services":    "ecs_list_services",
	"query_host_logs":        "log_search",
	"query_active_alerts":    "alert_list_active",
	"query_alert_detail":     "alert_get_detail",
	"aiops_context_hosts":    "asset_search",
	"aiops_context_database": "db_search_assets",
	"aiops_context_k8s":      "k8s_list_clusters",
	"knowledge_search":       "knowledge_search",
}

type MCPCatalogService struct {
	db *gorm.DB
}

func NewMCPCatalogService() *MCPCatalogService {
	return &MCPCatalogService{db: common.GetDB()}
}

func NewMCPCatalogServiceWithDB(db *gorm.DB) *MCPCatalogService {
	return &MCPCatalogService{db: db}
}

func (s *MCPCatalogService) ListServers(domain string) ([]aiopsmodel.MCPServerView, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain != "" && !isMCPDomain(domain) {
		return nil, fmt.Errorf("unknown MCP domain: %s", domain)
	}

	serverOverrides := make(map[string]aiopsmodel.MCPServerConfig)
	toolOverrides := make(map[string]aiopsmodel.MCPToolConfig)
	if s.db != nil {
		var servers []aiopsmodel.MCPServerConfig
		if err := s.db.Find(&servers).Error; err != nil {
			return nil, err
		}
		for _, item := range servers {
			serverOverrides[item.Code] = item
		}
		var tools []aiopsmodel.MCPToolConfig
		query := s.db
		if domain != "" {
			query = query.Where("server_code = ?", domain)
		}
		if err := query.Find(&tools).Error; err != nil {
			return nil, err
		}
		for _, item := range tools {
			toolOverrides[mcpToolConfigKey(item.ServerCode, item.ToolName)] = item
		}
	}
	return buildMCPServerViews(domain, serverOverrides, toolOverrides), nil
}

func buildMCPServerViews(domain string, serverOverrides map[string]aiopsmodel.MCPServerConfig, toolOverrides map[string]aiopsmodel.MCPToolConfig) []aiopsmodel.MCPServerView {
	definitions := mcpCatalogDefinitions()
	grouped := make(map[string][]aiopsmodel.MCPToolDefinition)
	for _, definition := range definitions {
		grouped[definition.Domain] = append(grouped[definition.Domain], definition)
	}

	result := make([]aiopsmodel.MCPServerView, 0, len(mcpDomains))
	for _, definition := range mcpDomains {
		if domain != "" && definition.Code != domain {
			continue
		}
		server := aiopsmodel.MCPServerView{
			Code: definition.Code, DisplayName: definition.DisplayName,
			Description: definition.Description, Icon: definition.Icon,
			Enabled: true, Version: "1.0.0", ConfigVersion: 1,
			Tools: make([]aiopsmodel.MCPToolView, 0, len(grouped[definition.Code])),
		}
		if override, ok := serverOverrides[definition.Code]; ok {
			server.Enabled = override.Enabled
			server.ConfigVersion = override.ConfigVersion
		}
		for _, tool := range grouped[definition.Code] {
			view := defaultMCPToolView(tool)
			if override, ok := toolOverrides[mcpToolConfigKey(tool.Domain, tool.Name)]; ok {
				applyMCPToolOverride(&view, override)
			}
			if !view.Supported || view.EffectiveRisk == aiopsmodel.MCPRiskR4 {
				view.Enabled = false
			}
			server.TotalTools++
			if view.Supported {
				server.SupportedTools++
			}
			if server.Enabled && view.Enabled {
				server.EnabledTools++
			}
			server.Tools = append(server.Tools, view)
		}
		result = append(result, server)
	}
	return result
}

func (s *MCPCatalogService) ListCallableTools(domain string) ([]aiopsmodel.MCPToolView, error) {
	servers, err := s.ListServers(domain)
	if err != nil {
		return nil, err
	}
	result := make([]aiopsmodel.MCPToolView, 0)
	for _, server := range servers {
		if !server.Enabled {
			continue
		}
		for _, tool := range server.Tools {
			if tool.Supported && tool.Enabled {
				result = append(result, tool)
			}
		}
	}
	return result, nil
}

func (s *MCPCatalogService) AuthorizeTool(domain, name string) (aiopsmodel.MCPToolView, error) {
	name = strings.TrimSpace(name)
	servers, err := s.ListServers(domain)
	if err != nil {
		return aiopsmodel.MCPToolView{}, err
	}
	return authorizeMCPTool(servers, name)
}

func (s *MCPCatalogService) AuthorizeGatewayTool(handlerKey, explicitMCPTool string) (aiopsmodel.MCPToolView, error) {
	servers, err := s.ListServers("")
	if err != nil {
		return aiopsmodel.MCPToolView{}, err
	}
	return authorizeGatewayMCPTool(servers, handlerKey, explicitMCPTool)
}

func authorizeGatewayMCPTool(servers []aiopsmodel.MCPServerView, handlerKey, explicitMCPTool string) (aiopsmodel.MCPToolView, error) {
	handlerKey = strings.TrimSpace(handlerKey)
	toolName := strings.TrimSpace(explicitMCPTool)
	explicit := toolName != ""
	if toolName == "" {
		toolName = agentGatewayMCPToolMap[handlerKey]
	}
	if toolName == "" {
		return aiopsmodel.MCPToolView{}, fmt.Errorf("%w: Agent Tool %s has no MCP policy mapping", ErrMCPToolUnsupported, handlerKey)
	}
	for _, server := range servers {
		for _, tool := range server.Tools {
			if tool.Name != toolName {
				continue
			}
			if explicit && tool.HandlerKey != handlerKey {
				return tool, fmt.Errorf("MCP tool %s is not backed by handler %s", toolName, handlerKey)
			}
			return authorizeMCPTool(servers, toolName)
		}
	}
	return aiopsmodel.MCPToolView{}, fmt.Errorf("unknown MCP tool: %s", toolName)
}

func authorizeMCPTool(servers []aiopsmodel.MCPServerView, name string) (aiopsmodel.MCPToolView, error) {
	for _, server := range servers {
		for _, tool := range server.Tools {
			if tool.Name != name {
				continue
			}
			if !server.Enabled {
				return tool, ErrMCPServerDisabled
			}
			if !tool.Supported {
				return tool, ErrMCPToolUnsupported
			}
			if !tool.Enabled {
				return tool, ErrMCPToolDisabled
			}
			return tool, nil
		}
	}
	return aiopsmodel.MCPToolView{}, fmt.Errorf("unknown tool: %s", name)
}

func (s *MCPCatalogService) UpdateServer(code string, enabled bool, operator uint) (*aiopsmodel.MCPServerView, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	if !isMCPDomain(code) {
		return nil, fmt.Errorf("unknown MCP domain: %s", code)
	}
	var config aiopsmodel.MCPServerConfig
	err := s.db.Where("code = ?", code).First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		config = aiopsmodel.MCPServerConfig{Code: code, Enabled: enabled, ConfigVersion: 1, CreatedBy: operator, UpdatedBy: operator}
		if err := s.db.Create(&config).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		config.Enabled = enabled
		config.UpdatedBy = operator
		config.ConfigVersion++
		if err := s.db.Save(&config).Error; err != nil {
			return nil, err
		}
	}
	servers, err := s.ListServers(code)
	if err != nil || len(servers) == 0 {
		return nil, err
	}
	return &servers[0], nil
}

func (s *MCPCatalogService) UpdateTool(code, name string, req aiopsmodel.MCPToolConfigRequest, operator uint) (*aiopsmodel.MCPToolView, error) {
	definition, ok := findMCPToolDefinition(code, name)
	if !ok {
		return nil, fmt.Errorf("unknown tool %s in MCP %s", name, code)
	}
	if req.Enabled != nil && *req.Enabled && !definition.Supported {
		return nil, fmt.Errorf("%w: tool %s has no executable handler in this version", ErrMCPToolUnsupported, name)
	}
	if req.Enabled != nil && *req.Enabled && definition.RiskFloor == aiopsmodel.MCPRiskR4 {
		return nil, fmt.Errorf("R4 tools cannot be enabled")
	}
	if req.RiskLevel != nil {
		risk := normalizeRisk(*req.RiskLevel)
		if risk == "" {
			return nil, fmt.Errorf("riskLevel must be R0, R1, R2, R3 or R4")
		}
		if riskRank(risk) < riskRank(definition.RiskFloor) {
			return nil, fmt.Errorf("riskLevel cannot be lower than code-defined floor %s", definition.RiskFloor)
		}
	}
	if err := validateMCPToolLimits(req); err != nil {
		return nil, err
	}

	var config aiopsmodel.MCPToolConfig
	err := s.db.Where("server_code = ? AND tool_name = ?", definition.Domain, definition.Name).First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		view := defaultMCPToolView(definition)
		config = aiopsmodel.MCPToolConfig{
			ServerCode: definition.Domain, ToolName: definition.Name,
			Enabled: view.Enabled, RiskLevel: view.EffectiveRisk,
			RequiredConfirmation: view.RequiredConfirmation,
			TimeoutSeconds:       view.TimeoutSeconds, RateLimitPerMinute: view.RateLimitPerMinute,
			MaxResultRows: view.MaxResultRows, MaxResultBytes: view.MaxResultBytes,
			ConfigVersion: 1, CreatedBy: operator, UpdatedBy: operator,
		}
	} else if err != nil {
		return nil, err
	} else {
		config.ConfigVersion++
		config.UpdatedBy = operator
	}
	applyMCPToolRequest(&config, req)
	if riskRank(config.RiskLevel) >= riskRank(aiopsmodel.MCPRiskR2) {
		config.RequiredConfirmation = true
	}
	if config.ID == 0 {
		err = s.db.Create(&config).Error
	} else {
		err = s.db.Save(&config).Error
	}
	if err != nil {
		return nil, err
	}

	servers, err := s.ListServers(definition.Domain)
	if err != nil {
		return nil, err
	}
	for _, tool := range servers[0].Tools {
		if tool.Name == definition.Name {
			return &tool, nil
		}
	}
	return nil, fmt.Errorf("tool disappeared after update: %s", definition.Name)
}

func mcpCatalogDefinitions() []aiopsmodel.MCPToolDefinition {
	gatewayTools := agentToolDefinitions()
	definitions := make([]aiopsmodel.MCPToolDefinition, 0, 160)
	for _, domain := range mcpDomains {
		for _, spec := range mcpToolSpecs[domain.Code] {
			handler := mcpHandlerMap[spec.Name]
			definition := aiopsmodel.MCPToolDefinition{
				Domain: domain.Code, Name: spec.Name, DisplayName: spec.DisplayName,
				Description: spec.Description, Category: mcpToolCategory(spec.Name),
				HandlerKey: handler, InputSchema: emptyMCPObjectSchema(),
				Readonly: !strings.Contains(spec.Name, "_request_"), RiskFloor: spec.Risk,
				Supported: handler != "", Version: "1.0.0",
			}
			if spec.Name == "ecs_request_cleanup_logs" {
				definition.Readonly = true // This tool only creates a signed preview; confirmation executes through the JWT endpoint.
			}
			if spec.Name == "aiops_confirm_action" {
				definition.Readonly = false
			}
			if strings.HasPrefix(spec.Name, "k8s_request_") && spec.Name != "k8s_request_recreate_pod" && spec.Name != "k8s_request_rerun_job" && spec.Name != "k8s_request_apply_yaml" && spec.Name != "k8s_request_cordon_node" && spec.Name != "k8s_request_drain_node" {
				definition.Readonly = true // K8s mutation tools create previews; confirmation executes through the JWT endpoint.
			}
			if gateway, ok := gatewayTools[handler]; ok {
				definition.InputSchema = gateway.InputSchema
			}
			if schema, ok := mcpReadToolSchemas[spec.Name]; ok {
				definition.InputSchema = schema
			}
			switch spec.Name {
			case "aiops_chat":
				definition.InputSchema = mcpObjectSchema(map[string]string{"question": "string", "sessionId": "string", "userId": "string", "conversationId": "string", "chatType": "string", "confirmationId": "string"}, []string{"question"})
			case "aiops_confirm_action":
				definition.InputSchema = mcpObjectSchema(map[string]string{"confirmationId": "string"}, []string{"confirmationId"})
			case "aiops_skill_catalog":
				definition.InputSchema = mcpObjectSchema(map[string]string{"detail": "boolean"}, nil)
			case "aiops_skill_execute":
				definition.InputSchema = map[string]interface{}{"type": "object", "properties": map[string]interface{}{"skillName": map[string]interface{}{"type": "string"}, "arguments": map[string]interface{}{"type": "object"}, "userPrompt": map[string]interface{}{"type": "string"}}, "required": []string{"skillName"}}
			}
			definitions = append(definitions, definition)
		}
	}
	return definitions
}

var mcpReadToolSchemas = map[string]map[string]interface{}{
	"ecs_list_hosts":                mcpObjectSchema(map[string]string{"keyword": "string", "limit": "number"}, nil),
	"ecs_list_filesystems":          mcpObjectSchema(map[string]string{"hostId": "number", "authId": "number"}, []string{"hostId"}),
	"ecs_inspect_path":              mcpObjectSchema(map[string]string{"hostId": "number", "authId": "number", "path": "string", "maxEntries": "number", "tailLines": "number"}, []string{"hostId", "path"}),
	"ecs_analyze_disk_usage":        mcpObjectSchema(map[string]string{"hostId": "number", "authId": "number", "path": "string", "depth": "number", "topN": "number", "minFileSizeMB": "number"}, []string{"hostId"}),
	"ecs_list_log_files":            mcpObjectSchema(map[string]string{"hostId": "number", "authId": "number", "path": "string", "extensions": "array", "olderThanDays": "number", "minSizeMB": "number", "maxFiles": "number", "maxTotalSizeMB": "number"}, []string{"hostId", "path"}),
	"ecs_request_restart_service":   mcpObjectSchema(map[string]string{"hostId": "number", "authId": "number", "service": "string", "confirmed": "boolean", "confirmationId": "string"}, []string{"hostId", "service"}),
	"ecs_request_start_service":     mcpObjectSchema(map[string]string{"hostId": "number", "authId": "number", "service": "string", "confirmed": "boolean", "confirmationId": "string"}, []string{"hostId", "service"}),
	"ecs_request_stop_service":      mcpObjectSchema(map[string]string{"hostId": "number", "authId": "number", "service": "string", "confirmed": "boolean", "confirmationId": "string"}, []string{"hostId", "service"}),
	"ecs_request_cleanup_logs":      mcpObjectSchema(map[string]string{"hostId": "number", "authId": "number", "path": "string", "extensions": "array", "olderThanDays": "number", "minSizeMB": "number", "maxFiles": "number", "maxTotalSizeMB": "number"}, []string{"hostId", "path"}),
	"k8s_list_clusters":             mcpObjectSchema(map[string]string{"clusterId": "number", "keyword": "string", "page": "number", "pageSize": "number"}, nil),
	"k8s_check_cluster":             mcpObjectSchema(map[string]string{"clusterId": "number"}, []string{"clusterId"}),
	"k8s_list_namespaces":           mcpObjectSchema(map[string]string{"clusterId": "number"}, []string{"clusterId"}),
	"k8s_list_nodes":                mcpObjectSchema(map[string]string{"clusterId": "number"}, []string{"clusterId"}),
	"k8s_get_node_status":           mcpObjectSchema(map[string]string{"clusterId": "number", "node": "string", "includeMetrics": "boolean"}, []string{"clusterId", "node"}),
	"k8s_search_resources":          mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "kind": "string", "keyword": "string", "exactName": "boolean", "page": "number", "pageSize": "number"}, []string{"clusterId", "kind"}),
	"k8s_get_resource":              mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "kind": "string", "name": "string"}, []string{"clusterId", "namespace", "kind", "name"}),
	"k8s_get_workload_status":       mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "workloadType": "string", "workloadName": "string"}, []string{"clusterId", "namespace", "workloadType", "workloadName"}),
	"k8s_get_workload_history":      mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "workloadName": "string"}, []string{"clusterId", "namespace", "workloadName"}),
	"k8s_get_pod_status":            mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "pod": "string"}, []string{"clusterId", "namespace", "pod"}),
	"k8s_get_service_endpoints":     mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "service": "string"}, []string{"clusterId", "namespace", "service"}),
	"k8s_query_events":              mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "kind": "string", "name": "string", "limit": "number"}, []string{"clusterId"}),
	"k8s_query_logs":                mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "pod": "string", "container": "string", "tailLines": "number", "limit": "number", "keyword": "string", "regex": "string", "previous": "boolean", "timestamps": "boolean"}, []string{"clusterId", "namespace", "pod"}),
	"k8s_query_metrics":             mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "pod": "string", "node": "string", "container": "string", "range": "string", "start": "number", "end": "number", "step": "string"}, []string{"clusterId"}),
	"k8s_diagnose_workload":         mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "workloadType": "string", "workloadName": "string", "container": "string", "tailLines": "number", "limit": "number", "eventLimit": "number"}, []string{"clusterId", "namespace", "workloadType", "workloadName"}),
	"k8s_diagnose_pod":              mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "pod": "string", "container": "string", "tailLines": "number", "limit": "number", "eventLimit": "number"}, []string{"clusterId", "namespace", "pod"}),
	"k8s_diagnose_ingress":          mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "ingress": "string", "host": "string", "path": "string", "statusCode": "number", "observedError": "string", "tailLines": "number", "limit": "number", "eventLimit": "number"}, []string{"clusterId", "namespace"}),
	"k8s_request_rollout_restart":   mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "workloadType": "string", "workloadName": "string", "confirmed": "boolean", "confirmationId": "string"}, []string{"clusterId", "namespace", "workloadType", "workloadName"}),
	"k8s_request_scale_workload":    mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "workloadType": "string", "workloadName": "string", "replicas": "number", "expectedReplicas": "number", "confirmed": "boolean", "confirmationId": "string"}, []string{"clusterId", "namespace", "workloadType", "workloadName", "replicas"}),
	"k8s_request_rollback_workload": mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "workloadName": "string", "revision": "number", "imageVersion": "string", "confirmed": "boolean", "confirmationId": "string"}, []string{"clusterId", "namespace", "workloadName"}),
	"k8s_request_update_image":      mcpObjectSchema(map[string]string{"clusterId": "number", "namespace": "string", "workloadType": "string", "workloadName": "string", "container": "string", "image": "string", "confirmed": "boolean", "confirmationId": "string"}, []string{"clusterId", "namespace", "workloadType", "workloadName", "container", "image"}),
	"db_search_assets":              mcpObjectSchema(map[string]string{"keyword": "string", "type": "number", "environment": "string", "status": "number", "page": "number", "pageSize": "number"}, nil),
	"db_search_objects":             mcpObjectSchema(map[string]string{"assetId": "number", "database": "string", "schema": "string", "objectName": "string", "keyword": "string"}, []string{"assetId"}),
	"db_get_table_schema":           mcpObjectSchema(map[string]string{"assetId": "number", "database": "string", "schema": "string", "objectName": "string", "objectType": "string"}, []string{"assetId", "database", "objectName"}),
	"db_execute_readonly_sql":       mcpObjectSchema(map[string]string{"assetId": "number", "database": "string", "schema": "string", "sql": "string", "page": "number", "pageSize": "number"}, []string{"assetId", "sql"}),
	"db_explain_sql":                mcpObjectSchema(map[string]string{"assetId": "number", "database": "string", "schema": "string", "sql": "string"}, []string{"assetId", "sql"}),
	"db_get_monitor_snapshot":       mcpObjectSchema(map[string]string{"assetId": "number", "range": "string", "step": "string"}, []string{"assetId"}),
	"db_get_slow_sql_top":           mcpObjectSchema(map[string]string{"assetId": "number", "authId": "number", "database": "string", "keyword": "string", "range": "string", "pageSize": "number", "tailLines": "number"}, []string{"assetId"}),
	"db_get_instance_health":        mcpObjectSchema(map[string]string{"assetId": "number", "range": "string", "step": "string"}, []string{"assetId"}),
	"log_list_business_groups":      mcpObjectSchema(map[string]string{"keyword": "string", "page": "number", "pageSize": "number"}, nil),
	"log_list_projects":             mcpObjectSchema(map[string]string{"keyword": "string", "status": "number", "page": "number", "pageSize": "number"}, nil),
	"log_list_stores":               mcpObjectSchema(map[string]string{"projectId": "number", "keyword": "string", "status": "number", "page": "number", "pageSize": "number"}, nil),
	"log_check_pipeline":            mcpObjectSchema(map[string]string{}, nil),
	"log_search":                    mcpObjectSchema(map[string]string{"storeId": "number", "storeCode": "string", "query": "string", "keyword": "string", "start": "string", "end": "string", "limit": "number", "page": "number", "pageSize": "number"}, []string{"storeId"}),
	"log_aggregate":                 mcpObjectSchema(map[string]string{"storeId": "number", "storeCode": "string", "query": "string", "keyword": "string", "start": "string", "end": "string", "step": "string"}, []string{"storeId"}),
	"alert_list_active":             mcpObjectSchema(map[string]string{"keyword": "string", "severity": "string", "businessId": "string", "page": "number", "pageSize": "number"}, nil),
	"alert_get_detail":              mcpObjectSchema(map[string]string{"eventId": "number", "eventType": "string"}, []string{"eventId"}),
	"alert_get_timeline":            mcpObjectSchema(map[string]string{"eventId": "number"}, []string{"eventId"}),
	"alert_get_history":             mcpObjectSchema(map[string]string{"keyword": "string", "severity": "string", "businessId": "string", "fingerprint": "string", "page": "number", "pageSize": "number"}, nil),
	"alert_get_notification_status": mcpObjectSchema(map[string]string{"eventId": "number", "eventType": "string"}, []string{"eventId"}),
	"alert_list_silences":           mcpObjectSchema(map[string]string{"keyword": "string", "enabled": "string", "page": "number", "pageSize": "number"}, nil),
	"knowledge_list_groups":         mcpObjectSchema(map[string]string{}, nil),
	"knowledge_list_categories":     mcpObjectSchema(map[string]string{"limit": "number"}, nil),
	"knowledge_get_document":        mcpObjectSchema(map[string]string{"documentId": "number"}, []string{"documentId"}),
	"knowledge_find_runbooks":       mcpObjectSchema(map[string]string{"query": "string", "serviceName": "string", "category": "string", "limit": "number"}, []string{"query"}),
	"knowledge_find_known_issues":   mcpObjectSchema(map[string]string{"query": "string", "serviceName": "string", "category": "string", "limit": "number"}, []string{"query"}),
	"knowledge_get_citation":        mcpObjectSchema(map[string]string{"documentId": "number"}, []string{"documentId"}),
	"asset_search":                  mcpObjectSchema(map[string]string{"keyword": "string", "type": "string", "limit": "number"}, []string{"keyword"}),
}

func defaultMCPToolView(definition aiopsmodel.MCPToolDefinition) aiopsmodel.MCPToolView {
	confirmation := riskRank(definition.RiskFloor) >= riskRank(aiopsmodel.MCPRiskR2)
	return aiopsmodel.MCPToolView{
		MCPToolDefinition: definition, Enabled: definition.Supported && definition.RiskFloor != aiopsmodel.MCPRiskR4,
		EffectiveRisk: definition.RiskFloor, RequiredConfirmation: confirmation,
		TimeoutSeconds: 30, RateLimitPerMinute: 60, MaxResultRows: 1000,
		MaxResultBytes: 1024 * 1024, AllowedEnvironments: []string{"dev", "test", "prod"}, ConfigVersion: 1,
	}
}

func applyMCPToolOverride(view *aiopsmodel.MCPToolView, override aiopsmodel.MCPToolConfig) {
	view.Enabled = override.Enabled
	if riskRank(override.RiskLevel) >= riskRank(view.RiskFloor) {
		view.EffectiveRisk = normalizeRisk(override.RiskLevel)
	}
	view.RequiredConfirmation = override.RequiredConfirmation || riskRank(view.EffectiveRisk) >= riskRank(aiopsmodel.MCPRiskR2)
	if override.TimeoutSeconds > 0 {
		view.TimeoutSeconds = override.TimeoutSeconds
	}
	if override.RateLimitPerMinute > 0 {
		view.RateLimitPerMinute = override.RateLimitPerMinute
	}
	if override.MaxResultRows > 0 {
		view.MaxResultRows = override.MaxResultRows
	}
	if override.MaxResultBytes > 0 {
		view.MaxResultBytes = override.MaxResultBytes
	}
	if environments := decodeMCPEnvironments(override.AllowedEnvironments); len(environments) > 0 {
		view.AllowedEnvironments = environments
	}
	view.ConfigVersion = override.ConfigVersion
}

func applyMCPToolRequest(config *aiopsmodel.MCPToolConfig, req aiopsmodel.MCPToolConfigRequest) {
	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}
	if req.RiskLevel != nil {
		config.RiskLevel = normalizeRisk(*req.RiskLevel)
	}
	if req.RequiredConfirmation != nil {
		config.RequiredConfirmation = *req.RequiredConfirmation
	}
	if req.TimeoutSeconds != nil {
		config.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.RateLimitPerMinute != nil {
		config.RateLimitPerMinute = *req.RateLimitPerMinute
	}
	if req.MaxResultRows != nil {
		config.MaxResultRows = *req.MaxResultRows
	}
	if req.MaxResultBytes != nil {
		config.MaxResultBytes = *req.MaxResultBytes
	}
	if req.AllowedEnvironments != nil {
		data, _ := json.Marshal(normalizeMCPEnvironments(*req.AllowedEnvironments))
		config.AllowedEnvironments = string(data)
	}
}

func validateMCPToolLimits(req aiopsmodel.MCPToolConfigRequest) error {
	if req.TimeoutSeconds != nil && (*req.TimeoutSeconds < 1 || *req.TimeoutSeconds > 600) {
		return fmt.Errorf("timeoutSeconds must be between 1 and 600")
	}
	if req.RateLimitPerMinute != nil && (*req.RateLimitPerMinute < 1 || *req.RateLimitPerMinute > 10000) {
		return fmt.Errorf("rateLimitPerMinute must be between 1 and 10000")
	}
	if req.MaxResultRows != nil && (*req.MaxResultRows < 1 || *req.MaxResultRows > 100000) {
		return fmt.Errorf("maxResultRows must be between 1 and 100000")
	}
	if req.MaxResultBytes != nil && (*req.MaxResultBytes < 1024 || *req.MaxResultBytes > 50*1024*1024) {
		return fmt.Errorf("maxResultBytes must be between 1024 and 52428800")
	}
	return nil
}

func findMCPToolDefinition(code, name string) (aiopsmodel.MCPToolDefinition, bool) {
	code = strings.ToLower(strings.TrimSpace(code))
	name = strings.TrimSpace(name)
	for _, definition := range mcpCatalogDefinitions() {
		if definition.Domain == code && definition.Name == name {
			return definition, true
		}
	}
	return aiopsmodel.MCPToolDefinition{}, false
}

func isMCPDomain(code string) bool {
	for _, item := range mcpDomains {
		if item.Code == code {
			return true
		}
	}
	return false
}

func mcpToolConfigKey(code, name string) string { return code + "\x00" + name }

func mcpToolCategory(name string) string {
	if strings.Contains(name, "_request_") {
		return "受控变更"
	}
	if strings.Contains(name, "diagnose") || strings.Contains(name, "analyze") || strings.Contains(name, "detect") || strings.Contains(name, "check") {
		return "诊断分析"
	}
	return "查询"
}

func normalizeRisk(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if riskRank(value) < 0 {
		return ""
	}
	return value
}

func riskRank(value string) int {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "R0":
		return 0
	case "R1":
		return 1
	case "R2":
		return 2
	case "R3":
		return 3
	case "R4":
		return 4
	default:
		return -1
	}
}

func normalizeMCPEnvironments(items []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func decodeMCPEnvironments(value string) []string {
	var result []string
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return nil
	}
	return normalizeMCPEnvironments(result)
}

func emptyMCPObjectSchema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func mcpObjectSchema(properties map[string]string, required []string) map[string]interface{} {
	items := make(map[string]interface{}, len(properties))
	for name, dataType := range properties {
		items[name] = map[string]interface{}{"type": dataType}
	}
	result := map[string]interface{}{"type": "object", "properties": items}
	if len(required) > 0 {
		result["required"] = required
	}
	return result
}
