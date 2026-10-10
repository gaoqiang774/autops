package notify

import (
	"testing"

	"dodevops-api/api/alertmanager/model"
)

func TestBuildFeishuCardTemplatePayloadButtonStyle(t *testing.T) {
	var tpl model.AlertNotifyTemplate
	for _, item := range model.DefaultAlertNotifyTemplates() {
		if item.Ident == "builtin_feishu_card" {
			tpl = item
			break
		}
	}
	if tpl.Ident == "" {
		t.Fatal("builtin_feishu_card template not found")
	}

	_, _, card, err := BuildChannelTemplatePayload(model.NotifyChannelFeishuCard, tpl, model.AlertNotifyTemplatePreviewRequest{})
	if err != nil {
		t.Fatalf("BuildChannelTemplatePayload() error = %v", err)
	}

	body := mapValue(t, card, "body")
	elements := sliceValue(t, body, "elements")
	columnSet := mapValueFromSlice(t, elements, 2)
	if columnSet["horizontal_spacing"] != "16px" {
		t.Fatalf("button horizontal spacing = %v", columnSet["horizontal_spacing"])
	}

	columns := sliceValue(t, columnSet, "columns")
	claimColumn := mapValueFromSlice(t, columns, 0)
	claimButton := mapValueFromSlice(t, sliceValue(t, claimColumn, "elements"), 0)
	if claimButton["type"] != "primary_filled" {
		t.Fatalf("claim button type = %v", claimButton["type"])
	}

	silenceColumn := mapValueFromSlice(t, columns, 1)
	silenceButton := mapValueFromSlice(t, sliceValue(t, silenceColumn, "elements"), 0)
	if silenceButton["type"] != "danger_filled" {
		t.Fatalf("silence button type = %v", silenceButton["type"])
	}
}

func mapValue(t *testing.T, raw map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	value, ok := raw[key].(map[string]interface{})
	if !ok {
		t.Fatalf("%s is %T, want map[string]interface{}", key, raw[key])
	}
	return value
}

func sliceValue(t *testing.T, raw map[string]interface{}, key string) []interface{} {
	t.Helper()
	value, ok := raw[key].([]interface{})
	if !ok {
		t.Fatalf("%s is %T, want []interface{}", key, raw[key])
	}
	return value
}

func mapValueFromSlice(t *testing.T, raw []interface{}, index int) map[string]interface{} {
	t.Helper()
	if index >= len(raw) {
		t.Fatalf("index %d out of range, len=%d", index, len(raw))
	}
	value, ok := raw[index].(map[string]interface{})
	if !ok {
		t.Fatalf("item %d is %T, want map[string]interface{}", index, raw[index])
	}
	return value
}
