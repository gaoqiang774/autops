package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dodevops-api/api/alertmanager/model"
	"dodevops-api/common/util"
)

type dispatcherTestStore struct {
	policies          []model.AlertNotifyRule
	channel           model.AlertNotifyChannel
	notifyGroup       model.AlertNotifyGroup
	notifyGroups      map[uint]model.AlertNotifyGroup
	routeRules        []model.AlertNotifyRouteRule
	template          model.AlertNotifyTemplate
	groupLastSent     map[string]time.Time
	markedGroupKeys   []string
	notifiedEventIDs  []uint64
	notifyRecords     []model.AlertNotifyRecord
	notifyRecordCount int
}

func newTestDispatcher(store *dispatcherTestStore) *Dispatcher {
	return NewDispatcherWithSender(store, func(context.Context, SendRequest) SendResult {
		return SendResult{Status: model.NotifyStatusSuccess, Target: "test"}
	})
}

func (s *dispatcherTestStore) ListEnabledNotifyRulesByIDs(ids []uint) ([]model.AlertNotifyRule, error) {
	return s.policies, nil
}

func (s *dispatcherTestStore) GetNotifyChannel(id uint) (model.AlertNotifyChannel, error) {
	return s.channel, nil
}

func (s *dispatcherTestStore) GetNotifyGroupByID(id uint) (model.AlertNotifyGroup, error) {
	if group, ok := s.notifyGroups[id]; ok {
		return group, nil
	}
	return s.notifyGroup, nil
}

func (s *dispatcherTestStore) GetNotifyGroupByBusinessID(businessID, channelType string) (model.AlertNotifyGroup, error) {
	return s.notifyGroup, nil
}

func (s *dispatcherTestStore) ListEnabledNotifyRouteRules() ([]model.AlertNotifyRouteRule, error) {
	return append([]model.AlertNotifyRouteRule(nil), s.routeRules...), nil
}

func (s *dispatcherTestStore) GetNotifyTemplate(id uint) (model.AlertNotifyTemplate, error) {
	return s.template, nil
}

func (s *dispatcherTestStore) CreateNotifyRecord(item *model.AlertNotifyRecord) error {
	s.notifyRecordCount++
	if item != nil {
		s.notifyRecords = append(s.notifyRecords, *item)
	}
	return nil
}

func (s *dispatcherTestStore) MarkEventNotified(eventID uint64, now time.Time) error {
	return nil
}

func (s *dispatcherTestStore) MarkEventsNotified(eventIDs []uint64, now time.Time) error {
	s.notifiedEventIDs = append(s.notifiedEventIDs, eventIDs...)
	return nil
}

func (s *dispatcherTestStore) ReserveNotifyGroup(groupKey string, now time.Time, intervalSeconds int64) (bool, time.Time, error) {
	if s.groupLastSent == nil {
		s.groupLastSent = make(map[string]time.Time)
	}
	lastSentAt, ok := s.groupLastSent[groupKey]
	if ok && !lastSentAt.IsZero() && now.Before(lastSentAt.Add(time.Duration(intervalSeconds)*time.Second)) {
		return false, lastSentAt, nil
	}
	s.groupLastSent[groupKey] = now
	s.markedGroupKeys = append(s.markedGroupKeys, groupKey)
	return true, lastSentAt, nil
}

func (s *dispatcherTestStore) ReleaseNotifyGroup(groupKey string) error {
	delete(s.groupLastSent, groupKey)
	return nil
}

func TestResolvePolicyChannelReceiversUsesLabelRoute(t *testing.T) {
	store := &dispatcherTestStore{
		notifyGroup: model.AlertNotifyGroup{ID: 2, Name: "大数据组", UserIDs: model.JSONStringArray{"worker-1"}, Enabled: 1},
		routeRules: []model.AlertNotifyRouteRule{{
			ID: 1, Name: "大数据服务路由",
			Conditions:    model.AlertNotifyRouteConditions{{LabelKey: "service", MatchMode: model.NotifyRouteMatchExact, MatchValue: "bigdata"}},
			NotifyGroupID: 2, Priority: 100, Enabled: 1,
		}},
	}
	resolved, err := NewDispatcher(store).resolvePolicyChannelReceivers(
		NotifyPolicyChannel{},
		DispatchRequest{Event: model.AlertActiveEvent{Labels: model.JSONMap{"service": "BIGDATA"}}},
		model.AlertNotifyChannel{Type: model.NotifyChannelLeliao},
	)
	if err != nil {
		t.Fatalf("resolvePolicyChannelReceivers() error = %v", err)
	}
	if resolved.ReceiverGroupID != 2 || len(resolved.UserIDs) != 1 || resolved.UserIDs[0] != "worker-1" {
		t.Fatalf("resolved = %#v", resolved)
	}
}

func TestResolvePolicyChannelReceiversRouteCanOverridePolicyGroup(t *testing.T) {
	store := &dispatcherTestStore{
		notifyGroup: model.AlertNotifyGroup{ID: 2, Name: "大数据组", UserIDs: model.JSONStringArray{"worker-1"}, Enabled: 1},
		routeRules: []model.AlertNotifyRouteRule{{
			ID: 1, Name: "大数据服务路由", NotifyRuleID: 7,
			Conditions:    model.AlertNotifyRouteConditions{{LabelKey: "service", MatchMode: model.NotifyRouteMatchExact, MatchValue: "bigdata"}},
			NotifyGroupID: 2, OverrideGroup: 1, Priority: 100, Enabled: 1,
		}},
	}
	resolved, err := NewDispatcher(store).resolvePolicyChannelReceiversForPolicy(
		NotifyPolicyChannel{ReceiverGroupID: 9, UserIDs: []string{"default-user"}},
		DispatchRequest{Event: model.AlertActiveEvent{Labels: model.JSONMap{"service": "bigdata"}}},
		model.AlertNotifyChannel{Type: model.NotifyChannelLeliao}, 7,
	)
	if err != nil {
		t.Fatalf("resolvePolicyChannelReceiversForPolicy() error = %v", err)
	}
	if resolved.ReceiverGroupID != 2 || len(resolved.UserIDs) != 1 || resolved.UserIDs[0] != "worker-1" {
		t.Fatalf("resolved = %#v", resolved)
	}
}

func TestNotifyGroupKeySeparatesServices(t *testing.T) {
	bigdata := model.AlertActiveEvent{RuleID: 1, Labels: model.JSONMap{"service": "bigdata"}}
	crm := model.AlertActiveEvent{RuleID: 1, Labels: model.JSONMap{"service": "crm"}}
	if notifyGroupKey(2, bigdata) == notifyGroupKey(2, crm) {
		t.Fatal("notifyGroupKey must separate events from different services")
	}
}

func TestDispatchRoutesEachEventToItsMatchedNotifyGroup(t *testing.T) {
	now := time.Date(2026, 7, 16, 16, 30, 0, 0, time.Local)
	policy := model.AlertNotifyRule{
		ID:                    5,
		Name:                  "默认通知",
		Enabled:               1,
		RepeatNotify:          1,
		RepeatMode:            "uniform",
		RepeatIntervalSeconds: 60,
		RecoveryNotify:        1,
		AggregationEnabled:    0,
		Channels: model.JSONMap{"items": []interface{}{
			map[string]interface{}{"channelId": 4, "templateId": 5, "receiverGroupId": 1},
		}},
	}
	events := []model.AlertActiveEvent{
		{
			ID: 101, RuleID: 426, RuleName: "KafkaConsumersGroupLag", Status: model.EventStatusFiring,
			FirstTriggeredAt: util.HTime{Time: now.Add(-time.Minute)},
			Labels:           model.JSONMap{"service": "public", "topic": "hrEmpLeaveOffice", "consumergroup": "jy_main"},
		},
		{
			ID: 102, RuleID: 426, RuleName: "KafkaConsumersGroupLag", Status: model.EventStatusFiring,
			FirstTriggeredAt: util.HTime{Time: now.Add(-time.Minute)},
			Labels:           model.JSONMap{"service": "public", "topic": "syncAppIndexDataMsg", "consumergroup": "college-consumer"},
		},
		{
			ID: 103, RuleID: 426, RuleName: "KafkaConsumersGroupLag", Status: model.EventStatusFiring,
			FirstTriggeredAt: util.HTime{Time: now.Add(-time.Minute)},
			Labels:           model.JSONMap{"service": "bigdata", "topic": "flume_app_event", "consumergroup": "flume_app_event_paimon_online"},
		},
	}
	store := kafkaRouteDispatcherStore(policy)
	dispatcher := NewDispatcherWithSender(store, func(context.Context, SendRequest) SendResult {
		return SendResult{Status: model.NotifyStatusSuccess, Target: "test"}
	})

	ids, err := dispatcher.Dispatch(context.Background(), DispatchRequest{
		Rule:   model.AlertRule{ID: 426, Name: "KafkaConsumersGroupLag", NotifyRuleIDs: model.JSONUintArray{policy.ID}},
		Events: events, EventType: model.EventStatusFiring, Now: now,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("notified event IDs = %v, want all 3 events", ids)
	}
	if len(store.notifyRecords) != 3 {
		t.Fatalf("notify records = %d, want 3 route groups", len(store.notifyRecords))
	}

	groupsByEvent := make(map[uint64]uint, len(store.notifyRecords))
	for _, record := range store.notifyRecords {
		groupsByEvent[record.EventID] = record.NotifyGroupID
		if record.EventCount != 1 {
			t.Fatalf("event %d record EventCount = %d, want 1", record.EventID, record.EventCount)
		}
	}
	if groupsByEvent[101] != 9 || groupsByEvent[102] != 1 || groupsByEvent[103] != 2 {
		t.Fatalf("groups by event = %v, want hr=9 default=1 bigdata=2", groupsByEvent)
	}
}

func TestNotifyRouteForDispatchRequestUsesPrecomputedEventRoute(t *testing.T) {
	routes := []model.AlertNotifyRouteRule{
		{
			ID: 7, Name: "人事业务路由", NotifyRuleID: 5, NotifyGroupID: 9, Enabled: 1,
			Conditions: model.AlertNotifyRouteConditions{{LabelKey: "topic", MatchMode: model.NotifyRouteMatchContains, MatchValue: "hrEmp"}},
		},
	}
	req := DispatchRequest{
		Event:         model.AlertActiveEvent{Labels: model.JSONMap{"service": "public", "topic": "another-topic"}},
		notifyRouteID: 7,
	}

	route, matched := notifyRouteForDispatchRequest(routes, 5, req)
	if !matched || route.ID != 7 || route.NotifyGroupID != 9 {
		t.Fatalf("route = %#v matched=%t, want precomputed route 7 to group 9", route, matched)
	}
}

func TestDispatchDifferentRoutesWithinSameServiceUseDifferentThrottleKeys(t *testing.T) {
	now := time.Date(2026, 7, 16, 16, 30, 0, 0, time.Local)
	policy := model.AlertNotifyRule{
		ID: 5, Name: "默认通知", Enabled: 1, RepeatNotify: 1,
		RepeatMode: "uniform", RepeatIntervalSeconds: 60, RecoveryNotify: 1,
		AggregationEnabled: 1, GroupWaitSeconds: 0, GroupIntervalSeconds: 600,
		Channels: model.JSONMap{"items": []interface{}{
			map[string]interface{}{"channelId": 4, "templateId": 5, "receiverGroupId": 1},
		}},
	}
	events := []model.AlertActiveEvent{
		{
			ID: 201, RuleID: 426, RuleName: "KafkaConsumersGroupLag", Status: model.EventStatusFiring,
			FirstTriggeredAt: util.HTime{Time: now.Add(-time.Minute)},
			Labels:           model.JSONMap{"service": "public", "topic": "hrEmpLeaveOffice"},
		},
		{
			ID: 202, RuleID: 426, RuleName: "KafkaConsumersGroupLag", Status: model.EventStatusFiring,
			FirstTriggeredAt: util.HTime{Time: now.Add(-time.Minute)},
			Labels:           model.JSONMap{"service": "public", "topic": "syncAppIndexDataMsg"},
		},
	}
	store := kafkaRouteDispatcherStore(policy)

	ids, err := newTestDispatcher(store).Dispatch(context.Background(), DispatchRequest{
		Rule:   model.AlertRule{ID: 426, NotifyRuleIDs: model.JSONUintArray{policy.ID}},
		Events: events, EventType: model.EventStatusFiring, Now: now,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if len(ids) != 2 || len(store.notifyRecords) != 2 {
		t.Fatalf("ids=%v records=%d, want both route groups sent", ids, len(store.notifyRecords))
	}
	if len(store.markedGroupKeys) != 2 || store.markedGroupKeys[0] == store.markedGroupKeys[1] {
		t.Fatalf("marked group keys = %v, want two distinct keys", store.markedGroupKeys)
	}
	foundRouteKey := false
	for _, key := range store.markedGroupKeys {
		if strings.Contains(key, "|route:7") {
			foundRouteKey = true
		}
	}
	if !foundRouteKey {
		t.Fatalf("marked group keys = %v, want route dimension", store.markedGroupKeys)
	}
}

func TestDispatchRecoveredEventsAreSeparatedByNotifyRoute(t *testing.T) {
	now := time.Date(2026, 7, 16, 16, 30, 0, 0, time.Local)
	policy := model.AlertNotifyRule{
		ID: 5, Name: "默认通知", Enabled: 1, RecoveryNotify: 1,
		AggregationEnabled: 1,
		Channels: model.JSONMap{"items": []interface{}{
			map[string]interface{}{"channelId": 4, "templateId": 5, "receiverGroupId": 1},
		}},
	}
	events := []model.AlertActiveEvent{
		{ID: 301, RuleID: 426, Status: model.EventStatusResolved, Labels: model.JSONMap{"service": "public", "topic": "hrEmpLeaveOffice"}},
		{ID: 302, RuleID: 426, Status: model.EventStatusResolved, Labels: model.JSONMap{"service": "public", "topic": "syncAppIndexDataMsg"}},
	}
	store := kafkaRouteDispatcherStore(policy)

	ids, err := newTestDispatcher(store).Dispatch(context.Background(), DispatchRequest{
		Rule:   model.AlertRule{ID: 426, NotifyRuleIDs: model.JSONUintArray{policy.ID}, NotifyRecovered: 1},
		Events: events, EventType: model.EventStatusResolved, Now: now,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if len(ids) != 2 || len(store.notifyRecords) != 2 {
		t.Fatalf("ids=%v records=%d, want recovered events split by route", ids, len(store.notifyRecords))
	}
	groups := map[uint]bool{}
	for _, record := range store.notifyRecords {
		groups[record.NotifyGroupID] = true
		if record.EventType != model.EventStatusResolved {
			t.Fatalf("record EventType = %q, want resolved", record.EventType)
		}
	}
	if !groups[1] || !groups[9] {
		t.Fatalf("recovery notify groups = %v, want default and HR groups", groups)
	}
}

func kafkaRouteDispatcherStore(policy model.AlertNotifyRule) *dispatcherTestStore {
	return &dispatcherTestStore{
		policies: []model.AlertNotifyRule{policy},
		channel:  model.AlertNotifyChannel{ID: 4, Name: "乐聊", Type: model.NotifyChannelLeliao},
		template: model.AlertNotifyTemplate{ID: 5, ChannelType: model.NotifyChannelLeliao, Content: model.JSONMap{
			"title": "title", "content": "content",
		}},
		notifyGroup: model.AlertNotifyGroup{ID: 1, Name: "运维组", UserIDs: model.JSONStringArray{"ops"}, Enabled: 1},
		notifyGroups: map[uint]model.AlertNotifyGroup{
			1: {ID: 1, Name: "运维组", UserIDs: model.JSONStringArray{"ops"}, Enabled: 1},
			2: {ID: 2, Name: "大数据组", UserIDs: model.JSONStringArray{"bigdata"}, Enabled: 1},
			9: {ID: 9, Name: "人事组", UserIDs: model.JSONStringArray{"hr"}, Enabled: 1},
		},
		routeRules: []model.AlertNotifyRouteRule{
			{
				ID: 7, Name: "人事业务路由", NotifyRuleID: policy.ID, NotifyGroupID: 9, OverrideGroup: 1,
				Priority: 100, Enabled: 1,
				Conditions: model.AlertNotifyRouteConditions{{LabelKey: "topic", MatchMode: model.NotifyRouteMatchContains, MatchValue: "hrEmp"}},
			},
			{
				ID: 1, Name: "大数据业务路由", NotifyRuleID: policy.ID, NotifyGroupID: 2, OverrideGroup: 1,
				Priority: 100, Enabled: 1,
				Conditions: model.AlertNotifyRouteConditions{{LabelKey: "service", MatchMode: model.NotifyRouteMatchExact, MatchValue: "bigdata"}},
			},
		},
	}
}

func TestDispatchLeliaoNotifyGroupFiltersBlockedUserIDInRecord(t *testing.T) {
	var workernos string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		workernos = r.Form.Get("workernos")
		_, _ = w.Write([]byte(`{"STATUS":"1"}`))
	}))
	defer server.Close()

	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.Local)
	policy := model.AlertNotifyRule{
		ID:                    7,
		Name:                  "leliao group",
		Enabled:               1,
		RepeatNotify:          1,
		RepeatMode:            "uniform",
		RepeatIntervalSeconds: 60,
		RecoveryNotify:        1,
		AggregationEnabled:    0,
		Channels: model.JSONMap{
			"items": []interface{}{
				map[string]interface{}{"channelId": 1, "templateId": 1, "receiverGroupId": 9},
			},
		},
	}
	store := &dispatcherTestStore{
		policies: []model.AlertNotifyRule{policy},
		channel: model.AlertNotifyChannel{
			ID:   1,
			Name: "leliao channel",
			Type: model.NotifyChannelLeliao,
			Config: model.JSONMap{
				"webhook": server.URL,
				"postKey": "post-key",
			},
		},
		notifyGroup: model.AlertNotifyGroup{
			ID:      9,
			Name:    "运维通知组",
			UserIDs: model.JSONStringArray{"000001", "u100", " u200 "},
		},
		template: model.AlertNotifyTemplate{
			ID:          1,
			ChannelType: model.NotifyChannelLeliao,
			Content: model.JSONMap{
				"title":   "title",
				"content": "content",
			},
		},
	}

	notifiedEventIDs, err := NewDispatcher(store).Dispatch(context.Background(), DispatchRequest{
		Rule: model.AlertRule{
			ID:            11,
			Name:          "cpu high",
			NotifyRuleIDs: model.JSONUintArray{policy.ID},
		},
		Event: model.AlertActiveEvent{
			ID:               101,
			RuleID:           11,
			RuleName:         "cpu high",
			Status:           model.EventStatusFiring,
			Severity:         "P1",
			BusinessID:       "business-a",
			FirstTriggeredAt: util.HTime{Time: now.Add(-10 * time.Minute)},
		},
		EventType: model.EventStatusFiring,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	if len(notifiedEventIDs) != 1 || notifiedEventIDs[0] != 101 {
		t.Fatalf("notifiedEventIDs = %+v, want [101]", notifiedEventIDs)
	}
	if workernos != "u100,u200" {
		t.Fatalf("workernos = %q, want %q", workernos, "u100,u200")
	}
	if len(store.notifyRecords) != 1 {
		t.Fatalf("notifyRecords length = %d, want 1", len(store.notifyRecords))
	}
	var request struct {
		ReceiverGroupID   uint     `json:"receiverGroupId"`
		ReceiverGroupName string   `json:"receiverGroupName"`
		UserIDs           []string `json:"userIds"`
	}
	if err := json.Unmarshal([]byte(store.notifyRecords[0].Request), &request); err != nil {
		t.Fatalf("request json unmarshal error = %v", err)
	}
	if request.ReceiverGroupID != 9 {
		t.Fatalf("receiverGroupId = %d, want 9", request.ReceiverGroupID)
	}
	if request.ReceiverGroupName != "运维通知组" {
		t.Fatalf("receiverGroupName = %q, want %q", request.ReceiverGroupName, "运维通知组")
	}
	if record := store.notifyRecords[0]; record.NotifyGroupID != 9 || record.NotifyGroupName != "运维通知组" {
		t.Fatalf("notify record group snapshot = (%d, %q), want (9, %q)", record.NotifyGroupID, record.NotifyGroupName, "运维通知组")
	}
	if len(request.UserIDs) != 2 || request.UserIDs[0] != "u100" || request.UserIDs[1] != "u200" {
		t.Fatalf("request userIds = %+v, want [u100 u200]", request.UserIDs)
	}
}

func TestDispatchAggregatedEventsThrottleByNotifyRuleAndAlertRule(t *testing.T) {
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.Local)
	firstTriggeredAt := util.HTime{Time: now.Add(-10 * time.Minute)}
	policy := model.AlertNotifyRule{
		ID:                    7,
		Name:                  "group throttle",
		Enabled:               1,
		RepeatNotify:          1,
		RepeatMode:            "uniform",
		RepeatIntervalSeconds: 60,
		RecoveryNotify:        1,
		AggregationEnabled:    1,
		GroupWaitSeconds:      0,
		GroupIntervalSeconds:  600,
		Channels: model.JSONMap{
			"items": []interface{}{
				map[string]interface{}{"channelId": 1, "templateId": 1},
			},
		},
	}
	events := []model.AlertActiveEvent{
		{
			ID:               101,
			RuleID:           11,
			RuleName:         "cpu high",
			Status:           model.EventStatusFiring,
			Severity:         "P1",
			BusinessID:       "business-a",
			FirstTriggeredAt: firstTriggeredAt,
		},
		{
			ID:               102,
			RuleID:           11,
			RuleName:         "cpu high",
			Status:           model.EventStatusFiring,
			Severity:         "P2",
			BusinessID:       "business-b",
			FirstTriggeredAt: firstTriggeredAt,
		},
	}
	throttledGroupKey := notifyGroupKey(policy.ID, events[0])
	store := &dispatcherTestStore{
		policies: []model.AlertNotifyRule{policy},
		channel: model.AlertNotifyChannel{
			ID:   1,
			Name: "fake channel",
			Type: model.NotifyChannelFeishu,
		},
		template: model.AlertNotifyTemplate{
			ID:          1,
			ChannelType: model.NotifyChannelFeishu,
			Content: model.JSONMap{
				"title":   "title",
				"content": "content",
			},
		},
		groupLastSent: map[string]time.Time{
			throttledGroupKey: now.Add(-5 * time.Minute),
		},
	}

	notifiedEventIDs, err := newTestDispatcher(store).Dispatch(context.Background(), DispatchRequest{
		Rule: model.AlertRule{
			ID:            11,
			Name:          "cpu high",
			NotifyRuleIDs: model.JSONUintArray{policy.ID},
		},
		Events:    events,
		EventType: model.EventStatusFiring,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	if len(notifiedEventIDs) != 1 || notifiedEventIDs[0] != 102 {
		t.Fatalf("notifiedEventIDs = %+v, want [102]", notifiedEventIDs)
	}
	if len(store.notifiedEventIDs) != 1 || store.notifiedEventIDs[0] != 102 {
		t.Fatalf("store.notifiedEventIDs = %+v, want [102]", store.notifiedEventIDs)
	}
	if len(store.markedGroupKeys) != 1 || store.markedGroupKeys[0] != notifyGroupKey(policy.ID, events[1]) {
		t.Fatalf("markedGroupKeys = %+v, want second business group", store.markedGroupKeys)
	}
	if store.notifyRecordCount != 1 {
		t.Fatalf("notifyRecordCount = %d, want 1", store.notifyRecordCount)
	}
}

func TestDispatchAggregatedEventsDoesNotThrottleDifferentAlertRules(t *testing.T) {
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.Local)
	firstTriggeredAt := util.HTime{Time: now.Add(-10 * time.Minute)}
	policy := model.AlertNotifyRule{
		ID:                    7,
		Name:                  "group throttle",
		Enabled:               1,
		RepeatNotify:          1,
		RepeatMode:            "uniform",
		RepeatIntervalSeconds: 60,
		RecoveryNotify:        1,
		AggregationEnabled:    1,
		GroupWaitSeconds:      0,
		GroupIntervalSeconds:  600,
		Channels: model.JSONMap{
			"items": []interface{}{
				map[string]interface{}{"channelId": 1, "templateId": 1},
			},
		},
	}
	events := []model.AlertActiveEvent{
		{
			ID:               101,
			RuleID:           11,
			RuleName:         "cpu high",
			Status:           model.EventStatusFiring,
			Severity:         "P1",
			FirstTriggeredAt: firstTriggeredAt,
		},
		{
			ID:               102,
			RuleID:           12,
			RuleName:         "memory high",
			Status:           model.EventStatusFiring,
			Severity:         "P1",
			FirstTriggeredAt: firstTriggeredAt,
		},
	}
	throttledGroupKey := notifyGroupKey(policy.ID, events[0])
	store := &dispatcherTestStore{
		policies: []model.AlertNotifyRule{policy},
		channel: model.AlertNotifyChannel{
			ID:   1,
			Name: "fake channel",
			Type: model.NotifyChannelFeishu,
		},
		template: model.AlertNotifyTemplate{
			ID:          1,
			ChannelType: model.NotifyChannelFeishu,
			Content: model.JSONMap{
				"title":   "title",
				"content": "content",
			},
		},
		groupLastSent: map[string]time.Time{
			throttledGroupKey: now.Add(-5 * time.Minute),
		},
	}

	notifiedEventIDs, err := newTestDispatcher(store).Dispatch(context.Background(), DispatchRequest{
		Rule: model.AlertRule{
			ID:            11,
			Name:          "cpu high",
			NotifyRuleIDs: model.JSONUintArray{policy.ID},
		},
		Events:    events,
		EventType: model.EventStatusFiring,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	if len(notifiedEventIDs) != 1 || notifiedEventIDs[0] != 102 {
		t.Fatalf("notifiedEventIDs = %+v, want [102]", notifiedEventIDs)
	}
	if len(store.markedGroupKeys) != 1 || store.markedGroupKeys[0] != notifyGroupKey(policy.ID, events[1]) {
		t.Fatalf("markedGroupKeys = %+v, want second rule group", store.markedGroupKeys)
	}
	if store.notifyRecordCount != 1 {
		t.Fatalf("notifyRecordCount = %d, want 1", store.notifyRecordCount)
	}
}

func TestDispatchMultipleResourcesSendOneBatch(t *testing.T) {
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.Local)
	policy := model.AlertNotifyRule{
		ID:                    7,
		Name:                  "single batch",
		Enabled:               1,
		RepeatNotify:          1,
		RepeatMode:            "uniform",
		RepeatIntervalSeconds: 60,
		RecoveryNotify:        1,
		AggregationEnabled:    0,
		Channels: model.JSONMap{
			"items": []interface{}{
				map[string]interface{}{"channelId": 1, "templateId": 1},
			},
		},
	}
	events := []model.AlertActiveEvent{
		{
			ID:               101,
			RuleID:           11,
			RuleName:         "cpu high",
			Status:           model.EventStatusFiring,
			Severity:         "P1",
			FirstTriggeredAt: util.HTime{Time: now.Add(-10 * time.Minute)},
		},
		{
			ID:               102,
			RuleID:           11,
			RuleName:         "cpu high",
			Status:           model.EventStatusFiring,
			Severity:         "P1",
			FirstTriggeredAt: util.HTime{Time: now.Add(-10 * time.Minute)},
		},
	}
	store := &dispatcherTestStore{
		policies: []model.AlertNotifyRule{policy},
		channel: model.AlertNotifyChannel{
			ID:   1,
			Name: "fake channel",
			Type: model.NotifyChannelFeishu,
		},
		template: model.AlertNotifyTemplate{
			ID:          1,
			ChannelType: model.NotifyChannelFeishu,
			Content: model.JSONMap{
				"title":   "title",
				"content": "content",
			},
		},
	}

	notifiedEventIDs, err := newTestDispatcher(store).Dispatch(context.Background(), DispatchRequest{
		Rule: model.AlertRule{
			ID:            11,
			Name:          "cpu high",
			NotifyRuleIDs: model.JSONUintArray{policy.ID},
		},
		Events:    events,
		EventType: model.EventStatusFiring,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	if len(notifiedEventIDs) != 2 || notifiedEventIDs[0] != 101 || notifiedEventIDs[1] != 102 {
		t.Fatalf("notifiedEventIDs = %+v, want [101 102]", notifiedEventIDs)
	}
	if store.notifyRecordCount != 1 {
		t.Fatalf("notifyRecordCount = %d, want 1", store.notifyRecordCount)
	}
}

func TestDispatchGroupIntervalSuppressesMinuteRepeat(t *testing.T) {
	start := time.Date(2026, 7, 1, 10, 0, 0, 0, time.Local)
	policy := model.AlertNotifyRule{
		ID:                    7,
		Name:                  "group throttle",
		Enabled:               1,
		RepeatNotify:          1,
		RepeatMode:            "uniform",
		RepeatIntervalSeconds: 60,
		RecoveryNotify:        1,
		AggregationEnabled:    1,
		GroupWaitSeconds:      0,
		GroupIntervalSeconds:  600,
		Channels: model.JSONMap{
			"items": []interface{}{
				map[string]interface{}{"channelId": 1, "templateId": 1},
			},
		},
	}
	store := &dispatcherTestStore{
		policies: []model.AlertNotifyRule{policy},
		channel: model.AlertNotifyChannel{
			ID:   1,
			Name: "fake channel",
			Type: model.NotifyChannelFeishu,
		},
		template: model.AlertNotifyTemplate{
			ID:          1,
			ChannelType: model.NotifyChannelFeishu,
			Content: model.JSONMap{
				"title":   "title",
				"content": "content",
			},
		},
	}
	dispatcher := newTestDispatcher(store)
	rule := model.AlertRule{
		ID:            11,
		Name:          "cpu high",
		NotifyRuleIDs: model.JSONUintArray{policy.ID},
	}
	event := model.AlertActiveEvent{
		ID:               101,
		RuleID:           rule.ID,
		RuleName:         rule.Name,
		Status:           model.EventStatusFiring,
		Severity:         "P1",
		FirstTriggeredAt: util.HTime{Time: start.Add(-10 * time.Minute)},
	}

	firstIDs, err := dispatcher.Dispatch(context.Background(), DispatchRequest{
		Rule:      rule,
		Event:     event,
		EventType: model.EventStatusFiring,
		Now:       start,
	})
	if err != nil {
		t.Fatalf("first Dispatch() error = %v", err)
	}
	if len(firstIDs) != 1 || firstIDs[0] != event.ID {
		t.Fatalf("firstIDs = %+v, want [%d]", firstIDs, event.ID)
	}

	event.LastNotifiedAt = util.HTime{Time: start}
	minuteIDs, err := dispatcher.Dispatch(context.Background(), DispatchRequest{
		Rule:      rule,
		Event:     event,
		EventType: model.EventStatusFiring,
		Now:       start.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("minute Dispatch() error = %v", err)
	}
	if len(minuteIDs) != 0 {
		t.Fatalf("minuteIDs = %+v, want empty", minuteIDs)
	}

	event.LastNotifiedAt = util.HTime{Time: start}
	tenMinuteIDs, err := dispatcher.Dispatch(context.Background(), DispatchRequest{
		Rule:      rule,
		Event:     event,
		EventType: model.EventStatusFiring,
		Now:       start.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("ten minute Dispatch() error = %v", err)
	}
	if len(tenMinuteIDs) != 1 || tenMinuteIDs[0] != event.ID {
		t.Fatalf("tenMinuteIDs = %+v, want [%d]", tenMinuteIDs, event.ID)
	}

	if len(store.markedGroupKeys) != 2 {
		t.Fatalf("markedGroupKeys = %+v, want 2 sends", store.markedGroupKeys)
	}
	if store.notifyRecordCount != 2 {
		t.Fatalf("notifyRecordCount = %d, want 2", store.notifyRecordCount)
	}
}

func TestDispatchFailedSendDoesNotMarkEventOrKeepReservation(t *testing.T) {
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.Local)
	policy := model.AlertNotifyRule{
		ID: 7, Name: "failed send", Enabled: 1, RepeatNotify: 1,
		RepeatMode: "uniform", RepeatIntervalSeconds: 60, RecoveryNotify: 1,
		AggregationEnabled: 1, GroupWaitSeconds: 0, GroupIntervalSeconds: 600,
		Channels: model.JSONMap{"items": []interface{}{map[string]interface{}{"channelId": 1, "templateId": 1}}},
	}
	store := &dispatcherTestStore{
		policies: []model.AlertNotifyRule{policy},
		channel:  model.AlertNotifyChannel{ID: 1, Name: "fake channel", Type: model.NotifyChannelFeishu},
		template: model.AlertNotifyTemplate{ID: 1, ChannelType: model.NotifyChannelFeishu, Content: model.JSONMap{"title": "title", "content": "content"}},
	}
	dispatcher := NewDispatcherWithSender(store, func(context.Context, SendRequest) SendResult {
		return SendResult{Status: model.NotifyStatusFailed, ErrorMessage: "test failure"}
	})
	event := model.AlertActiveEvent{ID: 101, RuleID: 11, RuleName: "cpu high", Status: model.EventStatusFiring, Severity: "P1", FirstTriggeredAt: util.HTime{Time: now.Add(-time.Minute)}}
	ids, err := dispatcher.Dispatch(context.Background(), DispatchRequest{
		Rule: model.AlertRule{ID: 11, NotifyRuleIDs: model.JSONUintArray{policy.ID}}, Event: event,
		EventType: model.EventStatusFiring, Now: now,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if len(ids) != 0 || len(store.notifiedEventIDs) != 0 {
		t.Fatalf("failed send marked event: ids=%v notified=%v", ids, store.notifiedEventIDs)
	}
	if _, ok := store.groupLastSent[notifyGroupKey(policy.ID, event)]; ok {
		t.Fatal("failed send kept aggregation reservation")
	}
}

func TestBuildDispatchTemplateEventUsesValueSummaryWithoutQueryTail(t *testing.T) {
	now := time.Date(2026, 7, 8, 11, 36, 4, 0, time.Local)
	event := model.AlertActiveEvent{
		ID:               165202,
		RuleID:           301,
		RuleName:         "nginx日志4xx 告警",
		Status:           model.EventStatusFiring,
		Severity:         "info",
		Value:            "11",
		TriggerQuery:     "log_type:nginx status:>=400 status:<500",
		TriggerCondition: "log_type:nginx status:>=400 status:<500 [window=5m] >= 10",
		FirstTriggeredAt: util.HTime{Time: now.Add(-10 * time.Minute)},
		Labels: model.JSONMap{
			"__name__": "victorialogs_hits",
			"window":   "5m",
		},
	}

	data := buildDispatchTemplateEvent(NotifyPolicy{Name: "log policy"}, DispatchRequest{
		Rule: model.AlertRule{
			ID:   301,
			Name: "nginx日志4xx 告警",
		},
		Event:     event,
		Events:    []model.AlertActiveEvent{event},
		EventType: model.EventStatusFiring,
		Now:       now,
	}, now)

	if data["Threshold"] != "10" {
		t.Fatalf("Threshold = %q, want 10", data["Threshold"])
	}
	if data["ValueSummary"] != "11，阈值 >= 10" {
		t.Fatalf("ValueSummary = %q, want current value with threshold", data["ValueSummary"])
	}
}
