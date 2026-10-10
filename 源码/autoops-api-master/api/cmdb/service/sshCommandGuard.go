package service

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"

	systemauthz "dodevops-api/api/system/authz"
	wsutil "dodevops-api/common/util/websocket"

	"github.com/gin-gonic/gin"
)

const (
	CommandPolicyForbidden       = "DANGEROUS_COMMAND_FORBIDDEN"
	CommandPolicyConfirmRequired = "DANGEROUS_COMMAND_CONFIRM_REQUIRED"
)

type CommandRisk struct {
	Dangerous bool   `json:"dangerous"`
	Category  string `json:"category"`
	Matched   string `json:"matched"`
	Reason    string `json:"reason"`
}

type CommandPolicyError struct {
	StatusCode           int          `json:"statusCode"`
	Code                 string       `json:"code"`
	Message              string       `json:"message"`
	RequiresConfirmation bool         `json:"requiresConfirmation"`
	Risk                 *CommandRisk `json:"risk,omitempty"`
}

func (e *CommandPolicyError) Error() string {
	return e.Message
}

func (s *CmdbHostSSHServiceImpl) checkCommandExecutionPolicy(c *gin.Context, command string, confirmed bool) error {
	risk := DetectDangerousSSHCommand(command)
	if !risk.Dangerous {
		return nil
	}

	isAdmin, err := isSSHCommandAdmin(c)
	if err != nil {
		return &CommandPolicyError{
			StatusCode: http.StatusUnauthorized,
			Code:       CommandPolicyForbidden,
			Message:    "未获取到当前用户信息，禁止执行危险命令",
			Risk:       &risk,
		}
	}
	if !isAdmin {
		return &CommandPolicyError{
			StatusCode: http.StatusForbidden,
			Code:       CommandPolicyForbidden,
			Message:    "当前用户不是管理员，禁止执行危险删除命令",
			Risk:       &risk,
		}
	}
	if !confirmed {
		return &CommandPolicyError{
			StatusCode:           http.StatusConflict,
			Code:                 CommandPolicyConfirmRequired,
			Message:              "危险删除命令需要确认后执行",
			RequiresConfirmation: true,
			Risk:                 &risk,
		}
	}

	return nil
}

func (s *CmdbHostSSHServiceImpl) buildTerminalCommandPolicy(c *gin.Context) wsutil.TerminalCommandPolicy {
	isAdmin, err := isSSHCommandAdmin(c)

	return func(command string) wsutil.TerminalCommandDecision {
		risk := DetectDangerousSSHCommand(command)
		if !risk.Dangerous {
			return wsutil.TerminalCommandDecision{}
		}
		if err != nil {
			return wsutil.TerminalCommandDecision{
				Dangerous:  true,
				CanConfirm: false,
				Message:    "未获取到当前用户信息，禁止执行危险命令",
			}
		}
		if !isAdmin {
			return wsutil.TerminalCommandDecision{
				Dangerous:  true,
				CanConfirm: false,
				Message:    "当前用户不是管理员，禁止执行危险删除命令",
			}
		}
		return wsutil.TerminalCommandDecision{
			Dangerous:  true,
			CanConfirm: true,
			Message:    "检测到危险删除命令，请确认是否继续执行",
		}
	}
}

func (s *CmdbHostSSHServiceImpl) ConfigureTerminalPolicy(c *gin.Context, webSSH *wsutil.WebSSH) {
	if webSSH == nil {
		return
	}
	webSSH.SetTerminalCommandPolicy(s.buildTerminalCommandPolicy(c))
}

func isSSHCommandAdmin(c *gin.Context) (bool, error) {
	return systemauthz.IsSystemAdmin(c)
}

func DetectDangerousSSHCommand(command string) CommandRisk {
	segments := splitShellSegments(command)
	for _, segment := range segments {
		if risk := detectDangerousShellSegment(segment); risk.Dangerous {
			return risk
		}
	}
	return CommandRisk{}
}

func detectDangerousShellSegment(segment string) CommandRisk {
	tokens := shellFields(segment)
	if len(tokens) == 0 {
		return CommandRisk{}
	}

	commandName, args := effectiveCommand(tokens)
	if commandName == "" {
		return CommandRisk{}
	}
	baseName := strings.ToLower(filepath.Base(commandName))

	if isDeleteExecutable(baseName) {
		return CommandRisk{
			Dangerous: true,
			Category:  "delete",
			Matched:   commandName,
			Reason:    fmt.Sprintf("检测到删除命令 %s", commandName),
		}
	}
	if baseName == "truncate" {
		return CommandRisk{
			Dangerous: true,
			Category:  "truncate",
			Matched:   commandName,
			Reason:    "检测到文件清空命令 truncate",
		}
	}

	if baseName == "find" && hasToken(args, "-delete") {
		return CommandRisk{
			Dangerous: true,
			Category:  "delete",
			Matched:   "find -delete",
			Reason:    "检测到 find -delete 删除操作",
		}
	}
	if baseName == "find" && findExecutesDelete(args) {
		return CommandRisk{
			Dangerous: true,
			Category:  "delete",
			Matched:   "find -exec rm",
			Reason:    "检测到 find -exec 删除命令",
		}
	}

	if isShellExecutable(baseName) {
		if inline, ok := shellInlineCommand(args); ok {
			return DetectDangerousSSHCommand(inline)
		}
	}

	if (baseName == "xargs" || baseName == "parallel") && containsDeleteExecutable(args) {
		return CommandRisk{
			Dangerous: true,
			Category:  "delete",
			Matched:   baseName,
			Reason:    fmt.Sprintf("检测到 %s 删除命令", baseName),
		}
	}

	return CommandRisk{}
}

func isDeleteExecutable(name string) bool {
	switch name {
	case "rm", "rmdir", "unlink":
		return true
	default:
		return false
	}
}

func isShellExecutable(name string) bool {
	switch name {
	case "sh", "bash", "zsh", "ksh", "dash":
		return true
	default:
		return false
	}
}

func shellInlineCommand(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-c" {
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", false
		}
		if strings.HasPrefix(arg, "-") && strings.Contains(arg, "c") && len(arg) > 2 {
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", false
		}
	}
	return "", false
}

func containsDeleteExecutable(tokens []string) bool {
	for _, token := range tokens {
		baseName := strings.ToLower(filepath.Base(token))
		if isDeleteExecutable(baseName) {
			return true
		}
	}
	return false
}

func findExecutesDelete(tokens []string) bool {
	for i, token := range tokens {
		if token != "-exec" && token != "-execdir" {
			continue
		}
		if i+1 >= len(tokens) {
			return false
		}
		commandName, _ := effectiveCommand(tokens[i+1:])
		if isDeleteExecutable(strings.ToLower(filepath.Base(commandName))) {
			return true
		}
	}
	return false
}

func hasToken(tokens []string, target string) bool {
	for _, token := range tokens {
		if token == target {
			return true
		}
	}
	return false
}

func effectiveCommand(tokens []string) (string, []string) {
	for len(tokens) > 0 {
		token := tokens[0]
		if isAssignment(token) {
			tokens = tokens[1:]
			continue
		}
		baseName := strings.ToLower(filepath.Base(token))
		switch baseName {
		case "sudo", "doas":
			tokens = skipPrivilegeWrapper(tokens[1:])
			continue
		case "env":
			tokens = skipEnvWrapper(tokens[1:])
			continue
		case "command", "builtin", "nohup":
			tokens = tokens[1:]
			continue
		case "time":
			tokens = skipTimeWrapper(tokens[1:])
			continue
		}
		return token, tokens[1:]
	}
	return "", nil
}

func skipPrivilegeWrapper(tokens []string) []string {
	for len(tokens) > 0 && strings.HasPrefix(tokens[0], "-") {
		option := tokens[0]
		tokens = tokens[1:]
		if consumesSudoArgument(option) && len(tokens) > 0 {
			tokens = tokens[1:]
		}
	}
	return tokens
}

func consumesSudoArgument(option string) bool {
	if strings.HasPrefix(option, "--user=") ||
		strings.HasPrefix(option, "--group=") ||
		strings.HasPrefix(option, "--host=") ||
		strings.HasPrefix(option, "--prompt=") ||
		strings.HasPrefix(option, "--close-from=") ||
		strings.HasPrefix(option, "--command-timeout=") {
		return false
	}
	switch option {
	case "-u", "--user", "-g", "--group", "-h", "--host", "-p", "--prompt", "-C", "--close-from", "-T", "--command-timeout":
		return true
	default:
		return strings.HasPrefix(option, "-") && !strings.HasPrefix(option, "--") && strings.ContainsAny(option[1:], "ughpCT")
	}
}

func skipEnvWrapper(tokens []string) []string {
	for len(tokens) > 0 {
		token := tokens[0]
		if isAssignment(token) {
			tokens = tokens[1:]
			continue
		}
		if strings.HasPrefix(token, "-") {
			tokens = tokens[1:]
			if (token == "-u" || token == "--unset") && len(tokens) > 0 {
				tokens = tokens[1:]
			}
			continue
		}
		break
	}
	return tokens
}

func skipTimeWrapper(tokens []string) []string {
	for len(tokens) > 0 && strings.HasPrefix(tokens[0], "-") {
		tokens = tokens[1:]
	}
	return tokens
}

func isAssignment(token string) bool {
	if token == "" {
		return false
	}
	idx := strings.IndexByte(token, '=')
	if idx <= 0 {
		return false
	}
	for _, r := range token[:idx] {
		if !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	first, _ := utf8FirstRune(token)
	return first == '_' || unicode.IsLetter(first)
}

func utf8FirstRune(s string) (rune, bool) {
	for _, r := range s {
		return r, true
	}
	return 0, false
}

func splitShellSegments(command string) []string {
	var segments []string
	var current strings.Builder
	var quote rune
	escaped := false

	flush := func() {
		segment := strings.TrimSpace(current.String())
		if segment != "" {
			segments = append(segments, segment)
		}
		current.Reset()
	}

	for _, r := range command {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			current.WriteRune(r)
			escaped = true
			continue
		}
		if quote != 0 {
			current.WriteRune(r)
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			current.WriteRune(r)
			continue
		}
		switch r {
		case ';', '|', '&', '\n', '\r':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return segments
}

func shellFields(input string) []string {
	var fields []string
	var current strings.Builder
	var quote rune
	escaped := false

	flush := func() {
		if current.Len() > 0 {
			fields = append(fields, current.String())
			current.Reset()
		}
	}

	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if unicode.IsSpace(r) {
			flush()
			continue
		}
		current.WriteRune(r)
	}
	if escaped {
		current.WriteRune('\\')
	}
	flush()
	return fields
}
