package license

import (
	"regexp"
	"testing"
)

func TestMachineCodeFromIdentityIsStableAndFormatted(t *testing.T) {
	first := machineCodeFromIdentity("linux|machine-id|abc")
	second := machineCodeFromIdentity("linux|machine-id|abc")
	if first != second {
		t.Fatalf("machine code is not stable: %q != %q", first, second)
	}
	if !regexp.MustCompile(`^AOPS-(?:[A-F0-9]{4}-){7}[A-F0-9]{4}$`).MatchString(first) {
		t.Fatalf("unexpected machine code format: %q", first)
	}
	if first == machineCodeFromIdentity("linux|machine-id|xyz") {
		t.Fatal("different identities produced the same machine code")
	}
}
