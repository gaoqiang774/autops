package service

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common/config"
	cmdbwinrm "dodevops-api/common/winrm"
)

type WindowsFileTransferService struct {
	snmpDeviceDao *dao.CmdbSnmpDeviceDao
	hostDao       dao.CmdbHostDao
	ecsAuthDao    EcsAuthDao
}

type WindowsFileTransferResult struct {
	HostID     uint   `json:"hostId"`
	FileName   string `json:"fileName"`
	Mode       string `json:"mode"`
	RemotePath string `json:"remotePath"`
}

var (
	windowsFileUploadTasks   sync.Map
	windowsFileUploadTaskSeq uint64
)

func NewWindowsFileTransferService() *WindowsFileTransferService {
	return &WindowsFileTransferService{
		snmpDeviceDao: dao.NewCmdbSnmpDeviceDao(),
		hostDao:       dao.NewCmdbHostDao(),
		ecsAuthDao:    NewEcsAuthDao(),
	}
}

func nextWindowsFileUploadTaskID() string {
	seq := atomic.AddUint64(&windowsFileUploadTaskSeq, 1)
	return fmt.Sprintf("winrm-upload-%d-%d", time.Now().UnixNano(), seq)
}

func cloneWindowsFileUploadTask(task *model.WindowsFileUploadTask) *model.WindowsFileUploadTask {
	if task == nil {
		return nil
	}
	copy := *task
	return &copy
}

func (s *WindowsFileTransferService) UploadFileToHostDownloads(hostID uint, fileHeader *multipart.FileHeader, requestedName, baseURL string) (*WindowsFileTransferResult, error) {
	localPath, fileName, relativePath, err := s.saveUploadToStaging(fileHeader, requestedName)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.Remove(localPath)
	}()

	return s.transferStagedFileToHostDownloads(hostID, localPath, fileName, relativePath, baseURL, nil)
}

func (s *WindowsFileTransferService) CreateUploadTask(hostID uint, fileHeader *multipart.FileHeader, requestedName, baseURL string, createdBy uint, createdByName string) (*model.WindowsFileUploadTask, error) {
	if hostID == 0 {
		return nil, fmt.Errorf("missing host id")
	}
	if fileHeader == nil {
		return nil, fmt.Errorf("missing upload file")
	}

	localPath, fileName, relativePath, err := s.saveUploadToStaging(fileHeader, requestedName)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(localPath)
	if err != nil {
		_ = os.Remove(localPath)
		return nil, fmt.Errorf("stat staging file failed: %w", err)
	}

	now := time.Now()
	task := &model.WindowsFileUploadTask{
		TaskID:        nextWindowsFileUploadTaskID(),
		HostID:        hostID,
		FileName:      fileName,
		FileSize:      fileInfo.Size(),
		Status:        model.WindowsFileUploadTaskPending,
		Progress:      0,
		UploadedBytes: 0,
		TotalBytes:    fileInfo.Size(),
		Message:       "任务已创建，等待上传",
		CreatedBy:     createdBy,
		CreatedByName: createdByName,
		CreatedAt:     now,
	}
	windowsFileUploadTasks.Store(task.TaskID, task)

	go func() {
		defer func() {
			_ = os.Remove(localPath)
		}()

		startedAt := time.Now()
		task.StartedAt = &startedAt
		task.Status = model.WindowsFileUploadTaskUploading
		task.Message = "正在准备 WinRM 上传"

		progress := func(uploaded, total int64, mode, message string) {
			task.Mode = mode
			task.UploadedBytes = uploaded
			task.TotalBytes = total
			if total > 0 {
				task.Progress = float64(uploaded) / float64(total) * 100
			}
			task.Message = message
		}

		transferResult, err := s.transferStagedFileToHostDownloads(hostID, localPath, fileName, relativePath, baseURL, progress)
		completedAt := time.Now()
		task.CompletedAt = &completedAt

		if err != nil {
			task.Status = model.WindowsFileUploadTaskFailed
			task.Error = err.Error()
			task.Message = "上传失败"
			log.Printf("[WinFileTransfer] async upload failed: taskId=%s hostId=%d file=%s err=%v", task.TaskID, hostID, fileName, err)
			return
		}

		task.Status = model.WindowsFileUploadTaskCompleted
		task.Progress = 100
		task.UploadedBytes = task.TotalBytes
		task.Mode = transferResult.Mode
		task.RemotePath = transferResult.RemotePath
		task.Message = "上传完成"
		log.Printf("[WinFileTransfer] async upload success: taskId=%s hostId=%d file=%s mode=%s remotePath=%s", task.TaskID, hostID, fileName, transferResult.Mode, transferResult.RemotePath)
	}()

	return cloneWindowsFileUploadTask(task), nil
}

func (s *WindowsFileTransferService) GetUploadTask(taskID string) (*model.WindowsFileUploadTask, error) {
	taskValue, ok := windowsFileUploadTasks.Load(taskID)
	if !ok {
		return nil, fmt.Errorf("upload task not found: %s", taskID)
	}

	task, ok := taskValue.(*model.WindowsFileUploadTask)
	if !ok {
		return nil, fmt.Errorf("upload task data invalid: %s", taskID)
	}

	return cloneWindowsFileUploadTask(task), nil
}

func (s *WindowsFileTransferService) transferStagedFileToHostDownloads(hostID uint, localPath, fileName, relativePath, baseURL string, progress cmdbwinrm.TransferProgressFunc) (*WindowsFileTransferResult, error) {
	if hostID == 0 {
		return nil, fmt.Errorf("missing host id")
	}

	host, err := s.hostDao.GetCmdbHostById(hostID)
	if err != nil {
		return nil, fmt.Errorf("get host failed: %w", err)
	}
	if host.HostType != model.HostTypeWindows && !strings.Contains(strings.ToLower(strings.TrimSpace(host.OS)), "windows") {
		return nil, fmt.Errorf("host is not windows")
	}

	winrmMgr, err := s.newHostWinRMManager(&host)
	if err != nil {
		return nil, err
	}
	if err := winrmMgr.TestConnection(); err != nil {
		return nil, fmt.Errorf("winrm connection failed: %w", err)
	}
	log.Printf("[WinFileTransfer] winrm ready: hostId=%d host=%s", hostID, winrmMgr.Host)

	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	fileURL := ""
	if baseURL != "" {
		fileURL = baseURL + "/api/v1/upload/" + relativePath
		fileURL = strings.ReplaceAll(fileURL, "\\", "/")
	}
	log.Printf("[WinFileTransfer] start upload: hostId=%d fileName=%s localPath=%s fileURL=%s", hostID, fileName, localPath, fileURL)

	remotePath, mode, err := winrmMgr.TransferFileToDownloadsWithProgress(fileURL, localPath, fileName, progress)
	if err != nil {
		log.Printf("[WinFileTransfer] upload failed: hostId=%d fileName=%s err=%v", hostID, fileName, err)
		return nil, err
	}
	log.Printf("[WinFileTransfer] upload success: hostId=%d fileName=%s mode=%s remotePath=%s", hostID, fileName, mode, remotePath)

	return &WindowsFileTransferResult{
		HostID:     hostID,
		FileName:   fileName,
		Mode:       mode,
		RemotePath: remotePath,
	}, nil
}

func (s *WindowsFileTransferService) newHostWinRMManager(host *model.CmdbHost) (*cmdbwinrm.Manager, error) {
	if host == nil {
		return nil, fmt.Errorf("host is nil")
	}

	if winrmMgr, ok := s.buildWinRMManagerFromWindowsDevice(host); ok {
		return winrmMgr, nil
	}

	hostAddr := chooseString(strings.TrimSpace(host.SSHIP), strings.TrimSpace(host.PrivateIP), strings.TrimSpace(host.PublicIP))
	if hostAddr == "" {
		return nil, fmt.Errorf("host address is empty")
	}

	username := strings.TrimSpace(host.SSHName)
	password := ""
	if host.SSHKeyID > 0 {
		auth, err := s.ecsAuthDao.GetEcsAuthById(host.SSHKeyID)
		if err != nil {
			return nil, fmt.Errorf("get credential failed: %w", err)
		}
		if username == "" {
			username = strings.TrimSpace(auth.Username)
		}
		password = chooseString(auth.TargetPassword, auth.Password)
	}

	if username == "" {
		return nil, fmt.Errorf("winrm username is empty")
	}
	if password == "" {
		return nil, fmt.Errorf("winrm password is empty")
	}

	return cmdbwinrm.NewManager(cmdbwinrm.Config{
		Host:     hostAddr,
		Port:     5985,
		Username: username,
		Password: password,
		UseHTTPS: false,
	})
}

func (s *WindowsFileTransferService) buildWinRMManagerFromWindowsDevice(host *model.CmdbHost) (*cmdbwinrm.Manager, bool) {
	if s.snmpDeviceDao == nil || host == nil {
		return nil, false
	}

	var device *model.CmdbSnmpDevice
	if byID, err := s.snmpDeviceDao.GetSnmpDeviceById(host.ID); err == nil {
		device = byID
	} else {
		hostIP := chooseString(strings.TrimSpace(host.PrivateIP), strings.TrimSpace(host.SSHIP), strings.TrimSpace(host.PublicIP))
		if hostIP != "" {
			if byIP, ipErr := s.snmpDeviceDao.GetSnmpDeviceByIP(hostIP); ipErr == nil {
				device = byIP
			}
		}
	}

	if device == nil {
		return nil, false
	}
	if !strings.EqualFold(strings.TrimSpace(device.DeviceType), "windows") {
		return nil, false
	}

	username := strings.TrimSpace(device.RemoteUsername)
	password, err := decryptSnmpDeviceRemotePassword(device)
	if err != nil {
		return nil, false
	}
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return nil, false
	}

	if domain := strings.TrimSpace(device.RemoteDomain); domain != "" && !strings.Contains(username, `\`) && !strings.Contains(username, "@") {
		username = domain + `\` + username
	}

	manager, err := cmdbwinrm.NewManager(cmdbwinrm.Config{
		Host:     chooseString(strings.TrimSpace(device.IPAddress), strings.TrimSpace(host.PrivateIP), strings.TrimSpace(host.SSHIP), strings.TrimSpace(host.PublicIP)),
		Port:     5985,
		Username: username,
		Password: password,
		UseHTTPS: false,
	})
	if err != nil {
		return nil, false
	}

	return manager, true
}

func (s *WindowsFileTransferService) saveUploadToStaging(fileHeader *multipart.FileHeader, requestedName string) (string, string, string, error) {
	if fileHeader == nil {
		return "", "", "", fmt.Errorf("missing upload file")
	}

	fileName := sanitizeUploadFileName(requestedName)
	if fileName == "" {
		fileName = sanitizeUploadFileName(fileHeader.Filename)
	}
	if fileName == "" {
		return "", "", "", fmt.Errorf("invalid file name")
	}

	uploadRoot := "./upload"
	if config.Config != nil && strings.TrimSpace(config.Config.ImageSettings.UploadDir) != "" {
		uploadRoot = strings.TrimSpace(config.Config.ImageSettings.UploadDir)
	}
	if !filepath.IsAbs(uploadRoot) {
		if absPath, err := filepath.Abs(uploadRoot); err == nil {
			uploadRoot = absPath
		}
	}

	subDir := filepath.Join("winrm", time.Now().Format("20060102"))
	stageDir := filepath.Join(uploadRoot, subDir)
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return "", "", "", fmt.Errorf("create staging dir failed: %w", err)
	}

	stageName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), fileName)
	localPath := filepath.Join(stageDir, stageName)

	src, err := fileHeader.Open()
	if err != nil {
		return "", "", "", fmt.Errorf("open uploaded file failed: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(localPath)
	if err != nil {
		return "", "", "", fmt.Errorf("create staging file failed: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", "", "", fmt.Errorf("save staging file failed: %w", err)
	}

	relativePath := filepath.ToSlash(filepath.Join(subDir, url.PathEscape(stageName)))
	return localPath, fileName, relativePath, nil
}
