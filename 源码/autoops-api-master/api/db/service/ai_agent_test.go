package service

import (
	"testing"

	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"
)

func TestEnrichDBAIChatRequestContextExtractsDatabaseAndTable(t *testing.T) {
	req := enrichDBAIChatRequestContext(dbmodel.DBAIChatRequest{
		AssetID:  1,
		Question: "分析这个数据库的 devops库，cmdb_host表的表结构",
	})

	if req.Database != "devops" {
		t.Fatalf("database = %q, want devops", req.Database)
	}
	if req.ObjectName != "cmdb_host" {
		t.Fatalf("objectName = %q, want cmdb_host", req.ObjectName)
	}
	if req.ObjectType != dbmodel.ObjectTypeTable {
		t.Fatalf("objectType = %q, want %q", req.ObjectType, dbmodel.ObjectTypeTable)
	}

	intent := inferDBAIIntent(req, cmdbmodel.DatabaseTypeMySQL)
	if !intent.NeedsSchema || intent.Category != "schema_analysis" {
		t.Fatalf("intent = %#v, want schema_analysis with NeedsSchema", intent)
	}

	plan := buildDBAIToolPlan(req, cmdbmodel.DatabaseTypeMySQL, intent)
	found := false
	for _, item := range plan {
		if item.Tool == dbmodel.DBAIToolGetTableSchema {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("tool plan = %#v, want db_get_table_schema", plan)
	}
}

func TestEnrichDBAIChatRequestContextKeepsExistingDatabaseWhenQuestionHasNoExplicitDatabase(t *testing.T) {
	req := enrichDBAIChatRequestContext(dbmodel.DBAIChatRequest{
		AssetID:    1,
		Database:   "devops",
		Question:   "分析 cmdb_host 表结构",
		ObjectType: "",
	})

	if req.Database != "devops" {
		t.Fatalf("database = %q, want devops", req.Database)
	}
	if req.ObjectName != "cmdb_host" {
		t.Fatalf("objectName = %q, want cmdb_host", req.ObjectName)
	}
}

func TestDBAIAgentToolKeywordDoesNotFilterSlowSQLByQuestion(t *testing.T) {
	req := dbmodel.DBAIChatRequest{
		AssetID:  1,
		Question: "查看最近 6 小时慢 SQL Top，并说明哪些 SQL 最需要关注。",
	}

	if keyword := dbAIAgentToolKeyword(dbmodel.DBAIToolGetSlowSQLTop, req); keyword != "" {
		t.Fatalf("slow sql keyword = %q, want empty", keyword)
	}
}

func TestDBAIAgentRowCountQuestionBuildsReadonlyCountSQL(t *testing.T) {
	req := enrichDBAIChatRequestContext(dbmodel.DBAIChatRequest{
		AssetID:  1,
		Question: "帮忙看看 devops库 cmdb_host表有多少条数据",
	})

	if req.Database != "devops" || req.ObjectName != "cmdb_host" {
		t.Fatalf("context = database %q object %q, want devops cmdb_host", req.Database, req.ObjectName)
	}
	if !isDBAIRowCountQuestion(req) {
		t.Fatalf("row count question not detected")
	}
	intent := inferDBAIIntent(req, cmdbmodel.DatabaseTypeMySQL)
	if intent.Category != "readonly_query" || !intent.NeedsSQL {
		t.Fatalf("intent = %#v, want readonly_query with NeedsSQL", intent)
	}
	plan := buildDBAIToolPlan(req, cmdbmodel.DatabaseTypeMySQL, intent)
	found := false
	for _, item := range plan {
		if item.Tool == dbmodel.DBAIToolExecuteReadonlySQL {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("tool plan = %#v, want db_execute_readonly_sql", plan)
	}
	sqlText := dbAIAgentToolSQL(dbmodel.DBAIToolExecuteReadonlySQL, req, cmdbmodel.DatabaseTypeMySQL)
	if sqlText != "SELECT COUNT(*) AS total_count FROM `cmdb_host`" {
		t.Fatalf("sql = %q", sqlText)
	}
}

func TestParseDBAIAgentAnswerExtractsJSONFromMixedText(t *testing.T) {
	result := parseDBAIAgentAnswer(`结论
{"answer":"## 慢 SQL Top 5\n\n1. **SQL 1**：需要关注","status":"warning","summary":"发现慢 SQL","suggestions":["优化索引"],"warnings":[],"missingEvidence":[]}`)

	if result.Answer != "## 慢 SQL Top 5\n\n1. **SQL 1**：需要关注" {
		t.Fatalf("answer = %q", result.Answer)
	}
	if result.Status != "warning" {
		t.Fatalf("status = %q, want warning", result.Status)
	}
	if result.Summary != "发现慢 SQL" {
		t.Fatalf("summary = %q", result.Summary)
	}
}

func TestParseDBAIAgentAnswerExtractsFencedJSON(t *testing.T) {
	result := parseDBAIAgentAnswer("```json\n{\"answer\":\"## 结论\",\"status\":\"healthy\",\"summary\":\"正常\"}\n```")

	if result.Answer != "## 结论" {
		t.Fatalf("answer = %q", result.Answer)
	}
	if result.Status != "healthy" {
		t.Fatalf("status = %q, want healthy", result.Status)
	}
}

func TestParseDBAIAgentAnswerFallsBackToAnswerFieldWhenJSONIsBroken(t *testing.T) {
	result := parseDBAIAgentAnswer("```json { \"answer\": \"## 慢 SQL Top 分析报告\\n\\n### 1. **SQL**\\n- **建议**: 建索引\", \"status\": \"warning\", \"summary\": \"发现慢 SQL\", \"suggestions\": [\"优化 CUSTOMER_COMMENT �")

	if result.Answer != "## 慢 SQL Top 分析报告\n\n### 1. **SQL**\n- **建议**: 建索引" {
		t.Fatalf("answer = %q", result.Answer)
	}
	if result.Status != "warning" {
		t.Fatalf("status = %q, want warning", result.Status)
	}
	if result.Summary != "发现慢 SQL" {
		t.Fatalf("summary = %q", result.Summary)
	}
}
