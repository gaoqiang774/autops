package dao

import (
	"reflect"
	"testing"

	"dodevops-api/api/aiops/model"
)

func TestKnowledgeGroupDeleteOrderIncludesDescendants(t *testing.T) {
	groups := []model.KnowledgeGroup{
		{ID: 1},
		{ID: 2, ParentID: 1},
		{ID: 3, ParentID: 2},
		{ID: 4, ParentID: 1},
		{ID: 5},
		{ID: 6, ParentID: 5},
	}

	got, ok := knowledgeGroupDeleteOrder(groups, 1)
	if !ok {
		t.Fatal("expected root group to exist")
	}
	want := []uint{3, 2, 4, 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("delete order = %v, want %v", got, want)
	}
}

func TestKnowledgeGroupDeleteOrderRejectsMissingRoot(t *testing.T) {
	groups := []model.KnowledgeGroup{{ID: 1}, {ID: 2, ParentID: 1}}

	got, ok := knowledgeGroupDeleteOrder(groups, 99)
	if ok || got != nil {
		t.Fatalf("delete order = %v, exists = %v; want nil, false", got, ok)
	}
}
