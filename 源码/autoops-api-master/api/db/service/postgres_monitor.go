package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"
)

func (s *DatabaseService) PostgreSQLMonitorDashboard(ctx context.Context, assetID uint, query dbmodel.PostgreSQLMonitorQuery) (*dbmodel.PostgreSQLMonitorDashboard, error) {
	asset, err := s.assetDao.GetByID(assetID)
	if err != nil {
		return nil, err
	}
	if asset.Type != cmdbmodel.DatabaseTypePostgreSQL {
		return nil, fmt.Errorf("database asset is not PostgreSQL")
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
	target, warnings := s.resolvePostgreSQLPrometheusTarget(ctx, client, asset)
	selector := target.Selector
	if selector == "" {
		selector = strings.TrimSpace(asset.PrometheusLabelSelector)
	}

	dashboard := &dbmodel.PostgreSQLMonitorDashboard{
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
		dashboard.Warnings = append(dashboard.Warnings, "postgresql monitor target is not matched, skip metric queries")
		return dashboard, nil
	}

	overview, overviewWarnings := s.queryPostgreSQLOverview(ctx, client, asset, selector)
	dashboard.Overview = overview
	dashboard.Warnings = append(dashboard.Warnings, overviewWarnings...)

	trends, trendWarnings := s.queryPostgreSQLTrends(ctx, client, asset, selector, start, end, step)
	dashboard.Trends = trends
	dashboard.Warnings = append(dashboard.Warnings, trendWarnings...)

	breakdown, breakdownWarnings := s.queryPostgreSQLBreakdown(ctx, client, selector)
	dashboard.Breakdown = breakdown
	dashboard.Warnings = append(dashboard.Warnings, breakdownWarnings...)
	return dashboard, nil
}

func (s *DatabaseService) resolvePostgreSQLPrometheusTarget(ctx context.Context, client *prometheusClient, asset *cmdbmodel.CmdbSQL) (dbmodel.MySQLMonitorTarget, []string) {
	target := dbmodel.MySQLMonitorTarget{Prometheus: client.baseURL}
	if selector := strings.TrimSpace(asset.PrometheusLabelSelector); selector != "" {
		target.Matched = true
		target.Selector = selector
		target.MatchBy = "prometheusLabelSelector"
		target.MetricsURL = defaultDatabaseMetricsURL(asset, "postgresql")
		target.ScrapeURL = target.MetricsURL
		enrichMonitorTargetDisplay(client, asset, &target, "postgresql")
		return target, nil
	}

	targets, err := client.targets(ctx)
	if err != nil {
		return target, []string{"query prometheus targets failed: " + err.Error()}
	}

	bestScore := 0
	var best prometheusTarget
	for _, item := range targets {
		score := scorePostgreSQLTarget(asset, item)
		if score > bestScore {
			bestScore = score
			best = item
		}
	}

	if bestScore == 0 {
		return target, []string{"no matching postgresql prometheus target found"}
	}

	labels := mergeTargetLabels(best)
	target.Matched = true
	target.Health = best.Health
	target.ScrapeURL = firstNonEmpty(best.ScrapeURL, best.GlobalURL)
	target.MetricsURL = target.ScrapeURL
	target.LastError = best.LastError
	target.Labels = labels
	target.Selector = buildPrometheusSelector(labels)
	target.MatchBy = postgreSQLTargetMatchBy(asset, best)
	enrichMonitorTargetDisplay(client, asset, &target, "postgresql")
	if target.Selector == "" {
		return target, []string{"matched target has no stable labels for selector"}
	}
	return target, nil
}

func scorePostgreSQLTarget(asset *cmdbmodel.CmdbSQL, target prometheusTarget) int {
	labels := mergeTargetLabels(target)
	haystack := prometheusTargetHaystack(target, labels)
	if !isPostgreSQLTargetCandidate(labels, haystack) {
		return 0
	}

	score := 50
	if strings.Contains(haystack, "postgres") || strings.Contains(haystack, "postgresql") || strings.Contains(haystack, "pg_") {
		score += 20
	}
	for _, key := range []string{"target", "__param_target", "instance"} {
		if targetAddressMatchesAsset(asset, strings.TrimSpace(labels[key])) {
			score += 120
		}
	}
	if targetAddressMatchesAsset(asset, strings.TrimSpace(labels["__address__"])) && !isNodeExporterTarget(labels, haystack) {
		score += 30
	}
	for _, needle := range postgreSQLTargetNeedles(asset) {
		if needle != "" && strings.Contains(haystack, strings.ToLower(needle)) {
			score += 10
		}
	}
	return score
}

func postgreSQLTargetMatchBy(asset *cmdbmodel.CmdbSQL, target prometheusTarget) string {
	labels := mergeTargetLabels(target)
	for _, key := range []string{"target", "__param_target", "instance"} {
		if targetAddressMatchesAsset(asset, strings.TrimSpace(labels[key])) {
			return key
		}
	}
	haystack := prometheusTargetHaystack(target, labels)
	if targetAddressMatchesAsset(asset, strings.TrimSpace(labels["__address__"])) && !isNodeExporterTarget(labels, haystack) {
		return "__address__"
	}
	for _, key := range []string{"instance", "alias", "hostname", "job"} {
		value := strings.TrimSpace(labels[key])
		if value == "" {
			continue
		}
		for _, needle := range postgreSQLTargetNeedles(asset) {
			if needle != "" && strings.Contains(strings.ToLower(value), strings.ToLower(needle)) {
				return key
			}
		}
	}
	return "auto"
}

func isPostgreSQLTargetCandidate(labels map[string]string, haystack string) bool {
	if isNodeExporterTarget(labels, haystack) {
		return false
	}
	for _, key := range []string{"job", "role", "service", "auth_module", "__param_auth_module"} {
		if strings.Contains(strings.ToLower(strings.TrimSpace(labels[key])), "postgres") {
			return true
		}
	}
	return strings.Contains(haystack, "postgres") || strings.Contains(haystack, "postgresql") || strings.Contains(haystack, "pg_")
}

func isNodeExporterTarget(labels map[string]string, haystack string) bool {
	job := strings.ToLower(strings.TrimSpace(labels["job"]))
	address := strings.ToLower(strings.TrimSpace(labels["__address__"]))
	if strings.Contains(job, "node") || strings.Contains(job, "consul-node") {
		return true
	}
	if strings.HasSuffix(address, ":9100") {
		return true
	}
	return strings.Contains(haystack, "node_exporter")
}

func postgreSQLTargetNeedles(asset *cmdbmodel.CmdbSQL) []string {
	needles := []string{asset.HostIP, asset.Endpoint, asset.Name, asset.InstanceName}
	ipParts := strings.Split(asset.HostIP, ".")
	if len(ipParts) == 4 {
		needles = append(needles,
			ipParts[2]+"."+ipParts[3],
			ipParts[2]+"_"+ipParts[3],
			"postgres"+ipParts[2]+"_"+ipParts[3],
			"postgresql"+ipParts[2]+"_"+ipParts[3],
			"pg"+ipParts[2]+"_"+ipParts[3],
			"db"+ipParts[2]+"_"+ipParts[3],
			"node"+ipParts[2]+"-"+ipParts[3],
		)
	}
	return needles
}

func (s *DatabaseService) queryPostgreSQLOverview(ctx context.Context, client *prometheusClient, asset *cmdbmodel.CmdbSQL, selector string) ([]dbmodel.MonitorMetricCard, []string) {
	nodeSelector := nodeInstanceSelector(asset.HostIP)
	defs := []mysqlMonitorMetricDef{
		{Key: "up", Name: "PostgreSQL 状态", Unit: "", PromQL: pgMaxQuery("pg_up", selector), Status: statusUp, Description: "1 表示 exporter 可连接 PostgreSQL"},
		{Key: "scrapeError", Name: "采集错误", Unit: "", PromQL: pgMaxQuery("pg_exporter_last_scrape_error", selector), Status: statusCriticalAboveZero, Description: "1 表示最近一次 PostgreSQL 采集失败"},
		{Key: "scrapeDuration", Name: "采集耗时", Unit: "s", PromQL: pgMaxQuery("pg_exporter_last_scrape_duration_seconds", selector), Status: statusNormal, Description: "最近一次 PostgreSQL 指标采集耗时"},
		{Key: "cpuUsage", Name: "主机 CPU 使用率", Unit: "%", PromQL: nodeCPUUsageQuery(nodeSelector), Status: statusPercent, Description: "资产 HostIP 对应 node_exporter 采集的主机 CPU 使用率"},
		{Key: "memoryUsage", Name: "主机内存使用率", Unit: "%", PromQL: nodeMemoryUsageQuery(nodeSelector), Status: statusPercent, Description: "资产 HostIP 对应 node_exporter 采集的主机内存使用率"},
		{Key: "swapUsage", Name: "主机 Swap 使用率", Unit: "%", PromQL: nodeSwapUsageQuery(nodeSelector), Status: statusPercent, Description: "资产 HostIP 对应 node_exporter 采集的主机 Swap 使用率"},
		{Key: "diskUsage", Name: "主机磁盘使用率", Unit: "%", PromQL: nodeDiskUsageQuery(nodeSelector), Status: statusPercent, Description: "资产 HostIP 对应主机的数据盘或根分区最高使用率"},
		{Key: "diskReadIOPS", Name: "主机磁盘读 IOPS", Unit: "ops/s", PromQL: nodeDiskReadIOPSQuery(nodeSelector), Status: statusNormal, Description: "node_exporter 采集的主机磁盘读 IOPS"},
		{Key: "diskWriteIOPS", Name: "主机磁盘写 IOPS", Unit: "ops/s", PromQL: nodeDiskWriteIOPSQuery(nodeSelector), Status: statusNormal, Description: "node_exporter 采集的主机磁盘写 IOPS"},
		{Key: "diskReadLatency", Name: "主机磁盘读延迟", Unit: "ms", PromQL: nodeDiskReadLatencyQuery(nodeSelector), Status: statusDiskLatency, Description: "node_exporter 采集的主机磁盘平均读延迟"},
		{Key: "diskWriteLatency", Name: "主机磁盘写延迟", Unit: "ms", PromQL: nodeDiskWriteLatencyQuery(nodeSelector), Status: statusDiskLatency, Description: "node_exporter 采集的主机磁盘平均写延迟"},
		{Key: "osNetworkReceive", Name: "主机网络接收", Unit: "bytes/s", PromQL: nodeNetworkReceiveQuery(nodeSelector), Status: statusNormal, Description: "node_exporter 采集的主机入方向网络流量"},
		{Key: "osNetworkTransmit", Name: "主机网络发送", Unit: "bytes/s", PromQL: nodeNetworkTransmitQuery(nodeSelector), Status: statusNormal, Description: "node_exporter 采集的主机出方向网络流量"},
		{Key: "connections", Name: "当前连接数", Unit: "个", PromQL: pgSumQuery("pg_stat_database_numbackends", selector), Status: statusNormal, Description: "所有数据库当前连接数总和"},
		{Key: "maxConnections", Name: "最大连接数", Unit: "个", PromQL: pgMaxQuery("pg_settings_max_connections", selector), Status: statusNormal, Description: "PostgreSQL max_connections"},
		{Key: "connectionsUsage", Name: "连接使用率", Unit: "%", PromQL: pgConnectionUsageQuery(selector), Status: statusPercent, Description: "当前连接数 / max_connections"},
		{Key: "tps", Name: "TPS", Unit: "次/s", PromQL: pgTPSQuery(selector), Status: statusNormal, Description: "commit + rollback 每秒增长速率"},
		{Key: "commitRate", Name: "提交速率", Unit: "次/s", PromQL: pgSumRateQuery("pg_stat_database_xact_commit", selector), Status: statusNormal, Description: "事务提交每秒增长速率"},
		{Key: "rollbackRate", Name: "回滚速率", Unit: "次/s", PromQL: pgSumRateQuery("pg_stat_database_xact_rollback", selector), Status: statusWarnAboveZero, Description: "事务回滚每秒增长速率"},
		{Key: "cacheHitRate", Name: "缓存命中率", Unit: "%", PromQL: pgCacheHitRateQuery(selector), Status: statusHighPercent, Description: "blks_hit / (blks_hit + blks_read)"},
		{Key: "pgBlockReadRate", Name: "PG 数据块读取", Unit: "blocks/s", PromQL: pgSumRateQuery("pg_stat_database_blks_read", selector), Status: statusNormal, Description: "PostgreSQL pg_stat_database_blks_read 每秒增长速率"},
		{Key: "pgAvgBlockReadTime", Name: "PG 平均块读耗时", Unit: "ms", PromQL: pgAverageBlockReadTimeQuery(selector), Status: statusDiskLatency, Description: "PostgreSQL blk_read_time / blks_read"},
		{Key: "pgBackendWriteBuffers", Name: "PG Backend 写缓冲", Unit: "buffers/s", PromQL: pgSumRateQuery("pg_stat_bgwriter_buffers_backend_total", selector), Status: statusNormal, Description: "PostgreSQL backend 直接写缓冲速率"},
		{Key: "pgCheckpointWriteBuffers", Name: "PG Checkpoint 写缓冲", Unit: "buffers/s", PromQL: pgSumRateQuery("pg_stat_bgwriter_buffers_checkpoint_total", selector), Status: statusNormal, Description: "PostgreSQL checkpoint 写缓冲速率"},
		{Key: "deadlocks", Name: "死锁速率", Unit: "次/s", PromQL: pgSumRateQuery("pg_stat_database_deadlocks", selector), Status: statusWarnAboveZero, Description: "死锁每秒增长速率"},
		{Key: "tempBytes", Name: "临时文件写入", Unit: "bytes/s", PromQL: pgSumRateQuery("pg_stat_database_temp_bytes", selector), Status: statusWarnAboveZero, Description: "临时文件写入字节速率"},
		{Key: "tempFiles", Name: "临时文件数", Unit: "个/s", PromQL: pgSumRateQuery("pg_stat_database_temp_files", selector), Status: statusWarnAboveZero, Description: "临时文件创建速率"},
		{Key: "locks", Name: "锁数量", Unit: "个", PromQL: pgSumQuery("pg_locks_count", selector), Status: statusNormal, Description: "当前锁总数"},
		{Key: "exclusiveLocks", Name: "排他锁数量", Unit: "个", PromQL: pgSumQuery("pg_locks_count", appendPrometheusSelector(selector, `mode=~"accessexclusivelock|exclusivelock"`)), Status: statusWarnAboveZero, Description: "AccessExclusiveLock / ExclusiveLock 数量"},
		{Key: "replicationLag", Name: "复制延迟", Unit: "s", PromQL: pgMaxQuery("pg_replication_lag_seconds", selector), Status: statusReplicationLag, Description: "副本复制延迟"},
		{Key: "isReplica", Name: "副本角色", Unit: "", PromQL: pgMaxQuery("pg_replication_is_replica", selector), Status: statusNormal, Description: "1 表示当前实例是副本"},
		{Key: "walSize", Name: "WAL 大小", Unit: "bytes", PromQL: pgMaxQuery("pg_wal_size_bytes", selector), Status: statusNormal, Description: "当前 WAL 文件总大小"},
		{Key: "databaseSize", Name: "数据库总大小", Unit: "bytes", PromQL: pgSumQuery("pg_database_size_bytes", selector), Status: statusNormal, Description: "所有数据库磁盘空间总和"},
	}

	cards := make([]dbmodel.MonitorMetricCard, 0, len(defs))
	warnings := make([]string, 0)
	for _, def := range defs {
		def.Required = isRequiredSystemMetric(def.Key)
		value, err := client.queryInstantValue(ctx, def.PromQL)
		errorMessage := ""
		if err != nil {
			errorMessage = err.Error()
			warnings = append(warnings, def.Key+": "+errorMessage)
			value = 0
		}
		status := "normal"
		if def.Required && err != nil {
			status = "missing"
		} else if def.Status != nil {
			status = def.Status(value)
		}
		cards = append(cards, dbmodel.MonitorMetricCard{
			Key:         def.Key,
			Name:        def.Name,
			Value:       roundFloat(value, 2),
			Unit:        def.Unit,
			Status:      status,
			Description: def.Description,
			PromQL:      def.PromQL,
			Required:    def.Required,
			Error:       errorMessage,
		})
	}
	return cards, warnings
}

func (s *DatabaseService) queryPostgreSQLTrends(ctx context.Context, client *prometheusClient, asset *cmdbmodel.CmdbSQL, selector string, start, end time.Time, step string) ([]dbmodel.MonitorTrendPanel, []string) {
	nodeSelector := nodeInstanceSelector(asset.HostIP)
	defs := []mysqlMonitorTrendDef{
		{
			Key:   "systemResource",
			Title: "主机系统资源",
			Unit:  "%",
			Series: []mysqlMonitorMetricDef{
				{Name: "主机 CPU 使用率", PromQL: nodeCPUUsageQuery(nodeSelector), Required: true},
				{Name: "主机内存使用率", PromQL: nodeMemoryUsageQuery(nodeSelector), Required: true},
				{Name: "主机 Swap 使用率", PromQL: nodeSwapUsageQuery(nodeSelector), Required: true},
				{Name: "主机磁盘使用率", PromQL: nodeDiskUsageQuery(nodeSelector), Required: true},
			},
		},
		{
			Key:   "diskIO",
			Title: "主机磁盘 IO",
			Unit:  "ops/s 或 ms",
			Series: []mysqlMonitorMetricDef{
				{Name: "读 IOPS", PromQL: nodeDiskReadIOPSQuery(nodeSelector), Required: true},
				{Name: "写 IOPS", PromQL: nodeDiskWriteIOPSQuery(nodeSelector), Required: true},
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
			Key:   "connections",
			Title: "连接",
			Unit:  "个 / %",
			Series: []mysqlMonitorMetricDef{
				{Name: "当前连接", PromQL: pgSumQuery("pg_stat_database_numbackends", selector)},
				{Name: "连接使用率", PromQL: pgConnectionUsageQuery(selector)},
				{Name: "最大连接数", PromQL: pgMaxQuery("pg_settings_max_connections", selector)},
			},
		},
		{
			Key:   "transactions",
			Title: "事务",
			Unit:  "次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "TPS", PromQL: pgTPSQuery(selector)},
				{Name: "提交", PromQL: pgSumRateQuery("pg_stat_database_xact_commit", selector)},
				{Name: "回滚", PromQL: pgSumRateQuery("pg_stat_database_xact_rollback", selector)},
				{Name: "死锁", PromQL: pgSumRateQuery("pg_stat_database_deadlocks", selector)},
			},
		},
		{
			Key:   "cache",
			Title: "PostgreSQL 块读与缓存",
			Unit:  "% / blocks/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "缓存命中率", PromQL: pgCacheHitRateQuery(selector)},
				{Name: "缓存命中", PromQL: pgSumRateQuery("pg_stat_database_blks_hit", selector)},
				{Name: "磁盘块读取", PromQL: pgSumRateQuery("pg_stat_database_blks_read", selector)},
				{Name: "平均块读耗时", PromQL: pgAverageBlockReadTimeQuery(selector)},
				{Name: "读耗时", PromQL: pgSumRateQuery("pg_stat_database_blk_read_time", selector)},
				{Name: "写耗时", PromQL: pgSumRateQuery("pg_stat_database_blk_write_time", selector)},
			},
		},
		{
			Key:   "tuples",
			Title: "行操作",
			Unit:  "行/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "返回", PromQL: pgSumRateQuery("pg_stat_database_tup_returned", selector)},
				{Name: "获取", PromQL: pgSumRateQuery("pg_stat_database_tup_fetched", selector)},
				{Name: "插入", PromQL: pgSumRateQuery("pg_stat_database_tup_inserted", selector)},
				{Name: "更新", PromQL: pgSumRateQuery("pg_stat_database_tup_updated", selector)},
				{Name: "删除", PromQL: pgSumRateQuery("pg_stat_database_tup_deleted", selector)},
			},
		},
		{
			Key:   "temp",
			Title: "临时文件",
			Unit:  "bytes/s 或 个/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "临时文件写入", PromQL: pgSumRateQuery("pg_stat_database_temp_bytes", selector)},
				{Name: "临时文件数", PromQL: pgSumRateQuery("pg_stat_database_temp_files", selector)},
			},
		},
		{
			Key:   "locks",
			Title: "锁",
			Unit:  "个",
			Series: []mysqlMonitorMetricDef{
				{Name: "总锁数", PromQL: pgSumQuery("pg_locks_count", selector)},
				{Name: "排他锁", PromQL: pgSumQuery("pg_locks_count", appendPrometheusSelector(selector, `mode=~"accessexclusivelock|exclusivelock"`))},
				{Name: "访问共享锁", PromQL: pgSumQuery("pg_locks_count", appendPrometheusSelector(selector, `mode="accesssharelock"`))},
			},
		},
		{
			Key:   "replication",
			Title: "复制",
			Unit:  "s / 状态",
			Series: []mysqlMonitorMetricDef{
				{Name: "复制延迟", PromQL: pgMaxQuery("pg_replication_lag_seconds", selector)},
				{Name: "最后回放时间", PromQL: pgMaxQuery("pg_replication_last_replay_seconds", selector)},
				{Name: "是否副本", PromQL: pgMaxQuery("pg_replication_is_replica", selector)},
			},
		},
		{
			Key:   "bgwriter",
			Title: "PostgreSQL 写缓冲",
			Unit:  "buffers/s / 次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "Checkpoint 写缓冲", PromQL: pgSumRateQuery("pg_stat_bgwriter_buffers_checkpoint_total", selector)},
				{Name: "Backend 写缓冲", PromQL: pgSumRateQuery("pg_stat_bgwriter_buffers_backend_total", selector)},
				{Name: "BGWriter 写缓冲", PromQL: pgSumRateQuery("pg_stat_bgwriter_buffers_clean_total", selector)},
				{Name: "Backend fsync", PromQL: pgSumRateQuery("pg_stat_bgwriter_buffers_backend_fsync_total", selector)},
				{Name: "请求 Checkpoint", PromQL: pgSumRateQuery("pg_stat_bgwriter_checkpoints_req_total", selector)},
				{Name: "定时 Checkpoint", PromQL: pgSumRateQuery("pg_stat_bgwriter_checkpoints_timed_total", selector)},
			},
		},
		{
			Key:   "wal",
			Title: "WAL",
			Unit:  "bytes / 个",
			Series: []mysqlMonitorMetricDef{
				{Name: "WAL 大小", PromQL: pgMaxQuery("pg_wal_size_bytes", selector)},
				{Name: "WAL 段数", PromQL: pgMaxQuery("pg_wal_segments", selector)},
				{Name: "归档成功", PromQL: pgSumRateQuery("pg_stat_archiver_archived_count", selector)},
				{Name: "归档失败", PromQL: pgSumRateQuery("pg_stat_archiver_failed_count", selector)},
			},
		},
	}

	panels := make([]dbmodel.MonitorTrendPanel, 0, len(defs))
	warnings := make([]string, 0)
	for _, panelDef := range defs {
		panel := dbmodel.MonitorTrendPanel{Key: panelDef.Key, Title: panelDef.Title, Unit: panelDef.Unit}
		for _, seriesDef := range panelDef.Series {
			points, err := client.queryRangePoints(ctx, seriesDef.PromQL, start, end, step)
			errorMessage := ""
			if err != nil {
				errorMessage = err.Error()
				warnings = append(warnings, panelDef.Key+"/"+seriesDef.Name+": "+errorMessage)
			}
			panel.Series = append(panel.Series, dbmodel.MonitorMetricSeries{
				Name:     seriesDef.Name,
				PromQL:   seriesDef.PromQL,
				Points:   points,
				Required: seriesDef.Required,
				Error:    errorMessage,
			})
		}
		panels = append(panels, panel)
	}
	return panels, warnings
}

func (s *DatabaseService) queryPostgreSQLBreakdown(ctx context.Context, client *prometheusClient, selector string) ([]dbmodel.MonitorTablePanel, []string) {
	panels := make([]dbmodel.MonitorTablePanel, 0, 4)
	warnings := make([]string, 0)

	databaseSize, err := client.queryInstantRows(ctx, topKQuery(pgMetricQuery("pg_database_size_bytes", selector), 10), "datname")
	if err != nil {
		warnings = append(warnings, "database size: "+err.Error())
	}
	panels = append(panels, dbmodel.MonitorTablePanel{
		Key:   "databaseSize",
		Title: "数据库大小 Top 10",
		Columns: []dbmodel.MonitorTableColumn{
			{Key: "name", Title: "数据库"},
			{Key: "value", Title: "大小", Unit: "bytes"},
		},
		Rows: databaseSize,
	})

	databaseConnections, err := client.queryInstantRows(ctx, topKQuery(pgMetricQuery("pg_stat_database_numbackends", selector), 10), "datname")
	if err != nil {
		warnings = append(warnings, "database connections: "+err.Error())
	}
	panels = append(panels, dbmodel.MonitorTablePanel{
		Key:   "databaseConnections",
		Title: "数据库连接 Top 10",
		Columns: []dbmodel.MonitorTableColumn{
			{Key: "name", Title: "数据库"},
			{Key: "value", Title: "连接数", Unit: "个"},
		},
		Rows: databaseConnections,
	})

	lockModes, err := client.queryInstantRows(ctx, "sum by (mode) ("+pgMetricQuery("pg_locks_count", selector)+")", "mode")
	if err != nil {
		warnings = append(warnings, "lock modes: "+err.Error())
	}
	panels = append(panels, dbmodel.MonitorTablePanel{
		Key:   "lockModes",
		Title: "锁模式分布",
		Columns: []dbmodel.MonitorTableColumn{
			{Key: "name", Title: "锁模式"},
			{Key: "value", Title: "数量", Unit: "个"},
		},
		Rows: lockModes,
	})

	transactionRate, err := client.queryInstantRows(ctx, topKQuery("sum by (datname) ("+pgRateQuery("pg_stat_database_xact_commit", selector)+" + "+pgRateQuery("pg_stat_database_xact_rollback", selector)+")", 10), "datname")
	if err != nil {
		warnings = append(warnings, "database transaction rate: "+err.Error())
	}
	panels = append(panels, dbmodel.MonitorTablePanel{
		Key:   "databaseTransactionRate",
		Title: "数据库 TPS Top 10",
		Columns: []dbmodel.MonitorTableColumn{
			{Key: "name", Title: "数据库"},
			{Key: "value", Title: "TPS", Unit: "次/s"},
		},
		Rows: transactionRate,
	})
	return panels, warnings
}

func pgMetricQuery(metric, selector string) string {
	if strings.TrimSpace(selector) == "" {
		return metric
	}
	return metric + "{" + selector + "}"
}

func pgRateQuery(metric, selector string) string {
	return "rate(" + pgMetricQuery(metric, selector) + "[5m])"
}

func pgSumQuery(metric, selector string) string {
	return "sum(" + pgMetricQuery(metric, selector) + ")"
}

func pgMaxQuery(metric, selector string) string {
	return "max(" + pgMetricQuery(metric, selector) + ")"
}

func pgSumRateQuery(metric, selector string) string {
	return "sum(" + pgRateQuery(metric, selector) + ")"
}

func pgConnectionUsageQuery(selector string) string {
	return "(" + pgSumQuery("pg_stat_database_numbackends", selector) + " / " + pgMaxQuery("pg_settings_max_connections", selector) + ") * 100"
}

func pgTPSQuery(selector string) string {
	return pgSumRateQuery("pg_stat_database_xact_commit", selector) + " + " + pgSumRateQuery("pg_stat_database_xact_rollback", selector)
}

func pgCacheHitRateQuery(selector string) string {
	hits := pgSumRateQuery("pg_stat_database_blks_hit", selector)
	reads := pgSumRateQuery("pg_stat_database_blks_read", selector)
	return "(" + hits + " / (" + hits + " + " + reads + ")) * 100"
}

func pgAverageBlockReadTimeQuery(selector string) string {
	readTime := pgSumRateQuery("pg_stat_database_blk_read_time", selector)
	reads := pgSumRateQuery("pg_stat_database_blks_read", selector)
	return readTime + " / clamp_min(" + reads + ", 1)"
}

func statusCriticalAboveZero(value float64) string {
	if value > 0 {
		return "critical"
	}
	return "normal"
}
