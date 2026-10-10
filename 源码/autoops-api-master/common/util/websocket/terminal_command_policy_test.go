package websocket

import (
	"strings"
	"testing"
	"time"

	"dodevops-api/common/util/recording"
)

func TestEvaluateTerminalCommandBlocksNonAdminDangerousCommand(t *testing.T) {
	webSSH := &WebSSH{
		commandParser: recording.NewCommandParser(),
		commandPolicy: func(command string) TerminalCommandDecision {
			if strings.HasPrefix(command, "rm ") {
				return TerminalCommandDecision{
					Dangerous:  true,
					CanConfirm: false,
					Message:    "blocked",
				}
			}
			return TerminalCommandDecision{}
		},
	}

	result := webSSH.evaluateTerminalCommand("rm -rf /tmp/a", time.Now())
	if result.Allow {
		t.Fatalf("expected command to be blocked")
	}
	if result.Prompt {
		t.Fatalf("non-confirmable command should not prompt")
	}
	if !strings.Contains(result.Notice, "blocked") {
		t.Fatalf("unexpected notice: %q", result.Notice)
	}
}

func TestEvaluateTerminalCommandPromptsForConfirmation(t *testing.T) {
	webSSH := newConfirmableDangerousWebSSH()

	result := webSSH.evaluateTerminalCommand("rm -rf /tmp/a", time.Now())
	if result.Allow {
		t.Fatalf("first dangerous command should require confirmation")
	}
	if !result.Prompt {
		t.Fatalf("dangerous admin command should prompt for y/n confirmation")
	}
	if !strings.Contains(result.Notice, "是否确认删除") {
		t.Fatalf("unexpected prompt: %q", result.Notice)
	}
	if webSSH.pendingCommand == nil {
		t.Fatalf("pending command should be stored")
	}
}

func TestFilterTerminalConfirmationAllowsYes(t *testing.T) {
	webSSH := newConfirmableDangerousWebSSH()
	now := time.Now()
	_ = webSSH.evaluateTerminalCommand("rm -rf /tmp/a", now)

	data, blocked := webSSH.filterTerminalConfirmationInput([]byte("y\r"), now.Add(time.Second))
	if blocked {
		t.Fatalf("yes confirmation should allow command execution")
	}
	if string(data) != "rm -rf /tmp/a\n" {
		t.Fatalf("unexpected command sent after confirmation: %q", data)
	}
	if webSSH.pendingCommand != nil {
		t.Fatalf("pending command should be cleared")
	}
}

func TestFilterTerminalConfirmationBlocksNo(t *testing.T) {
	webSSH := newConfirmableDangerousWebSSH()
	now := time.Now()
	_ = webSSH.evaluateTerminalCommand("rm -rf /tmp/a", now)

	data, blocked := webSSH.filterTerminalConfirmationInput([]byte("n\r"), now.Add(time.Second))
	if !blocked {
		t.Fatalf("no confirmation should block command execution")
	}
	if len(data) != 0 {
		t.Fatalf("no command should be sent after cancel, got %q", data)
	}
	if webSSH.pendingCommand != nil {
		t.Fatalf("pending command should be cleared")
	}
}

func TestFilterTerminalConfirmationExpires(t *testing.T) {
	webSSH := newConfirmableDangerousWebSSH()
	now := time.Now()
	_ = webSSH.evaluateTerminalCommand("rm -rf /tmp/a", now)

	data, blocked := webSSH.filterTerminalConfirmationInput([]byte("y\r"), now.Add(terminalCommandConfirmationWindow+time.Second))
	if !blocked {
		t.Fatalf("expired confirmation should block command execution")
	}
	if len(data) != 0 {
		t.Fatalf("expired command should not be sent, got %q", data)
	}
	if webSSH.pendingCommand != nil {
		t.Fatalf("pending command should be cleared after expiration")
	}
}

func newConfirmableDangerousWebSSH() *WebSSH {
	return &WebSSH{
		commandParser: recording.NewCommandParser(),
		commandPolicy: func(command string) TerminalCommandDecision {
			if strings.HasPrefix(command, "rm ") {
				return TerminalCommandDecision{
					Dangerous:  true,
					CanConfirm: true,
					Message:    "检测到危险删除命令，请确认是否继续执行",
				}
			}
			return TerminalCommandDecision{}
		},
	}
}
