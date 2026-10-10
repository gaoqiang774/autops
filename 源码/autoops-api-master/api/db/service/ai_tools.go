package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	aiopsservice "dodevops-api/api/aiops/service"
	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"

	"github.com/gin-gonic/gin"
)

func (s *DatabaseService) ListAITools() []dbmodel.DBAIToolDefinition {
	return []dbmodel.DBAIToolDefinition{
		{
			Name:           dbmodel.DBAIToolSearchObjects,
			Description:    "搜索数据库、schema、表、视图、Redis DB/Key 等对象，供 AI 助手定位上下文。",
			ReadOnly:       true,
			RequiredFields: []string{"assetId"},
			OptionalFields: []string{"database", "schema", "objectName", "keyword"},
		},
		{
			Name:           dbmodel.DBAIToolGetTableSchema,
			Description:    "获取表或视图的字段、索引和 DDL。",
			ReadOnly:       true,
			RequiredFields: []string{"assetId", "database", "objectName"},
			OptionalFields: []string{"schema", "objectType"},
		},
		{
			Name:           dbmodel.DBAIToolExecuteReadonlySQL,
			Description:    "执行只读 SELECT/WITH SQL 或 Redis 只读命令，结果自动分页并保留现有只读保护。",
			ReadOnly:       true,
			RequiredFields: []string{"assetId", "sql"},
			OptionalFields: []string{"database", "schema", "page", "pageSize", "confirmed"},
		},
		{
			Name:           dbmodel.DBAIToolExplainSQL,
			Description:    "获取只读 SQL 的执行计划，不执行 ANALYZE。",
			ReadOnly:       true,
			RequiredFields: []string{"assetId", "sql"},
			OptionalFields: []string{"database", "schema"},
		},
		{
			Name:           dbmodel.DBAIToolGetMonitorSnapshot,
			Description:    "获取数据库监控快照，返回目标匹配信息和当前概览指标。",
			ReadOnly:       true,
			RequiredFields: []string{"assetId"},
			OptionalFields: []string{"range", "step"},
		},
		{
			Name:           dbmodel.DBAIToolGetSlowSQLTop,
			Description:    "获取慢 SQL Top。MySQL 会读取数据库资产 slowLogPath 配置的慢日志并按 SQL 指纹聚合。",
			ReadOnly:       true,
			RequiredFields: []string{"assetId"},
			OptionalFields: []string{"authId", "database", "keyword", "range", "pageSize", "tailLines"},
		},
		{
			Name:           dbmodel.DBAIToolGetInstanceHealth,
			Description:    "汇总实例健康状态、Prometheus target、关键异常指标和监控告警线索。",
			ReadOnly:       true,
			RequiredFields: []string{"assetId"},
			OptionalFields: []string{"range", "step"},
		},
	}
}

func (s *DatabaseService) RunAITool(ctx *gin.Context, req dbmodel.DBAIToolRequest) (*dbmodel.DBAIToolResult, error) {
	req.Tool = strings.TrimSpace(req.Tool)
	if req.Tool == "" {
		return nil, fmt.Errorf("tool is required")
	}
	if req.AssetID == 0 {
		return nil, fmt.Errorf("assetId is required")
	}

	asset, err := s.assetDao.GetByID(req.AssetID)
	if err != nil {
		return nil, err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionAITool, auditSafeAIToolRequest(req))
	recorder.SetObject(req.Database, req.Schema, req.ObjectName, firstNonEmpty(req.ObjectType, "ai_tool:"+req.Tool))
	defer func() { recorder.Finish(err) }()

	var result *dbmodel.DBAIToolResult
	switch req.Tool {
	case dbmodel.DBAIToolSearchObjects:
		result, err = s.aiSearchObjects(ctx, req)
	case dbmodel.DBAIToolGetTableSchema:
		result, err = s.aiGetTableSchema(ctx, req)
	case dbmodel.DBAIToolExecuteReadonlySQL:
		result, err = s.aiExecuteReadonlySQL(ctx, req)
	case dbmodel.DBAIToolExplainSQL:
		result, err = s.aiExplainSQL(ctx, req)
	case dbmodel.DBAIToolGetMonitorSnapshot:
		result, err = s.aiGetMonitorSnapshot(ctx, req)
	case dbmodel.DBAIToolGetSlowSQLTop:
		result, err = s.aiGetSlowSQLTop(ctx, req)
	case dbmodel.DBAIToolGetInstanceHealth:
		result, err = s.aiGetInstanceHealth(ctx, req)
	default:
		err = fmt.Errorf("unsupported ai tool: %s", req.Tool)
	}
	if result != nil {
		recorder.SetRowsAffected(aiToolRowsAffected(result))
	}
	return result, err
}

func (s *DatabaseService) aiSearchObjects(ctx *gin.Context, req dbmodel.DBAIToolRequest) (*dbmodel.DBAIToolResult, error) {
	query := dbmodel.SQLCompletionQuery{
		AssetID:    req.AssetID,
		Database:   req.Database,
		Schema:     req.Schema,
		ObjectName: req.ObjectName,
		Keyword:    req.Keyword,
	}
	data, err := s.Completion(ctx, query)
	if err != nil {
		return nil, err
	}
	total := len(data.Databases) + len(data.Tables) + len(data.Columns)
	return &dbmodel.DBAIToolResult{
		Tool:      req.Tool,
		Supported: true,
		Summary:   fmt.Sprintf("匹配到 %d 个数据库对象候选", total),
		Data:      data,
		Warnings:  []string{},
	}, nil
}

func (s *DatabaseService) aiGetTableSchema(ctx *gin.Context, req dbmodel.DBAIToolRequest) (*dbmodel.DBAIToolResult, error) {
	objectReq := dbmodel.ObjectRequest{
		AssetID:    req.AssetID,
		Database:   req.Database,
		Schema:     req.Schema,
		ObjectName: req.ObjectName,
		ObjectType: firstNonEmpty(req.ObjectType, dbmodel.ObjectTypeTable),
	}
	if strings.TrimSpace(objectReq.Database) == "" {
		return nil, fmt.Errorf("database is required")
	}
	if strings.TrimSpace(objectReq.ObjectName) == "" {
		return nil, fmt.Errorf("objectName is required")
	}

	result := dbmodel.DBTableSchemaResult{}
	warnings := []string{}
	columns, err := s.Columns(ctx, objectReq)
	if err != nil {
		return nil, err
	}
	result.Columns = columns

	if indexes, err := s.Indexes(ctx, objectReq); err == nil {
		result.Indexes = indexes
	} else {
		warnings = append(warnings, "indexes: "+err.Error())
	}
	if ddl, err := s.DDL(ctx, objectReq); err == nil {
		result.DDL = ddl
	} else {
		warnings = append(warnings, "ddl: "+err.Error())
	}

	return &dbmodel.DBAIToolResult{
		Tool:      req.Tool,
		Supported: true,
		Summary:   fmt.Sprintf("已获取对象 %s 的结构信息，字段 %d 个，索引 %d 个", objectReq.ObjectName, len(result.Columns), len(result.Indexes)),
		Data:      result,
		Warnings:  warnings,
	}, nil
}

func (s *DatabaseService) aiExecuteReadonlySQL(ctx *gin.Context, req dbmodel.DBAIToolRequest) (*dbmodel.DBAIToolResult, error) {
	page, pageSize := dbmodel.NormalizePage(req.Page, req.PageSize)
	if pageSize > 50 {
		pageSize = 50
	}
	data, err := s.ExecuteSQL(ctx, dbmodel.SQLExecuteRequest{
		AssetID:   req.AssetID,
		Database:  req.Database,
		Schema:    req.Schema,
		SQL:       req.SQL,
		Page:      page,
		PageSize:  pageSize,
		Confirmed: req.Confirmed,
	})
	if err != nil {
		return nil, err
	}
	summary := fmt.Sprintf("只读查询返回 %d 行，总计 %d 行", len(data.Rows), data.Total)
	if data.RequireConfirm {
		summary = "SQL 预计耗时较高，需要确认后再执行"
	} else {
		data.Rows = redactSQLRows(data.Columns, data.Rows)
	}
	return &dbmodel.DBAIToolResult{
		Tool:      req.Tool,
		Supported: true,
		Summary:   summary,
		Data:      data,
		Warnings:  []string{},
	}, nil
}

func (s *DatabaseService) aiExplainSQL(ctx *gin.Context, req dbmodel.DBAIToolRequest) (*dbmodel.DBAIToolResult, error) {
	asset, cfg, err := s.BuildConfig(req.AssetID, req.Database)
	if err != nil {
		return nil, err
	}
	sqlText := normalizeSQLText(req.SQL)
	if sqlText == "" {
		return nil, fmt.Errorf("sql is required")
	}
	if !isReadOnlySQL(sqlText) {
		return nil, fmt.Errorf("only SELECT/WITH read-only SQL is allowed")
	}

	var data *dbmodel.DBExplainSQLResult
	switch asset.Type {
	case cmdbmodel.DatabaseTypeMySQL:
		data, err = explainMySQLSQL(contextFromGin(ctx), cfg, req.Database, sqlText)
	case cmdbmodel.DatabaseTypePostgreSQL:
		data, err = explainPostgresSQL(contextFromGin(ctx), cfg, req.Database, sqlText)
	case cmdbmodel.DatabaseTypeRedis:
		return &dbmodel.DBAIToolResult{
			Tool:      req.Tool,
			Supported: false,
			Summary:   "Redis 不支持 SQL 执行计划",
			Data:      nil,
			Warnings:  []string{"redis command has no SQL explain plan"},
		}, nil
	default:
		return nil, fmt.Errorf("database type %d does not support explain", asset.Type)
	}
	if err != nil {
		return nil, err
	}
	data.Engine = asset.Type
	data.Summary = summarizeExplainPlan(asset.Type, data.Plan, data.Estimate)
	return &dbmodel.DBAIToolResult{
		Tool:      req.Tool,
		Supported: true,
		Summary:   fmt.Sprintf("已获取执行计划，共 %d 行", len(data.Plan)),
		Data:      data,
		Warnings:  []string{},
	}, nil
}

func (s *DatabaseService) aiGetMonitorSnapshot(ctx *gin.Context, req dbmodel.DBAIToolRequest) (*dbmodel.DBAIToolResult, error) {
	dashboard, warnings, err := s.aiLoadMonitorDashboard(contextFromGin(ctx), req)
	if err != nil {
		return nil, err
	}
	snapshot := dbmodel.DBMonitorSnapshot{
		Asset:    dashboard.Asset,
		Target:   dashboard.Target,
		Overview: nonEmptyMetricCards(dashboard.Overview),
		Range:    dashboard.Range,
	}
	warnings = append(warnings, dashboard.Warnings...)
	return &dbmodel.DBAIToolResult{
		Tool:      req.Tool,
		Supported: true,
		Summary:   fmt.Sprintf("监控快照获取完成，目标状态: %s，概览指标 %d 个", firstNonEmpty(snapshot.Target.Health, "unknown"), len(snapshot.Overview)),
		Data:      snapshot,
		Warnings:  compactStrings(warnings),
	}, nil
}

func (s *DatabaseService) aiGetSlowSQLTop(ctx *gin.Context, req dbmodel.DBAIToolRequest) (*dbmodel.DBAIToolResult, error) {
	result, err := s.analyzeSlowLogRecent50MB(ctx, req)
	if err != nil {
		return nil, errors.New(formatSlowLogAIError(err))
	}
	message := "用户未配置慢日志路径，无法分析，请先配置"
	if result.Source.Configured {
		message = fmt.Sprintf("已分析最近 50MB 慢日志，返回耗时最长的 Top %d，聚合 SQL 指纹 %d 个，匹配慢日志 %d 条", len(result.Items), result.Summary.UniqueSQL, result.Summary.MatchedEntries)
	}
	items := make([]map[string]interface{}, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, map[string]interface{}{
			"digest":            item.Digest,
			"database":          item.Database,
			"user":              item.User,
			"host":              item.Host,
			"count":             item.Count,
			"queryTimeAvg":      item.QueryTimeAvg,
			"queryTimeP95":      item.QueryTimeP95,
			"queryTimeMax":      item.QueryTimeMax,
			"queryTimeTotal":    item.QueryTimeTotal,
			"lockTimeTotal":     item.LockTimeTotal,
			"rowsExaminedTotal": item.RowsExaminedTotal,
			"rowsExaminedMax":   item.RowsExaminedMax,
			"rowsSentTotal":     item.RowsSentTotal,
			"rowsSentMax":       item.RowsSentMax,
			"aiSuggestion":      item.AISuggestion,
			"fingerprint":       item.Fingerprint,
			"sampleSql":         item.SampleSQL,
			"firstSeen":         item.FirstSeen,
			"lastSeen":          item.LastSeen,
		})
	}
	data := dbmodel.DBSlowSQLTopResult{
		SourceConfigured: result.Source.Configured,
		Source:           result.Source.Path,
		Items:            items,
		Message:          message,
	}
	return &dbmodel.DBAIToolResult{
		Tool:      req.Tool,
		Supported: true,
		Summary:   message,
		Data:      data,
		Warnings:  result.Warnings,
	}, nil
}

func (s *DatabaseService) analyzeSlowLogRecent50MB(ctx *gin.Context, req dbmodel.DBAIToolRequest) (*dbmodel.SlowLogAnalyzeResult, error) {
	asset, err := s.assetDao.GetByID(req.AssetID)
	if err != nil {
		return nil, err
	}
	if asset.Type != cmdbmodel.DatabaseTypeMySQL {
		return nil, fmt.Errorf("slow log analysis currently supports MySQL only")
	}
	if strings.TrimSpace(asset.SlowLogPath) == "" {
		return &dbmodel.SlowLogAnalyzeResult{
			Asset: buildAssetVO(*asset),
			Source: dbmodel.SlowLogSource{
				Configured: false,
				Type:       "mysql_slow_log",
			},
			Summary: dbmodel.SlowLogSummary{},
			Items:   []dbmodel.SlowLogItem{},
			Warnings: []string{
				"用户未配置慢日志路径，无法分析，请先配置",
			},
			GeneratedAt: time.Now().Format(time.RFC3339),
		}, nil
	}
	hostID, fallbackHost, err := resolveSlowLogHost(asset)
	if err != nil {
		return nil, err
	}

	query := dbmodel.SlowLogQuery{
		AuthID:    req.AuthID,
		Database:  req.Database,
		Keyword:   req.Keyword,
		SortBy:    "queryTimeMax",
		SortOrder: "desc",
		Page:      1,
		PageSize:  5,
	}
	filter, err := buildSlowLogFilter(query)
	if err != nil {
		return nil, err
	}
	acc := newSlowLogAccumulator(asset, filter)
	parser := newMySQLSlowLogStreamParser(func(entry mysqlSlowLogEntry) {
		acc.add(entry)
	})

	var hostName string
	hostIP := asset.HostIP
	var totalBytes int64
	host, err := aiopsservice.NewLogAnalysisService().ReadHostFileTailChunks(ctx, hostID, req.AuthID, asset.SlowLogPath, 256*1024, maxSlowLogAIReadBytes, func(chunk []byte, readBytes, remoteTotalBytes int64) error {
		totalBytes = remoteTotalBytes
		parser.Write(chunk)
		return nil
	})
	if err != nil {
		return nil, err
	}
	parser.Finish()
	if host != nil {
		hostName = firstNonEmpty(host.HostName, host.Name)
		hostIP = firstNonEmpty(host.PrivateIP, host.SSHIP, host.PublicIP, asset.HostIP)
	} else if fallbackHost != nil {
		hostName = firstNonEmpty(fallbackHost.HostName, fallbackHost.Name)
		hostIP = firstNonEmpty(fallbackHost.PrivateIP, fallbackHost.SSHIP, fallbackHost.PublicIP, asset.HostIP)
	}

	result := acc.result(query, 5)
	warnings := make([]string, 0)
	if totalBytes > maxSlowLogAIReadBytes {
		warnings = append(warnings, fmt.Sprintf("日志文件超过 50MB，本次分析最近 %d MB 内容", maxSlowLogAIReadBytes/1024/1024))
	}
	enrichSlowLogFastSuggestions(&result)
	result.Asset = buildAssetVO(*asset)
	result.Source = dbmodel.SlowLogSource{
		Configured: true,
		Type:       "mysql_slow_log_tail_50mb",
		Path:       asset.SlowLogPath,
		HostID:     hostID,
		HostIP:     hostIP,
		HostName:   hostName,
		ReadLines:  result.Summary.TotalEntries,
	}
	result.Warnings = warnings
	result.GeneratedAt = time.Now().Format(time.RFC3339)
	return &result, nil
}

func enrichSlowLogFastSuggestions(result *dbmodel.SlowLogAnalyzeResult) {
	if result == nil {
		return
	}
	for i := range result.Items {
		if !slowLogAISuggestionEmpty(result.Items[i].AISuggestion) {
			continue
		}
		result.Items[i].AISuggestion = buildSlowLogFastSuggestion(result.Items[i])
	}
}

func buildSlowLogFastSuggestion(item dbmodel.SlowLogItem) dbmodel.SlowLogAISuggestion {
	sqlText := strings.ToLower(firstNonEmpty(item.Fingerprint, item.SampleSQL))
	suggestion := dbmodel.SlowLogAISuggestion{
		SlowReason:      "执行耗时较高，需要结合 EXPLAIN 确认执行计划、索引使用情况和实际扫描范围。",
		SQLOptimization: "控制返回数据量，避免一次性扫描或返回大量数据；必要时按主键、时间或业务维度分批查询。",
		ExpectedEffect:  "预期降低扫描行数、I/O 和网络开销，缩短查询响应时间。",
	}
	switch {
	case strings.Contains(sqlText, "select *"):
		suggestion.SlowReason = "查询使用 SELECT *，可能读取过多字段并放大 I/O、网络传输和客户端处理开销。"
		suggestion.SQLOptimization = "避免 SELECT *，只查询必要字段；增加 WHERE 条件或分页，避免一次性返回整表数据。"
	case !strings.Contains(sqlText, " where ") && strings.HasPrefix(strings.TrimSpace(sqlText), "select"):
		suggestion.SlowReason = "查询缺少 WHERE 条件，存在全表扫描和大结果集返回风险。"
		suggestion.SQLOptimization = "补充业务过滤条件；如确需全量读取，改为分页、异步导出或离线任务。"
	case strings.Contains(sqlText, " join "):
		suggestion.SlowReason = "查询包含多表关联，关联字段或过滤字段索引不足时容易产生大量扫描、临时表或文件排序。"
		suggestion.IndexOptimization = "检查 JOIN 条件、WHERE、GROUP BY、ORDER BY 字段，优先为高选择性过滤字段和关联字段建立复合索引。"
		suggestion.SQLOptimization = "先过滤再关联，减少参与 JOIN 的数据量；必要时拆分复杂 SQL 或使用中间结果表。"
	case strings.Contains(sqlText, " group by ") || strings.Contains(sqlText, " order by "):
		suggestion.SlowReason = "查询包含排序或聚合，索引不匹配时可能产生临时表和文件排序。"
		suggestion.IndexOptimization = "结合过滤条件、GROUP BY、ORDER BY 字段设计联合索引，减少排序和临时表开销。"
	}
	if item.RowsExaminedMax >= 1000000 || item.RowsExaminedTotal >= 1000000 {
		suggestion.IndexOptimization = firstNonEmpty(suggestion.IndexOptimization, "扫描行数较高，建议基于 WHERE 条件、关联字段和排序字段补充高选择性索引。")
	}
	if strings.Contains(sqlText, " tenant_id") {
		suggestion.IndexOptimization = firstNonEmpty(suggestion.IndexOptimization, "如 tenant_id 是常用过滤条件，建议评估 tenant_id 与时间/状态等字段的复合索引。")
	}
	if strings.Contains(sqlText, "date") || strings.Contains(sqlText, "time") {
		suggestion.IndexOptimization = firstNonEmpty(suggestion.IndexOptimization, "如时间字段是常用过滤条件，建议评估时间字段索引或按时间范围分区。")
	}
	return suggestion
}

func formatSlowLogAIError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "host has multiple ssh credentials") || strings.Contains(msg, "multiple SSH credentials"):
		return "主机绑定了多个 SSH 凭据，请选择登录凭据 authId 后再分析慢日志"
	case strings.Contains(msg, "host has no ssh credential"):
		return "关联主机未绑定 SSH 登录凭据，无法登录主机读取慢日志"
	case strings.Contains(msg, "selected ssh credential is not bound to host"):
		return "选择的 SSH 登录凭据未绑定到该主机，请重新选择"
	case strings.Contains(msg, "host ssh info is incomplete"):
		return "关联主机 SSH 信息不完整，请检查 SSH 地址和端口"
	case strings.Contains(msg, "ssh username is empty"):
		return "SSH 登录用户名为空，请检查主机登录凭据配置"
	case strings.Contains(msg, "open remote log file failed"):
		return "打开远程慢日志文件失败，请检查慢日志路径和文件权限"
	case strings.Contains(msg, "stat remote log file failed"):
		return "读取远程慢日志文件信息失败，请检查慢日志路径和文件权限"
	case strings.Contains(msg, "read remote log file failed"):
		return "读取远程慢日志文件失败，请检查主机连接和文件权限"
	case strings.Contains(msg, "database asset has no hostId or hostIp"):
		return "数据库资产未关联主机，无法登录主机读取慢日志"
	case strings.Contains(msg, "cannot find cmdb host by hostIp"):
		return "未找到数据库资产关联的主机，无法登录主机读取慢日志"
	default:
		return msg
	}
}

func (s *DatabaseService) aiGetInstanceHealth(ctx *gin.Context, req dbmodel.DBAIToolRequest) (*dbmodel.DBAIToolResult, error) {
	data, err := s.GetInstanceHealth(contextFromGin(ctx), req)
	if err != nil {
		return nil, err
	}
	return &dbmodel.DBAIToolResult{
		Tool:      req.Tool,
		Supported: true,
		Summary:   fmt.Sprintf("实例健康状态: %s，异常指标 %d 个", data.Status, len(data.Abnormal)),
		Data:      data,
		Warnings:  data.Warnings,
	}, nil
}

func (s *DatabaseService) GetInstanceHealth(ctx context.Context, req dbmodel.DBAIToolRequest) (*dbmodel.DBInstanceHealthResult, error) {
	dashboard, warnings, err := s.aiLoadMonitorDashboard(ctx, req)
	if err != nil {
		return nil, err
	}
	abnormal := abnormalMetricCards(dashboard.Overview)
	warnings = append(warnings, dashboard.Warnings...)
	health := firstNonEmpty(dashboard.Target.Health, "unknown")
	status := "healthy"
	if !dashboard.Target.Matched || health == "down" || health == "error" {
		status = "critical"
	} else if len(abnormal) > 0 || len(warnings) > 0 {
		status = "warning"
	}
	data := dbmodel.DBInstanceHealthResult{
		Asset:       dashboard.Asset,
		Target:      dashboard.Target,
		Health:      health,
		Status:      status,
		Abnormal:    abnormal,
		Warnings:    compactStrings(warnings),
		Prometheus:  dashboard.Target.Prometheus,
		MetricsURL:  firstNonEmpty(dashboard.Target.MetricsURL, dashboard.Target.ScrapeURL),
		HostMetrics: dashboard.Target.HostScrapeURL,
	}
	return &data, nil
}

func (s *DatabaseService) aiLoadMonitorDashboard(ctx context.Context, req dbmodel.DBAIToolRequest) (*dbmodel.MySQLMonitorDashboard, []string, error) {
	asset, err := s.assetDao.GetByID(req.AssetID)
	if err != nil {
		return nil, nil, err
	}
	query := dbmodel.MySQLMonitorQuery{
		Range: firstNonEmpty(req.Range, "1h"),
		Step:  firstNonEmpty(req.Step, "1m"),
	}
	warnings := []string{}
	var dashboard *dbmodel.MySQLMonitorDashboard
	switch asset.Type {
	case cmdbmodel.DatabaseTypeMySQL:
		dashboard, err = s.MySQLMonitorDashboard(ctx, req.AssetID, query)
	case cmdbmodel.DatabaseTypePostgreSQL:
		dashboard, err = s.PostgreSQLMonitorDashboard(ctx, req.AssetID, query)
	case cmdbmodel.DatabaseTypeRedis:
		dashboard, err = s.RedisMonitorDashboard(ctx, req.AssetID, query)
	default:
		return nil, nil, fmt.Errorf("database type %d does not support monitor dashboard", asset.Type)
	}
	if err != nil {
		warnings = append(warnings, err.Error())
		return nil, warnings, err
	}
	return dashboard, warnings, nil
}

func explainMySQLSQL(ctx context.Context, cfg ConnConfig, database, sqlText string) (*dbmodel.DBExplainSQLResult, error) {
	db, err := openMySQL(cfg, "")
	if err != nil {
		return nil, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if strings.TrimSpace(database) != "" {
		if err := validateIdentifier(database); err != nil {
			return nil, fmt.Errorf("invalid database: %w", err)
		}
		if _, err := conn.ExecContext(ctx, "USE "+quoteMySQLIdentifier(database)); err != nil {
			return nil, err
		}
	}
	rows, err := conn.QueryContext(ctx, "EXPLAIN "+sqlText)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plan, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	return &dbmodel.DBExplainSQLResult{
		SQL:      sqlText,
		Plan:     plan,
		Estimate: estimateMySQLReadOnlySQL(ctx, conn, sqlText),
	}, nil
}

func explainPostgresSQL(ctx context.Context, cfg ConnConfig, database, sqlText string) (*dbmodel.DBExplainSQLResult, error) {
	dbName := firstNonEmpty(database, cfg.DatabaseName)
	if err := validatePostgresIdentifier(dbName); err != nil {
		return nil, fmt.Errorf("invalid database: %w", err)
	}
	db, err := openPostgres(cfg, dbName)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, "EXPLAIN (FORMAT JSON, COSTS TRUE, VERBOSE FALSE) "+sqlText)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plan, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	return &dbmodel.DBExplainSQLResult{
		SQL:      sqlText,
		Plan:     plan,
		Estimate: estimatePostgresReadOnlySQL(ctx, db, sqlText),
	}, nil
}

func auditSafeAIToolRequest(req dbmodel.DBAIToolRequest) map[string]interface{} {
	return map[string]interface{}{
		"tool":       req.Tool,
		"assetId":    req.AssetID,
		"database":   req.Database,
		"schema":     req.Schema,
		"objectName": req.ObjectName,
		"objectType": req.ObjectType,
		"keyword":    req.Keyword,
		"sql":        truncateSQLForAudit(req.SQL),
		"range":      req.Range,
		"step":       req.Step,
		"page":       req.Page,
		"pageSize":   req.PageSize,
		"authId":     req.AuthID,
		"tailLines":  req.TailLines,
		"confirmed":  req.Confirmed.Bool(),
	}
}

func slowLogRangeToTime(value string) (string, string) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", ""
	}
	unit := value[len(value)-1:]
	numberText := strings.TrimSpace(value[:len(value)-1])
	var duration time.Duration
	switch unit {
	case "m":
		if n, err := parsePositiveDurationNumber(numberText); err == nil {
			duration = time.Duration(n) * time.Minute
		}
	case "h":
		if n, err := parsePositiveDurationNumber(numberText); err == nil {
			duration = time.Duration(n) * time.Hour
		}
	case "d":
		if n, err := parsePositiveDurationNumber(numberText); err == nil {
			duration = time.Duration(n) * 24 * time.Hour
		}
	}
	if duration <= 0 {
		return "", ""
	}
	end := time.Now()
	begin := end.Add(-duration)
	return begin.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05")
}

func parsePositiveDurationNumber(value string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid duration number")
	}
	if n > 365*24*60 {
		n = 365 * 24 * 60
	}
	return n, nil
}

func aiToolRowsAffected(result *dbmodel.DBAIToolResult) int64 {
	if result == nil || result.Data == nil {
		return 0
	}
	if data, ok := result.Data.(*dbmodel.SQLExecuteResult); ok && data != nil {
		return int64(len(data.Rows))
	}
	if data, ok := result.Data.(dbmodel.SQLExecuteResult); ok {
		return int64(len(data.Rows))
	}
	return 0
}

func truncateSQLForAudit(sqlText string) string {
	sqlText = strings.TrimSpace(sqlText)
	if len(sqlText) <= 2000 {
		return sqlText
	}
	return sqlText[:2000] + "...(truncated)"
}

var (
	phonePattern   = regexp.MustCompile(`^1[3-9]\d{9}$`)
	emailPattern   = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)
	idCardPattern  = regexp.MustCompile(`^\d{15}$|^\d{17}[\dXx]$`)
	tokenLikeRegex = regexp.MustCompile(`(?i)(bearer\s+)?[a-z0-9_\-]{24,}`)
)

func redactSQLRows(columns []dbmodel.ColumnInfo, rows []map[string]interface{}) []map[string]interface{} {
	if len(rows) == 0 {
		return rows
	}
	columnNames := map[string]struct{}{}
	for _, column := range columns {
		columnNames[column.Name] = struct{}{}
	}
	result := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		item := make(map[string]interface{}, len(row))
		for key, value := range row {
			if _, ok := columnNames[key]; !ok && len(columnNames) > 0 {
				item[key] = value
				continue
			}
			item[key] = redactSQLValue(key, value)
		}
		result = append(result, item)
	}
	return result
}

func redactSQLValue(column string, value interface{}) interface{} {
	if value == nil {
		return nil
	}
	text := fmt.Sprint(value)
	if shouldRedactColumn(column) || looksSensitiveValue(text) {
		return maskSensitiveText(text)
	}
	return value
}

func shouldRedactColumn(column string) bool {
	name := strings.ToLower(strings.TrimSpace(column))
	for _, token := range []string{
		"password", "passwd", "pwd", "secret", "token", "access_key", "accesskey",
		"private_key", "apikey", "api_key", "credential", "salt", "phone", "mobile",
		"email", "id_card", "idcard", "identity", "身份证", "手机号", "邮箱",
	} {
		if strings.Contains(name, token) {
			return true
		}
	}
	return false
}

func looksSensitiveValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return phonePattern.MatchString(value) ||
		emailPattern.MatchString(value) ||
		idCardPattern.MatchString(value) ||
		tokenLikeRegex.MatchString(value)
}

func maskSensitiveText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if emailPattern.MatchString(value) {
		parts := strings.SplitN(value, "@", 2)
		return maskKeepEdges(parts[0], 1, 1) + "@" + parts[1]
	}
	if phonePattern.MatchString(value) {
		return value[:3] + "****" + value[len(value)-4:]
	}
	if idCardPattern.MatchString(value) {
		return value[:4] + "**********" + value[len(value)-4:]
	}
	if len(value) <= 8 {
		return "******"
	}
	return maskKeepEdges(value, 4, 4)
}

func maskKeepEdges(value string, left, right int) string {
	runes := []rune(value)
	if len(runes) <= left+right {
		return "******"
	}
	return string(runes[:left]) + "******" + string(runes[len(runes)-right:])
}

func summarizeExplainPlan(engine int, plan []map[string]interface{}, estimate dbmodel.SQLExecutionEstimate) dbmodel.DBExplainSummary {
	summary := dbmodel.DBExplainSummary{
		RiskLevel:       firstNonEmpty(estimate.RiskLevel, "unknown"),
		EstimatedRows:   estimate.EstimatedRows,
		ProblemTables:   []string{},
		UsedIndexes:     []string{},
		Suggestions:     []string{},
		RawPlanAnalyzed: len(plan) > 0,
	}
	switch engine {
	case cmdbmodel.DatabaseTypeMySQL:
		summarizeMySQLExplain(plan, &summary)
	case cmdbmodel.DatabaseTypePostgreSQL:
		summarizePostgresExplain(plan, &summary)
	}
	if summary.FullScan {
		summary.Suggestions = append(summary.Suggestions, "执行计划存在全表扫描，优先检查过滤条件和索引覆盖情况")
	}
	if !summary.UsesIndex && len(plan) > 0 {
		summary.Suggestions = append(summary.Suggestions, "执行计划未识别到索引使用，建议结合表结构检查 WHERE/JOIN 字段索引")
	}
	if summary.EstimatedRows >= 100000 && summary.RiskLevel == "low" {
		summary.RiskLevel = "medium"
	}
	if summary.EstimatedRows >= 1000000 {
		summary.RiskLevel = "high"
	}
	summary.ProblemTables = uniqueStrings(summary.ProblemTables)
	summary.UsedIndexes = uniqueStrings(summary.UsedIndexes)
	summary.Suggestions = uniqueStrings(summary.Suggestions)
	return summary
}

func summarizeMySQLExplain(plan []map[string]interface{}, summary *dbmodel.DBExplainSummary) {
	for _, row := range plan {
		table := fmt.Sprint(row["table"])
		accessType := strings.ToLower(fmt.Sprint(row["type"]))
		key := strings.TrimSpace(fmt.Sprint(row["key"]))
		rows := toInt64(row["rows"])
		if rows > summary.EstimatedRows {
			summary.EstimatedRows = rows
		}
		if key != "" && !strings.EqualFold(key, "<nil>") && !strings.EqualFold(key, "NULL") {
			summary.UsesIndex = true
			summary.UsedIndexes = append(summary.UsedIndexes, key)
		}
		if accessType == "all" {
			summary.FullScan = true
			if table != "" && !strings.EqualFold(table, "<nil>") {
				summary.ProblemTables = append(summary.ProblemTables, table)
			}
		}
	}
}

func summarizePostgresExplain(plan []map[string]interface{}, summary *dbmodel.DBExplainSummary) {
	for _, row := range plan {
		text := strings.ToLower(fmt.Sprint(row))
		if strings.Contains(text, "seq scan") {
			summary.FullScan = true
		}
		if strings.Contains(text, "index scan") || strings.Contains(text, "bitmap index scan") || strings.Contains(text, "index only scan") {
			summary.UsesIndex = true
		}
	}
	if summary.FullScan {
		summary.ProblemTables = append(summary.ProblemTables, "see raw postgres plan")
	}
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func nonEmptyMetricCards(cards []dbmodel.MonitorMetricCard) []dbmodel.MonitorMetricCard {
	result := make([]dbmodel.MonitorMetricCard, 0, len(cards))
	for _, card := range cards {
		if strings.TrimSpace(card.Error) != "" {
			continue
		}
		result = append(result, card)
	}
	return result
}

func abnormalMetricCards(cards []dbmodel.MonitorMetricCard) []dbmodel.MonitorMetricCard {
	result := make([]dbmodel.MonitorMetricCard, 0)
	for _, card := range cards {
		status := strings.ToLower(strings.TrimSpace(card.Status))
		if status == "" || status == "normal" || status == "success" || status == "ok" {
			continue
		}
		result = append(result, card)
	}
	return result
}

func compactStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
