package service

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	logtailmodel "dodevops-api/api/logtail/model"
	"dodevops-api/common/config"

	"github.com/gin-gonic/gin"
)

const (
	vlogsEndpointQuery           = "/select/logsql/query"
	vlogsEndpointHits            = "/select/logsql/hits"
	vlogsEndpointFieldNames      = "/select/logsql/field_names"
	vlogsEndpointFieldValues     = "/select/logsql/field_values"
	vlogsEndpointStatsQueryRange = "/select/logsql/stats_query_range"

	defaultLogQueryRange = 15 * time.Minute
	defaultLogQueryLimit = 50
	maxLogQueryLimit     = 1000

	defaultLogFieldValuesLimit = 5
	maxLogFieldValuesLimit     = 100

	defaultLogDownloadLimit = 10000
	maxLogDownloadLimit     = 100000
)

var logFieldNamePattern = regexp.MustCompile(`^[@A-Za-z_][@A-Za-z0-9_.-]*$`)
var simpleLogFieldNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var logQueryAggregatePipePattern = regexp.MustCompile(`(?i)\|\s*(stats|top|facets|count\s*\()`)
var logQuerySyntaxPattern = regexp.MustCompile(`[\s:|()[\]{},'"=<>~!*?]`)
var logQueryUUIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var logQueryIdentifierFields = []string{
	"trace_id",
	"x_zai_trace_id",
	"span_id",
	"request_id",
	"session_id",
}
var defaultLogQueryFields = []string{
	"_time",
	"message",
	"project_code",
	"store_code",
	"collect_task_name",
	"collect_task_code",
	"env",
	"app",
	"service",
	"level",
	"severity",
	"log_source",
	"log_type",
	"host_name",
	"hostname",
	"agent_id",
	"host_id",
	"file_path",
	"log_file_name",
	"cluster",
	"severity_text",
	"trace_id",
	"span_id",
	"k8s_container_name",
	"k8s_log_source",
	"k8s_cluster_name",
	"k8s_namespace",
	"k8s_workload_type",
	"k8s_workload_name",
	"k8s_pod_name",
	"k8s_node_name",
}

var commonLogIndexFields = []string{
	"env",
	"app",
	"service",
	"host",
	"host_name",
	"hostname",
	"file_path",
	"log_file_name",
	"path",
	"collect_task_name",
	"collect_task_code",
	"log_source",
	"log_type",
	"level",
	"severity",
	"status_text",
	"success",
	"error",
	"error_code",
	"error_msg",
	"errorMsg",
	"timeout",
	"request_id",
	"trace_id",
	"x_zai_trace_id",
	"cluster",
	"k8s_log_source",
	"k8s_cluster_name",
	"k8s_namespace",
	"k8s_workload_type",
	"k8s_workload_name",
	"k8s_pod_name",
	"k8s_container_name",
	"k8s_node_name",
}

var builtinLogParseFields = []string{
	"remote_addr",
	"remote_user",
	"request_method",
	"request_uri",
	"request_path",
	"request_query",
	"http_version",
	"status",
	"body_bytes_sent",
	"http_referer",
	"http_user_agent",
	"slow_time",
	"db_user",
	"db_host",
	"thread_id",
	"database",
	"last_errno",
	"killed",
	"query_time",
	"lock_time",
	"rows_sent",
	"rows_examined",
	"rows_affected",
	"bytes_sent",
	"use_database",
	"unix_ts",
	"sql_text",
	"sql_operation",
}

var logTypeIndexFields = map[string][]string{
	"nginx": {
		"domain",
		"remote_addr",
		"clientip",
		"client_ip",
		"ip",
		"upstreamaddr",
		"request_method",
		"request",
		"request_uri",
		"request_path",
		"request_query",
		"status",
		"code",
		"body_bytes_sent",
		"size",
		"request_time",
		"responsetime",
		"upstream_response_time",
		"upstreamtime",
		"http_referer",
		"referer",
		"http_user_agent",
		"http_version",
		"args",
		"x_zai_user",
		"x_zeroai_user",
		"has_x_zai_api_key",
	},
	"db_slow_log": {
		"slow_time",
		"db_user",
		"db_host",
		"thread_id",
		"database",
		"query_time",
		"lock_time",
		"rows_sent",
		"rows_examined",
		"rows_affected",
		"bytes_sent",
		"sql_operation",
	},
	"json":  nil,
	"regex": nil,
	"txt":   {},
	"raw":   {},
}

type victoriaLogsTenant struct {
	AccountID string
	ProjectID string
}

type victoriaLogsValuesResponse struct {
	Values []logtailmodel.LogQueryFieldItem `json:"values"`
}

type victoriaLogsHitsResponse struct {
	Hits []logtailmodel.LogQueryHitSeries `json:"hits"`
}

type logDownloadFile struct {
	FileName    string
	ContentType string
	Data        []byte
}

func (s *LogtailService) QueryLogs(c *gin.Context, req logtailmodel.LogQueryLogsRequest) (*logtailmodel.LogQueryLogsResponse, error) {
	store, query, form, tenant, start, end, err := s.prepareVictoriaLogsRequest(c, req.LogQueryBaseRequest)
	if err != nil {
		return nil, err
	}
	query, fields, deferFields, err := prepareLogQueryFields(query, req.Fields)
	if err != nil {
		return nil, err
	}
	form.Set("query", query)

	limit := normalizeLogQueryLimit(req.Limit, req.PageSize)
	offset := normalizeLogQueryOffset(req.Offset, req.Page, req.PageSize, limit)
	form.Set("limit", strconv.Itoa(limit))
	if offset > 0 {
		form.Set("offset", strconv.Itoa(offset))
	}

	startedAt := time.Now()
	body, headers, err := s.callVictoriaLogs(vlogsEndpointQuery, form, tenant)
	if err != nil {
		return nil, err
	}
	logs, err := decodeVictoriaLogsLines(body)
	if err != nil {
		return nil, err
	}
	normalizeLogRowsForDisplay(logs)

	if deferFields {
		fields = deriveLogFieldsFromRows(logs)
	}
	logType := detectLogResultType(query, logs)
	displayMode := detectLogDisplayMode(query, logType)
	tableColumns := buildLogQueryTableColumns(displayMode, logType, logs)

	total := int64(0)
	if req.IncludeTotal {
		total, _ = s.countLogsByHits(c, req.LogQueryBaseRequest)
	}

	return &logtailmodel.LogQueryLogsResponse{
		LogQueryResponseMeta: logtailmodel.LogQueryResponseMeta{
			Store:                   toQueryStoreRef(store),
			Query:                   query,
			Start:                   start,
			End:                     end,
			DurationMs:              time.Since(startedAt).Milliseconds(),
			VictoriaLogsDurationSec: headers.Get("VL-Request-Duration-Seconds"),
		},
		Limit:           limit,
		Offset:          offset,
		Count:           len(logs),
		Total:           total,
		HasMore:         len(logs) >= limit,
		Fields:          fields,
		Logs:            logs,
		DisplayMode:     displayMode,
		LogType:         logType,
		TableColumns:    tableColumns,
		AnalysisPresets: buildLogAnalysisPresets(logType),
	}, nil
}

func (s *LogtailService) DownloadLogs(c *gin.Context, req logtailmodel.LogQueryDownloadRequest) (*logDownloadFile, error) {
	store, query, form, tenant, _, _, err := s.prepareVictoriaLogsRequest(c, req.LogQueryBaseRequest)
	if err != nil {
		return nil, err
	}
	query, fields, deferFields, err := prepareLogQueryFields(query, req.Fields)
	if err != nil {
		return nil, err
	}
	limit := normalizeLogDownloadLimit(req.Limit)
	form.Set("query", query)
	form.Set("limit", strconv.Itoa(limit))
	if req.Offset > 0 {
		form.Set("offset", strconv.Itoa(req.Offset))
	}

	body, _, err := s.callVictoriaLogs(vlogsEndpointQuery, form, tenant)
	if err != nil {
		return nil, err
	}
	logs, err := decodeVictoriaLogsLines(body)
	if err != nil {
		return nil, err
	}
	normalizeLogRowsForDisplay(logs)
	if deferFields || len(fields) == 0 {
		fields = deriveLogFieldsFromRows(logs)
	}

	format := normalizeLogDownloadFormat(req.Format)
	data, contentType, ext, err := buildLogDownloadContent(logs, fields, format)
	if err != nil {
		return nil, err
	}
	fileName := buildLogDownloadFileName(store, format, ext)
	return &logDownloadFile{
		FileName:    fileName,
		ContentType: contentType,
		Data:        data,
	}, nil
}

func (s *LogtailService) QueryHits(c *gin.Context, req logtailmodel.LogQueryHitsRequest) (*logtailmodel.LogQueryHitsResponse, error) {
	store, query, form, tenant, start, end, err := s.prepareVictoriaLogsRequest(c, req.LogQueryBaseRequest)
	if err != nil {
		return nil, err
	}
	step := strings.TrimSpace(req.Step)
	if step == "" {
		step = "1m"
	}
	form.Set("step", step)
	if offset := strings.TrimSpace(req.Offset); offset != "" {
		form.Set("offset", offset)
	}
	if req.FieldsLimit > 0 {
		form.Set("fields_limit", strconv.Itoa(req.FieldsLimit))
	}
	for _, field := range req.Fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if !isValidLogFieldName(field) {
			return nil, fmt.Errorf("字段名不合法: %s", field)
		}
		form.Add("field", field)
	}

	startedAt := time.Now()
	body, headers, err := s.callVictoriaLogs(vlogsEndpointHits, form, tenant)
	if err != nil {
		return nil, err
	}
	var raw victoriaLogsHitsResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("解析 VictoriaLogs hits 响应失败: %w", err)
	}
	for i := range raw.Hits {
		for j, timestamp := range raw.Hits[i].Timestamps {
			raw.Hits[i].Timestamps[j] = normalizeLogTimeForDisplay(timestamp)
		}
	}

	return &logtailmodel.LogQueryHitsResponse{
		LogQueryResponseMeta: logtailmodel.LogQueryResponseMeta{
			Store:                   toQueryStoreRef(store),
			Query:                   query,
			Start:                   start,
			End:                     end,
			DurationMs:              time.Since(startedAt).Milliseconds(),
			VictoriaLogsDurationSec: headers.Get("VL-Request-Duration-Seconds"),
		},
		Step: step,
		Hits: raw.Hits,
	}, nil
}

func (s *LogtailService) QueryFieldNames(c *gin.Context, req logtailmodel.LogQueryFieldsRequest) (*logtailmodel.LogQueryFieldsResponse, error) {
	store, query, form, tenant, start, end, err := s.prepareVictoriaLogsRequest(c, req.LogQueryBaseRequest)
	if err != nil {
		return nil, err
	}
	if filter := strings.TrimSpace(req.Filter); filter != "" {
		form.Set("filter", filter)
	}

	startedAt := time.Now()
	body, headers, err := s.callVictoriaLogs(vlogsEndpointFieldNames, form, tenant)
	if err != nil {
		return nil, err
	}
	var raw victoriaLogsValuesResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("解析 VictoriaLogs 字段名响应失败: %w", err)
	}

	detectedLogType := detectLogTypeFromFieldItems(query, raw.Values)
	indexFieldDefaults := logIndexFieldDefaultsForType(detectedLogType)
	indexFields := mergeLogFieldItems(raw.Values, indexFieldDefaults, req.Filter, true, true)
	normalFields := buildNormalLogFieldItems(raw.Values, indexFields, req.Filter)
	total := maxLogFieldItemHits(raw.Values)
	indexFields = enrichLogFieldItems(indexFields, total)
	normalFields = enrichLogFieldItems(normalFields, total)
	values := append([]logtailmodel.LogQueryFieldItem{}, indexFields...)
	values = append(values, normalFields...)

	return &logtailmodel.LogQueryFieldsResponse{
		LogQueryResponseMeta: logtailmodel.LogQueryResponseMeta{
			Store:                   toQueryStoreRef(store),
			Query:                   query,
			Start:                   start,
			End:                     end,
			DurationMs:              time.Since(startedAt).Milliseconds(),
			VictoriaLogsDurationSec: headers.Get("VL-Request-Duration-Seconds"),
		},
		Values:          values,
		NormalFields:    normalFields,
		IndexFields:     indexFields,
		QuickFilters:    buildLogQuickFilters(detectedLogType),
		AnalysisPresets: buildLogAnalysisPresets(detectedLogType),
	}, nil
}

func (s *LogtailService) QueryFieldValues(c *gin.Context, req logtailmodel.LogQueryFieldValuesRequest) (*logtailmodel.LogQueryFieldsResponse, error) {
	store, query, form, tenant, start, end, err := s.prepareVictoriaLogsRequest(c, req.LogQueryBaseRequest)
	if err != nil {
		return nil, err
	}
	field := strings.TrimSpace(req.Field)
	if field == "" {
		return nil, errors.New("字段名不能为空")
	}
	if !isValidLogFieldName(field) {
		return nil, fmt.Errorf("字段名不合法: %s", field)
	}
	form.Set("field", field)
	limit := normalizeLogFieldValuesLimit(req.Limit)

	startedAt := time.Now()
	statsQuery := buildLogFieldValuesStatsQuery(query, field, limit)
	form.Set("query", statsQuery)
	form.Set("limit", strconv.Itoa(limit))
	body, headers, err := s.callVictoriaLogs(vlogsEndpointQuery, form, tenant)
	if err != nil {
		return nil, err
	}
	rows, err := decodeVictoriaLogsLines(body)
	if err != nil {
		return nil, fmt.Errorf("解析 VictoriaLogs 字段值响应失败: %w", err)
	}
	total, _ := s.countLogsByHits(c, req.LogQueryBaseRequest)
	values := buildLogFieldValueItems(rows, field, req.Filter, limit)
	values = appendOtherLogFieldItem(values, total)
	values = enrichLogFieldItems(values, total)

	return &logtailmodel.LogQueryFieldsResponse{
		LogQueryResponseMeta: logtailmodel.LogQueryResponseMeta{
			Store:                   toQueryStoreRef(store),
			Query:                   query,
			Start:                   start,
			End:                     end,
			DurationMs:              time.Since(startedAt).Milliseconds(),
			VictoriaLogsDurationSec: headers.Get("VL-Request-Duration-Seconds"),
		},
		Values: values,
	}, nil
}

func (s *LogtailService) QueryStatsRange(c *gin.Context, req logtailmodel.LogQueryStatsRangeRequest) (*logtailmodel.LogQueryStatsRangeResponse, error) {
	store, query, form, tenant, start, end, err := s.prepareVictoriaLogsRequest(c, req.LogQueryBaseRequest)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(strings.ToLower(query), "| stats") {
		return nil, errors.New("stats range 查询需要包含 LogsQL stats 管道")
	}
	step := strings.TrimSpace(req.Step)
	if step == "" {
		step = "1m"
	}
	form.Set("step", step)
	if offset := strings.TrimSpace(req.Offset); offset != "" {
		form.Set("offset", offset)
	}

	startedAt := time.Now()
	body, headers, err := s.callVictoriaLogs(vlogsEndpointStatsQueryRange, form, tenant)
	if err != nil {
		return nil, err
	}
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("解析 VictoriaLogs stats_range 响应失败: %w", err)
	}
	normalizeLogTimeValuesForDisplay(raw)

	return &logtailmodel.LogQueryStatsRangeResponse{
		LogQueryResponseMeta: logtailmodel.LogQueryResponseMeta{
			Store:                   toQueryStoreRef(store),
			Query:                   query,
			Start:                   start,
			End:                     end,
			DurationMs:              time.Since(startedAt).Milliseconds(),
			VictoriaLogsDurationSec: headers.Get("VL-Request-Duration-Seconds"),
		},
		Step: step,
		Raw:  raw,
	}, nil
}

func (s *LogtailService) prepareVictoriaLogsRequest(c *gin.Context, base logtailmodel.LogQueryBaseRequest) (*logtailmodel.LogStore, string, url.Values, victoriaLogsTenant, string, string, error) {
	var store *logtailmodel.LogStore
	if base.StoreID > 0 {
		var err error
		store, err = s.storeDAO.GetByID(base.StoreID)
		if err != nil {
			return nil, "", nil, victoriaLogsTenant{}, "", "", fmt.Errorf("查询日志库失败: %w", err)
		}
		if strings.TrimSpace(store.Code) == "" {
			return nil, "", nil, victoriaLogsTenant{}, "", "", errors.New("日志库编码不能为空")
		}
	} else if storeCode := strings.TrimSpace(base.StoreCode); storeCode != "" {
		store = &logtailmodel.LogStore{Code: storeCode, Name: storeCode}
	} else if !base.Direct {
		return nil, "", nil, victoriaLogsTenant{}, "", "", errors.New("日志库ID不能为空；如需直查现有 VictoriaLogs 日志，请传 direct=true")
	}
	if err := s.ensureLogStoreQueryPermission(c, store); err != nil {
		return nil, "", nil, victoriaLogsTenant{}, "", "", err
	}

	start, end := normalizeVictoriaLogsTimeRange(base.Start, base.End)
	query := normalizeVictoriaLogsQuery(base.Query, base.Keyword)
	extraFilters, err := buildVictoriaLogsExtraFilters(store, base.ExtraFilters)
	if err != nil {
		return nil, "", nil, victoriaLogsTenant{}, "", "", err
	}

	form := url.Values{}
	form.Set("query", query)
	form.Set("start", start)
	form.Set("end", end)
	if timeout := strings.TrimSpace(base.Timeout); timeout != "" {
		form.Set("timeout", timeout)
	}
	if len(extraFilters) > 0 {
		payload, err := json.Marshal(extraFilters)
		if err != nil {
			return nil, "", nil, victoriaLogsTenant{}, "", "", fmt.Errorf("生成 VictoriaLogs extra_filters 失败: %w", err)
		}
		form.Add("extra_filters", string(payload))
	}

	return store, query, form, resolveVictoriaLogsTenant(store, base), start, end, nil
}

func (s *LogtailService) callVictoriaLogs(endpoint string, form url.Values, tenant victoriaLogsTenant) ([]byte, http.Header, error) {
	baseURL := strings.TrimSpace(config.Config.Logtail.VictoriaLogs.URL)
	if baseURL == "" {
		return nil, nil, errors.New("VictoriaLogs URL 未配置")
	}
	targetURL := strings.TrimRight(baseURL, "/") + endpoint
	timeout := time.Duration(config.Config.Logtail.VictoriaLogs.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	httpReq, err := http.NewRequest(http.MethodPost, targetURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if tenant.AccountID != "" {
		httpReq.Header.Set("AccountID", tenant.AccountID)
	}
	if tenant.ProjectID != "" {
		httpReq.Header.Set("ProjectID", tenant.ProjectID)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("VictoriaLogs 返回 HTTP %d: %s", resp.StatusCode, truncateForError(body))
	}
	return body, resp.Header, nil
}

func (s *LogtailService) countLogsByHits(c *gin.Context, base logtailmodel.LogQueryBaseRequest) (int64, error) {
	req := logtailmodel.LogQueryHitsRequest{
		LogQueryBaseRequest: base,
		Step:                "1h",
	}
	resp, err := s.QueryHits(c, req)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, hit := range resp.Hits {
		if hit.Total > 0 {
			total += hit.Total
			continue
		}
		for _, value := range hit.Values {
			total += value
		}
	}
	return total, nil
}

func normalizeVictoriaLogsTimeRange(start, end string) (string, string) {
	now := time.Now().UTC()
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)
	if start == "" {
		start = now.Add(-defaultLogQueryRange).Format(time.RFC3339Nano)
	} else {
		start = normalizeVictoriaLogsQueryTime(start)
	}
	if end == "" {
		end = now.Format(time.RFC3339Nano)
	} else {
		end = normalizeVictoriaLogsQueryTime(end)
	}
	return start, end
}

var logDisplayLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

var logTimeFields = map[string]struct{}{
	"_time":           {},
	"@timestamp":      {},
	"timestamp":       {},
	"event_timestamp": {},
	"slow_time":       {},
	"first_seen":      {},
	"last_seen":       {},
}

func normalizeLogRowsForDisplay(rows []map[string]interface{}) {
	for _, row := range rows {
		normalizeLogTimeValuesForDisplay(row)
	}
}

func normalizeLogTimeValuesForDisplay(value interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, item := range typed {
			if _, ok := logTimeFields[strings.ToLower(strings.TrimSpace(key))]; ok {
				if text, ok := item.(string); ok {
					typed[key] = normalizeLogTimeForDisplay(text)
				}
				continue
			}
			if strings.EqualFold(strings.TrimSpace(key), "timestamps") {
				if items, ok := item.([]interface{}); ok {
					for i, timestamp := range items {
						if text, ok := timestamp.(string); ok {
							items[i] = normalizeLogTimeForDisplay(text)
						}
					}
				}
				continue
			}
			normalizeLogTimeValuesForDisplay(item)
		}
	case []interface{}:
		for _, item := range typed {
			normalizeLogTimeValuesForDisplay(item)
		}
	}
}

func normalizeLogTimeForDisplay(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return formatLogTimeForDisplay(parsed)
		}
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02 15:04:05.999999999",
		"[2006-01-02 15:04:05.999999999]",
	} {
		if parsed, err := time.ParseInLocation(layout, strings.Replace(value, ",", ".", 1), logDisplayLocation); err == nil {
			return parsed.Format(time.RFC3339Nano)
		}
	}
	return value
}

func formatLogTimeForDisplay(value time.Time) string {
	return value.In(logDisplayLocation).Format(time.RFC3339Nano)
}

func normalizeVictoriaLogsQueryTime(value string) string {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC().Format(time.RFC3339Nano)
		}
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02 15:04:05.999999999",
	} {
		if parsed, err := time.ParseInLocation(layout, strings.Replace(value, ",", ".", 1), logDisplayLocation); err == nil {
			return parsed.UTC().Format(time.RFC3339Nano)
		}
	}
	return value
}

func normalizeVictoriaLogsQuery(query, keyword string) string {
	query = strings.TrimSpace(query)
	if query != "" {
		return normalizeVictoriaLogsSearchInput(query)
	}
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		return normalizeVictoriaLogsSearchInput(keyword)
	}
	return "*"
}

func normalizeVictoriaLogsSearchInput(input string) string {
	if input == "*" || logQuerySyntaxPattern.MatchString(input) {
		return input
	}
	if logQueryUUIDPattern.MatchString(input) {
		filters := make([]string, 0, len(logQueryIdentifierFields)+1)
		filters = append(filters, "*"+input+"*")
		for _, field := range logQueryIdentifierFields {
			filters = append(filters, fmt.Sprintf(`%s:=%s`, field, strconv.Quote(input)))
		}
		return "(" + strings.Join(filters, " OR ") + ")"
	}
	return "*" + input + "*"
}

func normalizeLogQueryLimit(limit, pageSize int) int {
	if limit <= 0 {
		limit = pageSize
	}
	if limit <= 0 {
		limit = defaultLogQueryLimit
	}
	if limit > maxLogQueryLimit {
		return maxLogQueryLimit
	}
	return limit
}

func normalizeLogQueryOffset(offset, page, pageSize, limit int) int {
	if offset > 0 {
		return offset
	}
	if page <= 1 {
		return 0
	}
	if pageSize <= 0 {
		pageSize = limit
	}
	if pageSize <= 0 {
		pageSize = defaultLogQueryLimit
	}
	return (page - 1) * pageSize
}

func normalizeLogDownloadLimit(limit int) int {
	if limit <= 0 {
		return defaultLogDownloadLimit
	}
	if limit > maxLogDownloadLimit {
		return maxLogDownloadLimit
	}
	return limit
}

func normalizeLogDownloadFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json", "jsonl", "ndjson":
		return "jsonl"
	case "txt", "text", "raw":
		return "txt"
	default:
		return "csv"
	}
}

func prepareLogQueryFields(query string, requested []string) (string, []string, bool, error) {
	if hasLogAggregatePipe(query) || shouldUseDynamicLogFields(query) {
		fields := cleanRequestedLogFields(requested)
		if len(fields) == 0 {
			return query, nil, true, nil
		}
		queryWithFields, err := appendFieldsPipe(query, fields)
		return queryWithFields, fields, false, err
	}

	fields := mergeLogQueryFields(requested)
	queryWithFields, err := appendFieldsPipe(query, fields)
	return queryWithFields, fields, false, err
}

func shouldUseDynamicLogFields(query string) bool {
	switch logTypeFromQuery(query) {
	case "json", "regex":
		return true
	default:
		return false
	}
}

func hasLogAggregatePipe(query string) bool {
	return logQueryAggregatePipePattern.MatchString(query)
}

func cleanRequestedLogFields(requested []string) []string {
	fields := make([]string, 0, len(requested))
	seen := map[string]struct{}{}
	for _, field := range requested {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		fields = append(fields, field)
	}
	return fields
}

func mergeLogQueryFields(requested []string) []string {
	fields := make([]string, 0, len(defaultLogQueryFields)+len(requested))
	seen := map[string]struct{}{}
	add := func(field string) {
		field = strings.TrimSpace(field)
		if field == "" {
			return
		}
		if _, ok := seen[field]; ok {
			return
		}
		seen[field] = struct{}{}
		fields = append(fields, field)
	}
	for _, field := range defaultLogQueryFields {
		add(field)
	}
	for _, field := range builtinLogParseFields {
		add(field)
	}
	for _, field := range requested {
		add(field)
	}
	return fields
}

func appendFieldsPipe(query string, fields []string) (string, error) {
	cleanFields := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if !isValidLogFieldName(field) {
			return "", fmt.Errorf("字段名不合法: %s", field)
		}
		cleanFields = append(cleanFields, quoteLogFieldName(field))
	}
	if len(cleanFields) == 0 {
		return query, nil
	}
	return strings.TrimSpace(query) + " | fields " + strings.Join(cleanFields, ", "), nil
}

func quoteLogFieldName(field string) string {
	if simpleLogFieldNamePattern.MatchString(field) {
		return field
	}
	return strconv.Quote(field)
}

func deriveLogFieldsFromRows(rows []map[string]interface{}) []string {
	if len(rows) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	fields := make([]string, 0)
	add := func(field string) {
		field = strings.TrimSpace(field)
		if field == "" {
			return
		}
		if _, ok := seen[field]; ok {
			return
		}
		for _, row := range rows {
			if _, ok := row[field]; ok {
				seen[field] = struct{}{}
				fields = append(fields, field)
				return
			}
		}
	}
	for _, field := range defaultLogQueryFields {
		add(field)
	}
	for _, field := range builtinLogParseFields {
		add(field)
	}
	rest := make([]string, 0)
	for _, row := range rows {
		for field := range row {
			if _, ok := seen[field]; ok {
				continue
			}
			seen[field] = struct{}{}
			rest = append(rest, field)
		}
	}
	sort.Strings(rest)
	fields = append(fields, rest...)
	return fields
}

func buildLogDownloadContent(logs []map[string]interface{}, fields []string, format string) ([]byte, string, string, error) {
	switch normalizeLogDownloadFormat(format) {
	case "jsonl":
		data, err := buildLogJSONLDownload(logs)
		return data, "application/x-ndjson; charset=utf-8", "jsonl", err
	case "txt":
		return buildLogTextDownload(logs), "text/plain; charset=utf-8", "log", nil
	default:
		data, err := buildLogCSVDownload(logs, fields)
		return data, "text/csv; charset=utf-8", "csv", err
	}
}

func buildLogCSVDownload(logs []map[string]interface{}, fields []string) ([]byte, error) {
	if len(fields) == 0 {
		fields = deriveLogFieldsFromRows(logs)
	}
	var buffer bytes.Buffer
	buffer.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buffer)
	if err := writer.Write(fields); err != nil {
		return nil, err
	}
	for _, log := range logs {
		record := make([]string, 0, len(fields))
		for _, field := range fields {
			record = append(record, logDownloadValue(log[field]))
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func buildLogJSONLDownload(logs []map[string]interface{}) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	for _, log := range logs {
		if err := encoder.Encode(log); err != nil {
			return nil, err
		}
	}
	return buffer.Bytes(), nil
}

func buildLogTextDownload(logs []map[string]interface{}) []byte {
	var buffer bytes.Buffer
	for _, log := range logs {
		buffer.WriteString(logRawText(log))
		if !strings.HasSuffix(buffer.String(), "\n") {
			buffer.WriteByte('\n')
		}
	}
	return buffer.Bytes()
}

func logRawText(log map[string]interface{}) string {
	for _, field := range []string{"_raw", "message", "_msg"} {
		if value, ok := log[field]; ok {
			text := strings.TrimRight(fmt.Sprint(value), "\r\n")
			if text != "" {
				return text
			}
		}
	}
	payload, err := json.Marshal(log)
	if err != nil {
		return fmt.Sprint(log)
	}
	return string(payload)
}

func logDownloadValue(value interface{}) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		payload, err := json.Marshal(typed)
		if err == nil && (strings.HasPrefix(string(payload), "{") || strings.HasPrefix(string(payload), "[")) {
			return string(payload)
		}
		return fmt.Sprint(typed)
	}
}

func buildLogDownloadFileName(store *logtailmodel.LogStore, format, ext string) string {
	name := "logs"
	if store != nil {
		if text := strings.TrimSpace(store.Code); text != "" {
			name = text
		} else if text := strings.TrimSpace(store.Name); text != "" {
			name = text
		}
	}
	name = sanitizeLogDownloadFileName(name)
	return fmt.Sprintf("%s_%s.%s", name, time.Now().Format("20060102150405"), ext)
}

func sanitizeLogDownloadFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "logs"
	}
	var builder strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('_')
	}
	clean := strings.Trim(builder.String(), "_")
	if clean == "" {
		return "logs"
	}
	return clean
}

func detectLogResultType(query string, rows []map[string]interface{}) string {
	for _, row := range rows {
		if value, ok := row["log_type"]; ok {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
				return text
			}
		}
	}
	if logType := logTypeFromQuery(query); logType != "" {
		return logType
	}
	if rowsHaveAnyField(rows, "sql_text", "query_time", "rows_examined") {
		return "db_slow_log"
	}
	if rowsHaveAnyField(rows, "request_uri", "request_path", "remote_addr", "http_user_agent") {
		return "nginx"
	}
	return ""
}

func logTypeFromQuery(query string) string {
	if match := regexp.MustCompile(`(?i)\blog_type\s*:\s*([A-Za-z0-9_-]+)`).FindStringSubmatch(query); len(match) > 1 {
		return strings.ToLower(match[1])
	}
	return ""
}

func detectLogDisplayMode(query, logType string) string {
	if hasLogAggregatePipe(query) {
		return "aggregate"
	}
	switch strings.ToLower(strings.TrimSpace(logType)) {
	case "", "txt", "raw":
		return "raw"
	default:
		return "structured"
	}
}

func buildLogQueryTableColumns(displayMode, logType string, rows []map[string]interface{}) []logtailmodel.LogQueryTableColumn {
	switch displayMode {
	case "aggregate":
		return buildColumnsFromFields(deriveLogFieldsFromRows(rows), nil)
	}

	switch strings.ToLower(strings.TrimSpace(logType)) {
	case "db_slow_log":
		return buildColumnsFromFields([]string{
			"_time",
			"query_time",
			"rows_examined",
			"rows_sent",
			"db_user",
			"database",
			"db_host",
			"sql_operation",
			"sql_text",
			"file_path",
		}, rows)
	case "nginx":
		return buildColumnsFromFields([]string{
			"_time",
			"remote_addr",
			"request_method",
			"request_path",
			"status",
			"body_bytes_sent",
			"http_referer",
			"http_user_agent",
			"host_name",
		}, rows)
	case "json", "regex":
		return buildStructuredDynamicColumns(rows)
	default:
		return buildColumnsFromFields([]string{
			"_time",
			"level",
			"app",
			"host_name",
			"file_path",
			"log_source",
			"message",
		}, rows)
	}
}

func buildStructuredDynamicColumns(rows []map[string]interface{}) []logtailmodel.LogQueryTableColumn {
	fields := make([]string, 0)
	for _, field := range []string{"_time", "level", "severity", "app", "service", "host_name", "message"} {
		if rowsHaveAnyField(rows, field) {
			fields = append(fields, field)
		}
	}
	for _, field := range deriveLogFieldsFromRows(rows) {
		if isLogMetaField(field) || field == "_raw" || containsField(fields, field) {
			continue
		}
		fields = append(fields, field)
	}
	if len(fields) == 0 {
		return buildColumnsFromFields([]string{"_time", "message", "_raw"}, rows)
	}
	return buildColumnsFromFields(fields, rows)
}

func buildColumnsFromFields(fields []string, rows []map[string]interface{}) []logtailmodel.LogQueryTableColumn {
	columns := make([]logtailmodel.LogQueryTableColumn, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" || (len(rows) > 0 && !rowsHaveAnyField(rows, field)) {
			continue
		}
		columns = append(columns, logtailmodel.LogQueryTableColumn{
			Field:    field,
			Title:    logQueryFieldTitle(field),
			Type:     logQueryFieldType(field),
			Width:    logQueryFieldWidth(field),
			Sortable: logQueryFieldSortable(field),
			Ellipsis: logQueryFieldEllipsis(field),
			Primary:  field == "message" || field == "sql_text" || field == "request_path",
		})
	}
	return columns
}

func buildLogAnalysisPresets(logType string) []logtailmodel.LogQueryAnalysisPreset {
	switch strings.ToLower(strings.TrimSpace(logType)) {
	case "nginx":
		return []logtailmodel.LogQueryAnalysisPreset{
			{
				Key:         "nginx_status_ratio",
				Title:       "HTTP 状态码占比",
				Description: "按状态码统计请求数和占比，适合查看 404/405/503 等错误占比",
				LogType:     "nginx",
				Query:       `log_type:nginx | stats by (status) count() as requests | sort by (requests desc)`,
				ResultType:  "ratio_table",
				Columns:     buildColumnsFromFields([]string{"status", "requests", "ratio"}, nil),
			},
			{
				Key:         "nginx_error_class_ratio",
				Title:       "4xx/5xx 占比",
				Description: "分别统计 4xx 和 5xx 请求量占比",
				LogType:     "nginx",
				Query:       `log_type:nginx status:>=400 | stats by (status) count() as errors | sort by (errors desc)`,
				ResultType:  "ratio_table",
				Columns:     buildColumnsFromFields([]string{"status", "errors", "ratio"}, nil),
			},
			{
				Key:         "nginx_top_request_path",
				Title:       "请求最多的接口",
				Description: "按 request_path 统计请求量 Top 20",
				LogType:     "nginx",
				Query:       `log_type:nginx | stats by (request_path) count() as requests | sort by (requests desc) | limit 20`,
				ResultType:  "table",
				Columns:     buildColumnsFromFields([]string{"request_path", "requests"}, nil),
			},
			{
				Key:         "nginx_top_remote_addr",
				Title:       "访问最频繁 IP",
				Description: "按 remote_addr 统计访问量 Top 20",
				LogType:     "nginx",
				Query:       `log_type:nginx | stats by (remote_addr) count() as requests | sort by (requests desc) | limit 20`,
				ResultType:  "table",
				Columns:     buildColumnsFromFields([]string{"remote_addr", "requests"}, nil),
			},
			{
				Key:         "nginx_error_request_path",
				Title:       "4xx/5xx 最多接口",
				Description: "按接口和状态码统计 HTTP 错误 Top 20",
				LogType:     "nginx",
				Query:       `log_type:nginx status:>=400 | stats by (request_path, status) count() as errors | sort by (errors desc) | limit 20`,
				ResultType:  "table",
				Columns:     buildColumnsFromFields([]string{"request_path", "status", "errors"}, nil),
			},
		}
	case "db_slow_log":
		return []logtailmodel.LogQueryAnalysisPreset{
			{
				Key:         "db_slow_log_query_time_ratio",
				Title:       "SQL 耗时占比",
				Description: "按 query_time 区间统计慢 SQL 数量和占比，可重点关注 5s/10s/20s 以上",
				LogType:     "db_slow_log",
				Query:       `log_type:db_slow_log | stats count() as total, count() if (query_time:>5) as gt_5s, count() if (query_time:>10) as gt_10s, count() if (query_time:>20) as gt_20s`,
				ResultType:  "ratio_summary",
				Columns:     buildColumnsFromFields([]string{"total", "gt_5s", "gt_10s", "gt_20s", "ratio"}, nil),
			},
			{
				Key:         "db_slow_log_top_query_time",
				Title:       "最慢 SQL",
				Description: "按单次 query_time 倒序查看慢 SQL",
				LogType:     "db_slow_log",
				Query:       `log_type:db_slow_log query_time:>0 | sort by (query_time desc) | limit 50`,
				ResultType:  "logs",
				Columns:     buildColumnsFromFields([]string{"_time", "query_time", "rows_examined", "db_user", "database", "sql_operation", "sql_text"}, nil),
			},
			{
				Key:         "db_slow_log_user_database",
				Title:       "慢 SQL 用户/库分布",
				Description: "按用户和数据库统计慢 SQL 数量、最大耗时、平均耗时",
				LogType:     "db_slow_log",
				Query:       `log_type:db_slow_log | stats by (db_user, database) count() as slow_count, max(query_time) as max_query_time, avg(query_time) as avg_query_time | sort by (slow_count desc) | limit 20`,
				ResultType:  "table",
				Columns:     buildColumnsFromFields([]string{"db_user", "database", "slow_count", "max_query_time", "avg_query_time"}, nil),
			},
			{
				Key:         "db_slow_log_operation",
				Title:       "SQL 操作类型分布",
				Description: "按 SELECT/UPDATE/DELETE 等操作类型统计",
				LogType:     "db_slow_log",
				Query:       `log_type:db_slow_log | stats by (sql_operation) count() as slow_count, max(query_time) as max_query_time | sort by (slow_count desc) | limit 20`,
				ResultType:  "table",
				Columns:     buildColumnsFromFields([]string{"sql_operation", "slow_count", "max_query_time"}, nil),
			},
		}
	default:
		return nil
	}
}

func buildLogQuickFilters(logType string) []logtailmodel.LogQueryQuickFilter {
	switch strings.ToLower(strings.TrimSpace(logType)) {
	case "db_slow_log":
		return []logtailmodel.LogQueryQuickFilter{
			{Key: "db_slow_log_gt_5s", Title: "SQL耗时 > 5s", LogType: "db_slow_log", Query: "log_type:db_slow_log query_time:>5"},
			{Key: "db_slow_log_gt_10s", Title: "SQL耗时 > 10s", LogType: "db_slow_log", Query: "log_type:db_slow_log query_time:>10"},
			{Key: "db_slow_log_gt_20s", Title: "SQL耗时 > 20s", LogType: "db_slow_log", Query: "log_type:db_slow_log query_time:>20"},
			{Key: "db_slow_log_rows_examined_gt_100w", Title: "扫描行数 > 100万", LogType: "db_slow_log", Query: "log_type:db_slow_log rows_examined:>1000000"},
		}
	case "nginx":
		return []logtailmodel.LogQueryQuickFilter{
			{Key: "nginx_4xx", Title: "4xx 请求", LogType: "nginx", Query: "log_type:nginx status:>=400 status:<500"},
			{Key: "nginx_5xx", Title: "5xx 请求", LogType: "nginx", Query: "log_type:nginx status:>=500 status:<600"},
			{Key: "nginx_404", Title: "404 请求", LogType: "nginx", Query: "log_type:nginx status:404"},
			{Key: "nginx_405", Title: "405 请求", LogType: "nginx", Query: "log_type:nginx status:405"},
			{Key: "nginx_503", Title: "503 请求", LogType: "nginx", Query: "log_type:nginx status:503"},
		}
	default:
		return nil
	}
}

func logDisplayFieldDefaults(logType string) []string {
	switch strings.ToLower(strings.TrimSpace(logType)) {
	case "nginx":
		return []string{"_time", "_msg", "message", "request_method", "request_path", "status", "remote_addr", "host_name"}
	case "db_slow_log":
		return []string{"_time", "_msg", "message", "query_time", "rows_examined", "db_user", "database", "sql_operation"}
	case "json", "regex":
		return []string{"_time", "_msg", "message", "level", "severity", "app", "service", "host_name"}
	default:
		return []string{"_time", "_msg", "message", "host_name", "file_path"}
	}
}

func logIndexFieldDefaults(query string) []string {
	return logIndexFieldDefaultsForType(logTypeFromQuery(query))
}

func logIndexFieldDefaultsForType(logType string) []string {
	fields := make([]string, 0, len(commonLogIndexFields)+16)
	seen := map[string]struct{}{}
	add := func(field string) {
		field = strings.TrimSpace(field)
		if field == "" {
			return
		}
		if _, ok := seen[field]; ok {
			return
		}
		seen[field] = struct{}{}
		fields = append(fields, field)
	}
	for _, field := range commonLogIndexFields {
		add(field)
	}
	if logType != "" {
		for _, field := range logTypeIndexFields[logType] {
			add(field)
		}
		return fields
	}
	for _, logTypeFields := range logTypeIndexFields {
		for _, field := range logTypeFields {
			add(field)
		}
	}
	return fields
}

func detectLogTypeFromFieldItems(query string, items []logtailmodel.LogQueryFieldItem) string {
	if logType := logTypeFromQuery(query); logType != "" {
		return logType
	}
	hitsByField := make(map[string]int64, len(items))
	for _, item := range items {
		hitsByField[strings.TrimSpace(item.Value)] = item.Hits
	}
	if hitsByField["query_time"] > 0 || hitsByField["rows_examined"] > 0 || hitsByField["sql_operation"] > 0 {
		return "db_slow_log"
	}
	if hitsByField["request_path"] > 0 || hitsByField["remote_addr"] > 0 || hitsByField["status"] > 0 {
		return "nginx"
	}
	return ""
}

func buildNormalLogFieldItems(raw, indexed []logtailmodel.LogQueryFieldItem, filter string) []logtailmodel.LogQueryFieldItem {
	filter = strings.ToLower(strings.TrimSpace(filter))
	indexedNames := make(map[string]struct{}, len(indexed))
	for _, item := range indexed {
		name := strings.TrimSpace(item.Value)
		if name != "" {
			indexedNames[name] = struct{}{}
		}
	}

	values := make([]logtailmodel.LogQueryFieldItem, 0, len(raw))
	seen := map[string]struct{}{}
	for _, item := range raw {
		name := strings.TrimSpace(item.Value)
		if name == "" || item.Hits <= 0 || !fieldMatchesFilter(name, filter) || isHiddenLogField(name) {
			continue
		}
		if _, ok := indexedNames[name]; ok {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		values = append(values, item)
	}
	return values
}

func buildOtherLogFieldItems(raw, indexed []logtailmodel.LogQueryFieldItem, filter string) []logtailmodel.LogQueryFieldItem {
	return buildNormalLogFieldItems(raw, indexed, filter)
}

func isHiddenLogField(field string) bool {
	field = strings.ToLower(strings.TrimSpace(field))
	if field == "request_body_json" || strings.HasPrefix(field, "request_body_json.") ||
		field == "post_body_json" || strings.HasPrefix(field, "post_body_json.") {
		return true
	}
	switch field {
	case "_time", "_msg", "_raw", "message",
		"offset", "partition", "topic",
		"project_code", "store_code", "agent_id", "host_id",
		"_stream", "_stream_id", "agent", "file",
		"client", "method", "protocol",
		"timestamp", "source_type":
		return true
	default:
		return false
	}
}

func mergeLogFieldItems(raw []logtailmodel.LogQueryFieldItem, defaults []string, filter string, onlyHits bool, defaultsOnly bool) []logtailmodel.LogQueryFieldItem {
	filter = strings.ToLower(strings.TrimSpace(filter))
	rawByName := make(map[string]logtailmodel.LogQueryFieldItem, len(raw))
	for _, item := range raw {
		name := strings.TrimSpace(item.Value)
		if name == "" || !fieldMatchesFilter(name, filter) || (onlyHits && item.Hits <= 0) {
			continue
		}
		rawByName[name] = item
	}

	values := make([]logtailmodel.LogQueryFieldItem, 0, len(defaults)+len(raw))
	seen := map[string]struct{}{}
	add := func(name string, hits int64) {
		name = strings.TrimSpace(name)
		if name == "" || !fieldMatchesFilter(name, filter) {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		values = append(values, logtailmodel.LogQueryFieldItem{Value: name, Hits: hits})
	}

	for _, field := range defaults {
		if item, ok := rawByName[field]; ok {
			add(item.Value, item.Hits)
			continue
		}
		if !onlyHits {
			add(field, 0)
		}
	}
	if !defaultsOnly {
		for _, item := range raw {
			if onlyHits && item.Hits <= 0 {
				continue
			}
			add(item.Value, item.Hits)
		}
	}
	return values
}

func normalizeLogFieldValuesLimit(limit int) int {
	if limit <= 0 {
		return defaultLogFieldValuesLimit
	}
	if limit > maxLogFieldValuesLimit {
		return maxLogFieldValuesLimit
	}
	return limit
}

func buildLogFieldValuesStatsQuery(query, field string, limit int) string {
	return strings.TrimSpace(query) + " | stats by (" + field + ") count() as hits | sort by (hits desc) | limit " + strconv.Itoa(limit)
}

func buildLogFieldValueItems(rows []map[string]interface{}, field, filter string, limit int) []logtailmodel.LogQueryFieldItem {
	filter = strings.ToLower(strings.TrimSpace(filter))
	values := make([]logtailmodel.LogQueryFieldItem, 0, limit)
	seen := map[string]struct{}{}
	for _, row := range rows {
		value := strings.TrimSpace(fmt.Sprint(row[field]))
		if value == "" || !fieldMatchesFilter(value, filter) {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		hits := logFieldItemHits(row["hits"])
		if hits <= 0 {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, logtailmodel.LogQueryFieldItem{Value: value, Hits: hits})
		if len(values) >= limit {
			break
		}
	}
	return values
}

func appendOtherLogFieldItem(items []logtailmodel.LogQueryFieldItem, total int64) []logtailmodel.LogQueryFieldItem {
	if total <= 0 {
		return items
	}
	var topHits int64
	for _, item := range items {
		topHits += item.Hits
	}
	otherHits := total - topHits
	if otherHits <= 0 {
		return items
	}
	return append(items, logtailmodel.LogQueryFieldItem{
		Value:   "其他",
		Hits:    otherHits,
		IsOther: true,
	})
}

func enrichLogFieldItems(items []logtailmodel.LogQueryFieldItem, total int64) []logtailmodel.LogQueryFieldItem {
	if total <= 0 || len(items) == 0 {
		return items
	}
	enriched := make([]logtailmodel.LogQueryFieldItem, 0, len(items))
	for _, item := range items {
		item.Total = total
		item.Ratio = roundLogFieldRatio(float64(item.Hits) / float64(total))
		item.Percent = roundLogFieldRatio(item.Ratio * 100)
		enriched = append(enriched, item)
	}
	return enriched
}

func maxLogFieldItemHits(items []logtailmodel.LogQueryFieldItem) int64 {
	var total int64
	for _, item := range items {
		if item.Hits > total {
			total = item.Hits
		}
	}
	return total
}

func logFieldItemHits(value interface{}) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case json.Number:
		hits, _ := typed.Int64()
		return hits
	case string:
		hits, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return hits
	default:
		return 0
	}
}

func roundLogFieldRatio(value float64) float64 {
	return float64(int64(value*10000+0.5)) / 10000
}

func fieldMatchesFilter(field, filter string) bool {
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(field), filter)
}

func rowsHaveAnyField(rows []map[string]interface{}, fields ...string) bool {
	if len(rows) == 0 {
		return false
	}
	for _, row := range rows {
		for _, field := range fields {
			if _, ok := row[field]; ok {
				return true
			}
		}
	}
	return false
}

func containsField(fields []string, want string) bool {
	for _, field := range fields {
		if field == want {
			return true
		}
	}
	return false
}

func isLogMetaField(field string) bool {
	switch field {
	case "project_code", "store_code", "collect_task_name", "collect_task_code", "env", "app",
		"log_source", "k8s_log_source", "log_type", "host_id", "agent_id", "file_path", "log_file_name",
		"cluster", "k8s_cluster_name", "k8s_workload_type", "k8s_workload_name",
		"k8s_container_name", "k8s_namespace", "k8s_pod_name", "k8s_node_name", "hostname":
		return true
	default:
		return false
	}
}

func logQueryFieldTitle(field string) string {
	titles := map[string]string{
		"_time":              "时间",
		"_msg":               "日志内容",
		"_raw":               "原始日志",
		"message":            "日志内容",
		"level":              "级别",
		"severity":           "级别",
		"severity_text":      "级别",
		"app":                "应用",
		"service":            "服务",
		"collect_task_name":  "采集任务",
		"collect_task_code":  "采集任务编码",
		"host_name":          "主机",
		"file_path":          "路径",
		"log_file_name":      "日志文件",
		"log_source":         "来源",
		"k8s_log_source":     "K8S采集来源",
		"k8s_cluster_name":   "K8S集群",
		"k8s_namespace":      "命名空间",
		"k8s_workload_type":  "工作负载类型",
		"k8s_workload_name":  "工作负载",
		"k8s_pod_name":       "Pod副本",
		"k8s_container_name": "容器",
		"k8s_node_name":      "节点",
		"log_type":           "日志类型",
		"remote_addr":        "客户端 IP",
		"remote_user":        "访问用户",
		"request_method":     "方法",
		"request_uri":        "请求 URI",
		"request_path":       "接口路径",
		"request_query":      "请求参数",
		"http_version":       "HTTP 版本",
		"status":             "状态码",
		"body_bytes_sent":    "响应字节",
		"http_referer":       "Referer",
		"http_user_agent":    "User-Agent",
		"slow_time":          "慢日志时间",
		"db_user":            "数据库用户",
		"db_host":            "来源主机",
		"thread_id":          "线程 ID",
		"database":           "数据库",
		"last_errno":         "错误码",
		"killed":             "Killed",
		"query_time":         "SQL 耗时",
		"lock_time":          "锁等待",
		"rows_sent":          "返回行数",
		"rows_examined":      "扫描行数",
		"rows_affected":      "影响行数",
		"bytes_sent":         "发送字节",
		"unix_ts":            "Unix 时间",
		"sql_text":           "SQL 语句",
		"sql_operation":      "操作类型",
		"requests":           "请求数",
		"errors":             "错误数",
		"total":              "总数",
		"gt_5s":              "> 5s",
		"gt_10s":             "> 10s",
		"gt_20s":             "> 20s",
		"ratio":              "占比",
		"slow_count":         "慢 SQL 数",
		"max_query_time":     "最大耗时",
		"avg_query_time":     "平均耗时",
	}
	if title, ok := titles[field]; ok {
		return title
	}
	return field
}

func logQueryFieldType(field string) string {
	switch field {
	case "_time", "slow_time":
		return "time"
	case "query_time", "lock_time", "avg_query_time", "max_query_time":
		return "duration"
	case "status", "body_bytes_sent", "thread_id", "last_errno", "killed",
		"rows_sent", "rows_examined", "rows_affected", "bytes_sent", "unix_ts",
		"requests", "errors", "slow_count", "total", "gt_5s", "gt_10s", "gt_20s":
		return "number"
	case "ratio":
		return "percent"
	default:
		return "string"
	}
}

func logQueryFieldWidth(field string) int {
	switch field {
	case "_time", "slow_time":
		return 190
	case "message", "_msg", "_raw", "sql_text", "http_user_agent":
		return 420
	case "request_uri", "request_path", "http_referer", "file_path":
		return 280
	case "query_time", "lock_time", "rows_examined", "body_bytes_sent", "status":
		return 110
	default:
		return 140
	}
}

func logQueryFieldSortable(field string) bool {
	switch logQueryFieldType(field) {
	case "time", "duration", "number":
		return true
	default:
		return field == "request_path" || field == "remote_addr" || field == "db_user" || field == "database"
	}
}

func logQueryFieldEllipsis(field string) bool {
	switch field {
	case "message", "_msg", "_raw", "sql_text", "request_uri", "request_path",
		"http_referer", "http_user_agent", "file_path":
		return true
	default:
		return false
	}
}

func buildVictoriaLogsExtraFilters(store *logtailmodel.LogStore, filters map[string]interface{}) (map[string]interface{}, error) {
	extra := map[string]interface{}{}
	forcedStoreCode := store != nil && strings.TrimSpace(store.Code) != ""
	if forcedStoreCode {
		extra["store_code"] = strings.TrimSpace(store.Code)
	}
	for key, value := range filters {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if forcedStoreCode && key == "store_code" {
			continue
		}
		if !isValidLogFieldName(key) {
			return nil, fmt.Errorf("过滤字段不合法: %s", key)
		}
		extra[key] = normalizeVictoriaLogsFilterValue(value)
	}
	return extra, nil
}

func normalizeVictoriaLogsFilterValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case []interface{}:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			values = append(values, fmt.Sprint(item))
		}
		return values
	case []string:
		return typed
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func resolveVictoriaLogsTenant(store *logtailmodel.LogStore, base logtailmodel.LogQueryBaseRequest) victoriaLogsTenant {
	tenant := victoriaLogsTenant{}
	if store != nil {
		tenant = parseVictoriaLogsTenant(store.VlogsTenant)
	}
	if accountID := strings.TrimSpace(base.VLogsAccountID); accountID != "" {
		tenant.AccountID = accountID
	}
	if projectID := strings.TrimSpace(base.VLogsProjectID); projectID != "" {
		tenant.ProjectID = projectID
	}
	return tenant
}

func parseVictoriaLogsTenant(raw string) victoriaLogsTenant {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return victoriaLogsTenant{}
	}
	var payload struct {
		AccountID string `json:"accountId"`
		ProjectID string `json:"projectId"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err == nil {
		return victoriaLogsTenant{
			AccountID: strings.TrimSpace(payload.AccountID),
			ProjectID: strings.TrimSpace(payload.ProjectID),
		}
	}

	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ':' || r == '/' || r == ',' || r == ';'
	})
	if len(fields) >= 2 {
		return victoriaLogsTenant{
			AccountID: strings.TrimSpace(strings.TrimPrefix(fields[0], "account=")),
			ProjectID: strings.TrimSpace(strings.TrimPrefix(fields[1], "project=")),
		}
	}
	return victoriaLogsTenant{}
}

func decodeVictoriaLogsLines(body []byte) ([]map[string]interface{}, error) {
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
			return nil, fmt.Errorf("解析 VictoriaLogs 日志行失败: %w", err)
		}
		logs = append(logs, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}

func toQueryStoreRef(store *logtailmodel.LogStore) logtailmodel.LogQueryStoreRef {
	if store == nil {
		return logtailmodel.LogQueryStoreRef{}
	}
	return logtailmodel.LogQueryStoreRef{
		ID:   store.ID,
		Code: store.Code,
		Name: store.Name,
	}
}

func isValidLogFieldName(field string) bool {
	return logFieldNamePattern.MatchString(field)
}

func truncateForError(body []byte) string {
	const limit = 512
	if len(body) <= limit {
		return string(body)
	}
	return string(body[:limit]) + "..."
}
