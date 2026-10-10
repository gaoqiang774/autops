package controller

import (
	"dodevops-api/api/aiops/model"
	"dodevops-api/api/aiops/service"
	systemmodel "dodevops-api/api/system/model"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

// ListMCPServers returns the seven code-owned MCP domains and effective tool policies.
func ListMCPServers(c *gin.Context) {
	servers, err := service.NewMCPCatalogService().ListServers(c.Query("domain"))
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询 MCP 配置失败: "+err.Error())
		return
	}
	result.Success(c, gin.H{"servers": servers})
}

func UpdateMCPServerStatus(c *gin.Context) {
	var req model.MCPServerStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	server, err := service.NewMCPCatalogService().UpdateServer(c.Param("code"), req.Enabled, currentMCPUserID(c))
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "更新 MCP 状态失败: "+err.Error())
		return
	}
	result.Success(c, server)
}

func UpdateMCPToolConfig(c *gin.Context) {
	var req model.MCPToolConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	tool, err := service.NewMCPCatalogService().UpdateTool(c.Param("code"), c.Param("toolName"), req, currentMCPUserID(c))
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "更新 Tool 配置失败: "+err.Error())
		return
	}
	result.Success(c, tool)
}

func currentMCPUserID(c *gin.Context) uint {
	value, ok := c.Get(constant.ContextKeyUserObj)
	if !ok {
		return 0
	}
	admin, ok := value.(*systemmodel.JwtAdmin)
	if !ok || admin == nil {
		return 0
	}
	return admin.ID
}
