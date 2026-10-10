package skill

import "testing"

func TestParseDocument(t *testing.T) {
	doc, err := ParseDocument([]byte(`---
name: host-pressure
description: 分析主机压力
category: diagnosis
domain: monitor
skillGroup: monitor_skill
runtime: declarative
status: enabled
riskLevel: readonly
readonly: true
tools:
  - query_host_metrics
---

## 触发条件

主机 CPU 高时使用。
`))
	if err != nil {
		t.Fatalf("ParseDocument() error = %v", err)
	}
	if doc.Manifest.Name != "host-pressure" {
		t.Fatalf("name = %q, want host-pressure", doc.Manifest.Name)
	}
	if len(doc.Manifest.Tools) != 1 || doc.Manifest.Tools[0] != "query_host_metrics" {
		t.Fatalf("tools = %#v, want query_host_metrics", doc.Manifest.Tools)
	}
	if doc.Body == "" {
		t.Fatal("body is empty")
	}
}

func TestParseDocumentDefaults(t *testing.T) {
	doc, err := ParseDocument([]byte(`---
name: draft-skill
description: draft
category: query
domain: cmdb
skillGroup: cmdb_skill
---
body
`))
	if err != nil {
		t.Fatalf("ParseDocument() error = %v", err)
	}
	if doc.Manifest.Runtime != SkillRuntimeDeclarative {
		t.Fatalf("runtime = %q, want %q", doc.Manifest.Runtime, SkillRuntimeDeclarative)
	}
	if doc.Manifest.Status != SkillStatusDraft {
		t.Fatalf("status = %q, want %q", doc.Manifest.Status, SkillStatusDraft)
	}
	if doc.Manifest.RiskLevel != SkillRiskReadonly {
		t.Fatalf("riskLevel = %q, want %q", doc.Manifest.RiskLevel, SkillRiskReadonly)
	}
}

func TestParseDocumentRejectsInvalidStatus(t *testing.T) {
	_, err := ParseDocument([]byte(`---
name: bad
description: bad
category: query
domain: cmdb
skillGroup: cmdb_skill
status: unknown
---
body
`))
	if err == nil {
		t.Fatal("ParseDocument() error = nil, want error")
	}
}
