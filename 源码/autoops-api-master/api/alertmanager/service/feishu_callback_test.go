package service

import (
	"net/http"
	"testing"

	"dodevops-api/api/alertmanager/model"
)

func TestMatchFeishuHTTPCallback(t *testing.T) {
	raw := []byte(`{"schema":"2.0","header":{"event_id":"evt_1","event_type":"im.message.receive_v1","app_id":"cli_test","token":"token"},"event":{"message":{"chat_id":"oc_test","chat_type":"group"}}}`)
	channels := []model.AlertNotifyChannel{{
		ID:      5,
		Type:    model.NotifyChannelFeishuCard,
		Enabled: model.RuleStatusEnabled,
		Config: model.JSONMap{
			"appId":             "cli_test",
			"appSecret":         "secret",
			"eventMode":         "http",
			"verificationToken": "token",
		},
	}}
	matched, err := matchFeishuHTTPCallback(http.Header{}, raw, channels)
	if err != nil {
		t.Fatalf("matchFeishuHTTPCallback() error = %v", err)
	}
	if matched.channel.ID != 5 || matched.envelope.Header.EventID != "evt_1" {
		t.Fatalf("matchFeishuHTTPCallback() matched channel=%d eventID=%q", matched.channel.ID, matched.envelope.Header.EventID)
	}
}

func TestMatchFeishuHTTPCallbackRejectsTokenMismatch(t *testing.T) {
	raw := []byte(`{"schema":"2.0","header":{"event_id":"evt_1","event_type":"card.action.trigger","app_id":"cli_test","token":"wrong"},"event":{}}`)
	channels := []model.AlertNotifyChannel{{
		ID:      5,
		Type:    model.NotifyChannelFeishuCard,
		Enabled: model.RuleStatusEnabled,
		Config: model.JSONMap{
			"appId":             "cli_test",
			"appSecret":         "secret",
			"eventMode":         "http",
			"verificationToken": "token",
		},
	}}
	if _, err := matchFeishuHTTPCallback(http.Header{}, raw, channels); err == nil {
		t.Fatal("matchFeishuHTTPCallback() error = nil, want token mismatch rejection")
	}
}
