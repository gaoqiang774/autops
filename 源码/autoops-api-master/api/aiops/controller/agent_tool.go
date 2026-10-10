package controller

import (
	"dodevops-api/api/aiops/model"
	"dodevops-api/api/aiops/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

// @Summary 获取 Agent 工具列表
// @Description 获取第一阶段开放给 Agent Runtime 的只读工具白名单
// @Produce json
// @Tags AIOps Agent Tool Gateway
// @Success 200 {object} result.Result
// @router /api/v1/aiops/agent/tools [get]
// @Security ApiKeyAuth
func ListAgentTools(c *gin.Context) {
	tools, err := service.NewAgentToolGatewayService().ListEnabledTools()
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "获取工具列表失败: "+err.Error())
		return
	}
	result.Success(c, tools)
}

// @Summary 调用 Agent 工具
// @Description 通过 AutoOps Tool Gateway 调用受控只读工具，并写入 run、step、evidence 审计
// @Produce json
// @Tags AIOps Agent Tool Gateway
// @Param data body model.AgentToolInvokeRequest true "工具调用请求"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/agent/tools/invoke [post]
// @Security ApiKeyAuth
func InvokeAgentTool(c *gin.Context) {
	var req model.AgentToolInvokeRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	data, err := service.NewAgentToolGatewayService().Invoke(c, &req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "工具调用失败: "+err.Error())
		return
	}
	result.Success(c, data)
}
