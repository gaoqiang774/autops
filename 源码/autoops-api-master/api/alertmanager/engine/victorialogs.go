package engine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dodevops-api/api/alertmanager/model"

	prommodel "github.com/prometheus/common/model"
)

const defaultVictoriaLogsAlertWindowSeconds int64 = 300

type VictoriaLogsCountResult struct {
	Query         string                 `json:"query"`
	Count         int64                  `json:"count"`
	Start         string                 `json:"start"`
	End           string                 `json:"end"`
	WindowSeconds int64                  `json:"windowSeconds"`
	Raw           map[string]interface{} `json:"raw,omitempty"`
}

type VictoriaLogsSampleLogsResult struct {
	Query         string                   `json:"query"`
	Count         int                      `json:"count"`
	Limit         int                      `json:"limit"`
	Start         string                   `json:"start"`
	End           string                   `json:"end"`
	WindowSeconds int64                    `json:"windowSeconds"`
	Logs          []map[string]interface{} `json:"logs"`
}

type victoriaLogsHitsResponse struct {
	Hits []victoriaLogsHitSeries `json:"hits"`
}

type victoriaLogsHitSeries struct {
	Fields map[string]string `json:"fields"`
	Values []interface{}     `json:"values"`
	Total  interface{}       `json:"total"`
}

func evaluateVictoriaLogsRule(ctx context.Context, rule model.AlertRule, ds model.AlertDatasource) ([]alertSample, error) {
	if strings.TrimSpace(ds.URL) == "" {
		return nil, fmt.Errorf("datasource url is empty")
	}
	release, err := acquireRuleEvalSlot(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	queries := extractRuleQueries(rule)
	if len(queries) == 0 {
		return nil, fmt.Errorf("query is empty")
	}

	bestByFingerprint := make(map[string]alertSample)
	for _, query := range queries {
		windowSeconds := normalizeVictoriaLogsAlertWindow(rule, query)
		query.WindowSeconds = windowSeconds
		result, err := QueryVictoriaLogsCount(ctx, ds, query.Query, time.Now(), windowSeconds, rule.RuleConfig)
		if err != nil {
			return nil, fmt.Errorf("query %q failed: %w", query.Query, err)
		}
		value := float64(result.Count)
		if query.HasCompare && !compareSample(value, query.Operator, query.Threshold) {
			continue
		}

		metric := prommodel.Metric{
			"__name__":        "victorialogs_hits",
			"datasource_type": prommodel.LabelValue(model.AlertDatasourceVictoriaLogs),
			"query":           prommodel.LabelValue(query.Query),
			"window":          prommodel.LabelValue(formatRuleQueryWindow(windowSeconds)),
		}
		fp := fingerprint(rule.ID, metric)
		candidate := alertSample{
			Fingerprint:      fp,
			Labels:           buildSampleLabels(rule.Labels, metric, query),
			Severity:         query.Severity,
			Value:            strconv.FormatInt(result.Count, 10),
			Metric:           metric,
			TriggerQuery:     query.Query,
			TriggerCondition: query.conditionText(),
		}
		current, exists := bestByFingerprint[fp]
		if !exists || severityRank(candidate.Severity, query.Level) > severityRank(current.Severity, labelString(current.Labels, "level")) {
			bestByFingerprint[fp] = candidate
		}
	}

	samples := make([]alertSample, 0, len(bestByFingerprint))
	for _, sample := range bestByFingerprint {
		samples = append(samples, sample)
	}
	return samples, nil
}

func QueryVictoriaLogsCount(ctx context.Context, ds model.AlertDatasource, query string, end time.Time, windowSeconds int64, ruleConfig ...model.JSONMap) (VictoriaLogsCountResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return VictoriaLogsCountResult{}, fmt.Errorf("query is empty")
	}
	windowSeconds = normalizeWindowSeconds(windowSeconds)
	if end.IsZero() {
		end = time.Now()
	}
	start := end.Add(-time.Duration(windowSeconds) * time.Second)

	form := url.Values{}
	form.Set("query", query)
	form.Set("start", start.UTC().Format(time.RFC3339Nano))
	form.Set("end", end.UTC().Format(time.RFC3339Nano))
	form.Set("step", formatRuleQueryWindow(windowSeconds))
	addVictoriaLogsScopeFilters(form, firstVictoriaLogsRuleConfig(ruleConfig...))

	body, err := callVictoriaLogs(ctx, ds, "/select/logsql/hits", form)
	if err != nil {
		return VictoriaLogsCountResult{}, err
	}

	var raw victoriaLogsHitsResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return VictoriaLogsCountResult{}, fmt.Errorf("decode hits response failed: %w", err)
	}

	var rawMap map[string]interface{}
	_ = json.Unmarshal(body, &rawMap)

	return VictoriaLogsCountResult{
		Query:         query,
		Count:         sumVictoriaLogsHits(raw.Hits),
		Start:         start.UTC().Format(time.RFC3339Nano),
		End:           end.UTC().Format(time.RFC3339Nano),
		WindowSeconds: windowSeconds,
		Raw:           rawMap,
	}, nil
}

func QueryVictoriaLogsSampleLogs(ctx context.Context, ds model.AlertDatasource, query string, end time.Time, windowSeconds int64, limit int, ruleConfig ...model.JSONMap) (VictoriaLogsSampleLogsResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return VictoriaLogsSampleLogsResult{}, fmt.Errorf("query is empty")
	}
	windowSeconds = normalizeWindowSeconds(windowSeconds)
	limit = normalizeSampleLogsLimit(limit)
	if end.IsZero() {
		end = time.Now()
	}
	start := end.Add(-time.Duration(windowSeconds) * time.Second)

	form := url.Values{}
	form.Set("query", query)
	form.Set("start", start.UTC().Format(time.RFC3339Nano))
	form.Set("end", end.UTC().Format(time.RFC3339Nano))
	form.Set("limit", strconv.Itoa(limit))
	addVictoriaLogsScopeFilters(form, firstVictoriaLogsRuleConfig(ruleConfig...))

	body, err := callVictoriaLogs(ctx, ds, "/select/logsql/query", form)
	if err != nil {
		return VictoriaLogsSampleLogsResult{}, err
	}
	logs, err := decodeVictoriaLogsLogLines(body)
	if err != nil {
		return VictoriaLogsSampleLogsResult{}, err
	}
	return VictoriaLogsSampleLogsResult{
		Query:         query,
		Count:         len(logs),
		Limit:         limit,
		Start:         start.UTC().Format(time.RFC3339Nano),
		End:           end.UTC().Format(time.RFC3339Nano),
		WindowSeconds: windowSeconds,
		Logs:          logs,
	}, nil
}

func callVictoriaLogs(ctx context.Context, ds model.AlertDatasource, endpoint string, form url.Values) ([]byte, error) {
	targetURL := strings.TrimRight(strings.TrimSpace(ds.URL), "/") + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	applyVictoriaLogsAuth(req, ds)
	applyVictoriaLogsConfigHeaders(req, ds)

	client := &http.Client{Timeout: victoriaLogsDatasourceTimeout(ds)}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("VictoriaLogs returned HTTP %d: %s", resp.StatusCode, truncateBodyForError(body))
	}
	return body, nil
}

func firstVictoriaLogsRuleConfig(configs ...model.JSONMap) model.JSONMap {
	for _, config := range configs {
		if len(config) > 0 {
			return config
		}
	}
	return nil
}

func addVictoriaLogsScopeFilters(form url.Values, config model.JSONMap) {
	if config == nil {
		return
	}
	extraFilters := map[string]interface{}{}
	if projectCode := stringFromJSONMap(config, "projectCode", "project_code"); projectCode != "" {
		extraFilters["project_code"] = projectCode
	}
	if storeCode := stringFromJSONMap(config, "storeCode", "store_code"); storeCode != "" {
		extraFilters["store_code"] = storeCode
	}
	if len(extraFilters) == 0 {
		return
	}
	payload, err := json.Marshal(extraFilters)
	if err != nil {
		return
	}
	form.Add("extra_filters", string(payload))
}

func applyVictoriaLogsAuth(req *http.Request, ds model.AlertDatasource) {
	switch strings.ToLower(strings.TrimSpace(ds.AuthType)) {
	case "basic":
		req.SetBasicAuth(ds.Username, ds.Password)
	case "bearer":
		if token := strings.TrimSpace(ds.Token); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
}

func applyVictoriaLogsConfigHeaders(req *http.Request, ds model.AlertDatasource) {
	if ds.Config == nil {
		return
	}
	if accountID := stringFromJSONMap(ds.Config, "accountID", "accountId", "account_id"); accountID != "" {
		req.Header.Set("AccountID", accountID)
	}
	if projectID := stringFromJSONMap(ds.Config, "projectID", "projectId", "project_id"); projectID != "" {
		req.Header.Set("ProjectID", projectID)
	}
	if rawHeaders, ok := ds.Config["headers"].(map[string]interface{}); ok {
		for key, value := range rawHeaders {
			key = strings.TrimSpace(key)
			if key == "" || value == nil {
				continue
			}
			req.Header.Set(key, strings.TrimSpace(fmt.Sprint(value)))
		}
	}
}

func victoriaLogsDatasourceTimeout(ds model.AlertDatasource) time.Duration {
	if ds.Config != nil {
		if seconds := durationSecondsFromJSONMap(ds.Config, "timeoutSeconds", "timeout_seconds", "timeout"); seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return 15 * time.Second
}

func normalizeVictoriaLogsAlertWindow(rule model.AlertRule, query ruleQuery) int64 {
	if query.WindowSeconds > 0 {
		return normalizeWindowSeconds(query.WindowSeconds)
	}
	if rule.RuleConfig != nil {
		if seconds := durationSecondsFromJSONMap(rule.RuleConfig, "windowSeconds", "lookbackSeconds", "rangeSeconds", "durationSeconds", "window", "lookback", "range"); seconds > 0 {
			return normalizeWindowSeconds(seconds)
		}
	}
	return defaultVictoriaLogsAlertWindowSeconds
}

func normalizeWindowSeconds(seconds int64) int64 {
	if seconds <= 0 {
		return defaultVictoriaLogsAlertWindowSeconds
	}
	return seconds
}

func normalizeSampleLogsLimit(limit int) int {
	if limit <= 0 {
		return 5
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func sumVictoriaLogsHits(hits []victoriaLogsHitSeries) int64 {
	var total int64
	for _, hit := range hits {
		if value, ok := numericInt64(hit.Total); ok && value > 0 {
			total += value
			continue
		}
		for _, value := range hit.Values {
			if parsed, ok := numericInt64(value); ok {
				total += parsed
			}
		}
	}
	return total
}

func decodeVictoriaLogsLogLines(body []byte) ([]map[string]interface{}, error) {
	logs := make([]map[string]interface{}, 0)
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item map[string]interface{}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("decode log line failed: %w", err)
		}
		logs = append(logs, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}

func numericInt64(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case nil:
		return 0, false
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case int32:
		return int64(typed), true
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, false
		}
		return int64(typed), true
	case float32:
		if math.IsNaN(float64(typed)) || math.IsInf(float64(typed), 0) {
			return 0, false
		}
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed, true
		}
		floatValue, err := strconv.ParseFloat(string(typed), 64)
		return int64(floatValue), err == nil
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0, false
		}
		parsed, err := strconv.ParseInt(text, 10, 64)
		if err == nil {
			return parsed, true
		}
		floatValue, err := strconv.ParseFloat(text, 64)
		return int64(floatValue), err == nil
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" || text == "<nil>" {
			return 0, false
		}
		parsed, err := strconv.ParseInt(text, 10, 64)
		if err == nil {
			return parsed, true
		}
		floatValue, err := strconv.ParseFloat(text, 64)
		return int64(floatValue), err == nil
	}
}

func stringFromJSONMap(values model.JSONMap, keys ...string) string {
	for _, key := range keys {
		if raw, ok := values[key]; ok && raw != nil {
			text := strings.TrimSpace(fmt.Sprint(raw))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func durationSecondsFromJSONMap(values model.JSONMap, keys ...string) int64 {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok || raw == nil {
			continue
		}
		switch typed := raw.(type) {
		case int:
			return int64(typed)
		case int64:
			return typed
		case int32:
			return int64(typed)
		case float64:
			return int64(typed)
		case float32:
			return int64(typed)
		case json.Number:
			parsed, err := typed.Int64()
			if err == nil {
				return parsed
			}
		case string:
			text := strings.TrimSpace(typed)
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

func truncateBodyForError(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) <= 500 {
		return text
	}
	return text[:500] + "...(truncated)"
}

func isVictoriaLogsDatasource(datasourceType string) bool {
	return strings.TrimSpace(datasourceType) == model.AlertDatasourceVictoriaLogs
}
