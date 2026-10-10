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

	cmdbmodel "dodevops-api/api/cmdb/model"
	logtailmodel "dodevops-api/api/logtail/model"

	"github.com/gin-gonic/gin"
)

const maxBusinessLogAnalysisStores = 8

var ErrInvalidBusinessLogAnalysisFilter = errors.New("无效的业务日志分析筛选条件")

type BusinessLogAnalysisQuerier interface {
	QuerySLAStore(*gin.Context, logtailmodel.LogSLAStoreQuery) (*logtailmodel.LogSLAStoreSnapshot, error)
	QueryLogs(*gin.Context, logtailmodel.LogQueryLogsRequest) (*logtailmodel.LogQueryLogsResponse, error)
}

type businessLogAnalysisObservation struct {
	businessStoreObservation
	Domains     []map[string]interface{}
	Statuses    []map[string]interface{}
	Clients     []map[string]interface{}
	PathClients []map[string]interface{}
	Warnings    []string
}

func (s *CmdbService) GetBusinessModelLogAnalysis(c *gin.Context, querier BusinessLogAnalysisQuerier, businessID uint, query cmdbmodel.BusinessLogAnalysisQuery) (map[string]interface{}, error) {
	start, end, step, windowLabel, err := normalizeBusinessObservabilityWindow(query.BusinessObservabilityQuery)
	if err != nil {
		return nil, err
	}
	overview, err := s.GetBusinessModelOverview(businessID)
	if err != nil {
		return nil, err
	}
	topology, err := s.getBusinessModelTopology(businessID, 8, "downstream", false)
	if err != nil {
		return nil, err
	}
	bindings, warnings, err := s.resolveBusinessObservabilityStores(topology)
	if err != nil {
		return nil, err
	}
	sourceType, err := normalizeBusinessLogSourceType(query.SourceType)
	if err != nil {
		return nil, err
	}
	bindings, availableSources := filterBusinessLogAnalysisBindings(bindings, topology, sourceType)
	if query.StoreID > 0 {
		bindings = filterBusinessLogAnalysisBindingsByStore(bindings, query.StoreID)
	}
	if len(bindings) > maxBusinessLogAnalysisStores {
		warnings = append(warnings, fmt.Sprintf("匹配 %d 个入口日志库，本次最多分析前 %d 个", len(bindings), maxBusinessLogAnalysisStores))
		bindings = bindings[:maxBusinessLogAnalysisStores]
	}

	filter, recentFilter, err := buildBusinessLogStatusFilters(query.StatusClass)
	if err != nil {
		return nil, err
	}
	extraFilters := make(map[string]interface{})
	if value := strings.TrimSpace(query.Domain); value != "" {
		extraFilters["domain"] = value
	}
	if value := strings.TrimSpace(query.RequestPath); value != "" {
		extraFilters["request_path"] = value
	}
	if value := strings.TrimSpace(query.ClientIP); value != "" {
		extraFilters["remote_addr"] = value
	}

	observations := queryBusinessLogAnalysisStores(c, querier, bindings, start, end, step, filter, recentFilter, extraFilters)
	baseObservations := make([]businessStoreObservation, 0, len(observations))
	for _, observation := range observations {
		baseObservations = append(baseObservations, observation.businessStoreObservation)
		warnings = append(warnings, observation.Warnings...)
	}
	metrics := aggregateBusinessSLAMetrics(baseObservations, topology)
	clientSet := collectBusinessLogAnalysisClients(observations)
	metrics.UniqueClientCount = int64(len(clientSet))
	topAPIs := aggregateBusinessSLATopAPIs(baseObservations)
	enrichBusinessLogAnalysisAPIClients(topAPIs, observations)
	logSources, queryWarnings, successfulQueries, partialQueries := buildBusinessLogSources(baseObservations)
	warnings = append(warnings, queryWarnings...)

	dataStatus := "ready"
	switch {
	case len(bindings) == 0:
		dataStatus = "unconfigured"
	case successfulQueries == 0:
		dataStatus = "unavailable"
	case successfulQueries < len(bindings) || partialQueries > 0:
		dataStatus = "partial"
	}

	durationSeconds := end.Sub(start).Seconds()
	averageQPS := 0.0
	if durationSeconds > 0 {
		averageQPS = roundObservability(float64(metrics.RequestTotal) / durationSeconds)
	}
	return map[string]interface{}{
		"summary":          overview["summary"],
		"metrics":          metrics,
		"derived":          map[string]interface{}{"averageQps": averageQPS},
		"trafficTrend":     aggregateBusinessSLATrend(baseObservations),
		"domains":          aggregateBusinessLogDomains(observations),
		"statuses":         aggregateBusinessLogStatuses(observations),
		"topClients":       aggregateBusinessLogClients(observations),
		"topApis":          topAPIs,
		"sourceBreakdown":  buildBusinessLogSourceBreakdown(observations, topology),
		"recentErrors":     aggregateBusinessSLARecentErrors(baseObservations),
		"availableSources": availableSources,
		"logSources":       logSources,
		"filters": map[string]interface{}{
			"sourceType":  sourceType,
			"storeId":     query.StoreID,
			"domain":      strings.TrimSpace(query.Domain),
			"statusClass": normalizeBusinessLogStatusClass(query.StatusClass),
			"requestPath": strings.TrimSpace(query.RequestPath),
			"clientIp":    strings.TrimSpace(query.ClientIP),
		},
		"window": map[string]interface{}{
			"label": windowLabel,
			"start": start.Format(time.RFC3339Nano),
			"end":   end.Format(time.RFC3339Nano),
			"step":  step,
		},
		"dataStatus":  dataStatus,
		"partial":     dataStatus == "partial" || dataStatus == "unavailable" || len(warnings) > 0,
		"warnings":    uniqueStrings(warnings),
		"generatedAt": time.Now().Format(time.RFC3339Nano),
	}, nil
}

func normalizeBusinessLogSourceType(value string) (string, error) {
	switch normalized := strings.ToLower(strings.TrimSpace(value)); normalized {
	case "", "all":
		return "all", nil
	case cmdbmodel.CmdbResourceTypeNginx, cmdbmodel.CmdbResourceTypeGateway:
		return normalized, nil
	default:
		return "", fmt.Errorf("%w: sourceType 仅支持 all、nginx、gateway", ErrInvalidBusinessLogAnalysisFilter)
	}
}

func normalizeBusinessLogStatusClass(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "all" {
		return ""
	}
	return normalized
}

func buildBusinessLogStatusFilters(value string) (string, string, error) {
	normalized := normalizeBusinessLogStatusClass(value)
	if normalized == "" {
		return "status:>=100 status:<600", "status:>=400 status:<600", nil
	}
	if len(normalized) == 3 && normalized[1:] == "xx" && normalized[0] >= '1' && normalized[0] <= '5' {
		lower := int(normalized[0]-'0') * 100
		filter := fmt.Sprintf("status:>=%d status:<%d", lower, lower+100)
		return filter, filter, nil
	}
	status, err := strconv.Atoi(normalized)
	if err != nil || status < 100 || status > 599 {
		return "", "", fmt.Errorf("%w: statusClass 仅支持 1xx-5xx 或具体 HTTP 状态码", ErrInvalidBusinessLogAnalysisFilter)
	}
	filter := fmt.Sprintf("status:%d", status)
	return filter, filter, nil
}

func filterBusinessLogAnalysisBindings(bindings []businessStoreBinding, topology map[string]interface{}, sourceType string) ([]businessStoreBinding, []map[string]interface{}) {
	nodes, _ := topology["nodes"].([]map[string]interface{})
	nodeByID := make(map[uint]map[string]interface{}, len(nodes))
	for _, node := range nodes {
		nodeByID[uintFromAny(node["id"])] = node
	}
	selected := make([]businessStoreBinding, 0)
	availableMap := make(map[string]map[string]interface{})
	for _, binding := range bindings {
		nodeIDs := binding.DirectNodeIDs
		if len(nodeIDs) == 0 {
			nodeIDs = binding.NodeIDs
		}
		matched := false
		for _, nodeID := range nodeIDs {
			node := nodeByID[nodeID]
			nodeType := normalizeResourceType(stringFromAny(node["resourceType"]))
			if nodeType != cmdbmodel.CmdbResourceTypeNginx && nodeType != cmdbmodel.CmdbResourceTypeGateway {
				continue
			}
			key := nodeType + ":" + strconv.FormatUint(uint64(nodeID), 10)
			availableMap[key] = map[string]interface{}{
				"nodeId":     nodeID,
				"sourceType": nodeType,
				"name":       stringFromAny(node["displayName"]),
				"storeId":    binding.Store.ID,
				"storeName":  binding.Store.Name,
				"storeCode":  binding.Store.Code,
			}
			if sourceType == "all" || sourceType == nodeType {
				matched = true
			}
		}
		if matched {
			selected = append(selected, binding)
		}
	}
	available := make([]map[string]interface{}, 0, len(availableMap))
	for _, item := range availableMap {
		available = append(available, item)
	}
	sort.Slice(available, func(i, j int) bool {
		return stringFromAny(available[i]["name"]) < stringFromAny(available[j]["name"])
	})
	return selected, available
}

func filterBusinessLogAnalysisBindingsByStore(bindings []businessStoreBinding, storeID uint) []businessStoreBinding {
	if storeID == 0 {
		return bindings
	}
	selected := make([]businessStoreBinding, 0, len(bindings))
	for _, binding := range bindings {
		if binding.Store.ID == storeID {
			selected = append(selected, binding)
		}
	}
	return selected
}

func queryBusinessLogAnalysisStores(c *gin.Context, querier BusinessLogAnalysisQuerier, bindings []businessStoreBinding, start, end time.Time, step, filter, recentFilter string, extraFilters map[string]interface{}) []businessLogAnalysisObservation {
	results := make([]businessLogAnalysisObservation, len(bindings))
	var wg sync.WaitGroup
	wg.Add(len(bindings))
	for index := range bindings {
		index := index
		go func() {
			defer wg.Done()
			binding := bindings[index]
			results[index].Binding = binding
			var inner sync.WaitGroup
			var warningMu sync.Mutex
			inner.Add(5)
			go func() {
				defer inner.Done()
				results[index].Snapshot, results[index].Err = querier.QuerySLAStore(copyBusinessLogContext(c), logtailmodel.LogSLAStoreQuery{
					StoreID: binding.Store.ID, Start: start.Format(time.RFC3339Nano), End: end.Format(time.RFC3339Nano), Step: step,
					SlowRequestThreshold: defaultSlowRequestThreshold, Filter: filter, RecentFilter: recentFilter, ExtraFilters: extraFilters, TopAPISort: "requests",
				})
			}()
			queryRows := func(query string, limit int, target *[]map[string]interface{}, label string) {
				defer inner.Done()
				response, err := querier.QueryLogs(copyBusinessLogContext(c), logtailmodel.LogQueryLogsRequest{
					LogQueryBaseRequest: logtailmodel.LogQueryBaseRequest{
						StoreID: binding.Store.ID, Query: filter + " | " + query, Start: start.Format(time.RFC3339Nano), End: end.Format(time.RFC3339Nano), ExtraFilters: extraFilters,
					},
					Limit: limit,
				})
				if err != nil {
					warningMu.Lock()
					results[index].Warnings = append(results[index].Warnings, fmt.Sprintf("日志库 %s %s失败: %v", binding.Store.Name, label, err))
					warningMu.Unlock()
					return
				}
				if response != nil {
					*target = response.Logs
				}
			}
			go queryRows("stats by (domain) count() as request_total, avg(request_time) as avg_latency_seconds, quantile(0.95, request_time) as p95_latency_seconds, sum(body_bytes_sent) as response_bytes | sort by (request_total desc) | limit 100", 100, &results[index].Domains, "域名聚合")
			go queryRows("stats by (status) count() as request_total | sort by (request_total desc) | limit 100", 100, &results[index].Statuses, "状态码聚合")
			go queryRows("stats by (remote_addr) count() as request_total | sort by (request_total desc) | limit 1000", 1000, &results[index].Clients, "客户端聚合")
			go queryRows("stats by (request_path, remote_addr) count() as request_total | sort by (request_total desc) | limit 5000", 5000, &results[index].PathClients, "接口 UV 聚合")
			inner.Wait()
		}()
	}
	wg.Wait()
	return results
}

func copyBusinessLogContext(c *gin.Context) *gin.Context {
	if c == nil {
		return nil
	}
	return c.Copy()
}

func collectBusinessLogAnalysisClients(observations []businessLogAnalysisObservation) map[string]struct{} {
	clients := make(map[string]struct{})
	for _, observation := range observations {
		for _, row := range observation.Clients {
			if client := stringFromAny(firstBusinessLogValue(row, "remote_addr", "clientip", "client_ip")); client != "" {
				clients[client] = struct{}{}
			}
		}
	}
	return clients
}

func aggregateBusinessLogDomains(observations []businessLogAnalysisObservation) []map[string]interface{} {
	type domainAggregation struct {
		requests        int64
		responseBytes   int64
		weightedLatency float64
		p95LatencyMs    float64
	}
	items := make(map[string]*domainAggregation)
	for _, observation := range observations {
		for _, row := range observation.Domains {
			domain := stringFromAny(firstBusinessLogValue(row, "domain", "host"))
			if domain == "" {
				domain = "未识别域名"
			}
			requests := int64(observabilityFloat(firstBusinessLogValue(row, "request_total", "requests")))
			item := items[domain]
			if item == nil {
				item = &domainAggregation{}
				items[domain] = item
			}
			item.requests += requests
			item.responseBytes += int64(observabilityFloat(firstBusinessLogValue(row, "response_bytes", "body_bytes_sent")))
			item.weightedLatency += observabilityFloat(firstBusinessLogValue(row, "avg_latency_seconds")) * 1000 * float64(requests)
			item.p95LatencyMs = math.Max(item.p95LatencyMs, observabilityFloat(firstBusinessLogValue(row, "p95_latency_seconds"))*1000)
		}
	}
	result := make([]map[string]interface{}, 0, len(items))
	for domain, item := range items {
		avgLatency := 0.0
		if item.requests > 0 {
			avgLatency = item.weightedLatency / float64(item.requests)
		}
		result = append(result, map[string]interface{}{
			"domain": domain, "requestTotal": item.requests, "avgLatencyMs": roundObservability(avgLatency),
			"p95LatencyMs": roundObservability(item.p95LatencyMs), "responseBytes": item.responseBytes,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return observabilityFloat(result[i]["requestTotal"]) > observabilityFloat(result[j]["requestTotal"])
	})
	return result
}

func aggregateBusinessLogStatuses(observations []businessLogAnalysisObservation) []map[string]interface{} {
	items := make(map[string]int64)
	for _, observation := range observations {
		for _, row := range observation.Statuses {
			status := stringFromAny(firstBusinessLogValue(row, "status", "code"))
			if status == "" {
				continue
			}
			items[status] += int64(observabilityFloat(firstBusinessLogValue(row, "request_total", "requests")))
		}
	}
	result := make([]map[string]interface{}, 0, len(items))
	for status, requests := range items {
		result = append(result, map[string]interface{}{"status": status, "requestTotal": requests})
	}
	sort.Slice(result, func(i, j int) bool {
		return observabilityFloat(result[i]["requestTotal"]) > observabilityFloat(result[j]["requestTotal"])
	})
	return result
}

func aggregateBusinessLogClients(observations []businessLogAnalysisObservation) []map[string]interface{} {
	items := make(map[string]int64)
	for _, observation := range observations {
		for _, row := range observation.Clients {
			client := stringFromAny(firstBusinessLogValue(row, "remote_addr", "clientip", "client_ip"))
			if client == "" {
				continue
			}
			items[client] += int64(observabilityFloat(firstBusinessLogValue(row, "request_total", "requests")))
		}
	}
	result := make([]map[string]interface{}, 0, len(items))
	for client, requests := range items {
		result = append(result, map[string]interface{}{"clientIp": client, "requestTotal": requests})
	}
	sort.Slice(result, func(i, j int) bool {
		return observabilityFloat(result[i]["requestTotal"]) > observabilityFloat(result[j]["requestTotal"])
	})
	if len(result) > 20 {
		result = result[:20]
	}
	return result
}

func enrichBusinessLogAnalysisAPIClients(items []cmdbmodel.BusinessSLATopAPIVo, observations []businessLogAnalysisObservation) {
	clientsByPath := make(map[string]map[string]struct{})
	for _, observation := range observations {
		for _, row := range observation.PathClients {
			path := stringFromAny(firstBusinessLogValue(row, "request_path", "request", "request_uri"))
			client := stringFromAny(firstBusinessLogValue(row, "remote_addr", "clientip", "client_ip"))
			if path == "" || client == "" {
				continue
			}
			if clientsByPath[path] == nil {
				clientsByPath[path] = make(map[string]struct{})
			}
			clientsByPath[path][client] = struct{}{}
		}
	}
	for index := range items {
		items[index].UniqueClients = int64(len(clientsByPath[items[index].RequestPath]))
	}
}

func buildBusinessLogSourceBreakdown(observations []businessLogAnalysisObservation, topology map[string]interface{}) []map[string]interface{} {
	nodes, _ := topology["nodes"].([]map[string]interface{})
	nodeByID := make(map[uint]map[string]interface{}, len(nodes))
	for _, node := range nodes {
		nodeByID[uintFromAny(node["id"])] = node
	}
	result := make([]map[string]interface{}, 0, len(observations))
	for _, observation := range observations {
		item := map[string]interface{}{
			"storeId": observation.Binding.Store.ID, "storeName": observation.Binding.Store.Name, "storeCode": observation.Binding.Store.Code,
			"requestTotal": int64(0), "http4xx": int64(0), "http5xx": int64(0), "avgLatencyMs": float64(0), "p95LatencyMs": float64(0),
		}
		if observation.Snapshot != nil {
			item["requestTotal"] = observation.Snapshot.Metrics.RequestTotal
			item["http4xx"] = observation.Snapshot.Metrics.HTTP4xx
			item["http5xx"] = observation.Snapshot.Metrics.HTTP5xx
			item["avgLatencyMs"] = observation.Snapshot.Metrics.AvgLatencyMs
			item["p95LatencyMs"] = observation.Snapshot.Metrics.P95LatencyMs
		}
		names := make([]string, 0)
		types := make([]string, 0)
		for _, nodeID := range observation.Binding.DirectNodeIDs {
			node := nodeByID[nodeID]
			if node == nil {
				continue
			}
			names = append(names, stringFromAny(node["displayName"]))
			types = append(types, stringFromAny(node["resourceType"]))
		}
		item["nodeNames"] = uniqueStrings(names)
		item["sourceTypes"] = uniqueStrings(types)
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		return observabilityFloat(result[i]["requestTotal"]) > observabilityFloat(result[j]["requestTotal"])
	})
	return result
}

func firstBusinessLogValue(row map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if value, exists := row[key]; exists {
			return value
		}
	}
	return nil
}
