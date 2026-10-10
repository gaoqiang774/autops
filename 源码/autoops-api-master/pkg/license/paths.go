package license

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	DataDirEnv            = "AUTOOPS_DATA_DIR"
	LicenseFileEnv        = "AUTOOPS_LICENSE_FILE"
	InstallationIDFileEnv = "AUTOOPS_INSTALLATION_ID_FILE"
)

// DataDir contains both the installation identity and the installed license.
// Production deployments should set AUTOOPS_DATA_DIR to an absolute,
// persistent path. The relative default preserves existing deployments.
func DataDir() string {
	if dir := strings.TrimSpace(os.Getenv(DataDirEnv)); dir != "" {
		return filepath.Clean(dir)
	}
	if path := strings.TrimSpace(os.Getenv(LicenseFileEnv)); path != "" {
		return filepath.Dir(filepath.Clean(path))
	}
	return "data"
}

func DefaultPath() string {
	if path := strings.TrimSpace(os.Getenv(LicenseFileEnv)); path != "" {
		return filepath.Clean(path)
	}
	return filepath.Join(DataDir(), "license.key")
}

func InstallationIDPath() string {
	if path := strings.TrimSpace(os.Getenv(InstallationIDFileEnv)); path != "" {
		return filepath.Clean(path)
	}
	return filepath.Join(DataDir(), "instance.id")
}
