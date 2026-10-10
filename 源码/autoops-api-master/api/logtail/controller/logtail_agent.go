package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	logtailmodel "dodevops-api/api/logtail/model"
	"dodevops-api/common/config"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

// GetVectorPackageInfo 获取当前默认 Vector 包信息。
// @Summary 获取当前默认 Vector 包信息
// @Description 根据配置中的 packagePath 动态计算 Vector 包下载地址、大小、SHA256 和版本信息。
// @Tags 日志管理
// @Produce json
// @Success 200 {object} result.Result "查询成功"
// @Router /api/v1/logtail/vector/package [get]
// @Security ApiKeyAuth
func (c *LogtailController) GetVectorPackageInfo(ctx *gin.Context) {
	result.Success(ctx, c.service.GetVectorPackageInfo())
}

// UploadVectorPackage 上传当前默认 Vector 包。
// @Summary 上传当前默认 Vector 包
// @Description 上传默认 Vector 二进制到后端临时目录后创建后台保存任务，支持 replace=true 覆盖已有文件。
// @Tags 日志管理
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Vector 二进制文件"
// @Param replace query bool false "是否覆盖已有文件"
// @Success 200 {object} result.Result "任务创建成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/vector/package [post]
// @Security ApiKeyAuth
func (c *LogtailController) UploadVectorPackage(ctx *gin.Context) {
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		result.Failed(ctx, 400, "请上传 Vector 文件")
		return
	}
	if fileHeader.Size <= 0 {
		result.Failed(ctx, 400, "上传文件不能为空文件")
		return
	}
	replaceRaw := strings.TrimSpace(ctx.Query("replace"))
	if replaceRaw == "" {
		replaceRaw = strings.TrimSpace(ctx.PostForm("replace"))
	}
	replace, _ := strconv.ParseBool(replaceRaw)

	tempDir := filepath.Join(os.TempDir(), "aiops-vector-package-uploads")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		result.Failed(ctx, 500, "创建上传临时目录失败: "+err.Error())
		return
	}
	tempFileName := fmt.Sprintf("%d-%s", time.Now().UnixNano(), sanitizeVectorUploadFileName(fileHeader.Filename))
	tempFilePath := filepath.Join(tempDir, tempFileName)
	if err := ctx.SaveUploadedFile(fileHeader, tempFilePath); err != nil {
		result.Failed(ctx, 500, "保存上传临时文件失败: "+err.Error())
		return
	}
	if err := os.Chmod(tempFilePath, 0o644); err != nil {
		_ = os.Remove(tempFilePath)
		result.Failed(ctx, 500, "设置上传临时文件权限失败: "+err.Error())
		return
	}

	task, err := c.service.CreateVectorPackageUploadTask(tempFilePath, fileHeader.Filename, replace)
	if err != nil {
		_ = os.Remove(tempFilePath)
		result.Failed(ctx, 500, "创建 Vector 包上传任务失败: "+err.Error())
		return
	}
	result.Success(ctx, gin.H{
		"task":    task,
		"message": "文件已接收，后台上传任务已启动",
	})
}

// GetVectorPackageUploadTaskStatus 查询默认 Vector 包上传任务状态。
// @Summary 查询默认 Vector 包上传任务状态
// @Description 查询后台 Vector 包保存任务的实时进度和结果。
// @Tags 日志管理
// @Produce json
// @Param taskId path string true "上传任务ID"
// @Success 200 {object} result.Result "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 404 {object} result.Result "任务不存在"
// @Router /api/v1/logtail/vector/package/upload/task/{taskId} [get]
// @Security ApiKeyAuth
func (c *LogtailController) GetVectorPackageUploadTaskStatus(ctx *gin.Context) {
	taskID := strings.TrimSpace(ctx.Param("taskId"))
	if taskID == "" {
		result.Failed(ctx, 400, "任务ID不能为空")
		return
	}
	task, err := c.service.GetVectorPackageUploadTask(taskID)
	if err != nil {
		result.Failed(ctx, 404, err.Error())
		return
	}
	result.Success(ctx, gin.H{"task": task})
}

// DeleteVectorPackage 删除当前默认 Vector 包。
// @Summary 删除当前默认 Vector 包
// @Description 删除 logtail.vector.packagePath 指向的当前默认 Vector 二进制文件。
// @Tags 日志管理
// @Produce json
// @Success 200 {object} result.Result "删除成功"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/vector/package [delete]
// @Security ApiKeyAuth
func (c *LogtailController) DeleteVectorPackage(ctx *gin.Context) {
	data, err := c.service.DeleteVectorPackage()
	if err != nil {
		result.Failed(ctx, 500, "删除 Vector 包失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListVectorAgents 查询 Vector 主机总览。
// @Summary 查询 Vector 主机总览
// @Description 用于 Vector 管理 Tab，默认返回全部 Agent，包含已安装、未安装和异常 Vector。
// @Tags 日志管理
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "主机名或IP关键字"
// @Param onlineStatus query int false "Agent在线状态：1在线，2离线"
// @Param vectorStatus query string false "Vector状态：not_installed/installed/running/error"
// @Param configStatus query string false "配置状态：pending/applied/failed"
// @Success 200 {object} result.Result "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/vector/agents [get]
// @Security ApiKeyAuth
func (c *LogtailController) ListVectorAgents(ctx *gin.Context) {
	var query logtailmodel.LogAgentQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	items, total, normalized, err := c.service.ListLogAgents(query)
	if err != nil {
		result.Failed(ctx, 500, "查询Vector主机失败: "+err.Error())
		return
	}
	result.SuccessWithPage(ctx, items, total, normalized.Page, normalized.PageSize)
}

// PreviewVectorAggregatorConfig 预览 Kafka 到 VictoriaLogs 的 Vector Aggregator 配置。
// @Summary 预览 Vector Aggregator 配置
// @Description 根据当前 logtail.kafka 与 victorialogs 配置渲染 Kafka -> VictoriaLogs 的 vector.yaml。
// @Tags 日志管理
// @Produce json
// @Success 200 {object} result.Result "查询成功"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/vector/aggregator/config-preview [get]
// @Security ApiKeyAuth
func (c *LogtailController) PreviewVectorAggregatorConfig(ctx *gin.Context) {
	data, err := c.service.PreviewVectorAggregatorConfig()
	if err != nil {
		result.Failed(ctx, 500, "预览Vector Aggregator配置失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListAgents 查询日志采集 Agent 列表。
// @Summary 查询日志采集 Agent 列表
// @Description 聚合 CMDB Agent、主机资产和 Vector 运行状态。
// @Tags 日志管理
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "主机名或IP关键字"
// @Param onlineStatus query int false "Agent在线状态：1在线，2离线"
// @Param vectorStatus query string false "Vector状态：not_installed/installed/running/error"
// @Param configStatus query string false "配置状态：pending/applied/failed"
// @Success 200 {object} result.Result "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/agents [get]
// @Security ApiKeyAuth
func (c *LogtailController) ListAgents(ctx *gin.Context) {
	var query logtailmodel.LogAgentQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	items, total, normalized, err := c.service.ListLogAgents(query)
	if err != nil {
		result.Failed(ctx, 500, "查询日志采集Agent失败: "+err.Error())
		return
	}
	result.SuccessWithPage(ctx, items, total, normalized.Page, normalized.PageSize)
}

// GetAgent 获取日志采集 Agent 详情。
// @Summary 获取日志采集 Agent 详情
// @Description 根据 Agent ID 查询基础 Agent、主机和 Vector 状态。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "Agent ID"
// @Success 200 {object} result.Result "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/agents/{id} [get]
// @Security ApiKeyAuth
func (c *LogtailController) GetAgent(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "Agent ID不能为空")
		return
	}
	data, err := c.service.GetLogAgent(id)
	if err != nil {
		result.Failed(ctx, 500, "查询日志采集Agent失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// PreviewAgentConfig 预览 Agent 的 Vector 配置。
// @Summary 预览 Agent 的 Vector 配置
// @Description 根据当前 Agent 命中的采集任务渲染最终 vector.yaml。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "Agent ID"
// @Success 200 {object} result.Result "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/agents/{id}/config-preview [get]
// @Security ApiKeyAuth
func (c *LogtailController) PreviewAgentConfig(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "Agent ID不能为空")
		return
	}
	data, err := c.service.PreviewAgentConfig(id)
	if err != nil {
		result.Failed(ctx, 500, "预览Vector配置失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListAgentCollectTasks 查询 Agent 命中的采集任务。
// @Summary 查询 Agent 命中的采集任务
// @Description 查询指定 Agent 当前会执行的日志采集任务。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "Agent ID"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param projectId query uint false "项目ID"
// @Param storeId query uint false "日志库ID"
// @Param keyword query string false "任务名称或路径关键字"
// @Param collectType query string false "采集类型"
// @Param status query int false "状态"
// @Success 200 {object} result.Result "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/agents/{id}/collect-tasks [get]
// @Security ApiKeyAuth
func (c *LogtailController) ListAgentCollectTasks(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "Agent ID不能为空")
		return
	}
	var query logtailmodel.LogCollectTaskQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	items, total, normalized, err := c.service.ListAgentCollectTasks(id, query)
	if err != nil {
		result.Failed(ctx, 500, "查询Agent采集任务失败: "+err.Error())
		return
	}
	result.SuccessWithPage(ctx, items, total, normalized.Page, normalized.PageSize)
}

// CreateAgentOperation 创建 Agent 操作记录。
// @Summary 创建 Agent 操作记录
// @Description 创建安装、校验、重载、重启 Vector 等操作记录，执行面由自研 Agent 或资产模块消费。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param id path uint true "Agent ID"
// @Param data body logtailmodel.LogAgentOperationRequest true "操作参数"
// @Success 200 {object} result.Result "创建成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/agents/{id}/operations [post]
// @Security ApiKeyAuth
func (c *LogtailController) CreateAgentOperation(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "Agent ID不能为空")
		return
	}
	var req logtailmodel.LogAgentOperationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	data, err := c.service.CreateAgentOperation(id, req)
	if err != nil {
		result.Failed(ctx, 500, "创建Agent操作失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ListAgentOperations 查询 Agent 操作记录。
// @Summary 查询 Agent 操作记录
// @Description 查询安装、校验、重载、重启 Vector 等操作历史。
// @Tags 日志管理
// @Produce json
// @Param id path uint true "Agent ID"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param opType query string false "操作类型"
// @Param opStatus query string false "操作状态"
// @Success 200 {object} result.Result "查询成功"
// @Failure 400 {object} result.Result "参数错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/agents/{id}/operations [get]
// @Security ApiKeyAuth
func (c *LogtailController) ListAgentOperations(ctx *gin.Context) {
	id := parseIDParam(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "Agent ID不能为空")
		return
	}
	var query logtailmodel.LogAgentOperationQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	items, total, normalized, err := c.service.ListAgentOperations(id, query)
	if err != nil {
		result.Failed(ctx, 500, "查询Agent操作记录失败: "+err.Error())
		return
	}
	result.SuccessWithPage(ctx, items, total, normalized.Page, normalized.PageSize)
}

// GetRuntimeConfig Agent 拉取 Vector 配置。
// @Summary Agent 拉取 Vector 配置
// @Description 自研 Agent 根据 agentId 或 hostId 拉取当前应应用的 vector.yaml。
// @Tags 日志管理
// @Produce json
// @Param token query string false "Agent Token"
// @Param agentId query uint false "Agent ID"
// @Param hostId query uint false "Host ID"
// @Param currentVersion query int false "当前配置版本"
// @Param currentHash query string false "当前配置Hash"
// @Success 200 {object} result.Result "查询成功"
// @Failure 401 {object} result.Result "Token错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/agent/config [get]
func (c *LogtailController) GetRuntimeConfig(ctx *gin.Context) {
	if !validateAgentToken(ctx, "") {
		result.Failed(ctx, 401, "Token 验证失败")
		return
	}
	var req logtailmodel.LogAgentRuntimeRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	data, err := c.service.GetRuntimeConfig(req)
	if err != nil {
		result.Failed(ctx, 500, "拉取Vector配置失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ClaimRuntimeOperations Agent 拉取待执行操作。
// @Summary Agent 拉取待执行操作
// @Description 自研 Agent 拉取 pending 操作，后端会将操作标记为 running。
// @Tags 日志管理
// @Produce json
// @Param token query string false "Agent Token"
// @Param agentId query uint false "Agent ID"
// @Param hostId query uint false "Host ID"
// @Param limit query int false "最多拉取数量"
// @Success 200 {object} result.Result "查询成功"
// @Failure 401 {object} result.Result "Token错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/agent/operations [get]
func (c *LogtailController) ClaimRuntimeOperations(ctx *gin.Context) {
	if !validateAgentToken(ctx, "") {
		result.Failed(ctx, 401, "Token 验证失败")
		return
	}
	var req logtailmodel.LogAgentRuntimeRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		result.Failed(ctx, 400, "参数错误")
		return
	}
	data, err := c.service.ClaimRuntimeOperations(req)
	if err != nil {
		result.Failed(ctx, 500, "拉取Agent操作失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ReportRuntimeStatus Agent 上报 Vector 状态。
// @Summary Agent 上报 Vector 状态
// @Description 自研 Agent 上报 Vector 运行状态、配置版本和错误信息。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param data body logtailmodel.LogAgentStatusReportRequest true "状态上报"
// @Success 200 {object} result.Result "上报成功"
// @Failure 401 {object} result.Result "Token错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/agent/status [post]
func (c *LogtailController) ReportRuntimeStatus(ctx *gin.Context) {
	var req logtailmodel.LogAgentStatusReportRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	if !validateAgentToken(ctx, req.Token) {
		result.Failed(ctx, 401, "Token 验证失败")
		return
	}
	data, err := c.service.ReportRuntimeStatus(req)
	if err != nil {
		result.Failed(ctx, 500, "上报Vector状态失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ReportRuntimeOperation Agent 上报操作执行结果。
// @Summary Agent 上报操作执行结果
// @Description 自研 Agent 上报安装、校验、重载、重启 Vector 等操作结果。
// @Tags 日志管理
// @Accept json
// @Produce json
// @Param operationId path uint true "操作ID"
// @Param data body logtailmodel.LogAgentOperationReportRequest true "操作结果"
// @Success 200 {object} result.Result "上报成功"
// @Failure 401 {object} result.Result "Token错误"
// @Failure 500 {object} result.Result "服务器错误"
// @Router /api/v1/logtail/agent/operations/{operationId}/report [post]
func (c *LogtailController) ReportRuntimeOperation(ctx *gin.Context) {
	operationID := parseUintParam(ctx, "operationId")
	if operationID == 0 {
		result.Failed(ctx, 400, "操作ID不能为空")
		return
	}
	var req logtailmodel.LogAgentOperationReportRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	if !validateAgentToken(ctx, req.Token) {
		result.Failed(ctx, 401, "Token 验证失败")
		return
	}
	data, err := c.service.ReportRuntimeOperation(operationID, req)
	if err != nil {
		result.Failed(ctx, 500, "上报Agent操作结果失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

func validateAgentToken(ctx *gin.Context, bodyToken string) bool {
	expectedToken := ""
	if config.Config != nil {
		expectedToken = strings.TrimSpace(config.Config.Monitor.Agent.HeartbeatToken)
	}
	if expectedToken == "" {
		expectedToken = "agent-heartbeat-token-2024"
	}
	token := strings.TrimSpace(bodyToken)
	if token == "" {
		token = strings.TrimSpace(ctx.Query("token"))
	}
	if token == "" {
		token = strings.TrimSpace(ctx.GetHeader("X-Agent-Token"))
	}
	if token == "" {
		auth := strings.TrimSpace(ctx.GetHeader("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			token = strings.TrimSpace(auth[7:])
		}
	}
	return token != "" && token == expectedToken
}

func parseUintParam(ctx *gin.Context, name string) uint {
	value := ctx.Param(name)
	id, _ := strconv.Atoi(value)
	if id <= 0 {
		return 0
	}
	return uint(id)
}

func sanitizeVectorUploadFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "vector"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	name = replacer.Replace(name)
	name = strings.TrimSpace(name)
	if name == "" {
		return "vector"
	}
	return name
}
