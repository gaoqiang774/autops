package service

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	logtailmodel "dodevops-api/api/logtail/model"

	"github.com/gin-gonic/gin"
)

const maxLogTraceListDiscovery = 1000

var logTraceListFields = []string{"trace_id", "x_zai_trace_id"}

type logTraceListStat struct {
	TraceID       string
	Store         logtailmodel.LogStore
	FirstSeen     time.Time
	LastSeen      time.Time
	EventCount    int
	ErrorCount    int
	WarningCount  int
	DurationMs    float64
	Service       string
	Operation     string
	OperationRank int
}

type logTraceListBuilder struct {
	Item          logtailmodel.LogTraceListItem
	FirstSeen     time.Time
	LastSeen      time.Time
	Services      map[string]struct{}
	StoreCodes    map[string]struct{}
	OperationRank int
	OperationTime float64
}

// QueryTraceList returns the recent trace summaries used by the trace workbench master list.
func (s *LogtailService) QueryTraceList(c *gin.Context, req logtailmodel.LogTraceListQueryRequest) (*logtailmodel.LogTraceListResponse, error) {
	startedAt := time.Now()
	if req.ProjectID == 0 {
		return nil, errors.New("项目ID不能为空")
	}
	traceID := strings.TrimSpace(req.TraceID)
	if traceID != "" && !logTraceIDPattern.MatchString(traceID) {
		return nil, errors.New("traceId格式不合法")
	}
	status, err := normalizeLogTraceListStatus(req.Status)
	if err != nil {
		return nil, err
	}
	page, pageSize := normalizeLogTraceListPage(req.Page, req.PageSize)
	start, end := normalizeVictoriaLogsTimeRange(req.Start, req.End)

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

	pageEnd := page * pageSize
	if (page-1)*pageSize >= maxLogTraceListDiscovery {
		return nil, fmt.Errorf("trace列表最多支持浏览最近%d条，请缩小时间范围", maxLogTraceListDiscovery)
	}
	discoveryLimit := pageEnd + 1
	if discoveryLimit < pageSize*4 {
		discoveryLimit = pageSize * 4
	}
	if status != "" || strings.TrimSpace(req.Service) != "" {
		discoveryLimit = maxLogTraceListDiscovery
	}
	if discoveryLimit > maxLogTraceListDiscovery {
		discoveryLimit = maxLogTraceListDiscovery
	}

	stats := make([]logTraceListStat, 0)
	warnings := make([]string, 0)
	mayHaveMore := false
	successfulQueries := 0
	for _, store := range stores {
		for _, field := range logTraceListFields {
			query := buildLogTraceListStatsQuery(field, traceID, discoveryLimit)
			response, queryErr := s.QueryLogs(copyGinContext(c), logtailmodel.LogQueryLogsRequest{
				LogQueryBaseRequest: logtailmodel.LogQueryBaseRequest{
					StoreID: store.ID,
					Query:   query,
					Start:   start,
					End:     end,
				},
				Limit: discoveryLimit,
			})
			if queryErr != nil {
				warnings = append(warnings, fmt.Sprintf("日志库 %s 的 %s 聚合失败: %v", store.Name, field, queryErr))
				continue
			}
			successfulQueries++
			if len(response.Logs) >= discoveryLimit {
				mayHaveMore = true
			}
			for _, row := range response.Logs {
				if stat, ok := parseLogTraceListStat(store, field, row); ok {
					stats = append(stats, stat)
				}
			}
		}
	}
	if successfulQueries == 0 {
		return nil, errors.New("所有日志库聚合均失败: " + strings.Join(warnings, "; "))
	}

	items := buildLogTraceListItems(stats)
	items = filterLogTraceListItems(items, status, req.Service)
	offset := (page - 1) * pageSize
	hasMore := mayHaveMore || len(items) > pageEnd
	pageItems := make([]logtailmodel.LogTraceListItem, 0)
	if offset < len(items) {
		limit := pageEnd
		if limit > len(items) {
			limit = len(items)
		}
		pageItems = append(pageItems, items[offset:limit]...)
	}
	if traceID != "" {
		hasMore = false
	}

	return &logtailmodel.LogTraceListResponse{
		Project:         logtailmodel.LogTraceProjectRef{ID: project.ID, Code: project.Code, Name: project.Name},
		Start:           start,
		End:             end,
		Page:            page,
		PageSize:        pageSize,
		Count:           len(pageItems),
		HasMore:         hasMore,
		Items:           pageItems,
		Partial:         len(warnings) > 0,
		Warnings:        warnings,
		QueryDurationMs: time.Since(startedAt).Milliseconds(),
	}, nil
}

func normalizeLogTraceListPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func normalizeLogTraceListStatus(value string) (string, error) {
	switch value = strings.ToLower(strings.TrimSpace(value)); value {
	case "", "all":
		return "", nil
	case "success", "warning", "error":
		return value, nil
	default:
		return "", fmt.Errorf("status不合法: %s", value)
	}
}

func buildLogTraceListStatsQuery(field, traceID string, limit int) string {
	filter := field + ":*"
	if traceID != "" {
		filter = fmt.Sprintf(`%s:=%s`, field, strconv.Quote(traceID))
	}
	query := fmt.Sprintf(
		`%s | stats by (%s) count() as event_count, min(_time) as first_seen, max(_time) as last_seen, max(duration_ms) as max_duration_ms, max(request_time) as max_request_seconds, max(responsetime) as max_response_seconds, max(upstream_response_time) as max_upstream_seconds, max(upstreamtime) as max_upstream_time_seconds, any(request_method) as request_method, any(request_path) as request_path, any(request) as request, any(event) as event, any(service) as service, any(collect_task_name) as collect_task_name, count() if (status:>=400 status:<500) as http_4xx, count() if (status:>=500 status:<600) as http_5xx, count() if (status:error OR status:failed OR status:failure OR status:fatal OR status:ERROR OR status:FAILED OR status:FAILURE OR status:FATAL) as status_errors, count() if (status:warn OR status:warning OR status:WARN OR status:WARNING) as status_warnings, count() if (level:error OR level:fatal OR level:ERROR OR level:FATAL) as error_logs, count() if (level:warn OR level:warning OR level:WARN OR level:WARNING) as warning_logs`,
		filter,
		field,
	)
	return fmt.Sprintf(`%s | sort by (last_seen desc) | limit %d`, query, limit)
}

func parseLogTraceListStat(store logtailmodel.LogStore, field string, row map[string]interface{}) (logTraceListStat, bool) {
	traceID := strings.TrimSpace(slaString(row, field))
	if !logTraceIDPattern.MatchString(traceID) {
		return logTraceListStat{}, false
	}
	firstSeen, firstOK := parseLogTraceTime(slaString(row, "first_seen"))
	lastSeen, lastOK := parseLogTraceTime(slaString(row, "last_seen"))
	if !firstOK || !lastOK {
		return logTraceListStat{}, false
	}

	http4xx := int(slaInt64(row, "http_4xx"))
	http5xx := int(slaInt64(row, "http_5xx"))
	statusErrors := int(slaInt64(row, "status_errors"))
	statusWarnings := int(slaInt64(row, "status_warnings"))
	errorLogs := int(slaInt64(row, "error_logs"))
	warningLogs := int(slaInt64(row, "warning_logs"))
	errorCount := http5xx
	if statusErrors > errorCount {
		errorCount = statusErrors
	}
	if errorLogs > errorCount {
		errorCount = errorLogs
	}
	warningCount := http4xx
	if statusWarnings > warningCount {
		warningCount = statusWarnings
	}
	if warningLogs > warningCount {
		warningCount = warningLogs
	}

	durationMs := slaFloat64(row, "max_duration_ms")
	for _, field := range []string{"max_request_seconds", "max_response_seconds", "max_upstream_seconds", "max_upstream_time_seconds"} {
		if seconds := slaFloat64(row, field); seconds*1000 > durationMs {
			durationMs = seconds * 1000
		}
	}
	method := strings.ToUpper(slaString(row, "request_method"))
	path := slaString(row, "request_path", "request")
	operation := strings.TrimSpace(strings.Join([]string{method, path}, " "))
	operationRank := 2
	if operation == "" {
		operation = slaString(row, "event")
		operationRank = 1
	}
	service := firstNonEmpty(slaString(row, "service", "collect_task_name"), store.Name, store.Code)
	if operation == "" {
		operation = service
		operationRank = 0
	}

	return logTraceListStat{
		TraceID:       traceID,
		Store:         store,
		FirstSeen:     firstSeen,
		LastSeen:      lastSeen,
		EventCount:    int(slaInt64(row, "event_count")),
		ErrorCount:    errorCount,
		WarningCount:  warningCount,
		DurationMs:    roundSLA(durationMs),
		Service:       service,
		Operation:     operation,
		OperationRank: operationRank,
	}, true
}

func buildLogTraceListItems(stats []logTraceListStat) []logtailmodel.LogTraceListItem {
	// A log may contain both trace_id aliases. Collapse duplicates within one store first.
	byTraceStore := make(map[string]logTraceListStat)
	for _, stat := range stats {
		key := stat.TraceID + "\x00" + stat.Store.Code
		if existing, ok := byTraceStore[key]; ok {
			byTraceStore[key] = mergeLogTraceListStoreStat(existing, stat)
		} else {
			byTraceStore[key] = stat
		}
	}

	builders := make(map[string]*logTraceListBuilder)
	for _, stat := range byTraceStore {
		builder := builders[stat.TraceID]
		if builder == nil {
			builder = &logTraceListBuilder{
				Item:       logtailmodel.LogTraceListItem{TraceID: stat.TraceID, Status: "success"},
				Services:   make(map[string]struct{}),
				StoreCodes: make(map[string]struct{}),
			}
			builders[stat.TraceID] = builder
		}
		if builder.FirstSeen.IsZero() || stat.FirstSeen.Before(builder.FirstSeen) {
			builder.FirstSeen = stat.FirstSeen
		}
		if builder.LastSeen.IsZero() || stat.LastSeen.After(builder.LastSeen) {
			builder.LastSeen = stat.LastSeen
		}
		builder.Item.EventCount += stat.EventCount
		builder.Item.ErrorCount += stat.ErrorCount
		builder.Item.WarningCount += stat.WarningCount
		if stat.DurationMs > builder.Item.MaxDurationMs {
			builder.Item.MaxDurationMs = stat.DurationMs
		}
		if stat.Service != "" {
			builder.Services[stat.Service] = struct{}{}
		}
		if stat.Store.Code != "" {
			builder.StoreCodes[stat.Store.Code] = struct{}{}
		}
		if stat.OperationRank > builder.OperationRank || (stat.OperationRank == builder.OperationRank && stat.DurationMs > builder.OperationTime) {
			builder.Item.PrimaryOperation = stat.Operation
			builder.OperationRank = stat.OperationRank
			builder.OperationTime = stat.DurationMs
		}
	}

	items := make([]logtailmodel.LogTraceListItem, 0, len(builders))
	for _, builder := range builders {
		builder.Item.Services = sortedLogTraceSet(builder.Services)
		builder.Item.StoreCodes = sortedLogTraceSet(builder.StoreCodes)
		builder.Item.ServiceCount = len(builder.Item.Services)
		builder.Item.FirstSeen = formatLogTimeForDisplay(builder.FirstSeen)
		builder.Item.LastSeen = formatLogTimeForDisplay(builder.LastSeen)
		builder.Item.ObservedDurationMs = roundSLA(float64(builder.LastSeen.Sub(builder.FirstSeen)) / float64(time.Millisecond))
		switch {
		case builder.Item.ErrorCount > 0:
			builder.Item.Status = "error"
		case builder.Item.WarningCount > 0:
			builder.Item.Status = "warning"
		}
		items = append(items, builder.Item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].LastSeen != items[j].LastSeen {
			return items[i].LastSeen > items[j].LastSeen
		}
		return items[i].TraceID < items[j].TraceID
	})
	return items
}

func mergeLogTraceListStoreStat(left, right logTraceListStat) logTraceListStat {
	preferRightOperation := right.OperationRank > left.OperationRank || (right.OperationRank == left.OperationRank && right.DurationMs > left.DurationMs)
	if right.FirstSeen.Before(left.FirstSeen) {
		left.FirstSeen = right.FirstSeen
	}
	if right.LastSeen.After(left.LastSeen) {
		left.LastSeen = right.LastSeen
	}
	if right.EventCount > left.EventCount {
		left.EventCount = right.EventCount
	}
	if right.ErrorCount > left.ErrorCount {
		left.ErrorCount = right.ErrorCount
	}
	if right.WarningCount > left.WarningCount {
		left.WarningCount = right.WarningCount
	}
	if right.DurationMs > left.DurationMs {
		left.DurationMs = right.DurationMs
	}
	if preferRightOperation {
		left.Operation = right.Operation
		left.OperationRank = right.OperationRank
	}
	if left.Service == "" {
		left.Service = right.Service
	}
	return left
}

func filterLogTraceListItems(items []logtailmodel.LogTraceListItem, status, service string) []logtailmodel.LogTraceListItem {
	service = strings.ToLower(strings.TrimSpace(service))
	result := make([]logtailmodel.LogTraceListItem, 0, len(items))
	for _, item := range items {
		if status != "" && item.Status != status {
			continue
		}
		if service != "" {
			matched := false
			for _, value := range item.Services {
				if strings.Contains(strings.ToLower(value), service) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		result = append(result, item)
	}
	return result
}
