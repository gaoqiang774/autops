package websocket

import (
	"fmt"
	"strings"
	"time"

	"dodevops-api/common/util/recording"
)

const terminalCommandConfirmationWindow = 15 * time.Second

type TerminalCommandDecision struct {
	Dangerous  bool
	CanConfirm bool
	Message    string
}

type TerminalCommandPolicy func(command string) TerminalCommandDecision

type pendingTerminalCommand struct {
	Command   string
	ExpiresAt time.Time
	Response  []rune
}

type terminalCommandEvaluation struct {
	Allow  bool
	Notice string
	Prompt bool
}

func (w *WebSSH) SetTerminalCommandPolicy(policy TerminalCommandPolicy) {
	w.commandPolicy = policy
	if policy != nil && w.commandParser == nil {
		w.commandParser = recording.NewCommandParser()
	}
}

func (w *WebSSH) filterTerminalInput(data []byte) ([]byte, bool) {
	if w.commandPolicy == nil {
		return data, false
	}
	if w.commandParser == nil {
		w.commandParser = recording.NewCommandParser()
	}
	if w.pendingCommand != nil {
		return w.filterTerminalConfirmationInput(data, time.Now())
	}

	completed, parsed := w.commandParser.ParseInput(data, float64(time.Now().UnixNano())/1e9)
	if !completed || parsed == nil {
		return data, false
	}

	result := w.evaluateTerminalCommand(parsed.Command, time.Now())
	if result.Allow {
		return data, false
	}

	if result.Prompt {
		_ = w.printTerminalPrompt(result.Notice)
	} else {
		_ = w.printTerminalNotice(result.Notice)
	}
	return nil, true
}

func (w *WebSSH) evaluateTerminalCommand(command string, now time.Time) terminalCommandEvaluation {
	command = strings.TrimSpace(command)
	if command == "" {
		return terminalCommandEvaluation{Allow: true}
	}

	if w.pendingCommand != nil && now.After(w.pendingCommand.ExpiresAt) {
		w.pendingCommand = nil
	}

	decision := w.commandPolicy(command)
	if !decision.Dangerous {
		return terminalCommandEvaluation{Allow: true}
	}

	if !decision.CanConfirm {
		w.pendingCommand = nil
		return terminalCommandEvaluation{
			Allow:  false,
			Notice: decorateTerminalNotice(decision.Message),
		}
	}

	w.pendingCommand = &pendingTerminalCommand{
		Command:   command,
		ExpiresAt: now.Add(terminalCommandConfirmationWindow),
		Response:  make([]rune, 0, 8),
	}
	return terminalCommandEvaluation{
		Allow:  false,
		Notice: buildDeleteConfirmationPrompt(command, decision.Message),
		Prompt: true,
	}
}

func (w *WebSSH) filterTerminalConfirmationInput(data []byte, now time.Time) ([]byte, bool) {
	pending := w.pendingCommand
	if pending == nil {
		return data, false
	}

	if now.After(pending.ExpiresAt) {
		w.pendingCommand = nil
		_ = w.printTerminalNotice(decorateTerminalNotice("确认超时，已取消删除命令。"))
		return nil, true
	}

	completed, answer, echo := pending.consumeInput(data)
	if len(echo) > 0 {
		_ = w.writeTerminalOutput(echo)
	}
	if !completed {
		return nil, true
	}

	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		w.pendingCommand = nil
		_ = w.printTerminalNotice(decorateTerminalNotice("已确认删除，正在执行命令。"))
		return []byte(pending.Command + "\n"), false
	case "n", "no":
		w.pendingCommand = nil
		_ = w.printTerminalNotice(decorateTerminalNotice("已取消删除命令。"))
		return nil, true
	default:
		pending.Response = pending.Response[:0]
		_ = w.printTerminalPrompt(decorateTerminalNotice("请输入 y 确认删除，或输入 n 取消："))
		return nil, true
	}
}

func (p *pendingTerminalCommand) consumeInput(data []byte) (bool, string, []byte) {
	var echo []byte
	for _, b := range data {
		switch b {
		case '\r', '\n':
			echo = append(echo, "\r\n"...)
			return true, string(p.Response), echo
		case '\b', 0x7f:
			if len(p.Response) > 0 {
				p.Response = p.Response[:len(p.Response)-1]
				echo = append(echo, "\b \b"...)
			}
		default:
			if b >= 32 && b <= 126 {
				p.Response = append(p.Response, rune(b))
				echo = append(echo, b)
			}
		}
	}
	return false, "", echo
}

func (w *WebSSH) syncTerminalParserOutput(data []byte) {
	if w.commandParser == nil || len(data) == 0 {
		return
	}
	_ = w.commandParser.ParseOutput(data, float64(time.Now().UnixNano())/1e9)
}

func (w *WebSSH) printTerminalNotice(message string) error {
	clearErr := w.clearRemoteInputLine()
	if err := w.writeTerminalOutput([]byte("\r\x1b[2K" + message + "\r\n")); err != nil {
		return err
	}
	return clearErr
}

func (w *WebSSH) printTerminalPrompt(message string) error {
	clearErr := w.clearRemoteInputLine()
	if err := w.writeTerminalOutput([]byte("\r\x1b[2K" + message)); err != nil {
		return err
	}
	return clearErr
}

func (w *WebSSH) clearRemoteInputLine() error {
	if w.stdinPipe == nil {
		return nil
	}
	// Ctrl+U clears the current input line in common shell line editors.
	_, err := w.stdinPipe.Write([]byte{0x15})
	return err
}

func buildDeleteConfirmationPrompt(command, message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		message = fmt.Sprintf("检测到危险删除命令：%s", command)
	}
	return fmt.Sprintf("%s\r\n[AIOps] 是否确认删除？请输入 y 确认，n 取消：", decorateTerminalNotice(message))
}

func decorateTerminalNotice(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "[AIOps] 命令已拦截。"
	}
	return "[AIOps] " + message
}
