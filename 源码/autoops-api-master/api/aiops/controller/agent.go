package controller

import (
	"net/http"
	"strconv"
	"time"

	"dodevops-api/api/aiops/model"
	"dodevops-api/api/aiops/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

// @Summary 告警驱动只读 SRE 诊断
// @Description 根据 Prometheus 告警补齐 CMDB、指标、日志证据并生成诊断报告
// @Produce json
// @Tags AIOps Agent
// @Param data body model.AlertAnalyzeRequest true "告警诊断请求"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/agent/alerts/analyze [post]
// @Security ApiKeyAuth
func AnalyzeAlert(c *gin.Context) {
	var req model.AlertAnalyzeRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	data, err := service.NewAlertDiagnosisService().AnalyzeAlert(c, &req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "告警诊断失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 提交 Agent 回答反馈
// @Description 记录有用、无用、结论错误、证据不足或工具错误反馈，用于 Bad Case 沉淀
// @Produce json
// @Tags AIOps Agent
// @Param data body model.AgentFeedbackRequest true "反馈请求"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/agent/feedback [post]
// @Security ApiKeyAuth
func CreateAgentFeedback(c *gin.Context) {
	var req model.AgentFeedbackRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	data, err := service.NewAgentFeedbackService().Create(c, &req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "提交反馈失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 获取 Agent 执行详情
// @Description 获取告警诊断 run、step、evidence 和结果
// @Produce json
// @Tags AIOps Agent
// @Param id path int true "Run ID"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/agent/runs/:id [get]
// @Security ApiKeyAuth
func GetAgentRun(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	data, err := service.NewAlertDiagnosisService().GetRunDetail(id)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询 Agent 执行详情失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary Stream Agent 执行轨迹
// @Description 通过 SSE 返回 Agent run、step 和 result 事件
// @Produce text/event-stream
// @Tags AIOps Agent
// @Param id path int true "Run ID"
// @router /api/v1/aiops/agent/runs/:id/stream [get]
// @Security ApiKeyAuth
func StreamAgentRun(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	events, err := service.NewAlertDiagnosisService().BuildRunStreamEvents(id)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询 Agent 执行流失败: "+err.Error())
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	flusher, _ := c.Writer.(http.Flusher)
	for _, event := range events {
		c.SSEvent(event.Type, event)
		if flusher != nil {
			flusher.Flush()
		}
		time.Sleep(80 * time.Millisecond)
	}
}
