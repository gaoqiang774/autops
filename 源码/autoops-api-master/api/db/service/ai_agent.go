package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	aiopsservice "dodevops-api/api/aiops/service"
	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"

	"github.com/gin-gonic/gin"
)

const dbAIAgentSystemPrompt = "You are a senior database operations AI agent. Answer in Chinese. Use only the provided evidence. Return one compact valid JSON object only."

func (s *DatabaseService) DBAIChat(ctx *gin.Context, req dbmodel.DBAIChatRequest) (*dbmodel.DBAIChatResult, error) {
	if req.AssetID == 0 {
		return nil, fmt.Errorf("assetId is required")
	}
	if strings.TrimSpace(req.Question) == "" && strings.TrimSpace(req.SQL) == "" {
		return nil, fmt.Errorf("question or sql is required")
	}
	req = enrichDBAIChatRequestContext(req)

	asset, err := s.assetDao.GetByID(req.AssetID)
	if err != nil {
		return nil, err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionAIChat, auditSafeDBAIChatRequest(req))
	recorder.SetObject(req.Database, req.Schema, req.ObjectName, firstNonEmpty(req.ObjectType, "ai_agent"))
	defer func() { recorder.Finish(err) }()

	intent := inferDBAIIntent(req, asset.Type)
	plan := buildDBAIToolPlan(req, asset.Type, intent)
	tools := toolNamesFromPlan(plan)
	evidence, toolWarnings, plan := s.collectDBAIAgentEvidence(ctx, req, asset.Type, plan)
	if len(evidence) == 0 {
		return nil, fmt.Errorf("no evidence collected for database ai agent")
	}

	missingEvidence := inferDBAIMissingEvidence(intent, evidence, toolWarnings)
	if report, ok := buildDBAISlowSQLMarkdown(evidence); ok && intent.Category == "slow_sql_analysis" {
		result := &dbmodel.DBAIChatResult{
			Answer:          report.answer,
			Status:          report.status,
			Summary:         report.summary,
			Intent:          intent,
			ToolPlan:        plan,
			Suggestions:     report.suggestions,
			Warnings:        compactStrings(toolWarnings),
			MissingEvidence: compactStrings(missingEvidence),
			Evidence:        evidence,
			ToolCalls:       tools,
			UsedLLM:         false,
		}
		if !req.IncludeRawEvidence {
			result.Evidence = stripDBAIEvidenceData(result.Evidence)
		}
		recorder.SetRowsAffected(int64(len(evidence)))
		return result, nil
	}

	provider, apiKey, err := aiopsservice.ResolveLLMProvider(req.ProviderID)
	if err != nil {
		return nil, err
	}
	prompt := buildDBAIAgentPrompt(req, asset, intent, plan, evidence, toolWarnings, missingEvidence)
	llmCtx, cancel := context.WithTimeout(contextFromGin(ctx), llmTimeout(provider.TimeoutSeconds))
	defer cancel()

	answer, err := aiopsservice.CallLLM(llmCtx, provider, apiKey, dbAIAgentSystemPrompt, prompt)
	if err != nil {
		return nil, err
	}

	result := parseDBAIAgentAnswer(answer)
	result.RawAnswer = answer
	result.UsedLLM = true
	result.ProviderID = provider.ID
	result.ProviderName = provider.Name
	result.ModelCode = provider.DefaultModel
	result.Intent = intent
	result.ToolPlan = plan
	result.Evidence = evidence
	result.ToolCalls = tools
	result.Warnings = compactStrings(append(result.Warnings, toolWarnings...))
	result.MissingEvidence = compactStrings(append(result.MissingEvidence, missingEvidence...))
	applyDBAIAgentStructuredAnswer(result, intent, evidence)
	if !req.IncludeRawEvidence {
		result.Evidence = stripDBAIEvidenceData(result.Evidence)
	}
	recorder.SetRowsAffected(int64(len(evidence)))
	return result, nil
}

func applyDBAIAgentStructuredAnswer(result *dbmodel.DBAIChatResult, intent dbmodel.DBAIIntent, evidence []dbmodel.DBAIToolEvidence) {
	if result == nil || intent.Category != "slow_sql_analysis" {
		return
	}
	report, ok := buildDBAISlowSQLMarkdown(evidence)
	if !ok {
		return
	}
	result.Answer = report.answer
	result.Summary = report.summary
	result.Status = report.status
	if len(result.Suggestions) == 0 {
		result.Suggestions = report.suggestions
	}
}

type dbAISlowSQLMarkdownReport struct {
	answer      string
	status      string
	summary     string
	suggestions []string
}

func buildDBAISlowSQLMarkdown(evidence []dbmodel.DBAIToolEvidence) (dbAISlowSQLMarkdownReport, bool) {
	var slowData dbmodel.DBSlowSQLTopResult
	var summary string
	var warnings []string
	found := false
	for _, item := range evidence {
		if item.Tool != dbmodel.DBAIToolGetSlowSQLTop {
			continue
		}
		summary = item.Summary
		warnings = item.Warnings
		switch data := item.Data.(type) {
		case dbmodel.DBSlowSQLTopResult:
			slowData = data
			found = true
		case *dbmodel.DBSlowSQLTopResult:
			if data != nil {
				slowData = *data
				found = true
			}
		case map[string]interface{}:
			slowData = dbSlowSQLTopResultFromMap(data)
			found = true
		}
		break
	}
	if !found {
		return dbAISlowSQLMarkdownReport{}, false
	}
	if !slowData.SourceConfigured {
		msg := firstNonEmpty(slowData.Message, "用户未配置慢日志路径，无法分析，请先配置")
		return dbAISlowSQLMarkdownReport{
			answer:  "## 慢 SQL 分析结论\n\n" + msg,
			status:  "unknown",
			summary: msg,
		}, true
	}
	if len(slowData.Items) == 0 {
		msg := firstNonEmpty(slowData.Message, "已分析最近 50MB 慢日志，未发现慢 SQL 记录")
		return dbAISlowSQLMarkdownReport{
			answer:  "## 慢 SQL 分析结论\n\n" + msg,
			status:  "healthy",
			summary: msg,
		}, true
	}

	var builder strings.Builder
	builder.WriteString("## 慢 SQL 分析结论\n\n")
	builder.WriteString("本次 Top 5 慢 SQL 均存在明显性能风险，优先关注最大耗时高、扫描行数大、返回数据量不可控的查询。\n\n")
	builder.WriteString("### Top 5 概览\n\n")
	for idx, item := range slowData.Items {
		builder.WriteString(fmt.Sprintf("%d. **%s** %s\n",
			idx+1,
			slowSQLRiskLevel(item),
			markdownInlineCode(shortSQLText(dbAIMapString(item, "fingerprint", "sampleSql"), 90)),
		))
		builder.WriteString(fmt.Sprintf("   - 次数：%d，最大耗时：%s 秒，平均耗时：%s 秒，总耗时：%s 秒，最大扫描行数：%d\n\n",
			dbAIMapInt(item, "count"),
			formatSlowFloat(dbAIMapFloat(item, "queryTimeMax")),
			formatSlowFloat(dbAIMapFloat(item, "queryTimeAvg")),
			formatSlowFloat(dbAIMapFloat(item, "queryTimeTotal")),
			dbAIMapInt64(item, "rowsExaminedMax", "rowsExaminedTotal"),
		))
	}

	builder.WriteString("\n### Top 详情\n\n")
	for idx, item := range slowData.Items {
		suggestion := dbAISlowLogSuggestionFromMap(item)
		title := shortSQLText(dbAIMapString(item, "fingerprint", "sampleSql"), 96)
		builder.WriteString(fmt.Sprintf("#### %d. %s\n\n", idx+1, markdownInlineCode(title)))
		builder.WriteString(fmt.Sprintf("- **风险等级**：%s\n", slowSQLRiskLevel(item)))
		builder.WriteString(fmt.Sprintf("- **关键指标**：次数 %d，最大耗时 %s 秒，平均耗时 %s 秒，总耗时 %s 秒，最大扫描行数 %d\n",
			dbAIMapInt(item, "count"),
			formatSlowFloat(dbAIMapFloat(item, "queryTimeMax")),
			formatSlowFloat(dbAIMapFloat(item, "queryTimeAvg")),
			formatSlowFloat(dbAIMapFloat(item, "queryTimeTotal")),
			dbAIMapInt64(item, "rowsExaminedMax", "rowsExaminedTotal"),
		))
		if database := dbAIMapString(item, "database"); database != "" {
			builder.WriteString(fmt.Sprintf("- **数据库**：%s\n", database))
		}
		if reason := firstNonEmpty(suggestion.SlowReason, slowSQLDefaultReason(item)); reason != "" {
			builder.WriteString(fmt.Sprintf("- **慢查询原因**：%s\n", reason))
		}
		if indexAdvice := suggestion.IndexOptimization; indexAdvice != "" {
			builder.WriteString(fmt.Sprintf("- **索引优化**：%s\n", indexAdvice))
		}
		if sqlAdvice := firstNonEmpty(suggestion.SQLOptimization, slowSQLDefaultAdvice(item)); sqlAdvice != "" {
			builder.WriteString(fmt.Sprintf("- **SQL 优化**：%s\n", sqlAdvice))
		}
		if effect := suggestion.ExpectedEffect; effect != "" {
			builder.WriteString(fmt.Sprintf("- **预期效果**：%s\n", effect))
		}
		if sample := dbAIMapString(item, "sampleSql", "fingerprint"); sample != "" {
			builder.WriteString("\n```sql\n")
			builder.WriteString(shortSQLText(sample, 700))
			builder.WriteString("\n```\n")
		}
		builder.WriteString("\n")
	}

	limit := minInt(len(slowData.Items), 3)
	suggestions := make([]string, 0, limit+2)
	for i := 0; i < limit; i++ {
		item := slowData.Items[i]
		suggestion := dbAISlowLogSuggestionFromMap(item)
		advice := firstNonEmpty(suggestion.IndexOptimization, suggestion.SQLOptimization, slowSQLDefaultAdvice(item), "结合 EXPLAIN 确认执行计划并减少扫描范围")
		line := fmt.Sprintf("优先处理 Top %d：%s，%s", i+1, shortSQLText(dbAIMapString(item, "fingerprint", "sampleSql"), 48), advice)
		suggestions = append(suggestions, line)
	}
	if len(warnings) > 0 {
		builder.WriteString("### 注意事项\n\n")
		for _, warning := range warnings {
			if strings.TrimSpace(warning) != "" {
				builder.WriteString("- ")
				builder.WriteString(strings.TrimSpace(warning))
				builder.WriteString("\n")
			}
		}
	}

	return dbAISlowSQLMarkdownReport{
		answer:      strings.TrimSpace(builder.String()),
		status:      "warning",
		summary:     firstNonEmpty(slowData.Message, summary),
		suggestions: suggestions,
	}, true
}

func dbSlowSQLTopResultFromMap(data map[string]interface{}) dbmodel.DBSlowSQLTopResult {
	result := dbmodel.DBSlowSQLTopResult{
		SourceConfigured: dbAIMapBool(data, "sourceConfigured"),
		Source:           dbAIMapString(data, "source"),
		Message:          dbAIMapString(data, "message"),
		Items:            []map[string]interface{}{},
	}
	if rawItems, ok := data["items"].([]interface{}); ok {
		for _, raw := range rawItems {
			if item, ok := raw.(map[string]interface{}); ok {
				result.Items = append(result.Items, item)
			}
		}
	}
	return result
}

func dbAIMapString(item map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return strings.TrimSpace(typed)
				}
			case fmt.Stringer:
				if strings.TrimSpace(typed.String()) != "" {
					return strings.TrimSpace(typed.String())
				}
			}
		}
	}
	return ""
}

func dbAIMapBool(item map[string]interface{}, key string) bool {
	if value, ok := item[key]; ok {
		if typed, ok := value.(bool); ok {
			return typed
		}
	}
	return false
}

func dbAIMapFloat(item map[string]interface{}, key string) float64 {
	if value, ok := item[key]; ok {
		switch typed := value.(type) {
		case float64:
			return typed
		case float32:
			return float64(typed)
		case int:
			return float64(typed)
		case int64:
			return float64(typed)
		case json.Number:
			parsed, _ := typed.Float64()
			return parsed
		}
	}
	return 0
}

func dbAIMapInt(item map[string]interface{}, key string) int {
	return int(dbAIMapInt64(item, key))
}

func dbAIMapInt64(item map[string]interface{}, keys ...string) int64 {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch typed := value.(type) {
			case int:
				return int64(typed)
			case int64:
				return typed
			case int32:
				return int64(typed)
			case float64:
				return int64(typed)
			case float32:
				return int64(typed)
			case json.Number:
				parsed, _ := typed.Int64()
				return parsed
			}
		}
	}
	return 0
}

func dbAISlowLogSuggestionFromMap(item map[string]interface{}) dbmodel.SlowLogAISuggestion {
	raw, ok := item["aiSuggestion"]
	if !ok || raw == nil {
		return dbmodel.SlowLogAISuggestion{}
	}
	switch typed := raw.(type) {
	case dbmodel.SlowLogAISuggestion:
		return typed
	case map[string]interface{}:
		return dbmodel.SlowLogAISuggestion{
			SlowReason:         dbAIMapString(typed, "slowReason"),
			SchemaOptimization: dbAIMapString(typed, "schemaOptimization"),
			IndexOptimization:  dbAIMapString(typed, "indexOptimization"),
			SQLOptimization:    dbAIMapString(typed, "sqlOptimization"),
			ExpectedEffect:     dbAIMapString(typed, "expectedEffect"),
		}
	}
	return dbmodel.SlowLogAISuggestion{}
}

func slowSQLRiskLevel(item map[string]interface{}) string {
	maxTime := dbAIMapFloat(item, "queryTimeMax")
	rows := dbAIMapInt64(item, "rowsExaminedMax", "rowsExaminedTotal")
	switch {
	case maxTime >= 60 || rows >= 10000000:
		return "严重"
	case maxTime >= 10 || rows >= 1000000:
		return "高"
	case maxTime >= 3 || rows >= 100000:
		return "中"
	default:
		return "低"
	}
}

func slowSQLDefaultReason(item map[string]interface{}) string {
	rows := dbAIMapInt64(item, "rowsExaminedMax", "rowsExaminedTotal")
	sample := strings.ToLower(dbAIMapString(item, "fingerprint", "sampleSql"))
	switch {
	case strings.Contains(sample, "select *"):
		return "查询使用 SELECT *，可能返回过多字段并放大 I/O 与网络开销。"
	case !strings.Contains(sample, " where ") && strings.HasPrefix(strings.TrimSpace(sample), "select"):
		return "查询缺少 WHERE 条件，存在全表扫描风险。"
	case rows >= 1000000:
		return "扫描行数较高，可能缺少有效索引或过滤条件选择性不足。"
	default:
		return "执行耗时较高，需要结合 EXPLAIN 进一步确认索引和扫描范围。"
	}
}

func slowSQLDefaultAdvice(item map[string]interface{}) string {
	sample := strings.ToLower(dbAIMapString(item, "fingerprint", "sampleSql"))
	switch {
	case strings.Contains(sample, "select *"):
		return "避免 SELECT *，只返回必要字段，并增加业务过滤条件或分页。"
	case strings.Contains(sample, " join "):
		return "检查 JOIN 条件和过滤字段索引，优先为关联字段建立合适的复合索引。"
	default:
		return "结合 WHERE、ORDER BY、GROUP BY 字段补充索引，并控制返回数据量。"
	}
}

func shortSQLText(value string, limit int) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func markdownTableText(value string) string {
	replacer := strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ")
	return replacer.Replace(strings.TrimSpace(value))
}

func markdownInlineCode(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "`", "'")
	if value == "" {
		return "`未知 SQL`"
	}
	return "`" + value + "`"
}

func inferDBAIIntent(req dbmodel.DBAIChatRequest, dbType int) dbmodel.DBAIIntent {
	text := strings.ToLower(strings.TrimSpace(req.Question + " " + req.SQL + " " + req.ObjectName))
	intentText := strings.ToLower(strings.TrimSpace(req.Question + " " + req.ObjectName))
	intent := dbmodel.DBAIIntent{
		Category:    "general",
		Description: "数据库通用问题",
		ReadOnly:    true,
		RiskLevel:   "low",
	}
	if containsWriteIntent(intentText) || (strings.TrimSpace(req.SQL) != "" && !isReadOnlySQL(req.SQL) && dbType != cmdbmodel.DatabaseTypeRedis) {
		return dbmodel.DBAIIntent{
			Category:    "change_request",
			Description: "数据库变更或高风险操作请求",
			ReadOnly:    false,
			RiskLevel:   "high",
			Reason:      "用户问题或 SQL 涉及写入、结构变更、删除、授权、执行变更等操作，AI-Agent 不直接执行",
		}
	}
	if containsAny(text, "健康", "异常", "故障", "监控", "大盘", "状态", "告警", "cpu", "内存", "磁盘", "io", "iops", "连接", "qps", "延迟", "瓶颈", "资源") {
		intent.Category = "health_diagnosis"
		intent.Description = "数据库实例健康和监控诊断"
		intent.NeedsMonitor = true
	}
	if containsAny(text, "慢sql", "慢 sql", "slow sql", "慢查询") {
		intent.Category = "slow_sql_analysis"
		intent.Description = "慢 SQL 和性能分析"
		intent.NeedsSQL = true
	}
	if strings.TrimSpace(req.ObjectName) != "" && containsAny(text, "表结构", "结构", "字段", "索引", "ddl", "schema", "建表") {
		intent.Category = "schema_analysis"
		intent.Description = "数据库对象结构分析"
		intent.NeedsSchema = true
	}
	if strings.TrimSpace(req.SQL) != "" && dbType != cmdbmodel.DatabaseTypeRedis && containsAny(text, "explain", "执行计划", "走索引", "全表扫描", "优化", "为什么慢", "慢") {
		intent.Category = "sql_performance"
		intent.Description = "SQL 执行计划和性能分析"
		intent.NeedsSQL = true
		intent.NeedsSchema = strings.TrimSpace(req.ObjectName) != ""
	}
	if strings.TrimSpace(req.SQL) != "" && containsAny(text, "执行", "查询结果", "查一下", "跑一下", "返回", "统计") && !containsAny(text, "不要执行", "只分析", "只解释") {
		intent.Category = "readonly_query"
		intent.Description = "只读查询与结果总结"
		intent.NeedsSQL = true
	}
	if dbType != cmdbmodel.DatabaseTypeRedis && isDBAIRowCountQuestion(req) {
		intent.Category = "readonly_query"
		intent.Description = "表行数统计查询"
		intent.NeedsSQL = true
	}
	if containsAny(text, "找", "搜索", "有哪些表", "哪个表", "字段在哪", "对象", "库表") {
		intent.Category = "object_search"
		intent.Description = "数据库对象搜索"
		intent.NeedsSchema = true
	}
	if intent.Category == "general" && strings.TrimSpace(req.SQL) != "" && dbType != cmdbmodel.DatabaseTypeRedis {
		intent.Category = "sql_performance"
		intent.Description = "SQL 执行计划和性能分析"
		intent.NeedsSQL = true
	}
	return intent
}

func enrichDBAIChatRequestContext(req dbmodel.DBAIChatRequest) dbmodel.DBAIChatRequest {
	text := strings.TrimSpace(req.Question)
	if text == "" {
		return req
	}
	if database := extractDBAIQuestionValue(text, []string{
		`(?i)([A-Za-z0-9_@$-]+)\s*(?:库|数据库)`,
		`(?i)(?:库|数据库|database|db)\s*[:：=]\s*([A-Za-z0-9_@$-]+)`,
		`(?i)(?:database|db)\s+([A-Za-z0-9_@$-]+)`,
	}); database != "" {
		req.Database = database
	}
	if objectName := extractDBAIQuestionValue(text, []string{
		`(?i)([A-Za-z0-9_@$-]+)\s*(?:表|表结构)`,
		`(?i)([A-Za-z0-9_@$-]+)\s*table(?:\b|$)`,
		`(?i)(?:表|表名|表结构|table)\s*[:：=]?\s*([A-Za-z0-9_@$-]+)`,
	}); objectName != "" {
		req.ObjectName = objectName
	}
	if strings.TrimSpace(req.ObjectType) == "" && strings.TrimSpace(req.ObjectName) != "" {
		req.ObjectType = dbmodel.ObjectTypeTable
	}
	return req
}

func extractDBAIQuestionValue(text string, patterns []string) string {
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(text)
		if len(matches) < 2 {
			continue
		}
		value := cleanDBAIQuestionIdentifier(matches[1])
		if value != "" && !isDBAIQuestionStopWord(value) {
			return value
		}
	}
	return ""
}

func cleanDBAIQuestionIdentifier(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`'\"，,。.；;：:（）()[]{}")
	return value
}

func isDBAIQuestionStopWord(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "这个", "当前", "数据库", "db", "database", "table", "schema":
		return true
	default:
		return false
	}
}

func buildDBAIToolPlan(req dbmodel.DBAIChatRequest, dbType int, intent dbmodel.DBAIIntent) []dbmodel.DBAIToolPlanItem {
	if len(req.Tools) > 0 {
		tools := normalizeDBAIToolNames(req.Tools)
		plan := make([]dbmodel.DBAIToolPlanItem, 0, len(tools))
		for _, tool := range tools {
			plan = append(plan, dbmodel.DBAIToolPlanItem{Tool: tool, Reason: "用户显式指定工具", Required: true})
		}
		return plan
	}

	text := strings.ToLower(strings.TrimSpace(req.Question + " " + req.SQL + " " + req.ObjectName))
	plan := make([]dbmodel.DBAIToolPlanItem, 0)
	add := func(tool, reason string, required bool) {
		plan = append(plan, dbmodel.DBAIToolPlanItem{Tool: tool, Reason: reason, Required: required})
	}

	if !intent.ReadOnly {
		add(dbmodel.DBAIToolGetInstanceHealth, "变更请求前先查看实例健康和风险背景", false)
		return dedupeDBAIToolPlan(plan)
	}
	if intent.NeedsMonitor || containsAny(text, "健康", "异常", "故障", "监控", "大盘", "状态", "告警", "cpu", "内存", "磁盘", "io", "iops", "连接", "qps", "延迟", "瓶颈", "资源") {
		add(dbmodel.DBAIToolGetInstanceHealth, "判断实例总体健康状态", true)
		add(dbmodel.DBAIToolGetMonitorSnapshot, "获取监控指标快照作为诊断证据", true)
	}
	if containsAny(text, "慢sql", "慢 sql", "slow sql", "慢查询") {
		add(dbmodel.DBAIToolGetSlowSQLTop, "用户询问慢 SQL，需要获取慢 SQL Top", false)
	}
	if strings.TrimSpace(req.ObjectName) != "" && containsAny(text, "表结构", "结构", "字段", "索引", "ddl", "schema", "建表") {
		add(dbmodel.DBAIToolGetTableSchema, "用户询问表结构、字段或索引", true)
	}
	if strings.TrimSpace(req.SQL) != "" && dbType != cmdbmodel.DatabaseTypeRedis && containsAny(text, "explain", "执行计划", "走索引", "全表扫描", "优化", "为什么慢", "慢") {
		if strings.TrimSpace(req.ObjectName) != "" {
			add(dbmodel.DBAIToolGetTableSchema, "结合表结构判断 SQL 索引风险", false)
		}
		add(dbmodel.DBAIToolExplainSQL, "用户询问 SQL 性能或执行计划", true)
	}
	if strings.TrimSpace(req.SQL) != "" && containsAny(text, "执行", "查询结果", "查一下", "跑一下", "返回", "统计") && !containsAny(text, "不要执行", "只分析", "只解释") {
		add(dbmodel.DBAIToolExecuteReadonlySQL, "用户要求执行明确的只读查询并总结结果", true)
	}
	if dbType != cmdbmodel.DatabaseTypeRedis && isDBAIRowCountQuestion(req) {
		add(dbmodel.DBAIToolExecuteReadonlySQL, "用户询问表数据量，执行只读 COUNT 统计", true)
	}
	if containsAny(text, "找", "搜索", "有哪些表", "哪个表", "字段在哪", "对象", "库表") {
		add(dbmodel.DBAIToolSearchObjects, "用户询问对象或字段位置", true)
	}
	if len(plan) == 0 {
		if strings.TrimSpace(req.SQL) != "" && dbType != cmdbmodel.DatabaseTypeRedis {
			add(dbmodel.DBAIToolExplainSQL, "默认对 SQL 进行只读执行计划分析", true)
		} else {
			add(dbmodel.DBAIToolGetInstanceHealth, "默认先检查实例健康", true)
			add(dbmodel.DBAIToolGetMonitorSnapshot, "默认收集监控快照", false)
		}
	}
	return dedupeDBAIToolPlan(plan)
}

func (s *DatabaseService) collectDBAIAgentEvidence(ctx *gin.Context, req dbmodel.DBAIChatRequest, dbType int, plan []dbmodel.DBAIToolPlanItem) ([]dbmodel.DBAIToolEvidence, []string, []dbmodel.DBAIToolPlanItem) {
	evidence := make([]dbmodel.DBAIToolEvidence, 0, len(plan))
	warnings := make([]string, 0)
	for i, item := range plan {
		tool := item.Tool
		toolReq := dbmodel.DBAIToolRequest{
			Tool:       tool,
			AssetID:    req.AssetID,
			Database:   req.Database,
			Schema:     req.Schema,
			ObjectName: req.ObjectName,
			ObjectType: req.ObjectType,
			Keyword:    dbAIAgentToolKeyword(tool, req),
			SQL:        dbAIAgentToolSQL(tool, req, dbType),
			Range:      firstNonEmpty(req.Range, "6h"),
			Step:       firstNonEmpty(req.Step, "1m"),
			Page:       1,
			PageSize:   20,
		}
		if tool == dbmodel.DBAIToolExecuteReadonlySQL && isDBAIRowCountQuestion(req) && strings.TrimSpace(req.SQL) == "" {
			toolReq.Confirmed = dbmodel.SQLConfirm(true)
		}
		if skipReason := skipDBAIAgentTool(toolReq); skipReason != "" {
			warnings = append(warnings, skipReason)
			plan[i].Skipped = true
			plan[i].SkipReason = skipReason
			continue
		}
		result, err := s.RunAITool(ctx, toolReq)
		if err != nil {
			warnings = append(warnings, tool+": "+err.Error())
			evidence = append(evidence, dbmodel.DBAIToolEvidence{
				Tool:      tool,
				Supported: false,
				Summary:   err.Error(),
				Warnings:  []string{err.Error()},
			})
			continue
		}
		evidence = append(evidence, dbmodel.DBAIToolEvidence{
			Tool:      result.Tool,
			Supported: result.Supported,
			Summary:   result.Summary,
			Warnings:  result.Warnings,
			Data:      result.Data,
		})
		warnings = append(warnings, result.Warnings...)
	}
	return evidence, compactStrings(warnings), plan
}

func dbAIAgentToolKeyword(tool string, req dbmodel.DBAIChatRequest) string {
	switch tool {
	case dbmodel.DBAIToolGetSlowSQLTop:
		return ""
	case dbmodel.DBAIToolSearchObjects:
		return firstNonEmpty(req.ObjectName, req.Question)
	default:
		return req.ObjectName
	}
}

func dbAIAgentToolSQL(tool string, req dbmodel.DBAIChatRequest, dbType int) string {
	if strings.TrimSpace(req.SQL) != "" {
		return req.SQL
	}
	if tool == dbmodel.DBAIToolExecuteReadonlySQL && dbType != cmdbmodel.DatabaseTypeRedis && isDBAIRowCountQuestion(req) {
		return buildDBAIRowCountSQL(req, dbType)
	}
	return ""
}

func skipDBAIAgentTool(req dbmodel.DBAIToolRequest) string {
	switch req.Tool {
	case dbmodel.DBAIToolGetTableSchema:
		if strings.TrimSpace(req.Database) == "" || strings.TrimSpace(req.ObjectName) == "" {
			return "db_get_table_schema skipped: database and objectName are required"
		}
	case dbmodel.DBAIToolExplainSQL, dbmodel.DBAIToolExecuteReadonlySQL:
		if strings.TrimSpace(req.SQL) == "" {
			return req.Tool + " skipped: sql is required"
		}
	}
	return ""
}

func isDBAIRowCountQuestion(req dbmodel.DBAIChatRequest) bool {
	text := strings.ToLower(strings.TrimSpace(req.Question + " " + req.ObjectName))
	if strings.TrimSpace(req.Database) == "" || strings.TrimSpace(req.ObjectName) == "" {
		return false
	}
	return containsAny(text,
		"多少条", "多少行", "几条", "几行", "条数据", "行数据", "数据量", "记录数", "总数",
		"count", "row count", "rows", "records",
	)
}

func buildDBAIRowCountSQL(req dbmodel.DBAIChatRequest, dbType int) string {
	table := quoteDBAIIdentifier(req.ObjectName, dbType)
	if strings.TrimSpace(req.Schema) != "" && dbType == cmdbmodel.DatabaseTypePostgreSQL {
		table = quoteDBAIIdentifier(req.Schema, dbType) + "." + table
	}
	return fmt.Sprintf("SELECT COUNT(*) AS total_count FROM %s", table)
}

func quoteDBAIIdentifier(value string, dbType int) string {
	value = strings.TrimSpace(value)
	switch dbType {
	case cmdbmodel.DatabaseTypePostgreSQL:
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	default:
		return "`" + strings.ReplaceAll(value, "`", "``") + "`"
	}
}

func buildDBAIAgentPrompt(req dbmodel.DBAIChatRequest, asset *cmdbmodel.CmdbSQL, intent dbmodel.DBAIIntent, plan []dbmodel.DBAIToolPlanItem, evidence []dbmodel.DBAIToolEvidence, warnings []string, missingEvidence []string) string {
	promptEvidence := evidence
	if !req.IncludeRawEvidence {
		promptEvidence = compactDBAIEvidenceForPrompt(evidence)
	}
	data, _ := json.Marshal(map[string]interface{}{
		"asset": map[string]interface{}{
			"id":           asset.ID,
			"name":         asset.Name,
			"instanceName": asset.InstanceName,
			"type":         databaseTypeName(asset.Type),
			"environment":  asset.Environment,
			"hostIp":       asset.HostIP,
			"endpoint":     asset.Endpoint,
			"port":         asset.Port,
			"business":     asset.BusinessSystem,
			"owner":        asset.Owner,
		},
		"context": map[string]interface{}{
			"database":   req.Database,
			"schema":     req.Schema,
			"objectName": req.ObjectName,
			"objectType": req.ObjectType,
			"range":      firstNonEmpty(req.Range, "6h"),
			"step":       firstNonEmpty(req.Step, "1m"),
			"sql":        truncateSQLForAudit(req.SQL),
		},
		"question":        req.Question,
		"intent":          intent,
		"toolPlan":        plan,
		"evidence":        promptEvidence,
		"warnings":        warnings,
		"missingEvidence": missingEvidence,
	})

	var builder strings.Builder
	builder.WriteString("你是数据库运维 AI-Agent。请先理解用户意图，再基于工具证据回答，不能编造未提供的数据。\n")
	builder.WriteString("回答要求：\n")
	builder.WriteString("1. 只根据 evidence 作结论；证据不足要明确说明还缺什么。\n")
	builder.WriteString("2. 如果发现风险，给出可能原因、排查顺序和建议动作。\n")
	builder.WriteString("3. 如果 intent.readOnly=false，必须明确拒绝直接执行变更，只能建议走 SQL 审核平台或人工确认流程。\n")
	builder.WriteString("4. 返回 JSON，不要包裹 Markdown 代码块，不要在 JSON 前后输出任何解释文字。\n")
	builder.WriteString("5. answer 字段必须使用 Markdown 格式组织内容，推荐使用二级标题、编号列表、加粗字段和 SQL 代码块；不要把整个 JSON 当作 answer 文本。\n")
	builder.WriteString("6. 控制输出长度，answer 优先给结论、Top 项和行动建议；suggestions/warnings/missingEvidence 只放短句，避免重复 answer 全文。\n")
	builder.WriteString("JSON 格式：{\"answer\":\"Markdown 正文\",\"status\":\"healthy|warning|critical|unknown\",\"summary\":\"一句话摘要\",\"suggestions\":[],\"warnings\":[],\"missingEvidence\":[]}\n")
	if strings.TrimSpace(req.CustomPrompt) != "" {
		builder.WriteString("\n额外要求：\n")
		builder.WriteString(req.CustomPrompt)
		builder.WriteString("\n")
	}
	builder.WriteString("\n输入 JSON：\n")
	builder.WriteString(truncateForDBAIPrompt(string(data), 30000))
	return builder.String()
}

func parseDBAIAgentAnswer(answer string) *dbmodel.DBAIChatResult {
	raw := strings.TrimSpace(answer)
	cleaned := extractDBAIJSONPayload(raw)
	result := &dbmodel.DBAIChatResult{
		Answer:      raw,
		Status:      "unknown",
		Summary:     raw,
		Suggestions: []string{},
		Warnings:    []string{},
	}
	var parsed struct {
		Answer          string   `json:"answer"`
		Status          string   `json:"status"`
		Summary         string   `json:"summary"`
		Suggestions     []string `json:"suggestions"`
		Warnings        []string `json:"warnings"`
		MissingEvidence []string `json:"missingEvidence"`
	}
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		if fallback := parseDBAIAgentAnswerFields(cleaned); fallback != nil {
			return fallback
		}
		if cleaned != raw {
			if fallback := parseDBAIAgentAnswerFields(raw); fallback != nil {
				return fallback
			}
		}
		return result
	}
	result.Answer = firstNonEmpty(parsed.Answer, parsed.Summary, raw)
	result.Status = normalizeDBAIStatus(parsed.Status)
	result.Summary = firstNonEmpty(parsed.Summary, parsed.Answer)
	result.Suggestions = parsed.Suggestions
	if result.Suggestions == nil {
		result.Suggestions = []string{}
	}
	result.Warnings = parsed.Warnings
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	result.MissingEvidence = parsed.MissingEvidence
	if result.MissingEvidence == nil {
		result.MissingEvidence = []string{}
	}
	return result
}

func parseDBAIAgentAnswerFields(value string) *dbmodel.DBAIChatResult {
	answer := extractDBAIJSONStringField(value, "answer")
	if strings.TrimSpace(answer) == "" {
		return nil
	}
	summary := extractDBAIJSONStringField(value, "summary")
	status := extractDBAIJSONStringField(value, "status")
	warnings := extractDBAIJSONStringArrayField(value, "warnings")
	missingEvidence := extractDBAIJSONStringArrayField(value, "missingEvidence")
	suggestions := extractDBAIJSONStringArrayField(value, "suggestions")
	return &dbmodel.DBAIChatResult{
		Answer:          answer,
		Status:          normalizeDBAIStatus(status),
		Summary:         firstNonEmpty(summary, firstMarkdownLine(answer)),
		Suggestions:     suggestions,
		Warnings:        warnings,
		MissingEvidence: missingEvidence,
	}
}

func extractDBAIJSONStringField(value, field string) string {
	idx := strings.Index(value, `"`+field+`"`)
	if idx < 0 {
		return ""
	}
	rest := value[idx+len(field)+2:]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return ""
	}
	rest = strings.TrimLeft(rest[colon+1:], " \r\n\t")
	if !strings.HasPrefix(rest, `"`) {
		return ""
	}
	var builder strings.Builder
	escaped := false
	for i := 1; i < len(rest); i++ {
		ch := rest[i]
		if escaped {
			builder.WriteByte('\\')
			builder.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			quoted := `"` + builder.String() + `"`
			var decoded string
			if err := json.Unmarshal([]byte(quoted), &decoded); err == nil {
				return strings.TrimSpace(decoded)
			}
			return strings.TrimSpace(builder.String())
		}
		builder.WriteByte(ch)
	}
	return ""
}

func extractDBAIJSONStringArrayField(value, field string) []string {
	idx := strings.Index(value, `"`+field+`"`)
	if idx < 0 {
		return []string{}
	}
	rest := value[idx+len(field)+2:]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return []string{}
	}
	rest = strings.TrimLeft(rest[colon+1:], " \r\n\t")
	if !strings.HasPrefix(rest, "[") {
		return []string{}
	}
	end := findDBAIJSONArrayEnd(rest)
	if end <= 0 {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal([]byte(rest[:end]), &values); err != nil {
		return []string{}
	}
	if values == nil {
		return []string{}
	}
	return values
}

func findDBAIJSONArrayEnd(value string) int {
	inString := false
	escaped := false
	depth := 0
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

func firstMarkdownLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "#*-0123456789. "))
		if line != "" {
			return line
		}
	}
	return ""
}

func extractDBAIJSONPayload(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return value
	}
	if strings.HasPrefix(value, "```") {
		lines := strings.Split(value, "\n")
		if len(lines) >= 2 {
			value = strings.Join(lines[1:], "\n")
			value = strings.TrimSpace(strings.TrimSuffix(value, "```"))
		}
	}
	if json.Valid([]byte(value)) {
		return value
	}
	if fenced := extractDBAIFencedJSON(value); fenced != "" && json.Valid([]byte(fenced)) {
		return fenced
	}
	if object := extractDBAIJSONObject(value); object != "" && json.Valid([]byte(object)) {
		return object
	}
	return value
}

func extractDBAIFencedJSON(value string) string {
	start := strings.Index(value, "```json")
	if start < 0 {
		start = strings.Index(value, "```")
	}
	if start < 0 {
		return ""
	}
	bodyStart := strings.Index(value[start:], "\n")
	if bodyStart < 0 {
		return ""
	}
	bodyStart += start + 1
	end := strings.Index(value[bodyStart:], "```")
	if end < 0 {
		return strings.TrimSpace(value[bodyStart:])
	}
	return strings.TrimSpace(value[bodyStart : bodyStart+end])
}

func extractDBAIJSONObject(value string) string {
	start := strings.IndexByte(value, '{')
	if start < 0 {
		return ""
	}
	inString := false
	escaped := false
	depth := 0
	for i := start; i < len(value); i++ {
		ch := value[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(value[start : i+1])
			}
		}
	}
	return ""
}

func auditSafeDBAIChatRequest(req dbmodel.DBAIChatRequest) map[string]interface{} {
	return map[string]interface{}{
		"assetId":            req.AssetID,
		"database":           req.Database,
		"schema":             req.Schema,
		"objectName":         req.ObjectName,
		"objectType":         req.ObjectType,
		"question":           truncateForDBAIPrompt(req.Question, 2000),
		"sql":                truncateSQLForAudit(req.SQL),
		"range":              req.Range,
		"step":               req.Step,
		"providerId":         req.ProviderID,
		"tools":              req.Tools,
		"includeRawEvidence": req.IncludeRawEvidence,
	}
}

func normalizeDBAIToolNames(values []string) []string {
	allowed := map[string]struct{}{
		dbmodel.DBAIToolSearchObjects:      {},
		dbmodel.DBAIToolGetTableSchema:     {},
		dbmodel.DBAIToolExecuteReadonlySQL: {},
		dbmodel.DBAIToolExplainSQL:         {},
		dbmodel.DBAIToolGetMonitorSnapshot: {},
		dbmodel.DBAIToolGetSlowSQLTop:      {},
		dbmodel.DBAIToolGetInstanceHealth:  {},
	}
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if _, ok := allowed[value]; !ok {
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

func dedupeDBAIToolPlan(plan []dbmodel.DBAIToolPlanItem) []dbmodel.DBAIToolPlanItem {
	seen := map[string]struct{}{}
	result := make([]dbmodel.DBAIToolPlanItem, 0, len(plan))
	for _, item := range plan {
		item.Tool = strings.TrimSpace(item.Tool)
		if item.Tool == "" {
			continue
		}
		if _, ok := seen[item.Tool]; ok {
			continue
		}
		seen[item.Tool] = struct{}{}
		result = append(result, item)
	}
	return result
}

func toolNamesFromPlan(plan []dbmodel.DBAIToolPlanItem) []string {
	names := make([]string, 0, len(plan))
	for _, item := range plan {
		if strings.TrimSpace(item.Tool) == "" {
			continue
		}
		names = append(names, item.Tool)
	}
	return names
}

func inferDBAIMissingEvidence(intent dbmodel.DBAIIntent, evidence []dbmodel.DBAIToolEvidence, warnings []string) []string {
	missing := make([]string, 0)
	seenTools := map[string]dbmodel.DBAIToolEvidence{}
	for _, item := range evidence {
		seenTools[item.Tool] = item
		if !item.Supported {
			missing = append(missing, item.Tool+" unsupported: "+item.Summary)
		}
	}
	if intent.NeedsMonitor {
		if _, ok := seenTools[dbmodel.DBAIToolGetInstanceHealth]; !ok {
			missing = append(missing, "缺少实例健康 evidence")
		}
		if _, ok := seenTools[dbmodel.DBAIToolGetMonitorSnapshot]; !ok {
			missing = append(missing, "缺少监控快照 evidence")
		}
	}
	if intent.NeedsSchema {
		if _, ok := seenTools[dbmodel.DBAIToolGetTableSchema]; !ok {
			missing = append(missing, "缺少表结构或对象元数据 evidence")
		}
	}
	if intent.NeedsSQL {
		if _, ok := seenTools[dbmodel.DBAIToolExplainSQL]; !ok {
			if _, ok := seenTools[dbmodel.DBAIToolExecuteReadonlySQL]; !ok {
				if _, ok := seenTools[dbmodel.DBAIToolGetSlowSQLTop]; !ok {
					missing = append(missing, "缺少 SQL 执行计划、只读查询结果或慢 SQL evidence")
				}
			}
		}
	}
	for _, warning := range warnings {
		if strings.Contains(strings.ToLower(warning), "skipped") ||
			strings.Contains(strings.ToLower(warning), "not configured") ||
			strings.Contains(strings.ToLower(warning), "empty result") {
			missing = append(missing, warning)
		}
	}
	return compactStrings(missing)
}

func compactDBAIEvidenceForPrompt(evidence []dbmodel.DBAIToolEvidence) []dbmodel.DBAIToolEvidence {
	result := make([]dbmodel.DBAIToolEvidence, 0, len(evidence))
	for _, item := range evidence {
		compact := item
		switch data := item.Data.(type) {
		case dbmodel.DBMonitorSnapshot:
			compact.Data = map[string]interface{}{
				"asset":    data.Asset,
				"target":   data.Target,
				"overview": data.Overview,
				"range":    data.Range,
			}
		case dbmodel.DBInstanceHealthResult:
			compact.Data = data
		case dbmodel.DBExplainSQLResult:
			compact.Data = map[string]interface{}{
				"sql":      data.SQL,
				"engine":   data.Engine,
				"summary":  data.Summary,
				"estimate": data.Estimate,
			}
		case dbmodel.DBTableSchemaResult:
			compact.Data = map[string]interface{}{
				"columns": data.Columns,
				"indexes": data.Indexes,
			}
		case *dbmodel.SQLExecuteResult:
			if data != nil {
				compact.Data = map[string]interface{}{
					"columns":        data.Columns,
					"rows":           data.Rows,
					"total":          data.Total,
					"durationMs":     data.DurationMS,
					"readOnly":       data.ReadOnly,
					"requireConfirm": data.RequireConfirm,
					"estimate":       data.Estimate,
				}
			}
		}
		result = append(result, compact)
	}
	return result
}

func stripDBAIEvidenceData(evidence []dbmodel.DBAIToolEvidence) []dbmodel.DBAIToolEvidence {
	result := make([]dbmodel.DBAIToolEvidence, 0, len(evidence))
	for _, item := range evidence {
		item.Data = nil
		result = append(result, item)
	}
	return result
}

func normalizeDBAIStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "healthy", "warning", "critical":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}

func containsAny(text string, needles ...string) bool {
	text = strings.ToLower(text)
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func containsWriteIntent(text string) bool {
	return containsAny(text,
		"插入", "新增", "更新", "删除", "清空", "修改", "变更", "修复", "执行变更",
		"建索引", "创建索引", "删索引", "删除索引", "建表", "删表", "授权", "回收权限",
		"kill", "kill session", "kill connection", "truncate", "drop", "alter", "insert",
		"update", "delete", "create", "grant", "revoke", "rename", "replace",
	)
}

func truncateForDBAIPrompt(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
}

func dbAIAgentNow() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
