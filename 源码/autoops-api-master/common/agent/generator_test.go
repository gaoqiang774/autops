package agent

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestCreateAgentMainFileParses(t *testing.T) {
	code := CreateAgentMainFile(
		"http://127.0.0.1:8000/api/v1/cmdb/agent/heartbeat",
		"agent-heartbeat-token-2024",
		"http://127.0.0.1:8428/api/v1/import/prometheus",
		1,
		"default",
		"2.1.0",
		"v2",
	)

	if strings.Contains(code, "%!") {
		t.Fatalf("generated agent code contains fmt template error: %s", code[strings.Index(code, "%!"):])
	}
	for _, want := range []string{
		"HostAliases   []KafkaHostAlias",
		"applyKafkaHostAliases(cfg.HostAliases)",
		"[Logtail] apply kafka host aliases failed",
		"# BEGIN AIOPS VECTOR KAFKA HOST ALIASES",
		"json:\"server_url\"",
		"flag.String(\"config\"",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("generated agent code missing %q", want)
		}
	}
	if err := ValidateAgentCode(code); err != nil {
		t.Fatalf("generated agent validation failed: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "agent_main.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated agent code parse failed: %v", err)
	}
}
