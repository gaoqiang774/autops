package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dodevops-api/pkg/license"

	"github.com/google/uuid"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "keygen":
		err = runKeygen(os.Args[2:])
	case "issue":
		err = runIssue(os.Args[2:])
	case "machine":
		err = runMachine(os.Args[2:])
	default:
		usage()
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "licensectl:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
	licensectl machine [--data-dir <persistent-directory>]
  licensectl keygen --private <file> --public <file>
  licensectl issue --private <file> --machine <code> [--duration 3m|6m|1y|unlimited] [--expires-at <RFC3339|YYYY-MM-DD>] [--customer <name>]`)
}

func runMachine(args []string) error {
	fs := flag.NewFlagSet("machine", flag.ContinueOnError)
	dataDir := fs.String("data-dir", "", "persistent AutoOps data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*dataDir) != "" {
		if err := os.Setenv(license.DataDirEnv, strings.TrimSpace(*dataDir)); err != nil {
			return err
		}
	}
	code, err := license.MachineCode()
	if err != nil {
		return err
	}
	fmt.Println(code)
	return nil
}

func runKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	privatePath := fs.String("private", "", "private key PEM path")
	publicPath := fs.String("public", "", "public key PEM path")
	force := fs.Bool("force", false, "overwrite existing key files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*privatePath) == "" || strings.TrimSpace(*publicPath) == "" {
		return errors.New("--private and --public are required")
	}
	if !*force {
		for _, path := range []string{*privatePath, *publicPath} {
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("%s already exists (use --force to replace)", path)
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return err
	}
	if err := writePEM(*privatePath, "PRIVATE KEY", privateDER, 0600); err != nil {
		return err
	}
	if err := writePEM(*publicPath, "PUBLIC KEY", publicDER, 0644); err != nil {
		return err
	}
	fmt.Printf("Private key: %s\nPublic key:  %s\nEmbedded key: %s\n", *privatePath, *publicPath, base64.RawStdEncoding.EncodeToString(publicKey))
	return nil
}

func runIssue(args []string) error {
	fs := flag.NewFlagSet("issue", flag.ContinueOnError)
	privatePath := fs.String("private", "", "private key PEM path")
	machineCode := fs.String("machine", "", "target machine code")
	duration := fs.String("duration", "", "3m, 6m, 1y, or unlimited")
	expiresAtValue := fs.String("expires-at", "", "custom expiration (RFC3339 or YYYY-MM-DD)")
	customer := fs.String("customer", "", "customer name")
	licenseID := fs.String("license-id", "", "optional stable license ID")
	notBeforeValue := fs.String("not-before", "", "optional start time (RFC3339)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*privatePath) == "" || strings.TrimSpace(*machineCode) == "" {
		return errors.New("--private and --machine are required")
	}
	if strings.TrimSpace(*duration) != "" && strings.TrimSpace(*expiresAtValue) != "" {
		return errors.New("use either --duration or --expires-at, not both")
	}
	if strings.TrimSpace(*duration) == "" && strings.TrimSpace(*expiresAtValue) == "" {
		return errors.New("--duration or --expires-at is required")
	}

	privateKey, err := readPrivateKey(*privatePath)
	if err != nil {
		return err
	}
	now := time.Now()
	expiresAt, err := parseExpiration(now, *duration, *expiresAtValue)
	if err != nil {
		return err
	}
	var notBefore int64
	if strings.TrimSpace(*notBeforeValue) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*notBeforeValue))
		if err != nil {
			return fmt.Errorf("invalid --not-before: %w", err)
		}
		notBefore = parsed.Unix()
	}
	id := strings.TrimSpace(*licenseID)
	if id == "" {
		id = uuid.NewString()
	}
	claims := license.Claims{
		Version:     license.FormatVersion,
		Product:     license.ProductName,
		LicenseID:   id,
		Customer:    strings.TrimSpace(*customer),
		MachineCode: strings.ToUpper(strings.TrimSpace(*machineCode)),
		IssuedAt:    now.Unix(),
		NotBefore:   notBefore,
		BindingType: license.BindingInstallationV1,
	}
	if !expiresAt.IsZero() {
		claims.ExpiresAt = expiresAt.Unix()
	}
	token, err := license.Sign(claims, privateKey)
	if err != nil {
		return err
	}
	fmt.Println(token)
	return nil
}

func parseExpiration(now time.Time, durationValue, expiresAtValue string) (time.Time, error) {
	switch strings.ToLower(strings.TrimSpace(durationValue)) {
	case "3m":
		return now.AddDate(0, 3, 0), nil
	case "6m":
		return now.AddDate(0, 6, 0), nil
	case "1y":
		return now.AddDate(1, 0, 0), nil
	case "unlimited":
		return time.Time{}, nil
	case "":
	default:
		return time.Time{}, fmt.Errorf("unsupported --duration %q", durationValue)
	}

	value := strings.TrimSpace(expiresAtValue)
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		if !parsed.After(now) {
			return time.Time{}, errors.New("expiration must be in the future")
		}
		return parsed, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --expires-at: use RFC3339 or YYYY-MM-DD")
	}
	parsed = parsed.Add(24*time.Hour - time.Second)
	if !parsed.After(now) {
		return time.Time{}, errors.New("expiration must be in the future")
	}
	return parsed, nil
}

func readPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("private key is not valid PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	privateKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not Ed25519")
	}
	return privateKey, nil
}

func writePEM(path, blockType string, der []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if err := pem.Encode(file, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
