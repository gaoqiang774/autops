package model

import "dodevops-api/common/util"

const (
	KnowledgeDocTypeIncident = "incident"
	KnowledgeDocTypeRunbook  = "runbook"
	KnowledgeDocTypeFAQ      = "faq"
	KnowledgeDocTypeNote     = "note"

	KnowledgeStatusDraft     = "draft"
	KnowledgeStatusPublished = "published"
	KnowledgeStatusDisabled  = "disabled"

	KnowledgeSearchReady  = "ready"
	KnowledgeSearchFailed = "failed"

	KnowledgeSummaryPending    = "pending"
	KnowledgeSummaryProcessing = "processing"
	KnowledgeSummaryCompleted  = "completed"
	KnowledgeSummaryFailed     = "failed"
)

type KnowledgeDocument struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`                                         // 文档主键ID
	Title               string     `gorm:"size:255;not null;index" json:"title"`                         // 文档标题，用于列表展示和检索
	DocType             string     `gorm:"size:32;not null;default:incident;index" json:"docType"`       // 文档类型：incident故障复盘、runbook操作手册、faq问答、note普通笔记
	Status              string     `gorm:"size:32;not null;default:draft;index" json:"status"`           // 文档状态：draft草稿、published已发布、disabled停用
	GroupID             uint       `gorm:"not null;default:0;index" json:"groupId"`                      // 独立知识分组ID，0表示尚未归组
	GroupPath           string     `gorm:"-" json:"groupPath"`                                           // 查询时计算的完整分组路径
	GroupName           string     `gorm:"size:128;index" json:"groupName"`                              // 知识分组/团队/业务域，例如数据库组、中间件组、交易系统
	Category            string     `gorm:"size:128;index" json:"category"`                               // 文档分类/故障分类，例如MySQL主从延迟、Nginx 502、CPU高负载
	Severity            string     `gorm:"size:32;index" json:"severity"`                                // 严重级别，例如low、medium、high、critical
	ServiceName         string     `gorm:"size:255;index" json:"serviceName"`                            // 影响服务或业务系统，可用逗号分隔多个服务
	Env                 string     `gorm:"size:64;index" json:"env"`                                     // 适用环境，例如prod、test、dev
	TagsJSON            string     `gorm:"column:tags_json;type:text" json:"-"`                          // 标签JSON数组，用于人工筛选和检索
	KeywordsJSON        string     `gorm:"column:keywords_json;type:text" json:"-"`                      // 关键词JSON数组，支持AI总结后补充
	AlertNamesJSON      string     `gorm:"column:alert_names_json;type:text" json:"-"`                   // 关联告警名称JSON数组，例如Prometheus alertname
	LogKeywordsJSON     string     `gorm:"column:log_keywords_json;type:text" json:"-"`                  // 关联日志关键词JSON数组，用于日志诊断匹配
	OwnersJSON          string     `gorm:"column:owners_json;type:text" json:"-"`                        // 责任人/维护人JSON数组
	ContentMarkdown     string     `gorm:"column:content_markdown;type:longtext" json:"contentMarkdown"` // Markdown正文原文，作为人类编辑和AI切分的主内容
	Summary             string     `gorm:"type:text" json:"summary"`                                     // AI或人工维护的一句话摘要
	SymptomsJSON        string     `gorm:"column:symptoms_json;type:text" json:"-"`                      // AI提取的故障现象JSON数组
	RootCausesJSON      string     `gorm:"column:root_causes_json;type:text" json:"-"`                   // AI提取的根因JSON数组
	ResolutionStepsJSON string     `gorm:"column:resolution_steps_json;type:text" json:"-"`              // AI提取的处理步骤JSON数组
	VerificationJSON    string     `gorm:"column:verification_json;type:text" json:"-"`                  // AI提取的验证方式JSON数组
	RollbackPlan        string     `gorm:"type:text" json:"rollbackPlan"`                                // 回滚方案或风险处置方案
	SourceFilename      string     `gorm:"size:255" json:"sourceFilename"`                               // 导入来源文件名，手工创建时为空
	SummaryStatus       string     `gorm:"size:32;not null;default:pending;index" json:"summaryStatus"`  // 摘要处理状态
	SummaryError        string     `gorm:"type:text" json:"summaryError"`                                // 摘要处理失败原因
	SearchStatus        string     `gorm:"size:32;not null;default:ready;index" json:"searchStatus"`     // MySQL检索数据状态
	SearchError         string     `gorm:"type:text" json:"searchError"`                                 // 检索数据刷新失败原因
	SearchVersion       int        `gorm:"not null;default:1" json:"searchVersion"`                      // 检索分块或关键词更新版本
	ChunkCount          int        `gorm:"not null;default:0" json:"chunkCount"`                         // 当前文档分块数量
	CreatedBy           uint       `gorm:"index" json:"createdBy,omitempty"`                             // 创建人用户ID
	UpdatedBy           uint       `gorm:"index" json:"updatedBy,omitempty"`                             // 最近更新人用户ID
	CreatedAt           util.HTime `json:"createdAt"`                                                    // 创建时间
	UpdatedAt           util.HTime `json:"updatedAt"`                                                    // 更新时间
}

func (KnowledgeDocument) TableName() string {
	return "aiops_knowledge_document"
}

type KnowledgeChunk struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	DocumentID   uint       `gorm:"not null;index" json:"documentId"`
	ChunkIndex   int        `gorm:"not null" json:"chunkIndex"`
	Heading      string     `gorm:"size:255;index" json:"heading"`
	SectionPath  string     `gorm:"size:1000" json:"sectionPath"`
	Content      string     `gorm:"type:longtext" json:"content"`
	ContentHash  string     `gorm:"size:64;index" json:"contentHash"`
	KeywordsJSON string     `gorm:"column:keywords_json;type:text" json:"-"`
	CreatedAt    util.HTime `json:"createdAt"`
	UpdatedAt    util.HTime `json:"updatedAt"`
}

func (KnowledgeChunk) TableName() string {
	return "aiops_knowledge_chunk"
}

type KnowledgeDocumentDTO struct {
	ID              uint     `json:"id"`
	Title           string   `json:"title" binding:"required"`
	DocType         string   `json:"docType"`
	Status          string   `json:"status"`
	GroupID         uint     `json:"groupId"`
	GroupName       string   `json:"groupName"`
	Category        string   `json:"category"`
	Severity        string   `json:"severity"`
	ServiceName     string   `json:"serviceName"`
	Env             string   `json:"env"`
	Tags            []string `json:"tags"`
	Keywords        []string `json:"keywords"`
	AlertNames      []string `json:"alertNames"`
	LogKeywords     []string `json:"logKeywords"`
	Owners          []string `json:"owners"`
	ContentMarkdown string   `json:"contentMarkdown" binding:"required"`
	Summary         string   `json:"summary"`
	RollbackPlan    string   `json:"rollbackPlan"`
}

type KnowledgeDocumentQuery struct {
	Page        int    `form:"page"`
	PageSize    int    `form:"pageSize"`
	Keyword     string `form:"keyword"`
	DocType     string `form:"docType"`
	Status      string `form:"status"`
	GroupID     uint   `form:"groupId"`
	Recursive   bool   `form:"recursive"`
	GroupIDs    []uint `form:"-"`
	GroupName   string `form:"groupName"`
	Category    string `form:"category"`
	ServiceName string `form:"serviceName"`
	Env         string `form:"env"`
}

type KnowledgeSearchRequest struct {
	Query       string   `json:"query" binding:"required"`
	QueryTerms  []string `json:"-"`
	Limit       int      `json:"limit"`
	DocType     string   `json:"docType"`
	Status      string   `json:"status"`
	GroupID     uint     `json:"groupId"`
	GroupIDs    []uint   `json:"-"`
	GroupName   string   `json:"groupName"`
	Category    string   `json:"category"`
	ServiceName string   `json:"serviceName"`
	Tags        []string `json:"tags"`
}

type KnowledgeSearchItem struct {
	ID            uint     `json:"id"`
	Title         string   `json:"title"`
	DocType       string   `json:"docType"`
	Status        string   `json:"status"`
	GroupID       uint     `json:"groupId"`
	GroupPath     string   `json:"groupPath"`
	GroupName     string   `json:"groupName"`
	Category      string   `json:"category"`
	ServiceName   string   `json:"serviceName"`
	Env           string   `json:"env"`
	Summary       string   `json:"summary"`
	MatchedFields []string `json:"matchedFields"`
	Score         float64  `json:"score"`
	Snippet       string   `json:"snippet"`
	UpdatedAt     string   `json:"updatedAt"`
}

type KnowledgeSummaryRequest struct {
	ProviderID uint `json:"providerId"`
}

type KnowledgeSummaryResult struct {
	DocumentID      uint     `json:"documentId"`
	Summary         string   `json:"summary"`
	Symptoms        []string `json:"symptoms"`
	RootCauses      []string `json:"rootCauses"`
	ResolutionSteps []string `json:"resolutionSteps"`
	Verification    []string `json:"verification"`
	Keywords        []string `json:"keywords"`
	UsedLLM         bool     `json:"usedLlm"`
	ProviderID      uint     `json:"providerId,omitempty"`
	ProviderName    string   `json:"providerName,omitempty"`
	ProviderType    string   `json:"providerType,omitempty"`
	ModelCode       string   `json:"modelCode,omitempty"`
}

type KnowledgeDocumentVO struct {
	ID              uint       `json:"id"`
	Title           string     `json:"title"`
	DocType         string     `json:"docType"`
	Status          string     `json:"status"`
	GroupID         uint       `json:"groupId"`
	GroupPath       string     `json:"groupPath"`
	GroupName       string     `json:"groupName"`
	Category        string     `json:"category"`
	Severity        string     `json:"severity"`
	ServiceName     string     `json:"serviceName"`
	Env             string     `json:"env"`
	Tags            []string   `json:"tags"`
	Keywords        []string   `json:"keywords"`
	AlertNames      []string   `json:"alertNames"`
	LogKeywords     []string   `json:"logKeywords"`
	Owners          []string   `json:"owners"`
	ContentMarkdown string     `json:"contentMarkdown"`
	Summary         string     `json:"summary"`
	SummaryStatus   string     `json:"summaryStatus"`
	SummaryError    string     `json:"summaryError"`
	SearchStatus    string     `json:"searchStatus"`
	SearchError     string     `json:"searchError"`
	SearchVersion   int        `json:"searchVersion"`
	ChunkCount      int        `json:"chunkCount"`
	Symptoms        []string   `json:"symptoms"`
	RootCauses      []string   `json:"rootCauses"`
	ResolutionSteps []string   `json:"resolutionSteps"`
	Verification    []string   `json:"verification"`
	RollbackPlan    string     `json:"rollbackPlan"`
	SourceFilename  string     `json:"sourceFilename"`
	CreatedBy       uint       `json:"createdBy,omitempty"`
	UpdatedBy       uint       `json:"updatedBy,omitempty"`
	CreatedAt       util.HTime `json:"createdAt"`
	UpdatedAt       util.HTime `json:"updatedAt"`
}
