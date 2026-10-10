package service

import (
	"strings"

	"dodevops-api/api/cmdb/model"
)

func decryptSnmpDeviceRemotePassword(device *model.CmdbSnmpDevice) (string, error) {
	if device == nil {
		return "", nil
	}
	return device.DecryptRemotePassword()
}

func normalizeSnmpRemotePasswordInput(value string) string {
	if model.IsSnmpMaskedSecretValue(strings.TrimSpace(value)) {
		return ""
	}
	password, err := model.DecryptSnmpSecretIfPossible(value)
	if err != nil {
		return value
	}
	return password
}
