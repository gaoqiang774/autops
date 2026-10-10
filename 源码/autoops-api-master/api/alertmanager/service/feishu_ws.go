package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"dodevops-api/api/alertmanager/dao"
	"dodevops-api/api/alertmanager/model"
	alertnotify "dodevops-api/api/alertmanager/notify"
	"dodevops-api/common"
	"dodevops-api/pkg/log"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

const (
	feishuEventDedupTTL     = 24 * time.Hour
	feishuRedisClaimTimeout = 800 * time.Millisecond
)

type feishuManagedWSClient struct {
	client    *larkws.Client
	appSecret string
	cancel    context.CancelFunc
}

type feishuEventClaim struct {
	key      string
	useRedis bool
}

type feishuHTTPChannelContextKey struct{}

type feishuWSWatcher struct {
	dao               *dao.AlertManagerDao
	mu                sync.Mutex
	ctx               context.Context
	clients           map[string]feishuManagedWSClient
	processedEvents   map[string]time.Time
	processedMessages map[string]time.Time
	processedActions  map[string]time.Time
}

var (
	sharedFeishuWatcherOnce sync.Once
	sharedFeishuWatcherInst *feishuWSWatcher
)

func newFeishuWSWatcher() *feishuWSWatcher {
	return &feishuWSWatcher{
		dao:               dao.NewAlertManagerDao(),
		clients:           make(map[string]feishuManagedWSClient),
		processedEvents:   make(map[string]time.Time),
		processedMessages: make(map[string]time.Time),
		processedActions:  make(map[string]time.Time),
	}
}

func sharedFeishuWatcher() *feishuWSWatcher {
	sharedFeishuWatcherOnce.Do(func() {
		sharedFeishuWatcherInst = newFeishuWSWatcher()
	})
	return sharedFeishuWatcherInst
}

func StartFeishuWSWatcher(ctx context.Context) {
	sharedFeishuWatcher().Run(ctx)
}

func refreshFeishuEventTransports() {
	w := sharedFeishuWatcher()
	w.sync(w.eventContext())
}

func (w *feishuWSWatcher) Run(ctx context.Context) {
	w.mu.Lock()
	w.ctx = ctx
	w.mu.Unlock()
	w.sync(ctx)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.closeAllClients()
			log.Log().Info("feishu ws watcher stopped")
			return
		case <-ticker.C:
			w.sync(ctx)
		}
	}
}

func (w *feishuWSWatcher) sync(ctx context.Context) {
	channels, err := w.dao.ListAllNotifyChannels()
	if err != nil {
		log.Log().Warnf("feishu ws watcher list notify channels failed: %v", err)
		return
	}

	httpApps := map[string]bool{}
	webSocketApps := map[string]string{}
	for _, channel := range channels {
		if strings.TrimSpace(channel.Type) != model.NotifyChannelFeishuCard || channel.Enabled != model.RuleStatusEnabled {
			continue
		}
		cfg, err := alertnotify.ParseFeishuCardAppConfig(channel.Config)
		if err != nil {
			log.Log().Warnf("feishu ws watcher parse config failed: channel=%d err=%v", channel.ID, err)
			continue
		}
		appID := strings.TrimSpace(cfg.AppID)
		appSecret := strings.TrimSpace(cfg.AppSecret)
		if appID == "" || appSecret == "" {
			continue
		}
		if cfg.UsesHTTPCallbacks() {
			httpApps[appID] = true
			continue
		}
		webSocketApps[appID] = appSecret
	}
	for appID := range httpApps {
		delete(webSocketApps, appID)
	}
	for appID, appSecret := range webSocketApps {
		w.ensureClient(ctx, appID, appSecret)
	}

	removed := make(map[string]feishuManagedWSClient)
	w.mu.Lock()
	for appID, client := range w.clients {
		if _, seen := webSocketApps[appID]; seen {
			continue
		}
		removed[appID] = client
		delete(w.clients, appID)
	}
	w.mu.Unlock()
	for appID, client := range removed {
		closeFeishuManagedWSClient(client)
		reason := "channel_disabled_or_removed"
		if httpApps[appID] {
			reason = "http_mode_enabled"
		}
		log.Log().Infof("feishu ws watcher stopped client: appId=%s reason=%s", appID, reason)
	}
}

func (w *feishuWSWatcher) ensureClient(ctx context.Context, appID, appSecret string) {
	w.mu.Lock()
	existing, exists := w.clients[appID]
	if exists && existing.appSecret == appSecret {
		w.mu.Unlock()
		return
	}
	if exists {
		delete(w.clients, appID)
	}
	clientCtx, cancel := context.WithCancel(ctx)
	var client *larkws.Client
	handler := w.buildEventDispatcher()
	client = larkws.NewClient(appID, appSecret,
		larkws.WithEventHandler(handler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
		larkws.WithOnReady(func() {
			w.mu.Lock()
			current, active := w.clients[appID]
			active = active && current.client == client
			w.mu.Unlock()
			if !active {
				client.Close()
				log.Log().Infof("feishu ws watcher closed stale client after connect: appId=%s", appID)
			}
		}),
	)
	w.clients[appID] = feishuManagedWSClient{client: client, appSecret: appSecret, cancel: cancel}
	w.mu.Unlock()

	if exists {
		closeFeishuManagedWSClient(existing)
		log.Log().Infof("feishu ws watcher restarting client: appId=%s reason=app_secret_changed", appID)
	}
	go func() {
		log.Log().Infof("feishu ws watcher starting client: appId=%s", appID)
		if err := client.Start(clientCtx); err != nil {
			log.Log().Warnf("feishu ws watcher client stopped: appId=%s err=%v", appID, err)
		}
	}()
}

func (w *feishuWSWatcher) buildEventDispatcher() *dispatcher.EventDispatcher {
	return dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(eventCtx context.Context, event *larkim.P2MessageReceiveV1) error {
			return w.handleMessageEvent(eventCtx, event)
		}).
		OnP2ChatAccessEventBotP2pChatEnteredV1(func(eventCtx context.Context, event *larkim.P2ChatAccessEventBotP2pChatEnteredV1) error {
			return w.handleP2PChatEnteredEvent(eventCtx, event)
		}).
		OnP2CardActionTrigger(func(eventCtx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
			return w.handleCardActionEvent(eventCtx, event)
		})
}

func (w *feishuWSWatcher) closeAllClients() {
	w.mu.Lock()
	clients := w.clients
	w.clients = make(map[string]feishuManagedWSClient)
	w.mu.Unlock()
	for _, client := range clients {
		closeFeishuManagedWSClient(client)
	}
}

func (w *feishuWSWatcher) stopWebSocketApp(appID string) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return
	}
	w.mu.Lock()
	client, exists := w.clients[appID]
	if exists {
		delete(w.clients, appID)
	}
	w.mu.Unlock()
	if exists {
		closeFeishuManagedWSClient(client)
		log.Log().Infof("feishu ws watcher stopped client: appId=%s reason=http_callback_received", appID)
	}
}

func closeFeishuManagedWSClient(client feishuManagedWSClient) {
	if client.cancel != nil {
		client.cancel()
	}
	if client.client != nil {
		client.client.Close()
	}
}

func (w *feishuWSWatcher) dispatchHTTPEvent(ctx context.Context, channelID uint, payload []byte) (interface{}, error) {
	return w.buildEventDispatcher().Do(context.WithValue(ctx, feishuHTTPChannelContextKey{}, channelID), payload)
}

func (w *feishuWSWatcher) claimHTTPEvent(ctx context.Context, appID, eventID string) (bool, feishuEventClaim) {
	key := "aiops:feishu:event:" + strings.TrimSpace(appID) + ":" + strings.TrimSpace(eventID)
	if client := common.GetRedisClient(); client != nil {
		opCtx, cancel := context.WithTimeout(ctx, feishuRedisClaimTimeout)
		claimed, err := client.SetNX(opCtx, key, "1", feishuEventDedupTTL).Result()
		cancel()
		if err == nil {
			return claimed, feishuEventClaim{key: key, useRedis: true}
		}
		log.Log().Warnf("feishu callback redis idempotency failed, using memory fallback: appId=%s eventId=%s err=%v", appID, eventID, err)
	}
	claimed := w.claimFeishuAIOpsDedup(w.processedEvents, key, feishuEventDedupTTL)
	return claimed, feishuEventClaim{key: key}
}

func (w *feishuWSWatcher) releaseHTTPEventClaim(claim feishuEventClaim) {
	if claim.key == "" {
		return
	}
	if claim.useRedis {
		if client := common.GetRedisClient(); client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), feishuRedisClaimTimeout)
			if err := client.Del(ctx, claim.key).Err(); err != nil {
				log.Log().Warnf("feishu callback release redis idempotency failed: key=%s err=%v", claim.key, err)
			}
			cancel()
		}
		return
	}
	w.mu.Lock()
	delete(w.processedEvents, claim.key)
	w.mu.Unlock()
}

func (w *feishuWSWatcher) eventContext() context.Context {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ctx != nil {
		return w.ctx
	}
	return context.Background()
}

func (w *feishuWSWatcher) handleP2PChatEnteredEvent(_ context.Context, event *larkim.P2ChatAccessEventBotP2pChatEnteredV1) error {
	if event == nil || event.Event == nil {
		return nil
	}
	appID := ""
	if event.EventV2Base != nil && event.EventV2Base.Header != nil {
		appID = strings.TrimSpace(event.EventV2Base.Header.AppID)
	}
	log.Log().Infof("feishu ws p2p chat entered: appId=%s chatId=%s", appID, strings.TrimSpace(pointerString(event.Event.ChatId)))
	return nil
}

func (w *feishuWSWatcher) handleMessageEvent(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil
	}
	chatID := strings.TrimSpace(pointerString(event.Event.Message.ChatId))
	chatType := strings.TrimSpace(pointerString(event.Event.Message.ChatType))
	appID := ""
	if event.EventV2Base != nil && event.EventV2Base.Header != nil {
		appID = strings.TrimSpace(event.EventV2Base.Header.AppID)
	}
	log.Log().Infof("feishu message event: appId=%s chatId=%s chatType=%s", appID, chatID, chatType)
	if appID == "" || chatID == "" {
		return nil
	}

	channels, err := w.dao.ListAllNotifyChannels()
	if err != nil {
		log.Log().Warnf("feishu ws handle list channels failed: %v", err)
		return err
	}
	var channel *model.AlertNotifyChannel
	if channelID, ok := ctx.Value(feishuHTTPChannelContextKey{}).(uint); ok && channelID > 0 {
		for i := range channels {
			if channels[i].ID == channelID {
				channel = &channels[i]
				break
			}
		}
	} else {
		channel = alertnotify.MatchFeishuCardChannelByAppID(channels, appID)
	}
	if channel == nil {
		log.Log().Warnf("feishu handle channel not found: appId=%s chatId=%s", appID, chatID)
		return nil
	}
	if channel.Enabled != model.RuleStatusEnabled || strings.TrimSpace(channel.Type) != model.NotifyChannelFeishuCard {
		log.Log().Infof("feishu message ignored: channel=%d appId=%s reason=channel_disabled_or_type_mismatch", channel.ID, appID)
		return nil
	}
	cfg, err := alertnotify.ParseFeishuCardAppConfig(channel.Config)
	if err != nil {
		log.Log().Warnf("feishu handle parse config failed: channel=%d err=%v", channel.ID, err)
		return err
	}
	if strings.TrimSpace(cfg.AppID) != appID {
		log.Log().Warnf("feishu message ignored: channel=%d eventAppId=%s configAppId=%s reason=app_id_mismatch", channel.ID, appID, cfg.AppID)
		return nil
	}
	newConfig := cloneJSONMap(channel.Config)
	newConfig["lastChatId"] = chatID
	if strings.TrimSpace(cfg.DefaultReceiveID) == "" && chatType == "group" {
		newConfig["defaultReceiveId"] = chatID
		if strings.TrimSpace(cfg.ReceiveIDType) == "" {
			newConfig["receiveIdType"] = "chat_id"
		}
	}
	if err := w.dao.UpdateNotifyChannelConfig(channel.ID, newConfig, "system"); err != nil {
		log.Log().Warnf("feishu save chat_id failed: channel=%d err=%v", channel.ID, err)
		return err
	}
	log.Log().Infof("feishu saved chat_id: channel=%d appId=%s chatId=%s defaultReceiveId=%v lastChatId=%v",
		channel.ID, appID, chatID, newConfig["defaultReceiveId"], newConfig["lastChatId"])
	w.dispatchAIOpsMessage(*channel, cfg, event)
	return nil
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (w *feishuWSWatcher) handleCardActionEvent(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	if event == nil || event.Event == nil || event.Event.Action == nil {
		log.Log().Warnf("feishu card action event is nil")
		return nil, nil
	}

	action := event.Event.Action
	openID := ""
	if event.Event.Operator != nil {
		openID = strings.TrimSpace(event.Event.Operator.OpenID)
	}

	log.Log().Infof("feishu card action received: tag=%s openId=%s", action.Tag, openID)

	// 解析按钮值
	actionData := action.Value
	if actionData == nil {
		actionData = make(map[string]interface{})
	}

	actionType := strings.TrimSpace(getAnyString(actionData, "action", "action_type", "actionType"))
	eventIDStr := strings.TrimSpace(getAnyString(actionData, "eventId", "event_id", "eventID"))
	eventIDStrs := strings.TrimSpace(getAnyString(actionData, "eventIds", "event_ids", "eventIDs"))
	fingerprint := strings.TrimSpace(getAnyString(actionData, "fingerprint"))

	eventIDs := parseEventIDs(eventIDStrs, eventIDStr)

	log.Log().Infof("feishu card action parsed: actionType=%s eventId=%s eventIds=%v fingerprint=%s", actionType, eventIDStr, eventIDs, fingerprint)

	if actionType == "" {
		return w.buildCardActionResponse("按钮动作类型为空", false), nil
	}

	// 根据动作类型处理
	switch actionType {
	case "claim":
		return w.handleClaimAction(ctx, eventIDs, fingerprint, openID)
	case "silence":
		return w.handleSilenceAction(ctx, eventIDs, fingerprint, actionData, openID)
	case "aiops_confirm":
		return w.handleAIOpsConfirmAction(event, actionData, openID)
	case "aiops_cancel":
		return w.handleAIOpsCancelAction(event, actionData, openID)
	default:
		return w.buildCardActionResponse("未知的按钮动作: "+actionType, false), nil
	}
}

func (w *feishuWSWatcher) handleClaimAction(ctx context.Context, eventIDs []uint64, fingerprint, operator string) (*callback.CardActionTriggerResponse, error) {
	if len(eventIDs) == 0 {
		log.Log().Warnf("feishu card claim invalid eventIds: fingerprint=%s", fingerprint)
		return w.buildCardActionResponse("无效的事件ID", false), nil
	}

	if operator == "" {
		operator = "feishu_user"
	}

	events, err := w.dao.ListActiveEventsByIDs(eventIDs)
	if err != nil {
		log.Log().Warnf("feishu card claim list events failed: eventIds=%v err=%v", eventIDs, err)
		return w.buildCardActionResponse("查询告警事件失败", false), nil
	}
	if len(events) == 0 {
		return w.buildCardActionResponse("告警事件不存在或已恢复", false), nil
	}

	svc := newAlertManagerService()
	claimed := 0
	alreadyClaimed := 0
	for _, event := range events {
		if strings.TrimSpace(event.ClaimUser) != "" {
			alreadyClaimed++
			continue
		}
		if err := svc.claimActiveEventInternal(event.ID, operator); err != nil {
			log.Log().Warnf("feishu card claim failed: eventId=%d operator=%s err=%v", event.ID, operator, err)
			return w.buildCardActionResponse("认领失败: "+err.Error(), false), nil
		}
		claimed++
	}

	if claimed == 0 && alreadyClaimed > 0 {
		return w.buildCardActionResponse("该聚合告警已被认领", false), nil
	}

	log.Log().Infof("feishu card claim success: eventIds=%v operator=%s claimed=%d alreadyClaimed=%d", eventIDs, operator, claimed, alreadyClaimed)

	return w.buildCardActionResponseWithCard(fmt.Sprintf("✅ 已认领 %d 个告警事件", claimed), true, buildHandledCard("已认领", operator, claimed)), nil
}

func (w *feishuWSWatcher) handleSilenceAction(ctx context.Context, eventIDs []uint64, fingerprint string, actionData map[string]interface{}, operator string) (*callback.CardActionTriggerResponse, error) {
	if len(eventIDs) == 0 {
		log.Log().Warnf("feishu card silence invalid eventIds: fingerprint=%s", fingerprint)
		return w.buildCardActionResponse("无效的事件ID", false), nil
	}

	events, err := w.dao.ListActiveEventsByIDs(eventIDs)
	if err != nil {
		log.Log().Warnf("feishu card silence list events failed: eventIds=%v err=%v", eventIDs, err)
		return w.buildCardActionResponse("查询告警事件失败: "+err.Error(), false), nil
	}
	if len(events) == 0 {
		return w.buildCardActionResponse("告警事件不存在或已恢复", false), nil
	}

	duration := strings.TrimSpace(getString(actionData, "duration"))
	if duration == "" {
		duration = "1h"
	}

	var durationMinutes int64 = 60
	if strings.HasSuffix(duration, "h") {
		hours, _ := strconv.Atoi(strings.TrimSuffix(duration, "h"))
		durationMinutes = int64(hours * 60)
	} else if strings.HasSuffix(duration, "m") {
		minutes, _ := strconv.Atoi(strings.TrimSuffix(duration, "m"))
		durationMinutes = int64(minutes)
	}

	endTime := time.Now().Add(time.Duration(durationMinutes) * time.Minute)

	if operator == "" {
		operator = "feishu_user"
	}

	svc := newAlertManagerService()
	silenced := 0
	for i := range events {
		if events[i].Silenced == 1 {
			continue
		}
		if _, err := svc.silenceActiveEventInternal(&events[i], endTime, "通过飞书卡片按钮静默", operator, nil); err != nil {
			log.Log().Warnf("feishu card silence failed: eventId=%d operator=%s err=%v", events[i].ID, operator, err)
			return w.buildCardActionResponse("静默失败: "+err.Error(), false), nil
		}
		silenced++
	}

	if silenced == 0 {
		return w.buildCardActionResponse("该聚合告警已静默", false), nil
	}

	log.Log().Infof("feishu card silence success: eventIds=%v operator=%s duration=%s silenced=%d", eventIDs, operator, duration, silenced)

	return w.buildCardActionResponseWithCard(fmt.Sprintf("✅ 已静默 %d 个告警事件，时长 %s", silenced, duration), true, buildHandledCard("已静默", operator, silenced)), nil
}

func (w *feishuWSWatcher) buildCardActionResponse(message string, success bool) *callback.CardActionTriggerResponse {
	return w.buildCardActionResponseWithCard(message, success, nil)
}

func (w *feishuWSWatcher) buildCardActionResponseWithCard(message string, success bool, card map[string]interface{}) *callback.CardActionTriggerResponse {
	toastType := "error"
	if success {
		toastType = "success"
	}
	resp := &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{
			Type:    toastType,
			Content: message,
		},
	}
	if card != nil {
		resp.Card = &callback.Card{
			Type: "raw",
			Data: card,
		}
	}
	return resp
}

func (w *feishuWSWatcher) updateCardAfterClaim(ctx context.Context, eventID, operator string) {
	// TODO: 实现批量更新卡片实体接口调用
	// 由于需要 message_id 和 sequence，暂时记录日志
	log.Log().Infof("feishu card update needed: eventId=%s operator=%s action=claim", eventID, operator)
	//
	// 完整实现需要：
	// 1. 从数据库查询 message_id（需要在发送卡片时记录）
	// 2. 维护 sequence 计数器（每次更新递增）
	// 3. 调用飞书的批量更新卡片实体接口
	//
	// 接口示例：
	// POST https://open.feishu.cn/open-apis/im/v1/messages/:message_id/card/update
	// {
	//   "sequence": 2,
	//   "update_element": [
	//     {
	//       "id": "status_info_1",
	//       "element": {
	//         "tag": "div",
	//         "text": {
	//           "tag": "lark_md",
	//           "content": "✅ **已认领** | 操作人：xxx"
	//         }
	//       }
	//     },
	//     {
	//       "id": "claim_button_1",
	//       "element": {
	//         "tag": "button",
	//         "text": {"tag": "plain_text", "content": "已认领"},
	//         "type": "default",
	//         "disabled": true
	//       }
	//     }
	//   ]
	// }
}

func (w *feishuWSWatcher) updateCardAfterSilence(ctx context.Context, eventID, operator string) {
	// TODO: 实现批量更新卡片实体接口调用
	log.Log().Infof("feishu card update needed: eventId=%s operator=%s action=silence", eventID, operator)
}

func buildHandledCard(status, operator string, count int) map[string]interface{} {
	if count <= 0 {
		count = 1
	}
	claimText := "已认领"
	silenceText := "已静默"
	return map[string]interface{}{
		"schema": "2.0",
		"header": map[string]interface{}{
			"template": "grey",
			"title": map[string]interface{}{
				"tag":     "plain_text",
				"content": "【" + status + "】告警已处理",
			},
		},
		"body": map[string]interface{}{
			"elements": []interface{}{
				map[string]interface{}{
					"tag": "div",
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**状态**: %s\n**操作人**: %s\n**事件数**: %d\n**时间**: %s", status, operator, count, time.Now().Format("2006-01-02 15:04:05")),
					},
				},
				map[string]interface{}{
					"tag": "hr",
				},
				map[string]interface{}{
					"tag":                "column_set",
					"flex_mode":          "none",
					"horizontal_spacing": "16px",
					"columns": []interface{}{
						map[string]interface{}{
							"tag":   "column",
							"width": "weighted",
							"elements": []interface{}{
								map[string]interface{}{
									"tag":      "button",
									"text":     map[string]interface{}{"tag": "plain_text", "content": claimText},
									"type":     "default",
									"disabled": true,
								},
							},
						},
						map[string]interface{}{
							"tag":   "column",
							"width": "weighted",
							"elements": []interface{}{
								map[string]interface{}{
									"tag":      "button",
									"text":     map[string]interface{}{"tag": "plain_text", "content": silenceText},
									"type":     "default",
									"disabled": true,
								},
							},
						},
					},
				},
			},
		},
	}
}

func getString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	value, ok := m[key]
	if !ok || value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

func getAnyString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := getString(m, key); value != "" {
			return value
		}
	}
	return ""
}

func parseEventIDs(values ...string) []uint64 {
	result := make([]uint64, 0)
	seen := map[uint64]bool{}
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			id, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64)
			if err != nil || id == 0 || seen[id] {
				continue
			}
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}
