package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoaderLoad(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "skills/cmdb_skill/host-query/SKILL.md", `---
name: host-query
description: 查询主机
category: query
domain: cmdb
skillGroup: cmdb_skill
runtime: declarative
status: enabled
riskLevel: readonly
---
body
`)
	writeSkill(t, root, "skills/bad_skill/bad/SKILL.md", `---
name:
description: bad
---
body
`)

	docs, errs, err := NewLoader(filepath.Join(root, "skills")).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("loaded docs = %d, want 1", len(docs))
	}
	if docs[0].Manifest.Name != "host-query" {
		t.Fatalf("loaded name = %q, want host-query", docs[0].Manifest.Name)
	}
	if len(errs) != 1 {
		t.Fatalf("load errors = %d, want 1", len(errs))
	}
}

func TestCatalogListFilters(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "skills/cmdb_skill/host-query/SKILL.md", `---
name: host-query
description: 查询主机
category: query
domain: cmdb
skillGroup: cmdb_skill
runtime: declarative
status: enabled
riskLevel: readonly
tags:
  - asset
---
body
`)
	writeSkill(t, root, "skills/monitor_skill/host-pressure/SKILL.md", `---
name: host-pressure
description: 分析主机压力
category: diagnosis
domain: monitor
skillGroup: monitor_skill
runtime: declarative
status: enabled
riskLevel: readonly
---
body
`)

	catalog := NewCatalog(filepath.Join(root, "skills"))
	if _, err := catalog.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	cmdb := catalog.List(map[string]string{"domain": "cmdb"})
	if len(cmdb) != 1 || cmdb[0].Name != "host-query" {
		t.Fatalf("cmdb filter = %#v, want host-query", cmdb)
	}
	keyword := catalog.List(map[string]string{"keyword": "pressure"})
	if len(keyword) != 1 || keyword[0].Name != "host-pressure" {
		t.Fatalf("keyword filter = %#v, want host-pressure", keyword)
	}
}

func writeSkill(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
