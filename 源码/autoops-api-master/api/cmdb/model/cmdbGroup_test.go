package model

import (
	"testing"
	"time"

	"dodevops-api/common/util"
)

func TestBuildCmdbGroupTreeWithHostCountSortsByUpdateTimeDesc(t *testing.T) {
	groups := []CmdbGroup{
		{
			ID:         1,
			ParentID:   0,
			Name:       "older-root",
			CreateTime: util.HTime{Time: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
			UpdateTime: util.HTime{Time: time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)},
		},
		{
			ID:         2,
			ParentID:   0,
			Name:       "newer-root",
			CreateTime: util.HTime{Time: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)},
			UpdateTime: util.HTime{Time: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)},
		},
		{
			ID:         3,
			ParentID:   2,
			Name:       "older-child",
			CreateTime: util.HTime{Time: time.Date(2026, 5, 3, 11, 0, 0, 0, time.UTC)},
			UpdateTime: util.HTime{Time: time.Date(2026, 5, 4, 11, 0, 0, 0, time.UTC)},
		},
		{
			ID:         4,
			ParentID:   2,
			Name:       "newer-child",
			CreateTime: util.HTime{Time: time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)},
			UpdateTime: util.HTime{Time: time.Date(2026, 5, 5, 11, 0, 0, 0, time.UTC)},
		},
	}

	hosts := []CmdbHost{
		{ID: 10, GroupID: 3},
		{ID: 11, GroupID: 4},
	}

	tree := BuildCmdbGroupTreeWithHostCount(groups, hosts)
	if len(tree) != 2 {
		t.Fatalf("expected 2 root nodes, got %d", len(tree))
	}
	if tree[0].ID != 2 {
		t.Fatalf("expected newer root first, got id %d", tree[0].ID)
	}
	if len(tree[0].Children) != 2 {
		t.Fatalf("expected 2 children under newer root, got %d", len(tree[0].Children))
	}
	if tree[0].Children[0].ID != 4 {
		t.Fatalf("expected newer child first, got id %d", tree[0].Children[0].ID)
	}
}

func TestBuildCmdbGroupTreeWithHostCountFallsBackToCreateTimeWhenUpdateTimeMissing(t *testing.T) {
	groups := []CmdbGroup{
		{
			ID:         1,
			ParentID:   0,
			Name:       "older-root",
			CreateTime: util.HTime{Time: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		},
		{
			ID:         2,
			ParentID:   0,
			Name:       "newer-root",
			CreateTime: util.HTime{Time: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)},
		},
	}

	tree := BuildCmdbGroupTreeWithHostCount(groups, nil)
	if len(tree) != 2 {
		t.Fatalf("expected 2 root nodes, got %d", len(tree))
	}
	if tree[0].ID != 2 {
		t.Fatalf("expected newer createTime first when updateTime is missing, got id %d", tree[0].ID)
	}
}
