package service

import (
	"strings"
	"testing"

	aiopsmodel "dodevops-api/api/aiops/model"
)

func TestNormalizeKnowledgeMarkdownFormatsImportedIncidentText(t *testing.T) {
	content := `250115消息平台大表操作引发主从同步故障致系统异常的质量事件

一、事件经过
1）2025-01-15 12:00 梦凡中午提交消息平台升级SQL；
2）2025-01-15 20:11 梦凡开始执行SQL；

影响范围：
电子合同访问异常，影响时间为2025-01-15 21:14~21:40，持续26分钟

负责人：杨梦凡`

	got := normalizeKnowledgeMarkdown("book", aiopsmodel.KnowledgeDocTypeIncident, content)
	assertKnowledgeContains(t, got, "# book")
	assertKnowledgeContains(t, got, "## 1. 事件经过")
	assertKnowledgeContains(t, got, "1. 2025-01-15 12:00")
	assertKnowledgeContains(t, got, "2. 2025-01-15 20:11")
	assertKnowledgeContains(t, got, "## 2. 影响范围")
	assertKnowledgeContains(t, got, "- **负责人**: 杨梦凡")
}

func TestNormalizeKnowledgeMarkdownDoesNotAppendEmptyIncidentSections(t *testing.T) {
	content := `只有一段普通事故说明，没有标准五段标题。`

	got := normalizeKnowledgeMarkdown("普通事故", aiopsmodel.KnowledgeDocTypeIncident, content)
	if strings.Contains(got, "## 1. 事件经过") {
		t.Fatalf("expected no empty incident template sections, got:\n%s", got)
	}
}

func TestAutoFormatKnowledgeMarkdownFormatsTabTable(t *testing.T) {
	content := "字段\t含义\t示例\nservice\t服务名\torder-api\nlevel\t级别\tcritical"

	got := normalizeKnowledgeMarkdown("字段说明", aiopsmodel.KnowledgeDocTypeNote, content)
	assertKnowledgeContains(t, got, "| 字段 | 含义 | 示例 |")
	assertKnowledgeContains(t, got, "| --- | --- | --- |")
	assertKnowledgeContains(t, got, "| service | 服务名 | order-api |")
}

func TestNormalizeKnowledgeMarkdownSupportsPastedContent(t *testing.T) {
	got := normalizeKnowledgeMarkdown("粘贴导入测试", aiopsmodel.KnowledgeDocTypeNote, "一、背景\n负责人：张三")

	assertKnowledgeContains(t, got, "# 粘贴导入测试")
	assertKnowledgeContains(t, got, "## 背景")
	assertKnowledgeContains(t, got, "- **负责人**: 张三")
}

func assertKnowledgeContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected markdown to contain %q, got:\n%s", want, got)
	}
}
