package service

import (
	"testing"

	cmdbmodel "dodevops-api/api/cmdb/model"
)

func TestSelectAssistantHostsForAnalysisIncludesAllHosts(t *testing.T) {
	hosts := make([]cmdbmodel.CmdbHost, 120)
	for i := range hosts {
		hosts[i].ID = uint(i + 1)
	}
	coverage := assistantAgentCoverage{
		ByHostID: map[uint]assistantAgentInfo{
			120: {HostID: 120, Installed: true, Online: true},
		},
	}

	selected, missing := selectAssistantHostsForAnalysis(hosts, coverage)
	if len(selected) != len(hosts) {
		t.Fatalf("selected hosts = %d, want %d", len(selected), len(hosts))
	}
	if len(missing) != 0 {
		t.Fatalf("unexpected missing info: %v", missing)
	}
	if selected[0].ID != 120 {
		t.Fatalf("first selected host = %d, want online host 120", selected[0].ID)
	}
}
