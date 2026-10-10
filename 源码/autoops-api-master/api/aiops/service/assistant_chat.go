package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	pathpkg "path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	aiopsdao "dodevops-api/api/aiops/dao"
	aiopsmodel "dodevops-api/api/aiops/model"
	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	accountdao "dodevops-api/api/configcenter/dao"
	k8smodel "dodevops-api/api/k8s/model"
	systemauthz "dodevops-api/api/system/authz"
	"dodevops-api/common"
	agentskill "dodevops-api/pkg/agent/skill"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	defaultAssistantChatMinutes = 30
	defaultAssistantChatStep    = "60s"
	defaultAssistantMemoryLimit = 8
	maxAssistantK8sClusters     = 10
	maxAssistantK8sNamespaces   = 30
	maxAssistantK8sPodPages     = 5
	assistantK8sDiscoveryPage   = 100
)

type assistantK8sClusterTarget struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type assistantK8sWorkloadTarget struct {
	ClusterID    uint   `json:"clusterId"`
	ClusterName  string `json:"clusterName"`
	Namespace    string `json:"namespace"`
	WorkloadType string `json:"workloadType"`
	WorkloadName string `json:"workloadName"`
}

type assistantK8sResourceHotspot struct {
	Cluster    string  `json:"cluster"`
	Namespace  string  `json:"namespace"`
	Pod        string  `json:"pod"`
	CPU        string  `json:"cpu,omitempty"`
	Memory     string  `json:"memory,omitempty"`
	CPURate    float64 `json:"cpuRate,omitempty"`
	MemoryRate float64 `json:"memoryRate,omitempty"`
	cpuMilli   int64
	memoryByte int64
}

type assistantK8sAbnormalPod struct {
	Cluster      string   `json:"cluster"`
	Namespace    string   `json:"namespace"`
	Pod          string   `json:"pod"`
	Status       string   `json:"status"`
	Node         string   `json:"node,omitempty"`
	RestartCount int      `json:"restartCount,omitempty"`
	Conditions   []string `json:"conditions,omitempty"`
}

type assistantK8sClusterScan struct {
	ClusterID           uint                          `json:"clusterId"`
	ClusterName         string                        `json:"clusterName"`
	LiveChecks          int                           `json:"liveChecks"`
	FailedChecks        []string                      `json:"failedChecks,omitempty"`
	NodeCount           int                           `json:"nodeCount"`
	NotReadyNodes       []string                      `json:"notReadyNodes,omitempty"`
	PressureNodes       []string                      `json:"pressureNodes,omitempty"`
	AuthorizedNS        int                           `json:"authorizedNamespaces"`
	ScannedNS           int                           `json:"scannedNamespaces"`
	SkippedNS           int                           `json:"skippedNamespaces,omitempty"`
	PodCount            int                           `json:"podCount"`
	PodResultsTruncated bool                          `json:"podResultsTruncated,omitempty"`
	AbnormalPods        []assistantK8sAbnormalPod     `json:"abnormalPods,omitempty"`
	WarningEvents       []string                      `json:"warningEvents,omitempty"`
	MetricsAvailable    bool                          `json:"metricsAvailable"`
	MetricsMissingNS    []string                      `json:"metricsMissingNamespaces,omitempty"`
	Hotspots            []assistantK8sResourceHotspot `json:"hotspots,omitempty"`
}

type assistantK8sScanReport struct {
	RequestedClusters int                       `json:"requestedClusters"`
	ScannedClusters   int                       `json:"scannedClusters"`
	SkippedClusters   int                       `json:"skippedClusters,omitempty"`
	Clusters          []assistantK8sClusterScan `json:"clusters"`
}

var assistantChatIPPattern = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
var assistantChatHostNamePattern = regexp.MustCompile(`(?i)\b[a-z0-9][a-z0-9._]*-[a-z0-9][a-z0-9._-]*\b`)
var assistantChatLinuxPathPattern = regexp.MustCompile(`/(?:[a-zA-Z0-9._@+-]+/?)+`)
var assistantChatDatabaseNamePattern = regexp.MustCompile(`(?i)\b(?:mysql|redis|postgres|postgresql|mongo|mongodb|oracle|sqlserver|db)[a-z0-9_.:-]*\b`)
var assistantChatIPDatabaseNamePattern = regexp.MustCompile(`(?i)\b(?:\d{1,3}\.){3}\d{1,3}[-_:.a-z0-9]*(?:mysql|redis|postgres|postgresql|mongo|mongodb|oracle|sqlserver|db)[-_:.a-z0-9]*\b`)
var assistantChatK8sResourcePattern = regexp.MustCompile(`(?i)(pod|node|deployment|statefulset|daemonset|cronjob|job|service|ingress|namespace)\s*(?:name|名称)?\s*[:：=]?\s+([a-z0-9][a-z0-9._-]*)`)
var assistantChatK8sChineseResourcePattern = regexp.MustCompile(`(?i)(集群|命名空间|节点)\s*(?:名称)?\s*[:：=]?\s*([a-z0-9][a-z0-9._-]*)`)
var assistantChatK8sNamedWorkloadPattern = regexp.MustCompile(`(?i)(?:名为|名称(?:为|是)?|name\s*[:=]?)\s*([a-z0-9][a-z0-9._-]*)\s*(?:的)?\s*(?:工作负载|workload)`)
var assistantChatK8sServiceWorkloadPattern = regexp.MustCompile(`(?i)(?:服务|应用|service|app)\s*[:：=]?\s*([a-z0-9][a-z0-9._-]*)\s*(?:的)?\s*(?:工作负载|workload)`)
var assistantChatK8sDirectWorkloadPattern = regexp.MustCompile(`(?i)([a-z0-9][a-z0-9._-]*)\s*(?:帮我\s*)?(?:的|这个|该|对\s*(?:这个|该)?)?\s*(?:工作负载|workload)`)
var assistantChatK8sWorkloadHistoryPattern = regexp.MustCompile(`(?i)([a-z0-9][a-z0-9._-]*)\s*(?:的)?\s*(?:工作负载|workload|deployment)\s*(?:的)?\s*(?:历史版本|版本历史|发布历史|发布记录|版本记录|rollout history|revision history|deployment history)`)
var assistantChatK8sRollbackRevisionPattern = regexp.MustCompile(`(?i)(?:回滚|rollback)[^\r\n]{0,40}?(?:版本号|revision)\s*(?:为|是|=|:|：|到|至)?\s*(\d+)`)
var assistantChatK8sRollbackImagePattern = regexp.MustCompile(`(?i)(?:回滚|rollback)[^\r\n]{0,50}?(?:镜像版本|镜像|image(?:\s+version)?)\s*(?:为|是|=|:|：|到|至)?\s*([a-z0-9][a-z0-9._-]*)`)
var assistantChatK8sReplicaPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:扩容|缩容|扩缩容|伸缩|scale)\s*(?:到|至|为|成|to)?\s*(\d+)\s*(?:个)?\s*(?:副本|replicas?)`),
	regexp.MustCompile(`(?i)(?:副本数?|replicas?)\s*(?:调整|扩容|缩容)?\s*(?:到|至|为|成|=|:|to)?\s*(\d+)`),
}
var assistantChatK8sReplicaDeltaPatterns = []struct {
	pattern *regexp.Regexp
	sign    int
}{
	{regexp.MustCompile(`(?i)(?:增加|新增|加上|多加|increase(?:\s+by)?)\s*(\d+|一|二|两|三|四|五|六|七|八|九|十)\s*(?:个)?\s*(?:副本(?:数)?|replicas?)`), 1},
	{regexp.MustCompile(`(?i)(?:(?:副本(?:数)?|replicas?)\s*(?:扩容\s*)?|扩容\s*)(?:增加|新增|加上|多加|increase(?:\s+by)?)\s*(\d+|一|二|两|三|四|五|六|七|八|九|十)\s*(?:个)?`), 1},
	{regexp.MustCompile(`(?i)(?:减少|减掉|少开|decrease(?:\s+by)?)\s*(\d+|一|二|两|三|四|五|六|七|八|九|十)\s*(?:个)?\s*(?:副本(?:数)?|replicas?)`), -1},
	{regexp.MustCompile(`(?i)(?:(?:副本(?:数)?|replicas?)\s*(?:缩容\s*)?|缩容\s*)(?:减少|减掉|少开|decrease(?:\s+by)?)\s*(\d+|一|二|两|三|四|五|六|七|八|九|十)\s*(?:个)?`), -1},
}

type AssistantChatService struct {
	agentDao      *aiopsdao.AgentDao
	llmDao        *aiopsdao.LLMProviderDao
	toolGateway   *AgentToolGatewayService
	querySvc      *AssistantQueryService
	streamEmitter assistantChatStreamEmitter
}

type assistantChatIntentDefinition struct {
	Name          string
	Keywords      []string
	ContextPages  []string
	RequiredOneOf []string
	ToolPlan      func(map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep
}

type assistantChatStreamEmitter func(aiopsmodel.AgentRunStreamEvent)

func assistantChatIntentRegistry() []assistantChatIntentDefinition {
	return []assistantChatIntentDefinition{
		{
			Name: aiopsmodel.AssistantChatIntentPlatformOverview,
			Keywords: []string{
				"平台", "整体", "全局", "总览", "风险", "异常", "健康", "运维情况", "系统情况",
				"platform", "overview", "risk", "risks", "health",
			},
			ContextPages: []string{"global_chat", "ops_chat", "assistant", "dashboard", "home"},
			ToolPlan:     buildAssistantChatPlatformToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentAlertOverview,
			Keywords: []string{
				"告警", "报警", "alert", "firing", "故障", "incident",
			},
			ContextPages: []string{"alert", "alerts", "alert_detail", "monitor_alert"},
			ToolPlan:     buildAssistantChatAlertToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentHostInventory,
			Keywords: []string{
				"多少主机", "多少台主机", "几台主机", "主机数量", "主机总数", "主机资产概览",
				"how many hosts", "host count", "hosts count",
			},
			ContextPages: []string{"global_chat", "ops_chat", "assistant", "host", "cmdb_host", "asset_host"},
			ToolPlan:     buildAssistantChatHostInventoryToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentHostPressure,
			Keywords: []string{
				"压力", "负载", "cpu", "内存", "磁盘", "检查", "异常", "慢", "卡",
				"状态", "健康", "怎么样", "如何", "看下", "看看", "再看", "复核",
				"pressure", "load", "status", "health",
			},
			ContextPages:  []string{"host", "host_detail", "cmdb_host", "asset_host"},
			RequiredOneOf: []string{"hostId", "ip", "hostName", "hostname", "instance", "keyword"},
			ToolPlan:      buildAssistantChatHostPressureToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentHostLogCleanup,
			Keywords: []string{
				"清空日志", "清理日志", "删除日志", "清空文件", "删除文件", "truncate log", "clear log",
			},
			ContextPages:  []string{"host", "host_detail", "cmdb_host", "asset_host"},
			RequiredOneOf: []string{"hostId", "ip", "hostName", "hostname", "instance", "keyword"},
			ToolPlan:      buildAssistantChatHostLogCleanupToolPlan,
		},
		{
			Name:         aiopsmodel.AssistantChatIntentUnsupportedFileOp,
			Keywords:     []string{"删除文件", "清理文件", "清空文件", "remove file", "delete file"},
			ContextPages: []string{"host", "host_detail", "cmdb_host", "asset_host", "global_chat", "ops_chat", "assistant"},
			ToolPlan:     buildAssistantChatUnsupportedFileOperationToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentHostServices,
			Keywords: []string{
				"部署了什么服务", "部署哪些服务", "上面部署", "运行了什么服务", "运行哪些服务",
				"有什么服务", "服务列表", "进程列表", "top进程", "top 进程", "端口服务",
				"services", "processes", "top processes",
			},
			ContextPages:  []string{"host", "host_detail", "cmdb_host", "asset_host"},
			RequiredOneOf: []string{"hostId", "ip", "hostName", "hostname", "instance", "keyword"},
			ToolPlan:      buildAssistantChatHostServicesToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentDatabaseOverview,
			Keywords: []string{
				"数据库", "实例", "库实例", "db", "database", "mysql", "postgres", "postgresql", "redis",
				"慢sql", "慢查询", "sql审核", "数据库资产", "数据库状态",
			},
			ContextPages: []string{"db", "database", "db_asset", "database_asset", "db_detail", "database_detail", "slow_log", "sql_review"},
			ToolPlan:     buildAssistantChatDatabaseToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentK8sOverview,
			Keywords: []string{
				"k8s", "kubernetes", "容器", "集群", "pod", "namespace", "命名空间", "deployment",
				"工作负载", "ingress", "service", "节点", "node", "事件", "yaml",
			},
			ContextPages: []string{"k8s", "kubernetes", "k8s_cluster", "k8s_cluster_detail", "k8s_pod", "k8s_workload", "k8s_namespace", "k8s_ingress", "k8s_service"},
			ToolPlan:     buildAssistantChatK8sToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentK8sRolloutRestart,
			Keywords: []string{
				"滚动重启", "重启工作负载", "重启 deployment", "重启 statefulset", "重启 daemonset", "rollout restart",
			},
			ContextPages: []string{"k8s", "kubernetes", "k8s_cluster", "k8s_cluster_detail", "k8s_workload", "k8s_namespace"},
			ToolPlan:     buildAssistantChatK8sRolloutRestartToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentK8sScaleWorkload,
			Keywords: []string{
				"扩容工作负载", "缩容工作负载", "调整副本", "副本数", "scale workload", "scale deployment",
			},
			ContextPages: []string{"k8s", "kubernetes", "k8s_cluster", "k8s_cluster_detail", "k8s_workload", "k8s_namespace"},
			ToolPlan:     buildAssistantChatK8sScaleWorkloadToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentK8sRollbackWorkload,
			Keywords: []string{
				"回滚工作负载", "工作负载回滚", "回滚 deployment", "rollback workload", "rollback deployment",
			},
			ContextPages: []string{"k8s", "kubernetes", "k8s_cluster", "k8s_cluster_detail", "k8s_workload", "k8s_namespace"},
			ToolPlan:     buildAssistantChatK8sRollbackWorkloadToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentKnowledgeSearch,
			Keywords: []string{
				"知识库", "知识", "文档", "手册", "runbook", "sop", "故障案例", "故障复盘", "处理方案",
				"有没有", "查一下文档", "查下文档", "相关资料",
			},
			ContextPages: []string{"knowledge", "runbook", "document", "docs"},
			ToolPlan:     buildAssistantChatKnowledgeToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentMCPCatalog,
			Keywords: []string{
				"mcp工具", "mcp 工具", "mcp服务", "mcp 服务", "mcp tool", "mcp server",
			},
			ContextPages: []string{"global_chat", "ops_chat", "assistant", "mcp", "mcp_management"},
			ToolPlan:     buildAssistantChatMCPCatalogToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentSkillCatalog,
			Keywords: []string{
				"支持哪些技能", "当前技能", "技能列表", "已加载技能", "skill catalog", "skills list",
			},
			ContextPages: []string{"global_chat", "ops_chat", "assistant"},
			ToolPlan:     buildAssistantChatSkillCatalogToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentResourceGovernance,
			Keywords: []string{
				"选址", "部署", "上线", "扩容", "回收", "释放", "容量", "巡检", "闲置", "资产",
				"长期低负载", "低负载", "可回收", "测试机器", "测试机", "哪些机器", "那些机器",
				"哪些主机", "那些主机", "哪些服务器", "那些服务器", "比较大", "排行", "排名", "top",
			},
			ContextPages: []string{"resource", "capacity", "cmdb", "assets", "asset"},
			ToolPlan:     buildAssistantChatResourceGovernanceToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentOpsGeneral,
			Keywords: []string{
				"脚本", "shell", "bash", "linux", "nginx", "服务", "安装", "部署", "配置", "命令", "巡检脚本", "检查脚本", "怎么写",
				"帮我写", "生成", "写一个", "写个", "示例", "排查步骤", "操作步骤", "sop", "runbook",
				"script", "command", "checklist",
			},
			ContextPages: []string{"global_chat", "ops_chat", "assistant"},
			ToolPlan:     buildAssistantChatOpsGeneralToolPlan,
		},
		{
			Name: aiopsmodel.AssistantChatIntentGeneralQA,
			Keywords: []string{
				"是什么", "什么是", "是谁", "有哪些", "介绍", "解释", "为什么", "如何理解",
				"what is", "who is", "explain",
			},
			ContextPages: []string{"global_chat", "assistant"},
			ToolPlan:     buildAssistantChatGeneralQAToolPlan,
		},
	}
}

func NewAssistantChatService() *AssistantChatService {
	return &AssistantChatService{
		agentDao:    aiopsdao.NewAgentDao(),
		llmDao:      aiopsdao.NewLLMProviderDao(),
		toolGateway: NewAgentToolGatewayService(),
		querySvc:    NewAssistantQueryService(),
	}
}

func NewAssistantChatStreamService(emitter assistantChatStreamEmitter) *AssistantChatService {
	svc := NewAssistantChatService()
	svc.streamEmitter = emitter
	return svc
}

func (s *AssistantChatService) ClearHistory(c *gin.Context, req *aiopsmodel.AssistantChatHistoryClearRequest) (*aiopsmodel.AssistantChatHistoryClearResult, error) {
	if req == nil {
		req = &aiopsmodel.AssistantChatHistoryClearRequest{}
	}
	return s.agentDao.ClearAssistantChatHistory(req.SessionID)
}

func (s *AssistantChatService) Chat(c *gin.Context, req *aiopsmodel.AssistantChatRequest) (*aiopsmodel.AssistantChatResult, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		return nil, errors.New("question is required")
	}
	req.SessionID = strings.TrimSpace(req.SessionID)
	if req.SessionID == "" {
		req.SessionID = fmt.Sprintf("chat-%d", time.Now().UnixNano())
	}
	userID, username := assistantChatUser(c)

	now := time.Now()
	memory, memoryErr := s.loadAssistantChatMemory(req.SessionID)
	if memoryErr != nil {
		memory = &aiopsmodel.AssistantChatMemoryContext{
			Used:         false,
			MessageCount: 0,
			Entities:     map[string]interface{}{},
		}
	}
	entities := detectAssistantChatEntities(req)
	mergeAssistantChatMemoryEntities(entities, memory, req.Question)
	normalizeAssistantChatDomainEntities(req.Question, entities)
	intent := detectAssistantChatIntent(req, entities)
	var pendingConfirmation map[string]interface{}
	if assistantChatQuestionConfirmsPendingAction(req.Question) {
		intent = aiopsmodel.AssistantChatIntentActionConfirmation
		pendingConfirmation, memoryErr = s.findPendingAssistantAction(req.SessionID)
		if explicitID := assistantChatContextString(req.Context, "confirmationId", "confirmation_id"); explicitID != "" {
			memoryErr = nil
			if pendingConfirmation == nil {
				pendingConfirmation = map[string]interface{}{}
			}
			pendingConfirmation["confirmationId"] = explicitID
			if action := assistantChatContextString(req.Context, "action", "tool"); action != "" {
				pendingConfirmation["tool"] = action
			}
			if endpoint := assistantChatContextString(req.Context, "confirmationEndpoint", "confirmation_endpoint"); endpoint != "" {
				pendingConfirmation["confirmationEndpoint"] = endpoint
			}
		}
	}
	clarifyQuestions := buildAssistantChatClarifyQuestions(intent, entities)
	if len(clarifyQuestions) > 0 {
		entities["pendingIntent"] = intent
		intent = aiopsmodel.AssistantChatIntentClarify
	}
	run := &aiopsmodel.AgentRun{
		RunType:   aiopsmodel.AgentRunTypeAssistantChat,
		Status:    aiopsmodel.AgentRunStatusRunning,
		Summary:   truncateAssistantChatText(req.Question, 180),
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.agentDao.CreateRun(run); err != nil {
		return nil, err
	}
	s.emitAssistantChatRunEvent(run.ID, aiopsmodel.AgentRunStatusRunning, "通用助手任务已创建")
	_ = s.agentDao.UpsertSession(&aiopsmodel.AgentSession{
		SessionID: req.SessionID,
		Title:     truncateAssistantChatText(req.Question, 80),
		Channel:   assistantChatChannel(req),
		UserID:    userID,
		Username:  username,
		Status:    "active",
		LastRunID: run.ID,
	})
	_ = s.agentDao.CreateMessage(&aiopsmodel.AgentMessage{
		SessionID:    req.SessionID,
		RunID:        run.ID,
		Role:         aiopsmodel.AgentMessageRoleUser,
		Content:      req.Question,
		Intent:       intent,
		EntitiesJSON: compactToolJSON(entities),
	})

	result := &aiopsmodel.AssistantChatResult{
		SessionID:        req.SessionID,
		RunID:            run.ID,
		Intent:           intent,
		Question:         req.Question,
		Entities:         entities,
		Memory:           memory,
		ToolPlan:         buildAssistantChatToolPlan(intent, entities),
		ClarifyQuestions: clarifyQuestions,
		Readonly:         true,
		GeneratedAt:      now.Format("2006-01-02 15:04:05"),
	}
	if intent == aiopsmodel.AssistantChatIntentActionConfirmation {
		result.PendingAction = pendingConfirmation
	}
	routedAt := time.Now()
	initializeAssistantChatThinkingProcess(result, now, routedAt)
	result.AssistantPlan = buildAssistantInvestigationPlan(result.Intent, result.Question, result.ToolPlan, result.Entities)
	startAssistantChatPlanStep(result, "understand_requirement")
	finishAssistantChatPlanStep(result, "understand_requirement", "")
	refreshAssistantChatThinkingProcess(result, time.Time{}, "")
	s.emitAssistantChatGenericEvent("intent", run.ID, aiopsmodel.AgentRunStatusRunning, "已识别意图："+intent, map[string]interface{}{
		"intent":           intent,
		"entities":         entities,
		"toolPlan":         result.ToolPlan,
		"progress":         result.Progress,
		"thinkingProcess":  result.ThinkingProcess,
		"clarifyQuestions": clarifyQuestions,
	})
	s.emitAssistantChatGenericEvent("plan", run.ID, aiopsmodel.AgentRunStatusRunning, "已生成调查计划", map[string]interface{}{
		"assistantPlan":   result.AssistantPlan,
		"progress":        result.Progress,
		"thinkingProcess": result.ThinkingProcess,
	})

	var handleErr error
	switch intent {
	case aiopsmodel.AssistantChatIntentHostPressure:
		handleErr = s.handleAssistantHostPressure(c, req, result)
	case aiopsmodel.AssistantChatIntentHostInventory:
		handleErr = s.handleAssistantHostInventory(c, req, result)
	case aiopsmodel.AssistantChatIntentHostLogCleanup:
		handleErr = s.handleAssistantHostLogCleanup(c, req, result)
	case aiopsmodel.AssistantChatIntentUnsupportedFileOp:
		result.Answer = "当前仅支持清空 .log、.txt 日志文件，保留文件本身；不支持删除或清空 .sh 等其他类型文件。"
		result.SuggestedActions = []string{"如需清空日志，请提供 .log 或 .txt 文件路径。"}
	case aiopsmodel.AssistantChatIntentHostServices:
		handleErr = s.handleAssistantHostServices(c, req, result)
	case aiopsmodel.AssistantChatIntentPlatformOverview:
		handleErr = s.handleAssistantPlatformOverview(c, req, result)
	case aiopsmodel.AssistantChatIntentAlertOverview:
		handleErr = s.handleAssistantAlertOverview(c, req, result)
	case aiopsmodel.AssistantChatIntentDatabaseOverview:
		handleErr = s.handleAssistantDatabaseOverview(c, req, result)
	case aiopsmodel.AssistantChatIntentK8sOverview:
		handleErr = s.handleAssistantK8sOverview(c, req, result)
	case aiopsmodel.AssistantChatIntentK8sRolloutRestart:
		handleErr = s.handleAssistantK8sRolloutRestart(c, req, result)
	case aiopsmodel.AssistantChatIntentK8sScaleWorkload:
		handleErr = s.handleAssistantK8sScaleWorkload(c, req, result)
	case aiopsmodel.AssistantChatIntentK8sRollbackWorkload:
		handleErr = s.handleAssistantK8sRollbackWorkload(c, req, result)
	case aiopsmodel.AssistantChatIntentKnowledgeSearch:
		handleErr = s.handleAssistantKnowledgeSearch(c, req, result)
	case aiopsmodel.AssistantChatIntentSkillCatalog:
		handleErr = s.handleAssistantSkillCatalog(c, req, result)
	case aiopsmodel.AssistantChatIntentMCPCatalog:
		handleErr = s.handleAssistantMCPCatalog(c, req, result)
	case aiopsmodel.AssistantChatIntentActionConfirmation:
		handleErr = s.handleAssistantActionConfirmation(c, req, result, memoryErr)
	case aiopsmodel.AssistantChatIntentResourceGovernance:
		handleErr = s.handleAssistantResourceGovernance(c, req, result)
	case aiopsmodel.AssistantChatIntentOpsGeneral:
		result.Answer = buildAssistantChatOpsGeneralFallbackAnswer(req.Question)
		result.SuggestedActions = []string{
			"先在测试环境验证脚本输出。",
			"脚本默认只做只读巡检，不直接修改系统配置。",
			"涉及重启、删除、清理、变更的动作需要走审批或工单。",
		}
	case aiopsmodel.AssistantChatIntentGeneralQA:
		result.Answer = buildAssistantChatGeneralQAFallbackAnswer()
		result.SuggestedActions = []string{
			"如果你想查询平台资产、告警或主机指标，可以补充 IP、主机名、告警名或分组。",
			"如果你只是问通用知识，我会直接回答，不调用生产工具。",
		}
	case aiopsmodel.AssistantChatIntentClarify:
		result.Answer = buildAssistantChatClarifyAnswer(result)
		result.SuggestedActions = buildAssistantChatClarifyActions(result)
	default:
		if assistantChatQuestionLooksLikePlatformOverview(req.Question) {
			result.Intent = aiopsmodel.AssistantChatIntentPlatformOverview
			result.ToolPlan = buildAssistantChatToolPlan(result.Intent, result.Entities)
			result.AssistantPlan = buildAssistantInvestigationPlan(result.Intent, result.Question, result.ToolPlan, result.Entities)
			startAssistantChatPlanStep(result, "understand_requirement")
			finishAssistantChatPlanStep(result, "understand_requirement", "")
			s.emitAssistantChatGenericEvent("plan", run.ID, aiopsmodel.AgentRunStatusRunning, "已更新调查计划", map[string]interface{}{
				"assistantPlan":   result.AssistantPlan,
				"progress":        result.Progress,
				"thinkingProcess": result.ThinkingProcess,
			})
			handleErr = s.handleAssistantPlatformOverview(c, req, result)
		} else {
			result.Answer = buildAssistantChatUnknownAnswer()
			result.SuggestedActions = []string{
				"可以询问平台整体风险、当前活跃告警、主机压力、资源治理、数据库资产、K8s 集群或知识库 Runbook。",
			}
		}
	}

	if handleErr != nil {
		if isAssistantChatToolPolicyError(handleErr) {
			message := assistantChatToolPolicyMessage(handleErr)
			result.Answer = message
			result.MissingInfo = append(result.MissingInfo, message)
		} else {
			result.MissingInfo = append(result.MissingInfo, handleErr.Error())
			if strings.TrimSpace(result.Answer) == "" {
				result.Answer = "本次通用助手任务没有完成：" + handleErr.Error()
			}
		}
	}
	applyAssistantChatActionMode(result)
	finalizeAssistantChatDiagnosisCard(result)
	result.Reflection = reflectAssistantEvidence(result)
	applyAssistantDiagnosisConfidence(result)
	s.emitAssistantChatGenericEvent("reflect", run.ID, aiopsmodel.AgentRunStatusRunning, "已完成证据完整性检查", map[string]interface{}{
		"reflection":      result.Reflection,
		"progress":        result.Progress,
		"thinkingProcess": result.ThinkingProcess,
	})

	if shouldAssistantChatUseLLM(result) {
		llmTool := assistantChatLLMStepToolName(result.Intent)
		llmStarted := time.Now()
		startAssistantChatPlanStep(result, llmTool)
		refreshAssistantChatThinkingProcess(result, time.Time{}, "")
		s.emitAssistantChatGenericEvent("llm", run.ID, aiopsmodel.AgentRunStatusRunning, "正在调用模型生成回答", map[string]interface{}{
			"intent":          result.Intent,
			"progress":        result.Progress,
			"thinkingProcess": result.ThinkingProcess,
		})
		if answer, provider, used, err := s.generateAssistantChatAnswer(c.Request.Context(), req, result); err == nil && used {
			result.Answer = answer
			applyAssistantChatLLMProvider(result, provider)
			result.UsedLLM = true
			finishAssistantChatPlanStep(result, llmTool, "")
			s.emitAssistantChatGenericEvent("llm", run.ID, aiopsmodel.AgentRunStatusSucceeded, "模型回答生成完成", map[string]interface{}{
				"providerId":      result.ProviderID,
				"providerName":    result.ProviderName,
				"providerType":    result.ProviderType,
				"modelCode":       result.ModelCode,
				"progress":        result.Progress,
				"thinkingProcess": result.ThinkingProcess,
			})
		} else if provider != nil {
			applyAssistantChatLLMProvider(result, provider)
			finishAssistantChatPlanStep(result, llmTool, "")
		} else if err != nil {
			result.MissingInfo = append(result.MissingInfo, "LLM 生成失败，已返回规则化结果："+err.Error())
			finishAssistantChatPlanStep(result, llmTool, err.Error())
			s.emitAssistantChatGenericEvent("llm", run.ID, aiopsmodel.AgentRunStatusPartial, "模型生成失败，返回规则化结果", map[string]interface{}{
				"error":           err.Error(),
				"progress":        result.Progress,
				"thinkingProcess": result.ThinkingProcess,
			})
		} else {
			finishAssistantChatPlanStep(result, llmTool, "")
		}
		llmFinished := time.Now()
		llmStatus, llmError := assistantChatToolPlanState(result, llmTool)
		appendAssistantChatTimeline(result, 0, llmTool, llmStatus, assistantChatToolPurpose(llmTool), llmStarted, llmFinished, llmError)
		refreshAssistantChatThinkingProcess(result, time.Time{}, "")
	}
	summaryTool := assistantChatSummaryStepToolName(result.Intent)
	summaryStarted := time.Now()
	startAssistantChatPlanStep(result, summaryTool)
	finishAssistantChatPlanStep(result, summaryTool, "")
	if !assistantChatTimelineContainsTool(result, summaryTool) {
		appendAssistantChatTimeline(result, 0, summaryTool, aiopsmodel.AssistantChatPlanSucceeded, assistantChatToolPurpose(summaryTool), summaryStarted, time.Now(), "")
	}
	finalizeAssistantChatDiagnosisCard(result)
	applyAssistantDiagnosisConfidence(result)
	refreshAssistantChatProgress(result)

	status := aiopsmodel.AgentRunStatusSucceeded
	message := ""
	if handleErr != nil {
		status = aiopsmodel.AgentRunStatusPartial
		message = handleErr.Error()
	}
	finalizeAssistantChatProgress(result, status)
	finishedAt := time.Now()
	refreshAssistantChatThinkingProcess(result, finishedAt, status)
	_ = s.updateAssistantChatRunResult(run.ID, result, finishedAt)
	_ = s.agentDao.FinishRun(run.ID, status, message, finishedAt)
	_ = s.agentDao.UpsertSession(&aiopsmodel.AgentSession{
		SessionID: req.SessionID,
		Title:     truncateAssistantChatText(req.Question, 80),
		Channel:   assistantChatChannel(req),
		UserID:    userID,
		Username:  username,
		Status:    "active",
		LastRunID: run.ID,
	})
	_ = s.agentDao.CreateMessage(&aiopsmodel.AgentMessage{
		SessionID:     req.SessionID,
		RunID:         run.ID,
		Role:          aiopsmodel.AgentMessageRoleAssistant,
		Content:       result.Answer,
		Intent:        result.Intent,
		EntitiesJSON:  compactToolJSON(result.Entities),
		ToolPlanJSON:  compactToolJSON(result.ToolPlan),
		TimelineJSON:  compactToolJSON(result.Timeline),
		DiagnosisJSON: compactToolJSON(result.DiagnosisCard),
		RawJSON:       compactToolJSON(result),
	})
	s.emitAssistantChatResultEvent(run.ID, status, result)
	s.emitAssistantChatDoneEvent(run.ID, status)
	return result, nil
}

func applyAssistantChatActionMode(result *aiopsmodel.AssistantChatResult) {
	if result == nil || len(result.PendingAction) == 0 {
		return
	}
	requiresConfirmation := assistantChatBool(result.PendingAction["requiresConfirmation"])
	confirmationID := firstNonEmptyAssistantChatString(result.PendingAction, "confirmationId")
	if requiresConfirmation && confirmationID != "" {
		result.Readonly = false
	}
}

func (s *AssistantChatService) findPendingAssistantAction(sessionID string) (map[string]interface{}, error) {
	messages, err := s.agentDao.ListRecentMessages(strings.TrimSpace(sessionID), defaultAssistantMemoryLimit)
	if err != nil {
		return nil, err
	}
	for index := len(messages) - 1; index >= 0; index-- {
		message := messages[index]
		if message.Role != aiopsmodel.AgentMessageRoleAssistant {
			continue
		}
		if strings.TrimSpace(message.RawJSON) == "" {
			continue
		}
		var previous aiopsmodel.AssistantChatResult
		if err := json.Unmarshal([]byte(message.RawJSON), &previous); err != nil {
			continue
		}
		pending := previous.PendingAction
		if message.Intent == aiopsmodel.AssistantChatIntentActionConfirmation &&
			(assistantChatBool(pending["executed"]) || assistantChatBool(pending["confirmed"]) || !assistantChatBool(pending["requiresConfirmation"])) {
			return nil, nil
		}
		if !assistantChatBool(pending["requiresConfirmation"]) || firstNonEmptyAssistantChatString(pending, "confirmationId") == "" {
			continue
		}
		if assistantChatBool(pending["executed"]) || assistantChatBool(pending["confirmed"]) {
			return nil, nil
		}
		return pending, nil
	}
	return nil, nil
}

func (s *AssistantChatService) handleAssistantActionConfirmation(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult, lookupErr error) error {
	if lookupErr != nil {
		return fmt.Errorf("读取待确认操作失败: %w", lookupErr)
	}
	pending := result.PendingAction
	confirmationID := firstNonEmptyAssistantChatString(pending, "confirmationId")
	if confirmationID == "" {
		result.Answer = "当前会话没有可确认的待执行操作，可能尚未生成预览、确认凭证已使用，或第三方没有沿用同一 sessionId。"
		result.SuggestedActions = []string{"请重新发起变更请求并核对预览，然后在同一会话回复“确认”。"}
		result.PendingAction = nil
		return nil
	}

	started := time.Now()
	startAssistantChatPlanStep(result, "confirm_pending_action")
	output, err := ConfirmMCPAction(c, confirmationID)
	finished := time.Now()
	if err != nil {
		finishAssistantChatPlanStep(result, "confirm_pending_action", err.Error())
		appendAssistantChatTimeline(result, 0, "confirm_pending_action", aiopsmodel.AssistantChatPlanFailed, "执行已签名的待确认操作", started, finished, err.Error())
		return err
	}
	finishAssistantChatPlanStep(result, "confirm_pending_action", "")
	appendAssistantChatTimeline(result, 0, "confirm_pending_action", aiopsmodel.AssistantChatPlanSucceeded, "执行已签名的待确认操作", started, finished, "")
	result.UsedTools = append(result.UsedTools, aiopsmodel.AssistantChatUsedTool{
		Name: "confirm_pending_action", DurationMS: finished.Sub(started).Milliseconds(), Success: true,
	})
	executed := assistantChatMap(output)
	result.PendingAction = executed
	result.Readonly = false
	result.Answer = firstNonEmptyAssistantChatString(executed, "message")
	if result.Answer == "" {
		result.Answer = "已确认并执行待处理操作。"
	}
	result.SuggestedActions = []string{"操作已触发；如返回 statusEndpoint，请继续查询实际执行进度，直到 succeeded 或 failed。"}
	return nil
}

func (s *AssistantChatService) handleAssistantHostPressure(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	target := extractAssistantChatHostTarget(req, result.Entities)
	hostID := uint(assistantChatNumber(result.Entities["hostId"]))
	var host map[string]interface{}
	if explicitIP := assistantChatQuestionIP(req.Question); explicitIP != "" {
		target = explicitIP
		hostID = 0
	}

	if hostID > 0 {
		tool, err := s.invokeAssistantTool(c, result, req.Question, "get_cmdb_host", map[string]interface{}{"hostId": hostID})
		if err != nil {
			return err
		}
		host = assistantChatMap(tool.Output)
	} else {
		if target == "" {
			return errors.New("没有识别到主机 IP、主机名或 hostId")
		}
		tool, err := s.resolveAssistantHostTool(c, req, result, target)
		if err != nil {
			return err
		}
		host = assistantChatResolvedHost(tool.Output)
		hostID = uint(assistantChatNumber(host["id"]))
	}
	mergeAssistantChatHostEntities(result, host)

	if hostID == 0 {
		return errors.New("没有在 CMDB 中定位到可用主机")
	}
	if assistantChatQuestionLooksLikePathInspection(req.Question) || assistantChatBool(result.Entities["pathInspection"]) {
		args := map[string]interface{}{
			"hostId": hostID, "path": firstNonEmptyAssistantChatString(result.Entities, "path"),
			"maxEntries": 100, "tailLines": 500,
		}
		if authID := assistantChatNumber(result.Entities["authId"]); authID > 0 {
			args["authId"] = authID
		}
		output, err := s.invokeAssistantMCPTool(c, result, req.Question, "ecs_inspect_path", args)
		if err != nil {
			return fmt.Errorf("主机路径检查失败: %w", err)
		}
		result.Answer = buildAssistantChatPathInspectionAnswer(host, assistantChatMap(output))
		result.SuggestedActions = []string{
			"目录结果如被截断，请指定更具体的子目录继续查看。",
			"文件内容仅返回最后 500 行并对常见凭据字段脱敏；涉及修改仍需通过受控变更流程。",
		}
		result.DiagnosisCard = buildAssistantPathInspectionCard(host, result.Evidence, result.MissingInfo, result.SuggestedActions)
		return nil
	}
	if assistantChatQuestionLooksLikeDiskUsageAnalysis(req.Question) || assistantChatBool(result.Entities["diskUsageAnalysis"]) {
		args := map[string]interface{}{
			"hostId": hostID, "path": "/", "depth": 2, "topN": 5, "minFileSizeMB": 100,
		}
		if authID := assistantChatNumber(result.Entities["authId"]); authID > 0 {
			args["authId"] = authID
		}
		output, err := s.invokeAssistantMCPTool(c, result, req.Question, "ecs_analyze_disk_usage", args)
		if err != nil {
			return fmt.Errorf("主机磁盘目录分析失败: %w", err)
		}
		result.Answer = buildAssistantChatDiskUsageAnswer(host, assistantChatMap(output))
		result.SuggestedActions = []string{
			"先确认大目录和大文件是否仍被业务使用，再决定归档或清理。",
			"需要清空 .log 或 .txt 文件时，先生成目标清单和固定命令，再由登录的系统管理员确认。",
		}
		result.DiagnosisCard = buildAssistantDiskUsageCard(host, result.Evidence, result.MissingInfo, result.SuggestedActions)
		return nil
	}

	agentTool, agentErr := s.invokeAssistantTool(c, result, req.Question, "get_agent_status", map[string]interface{}{
		"hostId": hostID,
	})
	if agentErr != nil {
		result.MissingInfo = append(result.MissingInfo, "Agent 状态查询失败："+agentErr.Error())
	}

	minutes := parseAssistantChatMinutes(req.Question)
	metricTool, metricErr := s.invokeAssistantTool(c, result, req.Question, "query_host_metrics", map[string]interface{}{
		"hostId":  hostID,
		"minutes": minutes,
		"step":    defaultAssistantChatStep,
	})
	if metricErr != nil {
		return metricErr
	}

	metrics := assistantChatMap(metricTool.Output)
	agent := assistantChatMapOutput(agentTool)
	result.Answer = buildAssistantChatHostPressureAnswer(host, agent, metrics, minutes)
	result.SuggestedActions = assistantChatHostPressureActions(metricTool.Output)
	result.DiagnosisCard = buildAssistantHostPressureCard(host, agent, metrics, minutes, result.Evidence, result.MissingInfo, result.SuggestedActions)
	return nil
}

func (s *AssistantChatService) handleAssistantHostServices(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	target := extractAssistantChatHostTarget(req, result.Entities)
	hostID := uint(assistantChatNumber(result.Entities["hostId"]))
	var host map[string]interface{}
	if explicitIP := assistantChatQuestionIP(req.Question); explicitIP != "" {
		target = explicitIP
		hostID = 0
	}

	if hostID > 0 {
		tool, err := s.invokeAssistantTool(c, result, req.Question, "get_cmdb_host", map[string]interface{}{"hostId": hostID})
		if err != nil {
			return err
		}
		host = assistantChatMap(tool.Output)
	} else {
		if target == "" {
			return errors.New("没有识别到主机 IP、主机名或 hostId")
		}
		tool, err := s.resolveAssistantHostTool(c, req, result, target)
		if err != nil {
			return err
		}
		host = assistantChatResolvedHost(tool.Output)
		hostID = uint(assistantChatNumber(host["id"]))
	}
	mergeAssistantChatHostEntities(result, host)
	if hostID == 0 {
		return errors.New("没有在 CMDB 中定位到可用主机")
	}

	agentTool, agentErr := s.invokeAssistantTool(c, result, req.Question, "get_agent_status", map[string]interface{}{
		"hostId": hostID,
	})
	if agentErr != nil {
		result.MissingInfo = append(result.MissingInfo, "Agent 状态查询失败："+agentErr.Error())
	} else if agent := assistantChatMapOutput(agentTool); assistantChatAgentUnavailable(agent) {
		result.Answer = buildAssistantHostAgentUnavailableAnswer(host, agent, "服务发现")
		result.SuggestedActions = []string{
			"先确认该主机采集 Agent 是否已安装并在线。",
			"如果 Agent 已安装，检查最近心跳、采集配置和 Prometheus target 状态。",
			"Agent 恢复后再重新查询 Top 进程和服务推断结果。",
		}
		result.DiagnosisCard = buildAssistantHostAgentUnavailableCard(host, agent, result.Evidence, result.MissingInfo, result.SuggestedActions)
		return nil
	}

	tool, err := s.invokeAssistantTool(c, result, req.Question, "query_host_services", map[string]interface{}{
		"hostId": hostID,
	})
	if err != nil {
		return err
	}
	output := assistantChatMap(tool.Output)
	result.Answer = buildAssistantChatHostServicesAnswer(host, output)
	result.SuggestedActions = assistantChatHostServicesActions(output)
	result.DiagnosisCard = buildAssistantHostServicesCard(host, output, result.Evidence, result.MissingInfo, result.SuggestedActions)
	return nil
}

func (s *AssistantChatService) handleAssistantHostLogCleanup(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	target := extractAssistantChatHostTarget(req, result.Entities)
	hostID := uint(assistantChatNumber(result.Entities["hostId"]))
	var host map[string]interface{}
	if explicitIP := assistantChatQuestionIP(req.Question); explicitIP != "" {
		target = explicitIP
		hostID = 0
	}

	if hostID > 0 {
		tool, err := s.invokeAssistantTool(c, result, req.Question, "get_cmdb_host", map[string]interface{}{"hostId": hostID})
		if err != nil {
			return err
		}
		host = assistantChatMap(tool.Output)
	} else {
		if target == "" {
			return errors.New("没有识别到主机 IP、主机名或 hostId")
		}
		tool, err := s.resolveAssistantHostTool(c, req, result, target)
		if err != nil {
			return err
		}
		host = assistantChatResolvedHost(tool.Output)
		hostID = uint(assistantChatNumber(host["id"]))
	}
	mergeAssistantChatHostEntities(result, host)
	if hostID == 0 {
		return errors.New("没有在 CMDB 中定位到可用主机")
	}

	args := map[string]interface{}{
		"hostId": hostID,
		"path":   firstNonEmptyAssistantChatString(result.Entities, "path"),
	}
	if authID := assistantChatNumber(result.Entities["authId"]); authID > 0 {
		args["authId"] = authID
	}
	output, err := s.invokeAssistantMCPTool(c, result, req.Question, "ecs_request_cleanup_logs", args)
	if err != nil {
		return fmt.Errorf("日志清空预览失败: %w", err)
	}
	preview := assistantChatMap(output)
	result.PendingAction = preview
	result.Answer = buildAssistantChatLogCleanupAnswer(host, preview)
	result.SuggestedActions = []string{
		"确认前核对主机、日志路径、当前大小和固定清空命令。",
		"确认接口会再次校验当前登录用户身份和系统管理员权限；未确认不会修改文件。",
	}
	result.DiagnosisCard = buildAssistantLogCleanupCard(host, preview, result.Evidence, result.MissingInfo, result.SuggestedActions)
	return nil
}

func (s *AssistantChatService) resolveAssistantHostTool(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult, target string) (*aiopsmodel.AgentToolInvokeResult, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.New("没有识别到主机 IP、主机名或 hostId")
	}
	if assistantChatLooksLikeIP(target) {
		tool, err := s.invokeAssistantTool(c, result, req.Question, "get_cmdb_host", map[string]interface{}{"ip": target})
		if err != nil {
			return nil, fmt.Errorf("没有在 CMDB 中查到主机 %s", target)
		}
		return tool, nil
	}
	tool, err := s.invokeAssistantTool(c, result, req.Question, "search_cmdb_hosts", map[string]interface{}{
		"keyword": target,
		"limit":   5,
	})
	if err != nil {
		return nil, err
	}
	if len(assistantChatSlice(assistantChatMap(tool.Output)["items"])) == 0 {
		return nil, fmt.Errorf("没有在 CMDB 中查到主机 %s", target)
	}
	return tool, nil
}

func (s *AssistantChatService) handleAssistantHostInventory(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	tool, err := s.invokeAssistantTool(c, result, req.Question, "aiops_context_hosts", map[string]interface{}{"limit": 10})
	if err != nil {
		return err
	}
	output := assistantChatMapOutput(tool)
	result.Answer = buildAssistantChatHostInventoryAnswer(output)
	result.SuggestedActions = []string{"可以继续查询主机认证状态、操作系统分布，或指定主机查看资源使用率和服务状态。"}
	return nil
}

func (s *AssistantChatService) handleAssistantAlertOverview(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	keyword := firstNonEmptyAssistantChatString(result.Entities, "keyword")
	if keyword == "" {
		keyword = firstNonEmptyAssistantChatString(result.Entities, "ip", "hostName", "hostname", "name", "instance")
	}
	tool, err := s.invokeAssistantTool(c, result, req.Question, "query_active_alerts", map[string]interface{}{
		"page":     1,
		"pageSize": 10,
		"keyword":  keyword,
		"severity": firstNonEmptyAssistantChatString(result.Entities, "severity"),
	})
	if err != nil {
		return err
	}
	result.Answer = buildAssistantChatAlertAnswer(tool.Output)
	result.SuggestedActions = []string{
		"优先查看 firing 且 severity 较高的告警。",
		"对单条告警继续发起诊断时，使用告警详情页的 AI 诊断入口。",
	}
	result.DiagnosisCard = buildAssistantAlertOverviewCard(tool.Output, result.Evidence, result.MissingInfo, result.SuggestedActions)
	return nil
}

func (s *AssistantChatService) handleAssistantPlatformOverview(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	context := map[string]interface{}{}
	var errs []string

	alertTool, alertErr := s.invokeAssistantTool(c, result, req.Question, "query_active_alerts", map[string]interface{}{
		"page":     1,
		"pageSize": 10,
	})
	if alertErr != nil {
		errs = append(errs, "活跃告警查询失败："+alertErr.Error())
	} else if alertTool != nil {
		context["alerts"] = assistantChatMap(alertTool.Output)
	}

	hostArgs := map[string]interface{}{"limit": 10}
	hostTool, hostErr := s.invokeAssistantTool(c, result, req.Question, "aiops_context_hosts", hostArgs)
	if hostErr != nil {
		errs = append(errs, "主机资产读取失败："+hostErr.Error())
	} else {
		hostOutput := assistantChatMapOutput(hostTool)
		context["hosts"] = hostOutput
	}

	dbArgs := map[string]interface{}{"scope": "all"}
	dbTool, dbErr := s.invokeAssistantTool(c, result, req.Question, "aiops_context_database", dbArgs)
	if dbErr != nil {
		errs = append(errs, "数据库资产读取失败："+dbErr.Error())
	} else {
		dbOutput := assistantChatMapOutput(dbTool)
		context["database"] = dbOutput
	}

	k8sArgs := map[string]interface{}{"scope": "all"}
	k8sTool, k8sErr := s.invokeAssistantTool(c, result, req.Question, "aiops_context_k8s", k8sArgs)
	if k8sErr != nil {
		errs = append(errs, "K8s 集群读取失败："+k8sErr.Error())
	} else {
		k8sOutput := assistantChatMapOutput(k8sTool)
		context["k8s"] = k8sOutput
	}

	if assistantChatQuestionNeedsKnowledge(req.Question) {
		items, knowledgeErr := s.searchAssistantKnowledge(c, req, result, 5)
		if knowledgeErr != nil {
			errs = append(errs, "知识库检索失败："+knowledgeErr.Error())
		} else {
			context["knowledge"] = map[string]interface{}{"items": items, "total": len(items)}
		}
	}

	result.MissingInfo = appendMissingStrings(result.MissingInfo, errs...)
	result.Answer = buildAssistantChatPlatformAnswer(context, errs)
	result.SuggestedActions = buildAssistantPlatformActions(context, errs)
	result.DiagnosisCard = buildAssistantPlatformOverviewCard(context, result.Evidence, result.MissingInfo, result.SuggestedActions)
	if len(context) == 0 && len(errs) > 0 {
		return errors.New(strings.Join(errs, "；"))
	}
	return nil
}

func (s *AssistantChatService) handleAssistantDatabaseOverview(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	args := map[string]interface{}{
		"assetId": assistantChatFirstEntityUInt(result.Entities, "assetId", "dbAssetId", "databaseAssetId", "pageResourceId"),
		"keyword": firstNonEmptyAssistantChatString(result.Entities, "keyword", "databaseName", "dbName", "instance", "ip"),
		"env":     firstNonEmptyAssistantChatString(result.Entities, "env", "environment"),
	}
	tool, err := s.invokeAssistantTool(c, result, req.Question, "aiops_context_database", args)
	if err != nil {
		return err
	}
	output := assistantChatMapOutput(tool)

	if assistantChatQuestionAsksDatabasePressure(req.Question) {
		result.Answer = buildAssistantChatDatabasePressureAnswer(output)
	} else {
		result.Answer = buildAssistantChatDatabaseAnswer(output)
	}
	result.SuggestedActions = []string{
		"如需分析慢 SQL，可进入数据库资产详情页发起慢日志分析任务。",
		"如需执行 SQL 或变更表结构，请走 SQL 审核工单，不由助手直接执行。",
		"如果问题关联具体实例，请在数据库详情页提问，前端传入 assetId 后我会聚焦该实例。",
	}
	result.DiagnosisCard = buildAssistantDatabaseOverviewCard(output, result.Evidence, result.MissingInfo, result.SuggestedActions)
	return nil
}

func (s *AssistantChatService) handleAssistantK8sOverview(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	if workloadName := firstNonEmptyAssistantChatString(result.Entities, "workloadName"); workloadName != "" && assistantChatQuestionAsksK8sWorkloadDiagnosis(req.Question) {
		return s.handleAssistantK8sWorkloadDiagnosis(c, req, result, workloadName)
	}
	if workloadName := firstNonEmptyAssistantChatString(result.Entities, "workloadName"); workloadName != "" && assistantChatQuestionAsksK8sWorkloadHistory(req.Question) {
		return s.handleAssistantK8sWorkloadHistory(c, req, result, workloadName)
	}
	if workloadName := firstNonEmptyAssistantChatString(result.Entities, "workloadName"); workloadName != "" && assistantChatQuestionAsksK8sReplicaCount(req.Question) {
		return s.handleAssistantK8sReplicaCount(c, req, result, workloadName)
	}
	if workloadName := firstNonEmptyAssistantChatString(result.Entities, "workloadName"); workloadName != "" && assistantChatQuestionAsksK8sWorkloadStatus(req.Question) {
		return s.handleAssistantK8sWorkloadStatus(c, req, result, workloadName)
	}
	listArgs := map[string]interface{}{
		"clusterId": assistantChatFirstEntityUInt(result.Entities, "clusterId", "k8sClusterId", "pageResourceId"),
		"keyword":   firstNonEmptyAssistantChatString(result.Entities, "clusterName"),
	}
	listOutput, err := s.invokeAssistantMCPTool(c, result, req.Question, "k8s_list_clusters", listArgs)
	if err != nil {
		return err
	}
	clusterContext := assistantChatMap(listOutput)
	clusterID := assistantChatFirstEntityUInt(result.Entities, "clusterId", "k8sClusterId", "pageResourceId")
	items := assistantChatSlice(clusterContext["items"])
	if clusterID == 0 {
		clusterID = selectAssistantK8sClusterID(req.Question, items)
	}
	if clusterID == 0 && len(items) == 1 {
		clusterID = uint(assistantChatNumber(assistantChatMap(items[0])["id"]))
	}

	if !assistantChatHasSpecificK8sResource(req.Question, result.Entities) {
		targets := selectAssistantK8sClusterTargets(clusterID, items)
		if len(targets) == 0 {
			result.Answer = buildAssistantChatK8sAnswer(clusterContext)
			result.MissingInfo = appendMissingStrings(result.MissingInfo, "没有找到授权范围内可检查的 K8s 集群。")
			result.SuggestedActions = []string{"检查集群资产登记和当前用户的 K8s 集群授权。"}
			result.DiagnosisCard = buildAssistantK8sOverviewCard(clusterContext, result.Evidence, result.MissingInfo, result.SuggestedActions)
			return nil
		}

		report := assistantK8sScanReport{
			RequestedClusters: int(assistantChatNumber(clusterContext["matched"])),
			Clusters:          make([]assistantK8sClusterScan, 0, len(targets)),
		}
		if report.RequestedClusters == 0 {
			report.RequestedClusters = len(targets)
		}
		if report.RequestedClusters > len(targets) {
			report.SkippedClusters = report.RequestedClusters - len(targets)
		}
		for _, target := range targets {
			scan := s.scanAssistantK8sCluster(c, req, result, target)
			report.Clusters = append(report.Clusters, scan)
			if scan.LiveChecks > 0 {
				report.ScannedClusters++
			}
		}
		if len(targets) == 1 {
			result.Entities["clusterId"] = targets[0].ID
			clusterContext["selectedClusterId"] = targets[0].ID
		} else {
			delete(result.Entities, "clusterId")
			clusterIDs := make([]uint, 0, len(targets))
			for _, target := range targets {
				clusterIDs = append(clusterIDs, target.ID)
			}
			result.Entities["clusterIds"] = clusterIDs
		}
		clusterContext["liveScan"] = report
		result.UsedTools = aggregateAssistantK8sUsedTools(result.UsedTools)
		result.Timeline = aggregateAssistantK8sTimeline(result.Timeline)
		result.Evidence = append(result.Evidence, aiopsmodel.AgentDiagnosisEvidence{
			Type: "k8s", Title: "K8s 授权范围实时健康扫描汇总", Reference: "k8s_health_scan_summary",
			Content: "已汇总各授权集群的实时节点、Pod、事件和指标检查结果", Raw: report,
		})
		result.Evidence = compactAssistantK8sScanEvidence(result.Evidence)
		result.MissingInfo = appendMissingStrings(result.MissingInfo, assistantK8sReportMissingInfo(report)...)
		result.Answer = buildAssistantChatK8sFleetAnswer(report)
		result.SuggestedActions = []string{
			"优先处理回答中列出的 NotReady/高重启/异常状态 Pod 和 Warning 事件。",
			"对 CPU 或内存热点 Pod，可继续指定集群、Namespace 和 Pod 获取状态、事件、日志与历史指标。",
			"涉及删除、重启、伸缩、改 YAML 的动作需要走确认或审批流程。",
		}
		result.DiagnosisCard = buildAssistantK8sOverviewCard(clusterContext, result.Evidence, result.MissingInfo, result.SuggestedActions)
		return nil
	}

	if clusterID == 0 {
		result.Answer = buildAssistantChatK8sAnswer(clusterContext)
		if len(items) > 1 {
			result.Answer += "\n问题指向了具体 K8s 资源，但检测到多个授权集群；请补充集群名称以避免检查同名资源。"
			result.MissingInfo = appendMissingStrings(result.MissingInfo, "需要明确 clusterId 或集群名称。")
		}
		result.SuggestedActions = []string{"选择一个集群后重新发起检查。"}
		result.DiagnosisCard = buildAssistantK8sOverviewCard(clusterContext, result.Evidence, result.MissingInfo, result.SuggestedActions)
		return nil
	}
	result.Entities["clusterId"] = clusterID
	clusterContext["selectedClusterId"] = clusterID

	checks := buildAssistantK8sRuntimeToolPlan(req.Question, result.Entities, clusterID)
	outputs := make(map[string]interface{}, len(checks))
	errs := make([]string, 0)
	for _, check := range checks {
		output, checkErr := s.invokeAssistantMCPTool(c, result, req.Question, check.ToolName, check.Arguments)
		if checkErr != nil {
			errs = append(errs, check.ToolName+"："+checkErr.Error())
			continue
		}
		outputs[check.ToolName] = output
	}
	result.MissingInfo = appendMissingStrings(result.MissingInfo, errs...)

	result.Answer = buildAssistantChatK8sLiveAnswer(clusterContext, outputs, errs)
	result.SuggestedActions = []string{
		"如需继续定位 Pod，请提供 namespace、podName 和容器名。",
		"如需分析工作负载或 Ingress，请提供资源类型、名称和 Namespace。",
		"涉及删除、重启、伸缩、改 YAML 的动作需要走确认或审批流程。",
	}
	result.DiagnosisCard = buildAssistantK8sOverviewCard(clusterContext, result.Evidence, result.MissingInfo, result.SuggestedActions)
	if len(outputs) == 0 && len(errs) > 0 {
		return errors.New(strings.Join(errs, "；"))
	}
	return nil
}

func (s *AssistantChatService) handleAssistantK8sWorkloadDiagnosis(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult, workloadName string) error {
	targets, discoveryErrs, err := s.discoverAssistantK8sWorkloadTargets(c, req, result, workloadName)
	if err != nil {
		return err
	}
	result.MissingInfo = appendMissingStrings(result.MissingInfo, discoveryErrs...)
	if len(targets) == 0 {
		result.Answer = fmt.Sprintf("已检查当前用户授权范围内的 K8s 集群和 Namespace，没有找到名为 %s 的 Deployment。", workloadName)
		result.SuggestedActions = []string{"核对工作负载名称，或确认当前账号拥有目标集群和 Namespace 的读取权限。"}
		return nil
	}
	if len(targets) > 1 {
		result.Entities["workloadCandidates"] = targets
		result.Answer = fmt.Sprintf("已找到 %d 个名为 %s 的 Deployment。请先选择集群和 Namespace，再执行异常诊断。", len(targets), workloadName)
		result.ClarifyQuestions = []aiopsmodel.AssistantChatClarifyQuestion{{
			Field:       "k8sTarget",
			Question:    "找到多个同名 Deployment，请选择要诊断的集群和 Namespace。",
			Suggestions: assistantK8sWorkloadTargetSuggestions(targets),
		}}
		result.SuggestedActions = result.ClarifyQuestions[0].Suggestions
		return nil
	}

	target := targets[0]
	result.Entities["clusterId"] = target.ClusterID
	result.Entities["clusterName"] = target.ClusterName
	result.Entities["namespace"] = target.Namespace
	result.Entities["workloadType"] = target.WorkloadType
	output, err := s.invokeAssistantMCPTool(c, result, req.Question, "k8s_diagnose_workload", map[string]interface{}{
		"clusterId": target.ClusterID, "namespace": target.Namespace,
		"workloadType": target.WorkloadType, "workloadName": target.WorkloadName,
		"tailLines": 500, "limit": 200, "eventLimit": 100,
	})
	if err != nil {
		return fmt.Errorf("K8s 工作负载诊断失败: %w", err)
	}
	diagnosis := assistantChatMap(output)
	result.Answer = firstNonEmptyAssistantChatString(diagnosis, "analysis", "conclusion", "summary")
	if result.Answer == "" {
		return errors.New("K8s 工作负载诊断没有返回结论")
	}
	result.UsedLLM = true
	result.ProviderID = uint(assistantChatNumber(diagnosis["providerId"]))
	result.ProviderName = firstNonEmptyAssistantChatString(diagnosis, "providerName")
	result.ModelCode = firstNonEmptyAssistantChatString(diagnosis, "modelCode")
	result.SuggestedActions = assistantChatStringSlice(diagnosis["remediation"])
	result.DiagnosisCard = buildAssistantK8sWorkloadDiagnosisCard(target, diagnosis, result.Evidence, result.MissingInfo, result.SuggestedActions)
	return nil
}

func buildAssistantK8sWorkloadDiagnosisCard(target assistantK8sWorkloadTarget, diagnosis map[string]interface{}, evidence []aiopsmodel.AgentDiagnosisEvidence, missing, actions []string) *aiopsmodel.AssistantChatDiagnosisCard {
	severity := aiopsmodel.AssistantChatSeverityWarning
	switch strings.ToLower(firstNonEmptyAssistantChatString(diagnosis, "severity")) {
	case "critical", "严重", "致命":
		severity = aiopsmodel.AssistantChatSeverityCritical
	case "info", "正常", "健康":
		severity = aiopsmodel.AssistantChatSeverityInfo
	}
	return &aiopsmodel.AssistantChatDiagnosisCard{
		Summary:           firstNonEmptyAssistantChatString(diagnosis, "summary", "conclusion"),
		Severity:          severity,
		ImpactedResources: []string{fmt.Sprintf("%s/%s/%s", target.ClusterName, target.Namespace, target.WorkloadName)},
		Evidence:          evidence,
		NextChecks:        actions,
		Remediation:       actions,
		HandoffNote:       "本次复用 K8s 页面 Pod AI 分析引擎，已采集工作负载、Pod、当前及 previous 日志、事件、资源、探针、Service、YAML 和可用指标。",
		MissingData:       append([]string{}, missing...),
	}
}

func (s *AssistantChatService) handleAssistantK8sWorkloadStatus(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult, workloadName string) error {
	targets, discoveryErrs, err := s.discoverAssistantK8sWorkloadTargets(c, req, result, workloadName)
	if err != nil {
		return err
	}
	result.MissingInfo = appendMissingStrings(result.MissingInfo, discoveryErrs...)
	if len(targets) == 0 {
		result.Answer = fmt.Sprintf("已检查当前用户授权范围内的 K8s 集群和 Namespace，没有找到名为 %s 的 Deployment。", workloadName)
		result.SuggestedActions = []string{"核对工作负载名称，或确认当前账号拥有目标集群和 Namespace 的读取权限。"}
		return nil
	}
	if len(targets) > 1 {
		result.Entities["workloadCandidates"] = targets
		result.Answer = fmt.Sprintf("已找到 %d 个名为 %s 的 Deployment。请先选择集群和 Namespace，再查询当前状态。", len(targets), workloadName)
		result.ClarifyQuestions = []aiopsmodel.AssistantChatClarifyQuestion{{
			Field:       "k8sTarget",
			Question:    "找到多个同名 Deployment，请选择要查看状态的集群和 Namespace。",
			Suggestions: assistantK8sWorkloadTargetSuggestions(targets),
		}}
		result.SuggestedActions = result.ClarifyQuestions[0].Suggestions
		return nil
	}

	target := targets[0]
	result.Entities["clusterId"] = target.ClusterID
	result.Entities["clusterName"] = target.ClusterName
	result.Entities["namespace"] = target.Namespace
	result.Entities["workloadType"] = target.WorkloadType
	statusOutput, err := s.invokeAssistantMCPTool(c, result, req.Question, "k8s_get_workload_status", map[string]interface{}{
		"clusterId": target.ClusterID, "namespace": target.Namespace,
		"workloadType": target.WorkloadType, "workloadName": target.WorkloadName,
	})
	if err != nil {
		return fmt.Errorf("读取 K8s 工作负载状态失败: %w", err)
	}
	status := assistantChatMap(statusOutput)
	healthy := assistantK8sWorkloadStatusHealthy(status)
	result.WorkloadStatus = buildAssistantK8sWorkloadStatus(target, status, healthy)
	result.Answer = buildAssistantK8sWorkloadStatusAnswer(target, status)
	if healthy {
		result.SuggestedActions = []string{"当前 K8s 状态正常；如需排查具体业务错误，请明确提供异常现象或要求执行日志诊断。"}
	} else {
		result.SuggestedActions = []string{"工作负载未完全就绪；如需定位原因，可继续执行 Pod 事件、探针和日志诊断。"}
	}
	severity := aiopsmodel.AssistantChatSeverityWarning
	if healthy {
		severity = aiopsmodel.AssistantChatSeverityInfo
	}
	result.DiagnosisCard = &aiopsmodel.AssistantChatDiagnosisCard{
		Summary:           result.Answer,
		Severity:          severity,
		ImpactedResources: []string{fmt.Sprintf("%s/%s/%s", target.ClusterName, target.Namespace, target.WorkloadName)},
		Evidence:          result.Evidence,
		NextChecks:        result.SuggestedActions,
		HandoffNote:       "本次只读取工作负载当前详情，未读取应用日志，也未执行根因分析或任何变更。",
		MissingData:       append([]string{}, result.MissingInfo...),
	}
	return nil
}

func buildAssistantK8sWorkloadStatus(target assistantK8sWorkloadTarget, status map[string]interface{}, healthy bool) *aiopsmodel.AssistantChatWorkloadStatus {
	pods := make([]aiopsmodel.AssistantChatWorkloadStatusPod, 0)
	for _, item := range assistantChatSlice(status["pods"]) {
		pod := assistantChatMap(item)
		pods = append(pods, aiopsmodel.AssistantChatWorkloadStatusPod{
			Name:         firstNonEmptyAssistantChatString(pod, "name"),
			Status:       firstNonEmptyAssistantChatString(pod, "status", "phase"),
			Phase:        firstNonEmptyAssistantChatString(pod, "phase", "status"),
			RestartCount: int(assistantChatNumber(pod["restartCount"])),
			NodeName:     firstNonEmptyAssistantChatString(pod, "nodeName"),
			PodIP:        firstNonEmptyAssistantChatString(pod, "podIP"),
			RunningTime:  firstNonEmptyAssistantChatString(pod, "runningTime"),
		})
	}
	warningEvents := 0
	for _, item := range assistantChatSlice(status["events"]) {
		if strings.EqualFold(firstNonEmptyAssistantChatString(assistantChatMap(item), "type"), "Warning") {
			warningEvents++
		}
	}
	return &aiopsmodel.AssistantChatWorkloadStatus{
		ClusterID:         target.ClusterID,
		ClusterName:       target.ClusterName,
		Namespace:         target.Namespace,
		WorkloadType:      target.WorkloadType,
		WorkloadName:      target.WorkloadName,
		Status:            firstNonEmptyAssistantChatString(status, "status"),
		Healthy:           healthy,
		DesiredReplicas:   int(assistantChatNumber(status["desiredReplicas"])),
		CurrentReplicas:   int(assistantChatNumber(status["currentReplicas"])),
		ReadyReplicas:     int(assistantChatNumber(status["readyReplicas"])),
		AvailableReplicas: int(assistantChatNumber(status["availableReplicas"])),
		Images:            assistantChatStringSlice(status["images"]),
		Pods:              pods,
		WarningEvents:     warningEvents,
	}
}

func buildAssistantK8sWorkloadStatusAnswer(target assistantK8sWorkloadTarget, status map[string]interface{}) string {
	desired := int(assistantChatNumber(status["desiredReplicas"]))
	current := int(assistantChatNumber(status["currentReplicas"]))
	ready := int(assistantChatNumber(status["readyReplicas"]))
	available := int(assistantChatNumber(status["availableReplicas"]))
	state := firstNonEmptyAssistantChatString(status, "status")
	healthLabel := "未完全就绪"
	if desired == 0 && current == 0 {
		healthLabel = "已缩容至 0"
	} else if assistantK8sWorkloadStatusHealthy(status) {
		healthLabel = "正常"
	}

	parts := []string{fmt.Sprintf("集群 %s、Namespace %s 中的 %s/%s 当前状态：%s", target.ClusterName, target.Namespace, target.WorkloadType, target.WorkloadName, healthLabel)}
	if state != "" {
		parts[0] += fmt.Sprintf("（K8s 状态 %s）", state)
	}
	parts = append(parts, fmt.Sprintf("副本：期望 %d，当前 %d，Ready %d，可用 %d", desired, current, ready, available))
	if images := assistantChatStringSlice(status["images"]); len(images) > 0 {
		parts = append(parts, "镜像："+strings.Join(images, "、"))
	}
	if pods := assistantChatSlice(status["pods"]); len(pods) > 0 {
		podParts := make([]string, 0, len(pods))
		for _, item := range pods {
			pod := assistantChatMap(item)
			podParts = append(podParts, fmt.Sprintf("%s %s，重启 %d 次，节点 %s",
				firstNonEmptyAssistantChatString(pod, "name"), firstNonEmptyAssistantChatString(pod, "phase", "status"),
				int(assistantChatNumber(pod["restartCount"])), firstNonEmptyAssistantChatString(pod, "nodeName")))
		}
		parts = append(parts, "Pod："+strings.Join(podParts, "；"))
	}
	warnings := 0
	for _, item := range assistantChatSlice(status["events"]) {
		if strings.EqualFold(firstNonEmptyAssistantChatString(assistantChatMap(item), "type"), "Warning") {
			warnings++
		}
	}
	if warnings == 0 {
		parts = append(parts, "当前详情未返回 Warning 事件")
	} else {
		parts = append(parts, fmt.Sprintf("当前返回 %d 条 Warning 事件", warnings))
	}
	return strings.Join(parts, "。") + "。"
}

func assistantK8sWorkloadStatusHealthy(status map[string]interface{}) bool {
	desired := int(assistantChatNumber(status["desiredReplicas"]))
	current := int(assistantChatNumber(status["currentReplicas"]))
	ready := int(assistantChatNumber(status["readyReplicas"]))
	available := int(assistantChatNumber(status["availableReplicas"]))
	if desired == 0 {
		return current == 0 && ready == 0 && available == 0
	}
	return strings.EqualFold(firstNonEmptyAssistantChatString(status, "status"), "Running") &&
		current == desired && ready == desired && available == desired
}

func (s *AssistantChatService) handleAssistantK8sWorkloadHistory(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult, workloadName string) error {
	targets, discoveryErrs, err := s.discoverAssistantK8sWorkloadTargets(c, req, result, workloadName)
	if err != nil {
		return err
	}
	result.MissingInfo = appendMissingStrings(result.MissingInfo, discoveryErrs...)
	if len(targets) == 0 {
		result.Answer = fmt.Sprintf("已检查当前用户授权范围内的 K8s 集群和 Namespace，没有找到名为 %s 的 Deployment。", workloadName)
		result.SuggestedActions = []string{"核对工作负载名称，或确认当前账号拥有目标集群和 Namespace 的读取权限。"}
		return nil
	}
	if len(targets) > 1 {
		result.Entities["workloadCandidates"] = targets
		result.Answer = fmt.Sprintf("已找到 %d 个名为 %s 的 Deployment。请先选择集群和 Namespace，再查看历史版本。", len(targets), workloadName)
		result.ClarifyQuestions = []aiopsmodel.AssistantChatClarifyQuestion{{
			Field:       "k8sTarget",
			Question:    "找到多个同名 Deployment，请选择要查看历史版本的集群和 Namespace。",
			Suggestions: assistantK8sWorkloadTargetSuggestions(targets),
		}}
		result.SuggestedActions = result.ClarifyQuestions[0].Suggestions
		return nil
	}

	target := targets[0]
	result.Entities["clusterId"] = target.ClusterID
	result.Entities["clusterName"] = target.ClusterName
	result.Entities["namespace"] = target.Namespace
	result.Entities["workloadType"] = target.WorkloadType
	historyOutput, err := s.invokeAssistantMCPTool(c, result, req.Question, "k8s_get_workload_history", map[string]interface{}{
		"clusterId": target.ClusterID, "namespace": target.Namespace, "workloadName": target.WorkloadName,
	})
	if err != nil {
		return fmt.Errorf("读取 K8s 工作负载历史版本失败: %w", err)
	}
	history := assistantChatMap(historyOutput)
	result.Entities["currentRevision"] = int64(assistantChatNumber(history["currentRevision"]))
	result.Entities["availableRevisions"] = assistantK8sHistoryRevisions(history)
	result.WorkloadHistory = buildAssistantK8sWorkloadHistory(target, history, 10)
	result.Answer = buildAssistantK8sWorkloadHistoryAnswer(result.WorkloadHistory)
	result.SuggestedActions = assistantK8sWorkloadHistoryActions(result.WorkloadHistory)
	return nil
}

func assistantK8sHistoryRevisions(history map[string]interface{}) []int64 {
	revisions := make([]int64, 0)
	for _, raw := range assistantChatSlice(history["revisions"]) {
		if revision := int64(assistantChatNumber(assistantChatMap(raw)["revision"])); revision > 0 {
			revisions = append(revisions, revision)
		}
	}
	return revisions
}

func buildAssistantK8sWorkloadHistory(target assistantK8sWorkloadTarget, history map[string]interface{}, limit int) *aiopsmodel.AssistantChatWorkloadHistory {
	if limit <= 0 {
		limit = 10
	}
	currentRevision := int64(assistantChatNumber(history["currentRevision"]))
	revisions := assistantChatSlice(history["revisions"])
	items := make([]aiopsmodel.AssistantChatWorkloadHistoryItem, 0, len(revisions))
	for _, raw := range revisions {
		revision := assistantChatMap(raw)
		revisionNumber := int64(assistantChatNumber(revision["revision"]))
		if revisionNumber <= 0 {
			continue
		}
		status := strings.ToLower(firstNonEmptyAssistantChatString(revision, "status"))
		current := revisionNumber == currentRevision || status == "current"
		if current {
			status = "current"
		} else {
			status = "historical"
		}
		images := assistantChatStringSlice(revision["images"])
		items = append(items, aiopsmodel.AssistantChatWorkloadHistoryItem{
			Revision:     revisionNumber,
			CreationTime: assistantK8sHistoryTime(firstNonEmptyAssistantChatString(revision, "creationTime")),
			ImageVersion: assistantK8sImageVersions(images),
			Images:       images,
			Status:       status,
			Current:      current,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Revision > items[j].Revision
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return &aiopsmodel.AssistantChatWorkloadHistory{
		ClusterID:       target.ClusterID,
		ClusterName:     target.ClusterName,
		Namespace:       target.Namespace,
		WorkloadType:    target.WorkloadType,
		WorkloadName:    target.WorkloadName,
		CurrentRevision: currentRevision,
		Total:           len(revisions),
		Limit:           limit,
		Items:           items,
	}
}

func assistantK8sHistoryTime(value string) string {
	value = strings.TrimSpace(value)
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.Format("2006-01-02 15:04:05")
	}
	if value == "" {
		return "-"
	}
	return value
}

func assistantK8sImageVersions(images []string) string {
	versions := make([]string, 0, len(images))
	seen := map[string]struct{}{}
	for _, image := range images {
		image = strings.TrimSpace(image)
		if image == "" {
			continue
		}
		version := image
		if digestAt := strings.LastIndex(image, "@"); digestAt >= 0 && digestAt+1 < len(image) {
			version = image[digestAt+1:]
		} else {
			lastSlash := strings.LastIndex(image, "/")
			if tagAt := strings.LastIndex(image, ":"); tagAt > lastSlash && tagAt+1 < len(image) {
				version = image[tagAt+1:]
			}
		}
		if _, exists := seen[version]; exists {
			continue
		}
		seen[version] = struct{}{}
		versions = append(versions, version)
	}
	if len(versions) == 0 {
		return "-"
	}
	return strings.Join(versions, ", ")
}

func buildAssistantK8sWorkloadHistoryAnswer(history *aiopsmodel.AssistantChatWorkloadHistory) string {
	if history == nil || len(history.Items) == 0 {
		if history == nil {
			return "没有可展示的历史版本。"
		}
		return fmt.Sprintf("%s/%s 没有可展示的历史版本。", history.WorkloadType, history.WorkloadName)
	}
	var answer strings.Builder
	fmt.Fprintf(&answer, "%s/%s 最近 %d 个历史版本：\n\n| 时间 | 版本号 | 镜像版本 | 状态 |\n|---|---:|---|---|",
		history.WorkloadType, history.WorkloadName, len(history.Items))
	for _, item := range history.Items {
		status := "历史版本"
		if item.Current {
			status = "当前版本"
		}
		fmt.Fprintf(&answer, "\n| %s | %d | %s | %s |", item.CreationTime, item.Revision, item.ImageVersion, status)
	}
	return answer.String()
}

func assistantK8sWorkloadHistoryActions(history *aiopsmodel.AssistantChatWorkloadHistory) []string {
	if history == nil {
		return nil
	}
	actions := make([]string, 0, len(history.Items))
	for _, item := range history.Items {
		if item.Current {
			continue
		}
		actions = append(actions, fmt.Sprintf("回滚到 revision %d", item.Revision))
	}
	return actions
}

func (s *AssistantChatService) handleAssistantK8sReplicaCount(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult, workloadName string) error {
	targets, discoveryErrs, err := s.discoverAssistantK8sWorkloadTargets(c, req, result, workloadName)
	if err != nil {
		return err
	}
	result.MissingInfo = appendMissingStrings(result.MissingInfo, discoveryErrs...)
	if len(targets) == 0 {
		result.Answer = fmt.Sprintf("已检查当前用户授权范围内的 K8s 集群和 Namespace，没有找到名为 %s 的 Deployment。", workloadName)
		result.SuggestedActions = []string{"核对工作负载名称，或确认当前账号拥有目标集群和 Namespace 的读取权限。"}
		return nil
	}
	if len(targets) > 1 {
		result.Entities["workloadCandidates"] = targets
		result.Answer = fmt.Sprintf("已找到 %d 个名为 %s 的 Deployment。请先选择集群和 Namespace，再查询副本状态。", len(targets), workloadName)
		result.ClarifyQuestions = []aiopsmodel.AssistantChatClarifyQuestion{{
			Field:       "k8sTarget",
			Question:    "找到多个同名 Deployment，请选择要查看的集群和 Namespace。",
			Suggestions: assistantK8sWorkloadTargetSuggestions(targets),
		}}
		result.SuggestedActions = result.ClarifyQuestions[0].Suggestions
		return nil
	}

	target := targets[0]
	result.Entities["clusterId"] = target.ClusterID
	result.Entities["clusterName"] = target.ClusterName
	result.Entities["namespace"] = target.Namespace
	result.Entities["workloadType"] = target.WorkloadType
	statusOutput, err := s.invokeAssistantMCPTool(c, result, req.Question, "k8s_get_workload_status", map[string]interface{}{
		"clusterId": target.ClusterID, "namespace": target.Namespace,
		"workloadType": target.WorkloadType, "workloadName": target.WorkloadName,
	})
	if err != nil {
		return fmt.Errorf("读取 K8s 工作负载副本状态失败: %w", err)
	}
	status := assistantChatMap(statusOutput)
	result.Answer = buildAssistantK8sReplicaCountAnswer(target, status)
	result.SuggestedActions = []string{"如需调整副本，请明确目标副本数或增减数量；变更会先生成确认预览。"}
	return nil
}

func buildAssistantK8sReplicaCountAnswer(target assistantK8sWorkloadTarget, status map[string]interface{}) string {
	desired := int(assistantChatNumber(status["desiredReplicas"]))
	if _, exists := status["desiredReplicas"]; !exists {
		desired = int(assistantChatNumber(status["replicas"]))
	}
	current := int(assistantChatNumber(status["currentReplicas"]))
	ready := int(assistantChatNumber(status["readyReplicas"]))
	available := int(assistantChatNumber(status["availableReplicas"]))
	return fmt.Sprintf("集群 %s、Namespace %s 中的 %s/%s：期望副本 %d，当前副本 %d，Ready %d，Available %d。",
		target.ClusterName, target.Namespace, target.WorkloadType, target.WorkloadName, desired, current, ready, available)
}

func (s *AssistantChatService) handleAssistantK8sRolloutRestart(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	workloadName := firstNonEmptyAssistantChatString(result.Entities, "workloadName")
	targets, discoveryErrs, err := s.discoverAssistantK8sWorkloadTargets(c, req, result, workloadName)
	if err != nil {
		return err
	}
	result.MissingInfo = appendMissingStrings(result.MissingInfo, discoveryErrs...)
	if len(targets) == 0 {
		result.Answer = fmt.Sprintf("已检查当前用户授权范围内的 K8s 集群和 Namespace，没有找到名为 %s 且支持滚动重启的工作负载。", workloadName)
		result.SuggestedActions = []string{"核对工作负载名称，或确认当前账号拥有目标集群和 Namespace 的读取权限。"}
		return nil
	}
	if len(targets) > 1 {
		result.Entities["workloadCandidates"] = targets
		result.PendingAction = map[string]interface{}{
			"requiresSelection": true,
			"action":            "k8s_request_rollout_restart",
			"workloadName":      workloadName,
			"candidates":        targets,
		}
		result.Answer = buildAssistantK8sWorkloadChoiceAnswer(workloadName, targets)
		result.ClarifyQuestions = []aiopsmodel.AssistantChatClarifyQuestion{{
			Field:       "k8sTarget",
			Question:    "找到多个同名工作负载，请选择要重启的集群、Namespace 和类型。",
			Suggestions: assistantK8sWorkloadTargetSuggestions(targets),
		}}
		result.SuggestedActions = result.ClarifyQuestions[0].Suggestions
		return nil
	}

	target := targets[0]
	result.Entities["clusterId"] = target.ClusterID
	result.Entities["clusterName"] = target.ClusterName
	result.Entities["namespace"] = target.Namespace
	result.Entities["workloadType"] = target.WorkloadType
	args := map[string]interface{}{
		"clusterId":    target.ClusterID,
		"namespace":    target.Namespace,
		"workloadType": target.WorkloadType,
		"workloadName": target.WorkloadName,
	}
	output, err := s.invokeAssistantMCPTool(c, result, req.Question, "k8s_request_rollout_restart", args)
	if err != nil {
		return fmt.Errorf("K8s 滚动重启预览失败: %w", err)
	}
	preview := assistantChatMap(output)
	preview["clusterName"] = target.ClusterName
	result.PendingAction = preview
	result.Answer = fmt.Sprintf("已在集群 %s、Namespace %s 中找到 %s/%s，并生成滚动重启预览。请确认是否重启；当前尚未执行。",
		target.ClusterName, target.Namespace, target.WorkloadType, target.WorkloadName)
	result.SuggestedActions = []string{"通过确认接口提交 confirmationId 后才会执行滚动重启。", "确认凭证 10 分钟内有效且只能使用一次。"}
	return nil
}

func (s *AssistantChatService) handleAssistantK8sScaleWorkload(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	workloadName := firstNonEmptyAssistantChatString(result.Entities, "workloadName")
	replicas := int(assistantChatNumber(result.Entities["replicas"]))
	replicaDelta := int(assistantChatNumber(result.Entities["replicaDelta"]))
	targets, discoveryErrs, err := s.discoverAssistantK8sWorkloadTargets(c, req, result, workloadName)
	if err != nil {
		return err
	}
	result.MissingInfo = appendMissingStrings(result.MissingInfo, discoveryErrs...)
	if len(targets) == 0 {
		result.Answer = fmt.Sprintf("已检查当前用户授权范围内的 K8s 集群和 Namespace，没有找到名为 %s 且支持扩缩容的 Deployment。", workloadName)
		result.SuggestedActions = []string{"核对工作负载名称，或确认当前账号拥有目标集群和 Namespace 的读取权限。"}
		return nil
	}
	if len(targets) > 1 {
		result.Entities["workloadCandidates"] = targets
		result.PendingAction = map[string]interface{}{
			"requiresSelection": true,
			"action":            "k8s_request_scale_workload",
			"workloadName":      workloadName,
			"replicas":          replicas,
			"replicaDelta":      replicaDelta,
			"candidates":        targets,
		}
		result.Answer = fmt.Sprintf("已找到 %d 个名为 %s 的 Deployment。为避免误操作，请先选择要扩缩容的集群和 Namespace；当前尚未生成确认凭证。", len(targets), workloadName)
		result.ClarifyQuestions = []aiopsmodel.AssistantChatClarifyQuestion{{
			Field:       "k8sTarget",
			Question:    "找到多个同名 Deployment，请选择要调整副本数的集群和 Namespace。",
			Suggestions: assistantK8sWorkloadTargetSuggestions(targets),
		}}
		result.SuggestedActions = result.ClarifyQuestions[0].Suggestions
		return nil
	}

	target := targets[0]
	result.Entities["clusterId"] = target.ClusterID
	result.Entities["clusterName"] = target.ClusterName
	result.Entities["namespace"] = target.Namespace
	result.Entities["workloadType"] = target.WorkloadType
	currentReplicas := -1
	if replicaDelta != 0 {
		statusArgs := map[string]interface{}{
			"clusterId": target.ClusterID, "namespace": target.Namespace,
			"workloadType": target.WorkloadType, "workloadName": target.WorkloadName,
		}
		statusOutput, statusErr := s.invokeAssistantMCPTool(c, result, req.Question, "k8s_get_workload_status", statusArgs)
		if statusErr != nil {
			return fmt.Errorf("读取 K8s 工作负载当前副本数失败: %w", statusErr)
		}
		var ok bool
		currentReplicas, ok = assistantK8sDesiredReplicas(statusOutput)
		if !ok {
			return errors.New("K8s 工作负载详情未返回期望副本数，无法计算相对扩缩容目标")
		}
		replicas = currentReplicas + replicaDelta
		if replicas < 0 {
			return fmt.Errorf("当前副本数为 %d，不能减少 %d 个副本", currentReplicas, -replicaDelta)
		}
		result.Entities["currentReplicas"] = currentReplicas
		result.Entities["replicas"] = replicas
	}
	args := map[string]interface{}{
		"clusterId":    target.ClusterID,
		"namespace":    target.Namespace,
		"workloadType": target.WorkloadType,
		"workloadName": target.WorkloadName,
		"replicas":     replicas,
	}
	if currentReplicas >= 0 {
		args["expectedReplicas"] = currentReplicas
	}
	output, err := s.invokeAssistantMCPTool(c, result, req.Question, "k8s_request_scale_workload", args)
	if err != nil {
		return fmt.Errorf("K8s 工作负载扩缩容预览失败: %w", err)
	}
	preview := assistantChatMap(output)
	preview["clusterName"] = target.ClusterName
	if currentReplicas >= 0 {
		preview["currentReplicas"] = currentReplicas
		preview["replicaDelta"] = replicaDelta
	}
	result.PendingAction = preview
	if currentReplicas >= 0 {
		result.Answer = fmt.Sprintf("已在集群 %s、Namespace %s 中找到 %s/%s，实时期望副本数为 %d；按请求计算目标为 %d，并生成确认预览。当前尚未执行。",
			target.ClusterName, target.Namespace, target.WorkloadType, target.WorkloadName, currentReplicas, replicas)
	} else {
		result.Answer = fmt.Sprintf("已在集群 %s、Namespace %s 中找到 %s/%s，并生成调整为 %d 个副本的确认预览。当前尚未执行。",
			target.ClusterName, target.Namespace, target.WorkloadType, target.WorkloadName, replicas)
	}
	result.SuggestedActions = []string{"确认后才会调整工作负载副本数。", "确认凭证 10 分钟内有效且只能使用一次。"}
	return nil
}

func (s *AssistantChatService) handleAssistantK8sRollbackWorkload(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	workloadName := firstNonEmptyAssistantChatString(result.Entities, "workloadName")
	targets, discoveryErrs, err := s.discoverAssistantK8sWorkloadTargets(c, req, result, workloadName)
	if err != nil {
		return err
	}
	result.MissingInfo = appendMissingStrings(result.MissingInfo, discoveryErrs...)
	if len(targets) == 0 {
		result.Answer = fmt.Sprintf("已检查当前用户授权范围内的 K8s 集群和 Namespace，没有找到名为 %s 且支持回滚的 Deployment。", workloadName)
		result.SuggestedActions = []string{"核对工作负载名称，或确认当前账号拥有目标集群和 Namespace 的读取权限。"}
		return nil
	}
	if len(targets) > 1 {
		result.Entities["workloadCandidates"] = targets
		result.PendingAction = map[string]interface{}{
			"requiresSelection": true,
			"action":            "k8s_request_rollback_workload",
			"workloadName":      workloadName,
			"revision":          result.Entities["revision"],
			"imageVersion":      result.Entities["imageVersion"],
			"candidates":        targets,
		}
		result.Answer = fmt.Sprintf("已找到 %d 个名为 %s 的 Deployment。为避免误操作，请先选择要回滚的集群和 Namespace；当前尚未生成确认凭证。", len(targets), workloadName)
		result.ClarifyQuestions = []aiopsmodel.AssistantChatClarifyQuestion{{
			Field:       "k8sTarget",
			Question:    "找到多个同名 Deployment，请选择要回滚的集群和 Namespace。",
			Suggestions: assistantK8sWorkloadTargetSuggestions(targets),
		}}
		result.SuggestedActions = result.ClarifyQuestions[0].Suggestions
		return nil
	}

	target := targets[0]
	result.Entities["clusterId"] = target.ClusterID
	result.Entities["clusterName"] = target.ClusterName
	result.Entities["namespace"] = target.Namespace
	result.Entities["workloadType"] = target.WorkloadType
	args := map[string]interface{}{
		"clusterId": target.ClusterID, "namespace": target.Namespace,
		"workloadType": target.WorkloadType, "workloadName": target.WorkloadName,
	}
	if revision := int64(assistantChatNumber(result.Entities["revision"])); revision > 0 {
		args["revision"] = revision
	} else if imageVersion := firstNonEmptyAssistantChatString(result.Entities, "imageVersion"); imageVersion != "" {
		args["imageVersion"] = imageVersion
	}
	output, err := s.invokeAssistantMCPTool(c, result, req.Question, "k8s_request_rollback_workload", args)
	if err != nil {
		return fmt.Errorf("K8s 工作负载回滚预览失败: %w", err)
	}
	preview := assistantChatMap(output)
	preview["clusterName"] = target.ClusterName
	result.PendingAction = preview
	change := assistantChatMap(preview["change"])
	resolvedRevision := int64(assistantChatNumber(change["toRevision"]))
	result.Entities["revision"] = resolvedRevision
	if imageVersion := firstNonEmptyAssistantChatString(change, "imageVersion"); imageVersion != "" {
		result.Entities["imageVersion"] = imageVersion
		result.Answer = fmt.Sprintf("已在集群 %s、Namespace %s 中确认 %s/%s 的镜像版本 %s 对应 revision %d，并生成回滚确认预览。当前尚未执行。",
			target.ClusterName, target.Namespace, target.WorkloadType, target.WorkloadName, imageVersion, resolvedRevision)
	} else {
		result.Answer = fmt.Sprintf("已在集群 %s、Namespace %s 中确认 %s/%s 存在 revision %d，并生成回滚确认预览。当前尚未执行。",
			target.ClusterName, target.Namespace, target.WorkloadType, target.WorkloadName, resolvedRevision)
	}
	result.SuggestedActions = []string{"确认后才会执行工作负载回滚。", "确认凭证 10 分钟内有效且只能使用一次。"}
	return nil
}

func (s *AssistantChatService) discoverAssistantK8sWorkloadTargets(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult, workloadName string) ([]assistantK8sWorkloadTarget, []string, error) {
	workloadName = strings.TrimSpace(workloadName)
	if workloadName == "" {
		return nil, nil, errors.New("workloadName is required")
	}
	clusters := make([]interface{}, 0)
	for page := 1; ; page++ {
		clusterOutput, err := s.invokeAssistantMCPTool(c, result, req.Question, "k8s_list_clusters", map[string]interface{}{
			"clusterId": assistantChatFirstEntityUInt(result.Entities, "clusterId", "k8sClusterId"),
			"keyword":   firstNonEmptyAssistantChatString(result.Entities, "clusterName"),
			"page":      page,
			"pageSize":  assistantK8sDiscoveryPage,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("读取授权 K8s 集群失败: %w", err)
		}
		clusterPage := assistantChatMap(clusterOutput)
		items := assistantChatSlice(clusterPage["items"])
		clusters = append(clusters, items...)
		if len(items) < assistantK8sDiscoveryPage || len(clusters) >= int(assistantChatNumber(clusterPage["matched"])) {
			break
		}
	}
	targets := make([]assistantK8sWorkloadTarget, 0)
	errs := make([]string, 0)
	requestedNamespace := firstNonEmptyAssistantChatString(result.Entities, "namespace")
	requestedType := normalizeMCPK8sKind(firstNonEmptyAssistantChatString(result.Entities, "workloadType", "kind"))

	for _, rawCluster := range clusters {
		cluster := assistantChatMap(rawCluster)
		clusterID := uint(assistantChatNumber(cluster["id"]))
		clusterName := firstNonEmptyAssistantChatString(cluster, "name")
		if clusterID == 0 {
			continue
		}
		searchOutput, searchErr := s.invokeAssistantMCPTool(c, result, req.Question, "k8s_search_resources", map[string]interface{}{
			"clusterId": clusterID, "namespace": requestedNamespace, "kind": "workload",
			"keyword": workloadName, "exactName": true, "page": 1, "pageSize": assistantK8sDiscoveryPage,
		})
		if searchErr != nil {
			errs = append(errs, fmt.Sprintf("集群 %s 工作负载搜索失败：%v", clusterName, searchErr))
			continue
		}
		for _, rawWorkload := range assistantChatSlice(assistantChatMap(searchOutput)["workloads"]) {
			workload := assistantChatMap(rawWorkload)
			workloadType := normalizeMCPK8sKind(firstNonEmptyAssistantChatString(workload, "type"))
			namespace := firstNonEmptyAssistantChatString(workload, "namespace")
			if workloadType != "deployment" {
				continue
			}
			if requestedType != "" && workloadType != requestedType {
				continue
			}
			targets = append(targets, assistantK8sWorkloadTarget{
				ClusterID: clusterID, ClusterName: clusterName, Namespace: namespace,
				WorkloadType: workloadType, WorkloadName: workloadName,
			})
		}
	}
	return targets, errs, nil
}

func buildAssistantK8sWorkloadChoiceAnswer(workloadName string, targets []assistantK8sWorkloadTarget) string {
	return fmt.Sprintf("已找到 %d 个名为 %s 的工作负载。为避免误操作，请先选择具体目标；当前尚未生成重启确认凭证。", len(targets), workloadName)
}

func assistantK8sWorkloadTargetSuggestions(targets []assistantK8sWorkloadTarget) []string {
	suggestions := make([]string, 0, len(targets))
	for _, target := range targets {
		suggestions = append(suggestions, fmt.Sprintf("集群 %s（clusterId %d）/ %s / %s/%s", target.ClusterName, target.ClusterID, target.Namespace, target.WorkloadType, target.WorkloadName))
	}
	return suggestions
}

func assistantChatHasSpecificK8sResource(question string, entities map[string]interface{}) bool {
	target := firstNonEmptyAssistantChatString(entities,
		"pod", "podName", "node", "workloadName", "service", "ingress", "resourceName",
	)
	if target == "" {
		return false
	}
	question = strings.ToLower(strings.TrimSpace(question))
	if strings.Contains(question, strings.ToLower(target)) {
		return true
	}
	pageType := strings.ToLower(firstNonEmptyAssistantChatString(entities, "pageType"))
	if containsAnyAssistantChatKeyword(pageType, []string{"pod", "node", "workload", "deployment", "statefulset", "daemonset", "service", "ingress"}) {
		return true
	}
	return containsAnyAssistantChatKeyword(question, []string{
		"这个 pod", "这个pod", "该 pod", "该pod", "这个节点", "该节点", "这个工作负载", "该工作负载",
		"this pod", "this node", "this workload",
	})
}

func selectAssistantK8sClusterTargets(selectedID uint, items []interface{}) []assistantK8sClusterTarget {
	targets := make([]assistantK8sClusterTarget, 0, len(items))
	for _, raw := range items {
		item := assistantChatMap(raw)
		id := uint(assistantChatNumber(item["id"]))
		if id == 0 || (selectedID != 0 && selectedID != id) {
			continue
		}
		name := firstNonEmptyAssistantChatString(item, "name")
		if name == "" {
			name = fmt.Sprintf("集群 #%d", id)
		}
		targets = append(targets, assistantK8sClusterTarget{ID: id, Name: name})
		if len(targets) >= maxAssistantK8sClusters {
			break
		}
	}
	return targets
}

func (s *AssistantChatService) scanAssistantK8sCluster(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult, target assistantK8sClusterTarget) assistantK8sClusterScan {
	scan := assistantK8sClusterScan{ClusterID: target.ID, ClusterName: target.Name}
	invoke := func(toolName, scope string, args map[string]interface{}) (interface{}, error) {
		output, err := s.invokeAssistantMCPTool(c, result, req.Question, toolName, args)
		if err != nil {
			label := toolName
			if scope != "" {
				label += "(" + scope + ")"
			}
			scan.FailedChecks = appendUniqueAssistantString(scan.FailedChecks, label+"："+err.Error())
			return nil, err
		}
		scan.LiveChecks++
		return output, nil
	}

	_, _ = invoke("k8s_check_cluster", "", map[string]interface{}{"clusterId": target.ID})
	if output, err := invoke("k8s_list_nodes", "", map[string]interface{}{"clusterId": target.ID}); err == nil {
		absorbAssistantK8sNodes(&scan, output)
	}

	var namespaces []string
	if output, err := invoke("k8s_list_namespaces", "", map[string]interface{}{"clusterId": target.ID}); err == nil {
		namespaces = assistantK8sNamespaceNames(output)
		scan.AuthorizedNS = len(namespaces)
	}
	if output, err := invoke("k8s_query_events", "", map[string]interface{}{"clusterId": target.ID, "limit": 100}); err == nil {
		absorbAssistantK8sEvents(&scan, output)
	}

	requestedNamespace := firstNonEmptyAssistantChatString(result.Entities, "namespace")
	if requestedNamespace != "" {
		if containsAssistantString(namespaces, requestedNamespace) {
			namespaces = []string{requestedNamespace}
		} else {
			scan.FailedChecks = appendUniqueAssistantString(scan.FailedChecks, "Namespace "+requestedNamespace+" 不在当前授权范围或不存在")
			namespaces = nil
		}
	}
	if len(namespaces) > maxAssistantK8sNamespaces {
		scan.SkippedNS = len(namespaces) - maxAssistantK8sNamespaces
		namespaces = namespaces[:maxAssistantK8sNamespaces]
	}

	resourceToolDisabled := false
	metricsToolDisabled := false
	for _, namespace := range namespaces {
		namespaceScanned := false
		if !resourceToolDisabled {
			for page := 1; page <= maxAssistantK8sPodPages; page++ {
				output, err := invoke("k8s_search_resources", namespace, map[string]interface{}{
					"clusterId": target.ID, "namespace": namespace, "kind": "pod",
					"keyword": "", "page": page, "pageSize": 100,
				})
				if err != nil {
					if errors.Is(err, ErrMCPToolDisabled) {
						resourceToolDisabled = true
					}
					break
				}
				namespaceScanned = true
				total, pages := absorbAssistantK8sPods(&scan, output, page == 1)
				if page >= pages || page*100 >= total {
					break
				}
				if page == maxAssistantK8sPodPages {
					scan.PodResultsTruncated = true
				}
			}
		}
		if !metricsToolDisabled {
			output, err := invoke("k8s_query_metrics", namespace, map[string]interface{}{"clusterId": target.ID, "namespace": namespace})
			if err != nil {
				if errors.Is(err, ErrMCPToolDisabled) {
					metricsToolDisabled = true
				}
			} else {
				namespaceScanned = true
				if !absorbAssistantK8sMetrics(&scan, output) {
					scan.MetricsMissingNS = appendUniqueAssistantString(scan.MetricsMissingNS, namespace)
				}
			}
		}
		if namespaceScanned {
			scan.ScannedNS++
		}
	}
	podStatusToolDisabled := false
	for i := range scan.AbnormalPods {
		if i >= 10 || podStatusToolDisabled {
			break
		}
		pod := &scan.AbnormalPods[i]
		output, err := invoke("k8s_get_pod_status", pod.Namespace+"/"+pod.Pod, map[string]interface{}{
			"clusterId": target.ID, "namespace": pod.Namespace, "pod": pod.Pod,
		})
		if err != nil {
			if errors.Is(err, ErrMCPToolDisabled) {
				podStatusToolDisabled = true
			}
			continue
		}
		absorbAssistantK8sPodDetail(pod, output)
	}
	finalizeAssistantK8sClusterScan(&scan)
	return scan
}

func assistantK8sNamespaceNames(output interface{}) []string {
	data := assistantChatMap(output)
	items := assistantChatSlice(data["namespaces"])
	names := make([]string, 0, len(items))
	for _, raw := range items {
		if name := firstNonEmptyAssistantChatString(assistantChatMap(raw), "name"); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func absorbAssistantK8sNodes(scan *assistantK8sClusterScan, output interface{}) {
	items := assistantChatSlice(output)
	scan.NodeCount = len(items)
	for _, raw := range items {
		node := assistantChatMap(raw)
		name := firstNonEmptyAssistantChatString(node, "name")
		if !strings.EqualFold(firstNonEmptyAssistantChatString(node, "status"), "Ready") {
			scan.NotReadyNodes = appendUniqueAssistantString(scan.NotReadyNodes, name)
		}
		pressure := false
		for _, rawCondition := range assistantChatSlice(node["conditions"]) {
			condition := assistantChatMap(rawCondition)
			conditionType := strings.ToLower(firstNonEmptyAssistantChatString(condition, "type"))
			if strings.EqualFold(firstNonEmptyAssistantChatString(condition, "status"), "True") &&
				(conditionType == "memorypressure" || conditionType == "diskpressure" || conditionType == "pidpressure" || conditionType == "networkunavailable") {
				pressure = true
			}
		}
		resources := assistantChatMap(node["resources"])
		cpuRate := assistantChatNumber(assistantChatMap(resources["cpu"])["usageRate"])
		memoryRate := assistantChatNumber(assistantChatMap(resources["memory"])["usageRate"])
		if pressure || cpuRate >= 80 || memoryRate >= 80 {
			detail := name
			if cpuRate >= 80 || memoryRate >= 80 {
				detail = fmt.Sprintf("%s(CPU %.1f%%, 内存 %.1f%%)", name, cpuRate, memoryRate)
			}
			scan.PressureNodes = appendUniqueAssistantString(scan.PressureNodes, detail)
		}
	}
}

func absorbAssistantK8sEvents(scan *assistantK8sClusterScan, output interface{}) {
	for _, raw := range assistantChatSlice(assistantChatMap(output)["events"]) {
		event := assistantChatMap(raw)
		if !strings.EqualFold(firstNonEmptyAssistantChatString(event, "type"), "Warning") {
			continue
		}
		reason := firstNonEmptyAssistantChatString(event, "reason")
		message := truncateAssistantChatText(firstNonEmptyAssistantChatString(event, "message"), 160)
		text := strings.TrimSpace(reason + "：" + message)
		if text != "：" {
			scan.WarningEvents = appendUniqueAssistantString(scan.WarningEvents, text)
		}
	}
}

func absorbAssistantK8sPods(scan *assistantK8sClusterScan, output interface{}, countTotal bool) (int, int) {
	data := assistantChatMap(output)
	total := int(assistantChatNumber(data["total"]))
	pages := int(assistantChatNumber(data["totalPages"]))
	if pages <= 0 {
		pages = 1
	}
	if countTotal {
		scan.PodCount += total
	}
	for _, raw := range assistantChatSlice(data["workloads"]) {
		pod := assistantChatMap(raw)
		status := firstNonEmptyAssistantChatString(pod, "status")
		restartCount := int(assistantChatNumber(pod["restartCount"]))
		if (strings.EqualFold(status, "Running") || strings.EqualFold(status, "Completed") || strings.EqualFold(status, "Succeeded")) && restartCount < 5 {
			continue
		}
		scan.AbnormalPods = append(scan.AbnormalPods, assistantK8sAbnormalPod{
			Cluster: scan.ClusterName, Namespace: firstNonEmptyAssistantChatString(pod, "namespace"),
			Pod: firstNonEmptyAssistantChatString(pod, "name"), Status: status, Node: firstNonEmptyAssistantChatString(pod, "nodeName"), RestartCount: restartCount,
		})
	}
	return total, pages
}

func absorbAssistantK8sMetrics(scan *assistantK8sClusterScan, output interface{}) bool {
	data := assistantChatMap(output)
	podCount := int(assistantChatNumber(data["podCount"]))
	items := assistantChatSlice(data["podMetrics"])
	if podCount > 0 && len(items) == 0 {
		return false
	}
	if len(items) > 0 || podCount == 0 {
		scan.MetricsAvailable = true
	}
	for _, raw := range items {
		metric := assistantChatMap(raw)
		usage := assistantChatMap(metric["usage"])
		rate := assistantChatMap(metric["usageRate"])
		hotspot := assistantK8sResourceHotspot{
			Cluster: scan.ClusterName, Namespace: firstNonEmptyAssistantChatString(metric, "namespace"),
			Pod: firstNonEmptyAssistantChatString(metric, "podName"),
			CPU: firstNonEmptyAssistantChatString(usage, "cpu"), Memory: firstNonEmptyAssistantChatString(usage, "memory"),
			CPURate: assistantChatNumber(rate["cpuRate"]), MemoryRate: assistantChatNumber(rate["memoryRate"]),
		}
		hotspot.cpuMilli = assistantK8sQuantityValue(hotspot.CPU, true)
		hotspot.memoryByte = assistantK8sQuantityValue(hotspot.Memory, false)
		scan.Hotspots = append(scan.Hotspots, hotspot)
	}
	return true
}

func absorbAssistantK8sPodDetail(pod *assistantK8sAbnormalPod, output interface{}) {
	if pod == nil {
		return
	}
	data := assistantChatMap(output)
	pod.RestartCount = int(assistantChatNumber(data["restartCount"]))
	for _, raw := range assistantChatSlice(data["conditions"]) {
		condition := assistantChatMap(raw)
		if strings.EqualFold(firstNonEmptyAssistantChatString(condition, "status"), "True") {
			continue
		}
		detail := firstNonEmptyAssistantChatString(condition, "reason", "type")
		if message := truncateAssistantChatText(firstNonEmptyAssistantChatString(condition, "message"), 100); message != "" {
			detail += "：" + message
		}
		pod.Conditions = appendUniqueAssistantString(pod.Conditions, detail)
	}
}

func assistantK8sQuantityValue(value string, cpu bool) int64 {
	quantity, err := resource.ParseQuantity(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	if cpu {
		return quantity.MilliValue()
	}
	return quantity.Value()
}

func finalizeAssistantK8sClusterScan(scan *assistantK8sClusterScan) {
	if len(scan.AbnormalPods) > 50 {
		scan.AbnormalPods = scan.AbnormalPods[:50]
		scan.PodResultsTruncated = true
	}
	if len(scan.WarningEvents) > 30 {
		scan.WarningEvents = scan.WarningEvents[:30]
	}
	scan.Hotspots = topAssistantK8sHotspots(scan.Hotspots, 10)
}

func topAssistantK8sHotspots(items []assistantK8sResourceHotspot, limit int) []assistantK8sResourceHotspot {
	if len(items) <= limit {
		return items
	}
	selected := make(map[string]assistantK8sResourceHotspot, limit*2)
	byCPU := append([]assistantK8sResourceHotspot(nil), items...)
	sort.Slice(byCPU, func(i, j int) bool { return byCPU[i].cpuMilli > byCPU[j].cpuMilli })
	byMemory := append([]assistantK8sResourceHotspot(nil), items...)
	sort.Slice(byMemory, func(i, j int) bool { return byMemory[i].memoryByte > byMemory[j].memoryByte })
	for i := 0; i < limit/2 && i < len(byCPU); i++ {
		selected[byCPU[i].Namespace+"/"+byCPU[i].Pod] = byCPU[i]
	}
	for i := 0; i < limit/2 && i < len(byMemory); i++ {
		selected[byMemory[i].Namespace+"/"+byMemory[i].Pod] = byMemory[i]
	}
	result := make([]assistantK8sResourceHotspot, 0, len(selected))
	for _, item := range selected {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		left := math.Max(result[i].CPURate, result[i].MemoryRate)
		right := math.Max(result[j].CPURate, result[j].MemoryRate)
		if left == right {
			return result[i].cpuMilli+result[i].memoryByte > result[j].cpuMilli+result[j].memoryByte
		}
		return left > right
	})
	return result
}

func containsAssistantString(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func appendUniqueAssistantString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || containsAssistantString(items, value) {
		return items
	}
	return append(items, value)
}

func compactAssistantK8sScanEvidence(items []aiopsmodel.AgentDiagnosisEvidence) []aiopsmodel.AgentDiagnosisEvidence {
	compacted := make([]aiopsmodel.AgentDiagnosisEvidence, 0, 2)
	for _, item := range items {
		if item.Type != "k8s" {
			compacted = append(compacted, item)
			continue
		}
		if item.Reference == "k8s_list_clusters" || item.Reference == "k8s_health_scan_summary" {
			compacted = append(compacted, item)
		}
	}
	return compacted
}

func aggregateAssistantK8sUsedTools(items []aiopsmodel.AssistantChatUsedTool) []aiopsmodel.AssistantChatUsedTool {
	aggregated := make([]aiopsmodel.AssistantChatUsedTool, 0, len(items))
	indexes := make(map[string]int)
	for _, item := range items {
		if !strings.HasPrefix(item.Name, "k8s_") {
			aggregated = append(aggregated, item)
			continue
		}
		if index, ok := indexes[item.Name]; ok {
			current := &aggregated[index]
			current.CallCount++
			current.DurationMS += item.DurationMS
			current.Success = current.Success && item.Success
			current.Arguments = nil
			if item.Error != "" && !strings.Contains(current.Error, item.Error) {
				current.Error = strings.Trim(strings.TrimSpace(current.Error)+"；"+item.Error, "；")
			}
			continue
		}
		item.CallCount = 1
		indexes[item.Name] = len(aggregated)
		aggregated = append(aggregated, item)
	}
	return aggregated
}

func aggregateAssistantK8sTimeline(items []aiopsmodel.AssistantChatTimelineEvent) []aiopsmodel.AssistantChatTimelineEvent {
	aggregated := make([]aiopsmodel.AssistantChatTimelineEvent, 0, len(items))
	indexes := make(map[string]int)
	counts := make(map[string]int)
	for _, item := range items {
		if !strings.HasPrefix(item.ToolName, "k8s_") {
			aggregated = append(aggregated, item)
			continue
		}
		counts[item.ToolName]++
		if index, ok := indexes[item.ToolName]; ok {
			current := &aggregated[index]
			current.DurationMS += item.DurationMS
			if item.FinishedAt != "" {
				current.FinishedAt = item.FinishedAt
			}
			if item.Status == aiopsmodel.AssistantChatPlanFailed {
				current.Status = item.Status
			}
			if item.Error != "" && !strings.Contains(current.Error, item.Error) {
				current.Error = strings.Trim(strings.TrimSpace(current.Error)+"；"+item.Error, "；")
			}
			continue
		}
		indexes[item.ToolName] = len(aggregated)
		aggregated = append(aggregated, item)
	}
	for toolName, count := range counts {
		if count <= 1 {
			continue
		}
		index := indexes[toolName]
		aggregated[index].Message = fmt.Sprintf("%s（调用 %d 次）", aggregated[index].Message, count)
	}
	for index := range aggregated {
		aggregated[index].Step = index + 1
	}
	return aggregated
}

func (s *AssistantChatService) invokeAssistantMCPTool(c *gin.Context, result *aiopsmodel.AssistantChatResult, userPrompt, toolName string, args map[string]interface{}) (interface{}, error) {
	domain := "k8s"
	if strings.HasPrefix(toolName, "ecs_") {
		domain = "ecs"
	}
	tool, authErr := NewMCPCatalogService().AuthorizeTool(domain, toolName)
	if (toolName == "k8s_diagnose_workload" || toolName == "k8s_diagnose_pod") && tool.TimeoutSeconds < 330 {
		tool.TimeoutSeconds = 330
	}
	ensureAssistantChatToolPlanStep(result, toolName, tool.DisplayName, args)
	started := time.Now()
	markAssistantChatPlan(result, toolName, aiopsmodel.AssistantChatPlanRunning, args, "")
	var output interface{}
	var err error
	if authErr != nil {
		err = authErr
	} else {
		output, err = NewMCPToolExecutor().ExecuteInRun(c, tool, args, result.RunID, func(ctx *gin.Context, callArgs map[string]interface{}, _ uint64) (interface{}, error) {
			if domain == "ecs" {
				return NewMCPHostOperationService().Invoke(ctx, toolName, callArgs)
			}
			if strings.HasPrefix(toolName, "k8s_request_") {
				return NewMCPK8sActionService().Preview(ctx, toolName, callArgs)
			}
			if toolName == "k8s_diagnose_workload" || toolName == "k8s_diagnose_pod" {
				return s.invokeAssistantK8sAnalyzeTask(ctx, result, userPrompt, callArgs)
			}
			return InvokeMCPK8sTool(ctx, toolName, callArgs)
		})
	}
	err = assistantChatNormalizeToolError(toolName, tool, err)
	finished := time.Now()
	status := aiopsmodel.AssistantChatPlanSucceeded
	errText := ""
	if err != nil {
		status = aiopsmodel.AssistantChatPlanFailed
		if isAssistantChatToolPolicyError(err) {
			status = aiopsmodel.AssistantChatPlanSkipped
		}
		errText = err.Error()
	}
	markAssistantChatPlan(result, toolName, status, args, errText)
	appendAssistantChatTimeline(result, 0, toolName, status, assistantChatToolPurpose(toolName), started, finished, errText)
	s.emitAssistantChatStepEvent(result, toolName, assistantChatAgentStepStatus(status), assistantChatToolPurpose(toolName), errText, started, finished)
	result.UsedTools = append(result.UsedTools, aiopsmodel.AssistantChatUsedTool{
		Name: toolName, Arguments: args, DurationMS: finished.Sub(started).Milliseconds(), Success: err == nil, Error: errText,
	})
	if err == nil {
		result.Evidence = append(result.Evidence, aiopsmodel.AgentDiagnosisEvidence{
			Type: domain, Title: tool.DisplayName, Reference: toolName,
			Content: "已执行 " + strings.ToUpper(domain) + " 实时只读工具 " + toolName, Raw: compactAssistantK8sEvidence(output),
		})
	}
	return output, err
}

func (s *AssistantChatService) invokeAssistantK8sAnalyzeTask(c *gin.Context, result *aiopsmodel.AssistantChatResult, userPrompt string, args map[string]interface{}) (interface{}, error) {
	var req aiopsmodel.K8sPodAnalyzeRequest
	if err := decodeServiceMCPArguments(args, &req); err != nil {
		return nil, err
	}
	req.Prompt = strings.TrimSpace(userPrompt)
	task, err := NewLogAnalysisService().StartK8sPodAnalyzeTask(&req)
	if err != nil {
		return nil, err
	}
	result.AnalysisTask = &aiopsmodel.AssistantChatAsyncTask{
		Type: "k8s_pod_analysis", TaskID: task.TaskID, Status: task.Status,
		Progress: 0, Message: "分析任务已创建", StreamURL: task.StreamURL, ResultURL: task.ResultURL,
	}
	var ctx context.Context = context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	logService := NewLogAnalysisService()
	if err := logService.StreamK8sPodAnalyzeTask(ctx, task.TaskID, func(event aiopsmodel.K8sPodAnalyzeProgressEvent) error {
		s.emitAssistantK8sAnalysisProgress(result, event)
		return nil
	}); err != nil {
		return nil, err
	}
	status, err := logService.GetK8sPodAnalyzeTask(task.TaskID)
	if err != nil {
		return nil, err
	}
	if result.AnalysisTask != nil {
		result.AnalysisTask.Status = status.Status
		result.AnalysisTask.CurrentStep = status.CurrentStep
		result.AnalysisTask.Progress = status.Progress
		result.AnalysisTask.Message = status.Message
	}
	if status.Status == "failed" {
		return nil, errors.New(firstNonEmptyString(status.Error, "K8s Pod analysis failed"))
	}
	if status.Result == nil {
		return nil, errors.New("K8s Pod analysis ended without a result")
	}
	return status.Result, nil
}

func (s *AssistantChatService) emitAssistantK8sAnalysisProgress(result *aiopsmodel.AssistantChatResult, event aiopsmodel.K8sPodAnalyzeProgressEvent) {
	if result == nil || result.AnalysisTask == nil {
		return
	}
	result.AnalysisTask.Status = event.Status
	result.AnalysisTask.CurrentStep = event.Step
	result.AnalysisTask.Progress = event.Progress
	result.AnalysisTask.Message = event.Message
	refreshAssistantChatProgress(result)
	refreshAssistantChatThinkingProcess(result, time.Time{}, "")
	s.emitAssistantChatGenericEvent("analysis_progress", result.RunID, event.Status, event.Message, map[string]interface{}{
		"analysisTask":    result.AnalysisTask,
		"event":           event,
		"progress":        result.Progress,
		"thinkingProcess": result.ThinkingProcess,
	})
	s.emitAssistantChatGenericEvent("progress", result.RunID, event.Status, event.Message, map[string]interface{}{
		"progress":        result.Progress,
		"analysisTask":    result.AnalysisTask,
		"thinkingProcess": result.ThinkingProcess,
	})
}

func compactAssistantK8sEvidence(output interface{}) interface{} {
	limited := LimitMCPResultRows(output, 100)
	data, err := json.Marshal(limited)
	if err != nil || len(data) <= 20000 {
		return limited
	}
	return map[string]interface{}{
		"truncated":   true,
		"jsonPreview": string(data[:20000]),
	}
}

type assistantChatToolPolicyError struct {
	message string
	cause   error
}

func (e *assistantChatToolPolicyError) Error() string { return e.message }
func (e *assistantChatToolPolicyError) Unwrap() error { return e.cause }

func assistantChatNormalizeToolError(toolName string, tool aiopsmodel.MCPToolView, err error) error {
	if err == nil {
		return nil
	}
	displayName := strings.TrimSpace(tool.DisplayName)
	if displayName == "" {
		displayName = assistantChatToolPurpose(toolName)
	}
	switch {
	case errors.Is(err, ErrMCPToolDisabled):
		if toolName == "ecs_request_cleanup_logs" {
			return &assistantChatToolPolicyError{message: "清空日志功能关闭，无法完成清空日志操作，请开启后再次尝试", cause: err}
		}
		return &assistantChatToolPolicyError{message: fmt.Sprintf("%s功能关闭，无法完成操作，请开启后再次尝试", displayName), cause: err}
	case errors.Is(err, ErrMCPServerDisabled):
		return &assistantChatToolPolicyError{message: fmt.Sprintf("%s所属 MCP 服务已关闭，无法完成操作，请开启后再次尝试", displayName), cause: err}
	case errors.Is(err, ErrMCPToolUnsupported):
		return &assistantChatToolPolicyError{message: fmt.Sprintf("%s功能暂未支持，无法完成操作", displayName), cause: err}
	default:
		return err
	}
}

func isAssistantChatToolPolicyError(err error) bool {
	return errors.Is(err, ErrMCPToolDisabled) ||
		errors.Is(err, ErrMCPServerDisabled) ||
		errors.Is(err, ErrMCPToolUnsupported)
}

func assistantChatToolPolicyMessage(err error) string {
	var policyErr *assistantChatToolPolicyError
	if errors.As(err, &policyErr) {
		return policyErr.Error()
	}
	return err.Error()
}

func ensureAssistantChatToolPlanStep(result *aiopsmodel.AssistantChatResult, toolName, purpose string, args map[string]interface{}) {
	if result == nil {
		return
	}
	for _, item := range result.ToolPlan {
		if item.ToolName == toolName {
			return
		}
	}
	step := aiopsmodel.AssistantChatToolPlanStep{
		ToolName: toolName, Purpose: purpose,
		Arguments: args, Status: aiopsmodel.AssistantChatPlanPending,
	}
	insertAt := len(result.ToolPlan)
	summaryTool := assistantChatSummaryStepToolName(result.Intent)
	for i, item := range result.ToolPlan {
		if item.ToolName == summaryTool {
			insertAt = i
			break
		}
	}
	result.ToolPlan = append(result.ToolPlan, aiopsmodel.AssistantChatToolPlanStep{})
	copy(result.ToolPlan[insertAt+1:], result.ToolPlan[insertAt:])
	result.ToolPlan[insertAt] = step
	for i := range result.ToolPlan {
		result.ToolPlan[i].Step = i + 1
	}
	refreshAssistantChatProgress(result)
}

func selectAssistantK8sClusterID(question string, items []interface{}) uint {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return 0
	}
	var matched uint
	for _, raw := range items {
		item := assistantChatMap(raw)
		name := strings.ToLower(strings.TrimSpace(firstNonEmptyAssistantChatString(item, "name")))
		id := uint(assistantChatNumber(item["id"]))
		if name == "" || id == 0 || !strings.Contains(question, name) {
			continue
		}
		if matched != 0 && matched != id {
			return 0
		}
		matched = id
	}
	return matched
}

func (s *AssistantChatService) handleAssistantKnowledgeSearch(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	items, err := s.searchAssistantKnowledge(c, req, result, 5)
	if err != nil {
		return err
	}

	result.Answer = buildAssistantChatKnowledgeAnswer(items)
	result.SuggestedActions = []string{
		"优先打开相关度最高的知识文档，核对适用环境和更新时间。",
		"如果没有命中合适文档，可以把本次处理过程沉淀为新的 Runbook 或故障复盘。",
	}
	result.DiagnosisCard = buildAssistantKnowledgeCard(items, result.Evidence, result.MissingInfo, result.SuggestedActions)
	return nil
}

func (s *AssistantChatService) handleAssistantSkillCatalog(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	started := time.Now()
	startAssistantChatPlanStep(result, "skill_catalog")
	snapshot, err := NewSkillCatalogService().Snapshot()
	finished := time.Now()
	if err != nil {
		finishAssistantChatPlanStep(result, "skill_catalog", err.Error())
		appendAssistantChatTimeline(result, 0, "skill_catalog", aiopsmodel.AssistantChatPlanFailed, "读取 AutoOps Skill Catalog 失败", started, finished, err.Error())
		return err
	}
	finishAssistantChatPlanStep(result, "skill_catalog", "")
	appendAssistantChatTimeline(result, 0, "skill_catalog", aiopsmodel.AssistantChatPlanSucceeded, "已读取 AutoOps Skill Catalog", started, finished, "")

	result.UsedTools = append(result.UsedTools, aiopsmodel.AssistantChatUsedTool{
		Name:       "skill_catalog",
		Arguments:  map[string]interface{}{"source": "skills/*_skill/*/SKILL.md"},
		DurationMS: finished.Sub(started).Milliseconds(),
		Success:    true,
	})
	result.Evidence = append(result.Evidence, aiopsmodel.AgentDiagnosisEvidence{
		Type:      "skill",
		Title:     "AutoOps Skill Catalog",
		Reference: "skills",
		Content:   fmt.Sprintf("已加载 %d 个声明式 Skill，加载错误 %d 个", snapshot.Loaded, len(snapshot.Errors)),
		Raw:       snapshot,
	})
	result.Answer = buildAssistantChatSkillCatalogAnswer(snapshot)
	result.SuggestedActions = []string{
		"前端可调用 GET /api/v1/aiops/skills 查看完整 Manifest 列表。",
		"前端可调用 GET /api/v1/aiops/skills/detail/{name} 查看单个 Skill 的 SKILL.md 正文。",
		"新增或修改 SKILL.md 后，可调用 POST /api/v1/aiops/skills/reload 手动重载。",
	}
	result.DiagnosisCard = &aiopsmodel.AssistantChatDiagnosisCard{
		Summary:           fmt.Sprintf("AutoOps 当前加载了 %d 个声明式 Skill。", snapshot.Loaded),
		Severity:          aiopsmodel.AssistantChatSeverityInfo,
		ImpactedResources: []string{"skills"},
		Evidence:          result.Evidence,
		NextChecks:        result.SuggestedActions,
		Remediation:       []string{},
		HandoffNote:       "本次只读取 Skill Catalog，没有执行生产工具或变更操作。",
		MissingData:       []string{},
	}
	if len(snapshot.Errors) > 0 {
		result.DiagnosisCard.Severity = aiopsmodel.AssistantChatSeverityWarning
		result.MissingInfo = append(result.MissingInfo, "部分 SKILL.md 加载失败，请查看 /api/v1/aiops/skills/categories 返回的 errors。")
	}
	return nil
}

func (s *AssistantChatService) handleAssistantMCPCatalog(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	started := time.Now()
	startAssistantChatPlanStep(result, "mcp_catalog")
	servers, err := NewMCPCatalogService().ListServers("")
	finished := time.Now()
	if err != nil {
		finishAssistantChatPlanStep(result, "mcp_catalog", err.Error())
		appendAssistantChatTimeline(result, 0, "mcp_catalog", aiopsmodel.AssistantChatPlanFailed, "读取 MCP 能力目录失败", started, finished, err.Error())
		return err
	}
	finishAssistantChatPlanStep(result, "mcp_catalog", "")
	appendAssistantChatTimeline(result, 0, "mcp_catalog", aiopsmodel.AssistantChatPlanSucceeded, "已读取 MCP 能力目录", started, finished, "")
	result.UsedTools = append(result.UsedTools, aiopsmodel.AssistantChatUsedTool{
		Name: "mcp_catalog", DurationMS: finished.Sub(started).Milliseconds(), Success: true,
	})
	result.Evidence = append(result.Evidence, aiopsmodel.AgentDiagnosisEvidence{
		Type: "mcp", Title: "MCP 能力目录", Reference: "mcp_catalog", Content: "已读取 MCP 服务与 Tool 的实时实现和启用状态", Raw: servers,
	})
	result.Answer = buildAssistantChatMCPCatalogAnswer(servers)
	result.SuggestedActions = []string{
		"MCP 客户端新增或启用 Tool 后需要重新执行 initialize 和 tools/list。",
		"管理页面可按 MCP 服务查看待实现或已关闭的 Tool。",
	}
	return nil
}

func (s *AssistantChatService) searchAssistantKnowledge(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult, limit int) ([]aiopsmodel.KnowledgeSearchItem, error) {
	if limit <= 0 {
		limit = 5
	}
	args := map[string]interface{}{"query": req.Question, "limit": limit}
	tool, err := s.invokeAssistantTool(c, result, req.Question, "knowledge_search", args)
	if err != nil {
		return nil, err
	}
	output := assistantChatMapOutput(tool)
	items := assistantChatKnowledgeItems(output["items"])
	for _, item := range items {
		result.Evidence = append(result.Evidence, aiopsmodel.AgentDiagnosisEvidence{
			Type:      "knowledge",
			Title:     item.Title,
			Reference: fmt.Sprintf("knowledge:%d", item.ID),
			Content:   firstNonEmptyAssistantChatText(item.Summary, item.Snippet),
			Raw:       item,
		})
	}
	return items, nil
}

func (s *AssistantChatService) handleAssistantResourceGovernance(c *gin.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) error {
	started := time.Now()
	startAssistantChatPlanStep(result, "resource_scope")
	finishAssistantChatPlanStep(result, "resource_scope", "")
	markAssistantChatPlan(result, "assistant_query", aiopsmodel.AssistantChatPlanRunning, map[string]interface{}{"question": req.Question}, "")

	queryReq := &aiopsmodel.AssistantQueryRequest{
		Question:   req.Question,
		ProviderID: req.ProviderID,
	}
	applyAssistantQueryContext(queryReq, req.Context)

	queryResult, err := s.querySvc.Query(c, queryReq)
	finished := time.Now()
	status := aiopsmodel.AgentStepStatusSucceeded
	planStatus := aiopsmodel.AssistantChatPlanSucceeded
	errText := ""
	if err != nil {
		status = aiopsmodel.AgentStepStatusFailed
		planStatus = aiopsmodel.AssistantChatPlanFailed
		errText = err.Error()
	}

	step := aiopsmodel.AgentStep{
		StepName:   "调用专用资源助手",
		ToolName:   "assistant_query",
		Status:     status,
		InputJSON:  compactToolJSON(queryReq),
		StartedAt:  started,
		FinishedAt: &finished,
		DurationMS: finished.Sub(started).Milliseconds(),
		Error:      errText,
	}
	if queryResult != nil {
		step.OutputJSON = compactToolJSON(queryResult)
	}
	_ = s.agentDao.AppendRunStep(result.RunID, step)
	markAssistantChatPlan(result, "assistant_query", planStatus, map[string]interface{}{"question": req.Question}, errText)
	appendAssistantChatTimeline(result, 0, "assistant_query", planStatus, "调用专用资源助手", started, finished, errText)
	if err == nil {
		startAssistantChatPlanStep(result, "resource_metrics")
		finishAssistantChatPlanStep(result, "resource_metrics", "")
	} else {
		finishAssistantChatPlanStep(result, "resource_metrics", errText)
	}
	s.emitAssistantChatStepEvent(result, "assistant_query", status, "调用专用资源助手", errText, started, finished)

	result.UsedTools = append(result.UsedTools, aiopsmodel.AssistantChatUsedTool{
		Name:       "assistant_query",
		Arguments:  map[string]interface{}{"question": req.Question},
		DurationMS: step.DurationMS,
		Success:    err == nil,
		Error:      errText,
	})
	if err != nil {
		return err
	}

	result.Answer = queryResult.Answer
	result.MissingInfo = append(result.MissingInfo, queryResult.MissingInfo...)
	result.UsedLLM = queryResult.UsedLLM
	result.ProviderID = queryResult.ProviderID
	result.ProviderName = queryResult.ProviderName
	result.ProviderType = queryResult.ProviderType
	result.ModelCode = queryResult.ModelCode
	for _, evidence := range queryResult.Evidence {
		result.Evidence = append(result.Evidence, aiopsmodel.AgentDiagnosisEvidence{
			Type:      evidence.Type,
			Title:     evidence.Title,
			Reference: evidence.Reference,
			Content:   evidence.Content,
			Raw:       evidence.Raw,
		})
	}
	result.SuggestedActions = []string{
		"在页面中查看 candidates 列表，按 score、风险和缺失信息做二次确认。",
		"涉及资源回收或变更时，需要走审批和变更工单，不应由助手直接执行。",
	}
	result.DiagnosisCard = buildAssistantResourceGovernanceCard(queryResult, result.Evidence, result.MissingInfo, result.SuggestedActions)
	return nil
}

func (s *AssistantChatService) emitAssistantChatRunEvent(runID uint64, status, message string) {
	s.emitAssistantChatEvent(aiopsmodel.AgentRunStreamEvent{
		Type:    "run",
		RunID:   runID,
		Status:  status,
		Message: message,
		Time:    time.Now().Format("2006-01-02 15:04:05"),
	})
}

func (s *AssistantChatService) emitAssistantChatGenericEvent(eventType string, runID uint64, status, message string, payload interface{}) {
	s.emitAssistantChatEvent(aiopsmodel.AgentRunStreamEvent{
		Type:    eventType,
		RunID:   runID,
		Status:  status,
		Result:  payload,
		Message: message,
		Time:    time.Now().Format("2006-01-02 15:04:05"),
	})
}

func startAssistantChatPlanStep(result *aiopsmodel.AssistantChatResult, toolName string) {
	markAssistantChatPlan(result, toolName, aiopsmodel.AssistantChatPlanRunning, nil, "")
	refreshAssistantChatProgress(result)
}

func finishAssistantChatPlanStep(result *aiopsmodel.AssistantChatResult, toolName string, errText string) {
	status := aiopsmodel.AssistantChatPlanSucceeded
	if strings.TrimSpace(errText) != "" {
		status = aiopsmodel.AssistantChatPlanFailed
	}
	markAssistantChatPlan(result, toolName, status, nil, errText)
	refreshAssistantChatProgress(result)
}

func (s *AssistantChatService) emitAssistantChatStepEvent(result *aiopsmodel.AssistantChatResult, toolName, status, message, errText string, started, finished time.Time) {
	if result == nil {
		return
	}
	step := aiopsmodel.AgentStep{
		StepName:   message,
		ToolName:   toolName,
		Status:     status,
		InputJSON:  compactToolJSON(assistantChatPlanArguments(result, toolName)),
		Error:      errText,
		StartedAt:  started,
		FinishedAt: &finished,
		DurationMS: finished.Sub(started).Milliseconds(),
	}
	refreshAssistantChatProgress(result)
	refreshAssistantChatThinkingProcess(result, time.Time{}, "")
	s.emitAssistantChatEvent(aiopsmodel.AgentRunStreamEvent{
		Type:      "step",
		RunID:     result.RunID,
		Status:    status,
		StepIndex: assistantChatPlanStep(result, toolName),
		Step:      &step,
		Result: map[string]interface{}{
			"progress":        result.Progress,
			"thinkingProcess": result.ThinkingProcess,
		},
		Message: message,
		Error:   errText,
		Time:    time.Now().Format("2006-01-02 15:04:05"),
	})
}

func (s *AssistantChatService) recordAssistantChatLocalStep(
	result *aiopsmodel.AssistantChatResult,
	toolName string,
	message string,
	args map[string]interface{},
	output interface{},
	err error,
	started time.Time,
	finished time.Time,
) {
	if result == nil {
		return
	}
	planStatus := aiopsmodel.AssistantChatPlanSucceeded
	stepStatus := aiopsmodel.AgentStepStatusSucceeded
	errText := ""
	if err != nil {
		planStatus = aiopsmodel.AssistantChatPlanFailed
		stepStatus = aiopsmodel.AgentStepStatusFailed
		errText = err.Error()
	}
	step := aiopsmodel.AgentStep{
		StepName:   message,
		ToolName:   toolName,
		Status:     stepStatus,
		InputJSON:  compactToolJSON(args),
		OutputJSON: compactToolJSON(output),
		Error:      errText,
		StartedAt:  started,
		FinishedAt: &finished,
		DurationMS: finished.Sub(started).Milliseconds(),
	}
	_ = s.agentDao.AppendRunStep(result.RunID, step)
	markAssistantChatPlan(result, toolName, planStatus, args, errText)
	appendAssistantChatTimeline(result, 0, toolName, planStatus, message, started, finished, errText)
	s.emitAssistantChatStepEvent(result, toolName, stepStatus, message, errText, started, finished)
	result.UsedTools = append(result.UsedTools, aiopsmodel.AssistantChatUsedTool{
		Name:       toolName,
		Arguments:  args,
		DurationMS: step.DurationMS,
		Success:    err == nil,
		Error:      errText,
	})
}

func (s *AssistantChatService) emitAssistantChatResultEvent(runID uint64, status string, result *aiopsmodel.AssistantChatResult) {
	s.emitAssistantChatEvent(aiopsmodel.AgentRunStreamEvent{
		Type:    "result",
		RunID:   runID,
		Status:  status,
		Result:  result,
		Message: "assistant result",
		Time:    time.Now().Format("2006-01-02 15:04:05"),
	})
}

func (s *AssistantChatService) emitAssistantChatDoneEvent(runID uint64, status string) {
	s.emitAssistantChatEvent(aiopsmodel.AgentRunStreamEvent{
		Type:    "done",
		RunID:   runID,
		Status:  status,
		Message: "stream completed",
		Time:    time.Now().Format("2006-01-02 15:04:05"),
	})
}

func (s *AssistantChatService) emitAssistantChatEvent(event aiopsmodel.AgentRunStreamEvent) {
	if s == nil || s.streamEmitter == nil {
		return
	}
	s.streamEmitter(event)
}

func (s *AssistantChatService) invokeAssistantTool(
	c *gin.Context,
	result *aiopsmodel.AssistantChatResult,
	userPrompt string,
	toolName string,
	args map[string]interface{},
) (*aiopsmodel.AgentToolInvokeResult, error) {
	started := time.Now()
	markAssistantChatPlan(result, toolName, aiopsmodel.AssistantChatPlanRunning, args, "")
	toolResult, err := s.toolGateway.Invoke(c, &aiopsmodel.AgentToolInvokeRequest{
		RunID:     result.RunID,
		ToolName:  toolName,
		Arguments: args,
		Context: aiopsmodel.AgentToolInvokeContext{
			AgentRuntime: "autoops_internal",
			UserPrompt:   userPrompt,
		},
	})
	err = assistantChatNormalizeToolError(toolName, aiopsmodel.MCPToolView{}, err)
	finished := time.Now()

	planStatus := aiopsmodel.AssistantChatPlanSucceeded
	errText := ""
	if err != nil {
		planStatus = aiopsmodel.AssistantChatPlanFailed
		if isAssistantChatToolPolicyError(err) {
			planStatus = aiopsmodel.AssistantChatPlanSkipped
		}
		errText = err.Error()
	}
	markAssistantChatPlan(result, toolName, planStatus, args, errText)
	appendAssistantChatTimeline(result, 0, toolName, planStatus, assistantChatToolPurpose(toolName), started, finished, errText)
	s.emitAssistantChatStepEvent(result, toolName, assistantChatAgentStepStatus(planStatus), assistantChatToolPurpose(toolName), errText, started, finished)

	usedTool := aiopsmodel.AssistantChatUsedTool{
		Name:       toolName,
		Arguments:  args,
		DurationMS: finished.Sub(started).Milliseconds(),
		Success:    err == nil,
		Error:      errText,
	}
	if toolResult != nil {
		usedTool.DurationMS = toolResult.DurationMS
		if toolResult.Evidence != nil {
			result.Evidence = append(result.Evidence, *toolResult.Evidence)
		}
	}
	result.UsedTools = append(result.UsedTools, usedTool)
	return toolResult, err
}

func (s *AssistantChatService) generateAssistantChatAnswer(ctx context.Context, req *aiopsmodel.AssistantChatRequest, result *aiopsmodel.AssistantChatResult) (string, *aiopsmodel.LLMProvider, bool, error) {
	provider, apiKey, err := s.resolveAssistantChatLLMProvider(req.ProviderID)
	if err != nil {
		return "", nil, false, err
	}
	timeout := time.Duration(provider.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	llmCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	prompt := buildAssistantChatLLMPrompt(result)
	answer, err := callLLMForLogAnalysis(llmCtx, provider, apiKey, prompt)
	if err != nil {
		return "", provider, false, err
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return "", provider, false, nil
	}
	return answer, provider, true, nil
}

func (s *AssistantChatService) resolveAssistantChatLLMProvider(providerID uint) (*aiopsmodel.LLMProvider, string, error) {
	querySvc := &AssistantQueryService{llmDao: s.llmDao}
	return querySvc.resolveLLMProvider(providerID)
}

func (s *AssistantChatService) updateAssistantChatRunResult(runID uint64, result *aiopsmodel.AssistantChatResult, generatedAt time.Time) error {
	data, _ := json.Marshal(result)
	return common.GetDB().Model(&aiopsmodel.AgentRun{}).
		Where("id = ?", runID).
		Updates(map[string]interface{}{
			"provider_id":   result.ProviderID,
			"provider_name": result.ProviderName,
			"provider_type": result.ProviderType,
			"model_code":    result.ModelCode,
			"summary":       truncateAssistantChatText(result.Answer, 1000),
			"result_json":   string(data),
			"used_llm":      result.UsedLLM,
			"generated_at":  generatedAt,
			"updated_at":    time.Now(),
		}).Error
}

func (s *AssistantChatService) loadAssistantChatMemory(sessionID string) (*aiopsmodel.AssistantChatMemoryContext, error) {
	memory := &aiopsmodel.AssistantChatMemoryContext{
		Used:     false,
		Entities: map[string]interface{}{},
		Messages: []aiopsmodel.AssistantChatMemoryMessage{},
	}
	messages, err := s.agentDao.ListRecentMessages(sessionID, defaultAssistantMemoryLimit)
	if err != nil {
		return memory, err
	}
	memory.MessageCount = len(messages)
	for _, message := range messages {
		entities := map[string]interface{}{}
		if strings.TrimSpace(message.EntitiesJSON) != "" {
			_ = json.Unmarshal([]byte(message.EntitiesJSON), &entities)
		}
		for key, value := range entities {
			if !assistantChatValueEmpty(value) {
				memory.Entities[key] = value
			}
		}
		memory.Messages = append(memory.Messages, aiopsmodel.AssistantChatMemoryMessage{
			Role:      message.Role,
			Intent:    message.Intent,
			Content:   truncateAssistantChatText(message.Content, 240),
			Entities:  entities,
			CreatedAt: message.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	if len(memory.Entities) == 0 {
		memory.Entities = nil
	}
	if len(memory.Messages) == 0 {
		memory.Messages = nil
	}
	return memory, nil
}

func detectAssistantChatIntent(req *aiopsmodel.AssistantChatRequest, entities map[string]interface{}) string {
	question := strings.ToLower(strings.TrimSpace(req.Question))
	if assistantChatQuestionLooksLikeCareerSkillQA(question) {
		return aiopsmodel.AssistantChatIntentGeneralQA
	}
	if assistantChatQuestionLooksLikeMCPCatalog(question) {
		return aiopsmodel.AssistantChatIntentMCPCatalog
	}
	hasHost := assistantChatNumber(entities["hostId"]) > 0 ||
		firstNonEmptyAssistantChatString(entities, "ip", "hostName", "hostname", "instance", "keyword") != ""
	pageType := strings.ToLower(firstNonEmptyAssistantChatString(entities, "pageType"))
	if assistantChatQuestionLooksLikeUnsupportedFileOperation(question) {
		return aiopsmodel.AssistantChatIntentUnsupportedFileOp
	}
	if hasHost && assistantChatQuestionLooksLikeLogCleanup(question) {
		return aiopsmodel.AssistantChatIntentHostLogCleanup
	}
	if assistantChatQuestionLooksLikeDatabaseOverview(question, pageType) {
		return aiopsmodel.AssistantChatIntentDatabaseOverview
	}
	if assistantChatHasContextReference(question) && firstNonEmptyAssistantChatString(entities, "databaseName", "dbName", "assetId", "dbAssetId", "databaseAssetId") != "" {
		return aiopsmodel.AssistantChatIntentDatabaseOverview
	}

	if assistantChatQuestionLooksLikeK8sRollbackWorkload(question, pageType) {
		return aiopsmodel.AssistantChatIntentK8sRollbackWorkload
	}
	if assistantChatQuestionLooksLikeK8sRolloutRestart(question, pageType) {
		return aiopsmodel.AssistantChatIntentK8sRolloutRestart
	}
	if assistantChatQuestionLooksLikeK8sScaleWorkload(question, pageType) {
		return aiopsmodel.AssistantChatIntentK8sScaleWorkload
	}
	if assistantChatQuestionLooksLikeK8sOverview(question, pageType) {
		return aiopsmodel.AssistantChatIntentK8sOverview
	}
	// Explicit path operations are more specific than service keywords. For
	// example, node_exporter contains the substring "port" but still refers to
	// a file path in a question such as "查看 /opt/node_exporter/nohup.out".
	if hasHost && (assistantChatQuestionLooksLikeDiskUsageAnalysis(question) || assistantChatQuestionLooksLikePathInspection(question)) {
		return aiopsmodel.AssistantChatIntentHostPressure
	}
	if assistantChatQuestionLooksLikeHostServices(question, hasHost) {
		return aiopsmodel.AssistantChatIntentHostServices
	}
	if assistantChatQuestionLooksLikeHostInventory(question) {
		return aiopsmodel.AssistantChatIntentHostInventory
	}
	if hasHost && assistantChatQuestionLooksLikeHostPressure(question) {
		return aiopsmodel.AssistantChatIntentHostPressure
	}
	// Explicit cross-host resource rankings are more specific than broad
	// phrases such as "系统中当前有哪些".
	if assistantChatQuestionLooksLikeResourceGovernance(question) {
		return aiopsmodel.AssistantChatIntentResourceGovernance
	}
	if assistantChatQuestionLooksLikePlatformOverview(question) {
		return aiopsmodel.AssistantChatIntentPlatformOverview
	}
	if assistantChatQuestionLooksLikeKnowledgeSearch(question, pageType) {
		return aiopsmodel.AssistantChatIntentKnowledgeSearch
	}
	if assistantChatQuestionLooksLikeSkillCatalog(question) {
		return aiopsmodel.AssistantChatIntentSkillCatalog
	}
	if assistantChatQuestionLooksLikeOpsGeneral(question) {
		return aiopsmodel.AssistantChatIntentOpsGeneral
	}
	for _, definition := range assistantChatIntentRegistry() {
		pageMatched := assistantChatContextPageMatches(pageType, definition.ContextPages)
		keywordMatched := containsAnyAssistantChatKeyword(question, definition.Keywords)
		if definition.Name == aiopsmodel.AssistantChatIntentHostPressure {
			if hasHost && keywordMatched {
				return definition.Name
			}
			if pageMatched && assistantChatQuestionLooksPageScoped(question) {
				return definition.Name
			}
			if !hasHost && assistantChatQuestionLooksLikeHostPressureWithoutTarget(question) {
				return definition.Name
			}
			continue
		}
		if definition.Name == aiopsmodel.AssistantChatIntentHostServices {
			if hasHost && keywordMatched {
				return definition.Name
			}
			if pageMatched && assistantChatQuestionLooksPageScoped(question) && keywordMatched {
				return definition.Name
			}
			continue
		}
		if keywordMatched || pageMatched && assistantChatQuestionLooksPageScoped(question) {
			return definition.Name
		}
	}
	if assistantChatQuestionLooksLikeGeneralQA(question) {
		return aiopsmodel.AssistantChatIntentGeneralQA
	}
	return aiopsmodel.AssistantChatIntentUnknown
}

// IsAssistantActionConfirmation reports whether the entire user message is an
// explicit approval. It intentionally rejects approval words embedded in a
// longer sentence so normal questions cannot trigger a pending mutation.
func IsAssistantActionConfirmation(question string) bool {
	return assistantChatQuestionConfirmsPendingAction(question)
}

func assistantChatQuestionConfirmsPendingAction(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	question = strings.Trim(question, " \t\r\n,.!?;:，。！？；：")
	switch question {
	case "确认", "确定", "同意", "执行", "确认执行", "确定执行", "同意执行", "可以执行", "开始执行", "ok", "okay", "yes", "y":
		return true
	default:
		return false
	}
}

func detectAssistantChatEntities(req *aiopsmodel.AssistantChatRequest) map[string]interface{} {
	entities := map[string]interface{}{}
	if req == nil {
		return entities
	}
	applyAssistantChatPageContext(entities, req.Context)
	if hostID := assistantChatContextUInt(req.Context, "hostId"); hostID > 0 {
		entities["hostId"] = hostID
	}
	for _, key := range []string{
		"ip", "hostName", "hostname", "instance", "keyword", "severity",
		"fingerprint", "alertName", "groupName", "vendor", "region", "env",
		"environment", "databaseName", "dbName", "clusterName", "namespace",
		"pod", "podName", "node", "kind", "workloadType", "workloadName",
		"service", "ingress", "container", "resourceName", "path", "imageVersion",
	} {
		if value := assistantChatContextString(req.Context, key); value != "" {
			entities[key] = value
		}
	}
	for _, key := range []string{"alertId", "groupId", "businessId", "appId", "authId", "assetId", "dbAssetId", "databaseAssetId", "clusterId", "k8sClusterId"} {
		if value := assistantChatContextUInt(req.Context, key); value > 0 {
			entities[key] = value
		}
	}
	if match := assistantChatQuestionIP(req.Question); match != "" {
		entities["ip"] = match
		entities["explicitIp"] = true
		delete(entities, "hostId")
		delete(entities, "pageResourceId")
	}
	if hostName := assistantChatQuestionHostName(req.Question); hostName != "" {
		entities["hostName"] = hostName
		entities["keyword"] = hostName
		delete(entities, "hostId")
		delete(entities, "pageResourceId")
	}
	if assistantChatQuestionLooksLikeDiskUsageAnalysis(req.Question) {
		entities["diskUsageAnalysis"] = true
	}
	if path := assistantChatQuestionLinuxPath(req.Question); path != "" {
		entities["path"] = path
		if assistantChatQuestionLooksLikeLogCleanup(req.Question) {
			entities["logCleanup"] = true
		} else if !assistantChatBool(entities["diskUsageAnalysis"]) && assistantChatQuestionLooksLikePathInspection(req.Question) {
			entities["pathInspection"] = true
		}
	}
	if name := assistantChatQuestionDatabaseName(req.Question); name != "" {
		entities["databaseName"] = name
		entities["keyword"] = name
		delete(entities, "hostId")
		delete(entities, "ip")
		delete(entities, "explicitIp")
		delete(entities, "pageResourceId")
	}
	mergeAssistantChatQuestionK8sEntities(entities, req.Question)
	if revision, ok := assistantChatQuestionK8sRollbackRevision(req.Question); ok {
		entities["revision"] = revision
		delete(entities, "imageVersion")
	} else if imageVersion := assistantChatQuestionK8sRollbackImageVersion(req.Question); imageVersion != "" {
		entities["imageVersion"] = imageVersion
		delete(entities, "revision")
	}
	if assistantChatQuestionAsksK8sWorkloadDiagnosis(req.Question) {
		entities["workloadDiagnosis"] = true
	}
	if assistantChatQuestionAsksK8sWorkloadStatus(req.Question) {
		entities["workloadStatusQuery"] = true
	}
	if assistantChatQuestionAsksK8sReplicaCount(req.Question) {
		entities["replicaCountQuery"] = true
	}
	if assistantChatQuestionAsksK8sWorkloadHistory(req.Question) {
		entities["workloadHistory"] = true
	}
	if replicas, ok := assistantChatQuestionK8sReplicas(req.Question); ok {
		entities["replicas"] = replicas
		delete(entities, "replicaDelta")
	} else if delta, ok := assistantChatQuestionK8sReplicaDelta(req.Question); ok {
		entities["replicaDelta"] = delta
		delete(entities, "replicas")
	}
	if minutes := parseAssistantChatMinutes(req.Question); minutes > 0 {
		entities["timeRangeMinutes"] = minutes
	}
	return entities
}

func mergeAssistantChatQuestionK8sEntities(entities map[string]interface{}, question string) {
	if entities == nil {
		return
	}
	text := strings.TrimSpace(question)
	lower := strings.ToLower(text)
	if !containsAnyAssistantChatKeyword(lower, []string{"k8s", "kubernetes", "pod", "node", "deployment", "statefulset", "daemonset", "job", "service", "ingress", "集群", "容器", "命名空间", "节点", "工作负载"}) {
		return
	}
	for _, match := range assistantChatK8sResourcePattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 3 {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(match[1]))
		name := strings.TrimSpace(match[2])
		switch kind {
		case "pod":
			entities["podName"] = name
		case "node":
			entities["node"] = name
		case "namespace":
			entities["namespace"] = name
		case "service":
			entities["service"] = name
		case "ingress":
			entities["ingress"] = name
		default:
			entities["workloadType"] = kind
			entities["workloadName"] = name
		}
	}
	for _, match := range assistantChatK8sChineseResourcePattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 3 {
			continue
		}
		name := strings.TrimSpace(match[2])
		switch strings.TrimSpace(match[1]) {
		case "集群":
			entities["clusterName"] = name
			entities["keyword"] = name
		case "命名空间":
			entities["namespace"] = name
		case "节点":
			entities["node"] = name
		}
	}
	if match := assistantChatK8sNamedWorkloadPattern.FindStringSubmatch(text); len(match) >= 2 {
		entities["workloadName"] = strings.TrimSpace(match[1])
	}
	if match := assistantChatK8sServiceWorkloadPattern.FindStringSubmatch(text); len(match) >= 2 {
		entities["workloadName"] = strings.TrimSpace(match[1])
	}
	if match := assistantChatK8sDirectWorkloadPattern.FindStringSubmatch(text); len(match) >= 2 {
		entities["workloadName"] = strings.TrimSpace(match[1])
	}
	if match := assistantChatK8sWorkloadHistoryPattern.FindStringSubmatch(text); len(match) >= 2 {
		entities["workloadName"] = strings.TrimSpace(match[1])
	}
}

func assistantChatQuestionK8sReplicas(question string) (int, bool) {
	for _, pattern := range assistantChatK8sReplicaPatterns {
		match := pattern.FindStringSubmatch(strings.TrimSpace(question))
		if len(match) < 2 {
			continue
		}
		replicas, err := strconv.Atoi(match[1])
		if err == nil && replicas >= 0 {
			return replicas, true
		}
	}
	return 0, false
}

func assistantChatQuestionK8sReplicaDelta(question string) (int, bool) {
	for _, item := range assistantChatK8sReplicaDeltaPatterns {
		match := item.pattern.FindStringSubmatch(strings.TrimSpace(question))
		if len(match) < 2 {
			continue
		}
		value, ok := assistantChatSmallPositiveInteger(match[1])
		if ok {
			return item.sign * value, true
		}
	}
	return 0, false
}

func assistantChatQuestionK8sRollbackRevision(question string) (int64, bool) {
	match := assistantChatK8sRollbackRevisionPattern.FindStringSubmatch(strings.TrimSpace(question))
	if len(match) < 2 {
		return 0, false
	}
	revision, err := strconv.ParseInt(match[1], 10, 64)
	return revision, err == nil && revision > 0
}

func assistantChatQuestionK8sRollbackImageVersion(question string) string {
	match := assistantChatK8sRollbackImagePattern.FindStringSubmatch(strings.TrimSpace(question))
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func assistantChatSmallPositiveInteger(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
		return parsed, true
	}
	values := map[string]int{"一": 1, "二": 2, "两": 2, "三": 3, "四": 4, "五": 5, "六": 6, "七": 7, "八": 8, "九": 9, "十": 10}
	parsed, ok := values[value]
	return parsed, ok
}

func assistantK8sDesiredReplicas(output interface{}) (int, bool) {
	data := assistantChatMap(output)
	for _, key := range []string{"desiredReplicas", "replicas"} {
		value, exists := data[key]
		if !exists {
			continue
		}
		switch value.(type) {
		case int, int64, uint, uint64, float32, float64, json.Number:
		default:
			continue
		}
		replicas := assistantChatNumber(value)
		if replicas >= 0 && replicas == math.Trunc(replicas) {
			return int(replicas), true
		}
	}
	return 0, false
}

func applyAssistantChatPageContext(entities map[string]interface{}, context map[string]interface{}) {
	if entities == nil || len(context) == 0 {
		return
	}
	pageType := firstNonEmptyAssistantChatContextString(context, "pageType", "page", "sourcePage", "scene", "module")
	if pageType != "" {
		entities["pageType"] = pageType
	}
	pageResourceID := assistantChatFirstContextUInt(context, "pageResourceId", "resourceId", "currentId", "id")
	if pageResourceID > 0 {
		entities["pageResourceId"] = pageResourceID
		switch strings.ToLower(pageType) {
		case "host", "host_detail", "cmdb_host", "asset_host":
			if assistantChatValueEmpty(entities["hostId"]) {
				entities["hostId"] = pageResourceID
			}
		case "alert", "alerts", "alert_detail", "monitor_alert":
			if assistantChatValueEmpty(entities["alertId"]) {
				entities["alertId"] = pageResourceID
			}
		case "db", "database", "db_asset", "database_asset", "db_detail", "database_detail", "slow_log", "sql_review":
			if assistantChatValueEmpty(entities["assetId"]) {
				entities["assetId"] = pageResourceID
			}
		case "k8s", "kubernetes", "k8s_cluster", "k8s_cluster_detail":
			if assistantChatValueEmpty(entities["clusterId"]) {
				entities["clusterId"] = pageResourceID
			}
		case "group", "host_group", "cmdb_group":
			if assistantChatValueEmpty(entities["groupId"]) {
				entities["groupId"] = pageResourceID
			}
		}
	}
	if host := assistantChatMap(context["host"]); len(host) > 0 {
		mergeAssistantChatEntityMap(entities, host, map[string]string{
			"id":        "hostId",
			"hostId":    "hostId",
			"hostName":  "hostName",
			"name":      "name",
			"privateIp": "ip",
			"sshIp":     "ip",
			"publicIp":  "publicIp",
			"groupId":   "groupId",
			"groupName": "groupName",
			"os":        "os",
		})
	}
	if alert := assistantChatMap(context["alert"]); len(alert) > 0 {
		mergeAssistantChatEntityMap(entities, alert, map[string]string{
			"id":          "alertId",
			"alertId":     "alertId",
			"fingerprint": "fingerprint",
			"alertName":   "alertName",
			"alertname":   "alertName",
			"severity":    "severity",
			"instance":    "instance",
			"host":        "instance",
		})
	}
	if group := assistantChatMap(context["group"]); len(group) > 0 {
		mergeAssistantChatEntityMap(entities, group, map[string]string{
			"id":        "groupId",
			"groupId":   "groupId",
			"name":      "groupName",
			"groupName": "groupName",
		})
	}
	if dbAsset := assistantChatMap(context["database"]); len(dbAsset) > 0 {
		mergeAssistantChatEntityMap(entities, dbAsset, map[string]string{
			"id":           "assetId",
			"assetId":      "assetId",
			"name":         "databaseName",
			"instanceName": "instance",
			"hostIp":       "ip",
			"endpoint":     "endpoint",
			"environment":  "environment",
			"env":          "env",
		})
	}
	if dbAsset := assistantChatMap(context["dbAsset"]); len(dbAsset) > 0 {
		mergeAssistantChatEntityMap(entities, dbAsset, map[string]string{
			"id":           "assetId",
			"assetId":      "assetId",
			"name":         "databaseName",
			"instanceName": "instance",
			"hostIp":       "ip",
			"endpoint":     "endpoint",
			"environment":  "environment",
			"env":          "env",
		})
	}
	if cluster := assistantChatMap(context["cluster"]); len(cluster) > 0 {
		mergeAssistantChatEntityMap(entities, cluster, map[string]string{
			"id":        "clusterId",
			"clusterId": "clusterId",
			"name":      "clusterName",
			"namespace": "namespace",
		})
	}
	if k8sObj := assistantChatMap(context["k8s"]); len(k8sObj) > 0 {
		mergeAssistantChatEntityMap(entities, k8sObj, map[string]string{
			"clusterId":    "clusterId",
			"namespace":    "namespace",
			"pod":          "podName",
			"podName":      "podName",
			"node":         "node",
			"kind":         "kind",
			"workloadType": "workloadType",
			"workloadName": "workloadName",
			"service":      "service",
			"ingress":      "ingress",
			"container":    "container",
			"name":         "clusterName",
		})
	}
}

func normalizeAssistantChatDomainEntities(question string, entities map[string]interface{}) {
	if entities == nil {
		return
	}
	pageType := firstNonEmptyAssistantChatString(entities, "pageType")
	if !assistantChatQuestionLooksLikeK8sDomain(strings.ToLower(strings.TrimSpace(question)), pageType) {
		return
	}
	if !assistantChatContextPageMatches(strings.ToLower(pageType), []string{
		"k8s", "kubernetes", "k8s_cluster", "k8s_cluster_detail", "k8s_pod", "k8s_workload", "k8s_namespace", "k8s_ingress", "k8s_service",
	}) {
		delete(entities, "pageResourceId")
	}
	for _, key := range []string{
		"hostId", "ip", "hostName", "hostname", "instance", "privateIp", "publicIp", "sshIp", "groupName", "os",
		"assetId", "dbAssetId", "databaseAssetId", "databaseName", "dbName", "endpoint",
		"alertId", "fingerprint", "alertName", "severity",
	} {
		delete(entities, key)
	}
}

func mergeAssistantChatEntityMap(entities map[string]interface{}, source map[string]interface{}, keyMap map[string]string) {
	for sourceKey, targetKey := range keyMap {
		value, ok := source[sourceKey]
		if !ok || assistantChatValueEmpty(value) || !assistantChatValueEmpty(entities[targetKey]) {
			continue
		}
		entities[targetKey] = value
	}
}

func mergeAssistantChatMemoryEntities(entities map[string]interface{}, memory *aiopsmodel.AssistantChatMemoryContext, question string) {
	if entities == nil || memory == nil || len(memory.Entities) == 0 {
		return
	}
	pathFollowup := assistantChatQuestionLinuxPath(question) != "" && (assistantChatQuestionLooksLikePathInspection(question) ||
		assistantChatQuestionLooksLikeDiskUsageAnalysis(question) ||
		assistantChatQuestionLooksLikeLogCleanup(question))
	if !assistantChatHasContextReference(question) && !pathFollowup {
		return
	}
	explicitIP := assistantChatBool(entities["explicitIp"])
	databaseQuestion := assistantChatQuestionLooksLikeDatabaseOverview(question, firstNonEmptyAssistantChatString(entities, "pageType"))
	k8sQuestion := assistantChatQuestionLooksLikeK8sDomain(strings.ToLower(strings.TrimSpace(question)), firstNonEmptyAssistantChatString(entities, "pageType"))
	explicitReplicaChange := assistantChatQuestionLooksLikeK8sScaleWorkload(question, "")
	explicitRollbackVersion := assistantChatQuestionLooksLikeK8sRollbackWorkload(question, "") &&
		(int64(assistantChatNumber(entities["revision"])) > 0 || firstNonEmptyAssistantChatString(entities, "imageVersion") != "")
	explicitWorkload := !assistantChatQuestionLooksLikeK8sTargetSelectionFollowup(question) && (assistantChatK8sNamedWorkloadPattern.MatchString(question) ||
		assistantChatK8sServiceWorkloadPattern.MatchString(question) ||
		assistantChatK8sDirectWorkloadPattern.MatchString(question) ||
		assistantChatK8sWorkloadHistoryPattern.MatchString(question))
	for _, key := range []string{
		"hostId", "ip", "hostName", "hostname", "name", "instance", "authId",
		"privateIp", "publicIp", "sshIp", "groupName", "os",
		"alertId", "fingerprint", "severity",
		"assetId", "dbAssetId", "databaseAssetId", "databaseName", "dbName",
		"clusterId", "k8sClusterId", "clusterName", "namespace", "pod", "podName", "node", "kind", "workloadType", "workloadName", "service", "ingress", "container", "replicas", "replicaDelta", "revision", "imageVersion",
	} {
		if k8sQuestion && !assistantChatIsK8sMemoryKey(key) {
			continue
		}
		if explicitWorkload && assistantChatIsK8sTargetMemoryKey(key) {
			continue
		}
		if explicitReplicaChange && (key == "replicas" || key == "replicaDelta") {
			continue
		}
		if explicitRollbackVersion && (key == "revision" || key == "imageVersion") {
			continue
		}
		if (explicitIP || databaseQuestion) && assistantChatIsHostMemoryKey(key) {
			continue
		}
		if key == "replicas" && int(assistantChatNumber(entities["replicaDelta"])) != 0 {
			continue
		}
		if !assistantChatValueEmpty(entities[key]) {
			continue
		}
		if value, ok := memory.Entities[key]; ok && !assistantChatValueEmpty(value) {
			entities[key] = value
			memory.Used = true
		}
	}
}

func assistantChatIsK8sTargetMemoryKey(key string) bool {
	switch key {
	case "clusterId", "k8sClusterId", "clusterName", "namespace", "pod", "podName", "workloadName", "service", "ingress", "container":
		return true
	default:
		return false
	}
}

func assistantChatIsK8sMemoryKey(key string) bool {
	switch key {
	case "clusterId", "k8sClusterId", "clusterName", "namespace", "pod", "podName", "node", "kind", "workloadType", "workloadName", "service", "ingress", "container", "replicas", "replicaDelta", "revision", "imageVersion":
		return true
	default:
		return false
	}
}

func assistantChatIsHostMemoryKey(key string) bool {
	switch key {
	case "hostId", "ip", "hostName", "hostname", "name", "instance", "privateIp", "publicIp", "sshIp", "groupName", "os", "authId":
		return true
	default:
		return false
	}
}

func buildAssistantChatToolPlan(intent string, entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	if intent == aiopsmodel.AssistantChatIntentActionConfirmation {
		return withAssistantChatLifecycleSteps(intent, []aiopsmodel.AssistantChatToolPlanStep{{
			ToolName: "confirm_pending_action", Purpose: "校验并执行当前会话中最近一次已签名的待确认操作", Status: aiopsmodel.AssistantChatPlanPending,
		}})
	}
	for _, definition := range assistantChatIntentRegistry() {
		if definition.Name == intent && definition.ToolPlan != nil {
			return withAssistantChatLifecycleSteps(intent, definition.ToolPlan(entities))
		}
	}
	return nil
}

func withAssistantChatLifecycleSteps(intent string, plan []aiopsmodel.AssistantChatToolPlanStep) []aiopsmodel.AssistantChatToolPlanStep {
	result := make([]aiopsmodel.AssistantChatToolPlanStep, 0, len(plan)+2)
	result = append(result, aiopsmodel.AssistantChatToolPlanStep{
		Step:     1,
		ToolName: "understand_requirement",
		Purpose:  "理解用户问题，识别意图和上下文对象",
		Status:   aiopsmodel.AssistantChatPlanPending,
	})
	for _, item := range plan {
		item.Step = len(result) + 1
		if item.Status == "" {
			item.Status = aiopsmodel.AssistantChatPlanPending
		}
		result = append(result, item)
	}
	result = append(result, aiopsmodel.AssistantChatToolPlanStep{
		Step:     len(result) + 1,
		ToolName: assistantChatSummaryStepToolName(intent),
		Purpose:  assistantChatSummaryStepPurpose(intent),
		Status:   aiopsmodel.AssistantChatPlanPending,
	})
	return result
}

func assistantChatSummaryStepToolName(intent string) string {
	switch intent {
	case aiopsmodel.AssistantChatIntentSkillCatalog, aiopsmodel.AssistantChatIntentOpsGeneral, aiopsmodel.AssistantChatIntentGeneralQA, aiopsmodel.AssistantChatIntentUnsupportedFileOp:
		return "direct_answer"
	default:
		return "ai_summary"
	}
}

func assistantChatLLMStepToolName(intent string) string {
	switch intent {
	case aiopsmodel.AssistantChatIntentOpsGeneral:
		return "llm_ops_general"
	case aiopsmodel.AssistantChatIntentGeneralQA:
		return "llm_general_qa"
	default:
		return "ai_summary"
	}
}

func assistantChatSummaryStepPurpose(intent string) string {
	switch intent {
	case aiopsmodel.AssistantChatIntentActionConfirmation:
		return "返回操作触发结果和后续进度查询方式"
	case aiopsmodel.AssistantChatIntentUnsupportedFileOp:
		return "说明当前允许的日志文件操作范围"
	case aiopsmodel.AssistantChatIntentResourceGovernance:
		return "根据候选清单、监控指标和风险项生成总结评分"
	case aiopsmodel.AssistantChatIntentSkillCatalog:
		return "返回 AutoOps 已加载的声明式 Skill 清单"
	case aiopsmodel.AssistantChatIntentOpsGeneral, aiopsmodel.AssistantChatIntentGeneralQA:
		return "直接生成通用回答"
	default:
		return "汇总证据、风险和下一步建议"
	}
}

func buildAssistantInvestigationPlan(intent, question string, toolPlan []aiopsmodel.AssistantChatToolPlanStep, entities map[string]interface{}) *aiopsmodel.AssistantInvestigationPlan {
	goal := "回答用户问题，并在只读边界内补齐必要证据。"
	strategy := "先识别目标对象和问题类型，再按工具计划采集证据，最后输出结论、缺失数据和下一步建议。"
	switch intent {
	case aiopsmodel.AssistantChatIntentActionConfirmation:
		goal = "确认并执行当前会话中最近一次待确认操作。"
		strategy = "只使用已签名且未过期的 confirmationId，重新校验当前账号权限并执行一次；不接受新的目标参数。"
	case aiopsmodel.AssistantChatIntentPlatformOverview:
		goal = "形成平台级健康和风险总览。"
		strategy = "同时查看活跃告警、主机资产、数据库资产和 K8s 集群摘要，必要时检索知识库补充 Runbook。"
	case aiopsmodel.AssistantChatIntentHostPressure:
		goal = "判断指定主机是否存在资源压力或采集异常。"
		strategy = "先定位 CMDB 主机，再检查 Agent 在线状态和 CPU、内存、磁盘、负载、Top 进程指标。"
	case aiopsmodel.AssistantChatIntentHostServices:
		goal = "识别指定主机上疑似运行的服务。"
		strategy = "先定位主机，再读取 Top 进程并推断服务清单。"
	case aiopsmodel.AssistantChatIntentHostLogCleanup:
		goal = "为指定主机日志生成保留文件的清空预览和二次确认凭证。"
		strategy = "先定位主机并核对日志元数据，再固化清空命令；本轮不执行清空。"
	case aiopsmodel.AssistantChatIntentAlertOverview:
		goal = "汇总当前活跃告警风险。"
		strategy = "读取活跃告警列表，按数量、级别和对象形成优先级判断。"
	case aiopsmodel.AssistantChatIntentDatabaseOverview:
		goal = "分析数据库资产上下文和可见风险。"
		strategy = "读取数据库资产清单、实例状态和负责人信息，不执行 SQL 或变更操作。"
	case aiopsmodel.AssistantChatIntentK8sOverview:
		goal = "分析 K8s 集群上下文和可见风险。"
		strategy = "读取集群登记、节点数量和同步状态，不修改任何集群资源。"
	case aiopsmodel.AssistantChatIntentK8sRolloutRestart:
		goal = "为指定 K8s 工作负载生成滚动重启预览和确认凭证。"
		strategy = "核对集群、Namespace、工作负载类型和名称并执行权限检查；本轮不执行重启。"
	case aiopsmodel.AssistantChatIntentK8sScaleWorkload:
		goal = "为指定 K8s 工作负载生成副本数调整预览和确认凭证。"
		strategy = "跨授权集群精确定位 Deployment，核对目标副本数并执行权限检查；本轮不执行扩缩容。"
	case aiopsmodel.AssistantChatIntentK8sRollbackWorkload:
		goal = "为指定 K8s Deployment 生成版本回滚预览和确认凭证。"
		strategy = "精确定位 Deployment，读取历史版本并将 revision 或镜像版本解析为唯一 revision；本轮不执行回滚。"
	case aiopsmodel.AssistantChatIntentKnowledgeSearch:
		goal = "从知识库找到可复用的 Runbook、故障案例或说明文档。"
		strategy = "按用户问题检索知识库，返回命中文档并提醒核对适用环境。"
	case aiopsmodel.AssistantChatIntentSkillCatalog:
		goal = "列出 AutoOps 后端当前加载的声明式 Skill。"
		strategy = "读取 Skill Catalog，按技能分组、状态和风险等级输出清单，不调用生产工具。"
	case aiopsmodel.AssistantChatIntentResourceGovernance:
		goal = "生成资源治理或容量分析的结构化结果。"
		strategy = "转交资源治理查询能力，使用只读数据形成候选清单和建议。"
	case aiopsmodel.AssistantChatIntentOpsGeneral, aiopsmodel.AssistantChatIntentGeneralQA:
		goal = "回答通用运维或知识问题。"
		strategy = "优先直接回答，不调用生产工具；涉及实时平台数据时提示补充对象。"
	case aiopsmodel.AssistantChatIntentClarify:
		goal = "补齐执行调查所需的关键对象。"
		strategy = "先向用户澄清主机、告警、数据库实例或集群等必要信息。"
	}

	steps := make([]aiopsmodel.AssistantInvestigationStep, 0, len(toolPlan))
	for _, item := range toolPlan {
		status := item.Status
		if status == "" {
			status = aiopsmodel.AssistantChatPlanPending
		}
		required := status != aiopsmodel.AssistantChatPlanSkipped
		steps = append(steps, aiopsmodel.AssistantInvestigationStep{
			Step:      item.Step,
			Name:      assistantChatToolPurpose(item.ToolName),
			Purpose:   item.Purpose,
			Tools:     []string{item.ToolName},
			Required:  required,
			Status:    status,
			DependsOn: assistantInvestigationDependsOn(item.Step, intent),
		})
	}
	if len(steps) == 0 {
		steps = append(steps, aiopsmodel.AssistantInvestigationStep{
			Step:     1,
			Name:     "直接回答",
			Purpose:  "基于用户问题和已有上下文生成回答",
			Required: true,
			Status:   aiopsmodel.AssistantChatPlanPending,
		})
	}

	assumptions := []string{"第一阶段所有工具均为只读调查能力。"}
	if strings.TrimSpace(question) != "" {
		assumptions = append(assumptions, "用户问题："+truncateAssistantChatText(question, 120))
	}
	if len(entities) > 0 {
		assumptions = append(assumptions, "已抽取实体会优先作为工具参数，缺失对象时进入澄清。")
	}
	return &aiopsmodel.AssistantInvestigationPlan{
		Goal:        goal,
		Strategy:    strategy,
		Steps:       steps,
		Budget:      aiopsmodel.AssistantInvestigationBudget{MaxSteps: len(steps), TimeoutSeconds: 60, MaxToolCalls: assistantInvestigationMaxToolCalls(steps)},
		Assumptions: assumptions,
	}
}

func assistantInvestigationDependsOn(step int, intent string) []int {
	if step <= 1 {
		return nil
	}
	switch intent {
	case aiopsmodel.AssistantChatIntentHostPressure, aiopsmodel.AssistantChatIntentHostServices, aiopsmodel.AssistantChatIntentHostLogCleanup:
		return []int{1}
	default:
		return nil
	}
}

func assistantInvestigationMaxToolCalls(steps []aiopsmodel.AssistantInvestigationStep) int {
	total := 0
	for _, step := range steps {
		if step.Required && len(step.Tools) > 0 {
			total += len(step.Tools)
		}
	}
	if total == 0 {
		return 1
	}
	return total
}

func buildAssistantChatHostInventoryToolPlan(_ map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	return []aiopsmodel.AssistantChatToolPlanStep{
		{
			ToolName:  "aiops_context_hosts",
			Purpose:   "读取当前用户授权范围内的主机资产总数和认证状态",
			Arguments: map[string]interface{}{"limit": 10},
			Status:    aiopsmodel.AssistantChatPlanPending,
		},
	}
}

func buildAssistantChatHostPressureToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	plan := []aiopsmodel.AssistantChatToolPlanStep{}
	add := func(toolName, purpose string, args map[string]interface{}) {
		plan = append(plan, aiopsmodel.AssistantChatToolPlanStep{
			Step:      len(plan) + 1,
			ToolName:  toolName,
			Purpose:   purpose,
			Arguments: args,
			Status:    aiopsmodel.AssistantChatPlanPending,
		})
	}
	if assistantChatNumber(entities["hostId"]) > 0 {
		add("get_cmdb_host", "根据 hostId 获取主机资产上下文", map[string]interface{}{"hostId": entities["hostId"]})
	} else if ip := firstNonEmptyAssistantChatString(entities, "ip"); ip != "" {
		add("get_cmdb_host", "根据 IP 精确获取主机资产上下文", map[string]interface{}{"ip": ip})
	} else {
		add("search_cmdb_hosts", "根据用户输入定位 CMDB 主机", map[string]interface{}{"keyword": firstNonEmptyAssistantChatString(entities, "ip", "hostName", "hostname", "instance", "keyword")})
	}
	if assistantChatBool(entities["pathInspection"]) {
		args := map[string]interface{}{
			"hostId": "$resolved.hostId", "path": firstNonEmptyAssistantChatString(entities, "path"),
			"maxEntries": 100, "tailLines": 500,
		}
		if authID := assistantChatNumber(entities["authId"]); authID > 0 {
			args["authId"] = authID
		}
		add("ecs_inspect_path", "列出目录内容或读取文本文件最后 500 行", args)
		return plan
	}
	if assistantChatBool(entities["diskUsageAnalysis"]) {
		args := map[string]interface{}{
			"hostId": "$resolved.hostId", "path": "/", "depth": 2, "topN": 5, "minFileSizeMB": 100,
		}
		if authID := assistantChatNumber(entities["authId"]); authID > 0 {
			args["authId"] = authID
		}
		add("ecs_analyze_disk_usage", "分析主机占用空间最大的目录和文件", args)
		return plan
	}
	add("get_agent_status", "确认采集 Agent 是否在线", map[string]interface{}{"hostId": "$resolved.hostId"})
	add("query_host_metrics", "查询 CPU、内存、磁盘、负载和 Top 进程", map[string]interface{}{
		"hostId":  "$resolved.hostId",
		"minutes": assistantChatNumber(entities["timeRangeMinutes"]),
		"step":    defaultAssistantChatStep,
	})
	return plan
}

func buildAssistantChatHostLogCleanupToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	plan := []aiopsmodel.AssistantChatToolPlanStep{}
	add := func(toolName, purpose string, args map[string]interface{}) {
		plan = append(plan, aiopsmodel.AssistantChatToolPlanStep{
			Step: len(plan) + 1, ToolName: toolName, Purpose: purpose,
			Arguments: args, Status: aiopsmodel.AssistantChatPlanPending,
		})
	}
	if assistantChatNumber(entities["hostId"]) > 0 {
		add("get_cmdb_host", "根据 hostId 获取主机资产上下文", map[string]interface{}{"hostId": entities["hostId"]})
	} else if ip := firstNonEmptyAssistantChatString(entities, "ip"); ip != "" {
		add("get_cmdb_host", "根据 IP 精确获取主机资产上下文", map[string]interface{}{"ip": ip})
	} else {
		add("search_cmdb_hosts", "根据用户输入定位 CMDB 主机", map[string]interface{}{"keyword": firstNonEmptyAssistantChatString(entities, "ip", "hostName", "hostname", "instance", "keyword")})
	}
	args := map[string]interface{}{
		"hostId": "$resolved.hostId",
		"path":   firstNonEmptyAssistantChatString(entities, "path"),
	}
	if authID := assistantChatNumber(entities["authId"]); authID > 0 {
		args["authId"] = authID
	}
	add("ecs_request_cleanup_logs", "生成保留文件的日志清空预览和二次确认凭证", args)
	return plan
}

func buildAssistantChatUnsupportedFileOperationToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	return []aiopsmodel.AssistantChatToolPlanStep{{
		Step: 1, ToolName: "direct_answer", Purpose: "说明当前允许的日志文件操作范围",
		Status: aiopsmodel.AssistantChatPlanPending,
	}}
}

func buildAssistantChatHostServicesToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	plan := []aiopsmodel.AssistantChatToolPlanStep{}
	add := func(toolName, purpose string, args map[string]interface{}) {
		plan = append(plan, aiopsmodel.AssistantChatToolPlanStep{
			Step:      len(plan) + 1,
			ToolName:  toolName,
			Purpose:   purpose,
			Arguments: args,
			Status:    aiopsmodel.AssistantChatPlanPending,
		})
	}
	if assistantChatNumber(entities["hostId"]) > 0 {
		add("get_cmdb_host", "根据 hostId 获取主机资产上下文", map[string]interface{}{"hostId": entities["hostId"]})
	} else if ip := firstNonEmptyAssistantChatString(entities, "ip"); ip != "" {
		add("get_cmdb_host", "根据 IP 精确获取主机资产上下文", map[string]interface{}{"ip": ip})
	} else {
		add("search_cmdb_hosts", "根据用户输入定位 CMDB 主机", map[string]interface{}{"keyword": firstNonEmptyAssistantChatString(entities, "ip", "hostName", "hostname", "instance", "keyword")})
	}
	add("get_agent_status", "确认采集 Agent 是否在线", map[string]interface{}{"hostId": "$resolved.hostId"})
	add("query_host_services", "查询主机 Top 进程并推断已部署服务", map[string]interface{}{"hostId": "$resolved.hostId"})
	return plan
}

func buildAssistantChatAlertToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	plan := []aiopsmodel.AssistantChatToolPlanStep{}
	plan = append(plan, aiopsmodel.AssistantChatToolPlanStep{
		Step:     1,
		ToolName: "query_active_alerts",
		Purpose:  "查询当前活跃告警概览",
		Arguments: map[string]interface{}{
			"page":     1,
			"pageSize": 10,
			"keyword":  firstNonEmptyAssistantChatString(entities, "keyword", "ip", "hostName", "hostname", "name", "instance", "alertName", "fingerprint"),
			"severity": firstNonEmptyAssistantChatString(entities, "severity"),
		},
		Status: aiopsmodel.AssistantChatPlanPending,
	})
	return plan
}

func buildAssistantChatPlatformToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	return []aiopsmodel.AssistantChatToolPlanStep{
		{
			Step:      1,
			ToolName:  "query_active_alerts",
			Purpose:   "查询当前活跃告警，作为全局风险入口",
			Arguments: map[string]interface{}{"page": 1, "pageSize": 10},
			Status:    aiopsmodel.AssistantChatPlanPending,
		},
		{
			Step:      2,
			ToolName:  "aiops_context_hosts",
			Purpose:   "读取主机资产摘要",
			Arguments: map[string]interface{}{"limit": 10},
			Status:    aiopsmodel.AssistantChatPlanPending,
		},
		{
			Step:      3,
			ToolName:  "aiops_context_database",
			Purpose:   "读取数据库资产摘要，不执行 SQL",
			Arguments: map[string]interface{}{"scope": "all"},
			Status:    aiopsmodel.AssistantChatPlanPending,
		},
		{
			Step:      4,
			ToolName:  "aiops_context_k8s",
			Purpose:   "读取 K8s 集群摘要，不访问或修改集群资源",
			Arguments: map[string]interface{}{"scope": "all"},
			Status:    aiopsmodel.AssistantChatPlanPending,
		},
		{
			Step:      5,
			ToolName:  "knowledge_search",
			Purpose:   "按需检索运维知识库，补充 Runbook 或故障案例",
			Arguments: map[string]interface{}{"query": "$user.question", "limit": 5},
			Status:    aiopsmodel.AssistantChatPlanSkipped,
		},
	}
}

func buildAssistantChatResourceGovernanceToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	return []aiopsmodel.AssistantChatToolPlanStep{
		{
			ToolName:  "resource_scope",
			Purpose:   "确定资源治理范围、时间窗口和筛选阈值",
			Arguments: map[string]interface{}{"question": "$user.question"},
			Status:    aiopsmodel.AssistantChatPlanPending,
		},
		{
			ToolName:  "assistant_query",
			Purpose:   "调用专用资源治理助手，扫描 CMDB、Agent 和监控指标",
			Arguments: map[string]interface{}{"question": "$user.question"},
			Status:    aiopsmodel.AssistantChatPlanPending,
		},
		{
			ToolName: "resource_metrics",
			Purpose:  "整理 CPU、内存、磁盘和服务指标，形成候选评分",
			Status:   aiopsmodel.AssistantChatPlanPending,
		},
	}
}

func buildAssistantChatDatabaseToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	return []aiopsmodel.AssistantChatToolPlanStep{
		{
			Step:     1,
			ToolName: "aiops_context_database",
			Purpose:  "读取数据库资产上下文，不执行 SQL",
			Arguments: map[string]interface{}{
				"assetId": assistantChatFirstEntityUInt(entities, "assetId", "dbAssetId", "databaseAssetId", "pageResourceId"),
				"keyword": firstNonEmptyAssistantChatString(entities, "keyword", "databaseName", "dbName", "instance", "ip"),
				"env":     firstNonEmptyAssistantChatString(entities, "env", "environment"),
			},
			Status: aiopsmodel.AssistantChatPlanPending,
		},
	}
}

func buildAssistantChatK8sToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	if assistantChatBool(entities["workloadDiagnosis"]) {
		workloadName := firstNonEmptyAssistantChatString(entities, "workloadName")
		return []aiopsmodel.AssistantChatToolPlanStep{
			{Step: 1, ToolName: "k8s_list_clusters", Purpose: "读取当前用户授权的全部 K8s 集群", Arguments: map[string]interface{}{"clusterId": assistantChatFirstEntityUInt(entities, "clusterId", "k8sClusterId"), "keyword": firstNonEmptyAssistantChatString(entities, "clusterName")}, Status: aiopsmodel.AssistantChatPlanPending},
			{Step: 2, ToolName: "k8s_search_resources", Purpose: "在授权 Namespace 中精确搜索同名 Deployment", Arguments: map[string]interface{}{"kind": "deployment", "keyword": workloadName, "exactName": true}, Status: aiopsmodel.AssistantChatPlanPending},
			{Step: 3, ToolName: "k8s_diagnose_workload", Purpose: "复用 Pod AI 分析任务采集日志、事件、资源和配置并诊断根因", Arguments: map[string]interface{}{"workloadName": workloadName}, Status: aiopsmodel.AssistantChatPlanPending},
		}
	}
	if assistantChatBool(entities["workloadStatusQuery"]) || assistantChatBool(entities["replicaCountQuery"]) {
		workloadName := firstNonEmptyAssistantChatString(entities, "workloadName")
		return []aiopsmodel.AssistantChatToolPlanStep{
			{Step: 1, ToolName: "k8s_list_clusters", Purpose: "读取当前用户授权的全部 K8s 集群", Arguments: map[string]interface{}{"clusterId": assistantChatFirstEntityUInt(entities, "clusterId", "k8sClusterId"), "keyword": firstNonEmptyAssistantChatString(entities, "clusterName")}, Status: aiopsmodel.AssistantChatPlanPending},
			{Step: 2, ToolName: "k8s_search_resources", Purpose: "在授权 Namespace 中精确搜索同名 Deployment", Arguments: map[string]interface{}{"kind": "deployment", "keyword": workloadName, "exactName": true}, Status: aiopsmodel.AssistantChatPlanPending},
			{Step: 3, ToolName: "k8s_get_workload_status", Purpose: "读取当前副本、Pod、镜像、条件和事件", Arguments: map[string]interface{}{"workloadName": workloadName}, Status: aiopsmodel.AssistantChatPlanPending},
		}
	}
	if assistantChatBool(entities["workloadHistory"]) {
		workloadName := firstNonEmptyAssistantChatString(entities, "workloadName")
		return []aiopsmodel.AssistantChatToolPlanStep{
			{
				Step: 1, ToolName: "k8s_list_clusters", Purpose: "读取当前用户授权的全部 K8s 集群",
				Arguments: map[string]interface{}{
					"clusterId": assistantChatFirstEntityUInt(entities, "clusterId", "k8sClusterId"),
					"keyword":   firstNonEmptyAssistantChatString(entities, "clusterName"),
				},
				Status: aiopsmodel.AssistantChatPlanPending,
			},
			{
				Step: 2, ToolName: "k8s_search_resources", Purpose: "在授权 Namespace 中精确搜索同名 Deployment",
				Arguments: map[string]interface{}{"kind": "deployment", "keyword": workloadName, "exactName": true},
				Status:    aiopsmodel.AssistantChatPlanPending,
			},
			{
				Step: 3, ToolName: "k8s_get_workload_history", Purpose: "读取 Deployment 历史 revision、镜像、创建时间和副本状态",
				Arguments: map[string]interface{}{"workloadName": workloadName},
				Status:    aiopsmodel.AssistantChatPlanPending,
			},
		}
	}
	return []aiopsmodel.AssistantChatToolPlanStep{
		{
			Step:     1,
			ToolName: "k8s_list_clusters",
			Purpose:  "读取当前用户授权的 K8s 集群范围，不修改集群资源",
			Arguments: map[string]interface{}{
				"clusterId": assistantChatFirstEntityUInt(entities, "clusterId", "k8sClusterId", "pageResourceId"),
				"keyword":   firstNonEmptyAssistantChatString(entities, "clusterName"),
			},
			Status: aiopsmodel.AssistantChatPlanPending,
		},
	}
}

func buildAssistantChatK8sRolloutRestartToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	workloadName := firstNonEmptyAssistantChatString(entities, "workloadName")
	return []aiopsmodel.AssistantChatToolPlanStep{
		{
			Step:     1,
			ToolName: "k8s_list_clusters",
			Purpose:  "读取当前用户授权的全部 K8s 集群",
			Arguments: map[string]interface{}{
				"clusterId": assistantChatFirstEntityUInt(entities, "clusterId", "k8sClusterId"),
				"keyword":   firstNonEmptyAssistantChatString(entities, "clusterName"),
			},
			Status: aiopsmodel.AssistantChatPlanPending,
		},
		{
			Step:      2,
			ToolName:  "k8s_search_resources",
			Purpose:   "在授权 Namespace 中精确搜索同名工作负载",
			Arguments: map[string]interface{}{"kind": "workload", "keyword": workloadName, "exactName": true},
			Status:    aiopsmodel.AssistantChatPlanPending,
		},
		{
			Step:      3,
			ToolName:  "k8s_request_rollout_restart",
			Purpose:   "唯一命中后检查变更权限并生成滚动重启确认预览，本轮不执行变更",
			Arguments: map[string]interface{}{"workloadName": workloadName},
			Status:    aiopsmodel.AssistantChatPlanPending,
		},
	}
}

func buildAssistantChatK8sScaleWorkloadToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	workloadName := firstNonEmptyAssistantChatString(entities, "workloadName")
	plan := []aiopsmodel.AssistantChatToolPlanStep{
		{
			Step:     1,
			ToolName: "k8s_list_clusters",
			Purpose:  "读取当前用户授权的全部 K8s 集群",
			Arguments: map[string]interface{}{
				"clusterId": assistantChatFirstEntityUInt(entities, "clusterId", "k8sClusterId"),
				"keyword":   firstNonEmptyAssistantChatString(entities, "clusterName"),
			},
			Status: aiopsmodel.AssistantChatPlanPending,
		},
		{
			Step:      2,
			ToolName:  "k8s_search_resources",
			Purpose:   "在授权 Namespace 中精确搜索同名 Deployment",
			Arguments: map[string]interface{}{"kind": "workload", "keyword": workloadName, "exactName": true},
			Status:    aiopsmodel.AssistantChatPlanPending,
		},
		{
			Step:      3,
			ToolName:  "k8s_request_scale_workload",
			Purpose:   "唯一命中后检查变更权限并生成扩缩容确认预览，本轮不执行变更",
			Arguments: map[string]interface{}{"workloadName": workloadName, "replicas": assistantChatNumber(entities["replicas"]), "replicaDelta": assistantChatNumber(entities["replicaDelta"])},
			Status:    aiopsmodel.AssistantChatPlanPending,
		},
	}
	if int(assistantChatNumber(entities["replicaDelta"])) != 0 {
		statusStep := aiopsmodel.AssistantChatToolPlanStep{
			Step:      3,
			ToolName:  "k8s_get_workload_status",
			Purpose:   "读取唯一命中 Deployment 的实时期望副本数并计算相对伸缩目标",
			Arguments: map[string]interface{}{"workloadName": workloadName},
			Status:    aiopsmodel.AssistantChatPlanPending,
		}
		plan = append(plan, aiopsmodel.AssistantChatToolPlanStep{})
		copy(plan[3:], plan[2:])
		plan[2] = statusStep
		for i := range plan {
			plan[i].Step = i + 1
		}
	}
	return plan
}

func buildAssistantChatK8sRollbackWorkloadToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	workloadName := firstNonEmptyAssistantChatString(entities, "workloadName")
	return []aiopsmodel.AssistantChatToolPlanStep{
		{
			Step: 1, ToolName: "k8s_list_clusters", Purpose: "读取当前用户授权的全部 K8s 集群",
			Arguments: map[string]interface{}{
				"clusterId": assistantChatFirstEntityUInt(entities, "clusterId", "k8sClusterId"),
				"keyword":   firstNonEmptyAssistantChatString(entities, "clusterName"),
			},
			Status: aiopsmodel.AssistantChatPlanPending,
		},
		{
			Step: 2, ToolName: "k8s_search_resources", Purpose: "在授权 Namespace 中精确搜索同名 Deployment",
			Arguments: map[string]interface{}{"kind": "deployment", "keyword": workloadName, "exactName": true},
			Status:    aiopsmodel.AssistantChatPlanPending,
		},
		{
			Step: 3, ToolName: "k8s_request_rollback_workload", Purpose: "读取历史版本，校验并固化目标 revision，生成回滚确认预览，本轮不执行变更",
			Arguments: map[string]interface{}{
				"workloadName": workloadName,
				"revision":     assistantChatNumber(entities["revision"]),
				"imageVersion": firstNonEmptyAssistantChatString(entities, "imageVersion"),
			},
			Status: aiopsmodel.AssistantChatPlanPending,
		},
	}
}

func buildAssistantK8sRuntimeToolPlan(question string, entities map[string]interface{}, clusterID uint) []aiopsmodel.AssistantChatToolPlanStep {
	steps := make([]aiopsmodel.AssistantChatToolPlanStep, 0, 6)
	appendStep := func(name, purpose string, args map[string]interface{}) {
		for _, item := range steps {
			if item.ToolName == name {
				return
			}
		}
		steps = append(steps, aiopsmodel.AssistantChatToolPlanStep{ToolName: name, Purpose: purpose, Arguments: args})
	}

	namespace := firstNonEmptyAssistantChatString(entities, "namespace")
	pod := firstNonEmptyAssistantChatString(entities, "pod", "podName")
	node := firstNonEmptyAssistantChatString(entities, "node")
	workloadName := firstNonEmptyAssistantChatString(entities, "workloadName")
	workloadType := normalizeMCPK8sKind(firstNonEmptyAssistantChatString(entities, "workloadType", "kind"))
	serviceName := firstNonEmptyAssistantChatString(entities, "service")
	ingressName := firstNonEmptyAssistantChatString(entities, "ingress")
	container := firstNonEmptyAssistantChatString(entities, "container")
	resourceName := firstNonEmptyAssistantChatString(entities, "resourceName")
	kind := normalizeMCPK8sKind(firstNonEmptyAssistantChatString(entities, "kind"))
	questionLower := strings.ToLower(strings.TrimSpace(question))
	needsDiagnosis := containsAnyAssistantChatKeyword(questionLower, []string{"异常", "故障", "问题", "诊断", "排查", "失败", "错误", "crash", "error", "pending", "oom"})
	needsLogs := containsAnyAssistantChatKeyword(questionLower, []string{"日志", "log", "报错", "错误栈"})

	specific := false
	if node != "" {
		specific = true
		appendStep("k8s_get_node_status", "读取节点状态和资源压力", map[string]interface{}{"clusterId": clusterID, "node": node, "includeMetrics": true})
		appendStep("k8s_query_metrics", "读取节点实时指标", map[string]interface{}{"clusterId": clusterID, "node": node})
	}
	if pod != "" && namespace != "" {
		specific = true
		appendStep("k8s_get_pod_status", "读取 Pod、容器和探针状态", map[string]interface{}{"clusterId": clusterID, "namespace": namespace, "pod": pod})
		appendStep("k8s_query_events", "读取 Pod 关联事件", map[string]interface{}{"clusterId": clusterID, "namespace": namespace, "kind": "Pod", "name": pod, "limit": 100})
		appendStep("k8s_query_metrics", "读取 Pod 实时指标", map[string]interface{}{"clusterId": clusterID, "namespace": namespace, "pod": pod, "container": container})
		if needsLogs {
			appendStep("k8s_query_logs", "读取 Pod 日志", map[string]interface{}{"clusterId": clusterID, "namespace": namespace, "pod": pod, "container": container, "tailLines": 500, "limit": 200})
		}
		if needsDiagnosis {
			appendStep("k8s_diagnose_pod", "聚合 Pod 状态、事件、日志和指标进行诊断", map[string]interface{}{"clusterId": clusterID, "namespace": namespace, "pod": pod, "container": container, "tailLines": 500, "limit": 200, "eventLimit": 100})
		}
	}
	if workloadName != "" && namespace != "" && workloadType != "" {
		specific = true
		appendStep("k8s_get_workload_status", "读取工作负载副本和发布状态", map[string]interface{}{"clusterId": clusterID, "namespace": namespace, "workloadType": workloadType, "workloadName": workloadName})
		appendStep("k8s_query_events", "读取工作负载关联事件", map[string]interface{}{"clusterId": clusterID, "namespace": namespace, "kind": workloadType, "name": workloadName, "limit": 100})
		if needsDiagnosis {
			appendStep("k8s_diagnose_workload", "聚合工作负载异常证据", map[string]interface{}{"clusterId": clusterID, "namespace": namespace, "workloadType": workloadType, "workloadName": workloadName, "container": container, "tailLines": 500, "limit": 200, "eventLimit": 100})
		}
	}
	if serviceName != "" && namespace != "" {
		specific = true
		appendStep("k8s_get_service_endpoints", "检查 Service 与后端 Endpoint", map[string]interface{}{"clusterId": clusterID, "namespace": namespace, "service": serviceName})
	}
	if ingressName != "" && namespace != "" {
		specific = true
		appendStep("k8s_diagnose_ingress", "诊断 Ingress、Service、Endpoint 和后端 Pod 链路", map[string]interface{}{"clusterId": clusterID, "namespace": namespace, "ingress": ingressName, "limit": 200, "eventLimit": 100})
	}
	if resourceName != "" && namespace != "" && kind != "" {
		specific = true
		appendStep("k8s_get_resource", "读取指定 K8s 资源详情", map[string]interface{}{"clusterId": clusterID, "namespace": namespace, "kind": kind, "name": resourceName})
	} else if kind != "" && namespace != "" && firstNonEmptyAssistantChatString(entities, "keyword") != "" {
		specific = true
		appendStep("k8s_search_resources", "搜索授权 Namespace 中的 K8s 资源", map[string]interface{}{"clusterId": clusterID, "namespace": namespace, "kind": kind, "keyword": firstNonEmptyAssistantChatString(entities, "keyword"), "page": 1, "pageSize": 50})
	}
	if !specific {
		appendStep("k8s_check_cluster", "检查集群 API、版本、节点和核心组件", map[string]interface{}{"clusterId": clusterID})
		appendStep("k8s_list_nodes", "检查节点 Ready 和资源压力", map[string]interface{}{"clusterId": clusterID})
		appendStep("k8s_list_namespaces", "读取授权范围内的 Namespace", map[string]interface{}{"clusterId": clusterID})
		if namespace != "" {
			appendStep("k8s_query_events", "读取授权 Namespace 的近期异常事件", map[string]interface{}{"clusterId": clusterID, "namespace": namespace, "limit": 100})
		}
	}
	for i := range steps {
		steps[i].Step = i + 2
		steps[i].Status = aiopsmodel.AssistantChatPlanPending
	}
	return steps
}

func buildAssistantChatKnowledgeToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	return []aiopsmodel.AssistantChatToolPlanStep{
		{
			Step:      1,
			ToolName:  "knowledge_search",
			Purpose:   "检索运维知识库并注入相关文档",
			Arguments: map[string]interface{}{"query": "$user.question", "limit": 5},
			Status:    aiopsmodel.AssistantChatPlanPending,
		},
	}
}

func buildAssistantChatSkillCatalogToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	return []aiopsmodel.AssistantChatToolPlanStep{
		{
			Step:     1,
			ToolName: "skill_catalog",
			Purpose:  "读取 AutoOps 后端 skills 目录中已加载的声明式 Skill 清单",
			Status:   aiopsmodel.AssistantChatPlanPending,
		},
	}
}

func buildAssistantChatMCPCatalogToolPlan(_ map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	return []aiopsmodel.AssistantChatToolPlanStep{
		{
			Step: 1, ToolName: "mcp_catalog", Purpose: "读取 MCP 服务和 Tool 的实时实现、启用与策略状态",
			Status: aiopsmodel.AssistantChatPlanPending,
		},
	}
}

func buildAssistantChatOpsGeneralToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	return []aiopsmodel.AssistantChatToolPlanStep{
		{
			Step:     1,
			ToolName: "llm_ops_general",
			Purpose:  "生成通用运维问答、脚本或排查步骤，不调用生产工具",
			Status:   aiopsmodel.AssistantChatPlanPending,
		},
	}
}

func buildAssistantChatGeneralQAToolPlan(entities map[string]interface{}) []aiopsmodel.AssistantChatToolPlanStep {
	return []aiopsmodel.AssistantChatToolPlanStep{
		{
			Step:     1,
			ToolName: "llm_general_qa",
			Purpose:  "回答通用知识问题，不调用生产工具",
			Status:   aiopsmodel.AssistantChatPlanPending,
		},
	}
}

func buildAssistantChatClarifyQuestions(intent string, entities map[string]interface{}) []aiopsmodel.AssistantChatClarifyQuestion {
	if intent == aiopsmodel.AssistantChatIntentUnknown || intent == aiopsmodel.AssistantChatIntentClarify {
		return nil
	}
	if intent == aiopsmodel.AssistantChatIntentK8sRolloutRestart {
		if firstNonEmptyAssistantChatString(entities, "workloadName") == "" {
			return []aiopsmodel.AssistantChatClarifyQuestion{{
				Field:       "workloadName",
				Question:    "请提供要重启的 K8s 工作负载名称。",
				Suggestions: []string{"例如：zero-ai-data-cli"},
			}}
		}
		return nil
	}
	if intent == aiopsmodel.AssistantChatIntentK8sScaleWorkload {
		if firstNonEmptyAssistantChatString(entities, "workloadName") == "" {
			return []aiopsmodel.AssistantChatClarifyQuestion{{
				Field:       "workloadName",
				Question:    "请提供要扩缩容的 K8s 工作负载名称。",
				Suggestions: []string{"例如：zero-ai-data-cli"},
			}}
		}
		if _, ok := entities["replicas"]; !ok {
			if _, deltaOK := entities["replicaDelta"]; deltaOK {
				return nil
			}
			return []aiopsmodel.AssistantChatClarifyQuestion{{
				Field:    "replicas",
				Question: "请提供目标副本数或增减数量，例如：扩容到 2 个副本，或增加 1 个副本。",
			}}
		}
		return nil
	}
	if intent == aiopsmodel.AssistantChatIntentK8sRollbackWorkload {
		if firstNonEmptyAssistantChatString(entities, "workloadName") == "" {
			return []aiopsmodel.AssistantChatClarifyQuestion{{
				Field:       "workloadName",
				Question:    "请提供要回滚的 K8s Deployment 名称。",
				Suggestions: []string{"例如：zero-ai-kos"},
			}}
		}
		if int64(assistantChatNumber(entities["revision"])) <= 0 && firstNonEmptyAssistantChatString(entities, "imageVersion") == "" {
			return []aiopsmodel.AssistantChatClarifyQuestion{{
				Field:       "rollbackVersion",
				Question:    "请提供目标版本号或镜像版本。",
				Suggestions: []string{"例如：版本号 14", "例如：镜像版本 v2026-0805-165529"},
			}}
		}
		return nil
	}
	if intent == aiopsmodel.AssistantChatIntentK8sOverview && assistantChatBool(entities["workloadHistory"]) && firstNonEmptyAssistantChatString(entities, "workloadName") == "" {
		return []aiopsmodel.AssistantChatClarifyQuestion{{
			Field:       "workloadName",
			Question:    "请提供要查看历史版本的 K8s Deployment 名称。",
			Suggestions: []string{"例如：zero-ai-data-cli"},
		}}
	}
	for _, definition := range assistantChatIntentRegistry() {
		if definition.Name != intent || len(definition.RequiredOneOf) == 0 {
			continue
		}
		for _, key := range definition.RequiredOneOf {
			if !assistantChatValueEmpty(entities[key]) {
				return nil
			}
		}
		switch intent {
		case aiopsmodel.AssistantChatIntentHostPressure, aiopsmodel.AssistantChatIntentHostLogCleanup:
			return []aiopsmodel.AssistantChatClarifyQuestion{
				{
					Field:    "host",
					Question: "你想检查哪台主机？请提供 IP、主机名，或者在主机详情页发起对话。",
					Suggestions: []string{
						"172.16.7.138",
						"主机名 lyj-zhangfan",
						"在主机详情页点击 AI 助手",
					},
				},
			}
		}
	}
	return nil
}

func buildAssistantChatClarifyAnswer(result *aiopsmodel.AssistantChatResult) string {
	if result == nil || len(result.ClarifyQuestions) == 0 {
		return "我需要更多信息才能继续。"
	}
	return result.ClarifyQuestions[0].Question
}

func buildAssistantChatClarifyActions(result *aiopsmodel.AssistantChatResult) []string {
	if result == nil || len(result.ClarifyQuestions) == 0 {
		return []string{"补充目标对象后重新提问。"}
	}
	actions := make([]string, 0, len(result.ClarifyQuestions[0].Suggestions)+1)
	actions = append(actions, "补充 "+result.ClarifyQuestions[0].Field+" 信息后我会继续执行。")
	actions = append(actions, result.ClarifyQuestions[0].Suggestions...)
	return actions
}

func shouldAssistantChatUseLLM(result *aiopsmodel.AssistantChatResult) bool {
	if result == nil || result.UsedLLM {
		return false
	}
	if assistantChatBool(result.Entities["pathInspection"]) || assistantChatBool(result.Entities["diskUsageAnalysis"]) || result.Intent == aiopsmodel.AssistantChatIntentHostLogCleanup {
		return false
	}
	if assistantChatBool(result.Entities["workloadHistory"]) || assistantChatBool(result.Entities["workloadStatusQuery"]) || assistantChatBool(result.Entities["replicaCountQuery"]) {
		return false
	}
	if result.Intent == aiopsmodel.AssistantChatIntentUnsupportedFileOp {
		return false
	}
	return result.Intent == aiopsmodel.AssistantChatIntentHostPressure ||
		result.Intent == aiopsmodel.AssistantChatIntentHostServices ||
		result.Intent == aiopsmodel.AssistantChatIntentPlatformOverview ||
		result.Intent == aiopsmodel.AssistantChatIntentAlertOverview ||
		result.Intent == aiopsmodel.AssistantChatIntentDatabaseOverview ||
		result.Intent == aiopsmodel.AssistantChatIntentK8sOverview ||
		result.Intent == aiopsmodel.AssistantChatIntentKnowledgeSearch ||
		result.Intent == aiopsmodel.AssistantChatIntentOpsGeneral ||
		result.Intent == aiopsmodel.AssistantChatIntentGeneralQA
}

func applyAssistantChatLLMProvider(result *aiopsmodel.AssistantChatResult, provider *aiopsmodel.LLMProvider) {
	if result == nil || provider == nil {
		return
	}
	result.ProviderID = provider.ID
	result.ProviderName = provider.Name
	result.ProviderType = provider.ProviderType
	result.ModelCode = provider.DefaultModel
}

func buildAssistantChatLLMPrompt(result *aiopsmodel.AssistantChatResult) string {
	payload := struct {
		Intent         string                                  `json:"intent"`
		Question       string                                  `json:"question"`
		FallbackAnswer string                                  `json:"fallbackAnswer"`
		Entities       map[string]interface{}                  `json:"entities"`
		Memory         *aiopsmodel.AssistantChatMemoryContext  `json:"memory,omitempty"`
		AssistantPlan  *aiopsmodel.AssistantInvestigationPlan  `json:"assistantPlan,omitempty"`
		Reflection     *aiopsmodel.AssistantReflection         `json:"reflection,omitempty"`
		ToolPlan       []aiopsmodel.AssistantChatToolPlanStep  `json:"toolPlan"`
		Timeline       []aiopsmodel.AssistantChatTimelineEvent `json:"timeline"`
		DiagnosisCard  *aiopsmodel.AssistantChatDiagnosisCard  `json:"diagnosisCard,omitempty"`
		UsedTools      []aiopsmodel.AssistantChatUsedTool      `json:"usedTools"`
		Evidence       []aiopsmodel.AgentDiagnosisEvidence     `json:"evidence"`
		MissingInfo    []string                                `json:"missingInfo"`
	}{
		Intent:         result.Intent,
		Question:       result.Question,
		FallbackAnswer: result.Answer,
		Entities:       result.Entities,
		Memory:         result.Memory,
		AssistantPlan:  result.AssistantPlan,
		Reflection:     result.Reflection,
		ToolPlan:       result.ToolPlan,
		Timeline:       result.Timeline,
		DiagnosisCard:  result.DiagnosisCard,
		UsedTools:      result.UsedTools,
		Evidence:       result.Evidence,
		MissingInfo:    result.MissingInfo,
	}
	data, _ := json.Marshal(payload)
	if result.Intent == aiopsmodel.AssistantChatIntentOpsGeneral {
		return "你是 AutoOps 平台内的通用运维助手。请直接回答用户的通用运维问题，可以给出 Linux/Shell 只读巡检脚本、排查步骤或 Runbook 建议。要求：\n1. 中文说明清楚用途和使用方式。\n2. 脚本默认只读，不包含 rm、mkfs、reboot、shutdown、kill、systemctl restart、写配置等危险变更动作。\n3. 如确实需要变更，只能说明需要走审批或工单，不要给出直接执行生产变更的命令。\n4. 不要编造 AutoOps 平台里没有提供的实时资产数据。\n\nJSON:\n" + string(data)
	}
	if result.Intent == aiopsmodel.AssistantChatIntentGeneralQA {
		return "你是 AutoOps 平台内的通用问答助手。用户问的是通用知识问题，不需要调用生产工具，也不要要求用户提供主机、告警或资产对象。请用中文直接回答；如果问题涉及实时平台数据或生产环境数据，再提示用户补充对象并使用平台工具。\n\nJSON:\n" + string(data)
	}
	return "你是 AutoOps 平台的中心通用 AI 运维助手，不是页面小助手。请基于 JSON 中的 evidence、timeline、diagnosisCard 和 fallbackAnswer 回答，不要编造证据中没有的平台资产、告警、指标、数据库或 K8s 状态。回答结构：先给结论，再列关键证据、风险判断、缺失数据、下一步建议。证据不足时明确说明缺什么数据。禁止输出危险变更命令；如果涉及重启、删除、扩缩容、执行 SQL 或修改配置，只能建议走审批或工单。\n\nJSON:\n" + string(data)
}

func extractAssistantChatHostTarget(req *aiopsmodel.AssistantChatRequest, entities map[string]interface{}) string {
	if req != nil {
		if match := assistantChatQuestionIP(req.Question); match != "" {
			return match
		}
	}
	for _, key := range []string{"ip", "hostName", "hostname", "instance", "keyword"} {
		if value := firstNonEmptyAssistantChatString(entities, key); value != "" {
			return value
		}
	}
	return ""
}

func assistantChatQuestionIP(question string) string {
	return assistantChatIPPattern.FindString(question)
}

func assistantChatQuestionHostName(question string) string {
	question = strings.TrimSpace(question)
	if question == "" || !containsAnyAssistantChatKeyword(strings.ToLower(question), []string{
		"主机", "机器", "服务器", "目录", "磁盘", "文件", "host", "server", "disk", "directory",
	}) {
		return ""
	}
	pathMatches := assistantChatLinuxPathPattern.FindAllStringIndex(question, -1)
	for _, match := range assistantChatHostNamePattern.FindAllStringIndex(question, -1) {
		if assistantChatMatchInsideAnyRange(match, pathMatches) {
			continue
		}
		candidate := question[match[0]:match[1]]
		if strings.IndexFunc(candidate, func(r rune) bool {
			return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		}) >= 0 {
			return candidate
		}
	}
	return ""
}

func assistantChatQuestionLinuxPath(question string) string {
	for _, match := range assistantChatLinuxPathPattern.FindAllStringIndex(question, -1) {
		prefix := strings.ToLower(question[:match[0]])
		if strings.HasSuffix(prefix, "http:/") || strings.HasSuffix(prefix, "https:/") {
			continue
		}
		return pathpkg.Clean(question[match[0]:match[1]])
	}
	return ""
}

func assistantChatMatchInsideAnyRange(match []int, ranges [][]int) bool {
	if len(match) != 2 {
		return false
	}
	for _, candidateRange := range ranges {
		if len(candidateRange) == 2 && match[0] >= candidateRange[0] && match[1] <= candidateRange[1] {
			return true
		}
	}
	return false
}

func assistantChatQuestionDatabaseName(question string) string {
	question = strings.TrimSpace(question)
	if question == "" {
		return ""
	}
	if match := assistantChatIPDatabaseNamePattern.FindString(question); match != "" {
		return strings.Trim(match, " ，。！？、；;,.!?()（）[]【】")
	}
	for _, match := range assistantChatDatabaseNamePattern.FindAllString(question, -1) {
		match = strings.Trim(match, " ，。！？、；;,.!?()（）[]【】")
		lower := strings.ToLower(match)
		if lower == "" {
			continue
		}
		switch lower {
		case "mysql", "redis", "postgres", "postgresql", "mongodb", "mongo", "oracle", "sqlserver", "db":
			continue
		default:
			return match
		}
	}
	return ""
}

func assistantChatLooksLikeIP(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && assistantChatIPPattern.FindString(value) == value
}

func parseAssistantChatMinutes(question string) int {
	question = strings.ToLower(question)
	if strings.Contains(question, "半小时") || strings.Contains(question, "半个小时") {
		return 30
	}
	minutePattern := regexp.MustCompile(`(\d+)\s*(分钟|min|m)`)
	if match := minutePattern.FindStringSubmatch(question); len(match) == 3 {
		if value := int(assistantChatNumber(match[1])); value > 0 {
			return value
		}
	}
	hourPattern := regexp.MustCompile(`(\d+)\s*(小时|hour|h)`)
	if match := hourPattern.FindStringSubmatch(question); len(match) == 3 {
		if value := int(assistantChatNumber(match[1])); value > 0 {
			return value * 60
		}
	}
	return defaultAssistantChatMinutes
}

func buildAssistantChatPathInspectionAnswer(host, inspection map[string]interface{}) string {
	hostName := firstNonEmptyAssistantChatString(host, "hostName", "name", "privateIp", "sshIp", "publicIp")
	target := firstNonEmptyAssistantChatString(inspection, "path")
	pathType := firstNonEmptyAssistantChatString(inspection, "pathType")
	var b strings.Builder
	b.WriteString(fmt.Sprintf("主机 %s 路径检查结果：`%s`。\n", hostName, target))

	if pathType == "directory" {
		entries := assistantChatSlice(inspection["entries"])
		b.WriteString("\n| 类型 | 名称 | 大小 | 权限 | 修改时间 |\n| --- | --- | ---: | --- | --- |\n")
		for _, raw := range entries {
			item := assistantChatMap(raw)
			b.WriteString(fmt.Sprintf("| %s | `%s` | %s | `%s` | %s |\n",
				firstNonEmptyAssistantChatString(item, "type"),
				strings.ReplaceAll(firstNonEmptyAssistantChatString(item, "name"), "|", "\\|"),
				firstNonEmptyAssistantChatString(item, "size"),
				firstNonEmptyAssistantChatString(item, "permissions"),
				firstNonEmptyAssistantChatString(item, "modifiedAt"),
			))
		}
		if len(entries) == 0 {
			b.WriteString("| - | 目录为空 | - | - | - |\n")
		}
		if assistantChatBool(inspection["truncated"]) {
			b.WriteString(fmt.Sprintf("\n结果已限制为前 %d 项，请指定子目录继续查看。", int(assistantChatNumber(inspection["maxEntries"]))))
		}
		return strings.TrimSpace(b.String())
	}

	if assistantChatBool(inspection["binary"]) {
		b.WriteString("\n该路径是二进制文件，为避免不可读内容和响应膨胀，未返回文件内容。")
		return b.String()
	}
	b.WriteString(fmt.Sprintf("\n文件大小：%s | 权限：`%s` | 修改时间：%s\n",
		firstNonEmptyAssistantChatString(inspection, "size"),
		firstNonEmptyAssistantChatString(inspection, "permissions"),
		firstNonEmptyAssistantChatString(inspection, "modifiedAt"),
	))
	b.WriteString(fmt.Sprintf("\n最后 %d 行：\n\n~~~~text\n", int(assistantChatNumber(inspection["tailLines"]))))
	content, _ := inspection["content"].(string)
	b.WriteString(strings.ReplaceAll(content, "~~~~", "~~~ ~"))
	b.WriteString("\n~~~~")
	if assistantChatBool(inspection["contentTruncated"]) {
		b.WriteString(fmt.Sprintf("\n\n内容超过 %d 字节，当前仅返回末尾部分。", int(assistantChatNumber(inspection["contentByteLimit"]))))
	}
	return strings.TrimSpace(b.String())
}

func buildAssistantChatLogCleanupAnswer(host, preview map[string]interface{}) string {
	hostName := firstNonEmptyAssistantChatString(host, "hostName", "name", "privateIp", "sshIp", "publicIp")
	files := assistantChatSlice(preview["files"])
	if !assistantChatBool(preview["requiresConfirmation"]) {
		return firstNonEmptyAssistantChatString(preview, "message")
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("已生成主机 %s 的日志清空预览。本次尚未执行，日志文件会保留。\n", hostName))
	b.WriteString("\n| 日志文件 | 当前大小 | 修改时间 |\n| --- | ---: | --- |\n")
	for _, raw := range files {
		item := assistantChatMap(raw)
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s |\n",
			strings.ReplaceAll(firstNonEmptyAssistantChatString(item, "path"), "|", "\\|"),
			firstNonEmptyAssistantChatString(item, "size"),
			firstNonEmptyAssistantChatString(item, "modifiedAt"),
		))
	}
	b.WriteString("\n清空命令：\n\n```sh\n")
	b.WriteString(firstNonEmptyAssistantChatString(preview, "command"))
	b.WriteString("\n```\n")
	b.WriteString("\n执行前需要二次确认，并重新校验当前登录用户的系统管理员身份。")
	if expiresAt := firstNonEmptyAssistantChatString(preview, "expiresAt"); expiresAt != "" {
		b.WriteString("\n\n确认凭证过期时间：`" + expiresAt + "`。")
	}
	return strings.TrimSpace(b.String())
}

func buildAssistantChatDiskUsageAnswer(host, diskUsage map[string]interface{}) string {
	hostName := firstNonEmptyAssistantChatString(host, "hostName", "name", "privateIp", "sshIp", "publicIp")
	root := firstNonEmptyAssistantChatString(diskUsage, "path")
	if root == "" {
		root = "/"
	}
	directories := assistantChatSlice(diskUsage["directories"])
	files := assistantChatSlice(diskUsage["files"])

	var b strings.Builder
	b.WriteString(fmt.Sprintf("已完成主机 %s 的磁盘占用分析，扫描路径：`%s`。\n", hostName, root))
	b.WriteString("\n目录占用 Top：\n\n| 排名 | 目录 | 大小 |\n| ---: | --- | ---: |\n")
	directoryRank := 0
	for _, raw := range directories {
		item := assistantChatMap(raw)
		path := firstNonEmptyAssistantChatString(item, "path")
		if path == "" || path == root {
			continue
		}
		directoryRank++
		b.WriteString(fmt.Sprintf("| %d | `%s` | %s |\n", directoryRank, strings.ReplaceAll(path, "|", "\\|"), firstNonEmptyAssistantChatString(item, "size")))
		if directoryRank >= 5 {
			break
		}
	}
	if directoryRank == 0 {
		b.WriteString("| - | 未解析到子目录占用数据 | - |\n")
	}

	b.WriteString("\n大文件 Top（默认仅列出超过 100 MiB 的文件）：\n\n| 排名 | 文件 | 大小 |\n| ---: | --- | ---: |\n")
	fileRank := 0
	for _, raw := range files {
		item := assistantChatMap(raw)
		path := firstNonEmptyAssistantChatString(item, "path")
		if path == "" {
			continue
		}
		fileRank++
		b.WriteString(fmt.Sprintf("| %d | `%s` | %s |\n", fileRank, strings.ReplaceAll(path, "|", "\\|"), firstNonEmptyAssistantChatString(item, "size")))
		if fileRank >= 5 {
			break
		}
	}
	if fileRank == 0 {
		b.WriteString("| - | 未发现超过阈值的大文件 | - |\n")
	}
	return strings.TrimSpace(b.String())
}

func buildAssistantChatHostPressureAnswer(host, agent, metrics map[string]interface{}, minutes int) string {
	hostName := firstNonEmptyAssistantChatString(host, "hostName", "name")
	hostDisplay := hostName
	if hostDisplay == "" {
		hostDisplay = firstNonEmptyAssistantChatString(host, "privateIp", "sshIp", "publicIp")
	}
	groupName := firstNonEmptyAssistantChatString(host, "groupName")
	osName := firstNonEmptyAssistantChatString(host, "os")
	ip := firstNonEmptyAssistantChatString(host, "privateIp", "sshIp", "publicIp")

	summary := assistantChatMap(metrics["summary"])
	diagnosis := assistantChatMap(metrics["diagnosis"])
	topProcesses := assistantChatMap(metrics["topProcesses"])

	statusText := "未知"
	if diagnosisStatus := firstNonEmptyAssistantChatString(diagnosis, "status"); diagnosisStatus == "healthy" {
		statusText = "健康，无明显压力"
	} else if diagnosisStatus == "warning" {
		statusText = "存在压力或风险，需要关注"
	} else if firstNonEmptyAssistantChatString(metrics, "summaryError") != "" {
		statusText = "监控摘要不可用"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("检查结果：%s。\n", statusText))
	b.WriteString(fmt.Sprintf("主机：%s", hostDisplay))
	if ip != "" {
		b.WriteString(fmt.Sprintf(" | IP：%s", ip))
	}
	if groupName != "" {
		b.WriteString(fmt.Sprintf(" | 分组：%s", groupName))
	}
	if osName != "" {
		b.WriteString(fmt.Sprintf(" | 系统：%s", osName))
	}
	b.WriteString("\n")

	if len(agent) > 0 {
		agentState := "未安装或离线"
		if assistantChatBool(agent["online"]) {
			agentState = "运行中"
		} else if firstNonEmptyAssistantChatString(agent, "statusText") != "" {
			agentState = firstNonEmptyAssistantChatString(agent, "statusText")
		}
		b.WriteString("Agent：" + agentState)
		if heartbeat := firstNonEmptyAssistantChatString(agent, "lastHeartbeat"); heartbeat != "" {
			b.WriteString(" | 最近心跳：" + heartbeat)
		}
		b.WriteString("\n")
	}

	if len(summary) > 0 {
		b.WriteString(fmt.Sprintf("最近 %d 分钟核心指标：CPU %.2f%%，内存 %.2f%%，磁盘 %.2f%%，5分钟负载 %.2f。\n",
			minutes,
			assistantChatNumber(summary["cpuUsage"]),
			assistantChatNumber(summary["memoryUsage"]),
			assistantChatNumber(summary["diskUsage"]),
			assistantChatNumber(summary["load5"]),
		))
	} else {
		b.WriteString("监控摘要暂无可用数据。\n")
	}

	if top := formatAssistantChatTopProcesses(topProcesses, "topCPU", "cpuPercent"); top != "" {
		b.WriteString("CPU Top 进程：" + top + "\n")
	}
	if top := formatAssistantChatTopProcesses(topProcesses, "topMemory", "memPercent"); top != "" {
		b.WriteString("内存 Top 进程：" + top + "\n")
	}

	if notes := assistantChatStringSlice(diagnosis["notes"]); len(notes) > 0 {
		b.WriteString("依据：" + strings.Join(notes, "；") + "\n")
	}
	if risks := assistantChatStringSlice(diagnosis["risks"]); len(risks) > 0 {
		b.WriteString("风险：" + strings.Join(risks, "；") + "\n")
	}
	return strings.TrimSpace(b.String())
}

func buildAssistantChatHostServicesAnswer(host, output map[string]interface{}) string {
	hostData := assistantChatMap(output["host"])
	if len(hostData) == 0 {
		hostData = host
	}
	hostDisplay := assistantChatHostDisplay(hostData)
	ip := firstNonEmptyAssistantChatString(hostData, "privateIp", "sshIp", "publicIp")
	groupName := firstNonEmptyAssistantChatString(hostData, "groupName")
	osName := firstNonEmptyAssistantChatString(hostData, "os")
	services := assistantChatSlice(output["services"])
	topProcesses := assistantChatMap(output["topProcesses"])

	var b strings.Builder
	b.WriteString("基于平台采集到的 Top 进程数据，初步判断这台主机上的服务如下。\n")
	b.WriteString("主机：" + hostDisplay)
	if ip != "" {
		b.WriteString(" | IP：" + ip)
	}
	if groupName != "" {
		b.WriteString(" | 分组：" + groupName)
	}
	if osName != "" {
		b.WriteString(" | 系统：" + osName)
	}
	b.WriteString("\n")

	if len(services) == 0 {
		b.WriteString("当前 Top 进程数据没有推断出明确业务服务，可能是采集数据为空、服务负载很低，或服务没有进入 Top 进程。")
		if top := formatAssistantChatTopProcesses(topProcesses, "topCPU", "cpuPercent"); top != "" {
			b.WriteString("\nCPU Top 进程：" + top)
		}
		if top := formatAssistantChatTopProcesses(topProcesses, "topMemory", "memPercent"); top != "" {
			b.WriteString("\n内存 Top 进程：" + top)
		}
		return strings.TrimSpace(b.String())
	}

	b.WriteString("疑似服务/进程：\n")
	limit := len(services)
	if limit > 8 {
		limit = 8
	}
	for i := 0; i < limit; i++ {
		item := assistantChatMap(services[i])
		name := firstNonEmptyAssistantChatString(item, "name")
		if name == "" {
			name = "unknown"
		}
		b.WriteString(fmt.Sprintf("%d. %s", i+1, name))
		if cpu := assistantChatNumber(item["cpuPercent"]); cpu > 0 {
			b.WriteString(fmt.Sprintf(" | CPU %.2f%%", cpu))
		}
		if mem := assistantChatNumber(item["memPercent"]); mem > 0 {
			b.WriteString(fmt.Sprintf(" | 内存 %.2f%%", mem))
		}
		if pids := assistantChatUintSlice(item["pids"]); len(pids) > 0 {
			b.WriteString(fmt.Sprintf(" | PID %s", formatAssistantChatUintSlice(pids)))
		}
		b.WriteString("\n")
	}
	b.WriteString("说明：这是基于进程指标的只读推断，不等同于 CMDB 中已登记的业务服务清单。若要精确确认服务用途，建议结合端口、配置文件、应用发布记录或服务注册信息复核。")
	return strings.TrimSpace(b.String())
}

func buildAssistantHostAgentUnavailableAnswer(host, agent map[string]interface{}, scene string) string {
	resource := assistantChatHostDisplay(host)
	statusText := firstNonEmptyAssistantChatString(agent, "statusText", "status")
	if statusText == "" {
		statusText = "未安装或未在线"
	}
	return fmt.Sprintf("已在 CMDB 中定位到主机 %s，但当前 Agent 状态为「%s」，没有可用的采集数据，无法完成%s。", resource, statusText, scene)
}

func assistantChatAgentUnavailable(agent map[string]interface{}) bool {
	if len(agent) == 0 {
		return true
	}
	if installed, ok := agent["installed"]; ok && !assistantChatBool(installed) {
		return true
	}
	if online, ok := agent["online"]; ok && !assistantChatBool(online) {
		return true
	}
	return false
}

func buildAssistantChatHostInventoryAnswer(output map[string]interface{}) string {
	total := int(assistantChatNumber(output["total"]))
	authSuccess := int(assistantChatNumber(output["authSuccess"]))
	authFailed := int(assistantChatNumber(output["authFailed"]))
	unauthenticated := int(assistantChatNumber(output["unauthenticated"]))
	return fmt.Sprintf("主机资产概览：平台当前登记 %d 台主机，认证成功 %d 台，认证失败 %d 台，未认证 %d 台。",
		total, authSuccess, authFailed, unauthenticated)
}

func buildAssistantChatPlatformAnswer(context map[string]interface{}, errs []string) string {
	alerts := assistantChatMap(context["alerts"])
	hosts := assistantChatMap(context["hosts"])
	databases := assistantChatMap(context["database"])
	k8s := assistantChatMap(context["k8s"])
	knowledge := assistantChatMap(context["knowledge"])

	alertTotal := int(assistantChatNumber(alerts["total"]))
	hostTotal := int(assistantChatNumber(hosts["total"]))
	dbTotal := int(assistantChatNumber(databases["total"]))
	k8sTotal := int(assistantChatNumber(k8s["total"]))
	k8sNotReady := countAssistantK8sNotReadyClusters(k8s)
	authFailed := int(assistantChatNumber(hosts["authFailed"]))

	var b strings.Builder
	b.WriteString("平台整体概览：")
	if len(context) == 0 {
		b.WriteString("当前没有拿到可用平台证据。")
	} else {
		b.WriteString(fmt.Sprintf("活跃告警 %d 条，主机 %d 台，数据库资产 %d 个，K8s 集群 %d 个。", alertTotal, hostTotal, dbTotal, k8sTotal))
	}
	if alertTotal > 0 || authFailed > 0 || k8sNotReady > 0 {
		b.WriteString("\n需要优先关注：")
		if alertTotal > 0 {
			b.WriteString(fmt.Sprintf("\n1. 当前存在 %d 条活跃告警，建议先按 severity 和影响对象排序处理。", alertTotal))
		}
		if authFailed > 0 {
			b.WriteString(fmt.Sprintf("\n2. 有 %d 台主机认证失败，可能影响资产采集、SSH 运维或自动化执行。", authFailed))
		}
		if k8sNotReady > 0 {
			b.WriteString(fmt.Sprintf("\n3. 有 %d 个 K8s 集群存在节点未全 Ready 的风险。", k8sNotReady))
		}
	} else if len(context) > 0 {
		b.WriteString("\n从当前可读证据看，未发现明显平台级风险信号。")
	}
	if items := assistantChatSlice(knowledge["items"]); len(items) > 0 {
		b.WriteString(fmt.Sprintf("\n知识库补充：命中 %d 篇相关 Runbook 或故障文档。", len(items)))
	}
	if len(errs) > 0 {
		b.WriteString("\n缺失信息：" + strings.Join(errs, "；"))
	}
	return strings.TrimSpace(b.String())
}

func buildAssistantPlatformActions(context map[string]interface{}, errs []string) []string {
	actions := []string{
		"先从活跃告警进入单条诊断，补齐 CMDB、指标、日志和知识库证据。",
		"对主机认证失败、数据库异常状态、K8s 节点未 Ready 的对象建立待处理清单。",
		"涉及重启、删除、扩缩容、执行 SQL 或修改配置的动作，必须进入审批或工单流程。",
	}
	if len(errs) > 0 {
		actions = append(actions, "补齐本次读取失败的数据源后，再重新发起平台总览分析。")
	}
	if _, ok := context["knowledge"]; ok {
		actions = append(actions, "打开知识库命中的 Runbook，核对适用环境后再执行。")
	}
	return actions
}

func assistantChatHostPressureActions(output interface{}) []string {
	metrics := assistantChatMap(output)
	diagnosis := assistantChatMap(metrics["diagnosis"])
	risks := assistantChatStringSlice(diagnosis["risks"])
	if len(risks) > 0 {
		return []string{
			"确认 Top 进程是否符合预期业务负载。",
			"如需处理进程、清理磁盘或重启服务，先走变更审批。",
		}
	}
	if !assistantChatBool(metrics["hasData"]) {
		return []string{
			"检查 Agent 心跳和 Prometheus target 是否正常。",
			"确认主机 IP、hostname、instance 标签是否与监控指标一致。",
		}
	}
	return []string{
		"当前无需处理，保持观察即可。",
		"如果用户仍反馈业务慢，再继续查看业务日志、端口连接和应用指标。",
	}
}

func assistantChatHostServicesActions(output interface{}) []string {
	services := assistantChatSlice(assistantChatMap(output)["services"])
	if len(services) == 0 {
		return []string{
			"查看主机端口监控或完整监控详情，确认是否存在监听服务。",
			"如果该主机应有业务应用，检查 Agent 进程指标采集是否完整。",
		}
	}
	return []string{
		"结合端口监控确认这些进程对应的监听端口。",
		"结合发布系统或 CMDB 服务关系确认哪些是业务服务，哪些是平台基础组件。",
	}
}

func buildAssistantChatAlertAnswer(output interface{}) string {
	data := assistantChatMap(output)
	total := int(assistantChatNumber(data["total"]))
	items := assistantChatSlice(data["items"])

	var b strings.Builder
	b.WriteString(fmt.Sprintf("当前活跃告警共 %d 条。", total))
	if len(items) == 0 {
		b.WriteString("没有返回具体告警列表。")
		return b.String()
	}
	b.WriteString("\n重点告警：")
	limit := len(items)
	if limit > 5 {
		limit = 5
	}
	for i := 0; i < limit; i++ {
		item := assistantChatMap(items[i])
		name := firstNonEmptyAssistantChatString(item, "alertName", "alertname", "name", "summary")
		severity := firstNonEmptyAssistantChatString(item, "severity", "level")
		instance := firstNonEmptyAssistantChatString(item, "instance", "host", "hostName")
		if name == "" {
			name = fmt.Sprintf("告警 #%d", i+1)
		}
		line := fmt.Sprintf("\n%d. %s", i+1, name)
		if severity != "" {
			line += " | 级别：" + severity
		}
		if instance != "" {
			line += " | 对象：" + instance
		}
		b.WriteString(line)
	}
	return b.String()
}

func buildAssistantHostInventoryContext(c *gin.Context) (map[string]interface{}, error) {
	db := common.GetDB()
	if db == nil {
		return nil, errors.New("database is not initialized")
	}
	authorizedIDs := []uint(nil)
	isSystemAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return nil, err
	}
	if !isSystemAdmin {
		admin, adminErr := jwt.GetAdmin(c)
		if adminErr != nil || admin == nil || admin.ID == 0 {
			return nil, ErrMCPUnauthorized
		}
		authDao := cmdbdao.NewBastionHostAuthorizationDao()
		authorizedIDs, err = authDao.GetUserAuthorizedHostIds(admin.ID)
		if err != nil {
			return nil, err
		}
	}
	hostQuery := func() *gorm.DB {
		query := db.Model(&cmdbmodel.CmdbHost{})
		if !isSystemAdmin {
			query = query.Where("id IN ?", authorizedIDs)
		}
		return query
	}
	var total int64
	if err := hostQuery().Count(&total).Error; err != nil {
		return nil, err
	}
	var linuxCount int64
	_ = hostQuery().Where("host_type = ?", cmdbmodel.HostTypeLinux).Count(&linuxCount).Error
	var windowsCount int64
	_ = hostQuery().Where("host_type = ?", cmdbmodel.HostTypeWindows).Count(&windowsCount).Error
	var authSuccess int64
	_ = hostQuery().Where("status = ?", 1).Count(&authSuccess).Error
	var authFailed int64
	_ = hostQuery().Where("status = ?", 3).Count(&authFailed).Error
	var unauthenticated int64
	_ = hostQuery().Where("status = ?", 2).Count(&unauthenticated).Error

	var hosts []cmdbmodel.CmdbHost
	if err := hostQuery().Order("update_time DESC, id DESC").Limit(10).Find(&hosts).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]interface{}, 0, len(hosts))
	for _, host := range hosts {
		items = append(items, map[string]interface{}{
			"id":        host.ID,
			"hostName":  host.HostName,
			"name":      host.Name,
			"privateIp": host.PrivateIP,
			"sshIp":     host.SSHIP,
			"publicIp":  host.PublicIP,
			"hostType":  assistantChatHostTypeName(host.HostType),
			"status":    assistantChatHostStatusName(host.Status),
			"os":        host.OS,
			"vendor":    host.Vendor,
			"region":    host.Region,
		})
	}
	return map[string]interface{}{
		"total":           total,
		"linuxCount":      linuxCount,
		"windowsCount":    windowsCount,
		"authSuccess":     authSuccess,
		"authFailed":      authFailed,
		"unauthenticated": unauthenticated,
		"recentInventory": items,
	}, nil
}

func buildAssistantDatabaseContext(c *gin.Context, entities map[string]interface{}) (map[string]interface{}, error) {
	db := common.GetDB()
	if db == nil {
		return nil, errors.New("database is not initialized")
	}
	assetID := assistantChatFirstEntityUInt(entities, "assetId", "dbAssetId", "databaseAssetId", "pageResourceId")
	keyword := firstNonEmptyAssistantChatString(entities, "keyword", "databaseName", "dbName", "instance", "ip")
	env := firstNonEmptyAssistantChatString(entities, "env", "environment")
	authorizedIDs := []uint(nil)
	isSystemAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return nil, err
	}
	if !isSystemAdmin {
		admin, adminErr := jwt.GetAdmin(c)
		if adminErr != nil || admin == nil || admin.ID == 0 {
			return nil, ErrMCPUnauthorized
		}
		authDao := cmdbdao.NewBastionAssetAuthorizationDao()
		authorizedIDs, err = authDao.GetUserAuthorizedAssetIDs(admin.ID, mcpDatabaseAssetType)
		if err != nil {
			return nil, err
		}
		if assetID > 0 && !containsMCPUint(authorizedIDs, assetID) {
			return nil, ErrMCPPermissionDenied
		}
	}
	assetQuery := func() *gorm.DB {
		query := db.Model(&cmdbmodel.CmdbSQL{})
		if !isSystemAdmin {
			query = query.Where("id IN ?", authorizedIDs)
		}
		return query
	}

	var total int64
	if err := assetQuery().Count(&total).Error; err != nil {
		return nil, err
	}

	query := assetQuery()
	if assetID > 0 {
		query = query.Where("id = ?", assetID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		if assistantChatQuestionDatabaseName(keyword) == keyword {
			query = query.Where("name = ? OR instance_name = ? OR endpoint = ?", keyword, keyword, keyword)
		} else {
			query = query.Where("name LIKE ? OR instance_name LIKE ? OR host_ip LIKE ? OR endpoint LIKE ? OR business_system LIKE ? OR owner LIKE ? OR dba_owner LIKE ?",
				like, like, like, like, like, like, like)
		}
	}
	if env != "" {
		query = query.Where("environment = ?", env)
	}

	var matched int64
	if err := query.Count(&matched).Error; err != nil {
		return nil, err
	}
	var assets []cmdbmodel.CmdbSQL
	if err := query.Order("updated_at DESC, id DESC").Limit(10).Find(&assets).Error; err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(assets))
	for _, asset := range assets {
		items = append(items, map[string]interface{}{
			"id":                      asset.ID,
			"name":                    asset.Name,
			"instanceName":            asset.InstanceName,
			"type":                    assistantChatDatabaseTypeName(asset.Type),
			"environment":             asset.Environment,
			"status":                  assistantChatDatabaseStatusName(asset.Status),
			"authStatus":              assistantChatDatabaseAuthStatusName(asset.AuthStatus),
			"hostIp":                  asset.HostIP,
			"endpoint":                asset.Endpoint,
			"port":                    asset.Port,
			"businessSystem":          asset.BusinessSystem,
			"owner":                   asset.Owner,
			"dbaOwner":                asset.DBAOwner,
			"slowLogPath":             asset.SlowLogPath,
			"monitorStatus":           asset.MonitorStatus,
			"alertStatus":             asset.AlertStatus,
			"prometheusAccountId":     asset.PrometheusAccountID,
			"prometheusLabelSelector": asset.PrometheusLabelSelector,
		})
	}

	return map[string]interface{}{
		"assetId": assetID,
		"keyword": keyword,
		"env":     env,
		"total":   total,
		"matched": matched,
		"items":   items,
	}, nil
}

func buildAssistantK8sContext(c *gin.Context, entities map[string]interface{}) (map[string]interface{}, error) {
	db := common.GetDB()
	if db == nil {
		return nil, errors.New("database is not initialized")
	}
	clusterID := assistantChatFirstEntityUInt(entities, "clusterId", "k8sClusterId", "pageResourceId")
	keyword := firstNonEmptyAssistantChatString(entities, "clusterName", "keyword")
	page := int(assistantChatNumber(entities["page"]))
	pageSize := int(assistantChatNumber(entities["pageSize"]))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > assistantK8sDiscoveryPage {
		pageSize = 10
	}

	query := db.Model(&k8smodel.KubeCluster{})
	isSystemAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return nil, err
	}
	if !isSystemAdmin {
		admin, adminErr := jwt.GetAdmin(c)
		if adminErr != nil || admin == nil {
			return nil, errors.New("user context is required")
		}
		authDao := cmdbdao.NewBastionAssetAuthorizationDao()
		authorizedIDs, authErr := authDao.GetUserAuthorizedAssetIDs(admin.ID, cmdbmodel.AssetTypeK8SCluster)
		if authErr != nil {
			return nil, authErr
		}
		query = query.Where("id IN ?", authorizedIDs)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	if clusterID > 0 {
		query = query.Where("id = ?", clusterID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR version LIKE ? OR description LIKE ?", like, like, like)
	}

	var matched int64
	if err := query.Count(&matched).Error; err != nil {
		return nil, err
	}
	var clusters []k8smodel.KubeCluster
	if err := query.Order("updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&clusters).Error; err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(clusters))
	for _, cluster := range clusters {
		items = append(items, map[string]interface{}{
			"id":          cluster.ID,
			"name":        cluster.Name,
			"version":     cluster.Version,
			"status":      cluster.GetStatusText(),
			"clusterType": assistantChatK8sClusterTypeName(cluster.ClusterType),
			"nodeCount":   cluster.NodeCount,
			"readyNodes":  cluster.ReadyNodes,
			"masterNodes": cluster.MasterNodes,
			"workerNodes": cluster.WorkerNodes,
			"lastSyncAt":  assistantChatFormatTimePtr(cluster.LastSyncAt),
			"description": cluster.Description,
		})
	}

	return map[string]interface{}{
		"clusterId": clusterID,
		"keyword":   keyword,
		"total":     total,
		"matched":   matched,
		"page":      page,
		"pageSize":  pageSize,
		"items":     items,
	}, nil
}

func buildAssistantChatDatabaseAnswer(output map[string]interface{}) string {
	total := int(assistantChatNumber(output["total"]))
	matched := int(assistantChatNumber(output["matched"]))
	items := assistantChatSlice(output["items"])
	var b strings.Builder
	b.WriteString(fmt.Sprintf("数据库资产概览：平台当前登记 %d 个数据库资产，本次匹配 %d 个。", total, matched))
	if len(items) == 0 {
		b.WriteString("没有找到匹配的数据库实例。")
		return b.String()
	}
	if matched == 1 || len(items) == 1 {
		item := assistantChatMap(items[0])
		name := firstNonEmptyAssistantChatString(item, "instanceName", "name")
		if name == "" {
			name = "该数据库实例"
		}
		status := firstNonEmptyAssistantChatString(item, "status")
		authStatus := firstNonEmptyAssistantChatString(item, "authStatus")
		monitorStatus := firstNonEmptyAssistantChatString(item, "monitorStatus")
		alertStatus := firstNonEmptyAssistantChatString(item, "alertStatus")
		b.WriteString(fmt.Sprintf("\n实例 %s 当前资产状态：", name))
		if status != "" {
			b.WriteString("运行状态=" + status)
		}
		if authStatus != "" {
			b.WriteString("，认证=" + authStatus)
		}
		if monitorStatus != "" {
			b.WriteString(fmt.Sprintf("，监控状态=%s", monitorStatus))
		}
		if alertStatus != "" {
			b.WriteString(fmt.Sprintf("，告警状态=%s", alertStatus))
		}
		b.WriteString("。")
		b.WriteString("\n当前数据库上下文工具只返回资产、认证、监控登记和告警状态，没有返回 CPU、连接数、QPS、慢 SQL、复制延迟等实时压力指标；如需判断压力，需要补充数据库监控指标或慢日志分析结果。")
	}
	b.WriteString("\n重点实例：")
	limit := len(items)
	if limit > 5 {
		limit = 5
	}
	for i := 0; i < limit; i++ {
		item := assistantChatMap(items[i])
		name := firstNonEmptyAssistantChatString(item, "instanceName", "name")
		if name == "" {
			name = fmt.Sprintf("数据库 #%d", i+1)
		}
		b.WriteString(fmt.Sprintf("\n%d. %s", i+1, name))
		if typ := firstNonEmptyAssistantChatString(item, "type"); typ != "" {
			b.WriteString(" | 类型：" + typ)
		}
		if env := firstNonEmptyAssistantChatString(item, "environment"); env != "" {
			b.WriteString(" | 环境：" + env)
		}
		if status := firstNonEmptyAssistantChatString(item, "status"); status != "" {
			b.WriteString(" | 状态：" + status)
		}
		if endpoint := firstNonEmptyAssistantChatString(item, "endpoint", "hostIp"); endpoint != "" {
			b.WriteString(" | 地址：" + endpoint)
		}
		if owner := firstNonEmptyAssistantChatString(item, "dbaOwner", "owner"); owner != "" {
			b.WriteString(" | 负责人：" + owner)
		}
	}
	return b.String()
}

func buildAssistantChatDatabasePressureAnswer(output map[string]interface{}) string {
	items := assistantChatSlice(output["items"])
	matched := int(assistantChatNumber(output["matched"]))
	if len(items) == 0 {
		return buildAssistantChatDatabaseAnswer(output) + "\n无法判断压力：没有匹配到具体数据库实例。"
	}
	if matched > 1 && len(items) > 1 {
		return buildAssistantChatDatabaseAnswer(output) + "\n无法判断单实例压力：本次匹配到多个数据库实例，请指定实例名或 assetId。"
	}
	item := assistantChatMap(items[0])
	assetID := uint(assistantChatNumber(item["id"]))
	name := firstNonEmptyAssistantChatString(item, "instanceName", "name")
	if name == "" {
		name = fmt.Sprintf("数据库 #%d", assetID)
	}
	typ := firstNonEmptyAssistantChatString(item, "type")
	status := firstNonEmptyAssistantChatString(item, "status")
	authStatus := firstNonEmptyAssistantChatString(item, "authStatus")
	monitorStatus := firstNonEmptyAssistantChatString(item, "monitorStatus")
	alertStatus := firstNonEmptyAssistantChatString(item, "alertStatus")
	endpoint := firstNonEmptyAssistantChatString(item, "endpoint", "hostIp")
	slowLogPath := firstNonEmptyAssistantChatString(item, "slowLogPath")
	prometheusAccountID := uint(assistantChatNumber(item["prometheusAccountId"]))
	prometheusLabelSelector := firstNonEmptyAssistantChatString(item, "prometheusLabelSelector")
	var b strings.Builder
	b.WriteString(fmt.Sprintf("数据库实例 %s 压力检查：", name))
	if typ != "" {
		b.WriteString("类型=" + typ)
	}
	if endpoint != "" {
		b.WriteString("，地址=" + endpoint)
	}
	if status != "" {
		b.WriteString("，运行状态=" + status)
	}
	if authStatus != "" {
		b.WriteString("，认证=" + authStatus)
	}
	if monitorStatus != "" {
		b.WriteString("，监控状态=" + monitorStatus)
	}
	if alertStatus != "" {
		b.WriteString("，告警状态=" + alertStatus)
	}
	b.WriteString("。\n")

	blockers := []string{}
	if authStatus != "" && authStatus != "认证成功" {
		blockers = append(blockers, "数据库资产未认证，无法进行数据库连接类检查")
	}
	if assetID == 0 {
		blockers = append(blockers, "资产 ID 为空，无法关联监控配置")
	}
	if prometheusAccountID == 0 {
		blockers = append(blockers, "当前上下文没有可用的数据库实时监控指标")
	}

	if prometheusAccountID > 0 && assistantDatabaseTypeSupportsPressureSnapshot(typ) {
		snapshot, err := queryAssistantDatabasePressureSnapshot(context.Background(), prometheusAccountID, prometheusLabelSelector, typ)
		if err == nil && len(snapshot.Metrics) > 0 {
			abnormal := snapshot.Abnormal()
			if len(abnormal) > 0 {
				b.WriteString(fmt.Sprintf("结论：存在压力或风险，发现 %d 个异常指标。", len(abnormal)))
			} else {
				b.WriteString("结论：当前监控大盘关键指标未发现明显压力。")
			}
			if snapshot.Selector != "" {
				b.WriteString("\n监控选择器：" + snapshot.Selector)
			}
			if len(snapshot.Warnings) > 0 {
				b.WriteString("\n监控提示：" + strings.Join(snapshot.Warnings, "；"))
			}
			b.WriteString("\n关键指标：" + snapshot.Summary())
			if len(abnormal) > 0 {
				b.WriteString("\n异常指标：")
				for i, metric := range abnormal {
					if i >= 8 {
						break
					}
					b.WriteString(fmt.Sprintf("\n%d. %s=%s%s，状态=%s", i+1, metric.Name, formatAssistantMetricValue(metric.Value), metric.Unit, metric.Status))
				}
			}
			if authStatus != "" && authStatus != "认证成功" {
				b.WriteString("\n补充说明：数据库资产未认证会影响连接类巡检和慢日志读取，但 Prometheus 指标已可用于压力初判。")
			}
			return strings.TrimSpace(b.String())
		}
		if err != nil {
			blockers = append(blockers, "读取 Prometheus 监控失败："+err.Error())
		}
	}

	if len(blockers) > 0 {
		b.WriteString("结论：暂时不能可靠判断该实例是否有压力。")
		b.WriteString("\n原因：" + strings.Join(blockers, "；") + "。")
	}
	if alertStatus != "" && alertStatus != "0" {
		b.WriteString("\n补充信号：该实例存在告警状态标记，建议先查看数据库相关告警详情。")
	}
	if slowLogPath != "" {
		b.WriteString("\n慢日志：资产已配置慢日志路径，可发起慢 SQL 分析辅助判断查询压力。")
	}
	if len(blockers) == 0 {
		b.WriteString("结论：资产状态未显示明显阻塞，但当前助手结果未包含实时指标明细，不能替代监控大盘判断。")
	}
	b.WriteString("\n要判断 MySQL 是否有压力，至少需要这些指标：CPU/内存/磁盘、连接数和连接使用率、Threads_running、QPS/TPS、慢查询速率、行锁等待、临时表落盘、Buffer Pool 命中率、InnoDB IO。")
	b.WriteString("\n下一步：完成数据库认证和 Prometheus/exporter 配置后，再读取 MySQL 监控大盘或慢日志分析结果。")
	if status != "" && status != "运行中" {
		b.WriteString("\n注意：当前资产运行状态不是运行中，应优先处理实例可用性。")
	}
	return strings.TrimSpace(b.String())
}

func buildAssistantChatK8sAnswer(output map[string]interface{}) string {
	total := int(assistantChatNumber(output["total"]))
	matched := int(assistantChatNumber(output["matched"]))
	items := assistantChatSlice(output["items"])
	var b strings.Builder
	b.WriteString(fmt.Sprintf("K8s 集群概览：平台当前登记 %d 个集群，本次匹配 %d 个。", total, matched))
	if len(items) == 0 {
		b.WriteString("没有找到匹配的集群。")
		return b.String()
	}
	b.WriteString("\n重点集群：")
	limit := len(items)
	if limit > 5 {
		limit = 5
	}
	for i := 0; i < limit; i++ {
		item := assistantChatMap(items[i])
		name := firstNonEmptyAssistantChatString(item, "name")
		if name == "" {
			name = fmt.Sprintf("集群 #%d", i+1)
		}
		b.WriteString(fmt.Sprintf("\n%d. %s", i+1, name))
		if version := firstNonEmptyAssistantChatString(item, "version"); version != "" {
			b.WriteString(" | 版本：" + version)
		}
		if status := firstNonEmptyAssistantChatString(item, "status"); status != "" {
			b.WriteString(" | 状态：" + status)
		}
		b.WriteString(fmt.Sprintf(" | 节点：%d/%d Ready",
			int(assistantChatNumber(item["readyNodes"])),
			int(assistantChatNumber(item["nodeCount"])),
		))
		if lastSync := firstNonEmptyAssistantChatString(item, "lastSyncAt"); lastSync != "" {
			b.WriteString(" | 最近同步：" + lastSync)
		}
	}
	return b.String()
}

func buildAssistantChatK8sLiveAnswer(clusterContext map[string]interface{}, outputs map[string]interface{}, errs []string) string {
	clusterName := "目标集群"
	items := assistantChatSlice(clusterContext["items"])
	selectedID := uint(assistantChatNumber(clusterContext["selectedClusterId"]))
	for _, raw := range items {
		item := assistantChatMap(raw)
		if selectedID != 0 && uint(assistantChatNumber(item["id"])) != selectedID {
			continue
		}
		if name := firstNonEmptyAssistantChatString(item, "name"); name != "" {
			clusterName = name
			break
		}
	}
	tools := make([]string, 0, len(outputs))
	for name := range outputs {
		tools = append(tools, name)
	}
	sort.Strings(tools)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("已对 %s 执行实时只读检查：%d 个工具成功", clusterName, len(outputs)))
	if len(errs) > 0 {
		b.WriteString(fmt.Sprintf("，%d 个工具失败", len(errs)))
	}
	b.WriteString("。")
	if len(tools) > 0 {
		b.WriteString("\n已采集证据：" + strings.Join(tools, "、") + "。")
	}
	if len(errs) > 0 {
		b.WriteString("\n未完成项：" + strings.Join(errs, "；") + "。")
	}
	if len(outputs) == 0 {
		b.WriteString("\n没有取得实时集群证据，不能判断集群是否正常。")
	} else {
		b.WriteString("\n以下结论基于本次实时工具返回，未执行重启、扩缩容、删除或 YAML 变更。")
	}
	return b.String()
}

func buildAssistantChatK8sFleetAnswer(report assistantK8sScanReport) string {
	var abnormalPods, warningEvents, notReadyNodes, pressureNodes int
	metricsClusters := 0
	for _, scan := range report.Clusters {
		abnormalPods += len(scan.AbnormalPods)
		warningEvents += len(scan.WarningEvents)
		notReadyNodes += len(scan.NotReadyNodes)
		pressureNodes += len(scan.PressureNodes)
		if scan.MetricsAvailable {
			metricsClusters++
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("已对授权范围内 %d/%d 个 K8s 集群发起实时只读扫描。", report.ScannedClusters, report.RequestedClusters))
	if report.SkippedClusters > 0 {
		b.WriteString(fmt.Sprintf("另有 %d 个集群因单次扫描上限未检查。", report.SkippedClusters))
	}
	if report.ScannedClusters == 0 {
		b.WriteString("没有取得实时证据，不能判断集群是否正常。")
		return b.String()
	}
	if abnormalPods == 0 && warningEvents == 0 && notReadyNodes == 0 && pressureNodes == 0 {
		b.WriteString("在已完成的检查范围内，未发现节点 NotReady、节点压力、异常/高重启 Pod 或 Warning 事件。")
	} else {
		b.WriteString(fmt.Sprintf("发现：NotReady 节点 %d 个、压力节点 %d 个、异常/高重启 Pod %d 个、近期 Warning 事件 %d 条。", notReadyNodes, pressureNodes, abnormalPods, warningEvents))
	}

	for _, scan := range report.Clusters {
		b.WriteString(fmt.Sprintf("\n\n%s：实时检查成功 %d 项", scan.ClusterName, scan.LiveChecks))
		if len(scan.FailedChecks) > 0 {
			b.WriteString(fmt.Sprintf("，失败 %d 项", len(scan.FailedChecks)))
		}
		b.WriteString(fmt.Sprintf("；节点 %d 个，授权 Namespace %d 个，已扫描 %d 个，Pod %d 个。", scan.NodeCount, scan.AuthorizedNS, scan.ScannedNS, scan.PodCount))
		if len(scan.NotReadyNodes) > 0 {
			b.WriteString("\n- NotReady 节点：" + strings.Join(scan.NotReadyNodes, "、"))
		}
		if len(scan.PressureNodes) > 0 {
			b.WriteString("\n- 节点压力：" + strings.Join(scan.PressureNodes, "、"))
		}
		if len(scan.AbnormalPods) > 0 {
			parts := make([]string, 0, minAssistantInt(len(scan.AbnormalPods), 8))
			for i, pod := range scan.AbnormalPods {
				if i >= 8 {
					break
				}
				detail := fmt.Sprintf("%s/%s(%s", pod.Namespace, pod.Pod, pod.Status)
				if pod.RestartCount > 0 {
					detail += fmt.Sprintf("，重启 %d 次", pod.RestartCount)
				}
				detail += ")"
				parts = append(parts, detail)
			}
			b.WriteString("\n- 需关注 Pod：" + strings.Join(parts, "、"))
			if len(scan.AbnormalPods) > len(parts) {
				b.WriteString(fmt.Sprintf(" 等 %d 个", len(scan.AbnormalPods)))
			}
		}
		if len(scan.WarningEvents) > 0 {
			limit := minAssistantInt(len(scan.WarningEvents), 5)
			b.WriteString("\n- Warning 事件：" + strings.Join(scan.WarningEvents[:limit], "；"))
		}
		if len(scan.FailedChecks) > 0 {
			limit := minAssistantInt(len(scan.FailedChecks), 4)
			b.WriteString("\n- 未完成：" + strings.Join(scan.FailedChecks[:limit], "；"))
		}
	}

	hotspots := make([]assistantK8sResourceHotspot, 0)
	for _, scan := range report.Clusters {
		hotspots = append(hotspots, scan.Hotspots...)
	}
	if len(hotspots) > 0 {
		byCPU := append([]assistantK8sResourceHotspot(nil), hotspots...)
		sort.Slice(byCPU, func(i, j int) bool { return byCPU[i].cpuMilli > byCPU[j].cpuMilli })
		byMemory := append([]assistantK8sResourceHotspot(nil), hotspots...)
		sort.Slice(byMemory, func(i, j int) bool { return byMemory[i].memoryByte > byMemory[j].memoryByte })
		b.WriteString("\n\nPod 资源热点（本次实时值）：")
		b.WriteString("\n- CPU Top：" + formatAssistantK8sHotspots(byCPU, true, 5))
		b.WriteString("\n- 内存 Top：" + formatAssistantK8sHotspots(byMemory, false, 5))
	} else if metricsClusters == 0 {
		b.WriteString("\n\n未取得 Pod 实时指标，不能判断 CPU/内存热点；这属于检查缺口，不能据此判定资源正常。")
	}
	b.WriteString("\n\n本次只读取已开启的 MCP Tool 和当前用户授权范围，没有执行任何集群变更。")
	return b.String()
}

func formatAssistantK8sHotspots(items []assistantK8sResourceHotspot, cpu bool, limit int) string {
	parts := make([]string, 0, minAssistantInt(len(items), limit))
	for _, item := range items {
		if len(parts) >= limit {
			break
		}
		usage := item.Memory
		rate := item.MemoryRate
		if cpu {
			usage = item.CPU
			rate = item.CPURate
		}
		if usage == "" {
			continue
		}
		value := fmt.Sprintf("%s/%s/%s %s", item.Cluster, item.Namespace, item.Pod, usage)
		if rate > 0 {
			value += fmt.Sprintf("(Request %.1f%%)", rate)
		}
		parts = append(parts, value)
	}
	if len(parts) == 0 {
		return "无可用数据"
	}
	return strings.Join(parts, "、")
}

func assistantK8sReportMissingInfo(report assistantK8sScanReport) []string {
	missing := make([]string, 0)
	if report.SkippedClusters > 0 {
		missing = append(missing, fmt.Sprintf("有 %d 个授权集群超过单次扫描上限，未执行实时检查。", report.SkippedClusters))
	}
	for _, scan := range report.Clusters {
		if scan.SkippedNS > 0 {
			missing = append(missing, fmt.Sprintf("%s 有 %d 个授权 Namespace 超过单次扫描上限。", scan.ClusterName, scan.SkippedNS))
		}
		if scan.PodResultsTruncated {
			missing = append(missing, scan.ClusterName+" 的 Pod 结果已截断，异常 Pod 数可能不完整。")
		}
		if len(scan.MetricsMissingNS) > 0 {
			missing = append(missing, fmt.Sprintf("%s 的 %d 个 Namespace 未返回 Pod Metrics。", scan.ClusterName, len(scan.MetricsMissingNS)))
		}
		if len(scan.FailedChecks) > 0 {
			missing = append(missing, fmt.Sprintf("%s 有 %d 项实时检查失败或被工具开关阻止。", scan.ClusterName, len(scan.FailedChecks)))
		}
	}
	return missing
}

func minAssistantInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func buildAssistantChatKnowledgeAnswer(items []aiopsmodel.KnowledgeSearchItem) string {
	if len(items) == 0 {
		return "知识库没有检索到匹配文档。建议补充更具体的告警名、服务名、错误关键字，或把本次处理过程沉淀为新的 Runbook。"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("知识库命中 %d 篇相关文档：", len(items)))
	limit := len(items)
	if limit > 5 {
		limit = 5
	}
	for i := 0; i < limit; i++ {
		item := items[i]
		b.WriteString(fmt.Sprintf("\n%d. %s", i+1, item.Title))
		if item.DocType != "" {
			b.WriteString(" | 类型：" + item.DocType)
		}
		if item.Category != "" {
			b.WriteString(" | 分类：" + item.Category)
		}
		if item.Score > 0 {
			b.WriteString(fmt.Sprintf(" | 相关度：%.0f%%", item.Score*100))
		}
		if text := firstNonEmptyAssistantChatText(item.Summary, item.Snippet); text != "" {
			b.WriteString("\n   " + truncateAssistantChatText(text, 160))
		}
	}
	return b.String()
}

func buildAssistantChatSkillCatalogAnswer(snapshot *agentskill.CatalogSnapshot) string {
	if snapshot == nil {
		return "AutoOps Skill Catalog 尚未加载。可以调用 POST /api/v1/aiops/skills/reload 重新加载。"
	}
	grouped := map[string][]agentskill.Manifest{}
	groupOrder := make([]string, 0)
	seenGroup := map[string]struct{}{}
	for _, manifest := range snapshot.Manifests {
		group := strings.TrimSpace(manifest.SkillGroup)
		if group == "" {
			group = "unknown"
		}
		if _, ok := seenGroup[group]; !ok {
			seenGroup[group] = struct{}{}
			groupOrder = append(groupOrder, group)
		}
		grouped[group] = append(grouped[group], manifest)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("AutoOps 后端当前加载了 %d 个声明式 Skill", snapshot.Loaded))
	if len(snapshot.Errors) > 0 {
		b.WriteString(fmt.Sprintf("，其中 %d 个文件加载失败", len(snapshot.Errors)))
	}
	if snapshot.LoadedAt != "" {
		b.WriteString("。加载时间：" + snapshot.LoadedAt)
	} else {
		b.WriteString("。")
	}
	b.WriteString("\n\n这些技能来自后端 `skills/*_skill/*/SKILL.md`。当前已支持加载、分类、查询、详情和重载；真实生产数据查询仍通过受控 Tool Gateway。")

	for _, group := range groupOrder {
		items := grouped[group]
		if len(items) == 0 {
			continue
		}
		b.WriteString("\n\n" + group + "：")
		for _, manifest := range items {
			b.WriteString(fmt.Sprintf("\n- %s", manifest.Name))
			if manifest.DisplayName != "" {
				b.WriteString("（" + manifest.DisplayName + "）")
			}
			b.WriteString("：" + manifest.Description)
			b.WriteString(fmt.Sprintf(" [status=%s, runtime=%s, risk=%s]", manifest.Status, manifest.Runtime, manifest.RiskLevel))
		}
	}
	if len(snapshot.Errors) > 0 {
		b.WriteString("\n\n加载错误：")
		limit := len(snapshot.Errors)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			item := snapshot.Errors[i]
			b.WriteString(fmt.Sprintf("\n- %s：%s", item.Path, item.Error))
		}
	}
	return b.String()
}

func buildAssistantChatMCPCatalogAnswer(servers []aiopsmodel.MCPServerView) string {
	runningServers := 0
	totalTools := 0
	supportedTools := 0
	enabledTools := 0
	for _, server := range servers {
		if server.Enabled {
			runningServers++
		}
		totalTools += server.TotalTools
		supportedTools += server.SupportedTools
		enabledTools += server.EnabledTools
	}
	disabledImplemented := supportedTools - enabledTools
	pendingTools := totalTools - supportedTools
	var b strings.Builder
	b.WriteString(fmt.Sprintf("当前接入 %d 个 MCP 服务，其中 %d 个正在运行；共声明 %d 个 Tool，已启用可调用 %d 个，已实现但关闭 %d 个，待实现 %d 个。\n\n",
		len(servers), runningServers, totalTools, enabledTools, disabledImplemented, pendingTools))
	b.WriteString("| MCP 服务 | 运行状态 | 已启用可调用 | 已实现 | 总数 | 待实现 |\n")
	b.WriteString("| --- | --- | ---: | ---: | ---: | ---: |\n")
	for _, server := range servers {
		status := "已关闭"
		if server.Enabled {
			status = "运行中"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %d | %d |\n",
			escapeAssistantMarkdownCell(server.DisplayName), status, server.EnabledTools, server.SupportedTools, server.TotalTools, server.TotalTools-server.SupportedTools))
	}
	return strings.TrimSpace(b.String())
}

func buildAssistantChatUnknownAnswer() string {
	return "我现在是 AutoOps 平台内的通用助手入口。支持主机压力检查、活跃告警概览、资源选址、资源回收、容量巡检、资产查询和通用运维问答。更复杂的多步外部任务可以交给 Hermes Runtime。"
}

func buildAssistantChatOpsGeneralFallbackAnswer(question string) string {
	normalized := strings.ToLower(strings.TrimSpace(question))
	if containsAnyAssistantChatKeyword(normalized, []string{"linux"}) &&
		containsAnyAssistantChatKeyword(normalized, []string{"巡检", "检查", "inspection", "check"}) &&
		containsAnyAssistantChatKeyword(normalized, []string{"脚本", "shell", "bash", "script"}) {
		return strings.TrimSpace(`下面是一个 Linux 只读巡检脚本示例，只采集系统信息，不执行重启、删除、清理、修改配置等变更动作。建议保存后先在测试环境执行。

~~~bash
#!/usr/bin/env bash
set -u

echo "===== 基础信息 ====="
echo "主机名: $(hostname 2>/dev/null || true)"
echo "当前时间: $(date '+%F %T' 2>/dev/null || true)"
echo "运行时长:"
uptime 2>/dev/null || true
echo

echo "===== 系统版本 ====="
if [[ -f /etc/os-release ]]; then
  cat /etc/os-release
else
  uname -a
fi
echo

echo "===== CPU 与负载 ====="
nproc 2>/dev/null | awk '{print "CPU 核数: "$0}' || true
top -bn1 | head -n 5 2>/dev/null || true
echo

echo "===== 内存 ====="
free -h 2>/dev/null || true
echo

echo "===== 磁盘空间 ====="
df -hT 2>/dev/null || true
echo

echo "===== inode 使用率 ====="
df -ih 2>/dev/null || true
echo

echo "===== 挂载信息 ====="
mount | column -t 2>/dev/null || mount 2>/dev/null || true
echo

echo "===== 网络监听端口 ====="
if command -v ss >/dev/null 2>&1; then
  ss -lntup
else
  netstat -lntup 2>/dev/null || true
fi
echo

echo "===== 路由与地址 ====="
ip addr show 2>/dev/null || ifconfig 2>/dev/null || true
ip route show 2>/dev/null || route -n 2>/dev/null || true
echo

echo "===== 进程资源 Top ====="
ps aux --sort=-%cpu | head -n 10 2>/dev/null || true
echo
ps aux --sort=-%mem | head -n 10 2>/dev/null || true
echo

echo "===== systemd 失败服务 ====="
if command -v systemctl >/dev/null 2>&1; then
  systemctl --failed --no-pager
fi
echo

echo "===== 最近内核日志 ====="
dmesg -T 2>/dev/null | tail -n 50 || dmesg 2>/dev/null | tail -n 50 || true
echo

echo "===== 最近登录 ====="
last -n 10 2>/dev/null || true
echo

echo "===== 巡检完成 ====="
~~~

执行方式：

~~~bash
chmod +x linux_readonly_check.sh
./linux_readonly_check.sh | tee linux_readonly_check_$(hostname)_$(date +%F_%H%M%S).log
~~~

说明：脚本只读查询系统状态。如果后续要重启服务、清理磁盘、修改配置或处理异常进程，需要单独确认影响范围并走审批。`)
	}
	if containsAnyAssistantChatKeyword(normalized, []string{"nginx"}) &&
		containsAnyAssistantChatKeyword(normalized, []string{"安装", "install", "部署"}) {
		return strings.TrimSpace(`下面是一个 Linux 安装 nginx 的只读友好脚本示例，默认只安装和启动服务，不会改业务配置。执行前建议先在测试机器验证。

~~~bash
#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "请使用 root 或 sudo 执行"
  exit 1
fi

if command -v apt-get >/dev/null 2>&1; then
  apt-get update
  apt-get install -y nginx
elif command -v yum >/dev/null 2>&1; then
  yum install -y epel-release || true
  yum install -y nginx
elif command -v dnf >/dev/null 2>&1; then
  dnf install -y nginx
else
  echo "未识别包管理器，请手动安装 nginx"
  exit 1
fi

nginx -t
systemctl enable nginx
systemctl restart nginx
systemctl status nginx --no-pager
~~~

验证命令：

~~~bash
curl -I http://127.0.0.1/
ss -lntp | grep ':80'
journalctl -u nginx -n 50 --no-pager
~~~`)
	}
	if containsAnyAssistantChatKeyword(normalized, []string{"nginx"}) &&
		containsAnyAssistantChatKeyword(normalized, []string{"502", "bad gateway", "排查", "故障"}) {
		return strings.TrimSpace(`nginx 502 通常表示 nginx 已收到请求，但上游服务不可用、超时或返回了异常响应。建议按这个顺序排查：

1. 看 nginx 错误日志：

~~~bash
tail -n 100 /var/log/nginx/error.log
tail -n 100 /var/log/nginx/access.log
~~~

2. 确认 upstream 后端是否存活：

~~~bash
grep -R "proxy_pass\|upstream" /etc/nginx/conf.d /etc/nginx/nginx.conf
ss -lntp
curl -v http://127.0.0.1:后端端口/health
~~~

3. 检查常见原因：

- 后端进程没启动，端口未监听。
- nginx 配置里的 upstream 地址或端口写错。
- 后端响应太慢，触发 proxy_read_timeout。
- 后端连接数打满，出现 connection refused/reset。
- 容器或 K8s Service/Endpoint 没有健康后端。
- SELinux、防火墙或安全组阻断本机到后端端口。

4. 修改配置前先校验：

~~~bash
nginx -t
systemctl reload nginx
~~~`)
	}
	if containsAnyAssistantChatKeyword(normalized, []string{"cpu", "使用率", "负载", "load", "高", "飙高", "过高"}) &&
		containsAnyAssistantChatKeyword(normalized, []string{"排查", "怎么", "如何", "问题", "异常", "高"}) {
		return strings.TrimSpace(`系统 CPU 使用率比较高，可以按“确认现象 -> 定位进程 -> 判断类型 -> 处理风险”的顺序排查。

1. 先确认 CPU 和负载是否真的异常：

~~~bash
uptime
top -bn1 | head -n 20
mpstat 1 5 2>/dev/null || true
~~~

重点看：

- load average 是否长期高于 CPU 核数。
- us 高：通常是业务进程计算消耗。
- sy 高：可能是内核调用、网络包、文件系统或驱动问题。
- wa 高：CPU 在等磁盘 IO，根因可能不是 CPU。
- st 高：虚拟化宿主机资源争抢。

2. 找出 CPU Top 进程和线程：

~~~bash
ps aux --sort=-%cpu | head -n 15
pidstat -u 1 5 2>/dev/null || true
top -H -p <PID>
~~~

如果是 Java 进程，可以继续看热点线程：

~~~bash
top -H -p <PID>
printf "%x\n" <线程ID>
jstack <PID> | grep -A 30 "<十六进制线程ID>"
~~~

3. 判断是业务流量、异常循环还是系统瓶颈：

~~~bash
ss -s
iostat -xz 1 5 2>/dev/null || true
vmstat 1 5
dmesg -T | tail -n 50 2>/dev/null || dmesg | tail -n 50
~~~

4. 常见原因：

- 业务流量突增，应用线程池或计算任务打满。
- 慢 SQL、缓存击穿、批处理任务导致应用 CPU 升高。
- 进程死循环、频繁 GC、日志量暴涨。
- 磁盘 IO 慢导致 iowait 高，看起来像机器很卡。
- 容器 CPU limit 太低，被 CFS throttling 限制。

5. 处理建议：

- 先保留现场：时间点、Top 进程、线程栈、应用日志、监控截图。
- 不要直接 kill 或重启生产进程，除非已有应急审批。
- 如果是流量突增，优先扩容、限流或切流。
- 如果是单进程异常，定位线程栈和最近发布变更。
- 如果是 iowait 高，转向磁盘、存储、慢查询排查。`)
	}
	return strings.TrimSpace(`这是一个通用运维问题。当前模型没有成功生成回答，我先给一个通用排查框架，帮助你继续推进：

1. 明确现象：发生时间、影响范围、报错信息、是否持续、是否和发布/变更有关。
2. 看监控趋势：CPU、内存、磁盘、网络、连接数、错误率、延迟、队列堆积。
3. 查日志：系统日志、应用日志、中间件日志，优先看异常时间点前后 10-30 分钟。
4. 定位对象：是单机、单实例、单服务，还是全局性问题。
5. 区分读操作和变更操作：排查命令尽量只读；重启、删除、清理、扩容、改配置需要审批。

常用只读命令：

~~~bash
uptime
top -bn1 | head -n 20
free -h
df -hT
ss -lntup
journalctl -xe --no-pager | tail -n 100
~~~

如果你补充具体对象、日志或指标，我可以继续给更聚焦的排查步骤。`)
}

func buildAssistantChatGeneralQAFallbackAnswer() string {
	return "这是一个通用知识问题，我会直接回答，不会调用生产工具。"
}

func buildAssistantPathInspectionCard(host map[string]interface{}, evidence []aiopsmodel.AgentDiagnosisEvidence, missing, actions []string) *aiopsmodel.AssistantChatDiagnosisCard {
	hostName := firstNonEmptyAssistantChatString(host, "hostName", "name", "privateIp", "sshIp", "publicIp")
	return &aiopsmodel.AssistantChatDiagnosisCard{
		Summary:           "已完成主机目录或文本文件只读检查",
		Severity:          aiopsmodel.AssistantChatSeverityInfo,
		ImpactedResources: []string{hostName},
		Evidence:          evidence,
		Hypotheses:        []aiopsmodel.AgentDiagnosisHypothesis{},
		NextChecks:        actions,
		Remediation:       []string{"当前结果仅用于核对；修改配置、删除文件或重启服务必须进入对应受控操作。"},
		HandoffNote:       "路径检查没有使用 sudo，也没有修改远程文件。",
		MissingData:       missing,
	}
}

func buildAssistantDiskUsageCard(host map[string]interface{}, evidence []aiopsmodel.AgentDiagnosisEvidence, missing, actions []string) *aiopsmodel.AssistantChatDiagnosisCard {
	hostName := firstNonEmptyAssistantChatString(host, "hostName", "name", "privateIp", "sshIp", "publicIp")
	return &aiopsmodel.AssistantChatDiagnosisCard{
		Summary:           "已完成主机磁盘目录和大文件占用分析",
		Severity:          aiopsmodel.AssistantChatSeverityInfo,
		ImpactedResources: []string{hostName},
		Evidence:          evidence,
		Hypotheses:        []aiopsmodel.AgentDiagnosisHypothesis{},
		NextChecks:        actions,
		Remediation:       []string{"清理前核对文件归属、活跃写入状态和保留要求，并按受控日志清理流程二次确认。"},
		HandoffNote:       "目录分析为只读操作，本次没有删除或修改任何文件。",
		MissingData:       missing,
	}
}

func buildAssistantLogCleanupCard(host, preview map[string]interface{}, evidence []aiopsmodel.AgentDiagnosisEvidence, missing, actions []string) *aiopsmodel.AssistantChatDiagnosisCard {
	hostName := firstNonEmptyAssistantChatString(host, "hostName", "name", "privateIp", "sshIp", "publicIp")
	summary := "日志清空预览已生成，等待系统管理员二次确认"
	if !assistantChatBool(preview["requiresConfirmation"]) {
		summary = firstNonEmptyAssistantChatString(preview, "message")
	}
	return &aiopsmodel.AssistantChatDiagnosisCard{
		Summary: summary, Severity: aiopsmodel.AssistantChatSeverityWarning,
		ImpactedResources: []string{hostName}, Evidence: evidence,
		Hypotheses: []aiopsmodel.AgentDiagnosisHypothesis{}, NextChecks: actions,
		Remediation: []string{"确认后仅清空日志内容，不删除日志文件。"},
		HandoffNote: "确认接口只接受签名凭证，并再次校验登录用户的系统管理员身份；远端命令不使用 sudo。",
		MissingData: missing,
	}
}

func buildAssistantHostPressureCard(host, agent, metrics map[string]interface{}, minutes int, evidence []aiopsmodel.AgentDiagnosisEvidence, missing, actions []string) *aiopsmodel.AssistantChatDiagnosisCard {
	diagnosis := assistantChatMap(metrics["diagnosis"])
	summary := assistantChatMap(metrics["summary"])
	risks := assistantChatStringSlice(diagnosis["risks"])
	severity := aiopsmodel.AssistantChatSeverityInfo
	if len(risks) > 0 {
		severity = aiopsmodel.AssistantChatSeverityWarning
	} else if !assistantChatBool(metrics["hasData"]) || len(summary) == 0 {
		severity = aiopsmodel.AssistantChatSeverityUnknown
	}

	resource := assistantChatHostDisplay(host)
	card := &aiopsmodel.AssistantChatDiagnosisCard{
		Summary:           fmt.Sprintf("%s 最近 %d 分钟资源压力检查完成。", resource, minutes),
		Severity:          severity,
		ImpactedResources: []string{resource},
		Evidence:          evidence,
		NextChecks:        actions,
		Remediation:       []string{},
		HandoffNote:       "本次检查为只读诊断，没有执行任何变更操作。",
		MissingData:       append([]string{}, missing...),
	}
	if len(risks) > 0 {
		card.Hypotheses = append(card.Hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
			Cause:              strings.Join(risks, "；"),
			Confidence:         "medium",
			SupportingEvidence: []string{"query_host_metrics"},
		})
		card.Remediation = append(card.Remediation, "如需处置高占用进程、清理磁盘或重启服务，需要发起审批型变更。")
	} else {
		card.Hypotheses = append(card.Hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
			Cause:              "当前核心指标处于低位，未发现明显资源压力。",
			Confidence:         "medium",
			SupportingEvidence: []string{"query_host_metrics"},
		})
	}
	if !assistantChatBool(agent["online"]) && len(agent) > 0 {
		card.MissingData = append(card.MissingData, "采集 Agent 未确认在线，监控结论需要结合 Agent 状态复核。")
	}
	return card
}

func buildAssistantHostServicesCard(host, output map[string]interface{}, evidence []aiopsmodel.AgentDiagnosisEvidence, missing, actions []string) *aiopsmodel.AssistantChatDiagnosisCard {
	hostData := assistantChatMap(output["host"])
	if len(hostData) == 0 {
		hostData = host
	}
	resource := assistantChatHostDisplay(hostData)
	services := assistantChatSlice(output["services"])
	severity := aiopsmodel.AssistantChatSeverityInfo
	if len(services) == 0 {
		severity = aiopsmodel.AssistantChatSeverityUnknown
	}
	card := &aiopsmodel.AssistantChatDiagnosisCard{
		Summary:           fmt.Sprintf("%s 服务发现完成，推断出 %d 个疑似服务/进程。", resource, len(services)),
		Severity:          severity,
		ImpactedResources: []string{resource},
		Evidence:          evidence,
		NextChecks:        actions,
		Remediation:       []string{},
		HandoffNote:       "本次结果基于只读进程指标推断，没有执行登录命令或变更操作。",
		MissingData:       append([]string{}, missing...),
	}
	if len(services) == 0 {
		card.MissingData = append(card.MissingData, "Top 进程数据未能推断出明确服务")
		card.Hypotheses = append(card.Hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
			Cause:              "采集数据为空、服务负载较低，或业务服务未进入 Top 进程。",
			Confidence:         "medium",
			SupportingEvidence: []string{"query_host_services"},
		})
		return card
	}
	names := make([]string, 0, len(services))
	for _, raw := range services {
		item := assistantChatMap(raw)
		if name := firstNonEmptyAssistantChatString(item, "name"); name != "" {
			names = append(names, name)
		}
	}
	card.Hypotheses = append(card.Hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
		Cause:              "主机当前存在疑似服务/进程：" + strings.Join(names, "、"),
		Confidence:         "medium",
		SupportingEvidence: []string{"query_host_services"},
	})
	return card
}

func buildAssistantHostAgentUnavailableCard(host, agent map[string]interface{}, evidence []aiopsmodel.AgentDiagnosisEvidence, missing, actions []string) *aiopsmodel.AssistantChatDiagnosisCard {
	resource := assistantChatHostDisplay(host)
	statusText := firstNonEmptyAssistantChatString(agent, "statusText", "status")
	if statusText == "" {
		statusText = "未安装或未在线"
	}
	card := &aiopsmodel.AssistantChatDiagnosisCard{
		Summary:           fmt.Sprintf("%s Agent 不可用，无法读取进程或指标采集数据。", resource),
		Severity:          aiopsmodel.AssistantChatSeverityUnknown,
		ImpactedResources: []string{resource},
		Evidence:          evidence,
		NextChecks:        actions,
		HandoffNote:       "本次检查为只读诊断，没有执行任何变更操作。",
		MissingData:       append([]string{}, missing...),
	}
	card.MissingData = appendMissingStrings(card.MissingData, "Agent 状态："+statusText, "缺少 Top 进程或指标采集数据。")
	card.Hypotheses = append(card.Hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
		Cause:              "Agent 未安装、离线或采集链路不可用，无法判断该主机部署服务。",
		Confidence:         "medium",
		SupportingEvidence: []string{"get_agent_status"},
	})
	return card
}

func buildAssistantAlertOverviewCard(output interface{}, evidence []aiopsmodel.AgentDiagnosisEvidence, missing, actions []string) *aiopsmodel.AssistantChatDiagnosisCard {
	data := assistantChatMap(output)
	total := int(assistantChatNumber(data["total"]))
	severity := aiopsmodel.AssistantChatSeverityInfo
	if total > 0 {
		severity = aiopsmodel.AssistantChatSeverityWarning
	}
	return &aiopsmodel.AssistantChatDiagnosisCard{
		Summary:           fmt.Sprintf("当前活跃告警共 %d 条。", total),
		Severity:          severity,
		ImpactedResources: assistantChatAlertResources(output),
		Evidence:          evidence,
		Hypotheses: []aiopsmodel.AgentDiagnosisHypothesis{
			{
				Cause:              "存在活跃告警时，需要进入单条告警诊断补齐 CMDB、指标和日志证据。",
				Confidence:         "medium",
				SupportingEvidence: []string{"query_active_alerts"},
			},
		},
		NextChecks:  actions,
		Remediation: []string{"先完成只读诊断，再决定是否进入变更或 Runbook 流程。"},
		HandoffNote: "本次仅查询活跃告警概览，没有执行任何处置动作。",
		MissingData: append([]string{}, missing...),
	}
}

func buildAssistantPlatformOverviewCard(context map[string]interface{}, evidence []aiopsmodel.AgentDiagnosisEvidence, missing, actions []string) *aiopsmodel.AssistantChatDiagnosisCard {
	alerts := assistantChatMap(context["alerts"])
	hosts := assistantChatMap(context["hosts"])
	databases := assistantChatMap(context["database"])
	k8s := assistantChatMap(context["k8s"])

	alertTotal := int(assistantChatNumber(alerts["total"]))
	authFailed := int(assistantChatNumber(hosts["authFailed"]))
	dbAbnormal := countAssistantDatabaseAbnormalAssets(databases)
	k8sNotReady := countAssistantK8sNotReadyClusters(k8s)

	severity := aiopsmodel.AssistantChatSeverityInfo
	if alertTotal > 0 || authFailed > 0 || dbAbnormal > 0 || k8sNotReady > 0 {
		severity = aiopsmodel.AssistantChatSeverityWarning
	}
	if len(context) == 0 {
		severity = aiopsmodel.AssistantChatSeverityUnknown
	}

	resources := []string{}
	resources = append(resources, assistantChatAlertResources(alerts)...)
	resources = append(resources, assistantChatPlatformK8sResources(k8s)...)
	resources = append(resources, assistantChatPlatformDatabaseResources(databases)...)
	if len(resources) > 8 {
		resources = resources[:8]
	}

	card := &aiopsmodel.AssistantChatDiagnosisCard{
		Summary: fmt.Sprintf("平台总览完成：活跃告警 %d 条，主机认证失败 %d 台，数据库异常资产 %d 个，K8s 非全 Ready 集群 %d 个。",
			alertTotal, authFailed, dbAbnormal, k8sNotReady),
		Severity:           severity,
		ImpactedResources:  resources,
		Evidence:           evidence,
		NextChecks:         actions,
		Remediation:        []string{"本次为全局只读分析；任何生产变更、SQL 执行或 K8s 修改都需要走审批或工单。"},
		HandoffNote:        "本次从通用助手入口汇总平台数据，不依赖页面上下文。",
		MissingData:        append([]string{}, missing...),
		RunbookSuggestions: assistantChatPlatformRunbookSuggestions(context),
	}
	if severity == aiopsmodel.AssistantChatSeverityWarning {
		card.Hypotheses = append(card.Hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
			Cause:              "平台存在活跃告警或资产健康风险，需要按告警、主机认证、数据库状态、K8s 节点就绪度逐层收敛。",
			Confidence:         "medium",
			SupportingEvidence: []string{"query_active_alerts", "aiops_context_hosts", "aiops_context_database", "aiops_context_k8s"},
		})
	} else if len(context) > 0 {
		card.Hypotheses = append(card.Hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
			Cause:              "当前可读平台证据未显示明显全局风险。",
			Confidence:         "medium",
			SupportingEvidence: []string{"query_active_alerts", "aiops_context_hosts", "aiops_context_database", "aiops_context_k8s"},
		})
	}
	return card
}

func buildAssistantResourceGovernanceCard(queryResult *aiopsmodel.AssistantQueryResult, evidence []aiopsmodel.AgentDiagnosisEvidence, missing, actions []string) *aiopsmodel.AssistantChatDiagnosisCard {
	severity := aiopsmodel.AssistantChatSeverityInfo
	if queryResult == nil || queryResult.Summary.MatchedHosts == 0 {
		severity = aiopsmodel.AssistantChatSeverityWarning
	}
	resources := []string{}
	if queryResult != nil {
		for i, item := range queryResult.Candidates {
			if i >= 5 {
				break
			}
			if resource := formatAssistantResourceGovernanceResource(item); resource != "" {
				resources = append(resources, resource)
			}
		}
	}
	summary := "资源治理专用助手已完成分析。"
	if queryResult != nil && queryResult.Answer != "" {
		summary = firstAssistantChatLine(queryResult.Answer)
	}
	hypotheses := []aiopsmodel.AgentDiagnosisHypothesis{}
	if queryResult != nil && queryResult.Summary.MatchedHosts > 0 {
		hypotheses = append(hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
			Cause:              fmt.Sprintf("资源治理规则扫描全部 %d 台主机，其中 %d 台有监控指标并参与排名，筛出 %d 台候选资源。", queryResult.Summary.SelectedHosts, queryResult.Summary.MetricHosts, queryResult.Summary.MatchedHosts),
			Confidence:         "medium",
			SupportingEvidence: []string{"assistant_query", "cmdb", "agent", "metric"},
		})
	} else {
		hypotheses = append(hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
			Cause:              "当前筛选条件下没有形成可推荐资源，需要补充范围或检查监控数据。",
			Confidence:         "medium",
			SupportingEvidence: []string{"assistant_query"},
		})
	}
	return &aiopsmodel.AssistantChatDiagnosisCard{
		Summary:           summary,
		Severity:          severity,
		ImpactedResources: resources,
		Evidence:          evidence,
		Hypotheses:        hypotheses,
		NextChecks:        actions,
		Remediation:       []string{"涉及资源回收、扩容或迁移时，需要进入审批和变更流程。"},
		HandoffNote:       "资源治理结果来自专用助手，适合页面结构化展示 candidates、score 和 risks。",
		MissingData:       append([]string{}, missing...),
	}
}

func formatAssistantResourceGovernanceResource(item aiopsmodel.AssistantHostCandidate) string {
	name := firstNonEmptyAssistantText(item.HostName, item.Name, item.IP, item.PrivateIP, item.PublicIP, item.SSHIP)
	if name == "" {
		return ""
	}
	ip := firstNonEmptyAssistantText(item.IP, item.PrivateIP, item.PublicIP, item.SSHIP)
	if ip != "" && ip != name {
		return fmt.Sprintf("%s (%s) CPU %s / 内存 %s / 磁盘 %s / 评分 %.2f",
			name,
			ip,
			formatAssistantPercent(item.AvgCPU),
			formatAssistantPercent(item.AvgMemory),
			formatAssistantPercent(item.AvgDisk),
			item.Score,
		)
	}
	return fmt.Sprintf("%s CPU %s / 内存 %s / 磁盘 %s / 评分 %.2f",
		name,
		formatAssistantPercent(item.AvgCPU),
		formatAssistantPercent(item.AvgMemory),
		formatAssistantPercent(item.AvgDisk),
		item.Score,
	)
}

func buildAssistantDatabaseOverviewCard(output map[string]interface{}, evidence []aiopsmodel.AgentDiagnosisEvidence, missing, actions []string) *aiopsmodel.AssistantChatDiagnosisCard {
	matched := int(assistantChatNumber(output["matched"]))
	items := assistantChatSlice(output["items"])
	severity := aiopsmodel.AssistantChatSeverityInfo
	if matched == 0 {
		severity = aiopsmodel.AssistantChatSeverityWarning
	}
	resources := make([]string, 0, len(items))
	for i, raw := range items {
		if i >= 5 {
			break
		}
		item := assistantChatMap(raw)
		name := firstNonEmptyAssistantChatString(item, "instanceName", "name", "endpoint", "hostIp")
		if name != "" {
			resources = append(resources, name)
		}
	}
	card := &aiopsmodel.AssistantChatDiagnosisCard{
		Summary:           fmt.Sprintf("数据库资产上下文读取完成，本次匹配 %d 个实例。", matched),
		Severity:          severity,
		ImpactedResources: resources,
		Evidence:          evidence,
		NextChecks:        actions,
		Remediation:       []string{"数据库变更、SQL 执行和结构调整必须进入 SQL 审核或变更流程。"},
		HandoffNote:       "本次仅读取 CMDB 中的数据库资产信息，没有连接数据库、执行 SQL 或修改数据。",
		MissingData:       append([]string{}, missing...),
	}
	if matched == 0 {
		card.MissingData = append(card.MissingData, "未命中数据库资产，请补充实例名、IP、业务系统或 assetId。")
		card.Hypotheses = append(card.Hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
			Cause:              "缺少明确数据库对象，无法聚焦到具体实例。",
			Confidence:         "medium",
			SupportingEvidence: []string{"aiops_context_database"},
		})
	} else {
		card.Hypotheses = append(card.Hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
			Cause:              "已根据页面上下文或问题关键词定位数据库资产，可继续进行慢 SQL、监控或审核链路分析。",
			Confidence:         "medium",
			SupportingEvidence: []string{"aiops_context_database"},
		})
	}
	return card
}

func buildAssistantK8sOverviewCard(output map[string]interface{}, evidence []aiopsmodel.AgentDiagnosisEvidence, missing, actions []string) *aiopsmodel.AssistantChatDiagnosisCard {
	matched := int(assistantChatNumber(output["matched"]))
	items := assistantChatSlice(output["items"])
	liveScan := assistantChatMap(output["liveScan"])
	severity := aiopsmodel.AssistantChatSeverityInfo
	if matched == 0 {
		severity = aiopsmodel.AssistantChatSeverityWarning
	}
	resources := make([]string, 0, len(items))
	notReady := false
	for i, raw := range items {
		if i >= 5 {
			break
		}
		item := assistantChatMap(raw)
		name := firstNonEmptyAssistantChatString(item, "name")
		if name != "" {
			resources = append(resources, name)
		}
		nodeCount := int(assistantChatNumber(item["nodeCount"]))
		readyNodes := int(assistantChatNumber(item["readyNodes"]))
		if nodeCount > 0 && readyNodes < nodeCount {
			notReady = true
		}
	}
	if notReady {
		severity = aiopsmodel.AssistantChatSeverityWarning
	}
	liveTools := make([]string, 0)
	for _, item := range evidence {
		if strings.HasPrefix(item.Reference, "k8s_") && item.Reference != "k8s_list_clusters" {
			liveTools = appendUniqueAssistantString(liveTools, item.Reference)
		}
	}
	summary := fmt.Sprintf("K8s 集群范围读取完成，本次匹配 %d 个集群。", matched)
	handoff := "本次只读取授权范围内的集群登记信息，没有修改集群资源。"
	if len(liveScan) > 0 {
		scanned := int(assistantChatNumber(liveScan["scannedClusters"]))
		requested := int(assistantChatNumber(liveScan["requestedClusters"]))
		abnormalPods := 0
		warningEvents := 0
		failedChecks := 0
		for _, raw := range assistantChatSlice(liveScan["clusters"]) {
			scan := assistantChatMap(raw)
			abnormalPods += len(assistantChatSlice(scan["abnormalPods"]))
			warningEvents += len(assistantChatSlice(scan["warningEvents"]))
			failedChecks += len(assistantChatSlice(scan["failedChecks"]))
			if len(assistantChatSlice(scan["notReadyNodes"])) > 0 || len(assistantChatSlice(scan["pressureNodes"])) > 0 {
				severity = aiopsmodel.AssistantChatSeverityWarning
			}
		}
		if abnormalPods > 0 || warningEvents > 0 || failedChecks > 0 || scanned < requested {
			severity = aiopsmodel.AssistantChatSeverityWarning
		}
		summary = fmt.Sprintf("K8s 实时只读扫描完成 %d/%d 个集群，发现异常/高重启 Pod %d 个、Warning 事件 %d 条。", scanned, requested, abnormalPods, warningEvents)
		handoff = "本次已访问授权集群的只读 API；没有执行重启、伸缩、删除或 YAML 变更。"
	} else if len(liveTools) > 0 {
		summary = fmt.Sprintf("K8s 实时只读检查完成，成功采集 %d 类集群证据。", len(liveTools))
		handoff = "本次已访问授权集群的只读 API；没有执行重启、伸缩、删除或 YAML 变更。"
	}
	card := &aiopsmodel.AssistantChatDiagnosisCard{
		Summary:           summary,
		Severity:          severity,
		ImpactedResources: resources,
		Evidence:          evidence,
		NextChecks:        actions,
		Remediation:       []string{"删除 Pod、伸缩工作负载、修改 YAML 等操作需要进入确认或审批流程。"},
		HandoffNote:       handoff,
		MissingData:       append([]string{}, missing...),
	}
	if matched == 0 {
		card.MissingData = append(card.MissingData, "未命中 K8s 集群，请补充 clusterId、集群名、命名空间或资源名。")
	} else if len(liveScan) > 0 {
		if severity == aiopsmodel.AssistantChatSeverityWarning {
			card.Hypotheses = append(card.Hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
				Cause:              "实时扫描发现异常资源、Warning 事件、检查失败或未覆盖范围，需要按扫描结果继续定位。",
				Confidence:         "high",
				SupportingEvidence: append([]string{"k8s_health_scan_summary"}, liveTools...),
			})
		} else {
			card.Hypotheses = append(card.Hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
				Cause:              "已扫描范围内未发现节点 NotReady、节点压力、异常/高重启 Pod 或 Warning 事件。",
				Confidence:         "high",
				SupportingEvidence: append([]string{"k8s_health_scan_summary"}, liveTools...),
			})
		}
	} else if notReady {
		card.Hypotheses = append(card.Hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
			Cause:              "存在 Ready 节点数小于节点总数的集群，需要进入集群详情页查看节点和事件。",
			Confidence:         "medium",
			SupportingEvidence: append([]string{"k8s_list_clusters"}, liveTools...),
		})
	} else {
		card.Hypotheses = append(card.Hypotheses, aiopsmodel.AgentDiagnosisHypothesis{
			Cause:              "集群摘要未显示节点就绪数量异常，可继续按 Pod、事件或 Ingress 链路细查。",
			Confidence:         "medium",
			SupportingEvidence: append([]string{"k8s_list_clusters"}, liveTools...),
		})
	}
	return card
}

func buildAssistantKnowledgeCard(items []aiopsmodel.KnowledgeSearchItem, evidence []aiopsmodel.AgentDiagnosisEvidence, missing, actions []string) *aiopsmodel.AssistantChatDiagnosisCard {
	severity := aiopsmodel.AssistantChatSeverityInfo
	if len(items) == 0 {
		severity = aiopsmodel.AssistantChatSeverityWarning
	}
	resources := make([]string, 0, len(items))
	for i, item := range items {
		if i >= 5 {
			break
		}
		resources = append(resources, item.Title)
	}
	card := &aiopsmodel.AssistantChatDiagnosisCard{
		Summary:           fmt.Sprintf("知识库检索完成，命中 %d 篇文档。", len(items)),
		Severity:          severity,
		ImpactedResources: resources,
		Evidence:          evidence,
		NextChecks:        actions,
		Remediation:       []string{"知识库内容需要结合当前环境复核后再执行；涉及变更仍需走审批。"},
		HandoffNote:       "本次仅检索知识库，没有执行生产工具。",
		MissingData:       append([]string{}, missing...),
	}
	if len(items) == 0 {
		card.MissingData = append(card.MissingData, "知识库暂无匹配文档。")
	}
	return card
}

func finalizeAssistantChatDiagnosisCard(result *aiopsmodel.AssistantChatResult) {
	if result == nil {
		return
	}
	if result.DiagnosisCard == nil {
		result.DiagnosisCard = &aiopsmodel.AssistantChatDiagnosisCard{
			Summary:     firstAssistantChatLine(result.Answer),
			Severity:    aiopsmodel.AssistantChatSeverityUnknown,
			Evidence:    result.Evidence,
			NextChecks:  result.SuggestedActions,
			MissingData: append([]string{}, result.MissingInfo...),
			HandoffNote: "本次请求未匹配到明确的工具编排路径。",
		}
	}
	if result.DiagnosisCard.Summary == "" {
		result.DiagnosisCard.Summary = firstAssistantChatLine(result.Answer)
	}
	if result.DiagnosisCard.Severity == "" {
		result.DiagnosisCard.Severity = aiopsmodel.AssistantChatSeverityUnknown
	}
	if len(result.DiagnosisCard.Evidence) == 0 {
		result.DiagnosisCard.Evidence = result.Evidence
	}
	if len(result.DiagnosisCard.NextChecks) == 0 {
		result.DiagnosisCard.NextChecks = result.SuggestedActions
	}
	if len(result.MissingInfo) > 0 {
		result.DiagnosisCard.MissingData = appendMissingStrings(result.DiagnosisCard.MissingData, result.MissingInfo...)
	}
	if strings.TrimSpace(result.Answer) != "" && !assistantChatBool(result.Entities["workloadDiagnosis"]) {
		result.DiagnosisCard.Summary = firstAssistantChatLine(result.Answer)
	}
}

func reflectAssistantEvidence(result *aiopsmodel.AssistantChatResult) *aiopsmodel.AssistantReflection {
	if result == nil {
		return nil
	}
	supportedBy := assistantEvidenceSupport(result)
	expected := assistantExpectedEvidence(result.Intent, result)
	missingEvidence := make([]string, 0)
	for _, item := range expected {
		if !assistantEvidenceSatisfied(item, supportedBy, result.UsedTools) {
			missingEvidence = append(missingEvidence, item)
		}
	}
	missing := appendMissingStrings([]string{}, missingEvidence...)

	successfulTools := 0
	failedTools := 0
	for _, tool := range result.UsedTools {
		if tool.Success {
			successfulTools++
		} else {
			failedTools++
		}
	}

	score := 0
	if len(expected) > 0 {
		score = (len(expected) - len(missingEvidence)) * 100 / len(expected)
	} else if len(supportedBy) > 0 || successfulTools > 0 || result.Intent == aiopsmodel.AssistantChatIntentOpsGeneral || result.Intent == aiopsmodel.AssistantChatIntentGeneralQA {
		score = 80
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	if failedTools > 0 && score > 20 {
		score -= 10
	}
	if len(result.MissingInfo) > 0 && score > 70 {
		score -= 5
	}
	confidence := assistantConfidenceFromScore(score, len(supportedBy), len(missingEvidence))
	risks := make([]string, 0)
	if failedTools > 0 {
		risks = append(risks, fmt.Sprintf("%d 个工具调用失败，结论需要复核。", failedTools))
	}
	if len(missingEvidence) > 0 {
		risks = append(risks, "证据不完整，不能把当前结论当作最终根因。")
	}
	if len(result.MissingInfo) > 0 {
		risks = append(risks, "存在边界说明或人工确认项，执行变更前需要复核。")
		missing = appendMissingStrings(missing, result.MissingInfo...)
	}
	if result.Intent == aiopsmodel.AssistantChatIntentDatabaseOverview {
		risks = append(risks, "当前能力只读取平台登记的数据库上下文，不执行数据库 SQL。")
	}
	if result.Intent == aiopsmodel.AssistantChatIntentK8sOverview {
		risks = append(risks, "K8s 能力只调用授权范围内的只读 API，不执行集群变更。")
	}
	notes := []string{"反思结果由本次工具调用、证据类型和缺失信息规则化计算。"}
	if len(result.Evidence) == 0 && len(result.UsedTools) == 0 {
		notes = append(notes, "本次未采集生产证据，仅能提供通用建议。")
	}
	return &aiopsmodel.AssistantReflection{
		CompletenessScore: score,
		Confidence:        confidence,
		SupportedBy:       supportedBy,
		MissingEvidence:   missing,
		Risks:             risks,
		Notes:             notes,
	}
}

func assistantEvidenceSupport(result *aiopsmodel.AssistantChatResult) []string {
	if result == nil {
		return nil
	}
	seen := map[string]bool{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
	}
	for _, evidence := range result.Evidence {
		add(evidence.Type)
		add(evidence.Reference)
	}
	for _, tool := range result.UsedTools {
		if tool.Success {
			add(tool.Name)
		}
	}
	supported := make([]string, 0, len(seen))
	for value := range seen {
		supported = append(supported, value)
	}
	return supported
}

func assistantExpectedEvidence(intent string, result *aiopsmodel.AssistantChatResult) []string {
	switch intent {
	case aiopsmodel.AssistantChatIntentPlatformOverview:
		expected := []string{"alert", "cmdb", "database", "k8s"}
		if result != nil && assistantChatQuestionNeedsKnowledge(result.Question) {
			expected = append(expected, "knowledge")
		}
		return expected
	case aiopsmodel.AssistantChatIntentAlertOverview:
		return []string{"alert"}
	case aiopsmodel.AssistantChatIntentHostPressure:
		return []string{"cmdb", "agent", "metric"}
	case aiopsmodel.AssistantChatIntentHostServices:
		return []string{"cmdb", "agent", "process"}
	case aiopsmodel.AssistantChatIntentDatabaseOverview:
		return []string{"database"}
	case aiopsmodel.AssistantChatIntentK8sOverview:
		return []string{"k8s"}
	case aiopsmodel.AssistantChatIntentKnowledgeSearch:
		return []string{"knowledge"}
	case aiopsmodel.AssistantChatIntentResourceGovernance:
		return []string{"assistant_query", "cmdb", "agent", "metric"}
	default:
		return nil
	}
}

func assistantEvidenceSatisfied(expected string, supported []string, tools []aiopsmodel.AssistantChatUsedTool) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return true
	}
	for _, item := range supported {
		if item == expected {
			return true
		}
	}
	toolAlias := map[string][]string{
		"alert":           {"query_active_alerts", "query_alert_detail"},
		"cmdb":            {"get_cmdb_host", "search_cmdb_hosts", "aiops_context_hosts"},
		"agent":           {"get_agent_status"},
		"metric":          {"query_host_metrics"},
		"process":         {"query_host_services"},
		"database":        {"aiops_context_database"},
		"k8s":             {"k8s_list_clusters", "k8s_check_cluster", "k8s_list_nodes", "k8s_get_node_status", "k8s_get_workload_status", "k8s_get_workload_history", "k8s_get_pod_status", "k8s_query_events", "k8s_query_logs", "k8s_query_metrics", "k8s_diagnose_workload", "k8s_diagnose_pod", "k8s_diagnose_ingress"},
		"knowledge":       {"knowledge_search"},
		"assistant_query": {"assistant_query", "resource_scope", "resource_metrics"},
	}
	for _, tool := range tools {
		if !tool.Success {
			continue
		}
		for _, alias := range toolAlias[expected] {
			if tool.Name == alias {
				return true
			}
		}
	}
	return false
}

func assistantConfidenceFromScore(score int, supportedCount int, missingCount int) string {
	switch {
	case supportedCount == 0 && score < 60:
		return "unknown"
	case score >= 85 && missingCount == 0:
		return "high"
	case score >= 60:
		return "medium"
	case score > 0:
		return "low"
	default:
		return "unknown"
	}
}

func applyAssistantDiagnosisConfidence(result *aiopsmodel.AssistantChatResult) {
	if result == nil || result.DiagnosisCard == nil || len(result.DiagnosisCard.Hypotheses) == 0 {
		return
	}
	base := "unknown"
	if result.Reflection != nil {
		base = result.Reflection.Confidence
	}
	supported := assistantEvidenceSupport(result)
	for i := range result.DiagnosisCard.Hypotheses {
		hypothesis := &result.DiagnosisCard.Hypotheses[i]
		hypothesis.Confidence = assistantHypothesisConfidence(base, len(hypothesis.SupportingEvidence), len(supported), result.Reflection)
		if len(hypothesis.SupportingEvidence) == 0 {
			hypothesis.SupportingEvidence = assistantDefaultHypothesisEvidence(result)
		}
	}
}

func assistantHypothesisConfidence(base string, hypothesisEvidenceCount int, totalSupportCount int, reflection *aiopsmodel.AssistantReflection) string {
	score := 0
	switch base {
	case "high":
		score = 85
	case "medium":
		score = 65
	case "low":
		score = 40
	default:
		score = 20
	}
	if hypothesisEvidenceCount >= 2 {
		score += 10
	} else if hypothesisEvidenceCount == 0 {
		score -= 10
	}
	if totalSupportCount >= 3 {
		score += 5
	}
	if reflection != nil && len(reflection.MissingEvidence) > 0 {
		score -= 15
	}
	return assistantConfidenceFromScore(score, totalSupportCount, 0)
}

func assistantDefaultHypothesisEvidence(result *aiopsmodel.AssistantChatResult) []string {
	if result == nil {
		return nil
	}
	items := make([]string, 0)
	for _, evidence := range result.Evidence {
		if strings.TrimSpace(evidence.Title) != "" {
			items = append(items, evidence.Title)
		}
		if len(items) >= 3 {
			break
		}
	}
	return items
}

func applyAssistantQueryContext(req *aiopsmodel.AssistantQueryRequest, context map[string]interface{}) {
	if len(context) == 0 {
		return
	}
	if scopeValue, ok := context["scope"]; ok {
		_ = decodeAssistantChatValue(scopeValue, &req.Scope)
	}
	if optionsValue, ok := context["options"]; ok {
		_ = decodeAssistantChatValue(optionsValue, &req.Options)
	}
}

func firstAssistantChatHost(output interface{}) map[string]interface{} {
	data := assistantChatMap(output)
	items := assistantChatSlice(data["items"])
	if len(items) == 0 {
		return nil
	}
	return assistantChatMap(items[0])
}

func assistantChatResolvedHost(output interface{}) map[string]interface{} {
	data := assistantChatMap(output)
	if len(data) == 0 {
		return nil
	}
	if items := assistantChatSlice(data["items"]); len(items) > 0 {
		return assistantChatMap(items[0])
	}
	return data
}

func assistantChatMapOutput(tool *aiopsmodel.AgentToolInvokeResult) map[string]interface{} {
	if tool == nil {
		return nil
	}
	return assistantChatMap(tool.Output)
}

func assistantChatKnowledgeItems(value interface{}) []aiopsmodel.KnowledgeSearchItem {
	if value == nil {
		return nil
	}
	if typed, ok := value.([]aiopsmodel.KnowledgeSearchItem); ok {
		return typed
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var items []aiopsmodel.KnowledgeSearchItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil
	}
	return items
}

func mergeAssistantChatHostEntities(result *aiopsmodel.AssistantChatResult, host map[string]interface{}) {
	if result == nil || len(host) == 0 {
		return
	}
	if result.Entities == nil {
		result.Entities = map[string]interface{}{}
	}
	for _, key := range []string{"id", "hostName", "name", "privateIp", "publicIp", "sshIp", "groupName", "os"} {
		if value, ok := host[key]; ok && strings.TrimSpace(fmt.Sprintf("%v", value)) != "" {
			switch key {
			case "id":
				result.Entities["hostId"] = value
			case "privateIp":
				result.Entities["ip"] = value
			default:
				result.Entities[key] = value
			}
		}
	}
}

func markAssistantChatPlan(result *aiopsmodel.AssistantChatResult, toolName, status string, args map[string]interface{}, errText string) {
	if result == nil {
		return
	}
	for i := range result.ToolPlan {
		if result.ToolPlan[i].ToolName != toolName {
			continue
		}
		result.ToolPlan[i].Status = status
		if args != nil {
			result.ToolPlan[i].Arguments = args
		}
		result.ToolPlan[i].Error = errText
		break
	}
	if result.AssistantPlan != nil {
		for i := range result.AssistantPlan.Steps {
			if len(result.AssistantPlan.Steps[i].Tools) == 0 {
				continue
			}
			for _, name := range result.AssistantPlan.Steps[i].Tools {
				if name == toolName {
					result.AssistantPlan.Steps[i].Status = status
					return
				}
			}
		}
	}
}

func assistantChatPlanArguments(result *aiopsmodel.AssistantChatResult, toolName string) map[string]interface{} {
	if result == nil {
		return nil
	}
	for _, item := range result.ToolPlan {
		if item.ToolName == toolName {
			return item.Arguments
		}
	}
	return nil
}

func assistantChatAgentStepStatus(planStatus string) string {
	switch planStatus {
	case aiopsmodel.AssistantChatPlanFailed:
		return aiopsmodel.AgentStepStatusFailed
	case aiopsmodel.AssistantChatPlanRunning:
		return aiopsmodel.AgentStepStatusRunning
	default:
		return aiopsmodel.AgentStepStatusSucceeded
	}
}

func appendAssistantChatTimeline(result *aiopsmodel.AssistantChatResult, step int, toolName, status, message string, started, finished time.Time, errText string) {
	if result == nil {
		return
	}
	if step <= 0 {
		step = assistantChatPlanStep(result, toolName)
	}
	event := aiopsmodel.AssistantChatTimelineEvent{
		Step:       step,
		ToolName:   toolName,
		Status:     status,
		Message:    message,
		StartedAt:  started.Format("2006-01-02 15:04:05"),
		FinishedAt: finished.Format("2006-01-02 15:04:05"),
		DurationMS: finished.Sub(started).Milliseconds(),
		Error:      errText,
	}
	result.Timeline = append(result.Timeline, event)
	refreshAssistantChatProgress(result)
}

func initializeAssistantChatThinkingProcess(result *aiopsmodel.AssistantChatResult, startedAt, routedAt time.Time) {
	if result == nil {
		return
	}
	result.ThinkingProcess = &aiopsmodel.AssistantChatThinkingProcess{
		Status:     aiopsmodel.AgentRunStatusRunning,
		StartedAt:  startedAt.Format("2006-01-02 15:04:05.000"),
		DurationMS: routedAt.Sub(startedAt).Milliseconds(),
		Items: []aiopsmodel.AssistantChatThinkingProcessItem{
			{
				Key:         "request_received",
				Name:        "排队中",
				Description: "已收到问题，正在准备上下文",
				Status:      aiopsmodel.AssistantChatPlanSucceeded,
				Time:        startedAt.Format("15:04:05"),
				StartedAt:   startedAt.Format("2006-01-02 15:04:05"),
				FinishedAt:  routedAt.Format("2006-01-02 15:04:05"),
				DurationMS:  routedAt.Sub(startedAt).Milliseconds(),
			},
			{
				Key:         "action_router",
				Name:        "Action Router",
				Description: "已命中动作：" + assistantChatActionDisplayName(result),
				Status:      aiopsmodel.AssistantChatPlanSucceeded,
				Time:        routedAt.Format("15:04:05"),
				StartedAt:   routedAt.Format("2006-01-02 15:04:05"),
				FinishedAt:  routedAt.Format("2006-01-02 15:04:05"),
			},
		},
	}
}

func refreshAssistantChatThinkingProcess(result *aiopsmodel.AssistantChatResult, finishedAt time.Time, runStatus string) {
	if result == nil {
		return
	}
	if result.ThinkingProcess == nil {
		now := time.Now()
		initializeAssistantChatThinkingProcess(result, now, now)
	}
	thinking := result.ThinkingProcess
	items := make([]aiopsmodel.AssistantChatThinkingProcessItem, 0, len(result.Timeline)+2)
	for _, item := range thinking.Items {
		if item.Key == "request_received" || item.Key == "action_router" {
			if item.Key == "action_router" {
				item.Description = "已命中动作：" + assistantChatActionDisplayName(result)
			}
			items = append(items, item)
		}
	}
	hasAnalysisTimeline := false
	for index, event := range result.Timeline {
		description := strings.TrimSpace(event.Message)
		status := event.Status
		if result.AnalysisTask != nil && (event.ToolName == "k8s_diagnose_workload" || event.ToolName == "k8s_diagnose_pod") {
			hasAnalysisTimeline = true
			if current := strings.TrimSpace(result.AnalysisTask.Message); current != "" {
				description = current
			}
			if result.AnalysisTask.Status == aiopsmodel.AgentRunStatusRunning || result.AnalysisTask.Status == "queued" {
				status = aiopsmodel.AssistantChatPlanRunning
			}
		}
		items = append(items, aiopsmodel.AssistantChatThinkingProcessItem{
			Key:         fmt.Sprintf("step_%d_%s", index+1, event.ToolName),
			Name:        assistantChatProgressStepName(event.ToolName),
			Description: description,
			Status:      status,
			Time:        assistantChatClockTime(event.StartedAt),
			StartedAt:   event.StartedAt,
			FinishedAt:  event.FinishedAt,
			DurationMS:  event.DurationMS,
			ToolName:    event.ToolName,
			Error:       event.Error,
		})
	}
	for _, planStep := range result.ToolPlan {
		if planStep.Status != aiopsmodel.AssistantChatPlanRunning || assistantChatTimelineContainsTool(result, planStep.ToolName) {
			continue
		}
		if result.AnalysisTask != nil && (planStep.ToolName == "k8s_diagnose_workload" || planStep.ToolName == "k8s_diagnose_pod") {
			continue
		}
		key := "running_" + planStep.ToolName
		runningItem := aiopsmodel.AssistantChatThinkingProcessItem{}
		for _, previous := range thinking.Items {
			if previous.Key == key {
				runningItem = previous
				break
			}
		}
		if runningItem.Key == "" {
			now := time.Now()
			runningItem = aiopsmodel.AssistantChatThinkingProcessItem{
				Key:       key,
				Time:      now.Format("15:04:05"),
				StartedAt: now.Format("2006-01-02 15:04:05.000"),
			}
		}
		runningItem.Name = assistantChatProgressStepName(planStep.ToolName)
		runningItem.Description = strings.TrimSpace(planStep.Purpose)
		runningItem.Status = aiopsmodel.AssistantChatPlanRunning
		runningItem.ToolName = planStep.ToolName
		items = append(items, runningItem)
	}
	if result.AnalysisTask != nil && !hasAnalysisTimeline {
		analysisIndex := -1
		for index := range thinking.Items {
			if thinking.Items[index].Key == "analysis_task" {
				items = append(items, thinking.Items[index])
				analysisIndex = len(items) - 1
				break
			}
		}
		if analysisIndex < 0 {
			now := time.Now()
			items = append(items, aiopsmodel.AssistantChatThinkingProcessItem{
				Key:       "analysis_task",
				Name:      "服务异常证据收集",
				Status:    aiopsmodel.AssistantChatPlanRunning,
				Time:      now.Format("15:04:05"),
				StartedAt: now.Format("2006-01-02 15:04:05.000"),
				ToolName:  "k8s_diagnose_workload",
			})
			analysisIndex = len(items) - 1
		}
		analysisItem := &items[analysisIndex]
		analysisItem.Description = strings.TrimSpace(result.AnalysisTask.Message)
		analysisItem.Status = assistantChatAnalysisThinkingStatus(result.AnalysisTask.Status)
		if analysisItem.Status != aiopsmodel.AssistantChatPlanRunning && analysisItem.FinishedAt == "" {
			analysisItem.FinishedAt = time.Now().Format("2006-01-02 15:04:05.000")
		}
	}
	thinking.Items = items
	thinking.ToolCallCount = assistantChatThinkingToolCallCount(result)
	thinking.TotalSteps = len(items)
	thinking.CompletedSteps = 0
	for _, item := range items {
		switch item.Status {
		case aiopsmodel.AssistantChatPlanSucceeded, aiopsmodel.AssistantChatPlanFailed, aiopsmodel.AssistantChatPlanSkipped,
			aiopsmodel.AgentRunStatusPartial:
			thinking.CompletedSteps++
		}
	}
	end := time.Now()
	if !finishedAt.IsZero() {
		end = finishedAt
		thinking.FinishedAt = finishedAt.Format("2006-01-02 15:04:05")
	}
	if startedAt, err := assistantChatParseThinkingTime(thinking.StartedAt); err == nil {
		thinking.DurationMS = end.Sub(startedAt).Milliseconds()
	}
	if runStatus != "" {
		thinking.Status = runStatus
	} else {
		thinking.Status = aiopsmodel.AgentRunStatusRunning
	}
}

func assistantChatAnalysisThinkingStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "succeeded", "success", "completed", "complete", "done":
		return aiopsmodel.AssistantChatPlanSucceeded
	case "failed", "error":
		return aiopsmodel.AssistantChatPlanFailed
	default:
		return aiopsmodel.AssistantChatPlanRunning
	}
}

func assistantChatParseThinkingTime(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02 15:04:05.000", "2006-01-02 15:04:05"} {
		if parsed, err := time.ParseInLocation(layout, strings.TrimSpace(value), time.Local); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid thinking process time %q", value)
}

func assistantChatThinkingToolCallCount(result *aiopsmodel.AssistantChatResult) int {
	usedCount := 0
	for _, item := range result.UsedTools {
		if assistantChatThinkingLifecycleTool(item.Name) {
			continue
		}
		count := item.CallCount
		if count <= 0 {
			count = 1
		}
		usedCount += count
	}
	timelineCount := 0
	for _, item := range result.Timeline {
		if !assistantChatThinkingLifecycleTool(item.ToolName) {
			timelineCount++
		}
	}
	if timelineCount > usedCount {
		return timelineCount
	}
	return usedCount
}

func assistantChatThinkingLifecycleTool(toolName string) bool {
	switch toolName {
	case "", "understand_requirement", "ai_summary", "direct_answer", "llm_ops_general", "llm_general_qa":
		return true
	default:
		return false
	}
}

func assistantChatClockTime(value string) string {
	value = strings.TrimSpace(value)
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
		return parsed.Format("15:04:05")
	}
	return value
}

func assistantChatTimelineContainsTool(result *aiopsmodel.AssistantChatResult, toolName string) bool {
	if result == nil {
		return false
	}
	for _, item := range result.Timeline {
		if item.ToolName == toolName {
			return true
		}
	}
	return false
}

func assistantChatToolPlanState(result *aiopsmodel.AssistantChatResult, toolName string) (string, string) {
	if result != nil {
		for _, item := range result.ToolPlan {
			if item.ToolName == toolName {
				return item.Status, item.Error
			}
		}
	}
	return aiopsmodel.AssistantChatPlanSucceeded, ""
}

func assistantChatIntentDisplayName(intent string) string {
	switch intent {
	case aiopsmodel.AssistantChatIntentActionConfirmation:
		return "确认待执行操作"
	case aiopsmodel.AssistantChatIntentMCPCatalog:
		return "MCP 能力查询"
	case aiopsmodel.AssistantChatIntentHostInventory:
		return "主机资产查询"
	case aiopsmodel.AssistantChatIntentHostPressure:
		return "主机异常分析"
	case aiopsmodel.AssistantChatIntentHostLogCleanup:
		return "主机日志清理"
	case aiopsmodel.AssistantChatIntentHostServices:
		return "主机服务查询"
	case aiopsmodel.AssistantChatIntentAlertOverview:
		return "告警根因分析"
	case aiopsmodel.AssistantChatIntentPlatformOverview:
		return "平台态势分析"
	case aiopsmodel.AssistantChatIntentDatabaseOverview:
		return "数据库状态分析"
	case aiopsmodel.AssistantChatIntentK8sOverview:
		return "K8s 资源分析"
	case aiopsmodel.AssistantChatIntentK8sRolloutRestart:
		return "K8s 滚动重启"
	case aiopsmodel.AssistantChatIntentK8sScaleWorkload:
		return "K8s 工作负载扩缩容"
	case aiopsmodel.AssistantChatIntentK8sRollbackWorkload:
		return "K8s 工作负载回滚"
	case aiopsmodel.AssistantChatIntentKnowledgeSearch:
		return "运维知识检索"
	case aiopsmodel.AssistantChatIntentResourceGovernance:
		return "资源治理分析"
	case aiopsmodel.AssistantChatIntentClarify:
		return "补充信息确认"
	case aiopsmodel.AssistantChatIntentOpsGeneral, aiopsmodel.AssistantChatIntentGeneralQA:
		return "直接问答"
	default:
		return firstNonEmptyString(strings.TrimSpace(intent), "通用分析")
	}
}

func assistantChatActionDisplayName(result *aiopsmodel.AssistantChatResult) string {
	if result != nil && result.Intent == aiopsmodel.AssistantChatIntentK8sOverview {
		switch {
		case assistantChatBool(result.Entities["workloadDiagnosis"]):
			return "K8s 工作负载健康诊断"
		case assistantChatBool(result.Entities["workloadHistory"]):
			return "K8s 工作负载历史版本查询"
		case assistantChatBool(result.Entities["replicaCountQuery"]):
			return "K8s 工作负载副本查询"
		case assistantChatBool(result.Entities["workloadStatusQuery"]):
			return "K8s 工作负载状态查询"
		}
	}
	if result == nil {
		return "通用分析"
	}
	return assistantChatIntentDisplayName(result.Intent)
}

func refreshAssistantChatProgress(result *aiopsmodel.AssistantChatResult) {
	if result == nil {
		return
	}
	steps := assistantChatProgressSteps(result)
	if len(steps) == 0 {
		result.Progress = nil
		return
	}
	completed := 0
	failed := 0
	currentStep := 0
	for _, step := range steps {
		switch step.Status {
		case aiopsmodel.AssistantChatPlanSucceeded, aiopsmodel.AssistantChatPlanSkipped:
			completed++
		case aiopsmodel.AssistantChatPlanFailed:
			completed++
			failed++
		case aiopsmodel.AssistantChatPlanRunning:
			if currentStep == 0 {
				currentStep = step.Step
			}
		default:
			if currentStep == 0 {
				currentStep = step.Step
			}
		}
	}
	percent := completed * 100 / len(steps)
	if result.AnalysisTask != nil {
		for _, step := range steps {
			if (step.ToolName == "k8s_diagnose_workload" || step.ToolName == "k8s_diagnose_pod") && step.Status == aiopsmodel.AssistantChatPlanRunning {
				percent += step.Percent / len(steps)
				break
			}
		}
	}
	if percent > 100 {
		percent = 100
	}
	status := aiopsmodel.AssistantChatPlanRunning
	if failed > 0 {
		status = aiopsmodel.AssistantChatPlanFailed
	} else if completed == len(steps) {
		status = aiopsmodel.AssistantChatPlanSucceeded
		currentStep = len(steps)
	}
	result.Progress = &aiopsmodel.AssistantChatProgress{
		Percent:     percent,
		CurrentStep: currentStep,
		Status:      status,
		Steps:       steps,
	}
}

func finalizeAssistantChatProgress(result *aiopsmodel.AssistantChatResult, runStatus string) {
	if result == nil {
		return
	}
	finalStatus := aiopsmodel.AssistantChatPlanSucceeded
	if runStatus == aiopsmodel.AgentRunStatusFailed {
		finalStatus = aiopsmodel.AssistantChatPlanFailed
	}
	for _, item := range result.ToolPlan {
		switch item.Status {
		case aiopsmodel.AssistantChatPlanSucceeded, aiopsmodel.AssistantChatPlanSkipped, aiopsmodel.AssistantChatPlanFailed:
			continue
		default:
			markAssistantChatPlan(result, item.ToolName, finalStatus, nil, item.Error)
		}
	}
	refreshAssistantChatProgress(result)
}

func assistantChatProgressSteps(result *aiopsmodel.AssistantChatResult) []aiopsmodel.AssistantChatProgressStep {
	steps := make([]aiopsmodel.AssistantChatProgressStep, 0, len(result.ToolPlan))
	for _, item := range result.ToolPlan {
		status := item.Status
		if status == "" {
			status = aiopsmodel.AssistantChatPlanPending
		}
		startedAt, finishedAt, durationMS := assistantChatStepTiming(result, item.ToolName)
		stepPercent := assistantChatStepPercent(status)
		if result.AnalysisTask != nil && (item.ToolName == "k8s_diagnose_workload" || item.ToolName == "k8s_diagnose_pod") && status == aiopsmodel.AssistantChatPlanRunning {
			stepPercent = result.AnalysisTask.Progress
		}
		steps = append(steps, aiopsmodel.AssistantChatProgressStep{
			Step:       item.Step,
			Key:        item.ToolName,
			Name:       assistantChatProgressStepName(item.ToolName),
			ToolName:   item.ToolName,
			Purpose:    item.Purpose,
			Status:     status,
			Percent:    stepPercent,
			Required:   status != aiopsmodel.AssistantChatPlanSkipped,
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
			DurationMS: durationMS,
			Error:      item.Error,
		})
	}
	return steps
}

func assistantChatStepTiming(result *aiopsmodel.AssistantChatResult, toolName string) (string, string, int64) {
	for i := len(result.Timeline) - 1; i >= 0; i-- {
		item := result.Timeline[i]
		if item.ToolName == toolName {
			return item.StartedAt, item.FinishedAt, item.DurationMS
		}
	}
	return "", "", 0
}

func assistantChatStepPercent(status string) int {
	switch status {
	case aiopsmodel.AssistantChatPlanSucceeded, aiopsmodel.AssistantChatPlanSkipped:
		return 100
	case aiopsmodel.AssistantChatPlanRunning:
		return 50
	case aiopsmodel.AssistantChatPlanFailed:
		return 100
	default:
		return 0
	}
}

func assistantChatPlanStep(result *aiopsmodel.AssistantChatResult, toolName string) int {
	if result == nil {
		return 0
	}
	for _, item := range result.ToolPlan {
		if item.ToolName == toolName {
			return item.Step
		}
	}
	return len(result.Timeline) + 1
}

func assistantChatToolPurpose(toolName string) string {
	switch toolName {
	case "understand_requirement":
		return "理解需求"
	case "ecs_list_hosts":
		return "读取主机资产"
	case "mcp_catalog":
		return "读取 MCP 能力目录"
	case "confirm_pending_action":
		return "确认并执行待处理操作"
	case "resource_scope":
		return "确定资源治理范围"
	case "get_cmdb_host":
		return "获取主机资产上下文"
	case "search_cmdb_hosts":
		return "搜索 CMDB 主机"
	case "get_agent_status":
		return "查询采集 Agent 状态"
	case "query_host_metrics":
		return "查询主机资源指标"
	case "ecs_inspect_path":
		return "检查目录内容或读取文本文件末尾"
	case "ecs_analyze_disk_usage":
		return "分析磁盘目录和大文件占用"
	case "ecs_request_cleanup_logs":
		return "生成日志清空预览和确认凭证"
	case "query_host_services":
		return "查询主机服务和 Top 进程"
	case "query_active_alerts":
		return "查询活跃告警"
	case "aiops_context_hosts":
		return "读取主机资产上下文"
	case "aiops_context_database":
		return "读取数据库资产上下文"
	case "aiops_context_k8s":
		return "读取 K8s 集群上下文"
	case "k8s_list_clusters":
		return "确定授权 K8s 集群范围"
	case "k8s_check_cluster":
		return "实时检查 K8s 集群"
	case "k8s_list_namespaces":
		return "读取 K8s Namespace"
	case "k8s_list_nodes", "k8s_get_node_status":
		return "检查 K8s 节点"
	case "k8s_search_resources", "k8s_get_resource":
		return "查询 K8s 资源"
	case "k8s_get_workload_status":
		return "检查 K8s 工作负载"
	case "k8s_get_workload_history":
		return "查询 K8s 工作负载历史版本"
	case "k8s_request_rollout_restart":
		return "生成 K8s 滚动重启预览"
	case "k8s_request_scale_workload":
		return "生成 K8s 扩缩容预览"
	case "k8s_request_rollback_workload":
		return "生成 K8s 回滚预览"
	case "k8s_get_pod_status":
		return "检查 K8s Pod"
	case "k8s_get_service_endpoints":
		return "检查 K8s Service 链路"
	case "k8s_query_events":
		return "查询 K8s 事件"
	case "k8s_query_logs":
		return "查询 K8s 日志"
	case "k8s_query_metrics":
		return "查询 K8s 指标"
	case "k8s_diagnose_workload", "k8s_diagnose_pod", "k8s_diagnose_ingress":
		return "执行 K8s 只读诊断"
	case "knowledge_search":
		return "检索运维知识库"
	case "assistant_query":
		return "调用资源治理工具"
	case "resource_metrics":
		return "整理资源指标"
	case "ai_summary":
		return "AI 总结评分"
	case "direct_answer":
		return "生成回答"
	default:
		return "调用工具"
	}
}

func assistantChatProgressStepName(toolName string) string {
	switch toolName {
	case "understand_requirement":
		return "理解需求"
	case "confirm_pending_action":
		return "确认并执行"
	case "resource_scope":
		return "确定范围"
	case "assistant_query":
		return "调用资源治理工具"
	case "resource_metrics":
		return "获取监控数据"
	case "ecs_inspect_path":
		return "检查主机路径"
	case "ecs_analyze_disk_usage":
		return "分析磁盘占用"
	case "ecs_request_cleanup_logs":
		return "预览日志清空"
	case "query_active_alerts":
		return "读取告警"
	case "aiops_context_hosts":
		return "读取主机"
	case "aiops_context_database":
		return "读取数据库"
	case "aiops_context_k8s":
		return "读取 K8S"
	case "k8s_list_clusters":
		return "确定集群"
	case "k8s_check_cluster":
		return "检查集群"
	case "k8s_list_namespaces":
		return "读取 Namespace"
	case "k8s_list_nodes", "k8s_get_node_status":
		return "检查节点"
	case "k8s_search_resources", "k8s_get_resource":
		return "查询资源"
	case "k8s_get_workload_status", "k8s_diagnose_workload":
		return "检查工作负载"
	case "k8s_get_workload_history":
		return "读取历史版本"
	case "k8s_request_rollout_restart":
		return "预览滚动重启"
	case "k8s_request_scale_workload":
		return "预览扩缩容"
	case "k8s_request_rollback_workload":
		return "预览回滚"
	case "k8s_get_pod_status", "k8s_diagnose_pod":
		return "检查 Pod"
	case "k8s_get_service_endpoints", "k8s_diagnose_ingress":
		return "检查流量链路"
	case "k8s_query_events":
		return "查询事件"
	case "k8s_query_logs":
		return "查询日志"
	case "k8s_query_metrics":
		return "查询指标"
	case "knowledge_search":
		return "检索知识库"
	case "get_cmdb_host", "search_cmdb_hosts":
		return "定位主机"
	case "get_agent_status":
		return "确认 Agent"
	case "query_host_metrics":
		return "获取监控数据"
	case "query_host_services":
		return "分析服务"
	case "ai_summary":
		return "AI 总结评分"
	case "direct_answer", "llm_ops_general", "llm_general_qa":
		return "生成回答"
	default:
		return assistantChatToolPurpose(toolName)
	}
}

func assistantChatMap(value interface{}) map[string]interface{} {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]interface{}); ok {
		return typed
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

func assistantChatSlice(value interface{}) []interface{} {
	if value == nil {
		return nil
	}
	if typed, ok := value.([]interface{}); ok {
		return typed
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var result []interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

func assistantChatStringSlice(value interface{}) []string {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []interface{}:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprintf("%v", item))
			if text != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		items := assistantChatSlice(value)
		result := make([]string, 0, len(items))
		for _, item := range items {
			text := strings.TrimSpace(fmt.Sprintf("%v", item))
			if text != "" {
				result = append(result, text)
			}
		}
		return result
	}
}

func assistantChatUintSlice(value interface{}) []uint {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []uint:
		return typed
	case []interface{}:
		result := make([]uint, 0, len(typed))
		for _, item := range typed {
			if number := uint(assistantChatNumber(item)); number > 0 {
				result = append(result, number)
			}
		}
		return result
	default:
		items := assistantChatSlice(value)
		result := make([]uint, 0, len(items))
		for _, item := range items {
			if number := uint(assistantChatNumber(item)); number > 0 {
				result = append(result, number)
			}
		}
		return result
	}
}

func formatAssistantChatUintSlice(items []uint) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("%d", item))
	}
	return strings.Join(parts, ",")
}

func assistantChatNumber(value interface{}) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case uint:
		return float64(typed)
	case uint64:
		return float64(typed)
	case float64:
		return typed
	case float32:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	case string:
		parsed, _ := json.Number(strings.TrimSpace(typed)).Float64()
		return parsed
	default:
		return 0
	}
}

func assistantChatBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true") || typed == "1" || typed == "是"
	case int:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

func assistantChatDatabaseTypeName(dbType int) string {
	switch dbType {
	case cmdbmodel.DatabaseTypeMySQL:
		return "MySQL"
	case cmdbmodel.DatabaseTypePostgreSQL:
		return "PostgreSQL"
	case cmdbmodel.DatabaseTypeOracle:
		return "Oracle"
	case cmdbmodel.DatabaseTypeRedis:
		return "Redis"
	case cmdbmodel.DatabaseTypeMongoDB:
		return "MongoDB"
	case cmdbmodel.DatabaseTypeSQLServer:
		return "SQL Server"
	case cmdbmodel.DatabaseTypeElasticsearch:
		return "Elasticsearch"
	default:
		return fmt.Sprintf("未知类型(%d)", dbType)
	}
}

func assistantChatDatabaseStatusName(status int) string {
	switch status {
	case cmdbmodel.DatabaseStatusRunning:
		return "运行中"
	case cmdbmodel.DatabaseStatusStopped:
		return "已停止"
	case cmdbmodel.DatabaseStatusError:
		return "异常"
	case cmdbmodel.DatabaseStatusOffline:
		return "离线"
	default:
		return fmt.Sprintf("未知(%d)", status)
	}
}

func assistantChatDatabaseAuthStatusName(status int) string {
	switch status {
	case cmdbmodel.DatabaseAuthStatusUnauthenticated:
		return "未认证"
	case cmdbmodel.DatabaseAuthStatusFailed:
		return "认证失败"
	case cmdbmodel.DatabaseAuthStatusSuccess:
		return "认证成功"
	default:
		return fmt.Sprintf("未知(%d)", status)
	}
}

type assistantDBPressureMetric struct {
	Key    string
	Name   string
	Unit   string
	Value  float64
	Status string
}

type assistantDBPressureSnapshot struct {
	Selector string
	Metrics  []assistantDBPressureMetric
	Warnings []string
}

func (s assistantDBPressureSnapshot) Abnormal() []assistantDBPressureMetric {
	result := []assistantDBPressureMetric{}
	for _, metric := range s.Metrics {
		status := strings.ToLower(strings.TrimSpace(metric.Status))
		if status != "" && status != "normal" && status != "ok" && status != "success" {
			result = append(result, metric)
		}
	}
	return result
}

func (s assistantDBPressureSnapshot) Summary() string {
	parts := make([]string, 0, len(s.Metrics))
	for _, metric := range s.Metrics {
		parts = append(parts, fmt.Sprintf("%s=%s%s", metric.Name, formatAssistantMetricValue(metric.Value), metric.Unit))
	}
	return strings.Join(parts, "，")
}

type assistantMySQLMetricDef struct {
	Key    string
	Name   string
	Unit   string
	Query  string
	Status func(float64) string
}

func queryAssistantMySQLPressureSnapshot(ctx context.Context, accountID uint, selector string) (*assistantDBPressureSnapshot, error) {
	return queryAssistantDatabasePressureSnapshot(ctx, accountID, selector, "mysql")
}

func queryAssistantDatabasePressureSnapshot(ctx context.Context, accountID uint, selector, typ string) (*assistantDBPressureSnapshot, error) {
	account, err := accountdao.NewAccountAuthDao().GetByID(accountID)
	if err != nil {
		return nil, fmt.Errorf("获取 Prometheus 账号失败: %w", err)
	}
	password, err := account.DecryptPassword()
	if err != nil {
		return nil, fmt.Errorf("解密 Prometheus 账号失败: %w", err)
	}
	baseURL := buildAssistantPrometheusURL(account.Host, account.Port)
	client := &http.Client{Timeout: 8 * time.Second}
	selector = strings.TrimSpace(selector)
	defs := assistantDatabasePressureMetricDefs(typ, selector)
	if len(defs) == 0 {
		return nil, fmt.Errorf("数据库类型 %s 暂不支持压力监控判断", typ)
	}
	snapshot := &assistantDBPressureSnapshot{Selector: selector}
	for _, def := range defs {
		value, queryErr := assistantPromQueryValue(ctx, client, baseURL, account.Name, password, def.Query)
		if queryErr != nil {
			snapshot.Warnings = append(snapshot.Warnings, def.Name+" 查询失败："+queryErr.Error())
			continue
		}
		status := "normal"
		if def.Status != nil {
			status = def.Status(value)
		}
		snapshot.Metrics = append(snapshot.Metrics, assistantDBPressureMetric{
			Key: def.Key, Name: def.Name, Unit: def.Unit, Value: value, Status: status,
		})
	}
	if len(snapshot.Metrics) == 0 {
		return snapshot, errors.New("没有查询到数据库监控指标")
	}
	return snapshot, nil
}

func assistantDatabasePressureMetricDefs(typ, selector string) []assistantMySQLMetricDef {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "mysql":
		return []assistantMySQLMetricDef{
			{Key: "up", Name: "MySQL 状态", Query: assistantPromMax("mysql_up", selector), Status: assistantStatusUp},
			{Key: "threadsConnected", Name: "当前连接数", Unit: "个", Query: assistantPromSum("mysql_global_status_threads_connected", selector), Status: assistantStatusNormal},
			{Key: "threadsRunning", Name: "运行线程", Unit: "个", Query: assistantPromSum("mysql_global_status_threads_running", selector), Status: assistantStatusNormal},
			{Key: "connectionsUsage", Name: "连接使用率", Unit: "%", Query: assistantPromBinary("mysql_global_status_threads_connected", "mysql_global_variables_max_connections", selector, "* 100"), Status: assistantStatusPercent},
			{Key: "qps", Name: "QPS", Unit: "次/s", Query: assistantPromSumRate("mysql_global_status_questions", selector), Status: assistantStatusNormal},
			{Key: "tps", Name: "TPS", Unit: "次/s", Query: assistantPromTPS(selector), Status: assistantStatusNormal},
			{Key: "slowQueries", Name: "慢查询速率", Unit: "次/s", Query: assistantPromSumRate("mysql_global_status_slow_queries", selector), Status: assistantStatusWarnAboveZero},
			{Key: "selectScan", Name: "全表扫描速率", Unit: "次/s", Query: assistantPromSumRate("mysql_global_status_select_scan", selector), Status: assistantStatusWarnAboveZero},
			{Key: "tmpDiskTables", Name: "磁盘临时表速率", Unit: "次/s", Query: assistantPromSumRate("mysql_global_status_created_tmp_disk_tables", selector), Status: assistantStatusWarnAboveZero},
			{Key: "bufferPoolHitRate", Name: "Buffer Pool 命中率", Unit: "%", Query: assistantPromBufferPoolHitRate(selector), Status: assistantStatusHighPercent},
			{Key: "rowLockWaits", Name: "行锁等待速率", Unit: "次/s", Query: assistantPromSumRate("mysql_global_status_innodb_row_lock_waits", selector), Status: assistantStatusWarnAboveZero},
		}
	case "redis":
		return []assistantMySQLMetricDef{
			{Key: "up", Name: "Redis 状态", Query: assistantPromMax("redis_up", selector), Status: assistantStatusUp},
			{Key: "connectedClients", Name: "客户端连接数", Unit: "个", Query: assistantPromSum("redis_connected_clients", selector), Status: assistantStatusNormal},
			{Key: "blockedClients", Name: "阻塞客户端", Unit: "个", Query: assistantPromSum("redis_blocked_clients", selector), Status: assistantStatusWarnAboveZero},
			{Key: "qps", Name: "QPS", Unit: "次/s", Query: assistantPromSumRate("redis_commands_processed_total", selector), Status: assistantStatusNormal},
			{Key: "hitRate", Name: "命中率", Unit: "%", Query: assistantPromRedisHitRate(selector), Status: assistantStatusHighPercent},
			{Key: "evictedKeys", Name: "淘汰 Key 速率", Unit: "个/s", Query: assistantPromSumRate("redis_evicted_keys_total", selector), Status: assistantStatusWarnAboveZero},
			{Key: "errorReplies", Name: "错误响应速率", Unit: "个/s", Query: assistantPromSumRate("redis_total_error_replies", selector), Status: assistantStatusWarnAboveZero},
		}
	default:
		return nil
	}
}

func buildAssistantPrometheusURL(host string, port int) string {
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return strings.TrimRight(host, "/")
	}
	if port > 0 {
		return fmt.Sprintf("http://%s:%d", host, port)
	}
	return "http://" + host
}

type assistantPromQueryResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		Result []struct {
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func assistantPromQueryValue(ctx context.Context, client *http.Client, baseURL, username, password, query string) (float64, error) {
	values := url.Values{"query": []string{query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/v1/query?"+values.Encode(), nil)
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(username) != "" || strings.TrimSpace(password) != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var parsed assistantPromQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, err
	}
	if parsed.Status != "success" {
		return 0, errors.New(parsed.Error)
	}
	if len(parsed.Data.Result) == 0 || len(parsed.Data.Result[0].Value) < 2 {
		return 0, errors.New("无数据")
	}
	valueText := fmt.Sprintf("%v", parsed.Data.Result[0].Value[1])
	value, err := strconv.ParseFloat(valueText, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("无效数值 %q", valueText)
	}
	return value, nil
}

func assistantPromMetric(metric, selector string) string {
	if strings.TrimSpace(selector) == "" {
		return metric
	}
	return metric + "{" + selector + "}"
}

func assistantPromSum(metric, selector string) string {
	return "sum(" + assistantPromMetric(metric, selector) + ")"
}

func assistantPromMax(metric, selector string) string {
	return "max(" + assistantPromMetric(metric, selector) + ")"
}

func assistantPromSumRate(metric, selector string) string {
	return "sum(rate(" + assistantPromMetric(metric, selector) + "[5m]))"
}

func assistantPromBinary(left, right, selector, suffix string) string {
	return "(sum(" + assistantPromMetric(left, selector) + ") / clamp_min(sum(" + assistantPromMetric(right, selector) + "), 1)) " + suffix
}

func assistantPromCommandRate(command, selector string) string {
	extra := `command="` + command + `"`
	if strings.TrimSpace(selector) != "" {
		extra = selector + "," + extra
	}
	return `rate(mysql_global_status_commands_total{` + extra + `}[5m])`
}

func assistantPromTPS(selector string) string {
	return "sum(" + assistantPromCommandRate("commit", selector) + ") + sum(" + assistantPromCommandRate("rollback", selector) + ")"
}

func assistantPromBufferPoolHitRate(selector string) string {
	readMisses := assistantPromSumRate("mysql_global_status_innodb_buffer_pool_reads", selector)
	readRequests := assistantPromSumRate("mysql_global_status_innodb_buffer_pool_read_requests", selector)
	return "(1 - (" + readMisses + " / clamp_min(" + readRequests + ", 1))) * 100"
}

func assistantPromRedisHitRate(selector string) string {
	hits := assistantPromSumRate("redis_keyspace_hits_total", selector)
	misses := assistantPromSumRate("redis_keyspace_misses_total", selector)
	return "(" + hits + " / clamp_min((" + hits + " + " + misses + "), 1)) * 100"
}

func assistantDatabaseTypeSupportsPressureSnapshot(typ string) bool {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "mysql", "redis":
		return true
	default:
		return false
	}
}

func assistantStatusUp(value float64) string {
	if value >= 1 {
		return "normal"
	}
	return "critical"
}

func assistantStatusPercent(value float64) string {
	switch {
	case value >= 90:
		return "critical"
	case value >= 80:
		return "warning"
	default:
		return "normal"
	}
}

func assistantStatusHighPercent(value float64) string {
	switch {
	case value < 95:
		return "critical"
	case value < 99:
		return "warning"
	default:
		return "normal"
	}
}

func assistantStatusWarnAboveZero(value float64) string {
	if value > 0 {
		return "warning"
	}
	return "normal"
}

func assistantStatusNormal(float64) string {
	return "normal"
}

func formatAssistantMetricValue(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
}

func assistantChatK8sClusterTypeName(clusterType int) string {
	switch clusterType {
	case k8smodel.ClusterTypeSelfBuilt:
		return "自建"
	case k8smodel.ClusterTypeImported:
		return "导入"
	default:
		return fmt.Sprintf("未知(%d)", clusterType)
	}
}

func assistantChatHostTypeName(hostType int) string {
	switch hostType {
	case cmdbmodel.HostTypeLinux:
		return "Linux"
	case cmdbmodel.HostTypeWindows:
		return "Windows"
	default:
		return fmt.Sprintf("未知(%d)", hostType)
	}
}

func assistantChatHostStatusName(status int) string {
	switch status {
	case 1:
		return "认证成功"
	case 2:
		return "未认证"
	case 3:
		return "认证失败"
	default:
		return fmt.Sprintf("未知(%d)", status)
	}
}

func assistantChatFormatTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func assistantChatContextString(context map[string]interface{}, keys ...string) string {
	if len(context) == 0 {
		return ""
	}
	for _, key := range keys {
		value, ok := context[key]
		if !ok || value == nil {
			continue
		}
		if text := strings.TrimSpace(fmt.Sprintf("%v", value)); text != "" {
			return text
		}
	}
	return ""
}

func assistantChatContextUInt(context map[string]interface{}, key string) uint {
	if len(context) == 0 {
		return 0
	}
	return uint(assistantChatNumber(context[key]))
}

func firstNonEmptyAssistantChatContextString(context map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := assistantChatContextString(context, key); value != "" {
			return value
		}
	}
	return ""
}

func assistantChatFirstContextUInt(context map[string]interface{}, keys ...string) uint {
	for _, key := range keys {
		if value := assistantChatContextUInt(context, key); value > 0 {
			return value
		}
	}
	return 0
}

func assistantChatFirstEntityUInt(entities map[string]interface{}, keys ...string) uint {
	for _, key := range keys {
		if value := uint(assistantChatNumber(entities[key])); value > 0 {
			return value
		}
	}
	return 0
}

func firstNonEmptyAssistantChatString(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(fmt.Sprintf("%v", data[key]))
		if value != "" && value != "<nil>" && value != "0" {
			return value
		}
	}
	return ""
}

func firstNonEmptyAssistantChatText(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func assistantChatContextPageMatches(pageType string, pages []string) bool {
	pageType = strings.ToLower(strings.TrimSpace(pageType))
	if pageType == "" {
		return false
	}
	for _, page := range pages {
		if pageType == strings.ToLower(page) {
			return true
		}
	}
	return false
}

func assistantChatQuestionLooksPageScoped(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	return containsAnyAssistantChatKeyword(question, []string{
		"这个", "这台", "当前", "本页", "这里", "它", "看下", "看看", "检查", "分析", "状态", "详情",
		"this", "current", "detail",
	})
}

func assistantChatQuestionLooksLikePlatformOverview(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	if assistantChatQuestionLooksLikeCareerSkillQA(question) {
		return false
	}
	if containsAnyAssistantChatKeyword(question, []string{
		"数据库", "db", "database", "mysql", "postgres", "redis",
		"k8s", "kubernetes", "pod", "namespace", "集群",
		"知识库", "查文档", "处理方案", "runbook", "sop",
		"脚本", "安装", "部署", "命令", "怎么写",
	}) && !containsAnyAssistantChatKeyword(question, []string{"平台", "整体", "全局", "总览"}) {
		return false
	}
	if containsAnyAssistantChatKeyword(question, []string{
		"平台整体", "全局", "总览", "整体风险", "平台风险", "系统健康", "运维情况", "整体情况",
		"当前平台", "现在平台", "平台有什么风险", "平台有哪些异常",
		"platform", "overall", "overview",
	}) {
		return true
	}
	hasBroadSubject := containsAnyAssistantChatKeyword(question, []string{
		"平台", "整体", "系统", "当前", "现在", "今天", "异常", "风险", "健康", "故障",
		"health", "risk",
	})
	hasBroadAsk := containsAnyAssistantChatKeyword(question, []string{
		"看下", "看看", "分析", "有什么", "有哪些", "需要关注", "怎么样", "如何", "是否", "有没有",
		"check", "analyze",
	})
	if hasBroadSubject && hasBroadAsk && containsAnyAssistantChatKeyword(question, []string{"异常", "风险", "健康", "故障", "整体", "平台", "系统"}) {
		return true
	}
	return false
}

func assistantChatQuestionLooksLikeHostInventory(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	hasHost := containsAnyAssistantChatKeyword(question, []string{"主机", "机器", "服务器", "host", "hosts", "server", "servers"})
	hasCount := containsAnyAssistantChatKeyword(question, []string{
		"多少", "几台", "数量", "总数", "资产概览", "资产总览", "how many", "count", "total",
	})
	return hasHost && hasCount
}

func assistantChatQuestionLooksLikeHostServices(question string, hasHost bool) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" || !hasHost {
		return false
	}
	hasServiceWord := containsAnyAssistantChatKeyword(question, []string{
		"服务", "部署", "进程", "端口", "应用", "运行", "跑了", "top进程", "top 进程",
		"service", "services", "process", "processes", "port", "application",
	})
	hasQueryWord := containsAnyAssistantChatKeyword(question, []string{
		"什么", "哪些", "哪个", "列表", "上面", "有什么", "部署了", "运行了", "跑了", "查看", "看下", "看看",
		"what", "which", "list",
	})
	return hasServiceWord && hasQueryWord
}

func assistantChatQuestionLooksLikeHostPressure(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	if assistantChatQuestionLooksLikeResourceGovernance(question) {
		return false
	}
	return containsAnyAssistantChatKeyword(question, []string{
		"压力", "负载", "cpu", "内存", "磁盘", "主机", "机器", "服务器", "健康", "状态",
		"pressure", "load", "host", "server", "health",
	})
}

func assistantChatQuestionLooksLikeDiskUsageAnalysis(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	hasDiskTarget := containsAnyAssistantChatKeyword(question, []string{
		"目录", "文件", "磁盘空间", "空间占用", "占用空间", "directory", "directories", "file", "disk usage",
	})
	hasRankingAsk := containsAnyAssistantChatKeyword(question, []string{
		"最大", "最多", "最占", "占用", "排行", "排名", "top", "largest", "biggest",
	})
	return hasDiskTarget && hasRankingAsk
}

func assistantChatQuestionLooksLikePathInspection(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if assistantChatQuestionLinuxPath(question) == "" || assistantChatQuestionLooksLikeDiskUsageAnalysis(question) {
		return false
	}
	return containsAnyAssistantChatKeyword(question, []string{
		"下面", "目录内容", "目录列表", "文件列表", "有哪些文件", "有啥文件", "查看文件", "看看文件",
		"文件内容", "配置内容", "配置文件", "配置", "最后500行", "最后 500 行", "查看", "看下", "看看",
		"读取", "打开", "内容", "tail", "list", "content", "read", "show",
	})
}

func assistantChatQuestionLooksLikeLogCleanup(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	target := assistantChatQuestionLinuxPath(question)
	if target == "" {
		return false
	}
	extension := strings.ToLower(pathpkg.Ext(target))
	isLogTarget := extension == ".log" || extension == ".txt" ||
		strings.Contains(strings.ToLower(target), "/log") || strings.Contains(question, "日志")
	if !isLogTarget {
		return false
	}
	return containsAnyAssistantChatKeyword(question, []string{
		"清空", "清理", "删除", "截断", "置空", "truncate", "clear", "empty", "remove", "delete",
	})
}

func assistantChatQuestionLooksLikeUnsupportedFileOperation(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	target := assistantChatQuestionLinuxPath(question)
	if target == "" || !containsAnyAssistantChatKeyword(question, []string{
		"删除", "清理", "清空", "remove", "delete", "clear", "truncate",
	}) {
		return false
	}
	extension := strings.ToLower(pathpkg.Ext(target))
	return extension != ".log" && extension != ".txt"
}

func assistantChatQuestionLooksLikeHostPressureWithoutTarget(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	if assistantChatQuestionLooksLikeOpsGeneral(question) ||
		assistantChatQuestionLooksLikeResourceGovernance(question) ||
		assistantChatQuestionLooksLikeGeneralQA(question) {
		return false
	}
	return containsAnyAssistantChatKeyword(question, []string{
		"压力", "负载", "cpu", "内存", "磁盘", "使用率", "卡顿", "很慢", "异常", "健康", "状态",
		"pressure", "load", "memory", "disk", "health",
	})
}

func assistantChatQuestionLooksLikeResourceGovernance(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	if assistantChatQuestionLooksLikeK8sScaleWorkload(question, "") {
		return false
	}
	if containsAnyAssistantChatKeyword(question, []string{
		"回收", "可回收", "释放", "闲置", "选址", "部署", "上线", "扩容", "容量", "巡检", "资产",
		"长期低负载", "低负载", "测试机器", "测试机",
		"recycle", "release", "idle", "capacity", "governance",
	}) {
		return true
	}
	hasMachine := containsAnyAssistantChatKeyword(question, []string{
		"机器", "主机", "服务器", "资源", "资产", "host", "server",
	})
	if hasMachine && detectAssistantQueryRanking(question, 5) != nil {
		return true
	}
	hasPluralTarget := containsAnyAssistantChatKeyword(question, []string{
		"哪些", "那些", "哪几", "几台", "一批", "列表", "排行", "排名", "top", "比较",
		"前一", "前二", "前两", "前三", "前四", "前五", "前六", "前七", "前八", "前九", "前十",
	})
	hasMetric := containsAnyAssistantChatKeyword(question, []string{
		"压力", "负载", "cpu", "内存", "磁盘", "容量", "使用率", "load", "memory", "disk",
	})
	return hasPluralTarget && hasMachine && hasMetric
}

func assistantChatQuestionLooksLikeDatabaseOverview(question, pageType string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	pageType = strings.ToLower(strings.TrimSpace(pageType))
	if question == "" {
		return false
	}
	if assistantChatContextPageMatches(pageType, []string{"db", "database", "db_asset", "database_asset", "db_detail", "database_detail", "slow_log", "sql_review"}) &&
		assistantChatQuestionLooksPageScoped(question) {
		return true
	}
	hasDatabaseTopic := containsAnyAssistantChatKeyword(question, []string{
		"数据库", "数据库资产", "库实例", "实例", "db", "database", "mysql", "postgres", "postgresql", "redis",
		"慢sql", "慢查询", "sql审核",
	})
	if assistantChatQuestionDatabaseName(question) != "" {
		hasDatabaseTopic = true
	}
	hasPlatformQuery := containsAnyAssistantChatKeyword(question, []string{
		"状态", "情况", "概览", "列表", "有哪些", "多少", "健康", "详情", "资产", "监控", "负责人",
		"压力", "负载", "异常", "风险", "是否", "有没有", "高", "慢", "卡",
		"status", "overview", "list", "health", "pressure", "load", "risk",
	})
	if hasDatabaseTopic && hasPlatformQuery {
		return true
	}
	return false
}

func assistantChatQuestionLooksLikeK8sOverview(question, pageType string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	pageType = strings.ToLower(strings.TrimSpace(pageType))
	if question == "" {
		return false
	}
	if assistantChatQuestionAsksK8sWorkloadHistory(question) {
		return true
	}
	if assistantChatContextPageMatches(pageType, []string{"k8s", "kubernetes", "k8s_cluster", "k8s_cluster_detail", "k8s_pod", "k8s_workload", "k8s_namespace", "k8s_ingress", "k8s_service"}) &&
		assistantChatQuestionLooksPageScoped(question) {
		return true
	}
	hasK8sTopic := containsAnyAssistantChatKeyword(question, []string{
		"k8s", "kubernetes", "容器", "集群", "pod", "namespace", "命名空间", "deployment",
		"工作负载", "ingress", "service", "节点", "node",
	})
	hasPlatformQuery := containsAnyAssistantChatKeyword(question, []string{
		"状态", "情况", "概览", "列表", "有哪些", "多少", "健康", "详情", "事件", "异常",
		"检查", "查看", "看看", "几个", "副本", "巡检", "诊断", "排查", "问题", "有没有", "正常",
		"status", "overview", "list", "health", "event", "check", "diagnose", "problem",
	})
	if hasK8sTopic && hasPlatformQuery {
		return true
	}
	return false
}

func assistantChatQuestionLooksLikeK8sRolloutRestart(question, pageType string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	hasRestart := containsAnyAssistantChatKeyword(question, []string{"滚动重启", "重启", "restart", "rollout restart"})
	hasWorkload := containsAnyAssistantChatKeyword(question, []string{
		"k8s", "kubernetes", "工作负载", "workload", "deployment", "statefulset", "daemonset",
	})
	if hasRestart && hasWorkload {
		return true
	}
	return hasRestart && assistantChatContextPageMatches(strings.ToLower(strings.TrimSpace(pageType)), []string{
		"k8s", "kubernetes", "k8s_cluster", "k8s_cluster_detail", "k8s_workload", "k8s_namespace",
	})
}

func assistantChatQuestionLooksLikeK8sRollbackWorkload(question, pageType string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	hasRollback := containsAnyAssistantChatKeyword(question, []string{"回滚", "rollback", "roll back"})
	hasWorkload := containsAnyAssistantChatKeyword(question, []string{
		"k8s", "kubernetes", "工作负载", "workload", "deployment", "版本号", "revision", "镜像版本",
	})
	if hasRollback && hasWorkload {
		return true
	}
	return hasRollback && assistantChatContextPageMatches(strings.ToLower(strings.TrimSpace(pageType)), []string{
		"k8s", "kubernetes", "k8s_cluster", "k8s_cluster_detail", "k8s_workload", "k8s_namespace",
	})
}

func assistantChatQuestionLooksLikeK8sScaleWorkload(question, pageType string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	hasScale := containsAnyAssistantChatKeyword(question, []string{
		"扩容", "缩容", "扩缩容", "伸缩", "调整副本", "增加副本", "新增副本", "减少副本", "加一个副本", "减一个副本", "scale",
	})
	if _, ok := assistantChatQuestionK8sReplicaDelta(question); ok {
		hasScale = true
	}
	hasWorkload := containsAnyAssistantChatKeyword(question, []string{
		"工作负载", "workload", "deployment", "副本", "replica",
	})
	if hasScale && hasWorkload {
		return true
	}
	return hasScale && assistantChatContextPageMatches(strings.ToLower(strings.TrimSpace(pageType)), []string{
		"k8s", "kubernetes", "k8s_cluster", "k8s_cluster_detail", "k8s_workload", "k8s_namespace",
	})
}

func assistantChatQuestionLooksLikeK8sTargetSelectionFollowup(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" || !containsAnyAssistantChatKeyword(question, []string{"选择", "选中", "select"}) {
		return false
	}
	return !assistantChatQuestionLooksLikeK8sScaleWorkload(question, "") &&
		!assistantChatQuestionLooksLikeK8sRolloutRestart(question, "") &&
		!assistantChatQuestionLooksLikeK8sRollbackWorkload(question, "")
}

func assistantChatQuestionAsksK8sReplicaCount(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" || assistantChatQuestionLooksLikeK8sScaleWorkload(question, "") {
		return false
	}
	hasReplica := containsAnyAssistantChatKeyword(question, []string{"副本", "replica"})
	hasQuery := containsAnyAssistantChatKeyword(question, []string{
		"查看", "看看", "查下", "查询", "现在", "当前", "几个", "多少", "状态", "有几", "how many", "current",
	})
	return hasReplica && hasQuery
}

func assistantChatQuestionAsksK8sWorkloadHistory(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	hasHistory := containsAnyAssistantChatKeyword(question, []string{
		"历史版本", "版本历史", "发布历史", "发布记录", "版本记录", "rollout history", "revision history", "deployment history",
	})
	hasWorkload := containsAnyAssistantChatKeyword(question, []string{
		"k8s", "kubernetes", "工作负载", "workload", "deployment",
	})
	return hasHistory && hasWorkload
}

func assistantChatQuestionAsksK8sWorkloadDiagnosis(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	hasDiagnosis := containsAnyAssistantChatKeyword(question, []string{
		"异常", "故障", "问题", "错误", "报错", "失败", "异常原因", "故障原因", "问题原因", "分析", "诊断", "排查", "根因", "为什么异常", "为什么失败",
		"是否正常", "是否健康", "正常吗", "健康吗", "运行正常",
		"diagnose", "diagnosis", "root cause", "troubleshoot", "error", "failed", "healthy", "running normally",
	})
	hasWorkload := containsAnyAssistantChatKeyword(question, []string{"工作负载", "workload", "deployment"})
	return hasDiagnosis && hasWorkload
}

func assistantChatQuestionAsksK8sWorkloadStatus(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" || assistantChatQuestionAsksK8sWorkloadDiagnosis(question) || assistantChatQuestionAsksK8sReplicaCount(question) {
		return false
	}
	hasWorkload := containsAnyAssistantChatKeyword(question, []string{"工作负载", "workload", "deployment"})
	hasStatus := containsAnyAssistantChatKeyword(question, []string{
		"状态", "运行情况", "运行得怎么样", "现在怎么样", "当前怎么样", "status",
	})
	return hasWorkload && hasStatus
}

func assistantChatQuestionLooksLikeK8sDomain(question, pageType string) bool {
	return assistantChatQuestionLooksLikeK8sOverview(question, pageType) ||
		assistantChatQuestionLooksLikeK8sRollbackWorkload(question, pageType) ||
		assistantChatQuestionLooksLikeK8sRolloutRestart(question, pageType) ||
		assistantChatQuestionLooksLikeK8sScaleWorkload(question, pageType)
}

func assistantChatQuestionLooksLikeKnowledgeSearch(question, pageType string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	pageType = strings.ToLower(strings.TrimSpace(pageType))
	if question == "" {
		return false
	}
	if assistantChatContextPageMatches(pageType, []string{"knowledge", "runbook", "document", "docs"}) &&
		assistantChatQuestionLooksPageScoped(question) {
		return true
	}
	return containsAnyAssistantChatKeyword(question, []string{
		"知识库", "查知识", "查文档", "文档里", "有没有文档", "有没有手册", "runbook", "故障案例", "故障复盘",
		"处理方案", "相关资料", "sop",
	})
}

func assistantChatQuestionLooksLikeSkillCatalog(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	if assistantChatQuestionLooksLikeCareerSkillQA(question) {
		return false
	}
	if containsAnyAssistantChatKeyword(question, []string{
		"支持哪些技能", "当前支持哪些技能", "查看一下当前支持哪些技能", "有哪些技能", "技能列表",
		"当前技能", "已加载技能", "加载了哪些技能", "启用了哪些技能", "安装并启用了哪些技能",
		"skill catalog", "skills list", "loaded skills", "available skills",
	}) {
		return true
	}
	hasSkillTerm := strings.Contains(question, "技能") || strings.Contains(question, "skill")
	hasCatalogTerm := containsAnyAssistantChatKeyword(question, []string{
		"当前", "系统", "平台", "autoops", "支持", "启用", "启用了", "加载", "已加载", "列表", "清单", "catalog",
	})
	return hasSkillTerm && hasCatalogTerm
}

func assistantChatQuestionLooksLikeMCPCatalog(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" || !strings.Contains(question, "mcp") {
		return false
	}
	return containsAnyAssistantChatKeyword(question, []string{
		"工具", "tool", "服务", "server", "接入", "启用", "可用", "支持", "实现", "多少", "几个", "数量", "列表", "清单", "能力",
	})
}

func assistantChatQuestionLooksLikeCareerSkillQA(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	return containsAnyAssistantChatKeyword(question, []string{
		"需要具备", "需要哪些", "要具备", "应该具备", "技能栈", "岗位要求", "职业发展", "学习路线",
		"开发需要", "平台开发", "运维开发", "人员能力", "能力要求",
	})
}

func assistantChatQuestionAsksDatabasePressure(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	return containsAnyAssistantChatKeyword(question, []string{
		"压力", "负载", "是否有压力", "有没有压力", "性能", "慢", "卡", "繁忙",
		"pressure", "load", "busy",
	})
}

func assistantChatQuestionNeedsKnowledge(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	return containsAnyAssistantChatKeyword(question, []string{
		"处理方案", "怎么处理", "怎么排查", "排查方案", "runbook", "sop", "知识库", "文档", "故障案例", "复盘",
		"nginx", "502", "mysql", "redis", "k8s", "pod", "慢sql", "慢查询",
	})
}

func assistantChatQuestionLooksLikeOpsGeneral(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	hasOpsTopic := containsAnyAssistantChatKeyword(question, []string{
		"linux", "shell", "bash", "脚本", "命令", "巡检脚本", "检查脚本", "排查脚本",
		"巡检", "排查", "nginx", "服务", "systemd", "docker", "k8s", "mysql", "redis",
		"端口", "防火墙", "日志", "配置", "部署", "安装", "sop", "runbook", "script", "command", "checklist",
	})
	hasActionVerb := containsAnyAssistantChatKeyword(question, []string{
		"帮我写", "写一个", "写个", "生成", "给我一个", "给个", "怎么写", "示例", "模板",
		"如何", "怎么", "怎样", "安装", "部署", "配置", "启动", "停止", "排查", "检查", "教程", "步骤",
		"generate", "write", "create", "how to",
	})
	if hasOpsTopic && hasActionVerb {
		return true
	}
	return containsAnyAssistantChatKeyword(question, []string{
		"linux 巡检脚本", "linux巡检脚本", "shell 脚本", "bash 脚本",
		"安装 nginx", "安装nginx", "nginx 服务", "nginx服务",
	})
}

func assistantChatQuestionLooksLikeGeneralQA(question string) bool {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return false
	}
	if assistantChatQuestionLooksLikeOpsGeneral(question) || assistantChatQuestionLooksLikeResourceGovernance(question) {
		return false
	}
	return containsAnyAssistantChatKeyword(question, []string{
		"是什么", "什么是", "是谁", "有哪些", "为什么", "介绍一下", "解释一下", "如何理解",
		"需要具备", "需要哪些", "要具备", "应该具备", "哪些技能", "什么技能", "技能栈",
		"学习路线", "怎么学习", "职业发展", "岗位要求", "能力要求",
		"四大名著", "what is", "who is", "explain",
	})
}

func assistantChatValueEmpty(value interface{}) bool {
	if value == nil {
		return true
	}
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		return text == "" || text == "<nil>" || text == "0"
	case int:
		return typed == 0
	case int64:
		return typed == 0
	case uint:
		return typed == 0
	case uint64:
		return typed == 0
	case float64:
		return typed == 0
	case float32:
		return typed == 0
	case json.Number:
		return assistantChatNumber(typed) == 0
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value)) == "" || strings.TrimSpace(fmt.Sprintf("%v", value)) == "<nil>"
	}
}

func assistantChatHasContextReference(question string) bool {
	text := strings.ToLower(strings.TrimSpace(question))
	if text == "" {
		return false
	}
	return containsAnyAssistantChatKeyword(text, []string{
		"它", "这台", "那台", "该主机", "这个主机", "这台机器", "那台机器",
		"这机器", "这主机", "刚才", "上面", "上一轮", "前面", "刚刚", "刚才那个", "上面那个",
		"这条告警", "这个告警", "该告警", "继续", "再看", "顺便",
		"这个工作负载", "该工作负载", "这个 deployment", "该 deployment", "这个deployment", "该deployment",
		"这个 pod", "该 pod", "这个pod", "该pod", "这个集群", "该集群", "这个命名空间", "该命名空间",
		"it", "that host", "this host", "that alert", "this alert", "this workload", "that workload", "this pod", "that pod",
	})
}

func formatAssistantChatTopProcesses(data map[string]interface{}, key, metricKey string) string {
	items := assistantChatSlice(data[key])
	if len(items) == 0 {
		return ""
	}
	limit := len(items)
	if limit > 3 {
		limit = 3
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		item := assistantChatMap(items[i])
		name := firstNonEmptyAssistantChatString(item, "name")
		if name == "" {
			name = "unknown"
		}
		parts = append(parts, fmt.Sprintf("%s(pid=%d, %.2f%%)", name, int(assistantChatNumber(item["pid"])), assistantChatNumber(item[metricKey])))
	}
	return strings.Join(parts, "；")
}

func containsAnyAssistantChatKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func decodeAssistantChatValue(value interface{}, target interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func truncateAssistantChatText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len([]rune(value)) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max]) + "..."
}

func assistantChatHostDisplay(host map[string]interface{}) string {
	value := firstNonEmptyAssistantChatString(host, "hostName", "name", "privateIp", "sshIp", "publicIp")
	if value == "" {
		return "未知主机"
	}
	return value
}

func assistantChatAlertResources(output interface{}) []string {
	data := assistantChatMap(output)
	items := assistantChatSlice(data["items"])
	resources := make([]string, 0, len(items))
	for i, item := range items {
		if i >= 5 {
			break
		}
		row := assistantChatMap(item)
		value := firstNonEmptyAssistantChatString(row, "instance", "host", "hostName", "alertName", "alertname", "name")
		if value != "" {
			resources = append(resources, value)
		}
	}
	return resources
}

func countAssistantK8sNotReadyClusters(output map[string]interface{}) int {
	count := 0
	for _, raw := range assistantChatSlice(output["items"]) {
		item := assistantChatMap(raw)
		nodeCount := int(assistantChatNumber(item["nodeCount"]))
		readyNodes := int(assistantChatNumber(item["readyNodes"]))
		if nodeCount > 0 && readyNodes < nodeCount {
			count++
		}
	}
	return count
}

func countAssistantDatabaseAbnormalAssets(output map[string]interface{}) int {
	count := 0
	for _, raw := range assistantChatSlice(output["items"]) {
		item := assistantChatMap(raw)
		status := firstNonEmptyAssistantChatString(item, "status")
		if status != "" && status != "运行中" {
			count++
		}
	}
	return count
}

func assistantChatPlatformK8sResources(output map[string]interface{}) []string {
	resources := []string{}
	for _, raw := range assistantChatSlice(output["items"]) {
		item := assistantChatMap(raw)
		name := firstNonEmptyAssistantChatString(item, "name")
		nodeCount := int(assistantChatNumber(item["nodeCount"]))
		readyNodes := int(assistantChatNumber(item["readyNodes"]))
		if name != "" && nodeCount > 0 && readyNodes < nodeCount {
			resources = append(resources, name)
		}
	}
	return resources
}

func assistantChatPlatformDatabaseResources(output map[string]interface{}) []string {
	resources := []string{}
	for _, raw := range assistantChatSlice(output["items"]) {
		item := assistantChatMap(raw)
		status := firstNonEmptyAssistantChatString(item, "status")
		if status == "" || status == "运行中" {
			continue
		}
		name := firstNonEmptyAssistantChatString(item, "instanceName", "name", "endpoint", "hostIp")
		if name != "" {
			resources = append(resources, name)
		}
	}
	return resources
}

func assistantChatPlatformRunbookSuggestions(context map[string]interface{}) []string {
	knowledge := assistantChatMap(context["knowledge"])
	items := assistantChatSlice(knowledge["items"])
	suggestions := make([]string, 0, len(items))
	for i, raw := range items {
		if i >= 5 {
			break
		}
		item := assistantChatMap(raw)
		title := firstNonEmptyAssistantChatString(item, "title")
		if title != "" {
			suggestions = append(suggestions, title)
		}
	}
	return suggestions
}

func firstAssistantChatLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, sep := range []string{"\n", "。"} {
		if idx := strings.Index(value, sep); idx >= 0 {
			return strings.TrimSpace(value[:idx])
		}
	}
	return truncateAssistantChatText(value, 160)
}

func appendMissingStrings(items []string, values ...string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(items)+len(values))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func assistantChatUser(c *gin.Context) (uint, string) {
	if c == nil {
		return 0, ""
	}
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		return 0, ""
	}
	return admin.ID, admin.Username
}

func assistantChatChannel(req *aiopsmodel.AssistantChatRequest) string {
	if req == nil {
		return "web"
	}
	if value := assistantChatContextString(req.Context, "channel"); value != "" {
		return value
	}
	if strings.TrimSpace(req.Mode) != "" {
		return strings.TrimSpace(req.Mode)
	}
	return "web"
}
