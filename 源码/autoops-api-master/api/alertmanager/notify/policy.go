package notify

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dodevops-api/api/alertmanager/model"
)

const (
	MatcherOpEqual     = "="
	MatcherOpNotEqual  = "!="
	MatcherOpRegex     = "=~"
	MatcherOpNotRegex  = "!~"
	MatcherOpIn        = "in"
	MatcherOpNotIn     = "not_in"
	MatcherOpExists    = "exists"
	MatcherOpNotExists = "not_exists"
)

type NotifyPolicy struct {
	ID                       uint
	Name                     string
	Enabled                  int
	Priority                 int
	RepeatNotify             int
	RepeatMode               string
	RepeatIntervalSeconds    int64
	RepeatIntervals          map[string]int64
	MaxRepeatTimes           int
	RecoveryNotify           int
	AggregationEnabled       int
	GroupWaitSeconds         int64
	GroupIntervalSeconds     int64
	AggregationWindowSeconds int64
	Matchers                 NotifyMatchers
	Channels                 []NotifyPolicyChannel
}

type NotifyMatchers struct {
	Severities      []string        `json:"severities"`
	DatasourceTypes []string        `json:"datasourceTypes"`
	DatasourceIDs   []uint          `json:"datasourceIds"`
	RuleIDs         []uint          `json:"ruleIds"`
	RuleGroupIDs    []uint          `json:"ruleGroupIds"`
	BusinessIDs     []string        `json:"businessIds"`
	Labels          []TagMatcher    `json:"labels"`
	Annotations     []TagMatcher    `json:"annotations"`
	TimeRanges      []TimeRangeRule `json:"timeRanges"`
}

type TagMatcher struct {
	Key    string   `json:"key"`
	Op     string   `json:"op"`
	Value  string   `json:"value"`
	Values []string `json:"values"`
}

type TimeRangeRule struct {
	Weekdays []int  `json:"weekdays"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

type NotifyPolicyChannel struct {
	ChannelID         uint     `json:"channelId"`
	TemplateID        uint     `json:"templateId"`
	ReceiverGroupID   uint     `json:"receiverGroupId"`
	ReceiverGroupName string   `json:"-"`
	Receivers         []string `json:"receivers"`
	UserIDs           []string `json:"userIds"`
	Enabled           *bool    `json:"enabled"`
}

type NotifyEventContext struct {
	Event       model.AlertActiveEvent
	Rule        model.AlertRule
	RuleGroupID uint
	Now         time.Time
}

type MatchResult struct {
	Matched bool
	Reasons []string
}

type notifyChannelsEnvelope struct {
	Items []NotifyPolicyChannel `json:"items"`
}

func ParseNotifyPolicy(rule model.AlertNotifyRule) (NotifyPolicy, error) {
	matchers, err := parseNotifyMatchers(rule.Matchers)
	if err != nil {
		return NotifyPolicy{}, err
	}
	channels, err := parseNotifyPolicyChannels(rule.Channels)
	if err != nil {
		return NotifyPolicy{}, err
	}
	return NotifyPolicy{
		ID:                       rule.ID,
		Name:                     strings.TrimSpace(rule.Name),
		Enabled:                  rule.Enabled,
		Priority:                 rule.Priority,
		RepeatNotify:             rule.RepeatNotify,
		RepeatMode:               normalizeRepeatMode(rule.RepeatMode),
		RepeatIntervalSeconds:    rule.RepeatIntervalSeconds,
		RepeatIntervals:          parseRepeatIntervals(rule.RepeatIntervals),
		MaxRepeatTimes:           rule.MaxRepeatTimes,
		RecoveryNotify:           rule.RecoveryNotify,
		AggregationEnabled:       rule.AggregationEnabled,
		GroupWaitSeconds:         rule.GroupWaitSeconds,
		GroupIntervalSeconds:     rule.GroupIntervalSeconds,
		AggregationWindowSeconds: rule.AggregationWindowSeconds,
		Matchers:                 matchers,
		Channels:                 channels,
	}, nil
}

func parseNotifyMatchers(raw model.JSONMap) (NotifyMatchers, error) {
	if len(raw) == 0 {
		return NotifyMatchers{}, nil
	}
	var matchers NotifyMatchers
	if err := decodeJSON(raw, &matchers); err != nil {
		return matchers, fmt.Errorf("invalid matchers: %w", err)
	}
	return matchers, nil
}

func parseNotifyPolicyChannels(raw model.JSONMap) ([]NotifyPolicyChannel, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var envelope notifyChannelsEnvelope
	if err := decodeJSON(raw, &envelope); err != nil {
		return nil, fmt.Errorf("invalid channels: %w", err)
	}
	if envelope.Items != nil {
		return envelope.Items, nil
	}

	var channels []NotifyPolicyChannel
	if value, ok := raw["items"]; ok {
		if err := decodeJSON(value, &channels); err != nil {
			return nil, fmt.Errorf("invalid channels.items: %w", err)
		}
		return channels, nil
	}

	if err := decodeJSON(raw, &channels); err == nil {
		return channels, nil
	}
	return nil, errors.New("invalid channels: expected {items:[...]} or [...]")
}

func ValidateNotifyPolicy(policy NotifyPolicy) error {
	if strings.TrimSpace(policy.Name) == "" {
		return errors.New("name is required")
	}
	switch normalizeRepeatMode(policy.RepeatMode) {
	case "uniform", "bySeverity":
	default:
		return errors.New("repeatMode must be uniform or bySeverity")
	}
	if policy.RepeatIntervalSeconds < 0 {
		return errors.New("repeatIntervalSeconds cannot be negative")
	}
	if policy.MaxRepeatTimes < 0 {
		return errors.New("maxRepeatTimes cannot be negative")
	}
	for severity, seconds := range policy.RepeatIntervals {
		switch severity {
		case "P0", "P1", "P2":
		default:
			return fmt.Errorf("repeatIntervals.%s is unsupported", severity)
		}
		if seconds < 0 {
			return fmt.Errorf("repeatIntervals.%s cannot be negative", severity)
		}
	}
	if policy.GroupWaitSeconds < 0 {
		return errors.New("groupWaitSeconds cannot be negative")
	}
	if policy.GroupIntervalSeconds < 0 {
		return errors.New("groupIntervalSeconds cannot be negative")
	}
	if policy.AggregationWindowSeconds < 0 {
		return errors.New("aggregationWindowSeconds cannot be negative")
	}
	for i := range policy.Matchers.Labels {
		if err := validateTagMatcher(policy.Matchers.Labels[i]); err != nil {
			return fmt.Errorf("labels[%d]: %w", i, err)
		}
	}
	for i := range policy.Matchers.Annotations {
		if err := validateTagMatcher(policy.Matchers.Annotations[i]); err != nil {
			return fmt.Errorf("annotations[%d]: %w", i, err)
		}
	}
	for i := range policy.Matchers.TimeRanges {
		if err := validateTimeRange(policy.Matchers.TimeRanges[i]); err != nil {
			return fmt.Errorf("timeRanges[%d]: %w", i, err)
		}
	}
	for i := range policy.Channels {
		if err := validatePolicyChannel(policy.Channels[i]); err != nil {
			return fmt.Errorf("channels[%d]: %w", i, err)
		}
	}
	return nil
}

func MatchNotifyPolicy(policy NotifyPolicy, ctx NotifyEventContext) MatchResult {
	now := ctx.Now
	if now.IsZero() {
		now = time.Now()
	}
	reasons := make([]string, 0)
	matchers := policy.Matchers

	if !matchStringAnyOf(eventSeverityCandidates(ctx.Event), matchers.Severities) {
		return MatchResult{Matched: false, Reasons: append(reasons, "severity not matched")}
	}
	if len(matchers.Severities) > 0 {
		reasons = append(reasons, "severity matched")
	}

	if !matchStringAny(ctx.Event.DatasourceType, matchers.DatasourceTypes) {
		return MatchResult{Matched: false, Reasons: append(reasons, "datasourceType not matched")}
	}
	if len(matchers.DatasourceTypes) > 0 {
		reasons = append(reasons, "datasourceType matched")
	}

	if !matchUintAny(ctx.Event.DatasourceID, matchers.DatasourceIDs) {
		return MatchResult{Matched: false, Reasons: append(reasons, "datasourceId not matched")}
	}
	if len(matchers.DatasourceIDs) > 0 {
		reasons = append(reasons, "datasourceId matched")
	}

	if !matchUintAny(ctx.Event.RuleID, matchers.RuleIDs) {
		return MatchResult{Matched: false, Reasons: append(reasons, "ruleId not matched")}
	}
	if len(matchers.RuleIDs) > 0 {
		reasons = append(reasons, "ruleId matched")
	}

	if !matchUintAny(ctx.RuleGroupID, matchers.RuleGroupIDs) {
		return MatchResult{Matched: false, Reasons: append(reasons, "ruleGroupId not matched")}
	}
	if len(matchers.RuleGroupIDs) > 0 {
		reasons = append(reasons, "ruleGroupId matched")
	}

	if !matchStringAny(ctx.Event.BusinessID, matchers.BusinessIDs) {
		return MatchResult{Matched: false, Reasons: append(reasons, "businessId not matched")}
	}
	if len(matchers.BusinessIDs) > 0 {
		reasons = append(reasons, "businessId matched")
	}

	if !matchTagMatchers(ctx.Event.Labels, matchers.Labels) {
		return MatchResult{Matched: false, Reasons: append(reasons, "labels not matched")}
	}
	if len(matchers.Labels) > 0 {
		reasons = append(reasons, "labels matched")
	}

	if !matchTagMatchers(ctx.Event.Annotations, matchers.Annotations) {
		return MatchResult{Matched: false, Reasons: append(reasons, "annotations not matched")}
	}
	if len(matchers.Annotations) > 0 {
		reasons = append(reasons, "annotations matched")
	}

	if !matchTimeRanges(now, matchers.TimeRanges) {
		return MatchResult{Matched: false, Reasons: append(reasons, "timeRange not matched")}
	}
	if len(matchers.TimeRanges) > 0 {
		reasons = append(reasons, "timeRange matched")
	}

	return MatchResult{Matched: true, Reasons: reasons}
}

func EnabledPolicyChannels(policy NotifyPolicy) []NotifyPolicyChannel {
	channels := make([]NotifyPolicyChannel, 0, len(policy.Channels))
	for _, channel := range policy.Channels {
		if channel.Enabled != nil && !*channel.Enabled {
			continue
		}
		channels = append(channels, channel)
	}
	return channels
}

func validateTagMatcher(matcher TagMatcher) error {
	if strings.TrimSpace(matcher.Key) == "" {
		return errors.New("key is required")
	}
	op := normalizeMatcherOp(matcher.Op)
	switch op {
	case MatcherOpEqual, MatcherOpNotEqual, MatcherOpRegex, MatcherOpNotRegex:
		if strings.TrimSpace(matcher.Value) == "" {
			return errors.New("value is required")
		}
	case MatcherOpIn, MatcherOpNotIn:
		if len(matcher.Values) == 0 && strings.TrimSpace(matcher.Value) == "" {
			return errors.New("values is required")
		}
	case MatcherOpExists, MatcherOpNotExists:
	default:
		return fmt.Errorf("unsupported op %q", matcher.Op)
	}
	if op == MatcherOpRegex || op == MatcherOpNotRegex {
		if _, err := regexp.Compile(matcher.Value); err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
	}
	return nil
}

func validateTimeRange(item TimeRangeRule) error {
	if len(item.Weekdays) == 0 {
		return errors.New("weekdays is required")
	}
	for _, weekday := range item.Weekdays {
		if weekday < 0 || weekday > 6 {
			return errors.New("weekday must be 0-6")
		}
	}
	if _, err := parseClockMinute(item.Start); err != nil {
		return fmt.Errorf("invalid start: %w", err)
	}
	if _, err := parseClockMinute(item.End); err != nil {
		return fmt.Errorf("invalid end: %w", err)
	}
	return nil
}

func validatePolicyChannel(channel NotifyPolicyChannel) error {
	if channel.ChannelID == 0 {
		return errors.New("channelId is required")
	}
	if channel.TemplateID == 0 {
		return errors.New("templateId is required")
	}
	return nil
}

func matchStringAny(value string, candidates []string) bool {
	if len(candidates) == 0 {
		return true
	}
	value = strings.TrimSpace(value)
	for _, candidate := range candidates {
		if strings.EqualFold(value, strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func matchStringAnyOf(values []string, candidates []string) bool {
	if len(candidates) == 0 {
		return true
	}
	for _, value := range values {
		if matchStringAny(value, candidates) {
			return true
		}
	}
	return false
}

func eventSeverityCandidates(event model.AlertActiveEvent) []string {
	values := make([]string, 0, 3)
	if level := jsonMapString(event.Labels, "level"); level != "" {
		values = append(values, level)
	}
	if event.Severity != "" {
		values = append(values, event.Severity)
	}
	if mapped := severityToLevel(event.Severity); mapped != "" {
		values = append(values, mapped)
	}
	return values
}

func jsonMapString(values model.JSONMap, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func severityToLevel(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "p0", "critical", "serious", "fatal":
		return "P0"
	case "p1", "warning", "warn":
		return "P1"
	case "p2", "info", "notice":
		return "P2"
	default:
		return ""
	}
}

func matchUintAny(value uint, candidates []uint) bool {
	if len(candidates) == 0 {
		return true
	}
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func matchTagMatchers(values model.JSONMap, matchers []TagMatcher) bool {
	for _, matcher := range matchers {
		if !matchTagMatcher(values, matcher) {
			return false
		}
	}
	return true
}

func matchTagMatcher(values model.JSONMap, matcher TagMatcher) bool {
	key := strings.TrimSpace(matcher.Key)
	raw, exists := values[key]
	op := normalizeMatcherOp(matcher.Op)
	if op == MatcherOpExists {
		return exists
	}
	if op == MatcherOpNotExists {
		return !exists
	}
	if !exists {
		return false
	}

	value := strings.TrimSpace(fmt.Sprint(raw))
	switch op {
	case MatcherOpEqual:
		return value == strings.TrimSpace(matcher.Value)
	case MatcherOpNotEqual:
		return value != strings.TrimSpace(matcher.Value)
	case MatcherOpRegex:
		return regexp.MustCompile(matcher.Value).MatchString(value)
	case MatcherOpNotRegex:
		return !regexp.MustCompile(matcher.Value).MatchString(value)
	case MatcherOpIn:
		return stringIn(value, matcherValues(matcher))
	case MatcherOpNotIn:
		return !stringIn(value, matcherValues(matcher))
	default:
		return false
	}
}

func matchTimeRanges(now time.Time, ranges []TimeRangeRule) bool {
	if len(ranges) == 0 {
		return true
	}
	for _, item := range ranges {
		if matchTimeRange(now, item) {
			return true
		}
	}
	return false
}

func matchTimeRange(now time.Time, item TimeRangeRule) bool {
	weekday := int(now.Weekday())
	if !intIn(weekday, item.Weekdays) {
		return false
	}
	start, err := parseClockMinute(item.Start)
	if err != nil {
		return false
	}
	end, err := parseClockMinute(item.End)
	if err != nil {
		return false
	}
	current := now.Hour()*60 + now.Minute()
	if start == end {
		return true
	}
	if start < end {
		return current >= start && current < end
	}
	return current >= start || current < end
}

func normalizeMatcherOp(op string) string {
	op = strings.TrimSpace(strings.ToLower(op))
	switch op {
	case "", "=", "==", "eq":
		return MatcherOpEqual
	case "!=", "ne":
		return MatcherOpNotEqual
	case "=~", "regex":
		return MatcherOpRegex
	case "!~", "not_regex":
		return MatcherOpNotRegex
	case "in":
		return MatcherOpIn
	case "not_in", "notin":
		return MatcherOpNotIn
	case "exists":
		return MatcherOpExists
	case "not_exists", "notexists":
		return MatcherOpNotExists
	default:
		return op
	}
}

func matcherValues(matcher TagMatcher) []string {
	if len(matcher.Values) > 0 {
		return matcher.Values
	}
	if strings.TrimSpace(matcher.Value) == "" {
		return nil
	}
	parts := strings.Split(matcher.Value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func parseRepeatIntervals(raw model.JSONMap) map[string]int64 {
	result := make(map[string]int64)
	if len(raw) == 0 {
		return result
	}
	for key, value := range raw {
		severity := strings.ToUpper(strings.TrimSpace(key))
		switch v := value.(type) {
		case int:
			result[severity] = int64(v)
		case int64:
			result[severity] = v
		case int32:
			result[severity] = int64(v)
		case uint:
			result[severity] = int64(v)
		case uint64:
			result[severity] = int64(v)
		case uint32:
			result[severity] = int64(v)
		case float64:
			result[severity] = int64(v)
		case float32:
			result[severity] = int64(v)
		case json.Number:
			if parsed, err := v.Int64(); err == nil {
				result[severity] = parsed
			}
		case string:
			if parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
				result[severity] = parsed
			}
		}
	}
	return result
}

func normalizeRepeatMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return "bySeverity"
	}
	switch strings.ToLower(mode) {
	case "uniform":
		return "uniform"
	case "byseverity", "by_severity", "severity":
		return "bySeverity"
	default:
		return mode
	}
}

func stringIn(value string, values []string) bool {
	for _, item := range values {
		if value == strings.TrimSpace(item) {
			return true
		}
	}
	return false
}

func intIn(value int, values []int) bool {
	for _, item := range values {
		if value == item {
			return true
		}
	}
	return false
}

func parseClockMinute(value string) (int, error) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, errors.New("expected HH:mm")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, errors.New("hour must be 0-23")
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, errors.New("minute must be 0-59")
	}
	return hour*60 + minute, nil
}

func decodeJSON(raw interface{}, target interface{}) error {
	bs, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(bs, target)
}
