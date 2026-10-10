package service

import (
	"testing"

	alertnotify "dodevops-api/api/alertmanager/notify"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestSupportsFeishuAIOpsChatType(t *testing.T) {
	for _, chatType := range []string{"p2p", "group", "topic_group", " P2P "} {
		if !supportsFeishuAIOpsChatType(chatType) {
			t.Fatalf("supportsFeishuAIOpsChatType(%q) = false", chatType)
		}
	}
	if supportsFeishuAIOpsChatType("private") {
		t.Fatal("supportsFeishuAIOpsChatType(private) = true")
	}
}

func TestRequiresFeishuAIOpsMention(t *testing.T) {
	requireMention := true
	cfg := alertnotify.FeishuCardAppConfig{AIOpsRequireMention: &requireMention}
	if requiresFeishuAIOpsMention(cfg, "p2p") {
		t.Fatal("p2p chat should not require a mention")
	}
	if !requiresFeishuAIOpsMention(cfg, "group") {
		t.Fatal("group chat should preserve the configured mention requirement")
	}
}

func TestParseFeishuAIOpsQuestionWithoutMentionForP2P(t *testing.T) {
	messageType := "text"
	content := `{"text":"查询当前告警"}`
	question, ok := parseFeishuAIOpsQuestion(&larkim.EventMessage{
		MessageType: &messageType,
		Content:     &content,
	}, false)
	if !ok || question != "查询当前告警" {
		t.Fatalf("parseFeishuAIOpsQuestion() = %q, %v", question, ok)
	}
}
