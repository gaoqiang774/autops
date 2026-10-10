package service

import (
	"strings"
	"testing"

	aiopsmodel "dodevops-api/api/aiops/model"
)

func TestTokenizeKnowledgeQuery(t *testing.T) {
	terms := tokenizeKnowledgeQuery("MySQL 主从延迟怎么处理？", 16)
	want := []string{"mysql", "主从延迟", "延迟怎么", "怎么处理", "主从", "延迟", "处理"}
	for _, term := range want {
		if !containsKnowledgeTerm(terms, term) {
			t.Fatalf("tokenizeKnowledgeQuery() = %#v, missing %q", terms, term)
		}
	}
	if containsKnowledgeTerm(terms, "怎么") {
		t.Fatalf("tokenizeKnowledgeQuery() should remove stop word: %#v", terms)
	}
}

func TestScoreKnowledgeDocumentUsesQueryTerms(t *testing.T) {
	relevant := aiopsmodel.KnowledgeDocument{
		Title:           "MySQL 主从延迟故障复盘",
		Summary:         "复制线程阻塞导致延迟升高",
		ContentMarkdown: "## 解决方案\n重启复制线程并验证延迟恢复。",
	}
	unrelated := aiopsmodel.KnowledgeDocument{
		Title:           "Nginx 证书更新",
		ContentMarkdown: "更新证书后重新加载配置。",
	}
	query := "MySQL 主从延迟怎么处理"
	relevantItem := scoreKnowledgeDocument(relevant, query)
	unrelatedItem := scoreKnowledgeDocument(unrelated, query)
	if relevantItem.Score <= unrelatedItem.Score {
		t.Fatalf("relevant score %v should be higher than unrelated score %v", relevantItem.Score, unrelatedItem.Score)
	}
	if !containsKnowledgeTerm(relevantItem.MatchedFields, "title") {
		t.Fatalf("matched fields = %#v, want title", relevantItem.MatchedFields)
	}
}

func TestBuildKnowledgeSnippetFallsBackToQueryTerm(t *testing.T) {
	content := "现象说明：业务请求正常。\n根因：MySQL 主从延迟持续升高。\n处理：检查复制线程。"
	snippet := buildKnowledgeSnippet(content, "MySQL 主从延迟怎么处理")
	if !strings.Contains(snippet, "主从延迟") {
		t.Fatalf("snippet = %q, want matched query term", snippet)
	}
}

func TestNormalizeKnowledgeEnvDefaultsToProd(t *testing.T) {
	if got := normalizeKnowledgeEnv(" "); got != "prod" {
		t.Fatalf("normalizeKnowledgeEnv() = %q, want prod", got)
	}
	if got := normalizeKnowledgeEnv(" test "); got != "test" {
		t.Fatalf("normalizeKnowledgeEnv() = %q, want test", got)
	}
}

func containsKnowledgeTerm(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
