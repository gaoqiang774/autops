package controller

import (
	"net/http"
	"strconv"
	"strings"

	"dodevops-api/api/cmdb/model"
	"dodevops-api/api/cmdb/service"
	"dodevops-api/common/config"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

type AgentController struct {
	agentService service.AgentServiceInterface
}

func NewAgentController() *AgentController {
	return &AgentController{
		agentService: service.NewAgentService(),
	}
}

func (c *AgentController) buildAgentListDto(ctx *gin.Context) *model.AgentListDto {
	dto := &model.AgentListDto{}

	if hostIDStr := ctx.Query("hostId"); hostIDStr != "" {
		if hostID, err := strconv.ParseUint(hostIDStr, 10, 32); err == nil {
			dto.HostID = uint(hostID)
		}
	}

	dto.HostName = ctx.Query("hostName")
	dto.SSHIP = ctx.Query("sshIp")
	if dto.SSHIP == "" {
		dto.SSHIP = ctx.Query("ip")
	}
	dto.Version = strings.TrimSpace(ctx.Query("version"))

	if statusStr := ctx.Query("status"); statusStr != "" {
		if status, err := strconv.Atoi(statusStr); err == nil {
			dto.Status = status
		}
	}

	if onlineStatusStr := ctx.Query("onlineStatus"); onlineStatusStr != "" {
		if onlineStatus, err := strconv.Atoi(onlineStatusStr); err == nil {
			dto.OnlineStatus = onlineStatus
		}
	}

	if pageStr := ctx.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil {
			dto.Page = page
		}
	}

	if pageSizeStr := ctx.Query("pageSize"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil {
			dto.PageSize = pageSize
		}
	}

	if dto.Page <= 0 {
		dto.Page = 1
	}
	if dto.PageSize <= 0 {
		dto.PageSize = 10
	}

	return dto
}

func (c *AgentController) buildAgentCandidateListDto(ctx *gin.Context) *model.AgentCandidateListDto {
	dto := &model.AgentCandidateListDto{
		Keyword:        strings.TrimSpace(ctx.Query("keyword")),
		DeployableOnly: true,
	}
	if dto.Keyword == "" {
		dto.Keyword = strings.TrimSpace(ctx.Query("name"))
	}
	if dto.Keyword == "" {
		dto.Keyword = strings.TrimSpace(ctx.Query("ip"))
	}

	if groupIDStr := ctx.Query("groupId"); groupIDStr != "" {
		if groupID, err := strconv.ParseUint(groupIDStr, 10, 32); err == nil {
			dto.GroupID = uint(groupID)
		}
	}
	if pageStr := ctx.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil {
			dto.Page = page
		}
	}
	if pageSizeStr := ctx.Query("pageSize"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil {
			dto.PageSize = pageSize
		}
	}
	if deployableOnly := strings.TrimSpace(ctx.Query("deployableOnly")); deployableOnly != "" {
		dto.DeployableOnly = deployableOnly == "true" || deployableOnly == "1"
	}
	if dto.Page <= 0 {
		dto.Page = 1
	}
	if dto.PageSize <= 0 {
		dto.PageSize = 10
	}

	return dto
}

func (c *AgentController) buildAgentCandidateGroupDto(ctx *gin.Context) *model.AgentCandidateGroupDto {
	dto := &model.AgentCandidateGroupDto{
		Keyword: strings.TrimSpace(ctx.Query("keyword")),
	}
	if dto.Keyword == "" {
		dto.Keyword = strings.TrimSpace(ctx.Query("name"))
	}
	if dto.Keyword == "" {
		dto.Keyword = strings.TrimSpace(ctx.Query("ip"))
	}
	return dto
}

// DeployAgent 部署 agent 到指定主机，支持单个或多个。
// @Summary 部署 agent 到指定主机
// @Description 默认自动编译 agent 二进制文件并拷贝启动；deployMode=offline 时从 upload/agent 选择预编译二进制，按主机分组自动生成配置后拷贝启动。单个主机传 [hostId]，多个主机传 [hostId1,hostId2,hostId3]
// @Tags 监控
// @Accept json
// @Produce json
// @Param request body model.BatchDeployAgentDto true "部署参数"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/deploy [post]
// @Security ApiKeyAuth
func (c *AgentController) DeployAgent(ctx *gin.Context) {
	var dto model.BatchDeployAgentDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if len(dto.HostIDs) == 1 {
		c.agentService.DeployAgentWithOptions(ctx, dto.HostIDs[0], dto.Version, dto.DeployOptions())
	} else {
		c.agentService.BatchDeployAgentWithOptions(ctx, &dto, dto.DeployOptions())
	}
}

// GetOfflineAssets returns files under upload/agent/; offline deployment selects binaries,
// while any JSON files are listed only for compatibility and are not uploaded by the UI.
func (c *AgentController) GetOfflineAssets(ctx *gin.Context) {
	c.agentService.GetOfflineAssets(ctx)
}

// UploadOfflineAgentBinary uploads a pre-built Agent binary to upload/agent/.
// @Summary 上传离线 Agent 二进制
// @Description 上传预编译的 x86_64 或 arm64 Agent 二进制，服务端保存到 upload/agent/；配置文件由部署任务自动生成
// @Tags 监控
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Agent二进制文件"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/offline-assets/upload [post]
// @Security ApiKeyAuth
func (c *AgentController) UploadOfflineAgentBinary(ctx *gin.Context) {
	c.agentService.UploadOfflineAgentBinary(ctx)
}

// CheckOfflineAssets reports whether x86_64 and arm64 packages have been uploaded.
// @Summary 检查离线 Agent 制品
// @Description 检查 upload/agent/ 下是否同时存在 x86_64 与 arm64 二进制
// @Tags 监控
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/offline-assets/check [get]
// @Security ApiKeyAuth
func (c *AgentController) CheckOfflineAssets(ctx *gin.Context) {
	c.agentService.CheckOfflineAssets(ctx)
}

// PrecheckOfflineDeployment verifies target CPU architectures before creating deployment tasks.
// @Summary 检查离线 Agent 部署条件
// @Description 通过 SSH 执行 uname -m，并校验每台主机存在对应架构的离线二进制
// @Tags 监控
// @Accept json
// @Produce json
// @Param request body model.AgentOfflinePrecheckDto true "离线部署预检查参数"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/offline-assets/precheck [post]
// @Security ApiKeyAuth
func (c *AgentController) PrecheckOfflineDeployment(ctx *gin.Context) {
	var dto model.AgentOfflinePrecheckDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	c.agentService.PrecheckOfflineDeployment(ctx, &dto)
}

// UninstallAgent 卸载指定主机的 agent，支持单个或多个。
// @Summary 卸载指定主机的 agent
// @Description 停止 agent 服务并删除相关文件；单个主机传 [hostId]，多个主机传 [hostId1,hostId2,hostId3]
// @Tags 监控
// @Accept json
// @Produce json
// @Param request body model.BatchDeployAgentDto true "卸载参数(只需 hostIds 字段)"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/uninstall [delete]
// @Security ApiKeyAuth
func (c *AgentController) UninstallAgent(ctx *gin.Context) {
	var dto model.BatchDeployAgentDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if len(dto.HostIDs) == 1 {
		c.agentService.UninstallAgent(ctx, dto.HostIDs[0])
	} else {
		c.agentService.BatchUninstallAgent(ctx, &dto)
	}
}

// RedeployAgent 重新部署指定主机的 agent，支持单个或多个。
// @Summary 重新部署指定主机的 agent
// @Description 先卸载旧 agent，再按当前配置和分组重新部署；单个主机传 [hostId]，多个主机传 [hostId1,hostId2,hostId3]
// @Tags 监控
// @Accept json
// @Produce json
// @Param request body model.RedeployAgentDto true "重部署参数(只需 hostIds 字段)"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/redeploy [post]
// @Security ApiKeyAuth
func (c *AgentController) RedeployAgent(ctx *gin.Context) {
	var dto model.RedeployAgentDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if len(dto.HostIDs) == 1 {
		c.agentService.RedeployAgent(ctx, dto.HostIDs[0])
	} else {
		c.agentService.BatchRedeployAgent(ctx, &dto)
	}
}

// GetAgentStatus 获取 agent 状态。
// @Summary 获取 agent 状态
// @Description 获取指定主机上 agent 的运行状态、版本信息等
// @Tags 监控
// @Accept json
// @Produce json
// @Param id path uint true "主机ID"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/status/{id} [get]
// @Security ApiKeyAuth
func (c *AgentController) GetAgentStatus(ctx *gin.Context) {
	hostIDStr := ctx.Param("id")
	hostID, err := strconv.ParseUint(hostIDStr, 10, 32)
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, "无效的主机ID")
		return
	}

	c.agentService.GetAgentStatus(ctx, uint(hostID))
}

// RestartAgent 重启 agent。
// @Summary 重启 agent
// @Description 重启指定主机上的 agent 服务
// @Tags 监控
// @Accept json
// @Produce json
// @Param id path uint true "主机ID"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/restart/{id} [post]
// @Security ApiKeyAuth
func (c *AgentController) RestartAgent(ctx *gin.Context) {
	hostIDStr := ctx.Param("id")
	hostID, err := strconv.ParseUint(hostIDStr, 10, 32)
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, "无效的主机ID")
		return
	}

	c.agentService.RestartAgent(ctx, uint(hostID))
}

// GetAgentList 获取 Agent 列表。
// @Summary 获取 Agent 列表
// @Description 获取所有 Agent 列表，支持主机名、IP 模糊查询，以及主机ID、状态和分页筛选
// @Tags 监控
// @Accept json
// @Produce json
// @Param hostId query uint false "主机ID"
// @Param hostName query string false "主机名称，支持模糊查询"
// @Param sshIp query string false "IP，支持模糊查询"
// @Param ip query string false "IP，支持模糊查询"
// @Param version query string false "Agent版本"
// @Param onlineStatus query int false "在线状态:1在线,2离线"
// @Param status query int false "状态"
// @Param page query int false "页码"
// @Param pageSize query int false "每页大小"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/list [get]
// @Security ApiKeyAuth
func (c *AgentController) GetAgentList(ctx *gin.Context) {
	c.agentService.GetAgentList(ctx, c.buildAgentListDto(ctx))
}

// GetAgentCandidateHosts 获取 Agent 可部署主机候选列表。
// @Summary 获取 Agent 可部署主机候选列表
// @Description 返回认证成功且为 Linux 的主机，并附带当前 Agent 部署状态
// @Tags 监控
// @Accept json
// @Produce json
// @Param keyword query string false "主机名称/IP关键字"
// @Param name query string false "兼容旧字段：主机名称关键字"
// @Param ip query string false "兼容旧字段：IP关键字"
// @Param groupId query uint false "分组ID"
// @Param deployableOnly query bool false "是否只返回可部署主机"
// @Param page query int false "页码"
// @Param pageSize query int false "每页大小"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/candidates [get]
// @Security ApiKeyAuth
func (c *AgentController) GetAgentCandidateHosts(ctx *gin.Context) {
	c.agentService.GetAgentCandidateHosts(ctx, c.buildAgentCandidateListDto(ctx))
}

// GetAgentCandidateGroups 获取 Agent 可部署主机分组树。
// @Summary 获取 Agent 可部署主机分组树
// @Description 返回存在可部署 Agent 主机的分组树，数量包含子分组
// @Tags 监控
// @Accept json
// @Produce json
// @Param keyword query string false "主机名称/IP关键字"
// @Param name query string false "兼容旧字段：主机名称关键字"
// @Param ip query string false "兼容旧字段：IP关键字"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/candidate-groups [get]
// @Security ApiKeyAuth
func (c *AgentController) GetAgentCandidateGroups(ctx *gin.Context) {
	c.agentService.GetAgentCandidateGroups(ctx, c.buildAgentCandidateGroupDto(ctx))
}

// GetAgentVersions 获取 Agent 版本列表。
// @Summary 获取 Agent 版本列表
// @Description 获取当前 Agent 版本列表及每个版本的 Agent 数量
// @Tags 监控
// @Accept json
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/versions [get]
// @Security ApiKeyAuth
func (c *AgentController) GetAgentVersions(ctx *gin.Context) {
	c.agentService.GetAgentVersions(ctx)
}

// QueryAgentList 查询 Agent 列表。
// @Summary 查询 Agent 列表
// @Description 根据主机名称和 IP 模糊查询 Agent 列表，支持主机ID、状态和分页筛选
// @Tags 监控
// @Accept json
// @Produce json
// @Param hostId query uint false "主机ID"
// @Param hostName query string false "主机名称，支持模糊查询"
// @Param sshIp query string false "IP，支持模糊查询"
// @Param ip query string false "IP，支持模糊查询"
// @Param version query string false "Agent版本"
// @Param onlineStatus query int false "在线状态:1在线,2离线"
// @Param status query int false "状态"
// @Param page query int false "页码"
// @Param pageSize query int false "每页大小"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/query [get]
// @Security ApiKeyAuth
func (c *AgentController) QueryAgentList(ctx *gin.Context) {
	c.agentService.GetAgentList(ctx, c.buildAgentListDto(ctx))
}

// UpdateHeartbeat 更新 Agent 心跳。
// @Summary 更新 Agent 心跳
// @Description Agent 主动上报心跳信息，通过 IP 自动识别主机
// @Tags 监控
// @Accept json
// @Produce json
// @Param heartbeat body model.AgentHeartbeatDto true "心跳数据"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/heartbeat [post]
func (c *AgentController) UpdateHeartbeat(ctx *gin.Context) {
	var heartbeat model.AgentHeartbeatDto
	if err := ctx.ShouldBindJSON(&heartbeat); err != nil {
		result.Failed(ctx, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	expectedToken := config.Config.Monitor.Agent.HeartbeatToken
	if expectedToken == "" {
		expectedToken = "agent-heartbeat-token-2024"
	}
	if heartbeat.Token != expectedToken {
		result.Failed(ctx, http.StatusUnauthorized, "Token 验证失败")
		return
	}

	if heartbeat.IP == "" {
		clientIP := ctx.ClientIP()
		if clientIP == "" {
			result.Failed(ctx, http.StatusBadRequest, "无法获取客户端 IP")
			return
		}
		heartbeat.IP = clientIP
	}

	c.agentService.UpdateHeartbeatByIP(ctx, &heartbeat)
}

// GetAgentStatistics 获取 Agent 统计信息。
// @Summary 获取 Agent 统计信息
// @Description 获取 Agent 的统计信息，包括各状态数量等
// @Tags 监控
// @Accept json
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/statistics [get]
// @Security ApiKeyAuth
func (c *AgentController) GetAgentStatistics(ctx *gin.Context) {
	c.agentService.GetAgentStatistics(ctx)
}

// DeleteAgent 删除 agent 数据。
// @Summary 删除 agent 数据
// @Description 删除指定的 agent 数据，用于服务器离线无法正常卸载的情况
// @Tags 监控
// @Accept json
// @Produce json
// @Param id path uint true "Agent ID"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/agent/delete/{id} [delete]
// @Security ApiKeyAuth
func (c *AgentController) DeleteAgent(ctx *gin.Context) {
	agentIDStr := ctx.Param("id")
	agentID, err := strconv.ParseUint(agentIDStr, 10, 32)
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, "无效的 Agent ID")
		return
	}

	c.agentService.DeleteAgentByID(ctx, uint(agentID))
}
