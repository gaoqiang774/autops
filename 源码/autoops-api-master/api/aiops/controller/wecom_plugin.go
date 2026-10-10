package controller

import (
	"strings"

	"dodevops-api/api/aiops/model"
	"dodevops-api/api/aiops/service"
	"dodevops-api/common/config"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

const weComPluginTokenHeader = "X-AIOPS-BOT-TOKEN"

// @Summary 企业微信智能机器人 API 插件入口
// @Description 企业微信智能机器人 API 模式调用该接口，后端校验插件 token 后复用 AIOps 通用助手。
// @Produce json
// @Tags AIOps智能助手
// @Param X-AIOPS-BOT-TOKEN header string true "企业微信插件 API Key"
// @Param data body model.WeComPluginChatRequest true "企业微信插件请求"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/wecom/plugin/chat [post]
func WeComPluginChat(c *gin.Context) {
	if !validateWeComPluginToken(c) {
		result.Failed(c, int(result.ApiCode.NOAUTH), "无效的企业微信插件认证 Token")
		return
	}

	var req model.WeComPluginChatRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	question := weComPluginQuestion(&req)
	if question == "" {
		result.Failed(c, constant.INVALID_PARAMS, "question/query/content/text 不能为空")
		return
	}

	sessionID := weComPluginSessionID(&req)
	assistantReq := &model.AssistantChatRequest{
		SessionID:  sessionID,
		Question:   question,
		ProviderID: req.ProviderID,
		Mode:       "wecom",
		Context:    weComPluginContext(&req),
	}

	data, err := service.NewAssistantChatService().Chat(c, assistantReq)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "企业微信智能体执行失败: "+err.Error())
		return
	}

	result.Success(c, &model.WeComPluginChatResponse{
		Answer:           data.Answer,
		SessionID:        data.SessionID,
		RunID:            data.RunID,
		Intent:           data.Intent,
		SuggestedActions: data.SuggestedActions,
		MissingInfo:      data.MissingInfo,
		Evidence:         data.Evidence,
		AssistantResult:  data,
	})
}

func validateWeComPluginToken(c *gin.Context) bool {
	if config.Config == nil {
		return false
	}
	expected := strings.TrimSpace(config.Config.WeCom.PluginToken)
	if expected == "" {
		return false
	}
	token := strings.TrimSpace(c.GetHeader(weComPluginTokenHeader))
	if token == "" {
		token = strings.TrimSpace(c.Query("token"))
	}
	return token == expected
}

func weComPluginQuestion(req *model.WeComPluginChatRequest) string {
	if req == nil {
		return ""
	}
	for _, value := range []string{req.Question, req.Query, req.Content, req.Text} {
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	return ""
}

func weComPluginSessionID(req *model.WeComPluginChatRequest) string {
	if req == nil {
		return "wecom:unknown"
	}
	if sessionID := strings.TrimSpace(req.SessionID); sessionID != "" {
		return sessionID
	}

	corpID := firstNonEmptyWeComPluginString(req.CorpID, "default")
	botID := firstNonEmptyWeComPluginString(req.BotID, "aiops")
	chatType := firstNonEmptyWeComPluginString(req.ChatType, "private")
	conversationID := firstNonEmptyWeComPluginString(req.ThreadID, req.ConversationID, req.ChatID, req.UserID, "default")

	parts := []string{"wecom", corpID, botID, chatType, conversationID}
	if chatType == "group" && strings.TrimSpace(req.ThreadID) == "" && strings.TrimSpace(req.UserID) != "" {
		parts = append(parts, strings.TrimSpace(req.UserID))
	}
	return strings.Join(parts, ":")
}

func weComPluginContext(req *model.WeComPluginChatRequest) map[string]interface{} {
	ctx := map[string]interface{}{
		"channel": "wecom",
	}
	if req == nil {
		return ctx
	}
	for k, v := range req.Context {
		ctx[k] = v
	}
	putIfNotEmpty(ctx, "corpId", req.CorpID)
	putIfNotEmpty(ctx, "botId", req.BotID)
	putIfNotEmpty(ctx, "userId", req.UserID)
	putIfNotEmpty(ctx, "userName", req.UserName)
	putIfNotEmpty(ctx, "conversationId", req.ConversationID)
	putIfNotEmpty(ctx, "chatId", req.ChatID)
	putIfNotEmpty(ctx, "chatType", req.ChatType)
	putIfNotEmpty(ctx, "threadId", req.ThreadID)
	putIfNotEmpty(ctx, "messageId", req.MessageID)
	return ctx
}

func putIfNotEmpty(m map[string]interface{}, key, value string) {
	if strings.TrimSpace(value) != "" {
		m[key] = strings.TrimSpace(value)
	}
}

func firstNonEmptyWeComPluginString(values ...string) string {
	for _, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	return ""
}
