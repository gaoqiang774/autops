package websocket

import (
	"bytes"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestNormalizeTerminalInputStripsBracketedPasteMarkers(t *testing.T) {
	input := []byte("\x1b[200~curl --location --request POST\x1b[201~")

	got := normalizeTerminalInput(input)
	want := []byte("curl --location --request POST")

	if !bytes.Equal(got, want) {
		t.Fatalf("normalizeTerminalInput() = %q, want %q", got, want)
	}
}

func TestNormalizeTerminalInputDropsCtrlC(t *testing.T) {
	input := []byte{'a', terminalCtrlC, 'b'}

	got := normalizeTerminalInput(input)
	want := []byte("ab")

	if !bytes.Equal(got, want) {
		t.Fatalf("normalizeTerminalInput() = %q, want %q", got, want)
	}
}

func TestTerminalModesMovesInterruptToCtrlZ(t *testing.T) {
	modes := terminalModes(1)

	if got := modes[ssh.VINTR]; got != terminalCtrlZ {
		t.Fatalf("VINTR = %d, want %d", got, terminalCtrlZ)
	}
	if got := modes[ssh.VSUSP]; got != terminalSpecialCharDisabled {
		t.Fatalf("VSUSP = %d, want %d", got, terminalSpecialCharDisabled)
	}
}
