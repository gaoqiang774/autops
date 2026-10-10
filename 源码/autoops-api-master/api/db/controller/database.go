package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	dbmodel "dodevops-api/api/db/model"
	"dodevops-api/api/db/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

type DatabaseController struct {
	service *service.DatabaseService
}

func NewDatabaseController() *DatabaseController {
	return &DatabaseController{service: service.NewDatabaseService()}
}

// ListAssets 获取数据库资产列表。
// @Summary 获取数据库资产列表
// @Description 分页查询已纳入 CMDB 的数据库资产，供数据库管理工作台左侧资产入口使用
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "关键字"
// @Param type query int false "数据库类型"
// @Param environment query string false "环境"
// @Param status query int false "资产状态"
// @Param authStatus query int false "认证状态"
// @Param businessSystem query string false "业务系统"
// @Param owner query string false "负责人"
// @Success 200 {object} result.Result
// @Router /api/v1/db/assets [get]
// @Security ApiKeyAuth
func (c *DatabaseController) ListAssets(ctx *gin.Context) {
	var query dbmodel.AssetQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid query params")
		return
	}

	list, total, err := c.service.ListAssets(query)
	if err != nil {
		result.Failed(ctx, 500, "query database assets failed: "+err.Error())
		return
	}
	query.Page, query.PageSize = dbmodel.NormalizePage(query.Page, query.PageSize)
	result.SuccessWithPage(ctx, list, total, query.Page, query.PageSize)
}

// Ping 测试数据库连接。
// @Summary 测试数据库连接
// @Description 根据数据库资产 ID 读取绑定账号并测试连接连通性
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param id path int true "数据库资产ID"
// @Success 200 {object} result.Result
// @Router /api/v1/db/assets/{id}/ping [post]
// @Security ApiKeyAuth
func (c *DatabaseController) Ping(ctx *gin.Context) {
	assetID, ok := parseAssetID(ctx)
	if !ok {
		return
	}
	data, err := c.service.Ping(ctx, assetID)
	if err != nil {
		result.Failed(ctx, 500, "ping database failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// MySQLMonitor 获取 MySQL 监控大盘数据。
// @Summary 获取 MySQL 监控大盘
// @Description 根据数据库资产绑定的 Prometheus 账号自动匹配 target，并聚合 MySQL 关键指标
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param id path int true "数据库资产ID"
// @Param range query string false "时间范围，例如 6h、24h"
// @Param step query string false "采样步长，例如 1m、5m"
// @Param start query int false "开始时间，毫秒时间戳"
// @Param end query int false "结束时间，毫秒时间戳"
// @Success 200 {object} result.Result
// @Router /api/v1/db/assets/{id}/monitor/mysql [get]
// @Security ApiKeyAuth
func (c *DatabaseController) MySQLMonitor(ctx *gin.Context) {
	assetID, ok := parseAssetID(ctx)
	if !ok {
		return
	}
	var query dbmodel.MySQLMonitorQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid query params")
		return
	}
	data, err := c.service.MySQLMonitorDashboard(ctx.Request.Context(), assetID, query)
	if err != nil {
		result.Failed(ctx, 500, "query mysql monitor failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// PostgreSQLMonitor 获取 PostgreSQL 监控大盘数据。
// @Summary 获取 PostgreSQL 监控大盘
// @Description 根据数据库资产绑定的 Prometheus 账号自动匹配 target，并聚合 PostgreSQL 关键指标
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param id path int true "数据库资产ID"
// @Param range query string false "时间范围，例如 6h、24h"
// @Param step query string false "采样步长，例如 1m、5m"
// @Param start query int false "开始时间，毫秒时间戳"
// @Param end query int false "结束时间，毫秒时间戳"
// @Success 200 {object} result.Result
// @Router /api/v1/db/assets/{id}/monitor/postgresql [get]
// @Security ApiKeyAuth
func (c *DatabaseController) PostgreSQLMonitor(ctx *gin.Context) {
	assetID, ok := parseAssetID(ctx)
	if !ok {
		return
	}
	var query dbmodel.PostgreSQLMonitorQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid query params")
		return
	}
	data, err := c.service.PostgreSQLMonitorDashboard(ctx.Request.Context(), assetID, query)
	if err != nil {
		result.Failed(ctx, 500, "query postgresql monitor failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// RedisMonitor 获取 Redis 监控大盘数据。
// @Summary 获取 Redis 监控大盘
// @Description 根据数据库资产绑定的 Prometheus 账号自动匹配 target，并聚合 Redis 关键指标
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param id path int true "数据库资产ID"
// @Param range query string false "时间范围，例如 6h、24h"
// @Param step query string false "采样步长，例如 1m、5m"
// @Param start query int false "开始时间，毫秒时间戳"
// @Param end query int false "结束时间，毫秒时间戳"
// @Success 200 {object} result.Result
// @Router /api/v1/db/assets/{id}/monitor/redis [get]
// @Security ApiKeyAuth
func (c *DatabaseController) RedisMonitor(ctx *gin.Context) {
	assetID, ok := parseAssetID(ctx)
	if !ok {
		return
	}
	var query dbmodel.RedisMonitorQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid query params")
		return
	}
	data, err := c.service.RedisMonitorDashboard(ctx.Request.Context(), assetID, query)
	if err != nil {
		result.Failed(ctx, 500, "query redis monitor failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// SlowLogAnalysis 获取数据库慢日志聚合分析。
// @Summary 获取慢日志聚合分析
// @Description 根据数据库资产配置的 slowLogPath，登录关联主机读取慢日志尾部内容并按 SQL 指纹聚合排序
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param id path int true "数据库资产ID"
// @Param authId query int false "主机 SSH 凭据ID；主机绑定多个凭据时必传"
// @Param keyword query string false "关键字"
// @Param database query string false "数据库名"
// @Param user query string false "用户"
// @Param beginTime query string false "开始时间，格式 2006-01-02 15:04:05"
// @Param endTime query string false "结束时间，格式 2006-01-02 15:04:05"
// @Param sortBy query string false "排序字段：queryTimeTotal/count/queryTimeP95/rowsExaminedTotal 等"
// @Param sortOrder query string false "asc/desc"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param tailLines query int false "读取慢日志尾部行数，默认 10000，最大 50000"
// @Success 200 {object} result.Result
// @Router /api/v1/db/assets/{id}/slow-log [get]
// @Security ApiKeyAuth
func (c *DatabaseController) SlowLogAnalysis(ctx *gin.Context) {
	assetID, ok := parseAssetID(ctx)
	if !ok {
		return
	}
	var query dbmodel.SlowLogQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid query params")
		return
	}
	data, err := c.service.AnalyzeSlowLog(ctx, assetID, query)
	if err != nil {
		result.Failed(ctx, 500, "analyze slow log failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// CreateSlowLogTask 提交慢 SQL 异步分析任务。
// @Summary 提交慢 SQL 异步分析任务
// @Description 创建后台慢日志分析任务，立即返回 taskId；任务完成后结果只保留排序后的 Top 100
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param id path int true "数据库资产ID"
// @Param request body model.SlowLogQuery true "慢日志分析参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/assets/{id}/slow-log/tasks [post]
// @Security ApiKeyAuth
func (c *DatabaseController) CreateSlowLogTask(ctx *gin.Context) {
	assetID, ok := parseAssetID(ctx)
	if !ok {
		return
	}
	var query dbmodel.SlowLogQuery
	if err := ctx.ShouldBindJSON(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}
	data, err := c.service.StartSlowLogTask(ctx, assetID, query)
	if err != nil {
		result.Failed(ctx, 500, "create slow log task failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// UploadSlowLogFile 上传慢日志文件并异步分析。
// @Summary 上传慢日志文件并异步分析
// @Description 用户手动上传小于 150MB 的 MySQL 慢日志文件，后端保存为临时文件后创建异步分析任务
// @Tags 数据库管理
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "数据库资产ID"
// @Param file formData file true "慢日志文件，最大 150MB"
// @Param database formData string false "数据库名"
// @Param user formData string false "用户"
// @Param keyword formData string false "关键字"
// @Param beginTime formData string false "开始时间，格式 2006-01-02 15:04:05"
// @Param endTime formData string false "结束时间，格式 2006-01-02 15:04:05"
// @Param sortBy formData string false "排序字段：queryTimeTotal/count/queryTimeP95/rowsExaminedTotal 等"
// @Param sortOrder formData string false "asc/desc"
// @Success 200 {object} result.Result
// @Router /api/v1/db/assets/{id}/slow-log/upload [post]
// @Security ApiKeyAuth
func (c *DatabaseController) UploadSlowLogFile(ctx *gin.Context) {
	assetID, ok := parseAssetID(ctx)
	if !ok {
		return
	}

	const maxUploadRequestBytes = 151 * 1024 * 1024
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxUploadRequestBytes)

	file, err := ctx.FormFile("file")
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, "获取上传慢日志文件失败: "+err.Error())
		return
	}

	query := dbmodel.SlowLogQuery{
		Database:  strings.TrimSpace(ctx.PostForm("database")),
		User:      strings.TrimSpace(ctx.PostForm("user")),
		Keyword:   strings.TrimSpace(ctx.PostForm("keyword")),
		BeginTime: strings.TrimSpace(ctx.PostForm("beginTime")),
		EndTime:   strings.TrimSpace(ctx.PostForm("endTime")),
		SortBy:    strings.TrimSpace(ctx.PostForm("sortBy")),
		SortOrder: strings.TrimSpace(ctx.PostForm("sortOrder")),
	}
	if value := strings.TrimSpace(ctx.PostForm("authId")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			result.Failed(ctx, constant.INVALID_PARAMS, "authId must be a number")
			return
		}
		query.AuthID = uint(parsed)
	}
	if value := strings.TrimSpace(ctx.PostForm("tailLines")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			result.Failed(ctx, constant.INVALID_PARAMS, "tailLines must be a number")
			return
		}
		query.TailLines = parsed
	}

	data, err := c.service.StartUploadedSlowLogTask(ctx, assetID, file, query)
	if err != nil {
		result.Failed(ctx, 500, "upload slow log task failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListSlowLogTasks 查询慢 SQL 历史分析任务。
// @Summary 查询慢 SQL 历史分析任务
// @Description 分页查询某个数据库资产的慢 SQL 异步分析历史，不返回 Top100 明细
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param id path int true "数据库资产ID"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param status query string false "任务状态"
// @Param keyword query string false "关键字"
// @Success 200 {object} result.Result
// @Router /api/v1/db/assets/{id}/slow-log/tasks [get]
// @Security ApiKeyAuth
func (c *DatabaseController) ListSlowLogTasks(ctx *gin.Context) {
	assetID, ok := parseAssetID(ctx)
	if !ok {
		return
	}
	var query dbmodel.SlowLogTaskQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid query params")
		return
	}
	list, total, err := c.service.ListSlowLogTasks(assetID, query)
	if err != nil {
		result.Failed(ctx, 500, "query slow log tasks failed: "+err.Error())
		return
	}
	query.Page, query.PageSize = dbmodel.NormalizePage(query.Page, query.PageSize)
	result.SuccessWithPage(ctx, list, total, query.Page, query.PageSize)
}

// GetSlowLogTask 查询慢 SQL 异步分析任务。
// @Summary 查询慢 SQL 异步分析任务
// @Description 查询任务状态、进度和完成后的 Top 100 慢 SQL 聚合结果
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param id path int true "数据库资产ID"
// @Param taskId path string true "任务ID"
// @Success 200 {object} result.Result
// @Router /api/v1/db/assets/{id}/slow-log/tasks/{taskId} [get]
// @Security ApiKeyAuth
func (c *DatabaseController) GetSlowLogTask(ctx *gin.Context) {
	taskID := strings.TrimSpace(ctx.Param("taskId"))
	if taskID == "" {
		result.Failed(ctx, constant.INVALID_PARAMS, "taskId is required")
		return
	}
	data, err := c.service.GetSlowLogTask(taskID)
	if err != nil {
		result.Failed(ctx, 500, "query slow log task failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// DeleteSlowLogTask 删除慢 SQL 历史分析任务。
// @Summary 删除慢 SQL 历史分析任务
// @Description 删除指定慢 SQL 历史任务；pending/running 任务不允许删除
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param id path int true "数据库资产ID"
// @Param taskId path string true "任务ID"
// @Success 200 {object} result.Result
// @Router /api/v1/db/assets/{id}/slow-log/tasks/{taskId} [delete]
// @Security ApiKeyAuth
func (c *DatabaseController) DeleteSlowLogTask(ctx *gin.Context) {
	assetID, ok := parseAssetID(ctx)
	if !ok {
		return
	}
	taskID := strings.TrimSpace(ctx.Param("taskId"))
	if taskID == "" {
		result.Failed(ctx, constant.INVALID_PARAMS, "taskId is required")
		return
	}
	if err := c.service.DeleteSlowLogTask(assetID, taskID); err != nil {
		result.Failed(ctx, 500, "delete slow log task failed: "+err.Error())
		return
	}
	result.Success(ctx, nil)
}

// ClearSlowLogTasks 清空慢 SQL 历史分析任务。
// @Summary 清空慢 SQL 历史分析任务
// @Description 清空某个数据库资产下已完成或失败的慢 SQL 历史任务，pending/running 任务保留
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param id path int true "数据库资产ID"
// @Success 200 {object} result.Result
// @Router /api/v1/db/assets/{id}/slow-log/tasks [delete]
// @Security ApiKeyAuth
func (c *DatabaseController) ClearSlowLogTasks(ctx *gin.Context) {
	assetID, ok := parseAssetID(ctx)
	if !ok {
		return
	}
	count, err := c.service.ClearSlowLogTasks(assetID)
	if err != nil {
		result.Failed(ctx, 500, "clear slow log tasks failed: "+err.Error())
		return
	}
	result.Success(ctx, map[string]interface{}{"deleted": count})
}

// GetSlowLogTaskResult 查询慢 SQL 任务结果并按指定字段排序。
// @Summary 查询慢 SQL 任务结果并排序
// @Description 对已完成任务保存的 Top 100 结果按用户选择字段重新排序，不重新分析日志
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param id path int true "数据库资产ID"
// @Param taskId path string true "任务ID"
// @Param sortBy query string false "排序字段：count/queryTimeP95/queryTimeAvg/queryTimeMax/queryTimeTotal/lockTimeTotal/rowsExaminedTotal"
// @Param sortOrder query string false "asc/desc"
// @Success 200 {object} result.Result
// @Router /api/v1/db/assets/{id}/slow-log/tasks/{taskId}/result [get]
// @Security ApiKeyAuth
func (c *DatabaseController) GetSlowLogTaskResult(ctx *gin.Context) {
	taskID := strings.TrimSpace(ctx.Param("taskId"))
	if taskID == "" {
		result.Failed(ctx, constant.INVALID_PARAMS, "taskId is required")
		return
	}
	data, err := c.service.GetSlowLogTaskResult(taskID, ctx.Query("sortBy"), ctx.Query("sortOrder"))
	if err != nil {
		result.Failed(ctx, 500, "query slow log task result failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ExportSlowLogTaskCSV 导出慢 SQL 异步任务 Top 100 CSV。
// @Summary 导出慢 SQL 异步任务 Top 100 CSV
// @Description 任务完成后导出排序后的 Top 100 慢 SQL 聚合结果
// @Tags 数据库管理
// @Accept json
// @Produce text/csv
// @Param id path int true "数据库资产ID"
// @Param taskId path string true "任务ID"
// @Success 200 {file} file
// @Router /api/v1/db/assets/{id}/slow-log/tasks/{taskId}/export [get]
// @Security ApiKeyAuth
func (c *DatabaseController) ExportSlowLogTaskCSV(ctx *gin.Context) {
	taskID := strings.TrimSpace(ctx.Param("taskId"))
	if taskID == "" {
		result.Failed(ctx, constant.INVALID_PARAMS, "taskId is required")
		return
	}
	fileName := buildCSVFileName("slow_log_top100")
	ctx.Header("Content-Type", "text/csv; charset=utf-8")
	ctx.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	ctx.Status(http.StatusOK)
	if _, err := ctx.Writer.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return
	}
	if _, err := c.service.ExportSlowLogTaskCSV(taskID, ctx.Writer); err != nil {
		ctx.Header("Content-Disposition", "")
		result.Failed(ctx, 500, "export slow log task csv failed: "+err.Error())
		return
	}
}

// ExportSlowLogCSV 导出慢日志聚合分析 CSV。
// @Summary 导出慢日志聚合分析 CSV
// @Description 按当前慢日志筛选条件导出聚合后的慢 SQL 排序结果
// @Tags 数据库管理
// @Accept json
// @Produce text/csv
// @Param id path int true "数据库资产ID"
// @Success 200 {file} file
// @Router /api/v1/db/assets/{id}/slow-log/export [get]
// @Security ApiKeyAuth
func (c *DatabaseController) ExportSlowLogCSV(ctx *gin.Context) {
	assetID, ok := parseAssetID(ctx)
	if !ok {
		return
	}
	var query dbmodel.SlowLogQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid query params")
		return
	}

	fileName := buildCSVFileName("slow_log")
	ctx.Header("Content-Type", "text/csv; charset=utf-8")
	ctx.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	ctx.Status(http.StatusOK)
	if _, err := ctx.Writer.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return
	}
	if _, err := c.service.ExportSlowLogCSV(ctx, assetID, query, ctx.Writer); err != nil {
		ctx.Header("Content-Disposition", "")
		result.Failed(ctx, 500, "export slow log csv failed: "+err.Error())
		return
	}
}

// Tree 获取数据库资源树。
// @Summary 获取数据库资源树
// @Description 按资产、库、schema、表、字段等层级懒加载数据库资源树
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param assetId query int true "数据库资产ID"
// @Param level query string false "树层级 root/database/table/view"
// @Param database query string false "数据库名"
// @Param schema query string false "Schema 名"
// @Param objectName query string false "对象名"
// @Param objectType query string false "对象类型"
// @Param keyword query string false "关键字"
// @Success 200 {object} result.Result
// @Router /api/v1/db/tree [get]
// @Security ApiKeyAuth
func (c *DatabaseController) Tree(ctx *gin.Context) {
	var query dbmodel.TreeQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid query params")
		return
	}
	if query.AssetID == 0 {
		result.Failed(ctx, constant.INVALID_PARAMS, "assetId is required")
		return
	}
	data, err := c.service.Tree(ctx, query)
	if err != nil {
		result.Failed(ctx, 500, "query database tree failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// Columns 获取数据库对象字段。
// @Summary 获取数据库对象字段
// @Description 查询指定数据库对象的字段信息，包括类型、主键、是否为空、默认值和注释
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param assetId query int true "数据库资产ID"
// @Param database query string true "数据库名"
// @Param schema query string false "Schema 名"
// @Param objectName query string true "对象名"
// @Param objectType query string false "对象类型"
// @Success 200 {object} result.Result
// @Router /api/v1/db/objects/columns [get]
// @Security ApiKeyAuth
func (c *DatabaseController) Columns(ctx *gin.Context) {
	req, ok := bindObjectQuery(ctx)
	if !ok {
		return
	}
	data, err := c.service.Columns(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "query columns failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// Indexes 获取数据库对象索引。
// @Summary 获取数据库对象索引
// @Description 查询指定数据库对象的索引信息，包括索引名、字段、唯一性和索引类型
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param assetId query int true "数据库资产ID"
// @Param database query string true "数据库名"
// @Param schema query string false "Schema 名"
// @Param objectName query string true "对象名"
// @Param objectType query string false "对象类型"
// @Success 200 {object} result.Result
// @Router /api/v1/db/objects/indexes [get]
// @Security ApiKeyAuth
func (c *DatabaseController) Indexes(ctx *gin.Context) {
	req, ok := bindObjectQuery(ctx)
	if !ok {
		return
	}
	data, err := c.service.Indexes(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "query indexes failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// DDL 获取数据库对象 DDL。
// @Summary 获取数据库对象 DDL
// @Description 查询指定数据库对象的建表或对象定义语句，第一版只读展示
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param assetId query int true "数据库资产ID"
// @Param database query string true "数据库名"
// @Param schema query string false "Schema 名"
// @Param objectName query string true "对象名"
// @Param objectType query string false "对象类型"
// @Success 200 {object} result.Result
// @Router /api/v1/db/objects/ddl [get]
// @Security ApiKeyAuth
func (c *DatabaseController) DDL(ctx *gin.Context) {
	req, ok := bindObjectQuery(ctx)
	if !ok {
		return
	}
	ddl, err := c.service.DDL(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "query ddl failed: "+err.Error())
		return
	}
	result.Success(ctx, dbmodel.DDLResponse{DDL: ddl})
}

// Query 查询表数据。
// @Summary 查询表数据
// @Description 分页查询指定数据库表数据，支持字段选择、筛选和排序
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param request body model.DataQueryRequest true "查询参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/data/query [post]
// @Security ApiKeyAuth
func (c *DatabaseController) Query(ctx *gin.Context) {
	var req dbmodel.DataQueryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}
	data, err := c.service.Query(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "query table data failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ExportCSV 导出表数据 CSV。
// @Summary 导出表数据 CSV
// @Description 按当前查询条件导出指定数据库表数据为 CSV 文件；默认只导出当前页查询结果，传 exportAll=true 时最多导出 10000 行
// @Tags 数据库管理
// @Accept json
// @Produce text/csv
// @Param request body model.DataQueryRequest true "导出参数"
// @Success 200 {file} file
// @Router /api/v1/db/data/export [post]
// @Security ApiKeyAuth
func (c *DatabaseController) ExportCSV(ctx *gin.Context) {
	var req dbmodel.DataQueryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}

	fileName := buildCSVFileName(req.ObjectName)
	ctx.Header("Content-Type", "text/csv; charset=utf-8")
	ctx.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	ctx.Status(http.StatusOK)
	if _, err := ctx.Writer.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return
	}
	if _, err := c.service.ExportCSV(ctx, req, ctx.Writer); err != nil {
		ctx.Header("Content-Disposition", "")
		result.Failed(ctx, 500, "export csv failed: "+err.Error())
		return
	}
}

// Create 新增表数据。
// @Summary 新增表数据
// @Description 向指定数据库表新增一行数据，字段和值由 values 指定
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param request body model.DataCreateRequest true "新增参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/data/create [post]
// @Security ApiKeyAuth
func (c *DatabaseController) Create(ctx *gin.Context) {
	var req dbmodel.DataCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}
	data, err := c.service.Create(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "create row failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// Update 更新表数据。
// @Summary 更新表数据
// @Description 按主键条件更新指定数据库表的一行数据，不允许无条件更新或更新主键字段
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param request body model.DataUpdateRequest true "更新参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/data/update [post]
// @Security ApiKeyAuth
func (c *DatabaseController) Update(ctx *gin.Context) {
	var req dbmodel.DataUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}
	data, err := c.service.Update(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "update row failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// Delete 删除表数据。
// @Summary 删除表数据
// @Description 按主键条件删除指定数据库表的一行数据，不允许无条件删除
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param request body model.DataDeleteRequest true "删除参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/data/delete [post]
// @Security ApiKeyAuth
func (c *DatabaseController) Delete(ctx *gin.Context) {
	var req dbmodel.DataDeleteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}
	data, err := c.service.Delete(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "delete row failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ExecuteSQL 执行只读 SQL。
// @Summary 执行只读 SQL
// @Description 在指定数据库资产上执行 SELECT/WITH 只读 SQL；执行前会基于 EXPLAIN 估算耗时，超过阈值时返回 requireConfirm=true，前端确认后携带 confirmed=true 再次请求
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param request body model.SQLExecuteRequest true "SQL执行参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/sql/execute [post]
// @Security ApiKeyAuth
func (c *DatabaseController) ExecuteSQL(ctx *gin.Context) {
	var req dbmodel.SQLExecuteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}
	data, err := c.service.ExecuteSQL(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "execute sql failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// Completion 获取 SQL 自动补全元数据。
// @Summary 获取 SQL 自动补全元数据
// @Description 返回 SQL 编辑器自动补全所需的关键字、库、表和字段元数据
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param assetId query int true "数据库资产ID"
// @Param database query string false "数据库名"
// @Param schema query string false "Schema 名"
// @Param objectName query string false "对象名"
// @Param keyword query string false "关键字"
// @Success 200 {object} result.Result
// @Router /api/v1/db/sql/completion [get]
// @Security ApiKeyAuth
func (c *DatabaseController) Completion(ctx *gin.Context) {
	var query dbmodel.SQLCompletionQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid query params")
		return
	}
	if query.AssetID == 0 {
		result.Failed(ctx, constant.INVALID_PARAMS, "assetId is required")
		return
	}
	data, err := c.service.Completion(ctx, query)
	if err != nil {
		result.Failed(ctx, 500, "query sql completion failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// SQLHistory 获取 SQL 执行历史。
// @Summary 获取 SQL 执行历史
// @Description 分页查询 SQL 查询窗口的只读 SQL 执行历史
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param assetId query int false "数据库资产ID"
// @Param databaseName query string false "数据库名"
// @Param success query bool false "是否成功"
// @Param operatorName query string false "操作人"
// @Param beginTime query string false "开始时间"
// @Param endTime query string false "结束时间"
// SQLAssistant AI SQL 助手
// @Summary AI SQL助手
// @Description 基于 AIOps 模型供应商能力，结合当前数据库元数据完成自然语言转 SQL、SQL 优化和 SQL 解释；后端只生成建议，不直接执行 SQL
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param request body model.SQLAssistantRequest true "AI SQL助手参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/ai/sql-assistant [post]
// @Security ApiKeyAuth
func (c *DatabaseController) SQLAssistant(ctx *gin.Context) {
	var req dbmodel.SQLAssistantRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}
	data, err := c.service.SQLAssistant(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "ai sql assistant failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListAITools 获取数据库 AI 工具清单。
// @Summary 获取数据库 AI 工具清单
// @Description 返回 AI 助手可调用的数据库工具定义
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/db/ai/tools [get]
// @Security ApiKeyAuth
func (c *DatabaseController) ListAITools(ctx *gin.Context) {
	result.Success(ctx, c.service.ListAITools())
}

// RunAITool 执行数据库 AI 工具。
// @Summary 执行数据库 AI 工具
// @Description 根据 tool 名调用数据库对象搜索、结构查询、只读 SQL、执行计划、监控快照、慢 SQL Top、实例健康等工具
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param request body model.DBAIToolRequest true "AI工具调用参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/ai/tools/run [post]
// @Security ApiKeyAuth
func (c *DatabaseController) RunAITool(ctx *gin.Context) {
	var req dbmodel.DBAIToolRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}
	data, err := c.service.RunAITool(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "run db ai tool failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// DBAIChat 数据库运维 AI-Agent 助手。
// @Summary 数据库运维 AI-Agent 助手
// @Description 根据用户数据库问题自动选择只读工具收集证据，并调用大模型生成诊断/分析回答
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param request body model.DBAIChatRequest true "数据库AI-Agent提问参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/ai/chat [post]
// @Security ApiKeyAuth
func (c *DatabaseController) DBAIChat(ctx *gin.Context) {
	var req dbmodel.DBAIChatRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}
	data, err := c.service.DBAIChat(ctx, req)
	if err != nil {
		result.Failed(ctx, 500, "database ai agent failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// SQLHistory 获取 SQL 执行历史。
// @Summary 获取 SQL 执行历史
// @Description 分页查询 SQL 查询窗口的只读 SQL 执行历史
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param assetId query int false "数据库资产ID"
// @Param databaseName query string false "数据库名"
// @Param success query bool false "是否成功"
// @Param operatorName query string false "操作人"
// @Param beginTime query string false "开始时间"
// @Param endTime query string false "结束时间"
// @Success 200 {object} result.Result
// @Router /api/v1/db/sql/history [get]
// @Security ApiKeyAuth
func (c *DatabaseController) SQLHistory(ctx *gin.Context) {
	var query dbmodel.AuditQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid query params")
		return
	}
	list, total, err := c.service.SQLHistory(query)
	if err != nil {
		result.Failed(ctx, 500, "query sql history failed: "+err.Error())
		return
	}
	query.Page, query.PageSize = dbmodel.NormalizePage(query.Page, query.PageSize)
	result.SuccessWithPage(ctx, list, total, query.Page, query.PageSize)
}

// ListAudits 获取数据库操作审计列表。
// @Summary 获取数据库操作审计列表
// @Description 分页查询数据库管理模块的连接、元数据、查询和变更操作审计记录
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param assetId query int false "数据库资产ID"
// @Param engine query int false "数据库类型"
// @Param databaseName query string false "数据库名"
// @Param objectName query string false "对象名"
// @Param action query string false "操作类型"
// @Param success query bool false "是否成功"
// @Param operatorName query string false "操作人"
// @Param beginTime query string false "开始时间"
// @Param endTime query string false "结束时间"
// @Success 200 {object} result.Result
// @Router /api/v1/db/audit/list [get]
// @Security ApiKeyAuth
func (c *DatabaseController) ListAudits(ctx *gin.Context) {
	var query dbmodel.AuditQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid query params")
		return
	}
	list, total, err := c.service.ListAudits(query)
	if err != nil {
		result.Failed(ctx, 500, "query audit list failed: "+err.Error())
		return
	}
	query.Page, query.PageSize = dbmodel.NormalizePage(query.Page, query.PageSize)
	result.SuccessWithPage(ctx, list, total, query.Page, query.PageSize)
}

// AuditInfo 获取数据库操作审计详情。
// @Summary 获取数据库操作审计详情
// @Description 根据审计记录 ID 查询数据库操作审计详情
// @Tags 数据库管理
// @Accept json
// @Produce json
// @Param id query int false "审计记录ID"
// @Success 200 {object} result.Result
// @Router /api/v1/db/audit/info [get]
// @Security ApiKeyAuth
func (c *DatabaseController) AuditInfo(ctx *gin.Context) {
	value := ctx.Query("id")
	if value == "" {
		value = ctx.Param("id")
	}
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil || id == 0 {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid id")
		return
	}
	data, err := c.service.GetAudit(uint(id))
	if err != nil {
		result.Failed(ctx, 500, "query audit info failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

func bindObjectQuery(ctx *gin.Context) (dbmodel.ObjectRequest, bool) {
	var req dbmodel.ObjectRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid query params")
		return req, false
	}
	if req.AssetID == 0 {
		result.Failed(ctx, constant.INVALID_PARAMS, "assetId is required")
		return req, false
	}
	if req.Database == "" {
		result.Failed(ctx, constant.INVALID_PARAMS, "database is required")
		return req, false
	}
	if req.ObjectName == "" {
		result.Failed(ctx, constant.INVALID_PARAMS, "objectName is required")
		return req, false
	}
	return req, true
}

func parseAssetID(ctx *gin.Context) (uint, bool) {
	value := ctx.Param("id")
	if value == "" {
		value = ctx.Query("assetId")
	}
	if value == "" {
		result.Failed(ctx, constant.INVALID_PARAMS, "assetId is required")
		return 0, false
	}
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil || id == 0 {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid assetId")
		return 0, false
	}
	return uint(id), true
}

func buildCSVFileName(objectName string) string {
	name := strings.TrimSpace(objectName)
	if name == "" {
		name = "db_export"
	}
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	name = replacer.Replace(name)
	return fmt.Sprintf("%s_%s.csv", name, time.Now().Format("20060102150405"))
}
