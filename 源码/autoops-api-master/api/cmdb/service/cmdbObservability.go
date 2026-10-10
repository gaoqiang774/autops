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

const (
	defaultObservabilityWindow       = 15 * time.Minute
	maxObservabilityWindow           = 30 * 24 * time.Hour
	maxBusinessObservabilityStores   = 20
	defaultBusinessSLOTarget         = 99.9
	defaultSlowRequestThreshold      = 1.0
	observabilityWarningHTTP5xxRate  = 1.0
	observabilityCriticalHTTP5xxRate = 5.0
	observabilityWarningHTTP4xxRate  = 10.0
	observabilityWarningP95Ms        = 1000.0
	observabilityCriticalP95Ms       = 3000.0
)

var ErrInvalidObservabilityWindow = errors.New("无效的可观测时间窗口")

type BusinessObservabilityLogQuerier interface {
	QuerySLAStore(*gin.Context, logtailmodel.LogSLAStoreQuery) (*logtailmodel.LogSLAStoreSnapshot, error)
}

type businessStoreBinding struct {
	Store         logtailmodel.LogStore
	NodeIDs       []uint
	DirectNodeIDs []uint
}

type businessStoreObservation struct {
	Binding  businessStoreBinding
	Snapshot *logtailmodel.LogSLAStoreSnapshot
	Err      error
}

// GetBusinessModelObservability returns the complete payload for the SLA observability workbench.
func (s *CmdbService) GetBusinessModelObservability(c *gin.Context, logQuerier BusinessObservabilityLogQuerier, businessID uint, query cmdbmodel.BusinessObservabilityQuery) (map[string]interface{}, error) {
	start, end, step, windowLabel, err := normalizeBusinessObservabilityWindow(query)
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

	bindings, bindingWarnings, err := s.resolveBusinessObservabilityStores(topology)
	if err != nil {
		return nil, err
	}
	if len(bindings) > maxBusinessObservabilityStores {
		bindingWarnings = append(bindingWarnings, fmt.Sprintf("业务共绑定 %d 个日志库，本次最多查询前 %d 个", len(bindings), maxBusinessObservabilityStores))
		bindings = bindings[:maxBusinessObservabilityStores]
	}

	observations := s.queryBusinessStoreObservations(c, logQuerier, bindings, start, end, step)
	metrics := aggregateBusinessSLAMetrics(observations, topology)
	alerts, activeAlertCount, alertErr := s.listBusinessFiringLogAlerts(bindings)
	if alertErr != nil {
		bindingWarnings = append(bindingWarnings, "活动日志告警查询失败: "+alertErr.Error())
	} else {
		metrics.ActiveAlertCount = activeAlertCount
	}

	logSources, queryWarnings, successfulQueries, partialQueries := buildBusinessLogSources(observations)
	warnings := append(bindingWarnings, queryWarnings...)
	trend := aggregateBusinessSLATrend(observations)
	topAPIs := aggregateBusinessSLATopAPIs(observations)
	recentErrors := aggregateBusinessSLARecentErrors(observations)
	nodeSignals := buildBusinessNodeSignals(observations)
	nodeAnalytics := buildBusinessNodeAnalytics(observations, topology)
	enrichBusinessTopologyWithSignals(topology, nodeSignals)

	dataStatus := "ready"
	switch {
	case len(bindings) == 0:
		dataStatus = "unconfigured"
	case successfulQueries == 0:
		dataStatus = "unavailable"
	case successfulQueries < len(bindings) || partialQueries > 0:
		dataStatus = "partial"
	}
	partial := dataStatus == "partial" || dataStatus == "unavailable" || len(warnings) > 0

	return map[string]interface{}{
		"summary":        overview["summary"],
		"dimensions":     overview["dimensions"],
		"metrics":        metrics,
		"topology":       topology,
		"nodeSignals":    nodeSignals,
		"nodeAnalytics":  nodeAnalytics,
		"riskNodes":      overview["riskNodes"],
		"coverage":       overview["coverage"],
		"scoreBreakdown": overview["scoreBreakdown"],
		"trafficTrend":   trend,
		"topApis":        topAPIs,
		"recentErrors":   recentErrors,
		"activeAlerts":   alerts,
		"logSources":     logSources,
		"window": map[string]interface{}{
			"label": windowLabel,
			"start": start.Format(time.RFC3339Nano),
			"end":   end.Format(time.RFC3339Nano),
			"step":  step,
		},
		"dataStatus":  dataStatus,
		"partial":     partial,
		"warnings":    warnings,
		"generatedAt": time.Now().Format(time.RFC3339Nano),
	}, nil
}

func buildBusinessNodeAnalytics(observations []businessStoreObservation, topology map[string]interface{}) map[string]map[string]interface{} {
	grouped := make(map[uint][]businessStoreObservation)
	storeIDs := make(map[uint]map[uint]struct{})
	for _, observation := range observations {
		nodeIDs := observation.Binding.DirectNodeIDs
		if len(nodeIDs) == 0 {
			nodeIDs = observation.Binding.NodeIDs
		}
		for _, nodeID := range nodeIDs {
			grouped[nodeID] = append(grouped[nodeID], observation)
			if storeIDs[nodeID] == nil {
				storeIDs[nodeID] = make(map[uint]struct{})
			}
			storeIDs[nodeID][observation.Binding.Store.ID] = struct{}{}
		}
	}

	result := make(map[string]map[string]interface{}, len(grouped))
	for nodeID, nodeObservations := range grouped {
		result[strconv.FormatUint(uint64(nodeID), 10)] = map[string]interface{}{
			"storeIds":     sortedUintSet(storeIDs[nodeID]),
			"metrics":      aggregateBusinessSLAMetrics(nodeObservations, topology),
			"trafficTrend": aggregateBusinessSLATrend(nodeObservations),
			"topApis":      aggregateBusinessSLATopAPIs(nodeObservations),
			"recentErrors": aggregateBusinessSLARecentErrors(nodeObservations),
		}
	}
	return result
}

func normalizeBusinessObservabilityWindow(query cmdbmodel.BusinessObservabilityQuery) (time.Time, time.Time, string, string, error) {
	now := time.Now()
	startText := strings.TrimSpace(query.Start)
	endText := strings.TrimSpace(query.End)
	if startText != "" || endText != "" {
		if startText == "" || endText == "" {
			return time.Time{}, time.Time{}, "", "", fmt.Errorf("%w: start 和 end 必须同时提供", ErrInvalidObservabilityWindow)
		}
		start, err := time.Parse(time.RFC3339, startText)
		if err != nil {
			return time.Time{}, time.Time{}, "", "", fmt.Errorf("%w: start 必须为 RFC3339 时间", ErrInvalidObservabilityWindow)
		}
		end, err := time.Parse(time.RFC3339, endText)
		if err != nil {
			return time.Time{}, time.Time{}, "", "", fmt.Errorf("%w: end 必须为 RFC3339 时间", ErrInvalidObservabilityWindow)
		}
		duration := end.Sub(start)
		if duration <= 0 || duration > maxObservabilityWindow {
			return time.Time{}, time.Time{}, "", "", fmt.Errorf("%w: 时间范围必须大于 0 且不超过 30 天", ErrInvalidObservabilityWindow)
		}
		return start, end, observabilityStep(duration), "custom", nil
	}

	duration, err := parseObservabilityDuration(strings.TrimSpace(query.Window))
	if err != nil {
		return time.Time{}, time.Time{}, "", "", err
	}
	return now.Add(-duration), now, observabilityStep(duration), duration.String(), nil
}

func parseObservabilityDuration(value string) (time.Duration, error) {
	if value == "" {
		return defaultObservabilityWindow, nil
	}
	lower := strings.ToLower(value)
	if strings.HasSuffix(lower, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(lower, "d"))
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("%w: window 格式不正确", ErrInvalidObservabilityWindow)
		}
		lower = fmt.Sprintf("%dh", days*24)
	}
	duration, err := time.ParseDuration(lower)
	if err != nil || duration <= 0 || duration > maxObservabilityWindow {
		return 0, fmt.Errorf("%w: window 必须大于 0 且不超过 30 天", ErrInvalidObservabilityWindow)
	}
	return duration, nil
}

func observabilityStep(duration time.Duration) string {
	switch {
	case duration <= time.Hour:
		return "1m"
	case duration <= 6*time.Hour:
		return "5m"
	case duration <= 24*time.Hour:
		return "15m"
	case duration <= 7*24*time.Hour:
		return "1h"
	default:
		return "6h"
	}
}

func (s *CmdbService) resolveBusinessObservabilityStores(topology map[string]interface{}) ([]businessStoreBinding, []string, error) {
	nodes, _ := topology["nodes"].([]map[string]interface{})
	edges, _ := topology["edges"].([]map[string]interface{})
	storeNodes := make(map[uint]map[uint]struct{})
	projectNodes := make(map[uint]map[uint]struct{})
	storeCodes := make(map[string]map[uint]struct{})
	projectCodes := make(map[string]map[uint]struct{})
	incoming := make(map[uint][]uint)
	for _, edge := range edges {
		sourceID := uintFromAny(firstPresent(edge, "sourceId", "source", "srcId"))
		targetID := uintFromAny(firstPresent(edge, "targetId", "target", "dstId"))
		if sourceID > 0 && targetID > 0 {
			incoming[targetID] = append(incoming[targetID], sourceID)
		}
	}

	addNode := func(target map[uint]map[uint]struct{}, sourceID, nodeID uint) {
		if sourceID == 0 || nodeID == 0 {
			return
		}
		if target[sourceID] == nil {
			target[sourceID] = make(map[uint]struct{})
		}
		target[sourceID][nodeID] = struct{}{}
	}
	for _, node := range nodes {
		nodeID := uintFromAny(node["id"])
		resourceType := normalizeResourceType(stringFromAny(node["resourceType"]))
		sourceType := normalizeResourceType(stringFromAny(node["sourceType"]))
		sourceID := uintFromAny(node["sourceId"])
		sourceKey := strings.TrimSpace(stringFromAny(node["sourceKey"]))
		nodeIDs := observabilityRelatedNodeIDs(nodeID, resourceType, incoming)
		for _, relatedNodeID := range nodeIDs {
			switch sourceType {
			case cmdbmodel.CmdbResourceTypeLogStore:
				addNode(storeNodes, sourceID, relatedNodeID)
			case cmdbmodel.CmdbResourceTypeLogProject:
				addNode(projectNodes, sourceID, relatedNodeID)
			}
			if resourceType == cmdbmodel.CmdbResourceTypeLogStore {
				addNode(storeNodes, sourceID, relatedNodeID)
			}
			if resourceType == cmdbmodel.CmdbResourceTypeLogProject {
				addNode(projectNodes, sourceID, relatedNodeID)
			}
		}
		if sourceID == 0 && sourceKey != "" {
			isProjectSource := sourceType == cmdbmodel.CmdbResourceTypeLogProject || resourceType == cmdbmodel.CmdbResourceTypeLogProject
			target := storeCodes
			if isProjectSource {
				target = projectCodes
			}
			if sourceType == cmdbmodel.CmdbResourceTypeLogStore || resourceType == cmdbmodel.CmdbResourceTypeLogStore || isProjectSource {
				if target[sourceKey] == nil {
					target[sourceKey] = make(map[uint]struct{})
				}
				for _, relatedNodeID := range nodeIDs {
					target[sourceKey][relatedNodeID] = struct{}{}
				}
			}
		}

		props, _ := node["properties"].(map[string]interface{})
		for _, storeID := range uintValuesFromProperties(props, "logStoreId", "storeId", "log_store_id", "logStoreIds", "storeIds") {
			for _, relatedNodeID := range nodeIDs {
				addNode(storeNodes, storeID, relatedNodeID)
			}
		}
		for _, projectID := range observabilityProjectIDsFromNode(resourceType, sourceType, props) {
			for _, relatedNodeID := range nodeIDs {
				addNode(projectNodes, projectID, relatedNodeID)
			}
		}
		for _, key := range []string{"storeCode", "logStore", "logStoreCode"} {
			code := strings.TrimSpace(stringFromAny(props[key]))
			if code == "" {
				continue
			}
			if storeCodes[code] == nil {
				storeCodes[code] = make(map[uint]struct{})
			}
			for _, relatedNodeID := range nodeIDs {
				storeCodes[code][relatedNodeID] = struct{}{}
			}
		}
	}

	storesByID := make(map[uint]logtailmodel.LogStore)
	if len(storeNodes) > 0 {
		ids := sortedUintMapKeys(storeNodes)
		var stores []logtailmodel.LogStore
		if err := s.dao.DB().Where("id IN ?", ids).Find(&stores).Error; err != nil {
			return nil, nil, err
		}
		for _, store := range stores {
			storesByID[store.ID] = store
		}
	}
	if len(projectCodes) > 0 {
		codes := make([]string, 0, len(projectCodes))
		for code := range projectCodes {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		var projects []logtailmodel.LogProject
		if err := s.dao.DB().Where("code IN ?", codes).Find(&projects).Error; err != nil {
			return nil, nil, err
		}
		for _, project := range projects {
			for nodeID := range projectCodes[project.Code] {
				addNode(projectNodes, project.ID, nodeID)
			}
		}
	}
	if len(projectNodes) > 0 {
		ids := sortedUintMapKeys(projectNodes)
		var stores []logtailmodel.LogStore
		if err := s.dao.DB().Where("project_id IN ?", ids).Find(&stores).Error; err != nil {
			return nil, nil, err
		}
		for _, store := range stores {
			storesByID[store.ID] = store
			for nodeID := range projectNodes[store.ProjectID] {
				addNode(storeNodes, store.ID, nodeID)
			}
		}
	}
	if len(storeCodes) > 0 {
		codes := make([]string, 0, len(storeCodes))
		for code := range storeCodes {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		var stores []logtailmodel.LogStore
		if err := s.dao.DB().Where("code IN ?", codes).Find(&stores).Error; err != nil {
			return nil, nil, err
		}
		for _, store := range stores {
			storesByID[store.ID] = store
			for nodeID := range storeCodes[store.Code] {
				addNode(storeNodes, store.ID, nodeID)
			}
		}
	}

	warnings := make([]string, 0)
	for storeID := range storeNodes {
		if _, exists := storesByID[storeID]; !exists {
			warnings = append(warnings, fmt.Sprintf("绑定的日志库 %d 不存在或已删除", storeID))
		}
	}
	bindings := make([]businessStoreBinding, 0, len(storesByID))
	for _, store := range storesByID {
		nodeIDs := sortedUintSet(storeNodes[store.ID])
		bindings = append(bindings, businessStoreBinding{
			Store:         store,
			NodeIDs:       nodeIDs,
			DirectNodeIDs: directBusinessStoreNodeIDs(topology, store),
		})
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].Store.ID < bindings[j].Store.ID })
	sort.Strings(warnings)
	return bindings, warnings, nil
}

func directBusinessStoreNodeIDs(topology map[string]interface{}, store logtailmodel.LogStore) []uint {
	nodes, _ := topology["nodes"].([]map[string]interface{})
	edges, _ := topology["edges"].([]map[string]interface{})
	incoming := make(map[uint][]uint)
	for _, edge := range edges {
		sourceID := uintFromAny(firstPresent(edge, "sourceId", "source", "srcId"))
		targetID := uintFromAny(firstPresent(edge, "targetId", "target", "dstId"))
		if sourceID > 0 && targetID > 0 {
			incoming[targetID] = append(incoming[targetID], sourceID)
		}
	}
	owners := make(map[uint]struct{})
	addOwner := func(id uint) {
		if id > 0 {
			owners[id] = struct{}{}
		}
	}
	for _, node := range nodes {
		nodeID := uintFromAny(node["id"])
		resourceType := normalizeResourceType(stringFromAny(node["resourceType"]))
		sourceType := normalizeResourceType(stringFromAny(node["sourceType"]))
		sourceID := uintFromAny(node["sourceId"])
		props, _ := node["properties"].(map[string]interface{})
		storeMatch := containsUint(uintValuesFromProperties(props, "logStoreId", "storeId", "log_store_id", "logStoreIds", "storeIds"), store.ID)
		if (sourceType == cmdbmodel.CmdbResourceTypeLogStore || resourceType == cmdbmodel.CmdbResourceTypeLogStore) && sourceID == store.ID {
			storeMatch = true
		}
		projectMatch := store.ProjectID > 0 && containsUint(observabilityProjectIDsFromNode(resourceType, sourceType, props), store.ProjectID)
		if store.ProjectID > 0 && (sourceType == cmdbmodel.CmdbResourceTypeLogProject || resourceType == cmdbmodel.CmdbResourceTypeLogProject) && sourceID == store.ProjectID {
			projectMatch = true
		}
		codeMatch := false
		for _, key := range []string{"storeCode", "logStore", "logStoreCode"} {
			if strings.TrimSpace(stringFromAny(props[key])) == store.Code {
				codeMatch = true
				break
			}
		}
		switch {
		case resourceType == cmdbmodel.CmdbResourceTypeLogStore && (storeMatch || codeMatch):
			for _, parentID := range incoming[nodeID] {
				addOwner(parentID)
			}
		case resourceType == cmdbmodel.CmdbResourceTypeLogProject && projectMatch:
			for _, parentID := range incoming[nodeID] {
				addOwner(parentID)
			}
		case sourceType == cmdbmodel.CmdbResourceTypeLogStore && (storeMatch || codeMatch):
			addOwner(nodeID)
		case sourceType == cmdbmodel.CmdbResourceTypeLogProject && projectMatch:
			addOwner(nodeID)
		case storeMatch || codeMatch || projectMatch:
			addOwner(nodeID)
		}
	}
	return sortedUintSet(owners)
}

func containsUint(values []uint, target uint) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func observabilityRelatedNodeIDs(nodeID uint, resourceType string, incoming map[uint][]uint) []uint {
	result := []uint{nodeID}
	if resourceType == cmdbmodel.CmdbResourceTypeLogStore || resourceType == cmdbmodel.CmdbResourceTypeLogProject {
		result = append(result, incoming[nodeID]...)
	}
	set := make(map[uint]struct{}, len(result))
	for _, id := range result {
		if id > 0 {
			set[id] = struct{}{}
		}
	}
	return sortedUintSet(set)
}

func observabilityProjectIDsFromNode(resourceType, sourceType string, props map[string]interface{}) []uint {
	set := make(map[uint]struct{})
	for _, id := range uintValuesFromProperties(props, "logProjectId", "log_project_id", "logProjectIds") {
		set[id] = struct{}{}
	}
	if resourceType == cmdbmodel.CmdbResourceTypeLogProject || sourceType == cmdbmodel.CmdbResourceTypeLogProject {
		for _, id := range uintValuesFromProperties(props, "projectId", "projectIds") {
			set[id] = struct{}{}
		}
	}
	return sortedUintSet(set)
}

func (s *CmdbService) queryBusinessStoreObservations(c *gin.Context, querier BusinessObservabilityLogQuerier, bindings []businessStoreBinding, start, end time.Time, step string) []businessStoreObservation {
	results := make([]businessStoreObservation, len(bindings))
	var wg sync.WaitGroup
	for index := range bindings {
		results[index].Binding = bindings[index]
		if querier == nil {
			results[index].Err = errors.New("日志查询服务未初始化")
			continue
		}
		if bindings[index].Store.Status != logtailmodel.LogStatusEnabled {
			results[index].Err = errors.New("日志库已停用")
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i].Snapshot, results[i].Err = querier.QuerySLAStore(c.Copy(), logtailmodel.LogSLAStoreQuery{
				StoreID:              bindings[i].Store.ID,
				Start:                start.Format(time.RFC3339Nano),
				End:                  end.Format(time.RFC3339Nano),
				Step:                 step,
				SlowRequestThreshold: defaultSlowRequestThreshold,
			})
		}(index)
	}
	wg.Wait()
	return results
}

func aggregateBusinessSLAMetrics(observations []businessStoreObservation, topology map[string]interface{}) cmdbmodel.BusinessSLAMetricsVo {
	result := cmdbmodel.BusinessSLAMetricsVo{
		SLOTarget:                   businessSLOTarget(topology),
		SLOStatus:                   "no_data",
		SlowRequestThresholdSeconds: defaultSlowRequestThreshold,
		AggregationMode:             "sum_weighted_avg_conservative_p95",
	}
	var weightedLatency float64
	for _, observation := range observations {
		if observation.Err != nil || observation.Snapshot == nil {
			continue
		}
		metrics := observation.Snapshot.Metrics
		result.RequestTotal += metrics.RequestTotal
		result.HTTP4xx += metrics.HTTP4xx
		result.HTTP5xx += metrics.HTTP5xx
		result.SlowRequestCount += metrics.SlowRequestCount
		result.ResponseBytes += metrics.ResponseBytes
		weightedLatency += metrics.AvgLatencyMs * float64(metrics.RequestTotal)
		result.P95LatencyMs = math.Max(result.P95LatencyMs, metrics.P95LatencyMs)
	}
	if result.RequestTotal <= 0 {
		return result
	}
	result.HasTraffic = true
	total := float64(result.RequestTotal)
	result.AvgLatencyMs = roundObservability(weightedLatency / total)
	result.P95LatencyMs = roundObservability(result.P95LatencyMs)
	result.HTTP4xxRate = roundObservability(float64(result.HTTP4xx) * 100 / total)
	result.HTTP5xxRate = roundObservability(float64(result.HTTP5xx) * 100 / total)
	result.ErrorRate = roundObservability(float64(result.HTTP4xx+result.HTTP5xx) * 100 / total)
	result.RequestErrorRate = result.ErrorRate
	result.SystemErrorRate = result.HTTP5xxRate
	result.SlowRequestRate = roundObservability(float64(result.SlowRequestCount) * 100 / total)
	result.SuccessRate = roundObservability(math.Max(0, 100-result.ErrorRate))
	result.AvailabilityRate = roundObservability(math.Max(0, 100-result.HTTP5xxRate))
	result.SLOMet = result.AvailabilityRate >= result.SLOTarget
	if result.SLOMet {
		result.SLOStatus = "met"
	} else {
		result.SLOStatus = "breached"
	}
	allowedErrors := total * (100 - result.SLOTarget) / 100
	if allowedErrors > 0 {
		result.ErrorBudgetConsumedPercent = roundObservability(float64(result.HTTP5xx) * 100 / allowedErrors)
	}
	return result
}

func businessSLOTarget(topology map[string]interface{}) float64 {
	rootID := uintFromAny(topology["rootId"])
	nodes, _ := topology["nodes"].([]map[string]interface{})
	for _, node := range nodes {
		if uintFromAny(node["id"]) != rootID {
			continue
		}
		props, _ := node["properties"].(map[string]interface{})
		for _, key := range []string{"sloTarget", "slaTarget", "availabilityTarget"} {
			if target := observabilityFloat(props[key]); target > 0 && target < 100 {
				return roundObservability(target)
			}
		}
	}
	return defaultBusinessSLOTarget
}

func buildBusinessLogSources(observations []businessStoreObservation) ([]cmdbmodel.BusinessLogSourceVo, []string, int, int) {
	sources := make([]cmdbmodel.BusinessLogSourceVo, 0, len(observations))
	warnings := make([]string, 0)
	successful := 0
	partial := 0
	for _, observation := range observations {
		source := cmdbmodel.BusinessLogSourceVo{
			StoreID:       observation.Binding.Store.ID,
			StoreCode:     observation.Binding.Store.Code,
			StoreName:     observation.Binding.Store.Name,
			ProjectID:     observation.Binding.Store.ProjectID,
			Status:        observation.Binding.Store.Status,
			QueryStatus:   "ready",
			BoundNodeIDs:  observation.Binding.NodeIDs,
			DirectNodeIDs: observation.Binding.DirectNodeIDs,
		}
		if observation.Err != nil {
			source.QueryStatus = "unavailable"
			source.WarningMessage = observation.Err.Error()
			warnings = append(warnings, fmt.Sprintf("日志库 %s 查询失败: %v", observation.Binding.Store.Name, observation.Err))
		} else if observation.Snapshot != nil {
			successful++
			source.QueryDuration = observation.Snapshot.QueryDuration
			if observation.Snapshot.Partial {
				partial++
				source.QueryStatus = "partial"
				for _, warning := range observation.Snapshot.Warnings {
					warnings = append(warnings, fmt.Sprintf("日志库 %s: %s", observation.Binding.Store.Name, warning))
				}
			}
		}
		sources = append(sources, source)
	}
	return sources, warnings, successful, partial
}

func aggregateBusinessSLATrend(observations []businessStoreObservation) []logtailmodel.LogSLATrendPoint {
	points := make(map[string]*logtailmodel.LogSLATrendPoint)
	for _, observation := range observations {
		if observation.Err != nil || observation.Snapshot == nil {
			continue
		}
		for _, item := range observation.Snapshot.Trend {
			point := points[item.Timestamp]
			if point == nil {
				point = &logtailmodel.LogSLATrendPoint{Timestamp: item.Timestamp}
				points[item.Timestamp] = point
			}
			point.RequestTotal += item.RequestTotal
			point.HTTP200 += item.HTTP200
			point.HTTP4xx += item.HTTP4xx
			point.HTTP5xx += item.HTTP5xx
		}
	}
	result := make([]logtailmodel.LogSLATrendPoint, 0, len(points))
	for _, point := range points {
		result = append(result, *point)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Timestamp < result[j].Timestamp })
	return result
}

func aggregateBusinessSLATopAPIs(observations []businessStoreObservation) []cmdbmodel.BusinessSLATopAPIVo {
	items := make(map[string]*cmdbmodel.BusinessSLATopAPIVo)
	weightedLatency := make(map[string]float64)
	storeSets := make(map[string]map[uint]struct{})
	for _, observation := range observations {
		if observation.Err != nil || observation.Snapshot == nil {
			continue
		}
		for _, api := range observation.Snapshot.TopAPIs {
			key := api.RequestMethod + "\x00" + api.RequestPath
			item := items[key]
			if item == nil {
				item = &cmdbmodel.BusinessSLATopAPIVo{RequestMethod: api.RequestMethod, RequestPath: api.RequestPath}
				items[key] = item
				storeSets[key] = make(map[uint]struct{})
			}
			item.RequestTotal += api.RequestTotal
			item.HTTP4xx += api.HTTP4xx
			item.HTTP5xx += api.HTTP5xx
			item.ResponseBytes += api.ResponseBytes
			item.P95LatencyMs = math.Max(item.P95LatencyMs, api.P95LatencyMs)
			weightedLatency[key] += api.AvgLatencyMs * float64(api.RequestTotal)
			storeSets[key][observation.Binding.Store.ID] = struct{}{}
		}
	}
	result := make([]cmdbmodel.BusinessSLATopAPIVo, 0, len(items))
	for key, item := range items {
		item.ErrorCount = item.HTTP4xx + item.HTTP5xx
		if item.RequestTotal > 0 {
			item.ErrorRate = roundObservability(float64(item.ErrorCount) * 100 / float64(item.RequestTotal))
			item.AvgLatencyMs = roundObservability(weightedLatency[key] / float64(item.RequestTotal))
		}
		item.P95LatencyMs = roundObservability(item.P95LatencyMs)
		item.StoreIDs = sortedUintSet(storeSets[key])
		result = append(result, *item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].ErrorCount != result[j].ErrorCount {
			return result[i].ErrorCount > result[j].ErrorCount
		}
		if result[i].P95LatencyMs != result[j].P95LatencyMs {
			return result[i].P95LatencyMs > result[j].P95LatencyMs
		}
		return result[i].RequestTotal > result[j].RequestTotal
	})
	if len(result) > 20 {
		result = result[:20]
	}
	return result
}

func aggregateBusinessSLARecentErrors(observations []businessStoreObservation) []cmdbmodel.BusinessSLARecentErrorVo {
	result := make([]cmdbmodel.BusinessSLARecentErrorVo, 0)
	for _, observation := range observations {
		if observation.Err != nil || observation.Snapshot == nil {
			continue
		}
		for _, item := range observation.Snapshot.RecentErrors {
			result = append(result, cmdbmodel.BusinessSLARecentErrorVo{
				Timestamp:     item.Timestamp,
				RequestMethod: item.RequestMethod,
				RequestPath:   item.RequestPath,
				Domain:        item.Domain,
				Status:        item.Status,
				LatencyMs:     item.LatencyMs,
				UpstreamAddr:  item.UpstreamAddr,
				ClientIP:      item.ClientIP,
				RequestID:     item.RequestID,
				TraceID:       item.TraceID,
				StoreID:       observation.Binding.Store.ID,
				StoreCode:     observation.Binding.Store.Code,
			})
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Timestamp > result[j].Timestamp })
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}

func buildBusinessNodeSignals(observations []businessStoreObservation) []cmdbmodel.BusinessNodeSignalVo {
	type nodeAggregation struct {
		metrics         cmdbmodel.BusinessNodeSignalVo
		weightedLatency float64
		storeIDs        map[uint]struct{}
	}
	nodes := make(map[uint]*nodeAggregation)
	for _, observation := range observations {
		if observation.Err != nil || observation.Snapshot == nil {
			continue
		}
		metrics := observation.Snapshot.Metrics
		for _, nodeID := range observation.Binding.NodeIDs {
			node := nodes[nodeID]
			if node == nil {
				node = &nodeAggregation{metrics: cmdbmodel.BusinessNodeSignalVo{ResourceID: nodeID}, storeIDs: make(map[uint]struct{})}
				nodes[nodeID] = node
			}
			node.metrics.RequestTotal += metrics.RequestTotal
			node.metrics.HTTP4xx += metrics.HTTP4xx
			node.metrics.HTTP5xx += metrics.HTTP5xx
			node.metrics.P95LatencyMs = math.Max(node.metrics.P95LatencyMs, metrics.P95LatencyMs)
			node.weightedLatency += metrics.AvgLatencyMs * float64(metrics.RequestTotal)
			node.storeIDs[observation.Binding.Store.ID] = struct{}{}
		}
	}
	result := make([]cmdbmodel.BusinessNodeSignalVo, 0, len(nodes))
	for _, node := range nodes {
		if node.metrics.RequestTotal > 0 {
			total := float64(node.metrics.RequestTotal)
			node.metrics.ErrorRate = roundObservability(float64(node.metrics.HTTP4xx+node.metrics.HTTP5xx) * 100 / total)
			node.metrics.HTTP5xxRate = roundObservability(float64(node.metrics.HTTP5xx) * 100 / total)
			node.metrics.AvgLatencyMs = roundObservability(node.weightedLatency / total)
			node.metrics.P95LatencyMs = roundObservability(node.metrics.P95LatencyMs)
		}
		node.metrics.BoundLogStoreIDs = sortedUintSet(node.storeIDs)
		node.metrics.HealthStatus, node.metrics.Message = businessNodeSignalHealth(node.metrics)
		node.metrics.HealthColor = healthColor(node.metrics.HealthStatus)
		result = append(result, node.metrics)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ResourceID < result[j].ResourceID })
	return result
}

func businessNodeSignalHealth(signal cmdbmodel.BusinessNodeSignalVo) (string, string) {
	if signal.RequestTotal <= 0 {
		return cmdbmodel.CmdbHealthStatusUnknown, "当前时间窗口无 HTTP 请求数据"
	}
	if signal.HTTP5xxRate >= observabilityCriticalHTTP5xxRate || signal.P95LatencyMs >= observabilityCriticalP95Ms {
		return cmdbmodel.CmdbHealthStatusCritical, fmt.Sprintf("5xx %.2f%%，P95 %.0fms", signal.HTTP5xxRate, signal.P95LatencyMs)
	}
	http4xxRate := float64(signal.HTTP4xx) * 100 / float64(signal.RequestTotal)
	if signal.HTTP5xxRate >= observabilityWarningHTTP5xxRate || http4xxRate >= observabilityWarningHTTP4xxRate || signal.P95LatencyMs >= observabilityWarningP95Ms {
		return cmdbmodel.CmdbHealthStatusWarning, fmt.Sprintf("4xx %.2f%%，5xx %.2f%%，P95 %.0fms", http4xxRate, signal.HTTP5xxRate, signal.P95LatencyMs)
	}
	return cmdbmodel.CmdbHealthStatusHealthy, fmt.Sprintf("请求 %d，5xx %.2f%%，P95 %.0fms", signal.RequestTotal, signal.HTTP5xxRate, signal.P95LatencyMs)
}

func enrichBusinessTopologyWithSignals(topology map[string]interface{}, signals []cmdbmodel.BusinessNodeSignalVo) {
	nodes, _ := topology["nodes"].([]map[string]interface{})
	byNodeID := make(map[uint]cmdbmodel.BusinessNodeSignalVo, len(signals))
	for _, signal := range signals {
		byNodeID[signal.ResourceID] = signal
	}
	for _, node := range nodes {
		signal, exists := byNodeID[uintFromAny(node["id"])]
		if !exists {
			continue
		}
		node["observability"] = signal
		currentStatus := stringFromAny(node["healthStatus"])
		displayStatus := currentStatus
		if healthSeverity(signal.HealthStatus) > healthSeverity(currentStatus) {
			displayStatus = signal.HealthStatus
		}
		node["displayHealthStatus"] = displayStatus
		node["displayHealthColor"] = healthColor(displayStatus)
	}
}

func (s *CmdbService) listBusinessFiringLogAlerts(bindings []businessStoreBinding) ([]map[string]interface{}, int64, error) {
	storeIDs := make([]uint, 0, len(bindings))
	for _, binding := range bindings {
		storeIDs = append(storeIDs, binding.Store.ID)
	}
	if len(storeIDs) == 0 {
		return []map[string]interface{}{}, 0, nil
	}
	baseQuery := s.dao.DB().Model(&logtailmodel.LogAlertEvent{}).Where("store_id IN ? AND LOWER(status) = ?", storeIDs, "firing")
	var count int64
	if err := baseQuery.Count(&count).Error; err != nil {
		return nil, 0, err
	}
	var events []logtailmodel.LogAlertEvent
	if err := baseQuery.Order("fired_at DESC, id DESC").Limit(50).Find(&events).Error; err != nil {
		return nil, 0, err
	}
	result := make([]map[string]interface{}, 0, len(events))
	for _, event := range events {
		item := map[string]interface{}{
			"id":            event.ID,
			"ruleId":        event.RuleID,
			"projectId":     event.ProjectID,
			"storeId":       event.StoreID,
			"sourceName":    event.SourceName,
			"severity":      event.Severity,
			"status":        event.Status,
			"summary":       event.Summary,
			"triggerReason": event.TriggerReason,
			"matchCount":    event.MatchCount,
		}
		if event.FiredAt != nil {
			item["firedAt"] = event.FiredAt.Format(time.RFC3339Nano)
		}
		result = append(result, item)
	}
	return result, count, nil
}

func uintValuesFromProperties(props map[string]interface{}, keys ...string) []uint {
	set := make(map[uint]struct{})
	for _, key := range keys {
		value, exists := props[key]
		if !exists {
			continue
		}
		switch typed := value.(type) {
		case []interface{}:
			for _, item := range typed {
				if id := uintFromAny(item); id > 0 {
					set[id] = struct{}{}
				}
			}
		case []uint:
			for _, id := range typed {
				if id > 0 {
					set[id] = struct{}{}
				}
			}
		default:
			if id := uintFromAny(value); id > 0 {
				set[id] = struct{}{}
			}
		}
	}
	return sortedUintSet(set)
}

func sortedUintMapKeys(values map[uint]map[uint]struct{}) []uint {
	keys := make([]uint, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func sortedUintSet(values map[uint]struct{}) []uint {
	keys := make([]uint, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func observabilityFloat(value interface{}) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	default:
		return 0
	}
}

func roundObservability(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*10000) / 10000
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
