package license

import (
	"errors"
	"time"
)

const (
	ProductName       = "autoops"
	FormatVersion     = 1
	StateValid        = "valid"
	StateMissing      = "unlicensed"
	StateExpired      = "expired"
	StateMachine      = "machine_mismatch"
	StateNotYet       = "not_yet_valid"
	StateInvalid      = "invalid"
	StateNoKey        = "public_key_unconfigured"
	StateInstallation = "installation_identity_error"

	BindingLegacy         = "legacy-hardware-v1"
	BindingInstallationV1 = "installation-v1"
)

var (
	ErrLicenseMissing       = errors.New("license not installed")
	ErrLicenseExpired       = errors.New("license expired")
	ErrMachineMismatch      = errors.New("license machine code mismatch")
	ErrLicenseNotYet        = errors.New("license is not yet valid")
	ErrInvalidLicense       = errors.New("invalid license")
	ErrPublicKey            = errors.New("license public key is not configured")
	ErrInstallationIdentity = errors.New("installation identity is unavailable")
)

// Claims is the signed, immutable payload contained in a license key.
type Claims struct {
	Version     int    `json:"version"`
	Product     string `json:"product"`
	LicenseID   string `json:"licenseId"`
	Customer    string `json:"customer,omitempty"`
	MachineCode string `json:"machineCode"`
	IssuedAt    int64  `json:"issuedAt"`
	NotBefore   int64  `json:"notBefore,omitempty"`
	ExpiresAt   int64  `json:"expiresAt,omitempty"`
	BindingType string `json:"bindingType,omitempty"`
}

// Status is safe to return to an authenticated frontend. It never contains the
// license key or signing material.
type Status struct {
	Licensed       bool       `json:"licensed"`
	State          string     `json:"state"`
	Message        string     `json:"message"`
	MachineCode    string     `json:"machineCode"`
	LicenseID      string     `json:"licenseId,omitempty"`
	Customer       string     `json:"customer,omitempty"`
	IssuedAt       *time.Time `json:"issuedAt,omitempty"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	Unlimited      bool       `json:"unlimited"`
	RemainingDays  int64      `json:"remainingDays,omitempty"`
	AllowedModules []string   `json:"allowedModules"`
	BindingType    string     `json:"bindingType"`
}

var basicModules = []string{"资产管理", "系统管理"}

func BasicModules() []string {
	return append([]string(nil), basicModules...)
}
