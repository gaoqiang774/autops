package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"dodevops-api/api/alertmanager/model"
	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	cmdbservice "dodevops-api/api/cmdb/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var deployPathPattern = regexp.MustCompile(`^/(?:[A-Za-z0-9._-]+/)*[A-Za-z0-9._-]+$`)

type certificateDeployment struct {
	service         *SSLCertificateService
	ssh             cmdbservice.CmdbHostSSHServiceInterface
	ctx             *gin.Context
	logID           uint
	hostID          uint
	deployPath      string
	certificatePath string
	privateKeyPath  string
	tempCertPath    string
	tempKeyPath     string
	backupPath      string
	certExisted     bool
	keyExisted      bool
	logs            []string
}

func (s *SSLCertificateService) Deploy(c *gin.Context, certificateID uint, input model.DeploySSLCertificateInput, operator string) (*model.SSLCertificateDeployLogVO, error) {
	item, err := s.getRecord(certificateID)
	if err != nil {
		return nil, err
	}
	if item.Status != model.SSLCertificateStatusIssued || strings.TrimSpace(item.Certificate) == "" || strings.TrimSpace(item.PrivateKey) == "" {
		return nil, ErrSSLCertificateNotIssued
	}

	deployPath, err := normalizeCertificateDeployPath(input.DeployPath)
	if err != nil {
		return nil, err
	}
	if input.HostID == 0 {
		return nil, errors.New("请选择目标主机")
	}
	hostDAO := cmdbdao.NewCmdbHostDao()
	host, err := hostDAO.GetCmdbHostById(input.HostID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("目标主机不存在")
	}
	if err != nil {
		return nil, fmt.Errorf("查询目标主机失败: %w", err)
	}
	if host.HostType == cmdbmodel.HostTypeWindows {
		return nil, errors.New("SSL 证书仅支持部署到 Linux 主机")
	}
	if strings.TrimSpace(host.SSHIP) == "" || host.SSHPort == 0 {
		return nil, errors.New("目标主机 SSH 信息不完整")
	}
	if host.SSHKeyID == 0 {
		return nil, errors.New("目标主机未配置默认 SSH 凭据")
	}
	deploying, err := s.dao.HasRunningSSLCertificateDeployment(certificateID, host.ID)
	if err != nil {
		return nil, fmt.Errorf("检查证书部署任务失败: %w", err)
	}
	if deploying {
		return nil, ErrSSLCertificateDeploying
	}

	authID := host.SSHKeyID
	baseName := certificateFileBase(item.Domain)
	timestamp := time.Now().Format("20060102_150405_000000000")
	certificatePath := pathpkg.Join(deployPath, baseName+".pem")
	privateKeyPath := pathpkg.Join(deployPath, baseName+".key")
	backupPath := pathpkg.Join(deployPath, "backup", timestamp)
	initialLogs := []string{deploymentLogLine(fmt.Sprintf("已创建部署任务，目标主机：%s (%s)", displayHostName(host), host.SSHIP))}
	deployLog := &model.SSLCertificateDeployLog{
		CertificateID:   certificateID,
		Domain:          item.Domain,
		HostID:          host.ID,
		HostName:        displayHostName(host),
		HostIP:          host.SSHIP,
		AuthID:          authID,
		DeployPath:      deployPath,
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		BackupPath:      backupPath,
		Status:          model.SSLCertificateDeployStatusRunning,
		CreatedBy:       operator,
	}
	if err := s.dao.CreateSSLCertificateDeployLog(deployLog, initialLogs); err != nil {
		return nil, fmt.Errorf("创建证书部署日志失败: %w", err)
	}

	deploymentContext := copyDeploymentContext(c, authID)
	fullChain := strings.TrimSpace(item.Certificate) + "\n"
	issuerCertificate := strings.TrimSpace(item.IssuerCertificate)
	if issuerCertificate != "" && !containsCertificate(fullChain, issuerCertificate) {
		fullChain += issuerCertificate + "\n"
	}
	privateKey := strings.TrimSpace(item.PrivateKey) + "\n"

	go s.executeCertificateDeployment(deploymentContext, deployLog, fullChain, privateKey, timestamp, initialLogs)
	return deployLog.ToVO(), nil
}

func (s *SSLCertificateService) ListDeployLogs(certificateID uint, query model.SSLCertificateDeployLogQuery) ([]model.SSLCertificateDeployLogVO, int64, error) {
	if _, err := s.getRecord(certificateID); err != nil {
		return nil, 0, err
	}
	items, total, err := s.dao.ListSSLCertificateDeployLogs(certificateID, query)
	if err != nil {
		return nil, 0, fmt.Errorf("查询证书部署日志失败: %w", err)
	}
	list := make([]model.SSLCertificateDeployLogVO, 0, len(items))
	for i := range items {
		list = append(list, *items[i].ToVO())
	}
	return list, total, nil
}

func (s *SSLCertificateService) GetDeployLog(certificateID, logID uint) (*model.SSLCertificateDeployLogVO, error) {
	if certificateID == 0 || logID == 0 {
		return nil, ErrSSLCertificateNotFound
	}
	item, err := s.dao.GetSSLCertificateDeployLog(certificateID, logID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSSLCertificateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询证书部署日志失败: %w", err)
	}
	return item.ToVO(), nil
}

func (s *SSLCertificateService) executeCertificateDeployment(ctx *gin.Context, item *model.SSLCertificateDeployLog, fullChain, privateKey, timestamp string, logs []string) {
	deployment := &certificateDeployment{
		service:         s,
		ssh:             cmdbservice.GetCmdbHostSSHService(),
		ctx:             ctx,
		logID:           item.ID,
		hostID:          item.HostID,
		deployPath:      item.DeployPath,
		certificatePath: item.CertificatePath,
		privateKeyPath:  item.PrivateKeyPath,
		tempCertPath:    item.CertificatePath + "." + timestamp + ".tmp",
		tempKeyPath:     item.PrivateKeyPath + "." + timestamp + ".tmp",
		backupPath:      item.BackupPath,
		logs:            logs,
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			deployment.fail(fmt.Errorf("部署任务异常终止: %v", recovered))
		}
	}()

	if err := deployment.run(fullChain, privateKey); err != nil {
		deployment.fail(err)
		return
	}
	deployment.appendLog("Nginx 已重启，证书更新完成")
	if err := s.dao.FinishSSLCertificateDeployLog(item.ID, model.SSLCertificateDeployStatusSuccess, deployment.logs, deployment.backupPath, ""); err != nil {
		return
	}
}

func (d *certificateDeployment) run(fullChain, privateKey string) error {
	d.appendLog("正在检查并创建证书目录")
	if _, err := d.execute(fmt.Sprintf("mkdir -p -- %s %s", shellQuote(d.deployPath), shellQuote(d.backupPath))); err != nil {
		return fmt.Errorf("创建部署或备份目录失败: %w", err)
	}

	var err error
	d.certExisted, err = d.remoteFileExists(d.certificatePath)
	if err != nil {
		return fmt.Errorf("检查原证书文件失败: %w", err)
	}
	d.keyExisted, err = d.remoteFileExists(d.privateKeyPath)
	if err != nil {
		return fmt.Errorf("检查原私钥文件失败: %w", err)
	}
	if d.certExisted {
		if _, err := d.execute(fmt.Sprintf("cp -p -- %s %s", shellQuote(d.certificatePath), shellQuote(pathpkg.Join(d.backupPath, pathpkg.Base(d.certificatePath))))); err != nil {
			return fmt.Errorf("备份原证书文件失败: %w", err)
		}
		d.appendLog("原证书已备份到 " + d.backupPath)
	}
	if d.keyExisted {
		if _, err := d.execute(fmt.Sprintf("cp -p -- %s %s", shellQuote(d.privateKeyPath), shellQuote(pathpkg.Join(d.backupPath, pathpkg.Base(d.privateKeyPath))))); err != nil {
			return fmt.Errorf("备份原私钥文件失败: %w", err)
		}
		d.appendLog("原私钥已备份到 " + d.backupPath)
	}
	if !d.certExisted && !d.keyExisted {
		d.appendLog("目标路径未发现同名旧证书，将作为首次部署")
	}

	d.appendLog("正在通过 CMDB SSH 通道上传新证书和私钥")
	if err := d.uploadTemporaryFiles(fullChain, privateKey); err != nil {
		d.cleanupTemporaryFiles()
		return err
	}
	if _, err := d.execute(fmt.Sprintf("chmod 0644 %s && chmod 0600 %s", shellQuote(d.tempCertPath), shellQuote(d.tempKeyPath))); err != nil {
		d.cleanupTemporaryFiles()
		return fmt.Errorf("设置临时证书文件权限失败: %w", err)
	}

	d.appendLog("正在校验证书格式及证书私钥匹配关系")
	certDigest, err := d.execute(publicKeyDigestCommand(d.tempCertPath, true))
	if err != nil {
		d.cleanupTemporaryFiles()
		return fmt.Errorf("证书格式校验失败: %w", err)
	}
	keyDigest, err := d.execute(publicKeyDigestCommand(d.tempKeyPath, false))
	if err != nil {
		d.cleanupTemporaryFiles()
		return fmt.Errorf("私钥格式校验失败: %w", err)
	}
	if strings.TrimSpace(certDigest) == "" || strings.TrimSpace(certDigest) != strings.TrimSpace(keyDigest) {
		d.cleanupTemporaryFiles()
		return errors.New("证书与私钥不匹配")
	}
	d.appendLog("证书和私钥校验通过")

	if _, err := d.execute(fmt.Sprintf("mv -f -- %s %s && mv -f -- %s %s && chmod 0644 %s && chmod 0600 %s",
		shellQuote(d.tempCertPath), shellQuote(d.certificatePath), shellQuote(d.tempKeyPath), shellQuote(d.privateKeyPath),
		shellQuote(d.certificatePath), shellQuote(d.privateKeyPath))); err != nil {
		return d.rollback(fmt.Errorf("替换证书文件失败: %w", err), false)
	}
	d.appendLog("新证书已写入 " + d.certificatePath)
	d.appendLog("新私钥已写入 " + d.privateKeyPath)

	d.appendLog("正在执行 nginx -t 配置校验")
	nginxOutput, err := d.execute("nginx -t")
	if err != nil {
		return d.rollback(fmt.Errorf("Nginx 配置校验失败: %w", err), false)
	}
	if strings.TrimSpace(nginxOutput) != "" {
		d.appendLog("nginx -t: " + compactDeploymentOutput(nginxOutput))
	}
	d.appendLog("Nginx 配置校验通过，正在重启 Nginx")
	restartOutput, err := d.execute(nginxRestartCommand())
	if err != nil {
		return d.rollback(fmt.Errorf("Nginx 重启失败: %w", err), true)
	}
	if strings.TrimSpace(restartOutput) != "" {
		d.appendLog("Nginx: " + compactDeploymentOutput(restartOutput))
	}
	return nil
}

func (d *certificateDeployment) rollback(cause error, restart bool) error {
	d.appendLog("更新失败，正在恢复更新前的证书")
	commands := make([]string, 0, 2)
	if d.certExisted {
		commands = append(commands, fmt.Sprintf("cp -p -- %s %s", shellQuote(pathpkg.Join(d.backupPath, pathpkg.Base(d.certificatePath))), shellQuote(d.certificatePath)))
	} else {
		commands = append(commands, moveFileIfExistsCommand(d.certificatePath, pathpkg.Join(d.backupPath, "failed-"+pathpkg.Base(d.certificatePath))))
	}
	if d.keyExisted {
		commands = append(commands, fmt.Sprintf("cp -p -- %s %s", shellQuote(pathpkg.Join(d.backupPath, pathpkg.Base(d.privateKeyPath))), shellQuote(d.privateKeyPath)))
	} else {
		commands = append(commands, moveFileIfExistsCommand(d.privateKeyPath, pathpkg.Join(d.backupPath, "failed-"+pathpkg.Base(d.privateKeyPath))))
	}
	if _, err := d.execute(strings.Join(commands, " && ")); err != nil {
		d.appendLog("恢复旧证书失败：" + err.Error())
		return fmt.Errorf("%v；恢复旧证书失败: %w", cause, err)
	}
	d.appendLog("旧证书已恢复")
	if restart {
		if _, err := d.execute("nginx -t && " + nginxRestartCommand()); err != nil {
			d.appendLog("恢复后重启 Nginx 失败：" + err.Error())
			return fmt.Errorf("%v；旧证书已恢复，但 Nginx 重启失败: %w", cause, err)
		}
		d.appendLog("旧证书恢复后 Nginx 已重新启动")
	}
	return cause
}

func (d *certificateDeployment) cleanupTemporaryFiles() {
	commands := []string{
		moveFileIfExistsCommand(d.tempCertPath, pathpkg.Join(d.backupPath, "failed-upload-"+pathpkg.Base(d.tempCertPath))),
		moveFileIfExistsCommand(d.tempKeyPath, pathpkg.Join(d.backupPath, "failed-upload-"+pathpkg.Base(d.tempKeyPath))),
	}
	_, _ = d.execute(strings.Join(commands, " && "))
}

func (d *certificateDeployment) uploadTemporaryFiles(fullChain, privateKey string) error {
	localDir, err := os.MkdirTemp("", "ssl-certificate-deploy-")
	if err != nil {
		return fmt.Errorf("创建本地临时目录失败: %w", err)
	}
	defer os.RemoveAll(localDir)

	localCertPath := filepath.Join(localDir, pathpkg.Base(d.tempCertPath))
	localKeyPath := filepath.Join(localDir, pathpkg.Base(d.tempKeyPath))
	if err := os.WriteFile(localCertPath, []byte(fullChain), 0600); err != nil {
		return fmt.Errorf("创建本地临时证书失败: %w", err)
	}
	if err := os.WriteFile(localKeyPath, []byte(privateKey), 0600); err != nil {
		return fmt.Errorf("创建本地临时私钥失败: %w", err)
	}
	if err := d.ssh.UploadFile(d.ctx, d.hostID, localCertPath, d.deployPath); err != nil {
		return fmt.Errorf("上传证书文件失败: %w", err)
	}
	if err := d.ssh.UploadFile(d.ctx, d.hostID, localKeyPath, d.deployPath); err != nil {
		return fmt.Errorf("上传私钥文件失败: %w", err)
	}
	return nil
}

func (d *certificateDeployment) remoteFileExists(remotePath string) (bool, error) {
	output, err := d.execute(fmt.Sprintf("if [ -f %s ]; then printf yes; else printf no; fi", shellQuote(remotePath)))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) == "yes", nil
}

func (d *certificateDeployment) execute(command string) (string, error) {
	response, err := d.ssh.ExecuteCommand(d.ctx, d.hostID, command, true)
	if err != nil {
		return "", err
	}
	if response == nil {
		return "", nil
	}
	return response.Output, nil
}

func (d *certificateDeployment) appendLog(message string) {
	d.logs = append(d.logs, deploymentLogLine(message))
	_ = d.service.dao.UpdateSSLCertificateDeployProgress(d.logID, d.logs, d.backupPath)
}

func (d *certificateDeployment) fail(err error) {
	d.cleanupTemporaryFiles()
	d.appendLog("部署失败：" + err.Error())
	_ = d.service.dao.FinishSSLCertificateDeployLog(d.logID, model.SSLCertificateDeployStatusFailed, d.logs, d.backupPath, err.Error())
}

func normalizeCertificateDeployPath(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = "/etc/nginx/ssl"
	}
	if len(trimmed) > 400 || strings.ContainsAny(trimmed, "\r\n\x00") || !deployPathPattern.MatchString(trimmed) {
		return "", errors.New("部署路径必须是安全的 Linux 绝对路径")
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "." || segment == ".." {
			return "", errors.New("部署路径不能包含当前目录或上级目录")
		}
	}
	cleaned := pathpkg.Clean(trimmed)
	if cleaned == "/" || strings.Contains(cleaned, "/../") || strings.HasSuffix(cleaned, "/..") {
		return "", errors.New("不允许部署到根目录或包含上级目录")
	}
	return cleaned, nil
}

func displayHostName(host cmdbmodel.CmdbHost) string {
	if value := strings.TrimSpace(host.HostName); value != "" {
		return value
	}
	if value := strings.TrimSpace(host.Name); value != "" {
		return value
	}
	return fmt.Sprintf("主机 #%d", host.ID)
}

func copyDeploymentContext(c *gin.Context, authID uint) *gin.Context {
	copyContext := c.Copy()
	request := c.Request.Clone(context.Background())
	requestURL := &url.URL{}
	if c.Request.URL != nil {
		*requestURL = *c.Request.URL
	}
	query := requestURL.Query()
	if authID > 0 {
		query.Set("authId", fmt.Sprint(authID))
	}
	requestURL.RawQuery = query.Encode()
	request.URL = requestURL
	copyContext.Request = request
	return copyContext
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func moveFileIfExistsCommand(source, destination string) string {
	return fmt.Sprintf("if [ -f %s ]; then mv -f -- %s %s; fi", shellQuote(source), shellQuote(source), shellQuote(destination))
}

func publicKeyDigestCommand(remotePath string, certificate bool) string {
	if certificate {
		return fmt.Sprintf("openssl x509 -in %s -noout && openssl x509 -in %s -pubkey -noout | openssl pkey -pubin -outform DER 2>/dev/null | openssl dgst -sha256", shellQuote(remotePath), shellQuote(remotePath))
	}
	return fmt.Sprintf("openssl pkey -in %s -noout && openssl pkey -in %s -pubout -outform DER 2>/dev/null | openssl dgst -sha256", shellQuote(remotePath), shellQuote(remotePath))
}

func nginxRestartCommand() string {
	return "if command -v systemctl >/dev/null 2>&1; then systemctl restart nginx || service nginx restart; else service nginx restart; fi"
}

func deploymentLogLine(message string) string {
	return fmt.Sprintf("[%s] %s", time.Now().Format("2006-01-02 15:04:05"), message)
}

func compactDeploymentOutput(output string) string {
	value := strings.Join(strings.Fields(strings.TrimSpace(output)), " ")
	runes := []rune(value)
	if len(runes) > 1000 {
		return string(runes[:1000]) + "..."
	}
	return value
}
