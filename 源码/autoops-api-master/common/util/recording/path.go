package recording

import (
	"path/filepath"
	"strings"

	"dodevops-api/common/config"
)

const sessionRecordingDir = "video"

func uploadRoot() string {
	uploadDir := "./upload"
	if config.Config != nil {
		if dir := strings.TrimSpace(config.Config.ImageSettings.UploadDir); dir != "" {
			uploadDir = dir
		}
	}
	return filepath.Clean(uploadDir)
}

// DefaultLocalStoragePath returns the local storage path for session recordings.
func DefaultLocalStoragePath() string {
	return filepath.Join(uploadRoot(), sessionRecordingDir)
}

// LegacyLocalStoragePath returns the pre-migration local storage path.
func LegacyLocalStoragePath() string {
	return uploadRoot()
}
