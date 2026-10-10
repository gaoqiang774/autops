package service

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"sync"
	"time"

	"dodevops-api/api/alertmanager/model"

	alidns "github.com/alibabacloud-go/alidns-20150109/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	teautils "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

type acmeCertificateResult struct {
	Certificate       string
	PrivateKey        string
	IssuerCertificate string
	CertificateURL    string
	IssuedAt          time.Time
	ExpiresAt         time.Time
}

type acmeUser struct {
	email        string
	registration *registration.Resource
	privateKey   crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string                        { return u.email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.registration }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.privateKey }

type aliyunDNSProvider struct {
	client    *alidns.Client
	mu        sync.Mutex
	recordIDs map[string]string
}

func newAliyunDNSProvider(accessKeyID, accessKeySecret string) (*aliyunDNSProvider, error) {
	config := &openapi.Config{
		AccessKeyId:     tea.String(strings.TrimSpace(accessKeyID)),
		AccessKeySecret: tea.String(strings.TrimSpace(accessKeySecret)),
		Endpoint:        tea.String("alidns.cn-hangzhou.aliyuncs.com"),
	}
	client, err := alidns.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("创建阿里云 DNS 客户端失败: %w", err)
	}
	return &aliyunDNSProvider{client: client, recordIDs: make(map[string]string)}, nil
}

func (p *aliyunDNSProvider) Present(domain, _ string, keyAuth string) error {
	fqdn, value := dns01.GetRecord(domain, keyAuth)
	zone, err := dns01.FindZoneByFqdn(fqdn)
	if err != nil {
		return fmt.Errorf("查找域名 %s 的 DNS 区域失败: %w", fqdn, err)
	}
	domainName, rr, err := splitAliyunDNSRecord(fqdn, zone)
	if err != nil {
		return err
	}
	response, err := p.client.AddDomainRecordWithOptions(&alidns.AddDomainRecordRequest{
		DomainName: tea.String(domainName),
		RR:         tea.String(rr),
		Type:       tea.String("TXT"),
		Value:      tea.String(value),
		TTL:        tea.Int64(600),
	}, &teautils.RuntimeOptions{})
	if err != nil {
		return fmt.Errorf("添加 ACME DNS TXT 记录失败: %w", err)
	}
	if response == nil || response.Body == nil || tea.StringValue(response.Body.RecordId) == "" {
		return fmt.Errorf("添加 ACME DNS TXT 记录失败: 阿里云未返回记录 ID")
	}
	p.mu.Lock()
	p.recordIDs[dnsRecordKey(fqdn, value)] = tea.StringValue(response.Body.RecordId)
	p.mu.Unlock()
	return nil
}

func (p *aliyunDNSProvider) CleanUp(domain, _ string, keyAuth string) error {
	fqdn, value := dns01.GetRecord(domain, keyAuth)
	key := dnsRecordKey(fqdn, value)
	p.mu.Lock()
	recordID := p.recordIDs[key]
	delete(p.recordIDs, key)
	p.mu.Unlock()
	if recordID == "" {
		return nil
	}
	_, err := p.client.DeleteDomainRecordWithOptions(&alidns.DeleteDomainRecordRequest{
		RecordId: tea.String(recordID),
	}, &teautils.RuntimeOptions{})
	if err != nil {
		return fmt.Errorf("清理 ACME DNS TXT 记录失败: %w", err)
	}
	return nil
}

func (p *aliyunDNSProvider) Timeout() (time.Duration, time.Duration) {
	return 10 * time.Minute, 5 * time.Second
}

func dnsRecordKey(fqdn, value string) string {
	return strings.ToLower(strings.TrimSuffix(fqdn, ".")) + "\x00" + value
}

func splitAliyunDNSRecord(fqdn, zone string) (string, string, error) {
	name := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(fqdn), "."))
	domainName := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(zone), "."))
	if name == "" || domainName == "" || name == domainName || !strings.HasSuffix(name, "."+domainName) {
		return "", "", fmt.Errorf("无法把 DNS 记录 %q 解析到区域 %q", fqdn, zone)
	}
	rr := strings.TrimSuffix(name, "."+domainName)
	return domainName, rr, nil
}

func issueACMECertificate(input model.ApplySSLCertificateInput, accessKeyID, accessKeySecret string) (*acmeCertificateResult, error) {
	accountKey, certKeyType, err := generateACMEAccountKey(input.Algorithm)
	if err != nil {
		return nil, err
	}
	user := &acmeUser{email: input.Email, privateKey: accountKey}
	config := lego.NewConfig(user)
	config.CADirURL = lego.LEDirectoryProduction
	config.Certificate.KeyType = certKeyType
	client, err := lego.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("创建 ACME 客户端失败: %w", err)
	}
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return nil, fmt.Errorf("注册 ACME 账户失败: %w", err)
	}
	user.registration = reg

	provider, err := newAliyunDNSProvider(accessKeyID, accessKeySecret)
	if err != nil {
		return nil, err
	}
	if err := client.Challenge.SetDNS01Provider(provider); err != nil {
		return nil, fmt.Errorf("设置 ACME DNS 认证失败: %w", err)
	}
	resource, err := client.Certificate.Obtain(certificate.ObtainRequest{
		Domains: []string{input.Domain},
		Bundle:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("申请 ACME 证书失败: %w", err)
	}
	block, _ := pem.Decode(resource.Certificate)
	if block == nil {
		return nil, fmt.Errorf("解析签发证书失败: 未找到 PEM 证书")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析签发证书失败: %w", err)
	}
	return &acmeCertificateResult{
		Certificate:       string(resource.Certificate),
		PrivateKey:        string(resource.PrivateKey),
		IssuerCertificate: string(resource.IssuerCertificate),
		CertificateURL:    resource.CertURL,
		IssuedAt:          cert.NotBefore,
		ExpiresAt:         cert.NotAfter,
	}, nil
}

func generateACMEAccountKey(algorithm string) (crypto.PrivateKey, certcrypto.KeyType, error) {
	switch algorithm {
	case model.SSLCertificateAlgorithmRSA2048:
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		return key, certcrypto.RSA2048, err
	case model.SSLCertificateAlgorithmRSA4096:
		key, err := rsa.GenerateKey(rand.Reader, 4096)
		return key, certcrypto.RSA4096, err
	case model.SSLCertificateAlgorithmEC256:
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		return key, certcrypto.EC256, err
	case model.SSLCertificateAlgorithmEC384:
		key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		return key, certcrypto.EC384, err
	default:
		return nil, certcrypto.RSA2048, fmt.Errorf("不支持的加密算法: %s", algorithm)
	}
}
