package service

import (
	"dodevops-api/api/alertmanager/model"
	"dodevops-api/common/util"
	"testing"
	"time"
)

func TestClassifyPrometheusTemplateUsesSourceExporterForNodeExporter(t *testing.T) {
	item := classifyPrometheusTemplate("Basic resource monitoring", awesomePrometheusRuleRef{
		Name:     "Host and hardware",
		Exporter: "node-exporter",
	}, prometheusRuleItem{
		Alert: "Host out of memory",
		Expr:  "node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes < .10",
	})

	if item.Component != "Linux" || item.Collector != "node_exporter" || item.ResourceType != "host" {
		t.Fatalf("unexpected classification: %+v", item)
	}
}

func TestDatasourceTypeOptionsIncludesVictoriaLogs(t *testing.T) {
	options := datasourceTypeOptions()
	for _, option := range options {
		if option.Type == model.AlertDatasourceVictoriaLogs {
			if option.QueryLang != "logsql" {
				t.Fatalf("victorialogs query lang = %q, want logsql", option.QueryLang)
			}
			return
		}
	}
	t.Fatalf("datasourceTypeOptions() missing victorialogs: %#v", options)
}

func TestFillDashboardTrendBucketsUsesHoursForShortRanges(t *testing.T) {
	start := time.Date(2020, 7, 14, 10, 30, 0, 0, time.Local)
	end := start.Add(3 * time.Hour)
	rows := []model.AlertDashboardNotifyTrendPoint{{
		Date:              "2020-07-14 11:00",
		NotificationCount: 8,
		AlertCount:        12,
	}}

	result := fillDashboardTrendBuckets(rows, start, end, "notify_records")

	if len(result) != 4 {
		t.Fatalf("hourly bucket count = %d, want 4", len(result))
	}
	if result[0].Date != "2020-07-14 10:00" || result[1].Date != "2020-07-14 11:00" {
		t.Fatalf("unexpected hourly buckets: %+v", result)
	}
	if result[1].NotificationCount != 8 || result[1].AlertCount != 12 {
		t.Fatalf("hourly data was not preserved: %+v", result[1])
	}
}

func TestFillDashboardTrendBucketsUsesDaysForLongRanges(t *testing.T) {
	start := time.Date(2020, 6, 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 0, 7)

	result := fillDashboardTrendBuckets(nil, start, end, "no_notify_records")

	if len(result) != 7 || result[0].Date != "2020-06-01" || result[6].Date != "2020-06-07" {
		t.Fatalf("unexpected daily buckets: %+v", result)
	}
}

func TestBuildBusinessRouteTopUsesNotificationRoutingRules(t *testing.T) {
	routes := []model.AlertNotifyRouteRule{
		{
			ID:            1,
			Name:          "大数据业务路由",
			Conditions:    model.AlertNotifyRouteConditions{{LabelKey: "service", MatchMode: model.NotifyRouteMatchExact, MatchValue: "bigdata"}},
			NotifyRuleID:  7,
			NotifyGroupID: 10,
			Priority:      100,
			Enabled:       model.RuleStatusEnabled,
		},
		{
			ID:            2,
			Name:          "全局数据路由",
			Conditions:    model.AlertNotifyRouteConditions{{LabelKey: "service", MatchMode: model.NotifyRouteMatchContains, MatchValue: "data"}},
			NotifyGroupID: 20,
			Priority:      20,
			Enabled:       model.RuleStatusEnabled,
		},
	}
	events := []model.AlertDashboardEventRow{
		{NotifyRuleIDs: model.JSONUintArray{7}, Labels: model.JSONMap{"service": "BIGDATA"}},
		{NotifyRuleIDs: model.JSONUintArray{8}, Labels: model.JSONMap{"service": "metadata"}},
		{NotifyRuleIDs: model.JSONUintArray{7}, Labels: model.JSONMap{"service": "api"}},
		{Labels: model.JSONMap{"service": "bigdata"}},
	}

	result := buildBusinessRouteTop(events, routes, 5)
	counts := make(map[string]int64, len(result))
	for _, item := range result {
		counts[item.GroupName] = item.Count
	}

	if counts["大数据业务组"] != 1 {
		t.Fatalf("scoped route count = %d, want 1; result=%+v", counts["大数据业务组"], result)
	}
	if counts["全局数据组"] != 1 {
		t.Fatalf("global route count = %d, want 1; result=%+v", counts["全局数据组"], result)
	}
	if counts["运维组"] != 2 {
		t.Fatalf("operations group count = %d, want 2; result=%+v", counts["运维组"], result)
	}
}

func TestBuildRuleGroupTopUsesAlertRuleGroups(t *testing.T) {
	events := []model.AlertDashboardEventRow{
		{RuleGroupID: 2, RuleGroupName: "aiops"},
		{RuleGroupID: 1, RuleGroupName: "zero-ai"},
		{RuleGroupID: 2, RuleGroupName: "aiops"},
		{RuleGroupID: 0, RuleGroupName: ""},
	}

	result := buildRuleGroupTop(events, 5)
	counts := make(map[string]int64, len(result))
	for _, item := range result {
		counts[item.GroupName] = item.Count
	}

	if counts["aiops"] != 2 {
		t.Fatalf("aiops count = %d, want 2; result=%+v", counts["aiops"], result)
	}
	if counts["zero-ai"] != 1 {
		t.Fatalf("zero-ai count = %d, want 1; result=%+v", counts["zero-ai"], result)
	}
	if counts["未分组"] != 1 {
		t.Fatalf("ungrouped count = %d, want 1; result=%+v", counts["未分组"], result)
	}
}
func TestMatchDashboardBusinessRouteChoosesHighestPriorityAcrossPolicies(t *testing.T) {
	routes := []model.AlertNotifyRouteRule{
		{
			ID:            10,
			Name:          "策略一业务路由",
			Conditions:    model.AlertNotifyRouteConditions{{LabelKey: "service", MatchMode: model.NotifyRouteMatchExact, MatchValue: "api"}},
			NotifyRuleID:  1,
			NotifyGroupID: 10,
			Priority:      50,
			Enabled:       model.RuleStatusEnabled,
		},
		{
			ID:            20,
			Name:          "策略二业务路由",
			Conditions:    model.AlertNotifyRouteConditions{{LabelKey: "service", MatchMode: model.NotifyRouteMatchExact, MatchValue: "api"}},
			NotifyRuleID:  2,
			NotifyGroupID: 20,
			Priority:      100,
			Enabled:       model.RuleStatusEnabled,
		},
	}
	event := model.AlertDashboardEventRow{
		NotifyRuleIDs: model.JSONUintArray{1, 2},
		Labels:        model.JSONMap{"service": "api"},
	}

	route, ok := matchDashboardBusinessRoute(routes, event)
	if !ok || route.ID != 20 {
		t.Fatalf("matched route = %+v, ok=%t; want route 20", route, ok)
	}
}

func TestActiveEventBusinessFilterUsesDashboardRouteMatching(t *testing.T) {
	routes := []model.AlertNotifyRouteRule{
		{
			ID:            10,
			Name:          "大数据业务路由",
			Conditions:    model.AlertNotifyRouteConditions{{LabelKey: "service", MatchMode: model.NotifyRouteMatchExact, MatchValue: "bigdata"}},
			NotifyRuleID:  7,
			NotifyGroupID: 10,
			Priority:      100,
			Enabled:       model.RuleStatusEnabled,
		},
		{
			ID:            20,
			Name:          "全局数据路由",
			Conditions:    model.AlertNotifyRouteConditions{{LabelKey: "service", MatchMode: model.NotifyRouteMatchContains, MatchValue: "data"}},
			NotifyGroupID: 20,
			Priority:      20,
			Enabled:       model.RuleStatusEnabled,
		},
	}
	events := []model.AlertActiveEvent{
		{ID: 1, RuleID: 101, Labels: model.JSONMap{"service": "BIGDATA"}},
		{ID: 2, RuleID: 102, Labels: model.JSONMap{"service": "metadata"}},
		{ID: 3, RuleID: 101, Labels: model.JSONMap{"service": "api"}},
		{ID: 4, RuleID: 103, Labels: model.JSONMap{"service": "bigdata"}},
	}
	notifyRuleIDs := map[uint]model.JSONUintArray{
		101: {7},
		102: {8},
	}

	bigData := filterActiveEventsByMatchedBusinessRoute(events, routes, notifyRuleIDs, "10")
	if len(bigData) != 1 || bigData[0].ID != 1 {
		t.Fatalf("big-data business events = %+v, want event 1", bigData)
	}
	globalData := filterActiveEventsByMatchedBusinessRoute(events, routes, notifyRuleIDs, "20")
	if len(globalData) != 1 || globalData[0].ID != 2 {
		t.Fatalf("global-data business events = %+v, want event 2", globalData)
	}
	operations := filterActiveEventsByMatchedBusinessRoute(events, routes, notifyRuleIDs, model.AlertBusinessRouteOperations)
	if len(operations) != 2 || operations[0].ID != 3 || operations[1].ID != 4 {
		t.Fatalf("operations business events = %+v, want events 3 and 4", operations)
	}
}

func TestBuildActiveEventBusinessOptionsMatchesDashboardNames(t *testing.T) {
	options := buildActiveEventBusinessOptions([]model.AlertNotifyRouteRule{
		{ID: 1, Name: "大数据业务路由", NotifyGroupID: 10, Enabled: model.RuleStatusEnabled},
		{ID: 2, Name: "停用路由", NotifyGroupID: 20, Enabled: model.RuleStatusDisabled},
	})

	if len(options) != 2 {
		t.Fatalf("business options = %+v, want one enabled route and operations", options)
	}
	if options[0].Value != "1" || options[0].Label != "大数据业务组" {
		t.Fatalf("route option = %+v, want dashboard group naming", options[0])
	}
	if options[1].Value != model.AlertBusinessRouteOperations || options[1].Label != "运维组" {
		t.Fatalf("fallback option = %+v, want operations", options[1])
	}
}

func TestClassifyPrometheusTemplateDoesNotTreatKubernetesNodeAsNodeExporter(t *testing.T) {
	item := classifyPrometheusTemplate("Orchestrators", awesomePrometheusRuleRef{
		Name:     "Kubernetes",
		Exporter: "kubestate-exporter",
	}, prometheusRuleItem{
		Alert: "Kubernetes Node memory pressure",
		Expr:  `kube_node_status_condition{condition="MemoryPressure",status="true"} == 1`,
	})

	if item.Component != "Kubernetes" || item.Collector != "kube-state-metrics" || item.ResourceType != "k8s" {
		t.Fatalf("unexpected classification: %+v", item)
	}
}

func TestClassifyPrometheusTemplateDoesNotTreatJenkinsNodeAsNodeExporter(t *testing.T) {
	item := classifyPrometheusTemplate("CI/CD", awesomePrometheusRuleRef{
		Name:     "Jenkins",
		Exporter: "metric-plugin",
	}, prometheusRuleItem{
		Alert: "Jenkins node offline",
		Expr:  "jenkins_node_offline_value > 0",
	})

	if item.Collector == "node_exporter" || item.Component == "Linux" {
		t.Fatalf("unexpected node_exporter classification: %+v", item)
	}
}

func TestClassifyPrometheusTemplateTreatsKubernetesSourceAsKubernetes(t *testing.T) {
	item := classifyPrometheusTemplate("Orchestrators", awesomePrometheusRuleRef{
		Name:     "Kubernetes",
		Exporter: "kubestate-exporter",
	}, prometheusRuleItem{
		Alert: "Kubernetes pod crash looping",
		Expr:  `kube_pod_container_status_restarts_total > 3`,
	})

	if item.Component != "Kubernetes" || item.Collector != "kube-state-metrics" {
		t.Fatalf("unexpected classification: %+v", item)
	}
}

func TestClassifyPrometheusTemplateDoesNotTreatCiliumK8sMetricAsKubernetesTemplate(t *testing.T) {
	item := classifyPrometheusTemplate("Network and security", awesomePrometheusRuleRef{
		Name:     "Cilium",
		Exporter: "embedded-exporter",
	}, prometheusRuleItem{
		Alert: "Cilium agent Kubernetes client errors",
		Expr:  `sum(rate(cilium_k8s_client_api_calls_total[5m])) by (pod) > 0.05`,
	})

	if item.Component == "Kubernetes" || item.Collector == "kube-state-metrics" {
		t.Fatalf("unexpected kubernetes classification: %+v", item)
	}
}

func TestMatchesPrometheusTemplateQuerySupportsLinuxNodeExporterKeyword(t *testing.T) {
	item := modelPrometheusTemplateImportItem("Linux", "node_exporter")

	if !matchesPrometheusTemplateQuery(item, model.PreviewPrometheusTemplateQuery{Keyword: "linux"}) {
		t.Fatal("expected linux keyword to match Linux/node_exporter item")
	}
	if !matchesPrometheusTemplateQuery(item, model.PreviewPrometheusTemplateQuery{Keyword: "node_exporter"}) {
		t.Fatal("expected node_exporter keyword to match Linux/node_exporter item")
	}
}

func TestMatchesPrometheusTemplateQueryDoesNotMatchKubernetesNodeForLinuxKeyword(t *testing.T) {
	item := modelPrometheusTemplateImportItem("Kubernetes", "kube-state-metrics")
	item.Name = "Kubernetes Node memory pressure"
	item.QueryTemplate = `kube_node_status_condition{condition="MemoryPressure",status="true"} == 1`

	if matchesPrometheusTemplateQuery(item, model.PreviewPrometheusTemplateQuery{Keyword: "linux"}) {
		t.Fatal("expected linux keyword not to match Kubernetes node item")
	}
	if matchesPrometheusTemplateQuery(item, model.PreviewPrometheusTemplateQuery{Keyword: "node_exporter"}) {
		t.Fatal("expected node_exporter keyword not to match Kubernetes node item")
	}
}

func TestMatchesPrometheusTemplateQuerySupportsKubernetesKeyword(t *testing.T) {
	item := modelPrometheusTemplateImportItem("Kubernetes", "kube-state-metrics")
	item.Name = "Kubernetes pod crash looping"
	item.QueryTemplate = `kube_pod_container_status_restarts_total > 3`

	if !matchesPrometheusTemplateQuery(item, model.PreviewPrometheusTemplateQuery{Keyword: "k8s"}) {
		t.Fatal("expected k8s keyword to match Kubernetes/kube-state-metrics item")
	}
	if !matchesPrometheusTemplateQuery(item, model.PreviewPrometheusTemplateQuery{Keyword: "kubernetes"}) {
		t.Fatal("expected kubernetes keyword to match Kubernetes/kube-state-metrics item")
	}
}

func TestMatchesPrometheusTemplateQueryDoesNotMatchCiliumForKubernetesKeyword(t *testing.T) {
	item := modelPrometheusTemplateImportItem("Cilium", "embedded-exporter")
	item.Name = "Cilium agent Kubernetes client errors"
	item.QueryTemplate = `sum(rate(cilium_k8s_client_api_calls_total[5m])) by (pod) > 0.05`

	if matchesPrometheusTemplateQuery(item, model.PreviewPrometheusTemplateQuery{Keyword: "k8s"}) {
		t.Fatal("expected k8s keyword not to match Cilium item")
	}
}

func TestActiveEventGroupVOsAggregatesEventsByRule(t *testing.T) {
	now := time.Date(2026, 6, 24, 14, 30, 0, 0, time.Local)
	service := &AlertManagerServiceImpl{}
	groups := service.activeEventGroupVOs([]model.AlertActiveEvent{
		activeEventForTest(1, "host-a", now.Add(-10*time.Minute), now.Add(-1*time.Minute)),
		activeEventForTest(2, "host-b", now.Add(-2*time.Minute), now),
	}, 20)

	if len(groups) != 1 {
		t.Fatalf("expected one group, got %d", len(groups))
	}
	group := groups[0]
	if group.ResourceCount != 2 || group.MemberTotal != 2 || group.FiringCount != 2 {
		t.Fatalf("unexpected group counts: %+v", group)
	}
	if len(group.AffectedResources) != 2 || group.AffectedResources[0] != "host-a" || group.AffectedResources[1] != "host-b" {
		t.Fatalf("unexpected resources: %+v", group.AffectedResources)
	}
}

func TestActiveEventGroupVOsLimitsMembers(t *testing.T) {
	now := time.Date(2026, 6, 24, 14, 30, 0, 0, time.Local)
	service := &AlertManagerServiceImpl{}
	groups := service.activeEventGroupVOs([]model.AlertActiveEvent{
		activeEventForTest(1, "host-a", now.Add(-10*time.Minute), now.Add(-1*time.Minute)),
		activeEventForTest(2, "host-b", now.Add(-2*time.Minute), now),
	}, 1)

	if len(groups) != 1 {
		t.Fatalf("expected one group, got %d", len(groups))
	}
	if len(groups[0].Members) != 1 || !groups[0].MembersLimited {
		t.Fatalf("expected limited members, got len=%d limited=%v", len(groups[0].Members), groups[0].MembersLimited)
	}
}

func TestActiveEventDisplayStatusShowsPendingAsObserving(t *testing.T) {
	status, name := activeEventDisplayStatus(model.AlertActiveEvent{Status: model.EventStatusPending})

	if status != model.EventDisplayStatusPending || name != "观察中" {
		t.Fatalf("unexpected pending display status: %s %s", status, name)
	}
}

func TestApplyRuleDurationCountConvertsCountToSeconds(t *testing.T) {
	rule := model.AlertRule{
		EvalIntervalSeconds: 60,
		ForDurationCount:    5,
		RuleConfig: model.JSONMap{
			"queries": []interface{}{
				map[string]interface{}{"prom_ql": "up == 0"},
			},
		},
	}

	applyRuleDefaults(&rule)

	if rule.ForDurationSeconds != 300 {
		t.Fatalf("ForDurationSeconds = %d, want 300", rule.ForDurationSeconds)
	}
	queries := ruleConfigQueries(rule.RuleConfig)
	if len(queries) != 1 || queries[0]["forDurationSeconds"] != int64(300) {
		t.Fatalf("unexpected query duration: %+v", queries)
	}
}

func TestDurationCountFromSecondsRoundsUp(t *testing.T) {
	if got := durationCountFromSeconds(301, 60); got != 6 {
		t.Fatalf("durationCountFromSeconds(301, 60) = %d, want 6", got)
	}
}

func TestApplyNotifyRuleDefaultsSetsNoiseReductionDefaults(t *testing.T) {
	rule := model.AlertNotifyRule{Name: "test notify rule"}

	applyNotifyRuleDefaults(&rule)

	if rule.RepeatNotify != 1 || rule.RepeatMode != "bySeverity" || rule.RepeatIntervalSeconds != 3600 {
		t.Fatalf("unexpected repeat defaults: %+v", rule)
	}
	if rule.RepeatIntervals["P0"] != int64(3600) || rule.RepeatIntervals["P1"] != int64(7200) || rule.RepeatIntervals["P2"] != int64(14400) {
		t.Fatalf("unexpected repeat intervals: %+v", rule.RepeatIntervals)
	}
	if rule.AggregationEnabled != 1 || rule.GroupWaitSeconds != 30 || rule.GroupIntervalSeconds != 300 || rule.AggregationWindowSeconds != 30 {
		t.Fatalf("unexpected aggregation defaults: %+v", rule)
	}
	if rule.RecoveryNotify != 1 {
		t.Fatalf("unexpected recovery notify default: %d", rule.RecoveryNotify)
	}
}

func TestNormalizeNotifyRuleForSaveKeepsExplicitDisabledNoiseReduction(t *testing.T) {
	rule := model.AlertNotifyRule{
		Name:                     "test notify rule",
		Enabled:                  0,
		RepeatNotify:             0,
		RepeatMode:               "severity",
		RepeatIntervalSeconds:    0,
		RepeatIntervals:          model.JSONMap{},
		AggregationEnabled:       0,
		GroupWaitSeconds:         0,
		GroupIntervalSeconds:     0,
		AggregationWindowSeconds: 0,
		RecoveryNotify:           1,
	}

	normalizeNotifyRuleForSave(&rule)

	if rule.Enabled != 0 || rule.RepeatNotify != 0 || rule.AggregationEnabled != 0 {
		t.Fatalf("explicit disabled fields were changed: %+v", rule)
	}
	if rule.RepeatMode != "bySeverity" {
		t.Fatalf("RepeatMode = %q, want bySeverity", rule.RepeatMode)
	}
	if len(rule.RepeatIntervals) != 0 {
		t.Fatalf("RepeatIntervals = %+v, want empty map", rule.RepeatIntervals)
	}
	if rule.GroupWaitSeconds != 0 || rule.GroupIntervalSeconds != 0 || rule.AggregationWindowSeconds != 0 {
		t.Fatalf("explicit zero aggregation fields were changed: %+v", rule)
	}
}

func TestDescendantRuleGroupIDsIncludesChildren(t *testing.T) {
	ids := descendantRuleGroupIDs([]model.AlertRuleGroup{
		{ID: 1, ParentID: 0},
		{ID: 2, ParentID: 1},
		{ID: 3, ParentID: 2},
		{ID: 4, ParentID: 0},
	}, 1)

	expected := []uint{1, 2, 3}
	if len(ids) != len(expected) {
		t.Fatalf("unexpected ids length: %+v", ids)
	}
	for i := range expected {
		if ids[i] != expected[i] {
			t.Fatalf("ids[%d] = %d, want %d; all ids=%+v", i, ids[i], expected[i], ids)
		}
	}
}

func TestFeishuCardActionReadsSnakeCaseEventID(t *testing.T) {
	action := map[string]interface{}{
		"action":      "claim",
		"event_id":    "45037",
		"fingerprint": "7961191cd44d5ebfe002b1e933a00991fb4fe628",
	}

	if got := getAnyString(action, "eventId", "event_id", "eventID"); got != "45037" {
		t.Fatalf("event id = %q, want 45037", got)
	}
	if got := getAnyString(action, "action", "action_type", "actionType"); got != "claim" {
		t.Fatalf("action = %q, want claim", got)
	}
}

func TestParseFeishuCardEventIDs(t *testing.T) {
	ids := parseEventIDs("45037,45038", "45037")
	expected := []uint64{45037, 45038}
	if len(ids) != len(expected) {
		t.Fatalf("ids = %+v, want %+v", ids, expected)
	}
	for i := range expected {
		if ids[i] != expected[i] {
			t.Fatalf("ids[%d] = %d, want %d; ids=%+v", i, ids[i], expected[i], ids)
		}
	}
}

func TestNormalizeNotifyRouteRuleKeepsConditionPairs(t *testing.T) {
	item := model.AlertNotifyRouteRule{Conditions: model.AlertNotifyRouteConditions{
		{LabelKey: "service", MatchMode: model.NotifyRouteMatchExact, MatchValue: "bigdata"},
		{LabelKey: "app", MatchMode: model.NotifyRouteMatchExact, MatchValue: "data-platform"},
	}}
	normalizeNotifyRouteRule(&item)
	if len(item.Conditions) != 2 || item.Conditions[1].LabelKey != "app" || item.Conditions[1].MatchValue != "data-platform" {
		t.Fatalf("conditions = %+v", item.Conditions)
	}
}

func modelPrometheusTemplateImportItem(component, collector string) model.PrometheusTemplateImportItem {
	return model.PrometheusTemplateImportItem{
		Name:          "Host out of memory",
		Description:   "Node memory is filling up",
		QueryTemplate: "node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes < .10",
		Component:     component,
		Collector:     collector,
		Category:      "memory",
	}
}

func activeEventForTest(id uint64, hostname string, firstTriggeredAt, lastEvaluatedAt time.Time) model.AlertActiveEvent {
	return model.AlertActiveEvent{
		ID:               id,
		Fingerprint:      hostname + "-fingerprint",
		RuleID:           53,
		RuleName:         "内存使用率大于 > 80%",
		DatasourceID:     1,
		DatasourceType:   "builtin",
		Severity:         "info",
		Status:           model.EventStatusFiring,
		Labels:           model.JSONMap{"hostname": hostname, "level": "P2"},
		Value:            "86",
		FirstTriggeredAt: util.HTime{Time: firstTriggeredAt},
		LastEvaluatedAt:  util.HTime{Time: lastEvaluatedAt},
	}
}
