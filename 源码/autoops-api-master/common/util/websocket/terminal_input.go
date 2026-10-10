package websocket

import (
	"bytes"

	"golang.org/x/crypto/ssh"
)

const (
	terminalCtrlC = byte(0x03)
	terminalCtrlZ = uint32(0x1a)

	// RFC 4254 uses 255 to disable a PTY special character.
	terminalSpecialCharDisabled = uint32(255)
)

var (
	bracketedPasteStart = []byte("\x1b[200~")
	bracketedPasteEnd   = []byte("\x1b[201~")
)

func terminalModes(echo uint32) ssh.TerminalModes {
	return ssh.TerminalModes{
		ssh.VINTR:         terminalCtrlZ,
		ssh.VSUSP:         terminalSpecialCharDisabled,
		ssh.ECHO:          echo,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
		ssh.ONLCR:         1,
		ssh.OCRNL:         0,
		ssh.INLCR:         0,
		ssh.IGNCR:         0,
		ssh.ICRNL:         1,
		ssh.OPOST:         1,
		ssh.ONLRET:        0,
		ssh.ONOCR:         0,
	}
}

func normalizeTerminalInput(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	data = bytes.ReplaceAll(data, bracketedPasteStart, nil)
	data = bytes.ReplaceAll(data, bracketedPasteEnd, nil)
	if bytes.IndexByte(data, terminalCtrlC) < 0 {
		return data
	}

	normalized := data[:0]
	for _, b := range data {
		if b != terminalCtrlC {
			normalized = append(normalized, b)
		}
	}
	return normalized
}
