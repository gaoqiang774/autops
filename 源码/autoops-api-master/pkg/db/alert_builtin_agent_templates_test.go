package db

import (
	"strings"
	"testing"

	"dodevops-api/common/util"
)

func TestDefaultAgentAlertBuiltinRuleTemplates(t *testing.T) {
	templates := defaultAgentAlertBuiltinRuleTemplates(util.HTime{})
	if len(templates) != 28 {
		t.Fatalf("template count = %d, want 28", len(templates))
	}

	wantCategories := map[string]int{
		"cpu": 3, "memory": 4, "load": 6, "disk": 3,
		"disk_io": 4, "network": 2, "connection": 2, "service": 4,
	}
	gotCategories := make(map[string]int)
	seenUUIDs := make(map[string]struct{}, len(templates))
	for _, item := range templates {
		if item.ResourceType != "host" || item.Component != "Linux" || item.Collector != "agent" {
			t.Fatalf("unexpected template path: %+v", item)
		}
		if item.Source != "platform" || !strings.HasPrefix(item.SourceUUID, "platform.agent.") {
			t.Fatalf("template is not a platform Agent template: %+v", item)
		}
		if _, exists := seenUUIDs[item.SourceUUID]; exists {
			t.Fatalf("duplicate source UUID: %s", item.SourceUUID)
		}
		seenUUIDs[item.SourceUUID] = struct{}{}

		metric, _ := item.Tags["metric"].(string)
		if metric == "" || !strings.Contains(item.QueryTemplate, metric) {
			t.Fatalf("query %q does not contain tagged metric %q", item.QueryTemplate, metric)
		}
		if item.Variables == nil || item.Variables["threshold"] == nil {
			t.Fatalf("template %s does not expose a threshold variable", item.SourceUUID)
		}
		gotCategories[item.Category]++
	}
	for category, want := range wantCategories {
		if gotCategories[category] != want {
			t.Fatalf("category %s count = %d, want %d", category, gotCategories[category], want)
		}
	}
}

func TestAgentBuiltinTemplateGroupDisplayNames(t *testing.T) {
	if got := alertBuiltinCollectorDisplayName("agent"); got != "Agent 监控模板" {
		t.Fatalf("collector display name = %q", got)
	}
	if got := alertBuiltinCategoryDisplayName("agent", "disk_io"); got != "磁盘 IO" {
		t.Fatalf("category display name = %q", got)
	}
	if got := alertBuiltinCategoryDisplayName("node_exporter", "disk"); got != "disk" {
		t.Fatalf("non-Agent category display name changed to %q", got)
	}
}
