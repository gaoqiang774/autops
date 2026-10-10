package service

import (
	"testing"

	aiopsmodel "dodevops-api/api/aiops/model"
)

func TestBuildKnowledgeGroupTreeAggregatesDescendantDocuments(t *testing.T) {
	groups := []aiopsmodel.KnowledgeGroup{
		{ID: 1, Name: "数据库运维", SortOrder: 1},
		{ID: 2, ParentID: 1, Name: "MySQL", SortOrder: 1},
		{ID: 3, ParentID: 1, Name: "Redis", SortOrder: 2},
	}
	tree := buildKnowledgeGroupTree(groups, map[uint]int64{1: 1, 2: 3, 3: 2})
	if len(tree) != 1 {
		t.Fatalf("tree size = %d, want 1", len(tree))
	}
	if tree[0].DirectCount != 1 || tree[0].DocCount != 6 {
		t.Fatalf("root counts = direct:%d total:%d, want direct:1 total:6", tree[0].DirectCount, tree[0].DocCount)
	}
	if len(tree[0].Children) != 2 || tree[0].Children[0].Path != "数据库运维 / MySQL" {
		t.Fatalf("unexpected child tree: %#v", tree[0].Children)
	}
}

func TestKnowledgeGroupIsDescendant(t *testing.T) {
	groups := []aiopsmodel.KnowledgeGroup{
		{ID: 1, Name: "root"},
		{ID: 2, ParentID: 1, Name: "child"},
		{ID: 3, ParentID: 2, Name: "grandchild"},
	}
	if !knowledgeGroupIsDescendant(groups, 1, 3) {
		t.Fatal("grandchild should be detected as descendant")
	}
	if knowledgeGroupIsDescendant(groups, 2, 1) {
		t.Fatal("parent must not be detected as descendant")
	}
}

func TestKnowledgeGroupLabelsRejectsCycle(t *testing.T) {
	groups := []aiopsmodel.KnowledgeGroup{
		{ID: 1, ParentID: 2, Name: "one"},
		{ID: 2, ParentID: 1, Name: "two"},
	}
	if _, ok := knowledgeGroupLabels(groups, 1); ok {
		t.Fatal("cyclic group path must be rejected")
	}
}
