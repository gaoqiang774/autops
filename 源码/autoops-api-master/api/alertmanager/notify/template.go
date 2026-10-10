package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	texttemplate "text/template"
	"time"

	"dodevops-api/api/alertmanager/model"
)

// PreviewTemplate renders a notification template with sample or caller-provided event data.
func PreviewTemplate(tpl model.AlertNotifyTemplate, req model.AlertNotifyTemplatePreviewRequest) (model.AlertNotifyTemplatePreviewResponse, error) {
	data := buildTemplateData(req)
	content := tpl.Content
	resp := model.AlertNotifyTemplatePreviewResponse{
		Data: model.JSONMap(data),
		Card: jsonMapValue(content, "card"),
	}

	raw := stringValue(content, "raw")
	if strings.TrimSpace(raw) != "" {
		rendered, err := renderNamedTemplate(raw, data)
		if err != nil {
			return resp, err
		}
		resp.Title = rendered["Title"]
		resp.TitleColor = rendered["TitleColor"]
		resp.Content = rendered["Event"]
		resp.Footer = rendered["Footer"]
		return resp, nil
	}

	var err error
	resp.Title, err = renderTemplateString(stringValue(content, "title"), data)
	if err != nil {
		return resp, err
	}
	resp.TitleColor, err = renderTemplateString(stringValue(content, "titleColor"), data)
	if err != nil {
		return resp, err
	}
	resp.Content, err = renderTemplateString(stringValue(content, "content"), data)
	if err != nil {
		return resp, err
	}
	resp.Footer, err = renderTemplateString(stringValue(content, "footer"), data)
	if err != nil {
		return resp, err
	}
	return resp, nil
}

func BuildChannelTemplatePayload(channelType string, tpl model.AlertNotifyTemplate, req model.AlertNotifyTemplatePreviewRequest) (string, string, model.JSONMap, error) {
	data := buildTemplateData(req)

	// 对于飞书卡片，直接渲染 JSON 模板
	if channelType == model.NotifyChannelFeishuCard && strings.TrimSpace(stringValue(tpl.Content, "raw")) != "" {
		raw := stringValue(tpl.Content, "raw")
		rendered, err := renderTemplateString(raw, data)
		if err != nil {
			return "", "", nil, err
		}
		var card model.JSONMap
		if err := json.Unmarshal([]byte(rendered), &card); err != nil {
			return "", "", nil, fmt.Errorf("invalid card JSON: %w", err)
		}
		title := extractCardTitle(card)
		return title, "", card, nil
	}

	// 其他渠道使用原有逻辑
	resp, err := PreviewTemplate(tpl, req)
	if err != nil {
		return "", "", nil, err
	}
	card := resp.Card
	if strings.TrimSpace(stringValue(tpl.Content, "raw")) != "" && channelType == model.NotifyChannelFeishuCard {
		card = buildFeishuCardFromRendered(resp)
	}
	content := resp.Content
	if strings.TrimSpace(resp.Footer) != "" && channelType != model.NotifyChannelFeishuCard {
		content = strings.TrimSpace(content + "\n\n" + resp.Footer)
	}
	return resp.Title, content, card, nil
}

func extractCardTitle(card model.JSONMap) string {
	if header, ok := card["header"].(map[string]interface{}); ok {
		if title, ok := header["title"].(map[string]interface{}); ok {
			if content, ok := title["content"].(string); ok {
				return content
			}
		}
	}
	return "AIOps 告警通知"
}

func buildTemplateData(req model.AlertNotifyTemplatePreviewRequest) map[string]interface{} {
	isRecovered := req.IsRecovered
	now := time.Now()
	data := map[string]interface{}{
		"PlatformName":     "AIOps 运维平台",
		"PolicyName":       "默认通知策略",
		"RuleName":         "系统磁盘使用率超过82%",
		"RuleNote":         "磁盘空间使用率持续过高，请及时关注",
		"Fingerprint":      "6e68448a572ac484e05d20d08310a33371918529",
		"EventID":          "1",
		"Severity":         "P0",
		"SeverityIcon":     "🔥🔥🔥",
		"Status":           "firing",
		"IsRecovered":      isRecovered,
		"ResourceName":     "ceshi-quanxian-3-153",
		"Instance":         "172.16.3.153",
		"DatasourceName":   "内置数据源",
		"Condition":        `node_filesystem_usage_percent{mountpoint="/"} >= 82`,
		"ConditionCard":    `node_filesystem_usage_percent{mountpoint=\"/\"} >= 82`,
		"Threshold":        "82",
		"Value":            "84.22",
		"ValueSummary":     "84.22，阈值 >= 82",
		"FirstTriggeredAt": now.Add(-33 * time.Minute).Format("2006-01-02 15:04:05"),
		"TriggeredAt":      now.Add(-33 * time.Minute).Format("2006-01-02 15:04:05"),
		"RecoveredAt":      now.Format("2006-01-02 15:04:05"),
		"Duration":         "33分钟",
		"EventCount":       1,
		"ResourceCount":    1,
		"DutyUser":         "未分配",
		"DetailURL":        "http://localhost:3000/alertmanager/events/1",
		"SilenceURL":       "http://localhost:3000/alertmanager/silence-rules?eventId=1",
		"SendTime":         now.Format("2006-01-02 15:04:05"),
		"LabelLines":       "- device: /dev/dm-0\n- hostname: ceshi-quanxian-3-153\n- instance: 172.16.3.153\n- mountpoint: /",
		"LabelLinesCard":   "- device: /dev/dm-0\\n- hostname: ceshi-quanxian-3-153\\n- instance: 172.16.3.153\\n- mountpoint: /",
		"Labels": map[string]interface{}{
			"hostname":   "ceshi-quanxian-3-153",
			"instance":   "172.16.3.153",
			"mountpoint": "/",
			"device":     "/dev/dm-0",
		},
		"Annotations": map[string]interface{}{
			"summary":     "系统磁盘使用率超过82%",
			"description": "系统磁盘使用率超过82%",
		},
	}
	if isRecovered {
		data["Status"] = "recovered"
	}
	for key, value := range req.Event {
		data[key] = value
	}
	return data
}

func renderNamedTemplate(raw string, data map[string]interface{}) (map[string]string, error) {
	tpl, err := texttemplate.New("notify").Funcs(templateFuncMap()).Parse(raw)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, name := range []string{"Title", "TitleColor", "Event", "Footer"} {
		if tpl.Lookup(name) == nil {
			continue
		}
		var buf bytes.Buffer
		if err := tpl.ExecuteTemplate(&buf, name, data); err != nil {
			return nil, err
		}
		result[name] = strings.TrimSpace(buf.String())
	}
	return result, nil
}

func renderTemplateString(raw string, data map[string]interface{}) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	tpl, err := texttemplate.New("notify-field").Funcs(templateFuncMap()).Parse(raw)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func templateFuncMap() texttemplate.FuncMap {
	return texttemplate.FuncMap{
		"timestamp": func() string {
			return time.Now().Format("2006-01-02 15:04:05")
		},
		"default": func(defaultValue interface{}, value interface{}) interface{} {
			if strings.TrimSpace(fmt.Sprint(value)) == "" || fmt.Sprint(value) == "<nil>" {
				return defaultValue
			}
			return value
		},
		"json": func(value interface{}) string {
			bs, err := json.Marshal(value)
			if err != nil {
				return fmt.Sprint(value)
			}
			return string(bs)
		},
	}
}

func stringValue(raw model.JSONMap, key string) string {
	if raw == nil {
		return ""
	}
	value, ok := raw[key]
	if !ok {
		return ""
	}
	return fmt.Sprint(value)
}

func jsonMapValue(raw model.JSONMap, key string) model.JSONMap {
	if raw == nil {
		return nil
	}
	value, ok := raw[key]
	if !ok {
		return nil
	}
	if item, ok := value.(model.JSONMap); ok {
		return item
	}
	if item, ok := value.(map[string]interface{}); ok {
		return model.JSONMap(item)
	}
	bs, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var item model.JSONMap
	if err := json.Unmarshal(bs, &item); err != nil {
		return nil
	}
	return item
}

func buildFeishuCardFromRendered(resp model.AlertNotifyTemplatePreviewResponse) model.JSONMap {
	title := strings.TrimSpace(resp.Title)
	if title == "" {
		title = "AIOps 告警通知"
	}
	color := strings.TrimSpace(resp.TitleColor)
	if color == "" {
		color = "red"
	}
	elements := []interface{}{
		map[string]interface{}{
			"tag":     "markdown",
			"content": resp.Content,
		},
	}
	if strings.TrimSpace(resp.Footer) != "" {
		elements = append(elements,
			map[string]interface{}{"tag": "hr"},
			map[string]interface{}{
				"tag":     "markdown",
				"content": resp.Footer,
			},
		)
	}
	return model.JSONMap{
		"schema": "2.0",
		"config": map[string]interface{}{
			"width_mode":     "fill",
			"enable_forward": true,
		},
		"header": map[string]interface{}{
			"template": color,
			"title": map[string]interface{}{
				"tag":     "plain_text",
				"content": title,
			},
		},
		"body": map[string]interface{}{
			"elements": elements,
		},
	}
}
