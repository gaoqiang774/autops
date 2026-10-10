package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"
)

func testClaims(now time.Time) Claims {
	return Claims{
		Version:     FormatVersion,
		Product:     ProductName,
		LicenseID:   "license-test-1",
		Customer:    "测试客户",
		MachineCode: "AOPS-1111-2222-3333-4444-5555-6666-7777-8888",
		IssuedAt:    now.Add(-time.Hour).Unix(),
		ExpiresAt:   now.Add(24 * time.Hour).Unix(),
	}
}

func TestSignAndVerify(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	claims := testClaims(now)
	token, err := Sign(claims, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := Verify(token, publicKey, claims.MachineCode, now)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if verified.LicenseID != claims.LicenseID {
		t.Fatalf("license ID = %q, want %q", verified.LicenseID, claims.LicenseID)
	}
}

func TestVerifyRejectsTamperedLicense(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now()
	claims := testClaims(now)
	token, err := Sign(claims, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	parts[0] = parts[0][:len(parts[0])-1] + "A"
	_, err = Verify(strings.Join(parts, "."), publicKey, claims.MachineCode, now)
	if !errors.Is(err, ErrInvalidLicense) {
		t.Fatalf("Verify() error = %v, want ErrInvalidLicense", err)
	}
}

func TestVerifyRejectsWrongMachineAndTimeBounds(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		mutate  func(*Claims)
		machine string
		wantErr error
	}{
		{name: "wrong machine", machine: "AOPS-OTHER", wantErr: ErrMachineMismatch},
		{name: "expired", mutate: func(c *Claims) { c.ExpiresAt = now.Add(-time.Minute).Unix() }, wantErr: ErrLicenseExpired},
		{name: "not yet valid", mutate: func(c *Claims) { c.NotBefore = now.Add(time.Hour).Unix() }, wantErr: ErrLicenseNotYet},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := testClaims(now)
			if tt.mutate != nil {
				tt.mutate(&claims)
			}
			token, err := Sign(claims, privateKey)
			if err != nil {
				t.Fatal(err)
			}
			machine := tt.machine
			if machine == "" {
				machine = claims.MachineCode
			}
			_, err = Verify(token, publicKey, machine, now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Verify() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyUnlimitedLicense(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now()
	claims := testClaims(now)
	claims.ExpiresAt = 0
	token, err := Sign(claims, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(token, publicKey, claims.MachineCode, now.AddDate(20, 0, 0)); err != nil {
		t.Fatalf("unlimited Verify() error = %v", err)
	}
}

func TestEmbeddedPublicKey(t *testing.T) {
	manager := DefaultManager()
	if len(manager.publicKey) != ed25519.PublicKeySize {
		t.Fatalf("embedded public key length = %d", len(manager.publicKey))
	}
}
