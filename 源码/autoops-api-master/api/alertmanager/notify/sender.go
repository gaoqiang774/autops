package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dodevops-api/api/alertmanager/model"
)

const defaultTimeoutSeconds = 10
const feishuTenantTokenURL = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
const feishuMessageCreateURL = "https://open.feishu.cn/open-apis/im/v1/messages"
const blockedLeliaoUserID = "000001"

const (
	FeishuEventModeWebSocket = "websocket"
	FeishuEventModeHTTP      = "http"
)

type SendRequest struct {
	Channel   model.AlertNotifyChannel
	Title     string
	Content   string
	Card      model.JSONMap
	Receivers []string
	UserIDs   []string
}

type SendResult struct {
	Target       string
	Status       string
	Response     string
	ErrorMessage string
	DurationMs   int64
}

type channelConfig struct {
	Webhook              string   `json:"webhook"`
	AccessToken          string   `json:"accessToken"`
	Key                  string   `json:"key"`
	Secret               string   `json:"secret"`
	AppID                string   `json:"appId"`
	AppSecret            string   `json:"appSecret"`
	VerificationToken    string   `json:"verificationToken"`
	EncryptKey           string   `json:"encryptKey"`
	EventMode            string   `json:"eventMode"`
	ReceiveIDType        string   `json:"receiveIdType"`
	DefaultReceiveID     string   `json:"defaultReceiveId"`
	LastChatID           string   `json:"lastChatId"`
	PostKey              string   `json:"postKey"`
	SourceName           string   `json:"sourceName"`
	Type                 int      `json:"type"`
	Receivers            []string `json:"receivers"`
	UserIDs              []string `json:"userIds"`
	TimeoutSeconds       int      `json:"timeoutSeconds"`
	RetryTimes           int      `json:"retryTimes"`
	RetryIntervalSeconds int      `json:"retryIntervalSeconds"`
	AIOpsEnabled         bool     `json:"aiopsEnabled"`
	AIOpsRequireMention  *bool    `json:"aiopsRequireMention"`
	Template             template `json:"template"`
	At                   atConfig `json:"at"`
}

type template struct {
	Title   string                 `json:"title"`
	Content string                 `json:"content"`
	Card    map[string]interface{} `json:"card"`
}

type atConfig struct {
	Enabled bool     `json:"enabled"`
	Mobiles []string `json:"mobiles"`
	UserIDs []string `json:"userIds"`
	IsAtAll bool     `json:"isAtAll"`
}

func Send(ctx context.Context, req SendRequest) SendResult {
	start := time.Now()
	target, response, err := send(ctx, req)
	result := SendResult{
		Target:     target,
		Response:   response,
		DurationMs: time.Since(start).Milliseconds(),
	}
	if err != nil {
		result.Status = model.NotifyStatusFailed
		result.ErrorMessage = err.Error()
		return result
	}
	result.Status = model.NotifyStatusSuccess
	return result
}

func ValidateChannel(channel model.AlertNotifyChannel) error {
	cfg, err := parseConfig(channel.Config)
	if err != nil {
		return err
	}
	channelType := strings.TrimSpace(channel.Type)
	if !IsSupportedChannelType(channelType) {
		return fmt.Errorf("unsupported notify channel type: %s", channelType)
	}
	if channelType == model.NotifyChannelFeishuCard {
		if strings.TrimSpace(cfg.AppID) == "" {
			return errors.New("appId is required")
		}
		if strings.TrimSpace(cfg.AppSecret) == "" {
			return errors.New("appSecret is required")
		}
		explicitEventMode := strings.ToLower(strings.TrimSpace(cfg.EventMode))
		switch explicitEventMode {
		case "", FeishuEventModeWebSocket, FeishuEventModeHTTP:
		default:
			return errors.New("eventMode must be websocket or http")
		}
		if cfg.resolvedFeishuEventMode() == FeishuEventModeHTTP {
			if strings.TrimSpace(cfg.VerificationToken) == "" {
				return errors.New("verificationToken is required when eventMode is http")
			}
		}
		if strings.TrimSpace(cfg.ReceiveIDType) != "" {
			switch strings.TrimSpace(cfg.ReceiveIDType) {
			case "chat_id", "open_id", "user_id", "email":
			default:
				return errors.New("receiveIdType must be chat_id, open_id, user_id or email")
			}
		}
		return nil
	}
	if channelType == model.NotifyChannelLeliao {
		if resolveWebhook(channelType, cfg) == "" {
			return errors.New("webhook is required")
		}
		if strings.TrimSpace(cfg.PostKey) == "" {
			return errors.New("postKey is required")
		}
		return nil
	}
	if resolveWebhook(channelType, cfg) == "" {
		return errors.New("webhook or accessToken/key is required")
	}
	return nil
}

func IsSupportedChannelType(channelType string) bool {
	switch strings.TrimSpace(channelType) {
	case model.NotifyChannelFeishu, model.NotifyChannelFeishuCard, model.NotifyChannelDingtalk, model.NotifyChannelWecom, model.NotifyChannelLeliao:
		return true
	default:
		return false
	}
}

func send(ctx context.Context, req SendRequest) (string, string, error) {
	cfg, err := parseConfig(req.Channel.Config)
	if err != nil {
		return "", "", err
	}
	channelType := strings.TrimSpace(req.Channel.Type)
	webhook := resolveWebhook(channelType, cfg)
	if webhook == "" && channelType == model.NotifyChannelLeliao {
		return "", "", errors.New("webhook is required")
	}
	if webhook == "" && channelType != model.NotifyChannelFeishuCard {
		return "", "", errors.New("webhook or accessToken/key is required")
	}

	title := firstNonEmpty(req.Title, cfg.Template.Title, "AIOps 告警通知测试")
	content := firstNonEmpty(req.Content, cfg.Template.Content, "这是一条来自 AIOps 平台的通知测试消息")

	var body interface{}
	switch channelType {
	case model.NotifyChannelFeishu:
		body = feishuTextBody(content)
		body = applyFeishuSign(body, cfg.Secret)
	case model.NotifyChannelFeishuCard:
		receivers := mergeReceivers(req.Receivers, req.UserIDs)
		if len(receivers) == 0 && strings.TrimSpace(cfg.DefaultReceiveID) != "" {
			receivers = []string{strings.TrimSpace(cfg.DefaultReceiveID)}
		}
		if len(receivers) == 0 && strings.TrimSpace(cfg.LastChatID) != "" {
			receivers = []string{strings.TrimSpace(cfg.LastChatID)}
		}
		if len(receivers) == 0 {
			return "", "", errors.New("defaultReceiveId or lastChatId or userIds is required")
		}
		card := firstNonEmptyCard(req.Card, cfg.Template.Card)
		if len(card) == 0 {
			card = feishuCardBody(title, content, nil)["card"].(map[string]interface{})
		}
		return postFeishuCardApp(ctx, receivers, card, cfg)
	case model.NotifyChannelDingtalk:
		if cfg.Secret != "" {
			webhook = applyDingtalkSign(webhook, cfg.Secret)
		}
		body = dingtalkMarkdownBody(title, content, cfg.At)
	case model.NotifyChannelWecom:
		body = wecomMarkdownBody(title, content)
	case model.NotifyChannelLeliao:
		receivers := mergeReceivers(req.Receivers, req.UserIDs)
		if len(receivers) == 0 {
			receivers = mergeReceivers(cfg.Receivers, cfg.UserIDs, cfg.At.UserIDs, cfg.At.Mobiles)
		}
		receivers = filterLeliaoReceivers(receivers)
		if len(receivers) == 0 {
			return webhook, "", errors.New("receivers or userIds is required")
		}
		return postLeliao(ctx, webhook, receivers, title, content, cfg)
	default:
		return webhook, "", fmt.Errorf("unsupported notify channel type: %s", channelType)
	}
	return postJSON(ctx, webhook, body, cfg)
}

func parseConfig(raw model.JSONMap) (channelConfig, error) {
	var cfg channelConfig
	if raw == nil {
		return cfg, nil
	}
	bs, err := json.Marshal(raw)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(bs, &cfg); err != nil {
		return cfg, err
	}
	applyChannelConfigStringAlias(&cfg.EventMode, raw, "event_mode")
	applyChannelConfigStringAlias(&cfg.VerificationToken, raw, "verification_token")
	applyChannelConfigStringAlias(&cfg.EncryptKey, raw, "encrypt_key")
	return cfg, nil
}

func applyChannelConfigStringAlias(target *string, raw model.JSONMap, alias string) {
	if target == nil || strings.TrimSpace(*target) != "" || raw == nil {
		return
	}
	value, ok := raw[alias].(string)
	if ok {
		*target = value
	}
}

func resolveWebhook(channelType string, cfg channelConfig) string {
	if strings.TrimSpace(cfg.Webhook) != "" {
		return strings.TrimSpace(cfg.Webhook)
	}
	token := firstNonEmpty(cfg.AccessToken, cfg.Key)
	switch channelType {
	case model.NotifyChannelFeishu, model.NotifyChannelFeishuCard:
		if token != "" {
			return "https://open.feishu.cn/open-apis/bot/v2/hook/" + token
		}
	case model.NotifyChannelDingtalk:
		if token != "" {
			return "https://oapi.dingtalk.com/robot/send?access_token=" + token
		}
	case model.NotifyChannelWecom:
		if token != "" {
			return "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=" + token
		}
	case model.NotifyChannelLeliao:
		return strings.TrimSpace(cfg.Webhook)
	}
	return ""
}

func feishuTextBody(content string) map[string]interface{} {
	return map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": content,
		},
	}
}

func feishuCardBody(title, content string, custom map[string]interface{}) map[string]interface{} {
	if len(custom) > 0 {
		return map[string]interface{}{
			"msg_type": "interactive",
			"card":     custom,
		}
	}
	return map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"config": map[string]interface{}{
				"wide_screen_mode": true,
				"enable_forward":   true,
			},
			"header": map[string]interface{}{
				"template": "red",
				"title": map[string]string{
					"tag":     "plain_text",
					"content": title,
				},
			},
			"elements": []map[string]interface{}{
				{
					"tag": "div",
					"text": map[string]string{
						"tag":     "lark_md",
						"content": content,
					},
				},
				{"tag": "hr"},
				{
					"tag": "note",
					"elements": []map[string]string{
						{"tag": "plain_text", "content": "AIOps 运维平台"},
					},
				},
			},
		},
	}
}

func postFeishuCardApp(ctx context.Context, receivers []string, card map[string]interface{}, cfg channelConfig) (string, string, error) {
	receiveIDType := normalizeFeishuReceiveIDType(cfg, receivers[0])
	token, err := fetchFeishuTenantAccessToken(ctx, cfg)
	if err != nil {
		return "", "", err
	}
	messageBody := map[string]interface{}{
		"receive_id": receivers[0],
		"msg_type":   "interactive",
		"content":    string(mustJSON(card)),
	}
	urlWithQuery := feishuMessageCreateURL + "?receive_id_type=" + receiveIDType
	return postJSONWithHeaders(ctx, urlWithQuery, messageBody, map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}, cfg, "feishu_card_app://"+receiveIDType+"/"+receivers[0])
}

func normalizeFeishuReceiveIDType(cfg channelConfig, receiver string) string {
	value := strings.TrimSpace(cfg.ReceiveIDType)
	if value != "" {
		return value
	}
	receiver = strings.TrimSpace(receiver)
	switch {
	case strings.HasPrefix(receiver, "oc_"):
		return "chat_id"
	case strings.HasPrefix(receiver, "ou_"):
		return "open_id"
	case strings.Contains(receiver, "@"):
		return "email"
	default:
		return "user_id"
	}
}

type FeishuCardAppConfig struct {
	AppID               string `json:"appId"`
	AppSecret           string `json:"appSecret"`
	VerificationToken   string `json:"verificationToken"`
	EncryptKey          string `json:"encryptKey"`
	EventMode           string `json:"eventMode"`
	ReceiveIDType       string `json:"receiveIdType"`
	DefaultReceiveID    string `json:"defaultReceiveId"`
	LastChatID          string `json:"lastChatId"`
	AIOpsEnabled        bool   `json:"aiopsEnabled"`
	AIOpsRequireMention *bool  `json:"aiopsRequireMention"`
}

func ParseFeishuCardAppConfig(raw model.JSONMap) (FeishuCardAppConfig, error) {
	cfg, err := parseConfig(raw)
	if err != nil {
		return FeishuCardAppConfig{}, err
	}
	return FeishuCardAppConfig{
		AppID:               cfg.AppID,
		AppSecret:           cfg.AppSecret,
		VerificationToken:   cfg.VerificationToken,
		EncryptKey:          cfg.EncryptKey,
		EventMode:           cfg.EventMode,
		ReceiveIDType:       cfg.ReceiveIDType,
		DefaultReceiveID:    cfg.DefaultReceiveID,
		LastChatID:          cfg.LastChatID,
		AIOpsEnabled:        cfg.AIOpsEnabled,
		AIOpsRequireMention: cfg.AIOpsRequireMention,
	}, nil
}

func (c FeishuCardAppConfig) UsesHTTPCallbacks() bool {
	return c.ResolvedEventMode() == FeishuEventModeHTTP
}

func (c FeishuCardAppConfig) UsesWebSocket() bool {
	return !c.UsesHTTPCallbacks()
}

func (c FeishuCardAppConfig) ResolvedEventMode() string {
	if strings.TrimSpace(c.VerificationToken) != "" {
		return FeishuEventModeHTTP
	}
	mode := strings.ToLower(strings.TrimSpace(c.EventMode))
	if mode == FeishuEventModeHTTP || mode == FeishuEventModeWebSocket {
		return mode
	}
	return FeishuEventModeWebSocket
}

func EnsureFeishuEventMode(channel *model.AlertNotifyChannel) {
	if channel == nil || strings.TrimSpace(channel.Type) != model.NotifyChannelFeishuCard {
		return
	}
	if channel.Config == nil {
		channel.Config = model.JSONMap{}
	}
	cfg, err := ParseFeishuCardAppConfig(channel.Config)
	if err == nil && strings.TrimSpace(cfg.VerificationToken) != "" {
		channel.Config["eventMode"] = FeishuEventModeHTTP
		return
	}
	if value, exists := channel.Config["eventMode"]; exists && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" {
		return
	}
	if value, exists := channel.Config["event_mode"]; exists && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" {
		channel.Config["eventMode"] = value
		return
	}
	if err != nil {
		return
	}
	channel.Config["eventMode"] = cfg.ResolvedEventMode()
}

func PreserveFeishuServerConfig(channel *model.AlertNotifyChannel, existing model.JSONMap) {
	if channel == nil || strings.TrimSpace(channel.Type) != model.NotifyChannelFeishuCard || existing == nil {
		return
	}
	if channel.Config == nil {
		channel.Config = model.JSONMap{}
	}
	incomingCfg, incomingErr := ParseFeishuCardAppConfig(channel.Config)
	existingCfg, existingErr := ParseFeishuCardAppConfig(existing)
	if incomingErr != nil || existingErr != nil || strings.TrimSpace(incomingCfg.AppID) == "" || strings.TrimSpace(incomingCfg.AppID) != strings.TrimSpace(existingCfg.AppID) {
		return
	}
	for _, field := range []struct {
		canonical string
		aliases   []string
	}{
		{canonical: "verificationToken", aliases: []string{"verification_token"}},
		{canonical: "encryptKey", aliases: []string{"encrypt_key"}},
		{canonical: "eventMode", aliases: []string{"event_mode"}},
		{canonical: "receiveIdType", aliases: []string{"receive_id_type"}},
		{canonical: "defaultReceiveId", aliases: []string{"default_receive_id"}},
		{canonical: "lastChatId", aliases: []string{"last_chat_id"}},
	} {
		preserveFeishuConfigValue(channel.Config, existing, field.canonical, field.aliases...)
	}
}

func preserveFeishuConfigValue(incoming, existing model.JSONMap, canonical string, aliases ...string) {
	keys := append([]string{canonical}, aliases...)
	for _, key := range keys {
		if _, exists := incoming[key]; exists {
			return
		}
	}
	for _, key := range keys {
		if value, exists := existing[key]; exists {
			incoming[canonical] = value
			return
		}
	}
}

func (c channelConfig) resolvedFeishuEventMode() string {
	return FeishuCardAppConfig{
		EventMode:         c.EventMode,
		VerificationToken: c.VerificationToken,
	}.ResolvedEventMode()
}

func (c FeishuCardAppConfig) AllowsAIOpsChat(chatID string) bool {
	if !c.AIOpsEnabled {
		return false
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return false
	}
	return true
}

func (c FeishuCardAppConfig) RequiresAIOpsMention() bool {
	return c.AIOpsRequireMention == nil || *c.AIOpsRequireMention
}

func fetchFeishuTenantAccessToken(ctx context.Context, cfg channelConfig) (string, error) {
	body := map[string]string{
		"app_id":     strings.TrimSpace(cfg.AppID),
		"app_secret": strings.TrimSpace(cfg.AppSecret),
	}
	timeoutSeconds := cfg.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultTimeoutSeconds
	}
	client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, feishuTenantTokenURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch tenant_access_token failed: http %d body=%s", resp.StatusCode, string(bs))
	}
	var result struct {
		Code              interface{} `json:"code"`
		Msg               string      `json:"msg"`
		TenantAccessToken string      `json:"tenant_access_token"`
	}
	if err := json.Unmarshal(bs, &result); err != nil {
		return "", err
	}
	if fmt.Sprint(result.Code) != "0" {
		return "", fmt.Errorf("fetch tenant_access_token failed: %s", firstNonEmpty(result.Msg, string(bs)))
	}
	if strings.TrimSpace(result.TenantAccessToken) == "" {
		return "", errors.New("tenant_access_token is empty")
	}
	return result.TenantAccessToken, nil
}

func firstNonEmptyCard(cards ...map[string]interface{}) map[string]interface{} {
	for _, card := range cards {
		if len(card) > 0 {
			return card
		}
	}
	return nil
}

func dingtalkMarkdownBody(title, content string, at atConfig) map[string]interface{} {
	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  fmt.Sprintf("**%s**\n\n%s", title, content),
		},
	}
	if at.Enabled {
		body["at"] = map[string]interface{}{
			"atMobiles": at.Mobiles,
			"atUserIds": at.UserIDs,
			"isAtAll":   at.IsAtAll,
		}
	}
	return body
}

func wecomMarkdownBody(title, content string) map[string]interface{} {
	return map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": fmt.Sprintf("**%s**\n\n%s", title, content),
		},
	}
}

func postLeliao(ctx context.Context, webhook string, receivers []string, title, content string, cfg channelConfig) (string, string, error) {
	if strings.TrimSpace(cfg.PostKey) == "" {
		return webhook, "", errors.New("postKey is required")
	}
	receivers = filterLeliaoReceivers(receivers)
	if len(receivers) == 0 {
		return webhook, "", errors.New("receivers or userIds is required")
	}
	targets := strings.Join(receivers, ",")
	messageType := cfg.Type
	if messageType == 0 {
		messageType = 1
	}
	form := url.Values{}
	form.Set("md5", md5Hex(targets+content))
	form.Set("content", content)
	form.Set("workernos", targets)
	form.Set("sourceName", normalizeLeliaoSourceName(cfg.SourceName))
	form.Set("title", normalizeLeliaoTitle(title))
	form.Set("type", strconv.Itoa(messageType))
	_, response, err := postForm(ctx, webhook, form, map[string]string{"POST-KEY": cfg.PostKey}, cfg)
	return "leliao://" + targets, response, err
}

func normalizeLeliaoSourceName(value string) string {
	sourceName := strings.TrimSpace(value)
	if sourceName == "大数据监控中心" {
		return ""
	}
	return sourceName
}

func normalizeLeliaoTitle(value string) string {
	title := strings.TrimSpace(value)
	for _, prefix := range []string{"大数据监控中心-"} {
		title = strings.TrimPrefix(title, prefix)
	}
	return title
}

func applyFeishuSign(body interface{}, secret string) interface{} {
	if secret == "" {
		return body
	}
	item, ok := body.(map[string]interface{})
	if !ok {
		return body
	}
	sign, timestamp := feishuSignature(secret)
	item["sign"] = sign
	item["timestamp"] = timestamp
	return item
}

func feishuSignature(secret string) (string, string) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	stringToSign := fmt.Sprintf("%s\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), timestamp
}

func applyDingtalkSign(webhook, secret string) string {
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/int64(time.Millisecond))
	stringToSign := fmt.Sprintf("%s\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))
	separator := "&"
	if !strings.Contains(webhook, "?") {
		separator = "?"
	}
	return fmt.Sprintf("%s%stimestamp=%s&sign=%s", webhook, separator, timestamp, sign)
}

func postJSON(ctx context.Context, webhook string, body interface{}, cfg channelConfig) (string, string, error) {
	return postJSONWithHeaders(ctx, webhook, body, map[string]string{
		"Content-Type": "application/json",
	}, cfg, webhook)
}

func postJSONWithHeaders(ctx context.Context, webhook string, body interface{}, headers map[string]string, cfg channelConfig, target string) (string, string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return target, "", err
	}
	timeoutSeconds := cfg.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultTimeoutSeconds
	}
	retryTimes := cfg.RetryTimes
	if retryTimes < 0 {
		retryTimes = 0
	}
	retryIntervalSeconds := cfg.RetryIntervalSeconds
	if retryIntervalSeconds <= 0 {
		retryIntervalSeconds = 1
	}

	client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	var lastResponse string
	var lastErr error
	for attempt := 0; attempt <= retryTimes; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(payload))
		if err != nil {
			return target, "", err
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < retryTimes {
				time.Sleep(time.Duration(retryIntervalSeconds) * time.Second)
			}
			continue
		}
		responseBody, readErr := readResponse(resp)
		if readErr != nil {
			lastErr = readErr
			lastResponse = readErr.Error()
		} else {
			lastResponse = responseBody
			lastErr = parsePlatformResponse(responseBody, resp.StatusCode)
		}
		if lastErr == nil {
			return target, lastResponse, nil
		}
		if attempt < retryTimes {
			time.Sleep(time.Duration(retryIntervalSeconds) * time.Second)
		}
	}
	return target, lastResponse, lastErr
}

func postForm(ctx context.Context, webhook string, form url.Values, headers map[string]string, cfg channelConfig) (string, string, error) {
	timeoutSeconds := cfg.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultTimeoutSeconds
	}
	retryTimes := cfg.RetryTimes
	if retryTimes < 0 {
		retryTimes = 0
	}
	retryIntervalSeconds := cfg.RetryIntervalSeconds
	if retryIntervalSeconds <= 0 {
		retryIntervalSeconds = 1
	}

	client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	var lastResponse string
	var lastErr error
	for attempt := 0; attempt <= retryTimes; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, strings.NewReader(form.Encode()))
		if err != nil {
			return webhook, "", err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < retryTimes {
				time.Sleep(time.Duration(retryIntervalSeconds) * time.Second)
			}
			continue
		}
		responseBody, readErr := readResponse(resp)
		if readErr != nil {
			lastErr = readErr
			lastResponse = readErr.Error()
		} else {
			lastResponse = responseBody
			lastErr = parsePlatformResponse(responseBody, resp.StatusCode)
		}
		if lastErr == nil {
			return webhook, lastResponse, nil
		}
		if attempt < retryTimes {
			time.Sleep(time.Duration(retryIntervalSeconds) * time.Second)
		}
	}
	return webhook, lastResponse, lastErr
}

func readResponse(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("status_code:%d,response:%s", resp.StatusCode, string(bs)), nil
}

func parsePlatformResponse(response string, statusCode int) error {
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("unexpected http status: %d", statusCode)
	}
	raw := response
	if index := strings.Index(response, "response:"); index >= 0 {
		raw = response[index+len("response:"):]
	}
	var item map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return nil
	}
	if code, ok := item["errcode"]; ok && fmt.Sprint(code) != "0" {
		return fmt.Errorf("%v", firstNonEmpty(fmt.Sprint(item["errmsg"]), fmt.Sprint(item["message"]), raw))
	}
	if code, ok := item["code"]; ok && fmt.Sprint(code) != "0" {
		return fmt.Errorf("%v", firstNonEmpty(fmt.Sprint(item["msg"]), fmt.Sprint(item["message"]), raw))
	}
	if status, ok := item["STATUS"]; ok && fmt.Sprint(status) != "1" {
		return fmt.Errorf("%v", firstNonEmpty(fmt.Sprint(item["MSG"]), fmt.Sprint(item["message"]), raw))
	}
	return nil
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func mergeReceivers(groups ...[]string) []string {
	seen := map[string]bool{}
	result := make([]string, 0)
	for _, group := range groups {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item == "" || item == blockedLeliaoUserID || seen[item] {
				continue
			}
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func filterLeliaoReceivers(receivers []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(receivers))
	for _, receiver := range receivers {
		receiver = strings.TrimSpace(receiver)
		if receiver == "" || receiver == blockedLeliaoUserID || seen[receiver] {
			continue
		}
		seen[receiver] = true
		result = append(result, receiver)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "<nil>" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mustJSON(value interface{}) []byte {
	bs, _ := json.Marshal(value)
	return bs
}
