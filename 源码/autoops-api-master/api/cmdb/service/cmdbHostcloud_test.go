package service

import (
	"errors"
	"testing"

	"dodevops-api/api/cmdb/model"
)

type fakeCloudGroupLookup struct {
	defaultGroup   model.CmdbGroup
	defaultErr     error
	groupsByName   map[string]*model.CmdbGroup
	groupsByID     map[uint]model.CmdbGroup
	defaultLookups int
}

func (f *fakeCloudGroupLookup) GetCmdbGroupById(id uint) (model.CmdbGroup, error) {
	group, ok := f.groupsByID[id]
	if !ok {
		return model.CmdbGroup{}, errors.New("group not found")
	}
	return group, nil
}

func (f *fakeCloudGroupLookup) GetCmdbGroupByExactName(name string) (*model.CmdbGroup, error) {
	group, ok := f.groupsByName[name]
	if !ok {
		return nil, errors.New("group not found")
	}
	return group, nil
}

func (f *fakeCloudGroupLookup) GetDefaultCmdbGroup() (model.CmdbGroup, error) {
	f.defaultLookups++
	return f.defaultGroup, f.defaultErr
}

func TestResolveCloudHostGroupIDUsesDefaultGroupActualID(t *testing.T) {
	lookup := &fakeCloudGroupLookup{defaultGroup: model.CmdbGroup{ID: 42, Name: model.DefaultCmdbGroupName}}

	groupID, err := resolveCloudHostGroupID(0, lookup)
	if err != nil {
		t.Fatalf("resolveCloudHostGroupID() error = %v", err)
	}
	if groupID != 42 {
		t.Fatalf("resolveCloudHostGroupID() = %d, want 42", groupID)
	}
}

func TestResolveCloudHostGroupIDKeepsExplicitGroup(t *testing.T) {
	lookup := &fakeCloudGroupLookup{defaultErr: errors.New("should not be queried")}

	groupID, err := resolveCloudHostGroupID(88, lookup)
	if err != nil {
		t.Fatalf("resolveCloudHostGroupID() error = %v", err)
	}
	if groupID != 88 {
		t.Fatalf("resolveCloudHostGroupID() = %d, want 88", groupID)
	}
	if lookup.defaultLookups != 0 {
		t.Fatalf("default group lookups = %d, want 0", lookup.defaultLookups)
	}
}

func TestMatchGroupByTagsFallsBackToResolvedDefaultGroup(t *testing.T) {
	lookup := &fakeCloudGroupLookup{groupsByName: map[string]*model.CmdbGroup{}}
	cloudService := &CmdbHostCloudServiceImpl{}

	groupID, _ := cloudService.matchGroupByTags(map[string]string{"env": "prod"}, lookup, 42)
	if groupID != 42 {
		t.Fatalf("matchGroupByTags() = %d, want 42", groupID)
	}
}

func TestMatchGroupByTagsPrefersMatchingBusinessGroup(t *testing.T) {
	lookup := &fakeCloudGroupLookup{
		groupsByName: map[string]*model.CmdbGroup{
			"orders": {ID: 99, Name: "orders"},
		},
	}
	cloudService := &CmdbHostCloudServiceImpl{}

	groupID, _ := cloudService.matchGroupByTags(map[string]string{"bline": "orders"}, lookup, 42)
	if groupID != 99 {
		t.Fatalf("matchGroupByTags() = %d, want 99", groupID)
	}
}
