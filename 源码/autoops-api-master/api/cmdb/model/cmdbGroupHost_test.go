package model

import (
	"testing"
	"time"

	"dodevops-api/common/util"
)

func TestBuildCmdbGroupHostTreeIncludesHostType(t *testing.T) {
	groups := []CmdbGroup{
		{
			ID:         1,
			ParentID:   0,
			Name:       "root",
			CreateTime: util.HTime{Time: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
			UpdateTime: util.HTime{Time: time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)},
		},
		{
			ID:         2,
			ParentID:   1,
			Name:       "child",
			CreateTime: util.HTime{Time: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
			UpdateTime: util.HTime{Time: time.Date(2026, 5, 2, 11, 0, 0, 0, time.UTC)},
		},
	}

	hosts := []CmdbHostVo{
		{
			ID:       10,
			HostName: "win-host",
			GroupID:  2,
			SSHIP:    "10.0.0.10",
			SSHName:  "Administrator",
			SSHPort:  5985,
			HostType: HostTypeWindows,
		},
	}

	tree := BuildCmdbGroupHostTree(groups, hosts)
	if len(tree) != 1 {
		t.Fatalf("expected 1 root node, got %d", len(tree))
	}

	root := tree[0]
	if root.HostCount != 1 {
		t.Fatalf("expected root hostCount 1, got %d", root.HostCount)
	}
	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child node, got %d", len(root.Children))
	}

	child := root.Children[0]
	if child.HostCount != 1 {
		t.Fatalf("expected child hostCount 1, got %d", child.HostCount)
	}
	if len(child.Hosts) != 1 {
		t.Fatalf("expected 1 host on child node, got %d", len(child.Hosts))
	}
	if child.Hosts[0].HostType != HostTypeWindows {
		t.Fatalf("expected hostType %d, got %d", HostTypeWindows, child.Hosts[0].HostType)
	}
}

func TestBuildCmdbGroupHostTreeSortsByUpdateTimeDesc(t *testing.T) {
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

	tree := BuildCmdbGroupHostTree(groups, nil)
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
