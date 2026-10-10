package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	mu          sync.RWMutex
	path        string
	publicKey   ed25519.PublicKey
	machineCode func() (string, error)
	now         func() time.Time
}

func NewManager(path string, publicKey ed25519.PublicKey) *Manager {
	return &Manager{
		path:        path,
		publicKey:   append(ed25519.PublicKey(nil), publicKey...),
		machineCode: MachineCode,
		now:         time.Now,
	}
}

var (
	defaultOnce sync.Once
	defaultMgr  *Manager
)

func DefaultManager() *Manager {
	defaultOnce.Do(func() {
		decoded, err := base64.RawStdEncoding.DecodeString(EmbeddedPublicKeyBase64)
		if err != nil || len(decoded) != ed25519.PublicKeySize {
			decoded = nil
		}
		defaultMgr = NewManager(DefaultPath(), ed25519.PublicKey(decoded))
	})
	return defaultMgr
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.statusLocked()
}

func (m *Manager) Licensed() bool {
	return m.Status().Licensed
}

func (m *Manager) Activate(token string) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	token = strings.Join(strings.Fields(token), "")
	machineCode, err := m.machineCode()
	if err != nil {
		return m.statusForError(machineCode, Claims{}, err), err
	}
	claims, err := m.verifyForCurrentInstallation(token, machineCode)
	if err != nil {
		return m.statusForError(machineCode, claims, err), err
	}
	if err := writeFileAtomic(m.path, []byte(token+"\n")); err != nil {
		return m.statusForError(machineCode, claims, fmt.Errorf("save license: %w", err)), err
	}
	return m.statusFromClaims(machineCode, claims), nil
}

func (m *Manager) statusLocked() Status {
	machineCode, machineErr := m.machineCode()
	if machineErr != nil {
		return m.statusForError(machineCode, Claims{}, machineErr)
	}
	if len(m.publicKey) != ed25519.PublicKeySize {
		return m.statusForError(machineCode, Claims{}, ErrPublicKey)
	}
	token, err := os.ReadFile(m.path)
	if errors.Is(err, os.ErrNotExist) {
		return m.statusForError(machineCode, Claims{}, ErrLicenseMissing)
	}
	if err != nil {
		return m.statusForError(machineCode, Claims{}, fmt.Errorf("read license: %w", err))
	}
	claims, err := m.verifyForCurrentInstallation(strings.TrimSpace(string(token)), machineCode)
	if err != nil {
		return m.statusForError(machineCode, claims, err)
	}
	return m.statusFromClaims(machineCode, claims)
}

func (m *Manager) statusFromClaims(machineCode string, claims Claims) Status {
	issuedAt := time.Unix(claims.IssuedAt, 0)
	status := Status{
		Licensed:       true,
		State:          StateValid,
		Message:        "授权有效",
		MachineCode:    machineCode,
		LicenseID:      claims.LicenseID,
		Customer:       claims.Customer,
		IssuedAt:       &issuedAt,
		Unlimited:      claims.ExpiresAt == 0,
		AllowedModules: []string{"全部模块"},
		BindingType:    effectiveBindingType(claims),
	}
	if claims.ExpiresAt > 0 {
		expiresAt := time.Unix(claims.ExpiresAt, 0)
		status.ExpiresAt = &expiresAt
		seconds := expiresAt.Sub(m.now()).Seconds()
		if seconds > 0 {
			status.RemainingDays = int64(math.Ceil(seconds / (24 * 60 * 60)))
		}
	}
	return status
}

func (m *Manager) statusForError(machineCode string, claims Claims, err error) Status {
	status := Status{
		Licensed:       false,
		State:          StateInvalid,
		Message:        "授权无效",
		MachineCode:    machineCode,
		LicenseID:      claims.LicenseID,
		Customer:       claims.Customer,
		Unlimited:      false,
		AllowedModules: BasicModules(),
		BindingType:    effectiveBindingType(claims),
	}
	if claims.IssuedAt > 0 {
		issuedAt := time.Unix(claims.IssuedAt, 0)
		status.IssuedAt = &issuedAt
	}
	if claims.ExpiresAt > 0 {
		expiresAt := time.Unix(claims.ExpiresAt, 0)
		status.ExpiresAt = &expiresAt
	}
	switch {
	case errors.Is(err, ErrLicenseMissing):
		status.State, status.Message = StateMissing, "系统尚未授权"
	case errors.Is(err, ErrLicenseExpired):
		status.State, status.Message = StateExpired, "授权已过期"
	case errors.Is(err, ErrMachineMismatch):
		status.State, status.Message = StateMachine, "授权码与当前机器不匹配"
	case errors.Is(err, ErrLicenseNotYet):
		status.State, status.Message = StateNotYet, "授权尚未生效"
	case errors.Is(err, ErrPublicKey):
		status.State, status.Message = StateNoKey, "发行公钥未配置"
	case errors.Is(err, ErrInstallationIdentity):
		status.State, status.Message = StateInstallation, "无法初始化安装实例，请检查 AutoOps 数据目录读写权限"
	}
	return status
}

func (m *Manager) verifyForCurrentInstallation(token, installationMachineCode string) (Claims, error) {
	claims, err := verifySignature(token, m.publicKey)
	if err != nil {
		return claims, err
	}
	err = validateRuntimeClaims(claims, installationMachineCode, m.now())
	if err == nil || !errors.Is(err, ErrMachineMismatch) || (claims.BindingType != "" && claims.BindingType != BindingLegacy) {
		return claims, err
	}

	// Licenses issued by earlier versions did not contain a binding type and
	// used an OS-specific hardware fingerprint. Keep them valid during upgrade.
	legacyMachineCode, legacyErr := LegacyMachineCode()
	if legacyErr != nil {
		return claims, err
	}
	return claims, validateRuntimeClaims(claims, legacyMachineCode, m.now())
}

func effectiveBindingType(claims Claims) string {
	if claims.BindingType == "" {
		if claims.LicenseID == "" {
			return BindingInstallationV1
		}
		return BindingLegacy
	}
	return claims.BindingType
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".license-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
