#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT_DIR="${1:-$ROOT_DIR/upload/agent}"
AGENT_VERSION="${AGENT_VERSION:-2.1.0}"
AGENT_VERSION_TAG="${AGENT_VERSION_TAG:-v2}"
AGENT_HEARTBEAT_URL="${AGENT_HEARTBEAT_URL:-http://127.0.0.1:8000/api/v1/cmdb/agent/heartbeat}"
AGENT_HEARTBEAT_TOKEN="${AGENT_HEARTBEAT_TOKEN:-agent-heartbeat-token-2024}"
AGENT_PUSHGATEWAY_URL="${AGENT_PUSHGATEWAY_URL:-http://8.130.14.34:9091}"
AGENT_GROUP_ID="${AGENT_GROUP_ID:-0}"
AGENT_GROUP_NAME="${AGENT_GROUP_NAME:-default}"
# Set GO_BIN explicitly when an interactive shell exposes a different Go
# through an alias or version manager than non-interactive Bash does.
GO_BIN="${GO_BIN:-go}"

if [[ "$OUTPUT_DIR" != /* ]]; then
	OUTPUT_DIR="$PWD/$OUTPUT_DIR"
fi

mkdir -p "$OUTPUT_DIR"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

cat > "$WORK_DIR/go.mod" <<'EOF'
module dodevops-api

go 1.20

require (
	github.com/prometheus/client_golang v1.19.1
	github.com/prometheus/client_model v0.6.1
	github.com/prometheus/common v0.48.0
	github.com/shirou/gopsutil/v3 v3.23.10
)
EOF

# The repository module currently requires Go 1.25.6. The generator itself
# only uses the standard library, so isolate it from that requirement and
# keep the build usable with the locally installed Go toolchain.
mkdir -p "$WORK_DIR/common/agent" "$WORK_DIR/scripts"
cp "$ROOT_DIR/common/agent/generator.go" "$WORK_DIR/common/agent/generator.go"
cp "$ROOT_DIR/scripts/generate-offline-agent.go" "$WORK_DIR/scripts/generate-offline-agent.go"

(
	cd "$WORK_DIR"
	GOTOOLCHAIN=local "$GO_BIN" run ./scripts/generate-offline-agent.go \
		-output "$WORK_DIR/main.go" \
		-heartbeat-url "$AGENT_HEARTBEAT_URL" \
		-heartbeat-token "$AGENT_HEARTBEAT_TOKEN" \
		-pushgateway-url "$AGENT_PUSHGATEWAY_URL" \
		-group-id "$AGENT_GROUP_ID" \
		-group-name "$AGENT_GROUP_NAME" \
		-version "$AGENT_VERSION" \
		-version-tag "$AGENT_VERSION_TAG"
)

(
	cd "$WORK_DIR"
	# Resolve imports from the generated source and write the required go.sum
	# entries before the cross-compilation steps.
	GOTOOLCHAIN=local "$GO_BIN" mod tidy
	GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		"$GO_BIN" build -trimpath -buildvcs=false -ldflags "-s -w" \
		-o "$OUTPUT_DIR/dodevops-agent-linux-amd64" main.go
	GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		"$GO_BIN" build -trimpath -buildvcs=false -ldflags "-s -w" \
		-o "$OUTPUT_DIR/dodevops-agent-linux-arm64" main.go
)

chmod 0755 "$OUTPUT_DIR/dodevops-agent-linux-amd64" "$OUTPUT_DIR/dodevops-agent-linux-arm64"
printf 'offline Agent binaries created in %s\n' "$OUTPUT_DIR"
