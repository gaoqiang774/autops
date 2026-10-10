package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManagerActivationPersistsVerifiedLicense(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "nested", "license.key")
	manager := NewManager(path, publicKey)
	manager.now = func() time.Time { return now }
	manager.machineCode = func() (string, error) { return "AOPS-TEST-MACHINE", nil }
	claims := testClaims(now)
	claims.MachineCode = "AOPS-TEST-MACHINE"
	token, err := Sign(claims, privateKey)
	if err != nil {
		t.Fatal(err)
	}

	status, err := manager.Activate(token)
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if !status.Licensed || !manager.Licensed() {
		t.Fatalf("status = %+v, want licensed", status)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("license mode = %o, want 600", info.Mode().Perm())
	}
}

func TestManagerDoesNotPersistInvalidLicense(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now()
	path := filepath.Join(t.TempDir(), "license.key")
	manager := NewManager(path, publicKey)
	manager.now = func() time.Time { return now }
	manager.machineCode = func() (string, error) { return "AOPS-CURRENT", nil }
	claims := testClaims(now)
	claims.MachineCode = "AOPS-ANOTHER"
	token, _ := Sign(claims, privateKey)

	if _, err := manager.Activate(token); err == nil {
		t.Fatal("Activate() error = nil, want machine mismatch")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid license was persisted: %v", err)
	}
}
