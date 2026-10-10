package notify

import (
	"testing"
	"time"

	"dodevops-api/api/alertmanager/model"
)

func TestParseNotifyPolicyParsesMatchersAndChannels(t *testing.T) {
	enabled := true
	policy, err := ParseNotifyPolicy(model.AlertNotifyRule{
		ID:      7,
		Name:    "P0 prod feishu",
		Enabled: 1,
		Matchers: model.JSONMap{
			"severities":      []interface{}{"P0", "P1"},
			"datasourceTypes": []interface{}{"prometheus"},
			"labels": []interface{}{
				map[string]interface{}{"key": "env", "op": "=", "value": "prod"},
			},
		},
		Channels: model.JSONMap{
			"items": []interface{}{
				map[string]interface{}{"channelId": 1, "templateId": 2, "receivers": []interface{}{"oc_xxx"}, "enabled": enabled},
			},
		},
	})
	if err != nil {
		t.Fatalf("ParseNotifyPolicy() error = %v", err)
	}

	if policy.ID != 7 || policy.Matchers.Severities[0] != "P0" {
		t.Fatalf("unexpected policy: %+v", policy)
	}
	if len(policy.Channels) != 1 || policy.Channels[0].ChannelID != 1 || policy.Channels[0].TemplateID != 2 {
		t.Fatalf("unexpected channels: %+v", policy.Channels)
	}
	if policy.Channels[0].Enabled == nil || !*policy.Channels[0].Enabled {
		t.Fatalf("expected enabled channel: %+v", policy.Channels[0])
	}
}

func TestParseNotifyPolicyParsesReceiverGroupID(t *testing.T) {
	policy, err := ParseNotifyPolicy(model.AlertNotifyRule{
		Name: "business leliao",
		Channels: model.JSONMap{
			"items": []interface{}{
				map[string]interface{}{"channelId": 1, "templateId": 2, "receiverGroupId": 9},
			},
		},
	})
	if err != nil {
		t.Fatalf("ParseNotifyPolicy() error = %v", err)
	}
	if len(policy.Channels) != 1 || policy.Channels[0].ReceiverGroupID != 9 {
		t.Fatalf("unexpected receiver group: %+v", policy.Channels)
	}
}

func TestMatchNotifyPolicyMatchesLabelsAndTimeRange(t *testing.T) {
	policy := NotifyPolicy{
		Matchers: NotifyMatchers{
			Severities:      []string{"P0"},
			DatasourceTypes: []string{"prometheus"},
			Labels: []TagMatcher{
				{Key: "env", Op: "=", Value: "prod"},
				{Key: "service", Op: "=~", Value: "mysql|redis"},
			},
			TimeRanges: []TimeRangeRule{
				{Weekdays: []int{1}, Start: "09:00", End: "18:00"},
			},
		},
	}

	result := MatchNotifyPolicy(policy, NotifyEventContext{
		Event: model.AlertActiveEvent{
			Severity:       "P0",
			DatasourceType: "prometheus",
			Labels:         model.JSONMap{"env": "prod", "service": "mysql"},
		},
		Now: time.Date(2026, 6, 22, 10, 30, 0, 0, time.Local), // Monday
	})

	if !result.Matched {
		t.Fatalf("expected matched, reasons=%+v", result.Reasons)
	}
}

func TestMatchNotifyPolicyMatchesPLevelFromEventLabelsAndSeverity(t *testing.T) {
	policy := NotifyPolicy{
		Matchers: NotifyMatchers{
			Severities: []string{"P0"},
		},
	}

	result := MatchNotifyPolicy(policy, NotifyEventContext{
		Event: model.AlertActiveEvent{
			Severity: "critical",
			Labels:   model.JSONMap{"level": "P0"},
		},
		Now: time.Date(2026, 6, 22, 10, 30, 0, 0, time.Local),
	})

	if !result.Matched {
		t.Fatalf("expected P0 policy to match critical event with level label, reasons=%+v", result.Reasons)
	}
}

func TestMatchNotifyPolicyRejectsLabelMismatch(t *testing.T) {
	policy := NotifyPolicy{
		Matchers: NotifyMatchers{
			Labels: []TagMatcher{{Key: "env", Op: "=", Value: "prod"}},
		},
	}

	result := MatchNotifyPolicy(policy, NotifyEventContext{
		Event: model.AlertActiveEvent{Labels: model.JSONMap{"env": "test"}},
		Now:   time.Date(2026, 6, 22, 10, 30, 0, 0, time.Local),
	})

	if result.Matched {
		t.Fatalf("expected not matched, reasons=%+v", result.Reasons)
	}
}

func TestMatchNotifyPolicySupportsOvernightTimeRange(t *testing.T) {
	policy := NotifyPolicy{
		Matchers: NotifyMatchers{
			TimeRanges: []TimeRangeRule{
				{Weekdays: []int{1}, Start: "21:00", End: "09:00"},
			},
		},
	}

	result := MatchNotifyPolicy(policy, NotifyEventContext{
		Event: model.AlertActiveEvent{},
		Now:   time.Date(2026, 6, 22, 23, 30, 0, 0, time.Local), // Monday
	})

	if !result.Matched {
		t.Fatalf("expected overnight range to match, reasons=%+v", result.Reasons)
	}
}

func TestValidateNotifyPolicyRejectsBadRegex(t *testing.T) {
	err := ValidateNotifyPolicy(NotifyPolicy{
		Name: "bad regex",
		Matchers: NotifyMatchers{
			Labels: []TagMatcher{{Key: "service", Op: "=~", Value: "["}},
		},
	})
	if err == nil {
		t.Fatal("expected invalid regex error")
	}
}

func TestParseNotifyPolicyParsesNoiseReductionFields(t *testing.T) {
	policy, err := ParseNotifyPolicy(model.AlertNotifyRule{
		Name:                     "noise policy",
		RepeatNotify:             1,
		RepeatMode:               "bySeverity",
		RepeatIntervalSeconds:    3600,
		RepeatIntervals:          model.JSONMap{"P0": 3600, "P1": 7200, "P2": 14400},
		MaxRepeatTimes:           5,
		RecoveryNotify:           1,
		AggregationEnabled:       1,
		GroupWaitSeconds:         30,
		GroupIntervalSeconds:     300,
		AggregationWindowSeconds: 30,
	})
	if err != nil {
		t.Fatalf("ParseNotifyPolicy() error = %v", err)
	}
	if policy.RepeatMode != "bySeverity" || policy.RepeatIntervals["P2"] != 14400 {
		t.Fatalf("unexpected repeat settings: %+v", policy)
	}
	if policy.MaxRepeatTimes != 5 {
		t.Fatalf("unexpected max repeat times: %+v", policy)
	}
	if policy.GroupWaitSeconds != 30 || policy.GroupIntervalSeconds != 300 {
		t.Fatalf("unexpected aggregation settings: %+v", policy)
	}
	if err := ValidateNotifyPolicy(policy); err != nil {
		t.Fatalf("ValidateNotifyPolicy() error = %v", err)
	}
}

func TestValidateNotifyPolicyAcceptsSeverityRepeatModeAlias(t *testing.T) {
	policy, err := ParseNotifyPolicy(model.AlertNotifyRule{
		Name:       "severity alias",
		RepeatMode: "severity",
		Channels: model.JSONMap{
			"items": []interface{}{
				map[string]interface{}{"channelId": 1, "templateId": 1},
			},
		},
	})
	if err != nil {
		t.Fatalf("ParseNotifyPolicy() error = %v", err)
	}
	if policy.RepeatMode != "bySeverity" {
		t.Fatalf("RepeatMode = %q, want bySeverity", policy.RepeatMode)
	}
	if err := ValidateNotifyPolicy(policy); err != nil {
		t.Fatalf("ValidateNotifyPolicy() error = %v", err)
	}
}

func TestEnabledPolicyChannelsFiltersDisabledItems(t *testing.T) {
	disabled := false
	enabled := true
	channels := EnabledPolicyChannels(NotifyPolicy{
		Channels: []NotifyPolicyChannel{
			{ChannelID: 1, TemplateID: 1, Enabled: &disabled},
			{ChannelID: 2, TemplateID: 2, Enabled: &enabled},
			{ChannelID: 3, TemplateID: 3},
		},
	})

	if len(channels) != 2 || channels[0].ChannelID != 2 || channels[1].ChannelID != 3 {
		t.Fatalf("unexpected enabled channels: %+v", channels)
	}
}
