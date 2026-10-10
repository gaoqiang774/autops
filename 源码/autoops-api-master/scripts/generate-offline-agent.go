package main

import (
	"flag"
	"fmt"
	"os"

	"dodevops-api/common/agent"
)

func main() {
	output := flag.String("output", "", "generated main.go path")
	heartbeatURL := flag.String("heartbeat-url", "", "default heartbeat URL")
	heartbeatToken := flag.String("heartbeat-token", "", "default heartbeat token")
	pushgatewayURL := flag.String("pushgateway-url", "", "default Pushgateway URL")
	groupID := flag.Uint("group-id", 0, "default CMDB group ID")
	groupName := flag.String("group-name", "default", "default CMDB group name")
	version := flag.String("version", "2.1.0", "Agent version")
	versionTag := flag.String("version-tag", "v2", "Agent version tag")
	flag.Parse()

	if *output == "" {
		fmt.Fprintln(os.Stderr, "--output is required")
		os.Exit(2)
	}

	code := agent.CreateAgentMainFile(
		*heartbeatURL,
		*heartbeatToken,
		*pushgatewayURL,
		*groupID,
		*groupName,
		*version,
		*versionTag,
	)
	if err := os.WriteFile(*output, []byte(code), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write generated Agent source: %v\n", err)
		os.Exit(1)
	}
}
