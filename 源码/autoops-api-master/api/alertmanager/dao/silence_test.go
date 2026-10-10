package dao

import (
	"dodevops-api/api/alertmanager/model"
	"testing"
)

func TestEventSilenceRuleIDs(t *testing.T) {
	event := model.AlertActiveEvent{ID: 9, Fingerprint: "fp-1"}
	rules := []model.AlertSilenceRule{
		{ID: 1, Name: "标签静默-HighCPU", Matchers: model.JSONMap{"eventId": 9, "items": []interface{}{}}},
		{ID: 2, Name: "事件静默-fp-1", Matchers: model.JSONMap{"fingerprint": "fp-1"}},
		{ID: 3, Name: "共享静默", Matchers: model.JSONMap{"items": []interface{}{}}},
	}

	ids := eventSilenceRuleIDs(rules, event)
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("eventSilenceRuleIDs() = %v, want [1 2]", ids)
	}
}

func TestMatchingSilenceRulesIncludesLabelRules(t *testing.T) {
	event := model.AlertActiveEvent{
		Fingerprint: "fp-1",
		RuleID:      42,
		RuleName:    "HighCPU",
		Severity:    "critical",
		Labels:      model.JSONMap{"service": "api"},
	}
	rules := []model.AlertSilenceRule{
		{ID: 1, Matchers: model.JSONMap{"fingerprint": "fp-1", "ruleId": 42}},
		{ID: 2, Matchers: model.JSONMap{"items": []interface{}{
			map[string]interface{}{"label": "service", "op": "=", "value": "api"},
		}}},
		{ID: 3, Matchers: model.JSONMap{"items": []interface{}{
			map[string]interface{}{"label": "service", "op": "=", "value": "web"},
		}}},
	}

	matched := matchingSilenceRules(rules, event)
	if len(matched) != 2 || matched[0].ID != 1 || matched[1].ID != 2 {
		t.Fatalf("matchingSilenceRules() ids = %v, want [1 2]", silenceRuleIDs(matched))
	}
}

func TestLabelSilenceRuleMatchesEveryEligibleEvent(t *testing.T) {
	rule := model.AlertSilenceRule{
		ID: 7,
		Matchers: model.JSONMap{"items": []interface{}{
			map[string]interface{}{"label": "service", "op": "=", "value": "api"},
			map[string]interface{}{"label": "instance", "op": "=~", "value": "node-[0-9]+"},
		}},
	}
	events := []model.AlertActiveEvent{
		{ID: 1, Labels: model.JSONMap{"service": "api", "instance": "node-1"}},
		{ID: 2, Labels: model.JSONMap{"service": "api", "instance": "node-2"}},
		{ID: 3, Labels: model.JSONMap{"service": "web", "instance": "node-3"}},
	}

	matchedIDs := make([]uint64, 0)
	for _, event := range events {
		if len(matchingSilenceRules([]model.AlertSilenceRule{rule}, event)) > 0 {
			matchedIDs = append(matchedIDs, event.ID)
		}
	}

	if len(matchedIDs) != 2 || matchedIDs[0] != 1 || matchedIDs[1] != 2 {
		t.Fatalf("matched event ids = %v, want [1 2]", matchedIDs)
	}
}
