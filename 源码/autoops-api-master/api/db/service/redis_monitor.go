package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"
)

func (s *DatabaseService) RedisMonitorDashboard(ctx context.Context, assetID uint, query dbmodel.RedisMonitorQuery) (*dbmodel.RedisMonitorDashboard, error) {
	asset, err := s.assetDao.GetByID(assetID)
	if err != nil {
		return nil, err
	}
	if asset.Type != cmdbmodel.DatabaseTypeRedis {
		return nil, fmt.Errorf("database asset is not Redis")
	}
	if asset.PrometheusAccountID == 0 {
		return nil, fmt.Errorf("database asset has no prometheusAccountId")
	}

	account, err := s.accountDao.GetByID(asset.PrometheusAccountID)
	if err != nil {
		return nil, fmt.Errorf("get prometheus account failed: %w", err)
	}
	password, err := account.DecryptPassword()
	if err != nil {
		return nil, fmt.Errorf("decrypt prometheus account password failed: %w", err)
	}

	client := newPrometheusClient(account, password)
	start, end, step := normalizeMySQLMonitorRange(query)
	target, warnings := s.resolveRedisPrometheusTarget(ctx, client, asset)
	selector := target.Selector
	if selector == "" {
		selector = strings.TrimSpace(asset.PrometheusLabelSelector)
	}

	dashboard := &dbmodel.RedisMonitorDashboard{
		Asset:    buildAssetVO(*asset),
		Target:   target,
		Warnings: warnings,
		Range: dbmodel.MonitorQueryRange{
			Start: start.UnixMilli(),
			End:   end.UnixMilli(),
			Step:  step,
		},
	}
	dashboard.Asset.PrometheusAccountAlias = account.Alias
	dashboard.Asset.PrometheusEndpoint = client.baseURL

	if !target.Matched || strings.TrimSpace(selector) == "" {
		dashboard.Warnings = append(dashboard.Warnings, "redis monitor target is not matched, fallback to global redis metrics")
	}

	overview, overviewWarnings := s.queryRedisOverview(ctx, client, asset, selector)
	dashboard.Overview = overview
	dashboard.Warnings = append(dashboard.Warnings, overviewWarnings...)

	trends, trendWarnings := s.queryRedisTrends(ctx, client, asset, selector, start, end, step)
	dashboard.Trends = trends
	dashboard.Warnings = append(dashboard.Warnings, trendWarnings...)

	breakdown, breakdownWarnings := s.queryRedisBreakdown(ctx, client, selector)
	dashboard.Breakdown = breakdown
	dashboard.Warnings = append(dashboard.Warnings, breakdownWarnings...)
	return dashboard, nil
}

func (s *DatabaseService) resolveRedisPrometheusTarget(ctx context.Context, client *prometheusClient, asset *cmdbmodel.CmdbSQL) (dbmodel.MySQLMonitorTarget, []string) {
	target := dbmodel.MySQLMonitorTarget{Prometheus: client.baseURL}
	if selector := strings.TrimSpace(asset.PrometheusLabelSelector); selector != "" {
		target.Matched = true
		target.Selector = selector
		target.MatchBy = "prometheusLabelSelector"
		target.MetricsURL = defaultDatabaseMetricsURL(asset, "redis")
		target.ScrapeURL = target.MetricsURL
		enrichMonitorTargetDisplay(client, asset, &target, "redis")
		return target, nil
	}

	metricTarget, err := resolveRedisTargetFromMetric(ctx, client, asset)
	if err == nil {
		return metricTarget, nil
	}
	metricWarning := "redis_up metric match failed: " + err.Error()
	warnings := make([]string, 0)

	targets, err := client.targets(ctx)
	if err != nil {
		return target, append(warnings, metricWarning, "query prometheus targets failed: "+err.Error())
	}

	bestScore := 0
	var best prometheusTarget
	for _, item := range targets {
		score := scoreRedisTarget(asset, item)
		if score > bestScore {
			bestScore = score
			best = item
		}
	}
	if bestScore == 0 {
		return target, append(warnings, metricWarning, "no matching redis prometheus target found")
	}

	labels := mergeTargetLabels(best)
	target.Matched = true
	target.Health = best.Health
	target.ScrapeURL = firstNonEmpty(best.ScrapeURL, best.GlobalURL)
	target.MetricsURL = target.ScrapeURL
	target.LastError = best.LastError
	target.Labels = labels
	target.Selector = buildRedisPrometheusSelector(labels)
	target.MatchBy = redisTargetMatchBy(asset, best)
	enrichMonitorTargetDisplay(client, asset, &target, "redis")
	if target.Selector == "" {
		return target, append(warnings, "matched redis target has no stable labels for selector")
	}
	return target, warnings
}

func resolveRedisTargetFromMetric(ctx context.Context, client *prometheusClient, asset *cmdbmodel.CmdbSQL) (dbmodel.MySQLMonitorTarget, error) {
	target := dbmodel.MySQLMonitorTarget{Prometheus: client.baseURL}
	rows, err := client.queryInstantMetricRows(ctx, "redis_up")
	if err != nil {
		return target, err
	}

	bestScore := 0
	bestLabels := map[string]string{}
	for _, row := range rows {
		score := scoreRedisMetricLabels(asset, row.Metric)
		if score > bestScore {
			bestScore = score
			bestLabels = row.Metric
		}
	}
	if bestScore == 0 {
		return target, fmt.Errorf("no matching redis_up series found")
	}

	labels := clonePrometheusLabels(bestLabels)
	target.Matched = true
	target.Health = "up"
	target.Labels = labels
	target.Selector = buildRedisPrometheusSelector(labels)
	target.MatchBy = "redis_up"
	target.ScrapeURL = buildScrapeURLFromMetricLabels(labels)
	target.MetricsURL = target.ScrapeURL
	enrichMonitorTargetDisplay(client, asset, &target, "redis")
	if target.Selector == "" {
		return target, fmt.Errorf("matched redis_up series has no stable labels")
	}
	return target, nil
}

func scoreRedisTarget(asset *cmdbmodel.CmdbSQL, target prometheusTarget) int {
	labels := mergeTargetLabels(target)
	haystack := prometheusTargetHaystack(target, labels)
	if !isRedisTargetCandidate(labels, haystack) {
		return 0
	}

	score := 50
	addressMatched := false
	for _, key := range []string{"target", "__param_target", "addr", "instance", "__address__"} {
		if targetAddressMatchesAsset(asset, strings.TrimSpace(labels[key])) {
			addressMatched = true
			score += 100
		}
	}
	if targetAddressMatchesAsset(asset, target.ScrapeURL) || targetAddressMatchesAsset(asset, target.GlobalURL) {
		addressMatched = true
		score += 40
	}
	if !addressMatched {
		return 0
	}
	for _, needle := range redisTargetNeedles(asset) {
		if needle != "" && strings.Contains(haystack, strings.ToLower(needle)) {
			score += 10
		}
	}
	return score
}

func scoreRedisMetricLabels(asset *cmdbmodel.CmdbSQL, labels map[string]string) int {
	haystackParts := make([]string, 0, len(labels))
	for key, value := range labels {
		haystackParts = append(haystackParts, key+"="+value)
	}
	haystack := strings.ToLower(strings.Join(haystackParts, " "))
	if !isRedisTargetCandidate(labels, haystack) && !strings.Contains(haystack, "redis") {
		return 0
	}

	score := 0
	addressMatched := false
	for _, key := range []string{"target", "__param_target", "addr", "instance", "__address__", "alias"} {
		if targetAddressMatchesAsset(asset, strings.TrimSpace(labels[key])) {
			addressMatched = true
			score += 100
		}
	}
	if !addressMatched {
		return 0
	}
	for _, needle := range redisTargetNeedles(asset) {
		if needle != "" && strings.Contains(haystack, strings.ToLower(needle)) {
			score += 10
		}
	}
	return score
}

func redisTargetMatchBy(asset *cmdbmodel.CmdbSQL, target prometheusTarget) string {
	labels := mergeTargetLabels(target)
	for _, key := range []string{"target", "__param_target", "addr", "instance", "__address__", "alias"} {
		if targetAddressMatchesAsset(asset, strings.TrimSpace(labels[key])) {
			return key
		}
	}
	return "auto"
}

func isRedisTargetCandidate(labels map[string]string, haystack string) bool {
	if isNodeExporterTarget(labels, haystack) {
		return false
	}
	for _, key := range []string{"job", "role", "service", "auth_module", "__param_auth_module"} {
		if strings.Contains(strings.ToLower(strings.TrimSpace(labels[key])), "redis") {
			return true
		}
	}
	return strings.Contains(haystack, "redis")
}

func redisTargetNeedles(asset *cmdbmodel.CmdbSQL) []string {
	needles := []string{asset.HostIP, asset.Endpoint, asset.Name, asset.InstanceName}
	ipParts := strings.Split(asset.HostIP, ".")
	if len(ipParts) == 4 {
		needles = append(needles,
			ipParts[2]+"."+ipParts[3],
			ipParts[2]+"_"+ipParts[3],
			"redis"+ipParts[2]+"_"+ipParts[3],
			"db"+ipParts[2]+"_"+ipParts[3],
		)
	}
	return needles
}

func buildRedisPrometheusSelector(labels map[string]string) string {
	for _, keys := range [][]string{
		{"job", "instance", "target"},
		{"job", "instance", "addr"},
		{"job", "instance"},
		{"target"},
		{"addr"},
		{"instance"},
		{"alias"},
		{"job"},
	} {
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			if value := strings.TrimSpace(labels[key]); value != "" {
				parts = append(parts, fmt.Sprintf(`%s="%s"`, key, escapePrometheusLabelValue(value)))
			}
		}
		if len(parts) == len(keys) {
			return strings.Join(parts, ",")
		}
	}
	return ""
}

func (s *DatabaseService) queryRedisOverview(ctx context.Context, client *prometheusClient, asset *cmdbmodel.CmdbSQL, selector string) ([]dbmodel.MonitorMetricCard, []string) {
	nodeSelector := nodeInstanceSelector(asset.HostIP)
	defs := []mysqlMonitorMetricDef{
		{Key: "up", Name: "Redis 状态", Unit: "", PromQL: redisMaxQuery("redis_up", selector), FallbackPromQL: redisMaxQuery("redis_up", ""), Status: statusUp, Description: "1 表示 exporter 可连接 Redis"},
		{Key: "cpuUsage", Name: "主机 CPU 使用率", Unit: "%", PromQL: nodeCPUUsageQuery(nodeSelector), Status: statusPercent, Description: "资产 HostIP 对应 node_exporter 采集的主机 CPU 使用率", Required: true},
		{Key: "memoryUsage", Name: "主机内存使用率", Unit: "%", PromQL: nodeMemoryUsageQuery(nodeSelector), Status: statusPercent, Description: "node_exporter 采集的主机内存使用率", Required: true},
		{Key: "diskUsage", Name: "主机磁盘使用率", Unit: "%", PromQL: nodeDiskUsageQuery(nodeSelector), Status: statusPercent, Description: "资产 HostIP 对应主机的数据盘或根分区最高使用率", Required: true},
		{Key: "diskReadIOPS", Name: "主机磁盘读 IOPS", Unit: "ops/s", PromQL: nodeDiskReadIOPSQuery(nodeSelector), Status: statusNormal, Description: "node_exporter 采集的主机磁盘读 IOPS", Required: true},
		{Key: "diskWriteIOPS", Name: "主机磁盘写 IOPS", Unit: "ops/s", PromQL: nodeDiskWriteIOPSQuery(nodeSelector), Status: statusNormal, Description: "node_exporter 采集的主机磁盘写 IOPS", Required: true},
		{Key: "diskReadLatency", Name: "主机磁盘读延迟", Unit: "ms", PromQL: nodeDiskReadLatencyQuery(nodeSelector), Status: statusDiskLatency, Description: "node_exporter 采集的主机磁盘平均读延迟", Required: true},
		{Key: "diskWriteLatency", Name: "主机磁盘写延迟", Unit: "ms", PromQL: nodeDiskWriteLatencyQuery(nodeSelector), Status: statusDiskLatency, Description: "node_exporter 采集的主机磁盘平均写延迟", Required: true},
		{Key: "usedMemory", Name: "Redis 内存使用", Unit: "bytes", PromQL: redisSumQuery("redis_memory_used_bytes", selector), FallbackPromQL: redisSumQuery("redis_memory_used_bytes", ""), Status: statusNormal, Description: "Redis used_memory"},
		{Key: "memoryUsageRedis", Name: "Redis 内存使用率", Unit: "%", PromQL: redisMemoryUsageQuery(selector), FallbackPromQL: redisMemoryUsageQuery(""), Status: statusPercent, Description: "used_memory / maxmemory"},
		{Key: "connectedClients", Name: "客户端连接数", Unit: "个", PromQL: redisSumQuery("redis_connected_clients", selector), FallbackPromQL: redisSumQuery("redis_connected_clients", ""), Status: statusNormal, Description: "connected_clients"},
		{Key: "blockedClients", Name: "阻塞客户端", Unit: "个", PromQL: redisSumQuery("redis_blocked_clients", selector), FallbackPromQL: redisSumQuery("redis_blocked_clients", ""), Status: statusWarnAboveZero, Description: "blocked_clients"},
		{Key: "qps", Name: "QPS", Unit: "次/s", PromQL: redisSumRateQuery("redis_commands_processed_total", selector), FallbackPromQL: redisSumRateQuery("redis_commands_processed_total", ""), Status: statusNormal, Description: "commands_processed 每秒增长速率"},
		{Key: "hitRate", Name: "命中率", Unit: "%", PromQL: redisHitRateQuery(selector), FallbackPromQL: redisHitRateQuery(""), Status: statusHighPercent, Description: "keyspace hits / (hits + misses)"},
		{Key: "keys", Name: "Key 数量", Unit: "个", PromQL: redisSumQuery("redis_db_keys", selector), FallbackPromQL: redisSumQuery("redis_db_keys", ""), Status: statusNormal, Description: "所有 DB key 数量"},
		{Key: "expiredKeys", Name: "过期 Key 速率", Unit: "个/s", PromQL: redisSumRateQuery("redis_expired_keys_total", selector), FallbackPromQL: redisSumRateQuery("redis_expired_keys_total", ""), Status: statusNormal, Description: "expired_keys 每秒增长速率"},
		{Key: "evictedKeys", Name: "淘汰 Key 速率", Unit: "个/s", PromQL: redisSumRateQuery("redis_evicted_keys_total", selector), FallbackPromQL: redisSumRateQuery("redis_evicted_keys_total", ""), Status: statusWarnAboveZero, Description: "evicted_keys 每秒增长速率"},
		{Key: "errorReplies", Name: "错误响应速率", Unit: "个/s", PromQL: redisSumRateQuery("redis_total_error_replies", selector), FallbackPromQL: redisSumRateQuery("redis_total_error_replies", ""), Status: statusWarnAboveZero, Description: "total_error_replies 每秒增长速率"},
		{Key: "uptime", Name: "运行时长", Unit: "s", PromQL: redisMaxQuery("redis_uptime_in_seconds", selector), FallbackPromQL: redisMaxQuery("redis_uptime_in_seconds", ""), Status: statusNormal, Description: "Redis uptime"},
	}
	return queryMetricCards(ctx, client, defs)
}

func (s *DatabaseService) queryRedisTrends(ctx context.Context, client *prometheusClient, asset *cmdbmodel.CmdbSQL, selector string, start, end time.Time, step string) ([]dbmodel.MonitorTrendPanel, []string) {
	nodeSelector := nodeInstanceSelector(asset.HostIP)
	defs := []mysqlMonitorTrendDef{
		{
			Key:   "systemResource",
			Title: "主机 CPU / 内存",
			Unit:  "% / load",
			Series: []mysqlMonitorMetricDef{
				{Name: "主机 CPU 使用率", PromQL: nodeCPUUsageQuery(nodeSelector), Required: true},
				{Name: "Load 1m", PromQL: nodeLoad1Query(nodeSelector), Required: true},
				{Name: "主机内存使用率", PromQL: nodeMemoryUsageQuery(nodeSelector), Required: true},
			},
		},
		{
			Key:   "diskThroughput",
			Title: "主机磁盘吞吐",
			Unit:  "bytes/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "读吞吐", PromQL: nodeDiskReadBytesQuery(nodeSelector), Required: true},
				{Name: "写吞吐", PromQL: nodeDiskWriteBytesQuery(nodeSelector), Required: true},
			},
		},
		{
			Key:   "diskIOPS",
			Title: "主机磁盘 IOPS",
			Unit:  "ops/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "读 IOPS", PromQL: nodeDiskReadIOPSQuery(nodeSelector), Required: true},
				{Name: "写 IOPS", PromQL: nodeDiskWriteIOPSQuery(nodeSelector), Required: true},
			},
		},
		{
			Key:   "diskLatency",
			Title: "主机磁盘延迟",
			Unit:  "ms",
			Series: []mysqlMonitorMetricDef{
				{Name: "读延迟", PromQL: nodeDiskReadLatencyQuery(nodeSelector), Required: true},
				{Name: "写延迟", PromQL: nodeDiskWriteLatencyQuery(nodeSelector), Required: true},
			},
		},
		{
			Key:   "osNetwork",
			Title: "主机网络",
			Unit:  "bytes/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "接收", PromQL: nodeNetworkReceiveQuery(nodeSelector), Required: true},
				{Name: "发送", PromQL: nodeNetworkTransmitQuery(nodeSelector), Required: true},
			},
		},
		{
			Key:   "memory",
			Title: "Redis 内存",
			Unit:  "bytes",
			Series: []mysqlMonitorMetricDef{
				{Name: "已用内存", PromQL: redisSumQuery("redis_memory_used_bytes", selector), FallbackPromQL: redisSumQuery("redis_memory_used_bytes", "")},
				{Name: "RSS 内存", PromQL: redisSumQuery("redis_memory_used_rss_bytes", selector), FallbackPromQL: redisSumQuery("redis_memory_used_rss_bytes", "")},
				{Name: "最大内存", PromQL: redisSumQuery("redis_memory_max_bytes", selector), FallbackPromQL: redisSumQuery("redis_memory_max_bytes", "")},
			},
		},
		{
			Key:   "memoryRatio",
			Title: "Redis 内存比率",
			Unit:  "%",
			Series: []mysqlMonitorMetricDef{
				{Name: "内存使用率", PromQL: redisMemoryUsageQuery(selector), FallbackPromQL: redisMemoryUsageQuery("")},
				{Name: "碎片率", PromQL: redisAvgQuery("redis_mem_fragmentation_ratio", selector), FallbackPromQL: redisAvgQuery("redis_mem_fragmentation_ratio", "")},
			},
		},
		{
			Key:   "clients",
			Title: "客户端",
			Unit:  "个 / 次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "连接客户端", PromQL: redisSumQuery("redis_connected_clients", selector), FallbackPromQL: redisSumQuery("redis_connected_clients", "")},
				{Name: "阻塞客户端", PromQL: redisSumQuery("redis_blocked_clients", selector), FallbackPromQL: redisSumQuery("redis_blocked_clients", "")},
				{Name: "拒绝连接", PromQL: redisSumRateQuery("redis_rejected_connections_total", selector), FallbackPromQL: redisSumRateQuery("redis_rejected_connections_total", "")},
			},
		},
		{
			Key:   "commands",
			Title: "命令与错误",
			Unit:  "次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "QPS", PromQL: redisSumRateQuery("redis_commands_processed_total", selector), FallbackPromQL: redisSumRateQuery("redis_commands_processed_total", "")},
				{Name: "错误响应", PromQL: redisSumRateQuery("redis_total_error_replies", selector), FallbackPromQL: redisSumRateQuery("redis_total_error_replies", "")},
				{Name: "失败命令", PromQL: redisSumRateQuery("redis_commands_failed_calls_total", selector), FallbackPromQL: redisSumRateQuery("redis_commands_failed_calls_total", "")},
			},
		},
		{
			Key:   "keyspace",
			Title: "Keyspace",
			Unit:  "个 / % / 个/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "Key 数量", PromQL: redisSumQuery("redis_db_keys", selector), FallbackPromQL: redisSumQuery("redis_db_keys", "")},
				{Name: "带 TTL Key", PromQL: redisSumQuery("redis_db_keys_expiring", selector), FallbackPromQL: redisSumQuery("redis_db_keys_expiring", "")},
				{Name: "命中率", PromQL: redisHitRateQuery(selector), FallbackPromQL: redisHitRateQuery("")},
				{Name: "过期 Key", PromQL: redisSumRateQuery("redis_expired_keys_total", selector), FallbackPromQL: redisSumRateQuery("redis_expired_keys_total", "")},
				{Name: "淘汰 Key", PromQL: redisSumRateQuery("redis_evicted_keys_total", selector), FallbackPromQL: redisSumRateQuery("redis_evicted_keys_total", "")},
			},
		},
		{
			Key:   "network",
			Title: "Redis 网络",
			Unit:  "bytes/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "输入", PromQL: redisSumRateQuery("redis_net_input_bytes_total", selector), FallbackPromQL: redisSumRateQuery("redis_net_input_bytes_total", "")},
				{Name: "输出", PromQL: redisSumRateQuery("redis_net_output_bytes_total", selector), FallbackPromQL: redisSumRateQuery("redis_net_output_bytes_total", "")},
			},
		},
		{
			Key:   "persistence",
			Title: "持久化",
			Unit:  "状态 / 秒 / 个",
			Series: []mysqlMonitorMetricDef{
				{Name: "RDB 保存中", PromQL: redisSumQuery("redis_rdb_bgsave_in_progress", selector), FallbackPromQL: redisSumQuery("redis_rdb_bgsave_in_progress", "")},
				{Name: "RDB 上次耗时", PromQL: redisMaxQuery("redis_rdb_last_bgsave_duration_sec", selector), FallbackPromQL: redisMaxQuery("redis_rdb_last_bgsave_duration_sec", "")},
				{Name: "AOF 重写中", PromQL: redisSumQuery("redis_aof_rewrite_in_progress", selector), FallbackPromQL: redisSumQuery("redis_aof_rewrite_in_progress", "")},
				{Name: "待保存变更", PromQL: redisSumQuery("redis_rdb_changes_since_last_save", selector), FallbackPromQL: redisSumQuery("redis_rdb_changes_since_last_save", "")},
			},
		},
		{
			Key:   "replication",
			Title: "复制",
			Unit:  "个 / 状态 / bytes",
			Series: []mysqlMonitorMetricDef{
				{Name: "从节点数", PromQL: redisSumQuery("redis_connected_slaves", selector), FallbackPromQL: redisSumQuery("redis_connected_slaves", "")},
				{Name: "主从链路", PromQL: redisAvgQuery("redis_master_link_up", selector), FallbackPromQL: redisAvgQuery("redis_master_link_up", "")},
				{Name: "复制积压", PromQL: redisSumQuery("redis_replication_backlog_bytes", selector), FallbackPromQL: redisSumQuery("redis_replication_backlog_bytes", "")},
			},
		},
		{
			Key:   "cluster",
			Title: "集群",
			Unit:  "状态 / 个 / 次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "集群状态", PromQL: redisAvgQuery("redis_cluster_state", selector), FallbackPromQL: redisAvgQuery("redis_cluster_state", "")},
				{Name: "集群节点", PromQL: redisMaxQuery("redis_cluster_known_nodes", selector), FallbackPromQL: redisMaxQuery("redis_cluster_known_nodes", "")},
				{Name: "失败槽位", PromQL: redisSumQuery("redis_cluster_slots_fail", selector), FallbackPromQL: redisSumQuery("redis_cluster_slots_fail", "")},
				{Name: "疑似失败槽位", PromQL: redisSumQuery("redis_cluster_slots_pfail", selector), FallbackPromQL: redisSumQuery("redis_cluster_slots_pfail", "")},
				{Name: "集群消息接收", PromQL: redisSumRateQuery("redis_cluster_messages_received_total", selector), FallbackPromQL: redisSumRateQuery("redis_cluster_messages_received_total", "")},
				{Name: "集群消息发送", PromQL: redisSumRateQuery("redis_cluster_messages_sent_total", selector), FallbackPromQL: redisSumRateQuery("redis_cluster_messages_sent_total", "")},
			},
		},
	}
	return queryTrendPanels(ctx, client, defs, start, end, step)
}

func (s *DatabaseService) queryRedisBreakdown(ctx context.Context, client *prometheusClient, selector string) ([]dbmodel.MonitorTablePanel, []string) {
	panels := make([]dbmodel.MonitorTablePanel, 0, 2)
	warnings := make([]string, 0)

	dbRows, err := client.queryInstantRows(ctx, redisMetricQuery("redis_db_keys", selector), "db")
	if err != nil {
		warnings = append(warnings, "redis db keys: "+err.Error())
	}
	if len(dbRows) > 0 {
		panels = append(panels, dbmodel.MonitorTablePanel{
			Key:   "dbKeys",
			Title: "DB Key 分布",
			Columns: []dbmodel.MonitorTableColumn{
				{Key: "name", Title: "DB"},
				{Key: "value", Title: "Key 数", Unit: "个"},
			},
			Rows: dbRows,
		})
	}

	commandRows, err := client.queryInstantRows(ctx, topKQuery(redisCommandRateByQuery(selector), 10), "cmd")
	if err != nil {
		warnings = append(warnings, "redis command top: "+err.Error())
	}
	if len(commandRows) > 0 {
		panels = append(panels, dbmodel.MonitorTablePanel{
			Key:   "commandTop",
			Title: "命令 QPS Top 10",
			Columns: []dbmodel.MonitorTableColumn{
				{Key: "name", Title: "命令"},
				{Key: "value", Title: "QPS", Unit: "次/s"},
			},
			Rows: commandRows,
		})
	}
	return panels, warnings
}

func queryMetricCards(ctx context.Context, client *prometheusClient, defs []mysqlMonitorMetricDef) ([]dbmodel.MonitorMetricCard, []string) {
	cards := make([]dbmodel.MonitorMetricCard, 0, len(defs))
	warnings := make([]string, 0)
	for _, def := range defs {
		promql := def.PromQL
		value, err := client.queryInstantValue(ctx, promql)
		if err != nil && shouldFallbackPromQL(def.PromQL, def.FallbackPromQL) {
			fallbackValue, fallbackErr := client.queryInstantValue(ctx, def.FallbackPromQL)
			if fallbackErr == nil {
				promql = def.FallbackPromQL
				value = fallbackValue
				err = nil
			}
		}
		if err != nil {
			warnings = append(warnings, def.Key+": "+err.Error())
			continue
		}

		status := "normal"
		if def.Status != nil {
			status = def.Status(value)
		}
		cards = append(cards, dbmodel.MonitorMetricCard{
			Key:         def.Key,
			Name:        def.Name,
			Value:       roundFloat(value, 2),
			Unit:        def.Unit,
			Status:      status,
			Description: def.Description,
			PromQL:      promql,
			Required:    def.Required,
		})
	}
	return cards, warnings
}

func queryTrendPanels(ctx context.Context, client *prometheusClient, defs []mysqlMonitorTrendDef, start, end time.Time, step string) ([]dbmodel.MonitorTrendPanel, []string) {
	panels := make([]dbmodel.MonitorTrendPanel, 0, len(defs))
	warnings := make([]string, 0)
	for _, panelDef := range defs {
		panel := dbmodel.MonitorTrendPanel{Key: panelDef.Key, Title: panelDef.Title, Unit: panelDef.Unit}
		for _, seriesDef := range panelDef.Series {
			promql := seriesDef.PromQL
			points, err := client.queryRangePoints(ctx, promql, start, end, step)
			if (err != nil || len(points) == 0) && shouldFallbackPromQL(seriesDef.PromQL, seriesDef.FallbackPromQL) {
				fallbackPoints, fallbackErr := client.queryRangePoints(ctx, seriesDef.FallbackPromQL, start, end, step)
				if fallbackErr == nil && len(fallbackPoints) > 0 {
					promql = seriesDef.FallbackPromQL
					points = fallbackPoints
					err = nil
				}
			}
			if err != nil {
				warnings = append(warnings, panelDef.Key+"/"+seriesDef.Name+": "+err.Error())
				continue
			}
			if len(points) == 0 {
				continue
			}
			panel.Series = append(panel.Series, dbmodel.MonitorMetricSeries{
				Name:     seriesDef.Name,
				PromQL:   promql,
				Points:   points,
				Required: seriesDef.Required,
			})
		}
		if len(panel.Series) > 0 {
			panels = append(panels, panel)
		}
	}
	return panels, warnings
}

func redisMetricQuery(metric, selector string) string {
	if strings.TrimSpace(selector) == "" {
		return metric
	}
	return metric + "{" + selector + "}"
}

func redisRateQuery(metric, selector string) string {
	return "rate(" + redisMetricQuery(metric, selector) + "[5m])"
}

func redisSumQuery(metric, selector string) string {
	return "sum(" + redisMetricQuery(metric, selector) + ")"
}

func redisMaxQuery(metric, selector string) string {
	return "max(" + redisMetricQuery(metric, selector) + ")"
}

func redisAvgQuery(metric, selector string) string {
	return "avg(" + redisMetricQuery(metric, selector) + ")"
}

func redisSumRateQuery(metric, selector string) string {
	return "sum(" + redisRateQuery(metric, selector) + ")"
}

func redisMemoryUsageQuery(selector string) string {
	return "(" + redisSumQuery("redis_memory_used_bytes", selector) + " / clamp_min(" + redisSumQuery("redis_memory_max_bytes", selector) + ", 1)) * 100"
}

func redisHitRateQuery(selector string) string {
	hits := redisSumRateQuery("redis_keyspace_hits_total", selector)
	misses := redisSumRateQuery("redis_keyspace_misses_total", selector)
	return "(" + hits + " / clamp_min((" + hits + " + " + misses + "), 1)) * 100"
}

func redisCommandRateByQuery(selector string) string {
	return "sum by (cmd) (" + redisRateQuery("redis_commands_total", selector) + ")"
}
