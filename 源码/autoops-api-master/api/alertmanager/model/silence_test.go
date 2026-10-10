package model

import (
	"dodevops-api/common/util"
	"testing"
	"time"
)

func TestSilenceRuleMatches(t *testing.T) {
	context := SilenceMatchContext{
		Fingerprint:  "fp-1",
		RuleID:       42,
		RuleName:     "HighCPU",
		DatasourceID: 7,
		Severity:     "critical",
		BusinessID:   "ops",
		Labels:       JSONMap{"instance": "node-1", "service": "api"},
	}
	tests := []struct {
		name     string
		matchers JSONMap
		want     bool
	}{
		{name: "direct event matcher", matchers: JSONMap{"fingerprint": "fp-1", "ruleId": 42, "eventId": 99}, want: true},
		{name: "multiple label matchers", matchers: JSONMap{"items": []interface{}{
			map[string]interface{}{"label": "service", "op": "=", "value": "api"},
			map[string]interface{}{"label": "instance", "op": "=~", "value": "node-[0-9]+"},
		}}, want: true},
		{name: "regex is anchored", matchers: JSONMap{"items": []interface{}{
			map[string]interface{}{"label": "service", "op": "=~", "value": "pi"},
		}}, want: false},
		{name: "negative matcher", matchers: JSONMap{"items": []interface{}{
			map[string]interface{}{"label": "severity", "op": "!=", "value": "critical"},
		}}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SilenceRuleMatches(AlertSilenceRule{Matchers: tt.matchers}, context); got != tt.want {
				t.Fatalf("SilenceRuleMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSilenceRuleMatchesPostgresInstanceAndRole(t *testing.T) {
	rule := AlertSilenceRule{Matchers: JSONMap{"items": []interface{}{
		map[string]interface{}{"label": "instance", "op": "=", "value": "192.168.34.146:5432"},
		map[string]interface{}{"label": "role", "op": "=", "value": "PostgreSQL"},
	}}}
	context := SilenceMatchContext{Labels: JSONMap{
		"instance": "192.168.34.146:5432",
		"role":     "PostgreSQL",
	}}

	if !SilenceRuleMatches(rule, context) {
		t.Fatal("PostgreSQL instance and role labels should match the silence rule")
	}
}

func TestEventStatusAfterSilence(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.Local)
	rule := AlertRule{ForDurationSeconds: 180, EvalIntervalSeconds: 60}

	tests := []struct {
		name  string
		event AlertActiveEvent
		want  string
	}{
		{name: "suppression is preserved", event: AlertActiveEvent{Suppressed: 1}, want: EventStatusSuppressed},
		{name: "hold duration pending", event: AlertActiveEvent{FirstTriggeredAt: util.HTime{Time: now.Add(-time.Minute)}}, want: EventStatusPending},
		{name: "hold duration elapsed", event: AlertActiveEvent{FirstTriggeredAt: util.HTime{Time: now.Add(-3 * time.Minute)}}, want: EventStatusFiring},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EventStatusAfterSilence(tt.event, rule, now); got != tt.want {
				t.Fatalf("EventStatusAfterSilence() = %q, want %q", got, tt.want)
			}
		})
	}
}
