package service

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func zipFixture(t *testing.T, name, content string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestUnzipContentRejectsPathTraversal(t *testing.T) {
	destination := t.TempDir()
	err := unzipContent(zipFixture(t, "../outside.txt", "secret"), destination)
	if err == nil {
		t.Fatal("path traversal archive was accepted")
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(destination), "outside.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("archive wrote outside destination: %v", statErr)
	}
}

func TestUnzipContentExtractsRegularFile(t *testing.T) {
	destination := t.TempDir()
	if err := unzipContent(zipFixture(t, "role/tasks/main.yml", "---\n"), destination); err != nil {
		t.Fatalf("unzipContent() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(destination, "role", "tasks", "main.yml"))
	if err != nil || string(content) != "---\n" {
		t.Fatalf("unexpected extracted content %q, error %v", content, err)
	}
}

func TestGenerateInventoryWritesPrivateKeyFile(t *testing.T) {
	projectDir := t.TempDir()
	collection := &HostSSHInfoCollection{
		Groups: map[string][]HostSSHInfo{
			"web": {{ID: 12, IP: "10.0.0.12", Port: 22, User: "root", Key: "PRIVATE KEY", AuthType: 2}},
		},
	}
	inventory, err := collection.GenerateInventory(projectDir)
	if err != nil {
		t.Fatalf("GenerateInventory() error = %v", err)
	}
	keyPath, err := filepath.Abs(filepath.Join(projectDir, ".keys", "host-12"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inventory, strconv.Quote(keyPath)) {
		t.Fatalf("inventory does not reference generated key: %s", inventory)
	}
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		t.Fatalf("private key permissions are too broad: %o", info.Mode().Perm())
	}
}

func TestValidateTaskDirectoryName(t *testing.T) {
	for _, valid := range []string{"deploy-prod", "巡检_01", "release.v2"} {
		if err := validateTaskDirectoryName(valid); err != nil {
			t.Fatalf("valid name %q rejected: %v", valid, err)
		}
	}
	for _, invalid := range []string{"../escape", "a/b", "..", "with space"} {
		if err := validateTaskDirectoryName(invalid); err == nil {
			t.Fatalf("invalid name %q accepted", invalid)
		}
	}
}
