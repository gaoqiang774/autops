package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dodevops-api/api/system/dao"
	"dodevops-api/api/system/model"
	"dodevops-api/common/util"
	"dodevops-api/common/util/recording"
)

type SessionRecordingService struct {
	storage       recording.Storage
	legacyStorage recording.Storage
}

func NewSessionRecordingService(storagePath string) (*SessionRecordingService, error) {
	if strings.TrimSpace(storagePath) == "" {
		storagePath = recording.DefaultLocalStoragePath()
	}

	storage, err := recording.NewLocalStorage(storagePath)
	if err != nil {
		return nil, fmt.Errorf("initialize recording storage failed: %w", err)
	}

	var legacyStorage recording.Storage
	legacyPath := recording.LegacyLocalStoragePath()
	if !sameStoragePath(storagePath, legacyPath) {
		if fallbackStorage, fallbackErr := recording.NewLocalStorage(legacyPath); fallbackErr == nil {
			legacyStorage = fallbackStorage
		}
	}

	return &SessionRecordingService{
		storage:       storage,
		legacyStorage: legacyStorage,
	}, nil
}

func sameStoragePath(a, b string) bool {
	return filepath.Clean(strings.TrimSpace(a)) == filepath.Clean(strings.TrimSpace(b))
}

func (s *SessionRecordingService) resolveStorage(sessionID string) (recording.Storage, error) {
	exists, err := s.storage.Exists(sessionID)
	if err != nil {
		return nil, err
	}
	if exists {
		return s.storage, nil
	}

	if s.legacyStorage != nil {
		exists, err = s.legacyStorage.Exists(sessionID)
		if err != nil {
			return nil, err
		}
		if exists {
			return s.legacyStorage, nil
		}
	}

	return s.storage, nil
}

func (s *SessionRecordingService) resolveRecordingFilePath(rec *model.SysSessionRecording) (string, error) {
	if rec.FilePath != "" {
		return rec.FilePath, nil
	}

	storage, err := s.resolveStorage(rec.SessionID)
	if err != nil {
		return "", err
	}
	return storage.GetPath(rec.SessionID), nil
}

func (s *SessionRecordingService) CreateRecorder(metadata recording.RecordingMetadata) recording.Recorder {
	return recording.NewAsciicastRecorder(s.storage, metadata)
}

func (s *SessionRecordingService) StartRecording(dto *model.CreateSessionRecordingDto) (*model.SysSessionRecording, error) {
	rec := &model.SysSessionRecording{
		SessionID:      dto.SessionID,
		AdminID:        dto.AdminID,
		Username:       dto.Username,
		HostID:         dto.HostID,
		HostName:       dto.HostName,
		HostIP:         dto.HostIP,
		SSHUser:        dto.SSHUser,
		TerminalWidth:  dto.TerminalWidth,
		TerminalHeight: dto.TerminalHeight,
		ClientIP:       dto.ClientIP,
		UserAgent:      dto.UserAgent,
		StartTime:      util.HTime{Time: time.Now()},
		Status:         model.SessionStatusRecording,
		StorageType:    model.StorageTypeLocal,
		FilePath:       "",
		CreateTime:     util.HTime{Time: time.Now()},
	}

	if err := dao.CreateSessionRecording(rec); err != nil {
		return nil, fmt.Errorf("create recording failed: %w", err)
	}

	return rec, nil
}

func (s *SessionRecordingService) FinishRecording(sessionID string, info *recording.RecordingInfo, recorder recording.Recorder) error {
	rec, err := dao.GetSessionRecordingBySessionID(sessionID)
	if err != nil {
		return fmt.Errorf("get recording failed: %w", err)
	}

	if asciiRecorder, ok := recorder.(*recording.AsciicastRecorder); ok {
		commands := asciiRecorder.GetParsedCommands()
		if len(commands) > 0 {
			cmdAuditService := NewCommandAuditService()

			if err := cmdAuditService.SaveCommands(rec.ID, sessionID, rec.StartTime.Time, commands); err != nil {
				fmt.Printf("save command audit failed: %v\n", err)
			}

			riskLevel, err := cmdAuditService.GetHighestRiskLevel(rec.ID)
			if err == nil && riskLevel > 0 {
				info.RiskLevel = riskLevel
			}

			sensitiveCount, err := cmdAuditService.GetSensitiveCommandCount(rec.ID)
			if err == nil && sensitiveCount > 0 {
				info.HasSensitiveCmd = true
			}
		}
	}

	updates := map[string]interface{}{
		"file_path":     info.FilePath,
		"file_size":     info.FileSize,
		"duration":      info.Duration,
		"input_count":   info.InputCount,
		"output_count":  info.OutputCount,
		"resize_count":  info.ResizeCount,
		"command_count": info.CommandCount,
		"end_time":      util.HTime{Time: time.Now()},
		"status":        model.SessionStatusCompleted,
		"update_time":   util.HTime{Time: time.Now()},
	}

	if info.RiskLevel > 0 {
		updates["risk_level"] = info.RiskLevel
	}
	if info.HasSensitiveCmd {
		updates["has_sensitive_cmd"] = true
	}

	if err := dao.UpdateSessionRecordingBySessionID(sessionID, updates); err != nil {
		return fmt.Errorf("update recording failed: %w", err)
	}

	return nil
}

func (s *SessionRecordingService) FailRecording(sessionID string, errMsg string) error {
	updates := map[string]interface{}{
		"status":      model.SessionStatusFailed,
		"error_msg":   errMsg,
		"end_time":    util.HTime{Time: time.Now()},
		"update_time": util.HTime{Time: time.Now()},
	}

	return dao.UpdateSessionRecordingBySessionID(sessionID, updates)
}

func (s *SessionRecordingService) GetRecordingByID(id uint) (*model.SysSessionRecording, error) {
	return dao.GetSessionRecordingByID(id)
}

func (s *SessionRecordingService) GetRecordingList(dto model.SessionRecordingListDto) ([]model.SysSessionRecording, int64, error) {
	return dao.GetSessionRecordingList(dto)
}

func (s *SessionRecordingService) GetRecordingContent(id uint) (io.ReadCloser, error) {
	rec, err := dao.GetSessionRecordingByID(id)
	if err != nil {
		return nil, fmt.Errorf("get recording failed: %w", err)
	}

	if rec.FilePath != "" {
		file, err := os.Open(rec.FilePath)
		if err != nil {
			return nil, fmt.Errorf("open recording file failed: %w", err)
		}
		return file, nil
	}

	storage, err := s.resolveStorage(rec.SessionID)
	if err != nil {
		return nil, fmt.Errorf("resolve recording storage failed: %w", err)
	}

	reader, err := storage.Load(rec.SessionID)
	if err != nil {
		return nil, fmt.Errorf("load recording file failed: %w", err)
	}

	return reader, nil
}

func (s *SessionRecordingService) DeleteRecording(id uint) error {
	rec, err := dao.GetSessionRecordingByID(id)
	if err != nil {
		return fmt.Errorf("get recording failed: %w", err)
	}

	if rec.FilePath != "" {
		_ = os.Remove(rec.FilePath)
	} else {
		storage, err := s.resolveStorage(rec.SessionID)
		if err != nil {
			return fmt.Errorf("resolve recording storage failed: %w", err)
		}
		if err := storage.Delete(rec.SessionID); err != nil {
			fmt.Printf("delete recording file failed: %v\n", err)
		}
	}

	if err := dao.DeleteSessionRecordingByID(id); err != nil {
		return fmt.Errorf("delete recording failed: %w", err)
	}

	return nil
}

func (s *SessionRecordingService) BatchDeleteRecording(ids []uint) error {
	for _, id := range ids {
		if err := s.DeleteRecording(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *SessionRecordingService) DownloadRecording(id uint) (string, *os.File, error) {
	rec, err := dao.GetSessionRecordingByID(id)
	if err != nil {
		return "", nil, fmt.Errorf("get recording failed: %w", err)
	}

	filename := fmt.Sprintf("session_%s_%s@%s.cast",
		rec.StartTime.Format("20060102_150405"),
		rec.SSHUser,
		rec.HostIP,
	)

	filePath, err := s.resolveRecordingFilePath(rec)
	if err != nil {
		return "", nil, fmt.Errorf("resolve recording file path failed: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", nil, fmt.Errorf("open recording file failed: %w", err)
	}

	return filename, file, nil
}
