package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"dodevops-api/api/alertmanager/model"
)

func TestSendLeliaoFiltersBlockedUserID(t *testing.T) {
	var workernos string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		workernos = r.Form.Get("workernos")
		_, _ = w.Write([]byte(`{"STATUS":"1"}`))
	}))
	defer server.Close()

	result := Send(context.Background(), SendRequest{
		Channel: model.AlertNotifyChannel{
			Type: model.NotifyChannelLeliao,
			Config: model.JSONMap{
				"webhook": server.URL,
				"postKey": "post-key",
			},
		},
		Title:   "title",
		Content: "content",
		UserIDs: []string{"u001", "000001", " u002 ", "000001"},
	})

	if result.Status != model.NotifyStatusSuccess {
		t.Fatalf("Send() status = %s, error = %s", result.Status, result.ErrorMessage)
	}
	if workernos != "u001,u002" {
		t.Fatalf("workernos = %q, want %q", workernos, "u001,u002")
	}
	if result.Target != "leliao://u001,u002" {
		t.Fatalf("target = %q, want %q", result.Target, "leliao://u001,u002")
	}
}

func TestSendLeliaoBlockedOnlyUserIDFailsWithoutPosting(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"STATUS":"1"}`))
	}))
	defer server.Close()

	result := Send(context.Background(), SendRequest{
		Channel: model.AlertNotifyChannel{
			Type: model.NotifyChannelLeliao,
			Config: model.JSONMap{
				"webhook": server.URL,
				"postKey": "post-key",
			},
		},
		Title:   "title",
		Content: "content",
		UserIDs: []string{"000001"},
	})

	if result.Status != model.NotifyStatusFailed {
		t.Fatalf("Send() status = %s, want failed", result.Status)
	}
	if called {
		t.Fatal("server was called, want blocked receiver to stop send before posting")
	}
}

func TestValidateFeishuCardAllowsAIOpsWithoutChatWhitelist(t *testing.T) {
	channel := model.AlertNotifyChannel{
		Type: model.NotifyChannelFeishuCard,
		Config: model.JSONMap{
			"appId":        "cli_test",
			"appSecret":    "secret",
			"aiopsEnabled": true,
		},
	}

	if err := ValidateChannel(channel); err != nil {
		t.Fatalf("ValidateChannel() error = %v", err)
	}

	cfg, err := ParseFeishuCardAppConfig(channel.Config)
	if err != nil {
		t.Fatalf("ParseFeishuCardAppConfig() error = %v", err)
	}
	if !cfg.AllowsAIOpsChat("oc_any_group") {
		t.Fatal("AllowsAIOpsChat() = false, want true when whitelist is empty")
	}
	if cfg.AllowsAIOpsChat("  ") {
		t.Fatal("AllowsAIOpsChat() = true for an empty chat ID")
	}
}

func TestFeishuCardAIOpsIgnoresLegacyChatWhitelist(t *testing.T) {
	cfg, err := ParseFeishuCardAppConfig(model.JSONMap{
		"appId":               "cli_test",
		"appSecret":           "secret",
		"aiopsEnabled":        true,
		"aiopsAllowedChatIds": []string{" oc_allowed ", "oc_allowed"},
	})
	if err != nil {
		t.Fatalf("ParseFeishuCardAppConfig() error = %v", err)
	}
	if !cfg.AllowsAIOpsChat("oc_allowed") {
		t.Fatal("AllowsAIOpsChat() = false for a valid group")
	}
	if !cfg.AllowsAIOpsChat("oc_other") {
		t.Fatal("AllowsAIOpsChat() = false because of a legacy whitelist")
	}
}

func TestValidateFeishuCardHTTPModeRequiresVerificationToken(t *testing.T) {
	channel := model.AlertNotifyChannel{
		Type: model.NotifyChannelFeishuCard,
		Config: model.JSONMap{
			"appId":     "cli_test",
			"appSecret": "secret",
			"eventMode": "http",
		},
	}
	if err := ValidateChannel(channel); err == nil {
		t.Fatal("ValidateChannel() error = nil, want verificationToken requirement")
	}
	channel.Config["verificationToken"] = "token"
	if err := ValidateChannel(channel); err != nil {
		t.Fatalf("ValidateChannel() error = %v", err)
	}
	cfg, err := ParseFeishuCardAppConfig(channel.Config)
	if err != nil {
		t.Fatalf("ParseFeishuCardAppConfig() error = %v", err)
	}
	if !cfg.UsesHTTPCallbacks() || cfg.UsesWebSocket() {
		t.Fatal("HTTP event mode was not preserved")
	}
}

func TestFeishuCardInfersHTTPModeFromVerificationToken(t *testing.T) {
	channel := model.AlertNotifyChannel{
		Type: model.NotifyChannelFeishuCard,
		Config: model.JSONMap{
			"appId":             "cli_test",
			"appSecret":         "secret",
			"verificationToken": "token",
			"eventMode":         "websocket",
		},
	}
	cfg, err := ParseFeishuCardAppConfig(channel.Config)
	if err != nil {
		t.Fatalf("ParseFeishuCardAppConfig() error = %v", err)
	}
	if !cfg.UsesHTTPCallbacks() {
		t.Fatal("UsesHTTPCallbacks() = false when verificationToken is configured")
	}
	EnsureFeishuEventMode(&channel)
	if channel.Config["eventMode"] != FeishuEventModeHTTP {
		t.Fatalf("eventMode = %v, want http", channel.Config["eventMode"])
	}
}

func TestPreserveFeishuServerConfig(t *testing.T) {
	channel := model.AlertNotifyChannel{
		Type: model.NotifyChannelFeishuCard,
		Config: model.JSONMap{
			"appId":        "cli_test",
			"appSecret":    "new_secret",
			"aiopsEnabled": true,
		},
	}
	PreserveFeishuServerConfig(&channel, model.JSONMap{
		"appId":             "cli_test",
		"verificationToken": "token",
		"encryptKey":        "encrypt",
		"eventMode":         "http",
		"lastChatId":        "oc_test",
	})
	for key, expected := range map[string]interface{}{
		"verificationToken": "token",
		"encryptKey":        "encrypt",
		"eventMode":         "http",
		"lastChatId":        "oc_test",
	} {
		if channel.Config[key] != expected {
			t.Fatalf("config[%q] = %v, want %v", key, channel.Config[key], expected)
		}
	}
}
