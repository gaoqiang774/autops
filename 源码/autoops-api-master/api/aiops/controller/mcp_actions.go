package controller

import (
	"net/http"
	"strings"

	"dodevops-api/api/aiops/service"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

type mcpLogCleanupConfirmRequest struct {
	ConfirmationID string `json:"confirmationId" binding:"required"`
}

// CancelMCPAction invalidates a signed action preview without executing it.
func CancelMCPAction(c *gin.Context) {
	var req mcpLogCleanupConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.ConfirmationID) == "" {
		result.Failed(c, http.StatusBadRequest, "confirmationId 不能为空")
		return
	}

	if err := service.CancelMCPAction(c, strings.TrimSpace(req.ConfirmationID)); err != nil {
		result.Failed(c, http.StatusForbidden, err.Error())
		return
	}
	result.Success(c, gin.H{
		"status":  "cancelled",
		"message": "操作已取消，确认凭证已失效",
	})
}

// ConfirmMCPLogCleanup confirms an MCP cleanup preview with the real JWT user.
// @Summary 确认 MCP 日志清空
// @Description 使用当前平台登录用户校验系统管理员身份，并执行预览中不可变的日志清空命令；保留日志文件
// @Tags AIOps MCP
// @Accept json
// @Produce json
// @Param data body mcpLogCleanupConfirmRequest true "日志清理确认"
// @Success 200 {object} result.Result
// @Router /api/v1/aiops/mcp/actions/cleanup-logs/confirm [post]
// @Security ApiKeyAuth
func ConfirmMCPLogCleanup(c *gin.Context) {
	var req mcpLogCleanupConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.ConfirmationID) == "" {
		result.Failed(c, http.StatusBadRequest, "confirmationId 不能为空")
		return
	}

	data, err := service.NewMCPHostOperationService().ConfirmLogCleanup(c, strings.TrimSpace(req.ConfirmationID))
	if err != nil {
		result.Failed(c, http.StatusForbidden, err.Error())
		return
	}
	result.Success(c, data)
}
