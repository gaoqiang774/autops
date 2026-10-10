package service

import (
	"testing"

	"dodevops-api/api/alertmanager/model"
)

func TestNormalizeSSLCertificateInput(t *testing.T) {
	input, err := normalizeSSLCertificateInput(model.ApplySSLCertificateInput{
		Domain:      " *.Example.COM. ",
		KeyManageID: 12,
		Email:       "ops@example.com",
		CAProvider:  "Let's Encrypt",
		Algorithm:   "RSA 2048",
	})
	if err != nil {
		t.Fatalf("normalize input: %v", err)
	}
	if input.Domain != "*.example.com" || input.CertType != model.SSLCertificateTypeACME || input.CAProvider != model.SSLCertificateCALetsEncrypt || input.Algorithm != model.SSLCertificateAlgorithmRSA2048 {
		t.Fatalf("unexpected normalized input: %#v", input)
	}
}

func TestNormalizeSSLCertificateInputRejectsInvalidValues(t *testing.T) {
	tests := []model.ApplySSLCertificateInput{
		{Domain: "foo.*.example.com", KeyManageID: 1, Email: "ops@example.com"},
		{Domain: "example.com", KeyManageID: 0, Email: "ops@example.com"},
		{Domain: "example.com", KeyManageID: 1, Email: "invalid"},
		{Domain: "example.com", KeyManageID: 1, Email: "ops@example.com", CAProvider: "ZeroSSL"},
	}
	for _, input := range tests {
		if _, err := normalizeSSLCertificateInput(input); err == nil {
			t.Fatalf("expected validation error for %#v", input)
		}
	}
}

func TestNormalizeSSLCertificateInputAllowsEmptyEmail(t *testing.T) {
	_, err := normalizeSSLCertificateInput(model.ApplySSLCertificateInput{
		Domain:      "example.com",
		KeyManageID: 1,
	})
	if err != nil {
		t.Fatalf("empty email should be optional: %v", err)
	}
}

func TestSplitAliyunDNSRecordSupportsMultiLabelPublicSuffix(t *testing.T) {
	domain, rr, err := splitAliyunDNSRecord("_acme-challenge.api.example.com.cn.", "example.com.cn.")
	if err != nil {
		t.Fatalf("split record: %v", err)
	}
	if domain != "example.com.cn" || rr != "_acme-challenge.api" {
		t.Fatalf("unexpected result: domain=%q rr=%q", domain, rr)
	}
}
