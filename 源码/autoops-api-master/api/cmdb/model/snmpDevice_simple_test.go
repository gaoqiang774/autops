package model

import "testing"

func TestSnmpRemotePasswordEncryption(t *testing.T) {
	device := &CmdbSnmpDevice{RemotePassword: "123456"}

	if err := device.EncryptRemotePassword(); err != nil {
		t.Fatalf("EncryptRemotePassword failed: %v", err)
	}
	if device.RemotePassword == "123456" {
		t.Fatal("RemotePassword was not encrypted")
	}

	encrypted := device.RemotePassword
	if err := device.EncryptRemotePassword(); err != nil {
		t.Fatalf("EncryptRemotePassword should be idempotent: %v", err)
	}
	if device.RemotePassword != encrypted {
		t.Fatal("EncryptRemotePassword double-encrypted an encrypted password")
	}

	decrypted, err := device.DecryptRemotePassword()
	if err != nil {
		t.Fatalf("DecryptRemotePassword failed: %v", err)
	}
	if decrypted != "123456" {
		t.Fatalf("unexpected decrypted password: %q", decrypted)
	}
}

func TestSnmpDeviceVOMasksRemotePassword(t *testing.T) {
	device := &CmdbSnmpDevice{RemotePassword: "123456"}

	vo := device.ToVO()
	if vo.SshPasswd != snmpMaskedSecretValue {
		t.Fatalf("SshPasswd should be masked, got %q", vo.SshPasswd)
	}
	if vo.SshPassword != snmpMaskedSecretValue {
		t.Fatalf("SshPassword should be masked, got %q", vo.SshPassword)
	}
	if vo.RdpPassword != snmpMaskedSecretValue {
		t.Fatalf("RdpPassword should be masked, got %q", vo.RdpPassword)
	}
	if !vo.HasRemotePassword {
		t.Fatal("HasRemotePassword should be true")
	}
}
