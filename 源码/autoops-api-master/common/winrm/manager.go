package winrm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	gowinrm "github.com/masterzen/winrm"
)

const defaultTimeout = 60 * time.Second
const smallFileChunkPushThreshold = 5 * 1024 * 1024
const chunkPushChunkSize = 2 * 1024
const remoteTextChunkSize = 1024

type Manager struct {
	Host     string
	Port     int
	Username string
	Password string
	UseHTTPS bool
	Timeout  time.Duration
	client   *gowinrm.Client
}

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	UseHTTPS bool
}

type VictoriaMetricsGrouping struct {
	Job       string
	GroupID   string
	GroupName string
	Hostname  string
}

type ExporterDiagnostics struct {
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
}

type TransferProgressFunc func(uploaded, total int64, mode, message string)

func NewManager(cfg Config) (*Manager, error) {
	port := cfg.Port
	if port == 0 {
		if cfg.UseHTTPS {
			port = 5986
		} else {
			port = 5985
		}
	}

	endpoint := gowinrm.NewEndpoint(cfg.Host, port, cfg.UseHTTPS, false, nil, nil, nil, defaultTimeout)
	client, err := gowinrm.NewClient(endpoint, cfg.Username, cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("create winrm client failed: %w", err)
	}

	return &Manager{
		Host:     cfg.Host,
		Port:     port,
		Username: cfg.Username,
		Password: cfg.Password,
		UseHTTPS: cfg.UseHTTPS,
		Timeout:  defaultTimeout,
		client:   client,
	}, nil
}

func (m *Manager) RunCommand(command string) (string, error) {
	if m == nil || m.client == nil {
		return "", fmt.Errorf("winrm client is not initialized")
	}

	stdout, stderr, exitCode, err := m.client.RunPSWithString(command, "")
	if err != nil {
		return stdout, fmt.Errorf(
			"run powershell failed: %w, stdout: %s, stderr: %s",
			err,
			summarizeOutput(stdout),
			summarizeOutput(stderr),
		)
	}
	if exitCode != 0 {
		return stdout, fmt.Errorf(
			"powershell exit code %d, stdout: %s, stderr: %s",
			exitCode,
			summarizeOutput(stdout),
			summarizeOutput(stderr),
		)
	}

	return stdout, nil
}

func (m *Manager) TestConnection() error {
	output, err := m.RunCommand("Write-Output 'OK'")
	if err != nil {
		return err
	}
	if !strings.Contains(output, "OK") {
		return fmt.Errorf("unexpected winrm response: %s", strings.TrimSpace(output))
	}
	return nil
}

func (m *Manager) writeRemoteUTF8File(remotePath, content string) error {
	if strings.TrimSpace(remotePath) == "" {
		return errors.New("remote path is required")
	}

	initCmd := fmt.Sprintf(`
$path = %s
$parent = Split-Path -Parent $path
if (-not (Test-Path $parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
[System.IO.File]::WriteAllBytes($path, [byte[]]@())
`, psQuote(remotePath))
	if _, err := m.RunCommand(initCmd); err != nil {
		return fmt.Errorf("initialize remote text file failed: %w", err)
	}

	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte(content)...)
	for offset := 0; offset < len(data); offset += remoteTextChunkSize {
		end := offset + remoteTextChunkSize
		if end > len(data) {
			end = len(data)
		}
		chunkCmd := fmt.Sprintf(`
$path = %s
$bytes = [Convert]::FromBase64String(%s)
$stream = [System.IO.File]::Open($path, [System.IO.FileMode]::Append)
try { $stream.Write($bytes, 0, $bytes.Length) } finally { $stream.Dispose() }
`, psQuote(remotePath), psQuote(encodeBase64(data[offset:end])))
		if _, err := m.RunCommand(chunkCmd); err != nil {
			return fmt.Errorf("write remote text chunk %d failed: %w", offset/remoteTextChunkSize+1, err)
		}
	}

	return nil
}

func (m *Manager) prepareDownloadsTarget(fileName string) (string, error) {
	if strings.TrimSpace(fileName) == "" {
		return "", fmt.Errorf("file name is empty")
	}

	cmd := fmt.Sprintf(`
$fileName = %s
$downloadsBase = $env:USERPROFILE
if ([string]::IsNullOrWhiteSpace($downloadsBase)) {
    $downloadsBase = $HOME
}
if ([string]::IsNullOrWhiteSpace($downloadsBase)) {
    throw 'downloads base path is empty'
}
$downloads = Join-Path $downloadsBase 'Downloads'
if (-not (Test-Path $downloads)) {
    New-Item -ItemType Directory -Path $downloads -Force | Out-Null
}
$destPath = Join-Path $downloads $fileName
Write-Output $destPath
`, psQuote(fileName))

	output, err := m.RunCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("resolve downloads path failed: %w", err)
	}
	remotePath := strings.TrimSpace(output)
	log.Printf("[WinRM] resolved downloads path: host=%s file=%s remotePath=%s", m.Host, fileName, remotePath)
	return remotePath, nil
}

func (m *Manager) remoteSHA256(remotePath string) (string, error) {
	cmd := fmt.Sprintf(`
$path = %s
if (-not (Test-Path $path)) {
    throw 'remote file not found'
}
$hash = (Get-FileHash -Path $path -Algorithm SHA256).Hash.ToLowerInvariant()
Write-Output $hash
`, psQuote(remotePath))

	output, err := m.RunCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("compute remote sha256 failed: %w", err)
	}
	return strings.TrimSpace(output), nil
}

func (m *Manager) DownloadFileToDownloads(fileURL, fileName string) (string, error) {
	remotePath, err := m.prepareDownloadsTarget(fileName)
	if err != nil {
		return "", err
	}
	log.Printf("[WinRM] remote-pull start: host=%s file=%s remotePath=%s url=%s", m.Host, fileName, remotePath, fileURL)

	cmd := fmt.Sprintf(`
$ProgressPreference = 'SilentlyContinue'
$url = %s
$destPath = %s
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Remove-Item -Path $destPath -Force -ErrorAction SilentlyContinue

try {
    if (Get-Command Start-BitsTransfer -ErrorAction SilentlyContinue) {
        Start-BitsTransfer -Source $url -Destination $destPath -ErrorAction Stop
    } else {
        throw 'bits_not_available'
    }
} catch {
    Invoke-WebRequest -Uri $url -OutFile $destPath -UseBasicParsing -ErrorAction Stop
}

if (-not (Test-Path $destPath)) {
    throw 'remote pull failed'
}

Write-Output $destPath
`, psQuote(fileURL), psQuote(remotePath))

	output, err := m.RunCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("remote pull download failed: %w", err)
	}
	finalPath := strings.TrimSpace(output)
	log.Printf("[WinRM] remote-pull success: host=%s file=%s remotePath=%s", m.Host, fileName, finalPath)
	return finalPath, nil
}

func localFileSHA256(localPath string) (string, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func (m *Manager) UploadFileToDownloads(localPath, fileName string) (string, error) {
	return m.UploadFileToDownloadsStreamWithProgress(localPath, fileName, nil)
}

func (m *Manager) UploadFileToDownloadsWithProgress(localPath, fileName string, progress TransferProgressFunc) (string, error) {
	remotePath, err := m.prepareDownloadsTarget(fileName)
	if err != nil {
		return "", err
	}
	log.Printf("[WinRM] chunk-push start: host=%s localPath=%s file=%s remotePath=%s", m.Host, localPath, fileName, remotePath)

	initCmd := fmt.Sprintf(`
$destPath = %s
$parent = Split-Path -Parent $destPath
if (-not (Test-Path $parent)) {
    New-Item -ItemType Directory -Path $parent -Force | Out-Null
}
[System.IO.File]::WriteAllBytes($destPath, [byte[]]@())
Write-Output $destPath
`, psQuote(remotePath))
	if _, err := m.RunCommand(initCmd); err != nil {
		return "", fmt.Errorf("initialize remote file failed: %w", err)
	}

	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open local file failed: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat local file failed: %w", err)
	}
	totalBytes := fileInfo.Size()

	buffer := make([]byte, chunkPushChunkSize)
	chunkIndex := 0
	var uploadedBytes int64
	for {
		n, readErr := file.Read(buffer)
		if n > 0 {
			chunkIndex++
			chunk := buffer[:n]
			base64Chunk := encodeBase64(chunk)
			chunkCmd := fmt.Sprintf(`
$destPath = %s
$bytes = [Convert]::FromBase64String(%s)
$stream = [System.IO.File]::Open($destPath, [System.IO.FileMode]::Append)
try {
    $stream.Write($bytes, 0, $bytes.Length)
} finally {
    $stream.Dispose()
}
`, psQuote(remotePath), psQuote(base64Chunk))
			if _, err := m.RunCommand(chunkCmd); err != nil {
				return "", fmt.Errorf("write remote chunk %d failed: %w", chunkIndex, err)
			}
			uploadedBytes += int64(n)
			if progress != nil {
				progress(uploadedBytes, totalBytes, "chunk-push", "正在分块上传文件")
			}
			log.Printf("[WinRM] chunk-push progress: host=%s file=%s chunk=%d bytes=%d", m.Host, fileName, chunkIndex, n)
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("read local file failed: %w", readErr)
		}
	}

	log.Printf("[WinRM] chunk-push success: host=%s file=%s remotePath=%s chunks=%d", m.Host, fileName, remotePath, chunkIndex)
	return remotePath, nil
}

func (m *Manager) UploadFileToDownloadsStreamWithProgress(localPath, fileName string, progress TransferProgressFunc) (string, error) {
	remotePath, err := m.prepareDownloadsTarget(fileName)
	if err != nil {
		return "", err
	}
	log.Printf("[WinRM] stream-push start: host=%s localPath=%s file=%s remotePath=%s", m.Host, localPath, fileName, remotePath)

	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open local file failed: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat local file failed: %w", err)
	}
	totalBytes := fileInfo.Size()
	if progress != nil {
		progress(0, totalBytes, "chunk-push", "正在通过 WinRM 流式上传文件")
	}

	script := fmt.Sprintf(`
$destPath = %s
$parent = Split-Path -Parent $destPath
if (-not (Test-Path $parent)) {
    New-Item -ItemType Directory -Path $parent -Force | Out-Null
}
$inputStream = [Console]::OpenStandardInput()
$output = [System.IO.File]::Open($destPath, [System.IO.FileMode]::Create, [System.IO.FileAccess]::Write, [System.IO.FileShare]::Read)
try {
    $buffer = New-Object byte[] 65536
    while (($read = $inputStream.Read($buffer, 0, $buffer.Length)) -gt 0) {
        $output.Write($buffer, 0, $read)
    }
} finally {
    $output.Dispose()
    $inputStream.Dispose()
}
Write-Output $destPath
`, psQuote(remotePath))

	command := gowinrm.Powershell(script)
	if command == "" {
		return "", fmt.Errorf("encode powershell command failed")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	reader := &progressReader{
		reader: file,
		total:  totalBytes,
		onRead: func(uploaded, total int64) {
			if progress != nil {
				progress(uploaded, total, "chunk-push", "正在通过 WinRM 流式上传文件")
			}
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	exitCode, runErr := m.client.RunWithContextWithInput(ctx, command, &stdout, &stderr, reader)
	if runErr != nil {
		return "", fmt.Errorf(
			"stream push failed: %w, stdout: %s, stderr: %s",
			runErr,
			summarizeOutput(stdout.String()),
			summarizeOutput(stderr.String()),
		)
	}
	if exitCode != 0 {
		return "", fmt.Errorf(
			"stream push exit code %d, stdout: %s, stderr: %s",
			exitCode,
			summarizeOutput(stdout.String()),
			summarizeOutput(stderr.String()),
		)
	}

	finalPath := strings.TrimSpace(stdout.String())
	if finalPath == "" {
		finalPath = remotePath
	}
	if progress != nil {
		progress(totalBytes, totalBytes, "chunk-push", "文件上传完成")
	}
	log.Printf("[WinRM] stream-push success: host=%s file=%s remotePath=%s", m.Host, fileName, finalPath)
	return finalPath, nil
}

func (m *Manager) TransferFileToDownloads(fileURL, localPath, fileName string) (string, string, error) {
	return m.TransferFileToDownloadsWithProgress(fileURL, localPath, fileName, nil)
}

func (m *Manager) TransferFileToDownloadsWithProgress(fileURL, localPath, fileName string, progress TransferProgressFunc) (string, string, error) {
	localHash, err := localFileSHA256(localPath)
	if err != nil {
		return "", "", fmt.Errorf("compute local sha256 failed: %w", err)
	}

	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return "", "", fmt.Errorf("stat local file failed: %w", err)
	}
	log.Printf("[WinRM] transfer decision: host=%s file=%s size=%d threshold=%d", m.Host, fileName, fileInfo.Size(), smallFileChunkPushThreshold)

	if fileInfo.Size() <= smallFileChunkPushThreshold {
		log.Printf("[WinRM] transfer mode selected: host=%s file=%s mode=chunk-push", m.Host, fileName)
		if progress != nil {
			progress(0, fileInfo.Size(), "chunk-push", "准备分块上传文件")
		}
		remotePath, pushErr := m.UploadFileToDownloadsStreamWithProgress(localPath, fileName, progress)
		if pushErr != nil {
			return "", "", pushErr
		}

		remoteHash, hashErr := m.remoteSHA256(remotePath)
		if hashErr != nil {
			return "", "", fmt.Errorf("chunk push verify failed: %w", hashErr)
		}
		if !strings.EqualFold(localHash, remoteHash) {
			return "", "", fmt.Errorf("chunk push checksum mismatch")
		}

		if progress != nil {
			progress(fileInfo.Size(), fileInfo.Size(), "chunk-push", "文件上传完成")
		}
		return remotePath, "chunk-push", nil
	}

	var (
		remotePath string
		pullErr    error
	)

	if strings.TrimSpace(fileURL) != "" {
		log.Printf("[WinRM] transfer mode selected: host=%s file=%s mode=remote-pull", m.Host, fileName)
		if progress != nil {
			progress(fileInfo.Size()/10, fileInfo.Size(), "remote-pull", "正在通过远端拉取文件")
		}
		remotePath, pullErr = m.DownloadFileToDownloads(fileURL, fileName)
		if pullErr == nil {
			if progress != nil {
				progress(fileInfo.Size()*9/10, fileInfo.Size(), "remote-pull", "远端拉取完成，正在校验文件")
			}
			remoteHash, hashErr := m.remoteSHA256(remotePath)
			if hashErr == nil && strings.EqualFold(localHash, remoteHash) {
				if progress != nil {
					progress(fileInfo.Size(), fileInfo.Size(), "remote-pull", "文件上传完成")
				}
				return remotePath, "remote-pull", nil
			}
			if hashErr == nil {
				pullErr = fmt.Errorf("remote pull checksum mismatch")
			} else {
				pullErr = fmt.Errorf("remote pull verify failed: %w", hashErr)
			}
		}
	} else {
		pullErr = fmt.Errorf("remote pull skipped: public url unavailable")
		log.Printf("[WinRM] remote-pull skipped: host=%s file=%s reason=%v", m.Host, fileName, pullErr)
	}

	log.Printf("[WinRM] fallback to chunk-push: host=%s file=%s reason=%v", m.Host, fileName, pullErr)
	if progress != nil {
		progress(fileInfo.Size()/10, fileInfo.Size(), "chunk-push", "远端拉取失败，回退为分块上传")
	}
	remotePath, pushErr := m.UploadFileToDownloadsStreamWithProgress(localPath, fileName, progress)
	if pushErr != nil {
		return "", "", fmt.Errorf("remote pull failed: %v; chunk push failed: %w", pullErr, pushErr)
	}

	remoteHash, hashErr := m.remoteSHA256(remotePath)
	if hashErr != nil {
		return "", "", fmt.Errorf("chunk push verify failed: %w", hashErr)
	}
	if !strings.EqualFold(localHash, remoteHash) {
		return "", "", fmt.Errorf("chunk push checksum mismatch")
	}

	if progress != nil {
		progress(fileInfo.Size(), fileInfo.Size(), "chunk-push", "文件上传完成")
	}
	return remotePath, "chunk-push", nil
}

func (m *Manager) InstallWindowsExporter(installerURL, version string, exporterPort int, collectors, victoriaMetricsImportURL string, importInterval int, deviceName string, grouping *VictoriaMetricsGrouping) error {
	installerPath := fmt.Sprintf("C:\\Temp\\windows_exporter-%s.msi", version)

	downloadCmd := fmt.Sprintf(`
if (-not (Test-Path 'C:\Temp')) {
    New-Item -ItemType Directory -Path 'C:\Temp' | Out-Null
}

$installerPath = %s
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest -Uri %s -OutFile $installerPath -UseBasicParsing

if (-not (Test-Path $installerPath)) {
    throw 'windows_exporter installer download failed'
}
`, psQuote(installerPath), psQuote(installerURL))
	if output, err := m.RunCommand(downloadCmd); err != nil {
		return fmt.Errorf("download installer failed: %w, output: %s", err, strings.TrimSpace(output))
	}

	portCleanupCmd := fmt.Sprintf(`
$exporterPort = %d

$service = Get-Service -Name 'windows_exporter' -ErrorAction SilentlyContinue
if ($service) {
    Stop-Service -Name 'windows_exporter' -Force -ErrorAction SilentlyContinue
}

$connections = @(Get-NetTCPConnection -LocalPort $exporterPort -ErrorAction SilentlyContinue)
$owningProcesses = @($connections | Select-Object -ExpandProperty OwningProcess -Unique)

foreach ($pid in $owningProcesses) {
    if (-not $pid -or $pid -le 0) {
        continue
    }

    $proc = Get-CimInstance Win32_Process -Filter ("ProcessId = " + $pid) -ErrorAction SilentlyContinue
    if (-not $proc) {
        continue
    }

    $procName = [string]$proc.Name
    $procPath = [string]$proc.ExecutablePath
    $identity = ($procName + ' ' + $procPath).ToLowerInvariant()

    if ($identity -like '*windows_exporter*') {
        Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
        Write-Output ("stopped stale exporter process pid=" + $pid + " path=" + $procPath)
    } else {
        throw ("port " + $exporterPort + " is already used by pid=" + $pid + " name=" + $procName + " path=" + $procPath)
    }
}

Start-Sleep -Seconds 2
$remaining = @(Get-NetTCPConnection -LocalPort $exporterPort -ErrorAction SilentlyContinue)
if ($remaining.Count -gt 0) {
    $remainingPid = (($remaining | Select-Object -ExpandProperty OwningProcess -Unique) -join ',')
    throw ("port " + $exporterPort + " is still occupied after cleanup, pids=" + $remainingPid)
}
`, exporterPort)
	if output, err := m.RunCommand(portCleanupCmd); err != nil {
		return fmt.Errorf("prepare exporter port failed: %w, output: %s", err, strings.TrimSpace(output))
	}

	installCmd := fmt.Sprintf(`
$installerPath = %s
$logPath = 'C:\Temp\windows_exporter_install.log'
$collectors = %s
$exporterPort = %d
$arguments = @('/i', $installerPath, '/quiet', '/qn', '/norestart', '/l*v', $logPath, "ENABLED_COLLECTORS=$collectors", "LISTEN_PORT=$exporterPort")
$process = Start-Process 'msiexec.exe' -ArgumentList $arguments -Wait -PassThru -NoNewWindow
if ($process.ExitCode -ne 0) {
    throw "msiexec failed with exit code $($process.ExitCode)"
}
`, psQuote(installerPath), psQuote(collectors), exporterPort)
	if output, err := m.RunCommand(installCmd); err != nil {
		return fmt.Errorf("install exporter failed: %w, output: %s", err, strings.TrimSpace(output))
	}

	serviceArgs := fmt.Sprintf("--collectors.enabled %s --web.listen-address=:%d", collectors, exporterPort)
	configureCmd := fmt.Sprintf(`
$exePath = 'C:\Program Files\windows_exporter\windows_exporter.exe'
$serviceArgs = %s
$exporterPort = %d

if (-not (Test-Path $exePath)) {
    throw 'windows_exporter executable not found'
}

$service = Get-Service -Name 'windows_exporter' -ErrorAction SilentlyContinue
if (-not $service) {
    throw 'windows_exporter service not found'
}

Stop-Service -Name 'windows_exporter' -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

$binPath = '"' + $exePath + '" ' + $serviceArgs
& sc.exe config windows_exporter binPath= $binPath | Out-Null

Start-Service -Name 'windows_exporter' -ErrorAction Stop

$running = $false
for ($i = 0; $i -lt 15; $i++) {
    Start-Sleep -Seconds 1
    $service = Get-Service -Name 'windows_exporter' -ErrorAction SilentlyContinue
    if ($service -and $service.Status -eq 'Running') {
        $running = $true
        break
    }
}

if (-not $running) {
    $service = Get-Service -Name 'windows_exporter' -ErrorAction SilentlyContinue
    $serviceStatus = if ($service) { $service.Status } else { 'NotFound' }
    $events = @(Get-EventLog -LogName Application -Newest 5 -ErrorAction SilentlyContinue |
        Where-Object { $_.Source -like '*windows_exporter*' } |
        ForEach-Object { "$($_.TimeGenerated) - $($_.EntryType) - $($_.Message)" })
    throw ('service failed to start: ' + $serviceStatus + [Environment]::NewLine + ($events -join [Environment]::NewLine))
}

$metricsUrl = "http://127.0.0.1:$exporterPort/metrics"
$metricsReady = $false
$metricsError = ''
for ($i = 0; $i -lt 15; $i++) {
    try {
        $metricsResponse = Invoke-WebRequest -Uri $metricsUrl -UseBasicParsing -TimeoutSec 2 -ErrorAction Stop
        if ($metricsResponse.StatusCode -ge 200 -and $metricsResponse.StatusCode -lt 300 -and $metricsResponse.Content -match 'windows_') {
            $metricsReady = $true
            break
        }
        $metricsError = "invalid HTTP response or metrics content"
    } catch {
        $metricsError = $_.Exception.Message
    }
    Start-Sleep -Seconds 1
}
if (-not $metricsReady) {
    throw ("metrics endpoint validation failed after retries: " + $metricsUrl + ", error: " + $metricsError)
}
Write-Output 'RUNNING_WITH_METRICS'
`, psQuote(serviceArgs), exporterPort)
	if output, err := m.RunCommand(configureCmd); err != nil {
		return fmt.Errorf("configure exporter service failed: %w, output: %s", err, strings.TrimSpace(output))
	}

	if victoriaMetricsImportURL != "" && importInterval > 0 {
		jobValue := deviceName
		groupIDValue := ""
		groupNameValue := ""
		hostnameValue := deviceName
		if grouping != nil {
			if strings.TrimSpace(grouping.Job) != "" {
				jobValue = grouping.Job
			}
			groupIDValue = strings.TrimSpace(grouping.GroupID)
			groupNameValue = strings.TrimSpace(grouping.GroupName)
			if strings.TrimSpace(grouping.Hostname) != "" {
				hostnameValue = grouping.Hostname
			}
		}

		scriptPath := `C:\Windows\System32\push_metrics_to_victoriametrics.ps1`
		importScriptContent := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$exporterUrl = 'http://localhost:%d/metrics'
$job = %s
$groupId = %s
$groupName = %s
$hostname = %s
$victoriaMetricsUrl = %s
$errorLogPath = 'C:\Temp\windows_exporter_victoriametrics_error.log'
try {
    $metrics = Invoke-WebRequest -Uri $exporterUrl -UseBasicParsing -TimeoutSec 15
    if ($metrics.StatusCode -lt 200 -or $metrics.StatusCode -ge 300 -or $metrics.Content -notmatch 'windows_') {
        throw "windows_exporter metrics endpoint returned invalid data"
    }

    $labels = @("job=$job")
    if (-not [string]::IsNullOrWhiteSpace($groupId)) { $labels += "group_id=$groupId" }
    if (-not [string]::IsNullOrWhiteSpace($groupName)) { $labels += "group_name=$groupName" }
    if (-not [string]::IsNullOrWhiteSpace($hostname)) { $labels += "hostname=$hostname" }
    $separator = if ($victoriaMetricsUrl.Contains('?')) { '&' } else { '?' }
    $query = ($labels | ForEach-Object { "extra_label=" + [System.Uri]::EscapeDataString($_) }) -join '&'
    $importUrl = $victoriaMetricsUrl + $separator + $query
    $pushTime = [int64]([DateTime]::UtcNow - [DateTime]'1970-01-01').TotalSeconds
    $payload = $metrics.Content.TrimEnd() + [Environment]::NewLine + "push_time_seconds " + $pushTime + [Environment]::NewLine
    $response = Invoke-WebRequest -Uri $importUrl -Method Post -Body $payload -ContentType "text/plain" -UseBasicParsing -TimeoutSec 30
    if ($response.StatusCode -lt 200 -or $response.StatusCode -ge 300) {
        throw ("VictoriaMetrics returned HTTP " + $response.StatusCode)
    }
    Remove-Item -Path $errorLogPath -Force -ErrorAction SilentlyContinue
} catch {
    $errorMessage = "VictoriaMetrics import failed: $($_.Exception.Message)"
    Set-Content -Path $errorLogPath -Value $errorMessage -Force -Encoding UTF8
    throw $errorMessage
}
`, exporterPort, psQuote(jobValue), psQuote(groupIDValue), psQuote(groupNameValue), psQuote(hostnameValue), psQuote(strings.TrimRight(victoriaMetricsImportURL, "/")))
		if err := m.writeRemoteUTF8File(scriptPath, importScriptContent); err != nil {
			return fmt.Errorf("write VictoriaMetrics import script failed: %w", err)
		}

		registerTaskCmd := fmt.Sprintf(`
$scriptPath = %s
if (-not (Test-Path $scriptPath)) {
    throw ("failed to create VictoriaMetrics import script: " + $scriptPath)
}

$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument "-NoProfile -ExecutionPolicy Bypass -File $scriptPath"
$trigger = New-ScheduledTaskTrigger -Once -At (Get-Date) -RepetitionInterval (New-TimeSpan -Seconds %d) -RepetitionDuration (New-TimeSpan -Days 9999)
$principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -ExecutionTimeLimit (New-TimeSpan -Minutes 2) -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
Unregister-ScheduledTask -TaskName 'PushMetricsToGateway' -Confirm:$false -ErrorAction SilentlyContinue
Register-ScheduledTask -TaskName 'PushMetricsToVictoriaMetrics' -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
`, psQuote(scriptPath), importInterval)
		if output, err := m.RunCommand(registerTaskCmd); err != nil {
			return fmt.Errorf("register VictoriaMetrics import task failed: %w, output: %s", err, strings.TrimSpace(output))
		}

		verifyTaskCmd := `
Start-ScheduledTask -TaskName 'PushMetricsToVictoriaMetrics'
for ($i = 0; $i -lt 45; $i++) {
    Start-Sleep -Seconds 1
    $task = Get-ScheduledTask -TaskName 'PushMetricsToVictoriaMetrics' -ErrorAction Stop
    if ($task.State -ne 'Running') {
        break
    }
}
$task = Get-ScheduledTask -TaskName 'PushMetricsToVictoriaMetrics' -ErrorAction Stop
$taskInfo = Get-ScheduledTaskInfo -TaskName 'PushMetricsToVictoriaMetrics' -ErrorAction Stop
if ($task.State -eq 'Running') {
    throw 'initial VictoriaMetrics import timed out'
}
if ($taskInfo.LastRunTime -le [DateTime]'2000-01-01' -or $taskInfo.LastTaskResult -ne 0) {
    $taskError = ''
    if (Test-Path 'C:\Temp\windows_exporter_victoriametrics_error.log') {
        $taskError = (Get-Content -Path 'C:\Temp\windows_exporter_victoriametrics_error.log' -Raw -ErrorAction SilentlyContinue).Trim()
    }
    throw ("initial VictoriaMetrics import failed, task result: " + $taskInfo.LastTaskResult + ", error: " + $taskError)
}
`
		if output, err := m.RunCommand(verifyTaskCmd); err != nil {
			return fmt.Errorf("verify VictoriaMetrics import task failed: %w, output: %s", err, strings.TrimSpace(output))
		}
	}

	cleanCmd := fmt.Sprintf(`
Remove-Item -Path %s -Force -ErrorAction SilentlyContinue
`, psQuote(installerPath))
	_, _ = m.RunCommand(cleanCmd)

	return nil
}

func (m *Manager) CheckExporterStatus(exporterPort, importInterval int, requireVictoriaMetrics bool) (bool, bool, string, string, error) {
	if exporterPort <= 0 {
		exporterPort = 9182
	}
	if importInterval < 60 {
		importInterval = 60
	}
	cmd := fmt.Sprintf(`
$requireVictoriaMetrics = %d
$maxImportAgeSeconds = %d
$service = Get-Service -Name 'windows_exporter' -ErrorAction SilentlyContinue
if (-not $service) {
    Write-Output 'not_installed'
    exit 0
}

$entries = @()
$entries += @(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*' -ErrorAction SilentlyContinue)
$entries += @(Get-ItemProperty 'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*' -ErrorAction SilentlyContinue)
$entry = $entries | Where-Object { $_.DisplayName -like '*windows_exporter*' } | Select-Object -First 1
$version = if ($entry) { $entry.DisplayVersion } else { '' }
$serviceReady = $service.Status -eq 'Running'
$metricsReady = $false
if ($serviceReady) {
    try {
        $metrics = Invoke-WebRequest -Uri 'http://127.0.0.1:%d/metrics' -UseBasicParsing -TimeoutSec 15
        $metricsReady = $metrics.StatusCode -ge 200 -and $metrics.StatusCode -lt 300 -and $metrics.Content -match 'windows_'
    } catch {
        $metricsReady = $false
    }
}
$victoriaMetricsReady = $true
$victoriaMetricsReason = ''
if ($requireVictoriaMetrics -eq 1) {
    $task = Get-ScheduledTask -TaskName 'PushMetricsToVictoriaMetrics' -ErrorAction SilentlyContinue
    $taskInfo = Get-ScheduledTaskInfo -TaskName 'PushMetricsToVictoriaMetrics' -ErrorAction SilentlyContinue
    if (-not $task -or -not $taskInfo) {
        $victoriaMetricsReady = $false
        $victoriaMetricsReason = 'victoriametrics_task_missing'
    } elseif ($task.State -ne 'Ready' -and $task.State -ne 'Running') {
        $victoriaMetricsReady = $false
        $victoriaMetricsReason = 'victoriametrics_task_state:' + [string]$task.State
    } elseif ($taskInfo.LastRunTime -le [DateTime]'2000-01-01') {
        $victoriaMetricsReady = $false
        $victoriaMetricsReason = 'victoriametrics_task_never_ran'
    } elseif ($task.State -ne 'Running' -and $taskInfo.LastTaskResult -ne 0) {
        $victoriaMetricsReady = $false
        $victoriaMetricsReason = 'victoriametrics_task_failed:' + [string]$taskInfo.LastTaskResult
    } else {
        $taskAgeSeconds = [int64]((Get-Date) - $taskInfo.LastRunTime).TotalSeconds
        if ($taskAgeSeconds -gt $maxImportAgeSeconds) {
            $victoriaMetricsReady = $false
            $victoriaMetricsReason = 'victoriametrics_task_stale:' + [string]$taskAgeSeconds
        }
    }
}
$healthReason = ''
if (-not $serviceReady) {
    $healthReason = 'service_not_running'
} elseif (-not $metricsReady) {
    $healthReason = 'metrics_endpoint_unavailable'
} elseif (-not $victoriaMetricsReady) {
    $healthReason = $victoriaMetricsReason
}
$healthy = $serviceReady -and $metricsReady -and $victoriaMetricsReady
Write-Output "installed|$($service.Status)|$version|$healthy|$healthReason"
`, boolToInt(requireVictoriaMetrics), importInterval*3, exporterPort)

	output, err := m.RunCommand(cmd)
	if err != nil {
		return false, false, "", "", err
	}

	output = strings.TrimSpace(output)
	if output == "not_installed" {
		return false, false, "", "", nil
	}

	parts := strings.Split(output, "|")
	if len(parts) < 5 || parts[0] != "installed" {
		return false, false, "", "", fmt.Errorf("unexpected exporter status output: %s", output)
	}

	return true, strings.EqualFold(parts[3], "true"), parts[2], parts[4], nil
}

func (m *Manager) UninstallExporter() error {
	cmd := `
$service = Get-Service -Name 'windows_exporter' -ErrorAction SilentlyContinue
if ($service) {
    Stop-Service -Name 'windows_exporter' -Force -ErrorAction SilentlyContinue
}

function Stop-ExporterProcesses {
    $procs = @(
        Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
        Where-Object {
            $_.Name -like '*windows_exporter*' -or
            ([string]$_.ExecutablePath).ToLowerInvariant() -like '*windows_exporter*'
        }
    )

    foreach ($proc in $procs) {
        if ($proc.ProcessId -and $proc.ProcessId -gt 0) {
            Stop-Process -Id $proc.ProcessId -Force -ErrorAction SilentlyContinue
        }
    }
}

Stop-ExporterProcesses

foreach ($taskName in @('PushMetricsToGateway', 'PushMetricsToVictoriaMetrics')) {
    $task = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
    if ($task) {
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
    }
}

Remove-Item -Path 'C:\Windows\System32\push_metrics_to_gateway.ps1' -Force -ErrorAction SilentlyContinue
Remove-Item -Path 'C:\Windows\System32\push_metrics_to_victoriametrics.ps1' -Force -ErrorAction SilentlyContinue
Remove-Item -Path 'C:\Temp\windows_exporter_victoriametrics_error.log' -Force -ErrorAction SilentlyContinue

$app = Get-WmiObject -Class Win32_Product | Where-Object { $_.Name -like '*windows_exporter*' }
if ($app) {
    $null = $app.Uninstall()
}

Start-Sleep -Seconds 2
Stop-ExporterProcesses

Remove-Item -Path 'C:\Temp\windows_exporter-*' -Force -ErrorAction SilentlyContinue
Write-Output 'OK'
`

	output, err := m.RunCommand(cmd)
	if err != nil {
		return fmt.Errorf("uninstall exporter failed: %w, output: %s", err, strings.TrimSpace(output))
	}

	return nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (m *Manager) GetExporterDiagnostics() (*ExporterDiagnostics, error) {
	statusCmd := `
$service = Get-Service -Name 'windows_exporter' -ErrorAction SilentlyContinue
$entries = @()
$entries += @(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*' -ErrorAction SilentlyContinue)
$entries += @(Get-ItemProperty 'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*' -ErrorAction SilentlyContinue)
$entry = $entries | Where-Object { $_.DisplayName -like '*windows_exporter*' } | Select-Object -First 1
$version = if ($entry) { [string]$entry.DisplayVersion } else { '' }
$logPath = 'C:\Temp\windows_exporter_install.log'
$metricsReady = $false
if ($service -and $service.Status -eq 'Running') {
    try {
        $metricsResponse = Invoke-WebRequest -Uri 'http://127.0.0.1:9182/metrics' -UseBasicParsing -TimeoutSec 15
        $metricsReady = $metricsResponse.StatusCode -ge 200 -and $metricsResponse.StatusCode -lt 300 -and $metricsResponse.Content -match 'windows_'
    } catch {
        $metricsReady = $false
    }
}
[PSCustomObject]@{
    installed = [bool]($service -or $version)
    running = [bool]($service -and $service.Status -eq 'Running')
    serviceExists = [bool]$service
    serviceStatus = if ($service) { [string]$service.Status } else { '' }
    version = $version
    logPath = if (Test-Path $logPath) { $logPath } else { '' }
    metricsReady = [bool]$metricsReady
} | ConvertTo-Json -Compress
`

	var diagnostics ExporterDiagnostics
	statusOutput, err := m.RunCommand(statusCmd)
	if err != nil {
		return nil, fmt.Errorf("get exporter runtime diagnostics failed: %w", err)
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(statusOutput)), &diagnostics); err != nil {
		return nil, fmt.Errorf("parse exporter runtime diagnostics failed: %w", err)
	}

	taskCmd := `
$victoriaMetricsTask = Get-ScheduledTask -TaskName 'PushMetricsToVictoriaMetrics' -ErrorAction SilentlyContinue
$victoriaMetricsTaskInfo = Get-ScheduledTaskInfo -TaskName 'PushMetricsToVictoriaMetrics' -ErrorAction SilentlyContinue
$errorPath = 'C:\Temp\windows_exporter_victoriametrics_error.log'
$pushError = if (Test-Path $errorPath) { (Get-Content -Path $errorPath -Raw -ErrorAction SilentlyContinue).Trim() } else { '' }
[PSCustomObject]@{
    victoriaMetricsTaskExists = [bool]$victoriaMetricsTask
    victoriaMetricsTaskState = if ($victoriaMetricsTask) { [string]$victoriaMetricsTask.State } else { '' }
    victoriaMetricsTaskLastRun = if ($victoriaMetricsTaskInfo) { $victoriaMetricsTaskInfo.LastRunTime.ToString('yyyy-MM-dd HH:mm:ss') } else { '' }
    victoriaMetricsTaskLastResult = if ($victoriaMetricsTaskInfo) { [int64]$victoriaMetricsTaskInfo.LastTaskResult } else { -1 }
    victoriaMetricsError = $pushError
} | ConvertTo-Json -Compress
`
	taskOutput, err := m.RunCommand(taskCmd)
	if err != nil {
		return nil, fmt.Errorf("get VictoriaMetrics task diagnostics failed: %w", err)
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(taskOutput)), &diagnostics); err != nil {
		return nil, fmt.Errorf("parse VictoriaMetrics task diagnostics failed: %w", err)
	}

	installLogOutput, err := m.RunCommand(`
$path = 'C:\Temp\windows_exporter_install.log'
if (Test-Path $path) { Get-Content -Path $path -Tail 200 -ErrorAction SilentlyContinue }
`)
	if err != nil {
		return nil, fmt.Errorf("get exporter install log failed: %w", err)
	}
	diagnostics.InstallLog = strings.TrimSpace(installLogOutput)

	serviceConfigOutput, err := m.RunCommand(`
$service = Get-Service -Name 'windows_exporter' -ErrorAction SilentlyContinue
if ($service) { & sc.exe qc windows_exporter }
`)
	if err != nil {
		return nil, fmt.Errorf("get exporter service config failed: %w", err)
	}
	diagnostics.ServiceConfig = strings.TrimSpace(serviceConfigOutput)

	eventsCmd := `
$applicationEvents = @(Get-EventLog -LogName Application -Newest 10 -ErrorAction SilentlyContinue | Where-Object { $_.Source -like '*windows_exporter*' } | ForEach-Object { "$($_.TimeGenerated) - $($_.EntryType) - $($_.Message)" })
$systemEvents = @(Get-EventLog -LogName System -Newest 20 -ErrorAction SilentlyContinue | Where-Object { $_.Source -eq 'Service Control Manager' -and $_.Message -like '*windows_exporter*' } | Select-Object -First 10 | ForEach-Object { "$($_.TimeGenerated) - $($_.EntryType) - $($_.Message)" })
[PSCustomObject]@{
    applicationEvents = $applicationEvents
    systemEvents = $systemEvents
} | ConvertTo-Json -Compress -Depth 6
`
	eventsOutput, err := m.RunCommand(eventsCmd)
	if err != nil {
		return nil, fmt.Errorf("get exporter event diagnostics failed: %w", err)
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(eventsOutput)), &diagnostics); err != nil {
		return nil, fmt.Errorf("parse exporter event diagnostics failed: %w", err)
	}

	return &diagnostics, nil
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func encodeBase64(data []byte) string {
	const table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	if len(data) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.Grow((len(data) + 2) / 3 * 4)

	for i := 0; i < len(data); i += 3 {
		var b0, b1, b2 byte
		b0 = data[i]
		if i+1 < len(data) {
			b1 = data[i+1]
		}
		if i+2 < len(data) {
			b2 = data[i+2]
		}

		builder.WriteByte(table[b0>>2])
		builder.WriteByte(table[((b0&0x03)<<4)|(b1>>4)])

		if i+1 < len(data) {
			builder.WriteByte(table[((b1&0x0f)<<2)|(b2>>6)])
		} else {
			builder.WriteByte('=')
		}

		if i+2 < len(data) {
			builder.WriteByte(table[b2&0x3f])
		} else {
			builder.WriteByte('=')
		}
	}

	return builder.String()
}

func summarizeOutput(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if len(text) > 300 {
		return text[:300] + "...(truncated)"
	}
	return text
}

type progressReader struct {
	reader       io.Reader
	total        int64
	read         int64
	lastReported int64
	onRead       func(uploaded, total int64)
	mu           sync.Mutex
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.mu.Lock()
		r.read += int64(n)
		shouldReport := r.read-r.lastReported >= 64*1024 || r.read == r.total
		current := r.read
		if shouldReport {
			r.lastReported = r.read
		}
		callback := r.onRead
		total := r.total
		r.mu.Unlock()

		if shouldReport && callback != nil {
			callback(current, total)
		}
	}
	return n, err
}
