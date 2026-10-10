package service

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	alertengine "dodevops-api/api/alertmanager/engine"
	"dodevops-api/api/alertmanager/model"
	alertnotify "dodevops-api/api/alertmanager/notify"
	"dodevops-api/common/result"
	"dodevops-api/common/util"

	"github.com/gin-gonic/gin"
)

func (s *AlertManagerServiceImpl) GetDashboardCharts(c *gin.Context, query model.AlertDashboardQuery) {
	normalized, start, end, err := normalizeDashboardRange(query)
	if err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	query.Range = normalized
	limit := normalizeDashboardLimit(query.Limit)

	events, err := s.dao.ListDashboardEvents(query, start, end)
	if err != nil {
		result.Failed(c, 500, "查询告警仪表盘事件失败: "+err.Error())
		return
	}
	routes, err := s.dao.ListEnabledNotifyRouteRules()
	if err != nil {
		result.Failed(c, 500, "查询告警业务路由失败: "+err.Error())
		return
	}
	notifyTrend, err := s.dashboardNotifyTrend(query, start, end)
	if err != nil {
		result.Failed(c, 500, "查询告警通知趋势失败: "+err.Error())
		return
	}

	businessRouteTop := buildBusinessRouteTop(events, routes, limit)
	ruleGroupTop := buildRuleGroupTop(events, limit)
	result.Success(c, model.AlertDashboardChartsVO{
		Range:            normalized,
		StartTime:        util.HTime{Time: start},
		EndTime:          util.HTime{Time: end},
		AlertNameTop:     buildAlertNameTop(events, limit),
		NotifyTrend:      notifyTrend,
		RuleGroupTop:     ruleGroupTop,
		BusinessRouteTop: businessRouteTop,
		HostGroupTop:     ruleGroupTop,
	})
}

func (s *AlertManagerServiceImpl) GetDashboardScreen(c *gin.Context, query model.AlertDashboardQuery) {
	events, source, err := s.dashboardActiveEvents(query)
	if err != nil {
		result.Failed(c, 500, "查询告警大屏数据失败: "+err.Error())
		return
	}
	result.Success(c, model.AlertDashboardScreenVO{
		SeverityDistribution: buildSeverityDistribution(events),
		LatestActiveTop5:     s.latestActiveEventTop(events, 5),
		AlertNameTop10:       buildActiveAlertNameTop(events, 10),
		TotalActive:          int64(len(events)),
		Source:               source,
	})
}

func (s *AlertManagerServiceImpl) GetDashboardAlertNameTop(c *gin.Context, query model.AlertDashboardQuery) {
	_, start, end, err := normalizeDashboardRange(query)
	if err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	events, err := s.dao.ListDashboardEvents(query, start, end)
	if err != nil {
		result.Failed(c, 500, "查询告警名称 Top 失败: "+err.Error())
		return
	}
	result.Success(c, buildAlertNameTop(events, normalizeDashboardLimit(query.Limit)))
}

func (s *AlertManagerServiceImpl) GetDashboardNotifyTrend(c *gin.Context, query model.AlertDashboardQuery) {
	_, start, end, err := normalizeDashboardRange(query)
	if err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	trend, err := s.dashboardNotifyTrend(query, start, end)
	if err != nil {
		result.Failed(c, 500, "查询告警通知趋势失败: "+err.Error())
		return
	}
	result.Success(c, trend)
}

func (s *AlertManagerServiceImpl) GetDashboardBusinessRouteTop(c *gin.Context, query model.AlertDashboardQuery) {
	_, start, end, err := normalizeDashboardRange(query)
	if err != nil {
		result.Failed(c, 400, err.Error())
		return
	}
	events, err := s.dao.ListDashboardEvents(query, start, end)
	if err != nil {
		result.Failed(c, 500, "查询告警业务路由 Top 失败: "+err.Error())
		return
	}
	routes, err := s.dao.ListEnabledNotifyRouteRules()
	if err != nil {
		result.Failed(c, 500, "查询告警业务路由失败: "+err.Error())
		return
	}
	result.Success(c, buildBusinessRouteTop(events, routes, normalizeDashboardLimit(query.Limit)))
}

func (s *AlertManagerServiceImpl) dashboardNotifyTrend(query model.AlertDashboardQuery, start, end time.Time) ([]model.AlertDashboardNotifyTrendPoint, error) {
	rows, hasNotifyRecords, err := s.dao.DashboardNotifyTrendFromRecords(query, start, end)
	if err != nil {
		return nil, err
	}
	if hasNotifyRecords {
		return fillDashboardTrendBuckets(rows, start, end, "notify_records"), nil
	}
	return fillDashboardTrendBuckets(nil, start, end, "no_notify_records"), nil
}

func (s *AlertManagerServiceImpl) dashboardActiveEvents(query model.AlertDashboardQuery) ([]model.AlertActiveEvent, string, error) {
	if list, ok := alertengine.ListActiveEventsFromState(); ok {
		filtered := make([]model.AlertActiveEvent, 0, len(list))
		for _, event := range list {
			if dashboardActiveEventMatchesQuery(event, query) {
				filtered = append(filtered, event)
			}
		}
		return filtered, "redis", nil
	}
	list, err := s.dao.ListDashboardActiveEvents(query)
	if err != nil {
		return nil, "", err
	}
	return list, "mysql", nil
}

func dashboardActiveEventMatchesQuery(event model.AlertActiveEvent, query model.AlertDashboardQuery) bool {
	if event.Status == model.EventStatusResolved {
		return false
	}
	if query.DatasourceID > 0 && event.DatasourceID != query.DatasourceID {
		return false
	}
	if query.DatasourceType != "" && event.DatasourceType != query.DatasourceType {
		return false
	}
	if query.Severity != "" && event.Severity != query.Severity {
		return false
	}
	if query.BusinessID != "" && event.BusinessID != query.BusinessID {
		return false
	}
	return true
}

func buildSeverityDistribution(events []model.AlertActiveEvent) []model.AlertDashboardSeverityDistributionItem {
	levels := []string{"P0", "P1", "P2"}
	names := map[string]string{"P0": "P0", "P1": "P1", "P2": "P2"}
	counts := map[string]int64{"P0": 0, "P1": 0, "P2": 0}
	var total int64
	for _, event := range events {
		level := activeEventLevel(event)
		if _, ok := counts[level]; !ok {
			level = "P2"
		}
		counts[level]++
		total++
	}
	result := make([]model.AlertDashboardSeverityDistributionItem, 0, len(levels))
	for _, level := range levels {
		result = append(result, model.AlertDashboardSeverityDistributionItem{
			Level:   level,
			Name:    names[level],
			Count:   counts[level],
			Percent: dashboardPercent(counts[level], total),
		})
	}
	return result
}

func (s *AlertManagerServiceImpl) latestActiveEventTop(events []model.AlertActiveEvent, limit int) []model.AlertEventVO {
	list := append([]model.AlertActiveEvent(nil), events...)
	sort.SliceStable(list, func(i, j int) bool {
		left := activeEventLatestTime(list[i])
		right := activeEventLatestTime(list[j])
		if !left.Equal(right) {
			return left.After(right)
		}
		return list[i].ID > list[j].ID
	})
	list = limitDashboardSlice(list, limit)
	return s.activeEventVOs(list)
}

func activeEventLatestTime(event model.AlertActiveEvent) time.Time {
	for _, value := range []time.Time{event.FirstTriggeredAt.Time, event.CreatedAt.Time, event.LastEvaluatedAt.Time, event.UpdatedAt.Time} {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func buildActiveAlertNameTop(events []model.AlertActiveEvent, limit int) []model.AlertDashboardAlertNameTopItem {
	rows := make([]model.AlertDashboardEventRow, 0, len(events))
	for _, event := range events {
		rows = append(rows, model.AlertDashboardEventRow{
			RuleID:   event.RuleID,
			RuleName: event.RuleName,
		})
	}
	return buildAlertNameTop(rows, limit)
}

func activeEventLevel(event model.AlertActiveEvent) string {
	if level := strings.ToUpper(strings.TrimSpace(eventLabelString(event.Labels, "level"))); level != "" {
		switch level {
		case "P0", "P1", "P2":
			return level
		}
	}
	switch strings.ToLower(strings.TrimSpace(event.Severity)) {
	case "p0", "critical", "serious", "fatal", "error":
		return "P0"
	case "p1", "warning", "warn":
		return "P1"
	case "p2", "info", "notice", "normal", "提示":
		return "P2"
	default:
		return "P2"
	}
}

func buildAlertNameTop(events []model.AlertDashboardEventRow, limit int) []model.AlertDashboardAlertNameTopItem {
	type bucket struct {
		ruleID uint
		name   string
		count  int64
	}
	buckets := map[string]*bucket{}
	var total int64
	for _, event := range events {
		name := strings.TrimSpace(event.RuleName)
		if name == "" {
			name = "未命名告警"
		}
		key := fmt.Sprintf("%d|%s", event.RuleID, name)
		if buckets[key] == nil {
			buckets[key] = &bucket{ruleID: event.RuleID, name: name}
		}
		buckets[key].count++
		total++
	}
	list := make([]model.AlertDashboardAlertNameTopItem, 0, len(buckets))
	for _, item := range buckets {
		list = append(list, model.AlertDashboardAlertNameTopItem{
			RuleID:  item.ruleID,
			Name:    item.name,
			Count:   item.count,
			Percent: dashboardPercent(item.count, total),
		})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return list[i].Name < list[j].Name
	})
	return limitDashboardSlice(list, limit)
}

func buildRuleGroupTop(events []model.AlertDashboardEventRow, limit int) []model.AlertDashboardHostGroupTopItem {
	type bucket struct {
		groupID   string
		groupName string
		count     int64
	}
	buckets := map[string]*bucket{}
	var total int64
	for _, event := range events {
		groupID := fmt.Sprint(event.RuleGroupID)
		groupName := strings.TrimSpace(event.RuleGroupName)
		if event.RuleGroupID == 0 || groupName == "" {
			groupID = "0"
			groupName = "未分组"
		}
		key := groupID + "|" + groupName
		if buckets[key] == nil {
			buckets[key] = &bucket{groupID: groupID, groupName: groupName}
		}
		buckets[key].count++
		total++
	}
	list := make([]model.AlertDashboardHostGroupTopItem, 0, len(buckets))
	for _, item := range buckets {
		list = append(list, model.AlertDashboardHostGroupTopItem{
			GroupID:   item.groupID,
			GroupName: item.groupName,
			Count:     item.count,
			Percent:   dashboardPercent(item.count, total),
		})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return list[i].GroupName < list[j].GroupName
	})
	return limitDashboardSlice(list, limit)
}
func buildBusinessRouteTop(events []model.AlertDashboardEventRow, routes []model.AlertNotifyRouteRule, limit int) []model.AlertDashboardHostGroupTopItem {
	type bucket struct {
		groupID   string
		groupName string
		count     int64
	}
	buckets := map[string]*bucket{}
	var total int64
	for _, event := range events {
		groupID := "operations"
		groupName := "运维组"
		if route, ok := matchDashboardBusinessRoute(routes, event); ok {
			groupID = fmt.Sprint(route.ID)
			groupName = dashboardBusinessGroupName(route.Name)
			if groupName == "" {
				groupName = groupID
			}
		}
		key := groupID + "|" + groupName
		if buckets[key] == nil {
			buckets[key] = &bucket{groupID: groupID, groupName: groupName}
		}
		buckets[key].count++
		total++
	}
	list := make([]model.AlertDashboardHostGroupTopItem, 0, len(buckets))
	for _, item := range buckets {
		list = append(list, model.AlertDashboardHostGroupTopItem{
			GroupID:   item.groupID,
			GroupName: item.groupName,
			Count:     item.count,
			Percent:   dashboardPercent(item.count, total),
		})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return list[i].GroupName < list[j].GroupName
	})
	return limitDashboardSlice(list, limit)
}

func dashboardBusinessGroupName(routeName string) string {
	name := strings.TrimSpace(routeName)
	if strings.HasSuffix(name, "路由") {
		return strings.TrimSuffix(name, "路由") + "组"
	}
	return name
}

func matchDashboardBusinessRoute(routes []model.AlertNotifyRouteRule, event model.AlertDashboardEventRow) (model.AlertNotifyRouteRule, bool) {
	return matchBusinessRoute(routes, event.NotifyRuleIDs, event.Labels)
}

func matchBusinessRoute(routes []model.AlertNotifyRouteRule, notifyRuleIDs model.JSONUintArray, labels model.JSONMap) (model.AlertNotifyRouteRule, bool) {
	var matched model.AlertNotifyRouteRule
	found := false
	seenPolicyIDs := make(map[uint]struct{}, len(notifyRuleIDs))
	// One alert must occupy one business bucket even when it is attached to
	// multiple notification policies. Reuse the notification matcher for each
	// policy, then keep the globally highest-priority applicable route.
	for _, policyID := range notifyRuleIDs {
		if policyID == 0 {
			continue
		}
		if _, ok := seenPolicyIDs[policyID]; ok {
			continue
		}
		seenPolicyIDs[policyID] = struct{}{}
		route, ok := alertnotify.MatchNotifyRouteRulesForPolicy(routes, policyID, labels)
		if !ok || found && !dashboardRoutePrecedes(route, matched) {
			continue
		}
		matched = route
		found = true
	}
	return matched, found
}

func dashboardRoutePrecedes(left, right model.AlertNotifyRouteRule) bool {
	if left.Priority != right.Priority {
		return left.Priority > right.Priority
	}
	return left.ID < right.ID
}

func fillDashboardTrendBuckets(rows []model.AlertDashboardNotifyTrendPoint, start, end time.Time, source string) []model.AlertDashboardNotifyTrendPoint {
	rowMap := map[string]model.AlertDashboardNotifyTrendPoint{}
	for _, row := range rows {
		if strings.TrimSpace(row.Date) == "" {
			continue
		}
		existing := rowMap[row.Date]
		existing.Date = row.Date
		existing.AlertCount += row.AlertCount
		existing.NotificationCount += row.NotificationCount
		existing.SuccessCount += row.SuccessCount
		existing.FailedCount += row.FailedCount
		existing.Source = firstNonEmpty(row.Source, source)
		rowMap[row.Date] = existing
	}
	hourly := dashboardTrendUsesHourlyBuckets(start, end)
	format := "2006-01-02"
	step := func(value time.Time) time.Time { return value.AddDate(0, 0, 1) }
	cursor := dayStart(start)
	if hourly {
		format = "2006-01-02 15:00"
		step = func(value time.Time) time.Time { return value.Add(time.Hour) }
		cursor = hourStart(start)
	}
	fillEnd := end
	if now := time.Now(); fillEnd.After(now) {
		fillEnd = now
	}
	result := make([]model.AlertDashboardNotifyTrendPoint, 0)
	for bucket := cursor; bucket.Before(fillEnd); bucket = step(bucket) {
		key := bucket.Format(format)
		item := rowMap[key]
		if item.Date == "" {
			item.Date = key
			item.Source = source
		}
		result = append(result, item)
	}
	return result
}

func dashboardTrendUsesHourlyBuckets(start, end time.Time) bool {
	return end.Sub(start) <= 48*time.Hour
}

func normalizeDashboardRange(query model.AlertDashboardQuery) (string, time.Time, time.Time, error) {
	now := time.Now()
	rangeValue := strings.ToLower(strings.TrimSpace(query.Range))
	if rangeValue == "" {
		rangeValue = "30d"
	}
	if query.StartTime != "" || query.EndTime != "" {
		start, err := parseDashboardTime(query.StartTime)
		if err != nil {
			return "", time.Time{}, time.Time{}, fmt.Errorf("startTime 格式错误")
		}
		end, err := parseDashboardTime(query.EndTime)
		if err != nil {
			return "", time.Time{}, time.Time{}, fmt.Errorf("endTime 格式错误")
		}
		if start.IsZero() || end.IsZero() || !end.After(start) {
			return "", time.Time{}, time.Time{}, fmt.Errorf("startTime/endTime 无效")
		}
		return "custom", start, end, nil
	}

	today := dayStart(now)
	switch rangeValue {
	case "today", "1d":
		return "today", today, today.AddDate(0, 0, 1), nil
	case "yesterday":
		return "yesterday", today.AddDate(0, 0, -1), today, nil
	case "7d", "7days":
		return "7d", today.AddDate(0, 0, -6), today.AddDate(0, 0, 1), nil
	case "30d", "30days":
		return "30d", today.AddDate(0, 0, -29), today.AddDate(0, 0, 1), nil
	default:
		return "", time.Time{}, time.Time{}, fmt.Errorf("range must be today, yesterday, 7d or 30d")
	}
}

func parseDashboardTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	formats := []string{
		"2006/01/02 15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		time.RFC3339,
	}
	for _, format := range formats {
		parsed, err := time.ParseInLocation(format, value, time.Local)
		if err == nil {
			if format == "2006-01-02" {
				return dayStart(parsed), nil
			}
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time")
}

func dayStart(value time.Time) time.Time {
	year, month, day := value.In(time.Local).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.Local)
}

func hourStart(value time.Time) time.Time {
	value = value.In(time.Local)
	return time.Date(value.Year(), value.Month(), value.Day(), value.Hour(), 0, 0, 0, time.Local)
}

func normalizeDashboardLimit(limit int) int {
	if limit <= 0 {
		return 5
	}
	if limit > 20 {
		return 20
	}
	return limit
}

func dashboardPercent(count, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(float64(count)*10000/float64(total)) / 100
}

func limitDashboardSlice[T any](list []T, limit int) []T {
	if limit <= 0 || len(list) <= limit {
		return list
	}
	return list[:limit]
}
