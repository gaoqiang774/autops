package util

import (
	"encoding/json"
	"os/exec"
	"testing"
)

func runSystemInfoScript(t *testing.T, commandMocks string) map[string]string {
	t.Helper()

	cmd := exec.Command("bash", "-c", commandMocks+"\n"+getScriptContent())
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("system info script failed: %v", err)
	}

	var info map[string]string
	if err := json.Unmarshal(output, &info); err != nil {
		t.Fatalf("invalid system info output %q: %v", output, err)
	}
	return info
}

func TestSystemInfoScriptUsesDefaultRouteSourceIP(t *testing.T) {
	info := runSystemInfoScript(t, `
ip() {
    case "$*" in
        "-4 route get 1.1.1.1")
            echo "1.1.1.1 via 192.168.96.1 dev enp3s0 src 192.168.96.24 uid 0"
            ;;
        *) return 1 ;;
    esac
}
curl() { printf "203.0.113.10"; }
hostname() {
    if [ "$1" = "-I" ]; then
        echo "172.17.0.1 172.18.0.1 192.168.96.24"
    else
        echo "openEuler-host"
    fi
}
`)

	if got := info["privateIp"]; got != "192.168.96.24" {
		t.Fatalf("privateIp = %q, want %q", got, "192.168.96.24")
	}
	if got := info["publicIp"]; got != "203.0.113.10" {
		t.Fatalf("publicIp = %q, want %q", got, "203.0.113.10")
	}
}

func TestSystemInfoScriptSkipsContainerInterfacesWithoutDefaultRoute(t *testing.T) {
	info := runSystemInfoScript(t, `
ip() {
    case "$*" in
        "-4 route get 1.1.1.1") return 1 ;;
        "-o -4 addr show scope global")
            echo "2: docker0    inet 172.17.0.1/16 scope global docker0"
            echo "3: br-abcd    inet 172.18.0.1/16 scope global br-abcd"
            echo "4: enp3s0     inet 192.168.96.24/24 scope global enp3s0"
            ;;
        *) return 1 ;;
    esac
}
curl() { return 1; }
hostname() {
    if [ "$1" = "-I" ]; then
        echo "172.17.0.1 172.18.0.1 192.168.96.24"
    else
        echo "openEuler-host"
    fi
}
`)

	if got := info["privateIp"]; got != "192.168.96.24" {
		t.Fatalf("privateIp = %q, want %q", got, "192.168.96.24")
	}
}
