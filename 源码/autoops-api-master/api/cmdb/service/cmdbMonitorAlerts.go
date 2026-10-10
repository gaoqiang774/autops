package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	alertdao "dodevops-api/api/alertmanager/dao"
	alertmodel "dodevops-api/api/alertmanager/model"
	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"

	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prommodel "github.com/prometheus/common/model"
)

const (
	businessMonitorAlertPropertyKey = "monitorAlert"
	maxBusinessMonitorAlertEvents   = 1000
	maxBusinessMonitorMetricRules   = 8
	maxBusinessMonitorRuleScan      = 30
	maxBusinessMonitorMetricSamples = 20
)

var (
	ErrInvalidBusinessMonitorAlertConfig = errors.New("无效的业务监控告警配置")
	selectorMatcherPattern               = regexp.MustCompile(`^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*(=~|!~|=|!=)\s*"(.*)"\s*$`)
	promQLComparePattern                 = regexp.MustCompile(`\s(>=|<=|==|!=|>|<|=)\s*(-?\d+(?:\.\d+)?)\s*$`)
)

type monitorAlertMatcher struct {
	Label string
	Op    string
	Value string
	Regex *regexp.Regexp
}

type monitorAlertRule struct {
	Name        string
	Selector    string
	Enabled     bool
	Description string
	Matchers    []monitorAlertMatcher
}

type businessMonitorRuleQuery struct {
	Query         string
	Level         string
	Severity      string
	Operator      string
	Threshold     float64
	HasCompare    bool
	WindowSeconds int64
}

type businessMonitorResourceMetricQuery struct {
	MetricKey         string
	MetricName        string
	Category          string
	Selector          string
	Query             string
	Unit              string
	WarningThreshold  float64
	CriticalThreshold float64
	HigherIsBad       bool
	ThresholdSource   string
}

// GetBusinessMonitorAlertConfig returns saved alert source and matching rules with selectable datasources.
func (s *CmdbService) GetBusinessMonitorAlertConfig(businessID uint) (map[string]interface{}, error) {
	business, err := s.dao.GetBusinessModelByID(businessID)
	if err != nil {
		return nil, err
	}
	config := readBusinessMonitorAlertConfig(business)
	datasources, err := s.enabledMonitorAlertDatasources()
	if err != nil {
		return nil, err
	}
	topology, _ := s.getBusinessModelTopology(businessID, 8, "downstream", false)
	return map[string]interface{}{
		"businessId":       businessID,
		"config":           businessMonitorAlertConfigPayload(config),
		"datasources":      datasources,
		"recommendedRules": buildBusinessMonitorAlertRecommendedRules(business, topology),
	}, nil
}

// SaveBusinessMonitorAlertConfig stores the business alert source and user-defined label selectors.
func (s *CmdbService) SaveBusinessMonitorAlertConfig(businessID uint, req cmdbmodel.BusinessMonitorAlertConfigDto) (map[string]interface{}, error) {
	business, err := s.dao.GetBusinessModelByID(businessID)
	if err != nil {
		return nil, err
	}
	config, err := s.normalizeBusinessMonitorAlertConfig(req)
	if err != nil {
		return nil, err
	}
	props := jsonToMap(business.Properties)
	props[businessMonitorAlertPropertyKey] = businessMonitorAlertConfigPayload(config)
	propsJSON, err := cmdbdao.JSONMapToDatatype(props)
	if err != nil {
		return nil, err
	}
	business.Properties = propsJSON
	if err := s.dao.UpdateBusinessModel(business); err != nil {
		return nil, err
	}
	return s.GetBusinessMonitorAlertConfig(businessID)
}

// GetBusinessMonitorAlerts returns active alerts matched by the business monitor alert config.
func (s *CmdbService) GetBusinessMonitorAlerts(businessID uint, query cmdbmodel.BusinessMonitorAlertQuery) (map[string]interface{}, error) {
	business, err := s.dao.GetBusinessModelByID(businessID)
	if err != nil {
		return nil, err
	}
	config := readBusinessMonitorAlertConfig(business)
	topology, err := s.getBusinessModelTopology(businessID, 8, "downstream", false)
	if err != nil {
		return nil, err
	}
	datasource, datasourceWarning, err := s.resolveBusinessMonitorAlertDatasource(config, query.DatasourceID)
	if err != nil {
		return nil, err
	}
	warnings := make([]string, 0)
	if datasourceWarning != "" {
		warnings = append(warnings, datasourceWarning)
	}

	rules, ruleWarnings := compileBusinessMonitorAlertRules(config.MatchRules)
	warnings = append(warnings, ruleWarnings...)
	enabled := businessMonitorAlertEnabled(config)
	dataStatus := "ready"
	if !enabled {
		dataStatus = "disabled"
	}
	if enabled && datasource == nil {
		dataStatus = "unconfigured"
	}
	if enabled && datasource != nil && len(rules) == 0 {
		dataStatus = "unconfigured"
		warnings = append(warnings, "未配置可用的告警匹配规则")
	}

	var matchedEvents []map[string]interface{}
	var nodeAlerts map[uint][]map[string]interface{}
	metricCoverage := emptyBusinessMonitorMetricCoverage()
	currentMetrics := []map[string]interface{}{}
	currentMetricSummary := map[string]interface{}{
		"total":       0,
		"firing":      0,
		"warning":     0,
		"critical":    0,
		"healthy":     0,
		"noData":      0,
		"queryFailed": 0,
	}
	if dataStatus == "ready" {
		nodeIndex := buildBusinessMonitorAlertNodeIndex(topology)
		coverage, coverageWarnings := queryBusinessMonitorMetricCoverage(datasource, rules, nodeIndex)
		metricCoverage = coverage
		warnings = append(warnings, coverageWarnings...)
		currentMetrics, currentMetricSummary, warnings = s.queryBusinessMonitorCurrentMetrics(business, config, datasource, topology, nodeIndex, warnings)

		alertDao := alertdao.NewAlertManagerDaoWithDB(s.dao.DB())
		eventQuery := alertmodel.AlertEventQuery{
			DatasourceID:   datasource.ID,
			DatasourceType: datasource.DatasourceType,
			Severity:       strings.TrimSpace(query.Severity),
			Status:         strings.TrimSpace(query.Status),
		}
		events, err := alertDao.ListActiveEventsForGroups(eventQuery)
		if err != nil {
			return nil, err
		}
		matchedEvents, nodeAlerts = matchBusinessMonitorAlertEvents(business, events, rules, nodeIndex)
		if len(matchedEvents) == 0 {
			if uintFromAny(metricCoverage["totalSeries"]) == 0 {
				dataStatus = "no_metric_matches"
			} else {
				dataStatus = "no_alerts"
			}
		}
	} else {
		nodeAlerts = map[uint][]map[string]interface{}{}
	}

	page, pageSize := normalizeBusinessMonitorAlertPage(query.Page, query.PageSize)
	pagedEvents := paginateMonitorAlertEvents(matchedEvents, page, pageSize)
	alertNodes := buildBusinessMonitorAlertNodePayload(topology, nodeAlerts)
	metricNodes := buildBusinessMonitorMetricNodePayload(topology, currentMetrics)
	nodes := mergeBusinessMonitorNodePayloads(alertNodes, metricNodes)
	if len(matchedEvents) == 0 && uintFromAny(currentMetricSummary["critical"]) > 0 {
		dataStatus = "metric_critical"
	} else if len(matchedEvents) == 0 && uintFromAny(currentMetricSummary["warning"]) > 0 {
		dataStatus = "metric_warning"
	}
	summary := summarizeBusinessMonitorAlerts(matchedEvents, nodes)
	summary = enrichBusinessMonitorSummaryWithCurrentMetrics(summary, currentMetricSummary)
	coverageNodes := businessMonitorCoverageNodes(metricCoverage)
	diagnosis := buildBusinessMonitorAlertDiagnosis(dataStatus, summary, metricCoverage, currentMetricSummary)
	pressureOverview := buildBusinessMonitorPressureOverview(dataStatus, currentMetrics, metricNodes, currentMetricSummary)
	coverageLimited := boolFromAny(metricCoverage["limited"])
	if coverageLimited {
		warnings = append(warnings, "指标覆盖样例已截断，仅返回部分 series 用于展示")
	}

	return map[string]interface{}{
		"businessId":           businessID,
		"config":               businessMonitorAlertConfigPayload(config),
		"datasource":           monitorAlertDatasourcePayload(datasource),
		"matchRules":           businessMonitorAlertRulePayloads(rules),
		"summary":              summary,
		"diagnosis":            diagnosis,
		"pressureOverview":     pressureOverview,
		"activeAlerts":         pagedEvents,
		"nodes":                nodes,
		"alertNodes":           alertNodes,
		"metricNodes":          metricNodes,
		"coverageNodes":        coverageNodes,
		"metricCoverage":       metricCoverage,
		"currentMetrics":       currentMetrics,
		"currentMetricSummary": currentMetricSummary,
		"pagination": map[string]interface{}{
			"page":     page,
			"pageSize": pageSize,
			"total":    len(matchedEvents),
		},
		"dataStatus":  dataStatus,
		"partial":     len(warnings) > 0 || coverageLimited,
		"warnings":    uniqueStrings(warnings),
		"generatedAt": time.Now().Format(time.RFC3339Nano),
	}, nil
}

func (s *CmdbService) normalizeBusinessMonitorAlertConfig(req cmdbmodel.BusinessMonitorAlertConfigDto) (cmdbmodel.BusinessMonitorAlertConfigDto, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	config := cmdbmodel.BusinessMonitorAlertConfigDto{
		Enabled:        boolPtr(enabled),
		DatasourceID:   req.DatasourceID,
		DatasourceType: strings.TrimSpace(strings.ToLower(req.DatasourceType)),
		MatchRules:     make([]cmdbmodel.BusinessMonitorAlertMatchRuleDto, 0, len(req.MatchRules)),
	}
	if config.DatasourceID > 0 {
		ds, err := alertdao.NewAlertManagerDaoWithDB(s.dao.DB()).GetDatasource(config.DatasourceID)
		if err != nil {
			return config, fmt.Errorf("%w: 告警源不存在", ErrInvalidBusinessMonitorAlertConfig)
		}
		if ds.Enabled != alertmodel.RuleStatusEnabled {
			return config, fmt.Errorf("%w: 告警源未启用", ErrInvalidBusinessMonitorAlertConfig)
		}
		config.DatasourceType = ds.DatasourceType
	}
	seen := make(map[string]struct{})
	for i, item := range req.MatchRules {
		selector := normalizeBusinessMonitorAlertSelector(item.Selector)
		if selector == "" {
			continue
		}
		if _, err := parseMonitorAlertSelector(selector); err != nil {
			return config, fmt.Errorf("%w: 第 %d 条匹配规则格式错误: %v", ErrInvalidBusinessMonitorAlertConfig, i+1, err)
		}
		if _, exists := seen[selector]; exists {
			continue
		}
		seen[selector] = struct{}{}
		ruleEnabled := true
		if item.Enabled != nil {
			ruleEnabled = *item.Enabled
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = fmt.Sprintf("规则%d", len(config.MatchRules)+1)
		}
		config.MatchRules = append(config.MatchRules, cmdbmodel.BusinessMonitorAlertMatchRuleDto{
			Name:        name,
			Selector:    selector,
			Enabled:     boolPtr(ruleEnabled),
			Description: strings.TrimSpace(item.Description),
		})
	}
	return config, nil
}

func (s *CmdbService) resolveBusinessMonitorAlertDatasource(config cmdbmodel.BusinessMonitorAlertConfigDto, overrideID uint) (*alertmodel.AlertDatasource, string, error) {
	datasourceID := config.DatasourceID
	if overrideID > 0 {
		datasourceID = overrideID
	}
	if datasourceID == 0 {
		return nil, "", nil
	}
	ds, err := alertdao.NewAlertManagerDaoWithDB(s.dao.DB()).GetDatasource(datasourceID)
	if err != nil {
		return nil, "", err
	}
	if ds.Enabled != alertmodel.RuleStatusEnabled {
		return ds, "当前绑定的告警源未启用", nil
	}
	return ds, "", nil
}

func (s *CmdbService) enabledMonitorAlertDatasources() ([]map[string]interface{}, error) {
	list, _, err := alertdao.NewAlertManagerDaoWithDB(s.dao.DB()).ListDatasources(alertmodel.AlertDatasourceQuery{
		PageQuery: alertmodel.PageQuery{Page: 1, PageSize: 100},
		Enabled:   strconv.Itoa(alertmodel.RuleStatusEnabled),
	})
	if err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, 0, len(list))
	for i := range list {
		result = append(result, monitorAlertDatasourcePayload(&list[i]))
	}
	return result, nil
}

func readBusinessMonitorAlertConfig(business *cmdbmodel.CmdbBusinessModel) cmdbmodel.BusinessMonitorAlertConfigDto {
	enabled := true
	config := cmdbmodel.BusinessMonitorAlertConfigDto{Enabled: &enabled}
	if business == nil {
		return config
	}
	raw, ok := jsonToMap(business.Properties)[businessMonitorAlertPropertyKey]
	if !ok || raw == nil {
		return config
	}
	bytes, err := json.Marshal(raw)
	if err != nil {
		return config
	}
	if err := json.Unmarshal(bytes, &config); err != nil {
		return cmdbmodel.BusinessMonitorAlertConfigDto{Enabled: &enabled}
	}
	if config.Enabled == nil {
		config.Enabled = &enabled
	}
	for i := range config.MatchRules {
		if config.MatchRules[i].Enabled == nil {
			config.MatchRules[i].Enabled = &enabled
		}
	}
	return config
}

func compileBusinessMonitorAlertRules(items []cmdbmodel.BusinessMonitorAlertMatchRuleDto) ([]monitorAlertRule, []string) {
	rules := make([]monitorAlertRule, 0, len(items))
	warnings := make([]string, 0)
	for i, item := range items {
		if item.Enabled != nil && !*item.Enabled {
			continue
		}
		selector := normalizeBusinessMonitorAlertSelector(item.Selector)
		if selector == "" {
			continue
		}
		matchers, err := parseMonitorAlertSelector(selector)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("第 %d 条告警匹配规则已忽略: %v", i+1, err))
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = fmt.Sprintf("规则%d", len(rules)+1)
		}
		rules = append(rules, monitorAlertRule{
			Name:        name,
			Selector:    selector,
			Enabled:     true,
			Description: strings.TrimSpace(item.Description),
			Matchers:    matchers,
		})
	}
	return rules, warnings
}

func queryBusinessMonitorMetricCoverage(datasource *alertmodel.AlertDatasource, rules []monitorAlertRule, nodeIndex []map[string]interface{}) (map[string]interface{}, []string) {
	coverage := emptyBusinessMonitorMetricCoverage()
	warnings := make([]string, 0)
	if datasource == nil || len(rules) == 0 {
		return coverage, warnings
	}
	if !isBusinessMonitorPromQLDatasource(datasource.DatasourceType) {
		warnings = append(warnings, "当前告警源不是 Prometheus/VictoriaMetrics，已跳过指标覆盖检查")
		return coverage, warnings
	}
	promAPI, err := newBusinessMonitorPromAPI(*datasource)
	if err != nil {
		warnings = append(warnings, "监控指标查询客户端创建失败: "+err.Error())
		return coverage, warnings
	}

	queryRules := rules
	if len(queryRules) > maxBusinessMonitorMetricRules {
		queryRules = queryRules[:maxBusinessMonitorMetricRules]
		warnings = append(warnings, fmt.Sprintf("匹配规则共 %d 条，指标覆盖检查最多查询前 %d 条", len(rules), maxBusinessMonitorMetricRules))
	}

	rulePayloads := make([]map[string]interface{}, 0, len(queryRules))
	totalSeries := 0
	limited := false
	affectedNodeIDs := map[uint]struct{}{}
	for _, rule := range queryRules {
		item := queryBusinessMonitorMetricRule(promAPI, rule, nodeIndex)
		rulePayloads = append(rulePayloads, item)
		totalSeries += intFromAny(item["total"])
		if boolFromAny(item["limited"]) {
			limited = true
		}
		if warning := stringFromAny(item["warning"]); warning != "" {
			warnings = append(warnings, warning)
		}
		for _, node := range mapSliceFromAny(item["matchedNodes"]) {
			if nodeID := uintFromAny(node["resourceId"]); nodeID > 0 {
				affectedNodeIDs[nodeID] = struct{}{}
			}
		}
	}
	coverage["totalSeries"] = totalSeries
	coverage["limited"] = limited
	coverage["rules"] = rulePayloads
	coverage["matchedNodeCount"] = len(affectedNodeIDs)
	coverage["matchedNodeIds"] = sortedUintSet(affectedNodeIDs)
	return coverage, warnings
}

func (s *CmdbService) queryBusinessMonitorCurrentMetrics(business *cmdbmodel.CmdbBusinessModel, config cmdbmodel.BusinessMonitorAlertConfigDto, datasource *alertmodel.AlertDatasource, topology map[string]interface{}, nodeIndex []map[string]interface{}, warnings []string) ([]map[string]interface{}, map[string]interface{}, []string) {
	summary := map[string]interface{}{
		"total":       0,
		"firing":      0,
		"warning":     0,
		"critical":    0,
		"healthy":     0,
		"noData":      0,
		"queryFailed": 0,
	}
	if datasource == nil || !isBusinessMonitorPromQLDatasource(datasource.DatasourceType) {
		return []map[string]interface{}{}, summary, warnings
	}

	promAPI, err := newBusinessMonitorPromAPI(*datasource)
	if err != nil {
		warnings = append(warnings, "监控指标查询客户端创建失败: "+err.Error())
		return []map[string]interface{}{}, summary, warnings
	}

	queries := buildBusinessMonitorResourceMetricQueries(config)
	if len(queries) == 0 {
		return []map[string]interface{}{}, summary, warnings
	}
	results := make([]map[string]interface{}, 0, len(queries))
	for _, query := range queries {
		payload, warn := queryBusinessMonitorResourceMetric(promAPI, query, nodeIndex)
		if warn != "" {
			warnings = append(warnings, warn)
		}
		if payload == nil {
			continue
		}
		results = append(results, payload)
		switch stringFromAny(payload["status"]) {
		case "critical":
			summary["critical"] = uintFromAny(summary["critical"]) + 1
			summary["firing"] = uintFromAny(summary["firing"]) + 1
		case "warning":
			summary["warning"] = uintFromAny(summary["warning"]) + 1
			summary["firing"] = uintFromAny(summary["firing"]) + 1
		case "firing":
			summary["firing"] = uintFromAny(summary["firing"]) + 1
		case "query_failed":
			summary["queryFailed"] = uintFromAny(summary["queryFailed"]) + 1
		case "no_data":
			summary["noData"] = uintFromAny(summary["noData"]) + 1
		default:
			summary["healthy"] = uintFromAny(summary["healthy"]) + 1
		}
	}
	summary["total"] = len(results)
	return results, summary, warnings
}

func buildBusinessMonitorResourceMetricQueries(config cmdbmodel.BusinessMonitorAlertConfigDto) []businessMonitorResourceMetricQuery {
	result := make([]businessMonitorResourceMetricQuery, 0)
	seen := make(map[string]struct{})
	for _, rule := range config.MatchRules {
		if rule.Enabled != nil && !*rule.Enabled {
			continue
		}
		selector := normalizeBusinessMonitorAlertSelector(rule.Selector)
		body := businessMonitorSelectorBody(selector)
		if body == "" {
			continue
		}
		kubeStateBody := rewriteBusinessMonitorSelectorLabel(body, "container_name", "container")
		items := []businessMonitorResourceMetricQuery{
			{
				MetricKey:         "k8s.container.cpu_usage_cores",
				MetricName:        "容器 CPU 使用量",
				Category:          "cpu",
				Selector:          selector,
				Query:             fmt.Sprintf(`sum by (namespace,pod,pod_name,container,container_name,instance,node) (rate(container_cpu_usage_seconds_total{%s}[5m]))`, body),
				Unit:              "cores",
				WarningThreshold:  1,
				CriticalThreshold: 2,
				HigherIsBad:       true,
				ThresholdSource:   "default",
			},
			{
				MetricKey:         "k8s.container.memory_usage_percent",
				MetricName:        "容器内存使用率",
				Category:          "memory",
				Selector:          selector,
				Query:             fmt.Sprintf(`(max by (namespace,pod,pod_name,container,container_name,instance,node) (container_memory_working_set_bytes{%s}) / max by (namespace,pod,pod_name,container,container_name,instance,node) (container_spec_memory_limit_bytes{%s} > 0)) * 100`, body, body),
				Unit:              "%",
				WarningThreshold:  80,
				CriticalThreshold: 90,
				HigherIsBad:       true,
				ThresholdSource:   "default",
			},
			{
				MetricKey:         "k8s.container.cpu_throttle_percent",
				MetricName:        "CPU 限流比例",
				Category:          "cpu",
				Selector:          selector,
				Query:             fmt.Sprintf(`(sum by (namespace,pod,pod_name,container,container_name,instance,node) (rate(container_cpu_cfs_throttled_periods_total{%s}[5m])) / sum by (namespace,pod,pod_name,container,container_name,instance,node) (rate(container_cpu_cfs_periods_total{%s}[5m]))) * 100`, body, body),
				Unit:              "%",
				WarningThreshold:  20,
				CriticalThreshold: 50,
				HigherIsBad:       true,
				ThresholdSource:   "default",
			},
			{
				MetricKey:         "k8s.pod.restart_count_15m",
				MetricName:        "Pod 重启次数",
				Category:          "restart",
				Selector:          selector,
				Query:             fmt.Sprintf(`sum by (namespace,pod,container) (increase(kube_pod_container_status_restarts_total{%s}[15m]))`, kubeStateBody),
				Unit:              "count",
				WarningThreshold:  1,
				CriticalThreshold: 3,
				HigherIsBad:       true,
				ThresholdSource:   "default",
			},
		}
		for _, item := range items {
			key := item.MetricKey + "|" + item.Query
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

func queryBusinessMonitorResourceMetric(promAPI promv1.API, query businessMonitorResourceMetricQuery, nodeIndex []map[string]interface{}) (map[string]interface{}, string) {
	payload := map[string]interface{}{
		"metricKey":         query.MetricKey,
		"metricName":        query.MetricName,
		"category":          query.Category,
		"selector":          query.Selector,
		"query":             query.Query,
		"unit":              query.Unit,
		"warningThreshold":  query.WarningThreshold,
		"criticalThreshold": query.CriticalThreshold,
		"thresholdSource":   query.ThresholdSource,
		"status":            "no_data",
		"currentValue":      nil,
		"sampleCount":       0,
		"samples":           []map[string]interface{}{},
		"matchedNodes":      []map[string]interface{}{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	value, promWarnings, err := promAPI.Query(ctx, query.Query, time.Now())
	if err != nil {
		payload["status"] = "query_failed"
		payload["error"] = err.Error()
		return payload, fmt.Sprintf("%s 查询失败: %v", query.MetricName, err)
	}
	warning := ""
	if len(promWarnings) > 0 {
		warning = strings.Join(businessMonitorPrometheusWarnings(promWarnings), "; ")
		payload["warning"] = warning
	}
	samples, total, limited := businessMonitorResourceMetricSamples(value, maxBusinessMonitorMetricSamples, nodeIndex, query)
	payload["sampleCount"] = total
	payload["limited"] = limited
	payload["samples"] = samples
	payload["matchedNodes"] = collectBusinessMonitorMetricMatchedNodes(samples)
	current, ok := businessMonitorResourceCurrentValue(samples)
	if !ok {
		return payload, warning
	}
	payload["currentValue"] = current
	payload["status"] = businessMonitorResourceMetricStatus(current, query)
	payload["pressure"] = businessMonitorResourcePressure(current, query)
	return payload, warning
}

func businessMonitorSelectorBody(selector string) string {
	selector = normalizeBusinessMonitorAlertSelector(selector)
	if !strings.HasPrefix(selector, "{") || !strings.HasSuffix(selector, "}") {
		return ""
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(selector, "{"), "}"))
}

func rewriteBusinessMonitorSelectorLabel(body, from, to string) string {
	if strings.TrimSpace(body) == "" || from == to {
		return body
	}
	pattern := regexp.MustCompile(`(^|,\s*)` + regexp.QuoteMeta(from) + `\s*(=~|!~|=|!=)`)
	return pattern.ReplaceAllString(body, `${1}`+to+`${2}`)
}

func businessMonitorResourceMetricSamples(value prommodel.Value, limit int, nodeIndex []map[string]interface{}, query businessMonitorResourceMetricQuery) ([]map[string]interface{}, int, bool) {
	if limit <= 0 {
		limit = maxBusinessMonitorMetricSamples
	}
	vector, ok := value.(prommodel.Vector)
	if !ok || len(vector) == 0 {
		return []map[string]interface{}{}, 0, false
	}
	total := len(vector)
	limited := total > limit
	if limited {
		vector = vector[:limit]
	}
	result := make([]map[string]interface{}, 0, len(vector))
	for _, sample := range vector {
		if sample == nil {
			continue
		}
		labels := prometheusMetricToMap(sample.Metric)
		current := float64(sample.Value)
		result = append(result, map[string]interface{}{
			"metric":       businessMonitorMetricDisplayLabels(labels),
			"value":        current,
			"unit":         query.Unit,
			"status":       businessMonitorResourceMetricStatus(current, query),
			"timestamp":    sample.Timestamp.Time().Format(time.RFC3339Nano),
			"matchedNodes": matchBusinessMonitorAlertNodes(alertmodel.JSONMap(labels), nodeIndex),
		})
	}
	return result, total, limited
}

func businessMonitorResourceCurrentValue(samples []map[string]interface{}) (float64, bool) {
	current := 0.0
	ok := false
	for _, sample := range samples {
		value, exists := firstBusinessMonitorFloat(sample, "value")
		if !exists || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		if !ok || value > current {
			current = value
			ok = true
		}
	}
	return current, ok
}

func businessMonitorResourceMetricStatus(value float64, query businessMonitorResourceMetricQuery) string {
	if query.HigherIsBad {
		switch {
		case value >= query.CriticalThreshold:
			return "critical"
		case value >= query.WarningThreshold:
			return "warning"
		default:
			return "healthy"
		}
	}
	switch {
	case value <= query.CriticalThreshold:
		return "critical"
	case value <= query.WarningThreshold:
		return "warning"
	default:
		return "healthy"
	}
}

func businessMonitorResourcePressure(value float64, query businessMonitorResourceMetricQuery) map[string]interface{} {
	percentOfWarning := 0.0
	percentOfCritical := 0.0
	if query.WarningThreshold != 0 {
		percentOfWarning = value / query.WarningThreshold * 100
	}
	if query.CriticalThreshold != 0 {
		percentOfCritical = value / query.CriticalThreshold * 100
	}
	return map[string]interface{}{
		"percentOfWarning":  roundBusinessMonitorFloat(percentOfWarning),
		"percentOfCritical": roundBusinessMonitorFloat(percentOfCritical),
		"warningLeft":       roundBusinessMonitorFloat(query.WarningThreshold - value),
		"criticalLeft":      roundBusinessMonitorFloat(query.CriticalThreshold - value),
	}
}

func roundBusinessMonitorFloat(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*100) / 100
}

func businessMonitorRuleMaybeRelevant(rule alertmodel.AlertRule, business *cmdbmodel.CmdbBusinessModel, config cmdbmodel.BusinessMonitorAlertConfigDto) bool {
	if business != nil {
		businessIDs := []string{
			strconv.FormatUint(uint64(business.ID), 10),
			business.Name,
			business.DisplayName,
			business.BizCode,
		}
		for _, candidate := range businessIDs {
			candidate = strings.TrimSpace(candidate)
			if candidate != "" && strings.EqualFold(strings.TrimSpace(rule.BusinessID), candidate) {
				return true
			}
		}
		for _, value := range businessIDs {
			value = strings.TrimSpace(value)
			if value != "" && strings.Contains(strings.ToLower(ruleSearchText(rule)), strings.ToLower(value)) {
				return true
			}
		}
	}
	for _, item := range config.MatchRules {
		if item.Enabled != nil && !*item.Enabled {
			continue
		}
		selector := strings.TrimSpace(item.Selector)
		if selector == "" {
			continue
		}
		if strings.Contains(rule.Query, selector) || strings.Contains(fmt.Sprint(rule.RuleConfig), selector) {
			return true
		}
		for _, matcher := range mustParseBusinessMonitorRuleMatchers(selector) {
			if matcher.Value != "" && strings.Contains(ruleSearchText(rule), matcher.Value) {
				return true
			}
		}
	}
	return false
}

func ruleSearchText(rule alertmodel.AlertRule) string {
	return strings.Join([]string{
		rule.Name,
		rule.Note,
		rule.Query,
		fmt.Sprint(rule.RuleConfig),
		fmt.Sprint(rule.Labels),
		fmt.Sprint(rule.Annotations),
		rule.BusinessID,
	}, " ")
}

func mustParseBusinessMonitorRuleMatchers(selector string) []monitorAlertMatcher {
	matchers, err := parseMonitorAlertSelector(selector)
	if err != nil {
		return nil
	}
	return matchers
}

func businessMonitorExtractRuleQueries(rule alertmodel.AlertRule) []businessMonitorRuleQuery {
	queries := make([]businessMonitorRuleQuery, 0)
	for _, item := range businessMonitorRuleConfigQueries(rule.RuleConfig) {
		query := businessMonitorRuleQuery{
			Query:    firstBusinessMonitorString(item, "prom_ql", "promQl", "promql", "query"),
			Level:    firstBusinessMonitorString(item, "level", "priority"),
			Severity: firstBusinessMonitorString(item, "severity"),
			Operator: firstBusinessMonitorString(item, "conditionOperator", "operator"),
		}
		if query.Severity == "" {
			query.Severity = rule.Severity
		}
		if threshold, ok := firstBusinessMonitorFloat(item, "conditionValue", "threshold", "value"); ok {
			query.Threshold = threshold
			query.HasCompare = strings.TrimSpace(query.Operator) != ""
		}
		applyBusinessMonitorPromQLCompare(&query)
		query.WindowSeconds = firstBusinessMonitorDurationSeconds(item, "windowSeconds", "lookbackSeconds", "rangeSeconds", "durationSeconds", "window", "lookback", "range")
		if strings.TrimSpace(query.Query) != "" {
			queries = append(queries, query)
		}
	}
	if len(queries) == 0 && strings.TrimSpace(rule.Query) != "" {
		queries = append(queries, businessMonitorRuleQuery{
			Query:      strings.TrimSpace(rule.Query),
			Severity:   rule.Severity,
			Operator:   strings.TrimSpace(rule.ConditionOperator),
			Threshold:  rule.ConditionValue,
			HasCompare: strings.TrimSpace(rule.ConditionOperator) != "",
		})
		applyBusinessMonitorPromQLCompare(&queries[len(queries)-1])
	}
	return queries
}

func queryBusinessMonitorCurrentMetricRule(promAPI promv1.API, rule alertmodel.AlertRule, query businessMonitorRuleQuery, nodeIndex []map[string]interface{}) (map[string]interface{}, string) {
	payload := map[string]interface{}{
		"ruleId":        rule.ID,
		"ruleName":      rule.Name,
		"severity":      firstNonEmpty(query.Severity, rule.Severity),
		"query":         query.Query,
		"operator":      strings.TrimSpace(query.Operator),
		"threshold":     nil,
		"thresholdText": "",
		"windowSeconds": query.WindowSeconds,
		"status":        "no_data",
		"currentValue":  nil,
		"sampleCount":   0,
		"samples":       []map[string]interface{}{},
		"matchedNodes":  []map[string]interface{}{},
	}
	if query.HasCompare {
		payload["threshold"] = query.Threshold
		payload["thresholdText"] = businessMonitorThresholdText(query.Operator, query.Threshold)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	value, promWarnings, err := promAPI.Query(ctx, query.Query, time.Now())
	if err != nil {
		payload["status"] = "query_failed"
		payload["error"] = err.Error()
		return payload, fmt.Sprintf("规则 %s 当前值查询失败: %v", rule.Name, err)
	}
	warning := ""
	if len(promWarnings) > 0 {
		warning = strings.Join(businessMonitorPrometheusWarnings(promWarnings), "; ")
		payload["warning"] = warning
	}

	samples, total, limited := businessMonitorCurrentMetricSamples(value, maxBusinessMonitorMetricSamples, nodeIndex, query)
	payload["sampleCount"] = total
	payload["limited"] = limited
	payload["samples"] = samples
	payload["matchedNodes"] = collectBusinessMonitorMetricMatchedNodes(samples)
	current, ok := businessMonitorCurrentValue(samples, query)
	if !ok {
		payload["status"] = "no_data"
		return payload, warning
	}
	payload["currentValue"] = current
	if query.HasCompare && businessMonitorCompare(current, query.Operator, query.Threshold) {
		payload["status"] = "firing"
	} else {
		payload["status"] = "healthy"
	}
	if query.HasCompare {
		payload["distance"] = businessMonitorThresholdDistance(current, query.Operator, query.Threshold)
	}
	return payload, warning
}

func businessMonitorRuleConfigQueries(ruleConfig alertmodel.JSONMap) []map[string]interface{} {
	if ruleConfig == nil {
		return nil
	}
	rawQueries, ok := ruleConfig["queries"]
	if !ok || rawQueries == nil {
		return nil
	}
	switch typed := rawQueries.(type) {
	case []interface{}:
		result := make([]map[string]interface{}, 0, len(typed))
		for _, raw := range typed {
			if item, ok := raw.(map[string]interface{}); ok {
				result = append(result, item)
			}
		}
		return result
	case []map[string]interface{}:
		return typed
	default:
		return nil
	}
}

func firstBusinessMonitorString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(stringFromAny(values[key])); value != "" {
			return value
		}
	}
	return ""
}

func firstBusinessMonitorFloat(values map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case float64:
			return value, true
		case float32:
			return float64(value), true
		case int:
			return float64(value), true
		case int8:
			return float64(value), true
		case int16:
			return float64(value), true
		case int32:
			return float64(value), true
		case int64:
			return float64(value), true
		case uint:
			return float64(value), true
		case uint8:
			return float64(value), true
		case uint16:
			return float64(value), true
		case uint32:
			return float64(value), true
		case uint64:
			return float64(value), true
		case json.Number:
			parsed, err := value.Float64()
			return parsed, err == nil
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			return parsed, err == nil
		default:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
			return parsed, err == nil
		}
	}
	return 0, false
}

func firstBusinessMonitorDurationSeconds(values map[string]interface{}, keys ...string) int64 {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case int:
			if value > 0 {
				return int64(value)
			}
		case int64:
			if value > 0 {
				return value
			}
		case float64:
			if value > 0 {
				return int64(value)
			}
		case string:
			text := strings.TrimSpace(value)
			if text == "" {
				continue
			}
			if parsed, err := strconv.ParseInt(text, 10, 64); err == nil && parsed > 0 {
				return parsed
			}
			if duration, err := time.ParseDuration(text); err == nil && duration > 0 {
				return int64(duration.Seconds())
			}
		}
	}
	return 0
}

func applyBusinessMonitorPromQLCompare(query *businessMonitorRuleQuery) {
	if query == nil || query.HasCompare {
		return
	}
	matches := promQLComparePattern.FindStringSubmatch(strings.TrimSpace(query.Query))
	if len(matches) != 3 {
		return
	}
	threshold, err := strconv.ParseFloat(matches[2], 64)
	if err != nil {
		return
	}
	query.Operator = matches[1]
	query.Threshold = threshold
	query.HasCompare = true
}

func businessMonitorCurrentMetricSamples(value prommodel.Value, limit int, nodeIndex []map[string]interface{}, query businessMonitorRuleQuery) ([]map[string]interface{}, int, bool) {
	if limit <= 0 {
		limit = maxBusinessMonitorMetricSamples
	}
	switch typed := value.(type) {
	case prommodel.Vector:
		total := len(typed)
		limited := total > limit
		if limited {
			typed = typed[:limit]
		}
		result := make([]map[string]interface{}, 0, len(typed))
		for _, sample := range typed {
			if sample == nil {
				continue
			}
			labels := prometheusMetricToMap(sample.Metric)
			current := float64(sample.Value)
			status := "healthy"
			if query.HasCompare && businessMonitorCompare(current, query.Operator, query.Threshold) {
				status = "firing"
			}
			result = append(result, map[string]interface{}{
				"metric":       businessMonitorMetricDisplayLabels(labels),
				"value":        current,
				"status":       status,
				"timestamp":    sample.Timestamp.Time().Format(time.RFC3339Nano),
				"matchedNodes": matchBusinessMonitorAlertNodes(alertmodel.JSONMap(labels), nodeIndex),
			})
		}
		return result, total, limited
	case *prommodel.Scalar:
		if typed == nil {
			return []map[string]interface{}{}, 0, false
		}
		current := float64(typed.Value)
		status := "healthy"
		if query.HasCompare && businessMonitorCompare(current, query.Operator, query.Threshold) {
			status = "firing"
		}
		return []map[string]interface{}{{
			"value":     current,
			"status":    status,
			"timestamp": typed.Timestamp.Time().Format(time.RFC3339Nano),
		}}, 1, false
	default:
		return []map[string]interface{}{}, 0, false
	}
}

func businessMonitorCurrentValue(samples []map[string]interface{}, query businessMonitorRuleQuery) (float64, bool) {
	if len(samples) == 0 {
		return 0, false
	}
	selected := 0.0
	ok := false
	for _, sample := range samples {
		value, exists := firstBusinessMonitorFloat(sample, "value")
		if !exists {
			continue
		}
		if !ok {
			selected = value
			ok = true
			continue
		}
		switch strings.TrimSpace(query.Operator) {
		case "<", "<=":
			if value < selected {
				selected = value
			}
		case "!=", "==", "=":
			if math.Abs(value-query.Threshold) > math.Abs(selected-query.Threshold) {
				selected = value
			}
		default:
			if value > selected {
				selected = value
			}
		}
	}
	return selected, ok
}

func businessMonitorCompare(value float64, operator string, threshold float64) bool {
	switch strings.TrimSpace(operator) {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "=", "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

func businessMonitorThresholdText(operator string, threshold float64) string {
	operator = strings.TrimSpace(operator)
	if operator == "" {
		return ""
	}
	return operator + " " + strconv.FormatFloat(threshold, 'f', -1, 64)
}

func businessMonitorThresholdDistance(value float64, operator string, threshold float64) map[string]interface{} {
	delta := threshold - value
	ratio := 0.0
	if threshold != 0 {
		ratio = math.Abs(delta / threshold)
	}
	return map[string]interface{}{
		"delta":    delta,
		"ratio":    ratio,
		"operator": strings.TrimSpace(operator),
	}
}

func queryBusinessMonitorMetricRule(promAPI promv1.API, rule monitorAlertRule, nodeIndex []map[string]interface{}) map[string]interface{} {
	payload := map[string]interface{}{
		"name":         rule.Name,
		"selector":     rule.Selector,
		"total":        0,
		"limited":      false,
		"resultType":   "",
		"sampleSeries": []map[string]interface{}{},
		"matchedNodes": []map[string]interface{}{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	value, promWarnings, err := promAPI.Query(ctx, rule.Selector, time.Now())
	if err != nil {
		payload["status"] = "query_failed"
		payload["warning"] = err.Error()
		return payload
	}
	if len(promWarnings) > 0 {
		payload["warning"] = strings.Join(businessMonitorPrometheusWarnings(promWarnings), "; ")
	}
	if value == nil {
		payload["status"] = "no_matches"
		payload["resultType"] = "unknown"
		return payload
	}
	payload["status"] = "ready"
	payload["resultType"] = businessMonitorPrometheusValueType(value.Type())
	samples, total, limited := businessMonitorMetricSamples(value, maxBusinessMonitorMetricSamples, nodeIndex)
	payload["total"] = total
	payload["limited"] = limited
	payload["sampleSeries"] = samples
	payload["matchedNodes"] = collectBusinessMonitorMetricMatchedNodes(samples)
	if total == 0 {
		payload["status"] = "no_matches"
	}
	return payload
}

func emptyBusinessMonitorMetricCoverage() map[string]interface{} {
	return map[string]interface{}{
		"totalSeries":      0,
		"matchedNodeCount": 0,
		"matchedNodeIds":   []uint{},
		"limited":          false,
		"rules":            []map[string]interface{}{},
	}
}

func businessMonitorMetricSamples(value prommodel.Value, limit int, nodeIndex []map[string]interface{}) ([]map[string]interface{}, int, bool) {
	if limit <= 0 {
		limit = maxBusinessMonitorMetricSamples
	}
	switch typed := value.(type) {
	case prommodel.Vector:
		total := len(typed)
		limited := total > limit
		if limited {
			typed = typed[:limit]
		}
		result := make([]map[string]interface{}, 0, len(typed))
		for _, sample := range typed {
			if sample == nil {
				continue
			}
			labels := prometheusMetricToMap(sample.Metric)
			result = append(result, map[string]interface{}{
				"metric":       businessMonitorMetricDisplayLabels(labels),
				"value":        float64(sample.Value),
				"timestamp":    sample.Timestamp.Time().Format(time.RFC3339Nano),
				"matchedNodes": matchBusinessMonitorAlertNodes(alertmodel.JSONMap(labels), nodeIndex),
			})
		}
		return result, total, limited
	case *prommodel.Scalar:
		if typed == nil {
			return []map[string]interface{}{}, 0, false
		}
		return []map[string]interface{}{{
			"value":     float64(typed.Value),
			"timestamp": typed.Timestamp.Time().Format(time.RFC3339Nano),
		}}, 1, false
	default:
		if value == nil {
			return []map[string]interface{}{}, 0, false
		}
		return []map[string]interface{}{{
			"value": value.String(),
		}}, 1, false
	}
}

func prometheusMetricToMap(metric prommodel.Metric) map[string]interface{} {
	result := make(map[string]interface{}, len(metric))
	for key, value := range metric {
		result[string(key)] = string(value)
	}
	return result
}

func businessMonitorMetricDisplayLabels(labels map[string]interface{}) map[string]interface{} {
	keys := []string{
		"__name__", "container_name", "container", "namespace", "pod", "pod_name",
		"instance", "node", "kubernetes_io_hostname", "job", "service", "app",
		"application", "name", "role", "position",
	}
	result := make(map[string]interface{})
	for _, key := range keys {
		if value := strings.TrimSpace(stringFromAny(labels[key])); value != "" {
			result[key] = value
		}
	}
	return result
}

func collectBusinessMonitorMetricMatchedNodes(samples []map[string]interface{}) []map[string]interface{} {
	byID := make(map[uint]map[string]interface{})
	for _, sample := range samples {
		for _, node := range mapSliceFromAny(sample["matchedNodes"]) {
			nodeID := uintFromAny(node["resourceId"])
			if nodeID == 0 {
				continue
			}
			existing, exists := byID[nodeID]
			if !exists || uintFromAny(node["matchScore"]) > uintFromAny(existing["matchScore"]) {
				byID[nodeID] = node
			}
		}
	}
	result := make([]map[string]interface{}, 0, len(byID))
	for _, node := range byID {
		result = append(result, node)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return uintFromAny(result[i]["matchScore"]) > uintFromAny(result[j]["matchScore"])
	})
	return result
}

func businessMonitorCoverageNodes(metricCoverage map[string]interface{}) []map[string]interface{} {
	rules := mapSliceFromAny(metricCoverage["rules"])
	byID := make(map[uint]map[string]interface{})
	for _, rule := range rules {
		for _, node := range mapSliceFromAny(rule["matchedNodes"]) {
			nodeID := uintFromAny(node["resourceId"])
			if nodeID == 0 {
				continue
			}
			if existing, ok := byID[nodeID]; ok {
				if uintFromAny(node["matchScore"]) <= uintFromAny(existing["matchScore"]) {
					continue
				}
			}
			byID[nodeID] = map[string]interface{}{
				"resourceId":    nodeID,
				"resourceType":  stringFromAny(node["resourceType"]),
				"resourceLabel": stringFromAny(node["resourceLabel"]),
				"name":          stringFromAny(node["name"]),
				"displayName":   stringFromAny(node["displayName"]),
				"matchScore":    node["matchScore"],
				"evidence":      node["evidence"],
			}
		}
	}
	result := make([]map[string]interface{}, 0, len(byID))
	for _, node := range byID {
		result = append(result, node)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return uintFromAny(result[i]["matchScore"]) > uintFromAny(result[j]["matchScore"])
	})
	return result
}

func buildBusinessMonitorAlertDiagnosis(dataStatus string, summary map[string]interface{}, metricCoverage map[string]interface{}, currentMetricSummary map[string]interface{}) map[string]interface{} {
	totalSeries := uintFromAny(metricCoverage["totalSeries"])
	matchedNodeCount := uintFromAny(metricCoverage["matchedNodeCount"])
	totalAlerts := uintFromAny(summary["total"])
	criticalCount := uintFromAny(currentMetricSummary["critical"])
	warningCount := uintFromAny(currentMetricSummary["warning"])
	healthStatus := stringFromAny(summary["healthStatus"])
	message := "当前没有可显示的监控告警"
	suggestions := []string{}

	switch {
	case totalAlerts > 0:
		message = "当前存在活跃监控告警"
	case criticalCount > 0:
		message = "当前业务存在严重监控压力"
	case warningCount > 0:
		message = "当前业务存在监控预警"
	case matchedNodeCount > 0 && totalSeries > 0:
		message = "监控规则已命中业务资源，但当前没有活跃告警"
		suggestions = append(suggestions, "可以继续查看覆盖节点和样例 series")
	case totalSeries == 0:
		message = "监控规则没有命中任何指标"
		suggestions = append(suggestions, "检查 selector 是否匹配容器名、namespace、instance 等标签")
	default:
		message = "监控数据已接入，当前没有活跃告警"
	}

	if dataStatus == "no_metric_matches" {
		suggestions = append(suggestions, "规则命中为 0 时，优先检查数据源和标签字段是否正确")
	}
	if dataStatus == "no_alerts" && matchedNodeCount > 0 {
		suggestions = append(suggestions, "如果希望节点飘红，需要对应的监控告警规则实际触发")
	}
	if criticalCount > 0 || warningCount > 0 {
		suggestions = append(suggestions, "当前监控值已超过阈值，节点可按压力状态飘红")
	}

	return map[string]interface{}{
		"status":           dataStatus,
		"healthStatus":     healthStatus,
		"message":          message,
		"suggestions":      uniqueStrings(suggestions),
		"matchedNodeCount": matchedNodeCount,
		"totalSeries":      totalSeries,
		"criticalCount":    criticalCount,
		"warningCount":     warningCount,
	}
}

func enrichBusinessMonitorSummaryWithCurrentMetrics(summary map[string]interface{}, currentMetricSummary map[string]interface{}) map[string]interface{} {
	if summary == nil {
		summary = map[string]interface{}{}
	}
	if uintFromAny(currentMetricSummary["critical"]) > 0 {
		summary["healthStatus"] = cmdbmodel.CmdbHealthStatusCritical
	} else if uintFromAny(currentMetricSummary["warning"]) > 0 && stringFromAny(summary["healthStatus"]) != cmdbmodel.CmdbHealthStatusCritical {
		summary["healthStatus"] = cmdbmodel.CmdbHealthStatusWarning
	}
	summary["currentMetricSummary"] = currentMetricSummary
	return summary
}

func buildBusinessMonitorPressureOverview(dataStatus string, metrics []map[string]interface{}, metricNodes []map[string]interface{}, currentMetricSummary map[string]interface{}) map[string]interface{} {
	worstMetric := businessMonitorWorstCurrentMetric(metrics)
	affectedNodes := make([]map[string]interface{}, 0, len(metricNodes))
	for _, node := range metricNodes {
		affectedNodes = append(affectedNodes, map[string]interface{}{
			"resourceId":   node["resourceId"],
			"name":         node["name"],
			"displayName":  node["displayName"],
			"resourceType": node["resourceType"],
			"healthStatus": node["healthStatus"],
			"worstMetric":  node["worstMetric"],
		})
	}
	message := "当前业务监控正常"
	if uintFromAny(currentMetricSummary["critical"]) > 0 {
		message = "当前业务存在严重监控压力"
	} else if uintFromAny(currentMetricSummary["warning"]) > 0 {
		message = "当前业务存在监控预警"
	}
	return map[string]interface{}{
		"status":            dataStatus,
		"message":           message,
		"primaryIssue":      worstMetric,
		"affectedNodeCount": len(affectedNodes),
		"affectedNodes":     affectedNodes,
		"suggestions":       businessMonitorPressureSuggestions(worstMetric),
	}
}

func businessMonitorWorstCurrentMetric(metrics []map[string]interface{}) map[string]interface{} {
	var worst map[string]interface{}
	for _, metric := range metrics {
		if !businessMonitorStatusAbnormal(stringFromAny(metric["status"])) {
			continue
		}
		if worst == nil || metricAlertSeverityRank(stringFromAny(metric["status"])) > metricAlertSeverityRank(stringFromAny(worst["status"])) {
			worst = metric
		}
	}
	if worst == nil {
		return nil
	}
	return map[string]interface{}{
		"metricKey":         worst["metricKey"],
		"metricName":        worst["metricName"],
		"category":          worst["category"],
		"currentValue":      roundBusinessMonitorFloat(floatFromAny(worst["currentValue"])),
		"unit":              worst["unit"],
		"status":            worst["status"],
		"warningThreshold":  worst["warningThreshold"],
		"criticalThreshold": worst["criticalThreshold"],
		"pressure":          worst["pressure"],
		"matchedNodeCount":  len(mapSliceFromAny(worst["matchedNodes"])),
	}
}

func businessMonitorPressureSuggestions(issue map[string]interface{}) []string {
	if issue == nil {
		return []string{}
	}
	switch stringFromAny(issue["category"]) {
	case "memory":
		return []string{"优先检查高内存 Pod 是否需要扩容、是否存在内存泄露或缓存异常增长"}
	case "cpu":
		return []string{"优先检查高 CPU Pod 的请求量、慢任务、线程池和副本数是否需要扩容"}
	case "restart":
		return []string{"优先检查 Pod 最近重启原因、事件和容器退出码"}
	default:
		return []string{"优先查看异常节点详情和最近变更"}
	}
}

func newBusinessMonitorPromAPI(ds alertmodel.AlertDatasource) (promv1.API, error) {
	if strings.TrimSpace(ds.URL) == "" {
		return nil, errors.New("告警数据源 URL 为空")
	}
	roundTripper := http.DefaultTransport
	switch strings.ToLower(strings.TrimSpace(ds.AuthType)) {
	case "", "none":
	case "basic":
		roundTripper = &businessMonitorBasicAuthRoundTripper{
			username: ds.Username,
			password: ds.Password,
			next:     roundTripper,
		}
	case "bearer":
		roundTripper = &businessMonitorBearerAuthRoundTripper{
			token: ds.Token,
			next:  roundTripper,
		}
	default:
		return nil, fmt.Errorf("不支持的认证方式: %s", ds.AuthType)
	}
	client, err := promapi.NewClient(promapi.Config{
		Address:      normalizeBusinessMonitorDatasourceURL(ds.URL),
		RoundTripper: roundTripper,
	})
	if err != nil {
		return nil, err
	}
	return promv1.NewAPI(client), nil
}

func normalizeBusinessMonitorDatasourceURL(rawURL string) string {
	url := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	for _, suffix := range []string{"/api/v1/import/prometheus", "/api/v1/write", "/write"} {
		if strings.HasSuffix(url, suffix) {
			return strings.TrimRight(strings.TrimSuffix(url, suffix), "/")
		}
	}
	return url
}

func isBusinessMonitorPromQLDatasource(datasourceType string) bool {
	switch strings.ToLower(strings.TrimSpace(datasourceType)) {
	case alertmodel.AlertDatasourcePrometheus, alertmodel.AlertDatasourceVictoriaMetrics, alertmodel.AlertDatasourceBuiltin:
		return true
	default:
		return false
	}
}

type businessMonitorBasicAuthRoundTripper struct {
	username string
	password string
	next     http.RoundTripper
}

func (rt *businessMonitorBasicAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(rt.username, rt.password)
	return rt.next.RoundTrip(req)
}

type businessMonitorBearerAuthRoundTripper struct {
	token string
	next  http.RoundTripper
}

func (rt *businessMonitorBearerAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.TrimSpace(rt.token) != "" {
		req.Header.Set("Authorization", "Bearer "+rt.token)
	}
	return rt.next.RoundTrip(req)
}

func businessMonitorPrometheusValueType(valueType prommodel.ValueType) string {
	switch valueType {
	case prommodel.ValScalar:
		return "scalar"
	case prommodel.ValVector:
		return "vector"
	case prommodel.ValMatrix:
		return "matrix"
	case prommodel.ValString:
		return "string"
	default:
		return "unknown"
	}
}

func businessMonitorPrometheusWarnings(warnings promv1.Warnings) []string {
	if len(warnings) == 0 {
		return nil
	}
	result := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		result = append(result, string(warning))
	}
	return result
}

func intFromAny(value interface{}) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case uint:
		return int(typed)
	case uint8:
		return int(typed)
	case uint16:
		return int(typed)
	case uint32:
		return int(typed)
	case uint64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func floatFromAny(value interface{}) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int8:
		return float64(typed)
	case int16:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case uint:
		return float64(typed)
	case uint8:
		return float64(typed)
	case uint16:
		return float64(typed)
	case uint32:
		return float64(typed)
	case uint64:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	default:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
		return parsed
	}
}

func mapSliceFromAny(value interface{}) []map[string]interface{} {
	switch typed := value.(type) {
	case []map[string]interface{}:
		return typed
	case []interface{}:
		result := make([]map[string]interface{}, 0, len(typed))
		for _, item := range typed {
			if mapped, ok := item.(map[string]interface{}); ok {
				result = append(result, mapped)
			}
		}
		return result
	default:
		return nil
	}
}

func parseMonitorAlertSelector(selector string) ([]monitorAlertMatcher, error) {
	selector = normalizeBusinessMonitorAlertSelector(selector)
	if !strings.HasPrefix(selector, "{") || !strings.HasSuffix(selector, "}") {
		return nil, errors.New("规则必须是 PromQL label selector，例如 {container_name=~\"zero-ai.*\"}")
	}
	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(selector, "{"), "}"))
	if body == "" {
		return nil, errors.New("selector 不能为空")
	}
	parts, err := splitMonitorAlertSelectorMatchers(body)
	if err != nil {
		return nil, err
	}
	matchers := make([]monitorAlertMatcher, 0, len(parts))
	for _, part := range parts {
		matches := selectorMatcherPattern.FindStringSubmatch(part)
		if len(matches) != 4 {
			return nil, fmt.Errorf("不支持的 matcher: %s", part)
		}
		value, err := strconv.Unquote(`"` + matches[3] + `"`)
		if err != nil {
			return nil, fmt.Errorf("matcher 值解析失败: %s", matches[1])
		}
		matcher := monitorAlertMatcher{
			Label: matches[1],
			Op:    matches[2],
			Value: value,
		}
		if matcher.Op == "=~" || matcher.Op == "!~" {
			regex, err := regexp.Compile(value)
			if err != nil {
				return nil, fmt.Errorf("正则表达式无效: %s", value)
			}
			matcher.Regex = regex
		}
		matchers = append(matchers, matcher)
	}
	return matchers, nil
}

func splitMonitorAlertSelectorMatchers(body string) ([]string, error) {
	parts := make([]string, 0)
	var builder strings.Builder
	inQuote := false
	escaped := false
	for _, r := range body {
		switch {
		case escaped:
			builder.WriteRune(r)
			escaped = false
		case r == '\\':
			builder.WriteRune(r)
			escaped = true
		case r == '"':
			builder.WriteRune(r)
			inQuote = !inQuote
		case r == ',' && !inQuote:
			part := strings.TrimSpace(builder.String())
			if part != "" {
				parts = append(parts, part)
			}
			builder.Reset()
		default:
			builder.WriteRune(r)
		}
	}
	if inQuote {
		return nil, errors.New("selector 引号未闭合")
	}
	part := strings.TrimSpace(builder.String())
	if part != "" {
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return nil, errors.New("selector 不能为空")
	}
	return parts, nil
}

func matchBusinessMonitorAlertEvents(business *cmdbmodel.CmdbBusinessModel, events []alertmodel.AlertActiveEvent, rules []monitorAlertRule, nodeIndex []map[string]interface{}) ([]map[string]interface{}, map[uint][]map[string]interface{}) {
	result := make([]map[string]interface{}, 0)
	nodeAlerts := make(map[uint][]map[string]interface{})
	for _, event := range events {
		matchedRules := matchMonitorAlertRules(event.Labels, rules)
		businessMatched := eventMatchesBusinessIdentity(business, event)
		if len(matchedRules) == 0 && !businessMatched {
			continue
		}
		nodes := matchBusinessMonitorAlertNodes(event.Labels, nodeIndex)
		alertPayload := businessMonitorAlertEventPayload(event, matchedRules, businessMatched, nodes)
		result = append(result, alertPayload)
		for _, node := range nodes {
			nodeID := uintFromAny(node["resourceId"])
			if nodeID == 0 {
				continue
			}
			nodeAlerts[nodeID] = append(nodeAlerts[nodeID], alertPayload)
		}
	}
	return result, nodeAlerts
}

func matchMonitorAlertRules(labels alertmodel.JSONMap, rules []monitorAlertRule) []string {
	matched := make([]string, 0)
	for _, rule := range rules {
		ok := true
		for _, matcher := range rule.Matchers {
			if !matcher.matches(labels) {
				ok = false
				break
			}
		}
		if ok {
			matched = append(matched, rule.Name)
		}
	}
	return matched
}

func (m monitorAlertMatcher) matches(labels alertmodel.JSONMap) bool {
	value := stringFromAny(labels[m.Label])
	switch m.Op {
	case "=":
		return value == m.Value
	case "!=":
		return value != m.Value
	case "=~":
		return m.Regex != nil && m.Regex.MatchString(value)
	case "!~":
		return m.Regex == nil || !m.Regex.MatchString(value)
	default:
		return false
	}
}

func eventMatchesBusinessIdentity(business *cmdbmodel.CmdbBusinessModel, event alertmodel.AlertActiveEvent) bool {
	if business == nil {
		return false
	}
	candidates := []string{
		strconv.FormatUint(uint64(business.ID), 10),
		business.Name,
		business.DisplayName,
		business.BizCode,
	}
	for _, candidate := range candidates {
		if candidate = strings.TrimSpace(candidate); candidate != "" && strings.EqualFold(strings.TrimSpace(event.BusinessID), candidate) {
			return true
		}
	}
	for _, key := range []string{"business_id", "businessId", "business", "biz_code", "bizCode", "app", "application"} {
		labelValue := stringFromAny(event.Labels[key])
		for _, candidate := range candidates {
			if candidate = strings.TrimSpace(candidate); candidate != "" && strings.EqualFold(labelValue, candidate) {
				return true
			}
		}
	}
	return false
}

func buildBusinessMonitorAlertNodeIndex(topology map[string]interface{}) []map[string]interface{} {
	nodes, _ := topology["nodes"].([]map[string]interface{})
	result := make([]map[string]interface{}, 0, len(nodes))
	for _, node := range nodes {
		resourceType := normalizeResourceType(stringFromAny(node["resourceType"]))
		if resourceType == "" || resourceType == cmdbmodel.CmdbResourceTypeBusinessSystem ||
			resourceType == cmdbmodel.CmdbResourceTypeLogProject || resourceType == cmdbmodel.CmdbResourceTypeLogStore {
			continue
		}
		candidates := monitorAlertNodeCandidates(node)
		if len(candidates) == 0 {
			continue
		}
		item := map[string]interface{}{
			"resourceId":    uintFromAny(node["id"]),
			"resourceType":  resourceType,
			"resourceLabel": resourceTypeLabel(resourceType),
			"name":          stringFromAny(node["name"]),
			"displayName":   stringFromAny(node["displayName"]),
			"candidates":    candidates,
		}
		result = append(result, item)
	}
	return result
}

func monitorAlertNodeCandidates(node map[string]interface{}) []string {
	values := []string{
		stringFromAny(node["name"]),
		stringFromAny(node["displayName"]),
		stringFromAny(node["sourceKey"]),
		stringFromAny(node["instanceId"]),
	}
	props, _ := node["properties"].(map[string]interface{})
	for _, key := range []string{
		"namespace", "workload", "workloadName", "pod", "podName", "pod_name",
		"container", "containerName", "container_name", "service", "serviceName",
		"app", "appName", "application", "host", "hostname", "hostName",
		"ip", "hostIp", "privateIp", "instance", "node", "database", "dbName",
	} {
		values = append(values, stringFromAny(props[key]))
	}
	values = uniqueStrings(values)
	sort.SliceStable(values, func(i, j int) bool {
		return len(values[i]) > len(values[j])
	})
	return values
}

func matchBusinessMonitorAlertNodes(labels alertmodel.JSONMap, nodeIndex []map[string]interface{}) []map[string]interface{} {
	matches := make([]map[string]interface{}, 0)
	for _, node := range nodeIndex {
		candidates, _ := node["candidates"].([]string)
		score, evidence := monitorAlertNodeMatchScore(labels, candidates)
		if score == 0 {
			continue
		}
		matches = append(matches, map[string]interface{}{
			"resourceId":    node["resourceId"],
			"resourceType":  node["resourceType"],
			"resourceLabel": node["resourceLabel"],
			"name":          node["name"],
			"displayName":   node["displayName"],
			"matchScore":    score,
			"evidence":      evidence,
		})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return uintFromAny(matches[i]["matchScore"]) > uintFromAny(matches[j]["matchScore"])
	})
	if len(matches) > 5 {
		return matches[:5]
	}
	return matches
}

func monitorAlertNodeMatchScore(labels alertmodel.JSONMap, candidates []string) (int, map[string]interface{}) {
	bestScore := 0
	bestEvidence := map[string]interface{}{}
	for labelKey, labelRaw := range labels {
		labelValue := strings.TrimSpace(stringFromAny(labelRaw))
		if labelValue == "" {
			continue
		}
		lowerLabel := strings.ToLower(labelValue)
		for _, candidate := range candidates {
			candidate = strings.TrimSpace(candidate)
			if len(candidate) < 3 {
				continue
			}
			lowerCandidate := strings.ToLower(candidate)
			score := 0
			switch {
			case lowerLabel == lowerCandidate:
				score = 100 + len(candidate)
			case strings.HasPrefix(lowerLabel, lowerCandidate):
				score = 80 + len(candidate)
			case strings.Contains(lowerLabel, lowerCandidate):
				score = 50 + len(candidate)
			}
			if score > bestScore {
				bestScore = score
				bestEvidence = map[string]interface{}{
					"labelKey":    labelKey,
					"labelValue":  labelValue,
					"matchedText": candidate,
				}
			}
		}
	}
	return bestScore, bestEvidence
}

func businessMonitorAlertEventPayload(event alertmodel.AlertActiveEvent, matchedRules []string, businessMatched bool, nodes []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":               event.ID,
		"fingerprint":      event.Fingerprint,
		"ruleId":           event.RuleID,
		"ruleName":         event.RuleName,
		"datasourceId":     event.DatasourceID,
		"datasourceType":   event.DatasourceType,
		"severity":         event.Severity,
		"status":           event.Status,
		"labels":           event.Labels,
		"annotations":      event.Annotations,
		"value":            event.Value,
		"triggerQuery":     event.TriggerQuery,
		"triggerCondition": event.TriggerCondition,
		"eventDetail":      event.EventDetail,
		"assetId":          event.AssetID,
		"businessId":       event.BusinessID,
		"firstTriggeredAt": event.FirstTriggeredAt,
		"lastEvaluatedAt":  event.LastEvaluatedAt,
		"suppressed":       event.Suppressed,
		"silenced":         event.Silenced,
		"matchedRules":     matchedRules,
		"businessMatched":  businessMatched,
		"matchedNodes":     nodes,
	}
}

func buildBusinessMonitorAlertNodePayload(topology map[string]interface{}, nodeAlerts map[uint][]map[string]interface{}) []map[string]interface{} {
	nodes, _ := topology["nodes"].([]map[string]interface{})
	result := make([]map[string]interface{}, 0)
	for _, node := range nodes {
		nodeID := uintFromAny(node["id"])
		alerts := nodeAlerts[nodeID]
		if len(alerts) == 0 {
			continue
		}
		severity := worstMonitorAlertSeverity(alerts)
		result = append(result, map[string]interface{}{
			"resourceId":    nodeID,
			"name":          stringFromAny(node["name"]),
			"displayName":   stringFromAny(node["displayName"]),
			"resourceType":  stringFromAny(node["resourceType"]),
			"resourceLabel": stringFromAny(node["resourceLabel"]),
			"alertCount":    len(alerts),
			"severity":      severity,
			"healthStatus":  monitorAlertHealthStatusBySeverity(severity),
			"alerts":        alerts,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if len(result[i]["alerts"].([]map[string]interface{})) != len(result[j]["alerts"].([]map[string]interface{})) {
			return len(result[i]["alerts"].([]map[string]interface{})) > len(result[j]["alerts"].([]map[string]interface{}))
		}
		return monitorAlertSeverityRank(stringFromAny(result[i]["severity"])) > monitorAlertSeverityRank(stringFromAny(result[j]["severity"]))
	})
	return result
}

func buildBusinessMonitorMetricNodePayload(topology map[string]interface{}, metrics []map[string]interface{}) []map[string]interface{} {
	nodes, _ := topology["nodes"].([]map[string]interface{})
	nodeByID := make(map[uint]map[string]interface{}, len(nodes))
	for _, node := range nodes {
		nodeByID[uintFromAny(node["id"])] = node
	}
	type metricNodeState struct {
		node       map[string]interface{}
		metrics    []map[string]interface{}
		metricKeys map[string]struct{}
		status     string
	}
	stateByID := make(map[uint]*metricNodeState)
	for _, metric := range metrics {
		samples := mapSliceFromAny(metric["samples"])
		if len(samples) == 0 {
			continue
		}
		for _, sample := range samples {
			sampleStatus := strings.ToLower(stringFromAny(sample["status"]))
			if !businessMonitorStatusAbnormal(sampleStatus) {
				continue
			}
			for _, matched := range mapSliceFromAny(sample["matchedNodes"]) {
				nodeID := uintFromAny(matched["resourceId"])
				if nodeID == 0 {
					continue
				}
				node := nodeByID[nodeID]
				if node == nil {
					continue
				}
				state := stateByID[nodeID]
				if state == nil {
					state = &metricNodeState{node: node, status: "healthy", metricKeys: map[string]struct{}{}}
					stateByID[nodeID] = state
				}
				metricKey := stringFromAny(metric["metricKey"]) + "|" + stringFromAny(sample["metric"])
				if _, exists := state.metricKeys[metricKey]; !exists {
					state.metricKeys[metricKey] = struct{}{}
					state.metrics = append(state.metrics, businessMonitorNodeMetricEvidence(metric, sample))
				}
				if metricAlertSeverityRank(sampleStatus) > metricAlertSeverityRank(state.status) {
					state.status = sampleStatus
				}
			}
		}
	}
	result := make([]map[string]interface{}, 0, len(stateByID))
	for nodeID, state := range stateByID {
		alertStatus := strings.ToLower(stringFromAny(state.node["healthStatus"]))
		if alertStatus == "" {
			alertStatus = "healthy"
		}
		healthStatus := alertStatus
		if metricAlertSeverityRank(state.status) > metricAlertSeverityRank(healthStatus) {
			healthStatus = state.status
		}
		nodeMetrics := make([]map[string]interface{}, 0, len(state.metrics))
		for _, metric := range state.metrics {
			nodeMetrics = append(nodeMetrics, metric)
		}
		result = append(result, map[string]interface{}{
			"resourceId":    nodeID,
			"name":          stringFromAny(state.node["name"]),
			"displayName":   stringFromAny(state.node["displayName"]),
			"resourceType":  stringFromAny(state.node["resourceType"]),
			"resourceLabel": stringFromAny(state.node["resourceLabel"]),
			"healthStatus":  healthStatus,
			"riskCount":     len(nodeMetrics),
			"worstMetric":   businessMonitorWorstMetricEvidence(nodeMetrics),
			"metrics":       nodeMetrics,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return metricAlertSeverityRank(stringFromAny(result[i]["healthStatus"])) > metricAlertSeverityRank(stringFromAny(result[j]["healthStatus"]))
	})
	return result
}

func mergeBusinessMonitorNodePayloads(alertNodes, metricNodes []map[string]interface{}) []map[string]interface{} {
	merged := make(map[uint]map[string]interface{})
	upsert := func(node map[string]interface{}) {
		nodeID := uintFromAny(node["resourceId"])
		if nodeID == 0 {
			return
		}
		existing, ok := merged[nodeID]
		if !ok {
			merged[nodeID] = node
			return
		}
		if metricAlertSeverityRank(stringFromAny(node["healthStatus"])) > metricAlertSeverityRank(stringFromAny(existing["healthStatus"])) {
			existing["healthStatus"] = node["healthStatus"]
		}
		if alertCount := uintFromAny(node["alertCount"]); alertCount > 0 {
			existing["alertCount"] = alertCount
			existing["alerts"] = node["alerts"]
		}
		if metrics := mapSliceFromAny(node["metrics"]); len(metrics) > 0 {
			existing["metrics"] = metrics
			existing["riskCount"] = node["riskCount"]
			existing["worstMetric"] = node["worstMetric"]
		}
	}
	for _, node := range metricNodes {
		upsert(node)
	}
	for _, node := range alertNodes {
		upsert(node)
	}
	result := make([]map[string]interface{}, 0, len(merged))
	for _, node := range merged {
		result = append(result, node)
	}
	sort.SliceStable(result, func(i, j int) bool {
		leftRank := metricAlertSeverityRank(stringFromAny(result[i]["healthStatus"]))
		rightRank := metricAlertSeverityRank(stringFromAny(result[j]["healthStatus"]))
		if leftRank != rightRank {
			return leftRank > rightRank
		}
		return uintFromAny(result[i]["resourceId"]) < uintFromAny(result[j]["resourceId"])
	})
	return result
}

func businessMonitorStatusAbnormal(status string) bool {
	return metricAlertSeverityRank(status) >= metricAlertSeverityRank("warning")
}

func businessMonitorNodeMetricEvidence(metric map[string]interface{}, sample map[string]interface{}) map[string]interface{} {
	value, _ := firstBusinessMonitorFloat(sample, "value")
	warningThreshold, _ := firstBusinessMonitorFloat(metric, "warningThreshold")
	criticalThreshold, _ := firstBusinessMonitorFloat(metric, "criticalThreshold")
	return map[string]interface{}{
		"metricKey":         stringFromAny(metric["metricKey"]),
		"metricName":        stringFromAny(metric["metricName"]),
		"category":          stringFromAny(metric["category"]),
		"value":             roundBusinessMonitorFloat(value),
		"unit":              stringFromAny(metric["unit"]),
		"status":            stringFromAny(sample["status"]),
		"warningThreshold":  warningThreshold,
		"criticalThreshold": criticalThreshold,
		"pressure":          businessMonitorSamplePressure(value, warningThreshold, criticalThreshold),
		"sample": map[string]interface{}{
			"metric":    sample["metric"],
			"timestamp": sample["timestamp"],
		},
	}
}

func businessMonitorWorstMetricEvidence(metrics []map[string]interface{}) map[string]interface{} {
	var worst map[string]interface{}
	for _, metric := range metrics {
		if worst == nil || metricAlertSeverityRank(stringFromAny(metric["status"])) > metricAlertSeverityRank(stringFromAny(worst["status"])) {
			worst = metric
		}
	}
	return worst
}

func businessMonitorSamplePressure(value, warningThreshold, criticalThreshold float64) map[string]interface{} {
	percentOfWarning := 0.0
	percentOfCritical := 0.0
	if warningThreshold != 0 {
		percentOfWarning = value / warningThreshold * 100
	}
	if criticalThreshold != 0 {
		percentOfCritical = value / criticalThreshold * 100
	}
	return map[string]interface{}{
		"percentOfWarning":  roundBusinessMonitorFloat(percentOfWarning),
		"percentOfCritical": roundBusinessMonitorFloat(percentOfCritical),
		"warningLeft":       roundBusinessMonitorFloat(warningThreshold - value),
		"criticalLeft":      roundBusinessMonitorFloat(criticalThreshold - value),
	}
}

func summarizeBusinessMonitorAlerts(events []map[string]interface{}, nodes []map[string]interface{}) map[string]interface{} {
	severityCounts := map[string]int{}
	statusCounts := map[string]int{}
	silenced := 0
	suppressed := 0
	for _, event := range events {
		severityCounts[strings.ToLower(stringFromAny(event["severity"]))]++
		statusCounts[strings.ToLower(stringFromAny(event["status"]))]++
		if uintFromAny(event["silenced"]) > 0 {
			silenced++
		}
		if uintFromAny(event["suppressed"]) > 0 {
			suppressed++
		}
	}
	worst := worstMonitorAlertSeverity(events)
	return map[string]interface{}{
		"total":             len(events),
		"affectedNodeCount": len(nodes),
		"severity":          worst,
		"healthStatus":      monitorAlertHealthStatusBySeverity(worst),
		"severityCounts":    severityCounts,
		"statusCounts":      statusCounts,
		"silenced":          silenced,
		"suppressed":        suppressed,
	}
}

func worstMonitorAlertSeverity(events []map[string]interface{}) string {
	worst := ""
	for _, event := range events {
		severity := stringFromAny(event["severity"])
		if monitorAlertSeverityRank(severity) > monitorAlertSeverityRank(worst) {
			worst = severity
		}
	}
	return worst
}

func monitorAlertSeverityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "fatal", "disaster", "p0", "p1", "严重":
		return 4
	case "warning", "warn", "p2", "告警":
		return 3
	case "info", "notice", "p3", "提示":
		return 2
	default:
		return 1
	}
}

func metricAlertSeverityRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "critical", "firing":
		return 4
	case "warning":
		return 3
	case "unknown", "no_data", "query_failed":
		return 2
	case "healthy":
		return 1
	default:
		return 0
	}
}

func monitorAlertHealthStatusBySeverity(severity string) string {
	switch {
	case monitorAlertSeverityRank(severity) >= 4:
		return cmdbmodel.CmdbHealthStatusCritical
	case monitorAlertSeverityRank(severity) >= 3:
		return cmdbmodel.CmdbHealthStatusWarning
	case strings.TrimSpace(severity) != "":
		return cmdbmodel.CmdbHealthStatusWarning
	default:
		return cmdbmodel.CmdbHealthStatusHealthy
	}
}

func businessMonitorAlertConfigPayload(config cmdbmodel.BusinessMonitorAlertConfigDto) map[string]interface{} {
	return map[string]interface{}{
		"enabled":        businessMonitorAlertEnabled(config),
		"datasourceId":   config.DatasourceID,
		"datasourceType": strings.TrimSpace(config.DatasourceType),
		"matchRules":     businessMonitorAlertMatchRulePayloads(config.MatchRules),
	}
}

func businessMonitorAlertMatchRulePayloads(items []cmdbmodel.BusinessMonitorAlertMatchRuleDto) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]interface{}{
			"name":        strings.TrimSpace(item.Name),
			"selector":    normalizeBusinessMonitorAlertSelector(item.Selector),
			"enabled":     monitorAlertRuleEnabled(item.Enabled),
			"description": strings.TrimSpace(item.Description),
		})
	}
	return result
}

func businessMonitorAlertRulePayloads(items []monitorAlertRule) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]interface{}{
			"name":        item.Name,
			"selector":    item.Selector,
			"enabled":     item.Enabled,
			"description": item.Description,
			"matchers":    len(item.Matchers),
		})
	}
	return result
}

func buildBusinessMonitorAlertRecommendedRules(business *cmdbmodel.CmdbBusinessModel, topology map[string]interface{}) []map[string]interface{} {
	recommended := make([]map[string]interface{}, 0)
	if business != nil {
		if name := strings.TrimSpace(business.Name); name != "" {
			recommended = append(recommended, map[string]interface{}{
				"name":        "按业务名匹配容器",
				"selector":    fmt.Sprintf(`{container_name=~"%s.*"}`, regexp.QuoteMeta(name)),
				"description": "适用于容器名以业务名开头的 K8S 业务",
			})
		}
	}
	nodes, _ := topology["nodes"].([]map[string]interface{})
	seen := make(map[string]struct{})
	for _, node := range nodes {
		resourceType := normalizeResourceType(stringFromAny(node["resourceType"]))
		if resourceType != cmdbmodel.CmdbResourceTypeK8sWorkload && resourceType != cmdbmodel.CmdbResourceTypeHost &&
			resourceType != cmdbmodel.CmdbResourceTypeBusinessService && resourceType != cmdbmodel.CmdbResourceTypeApplication {
			continue
		}
		name := stringFromAny(node["name"])
		if name == "" {
			name = stringFromAny(node["displayName"])
		}
		if name == "" {
			continue
		}
		var selector string
		switch resourceType {
		case cmdbmodel.CmdbResourceTypeHost:
			selector = fmt.Sprintf(`{instance=~"%s(:.*)?"}`, regexp.QuoteMeta(name))
		default:
			selector = fmt.Sprintf(`{container_name=~"%s.*"}`, regexp.QuoteMeta(name))
		}
		if _, exists := seen[selector]; exists {
			continue
		}
		seen[selector] = struct{}{}
		recommended = append(recommended, map[string]interface{}{
			"name":        resourceDisplayRuleName(resourceType, name),
			"selector":    selector,
			"description": "根据业务拓扑节点自动推荐",
		})
		if len(recommended) >= 10 {
			break
		}
	}
	return recommended
}

func resourceDisplayRuleName(resourceType, name string) string {
	label := resourceTypeLabel(resourceType)
	if label == "" {
		label = resourceType
	}
	return label + ": " + name
}

func monitorAlertDatasourcePayload(ds *alertmodel.AlertDatasource) map[string]interface{} {
	if ds == nil {
		return nil
	}
	return map[string]interface{}{
		"id":             ds.ID,
		"name":           ds.Name,
		"datasourceType": ds.DatasourceType,
		"url":            ds.URL,
		"authType":       ds.AuthType,
		"description":    ds.Description,
		"builtin":        ds.Builtin,
		"default":        ds.Default,
		"enabled":        ds.Enabled,
		"labels":         ds.Labels,
	}
}

func normalizeBusinessMonitorAlertSelector(selector string) string {
	return strings.TrimSpace(selector)
}

func businessMonitorAlertEnabled(config cmdbmodel.BusinessMonitorAlertConfigDto) bool {
	if config.Enabled == nil {
		return true
	}
	return *config.Enabled
}

func monitorAlertRuleEnabled(value *bool) bool {
	return value == nil || *value
}

func boolPtr(value bool) *bool {
	return &value
}

func normalizeBusinessMonitorAlertPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > maxBusinessMonitorAlertEvents {
		pageSize = maxBusinessMonitorAlertEvents
	}
	return page, pageSize
}

func paginateMonitorAlertEvents(events []map[string]interface{}, page, pageSize int) []map[string]interface{} {
	start := (page - 1) * pageSize
	if start >= len(events) {
		return []map[string]interface{}{}
	}
	end := start + pageSize
	if end > len(events) {
		end = len(events)
	}
	return events[start:end]
}
