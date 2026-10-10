package license

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
)

var stableIDFiles = []string{"/etc/machine-id", "/var/lib/dbus/machine-id"}

var installationIDMu sync.Mutex

// MachineCode is based on a persisted installation identity instead of OS or
// network hardware. This makes new licenses stable on macOS, Windows, Linux,
// virtual machines, and containers as long as the AutoOps data directory is
// preserved.
func MachineCode() (string, error) {
	installationID, err := InstallationID()
	if err != nil {
		return "", err
	}
	return machineCodeFromIdentity(BindingInstallationV1 + "|" + installationID), nil
}

func InstallationID() (string, error) {
	installationIDMu.Lock()
	defer installationIDMu.Unlock()

	path := InstallationIDPath()
	if value, err := readInstallationID(path); err == nil {
		return value, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("%w: %v", ErrInstallationIdentity, err)
	}

	if err := os.MkdirAll(filepathDir(path), 0750); err != nil {
		return "", fmt.Errorf("%w: create AutoOps data directory: %v", ErrInstallationIdentity, err)
	}
	generated := uuid.NewString()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		value, readErr := readInstallationID(path)
		if readErr != nil {
			return "", fmt.Errorf("%w: %v", ErrInstallationIdentity, readErr)
		}
		return value, nil
	}
	if err != nil {
		return "", fmt.Errorf("%w: create installation identity: %v", ErrInstallationIdentity, err)
	}
	if _, err := file.WriteString(generated + "\n"); err != nil {
		file.Close()
		os.Remove(path)
		return "", fmt.Errorf("%w: write installation identity: %v", ErrInstallationIdentity, err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(path)
		return "", fmt.Errorf("%w: sync installation identity: %v", ErrInstallationIdentity, err)
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("%w: close installation identity: %v", ErrInstallationIdentity, err)
	}
	return generated, nil
}

func readInstallationID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.ToLower(strings.TrimSpace(string(data)))
	parsed, err := uuid.Parse(value)
	if err != nil || parsed.String() != value {
		return "", fmt.Errorf("installation identity file %s is invalid", path)
	}
	return value, nil
}

func filepathDir(path string) string {
	dir := filepath.Dir(path)
	if dir == "" {
		return "."
	}
	return dir
}

// LegacyMachineCode preserves validation for licenses issued before the
// installation identity binding was introduced.
func LegacyMachineCode() (string, error) {
	identity, source := platformIdentity()
	if identity == "" {
		identity = fallbackIdentity()
		source = "host"
	}
	if identity == "" {
		return "", fmt.Errorf("cannot determine a stable machine identity")
	}
	return machineCodeFromIdentity(runtime.GOOS + "|" + source + "|" + identity), nil
}

func platformIdentity() (string, string) {
	switch runtime.GOOS {
	case "linux":
		for _, path := range stableIDFiles {
			value, err := os.ReadFile(path)
			if err == nil && strings.TrimSpace(string(value)) != "" {
				return strings.TrimSpace(string(value)), "machine-id"
			}
		}
	case "darwin":
		output, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
		if err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				if !strings.Contains(line, "IOPlatformUUID") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					value := strings.Trim(strings.TrimSpace(parts[1]), "\"")
					if value != "" {
						return value, "platform-uuid"
					}
				}
			}
		}
	case "windows":
		output, err := exec.Command("wmic", "csproduct", "get", "UUID").Output()
		if err == nil {
			lines := strings.Fields(string(output))
			if len(lines) > 1 && !strings.EqualFold(lines[1], "UUID") {
				return lines[1], "platform-uuid"
			}
		}
	}
	return "", ""
}

func fallbackIdentity() string {
	hostname, _ := os.Hostname()
	var macs []string
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || len(iface.HardwareAddr) == 0 {
			continue
		}
		macs = append(macs, strings.ToLower(iface.HardwareAddr.String()))
	}
	sort.Strings(macs)
	return strings.TrimSpace(hostname) + "|" + strings.Join(macs, ",")
}

func machineCodeFromIdentity(identity string) string {
	sum := sha256.Sum256([]byte(ProductName + "|machine|v1|" + strings.TrimSpace(identity)))
	hexValue := strings.ToUpper(hex.EncodeToString(sum[:16]))
	groups := make([]string, 0, 8)
	for i := 0; i < len(hexValue); i += 4 {
		groups = append(groups, hexValue[i:i+4])
	}
	return "AOPS-" + strings.Join(groups, "-")
}
