package service

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common/config"
	cmdbwinrm "dodevops-api/common/winrm"

	"gorm.io/gorm"
)

const (
	defaultWindowsExporterVersion    = "0.31.3"
	defaultWindowsExporterPort       = 9182
	defaultWindowsExporterCollectors = "cpu,logical_disk,net,os,system,service,memory,process"
	defaultWindowsExporterPushIntvl  = 60
)

var unsupportedWindowsExporterCollectors = map[string]struct{}{
	"cs": {},
}

var (
	ErrWindowsExporterDeviceNotFound      = errors.New("windows exporter device not found")
	ErrWindowsExporterDeviceTypeInvalid   = errors.New("windows exporter device type invalid")
	ErrWindowsExporterRemoteCredsRequired = errors.New("windows exporter remote credentials required")
)

type WindowsExporterService struct {
	statusDao  *dao.CmdbWindowsExporterStatusDao
	hostDao    dao.CmdbHostDao
	ecsAuthDao EcsAuthDao
}

func NewWindowsExporterService() *WindowsExporterService {
	return &WindowsExporterService{
		statusDao:  dao.NewCmdbWindowsExporterStatusDao(),
		hostDao:    dao.NewCmdbHostDao(),
		ecsAuthDao: NewEcsAuthDao(),
	}
}

type InstallExporterRequest struct {
	DeviceID     uint   `json:"deviceId" binding:"required"`
	InstallerURL string `json:"installerUrl"`
	Version      string `json:"version"`
	ExporterPort int    `json:"exporterPort"`
}

type ExporterStatusResponse struct {
	DeviceID    uint   `json:"deviceId"`
	Installed   bool   `json:"installed"`
	Running     bool   `json:"running"`
	Version     string `json:"version"`
	Port        int    `json:"port"`
	Status      int    `json:"status"`
	StatusText  string `json:"statusText"`
	StatusColor string `json:"statusColor"`
	LastError   string `json:"lastError,omitempty"`
}

type ExporterPrecheckItem struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

type ExporterPrecheckResponse struct {
	DeviceID  uint                   `json:"deviceId,omitempty"`
	Host      string                 `json:"host"`
	Port      int                    `json:"port"`
	Ready     bool                   `json:"ready"`
	CheckedAt string                 `json:"checkedAt"`
	Items     []ExporterPrecheckItem `json:"items"`
}

type ExporterPrecheckRequest struct {
	DeviceID  uint   `json:"deviceId"`
	Host      string `json:"host"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Domain    string `json:"domain"`
	WinRMPort int    `json:"winrmPort"`
}

type ExporterInstallLogResponse struct {
	DeviceID                      uint     `json:"deviceId"`
	Host                          string   `json:"host"`
	Installed                     bool     `json:"installed"`
	Running                       bool     `json:"running"`
	ServiceExists                 bool     `json:"serviceExists"`
	ServiceStatus                 string   `json:"serviceStatus"`
	Version                       string   `json:"version"`
	LogPath                       string   `json:"logPath"`
	InstallLog                    string   `json:"installLog"`
	ServiceConfig                 string   `json:"serviceConfig"`
	MetricsReady                  bool     `json:"metricsReady"`
	VictoriaMetricsTaskExists     bool     `json:"victoriaMetricsTaskExists"`
	VictoriaMetricsTaskState      string   `json:"victoriaMetricsTaskState"`
	VictoriaMetricsTaskLastRun    string   `json:"victoriaMetricsTaskLastRun"`
	VictoriaMetricsTaskLastResult int64    `json:"victoriaMetricsTaskLastResult"`
	VictoriaMetricsError          string   `json:"victoriaMetricsError"`
	ApplicationEvents             []string `json:"applicationEvents"`
	SystemEvents                  []string `json:"systemEvents"`
	RetrievedAt                   string   `json:"retrievedAt"`
}

type exporterPrecheckConnectionInfo struct {
	Host     string
	Username string
	Password string
	Domain   string
}

func (s *WindowsExporterService) InstallWindowsExporter(req *InstallExporterRequest) error {
	if req == nil {
		return fmt.Errorf("install request is nil")
	}

	host, err := s.resolveManagedWindowsHost(req.DeviceID)
	if err != nil {
		return err
	}
	if err := s.updateExporterStatus(host.ID, map[string]interface{}{
		"status":     2,
		"last_error": "",
	}); err != nil {
		return fmt.Errorf("update exporter status failed: %w", err)
	}

	connInfo, err := s.resolvePrecheckConnectionInfo(&ExporterPrecheckRequest{DeviceID: req.DeviceID})
	if err != nil {
		s.setExporterFailure(host.ID, 4, err)
		return err
	}
	if connInfo == nil || strings.TrimSpace(connInfo.Username) == "" || strings.TrimSpace(connInfo.Password) == "" {
		s.setExporterFailure(host.ID, 4, ErrWindowsExporterRemoteCredsRequired)
		return ErrWindowsExporterRemoteCredsRequired
	}

	version, exporterPort, collectors, installerURL, victoriaMetricsImportURL, importInterval, err := s.resolveInstallOptions(req)
	if err != nil {
		s.setExporterFailure(host.ID, 4, err)
		return err
	}

	if err := s.updateExporterStatus(host.ID, map[string]interface{}{
		"port":    exporterPort,
		"version": version,
	}); err != nil {
		return fmt.Errorf("update exporter status failed: %w", err)
	}

	winrmMgr, err := newWindowsExporterWinRMManager(connInfo)
	if err != nil {
		s.setExporterFailure(host.ID, 4, err)
		return err
	}

	if err := winrmMgr.TestConnection(); err != nil {
		installErr := fmt.Errorf("winrm connection failed: %w", err)
		s.setExporterFailure(host.ID, 4, installErr)
		return installErr
	}

	grouping := buildVictoriaMetricsGrouping(host, connInfo.Host)
	log.Printf(
		"install windows exporter for host %d: metrics import url=%s, interval=%ds",
		host.ID,
		victoriaMetricsImportURL,
		importInterval,
	)

	if err := winrmMgr.InstallWindowsExporter(installerURL, version, exporterPort, collectors, victoriaMetricsImportURL, importInterval, chooseString(host.Name, host.HostName), grouping); err != nil {
		status := 4
		errText := strings.ToLower(err.Error())
		if strings.Contains(errText, "service failed to start") || strings.Contains(errText, "start-service") || strings.Contains(errText, "metrics endpoint") {
			status = 3
		}
		installErr := fmt.Errorf("install windows exporter failed: %w", err)
		s.setExporterFailure(host.ID, status, installErr)
		return installErr
	}

	if err := s.updateExporterStatus(host.ID, map[string]interface{}{
		"status":     1,
		"port":       exporterPort,
		"version":    version,
		"last_error": "",
	}); err != nil {
		return fmt.Errorf("update exporter success status failed: %w", err)
	}

	return nil
}

func buildVictoriaMetricsGrouping(host *model.CmdbHost, connectionHost string) *cmdbwinrm.VictoriaMetricsGrouping {
	if host == nil {
		return &cmdbwinrm.VictoriaMetricsGrouping{Job: connectionHost, Hostname: connectionHost}
	}
	return &cmdbwinrm.VictoriaMetricsGrouping{
		Job:       chooseString(strings.TrimSpace(host.PrivateIP), strings.TrimSpace(host.SSHIP), strings.TrimSpace(host.PublicIP), connectionHost),
		GroupID:   fmt.Sprintf("%d", host.GroupID),
		GroupName: strings.TrimSpace(host.Group.Name),
		Hostname:  chooseString(strings.TrimSpace(host.Name), strings.TrimSpace(host.HostName), connectionHost),
	}
}

func (s *WindowsExporterService) PrecheckWindowsExporter(req *ExporterPrecheckRequest) (*ExporterPrecheckResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("precheck request is nil")
	}

	port := req.WinRMPort
	if port == 0 {
		port = 5985
	}

	resp := &ExporterPrecheckResponse{
		DeviceID:  req.DeviceID,
		Host:      strings.TrimSpace(req.Host),
		Port:      port,
		CheckedAt: time.Now().Format("2006-01-02 15:04:05"),
		Items:     make([]ExporterPrecheckItem, 0, 4),
	}

	addItem := func(key, name string, passed bool, message, value string) {
		resp.Items = append(resp.Items, ExporterPrecheckItem{
			Key:     key,
			Name:    name,
			Passed:  passed,
			Message: message,
			Value:   value,
		})
	}

	username := strings.TrimSpace(req.Username)
	password := normalizeSnmpRemotePasswordInput(req.Password)
	domain := strings.TrimSpace(req.Domain)

	info, err := s.resolvePrecheckConnectionInfo(req)
	if err != nil {
		return nil, err
	}
	if resp.Host == "" {
		resp.Host = info.Host
	}
	if username == "" {
		username = info.Username
	}
	if password == "" {
		password = info.Password
	}
	if domain == "" {
		domain = info.Domain
	}

	if resp.Host == "" {
		addItem("target_host", "target_host", false, "host is required", "")
		resp.Ready = false
		return resp, nil
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", resp.Host, port), 3*time.Second)
	if err != nil {
		addItem("tcp_connect", "tcp_connect", false, "winrm port is unreachable", err.Error())
		resp.Ready = false
		return resp, nil
	}
	conn.Close()
	addItem("tcp_connect", "tcp_connect", true, "winrm port is reachable", fmt.Sprintf("%s:%d", resp.Host, port))

	if username == "" || strings.TrimSpace(password) == "" {
		addItem("winrm_command", "winrm_command", false, "username or password is missing", "")
		resp.Ready = false
		return resp, nil
	}

	winrmUsername := username
	if domain != "" && !strings.Contains(winrmUsername, `\`) && !strings.Contains(winrmUsername, "@") {
		winrmUsername = domain + `\` + winrmUsername
	}

	winrmMgr, err := cmdbwinrm.NewManager(cmdbwinrm.Config{
		Host:     resp.Host,
		Port:     port,
		Username: winrmUsername,
		Password: password,
		UseHTTPS: false,
	})
	if err != nil {
		addItem("winrm_command", "winrm_command", false, "create winrm client failed", err.Error())
		resp.Ready = false
		return resp, nil
	}

	output, err := winrmMgr.RunCommand("Write-Output $env:COMPUTERNAME")
	if err != nil {
		addItem("winrm_command", "winrm_command", false, "winrm command execution failed", err.Error())
		resp.Ready = false
		return resp, nil
	}

	addItem("winrm_command", "winrm_command", true, "winrm command execution succeeded", strings.TrimSpace(output))
	resp.Ready = true
	return resp, nil
}

func (s *WindowsExporterService) resolvePrecheckConnectionInfo(req *ExporterPrecheckRequest) (*exporterPrecheckConnectionInfo, error) {
	info := &exporterPrecheckConnectionInfo{
		Host:     strings.TrimSpace(req.Host),
		Username: strings.TrimSpace(req.Username),
		Password: normalizeSnmpRemotePasswordInput(req.Password),
		Domain:   strings.TrimSpace(req.Domain),
	}

	if req.DeviceID == 0 {
		return info, nil
	}

	if info.Host != "" && info.Username != "" && info.Password != "" {
		return info, nil
	}

	host, err := s.hostDao.GetCmdbHostById(req.DeviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return info, nil
		}
		return nil, fmt.Errorf("get host failed: %w", err)
	}

	if info.Host == "" {
		info.Host = chooseString(strings.TrimSpace(host.SSHIP), strings.TrimSpace(host.PrivateIP), strings.TrimSpace(host.PublicIP))
	}
	if info.Username == "" {
		info.Username = strings.TrimSpace(host.SSHName)
	}

	if host.SSHKeyID > 0 && info.Password == "" {
		auth, err := s.ecsAuthDao.GetEcsAuthById(host.SSHKeyID)
		if err != nil {
			return nil, fmt.Errorf("get credential failed: %w", err)
		}
		if info.Username == "" {
			info.Username = strings.TrimSpace(auth.Username)
		}
		if info.Password == "" {
			info.Password = chooseString(auth.Password, auth.TargetPassword)
		}
	}

	return info, nil
}

func (s *WindowsExporterService) GetExporterStatus(deviceID uint) (*ExporterStatusResponse, error) {
	host, err := s.resolveManagedWindowsHost(deviceID)
	if err != nil {
		return nil, err
	}

	state, stateErr := s.statusDao.GetByHostID(host.ID)
	if stateErr != nil && !errors.Is(stateErr, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("get exporter status failed: %w", stateErr)
	}
	status, version, port, lastError := 0, "", defaultWindowsExporterPort, ""
	if state != nil {
		status, version, port, lastError = state.Status, state.Version, chooseInt(state.Port, defaultWindowsExporterPort), state.LastError
	}
	resp := buildExporterStatusResponse(deviceID, status, version, port)
	resp.LastError = lastError

	if status == 2 {
		return resp, nil
	}
	connInfo, connErr := s.resolvePrecheckConnectionInfo(&ExporterPrecheckRequest{DeviceID: host.ID})
	if connErr != nil || connInfo == nil || strings.TrimSpace(connInfo.Username) == "" || strings.TrimSpace(connInfo.Password) == "" {
		return resp, nil
	}
	winrmMgr, managerErr := newWindowsExporterWinRMManager(connInfo)
	if managerErr != nil {
		return resp, nil
	}
	requireVictoriaMetrics := true
	importInterval := defaultWindowsExporterPushIntvl
	if config.Config != nil && config.Config.WindowsExporter.PushMode.Interval >= 60 {
		importInterval = config.Config.WindowsExporter.PushMode.Interval
	}
	installed, healthy, detectedVersion, healthReason, checkErr := winrmMgr.CheckExporterStatus(port, importInterval, requireVictoriaMetrics)
	if checkErr != nil {
		resp = buildExporterStatusResponse(deviceID, 3, version, port)
		resp.LastError = "Windows Exporter 状态检查失败: " + checkErr.Error()
		_ = s.updateExporterStatus(host.ID, map[string]interface{}{
			"status":     resp.Status,
			"last_error": resp.LastError,
		})
		return resp, nil
	}
	switch {
	case !installed:
		resp = buildExporterStatusResponse(deviceID, 0, "", port)
	case healthy:
		resp = buildExporterStatusResponse(deviceID, 1, chooseString(detectedVersion, version), port)
	default:
		resp = buildExporterStatusResponse(deviceID, 3, chooseString(detectedVersion, version), port)
		if (status == 3 || status == 4) && strings.HasPrefix(lastError, "install windows exporter failed:") {
			resp = buildExporterStatusResponse(deviceID, status, chooseString(detectedVersion, version), port)
			resp.LastError = lastError
		} else {
			resp.LastError = exporterHealthMessage(healthReason)
		}
	}
	_ = s.updateExporterStatus(host.ID, map[string]interface{}{
		"status":     resp.Status,
		"port":       resp.Port,
		"version":    resp.Version,
		"last_error": resp.LastError,
	})

	return resp, nil
}

func exporterHealthMessage(reason string) string {
	switch reason {
	case "service_not_running":
		return "Windows Exporter 服务未运行"
	case "metrics_endpoint_unavailable":
		return "Windows Exporter /metrics 接口无有效指标"
	case "victoriametrics_task_missing":
		return "VictoriaMetrics 上报任务不存在，请重新安装 Windows Exporter"
	case "victoriametrics_task_never_ran":
		return "VictoriaMetrics 上报任务尚未成功执行，请检查计划任务"
	}
	if strings.HasPrefix(reason, "victoriametrics_task_state:") {
		return "VictoriaMetrics 上报任务状态异常: " + strings.TrimPrefix(reason, "victoriametrics_task_state:")
	}
	if strings.HasPrefix(reason, "victoriametrics_task_failed:") {
		return "VictoriaMetrics 上报任务执行失败，结果码: " + strings.TrimPrefix(reason, "victoriametrics_task_failed:")
	}
	if strings.HasPrefix(reason, "victoriametrics_task_stale:") {
		return "VictoriaMetrics 上报任务长时间未执行，距上次运行约 " + strings.TrimPrefix(reason, "victoriametrics_task_stale:") + " 秒"
	}
	return "Windows Exporter 运行状态异常"
}

func (s *WindowsExporterService) UninstallExporter(deviceID uint) error {
	host, err := s.resolveManagedWindowsHost(deviceID)
	if err != nil {
		return err
	}
	connInfo, err := s.resolvePrecheckConnectionInfo(&ExporterPrecheckRequest{DeviceID: host.ID})
	if err != nil {
		return err
	}
	if connInfo == nil || strings.TrimSpace(connInfo.Username) == "" || strings.TrimSpace(connInfo.Password) == "" {
		return ErrWindowsExporterRemoteCredsRequired
	}
	winrmMgr, err := newWindowsExporterWinRMManager(connInfo)
	if err != nil {
		return err
	}

	if err := winrmMgr.UninstallExporter(); err != nil {
		return err
	}

	return s.updateExporterStatus(host.ID, map[string]interface{}{
		"status":     0,
		"port":       defaultWindowsExporterPort,
		"version":    "",
		"last_error": "",
	})
}

func (s *WindowsExporterService) GetExporterInstallLog(deviceID uint) (*ExporterInstallLogResponse, error) {
	info, err := s.resolvePrecheckConnectionInfo(&ExporterPrecheckRequest{DeviceID: deviceID})
	if err != nil {
		return nil, err
	}
	if info == nil || strings.TrimSpace(info.Host) == "" {
		return nil, ErrWindowsExporterDeviceNotFound
	}
	if strings.TrimSpace(info.Username) == "" || strings.TrimSpace(info.Password) == "" {
		return nil, ErrWindowsExporterRemoteCredsRequired
	}

	winrmUsername := info.Username
	if info.Domain != "" && !strings.Contains(winrmUsername, `\`) && !strings.Contains(winrmUsername, "@") {
		winrmUsername = info.Domain + `\` + winrmUsername
	}

	winrmMgr, err := cmdbwinrm.NewManager(cmdbwinrm.Config{
		Host:     info.Host,
		Port:     5985,
		Username: winrmUsername,
		Password: info.Password,
		UseHTTPS: false,
	})
	if err != nil {
		return nil, err
	}

	diagnostics, err := winrmMgr.GetExporterDiagnostics()
	if err != nil {
		return nil, err
	}

	return &ExporterInstallLogResponse{
		DeviceID:                      deviceID,
		Host:                          info.Host,
		Installed:                     diagnostics.Installed,
		Running:                       diagnostics.Running,
		ServiceExists:                 diagnostics.ServiceExists,
		ServiceStatus:                 diagnostics.ServiceStatus,
		Version:                       diagnostics.Version,
		LogPath:                       diagnostics.LogPath,
		InstallLog:                    diagnostics.InstallLog,
		ServiceConfig:                 diagnostics.ServiceConfig,
		MetricsReady:                  diagnostics.MetricsReady,
		VictoriaMetricsTaskExists:     diagnostics.VictoriaMetricsTaskExists,
		VictoriaMetricsTaskState:      diagnostics.VictoriaMetricsTaskState,
		VictoriaMetricsTaskLastRun:    diagnostics.VictoriaMetricsTaskLastRun,
		VictoriaMetricsTaskLastResult: diagnostics.VictoriaMetricsTaskLastResult,
		VictoriaMetricsError:          diagnostics.VictoriaMetricsError,
		ApplicationEvents:             diagnostics.ApplicationEvents,
		SystemEvents:                  diagnostics.SystemEvents,
		RetrievedAt:                   time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func (s *WindowsExporterService) BatchInstallExporter(req *InstallExporterRequest, deviceIDs []uint) map[uint]error {
	results := make(map[uint]error, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		installReq := &InstallExporterRequest{}
		if req != nil {
			*installReq = *req
		}
		installReq.DeviceID = deviceID
		results[deviceID] = s.InstallWindowsExporter(installReq)
	}
	return results
}

func (s *WindowsExporterService) GetAllWindowsDevicesStatus() ([]*ExporterStatusResponse, error) {
	hosts := s.hostDao.GetCmdbHostsByHostType(model.HostTypeWindows)
	statuses := make([]*ExporterStatusResponse, 0, len(hosts))
	for _, host := range hosts {
		status, statusErr := s.GetExporterStatus(host.ID)
		if statusErr != nil {
			log.Printf("get exporter status failed for host %d: %v", host.ID, statusErr)
			continue
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

func (s *WindowsExporterService) GetInitWinRMScriptPath() (string, string, error) {
	uploadDir := "./upload"
	if config.Config != nil && strings.TrimSpace(config.Config.ImageSettings.UploadDir) != "" {
		uploadDir = config.Config.ImageSettings.UploadDir
	}

	scriptPath := filepath.Join(uploadDir, "windows_exporter", "init_winrm.ps1")
	absPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve init_winrm.ps1 path failed: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("init_winrm.ps1 not found")
		}
		return "", "", fmt.Errorf("stat init_winrm.ps1 failed: %w", err)
	}

	return absPath, "init_winrm.ps1", nil
}

func (s *WindowsExporterService) resolveManagedWindowsHost(hostID uint) (*model.CmdbHost, error) {
	host, err := s.hostDao.GetCmdbHostById(hostID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWindowsExporterDeviceNotFound
		}
		return nil, fmt.Errorf("get host failed: %w", err)
	}

	if host.HostType != model.HostTypeWindows && !strings.Contains(strings.ToLower(host.OS), "windows") {
		return nil, ErrWindowsExporterDeviceTypeInvalid
	}
	return &host, nil
}

func (s *WindowsExporterService) resolveInstallOptions(req *InstallExporterRequest) (string, int, string, string, string, int, error) {
	cfg := config.WindowsExporterConfig{}
	if config.Config != nil {
		cfg = config.Config.WindowsExporter
	}

	version := strings.TrimSpace(req.Version)
	if version == "" {
		version = strings.TrimSpace(cfg.DefaultVersion)
	}
	if version == "" {
		version = defaultWindowsExporterVersion
	}

	exporterPort := req.ExporterPort
	if exporterPort == 0 {
		exporterPort = cfg.DefaultPort
	}
	if exporterPort == 0 {
		exporterPort = defaultWindowsExporterPort
	}
	if exporterPort < 1 || exporterPort > 65535 {
		return "", 0, "", "", "", 0, fmt.Errorf("exporter port %d is invalid", exporterPort)
	}

	collectors := strings.TrimSpace(cfg.EnabledCollectors)
	if collectors == "" {
		collectors = defaultWindowsExporterCollectors
	}
	collectors = sanitizeWindowsExporterCollectors(collectors)

	installerURL := strings.TrimSpace(req.InstallerURL)
	if installerURL == "" {
		baseURL := strings.TrimRight(strings.TrimSpace(cfg.InstallerBaseURL), "/")
		if baseURL == "" {
			baseURL = defaultInstallerBaseURL()
		}
		installerURL = fmt.Sprintf("%s/windows_exporter-%s-amd64.msi", baseURL, version)
	}
	if _, err := url.ParseRequestURI(installerURL); err != nil {
		return "", 0, "", "", "", 0, fmt.Errorf("installer url is invalid: %w", err)
	}

	if !cfg.PushMode.Enabled {
		return "", 0, "", "", "", 0, errors.New("Windows Exporter VictoriaMetrics direct import is disabled")
	}

	victoriaMetricsImportURL := ""
	if config.Config != nil {
		victoriaMetricsImportURL = strings.TrimSpace(config.Config.Monitor.MetricsPush.URL)
	}
	if victoriaMetricsImportURL == "" {
		victoriaMetricsImportURL = strings.TrimSpace(cfg.PushMode.MetricsPushURL)
	}
	if victoriaMetricsImportURL == "" && config.Config != nil {
		baseURL := strings.TrimRight(strings.TrimSpace(config.Config.Monitor.VictoriaMetrics.URL), "/")
		if baseURL != "" {
			victoriaMetricsImportURL = baseURL + "/api/v1/import/prometheus"
		}
	}

	if victoriaMetricsImportURL == "" {
		return "", 0, "", "", "", 0, errors.New("VictoriaMetrics import URL is required")
	}
	parsedImportURL, err := url.ParseRequestURI(victoriaMetricsImportURL)
	if err != nil || parsedImportURL.Scheme == "" || parsedImportURL.Host == "" {
		return "", 0, "", "", "", 0, fmt.Errorf("VictoriaMetrics import URL is invalid: %s", victoriaMetricsImportURL)
	}

	importInterval := cfg.PushMode.Interval
	if importInterval < 60 {
		importInterval = defaultWindowsExporterPushIntvl
	}

	return version, exporterPort, collectors, installerURL, victoriaMetricsImportURL, importInterval, nil
}

func newWindowsExporterWinRMManager(info *exporterPrecheckConnectionInfo) (*cmdbwinrm.Manager, error) {
	if info == nil {
		return nil, ErrWindowsExporterRemoteCredsRequired
	}
	username := strings.TrimSpace(info.Username)
	if domain := strings.TrimSpace(info.Domain); domain != "" && !strings.Contains(username, `\`) && !strings.Contains(username, "@") {
		username = domain + `\` + username
	}

	return cmdbwinrm.NewManager(cmdbwinrm.Config{
		Host:     info.Host,
		Port:     5985,
		Username: username,
		Password: info.Password,
		UseHTTPS: false,
	})
}

func (s *WindowsExporterService) updateExporterStatus(hostID uint, fields map[string]interface{}) error {
	return s.statusDao.Save(hostID, fields)
}

func (s *WindowsExporterService) setExporterFailure(hostID uint, status int, err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	_ = s.updateExporterStatus(hostID, map[string]interface{}{
		"status":     status,
		"last_error": message,
	})
}

func defaultInstallerBaseURL() string {
	if config.Config != nil {
		publicURL := strings.TrimRight(strings.TrimSpace(config.Config.Server.PublicURL), "/")
		if publicURL != "" {
			return config.BuildRouteURL(publicURL, "/api/v1/upload/windows_exporter")
		}
	}
	return config.BuildRouteURL("http://127.0.0.1:8000", "/api/v1/upload/windows_exporter")
}

func buildExporterStatusResponse(deviceID uint, status int, version string, port int) *ExporterStatusResponse {
	resp := &ExporterStatusResponse{
		DeviceID: deviceID,
		Version:  version,
		Port:     port,
		Status:   status,
	}

	switch status {
	case 0:
		resp.Installed = false
		resp.Running = false
		resp.StatusText = "not_installed"
		resp.StatusColor = "default"
	case 1:
		resp.Installed = true
		resp.Running = true
		resp.StatusText = "running"
		resp.StatusColor = "success"
	case 2:
		resp.Installed = false
		resp.Running = false
		resp.StatusText = "installing"
		resp.StatusColor = "processing"
	case 3:
		resp.Installed = true
		resp.Running = false
		resp.StatusText = "installed_not_running"
		resp.StatusColor = "warning"
	case 4:
		resp.Installed = false
		resp.Running = false
		resp.StatusText = "install_failed"
		resp.StatusColor = "error"
	case 5:
		resp.Installed = true
		resp.Running = false
		resp.StatusText = "starting"
		resp.StatusColor = "processing"
	default:
		resp.Installed = false
		resp.Running = false
		resp.StatusText = "unknown"
		resp.StatusColor = "default"
	}

	return resp
}

func chooseString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func chooseInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func sanitizeWindowsExporterCollectors(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return defaultWindowsExporterCollectors
	}

	parts := strings.Split(raw, ",")
	collectors := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		collector := strings.ToLower(strings.TrimSpace(part))
		if collector == "" {
			continue
		}
		if _, unsupported := unsupportedWindowsExporterCollectors[collector]; unsupported {
			continue
		}
		if _, exists := seen[collector]; exists {
			continue
		}
		seen[collector] = struct{}{}
		collectors = append(collectors, collector)
	}

	if len(collectors) == 0 {
		return defaultWindowsExporterCollectors
	}

	return strings.Join(collectors, ",")
}
