package db

import "testing"

func TestDefaultKnowledgeGroupSpecs(t *testing.T) {
	wantNames := []string{
		"01-运维管理制度",
		"02-部署维护记录",
		"03-运维故障记录",
		"04-中间件维护记录",
		"05-数据库管理",
		"06-运维开发",
		"07-运维监控",
		"08-运维体系建设",
		"09-运维安全管理",
		"10-总结记录",
	}
	specs := defaultKnowledgeGroupSpecs()
	if len(specs) != len(wantNames) {
		t.Fatalf("default group count = %d, want %d", len(specs), len(wantNames))
	}
	seen := make(map[string]struct{}, len(specs))
	for i, spec := range specs {
		if spec.Name != wantNames[i] {
			t.Fatalf("group %d name = %q, want %q", i, spec.Name, wantNames[i])
		}
		if spec.SortOrder != (i+1)*10 {
			t.Fatalf("group %s sort order = %d", spec.Name, spec.SortOrder)
		}
		if _, exists := seen[spec.Name]; exists {
			t.Fatalf("duplicate default group: %s", spec.Name)
		}
		seen[spec.Name] = struct{}{}
	}
}
