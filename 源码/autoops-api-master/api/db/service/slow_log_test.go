package service

import (
	"strings"
	"testing"

	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"
)

func TestAnalyzeMySQLSlowLogLinesAggregatesByFingerprint(t *testing.T) {
	text := `
# Time: 2026-05-25T18:10:00.000000Z
# User@Host: app[app] @  [192.168.1.10]
# Query_time: 2.000000  Lock_time: 0.010000  Rows_sent: 1  Rows_examined: 100
use shop;
SET timestamp=1779703800;
select * from orders where id = 1;
# Time: 2026-05-25T18:11:00.000000Z
# User@Host: app[app] @  [192.168.1.10]
# Query_time: 4.000000  Lock_time: 0.020000  Rows_sent: 2  Rows_examined: 300
use shop;
SET timestamp=1779703860;
select * from orders where id = 2;
# Time: 2026-05-25T18:12:00.000000Z
# User@Host: root[root] @  [192.168.1.11]
# Query_time: 1.000000  Lock_time: 0.000000  Rows_sent: 0  Rows_examined: 0
SET timestamp=1779703920;
commit;
`
	lines := strings.Split(strings.TrimSpace(text), "\n")
	result, err := analyzeMySQLSlowLogLines(&cmdbmodel.CmdbSQL{Name: "shop"}, lines, dbmodel.SlowLogQuery{
		SortBy:    "queryTimeTotal",
		SortOrder: "desc",
		Page:      1,
		PageSize:  10,
	})
	if err != nil {
		t.Fatalf("analyzeMySQLSlowLogLines returned error: %v", err)
	}
	if result.Summary.TotalEntries != 3 {
		t.Fatalf("total entries = %d, want 3", result.Summary.TotalEntries)
	}
	if result.Summary.MatchedEntries != 3 {
		t.Fatalf("matched entries = %d, want 3", result.Summary.MatchedEntries)
	}
	if result.Summary.UniqueSQL != 2 {
		t.Fatalf("unique sql = %d, want 2", result.Summary.UniqueSQL)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items len = %d, want 2", len(result.Items))
	}
	top := result.Items[0]
	if top.Count != 2 {
		t.Fatalf("top count = %d, want 2", top.Count)
	}
	if top.QueryTimeTotal != 6 {
		t.Fatalf("top query time total = %.3f, want 6", top.QueryTimeTotal)
	}
	if top.RowsExaminedTotal != 400 {
		t.Fatalf("top rows examined total = %d, want 400", top.RowsExaminedTotal)
	}
	if top.Fingerprint != "select * from orders where id = ?" {
		t.Fatalf("fingerprint = %q", top.Fingerprint)
	}
}

func TestParseSlowLogAISuggestionAnswer(t *testing.T) {
	answer := "```json\n{\"items\":[{\"digest\":\"abc123\",\"slowReason\":\"扫描行数高\",\"schemaOptimization\":\"需结合表结构确认字段类型\",\"indexOptimization\":\"建议评估联合索引\",\"sqlOptimization\":\"先用 EXPLAIN 验证执行计划\",\"expectedEffect\":\"预计减少扫描行数\"}]}\n```"
	suggestions, err := parseSlowLogAISuggestionAnswer(answer)
	if err != nil {
		t.Fatalf("parseSlowLogAISuggestionAnswer returned error: %v", err)
	}
	if suggestions.ByDigest["abc123"].SlowReason != "扫描行数高" {
		t.Fatalf("slow reason = %q", suggestions.ByDigest["abc123"].SlowReason)
	}
	if suggestions.ByDigest["abc123"].IndexOptimization != "建议评估联合索引" {
		t.Fatalf("index optimization = %q", suggestions.ByDigest["abc123"].IndexOptimization)
	}
}

func TestParseSlowLogAISuggestionAnswerSupportsLegacySuggestion(t *testing.T) {
	answer := `{"items":[{"digest":"abc123","suggestion":"建议补充联合索引并结合 EXPLAIN 验证。"}]}`
	suggestions, err := parseSlowLogAISuggestionAnswer(answer)
	if err != nil {
		t.Fatalf("parseSlowLogAISuggestionAnswer returned error: %v", err)
	}
	if suggestions.ByDigest["abc123"].SlowReason != "建议补充联合索引并结合 EXPLAIN 验证。" {
		t.Fatalf("legacy suggestion = %q", suggestions.ByDigest["abc123"].SlowReason)
	}
}

func TestParseSlowLogAISuggestionAnswerSupportsChineseFieldsAndOrderedFallback(t *testing.T) {
	answer := `{"items":[{"慢查询原因":"扫描行数高","表结构优化":"检查字段类型","索引优化":"补充联合索引","SQL语句优化":"减少 JOIN 数据量","预期效果":"减少耗时"}]}`
	suggestions, err := parseSlowLogAISuggestionAnswer(answer)
	if err != nil {
		t.Fatalf("parseSlowLogAISuggestionAnswer returned error: %v", err)
	}
	if len(suggestions.Ordered) != 1 {
		t.Fatalf("ordered len = %d", len(suggestions.Ordered))
	}
	if suggestions.Ordered[0].SlowReason != "扫描行数高" {
		t.Fatalf("slow reason = %q", suggestions.Ordered[0].SlowReason)
	}
	if suggestions.Ordered[0].SQLOptimization != "减少 JOIN 数据量" {
		t.Fatalf("sql optimization = %q", suggestions.Ordered[0].SQLOptimization)
	}
}

func TestParseSlowLogAISuggestionAnswerSupportsPlainTextSections(t *testing.T) {
	answer := `慢查询原因: 扫描行数高
表结构优化: 检查字段类型
索引优化: 补充联合索引
SQL语句优化: 减少 JOIN 数据量
预期效果: 减少耗时`
	suggestions, err := parseSlowLogAISuggestionAnswer(answer)
	if err != nil {
		t.Fatalf("parseSlowLogAISuggestionAnswer returned error: %v", err)
	}
	if len(suggestions.Ordered) != 1 {
		t.Fatalf("ordered len = %d", len(suggestions.Ordered))
	}
	if suggestions.Ordered[0].IndexOptimization != "补充联合索引" {
		t.Fatalf("index optimization = %q", suggestions.Ordered[0].IndexOptimization)
	}
	if suggestions.Ordered[0].ExpectedEffect != "减少耗时" {
		t.Fatalf("expected effect = %q", suggestions.Ordered[0].ExpectedEffect)
	}
}

func TestParseSlowLogAISuggestionAnswerRejectsJSONFragmentAsPlainText(t *testing.T) {
	answer := `减少扫描行数，将平均查询时间降低。"}, { "digest": "abc123", "slowReason": "复杂查询"`
	_, err := parseSlowLogAISuggestionAnswer(answer)
	if err == nil {
		t.Fatalf("parseSlowLogAISuggestionAnswer should reject JSON fragment")
	}
}
