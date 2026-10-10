package service

import (
	"reflect"
	"strings"
	"testing"

	"dodevops-api/api/cmdb/model"
)

func TestValidateAgentDeployOptions(t *testing.T) {
	if err := validateAgentDeployOptions(model.AgentDeployOptions{}); err != nil {
		t.Fatalf("online deployment options should be valid: %v", err)
	}
	if err := validateAgentDeployOptions(model.AgentDeployOptions{DeployMode: "unknown"}); err == nil {
		t.Fatal("unsupported deployment mode should fail validation")
	}
	if err := validateAgentDeployOptions(model.AgentDeployOptions{DeployMode: "offline"}); err == nil || !strings.Contains(err.Error(), "二进制") {
		t.Fatalf("offline deployment without binary should fail clearly: %v", err)
	}
}

func TestResolveAgentOfflineAssetRejectsTraversal(t *testing.T) {
	if _, err := resolveAgentOfflineAsset("../outside-agent"); err == nil {
		t.Fatal("offline asset path traversal should be rejected")
	}
}

func TestPrioritizeAgentVersionStats_ConfiguredVersionFirst(t *testing.T) {
	input := []model.AgentVersionStat{
		{Version: "1.0.0", Count: 3},
		{Version: "2.0.0", Count: 1},
	}

	got := prioritizeAgentVersionStats(input, "2.0.0")
	want := []model.AgentVersionStat{
		{Version: "2.0.0", Count: 1},
		{Version: "1.0.0", Count: 3},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected stats order: got=%v want=%v", got, want)
	}
}

func TestPrioritizeAgentVersionStats_AddsConfiguredVersionWhenMissing(t *testing.T) {
	input := []model.AgentVersionStat{
		{Version: "1.0.0", Count: 3},
	}

	got := prioritizeAgentVersionStats(input, "2.0.0")
	want := []model.AgentVersionStat{
		{Version: "2.0.0", Count: 0},
		{Version: "1.0.0", Count: 3},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected stats order: got=%v want=%v", got, want)
	}
}
