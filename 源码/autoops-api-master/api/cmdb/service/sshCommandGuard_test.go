package service

import "testing"

func TestDetectDangerousSSHCommand(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		dangerous bool
	}{
		{name: "plain rm", command: "rm /tmp/a.txt", dangerous: true},
		{name: "sudo rm", command: "sudo -u root rm -rf /tmp/a", dangerous: true},
		{name: "sudo combined option rm", command: "sudo -iu root rm -rf /tmp/a", dangerous: true},
		{name: "chained rm", command: "echo ok && rm -rf /tmp/a", dangerous: true},
		{name: "shell inline rm", command: "bash -c 'rm -rf /tmp/a'", dangerous: true},
		{name: "find delete", command: "find /tmp -type f -delete", dangerous: true},
		{name: "find exec rm", command: "find /tmp -type f -exec rm -f {} ;", dangerous: true},
		{name: "safe list", command: "ls -la /tmp", dangerous: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectDangerousSSHCommand(tt.command)
			if got.Dangerous != tt.dangerous {
				t.Fatalf("DetectDangerousSSHCommand(%q).Dangerous = %v, want %v; risk=%+v", tt.command, got.Dangerous, tt.dangerous, got)
			}
		})
	}
}

func TestEffectiveCommandSkipsWrappers(t *testing.T) {
	commandName, args := effectiveCommand(shellFields("sudo -u root env A=B command rm -f /tmp/a"))
	if commandName != "rm" {
		t.Fatalf("commandName = %q, want rm; args=%v", commandName, args)
	}
}
