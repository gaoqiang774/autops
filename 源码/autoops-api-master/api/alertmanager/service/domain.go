package service

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"dodevops-api/api/alertmanager/dao"
	alertengine "dodevops-api/api/alertmanager/engine"
	"dodevops-api/api/alertmanager/model"
	"dodevops-api/common/util"
	"dodevops-api/pkg/log"
)

const (
	domainMonitorBatchLimit = 500
	domainMonitorWorkers    = 8
)

var (
	ErrInvalidDomainMonitor = errors.New("无效的域名监控配置")
	ErrDomainMonitorExists  = errors.New("域名监控已存在")
)

type DomainMonitorService struct {
	dao *dao.AlertManagerDao
}

type domainProbeResult struct {
	Status                   string
	StatusCode               int
	ResponseTimeMS           int64
	CertificateExpiresAt     time.Time
	CertificateRemainingDays int
	CertificateIssuer        string
	FailureReason            string
	FailureType              string
	Severity                 string
}

func GetDomainMonitorService() *DomainMonitorService {
	return &DomainMonitorService{dao: dao.NewAlertManagerDao()}
}

func (s *DomainMonitorService) Create(input model.AlertDomainInput, operator string) (*model.AlertDomain, error) {
	item, err := normalizeDomainInput(input)
	if err != nil {
		return nil, err
	}
	if s.dao.DomainExists(item.Domain, 0) {
		return nil, ErrDomainMonitorExists
	}
	if err := newAlertManagerService().validateRuleNotifyRuleIDs(item.NotifyRuleIDs); err != nil {
		return nil, err
	}
	datasource, err := s.dao.GetBuiltinAlertDatasource()
	if err != nil {
		return nil, fmt.Errorf("获取平台内置告警源失败: %w", err)
	}
	item.CreatedBy = operator
	item.UpdatedBy = operator
	item.Status = model.DomainStatusUnknown
	if item.Enabled == model.RuleStatusEnabled {
		item.NextCheckAt = &util.HTime{Time: time.Now()}
	}
	rule := domainAlertRule(item, datasource.ID, operator)
	if err := s.dao.CreateDomainWithRule(&item, &rule); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *DomainMonitorService) Update(id uint, input model.AlertDomainInput, operator string) (*model.AlertDomain, error) {
	existing, err := s.dao.GetDomain(id)
	if err != nil {
		return nil, err
	}
	item, err := normalizeDomainInput(input)
	if err != nil {
		return nil, err
	}
	if s.dao.DomainExists(item.Domain, id) {
		return nil, ErrDomainMonitorExists
	}
	if err := newAlertManagerService().validateRuleNotifyRuleIDs(item.NotifyRuleIDs); err != nil {
		return nil, err
	}
	item.ID = existing.ID
	item.AlertRuleID = existing.AlertRuleID
	item.Status = existing.Status
	item.StatusCode = existing.StatusCode
	item.ResponseTimeMS = existing.ResponseTimeMS
	item.CertificateExpiresAt = existing.CertificateExpiresAt
	item.CertificateRemainingDays = existing.CertificateRemainingDays
	item.CertificateIssuer = existing.CertificateIssuer
	item.FailureReason = existing.FailureReason
	item.LastCheckedAt = existing.LastCheckedAt
	item.NextCheckAt = existing.NextCheckAt
	item.CreatedBy = existing.CreatedBy
	item.CreatedAt = existing.CreatedAt
	item.UpdatedBy = operator
	if item.Enabled == model.RuleStatusEnabled && (existing.Enabled != model.RuleStatusEnabled || item.NextCheckAt == nil || item.NextCheckAt.IsZero()) {
		item.NextCheckAt = &util.HTime{Time: time.Now()}
	}
	if item.Enabled != model.RuleStatusEnabled {
		item.NextCheckAt = nil
	}
	rule := domainAlertRule(item, 0, operator)
	if err := s.dao.UpdateDomainWithRule(&item, &rule); err != nil {
		return nil, err
	}
	if existing.Enabled == model.RuleStatusEnabled && item.Enabled != model.RuleStatusEnabled {
		s.resolveDomainEvent(context.Background(), item)
	}
	return s.dao.GetDomain(id)
}

func (s *DomainMonitorService) Delete(ctx context.Context, id uint) error {
	item, err := s.dao.GetDomain(id)
	if err != nil {
		return err
	}
	s.resolveDomainEvent(ctx, *item)
	return s.dao.DeleteDomainWithRule(item.ID, item.AlertRuleID)
}

func (s *DomainMonitorService) Get(id uint) (*model.AlertDomain, error) {
	return s.dao.GetDomain(id)
}

func (s *DomainMonitorService) List(query model.AlertDomainQuery) ([]model.AlertDomain, int64, model.AlertDomainSummary, error) {
	list, total, err := s.dao.ListDomains(query)
	if err != nil {
		return nil, 0, model.AlertDomainSummary{}, err
	}
	summary, err := s.dao.DomainSummary(30)
	return list, total, summary, err
}

func (s *DomainMonitorService) ChangeEnabled(ctx context.Context, id uint, enabled int, operator string) (*model.AlertDomain, error) {
	if enabled != model.RuleStatusEnabled {
		enabled = model.RuleStatusDisabled
	}
	item, err := s.dao.GetDomain(id)
	if err != nil {
		return nil, err
	}
	var next *util.HTime
	if enabled == model.RuleStatusEnabled {
		next = &util.HTime{Time: time.Now()}
	}
	if err := s.dao.UpdateDomainEnabled(id, enabled, next, operator); err != nil {
		return nil, err
	}
	item.Enabled = enabled
	if enabled != model.RuleStatusEnabled {
		s.resolveDomainEvent(ctx, *item)
	}
	return s.dao.GetDomain(id)
}

func (s *DomainMonitorService) Check(ctx context.Context, id uint) (*model.AlertDomain, error) {
	item, err := s.dao.GetDomain(id)
	if err != nil {
		return nil, err
	}
	checked := s.checkOne(ctx, *item)
	return &checked, nil
}

func (s *DomainMonitorService) CheckBatch(ctx context.Context, ids []uint) ([]model.AlertDomain, error) {
	items, err := s.dao.ListDomainsByIDs(normalizeDomainIDs(ids), len(ids) == 0)
	if err != nil {
		return nil, err
	}
	if len(items) > domainMonitorBatchLimit {
		items = items[:domainMonitorBatchLimit]
	}
	return s.checkMany(ctx, items), nil
}

func StartDomainMonitor(ctx context.Context) {
	service := GetDomainMonitorService()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !alertengine.IsLeader() {
				continue
			}
			items, err := service.dao.ListDueDomains(time.Now(), domainMonitorBatchLimit)
			if err != nil {
				log.Log().Warnf("domain monitor list due domains failed: %v", err)
				continue
			}
			service.checkMany(ctx, items)
		}
	}
}

func (s *DomainMonitorService) checkMany(ctx context.Context, items []model.AlertDomain) []model.AlertDomain {
	if len(items) == 0 {
		return []model.AlertDomain{}
	}
	results := make([]model.AlertDomain, len(items))
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := domainMonitorWorkers
	if len(items) < workers {
		workers = len(items)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				results[index] = s.checkOne(ctx, items[index])
			}
		}()
	}
	for index := range items {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return results
		case jobs <- index:
		}
	}
	close(jobs)
	wg.Wait()
	return results
}

func (s *DomainMonitorService) checkOne(parent context.Context, item model.AlertDomain) model.AlertDomain {
	probe := probeDomain(parent, item)
	now := time.Now()
	item.Status = probe.Status
	item.StatusCode = probe.StatusCode
	item.ResponseTimeMS = probe.ResponseTimeMS
	item.CertificateRemainingDays = probe.CertificateRemainingDays
	item.CertificateIssuer = probe.CertificateIssuer
	item.FailureReason = probe.FailureReason
	item.CertificateExpiresAt = nil
	if !probe.CertificateExpiresAt.IsZero() {
		item.CertificateExpiresAt = &util.HTime{Time: probe.CertificateExpiresAt}
	}
	item.LastCheckedAt = &util.HTime{Time: now}
	item.NextCheckAt = &util.HTime{Time: now.Add(time.Duration(item.CheckIntervalSeconds) * time.Second)}
	item.UpdatedAt = util.HTime{Time: now}
	if err := s.dao.UpdateDomainCheckResult(item); err != nil {
		log.Log().Warnf("domain monitor save result failed: domain=%s err=%v", item.Domain, err)
		return item
	}
	if err := s.publishDomainEvent(parent, item, probe); err != nil {
		log.Log().Warnf("domain monitor publish alert failed: domain=%s err=%v", item.Domain, err)
	}
	return item
}

func (s *DomainMonitorService) publishDomainEvent(ctx context.Context, item model.AlertDomain, probe domainProbeResult) error {
	rule, err := s.dao.GetRule(item.AlertRuleID)
	if err != nil {
		return err
	}
	rule.Name = "域名监控 - " + item.Domain
	rule.NotifyRuleIDs = item.NotifyRuleIDs
	labels := model.JSONMap{
		"resourceType": "domain",
		"monitorType":  "domain",
		"domain":       item.Domain,
		"protocol":     item.Protocol,
		"failureType":  probe.FailureType,
	}
	annotations := model.JSONMap{
		"summary":     domainAlertSummary(item, probe),
		"description": domainAlertDescription(item, probe),
	}
	_, err = alertengine.PublishExternalEvent(ctx, s.dao.DB(), alertengine.ExternalEventInput{
		Rule:             *rule,
		Fingerprint:      domainFingerprint(item.ID),
		Labels:           labels,
		Annotations:      annotations,
		Value:            strconv.FormatInt(item.ResponseTimeMS, 10),
		Severity:         probe.Severity,
		TriggerCondition: domainTriggerCondition(item, probe),
		EventDetail:      domainAlertDescription(item, probe),
		Firing:           item.Status == model.DomainStatusAbnormal,
		Now:              nowFromHTime(item.LastCheckedAt),
	})
	return err
}

func (s *DomainMonitorService) resolveDomainEvent(ctx context.Context, item model.AlertDomain) {
	if item.AlertRuleID == 0 {
		return
	}
	rule, err := s.dao.GetRule(item.AlertRuleID)
	if err != nil {
		return
	}
	_, err = alertengine.PublishExternalEvent(ctx, s.dao.DB(), alertengine.ExternalEventInput{
		Rule:        *rule,
		Fingerprint: domainFingerprint(item.ID),
		Firing:      false,
		Now:         time.Now(),
	})
	if err != nil {
		log.Log().Warnf("domain monitor resolve alert failed: domain=%s err=%v", item.Domain, err)
	}
}

func probeDomain(parent context.Context, item model.AlertDomain) domainProbeResult {
	result := domainProbeResult{Status: model.DomainStatusNormal, Severity: "warning"}
	timeout := time.Duration(item.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, domainTargetURL(item), nil)
	if err != nil {
		return failedDomainProbe("request", "critical", err.Error())
	}
	req.Header.Set("User-Agent", "AutoOps-Domain-Monitor/1.0")
	client := &http.Client{Timeout: timeout}
	started := time.Now()
	resp, err := client.Do(req)
	result.ResponseTimeMS = time.Since(started).Milliseconds()
	if err != nil {
		if certificate, ok := certificateFromValidityError(ctx, item, err); ok {
			applyDomainCertificate(&result, item, certificate, time.Now())
			return result
		}
		failed := failedDomainProbe("availability", "critical", err.Error())
		failed.ResponseTimeMS = result.ResponseTimeMS
		return failed
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode
	if item.ExpectedStatus > 0 && resp.StatusCode != item.ExpectedStatus {
		result.Status = model.DomainStatusAbnormal
		result.FailureType = "http_status"
		result.FailureReason = fmt.Sprintf("HTTP 状态码为 %d，期望 %d", resp.StatusCode, item.ExpectedStatus)
	}
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		applyDomainCertificate(&result, item, resp.TLS.PeerCertificates[0], time.Now())
	}
	return result
}

func certificateFromValidityError(ctx context.Context, item model.AlertDomain, err error) (*x509.Certificate, bool) {
	var verificationError *tls.CertificateVerificationError
	if errors.As(err, &verificationError) && len(verificationError.UnverifiedCertificates) > 0 {
		certificate := verificationError.UnverifiedCertificates[0]
		if certificateOutsideValidity(certificate, time.Now()) {
			return certificate, true
		}
	}
	if !strings.Contains(strings.ToLower(err.Error()), "certificate has expired or is not yet valid") {
		return nil, false
	}
	certificate, inspectErr := inspectUnverifiedCertificate(ctx, item)
	if inspectErr != nil || !certificateOutsideValidity(certificate, time.Now()) {
		return nil, false
	}
	return certificate, true
}

func certificateOutsideValidity(certificate *x509.Certificate, now time.Time) bool {
	return certificate != nil && (now.Before(certificate.NotBefore) || now.After(certificate.NotAfter))
}

func inspectUnverifiedCertificate(ctx context.Context, item model.AlertDomain) (*x509.Certificate, error) {
	port := item.Port
	if port == 0 {
		port = 443
	}
	dialer := &tls.Dialer{
		Config: &tls.Config{
			ServerName:         item.Domain,
			InsecureSkipVerify: true, // Used only after the strict request reports an explicit certificate validity error.
		},
	}
	connection, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(item.Domain, strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	defer connection.Close()
	tlsConnection, ok := connection.(*tls.Conn)
	if !ok {
		return nil, errors.New("TLS 连接类型无效")
	}
	state := tlsConnection.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil, errors.New("服务端未返回证书")
	}
	return state.PeerCertificates[0], nil
}

func applyDomainCertificate(result *domainProbeResult, item model.AlertDomain, certificate *x509.Certificate, now time.Time) {
	if result == nil || certificate == nil {
		return
	}
	result.CertificateExpiresAt = certificate.NotAfter
	remaining := certificate.NotAfter.Sub(now)
	if remaining < 0 {
		result.CertificateRemainingDays = -int(math.Ceil(-remaining.Hours() / 24))
	} else {
		result.CertificateRemainingDays = int(remaining.Hours() / 24)
	}
	result.CertificateIssuer = strings.TrimSpace(certificate.Issuer.CommonName)
	if result.CertificateIssuer == "" {
		result.CertificateIssuer = certificate.Issuer.String()
	}

	if now.Before(certificate.NotBefore) {
		result.Status = model.DomainStatusAbnormal
		result.FailureType = "ssl_validity"
		result.Severity = "critical"
		result.FailureReason = fmt.Sprintf("SSL 证书尚未生效，生效时间为 %s", certificate.NotBefore.Local().Format("2006/01/02 15:04"))
		return
	}
	if now.After(certificate.NotAfter) {
		result.Status = model.DomainStatusAbnormal
		result.FailureType = "ssl_expiry"
		result.Severity = "critical"
		result.FailureReason = fmt.Sprintf("SSL 证书已过期 %d 天，到期时间为 %s", -result.CertificateRemainingDays, certificate.NotAfter.Local().Format("2006/01/02 15:04"))
		return
	}
	if result.CertificateRemainingDays <= item.SSLExpireWarnDays {
		result.Status = model.DomainStatusAbnormal
		result.FailureType = "ssl_expiry"
		result.Severity = "warning"
		result.FailureReason = fmt.Sprintf("SSL 证书将在 %d 天内到期", result.CertificateRemainingDays)
	}
}

func failedDomainProbe(failureType, severity, reason string) domainProbeResult {
	return domainProbeResult{
		Status:        model.DomainStatusAbnormal,
		FailureType:   failureType,
		Severity:      severity,
		FailureReason: strings.TrimSpace(reason),
	}
}

func nowFromHTime(value *util.HTime) time.Time {
	if value == nil || value.IsZero() {
		return time.Now()
	}
	return value.Time
}

func normalizeDomainInput(input model.AlertDomainInput) (model.AlertDomain, error) {
	protocol := strings.ToLower(strings.TrimSpace(input.Protocol))
	domain := strings.TrimSpace(input.Domain)
	path := strings.TrimSpace(input.Path)
	port := input.Port
	if strings.Contains(domain, "://") {
		parsed, err := url.Parse(domain)
		if err != nil || parsed.Hostname() == "" {
			return model.AlertDomain{}, fmt.Errorf("%w: 域名格式不正确", ErrInvalidDomainMonitor)
		}
		domain = parsed.Hostname()
		protocol = strings.ToLower(parsed.Scheme)
		if port == 0 && parsed.Port() != "" {
			port, _ = strconv.Atoi(parsed.Port())
		}
		if (path == "" || path == "/") && parsed.RequestURI() != "" {
			path = parsed.RequestURI()
		}
	}
	domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if protocol == "" {
		protocol = "https"
	}
	if protocol != "http" && protocol != "https" {
		return model.AlertDomain{}, fmt.Errorf("%w: 协议仅支持 http 或 https", ErrInvalidDomainMonitor)
	}
	if net.ParseIP(domain) == nil && !validDomainName(domain) {
		return model.AlertDomain{}, fmt.Errorf("%w: 域名格式不正确", ErrInvalidDomainMonitor)
	}
	if port < 0 || port > 65535 {
		return model.AlertDomain{}, fmt.Errorf("%w: 端口必须为 0 或 1-65535", ErrInvalidDomainMonitor)
	}
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	interval := input.CheckIntervalSeconds
	if interval == 0 {
		interval = 300
	}
	if interval < 30 || interval > 86400 {
		return model.AlertDomain{}, fmt.Errorf("%w: 检查间隔必须为 30-86400 秒", ErrInvalidDomainMonitor)
	}
	timeout := input.TimeoutSeconds
	if timeout == 0 {
		timeout = 10
	}
	if timeout < 1 || timeout > 60 {
		return model.AlertDomain{}, fmt.Errorf("%w: 超时时间必须为 1-60 秒", ErrInvalidDomainMonitor)
	}
	warnDays := input.SSLExpireWarnDays
	if warnDays == 0 {
		warnDays = 30
	}
	if warnDays < 1 || warnDays > 365 {
		return model.AlertDomain{}, fmt.Errorf("%w: 证书预警天数必须为 1-365", ErrInvalidDomainMonitor)
	}
	if input.ExpectedStatus != 0 && (input.ExpectedStatus < 100 || input.ExpectedStatus > 599) {
		return model.AlertDomain{}, fmt.Errorf("%w: 期望状态码必须为 100-599", ErrInvalidDomainMonitor)
	}
	enabled := model.RuleStatusEnabled
	if input.Enabled != nil && *input.Enabled != model.RuleStatusEnabled {
		enabled = model.RuleStatusDisabled
	}
	tags := normalizeDomainTags(input.Tags)
	return model.AlertDomain{
		Domain: domain, Protocol: protocol, Port: port, Path: path,
		Tags: model.JSONStringArray(tags), ExpectedStatus: input.ExpectedStatus,
		CheckIntervalSeconds: interval, TimeoutSeconds: timeout, SSLExpireWarnDays: warnDays,
		Enabled: enabled, Remarks: strings.TrimSpace(input.Remarks),
		NotifyRuleIDs: model.JSONUintArray(normalizeUintIDs([]uint(input.NotifyRuleIDs))),
	}, nil
}

func validDomainName(domain string) bool {
	if domain == "" || len(domain) > 253 {
		return false
	}
	for _, label := range strings.Split(domain, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}

func normalizeDomainTags(tags []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(tags))
	for _, raw := range tags {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalizeDomainIDs(ids []uint) []uint {
	seen := make(map[uint]struct{})
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func domainAlertRule(item model.AlertDomain, datasourceID uint, operator string) model.AlertRule {
	return model.AlertRule{
		ID: item.AlertRuleID, Name: "域名监控 - " + item.Domain,
		Note:         "域名可用性、HTTP 状态和 SSL 证书到期监控",
		DatasourceID: datasourceID, DatasourceType: model.AlertDatasourceBuiltin,
		Query:       "domain_probe{domain=\"" + item.Domain + "\"}",
		CronPattern: "@every 24h", EvalIntervalSeconds: 86400,
		Severity: "warning", Enabled: model.RuleStatusDisabled, Internal: 1,
		TimeZone: "Asia/Shanghai", NoDataPolicy: "ignore", ErrorPolicy: "ignore",
		Labels:        model.JSONMap{"resourceType": "domain", "monitorType": "domain", "domain": item.Domain},
		Annotations:   model.JSONMap{"summary": "域名监控异常", "description": "域名探测发现异常"},
		NotifyRuleIDs: item.NotifyRuleIDs, NotifyRecovered: 1,
		NotifyRepeatStepMinutes: 60, NotifyMaxNumber: 0,
		CreatedBy: operator, UpdatedBy: operator,
	}
}

func domainTargetURL(item model.AlertDomain) string {
	host := item.Domain
	defaultPort := (item.Protocol == "https" && item.Port == 443) || (item.Protocol == "http" && item.Port == 80)
	if item.Port > 0 && !defaultPort {
		host = net.JoinHostPort(host, strconv.Itoa(item.Port))
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return item.Protocol + "://" + host + item.Path
}

func domainFingerprint(id uint) string {
	digest := sha1.Sum([]byte(fmt.Sprintf("domain-monitor:%d", id)))
	return hex.EncodeToString(digest[:])
}

func domainAlertSummary(item model.AlertDomain, probe domainProbeResult) string {
	if item.Status == model.DomainStatusNormal {
		return "域名监控已恢复: " + item.Domain
	}
	return "域名监控异常: " + item.Domain
}

func domainAlertDescription(item model.AlertDomain, probe domainProbeResult) string {
	if probe.FailureReason != "" {
		return probe.FailureReason
	}
	return fmt.Sprintf("%s 响应正常，状态码 %d，耗时 %dms", item.Domain, item.StatusCode, item.ResponseTimeMS)
}

func domainTriggerCondition(item model.AlertDomain, probe domainProbeResult) string {
	if probe.FailureType != "" {
		return probe.FailureType + ": " + probe.FailureReason
	}
	return fmt.Sprintf("domain=%s status=%d", item.Domain, item.StatusCode)
}
