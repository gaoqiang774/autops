package engine

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"dodevops-api/api/alertmanager/model"

	prommodel "github.com/prometheus/common/model"
)

type alertSample struct {
	Fingerprint      string
	Labels           model.JSONMap
	Severity         string
	Value            string
	Metric           prommodel.Metric
	TriggerQuery     string
	TriggerCondition string
}

type ruleQuery struct {
	Query         string
	Level         string
	Severity      string
	Operator      string
	Threshold     float64
	HasCompare    bool
	WindowSeconds int64
}

const maxConcurrentRuleEvaluations = 32

var ruleEvalLimiter = make(chan struct{}, maxConcurrentRuleEvaluations)

func evaluateRule(ctx context.Context, rule model.AlertRule, ds model.AlertDatasource) ([]alertSample, error) {
	if isVictoriaLogsDatasource(ds.DatasourceType) {
		return evaluateVictoriaLogsRule(ctx, rule, ds)
	}
	if !isPromQLDatasource(ds.DatasourceType) {
		return nil, fmt.Errorf("unsupported datasource type: %s", ds.DatasourceType)
	}
	if strings.TrimSpace(ds.URL) == "" {
		return nil, fmt.Errorf("datasource url is empty")
	}
	release, err := acquireRuleEvalSlot(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	promAPI, err := newPromAPI(ds)
	if err != nil {
		return nil, err
	}

	queries := extractRuleQueries(rule)
	if len(queries) == 0 {
		return nil, fmt.Errorf("query is empty")
	}

	bestByFingerprint := make(map[string]alertSample)
	for _, query := range queries {
		value, _, err := promAPI.Query(ctx, query.Query, time.Now())
		if err != nil {
			return nil, fmt.Errorf("query %q failed: %w", query.Query, err)
		}

		vector, ok := value.(prommodel.Vector)
		if !ok {
			continue
		}

		for _, sample := range vector {
			if sample == nil {
				continue
			}
			if !sampleMatchesQuery(sample, query) {
				continue
			}
			fp := fingerprint(rule.ID, sample.Metric)
			candidate := alertSample{
				Fingerprint:      fp,
				Labels:           buildSampleLabels(rule.Labels, sample.Metric, query),
				Severity:         query.Severity,
				Value:            strconv.FormatFloat(float64(sample.Value), 'f', -1, 64),
				Metric:           sample.Metric,
				TriggerQuery:     query.Query,
				TriggerCondition: query.conditionText(),
			}
			current, exists := bestByFingerprint[fp]
			if !exists || severityRank(candidate.Severity, query.Level) > severityRank(current.Severity, labelString(current.Labels, "level")) {
				bestByFingerprint[fp] = candidate
			}
		}
	}

	samples := make([]alertSample, 0, len(bestByFingerprint))
	for _, sample := range bestByFingerprint {
		samples = append(samples, sample)
	}
	return samples, nil
}

func acquireRuleEvalSlot(ctx context.Context) (func(), error) {
	select {
	case ruleEvalLimiter <- struct{}{}:
		return func() { <-ruleEvalLimiter }, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for rule evaluation slot failed: %w", ctx.Err())
	}
}

func (q ruleQuery) conditionText() string {
	if strings.TrimSpace(q.Query) == "" {
		return ""
	}
	query := strings.TrimSpace(q.Query)
	if q.WindowSeconds > 0 {
		query = fmt.Sprintf("%s [window=%s]", query, formatRuleQueryWindow(q.WindowSeconds))
	}
	if !q.HasCompare || strings.TrimSpace(q.Operator) == "" {
		return query
	}
	return strings.TrimSpace(fmt.Sprintf("%s %s %s", query, q.Operator, strconv.FormatFloat(q.Threshold, 'f', -1, 64)))
}

func sampleMatchesQuery(sample *prommodel.Sample, query ruleQuery) bool {
	if !query.HasCompare {
		return true
	}
	return compareSample(float64(sample.Value), query.Operator, query.Threshold)
}

func compareSample(value float64, operator string, expected float64) bool {
	switch strings.TrimSpace(operator) {
	case ">":
		return value > expected
	case ">=":
		return value >= expected
	case "<":
		return value < expected
	case "<=":
		return value <= expected
	case "=", "==":
		return value == expected
	case "!=":
		return value != expected
	default:
		return true
	}
}

func buildSampleLabels(base model.JSONMap, metric prommodel.Metric, query ruleQuery) model.JSONMap {
	labels := mergeLabels(base, metricToJSONMap(metric))
	if query.Level != "" {
		labels["level"] = query.Level
	}
	if query.Severity != "" {
		labels["severity"] = query.Severity
	}
	return labels
}

func mergeLabels(base model.JSONMap, extra map[string]interface{}) model.JSONMap {
	merged := make(model.JSONMap, len(base)+len(extra))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}

func extractRuleQueries(rule model.AlertRule) []ruleQuery {
	queries := make([]ruleQuery, 0)
	if rule.RuleConfig != nil {
		if rawQueries, ok := rule.RuleConfig["queries"].([]interface{}); ok {
			for _, raw := range rawQueries {
				item, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				query := ruleQuery{
					Query:    firstString(item, "prom_ql", "promQl", "promql", "log_ql", "logQl", "logql", "logs_ql", "logsQl", "logsql", "query"),
					Level:    firstString(item, "level", "priority"),
					Severity: normalizeSeverity(firstString(item, "severity")),
				}
				if query.Severity == "" {
					query.Severity = normalizeSeverity(rule.Severity)
				}
				if query.Operator = firstString(item, "conditionOperator", "operator"); query.Operator != "" {
					if threshold, ok := firstFloat(item, "conditionValue", "threshold", "value"); ok {
						query.Threshold = threshold
						query.HasCompare = true
					}
				}
				query.WindowSeconds = firstDurationSeconds(item, "windowSeconds", "lookbackSeconds", "rangeSeconds", "durationSeconds", "window", "lookback", "range")
				if query.Query != "" {
					queries = append(queries, query)
				}
			}
		}
	}

	if len(queries) == 0 {
		query := strings.TrimSpace(rule.Query)
		if query != "" {
			queries = append(queries, ruleQuery{
				Query:      query,
				Severity:   normalizeSeverity(rule.Severity),
				Operator:   rule.ConditionOperator,
				Threshold:  rule.ConditionValue,
				HasCompare: rule.ConditionOperator != "",
			})
		}
	}
	return queries
}

func formatRuleQueryWindow(seconds int64) string {
	if seconds <= 0 {
		return ""
	}
	duration := time.Duration(seconds) * time.Second
	if duration%time.Hour == 0 {
		return fmt.Sprintf("%dh", int64(duration/time.Hour))
	}
	if duration%time.Minute == 0 {
		return fmt.Sprintf("%dm", int64(duration/time.Minute))
	}
	return fmt.Sprintf("%ds", seconds)
}

func firstString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func firstFloat(values map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok {
			continue
		}
		switch value := raw.(type) {
		case float64:
			return value, true
		case float32:
			return float64(value), true
		case int:
			return float64(value), true
		case int64:
			return float64(value), true
		case jsonNumber:
			floatValue, err := strconv.ParseFloat(string(value), 64)
			return floatValue, err == nil
		case string:
			floatValue, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			return floatValue, err == nil
		}
	}
	return math.NaN(), false
}

func firstDurationSeconds(values map[string]interface{}, keys ...string) int64 {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case float64:
			return int64(value)
		case float32:
			return int64(value)
		case int:
			return int64(value)
		case int64:
			return value
		case int32:
			return int64(value)
		case jsonNumber:
			parsed, err := strconv.ParseInt(string(value), 10, 64)
			if err == nil {
				return parsed
			}
		case string:
			text := strings.TrimSpace(value)
			if text == "" {
				continue
			}
			if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
				return parsed
			}
			if duration, err := time.ParseDuration(text); err == nil {
				return int64(duration.Seconds())
			}
		}
	}
	return 0
}

type jsonNumber string

func normalizeSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "p0", "critical", "serious", "fatal":
		return "critical"
	case "p1", "warning", "warn":
		return "warning"
	case "p2", "info", "notice":
		return "info"
	default:
		return strings.TrimSpace(value)
	}
}

func severityRank(severity, level string) int {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "p0":
		return 400
	case "p1":
		return 300
	case "p2":
		return 200
	case "p3":
		return 100
	}
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 400
	case "warning":
		return 300
	case "info":
		return 200
	default:
		return 0
	}
}

func labelString(labels model.JSONMap, key string) string {
	if labels == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(labels[key]))
}
