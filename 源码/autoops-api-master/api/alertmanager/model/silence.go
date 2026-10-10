package model

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SilenceMatchContext is the normalized alert identity used by both the
// evaluation engine and event-management APIs when checking silence rules.
type SilenceMatchContext struct {
	Fingerprint  string
	RuleID       uint
	RuleName     string
	DatasourceID uint
	Severity     string
	BusinessID   string
	Labels       JSONMap
}

func SilenceContextFromEvent(event AlertActiveEvent) SilenceMatchContext {
	return SilenceMatchContext{
		Fingerprint:  event.Fingerprint,
		RuleID:       event.RuleID,
		RuleName:     event.RuleName,
		DatasourceID: event.DatasourceID,
		Severity:     event.Severity,
		BusinessID:   event.BusinessID,
		Labels:       event.Labels,
	}
}

func SilenceRuleIsActive(rule AlertSilenceRule, now time.Time) bool {
	return rule.Enabled == RuleStatusEnabled &&
		!rule.StartTime.Time.After(now) &&
		!rule.EndTime.Time.Before(now)
}

func EventStatusAfterSilence(event AlertActiveEvent, rule AlertRule, now time.Time) string {
	if event.Suppressed == 1 {
		return EventStatusSuppressed
	}
	requiredSeconds := rule.ForDurationSeconds - rule.EvalIntervalSeconds
	if requiredSeconds <= 0 {
		return EventStatusFiring
	}
	if event.FirstTriggeredAt.IsZero() || now.Before(event.FirstTriggeredAt.Time.Add(time.Duration(requiredSeconds)*time.Second)) {
		return EventStatusPending
	}
	return EventStatusFiring
}

// SilenceRuleMatches applies the matcher format used by the UI and accepts
// direct fingerprint/ruleId maps used by legacy event silences.
func SilenceRuleMatches(rule AlertSilenceRule, context SilenceMatchContext) bool {
	labels := make(map[string]string, len(context.Labels)+8)
	for key, value := range context.Labels {
		labels[key] = fmt.Sprint(value)
	}
	labels["fingerprint"] = context.Fingerprint
	labels["ruleId"] = strconv.FormatUint(uint64(context.RuleID), 10)
	labels["ruleName"] = context.RuleName
	labels["alertname"] = context.RuleName
	labels["severity"] = context.Severity
	labels["businessId"] = context.BusinessID
	labels["datasourceId"] = strconv.FormatUint(uint64(context.DatasourceID), 10)

	if rawItems, exists := rule.Matchers["items"]; exists {
		items := silenceMatcherItems(rawItems)
		if len(items) == 0 {
			return false
		}
		for _, item := range items {
			if !silenceMatcherMatches(item, labels) {
				return false
			}
		}
		return true
	}

	matched := 0
	for key, expected := range rule.Matchers {
		if key == "eventId" {
			continue
		}
		matched++
		if labels[key] != fmt.Sprint(expected) {
			return false
		}
	}
	return matched > 0
}

func silenceMatcherItems(raw interface{}) []map[string]interface{} {
	switch items := raw.(type) {
	case []interface{}:
		result := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			if matcher, ok := item.(map[string]interface{}); ok {
				result = append(result, matcher)
			}
		}
		return result
	case []JSONMap:
		result := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			result = append(result, map[string]interface{}(item))
		}
		return result
	default:
		return nil
	}
}

func silenceMatcherMatches(matcher map[string]interface{}, labels map[string]string) bool {
	name := silenceMatcherValue(matcher, "name", "label")
	if name == "" {
		return false
	}
	op := silenceMatcherValue(matcher, "op", "operator")
	if op == "" {
		op = "="
	}
	expected := silenceMatcherValue(matcher, "value")
	actual := labels[name]
	switch op {
	case "=", "==":
		return actual == expected
	case "!=":
		return actual != expected
	case "=~":
		compiled, err := regexp.Compile("^(?:" + expected + ")$")
		return err == nil && compiled.MatchString(actual)
	case "!~":
		compiled, err := regexp.Compile("^(?:" + expected + ")$")
		return err == nil && !compiled.MatchString(actual)
	default:
		return false
	}
}

func silenceMatcherValue(matcher map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, exists := matcher[key]
		if !exists || value == nil {
			continue
		}
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return ""
}
