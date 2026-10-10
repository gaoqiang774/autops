package notify

import (
	"testing"

	"dodevops-api/api/alertmanager/model"
)

func TestMatchNotifyRouteRules(t *testing.T) {
	rules := []model.AlertNotifyRouteRule{
		{ID: 2, Conditions: routeConditions(routeCondition("service", model.NotifyRouteMatchContains, "data")), NotifyGroupID: 20, Priority: 10, Enabled: 1},
		{ID: 1, Conditions: routeConditions(routeCondition("service", model.NotifyRouteMatchExact, "bigdata"), routeCondition("service", model.NotifyRouteMatchExact, "大数据")), NotifyGroupID: 10, Priority: 100, Enabled: 1},
	}

	route, ok := MatchNotifyRouteRules(rules, model.JSONMap{"service": "BIGDATA"})
	if !ok || route.NotifyGroupID != 10 {
		t.Fatalf("route = %#v, %v; want notifyGroupId 10", route, ok)
	}
}

func TestMatchNotifyRouteRuleModes(t *testing.T) {
	tests := []struct {
		name  string
		mode  string
		value string
	}{
		{name: "exact", mode: model.NotifyRouteMatchExact, value: "大数据"},
		{name: "contains", mode: model.NotifyRouteMatchContains, value: "数据"},
		{name: "regex", mode: model.NotifyRouteMatchRegex, value: "^大.*据$"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := model.AlertNotifyRouteRule{Conditions: routeConditions(routeCondition("service", tt.mode, tt.value))}
			if !MatchNotifyRouteRule(rule, model.JSONMap{"service": "大数据"}) {
				t.Fatal("expected route to match")
			}
		})
	}
}

func TestMatchNotifyRouteRuleConditionsKeepKeyValuePairs(t *testing.T) {
	rule := model.AlertNotifyRouteRule{
		Conditions: model.AlertNotifyRouteConditions{
			{LabelKey: "service", MatchMode: model.NotifyRouteMatchExact, MatchValue: "bigdata"},
			{LabelKey: "app", MatchMode: model.NotifyRouteMatchExact, MatchValue: "data-platform"},
		},
	}
	if !MatchNotifyRouteRule(rule, model.JSONMap{"app": "data-platform"}) {
		t.Fatal("expected app condition to match")
	}
	if MatchNotifyRouteRule(rule, model.JSONMap{"service": "data-platform"}) {
		t.Fatal("did not expect value from another condition to match")
	}
}

func TestMatchNotifyRouteRulesForPolicyScopesRoutes(t *testing.T) {
	rules := []model.AlertNotifyRouteRule{
		{ID: 1, NotifyRuleID: 10, Conditions: routeConditions(routeCondition("service", model.NotifyRouteMatchExact, "bigdata")), NotifyGroupID: 100, Priority: 100, Enabled: 1},
		{ID: 2, NotifyRuleID: 20, Conditions: routeConditions(routeCondition("service", model.NotifyRouteMatchExact, "bigdata")), NotifyGroupID: 200, Priority: 100, Enabled: 1},
	}
	matched, ok := MatchNotifyRouteRulesForPolicy(rules, 20, model.JSONMap{"service": "bigdata"})
	if !ok || matched.NotifyGroupID != 200 {
		t.Fatalf("matched = %#v, %v; want policy 20 route", matched, ok)
	}
}

func routeCondition(labelKey, matchMode, matchValue string) model.AlertNotifyRouteCondition {
	return model.AlertNotifyRouteCondition{LabelKey: labelKey, MatchMode: matchMode, MatchValue: matchValue}
}

func routeConditions(conditions ...model.AlertNotifyRouteCondition) model.AlertNotifyRouteConditions {
	return conditions
}
