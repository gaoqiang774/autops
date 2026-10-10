package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	aiopsmodel "dodevops-api/api/aiops/model"
	aiopsservice "dodevops-api/api/aiops/service"
	alertmodel "dodevops-api/api/alertmanager/model"
	alertnotify "dodevops-api/api/alertmanager/notify"
	"dodevops-api/pkg/log"

	"github.com/gin-gonic/gin"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

const (
	feishuAIOpsMessageTTL   = time.Hour
	feishuAIOpsActionTTL    = 15 * time.Minute
	feishuAIOpsRequestLimit = 15 * time.Minute
	feishuAIOpsCardTextMax  = 12000
)

var feishuAIOpsGinEngine = &gin.Engine{}

type feishuAIOpsMessage struct {
	AppID           string
	ChatID          string
	ChatType        string
	ThreadID        string
	MessageID       string
	SenderOpenID    string
	SenderTenantKey string
	Question        string
}

type feishuAIOpsRolloutQuery struct {
	Action              string
	ClusterID           uint
	Namespace           string
	WorkloadType        string
	WorkloadName        string
	OperationGeneration int64
	TargetReplicas      int32
}

func (w *feishuWSWatcher) dispatchAIOpsMessage(channel alertmodel.AlertNotifyChannel, cfg alertnotify.FeishuCardAppConfig, event *larkim.P2MessageReceiveV1) {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		log.Log().Warnf("feishu aiops message ignored: event or message is nil")
		return
	}
	message := event.Event.Message
	chatID := strings.TrimSpace(pointerString(message.ChatId))
	chatType := strings.ToLower(strings.TrimSpace(pointerString(message.ChatType)))
	messageID := strings.TrimSpace(pointerString(message.MessageId))
	if !cfg.AllowsAIOpsChat(chatID) {
		log.Log().Infof("feishu aiops message ignored: channel=%d chatId=%s messageId=%s reason=aiops_disabled_or_empty_chat", channel.ID, chatID, messageID)
		return
	}
	if !supportsFeishuAIOpsChatType(chatType) {
		log.Log().Infof("feishu aiops message ignored: channel=%d chatId=%s messageId=%s chatType=%s reason=unsupported_chat_type", channel.ID, chatID, messageID, chatType)
		return
	}
	if event.Event.Sender == nil || !strings.EqualFold(pointerString(event.Event.Sender.SenderType), "user") {
		senderType := ""
		if event.Event.Sender != nil {
			senderType = strings.TrimSpace(pointerString(event.Event.Sender.SenderType))
		}
		log.Log().Infof("feishu aiops message ignored: channel=%d chatId=%s messageId=%s senderType=%s reason=non_user_sender", channel.ID, chatID, messageID, senderType)
		return
	}
	requireMention := requiresFeishuAIOpsMention(cfg, chatType)
	question, ok := parseFeishuAIOpsQuestion(message, requireMention)
	if !ok {
		log.Log().Infof("feishu aiops message ignored: channel=%d chatId=%s messageId=%s messageType=%s requireMention=%t mentionCount=%d reason=unsupported_or_empty_message", channel.ID, chatID, messageID, strings.TrimSpace(pointerString(message.MessageType)), requireMention, len(message.Mentions))
		return
	}
	eventID := ""
	appID := strings.TrimSpace(cfg.AppID)
	if event.EventV2Base != nil && event.EventV2Base.Header != nil {
		eventID = strings.TrimSpace(event.EventV2Base.Header.EventID)
		if headerAppID := strings.TrimSpace(event.EventV2Base.Header.AppID); headerAppID != "" {
			appID = headerAppID
		}
	}
	dedupID := messageID
	if dedupID == "" {
		dedupID = eventID
	}
	if dedupID == "" {
		log.Log().Infof("feishu aiops message ignored: channel=%d chatId=%s reason=missing_message_and_event_id", channel.ID, chatID)
		return
	}
	if !w.claimFeishuAIOpsDedup(w.processedMessages, appID+":"+dedupID, feishuAIOpsMessageTTL) {
		log.Log().Infof("feishu aiops message ignored: channel=%d chatId=%s messageId=%s reason=duplicate", channel.ID, chatID, messageID)
		return
	}

	openID := ""
	tenantKey := strings.TrimSpace(pointerString(event.Event.Sender.TenantKey))
	if event.Event.Sender.SenderId != nil {
		openID = strings.TrimSpace(pointerString(event.Event.Sender.SenderId.OpenId))
	}
	request := feishuAIOpsMessage{
		AppID: appID, ChatID: chatID, ChatType: chatType,
		ThreadID:  strings.TrimSpace(pointerString(message.ThreadId)),
		MessageID: messageID, SenderOpenID: openID,
		SenderTenantKey: tenantKey, Question: question,
	}
	log.Log().Infof("feishu aiops message accepted: channel=%d chatId=%s chatType=%s messageId=%s openId=%s", channel.ID, chatID, chatType, messageID, openID)
	go w.processFeishuAIOpsMessage(channel, cfg, request)
}

func supportsFeishuAIOpsChatType(chatType string) bool {
	switch strings.ToLower(strings.TrimSpace(chatType)) {
	case "p2p", "group", "topic_group":
		return true
	default:
		return false
	}
}

func requiresFeishuAIOpsMention(cfg alertnotify.FeishuCardAppConfig, chatType string) bool {
	return !strings.EqualFold(strings.TrimSpace(chatType), "p2p") && cfg.RequiresAIOpsMention()
}

func parseFeishuAIOpsQuestion(message *larkim.EventMessage, requireMention bool) (string, bool) {
	if message == nil || !strings.EqualFold(pointerString(message.MessageType), "text") {
		return "", false
	}
	if requireMention && len(message.Mentions) == 0 {
		return "", false
	}
	var content struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(pointerString(message.Content)), &content); err != nil {
		return "", false
	}
	question := content.Text
	for _, mention := range message.Mentions {
		if mention != nil {
			question = strings.ReplaceAll(question, pointerString(mention.Key), "")
		}
	}
	question = strings.TrimSpace(question)
	return question, question != ""
}

func (w *feishuWSWatcher) claimFeishuAIOpsDedup(items map[string]time.Time, key string, ttl time.Duration) bool {
	now := time.Now()
	w.mu.Lock()
	defer w.mu.Unlock()
	for itemKey, expiresAt := range items {
		if !expiresAt.After(now) {
			delete(items, itemKey)
		}
	}
	if expiresAt, exists := items[key]; exists && expiresAt.After(now) {
		return false
	}
	items[key] = now.Add(ttl)
	return true
}

func (w *feishuWSWatcher) processFeishuAIOpsMessage(channel alertmodel.AlertNotifyChannel, cfg alertnotify.FeishuCardAppConfig, message feishuAIOpsMessage) {
	ctx, cancel := context.WithTimeout(w.feishuAIOpsContext(), feishuAIOpsRequestLimit)
	defer cancel()
	processingCard := buildFeishuAIOpsCard("AIOps 正在分析", "blue", fmt.Sprintf("正在处理：%s", truncateFeishuAIOpsText(message.Question)))
	responseMessageID, sendErr := sendFeishuAIOpsCard(ctx, cfg, message.ChatID, processingCard, message.MessageID)
	if sendErr != nil {
		log.Log().Warnf("feishu aiops send processing card failed: channel=%d chatId=%s err=%v", channel.ID, message.ChatID, sendErr)
	}

	ginContext, err := newFeishuAIOpsGinContext(ctx)
	if err != nil {
		w.finishFeishuAIOpsMessage(ctx, cfg, message.ChatID, responseMessageID, buildFeishuAIOpsErrorCard(err))
		return
	}
	result, err := aiopsservice.NewAssistantChatService().Chat(ginContext, &aiopsmodel.AssistantChatRequest{
		SessionID: feishuAIOpsSessionID(message),
		Question:  message.Question,
		Mode:      "feishu",
		Context: map[string]interface{}{
			"channel": "feishu", "appId": message.AppID,
			"conversationId": message.ChatID, "chatId": message.ChatID,
			"chatType": message.ChatType, "threadId": message.ThreadID,
			"messageId": message.MessageID, "userId": message.SenderOpenID,
			"feishuOpenId": message.SenderOpenID, "tenantKey": message.SenderTenantKey,
		},
	})
	if err != nil {
		log.Log().Warnf("feishu aiops chat failed: channel=%d chatId=%s openId=%s err=%v", channel.ID, message.ChatID, message.SenderOpenID, err)
		w.finishFeishuAIOpsMessage(ctx, cfg, message.ChatID, responseMessageID, buildFeishuAIOpsErrorCard(err))
		return
	}
	w.finishFeishuAIOpsMessage(ctx, cfg, message.ChatID, responseMessageID, buildFeishuAIOpsResultCard(channel.ID, message, result))
	log.Log().Infof("feishu aiops chat completed: channel=%d chatId=%s openId=%s runId=%d intent=%s", channel.ID, message.ChatID, message.SenderOpenID, result.RunID, result.Intent)
	if responseMessageID != "" && feishuAIOpsBool(feishuAIOpsMap(result.PendingAction["polling"]), "shouldPoll") {
		w.monitorFeishuAIOpsOperation(ctx, cfg, responseMessageID, result.PendingAction, message.SenderOpenID)
	}
}

func (w *feishuWSWatcher) finishFeishuAIOpsMessage(ctx context.Context, cfg alertnotify.FeishuCardAppConfig, chatID, messageID string, card map[string]interface{}) {
	if messageID != "" {
		if err := patchFeishuAIOpsCard(ctx, cfg, messageID, card); err == nil {
			return
		} else {
			log.Log().Warnf("feishu aiops patch card failed: messageId=%s err=%v", messageID, err)
		}
	}
	if _, err := sendFeishuAIOpsCard(ctx, cfg, chatID, card, ""); err != nil {
		log.Log().Warnf("feishu aiops send result card failed: chatId=%s err=%v", chatID, err)
	}
}

func (w *feishuWSWatcher) feishuAIOpsContext() context.Context {
	if w.ctx != nil {
		return w.ctx
	}
	return context.Background()
}

func newFeishuAIOpsGinContext(ctx context.Context) (*gin.Context, error) {
	request := httptest.NewRequest(http.MethodPost, "/internal/feishu/aiops", nil).WithContext(ctx)
	ginContext := gin.CreateTestContextOnly(httptest.NewRecorder(), feishuAIOpsGinEngine)
	ginContext.Request = request
	if err := aiopsservice.InjectMCPServiceAccount(ginContext); err != nil {
		return nil, err
	}
	return ginContext, nil
}

func feishuAIOpsSessionID(message feishuAIOpsMessage) string {
	conversationID := message.ChatID
	if message.ThreadID != "" {
		conversationID += ":" + message.ThreadID
	}
	return strings.Join([]string{"feishu", message.AppID, message.ChatType, conversationID}, ":")
}

func sendFeishuAIOpsCard(ctx context.Context, cfg alertnotify.FeishuCardAppConfig, chatID string, card map[string]interface{}, uuid string) (string, error) {
	content, err := json.Marshal(card)
	if err != nil {
		return "", err
	}
	bodyBuilder := larkim.NewCreateMessageReqBodyBuilder().ReceiveId(chatID).MsgType("interactive").Content(string(content))
	if strings.TrimSpace(uuid) != "" {
		bodyBuilder.Uuid(uuid)
	}
	req := larkim.NewCreateMessageReqBuilder().ReceiveIdType("chat_id").Body(bodyBuilder.Build()).Build()
	resp, err := lark.NewClient(cfg.AppID, cfg.AppSecret).Im.Message.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if resp == nil || !resp.Success() {
		if resp == nil {
			return "", errors.New("feishu create message returned empty response")
		}
		return "", fmt.Errorf("feishu create message failed: code=%d message=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil {
		return "", errors.New("feishu create message returned no message data")
	}
	return strings.TrimSpace(pointerString(resp.Data.MessageId)), nil
}

func patchFeishuAIOpsCard(ctx context.Context, cfg alertnotify.FeishuCardAppConfig, messageID string, card map[string]interface{}) error {
	content, err := json.Marshal(card)
	if err != nil {
		return err
	}
	req := larkim.NewPatchMessageReqBuilder().MessageId(messageID).
		Body(larkim.NewPatchMessageReqBodyBuilder().Content(string(content)).Build()).Build()
	resp, err := lark.NewClient(cfg.AppID, cfg.AppSecret).Im.Message.Patch(ctx, req)
	if err != nil {
		return err
	}
	if resp == nil || !resp.Success() {
		if resp == nil {
			return errors.New("feishu patch message returned empty response")
		}
		return fmt.Errorf("feishu patch message failed: code=%d message=%s", resp.Code, resp.Msg)
	}
	return nil
}

func buildFeishuAIOpsResultCard(channelID uint, message feishuAIOpsMessage, result *aiopsmodel.AssistantChatResult) map[string]interface{} {
	if result == nil {
		return buildFeishuAIOpsErrorCard(errors.New("助手未返回结果"))
	}
	title := "AIOps 分析完成"
	template := "green"
	elements := []interface{}{feishuAIOpsMarkdownElement(truncateFeishuAIOpsText(result.Answer))}
	confirmationID := firstNonEmptyMapString(result.PendingAction, "confirmationId", "confirmation_id")
	if confirmationID != "" {
		title = "AIOps 等待确认"
		template = "orange"
		elements = append(elements, map[string]interface{}{
			"tag": "action",
			"actions": []interface{}{
				map[string]interface{}{
					"tag": "button", "type": "primary",
					"text": map[string]interface{}{"tag": "plain_text", "content": "确认执行"},
					"value": map[string]interface{}{
						"action": "aiops_confirm", "confirmationId": confirmationID,
						"channelId": channelID, "chatId": message.ChatID,
						"sessionId": result.SessionID, "runId": result.RunID,
					},
				},
				map[string]interface{}{
					"tag": "button", "type": "default",
					"text": map[string]interface{}{"tag": "plain_text", "content": "取消"},
					"value": map[string]interface{}{
						"action": "aiops_cancel", "confirmationId": confirmationID,
						"channelId": channelID, "chatId": message.ChatID,
						"sessionId": result.SessionID, "runId": result.RunID,
					},
				},
			},
		})
	}
	elements = append(elements, feishuAIOpsNoteElement(fmt.Sprintf("Run #%d · 操作人 %s", result.RunID, displayFeishuOpenID(message.SenderOpenID))))
	return buildFeishuAIOpsCardWithElements(title, template, elements)
}

func buildFeishuAIOpsCard(title, template, content string) map[string]interface{} {
	return buildFeishuAIOpsCardWithElements(title, template, []interface{}{feishuAIOpsMarkdownElement(content)})
}

func buildFeishuAIOpsCardWithElements(title, template string, elements []interface{}) map[string]interface{} {
	return map[string]interface{}{
		"config": map[string]interface{}{"wide_screen_mode": true, "enable_forward": false},
		"header": map[string]interface{}{
			"template": template,
			"title":    map[string]interface{}{"tag": "plain_text", "content": title},
		},
		"elements": elements,
	}
}

func buildFeishuAIOpsErrorCard(err error) map[string]interface{} {
	message := "执行失败，请稍后重试。"
	if err != nil {
		message = "执行失败：" + truncateFeishuAIOpsText(err.Error())
	}
	return buildFeishuAIOpsCard("AIOps 执行失败", "red", message)
}

func feishuAIOpsMarkdownElement(content string) map[string]interface{} {
	return map[string]interface{}{"tag": "div", "text": map[string]interface{}{"tag": "lark_md", "content": content}}
}

func feishuAIOpsNoteElement(content string) map[string]interface{} {
	return map[string]interface{}{"tag": "note", "elements": []interface{}{map[string]interface{}{"tag": "plain_text", "content": content}}}
}

func truncateFeishuAIOpsText(value string) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= feishuAIOpsCardTextMax {
		return value
	}
	runes := []rune(value)
	return string(runes[:feishuAIOpsCardTextMax]) + "\n\n内容过长，已截断。"
}

func displayFeishuOpenID(openID string) string {
	openID = strings.TrimSpace(openID)
	if len(openID) <= 12 {
		return openID
	}
	return openID[:8] + "..." + openID[len(openID)-4:]
}

func (w *feishuWSWatcher) handleAIOpsConfirmAction(event *callback.CardActionTriggerEvent, actionData map[string]interface{}, openID string) (*callback.CardActionTriggerResponse, error) {
	channel, cfg, chatID, err := w.resolveFeishuAIOpsAction(event, actionData)
	if err != nil {
		return w.buildCardActionResponse(err.Error(), false), nil
	}
	confirmationID := firstNonEmptyMapString(actionData, "confirmationId", "confirmation_id")
	if confirmationID == "" {
		return w.buildCardActionResponse("确认凭证为空，请重新发起操作", false), nil
	}
	if !w.claimFeishuAIOpsDedup(w.processedActions, "confirm:"+confirmationID, feishuAIOpsActionTTL) {
		return w.buildCardActionResponse("该操作正在执行或已经处理", false), nil
	}
	messageID := ""
	if event != nil && event.Event != nil && event.Event.Context != nil {
		messageID = strings.TrimSpace(event.Event.Context.OpenMessageID)
	}
	go w.executeFeishuAIOpsConfirmation(*channel, cfg, chatID, messageID, confirmationID, openID)
	card := buildFeishuAIOpsCardWithElements("AIOps 操作执行中", "blue", []interface{}{
		feishuAIOpsMarkdownElement("确认已接收，正在提交运维操作。"),
		feishuAIOpsNoteElement("操作人 " + displayFeishuOpenID(openID)),
	})
	return w.buildCardActionResponseWithCard("确认成功，正在执行", true, card), nil
}

func (w *feishuWSWatcher) handleAIOpsCancelAction(event *callback.CardActionTriggerEvent, actionData map[string]interface{}, openID string) (*callback.CardActionTriggerResponse, error) {
	_, _, _, err := w.resolveFeishuAIOpsAction(event, actionData)
	if err != nil {
		return w.buildCardActionResponse(err.Error(), false), nil
	}
	confirmationID := firstNonEmptyMapString(actionData, "confirmationId", "confirmation_id")
	ginContext, contextErr := newFeishuAIOpsGinContext(w.feishuAIOpsContext())
	if contextErr != nil {
		return w.buildCardActionResponse("取消失败："+contextErr.Error(), false), nil
	}
	if err := aiopsservice.CancelMCPAction(ginContext, confirmationID); err != nil {
		return w.buildCardActionResponse("取消失败："+err.Error(), false), nil
	}
	card := buildFeishuAIOpsCardWithElements("AIOps 操作已取消", "grey", []interface{}{
		feishuAIOpsMarkdownElement("待执行操作已取消，确认凭证已经作废。"),
		feishuAIOpsNoteElement("操作人 " + displayFeishuOpenID(openID)),
	})
	return w.buildCardActionResponseWithCard("操作已取消", true, card), nil
}

func (w *feishuWSWatcher) resolveFeishuAIOpsAction(event *callback.CardActionTriggerEvent, actionData map[string]interface{}) (*alertmodel.AlertNotifyChannel, alertnotify.FeishuCardAppConfig, string, error) {
	channelID, _ := strconv.ParseUint(firstNonEmptyMapString(actionData, "channelId", "channel_id"), 10, 64)
	if channelID == 0 {
		return nil, alertnotify.FeishuCardAppConfig{}, "", errors.New("卡片缺少飞书通道信息")
	}
	channel, err := w.dao.GetNotifyChannel(uint(channelID))
	if err != nil || channel == nil || channel.Enabled != alertmodel.RuleStatusEnabled || channel.Type != alertmodel.NotifyChannelFeishuCard {
		return nil, alertnotify.FeishuCardAppConfig{}, "", errors.New("飞书 AIOps 通道不存在或已停用")
	}
	cfg, err := alertnotify.ParseFeishuCardAppConfig(channel.Config)
	if err != nil {
		return nil, alertnotify.FeishuCardAppConfig{}, "", errors.New("飞书 AIOps 通道配置无效")
	}
	chatID := firstNonEmptyMapString(actionData, "chatId", "chat_id")
	callbackChatID := ""
	if event != nil && event.Event != nil && event.Event.Context != nil {
		callbackChatID = strings.TrimSpace(event.Event.Context.OpenChatID)
	}
	if callbackChatID != "" && chatID != callbackChatID {
		return nil, alertnotify.FeishuCardAppConfig{}, "", errors.New("卡片所属群与操作目标不一致")
	}
	if event != nil && event.EventV2Base != nil && event.EventV2Base.Header != nil {
		if appID := strings.TrimSpace(event.EventV2Base.Header.AppID); appID != "" && appID != strings.TrimSpace(cfg.AppID) {
			return nil, alertnotify.FeishuCardAppConfig{}, "", errors.New("卡片所属应用与通道配置不一致")
		}
	}
	if !cfg.AllowsAIOpsChat(chatID) {
		return nil, alertnotify.FeishuCardAppConfig{}, "", errors.New("飞书 AIOps 对话未启用或 chat_id 为空")
	}
	return channel, cfg, chatID, nil
}

func (w *feishuWSWatcher) executeFeishuAIOpsConfirmation(channel alertmodel.AlertNotifyChannel, cfg alertnotify.FeishuCardAppConfig, chatID, messageID, confirmationID, openID string) {
	ctx, cancel := context.WithTimeout(w.feishuAIOpsContext(), feishuAIOpsRequestLimit)
	defer cancel()
	ginContext, err := newFeishuAIOpsGinContext(ctx)
	if err != nil {
		w.finishFeishuAIOpsMessage(ctx, cfg, chatID, messageID, buildFeishuAIOpsErrorCard(err))
		return
	}
	data, err := aiopsservice.ConfirmMCPAction(ginContext, confirmationID)
	if err != nil {
		log.Log().Warnf("feishu aiops confirm failed: channel=%d chatId=%s openId=%s err=%v", channel.ID, chatID, openID, err)
		w.finishFeishuAIOpsMessage(ctx, cfg, chatID, messageID, buildFeishuAIOpsErrorCard(err))
		return
	}
	card := buildFeishuAIOpsOperationCard(data, openID)
	w.finishFeishuAIOpsMessage(ctx, cfg, chatID, messageID, card)
	log.Log().Infof("feishu aiops confirm triggered: channel=%d chatId=%s openId=%s", channel.ID, chatID, openID)
	if messageID != "" {
		w.monitorFeishuAIOpsOperation(ctx, cfg, messageID, data, openID)
	}
}

func (w *feishuWSWatcher) monitorFeishuAIOpsOperation(ctx context.Context, cfg alertnotify.FeishuCardAppConfig, messageID string, initial interface{}, openID string) {
	values := feishuAIOpsMap(initial)
	if !feishuAIOpsBool(feishuAIOpsMap(values["polling"]), "shouldPoll") {
		return
	}
	query, err := parseFeishuAIOpsRolloutQuery(feishuAIOpsString(values, "statusEndpoint"))
	if err != nil {
		log.Log().Warnf("feishu aiops parse rollout status endpoint failed: %v", err)
		return
	}
	polling := feishuAIOpsMap(values["polling"])
	observeLimit := time.Duration(feishuAIOpsInt64(polling, "autoObserveLimitSeconds")) * time.Second
	if observeLimit <= 0 || observeLimit > 15*time.Minute {
		observeLimit = 15 * time.Minute
	}
	started := time.Now()
	lastToken := feishuAIOpsString(polling, "progressToken")
	for {
		elapsed := time.Since(started)
		if elapsed >= observeLimit {
			paused := buildFeishuAIOpsCardWithElements("AIOps 自动监控已暂停", "orange", []interface{}{
				feishuAIOpsMarkdownElement("工作负载可能仍在更新，请在群内重新查询工作负载状态。"),
				feishuAIOpsNoteElement("操作人 " + displayFeishuOpenID(openID)),
			})
			_ = patchFeishuAIOpsCard(context.Background(), cfg, messageID, paused)
			return
		}
		interval := 10 * time.Second
		if elapsed < 30*time.Second {
			interval = 2 * time.Second
		} else if elapsed < 2*time.Minute {
			interval = 5 * time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
		ginContext, contextErr := newFeishuAIOpsGinContext(ctx)
		if contextErr != nil {
			return
		}
		status, statusErr := aiopsservice.NewMCPK8sActionService().RolloutStatus(
			ginContext, query.Action, query.ClusterID, query.Namespace,
			query.WorkloadType, query.WorkloadName, query.OperationGeneration, query.TargetReplicas,
		)
		if statusErr != nil {
			log.Log().Warnf("feishu aiops rollout polling failed: messageId=%s err=%v", messageID, statusErr)
			continue
		}
		statusValues := feishuAIOpsMap(status)
		statusPolling := feishuAIOpsMap(statusValues["polling"])
		progressToken := feishuAIOpsString(statusPolling, "progressToken")
		shouldPoll := feishuAIOpsBool(statusPolling, "shouldPoll")
		if progressToken != lastToken || !shouldPoll {
			if err := patchFeishuAIOpsCard(ctx, cfg, messageID, buildFeishuAIOpsOperationCard(status, openID)); err != nil {
				log.Log().Warnf("feishu aiops patch rollout progress failed: messageId=%s err=%v", messageID, err)
			}
			lastToken = progressToken
		}
		if !shouldPoll {
			return
		}
	}
}

func parseFeishuAIOpsRolloutQuery(endpoint string) (feishuAIOpsRolloutQuery, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed == nil {
		return feishuAIOpsRolloutQuery{}, errors.New("invalid rollout status endpoint")
	}
	queryValues := parsed.Query()
	clusterID, _ := strconv.ParseUint(queryValues.Get("clusterId"), 10, 64)
	generation, _ := strconv.ParseInt(queryValues.Get("generation"), 10, 64)
	targetReplicas, _ := strconv.ParseInt(queryValues.Get("targetReplicas"), 10, 32)
	result := feishuAIOpsRolloutQuery{
		Action: queryValues.Get("action"), ClusterID: uint(clusterID),
		Namespace: queryValues.Get("namespace"), WorkloadType: queryValues.Get("workloadType"),
		WorkloadName: queryValues.Get("workloadName"), OperationGeneration: generation,
		TargetReplicas: int32(targetReplicas),
	}
	if result.Action == "" {
		result.Action = "rollout_restart"
	}
	if result.ClusterID == 0 || result.Namespace == "" || result.WorkloadType == "" || result.WorkloadName == "" {
		return feishuAIOpsRolloutQuery{}, errors.New("rollout status endpoint is incomplete")
	}
	return result, nil
}

func buildFeishuAIOpsOperationCard(data interface{}, openID string) map[string]interface{} {
	values := feishuAIOpsMap(data)
	rollout := feishuAIOpsMap(values["rollout"])
	phase := firstNonEmptyMapString(values, "phase", "status")
	if phase == "" {
		phase = firstNonEmptyMapString(rollout, "phase", "status")
	}
	title := "AIOps 操作已触发"
	template := "blue"
	switch strings.ToLower(phase) {
	case "succeeded", "success", "completed":
		title, template = "AIOps 操作已完成", "green"
	case "failed", "error":
		title, template = "AIOps 操作失败", "red"
	case "progressing", "running":
		title, template = "AIOps 操作进行中", "blue"
	}
	message := firstNonEmptyMapString(values, "message")
	if message == "" {
		message = "运维操作已经提交。"
	}
	desired := feishuAIOpsInt64(rollout, "desiredReplicas")
	ready := feishuAIOpsInt64(rollout, "readyReplicas")
	updated := feishuAIOpsInt64(rollout, "updatedReplicas")
	available := feishuAIOpsInt64(rollout, "availableReplicas")
	unavailable := feishuAIOpsInt64(rollout, "unavailableReplicas")
	content := message
	if desired > 0 || ready > 0 || updated > 0 || available > 0 || unavailable > 0 {
		content += fmt.Sprintf("\n\n**就绪副本**：%d / %d\n**新版本副本**：%d / %d\n**可用副本**：%d\n**不可用副本**：%d", ready, desired, updated, desired, available, unavailable)
	}
	return buildFeishuAIOpsCardWithElements(title, template, []interface{}{
		feishuAIOpsMarkdownElement(truncateFeishuAIOpsText(content)),
		feishuAIOpsNoteElement("操作人 " + displayFeishuOpenID(openID)),
	})
}

func firstNonEmptyMapString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fmt.Sprint(values[key])); value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func feishuAIOpsMap(value interface{}) map[string]interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return typed
	case gin.H:
		return map[string]interface{}(typed)
	default:
		return map[string]interface{}{}
	}
}

func feishuAIOpsString(values map[string]interface{}, key string) string {
	return firstNonEmptyMapString(values, key)
}

func feishuAIOpsBool(values map[string]interface{}, key string) bool {
	switch value := values[key].(type) {
	case bool:
		return value
	case string:
		parsed, _ := strconv.ParseBool(value)
		return parsed
	default:
		return false
	}
}

func feishuAIOpsInt64(values map[string]interface{}, key string) int64 {
	switch value := values[key].(type) {
	case int:
		return int64(value)
	case int32:
		return int64(value)
	case int64:
		return value
	case uint:
		return int64(value)
	case uint64:
		return int64(value)
	case float64:
		return int64(value)
	case json.Number:
		parsed, _ := value.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(value, 10, 64)
		return parsed
	default:
		return 0
	}
}
