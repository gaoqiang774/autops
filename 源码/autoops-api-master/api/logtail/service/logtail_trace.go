package service

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	logtailmodel "dodevops-api/api/logtail/model"

	"github.com/gin-gonic/gin"
)

const (
	defaultLogTraceLimit = 500
	maxLogTraceLimit     = 1000
	maxLogTraceMessage   = 1200

	logTraceEventTypeHTTPRequest = "http_request"
	logTraceEventTypeModelCall   = "model_call"
	logTraceEventTypeModelUsage  = "model_usage"
	logTraceEventTypeError       = "error"
	logTraceEventTypeInternalLog = "internal_log"
)

var logTraceIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{7,255}$`)
var logTraceSecretHeaderPattern = regexp.MustCompile(`(?i)((?:authorization|x[-_]zai[-_](?:api[-_]key|session[-_]token|internal[-_]key)|lyj[-_]openclaw[-_]api[-_]key|api[-_]key|session[-_]token|access[-_]token)\s*:\s*['"]?(?:bearer\s+)?)[^'"\s,}]+`)
var logTraceSecretMapPattern = regexp.MustCompile(`(?i)(['"](?:authorization|x[-_]zai[-_](?:api[-_]key|session[-_]token|internal[-_]key)|lyj[-_]openclaw[-_]api[-_]key|api[-_]key|session[-_]token|access[-_]token)['"]\s*:\s*['"]?(?:bearer\s+)?)[^'"\s,}]+`)
var logTraceCurlMethodPattern = regexp.MustCompile(`(?i)\bcurl\s+-X\s+([A-Z]+)`)
var logTraceURLPattern = regexp.MustCompile(`(?i)https?://[^/'"\s]+([^'"\s]*)`)

var logTraceQueryFields = []string{
	"_time", "@timestamp", "timestamp", "message", "_msg",
	"project_code", "store_code", "collect_task_name", "collect_task_code",
	"app", "service", "event", "level", "severity", "status",
	"log_source", "log_type", "host_name", "file_path",
	"request_method", "request", "request_uri", "request_path", "domain",
	"request_time", "responsetime", "upstream_response_time", "upstreamtime", "duration_ms",
	"request_id", "trace_id", "x_zai_trace_id", "span_id", "parent_span_id",
	"error", "error_code", "error_msg", "errorMsg",
}

type logTraceRow struct {
	Store logtailmodel.LogStore
	Row   map[string]interface{}
}

type logTraceServiceBuilder struct {
	Summary    logtailmodel.LogTraceServiceSummary
	StoreCodes map[string]struct{}
	Operations map[string]struct{}
	FirstTime  time.Time
	LastTime   time.Time
}

// QueryTraceAnalysis correlates logs across all authorized stores in one project.
// The response is an observed log timeline, not a parent/child span topology.
func (s *LogtailService) QueryTraceAnalysis(c *gin.Context, req logtailmodel.LogTraceQueryRequest) (*logtailmodel.LogTraceAnalysisResponse, error) {
	startedAt := time.Now()
	traceID := strings.TrimSpace(req.TraceID)
	if req.ProjectID == 0 {
		return nil, errors.New("项目ID不能为空")
	}
	if !logTraceIDPattern.MatchString(traceID) {
		return nil, errors.New("traceId格式不合法")
	}

	stores, err := s.listAuthorizedTraceStores(c, req.ProjectID)
	if err != nil {
		return nil, err
	}
	if len(stores) == 0 {
		return nil, errors.New("项目下没有可访问的日志库")
	}

	project, err := s.projectDAO.GetByID(req.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("查询日志项目失败: %w", err)
	}
	start, end := normalizeVictoriaLogsTimeRange(req.Start, req.End)
	limit := normalizeLogTraceLimit(req.Limit)
	query := buildLogTraceQuery(traceID)
	rows := make([]logTraceRow, 0)
	warnings := make([]string, 0)
	truncated := false
	successfulQueries := 0

	for _, store := range stores {
		response, queryErr := s.QueryLogs(copyGinContext(c), logtailmodel.LogQueryLogsRequest{
			LogQueryBaseRequest: logtailmodel.LogQueryBaseRequest{
				StoreID: store.ID,
				Query:   query,
				Start:   start,
				End:     end,
			},
			Limit:  limit,
			Fields: logTraceQueryFields,
		})
		if queryErr != nil {
			warnings = append(warnings, fmt.Sprintf("日志库 %s 查询失败: %v", store.Name, queryErr))
			continue
		}
		successfulQueries++
		if response.HasMore {
			truncated = true
		}
		for _, row := range response.Logs {
			rows = append(rows, logTraceRow{Store: store, Row: row})
		}
	}

	if successfulQueries == 0 {
		return nil, errors.New("所有日志库查询均失败: " + strings.Join(warnings, "; "))
	}
	result := buildLogTraceAnalysis(*project, traceID, start, end, rows, limit)
	result.Partial = len(warnings) > 0
	result.Truncated = result.Truncated || truncated
	result.Warnings = warnings
	result.QueryDurationMs = time.Since(startedAt).Milliseconds()
	return result, nil
}

func (s *LogtailService) listAuthorizedTraceStores(c *gin.Context, projectID uint) ([]logtailmodel.LogStore, error) {
	stores := make([]logtailmodel.LogStore, 0)
	for page := 1; ; page++ {
		items, total, _, err := s.ListStores(c, logtailmodel.LogStoreQuery{
			Page:      page,
			PageSize:  100,
			ProjectID: projectID,
		})
		if err != nil {
			return nil, fmt.Errorf("查询项目日志库失败: %w", err)
		}
		stores = append(stores, items...)
		if len(stores) >= int(total) || len(items) == 0 {
			break
		}
	}
	return stores, nil
}

func normalizeLogTraceLimit(limit int) int {
	if limit <= 0 {
		return defaultLogTraceLimit
	}
	if limit > maxLogTraceLimit {
		return maxLogTraceLimit
	}
	return limit
}

func buildLogTraceQuery(traceID string) string {
	quoted := strconv.Quote(traceID)
	return fmt.Sprintf(
		`(*%s* OR trace_id:=%s OR x_zai_trace_id:=%s OR span_id:=%s OR request_id:=%s OR session_id:=%s)`,
		traceID, quoted, quoted, quoted, quoted, quoted,
	)
}

func buildLogTraceAnalysis(project logtailmodel.LogProject, traceID, start, end string, rows []logTraceRow, limit int) *logtailmodel.LogTraceAnalysisResponse {
	events := make([]logtailmodel.LogTraceEvent, 0, len(rows))
	for _, item := range rows {
		events = append(events, normalizeLogTraceEvent(traceID, item.Store, item.Row))
	}
	sort.SliceStable(events, func(i, j int) bool {
		left, leftOK := parseLogTraceTime(events[i].Timestamp)
		right, rightOK := parseLogTraceTime(events[j].Timestamp)
		if leftOK && rightOK {
			return left.Before(right)
		}
		if leftOK != rightOK {
			return leftOK
		}
		return events[i].Timestamp < events[j].Timestamp
	})

	truncated := false
	if len(events) > limit {
		events = events[:limit]
		truncated = true
	}
	for index := range events {
		events[index].ID = fmt.Sprintf("event-%d", index+1)
	}

	services, errorCount := summarizeLogTraceServices(events)
	status := "not_found"
	firstSeen := ""
	lastSeen := ""
	observedDuration := float64(0)
	if len(events) > 0 {
		status = "success"
		firstSeen = events[0].Timestamp
		lastSeen = events[len(events)-1].Timestamp
		if errorCount > 0 {
			status = "error"
		} else {
			for _, event := range events {
				if event.Status == "warning" {
					status = "warning"
					break
				}
			}
		}
		firstTime, firstOK := parseLogTraceTime(firstSeen)
		lastTime, lastOK := parseLogTraceTime(lastSeen)
		if firstOK && lastOK && !lastTime.Before(firstTime) {
			observedDuration = roundSLA(float64(lastTime.Sub(firstTime)) / float64(time.Millisecond))
		}
	}

	return &logtailmodel.LogTraceAnalysisResponse{
		Project: logtailmodel.LogTraceProjectRef{
			ID:   project.ID,
			Code: project.Code,
			Name: project.Name,
		},
		TraceID:            traceID,
		Mode:               "log_correlation",
		HasCallOrder:       false,
		Status:             status,
		Start:              start,
		End:                end,
		FirstSeen:          firstSeen,
		LastSeen:           lastSeen,
		ObservedDurationMs: observedDuration,
		ServiceCount:       len(services),
		EventCount:         len(events),
		ErrorCount:         errorCount,
		Services:           services,
		Events:             events,
		Truncated:          truncated,
		Warnings:           make([]string, 0),
	}
}

func normalizeLogTraceEvent(traceID string, store logtailmodel.LogStore, row map[string]interface{}) logtailmodel.LogTraceEvent {
	timestamp := slaString(row, "_time", "@timestamp", "timestamp")
	durationMs := logTraceDurationMs(row)
	observedAt, observedOK := parseLogTraceTime(timestamp)
	startedAt := timestamp
	endedAt := timestamp
	if observedOK {
		timestamp = formatLogTimeForDisplay(observedAt)
		startedAt = timestamp
		endedAt = timestamp
		if durationMs > 0 {
			startedAt = formatLogTimeForDisplay(observedAt.Add(-time.Duration(durationMs * float64(time.Millisecond))))
		}
	}

	message := redactLogTraceMessage(slaString(row, "message", "_msg"))
	requestMethod := strings.ToUpper(slaString(row, "request_method", "method"))
	requestPath := slaString(row, "request_path", "request_uri", "request")
	if requestMethod == "" {
		if matches := logTraceCurlMethodPattern.FindStringSubmatch(message); len(matches) > 1 {
			requestMethod = strings.ToUpper(matches[1])
		}
	}
	if requestPath == "" {
		if matches := logTraceURLPattern.FindStringSubmatch(message); len(matches) > 1 {
			requestPath = matches[1]
		}
	}
	app := slaString(row, "app")
	service := slaString(row, "service")
	collectTaskName := slaString(row, "collect_task_name")
	if service == "" || (service == app && collectTaskName != "") {
		service = collectTaskName
	}
	if service == "" {
		service = firstNonEmpty(store.Name, store.Code, app, "unknown")
	}
	eventName := slaString(row, "event")
	operation := strings.TrimSpace(strings.Join([]string{requestMethod, requestPath}, " "))
	if operation == "" {
		operation = eventName
	}
	if operation == "" {
		operation = truncateLogTraceText(message, 160)
	}

	httpStatus := int(slaInt64(row, "status", "http_status", "status_code"))
	status := normalizeLogTraceStatus(row, httpStatus)
	eventType, isKeyEvent, displayName := classifyLogTraceEvent(eventName, operation, message, requestMethod, requestPath, status, durationMs, httpStatus, slaString(row, "level", "severity"))
	return logtailmodel.LogTraceEvent{
		Timestamp:       timestamp,
		StartedAt:       startedAt,
		EndedAt:         endedAt,
		StoreID:         store.ID,
		StoreCode:       firstNonEmpty(slaString(row, "store_code"), store.Code),
		StoreName:       store.Name,
		Service:         service,
		App:             app,
		Event:           eventName,
		EventType:       eventType,
		IsKeyEvent:      isKeyEvent,
		DisplayName:     displayName,
		Operation:       operation,
		Status:          status,
		Level:           strings.ToUpper(slaString(row, "level", "severity")),
		HTTPStatus:      httpStatus,
		DurationMs:      durationMs,
		RequestMethod:   requestMethod,
		RequestPath:     requestPath,
		RequestID:       slaString(row, "request_id"),
		TraceID:         firstNonEmpty(slaString(row, "trace_id", "x_zai_trace_id"), traceID),
		SpanID:          slaString(row, "span_id"),
		ParentSpanID:    slaString(row, "parent_span_id"),
		LogSource:       slaString(row, "log_source"),
		HostName:        slaString(row, "host_name", "hostname", "host"),
		FilePath:        slaString(row, "file_path", "path"),
		Message:         message,
		CollectTaskCode: slaString(row, "collect_task_code"),
	}
}

func classifyLogTraceEvent(eventName, operation, message, requestMethod, requestPath, status string, durationMs float64, httpStatus int, level string) (string, bool, string) {
	level = strings.ToLower(strings.TrimSpace(level))
	eventName = strings.TrimSpace(eventName)
	operation = strings.TrimSpace(operation)

	if requestMethod != "" && requestPath != "" {
		return logTraceEventTypeHTTPRequest, true, operation
	}
	if eventName == "model.call" {
		return logTraceEventTypeModelCall, true, eventName
	}
	if eventName == "model.usage" {
		return logTraceEventTypeModelUsage, true, eventName
	}
	if status == "error" || level == "error" || level == "fatal" || httpStatus >= 500 {
		return logTraceEventTypeError, true, firstNonEmpty(operation, eventName, truncateLogTraceText(message, 160), "error")
	}
	if eventName != "" || durationMs > 0 || status == "warning" || httpStatus >= 400 {
		return logTraceEventTypeInternalLog, true, firstNonEmpty(operation, eventName, truncateLogTraceText(message, 160), "internal_log")
	}
	return logTraceEventTypeInternalLog, false, firstNonEmpty(eventName, truncateLogTraceText(message, 160), "internal_log")
}

func logTraceDurationMs(row map[string]interface{}) float64 {
	if duration := slaFloat64(row, "duration_ms"); duration > 0 {
		return roundSLA(duration)
	}
	if duration := slaFloat64(row, "request_time", "responsetime", "upstream_response_time", "upstreamtime"); duration > 0 {
		return roundSLA(duration * 1000)
	}
	return 0
}

func normalizeLogTraceStatus(row map[string]interface{}, httpStatus int) string {
	level := strings.ToLower(slaString(row, "level", "severity"))
	status := strings.ToLower(slaString(row, "status", "status_text", "result"))
	switch {
	case httpStatus >= 500:
		return "error"
	case httpStatus >= 400:
		return "warning"
	case strings.Contains(status, "fail") || strings.Contains(status, "error") || strings.Contains(status, "timeout"):
		return "error"
	case level == "fatal" || level == "error":
		return "error"
	case level == "warn" || level == "warning":
		return "warning"
	case httpStatus >= 100 || status == "success" || status == "ok" || level == "info" || level == "debug" || level == "trace":
		return "success"
	default:
		return "unknown"
	}
}

func summarizeLogTraceServices(events []logtailmodel.LogTraceEvent) ([]logtailmodel.LogTraceServiceSummary, int) {
	builders := make(map[string]*logTraceServiceBuilder)
	order := make([]string, 0)
	errorCount := 0
	for _, event := range events {
		key := firstNonEmpty(event.Service, event.App, event.StoreCode, "unknown")
		builder := builders[key]
		if builder == nil {
			builder = &logTraceServiceBuilder{
				Summary: logtailmodel.LogTraceServiceSummary{
					Service:    key,
					App:        event.App,
					Status:     event.Status,
					StoreCodes: make([]string, 0),
					Operations: make([]string, 0),
				},
				StoreCodes: make(map[string]struct{}),
				Operations: make(map[string]struct{}),
			}
			builders[key] = builder
			order = append(order, key)
		}
		builder.Summary.EventCount++
		if event.Status == "error" {
			builder.Summary.ErrorCount++
			errorCount++
		}
		builder.Summary.Status = higherLogTraceStatus(builder.Summary.Status, event.Status)
		if event.DurationMs > builder.Summary.MaxDurationMs {
			builder.Summary.MaxDurationMs = event.DurationMs
		}
		if event.StoreCode != "" {
			builder.StoreCodes[event.StoreCode] = struct{}{}
		}
		if event.Operation != "" {
			builder.Operations[event.Operation] = struct{}{}
		}
		if observedAt, ok := parseLogTraceTime(event.Timestamp); ok {
			if builder.FirstTime.IsZero() || observedAt.Before(builder.FirstTime) {
				builder.FirstTime = observedAt
			}
			if builder.LastTime.IsZero() || observedAt.After(builder.LastTime) {
				builder.LastTime = observedAt
			}
		}
	}

	result := make([]logtailmodel.LogTraceServiceSummary, 0, len(order))
	for _, key := range order {
		builder := builders[key]
		builder.Summary.StoreCodes = sortedLogTraceSet(builder.StoreCodes)
		builder.Summary.Operations = sortedLogTraceSet(builder.Operations)
		if !builder.FirstTime.IsZero() {
			builder.Summary.FirstSeen = formatLogTimeForDisplay(builder.FirstTime)
		}
		if !builder.LastTime.IsZero() {
			builder.Summary.LastSeen = formatLogTimeForDisplay(builder.LastTime)
		}
		result = append(result, builder.Summary)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].FirstSeen < result[j].FirstSeen })
	return result, errorCount
}

func higherLogTraceStatus(left, right string) string {
	rank := map[string]int{"unknown": 0, "success": 1, "warning": 2, "error": 3}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func sortedLogTraceSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func parseLogTraceTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	for _, layout := range []string{"2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05"} {
		if parsed, err := time.ParseInLocation(layout, value, shanghai); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func redactLogTraceMessage(value string) string {
	value = redactSensitiveTextFieldsForAI(value)
	value = logTraceSecretMapPattern.ReplaceAllString(value, `${1}<redacted>`)
	value = logTraceSecretHeaderPattern.ReplaceAllString(value, `${1}<redacted>`)
	return truncateLogTraceText(value, maxLogTraceMessage)
}

func truncateLogTraceText(value string, limit int) string {
	runes := []rune(value)
	if limit <= 0 || len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "...[truncated]..."
}
