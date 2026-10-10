package engine

import (
	"dodevops-api/api/alertmanager/model"
	"dodevops-api/common/util"
	"testing"
	"time"
)

func TestParseNotifyGroupKeyIDs(t *testing.T) {
	notifyRuleID, ruleID, ok := parseNotifyGroupKeyIDs("notify:1|rule:145")
	if !ok {
		t.Fatal("parseNotifyGroupKeyIDs() ok = false, want true")
	}
	if notifyRuleID != 1 || ruleID != 145 {
		t.Fatalf("parseNotifyGroupKeyIDs() = (%d, %d), want (1, 145)", notifyRuleID, ruleID)
	}
}

func TestNotifyGroupStateKeysIncludesLegacyKey(t *testing.T) {
	keys := notifyGroupStateKeys("notify:1|rule:145")

	if len(keys) != 2 {
		t.Fatalf("notifyGroupStateKeys() len = %d, want 2", len(keys))
	}
	if keys[0] != notifyGroupStateKey("notify:1|rule:145") {
		t.Fatalf("primary state key = %q, want current group key", keys[0])
	}
	if keys[1] != notifyGroupStateKey("notify:1") {
		t.Fatalf("legacy state key = %q, want notify:1 key", keys[1])
	}
}

func TestNotifyGroupStateKeysKeepsServiceRoutesIndependent(t *testing.T) {
	groupKey := "notify:1|rule:145|service:bigdata"
	keys := notifyGroupStateKeys(groupKey)
	if len(keys) != 1 {
		t.Fatalf("notifyGroupStateKeys() len = %d, want 1", len(keys))
	}
	if keys[0] != notifyGroupStateKey(groupKey) {
		t.Fatalf("primary state key = %q, want routed group key", keys[0])
	}
}

func TestNotifyGroupStateKeysKeepsBusinessRouteIDsIndependent(t *testing.T) {
	groupKey := "notify:1|rule:145|service:public|route:7"
	keys := notifyGroupStateKeys(groupKey)
	if len(keys) != 1 {
		t.Fatalf("notifyGroupStateKeys() len = %d, want 1", len(keys))
	}
	if keys[0] != notifyGroupStateKey(groupKey) {
		t.Fatalf("primary state key = %q, want route-specific group key", keys[0])
	}
}

func TestReserveNotifyGroupInMemoryChecksAllStateKeys(t *testing.T) {
	repo := &repository{notifyGroupLastSent: make(map[string]time.Time)}
	now := time.Date(2026, 7, 1, 18, 44, 0, 0, time.Local)
	keys := notifyGroupStateKeys("notify:1|rule:145")
	repo.notifyGroupLastSent[keys[1]] = now.Add(-time.Minute)

	allowed, lastSentAt := repo.reserveNotifyGroupInMemory(keys, now, 600)

	if allowed {
		t.Fatal("reserveNotifyGroupInMemory() allowed = true, want false")
	}
	if !lastSentAt.Equal(now.Add(-time.Minute)) {
		t.Fatalf("lastSentAt = %v, want %v", lastSentAt, now.Add(-time.Minute))
	}
}

func TestSilenceRuleMatchesAlertmanagerMatchers(t *testing.T) {
	rule := model.AlertRule{ID: 42, Name: "HighCPU", Severity: "warning", BusinessID: "ops"}
	sample := alertSample{
		Fingerprint: "fp-1",
		Severity:    "critical",
		Labels:      model.JSONMap{"instance": "node-1", "service": "api"},
	}
	tests := []struct {
		name     string
		matchers model.JSONMap
		want     bool
	}{
		{
			name:     "event fingerprint",
			matchers: model.JSONMap{"fingerprint": "fp-1", "ruleId": 42, "eventId": 99},
			want:     true,
		},
		{
			name: "label and regex",
			matchers: model.JSONMap{"items": []interface{}{
				map[string]interface{}{"label": "service", "op": "=", "value": "api"},
				map[string]interface{}{"label": "instance", "op": "=~", "value": "node-[0-9]+"},
			}},
			want: true,
		},
		{
			name: "negative matcher fails",
			matchers: model.JSONMap{"items": []interface{}{
				map[string]interface{}{"label": "severity", "op": "!=", "value": "critical"},
			}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := model.AlertSilenceRule{Matchers: tt.matchers}
			if got := silenceRuleMatches(item, sample, rule); got != tt.want {
				t.Fatalf("silenceRuleMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdatedActiveEventPreservesSilencedState(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.Local)
	existing := model.AlertActiveEvent{ID: 9, Silenced: 0, Status: model.EventStatusFiring}
	evaluated := model.AlertActiveEvent{
		Status:   model.EventStatusSilenced,
		Silenced: 1,
		Labels:   model.JSONMap{"service": "api", "instance": "node-2"},
	}

	updated := updatedActiveEvent(existing, evaluated, false, util.HTime{Time: now})

	if updated.Silenced != 1 || updated.Status != model.EventStatusSilenced {
		t.Fatalf("updated silence state = (%d, %q), want (1, %q)", updated.Silenced, updated.Status, model.EventStatusSilenced)
	}
	if updated.Labels["instance"] != "node-2" {
		t.Fatalf("updated labels = %v, want latest evaluated labels", updated.Labels)
	}
}

func TestShouldSkipUnchangedSilencedEventDBUpdate(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 10, 0, time.Local)
	existing := model.AlertActiveEvent{
		ID:        9,
		Status:    model.EventStatusSilenced,
		Silenced:  1,
		UpdatedAt: util.HTime{Time: now.Add(-5 * time.Second)},
	}
	evaluated := existing

	if !shouldSkipActiveEventDBUpdate(existing, evaluated, false, false, now) {
		t.Fatal("unchanged silenced event should use the in-memory/Redis update path")
	}
}

func TestEventsWithoutMatchingSilencesFiltersCurrentPostgresAlerts(t *testing.T) {
	rules := []model.AlertSilenceRule{{
		Matchers: model.JSONMap{"items": []interface{}{
			map[string]interface{}{"label": "instance", "op": "=", "value": "192.168.34.146:5432"},
			map[string]interface{}{"label": "role", "op": "=", "value": "PostgreSQL"},
		}},
	}}
	events := []model.AlertActiveEvent{
		{ID: 3142, Status: model.EventStatusFiring, Labels: model.JSONMap{"instance": "192.168.34.146:5432", "role": "PostgreSQL"}},
		{ID: 3144, Status: model.EventStatusFiring, Labels: model.JSONMap{"instance": "192.168.34.147:5432", "role": "PostgreSQL"}},
	}

	filtered := eventsWithoutMatchingSilences(events, rules)

	if len(filtered) != 1 || filtered[0].ID != 3144 {
		t.Fatalf("eventsWithoutMatchingSilences() = %#v, want only event 3144", filtered)
	}
}
