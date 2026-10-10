package service

import (
	"archive/zip"
	"bytes"
	"encoding/pem"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"dodevops-api/api/alertmanager/dao"
	"dodevops-api/api/alertmanager/model"
	configservice "dodevops-api/api/configcenter/service"
	"dodevops-api/pkg/log"

	"gorm.io/gorm"
)

var domainLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

var (
	ErrSSLCertificateNotFound  = errors.New("证书记录不存在")
	ErrSSLCertificateApplying  = errors.New("证书正在申请中，暂不能删除")
	ErrSSLCertificateNotIssued = errors.New("证书尚未签发，暂不能下载")
	ErrSSLCertificateDeploying = errors.New("证书正在更新到主机，请等待部署完成")
)

type SSLCertificateDownload struct {
	FileName string
	Content  []byte
}

type SSLCertificateService struct {
	dao *dao.AlertManagerDao
}

func GetSSLCertificateService() *SSLCertificateService {
	return &SSLCertificateService{dao: dao.NewAlertManagerDao()}
}

func (s *SSLCertificateService) List(query model.SSLCertificateQuery) ([]model.SSLCertificate, int64, error) {
	return s.dao.ListSSLCertificates(query)
}

func (s *SSLCertificateService) Get(id uint) (*model.SSLCertificateDetail, error) {
	item, err := s.getRecord(id)
	if err != nil {
		return nil, err
	}
	return item.ToDetail(), nil
}

func (s *SSLCertificateService) Download(id uint) (*SSLCertificateDownload, error) {
	item, err := s.getRecord(id)
	if err != nil {
		return nil, err
	}
	if item.Status != model.SSLCertificateStatusIssued || strings.TrimSpace(item.Certificate) == "" || strings.TrimSpace(item.PrivateKey) == "" {
		return nil, ErrSSLCertificateNotIssued
	}

	baseName := certificateFileBase(item.Domain)
	certificate := strings.TrimSpace(item.Certificate) + "\n"
	issuerCertificate := strings.TrimSpace(item.IssuerCertificate)
	fullChain := certificate
	if issuerCertificate != "" && !containsCertificate(certificate, issuerCertificate) {
		fullChain += issuerCertificate + "\n"
	}

	files := []struct {
		name    string
		content string
	}{
		{name: baseName + ".crt", content: leafCertificatePEM(certificate)},
		{name: baseName + ".fullchain.pem", content: fullChain},
		{name: baseName + ".key", content: strings.TrimSpace(item.PrivateKey) + "\n"},
	}
	if issuerCertificate != "" {
		files = append(files, struct {
			name    string
			content string
		}{name: baseName + ".issuer.crt", content: issuerCertificate + "\n"})
	}

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, file := range files {
		entry, createErr := writer.Create(file.name)
		if createErr != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("创建证书压缩包失败: %w", createErr)
		}
		if _, writeErr := entry.Write([]byte(file.content)); writeErr != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("写入证书压缩包失败: %w", writeErr)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("生成证书压缩包失败: %w", err)
	}

	return &SSLCertificateDownload{
		FileName: baseName + "-ssl-certificate.zip",
		Content:  buffer.Bytes(),
	}, nil
}

func (s *SSLCertificateService) Delete(id uint) error {
	item, err := s.getRecord(id)
	if err != nil {
		return err
	}
	if item.Status == model.SSLCertificateStatusApplying {
		return ErrSSLCertificateApplying
	}
	deploying, err := s.dao.HasRunningSSLCertificateDeployment(id, 0)
	if err != nil {
		return fmt.Errorf("检查证书部署任务失败: %w", err)
	}
	if deploying {
		return ErrSSLCertificateDeploying
	}
	deleted, err := s.dao.DeleteSSLCertificate(id)
	if err != nil {
		return fmt.Errorf("删除证书记录失败: %w", err)
	}
	if !deleted {
		return ErrSSLCertificateApplying
	}
	return nil
}

func (s *SSLCertificateService) getRecord(id uint) (*model.SSLCertificate, error) {
	if id == 0 {
		return nil, ErrSSLCertificateNotFound
	}
	item, err := s.dao.GetSSLCertificate(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSSLCertificateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询证书记录失败: %w", err)
	}
	return item, nil
}

func certificateFileBase(domain string) string {
	baseName := strings.TrimSpace(strings.ToLower(domain))
	if strings.HasPrefix(baseName, "*.") {
		baseName = "wildcard-" + strings.TrimPrefix(baseName, "*.")
	}
	baseName = strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_").Replace(baseName)
	if baseName == "" {
		return "ssl-certificate"
	}
	return baseName
}

func leafCertificatePEM(certificate string) string {
	block, _ := pem.Decode([]byte(certificate))
	if block == nil || block.Type != "CERTIFICATE" {
		return certificate
	}
	return string(pem.EncodeToMemory(block))
}

func containsCertificate(certificates, candidate string) bool {
	candidateBlock, _ := pem.Decode([]byte(candidate))
	if candidateBlock == nil {
		return false
	}
	remaining := []byte(certificates)
	for len(remaining) > 0 {
		block, rest := pem.Decode(remaining)
		if block == nil {
			return false
		}
		if block.Type == "CERTIFICATE" && bytes.Equal(block.Bytes, candidateBlock.Bytes) {
			return true
		}
		remaining = rest
	}
	return false
}

func (s *SSLCertificateService) Apply(input model.ApplySSLCertificateInput, operator string) (*model.SSLCertificate, error) {
	normalized, err := normalizeSSLCertificateInput(input)
	if err != nil {
		return nil, err
	}
	pending, err := s.dao.HasPendingSSLCertificate(normalized.Domain)
	if err != nil {
		return nil, fmt.Errorf("检查证书申请记录失败: %w", err)
	}
	if pending {
		return nil, errors.New("该域名已有正在处理的证书申请")
	}

	accessKeyID, accessKeySecret, keyType, err := configservice.NewKeyManageService().GetDecryptedKeyForCloudAPI(normalized.KeyManageID)
	if err != nil {
		return nil, fmt.Errorf("读取配置中心密钥失败: %w", err)
	}
	if keyType != 1 {
		return nil, errors.New("DNS 认证仅支持配置中心的阿里云密钥")
	}
	if strings.TrimSpace(accessKeyID) == "" || strings.TrimSpace(accessKeySecret) == "" {
		return nil, errors.New("配置中心的阿里云密钥不完整")
	}

	item := &model.SSLCertificate{
		Domain:      normalized.Domain,
		KeyManageID: normalized.KeyManageID,
		CAProvider:  normalized.CAProvider,
		Algorithm:   normalized.Algorithm,
		Email:       normalized.Email,
		Status:      model.SSLCertificateStatusApplying,
		CreatedBy:   operator,
	}
	if err := s.dao.CreateSSLCertificate(item); err != nil {
		return nil, fmt.Errorf("保存证书申请记录失败: %w", err)
	}

	go s.issue(item.ID, normalized, accessKeyID, accessKeySecret)
	return item, nil
}

func (s *SSLCertificateService) issue(id uint, input model.ApplySSLCertificateInput, accessKeyID, accessKeySecret string) {
	issued, err := issueACMECertificate(input, accessKeyID, accessKeySecret)
	if err != nil {
		message := err.Error()
		if updateErr := s.dao.MarkSSLCertificateFailed(id, message); updateErr != nil {
			log.Log().Errorf("update failed SSL certificate order: id=%d err=%v", id, updateErr)
		}
		log.Log().Warnf("SSL certificate application failed: id=%d domain=%s err=%v", id, input.Domain, err)
		return
	}
	if err := s.dao.MarkSSLCertificateIssued(id, issued.Certificate, issued.PrivateKey, issued.IssuerCertificate, issued.CertificateURL, issued.IssuedAt, issued.ExpiresAt); err != nil {
		log.Log().Errorf("save issued SSL certificate: id=%d domain=%s err=%v", id, input.Domain, err)
		return
	}
	log.Log().Infof("SSL certificate issued: id=%d domain=%s expiresAt=%s", id, input.Domain, issued.ExpiresAt.Format("2006-01-02"))
}

func normalizeSSLCertificateInput(input model.ApplySSLCertificateInput) (model.ApplySSLCertificateInput, error) {
	input.Domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(input.Domain), "."))
	if err := validateCertificateDomain(input.Domain); err != nil {
		return input, err
	}
	if input.KeyManageID == 0 {
		input.KeyManageID = input.AliyunConfigID
	}
	if input.KeyManageID == 0 {
		return input, errors.New("keyManageId 不能为空")
	}
	if input.CertType == 0 {
		input.CertType = model.SSLCertificateTypeACME
	}
	if input.CertType != model.SSLCertificateTypeACME {
		return input, errors.New("当前接口仅支持 ACME 免费证书")
	}
	input.CAProvider = strings.TrimSpace(input.CAProvider)
	if input.CAProvider == "" || strings.EqualFold(input.CAProvider, "Let's Encrypt") || strings.EqualFold(input.CAProvider, "letsencrypt") {
		input.CAProvider = model.SSLCertificateCALetsEncrypt
	}
	if input.CAProvider != model.SSLCertificateCALetsEncrypt {
		return input, errors.New("当前接口仅支持 Let's Encrypt")
	}
	input.Algorithm = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(input.Algorithm), " ", ""))
	if input.Algorithm == "" {
		input.Algorithm = model.SSLCertificateAlgorithmRSA2048
	}
	switch input.Algorithm {
	case model.SSLCertificateAlgorithmRSA2048, model.SSLCertificateAlgorithmRSA4096,
		model.SSLCertificateAlgorithmEC256, model.SSLCertificateAlgorithmEC384:
	default:
		return input, fmt.Errorf("不支持的加密算法: %s", input.Algorithm)
	}
	input.Email = strings.TrimSpace(input.Email)
	if input.Email != "" {
		address, err := mail.ParseAddress(input.Email)
		if err != nil || address.Address != input.Email {
			return input, errors.New("email 格式无效")
		}
	}
	return input, nil
}

func validateCertificateDomain(domain string) error {
	plain := strings.TrimPrefix(domain, "*.")
	if plain == domain && strings.Contains(domain, "*") {
		return errors.New("通配符只能出现在域名最左侧")
	}
	if plain == "" || len(plain) > 253 || !strings.Contains(plain, ".") {
		return errors.New("domain 格式无效")
	}
	for _, label := range strings.Split(plain, ".") {
		if !domainLabelPattern.MatchString(label) {
			return errors.New("domain 格式无效")
		}
	}
	return nil
}
