package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	logtailmodel "dodevops-api/api/logtail/model"
	logtailservice "dodevops-api/api/logtail/service"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

type LogtailController struct {
	service *logtailservice.LogtailService
}

func NewLogtailController() *LogtailController {
	return &LogtailController{service: logtailservice.NewLogtailService()}
}

// VictoriaLogsHealth 检测 VictoriaLogs 连通性。
// @Summary 检测 VictoriaLogs 连通性
// @Description 检测配置文件中的 VictoriaLogs 地址是否可访问，并返回 HTTP 状态码与请求耗时。
// @Tags 日志管理
// @Produce json
// @Success 200 {object} result.Result "检测成功"
// @Failure 500 {object} result.Result "检测失败"
// @Router /api/v1/logtail/victorialogs/health [get]
// @Security ApiKeyAuth
func (c *LogtailController) VictoriaLogsHealth(ctx *gin.Context) {
	data, err := c.service.VictoriaLogsHealth()
	if err != nil {
		result.Failed(ctx, 500, "检测 VictoriaLogs 失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// QueryLogs 查询日志明细。
// @Summary 查询日志明细
// @Description 根据日志库、时间范围、关键字或 LogsQL 查询 VictoriaLogs 日志明细。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogQueryLogsRequest true "日志查询参数"
// @Success 200 {object} result.Result{data=logtailmodel.LogQueryLogsResponse} "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/query/logs [post]
// @Security ApiKeyAuth
func (c *LogtailController) QueryLogs(ctx *gin.Context) {
	var req logtailmodel.LogQueryLogsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	data, err := c.service.QueryLogs(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "查询日志失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// QueryTraceList 查询 trace 摘要列表。
// @Summary 查询 trace 摘要列表
// @Description 在指定项目和时间范围内跨用户有权访问的日志库聚合 trace，用于链路分析页面左侧列表。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogTraceListQueryRequest true "trace 列表查询参数"
// @Success 200 {object} result.Result{data=logtailmodel.LogTraceListResponse} "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/query/traces [post]
// @Security ApiKeyAuth
func (c *LogtailController) QueryTraceList(ctx *gin.Context) {
	var req logtailmodel.LogTraceListQueryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	data, err := c.service.QueryTraceList(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "查询 trace 列表失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// QueryTraceAnalysis 按 trace_id 跨日志库关联日志。
// @Summary 查询日志关联链路
// @Description 在指定项目和时间范围内按 trace_id 关联用户有权访问的日志库，返回服务汇总和观测时间线；当前不推断父子调用顺序。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogTraceQueryRequest true "链路关联查询参数"
// @Success 200 {object} result.Result{data=logtailmodel.LogTraceAnalysisResponse} "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/query/trace [post]
// @Security ApiKeyAuth
func (c *LogtailController) QueryTraceAnalysis(ctx *gin.Context) {
	var req logtailmodel.LogTraceQueryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	data, err := c.service.QueryTraceAnalysis(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "查询日志关联链路失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// DownloadLogs 下载日志查询结果。
// @Summary 下载日志查询结果
// @Description 根据当前日志库、时间范围、查询语句和字段选择导出日志，支持 csv/jsonl/txt。
// @Tags 日志管理
// @Accept json
// @Produce application/octet-stream
// @Param data body logtailmodel.LogQueryDownloadRequest true "日志下载参数"
// @Success 200 {file} file "下载成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/query/logs/download [post]
// @Security ApiKeyAuth
func (c *LogtailController) DownloadLogs(ctx *gin.Context) {
	var req logtailmodel.LogQueryDownloadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	file, err := c.service.DownloadLogs(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "下载日志失败: "+err.Error())
		return
	}
	ctx.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.FileName))
	ctx.Header("Content-Transfer-Encoding", "binary")
	ctx.Header("Cache-Control", "no-store")
	ctx.Data(http.StatusOK, file.ContentType, file.Data)
}

// QueryHits 查询日志数量趋势。
// @Summary 查询日志数量趋势
// @Description 根据日志库、时间范围和查询条件获取日志命中趋势，可按字段拆分序列。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogQueryHitsRequest true "日志趋势查询参数"
// @Success 200 {object} result.Result{data=logtailmodel.LogQueryHitsResponse} "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/query/hits [post]
// @Security ApiKeyAuth
func (c *LogtailController) QueryHits(ctx *gin.Context) {
	var req logtailmodel.LogQueryHitsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	data, err := c.service.QueryHits(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "查询日志趋势失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// QueryFieldNames 查询可用字段。
// @Summary 查询日志可用字段
// @Description 根据日志库、时间范围和查询条件获取 VictoriaLogs 可用字段列表。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogQueryFieldsRequest true "字段查询参数"
// @Success 200 {object} result.Result{data=logtailmodel.LogQueryFieldsResponse} "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/query/fields [post]
// @Security ApiKeyAuth
func (c *LogtailController) QueryFieldNames(ctx *gin.Context) {
	var req logtailmodel.LogQueryFieldsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	data, err := c.service.QueryFieldNames(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "查询日志字段失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// QueryFieldValues 查询字段值。
// @Summary 查询日志字段值
// @Description 根据日志库、时间范围、查询条件和字段名获取字段值列表。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogQueryFieldValuesRequest true "字段值查询参数"
// @Success 200 {object} result.Result{data=logtailmodel.LogQueryFieldsResponse} "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/query/field-values [post]
// @Security ApiKeyAuth
func (c *LogtailController) QueryFieldValues(ctx *gin.Context) {
	var req logtailmodel.LogQueryFieldValuesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	data, err := c.service.QueryFieldValues(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "查询日志字段值失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// QueryStatsRange 查询日志聚合趋势。
// @Summary 查询日志聚合趋势
// @Description 透传 LogsQL stats 管道到 VictoriaLogs stats_query_range，用于 TopN、级别分布等聚合图表。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogQueryStatsRangeRequest true "聚合趋势查询参数"
// @Success 200 {object} result.Result{data=logtailmodel.LogQueryStatsRangeResponse} "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/query/stats-range [post]
// @Security ApiKeyAuth
func (c *LogtailController) QueryStatsRange(ctx *gin.Context) {
	var req logtailmodel.LogQueryStatsRangeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	data, err := c.service.QueryStatsRange(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "查询日志聚合失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// CreateProject 创建日志项目。
// @Summary 创建日志项目
// @Description 创建日志项目，项目用于承载多个日志库、采集任务和告警规则。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogProject true "日志项目"
// @Success 200 {object} result.Result{data=logtailmodel.LogProject} "创建成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/projects [post]
// @Security ApiKeyAuth
func (c *LogtailController) CreateProject(ctx *gin.Context) {
	var item logtailmodel.LogProject
	if err := ctx.ShouldBindJSON(&item); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	if err := c.service.CreateProject(&item); err != nil {
		result.Failed(ctx, 500, "创建项目失败: "+err.Error())
		return
	}
	result.Success(ctx, item)
}

// UpdateProject 更新日志项目。
// @Summary 更新日志项目
// @Description 根据项目 ID 更新日志项目基础信息。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param id path uint true "项目ID"
// @Param data body logtailmodel.LogProject true "日志项目"
// @Success 200 {object} result.Result{data=logtailmodel.LogProject} "更新成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/projects/{id} [put]
// @Security ApiKeyAuth
func (c *LogtailController) UpdateProject(ctx *gin.Context) {
	var item logtailmodel.LogProject
	if err := ctx.ShouldBindJSON(&item); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	if item.ID == 0 {
		item.ID = parseIDParam(ctx)
	}
	if err := c.service.UpdateProject(&item); err != nil {
		result.Failed(ctx, 500, "更新项目失败: "+err.Error())
		return
	}
	result.Success(ctx, item)
}

// DeleteProject 删除日志项目。
// @Summary 删除日志项目
// @Description 根据项目 ID 删除日志项目元数据。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "项目ID"
// @Success 200 {object} result.Result "删除成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/projects/{id} [delete]
// @Security ApiKeyAuth
func (c *LogtailController) DeleteProject(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "项目ID不能为空")
		return
	}
	if err := c.service.DeleteProject(id); err != nil {
		result.Failed(ctx, 500, "删除项目失败: "+err.Error())
		return
	}
	result.Success(ctx, nil)
}

// GetProject 获取日志项目详情。
// @Summary 获取日志项目详情
// @Description 根据项目 ID 查询日志项目详情。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "项目ID"
// @Success 200 {object} result.Result{data=logtailmodel.LogProject} "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/projects/{id} [get]
// @Security ApiKeyAuth
func (c *LogtailController) GetProject(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "项目ID不能为空")
		return
	}
	data, err := c.service.GetProject(id)
	if err != nil {
		result.Failed(ctx, 500, "查询项目失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListProjects 分页查询日志项目。
// @Summary 查询日志项目列表
// @Description 分页查询日志项目，支持关键字和状态筛选。
// @Tags 日志管理
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "项目名称或编码关键字"
// @Param status query int false "状态：1启用，2禁用"
// @Success 200 {object} result.Result "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/projects [get]
// @Security ApiKeyAuth
func (c *LogtailController) ListProjects(ctx *gin.Context) {
	var query logtailmodel.LogProjectQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	items, total, normalized, err := c.service.ListProjects(ctx, query)
	if err != nil {
		result.Failed(ctx, 500, "查询项目列表失败: "+err.Error())
		return
	}
	result.SuccessWithPage(ctx, items, total, normalized.Page, normalized.PageSize)
}

// ListProjectStoreCounts 查询项目下日志库数量。
// @Summary 查询项目日志库数量
// @Description 按项目聚合日志库数量，用于日志库管理左侧项目列表展示。
// @Tags 日志管理
// @Produce json
// @Param projectIds query string false "项目ID列表，逗号分隔；不传则返回全部项目的统计"
// @Success 200 {object} result.Result{data=[]logtailmodel.LogProjectStoreCount} "查询成功"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/projects/store-counts [get]
// @Security ApiKeyAuth
func (c *LogtailController) ListProjectStoreCounts(ctx *gin.Context) {
	items, err := c.service.ListProjectStoreCounts(ctx, parseUintListQuery(ctx.Query("projectIds")))
	if err != nil {
		result.Failed(ctx, 500, "查询项目日志库数量失败: "+err.Error())
		return
	}
	result.Success(ctx, items)
}

// CreateStore 创建日志库。
// @Summary 创建日志库
// @Description 在指定项目下创建日志库，日志库用于承载一类日志数据。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogStore true "日志库"
// @Success 200 {object} result.Result{data=logtailmodel.LogStore} "创建成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/stores [post]
// @Security ApiKeyAuth
func (c *LogtailController) CreateStore(ctx *gin.Context) {
	var item logtailmodel.LogStore
	if err := ctx.ShouldBindJSON(&item); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	if err := c.service.CreateStore(&item); err != nil {
		result.Failed(ctx, 500, "创建日志库失败: "+err.Error())
		return
	}
	result.Success(ctx, item)
}

// UpdateStore 更新日志库。
// @Summary 更新日志库
// @Description 根据日志库 ID 更新日志库配置。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param id path uint true "日志库ID"
// @Param data body logtailmodel.LogStore true "日志库"
// @Success 200 {object} result.Result{data=logtailmodel.LogStore} "更新成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/stores/{id} [put]
// @Security ApiKeyAuth
func (c *LogtailController) UpdateStore(ctx *gin.Context) {
	var item logtailmodel.LogStore
	if err := ctx.ShouldBindJSON(&item); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	if item.ID == 0 {
		item.ID = parseIDParam(ctx)
	}
	if err := c.service.UpdateStore(&item); err != nil {
		result.Failed(ctx, 500, "更新日志库失败: "+err.Error())
		return
	}
	result.Success(ctx, item)
}

// DeleteStore 删除日志库。
// @Summary 删除日志库
// @Description 根据日志库 ID 删除日志库元数据。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "日志库ID"
// @Success 200 {object} result.Result "删除成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/stores/{id} [delete]
// @Security ApiKeyAuth
func (c *LogtailController) DeleteStore(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "日志库ID不能为空")
		return
	}
	if err := c.service.DeleteStore(id); err != nil {
		result.Failed(ctx, 500, "删除日志库失败: "+err.Error())
		return
	}
	result.Success(ctx, nil)
}

// GetStore 获取日志库详情。
// @Summary 获取日志库详情
// @Description 根据日志库 ID 查询日志库详情。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "日志库ID"
// @Success 200 {object} result.Result{data=logtailmodel.LogStore} "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/stores/{id} [get]
// @Security ApiKeyAuth
func (c *LogtailController) GetStore(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "日志库ID不能为空")
		return
	}
	data, err := c.service.GetStore(id)
	if err != nil {
		result.Failed(ctx, 500, "查询日志库失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListStores 分页查询日志库。
// @Summary 查询日志库列表
// @Description 分页查询日志库，支持按项目、关键字、状态筛选。
// @Tags 日志管理
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param projectId query uint false "项目ID"
// @Param keyword query string false "日志库名称或编码关键字"
// @Param status query int false "状态：1启用，2禁用"
// @Success 200 {object} result.Result "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/stores [get]
// @Security ApiKeyAuth
func (c *LogtailController) ListStores(ctx *gin.Context) {
	var query logtailmodel.LogStoreQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	items, total, normalized, err := c.service.ListStores(ctx, query)
	if err != nil {
		result.Failed(ctx, 500, "查询日志库列表失败: "+err.Error())
		return
	}
	result.SuccessWithPage(ctx, items, total, normalized.Page, normalized.PageSize)
}

// CreateParseTemplate 创建解析模板。
// @Summary 创建解析模板
// @Description 创建日志解析模板，用于描述 raw、json、regex、nginx 等解析方式。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogParseTemplate true "解析模板"
// @Success 200 {object} result.Result{data=logtailmodel.LogParseTemplate} "创建成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/parse-templates [post]
// @Security ApiKeyAuth
func (c *LogtailController) CreateParseTemplate(ctx *gin.Context) {
	var item logtailmodel.LogParseTemplate
	if err := ctx.ShouldBindJSON(&item); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	if err := c.service.CreateParseTemplate(&item); err != nil {
		result.Failed(ctx, 500, "创建解析模板失败: "+err.Error())
		return
	}
	result.Success(ctx, item)
}

// GenerateParseTemplateWithAI 根据样例日志调用大模型生成解析模板。
// @Summary AI 生成日志解析模板
// @Description 复用 AIOps LLM Provider，根据样例日志生成可嵌入 Vector remap 的 VRL 脚本；save=true 时直接保存为解析模板。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogParseTemplateAIGenerateRequest true "AI生成解析模板参数"
// @Success 200 {object} result.Result{data=logtailmodel.LogParseTemplateAIGenerateResponse} "生成成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/parse-templates/ai-generate [post]
// @Security ApiKeyAuth
func (c *LogtailController) GenerateParseTemplateWithAI(ctx *gin.Context) {
	var req logtailmodel.LogParseTemplateAIGenerateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	data, err := c.service.GenerateParseTemplateWithAI(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "AI生成解析模板失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// UpdateParseTemplate 更新解析模板。
// @Summary 更新解析模板
// @Description 根据解析模板 ID 更新解析字段、多行规则和 VRL 脚本等配置。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param id path uint true "解析模板ID"
// @Param data body logtailmodel.LogParseTemplate true "解析模板"
// @Success 200 {object} result.Result{data=logtailmodel.LogParseTemplate} "更新成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/parse-templates/{id} [put]
// @Security ApiKeyAuth
func (c *LogtailController) UpdateParseTemplate(ctx *gin.Context) {
	var item logtailmodel.LogParseTemplate
	if err := ctx.ShouldBindJSON(&item); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	if item.ID == 0 {
		item.ID = parseIDParam(ctx)
	}
	if err := c.service.UpdateParseTemplate(&item); err != nil {
		result.Failed(ctx, 500, "更新解析模板失败: "+err.Error())
		return
	}
	result.Success(ctx, item)
}

// DeleteParseTemplate 删除解析模板。
// @Summary 删除解析模板
// @Description 根据解析模板 ID 删除解析模板元数据。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "解析模板ID"
// @Success 200 {object} result.Result "删除成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/parse-templates/{id} [delete]
// @Security ApiKeyAuth
func (c *LogtailController) DeleteParseTemplate(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "解析模板ID不能为空")
		return
	}
	if err := c.service.DeleteParseTemplate(id); err != nil {
		result.Failed(ctx, 500, "删除解析模板失败: "+err.Error())
		return
	}
	result.Success(ctx, nil)
}

// GetParseTemplate 获取解析模板详情。
// @Summary 获取解析模板详情
// @Description 根据解析模板 ID 查询解析模板详情。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "解析模板ID"
// @Success 200 {object} result.Result{data=logtailmodel.LogParseTemplate} "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/parse-templates/{id} [get]
// @Security ApiKeyAuth
func (c *LogtailController) GetParseTemplate(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "解析模板ID不能为空")
		return
	}
	data, err := c.service.GetParseTemplate(id)
	if err != nil {
		result.Failed(ctx, 500, "查询解析模板失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListParseTemplates 分页查询解析模板。
// @Summary 查询解析模板列表
// @Description 分页查询解析模板，支持按项目、解析模式、关键字、状态筛选。
// @Tags 日志管理
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param projectId query uint false "项目ID"
// @Param keyword query string false "模板名称或编码关键字"
// @Param parseMode query string false "解析模式：raw/json/regex/nginx"
// @Param status query int false "状态：1启用，2禁用"
// @Success 200 {object} result.Result "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/parse-templates [get]
// @Security ApiKeyAuth
func (c *LogtailController) ListParseTemplates(ctx *gin.Context) {
	var query logtailmodel.LogParseTemplateQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	items, total, normalized, err := c.service.ListParseTemplates(query)
	if err != nil {
		result.Failed(ctx, 500, "查询解析模板列表失败: "+err.Error())
		return
	}
	result.SuccessWithPage(ctx, items, total, normalized.Page, normalized.PageSize)
}

// CreateCollectTask 创建采集任务。
// @Summary 创建日志采集任务
// @Description 创建日志采集任务，用于指定执行 Agent、目标日志库、日志路径和解析模板。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogCollectTask true "采集任务"
// @Success 200 {object} result.Result{data=logtailmodel.LogCollectTask} "创建成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 409 {object} result.Result "任务名称或采集配置重复"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/collect-tasks [post]
// @Security ApiKeyAuth
func (c *LogtailController) CreateCollectTask(ctx *gin.Context) {
	var item logtailmodel.LogCollectTask
	if err := ctx.ShouldBindJSON(&item); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	if err := c.service.CreateCollectTask(&item); err != nil {
		var conflictErr *logtailservice.CollectTaskConflictError
		if errors.As(err, &conflictErr) {
			result.Failed(ctx, http.StatusConflict, conflictErr.Error())
			return
		}
		result.Failed(ctx, http.StatusInternalServerError, "创建采集任务失败: "+err.Error())
		return
	}
	result.Success(ctx, item)
}

// UpdateCollectTask 更新采集任务。
// @Summary 更新日志采集任务
// @Description 根据采集任务 ID 更新执行目标、日志路径、解析配置和启停状态。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param id path uint true "采集任务ID"
// @Param data body logtailmodel.LogCollectTask true "采集任务"
// @Success 200 {object} result.Result{data=logtailmodel.LogCollectTask} "更新成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 409 {object} result.Result "任务名称或采集配置重复"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/collect-tasks/{id} [put]
// @Security ApiKeyAuth
func (c *LogtailController) UpdateCollectTask(ctx *gin.Context) {
	var item logtailmodel.LogCollectTask
	if err := ctx.ShouldBindJSON(&item); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	if item.ID == 0 {
		item.ID = parseIDParam(ctx)
	}
	if err := c.service.UpdateCollectTask(&item); err != nil {
		var conflictErr *logtailservice.CollectTaskConflictError
		if errors.As(err, &conflictErr) {
			result.Failed(ctx, http.StatusConflict, conflictErr.Error())
			return
		}
		result.Failed(ctx, http.StatusInternalServerError, "更新采集任务失败: "+err.Error())
		return
	}
	result.Success(ctx, item)
}

// DeleteCollectTask 删除采集任务。
// @Summary 删除日志采集任务
// @Description 根据采集任务 ID 删除采集任务元数据。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "采集任务ID"
// @Success 200 {object} result.Result "删除成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/collect-tasks/{id} [delete]
// @Security ApiKeyAuth
func (c *LogtailController) DeleteCollectTask(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "采集任务ID不能为空")
		return
	}
	if err := c.service.DeleteCollectTask(id); err != nil {
		if errors.Is(err, logtailservice.ErrCollectTaskDeleteRequiresStopped) {
			result.Failed(ctx, 400, err.Error())
			return
		}
		result.Failed(ctx, 500, "删除采集任务失败: "+err.Error())
		return
	}
	result.Success(ctx, nil)
}

// GetCollectTask 获取采集任务详情。
// @Summary 获取日志采集任务详情
// @Description 根据采集任务 ID 查询采集任务详情。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "采集任务ID"
// @Success 200 {object} result.Result{data=logtailmodel.LogCollectTask} "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/collect-tasks/{id} [get]
// @Security ApiKeyAuth
func (c *LogtailController) GetCollectTask(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "采集任务ID不能为空")
		return
	}
	data, err := c.service.GetCollectTask(id)
	if err != nil {
		result.Failed(ctx, 500, "查询采集任务失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListCollectTasks 分页查询采集任务。
// @Summary 查询日志采集任务列表
// @Description 分页查询采集任务，支持按项目、日志库、采集类型、关键字、状态筛选。
// @Tags 日志管理
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param projectId query uint false "项目ID"
// @Param storeId query uint false "日志库ID"
// @Param keyword query string false "任务名称或编码关键字"
// @Param collectType query string false "采集类型：host/k8s"
// @Param status query int false "状态：1启用，2禁用"
// @Success 200 {object} result.Result "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/collect-tasks [get]
// @Security ApiKeyAuth
func (c *LogtailController) ListCollectTasks(ctx *gin.Context) {
	var query logtailmodel.LogCollectTaskQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	items, total, normalized, err := c.service.ListCollectTasks(query)
	if err != nil {
		result.Failed(ctx, 500, "查询采集任务列表失败: "+err.Error())
		return
	}
	result.SuccessWithPage(ctx, items, total, normalized.Page, normalized.PageSize)
}

// ListCollectTaskAgents 查询采集任务命中的 Agent。
// @Summary 查询采集任务命中 Agent
// @Description 根据采集任务的执行模式和执行目标，查询命中的 Agent 以及 Vector 状态摘要。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "采集任务ID"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "主机名或IP关键字"
// @Param onlineStatus query int false "Agent 在线状态"
// @Param vectorStatus query string false "Vector 状态"
// @Param configStatus query string false "配置状态"
// @Success 200 {object} result.Result{data=logtailmodel.LogCollectTaskAgentsResponse} "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/collect-tasks/{id}/agents [get]
// @Security ApiKeyAuth
func (c *LogtailController) ListCollectTaskAgents(ctx *gin.Context) {
	taskID := parseIDParam(ctx)
	if taskID == 0 {
		result.Failed(ctx, 400, "采集任务ID不能为空")
		return
	}
	var query logtailmodel.LogAgentQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	data, err := c.service.ListCollectTaskAgents(taskID, query)
	if err != nil {
		result.Failed(ctx, 500, "查询采集任务命中 Agent 失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ApplyCollectTask 应用采集任务到命中 Agent。
// @Summary 一键应用采集任务
// @Description 根据采集任务命中的 Agent 自动创建 install_vector / reload_vector 操作。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param id path uint true "采集任务ID"
// @Param data body logtailmodel.LogCollectTaskApplyRequest false "应用参数"
// @Success 200 {object} result.Result{data=logtailmodel.LogCollectTaskApplyResponse} "应用成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/collect-tasks/{id}/apply [post]
// @Security ApiKeyAuth
func (c *LogtailController) ApplyCollectTask(ctx *gin.Context) {
	taskID := parseIDParam(ctx)
	if taskID == 0 {
		result.Failed(ctx, 400, "采集任务ID不能为空")
		return
	}
	var req logtailmodel.LogCollectTaskApplyRequest
	if ctx.Request.ContentLength > 0 {
		if err := ctx.ShouldBindJSON(&req); err != nil {
			result.Failed(ctx, 400, "参数错误")
			return
		}
	}
	data, err := c.service.ApplyCollectTask(taskID, req)
	if err != nil {
		result.Failed(ctx, 500, "应用采集任务失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// StopCollectTask 停止主机采集任务。
// @Summary 停止主机采集任务
// @Description 将主机采集任务置为禁用，并对命中的 Agent 创建 reload_vector 操作，使最终 vector.yaml 移除该任务。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param id path uint true "采集任务ID"
// @Param data body logtailmodel.LogCollectTaskApplyRequest false "停止参数"
// @Success 200 {object} result.Result{data=logtailmodel.LogCollectTaskApplyResponse} "停止成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/collect-tasks/{id}/stop [post]
// @Security ApiKeyAuth
func (c *LogtailController) StopCollectTask(ctx *gin.Context) {
	taskID := parseIDParam(ctx)
	if taskID == 0 {
		result.Failed(ctx, 400, "采集任务ID不能为空")
		return
	}
	var req logtailmodel.LogCollectTaskApplyRequest
	if ctx.Request.ContentLength > 0 {
		if err := ctx.ShouldBindJSON(&req); err != nil {
			result.Failed(ctx, 400, "参数错误")
			return
		}
	}
	data, err := c.service.StopCollectTask(taskID, req)
	if err != nil {
		if errors.Is(err, logtailservice.ErrCollectTaskStopUnsupported) {
			result.Failed(ctx, 400, err.Error())
			return
		}
		result.Failed(ctx, 500, "停止采集任务失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListKubernetesClusterCollectors 查询 K8S 集群级 Collector 列表。
func (c *LogtailController) ListKubernetesClusterCollectors(ctx *gin.Context) {
	var query logtailmodel.LogK8SClusterCollectorQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	data, err := c.service.ListKubernetesClusterCollectors(query)
	if err != nil {
		result.Failed(ctx, 500, "查询K8S集群Collector列表失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// GetKubernetesClusterCollectorStatus 查询 K8S 集群级 Collector 实时状态。
func (c *LogtailController) GetKubernetesClusterCollectorStatus(ctx *gin.Context) {
	clusterID := parseIDParam(ctx)
	if clusterID == 0 {
		result.Failed(ctx, 400, "K8S集群ID不能为空")
		return
	}
	data, err := c.service.GetKubernetesClusterCollectorStatus(clusterID)
	if err != nil {
		result.Failed(ctx, 500, "查询K8S集群Collector状态失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// InstallKubernetesClusterCollector 安装 K8S 集群级 Collector。
func (c *LogtailController) InstallKubernetesClusterCollector(ctx *gin.Context) {
	c.handleKubernetesClusterCollectorAction(ctx, "install")
}

// UpdateKubernetesClusterCollector 更新 K8S 集群级 Collector。
func (c *LogtailController) UpdateKubernetesClusterCollector(ctx *gin.Context) {
	c.handleKubernetesClusterCollectorAction(ctx, "update")
}

// UninstallKubernetesClusterCollector 卸载 K8S 集群级 Collector。
func (c *LogtailController) UninstallKubernetesClusterCollector(ctx *gin.Context) {
	c.handleKubernetesClusterCollectorAction(ctx, "uninstall")
}

// ReinstallKubernetesClusterCollector 重装 K8S 集群级 Collector。
func (c *LogtailController) ReinstallKubernetesClusterCollector(ctx *gin.Context) {
	c.handleKubernetesClusterCollectorAction(ctx, "reinstall")
}

func (c *LogtailController) handleKubernetesClusterCollectorAction(ctx *gin.Context, action string) {
	clusterID := parseIDParam(ctx)
	if clusterID == 0 {
		result.Failed(ctx, 400, "K8S集群ID不能为空")
		return
	}
	var req logtailmodel.LogK8SClusterCollectorActionRequest
	if ctx.Request.ContentLength > 0 {
		if err := ctx.ShouldBindJSON(&req); err != nil {
			result.Failed(ctx, 400, "参数错误")
			return
		}
	}

	var (
		data *logtailmodel.LogK8SClusterCollectorVO
		err  error
	)
	switch action {
	case "install":
		data, err = c.service.InstallKubernetesClusterCollector(clusterID, req)
	case "update":
		data, err = c.service.UpdateKubernetesClusterCollector(clusterID, req)
	case "uninstall":
		data, err = c.service.UninstallKubernetesClusterCollector(clusterID, req)
	case "reinstall":
		data, err = c.service.ReinstallKubernetesClusterCollector(clusterID, req)
	default:
		result.Failed(ctx, 400, "不支持的K8S集群Collector操作")
		return
	}
	if err != nil {
		result.Failed(ctx, 500, "执行K8S集群Collector操作失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListKubernetesClusterCollectorOperations 查询 K8S 集群级 Collector 操作记录。
func (c *LogtailController) ListKubernetesClusterCollectorOperations(ctx *gin.Context) {
	clusterID := parseIDParam(ctx)
	if clusterID == 0 {
		result.Failed(ctx, 400, "K8S集群ID不能为空")
		return
	}
	var query logtailmodel.LogK8SClusterCollectorOperationQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	items, total, normalized, err := c.service.ListKubernetesClusterCollectorOperations(clusterID, query)
	if err != nil {
		result.Failed(ctx, 500, "查询K8S集群Collector操作记录失败: "+err.Error())
		return
	}
	result.SuccessWithPage(ctx, items, total, normalized.Page, normalized.PageSize)
}

// ListKubernetesCollectors 查询 K8S Collector 列表。
func (c *LogtailController) ListKubernetesCollectors(ctx *gin.Context) {
	var query logtailmodel.LogK8SCollectorQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	data, err := c.service.ListKubernetesCollectors(query)
	if err != nil {
		result.Failed(ctx, 500, "查询K8S Collector列表失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// GetKubernetesCollectorStatus 查询 K8S Collector 实时状态。
func (c *LogtailController) GetKubernetesCollectorStatus(ctx *gin.Context) {
	taskID := parseIDParam(ctx)
	if taskID == 0 {
		result.Failed(ctx, 400, "采集任务ID不能为空")
		return
	}
	targetIndex := parseTargetIndexQuery(ctx)
	data, err := c.service.GetKubernetesCollectorStatus(taskID, targetIndex)
	if err != nil {
		result.Failed(ctx, 500, "查询K8S Collector状态失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// InstallKubernetesCollector 安装 K8S Collector。
func (c *LogtailController) InstallKubernetesCollector(ctx *gin.Context) {
	c.handleKubernetesCollectorAction(ctx, "install")
}

// UpdateKubernetesCollector 更新 K8S Collector。
func (c *LogtailController) UpdateKubernetesCollector(ctx *gin.Context) {
	c.handleKubernetesCollectorAction(ctx, "update")
}

// UninstallKubernetesCollector 卸载 K8S Collector。
func (c *LogtailController) UninstallKubernetesCollector(ctx *gin.Context) {
	c.handleKubernetesCollectorAction(ctx, "uninstall")
}

// ReinstallKubernetesCollector 重装 K8S Collector。
func (c *LogtailController) ReinstallKubernetesCollector(ctx *gin.Context) {
	c.handleKubernetesCollectorAction(ctx, "reinstall")
}

func (c *LogtailController) handleKubernetesCollectorAction(ctx *gin.Context, action string) {
	taskID := parseIDParam(ctx)
	if taskID == 0 {
		result.Failed(ctx, 400, "采集任务ID不能为空")
		return
	}
	var req logtailmodel.LogK8SCollectorActionRequest
	if ctx.Request.ContentLength > 0 {
		if err := ctx.ShouldBindJSON(&req); err != nil {
			result.Failed(ctx, 400, "参数错误")
			return
		}
	}
	if req.TargetIndex == nil {
		if raw := strings.TrimSpace(ctx.Query("targetIndex")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 {
				result.Failed(ctx, 400, "targetIndex参数错误")
				return
			}
			req.TargetIndex = &parsed
		}
	}

	var (
		data *logtailmodel.LogCollectTaskApplyResponse
		err  error
	)
	switch action {
	case "install":
		data, err = c.service.InstallKubernetesCollector(taskID, req)
	case "update":
		data, err = c.service.UpdateKubernetesCollector(taskID, req)
	case "uninstall":
		data, err = c.service.UninstallKubernetesCollector(taskID, req)
	case "reinstall":
		data, err = c.service.ReinstallKubernetesCollector(taskID, req)
	default:
		result.Failed(ctx, 400, "不支持的K8S Collector操作")
		return
	}
	if err != nil {
		result.Failed(ctx, 500, "执行K8S Collector操作失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListKubernetesCollectorOperations 查询 K8S Collector 操作记录。
func (c *LogtailController) ListKubernetesCollectorOperations(ctx *gin.Context) {
	taskID := parseIDParam(ctx)
	if taskID == 0 {
		result.Failed(ctx, 400, "采集任务ID不能为空")
		return
	}
	var query logtailmodel.LogK8SCollectorOperationQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	items, total, normalized, err := c.service.ListKubernetesCollectorOperations(taskID, query)
	if err != nil {
		result.Failed(ctx, 500, "查询K8S Collector操作记录失败: "+err.Error())
		return
	}
	result.SuccessWithPage(ctx, items, total, normalized.Page, normalized.PageSize)
}

// CreateAlertRule 创建日志告警规则。
// @Summary 创建日志告警规则
// @Description 创建关键字、字段条件或查询表达式类日志告警规则。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogAlertRule true "告警规则"
// @Success 200 {object} result.Result{data=logtailmodel.LogAlertRule} "创建成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/alert-rules [post]
// @Security ApiKeyAuth
func (c *LogtailController) CreateAlertRule(ctx *gin.Context) {
	var item logtailmodel.LogAlertRule
	if err := ctx.ShouldBindJSON(&item); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	if err := c.service.CreateAlertRule(&item); err != nil {
		result.Failed(ctx, 500, "创建告警规则失败: "+err.Error())
		return
	}
	result.Success(ctx, item)
}

// UpdateAlertRule 更新日志告警规则。
// @Summary 更新日志告警规则
// @Description 根据告警规则 ID 更新扫描窗口、阈值、关键字、通知渠道和启停状态。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param id path uint true "告警规则ID"
// @Param data body logtailmodel.LogAlertRule true "告警规则"
// @Success 200 {object} result.Result{data=logtailmodel.LogAlertRule} "更新成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/alert-rules/{id} [put]
// @Security ApiKeyAuth
func (c *LogtailController) UpdateAlertRule(ctx *gin.Context) {
	var item logtailmodel.LogAlertRule
	if err := ctx.ShouldBindJSON(&item); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	if item.ID == 0 {
		item.ID = parseIDParam(ctx)
	}
	if err := c.service.UpdateAlertRule(&item); err != nil {
		result.Failed(ctx, 500, "更新告警规则失败: "+err.Error())
		return
	}
	result.Success(ctx, item)
}

// DeleteAlertRule 删除日志告警规则。
// @Summary 删除日志告警规则
// @Description 根据告警规则 ID 删除告警规则元数据。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "告警规则ID"
// @Success 200 {object} result.Result "删除成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/alert-rules/{id} [delete]
// @Security ApiKeyAuth
func (c *LogtailController) DeleteAlertRule(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "告警规则ID不能为空")
		return
	}
	if err := c.service.DeleteAlertRule(id); err != nil {
		result.Failed(ctx, 500, "删除告警规则失败: "+err.Error())
		return
	}
	result.Success(ctx, nil)
}

// GetAlertRule 获取日志告警规则详情。
// @Summary 获取日志告警规则详情
// @Description 根据告警规则 ID 查询告警规则详情。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "告警规则ID"
// @Success 200 {object} result.Result{data=logtailmodel.LogAlertRule} "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/alert-rules/{id} [get]
// @Security ApiKeyAuth
func (c *LogtailController) GetAlertRule(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "告警规则ID不能为空")
		return
	}
	data, err := c.service.GetAlertRule(id)
	if err != nil {
		result.Failed(ctx, 500, "查询告警规则失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListAlertRules 分页查询日志告警规则。
// @Summary 查询日志告警规则列表
// @Description 分页查询告警规则，支持按项目、日志库、严重级别、关键字、状态筛选。
// @Tags 日志管理
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param projectId query uint false "项目ID"
// @Param storeId query uint false "日志库ID"
// @Param keyword query string false "规则名称或编码关键字"
// @Param severity query string false "严重级别"
// @Param status query int false "状态：1启用，2禁用"
// @Success 200 {object} result.Result "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/alert-rules [get]
// @Security ApiKeyAuth
func (c *LogtailController) ListAlertRules(ctx *gin.Context) {
	var query logtailmodel.LogAlertRuleQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	items, total, normalized, err := c.service.ListAlertRules(query)
	if err != nil {
		result.Failed(ctx, 500, "查询告警规则列表失败: "+err.Error())
		return
	}
	result.SuccessWithPage(ctx, items, total, normalized.Page, normalized.PageSize)
}

// CreateAlertEvent 创建日志告警事件。
// @Summary 创建日志告警事件
// @Description 创建一条日志告警事件记录，通常由后台扫描任务触发写入。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogAlertEvent true "告警事件"
// @Success 200 {object} result.Result{data=logtailmodel.LogAlertEvent} "创建成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/alert-events [post]
// @Security ApiKeyAuth
func (c *LogtailController) CreateAlertEvent(ctx *gin.Context) {
	var item logtailmodel.LogAlertEvent
	if err := ctx.ShouldBindJSON(&item); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	if err := c.service.CreateAlertEvent(&item); err != nil {
		result.Failed(ctx, 500, "创建告警事件失败: "+err.Error())
		return
	}
	result.Success(ctx, item)
}

// UpdateAlertEvent 更新日志告警事件。
// @Summary 更新日志告警事件
// @Description 根据告警事件 ID 更新事件状态、通知结果、诊断报告等信息。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param id path uint true "告警事件ID"
// @Param data body logtailmodel.LogAlertEvent true "告警事件"
// @Success 200 {object} result.Result{data=logtailmodel.LogAlertEvent} "更新成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/alert-events/{id} [put]
// @Security ApiKeyAuth
func (c *LogtailController) UpdateAlertEvent(ctx *gin.Context) {
	var item logtailmodel.LogAlertEvent
	if err := ctx.ShouldBindJSON(&item); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	if item.ID == 0 {
		item.ID = parseIDParam(ctx)
	}
	if err := c.service.UpdateAlertEvent(&item); err != nil {
		result.Failed(ctx, 500, "更新告警事件失败: "+err.Error())
		return
	}
	result.Success(ctx, item)
}

// DeleteAlertEvent 删除日志告警事件。
// @Summary 删除日志告警事件
// @Description 根据告警事件 ID 删除告警事件历史记录。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "告警事件ID"
// @Success 200 {object} result.Result "删除成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/alert-events/{id} [delete]
// @Security ApiKeyAuth
func (c *LogtailController) DeleteAlertEvent(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "告警事件ID不能为空")
		return
	}
	if err := c.service.DeleteAlertEvent(id); err != nil {
		result.Failed(ctx, 500, "删除告警事件失败: "+err.Error())
		return
	}
	result.Success(ctx, nil)
}

// GetAlertEvent 获取日志告警事件详情。
// @Summary 获取日志告警事件详情
// @Description 根据告警事件 ID 查询告警事件详情。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "告警事件ID"
// @Success 200 {object} result.Result{data=logtailmodel.LogAlertEvent} "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/alert-events/{id} [get]
// @Security ApiKeyAuth
func (c *LogtailController) GetAlertEvent(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "告警事件ID不能为空")
		return
	}
	data, err := c.service.GetAlertEvent(id)
	if err != nil {
		result.Failed(ctx, 500, "查询告警事件失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListAlertEvents 分页查询日志告警事件。
// @Summary 查询日志告警事件列表
// @Description 分页查询告警事件，支持按项目、日志库、严重级别、事件状态、关键字筛选。
// @Tags 日志管理
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param projectId query uint false "项目ID"
// @Param storeId query uint false "日志库ID"
// @Param keyword query string false "告警内容关键字"
// @Param severity query string false "严重级别"
// @Param status query string false "事件状态：firing/resolved"
// @Success 200 {object} result.Result "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/alert-events [get]
// @Security ApiKeyAuth
func (c *LogtailController) ListAlertEvents(ctx *gin.Context) {
	var query logtailmodel.LogAlertEventQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	items, total, normalized, err := c.service.ListAlertEvents(query)
	if err != nil {
		result.Failed(ctx, 500, "查询告警事件列表失败: "+err.Error())
		return
	}
	result.SuccessWithPage(ctx, items, total, normalized.Page, normalized.PageSize)
}

func parseIDParam(ctx *gin.Context) uint {
	value := ctx.Param("id")
	if value == "" {
		value = ctx.Query("id")
	}
	id, _ := strconv.Atoi(value)
	if id <= 0 {
		return 0
	}
	return uint(id)
}

func parseTargetIndexQuery(ctx *gin.Context) int {
	raw := strings.TrimSpace(ctx.Query("targetIndex"))
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func parseUintListQuery(raw string) []uint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]uint, 0, len(parts))
	for _, part := range parts {
		id, _ := strconv.Atoi(strings.TrimSpace(part))
		if id > 0 {
			values = append(values, uint(id))
		}
	}
	return values
}
