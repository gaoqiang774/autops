package service

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	logtailmodel "dodevops-api/api/logtail/model"

	"github.com/gin-gonic/gin"
)

const defaultSLASlowRequestThreshold = 1.0

// QuerySLAStore returns the normalized HTTP stability signals for one log store.
func (s *LogtailService) QuerySLAStore(c *gin.Context, req logtailmodel.LogSLAStoreQuery) (*logtailmodel.LogSLAStoreSnapshot, error) {
	if req.StoreID == 0 {
		return nil, errors.New("日志库ID不能为空")
	}
	if req.SlowRequestThreshold <= 0 {
		req.SlowRequestThreshold = defaultSLASlowRequestThreshold
	}
	if strings.TrimSpace(req.Step) == "" {
		req.Step = "1m"
	}

	base := logtailmodel.LogQueryBaseRequest{
		StoreID:      req.StoreID,
		Start:        req.Start,
		End:          req.End,
		ExtraFilters: req.ExtraFilters,
	}
	filter := strings.TrimSpace(req.Filter)
	if filter == "" {
		filter = "status:>=100 status:<600"
	}
	recentFilter := strings.TrimSpace(req.RecentFilter)
	if recentFilter == "" {
		recentFilter = strings.TrimSpace(filter + " status:>=400 status:<600")
	}
	var (
		summaryResp *logtailmodel.LogQueryLogsResponse
		topAPIResp  *logtailmodel.LogQueryLogsResponse
		recentResp  *logtailmodel.LogQueryLogsResponse
		trendResp   *logtailmodel.LogQueryHitsResponse
		summaryErr  error
		topAPIErr   error
		recentErr   error
		trendErr    error
		wg          sync.WaitGroup
	)

	startedAt := time.Now()
	wg.Add(4)
	go func() {
		defer wg.Done()
		query := base
		query.Query = buildLogSLASummaryQueryWithFilter(filter, req.SlowRequestThreshold)
		summaryResp, summaryErr = s.QueryLogs(copyGinContext(c), logtailmodel.LogQueryLogsRequest{
			LogQueryBaseRequest: query,
			Limit:               1,
		})
	}()
	go func() {
		defer wg.Done()
		query := base
		query.Query = buildLogSLATopAPIQueryWithFilterAndSort(filter, req.TopAPISort)
		topAPIResp, topAPIErr = s.QueryLogs(copyGinContext(c), logtailmodel.LogQueryLogsRequest{
			LogQueryBaseRequest: query,
			Limit:               50,
		})
	}()
	go func() {
		defer wg.Done()
		query := base
		query.Query = recentFilter
		recentResp, recentErr = s.QueryLogs(copyGinContext(c), logtailmodel.LogQueryLogsRequest{
			LogQueryBaseRequest: query,
			Limit:               30,
			Fields: []string{
				"_time", "@timestamp", "request_method", "request_path", "request", "domain", "status",
				"request_time", "responsetime", "upstreamaddr", "upstream_addr", "clientip", "remote_addr",
				"request_id", "trace_id", "x_zai_trace_id",
			},
		})
	}()
	go func() {
		defer wg.Done()
		query := base
		query.Query = filter
		trendResp, trendErr = s.QueryHits(copyGinContext(c), logtailmodel.LogQueryHitsRequest{
			LogQueryBaseRequest: query,
			Step:                req.Step,
			Fields:              []string{"status"},
			FieldsLimit:         100,
		})
	}()
	wg.Wait()

	if summaryErr != nil {
		return nil, fmt.Errorf("查询 SLA 汇总指标失败: %w", summaryErr)
	}

	result := &logtailmodel.LogSLAStoreSnapshot{
		Trend:         make([]logtailmodel.LogSLATrendPoint, 0),
		TopAPIs:       make([]logtailmodel.LogSLATopAPI, 0),
		RecentErrors:  make([]logtailmodel.LogSLARecentError, 0),
		Warnings:      make([]string, 0),
		QueryDuration: time.Since(startedAt).Milliseconds(),
	}
	if summaryResp != nil {
		result.Store = summaryResp.Store
		result.Metrics = parseLogSLAMetrics(summaryResp.Logs)
	}
	if topAPIErr != nil {
		result.Partial = true
		result.Warnings = append(result.Warnings, "异常接口聚合失败: "+topAPIErr.Error())
	} else if topAPIResp != nil {
		result.TopAPIs = parseLogSLATopAPIs(topAPIResp.Logs)
	}
	if recentErr != nil {
		result.Partial = true
		result.Warnings = append(result.Warnings, "最近错误查询失败: "+recentErr.Error())
	} else if recentResp != nil {
		result.RecentErrors = parseLogSLARecentErrors(recentResp.Logs)
	}
	if trendErr != nil {
		result.Partial = true
		result.Warnings = append(result.Warnings, "请求趋势查询失败: "+trendErr.Error())
	} else if trendResp != nil {
		result.Trend = parseLogSLATrend(trendResp.Hits)
	}
	return result, nil
}

func buildLogSLASummaryQuery(slowRequestThreshold float64) string {
	return buildLogSLASummaryQueryWithFilter("status:>=100 status:<600", slowRequestThreshold)
}

func buildLogSLASummaryQueryWithFilter(filter string, slowRequestThreshold float64) string {
	threshold := strconv.FormatFloat(slowRequestThreshold, 'f', -1, 64)
	return fmt.Sprintf(
		"%s | stats count() as request_total, count() if (status:>=400 status:<500) as http_4xx, count() if (status:>=500 status:<600) as http_5xx, count() if (request_time:>%s) as slow_request_count, avg(request_time) as avg_latency_seconds, quantile(0.95, request_time) as p95_latency_seconds, sum(body_bytes_sent) as response_bytes",
		strings.TrimSpace(filter),
		threshold,
	)
}

func buildLogSLATopAPIQuery() string {
	return buildLogSLATopAPIQueryWithFilter("status:>=100 status:<600")
}

func buildLogSLATopAPIQueryWithFilter(filter string) string {
	return buildLogSLATopAPIQueryWithFilterAndSort(filter, "errors")
}

func buildLogSLATopAPIQueryWithFilterAndSort(filter, sortMode string) string {
	sortField := "error_count"
	if strings.EqualFold(strings.TrimSpace(sortMode), "requests") {
		sortField = "request_total"
	}
	return strings.TrimSpace(filter) + " | stats by (request_method, request_path) count() as request_total, count() if (status:>=400 status:<500) as http_4xx, count() if (status:>=500 status:<600) as http_5xx, count() if (status:>=400 status:<600) as error_count, avg(request_time) as avg_latency_seconds, quantile(0.95, request_time) as p95_latency_seconds, sum(body_bytes_sent) as response_bytes | sort by (" + sortField + " desc) | limit 50"
}

func copyGinContext(c *gin.Context) *gin.Context {
	if c == nil {
		return nil
	}
	return c.Copy()
}

func parseLogSLAMetrics(rows []map[string]interface{}) logtailmodel.LogSLAMetrics {
	metrics := logtailmodel.LogSLAMetrics{}
	if len(rows) == 0 {
		return metrics
	}
	row := rows[0]
	metrics.RequestTotal = slaInt64(row, "request_total", "requests", "total")
	metrics.HTTP4xx = slaInt64(row, "http_4xx", "http4xx")
	metrics.HTTP5xx = slaInt64(row, "http_5xx", "http5xx")
	metrics.SlowRequestCount = slaInt64(row, "slow_request_count", "slow_requests")
	metrics.AvgLatencyMs = roundSLA(slaFloat64(row, "avg_latency_seconds", "avg_request_time") * 1000)
	metrics.P95LatencyMs = roundSLA(slaFloat64(row, "p95_latency_seconds", "p95_request_time") * 1000)
	metrics.ResponseBytes = slaInt64(row, "response_bytes", "body_bytes_sent")
	fillLogSLARates(&metrics)
	return metrics
}

func fillLogSLARates(metrics *logtailmodel.LogSLAMetrics) {
	if metrics == nil || metrics.RequestTotal <= 0 {
		return
	}
	total := float64(metrics.RequestTotal)
	metrics.HTTP4xxRate = roundSLA(float64(metrics.HTTP4xx) * 100 / total)
	metrics.HTTP5xxRate = roundSLA(float64(metrics.HTTP5xx) * 100 / total)
	metrics.ErrorRate = roundSLA(float64(metrics.HTTP4xx+metrics.HTTP5xx) * 100 / total)
	metrics.SlowRequestRate = roundSLA(float64(metrics.SlowRequestCount) * 100 / total)
	metrics.SuccessRate = roundSLA(math.Max(0, 100-metrics.ErrorRate))
	metrics.AvailabilityRate = roundSLA(math.Max(0, 100-metrics.HTTP5xxRate))
}

func parseLogSLATopAPIs(rows []map[string]interface{}) []logtailmodel.LogSLATopAPI {
	items := make([]logtailmodel.LogSLATopAPI, 0, len(rows))
	for _, row := range rows {
		item := logtailmodel.LogSLATopAPI{
			RequestMethod: slaString(row, "request_method", "method"),
			RequestPath:   slaString(row, "request_path", "request", "path"),
			RequestTotal:  slaInt64(row, "request_total", "requests", "total"),
			HTTP4xx:       slaInt64(row, "http_4xx", "http4xx"),
			HTTP5xx:       slaInt64(row, "http_5xx", "http5xx"),
			AvgLatencyMs:  roundSLA(slaFloat64(row, "avg_latency_seconds", "avg_request_time") * 1000),
			P95LatencyMs:  roundSLA(slaFloat64(row, "p95_latency_seconds", "p95_request_time") * 1000),
			ResponseBytes: slaInt64(row, "response_bytes", "body_bytes_sent"),
		}
		item.ErrorCount = item.HTTP4xx + item.HTTP5xx
		if item.RequestTotal > 0 {
			item.ErrorRate = roundSLA(float64(item.ErrorCount) * 100 / float64(item.RequestTotal))
		}
		if item.RequestPath != "" {
			items = append(items, item)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].ErrorCount != items[j].ErrorCount {
			return items[i].ErrorCount > items[j].ErrorCount
		}
		if items[i].P95LatencyMs != items[j].P95LatencyMs {
			return items[i].P95LatencyMs > items[j].P95LatencyMs
		}
		return items[i].RequestTotal > items[j].RequestTotal
	})
	return items
}

func parseLogSLARecentErrors(rows []map[string]interface{}) []logtailmodel.LogSLARecentError {
	items := make([]logtailmodel.LogSLARecentError, 0, len(rows))
	for _, row := range rows {
		items = append(items, logtailmodel.LogSLARecentError{
			Timestamp:     slaString(row, "_time", "@timestamp", "timestamp"),
			RequestMethod: slaString(row, "request_method", "method"),
			RequestPath:   slaString(row, "request_path", "request", "path"),
			Domain:        slaString(row, "domain", "host"),
			Status:        int(slaInt64(row, "status", "code")),
			LatencyMs:     roundSLA(slaFloat64(row, "request_time", "responsetime") * 1000),
			UpstreamAddr:  slaString(row, "upstreamaddr", "upstream_addr"),
			ClientIP:      slaString(row, "clientip", "remote_addr", "client_ip"),
			RequestID:     slaString(row, "request_id"),
			TraceID:       slaString(row, "trace_id", "x_zai_trace_id"),
		})
	}
	return items
}

func parseLogSLATrend(series []logtailmodel.LogQueryHitSeries) []logtailmodel.LogSLATrendPoint {
	points := make(map[string]*logtailmodel.LogSLATrendPoint)
	for _, item := range series {
		status, _ := strconv.Atoi(strings.TrimSpace(item.Fields["status"]))
		for index, timestamp := range item.Timestamps {
			if index >= len(item.Values) {
				break
			}
			point := points[timestamp]
			if point == nil {
				point = &logtailmodel.LogSLATrendPoint{Timestamp: timestamp}
				points[timestamp] = point
			}
			value := item.Values[index]
			point.RequestTotal += value
			switch {
			case status == 200:
				point.HTTP200 += value
			case status >= 400 && status < 500:
				point.HTTP4xx += value
			case status >= 500 && status < 600:
				point.HTTP5xx += value
			}
		}
	}
	result := make([]logtailmodel.LogSLATrendPoint, 0, len(points))
	for _, point := range points {
		result = append(result, *point)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Timestamp < result[j].Timestamp })
	return result
}

func slaInt64(values map[string]interface{}, keys ...string) int64 {
	return int64(slaFloat64(values, keys...))
}

func slaFloat64(values map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		value, exists := values[key]
		if !exists || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed
		case float32:
			return float64(typed)
		case int:
			return float64(typed)
		case int64:
			return float64(typed)
		case int32:
			return float64(typed)
		case string:
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
				return parsed
			}
		case fmt.Stringer:
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed.String()), 64); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func slaString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, exists := values[key]
		if !exists || value == nil {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" {
			return text
		}
	}
	return ""
}

func roundSLA(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*10000) / 10000
}
