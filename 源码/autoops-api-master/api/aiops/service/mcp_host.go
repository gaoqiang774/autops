package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	pathpkg "path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	cmdbservice "dodevops-api/api/cmdb/service"
	systemauthz "dodevops-api/api/system/authz"
	"dodevops-api/common/config"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
)

const (
	mcpHostConfirmationVersion = 2
	mcpHostConfirmationTTL     = 5 * time.Minute
	mcpHostMaxLogFiles         = 100
	mcpHostMaxDiskItems        = 100
	mcpHostMaxPathEntries      = 500
	mcpHostMaxTailLines        = 500
	mcpHostMaxFileBytes        = 256 * 1024
)

var (
	mcpSystemdServicePattern = regexp.MustCompile(`^[A-Za-z0-9_.@-]{1,128}$`)
	mcpSensitiveConfigLine   = regexp.MustCompile(`(?i)^(\s*(?:export\s+)?[a-z0-9_.-]*(?:password|passwd|pwd|secret|token|api[_-]?key|access[_-]?key|private[_-]?key|authorization)[a-z0-9_.-]*\s*[:=]\s*).*$`)
	mcpSensitiveJSONLine     = regexp.MustCompile(`(?i)^(\s*"[^"]*(?:password|passwd|pwd|secret|token|api[_-]?key|access[_-]?key|private[_-]?key|authorization)[^"]*"\s*:\s*).*$`)
	mcpHostUsedConfirmations = struct {
		sync.Mutex
		items map[string]int64
	}{items: map[string]int64{}}
)

type MCPHostOperationService struct {
	ssh cmdbservice.CmdbHostSSHServiceInterface
}

type mcpHostConfirmation struct {
	Version   int    `json:"version"`
	UserID    uint   `json:"userId"`
	Tool      string `json:"tool"`
	HostID    uint   `json:"hostId"`
	AuthID    uint   `json:"authId"`
	Scope     string `json:"scope"`
	Command   string `json:"command"`
	ItemCount int    `json:"itemCount"`
	TotalSize uint64 `json:"totalSize"`
	ExpiresAt int64  `json:"expiresAt"`
	Nonce     string `json:"nonce"`
}

type mcpHostDiskItem struct {
	Path      string `json:"path"`
	SizeBytes uint64 `json:"sizeBytes"`
	Size      string `json:"size"`
}

type mcpHostLogFile struct {
	Path         string  `json:"path"`
	SizeBytes    uint64  `json:"sizeBytes"`
	Size         string  `json:"size"`
	ModifiedUnix float64 `json:"modifiedUnix"`
	ModifiedAt   string  `json:"modifiedAt"`
}

type mcpHostLogScope struct {
	Path           string   `json:"path"`
	Extensions     []string `json:"extensions"`
	OlderThanDays  int      `json:"olderThanDays"`
	MinSizeMB      int      `json:"minSizeMB"`
	MaxFiles       int      `json:"maxFiles"`
	MaxTotalSizeMB int      `json:"maxTotalSizeMB"`
}

type mcpHostPathEntry struct {
	Name         string  `json:"name"`
	Path         string  `json:"path"`
	Type         string  `json:"type"`
	SizeBytes    uint64  `json:"sizeBytes"`
	Size         string  `json:"size"`
	Permissions  string  `json:"permissions"`
	ModifiedUnix float64 `json:"modifiedUnix"`
	ModifiedAt   string  `json:"modifiedAt"`
}

func NewMCPHostOperationService() *MCPHostOperationService {
	return &MCPHostOperationService{ssh: cmdbservice.GetCmdbHostSSHService()}
}

func (s *MCPHostOperationService) Invoke(c *gin.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	switch toolName {
	case "ecs_list_filesystems":
		return s.listFilesystems(c, args)
	case "ecs_analyze_disk_usage":
		return s.analyzeDiskUsage(c, args)
	case "ecs_list_log_files":
		return s.listLogFiles(c, args)
	case "ecs_inspect_path":
		return s.inspectPath(c, args)
	case "ecs_request_start_service", "ecs_request_stop_service", "ecs_request_restart_service":
		return s.changeServiceState(c, toolName, args)
	case "ecs_request_cleanup_logs":
		return s.cleanupLogs(c, toolName, args)
	default:
		return nil, fmt.Errorf("unsupported MCP host operation: %s", toolName)
	}
}

func (s *MCPHostOperationService) listFilesystems(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	hostID, authID, host, err := mcpHostTarget(args)
	if err != nil {
		return nil, err
	}
	command := mcpHostPrivilegePrelude() + `$MCP_SUDO df -P -B1 -T 2>/dev/null`
	response, err := s.execute(c, hostID, authID, command, false)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0)
	for index, line := range strings.Split(response.Output, "\n") {
		if index == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		total, _ := strconv.ParseUint(fields[2], 10, 64)
		used, _ := strconv.ParseUint(fields[3], 10, 64)
		available, _ := strconv.ParseUint(fields[4], 10, 64)
		items = append(items, map[string]interface{}{
			"filesystem": fields[0], "type": fields[1], "totalBytes": total,
			"usedBytes": used, "availableBytes": available, "usePercent": fields[5],
			"mountpoint": strings.Join(fields[6:], " "),
		})
	}
	return map[string]interface{}{
		"hostId": hostID, "hostName": host.HostName, "filesystems": items,
		"count": len(items), "command": command,
	}, nil
}

func (s *MCPHostOperationService) analyzeDiskUsage(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	hostID, authID, host, err := mcpHostTarget(args)
	if err != nil {
		return nil, err
	}
	root, err := normalizeMCPReadPath(mcpStringArgument(args, "path"))
	if err != nil {
		return nil, err
	}
	depth := mcpHostIntArgument(args, "depth", 2)
	if depth < 1 || depth > 5 {
		return nil, errors.New("depth must be between 1 and 5")
	}
	topN := mcpHostIntArgument(args, "topN", 20)
	if topN < 1 || topN > mcpHostMaxDiskItems {
		return nil, fmt.Errorf("topN must be between 1 and %d", mcpHostMaxDiskItems)
	}
	minFileSizeMB := mcpHostIntArgument(args, "minFileSizeMB", 100)
	if minFileSizeMB < 1 {
		return nil, errors.New("minFileSizeMB must be greater than 0")
	}

	quotedRoot := mcpShellQuote(root)
	command := fmt.Sprintf(
		"%s$MCP_SUDO du -x -B1 --max-depth=%d -- %s 2>/dev/null | sort -nr | head -n %d; printf '\\n__MCP_FILES__\\n'; $MCP_SUDO find %s -xdev -type f -size +%dM -printf '%%s\\t%%p\\n' 2>/dev/null | sort -nr | head -n %d",
		mcpHostPrivilegePrelude(), depth, quotedRoot, topN, quotedRoot, minFileSizeMB, topN,
	)
	response, err := s.execute(c, hostID, authID, command, false)
	if err != nil {
		return nil, err
	}
	sections := strings.SplitN(response.Output, "__MCP_FILES__", 2)
	directories := parseMCPDiskItems(sections[0])
	files := []mcpHostDiskItem{}
	if len(sections) == 2 {
		files = parseMCPDiskItems(sections[1])
	}
	return map[string]interface{}{
		"hostId": hostID, "hostName": host.HostName, "path": root, "depth": depth,
		"topN": topN, "minFileSizeMB": minFileSizeMB, "directories": directories,
		"files": files, "command": command,
	}, nil
}

func (s *MCPHostOperationService) listLogFiles(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	hostID, authID, host, err := mcpHostTarget(args)
	if err != nil {
		return nil, err
	}
	scope, err := mcpHostLogScopeFromArgs(args)
	if err != nil {
		return nil, err
	}
	files, command, err := s.scanLogFiles(c, hostID, authID, scope)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"hostId": hostID, "hostName": host.HostName, "scope": scope,
		"files": files, "count": len(files), "totalBytes": totalMCPLogSize(files),
		"command": command,
	}, nil
}

func (s *MCPHostOperationService) inspectPath(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	hostID, authID, host, err := mcpHostTarget(args)
	if err != nil {
		return nil, err
	}
	target, err := normalizeMCPReadPath(mcpStringArgument(args, "path"))
	if err != nil {
		return nil, err
	}
	if isSensitiveMCPReadPath(target) {
		return nil, errors.New("path is protected and cannot be read through MCP")
	}
	maxEntries := mcpHostIntArgument(args, "maxEntries", 100)
	if maxEntries < 1 || maxEntries > mcpHostMaxPathEntries {
		return nil, fmt.Errorf("maxEntries must be between 1 and %d", mcpHostMaxPathEntries)
	}
	tailLines := mcpHostIntArgument(args, "tailLines", mcpHostMaxTailLines)
	if tailLines < 1 || tailLines > mcpHostMaxTailLines {
		return nil, fmt.Errorf("tailLines must be between 1 and %d", mcpHostMaxTailLines)
	}

	command := mcpInspectPathCommand(target, maxEntries, tailLines)
	response, err := s.execute(c, hostID, authID, command, false)
	if err != nil {
		return nil, err
	}
	result, err := parseMCPInspectPathOutput(response.Output, target, maxEntries, tailLines)
	if err != nil {
		return nil, err
	}
	result["hostId"] = hostID
	result["hostName"] = host.HostName
	result["command"] = command
	return result, nil
}

func (s *MCPHostOperationService) changeServiceState(c *gin.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	hostID, authID, host, err := mcpHostTarget(args)
	if err != nil {
		return nil, err
	}
	serviceName := strings.TrimSpace(mcpStringArgument(args, "service"))
	if !mcpSystemdServicePattern.MatchString(serviceName) {
		return nil, errors.New("service must be a valid systemd unit name")
	}
	action := strings.TrimPrefix(strings.TrimSuffix(toolName, "_service"), "ecs_request_")
	if action != "start" && action != "stop" && action != "restart" {
		return nil, errors.New("unsupported service action")
	}
	command := mcpSystemdActionCommand(action, serviceName)
	scope := action + ":" + serviceName

	if !mcpHostBoolArgument(args, "confirmed") {
		statusCommand := mcpSystemdStatusCommand(serviceName)
		status, statusErr := s.execute(c, hostID, authID, statusCommand, false)
		if statusErr != nil {
			return nil, fmt.Errorf("service status precheck failed: %w", statusErr)
		}
		confirmationID, expiresAt, tokenErr := newMCPHostConfirmation(c, toolName, hostID, authID, scope, command, 1, 0)
		if tokenErr != nil {
			return nil, tokenErr
		}
		return map[string]interface{}{
			"hostId": hostID, "hostName": host.HostName, "service": serviceName,
			"action": action, "currentStatus": status.Output,
			"requiresConfirmation": true, "confirmed": false,
			"message": fmt.Sprintf("是否确认对主机 %s 的服务 %s 执行 %s？", host.HostName, serviceName, mcpServiceActionText(action)),
			"command": command, "confirmationId": confirmationID, "expiresAt": expiresAt,
		}, nil
	}

	payload, err := verifyMCPHostConfirmation(c, mcpStringArgument(args, "confirmationId"), toolName, hostID, authID, scope)
	if err != nil {
		return nil, err
	}
	if err := consumeMCPHostConfirmation(mcpStringArgument(args, "confirmationId"), payload.ExpiresAt); err != nil {
		return nil, err
	}
	response, err := s.execute(c, hostID, authID, payload.Command, true)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"hostId": hostID, "hostName": host.HostName, "service": serviceName,
		"action": action, "executed": true, "confirmed": true,
		"command": payload.Command, "output": response.Output,
	}, nil
}

func (s *MCPHostOperationService) cleanupLogs(c *gin.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	hostID, authID, host, err := mcpHostTarget(args)
	if err != nil {
		return nil, err
	}
	target, err := normalizeMCPReadPath(mcpStringArgument(args, "path"))
	if err != nil {
		return nil, err
	}
	if mcpHostBoolArgument(args, "confirmed") || strings.TrimSpace(mcpStringArgument(args, "confirmationId")) != "" {
		return nil, errors.New("日志清理不能通过 MCP 直接确认；请使用平台登录态调用确认接口")
	}

	var scope interface{}
	var files []mcpHostLogFile
	var scanCommand string
	if isMCPExplicitLogFilePath(target) {
		if isSensitiveMCPReadPath(target) {
			return nil, errors.New("path is protected and cannot be cleared through MCP")
		}
		if !isAllowedMCPExplicitLogPath(target) {
			return nil, errors.New("explicit log file must be under /tmp, /var/tmp, /var/log, /opt, /data, /srv, /app, /apps, /usr/local or /home")
		}
		files, scanCommand, err = s.scanExplicitLogFile(c, hostID, authID, target)
		scope = map[string]interface{}{"path": target, "mode": "single_file"}
	} else {
		var directoryScope mcpHostLogScope
		directoryScope, err = mcpHostLogScopeFromArgs(args)
		if err == nil {
			files, scanCommand, err = s.scanLogFiles(c, hostID, authID, directoryScope)
			scope = directoryScope
		}
	}
	if err != nil {
		return nil, err
	}
	scopeJSON, _ := json.Marshal(scope)
	totalSize := totalMCPLogSize(files)
	if len(files) == 0 {
		return map[string]interface{}{
			"hostId": hostID, "hostName": host.HostName, "scope": scope,
			"files": files, "count": 0, "totalBytes": 0, "scanCommand": scanCommand,
			"requiresConfirmation": false, "message": "没有找到符合清空条件的 .log 或 .txt 文件。",
		}, nil
	}
	clearCommand := mcpLogClearCommand(files)
	confirmationID, expiresAt, err := newMCPHostConfirmation(c, toolName, hostID, authID, string(scopeJSON), clearCommand, len(files), totalSize)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"hostId": hostID, "hostName": host.HostName, "scope": scope,
		"files": files, "count": len(files), "totalBytes": totalSize,
		"totalSize": mcpHumanBytes(totalSize), "scanCommand": scanCommand,
		"requiresConfirmation": true, "confirmed": false,
		"action": "truncate", "preserveFiles": true,
		"message": fmt.Sprintf("是否清空 %d 个日志文件的内容，共 %s？文件本身会保留。请核对 files 和清空命令，并由已登录的系统管理员确认。", len(files), mcpHumanBytes(totalSize)),
		"command": clearCommand, "confirmationId": confirmationID, "expiresAt": expiresAt,
		"confirmationEndpoint": "/api/v1/aiops/mcp/actions/cleanup-logs/confirm",
	}, nil
}

// ConfirmLogCleanup executes a signed cleanup preview under the real platform
// user's JWT context. The endpoint accepts no host, path, file or command
// fields, so the targets cannot differ from the reviewed preview.
func (s *MCPHostOperationService) ConfirmLogCleanup(c *gin.Context, confirmationID string) (interface{}, error) {
	payload, err := decodeMCPHostConfirmation(confirmationID)
	if err != nil {
		return nil, err
	}
	if payload.Tool != "ecs_request_cleanup_logs" {
		return nil, errors.New("confirmationId is not a log cleanup confirmation")
	}
	if !strings.Contains(payload.Command, "truncate -s 0 --") || strings.Contains(payload.Command, " rm ") {
		return nil, errors.New("confirmationId does not contain an approved log clear command")
	}

	admin, err := mcpExecutionAdmin(c)
	if err != nil {
		return nil, errors.New("未获取到当前登录用户，禁止确认日志清空")
	}
	isAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return nil, fmt.Errorf("校验当前用户管理员身份失败: %w", err)
	}
	if !isAdmin {
		return nil, errors.New("当前登录用户不是系统管理员，禁止确认日志清空")
	}
	serviceUserID := uint(0)
	if config.Config != nil {
		serviceUserID = config.Config.MCP.UserID
	}
	if payload.UserID != admin.ID && (serviceUserID == 0 || payload.UserID != serviceUserID) {
		return nil, errors.New("confirmationId 不是由当前登录用户或 MCP 服务账号签发")
	}

	hostDao := cmdbdao.NewCmdbHostDao()
	host, err := hostDao.GetCmdbHostById(payload.HostID)
	if err != nil {
		return nil, fmt.Errorf("get host: %w", err)
	}
	if err := consumeMCPHostConfirmation(confirmationID, payload.ExpiresAt); err != nil {
		return nil, err
	}
	response, err := s.execute(c, payload.HostID, payload.AuthID, payload.Command, true)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"hostId": payload.HostID, "hostName": host.HostName, "executed": true, "confirmed": true,
		"confirmationIssuedByUserId": payload.UserID, "confirmedByUserId": admin.ID, "confirmedByUsername": admin.Username,
		"clearedFiles": payload.ItemCount, "clearedBytes": payload.TotalSize,
		"clearedSize": mcpHumanBytes(payload.TotalSize), "preserveFiles": true, "command": payload.Command,
		"output": response.Output,
	}, nil
}

func (s *MCPHostOperationService) scanExplicitLogFile(c *gin.Context, hostID, authID uint, target string) ([]mcpHostLogFile, string, error) {
	command := fmt.Sprintf(
		`target=%s; if [ ! -e "$target" ]; then printf '__MCP_NOT_FOUND__\n'; elif [ -L "$target" ]; then printf '__MCP_SYMLINK__\n'; elif [ ! -f "$target" ]; then printf '__MCP_NOT_FILE__\n'; else metadata=$(stat -c '%%s %%Y' "$target" 2>/dev/null) || { printf '__MCP_DENIED__\n'; exit 0; }; set -- $metadata; printf '%%s\t%%s\t%%s\n' "$1" "$2" "$target"; fi; exit 0`,
		mcpShellQuote(target),
	)
	response, err := s.execute(c, hostID, authID, command, false)
	if err != nil {
		return nil, command, err
	}
	output := strings.TrimSpace(response.Output)
	switch {
	case strings.HasPrefix(output, "__MCP_NOT_FOUND__"):
		return nil, command, errors.New("log file does not exist")
	case strings.HasPrefix(output, "__MCP_SYMLINK__"):
		return nil, command, errors.New("symbolic links cannot be cleared through MCP")
	case strings.HasPrefix(output, "__MCP_NOT_FILE__"):
		return nil, command, errors.New("path must be a regular .log or .txt file")
	case strings.HasPrefix(output, "__MCP_DENIED__"):
		return nil, command, errors.New("the selected SSH user cannot inspect the log file")
	}
	fields := strings.SplitN(output, "\t", 3)
	if len(fields) != 3 {
		return nil, command, errors.New("invalid remote log file metadata")
	}
	size, sizeErr := strconv.ParseUint(strings.TrimSpace(fields[0]), 10, 64)
	modified, modifiedErr := strconv.ParseInt(strings.TrimSpace(fields[1]), 10, 64)
	if sizeErr != nil || modifiedErr != nil {
		return nil, command, errors.New("invalid remote log file metadata")
	}
	return []mcpHostLogFile{{
		Path: target, SizeBytes: size, Size: mcpHumanBytes(size), ModifiedUnix: float64(modified),
		ModifiedAt: time.Unix(modified, 0).Format("2006-01-02 15:04:05"),
	}}, command, nil
}

func (s *MCPHostOperationService) scanLogFiles(c *gin.Context, hostID, authID uint, scope mcpHostLogScope) ([]mcpHostLogFile, string, error) {
	patterns := make([]string, 0, len(scope.Extensions))
	for _, extension := range scope.Extensions {
		patterns = append(patterns, "-name "+mcpShellQuote("*"+extension))
	}
	command := fmt.Sprintf("%s$MCP_SUDO find %s -xdev -type f \\( %s \\) -mtime +%d",
		mcpHostPrivilegePrelude(), mcpShellQuote(scope.Path), strings.Join(patterns, " -o "), scope.OlderThanDays-1)
	if scope.MinSizeMB > 0 {
		command += fmt.Sprintf(" -size +%dM", scope.MinSizeMB)
	}
	command += fmt.Sprintf(" -printf '%%s\\t%%T@\\t%%p\\0' 2>/dev/null | sort -z -nr | head -z -n %d", scope.MaxFiles)

	response, err := s.execute(c, hostID, authID, command, false)
	if err != nil {
		return nil, command, err
	}
	files := make([]mcpHostLogFile, 0)
	for _, record := range strings.Split(response.Output, "\x00") {
		fields := strings.SplitN(record, "\t", 3)
		if len(fields) != 3 {
			continue
		}
		size, sizeErr := strconv.ParseUint(strings.TrimSpace(fields[0]), 10, 64)
		modified, modifiedErr := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64)
		filePath := strings.TrimSpace(fields[2])
		if sizeErr != nil || modifiedErr != nil || filePath == "" {
			continue
		}
		files = append(files, mcpHostLogFile{
			Path: filePath, SizeBytes: size, Size: mcpHumanBytes(size), ModifiedUnix: modified,
			ModifiedAt: time.Unix(int64(modified), 0).Format("2006-01-02 15:04:05"),
		})
	}
	maxTotalBytes := uint64(scope.MaxTotalSizeMB) * 1024 * 1024
	limited := make([]mcpHostLogFile, 0, len(files))
	var selectedBytes uint64
	for _, file := range files {
		if file.SizeBytes > maxTotalBytes || selectedBytes > maxTotalBytes-file.SizeBytes {
			continue
		}
		limited = append(limited, file)
		selectedBytes += file.SizeBytes
	}
	return limited, command, nil
}

func (s *MCPHostOperationService) execute(c *gin.Context, hostID, authID uint, command string, confirmed bool) (*cmdbservice.CommandResponse, error) {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return nil, errors.New("invalid MCP host execution context")
	}
	originalQuery := c.Request.URL.RawQuery
	if authID > 0 {
		query := c.Request.URL.Query()
		query.Set("authId", strconv.FormatUint(uint64(authID), 10))
		c.Request.URL.RawQuery = query.Encode()
	}
	defer func() { c.Request.URL.RawQuery = originalQuery }()
	return s.ssh.ExecuteCommand(c, hostID, command, confirmed)
}

func mcpHostTarget(args map[string]interface{}) (uint, uint, cmdbmodel.CmdbHost, error) {
	hostID := mcpUintArgument(args, "hostId")
	if hostID == 0 {
		return 0, 0, cmdbmodel.CmdbHost{}, errors.New("hostId is required")
	}
	hostDao := cmdbdao.NewCmdbHostDao()
	host, err := hostDao.GetCmdbHostById(hostID)
	if err != nil {
		return 0, 0, cmdbmodel.CmdbHost{}, fmt.Errorf("get host: %w", err)
	}
	if host.HostType == cmdbmodel.HostTypeWindows {
		return 0, 0, cmdbmodel.CmdbHost{}, errors.New("this MCP host operation currently supports Linux hosts only")
	}
	return hostID, mcpUintArgument(args, "authId"), host, nil
}

func normalizeMCPReadPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "/"
	}
	if strings.ContainsAny(value, "\x00\t\r\n") || !strings.HasPrefix(value, "/") {
		return "", errors.New("path must be an absolute Linux path without control characters")
	}
	return pathpkg.Clean(value), nil
}

func isSensitiveMCPReadPath(target string) bool {
	lower := strings.ToLower(pathpkg.Clean(target))
	base := pathpkg.Base(lower)
	if lower == "/etc/shadow" || lower == "/etc/gshadow" || lower == "/etc/security/opasswd" || lower == "/proc/kcore" {
		return true
	}
	if strings.Contains(lower, "/.ssh/") || strings.HasSuffix(lower, "/.ssh") {
		return !strings.HasSuffix(lower, ".pub")
	}
	switch base {
	case "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519", "credentials":
		return true
	}
	switch strings.ToLower(pathpkg.Ext(base)) {
	case ".key", ".p12", ".pfx":
		return true
	default:
		return false
	}
}

func mcpInspectPathCommand(target string, maxEntries, tailLines int) string {
	return fmt.Sprintf(
		`target=%s; limit=%d; if [ ! -e "$target" ]; then printf '__MCP_NOT_FOUND__\n'; elif [ -L "$target" ]; then printf '__MCP_SYMLINK__\n'; elif [ -d "$target" ]; then if [ ! -r "$target" ] || [ ! -x "$target" ]; then printf '__MCP_DENIED__\n'; elif ! command -v stat >/dev/null 2>&1; then printf '__MCP_TOOL_MISSING__\tstat\n'; else printf '__MCP_DIRECTORY__\n'; count=0; for entry in "$target"/* "$target"/.[!.]* "$target"/..?*; do if [ ! -e "$entry" ] && [ ! -L "$entry" ]; then continue; fi; if [ -L "$entry" ]; then entry_type=l; elif [ -d "$entry" ]; then entry_type=d; elif [ -f "$entry" ]; then entry_type=f; else entry_type=o; fi; metadata=$(stat -c '%%s %%Y %%A' "$entry" 2>/dev/null) || continue; set -- $metadata; printf '%%s\t%%s\t%%s\t%%s\t%%s\0' "$entry_type" "$1" "$2" "$3" "$entry"; count=$((count + 1)); if [ "$count" -ge "$limit" ]; then break; fi; done; fi; elif [ -f "$target" ]; then if [ ! -r "$target" ]; then printf '__MCP_DENIED__\n'; elif ! command -v stat >/dev/null 2>&1; then printf '__MCP_TOOL_MISSING__\tstat\n'; else metadata=$(stat -c '%%s %%Y %%A' "$target" 2>/dev/null) || { printf '__MCP_READ_ERROR__\n'; exit 0; }; set -- $metadata; printf '__MCP_FILE__\n%%s\t%%s\t%%s\n' "$1" "$2" "$3"; if [ -s "$target" ] && head -c 8192 "$target" 2>/dev/null | od -An -tx1 2>/dev/null | grep -q ' 00'; then printf '__MCP_BINARY__\n'; else content_bytes=$(tail -n %d "$target" 2>/dev/null | wc -c); printf '__MCP_CONTENT__\t%%s\n' "$content_bytes"; tail -n %d "$target" 2>/dev/null | tail -c %d; fi; fi; else printf '__MCP_UNSUPPORTED__\n'; fi; exit 0`,
		mcpShellQuote(target), maxEntries+1, tailLines, tailLines, mcpHostMaxFileBytes,
	)
}

func parseMCPInspectPathOutput(output, target string, maxEntries, tailLines int) (map[string]interface{}, error) {
	switch {
	case strings.HasPrefix(output, "__MCP_NOT_FOUND__"):
		return nil, errors.New("path does not exist")
	case strings.HasPrefix(output, "__MCP_DENIED__"):
		return nil, errors.New("the selected SSH user does not have permission to read this path")
	case strings.HasPrefix(output, "__MCP_SYMLINK__"):
		return nil, errors.New("symbolic links are not followed by MCP path inspection")
	case strings.HasPrefix(output, "__MCP_UNSUPPORTED__"):
		return nil, errors.New("path is neither a regular file nor a directory")
	case strings.HasPrefix(output, "__MCP_TOOL_MISSING__"):
		tool := strings.TrimSpace(strings.TrimPrefix(output, "__MCP_TOOL_MISSING__\t"))
		return nil, fmt.Errorf("remote host is missing required path inspection tool: %s", tool)
	case strings.HasPrefix(output, "__MCP_READ_ERROR__"):
		return nil, errors.New("the selected SSH user could not read path metadata")
	case strings.HasPrefix(output, "__MCP_DIRECTORY__\n"):
		entries := parseMCPPathEntries(strings.TrimPrefix(output, "__MCP_DIRECTORY__\n"))
		truncated := len(entries) > maxEntries
		if truncated {
			entries = entries[:maxEntries]
		}
		return map[string]interface{}{
			"path": target, "pathType": "directory", "entries": entries,
			"count": len(entries), "maxEntries": maxEntries, "truncated": truncated,
		}, nil
	case strings.HasPrefix(output, "__MCP_FILE__\n"):
		return parseMCPFileInspection(output, target, tailLines)
	default:
		return nil, errors.New("unable to determine remote path type")
	}
}

func parseMCPPathEntries(output string) []mcpHostPathEntry {
	entries := make([]mcpHostPathEntry, 0)
	for _, record := range strings.Split(output, "\x00") {
		fields := strings.SplitN(record, "\t", 5)
		if len(fields) != 5 {
			continue
		}
		size, sizeErr := strconv.ParseUint(strings.TrimSpace(fields[1]), 10, 64)
		modified, modifiedErr := strconv.ParseFloat(strings.TrimSpace(fields[2]), 64)
		entryPath := strings.TrimSpace(fields[4])
		if sizeErr != nil || modifiedErr != nil || entryPath == "" {
			continue
		}
		entries = append(entries, mcpHostPathEntry{
			Name: pathpkg.Base(entryPath), Path: entryPath, Type: mcpPathEntryType(fields[0]),
			SizeBytes: size, Size: mcpHumanBytes(size), Permissions: strings.TrimSpace(fields[3]),
			ModifiedUnix: modified, ModifiedAt: time.Unix(int64(modified), 0).Format("2006-01-02 15:04:05"),
		})
	}
	return entries
}

func mcpPathEntryType(value string) string {
	switch strings.TrimSpace(value) {
	case "d":
		return "directory"
	case "f":
		return "file"
	case "l":
		return "symlink"
	default:
		return "other"
	}
}

func parseMCPFileInspection(output, target string, tailLines int) (map[string]interface{}, error) {
	body := strings.TrimPrefix(output, "__MCP_FILE__\n")
	if strings.Contains(body, "__MCP_BINARY__") {
		return map[string]interface{}{
			"path": target, "pathType": "file", "binary": true,
			"content": "", "message": "binary file content is not returned",
		}, nil
	}
	parts := strings.SplitN(body, "\n", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid remote file metadata")
	}
	metadata := strings.SplitN(parts[0], "\t", 3)
	if len(metadata) != 3 {
		return nil, errors.New("invalid remote file metadata")
	}
	size, err := strconv.ParseUint(strings.TrimSpace(metadata[0]), 10, 64)
	if err != nil {
		return nil, errors.New("invalid remote file size")
	}
	modified, _ := strconv.ParseInt(strings.TrimSpace(metadata[1]), 10, 64)
	contentParts := strings.SplitN(parts[1], "\n", 2)
	if len(contentParts) != 2 || !strings.HasPrefix(contentParts[0], "__MCP_CONTENT__\t") {
		return nil, errors.New("invalid remote file content marker")
	}
	contentBytes, _ := strconv.ParseUint(strings.TrimPrefix(contentParts[0], "__MCP_CONTENT__\t"), 10, 64)
	content := redactMCPFileContent(contentParts[1])
	return map[string]interface{}{
		"path": target, "pathType": "file", "binary": false,
		"sizeBytes": size, "size": mcpHumanBytes(size), "permissions": strings.TrimSpace(metadata[2]),
		"modifiedUnix": modified, "modifiedAt": time.Unix(modified, 0).Format("2006-01-02 15:04:05"),
		"tailLines": tailLines, "content": content, "contentBytesBeforeLimit": contentBytes,
		"contentByteLimit": mcpHostMaxFileBytes, "contentTruncated": contentBytes > mcpHostMaxFileBytes,
	}, nil
}

func redactMCPFileContent(content string) string {
	if strings.Contains(content, "-----BEGIN") && strings.Contains(content, "PRIVATE KEY-----") {
		return "***REDACTED PRIVATE KEY CONTENT***"
	}
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		if mcpSensitiveConfigLine.MatchString(line) {
			lines[index] = mcpSensitiveConfigLine.ReplaceAllString(line, "${1}***REDACTED***")
			continue
		}
		if mcpSensitiveJSONLine.MatchString(line) {
			lines[index] = mcpSensitiveJSONLine.ReplaceAllString(line, "${1}\"***REDACTED***\"")
		}
	}
	return strings.Join(lines, "\n")
}

func mcpHostLogScopeFromArgs(args map[string]interface{}) (mcpHostLogScope, error) {
	root, err := normalizeMCPReadPath(mcpStringArgument(args, "path"))
	if err != nil {
		return mcpHostLogScope{}, err
	}
	if !isAllowedMCPLogRoot(root) {
		return mcpHostLogScope{}, errors.New("path must be /var/log or a log/logs directory under /opt, /data, /srv, /app, /apps, /usr/local or /home")
	}
	extensions, err := normalizeMCPLogExtensions(mcpHostStringArray(args, "extensions"))
	if err != nil {
		return mcpHostLogScope{}, err
	}
	olderThanDays := mcpHostIntArgument(args, "olderThanDays", 7)
	if olderThanDays < 1 || olderThanDays > 3650 {
		return mcpHostLogScope{}, errors.New("olderThanDays must be between 1 and 3650")
	}
	minSizeMB := mcpHostIntArgument(args, "minSizeMB", 0)
	if minSizeMB < 0 {
		return mcpHostLogScope{}, errors.New("minSizeMB cannot be negative")
	}
	maxFiles := mcpHostIntArgument(args, "maxFiles", 50)
	if maxFiles < 1 || maxFiles > mcpHostMaxLogFiles {
		return mcpHostLogScope{}, fmt.Errorf("maxFiles must be between 1 and %d", mcpHostMaxLogFiles)
	}
	maxTotalSizeMB := mcpHostIntArgument(args, "maxTotalSizeMB", 1024)
	if maxTotalSizeMB < 1 || maxTotalSizeMB > 102400 {
		return mcpHostLogScope{}, errors.New("maxTotalSizeMB must be between 1 and 102400")
	}
	return mcpHostLogScope{
		Path: root, Extensions: extensions, OlderThanDays: olderThanDays,
		MinSizeMB: minSizeMB, MaxFiles: maxFiles, MaxTotalSizeMB: maxTotalSizeMB,
	}, nil
}

func isAllowedMCPLogRoot(root string) bool {
	if root == "/var/log" || strings.HasPrefix(root, "/var/log/") {
		return true
	}
	allowedPrefixes := []string{"/opt/", "/data/", "/srv/", "/app/", "/apps/", "/usr/local/", "/home/"}
	allowedPrefix := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(root+"/", prefix) {
			allowedPrefix = true
			break
		}
	}
	if !allowedPrefix {
		return false
	}
	for _, segment := range strings.Split(strings.Trim(root, "/"), "/") {
		if strings.EqualFold(segment, "log") || strings.EqualFold(segment, "logs") {
			return true
		}
	}
	return false
}

func normalizeMCPLogExtensions(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{".log", ".txt"}, nil
	}
	allowed := map[string]struct{}{".log": {}, ".txt": {}}
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && !strings.HasPrefix(value, ".") {
			value = "." + value
		}
		if _, ok := allowed[value]; !ok {
			return nil, fmt.Errorf("extension %q is not allowed; only .log and .txt are supported", value)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func mcpSystemdStatusCommand(serviceName string) string {
	unit := mcpShellQuote(serviceName)
	return fmt.Sprintf("%s$MCP_SUDO systemctl is-active -- %s 2>&1 || true; $MCP_SUDO systemctl is-enabled -- %s 2>&1 || true; $MCP_SUDO systemctl --no-pager -l status -- %s 2>&1 | head -n 30 || true", mcpHostPrivilegePrelude(), unit, unit, unit)
}

func mcpSystemdActionCommand(action, serviceName string) string {
	unit := mcpShellQuote(serviceName)
	return fmt.Sprintf("%s$MCP_SUDO systemctl %s -- %s && { $MCP_SUDO systemctl is-active -- %s 2>&1 || true; $MCP_SUDO systemctl --no-pager -l status -- %s 2>&1 | head -n 30 || true; }", mcpHostPrivilegePrelude(), action, unit, unit, unit)
}

func mcpServiceActionText(action string) string {
	switch action {
	case "start":
		return "启动"
	case "stop":
		return "停止"
	case "restart":
		return "重启"
	default:
		return action
	}
}

func mcpLogClearCommand(files []mcpHostLogFile) string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, mcpShellQuote(file.Path))
	}
	return fmt.Sprintf("truncate -s 0 -- %s", strings.Join(paths, " "))
}

func isMCPExplicitLogFilePath(target string) bool {
	switch strings.ToLower(pathpkg.Ext(pathpkg.Clean(target))) {
	case ".log", ".txt":
		return true
	default:
		return false
	}
}

func isAllowedMCPExplicitLogPath(target string) bool {
	target = pathpkg.Clean(target)
	for _, prefix := range []string{"/tmp/", "/var/tmp/", "/var/log/", "/opt/", "/data/", "/srv/", "/app/", "/apps/", "/usr/local/", "/home/"} {
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return false
}

func mcpHostPrivilegePrelude() string {
	return `MCP_SUDO=""; if [ "$(id -u)" -ne 0 ]; then MCP_SUDO="sudo -n"; fi; `
}

func mcpShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func parseMCPDiskItems(output string) []mcpHostDiskItem {
	items := make([]mcpHostDiskItem, 0)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			parts = strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			parts = []string{parts[0], strings.Join(parts[1:], " ")}
		}
		size, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil {
			continue
		}
		items = append(items, mcpHostDiskItem{Path: strings.TrimSpace(parts[1]), SizeBytes: size, Size: mcpHumanBytes(size)})
	}
	return items
}

func totalMCPLogSize(files []mcpHostLogFile) uint64 {
	var total uint64
	for _, file := range files {
		total += file.SizeBytes
	}
	return total
}

func mcpHumanBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := uint64(unit), 0
	for quotient := value / unit; quotient >= unit && exp < 5; quotient /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}

func mcpHostIntArgument(args map[string]interface{}, key string, fallback int) int {
	value, exists := args[key]
	if !exists || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case uint:
		return int(typed)
	case uint32:
		return int(typed)
	case uint64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed)
		}
	}
	return fallback
}

func mcpHostBoolArgument(args map[string]interface{}, key string) bool {
	value, _ := args[key].(bool)
	return value
}

func mcpHostStringArray(args map[string]interface{}, key string) []string {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil
	}
	switch values := raw.(type) {
	case []string:
		return append([]string(nil), values...)
	case []interface{}:
		result := make([]string, 0, len(values))
		for _, value := range values {
			result = append(result, fmt.Sprint(value))
		}
		return result
	default:
		return nil
	}
}

func newMCPHostConfirmation(c *gin.Context, tool string, hostID, authID uint, scope, command string, itemCount int, totalSize uint64) (string, string, error) {
	admin, err := mcpExecutionAdmin(c)
	if err != nil {
		return "", "", err
	}
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", "", fmt.Errorf("create confirmation nonce: %w", err)
	}
	expiresAt := time.Now().Add(mcpHostConfirmationTTL)
	payload := mcpHostConfirmation{
		Version: mcpHostConfirmationVersion, UserID: admin.ID, Tool: tool, HostID: hostID, AuthID: authID,
		Scope: scope, Command: command, ItemCount: itemCount, TotalSize: totalSize,
		ExpiresAt: expiresAt.Unix(), Nonce: base64.RawURLEncoding.EncodeToString(nonceBytes),
	}
	encoded, err := encodeMCPHostConfirmation(payload)
	if err != nil {
		return "", "", err
	}
	return encoded, expiresAt.Format(time.RFC3339), nil
}

func encodeMCPHostConfirmation(payload mcpHostConfirmation) (string, error) {
	secret, err := mcpHostConfirmationSecret()
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(data)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + signature, nil
}

func verifyMCPHostConfirmation(c *gin.Context, confirmationID, tool string, hostID, authID uint, scope string) (*mcpHostConfirmation, error) {
	payload, err := decodeMCPHostConfirmation(confirmationID)
	if err != nil {
		return nil, err
	}
	admin, err := mcpExecutionAdmin(c)
	if err != nil {
		return nil, err
	}
	if payload.UserID != admin.ID || payload.Tool != tool || payload.HostID != hostID || payload.AuthID != authID || payload.Scope != scope {
		return nil, errors.New("confirmationId does not match the current user, host or operation")
	}
	return payload, nil
}

func decodeMCPHostConfirmation(confirmationID string) (*mcpHostConfirmation, error) {
	if strings.TrimSpace(confirmationID) == "" {
		return nil, errors.New("confirmationId is required")
	}
	if len(confirmationID) > 1024*1024 {
		return nil, errors.New("confirmationId is too large")
	}
	parts := strings.Split(confirmationID, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid confirmationId")
	}
	secret, err := mcpHostConfirmationSecret()
	if err != nil {
		return nil, err
	}
	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid confirmationId signature")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(providedSignature, mac.Sum(nil)) {
		return nil, errors.New("confirmationId signature mismatch")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid confirmationId payload")
	}
	var payload mcpHostConfirmation
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, errors.New("invalid confirmationId payload")
	}
	if payload.Version != mcpHostConfirmationVersion || payload.UserID == 0 || payload.Tool == "" || payload.HostID == 0 || payload.Command == "" || payload.Nonce == "" {
		return nil, errors.New("invalid confirmationId payload")
	}
	if time.Now().Unix() > payload.ExpiresAt {
		return nil, errors.New("confirmationId has expired; request a new preview")
	}
	return &payload, nil
}

func mcpHostConfirmationSecret() ([]byte, error) {
	if strings.TrimSpace(string(jwt.Secret)) == "" {
		return nil, errors.New("JWT secret is required for action confirmation signing")
	}
	return append([]byte("mcp-host-confirmation-v2:"), jwt.Secret...), nil
}

func consumeMCPHostConfirmation(confirmationID string, expiresAt int64) error {
	digest := sha256.Sum256([]byte(confirmationID))
	key := fmt.Sprintf("%x", digest[:])
	now := time.Now().Unix()
	mcpHostUsedConfirmations.Lock()
	defer mcpHostUsedConfirmations.Unlock()
	for item, expiry := range mcpHostUsedConfirmations.items {
		if expiry < now {
			delete(mcpHostUsedConfirmations.items, item)
		}
	}
	if _, used := mcpHostUsedConfirmations.items[key]; used {
		return errors.New("confirmationId has already been used")
	}
	mcpHostUsedConfirmations.items[key] = expiresAt
	return nil
}
