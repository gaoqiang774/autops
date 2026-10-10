package controller

import (
	"strconv"
	"strings"

	"dodevops-api/api/aiops/model"
	"dodevops-api/api/aiops/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

// @Summary 创建运维知识库文档
// @Description 创建 Markdown 格式的故障文档、Runbook 或普通知识文档
// @Produce json
// @Tags AIOps知识库
// @Param data body model.KnowledgeDocumentDTO true "知识库文档"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/knowledge/documents [post]
// @Security ApiKeyAuth
func CreateKnowledgeDocument(c *gin.Context) {
	var dto model.KnowledgeDocumentDTO
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	data, err := service.NewKnowledgeService().CreateDocument(c, &dto)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "创建知识库文档失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 导入运维知识库文档
// @Description 上传 Markdown/文本文件，或粘贴 contentMarkdown/content 内容，并转换为标准 Markdown 知识库文档
// @Produce json
// @Tags AIOps知识库
// @Param file formData file false "文档文件"
// @Param contentMarkdown formData string false "粘贴的 Markdown 或普通文本内容"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/knowledge/documents/import [post]
// @Security ApiKeyAuth
func ImportKnowledgeDocument(c *gin.Context) {
	file, _ := c.FormFile("file")
	groupID, _ := strconv.ParseUint(strings.TrimSpace(c.PostForm("groupId")), 10, 64)
	dto := model.KnowledgeDocumentDTO{
		Title:           c.PostForm("title"),
		DocType:         c.PostForm("docType"),
		Status:          c.PostForm("status"),
		GroupID:         uint(groupID),
		GroupName:       c.PostForm("groupName"),
		Category:        c.PostForm("category"),
		Severity:        c.PostForm("severity"),
		ServiceName:     c.PostForm("serviceName"),
		Env:             c.PostForm("env"),
		ContentMarkdown: firstNonEmptyString(c.PostForm("contentMarkdown"), c.PostForm("content")),
	}
	data, err := service.NewKnowledgeService().ImportDocument(c, file, &dto)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "导入知识库文档失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 刷新知识库文档检索数据
// @Description 同步重建文档分块、关键词和 MySQL 检索字段
// @Produce json
// @Tags AIOps知识库
// @Param id path int true "文档ID"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/knowledge/documents/:id/search-data/refresh [post]
// @Security ApiKeyAuth
func RefreshKnowledgeDocumentSearchData(c *gin.Context) {
	id, ok := bindKnowledgeIDParam(c)
	if !ok {
		return
	}
	data, err := service.NewKnowledgeService().RefreshSearchData(id)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "刷新知识库检索数据失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 更新运维知识库文档
// @Description 更新 Markdown 知识库文档并重建分块
// @Produce json
// @Tags AIOps知识库
// @Param id path int true "文档ID"
// @Param data body model.KnowledgeDocumentDTO true "知识库文档"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/knowledge/documents/:id [put]
// @Security ApiKeyAuth
func UpdateKnowledgeDocument(c *gin.Context) {
	id, ok := bindKnowledgeIDParam(c)
	if !ok {
		return
	}
	var dto model.KnowledgeDocumentDTO
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	data, err := service.NewKnowledgeService().UpdateDocument(c, id, &dto)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "更新知识库文档失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 删除运维知识库文档
// @Description 删除知识库文档及其分块
// @Produce json
// @Tags AIOps知识库
// @Param id path int true "文档ID"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/knowledge/documents/:id [delete]
// @Security ApiKeyAuth
func DeleteKnowledgeDocument(c *gin.Context) {
	id, ok := bindKnowledgeIDParam(c)
	if !ok {
		return
	}
	if err := service.NewKnowledgeService().DeleteDocument(id); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "删除知识库文档失败: "+err.Error())
		return
	}
	result.Success(c, true)
}

// @Summary 获取运维知识库文档详情
// @Description 获取知识库文档 Markdown 原文、AI 摘要和结构化字段
// @Produce json
// @Tags AIOps知识库
// @Param id path int true "文档ID"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/knowledge/documents/:id [get]
// @Security ApiKeyAuth
func GetKnowledgeDocument(c *gin.Context) {
	id, ok := bindKnowledgeIDParam(c)
	if !ok {
		return
	}
	data, err := service.NewKnowledgeService().GetDocument(id)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询知识库文档失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 查询运维知识库文档列表
// @Description 分页查询知识库文档
// @Produce json
// @Tags AIOps知识库
// @Success 200 {object} result.Result
// @router /api/v1/aiops/knowledge/documents [get]
// @Security ApiKeyAuth
func ListKnowledgeDocuments(c *gin.Context) {
	var query model.KnowledgeDocumentQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	data, total, normalized, err := service.NewKnowledgeService().ListDocuments(query)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询知识库文档列表失败: "+err.Error())
		return
	}
	result.SuccessWithPage(c, data, total, normalized.Page, normalized.PageSize)
}

// @Summary 检索运维知识库
// @Description 根据问题、故障关键词或告警关键词检索知识库文档
// @Produce json
// @Tags AIOps知识库
// @Param data body model.KnowledgeSearchRequest true "检索条件"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/knowledge/search [post]
// @Security ApiKeyAuth
func SearchKnowledgeDocuments(c *gin.Context) {
	var req model.KnowledgeSearchRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	data, err := service.NewKnowledgeService().SearchDocuments(&req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "检索知识库失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary AI 总结运维知识库文档
// @Description 使用已配置 LLM 总结 Markdown 故障文档，提取现象、根因、处理步骤和关键词
// @Produce json
// @Tags AIOps知识库
// @Param id path int true "文档ID"
// @Param data body model.KnowledgeSummaryRequest false "模型配置"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/knowledge/documents/:id/summarize [post]
// @Security ApiKeyAuth
func SummarizeKnowledgeDocument(c *gin.Context) {
	id, ok := bindKnowledgeIDParam(c)
	if !ok {
		return
	}
	var req model.KnowledgeSummaryRequest
	_ = c.ShouldBindJSON(&req)
	data, err := service.NewKnowledgeService().SummarizeDocument(c.Request.Context(), id, &req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "AI 总结知识库文档失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

func bindKnowledgeIDParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return 0, false
	}
	return uint(id), true
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
