package controller

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"dodevops-api/api/aiops/model"
	"dodevops-api/api/aiops/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

// @Summary 平台内部查询助手
// @Description 自然语言查询资源选址、资源回收、容量巡检、资产信息
// @Produce json
// @Tags AIOps智能助手
// @Param data body model.AssistantQueryRequest true "查询条件"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/assistant/query [post]
// @Security ApiKeyAuth
func QueryAssistant(c *gin.Context) {
	var req model.AssistantQueryRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	data, err := service.NewAssistantQueryService().Query(c, &req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询助手执行失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 平台内部通用 AI 助手
// @Description 面向统一入口的对话式助手，自动编排只读工具并生成可审计 run
// @Produce json
// @Produce text/event-stream
// @Produce application/x-ndjson
// @Tags AIOps智能助手
// @Param data body model.AssistantChatRequest true "对话请求"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/assistant/chat [post]
// @Security ApiKeyAuth
func ChatAssistant(c *gin.Context) {
	var req model.AssistantChatRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	if shouldStreamAssistantChat(c, &req) {
		streamAssistantChat(c, &req)
		return
	}

	data, err := service.NewAssistantChatService().Chat(c, &req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "通用助手执行失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 清理通用 AI 助手历史
// @Description 清理通用助手会话、消息、assistant_chat run 和 evidence。传 sessionId 时只清理该会话，不传则清理全部助手聊天历史。
// @Produce json
// @Tags AIOps智能助手
// @Param data body model.AssistantChatHistoryClearRequest false "清理条件"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/assistant/chat/history [delete]
// @Security ApiKeyAuth
func ClearAssistantChatHistory(c *gin.Context) {
	var req model.AssistantChatHistoryClearRequest
	if c.Request != nil && c.Request.Body != nil {
		_ = c.ShouldBindJSON(&req)
	}
	if req.SessionID == "" {
		req.SessionID = strings.TrimSpace(c.Query("sessionId"))
	}
	data, err := service.NewAssistantChatService().ClearHistory(c, &req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "清理助手历史失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

func shouldStreamAssistantChat(c *gin.Context, req *model.AssistantChatRequest) bool {
	if c == nil {
		return false
	}
	stream := strings.ToLower(strings.TrimSpace(c.Query("stream")))
	if stream == "true" || stream == "1" || stream == "sse" || stream == "ndjson" {
		return true
	}
	if req != nil && strings.EqualFold(strings.TrimSpace(req.Mode), "stream") {
		return true
	}
	accept := strings.ToLower(c.GetHeader("Accept"))
	return strings.Contains(accept, "text/event-stream") || strings.Contains(accept, "application/x-ndjson")
}

func streamAssistantChat(c *gin.Context, req *model.AssistantChatRequest) {
	ndjson := wantsAssistantChatNDJSON(c)
	if ndjson {
		c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	} else {
		c.Header("Content-Type", "text/event-stream; charset=utf-8")
	}
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	flusher, _ := c.Writer.(http.Flusher)
	emit := func(event model.AgentRunStreamEvent) {
		if ndjson {
			if data, err := json.Marshal(event); err == nil {
				_, _ = c.Writer.Write(append(data, '\n'))
			}
		} else {
			c.SSEvent(event.Type, event)
		}
		if flusher != nil {
			flusher.Flush()
		}
	}

	if _, err := service.NewAssistantChatStreamService(emit).Chat(c, req); err != nil {
		emit(model.AgentRunStreamEvent{
			Type:    "error",
			Status:  model.AgentRunStatusFailed,
			Message: "通用助手执行失败",
			Error:   err.Error(),
			Time:    time.Now().Format("2006-01-02 15:04:05"),
		})
	}
}

func wantsAssistantChatNDJSON(c *gin.Context) bool {
	if c == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(c.Query("stream")), "ndjson") {
		return true
	}
	accept := strings.ToLower(c.GetHeader("Accept"))
	return strings.Contains(accept, "application/x-ndjson")
}
