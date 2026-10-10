package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"dodevops-api/api/alertmanager/model"
	"dodevops-api/common/util"
	"dodevops-api/pkg/log"
)

type DispatcherStore interface {
	ListEnabledNotifyRulesByIDs(ids []uint) ([]model.AlertNotifyRule, error)
	GetNotifyChannel(id uint) (model.AlertNotifyChannel, error)
	GetNotifyGroupByID(id uint) (model.AlertNotifyGroup, error)
	GetNotifyGroupByBusinessID(businessID, channelType string) (model.AlertNotifyGroup, error)
	ListEnabledNotifyRouteRules() ([]model.AlertNotifyRouteRule, error)
	GetNotifyTemplate(id uint) (model.AlertNotifyTemplate, error)
	CreateNotifyRecord(item *model.AlertNotifyRecord) error
	MarkEventNotified(eventID uint64, now time.Time) error
	MarkEventsNotified(eventIDs []uint64, now time.Time) error
	ReserveNotifyGroup(groupKey string, now time.Time, intervalSeconds int64) (bool, time.Time, error)
	ReleaseNotifyGroup(groupKey string) error
}

type DispatchRequest struct {
	Rule          model.AlertRule
	Event         model.AlertActiveEvent
	Events        []model.AlertActiveEvent
	EventType     string
	RuleGroupID   uint
	notifyRouteID uint
	Now           time.Time
}

type dispatchRouteGroup struct {
	notifyRouteID uint
	Events        []model.AlertActiveEvent
}

type Dispatcher struct {
	store DispatcherStore
	send  func(context.Context, SendRequest) SendResult
}

func NewDispatcher(store DispatcherStore) *Dispatcher {
	return NewDispatcherWithSender(store, Send)
}

func NewDispatcherWithSender(store DispatcherStore, sender func(context.Context, SendRequest) SendResult) *Dispatcher {
	if sender == nil {
		sender = Send
	}
	return &Dispatcher{store: store, send: sender}
}

func (d *Dispatcher) Dispatch(ctx context.Context, req DispatchRequest) ([]uint64, error) {
	if d == nil || d.store == nil || len(req.Rule.NotifyRuleIDs) == 0 {
		return nil, nil
	}
	req = normalizeDispatchRequest(req)
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	policies, err := d.store.ListEnabledNotifyRulesByIDs([]uint(req.Rule.NotifyRuleIDs))
	if err != nil {
		return nil, err
	}
	notifiedEventIDs := make([]uint64, 0)
	for _, rawPolicy := range policies {
		policy, err := ParseNotifyPolicy(rawPolicy)
		if err != nil {
			_ = d.recordPolicyFailure(rawPolicy.ID, req, "", "parse notify policy failed: "+err.Error(), now)
			continue
		}
		notifiedEventIDs = append(notifiedEventIDs, d.dispatchPolicy(ctx, policy, req, now)...)
	}
	if len(notifiedEventIDs) > 0 && req.EventType != model.EventStatusResolved {
		notifiedEventIDs = uniqueEventIDs(notifiedEventIDs)
		return notifiedEventIDs, d.store.MarkEventsNotified(notifiedEventIDs, now)
	}
	return uniqueEventIDs(notifiedEventIDs), nil
}

func (d *Dispatcher) dispatchPolicy(ctx context.Context, policy NotifyPolicy, req DispatchRequest, now time.Time) []uint64 {
	eligibleEvents := filterDispatchEvents(policy, req, now)
	if len(eligibleEvents) == 0 {
		return nil
	}
	notifiedEventIDs := make([]uint64, 0)
	for _, routeGroup := range d.groupDispatchEventsByNotifyRoute(policy, eligibleEvents) {
		routeReq := req
		routeReq.Events = routeGroup.Events
		routeReq.notifyRouteID = routeGroup.notifyRouteID
		if req.EventType != model.EventStatusResolved && policy.AggregationEnabled == 1 {
			for _, events := range groupDispatchEventsByNotifyGroup(policy, routeGroup.Events) {
				groupReq := routeReq
				groupReq.Events = events
				groupReq = normalizeDispatchRequest(groupReq)
				notifiedEventIDs = append(notifiedEventIDs, d.dispatchPolicyOnce(ctx, policy, groupReq, now)...)
			}
			continue
		}
		routeReq = normalizeDispatchRequest(routeReq)
		notifiedEventIDs = append(notifiedEventIDs, d.dispatchPolicyOnce(ctx, policy, routeReq, now)...)
	}
	return notifiedEventIDs
}

func (d *Dispatcher) dispatchPolicyOnce(ctx context.Context, policy NotifyPolicy, req DispatchRequest, now time.Time) []uint64 {
	if !shouldDispatchPolicy(policy, req, now) {
		return nil
	}
	match := MatchNotifyPolicy(policy, NotifyEventContext{
		Event:       req.Event,
		Rule:        req.Rule,
		RuleGroupID: req.RuleGroupID,
		Now:         now,
	})
	if !match.Matched {
		return nil
	}
	channels := EnabledPolicyChannels(policy)
	if len(channels) == 0 {
		return nil
	}
	groupKey := notifyGroupKeyForRequest(policy.ID, req)
	if req.EventType != model.EventStatusResolved && policy.AggregationEnabled == 1 && !d.reserveDispatchGroup(policy, groupKey, now) {
		return nil
	}
	sent := false
	for _, policyChannel := range channels {
		if d.dispatchChannel(ctx, policy, policyChannel, req, now) {
			sent = true
		}
	}
	if !sent {
		if req.EventType != model.EventStatusResolved && policy.AggregationEnabled == 1 {
			_ = d.store.ReleaseNotifyGroup(groupKey)
		}
		return nil
	}
	return dispatchEventIDs(req.Events)
}

func (d *Dispatcher) reserveDispatchGroup(policy NotifyPolicy, groupKey string, now time.Time) bool {
	if policy.GroupIntervalSeconds <= 0 || strings.TrimSpace(groupKey) == "" {
		return true
	}
	allowed, _, err := d.store.ReserveNotifyGroup(groupKey, now, policy.GroupIntervalSeconds)
	if err != nil {
		return true
	}
	return allowed
}

func filterDispatchEvents(policy NotifyPolicy, req DispatchRequest, now time.Time) []model.AlertActiveEvent {
	events := make([]model.AlertActiveEvent, 0, len(req.Events))
	for _, event := range req.Events {
		singleReq := req
		singleReq.Event = event
		singleReq.Events = []model.AlertActiveEvent{event}
		if shouldDispatchPolicy(policy, singleReq, now) {
			events = append(events, event)
		}
	}
	return events
}

func dispatchIntervalSeconds(policy NotifyPolicy, rule model.AlertRule, event model.AlertActiveEvent, eventType string) int64 {
	interval := repeatIntervalSeconds(policy, rule, event)
	if eventType != model.EventStatusResolved && policy.AggregationEnabled == 1 && policy.GroupIntervalSeconds > interval {
		return policy.GroupIntervalSeconds
	}
	return interval
}

func shouldRepeatDispatch(policy NotifyPolicy, rule model.AlertRule, event model.AlertActiveEvent, eventType string, now time.Time) bool {
	if !repeatEnabled(policy, rule) {
		return false
	}
	interval := dispatchIntervalSeconds(policy, rule, event, eventType)
	if interval <= 0 {
		return false
	}
	return !now.Before(event.LastNotifiedAt.Time.Add(time.Duration(interval) * time.Second))
}

func shouldInitialDispatch(policy NotifyPolicy, event model.AlertActiveEvent, now time.Time) bool {
	waitSeconds := policy.GroupWaitSeconds
	if policy.AggregationEnabled != 1 {
		waitSeconds = 0
	}
	if waitSeconds <= 0 || event.FirstTriggeredAt.IsZero() {
		return true
	}
	return !now.Before(event.FirstTriggeredAt.Time.Add(time.Duration(waitSeconds) * time.Second))
}

func shouldDispatchPolicy(policy NotifyPolicy, req DispatchRequest, now time.Time) bool {
	if req.Event.Status != model.EventStatusFiring && req.EventType != model.EventStatusResolved {
		return false
	}
	if req.EventType == model.EventStatusResolved {
		return req.Rule.NotifyRecovered == 1 && policy.RecoveryNotify == 1
	}
	if notifyMaxReached(policy, req.Rule, req.Event) {
		return false
	}
	if req.Event.LastNotifiedAt.IsZero() {
		return shouldInitialDispatch(policy, req.Event, now)
	}
	return shouldRepeatDispatch(policy, req.Rule, req.Event, req.EventType, now)
}

func groupDispatchEventsByNotifyGroup(policy NotifyPolicy, events []model.AlertActiveEvent) [][]model.AlertActiveEvent {
	groupIndex := make(map[string]int, len(events))
	groups := make([][]model.AlertActiveEvent, 0, len(events))
	for _, event := range events {
		groupKey := notifyGroupKey(policy.ID, event)
		if index, ok := groupIndex[groupKey]; ok {
			groups[index] = append(groups[index], event)
			continue
		}
		groupIndex[groupKey] = len(groups)
		groups = append(groups, []model.AlertActiveEvent{event})
	}
	return groups
}

func (d *Dispatcher) groupDispatchEventsByNotifyRoute(policy NotifyPolicy, events []model.AlertActiveEvent) []dispatchRouteGroup {
	if len(events) == 0 {
		return nil
	}
	if !d.policyUsesNotifyRoutes(policy) {
		return []dispatchRouteGroup{{Events: events}}
	}
	routes, err := d.store.ListEnabledNotifyRouteRules()
	if err != nil || len(routes) == 0 {
		return []dispatchRouteGroup{{Events: events}}
	}

	groupIndex := make(map[string]int, len(events))
	groups := make([]dispatchRouteGroup, 0, len(events))
	for _, event := range events {
		route, matched := MatchNotifyRouteRulesForPolicy(routes, policy.ID, event.Labels)
		routeID := uint(0)
		key := "default"
		if matched {
			routeID = route.ID
			key = fmt.Sprintf("route:%d", route.ID)
		} else if businessID := strings.TrimSpace(event.BusinessID); businessID != "" {
			key = "business:" + businessID
		}
		if index, ok := groupIndex[key]; ok {
			groups[index].Events = append(groups[index].Events, event)
			continue
		}
		groupIndex[key] = len(groups)
		groups = append(groups, dispatchRouteGroup{
			notifyRouteID: routeID,
			Events:        []model.AlertActiveEvent{event},
		})
	}
	return groups
}

func (d *Dispatcher) policyUsesNotifyRoutes(policy NotifyPolicy) bool {
	for _, policyChannel := range EnabledPolicyChannels(policy) {
		channel, err := d.store.GetNotifyChannel(policyChannel.ChannelID)
		if err == nil && strings.TrimSpace(channel.Type) == model.NotifyChannelLeliao {
			return true
		}
	}
	return false
}

func (d *Dispatcher) dispatchChannel(ctx context.Context, policy NotifyPolicy, policyChannel NotifyPolicyChannel, req DispatchRequest, now time.Time) bool {
	channel, err := d.store.GetNotifyChannel(policyChannel.ChannelID)
	if err != nil {
		_ = d.recordSendResult(policy, policyChannel, req, model.AlertNotifyChannel{ID: policyChannel.ChannelID}, SendResult{
			Status:       model.NotifyStatusFailed,
			ErrorMessage: "notify channel unavailable: " + err.Error(),
		}, "", "", nil, now)
		return false
	}
	tpl, err := d.store.GetNotifyTemplate(policyChannel.TemplateID)
	if err != nil {
		_ = d.recordSendResult(policy, policyChannel, req, channel, SendResult{
			Status:       model.NotifyStatusFailed,
			ErrorMessage: "notify template unavailable: " + err.Error(),
		}, "", "", nil, now)
		return false
	}
	if strings.TrimSpace(tpl.ChannelType) != "" && strings.TrimSpace(channel.Type) != strings.TrimSpace(tpl.ChannelType) {
		_ = d.recordSendResult(policy, policyChannel, req, channel, SendResult{
			Status:       model.NotifyStatusFailed,
			ErrorMessage: fmt.Sprintf("notify template channelType %s does not match channel type %s", tpl.ChannelType, channel.Type),
		}, "", "", nil, now)
		return false
	}
	resolvedChannel, err := d.resolvePolicyChannelReceiversForPolicy(policyChannel, req, channel, policy.ID)
	if err != nil {
		_ = d.recordSendResult(policy, policyChannel, req, channel, SendResult{
			Status:       model.NotifyStatusFailed,
			ErrorMessage: err.Error(),
		}, "", "", nil, now)
		return false
	}
	resolvedChannel = sanitizePolicyChannelReceivers(resolvedChannel, channel)

	templateReq := model.AlertNotifyTemplatePreviewRequest{
		IsRecovered: req.EventType == model.EventStatusResolved,
		Event:       buildDispatchTemplateEvent(policy, req, now),
	}
	title, content, card, err := BuildChannelTemplatePayload(channel.Type, tpl, templateReq)
	if err != nil {
		_ = d.recordSendResult(policy, policyChannel, req, channel, SendResult{
			Status:       model.NotifyStatusFailed,
			ErrorMessage: "render notify template failed: " + err.Error(),
		}, "", "", nil, now)
		return false
	}
	result := d.send(ctx, SendRequest{
		Channel:   channel,
		Title:     title,
		Content:   content,
		Card:      card,
		Receivers: resolvedChannel.Receivers,
		UserIDs:   resolvedChannel.UserIDs,
	})
	_ = d.recordSendResult(policy, resolvedChannel, req, channel, result, title, content, card, now)
	return result.Status == model.NotifyStatusSuccess
}

func (d *Dispatcher) resolvePolicyChannelReceivers(policyChannel NotifyPolicyChannel, req DispatchRequest, channel model.AlertNotifyChannel) (NotifyPolicyChannel, error) {
	return d.resolvePolicyChannelReceiversForPolicy(policyChannel, req, channel, 0)
}

func (d *Dispatcher) resolvePolicyChannelReceiversForPolicy(policyChannel NotifyPolicyChannel, req DispatchRequest, channel model.AlertNotifyChannel, policyID uint) (NotifyPolicyChannel, error) {
	resolved := policyChannel
	channelType := strings.TrimSpace(channel.Type)
	if channelType != model.NotifyChannelLeliao {
		return resolved, nil
	}

	businessID := strings.TrimSpace(req.Event.BusinessID)
	var businessErr error
	var businessGroup model.AlertNotifyGroup
	if resolved.ReceiverGroupID == 0 && len(resolved.UserIDs) == 0 && len(resolved.Receivers) == 0 && businessID != "" {
		group, err := d.store.GetNotifyGroupByBusinessID(businessID, channelType)
		if err == nil {
			businessGroup = group
		}
		businessErr = err
	}

	routes, err := d.store.ListEnabledNotifyRouteRules()
	if err != nil {
		return resolved, fmt.Errorf("list notify route rules failed: %w", err)
	}
	route, routeMatched := notifyRouteForDispatchRequest(routes, policyID, req)
	log.Log().Infof(
		"notify route decision: event=%d policy=%d service=%q routes=%d matched=%t route=%d targetGroup=%d override=%d defaultGroup=%d",
		req.Event.ID, policyID, jsonMapString(req.Event.Labels, "service"), len(routes), routeMatched,
		route.ID, route.NotifyGroupID, route.OverrideGroup, resolved.ReceiverGroupID,
	)
	if routeMatched && (resolved.ReceiverGroupID == 0 || route.OverrideGroup == 1) {
		group, err := d.store.GetNotifyGroupByID(route.NotifyGroupID)
		if err != nil {
			return resolved, fmt.Errorf("notify route %s group unavailable: %w", route.Name, err)
		}
		if route.OverrideGroup == 1 {
			resolved.Receivers = nil
			resolved.UserIDs = nil
		}
		return applyNotifyGroupReceivers(resolved, group), nil
	}
	if resolved.ReceiverGroupID > 0 {
		group, err := d.store.GetNotifyGroupByID(resolved.ReceiverGroupID)
		if err != nil {
			return resolved, fmt.Errorf("notify group unavailable: %w", err)
		}
		return applyNotifyGroupReceivers(resolved, group), nil
	}
	if len(resolved.UserIDs) > 0 || len(resolved.Receivers) > 0 {
		return resolved, nil
	}
	if businessGroup.ID > 0 {
		return applyNotifyGroupReceivers(resolved, businessGroup), nil
	}
	if businessErr != nil {
		return resolved, fmt.Errorf("notify group not found for businessId %s: %w", businessID, businessErr)
	}
	return resolved, nil
}

func notifyRouteForDispatchRequest(rules []model.AlertNotifyRouteRule, policyID uint, req DispatchRequest) (model.AlertNotifyRouteRule, bool) {
	if req.notifyRouteID == 0 {
		return MatchNotifyRouteRulesForPolicy(rules, policyID, req.Event.Labels)
	}
	for _, route := range rules {
		if route.ID != req.notifyRouteID || route.Enabled != model.RuleStatusEnabled || route.NotifyGroupID == 0 {
			continue
		}
		if route.NotifyRuleID != 0 && route.NotifyRuleID != policyID {
			continue
		}
		return route, true
	}
	return model.AlertNotifyRouteRule{}, false
}

func applyNotifyGroupReceivers(channel NotifyPolicyChannel, group model.AlertNotifyGroup) NotifyPolicyChannel {
	channel.ReceiverGroupID = group.ID
	channel.ReceiverGroupName = group.Name
	channel.UserIDs = mergeReceivers(channel.UserIDs, []string(group.UserIDs))
	return channel
}

func sanitizePolicyChannelReceivers(policyChannel NotifyPolicyChannel, channel model.AlertNotifyChannel) NotifyPolicyChannel {
	if strings.TrimSpace(channel.Type) != model.NotifyChannelLeliao {
		return policyChannel
	}
	policyChannel.Receivers = filterLeliaoReceivers(policyChannel.Receivers)
	policyChannel.UserIDs = filterLeliaoReceivers(policyChannel.UserIDs)
	return policyChannel
}

func notifyMaxReached(policy NotifyPolicy, rule model.AlertRule, event model.AlertActiveEvent) bool {
	limit := policy.MaxRepeatTimes
	if limit <= 0 {
		limit = rule.NotifyMaxNumber
	}
	return limit > 0 && event.NotifyCount >= limit
}

func repeatEnabled(policy NotifyPolicy, rule model.AlertRule) bool {
	if policy.RepeatNotify == 1 {
		return true
	}
	return policyRepeatUnset(policy) && rule.NotifyRepeatStepMinutes > 0
}

func policyRepeatUnset(policy NotifyPolicy) bool {
	return policy.RepeatNotify == 0 &&
		policy.RepeatIntervalSeconds == 0 &&
		len(policy.RepeatIntervals) == 0
}

func normalizeDispatchRequest(req DispatchRequest) DispatchRequest {
	if len(req.Events) == 0 && req.Event.ID > 0 {
		req.Events = []model.AlertActiveEvent{req.Event}
	}
	if len(req.Events) > 0 {
		req.Event = primaryDispatchEvent(req.Events)
	}
	return req
}

func primaryDispatchEvent(events []model.AlertActiveEvent) model.AlertActiveEvent {
	if len(events) == 0 {
		return model.AlertActiveEvent{}
	}
	primary := events[0]
	for _, event := range events[1:] {
		if primary.LastNotifiedAt.IsZero() && !event.LastNotifiedAt.IsZero() {
			continue
		}
		if !primary.LastNotifiedAt.IsZero() && event.LastNotifiedAt.IsZero() {
			primary = event
			continue
		}
		if !event.LastNotifiedAt.IsZero() && (primary.LastNotifiedAt.IsZero() || event.LastNotifiedAt.Time.Before(primary.LastNotifiedAt.Time)) {
			primary = event
			continue
		}
		if event.FirstTriggeredAt.Time.Before(primary.FirstTriggeredAt.Time) {
			primary = event
		}
	}
	return primary
}

func dispatchEventIDs(events []model.AlertActiveEvent) []uint64 {
	ids := make([]uint64, 0, len(events))
	seen := map[uint64]bool{}
	for _, event := range events {
		if event.ID == 0 || seen[event.ID] {
			continue
		}
		seen[event.ID] = true
		ids = append(ids, event.ID)
	}
	return ids
}

func uniqueEventIDs(ids []uint64) []uint64 {
	result := make([]uint64, 0, len(ids))
	seen := map[uint64]bool{}
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

func repeatIntervalSeconds(policy NotifyPolicy, rule model.AlertRule, event model.AlertActiveEvent) int64 {
	if normalizeRepeatMode(policy.RepeatMode) == "bySeverity" {
		level := eventLevel(event)
		if seconds := policy.RepeatIntervals[level]; seconds > 0 {
			return seconds
		}
	}
	if policy.RepeatIntervalSeconds > 0 {
		return policy.RepeatIntervalSeconds
	}
	if policyRepeatUnset(policy) && rule.NotifyRepeatStepMinutes > 0 {
		return rule.NotifyRepeatStepMinutes * 60
	}
	return 3600
}

func buildDispatchTemplateEvent(policy NotifyPolicy, req DispatchRequest, now time.Time) model.JSONMap {
	event := req.Event
	recovered := req.EventType == model.EventStatusResolved
	recoveredAt := event.RecoveredAt.Time
	if recoveredAt.IsZero() && recovered {
		recoveredAt = now
	}
	resources := dispatchResources(req.Events)
	metricName := jsonMapString(event.Labels, "__name__")
	description := annotationSummary(event.Annotations)
	labelLines := formatLabelLines(event.Labels, "- ")
	threshold := triggerThreshold(event.TriggerCondition, event.TriggerQuery)
	operator := triggerOperator(event.TriggerCondition, event.TriggerQuery)
	condition := triggerConditionText(event, metricName, threshold)
	eventIDs := dispatchEventIDs(req.Events)
	return model.JSONMap{
		"PlatformName":      "AIOps 运维平台",
		"RuleName":          firstNonEmpty(req.Rule.Name, event.RuleName),
		"RuleNote":          req.Rule.Note,
		"PolicyName":        firstNonEmpty(policy.Name, req.Rule.Name, event.RuleName),
		"Fingerprint":       event.Fingerprint,
		"EventID":           fmt.Sprint(event.ID),
		"EventIDs":          joinUint64s(eventIDs),
		"Severity":          eventLevel(event),
		"SeverityIcon":      severityIcon(eventLevel(event)),
		"Status":            dispatchStatus(req),
		"IsRecovered":       recovered,
		"ResourceName":      eventResourceName(event),
		"Instance":          eventInstance(event),
		"DatasourceName":    event.DatasourceType,
		"Condition":         condition,
		"ConditionCard":     jsonStringContent(condition),
		"MetricName":        metricName,
		"Operator":          operator,
		"Threshold":         threshold,
		"Value":             event.Value,
		"ValueSummary":      triggerValueSummary(event.Value, operator, threshold),
		"FirstTriggeredAt":  formatDispatchTime(event.FirstTriggeredAt.Time),
		"TriggeredAt":       formatDispatchTime(event.FirstTriggeredAt.Time),
		"RecoveredAt":       formatDispatchTime(recoveredAt),
		"Duration":          formatDuration(event.FirstTriggeredAt.Time, dispatchEndTime(event, recoveredAt, now)),
		"EventCount":        len(req.Events),
		"ResourceCount":     len(resources),
		"AffectedResources": strings.Join(resources, ", "),
		"DutyUser":          "未分配",
		"DetailURL":         fmt.Sprintf("/alertmanager/events/%d", event.ID),
		"SilenceURL":        fmt.Sprintf("/alertmanager/silence-rules?eventId=%d", event.ID),
		"SendTime":          formatDispatchTime(now),
		"Description":       description,
		"LabelLines":        labelLines,
		"LabelLinesCard":    jsonStringContent(labelLines),
		"Labels":            event.Labels,
		"Annotations":       event.Annotations,
	}
}

func joinUint64s(values []uint64) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		parts = append(parts, fmt.Sprint(value))
	}
	return strings.Join(parts, ",")
}

func severityIcon(level string) string {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "P0":
		return "🔥🔥🔥"
	case "P1":
		return "🔥🔥"
	case "P2":
		return "🔥"
	default:
		return ""
	}
}

func annotationSummary(annotations model.JSONMap) string {
	for _, key := range []string{"summary", "description", "message"} {
		value := jsonMapString(annotations, key)
		if value != "" {
			return value
		}
	}
	return ""
}

func formatLabelLines(labels model.JSONMap, prefix string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key, value := range labels {
		if strings.TrimSpace(key) == "" || value == nil {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(fmt.Sprint(labels[key]))
		if value == "" || value == "<nil>" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s%s: %s", prefix, key, value))
	}
	return strings.Join(lines, "\n")
}

func jsonStringContent(value string) string {
	bs, err := json.Marshal(value)
	if err != nil || len(bs) < 2 {
		return value
	}
	return string(bs[1 : len(bs)-1])
}

func triggerOperator(values ...string) string {
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text == "" {
			continue
		}
		for _, op := range []string{">=", "<=", "==", "!=", ">", "<", "="} {
			if strings.Contains(text, " "+op+" ") {
				return op
			}
		}
	}
	return ""
}

func triggerConditionText(event model.AlertActiveEvent, metricName, threshold string) string {
	for _, value := range []string{event.TriggerCondition, event.TriggerQuery} {
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	operator := triggerOperator(event.TriggerCondition, event.TriggerQuery)
	if metricName != "" && operator != "" && threshold != "" {
		return strings.Join([]string{metricName, operator, threshold}, " ")
	}
	return ""
}

func triggerThreshold(values ...string) string {
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text == "" {
			continue
		}
		for _, op := range []string{">=", "<=", "==", "!=", ">", "<", "="} {
			token := " " + op + " "
			index := strings.LastIndex(text, token)
			if index < 0 {
				continue
			}
			return strings.TrimSpace(text[index+len(token):])
		}
	}
	return ""
}

func triggerValueSummary(value, operator, threshold string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	operator = strings.TrimSpace(operator)
	threshold = strings.TrimSpace(threshold)
	if operator != "" && threshold != "" {
		return fmt.Sprintf("%s，阈值 %s %s", value, operator, threshold)
	}
	return value
}

func dispatchStatus(req DispatchRequest) string {
	if req.EventType == model.EventStatusResolved {
		return model.EventStatusResolved
	}
	return req.Event.Status
}

func dispatchEndTime(event model.AlertActiveEvent, recoveredAt, now time.Time) time.Time {
	if !recoveredAt.IsZero() {
		return recoveredAt
	}
	if !event.LastEvaluatedAt.IsZero() {
		return event.LastEvaluatedAt.Time
	}
	return now
}

func eventLevel(event model.AlertActiveEvent) string {
	if level := jsonMapString(event.Labels, "level"); level != "" {
		return strings.ToUpper(level)
	}
	if level := severityToLevel(event.Severity); level != "" {
		return level
	}
	return event.Severity
}

func eventResourceName(event model.AlertActiveEvent) string {
	for _, key := range []string{"hostname", "instance", "pod", "node", "container", "job", "name"} {
		if value := jsonMapString(event.Labels, key); value != "" {
			return value
		}
	}
	if event.AssetID > 0 {
		return fmt.Sprintf("asset:%d", event.AssetID)
	}
	return event.Fingerprint
}

func eventInstance(event model.AlertActiveEvent) string {
	if instance := jsonMapString(event.Labels, "instance"); instance != "" {
		return instance
	}
	return eventResourceName(event)
}

func dispatchResources(events []model.AlertActiveEvent) []string {
	resources := make([]string, 0, len(events))
	seen := map[string]bool{}
	for _, event := range events {
		resource := eventResourceName(event)
		if resource == "" || seen[resource] {
			continue
		}
		seen[resource] = true
		resources = append(resources, resource)
	}
	return resources
}

func formatDispatchTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func formatDuration(start, end time.Time) string {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return "0秒"
	}
	seconds := int64(end.Sub(start).Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%d秒", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%d分钟", minutes)
	}
	hours := minutes / 60
	if hours < 24 {
		return fmt.Sprintf("%d小时%d分钟", hours, minutes%60)
	}
	days := hours / 24
	return fmt.Sprintf("%d天%d小时", days, hours%24)
}

func (d *Dispatcher) recordPolicyFailure(notifyRuleID uint, req DispatchRequest, target, message string, now time.Time) error {
	return d.store.CreateNotifyRecord(&model.AlertNotifyRecord{
		EventID:        req.Event.ID,
		EventType:      notifyRecordEventType(req.EventType),
		NotifyGroupKey: notifyGroupKeyForRequest(notifyRuleID, req),
		NotifyRuleID:   notifyRuleID,
		RuleID:         req.Event.RuleID,
		RuleName:       req.Event.RuleName,
		DatasourceID:   req.Event.DatasourceID,
		DatasourceType: req.Event.DatasourceType,
		Severity:       eventLevel(req.Event),
		Receiver:       target,
		Status:         model.NotifyStatusFailed,
		ErrorMessage:   message,
		EventCount:     len(req.Events),
		ResourceCount:  len(dispatchResources(req.Events)),
		RuleNames:      model.JSONMap{fmt.Sprint(req.Event.RuleID): req.Event.RuleName},
		BusinessID:     req.Event.BusinessID,
		Operator:       "alert-engine",
		CreatedAt:      util.HTime{Time: now},
	})
}

func (d *Dispatcher) recordSendResult(policy NotifyPolicy, policyChannel NotifyPolicyChannel, req DispatchRequest, channel model.AlertNotifyChannel, sendResult SendResult, title, content string, card model.JSONMap, now time.Time) error {
	policyChannel = d.withNotifyGroupSnapshot(policyChannel)
	request := model.JSONMap{
		"title":             title,
		"content":           content,
		"card":              card,
		"receiverGroupId":   policyChannel.ReceiverGroupID,
		"receiverGroupName": policyChannel.ReceiverGroupName,
		"receivers":         policyChannel.Receivers,
		"userIds":           policyChannel.UserIDs,
	}
	requestJSON, _ := json.Marshal(request)
	return d.store.CreateNotifyRecord(&model.AlertNotifyRecord{
		EventID:         req.Event.ID,
		EventType:       notifyRecordEventType(req.EventType),
		NotifyGroupKey:  notifyGroupKeyForRequest(policy.ID, req),
		NotifyRuleID:    policy.ID,
		RuleID:          req.Event.RuleID,
		RuleName:        req.Event.RuleName,
		ChannelID:       channel.ID,
		ChannelName:     channel.Name,
		ChannelType:     channel.Type,
		NotifyGroupID:   policyChannel.ReceiverGroupID,
		NotifyGroupName: policyChannel.ReceiverGroupName,
		DatasourceID:    req.Event.DatasourceID,
		DatasourceType:  req.Event.DatasourceType,
		Severity:        eventLevel(req.Event),
		Receiver:        sendResult.Target,
		Status:          sendResult.Status,
		ErrorMessage:    sendResult.ErrorMessage,
		EventCount:      len(req.Events),
		ResourceCount:   len(dispatchResources(req.Events)),
		RuleNames:       model.JSONMap{fmt.Sprint(req.Event.RuleID): req.Event.RuleName},
		Request:         string(requestJSON),
		Response:        sendResult.Response,
		Content:         firstNonEmpty(content, title),
		BusinessID:      req.Event.BusinessID,
		Operator:        "alert-engine",
		CreatedAt:       util.HTime{Time: now},
	})
}

func (d *Dispatcher) withNotifyGroupSnapshot(channel NotifyPolicyChannel) NotifyPolicyChannel {
	if channel.ReceiverGroupID == 0 || strings.TrimSpace(channel.ReceiverGroupName) != "" {
		return channel
	}
	group, err := d.store.GetNotifyGroupByID(channel.ReceiverGroupID)
	if err == nil {
		channel.ReceiverGroupName = group.Name
	}
	return channel
}

func notifyRecordEventType(eventType string) string {
	if eventType == model.EventStatusResolved {
		return model.EventStatusResolved
	}
	return model.EventDisplayStatusActive
}

func notifyGroupKey(notifyRuleID uint, event model.AlertActiveEvent) string {
	return notifyGroupKeyWithRoute(notifyRuleID, event, 0)
}

func notifyGroupKeyForRequest(notifyRuleID uint, req DispatchRequest) string {
	return notifyGroupKeyWithRoute(notifyRuleID, req.Event, req.notifyRouteID)
}

func notifyGroupKeyWithRoute(notifyRuleID uint, event model.AlertActiveEvent, notifyRouteID uint) string {
	parts := []string{
		fmt.Sprintf("notify:%d", notifyRuleID),
		fmt.Sprintf("rule:%d", event.RuleID),
	}
	if businessID := strings.TrimSpace(event.BusinessID); businessID != "" {
		parts = append(parts, "business:"+businessID)
	}
	if service := jsonMapString(event.Labels, "service"); service != "" {
		parts = append(parts, "service:"+strings.ToLower(service))
	}
	if notifyRouteID > 0 {
		parts = append(parts, fmt.Sprintf("route:%d", notifyRouteID))
	}
	return strings.Join(parts, "|")
}
