package notify

import (
	"regexp"
	"sort"
	"strings"

	"dodevops-api/api/alertmanager/model"
)

// MatchNotifyRouteRules returns one deterministic route. Higher priority wins;
// the smaller ID wins when priorities are equal.
func MatchNotifyRouteRules(rules []model.AlertNotifyRouteRule, labels model.JSONMap) (model.AlertNotifyRouteRule, bool) {
	return MatchNotifyRouteRulesForPolicy(rules, 0, labels)
}

// MatchNotifyRouteRulesForPolicy returns the highest-priority route that is
// either global (notifyRuleId=0) or explicitly attached to policyID.
func MatchNotifyRouteRulesForPolicy(rules []model.AlertNotifyRouteRule, policyID uint, labels model.JSONMap) (model.AlertNotifyRouteRule, bool) {
	candidates := append([]model.AlertNotifyRouteRule(nil), rules...)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority == candidates[j].Priority {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].Priority > candidates[j].Priority
	})
	for _, rule := range candidates {
		if rule.Enabled != model.RuleStatusEnabled || rule.NotifyGroupID == 0 {
			continue
		}
		if rule.NotifyRuleID != 0 && rule.NotifyRuleID != policyID {
			continue
		}
		if MatchNotifyRouteRule(rule, labels) {
			return rule, true
		}
	}
	return model.AlertNotifyRouteRule{}, false
}

func MatchNotifyRouteRule(rule model.AlertNotifyRouteRule, labels model.JSONMap) bool {
	for _, condition := range rule.Conditions {
		actual := jsonMapString(labels, condition.LabelKey)
		if matchNotifyRouteValue(actual, condition.MatchValue, condition.MatchMode) {
			return true
		}
	}
	return false
}

func matchNotifyRouteValue(actual, candidate, mode string) bool {
	actual = strings.TrimSpace(actual)
	candidate = strings.TrimSpace(candidate)
	if actual == "" || candidate == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", model.NotifyRouteMatchExact:
		return strings.EqualFold(actual, candidate)
	case model.NotifyRouteMatchContains:
		return strings.Contains(strings.ToLower(actual), strings.ToLower(candidate))
	case model.NotifyRouteMatchRegex:
		expression, err := regexp.Compile(candidate)
		return err == nil && expression.MatchString(actual)
	default:
		return false
	}
}
