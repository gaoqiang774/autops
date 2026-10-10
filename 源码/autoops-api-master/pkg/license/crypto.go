package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

var rawEncoding = base64.RawURLEncoding

func Sign(claims Claims, privateKey ed25519.PrivateKey) (string, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("%w: Ed25519 private key length is invalid", ErrInvalidLicense)
	}
	if err := validateClaimsShape(claims); err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal license claims: %w", err)
	}
	signature := ed25519.Sign(privateKey, payload)
	return rawEncoding.EncodeToString(payload) + "." + rawEncoding.EncodeToString(signature), nil
}

func Verify(token string, publicKey ed25519.PublicKey, machineCode string, now time.Time) (Claims, error) {
	claims, err := verifySignature(token, publicKey)
	if err != nil {
		return claims, err
	}
	if err := validateRuntimeClaims(claims, machineCode, now); err != nil {
		return claims, err
	}
	return claims, nil
}

func verifySignature(token string, publicKey ed25519.PublicKey) (Claims, error) {
	var claims Claims
	if len(publicKey) != ed25519.PublicKeySize {
		return claims, ErrPublicKey
	}

	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 {
		return claims, fmt.Errorf("%w: malformed key", ErrInvalidLicense)
	}
	payload, err := rawEncoding.DecodeString(parts[0])
	if err != nil {
		return claims, fmt.Errorf("%w: malformed payload", ErrInvalidLicense)
	}
	signature, err := rawEncoding.DecodeString(parts[1])
	if err != nil || len(signature) != ed25519.SignatureSize {
		return claims, fmt.Errorf("%w: malformed signature", ErrInvalidLicense)
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return claims, fmt.Errorf("%w: signature verification failed", ErrInvalidLicense)
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, fmt.Errorf("%w: malformed claims", ErrInvalidLicense)
	}
	if err := validateClaimsShape(claims); err != nil {
		return Claims{}, err
	}
	return claims, nil
}

func validateRuntimeClaims(claims Claims, machineCode string, now time.Time) error {
	nowUnix := now.Unix()
	if claims.NotBefore > 0 && nowUnix < claims.NotBefore {
		return ErrLicenseNotYet
	}
	if claims.ExpiresAt > 0 && nowUnix >= claims.ExpiresAt {
		return ErrLicenseExpired
	}
	if strings.TrimSpace(machineCode) == "" || !strings.EqualFold(claims.MachineCode, machineCode) {
		return ErrMachineMismatch
	}
	return nil
}

func validateClaimsShape(claims Claims) error {
	if claims.Version != FormatVersion || claims.Product != ProductName {
		return fmt.Errorf("%w: unsupported product or version", ErrInvalidLicense)
	}
	if strings.TrimSpace(claims.LicenseID) == "" || strings.TrimSpace(claims.MachineCode) == "" || claims.IssuedAt <= 0 {
		return fmt.Errorf("%w: required claims are missing", ErrInvalidLicense)
	}
	if claims.ExpiresAt > 0 && claims.ExpiresAt <= claims.IssuedAt {
		return fmt.Errorf("%w: expiration must be after issue time", ErrInvalidLicense)
	}
	if claims.BindingType != "" && claims.BindingType != BindingInstallationV1 && claims.BindingType != BindingLegacy {
		return fmt.Errorf("%w: unsupported machine binding type", ErrInvalidLicense)
	}
	return nil
}
