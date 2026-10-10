package service

import (
	"context"
	"crypto/sha256"
	"debug/elf"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common"
	"dodevops-api/common/agent"
	"dodevops-api/common/config"
	"dodevops-api/common/util"
	"dodevops-api/pkg/redis"

	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

// EcsAuth SSH认证信息
type EcsAuth struct {
	ID                uint       `gorm:"column:id;comment:'主键';primaryKey;NOT NULL" json:"id"`
	Name              string     `gorm:"column:name;varchar(64);comment:'凭证名称';NOT NULL" json:"name"`
	Type              int        `gorm:"column:type;comment:'认证类型:1->密码,2->密钥,3->免密,4->跳板机';NOT NULL" json:"type"`
	Username          string     `gorm:"column:username;varchar(64);comment:'用户名(type=1时使用)'" json:"username"`
	Password          string     `gorm:"column:password;varchar(256);comment:'密码(type=1时使用)'" json:"password"`
	PublicKey         string     `gorm:"column:public_key;type:text;comment:'公钥(type=2时使用)'" json:"publicKey"`
	Port              int        `gorm:"column:port;comment:'端口号';default:22" json:"port"`
	CreateTime        util.HTime `gorm:"column:create_time;comment:'创建时间';NOT NULL" json:"createTime"`
	Remark            string     `gorm:"column:remark;varchar(500);comment:'备注'" json:"remark"`
	BastionHost       string     `gorm:"column:bastion_host;varchar(64);comment:'跳板机IP'" json:"bastionHost"`
	BastionPort       int        `gorm:"column:bastion_port;comment:'跳板机端口';default:22" json:"bastionPort"`
	BastionUsername   string     `gorm:"column:bastion_username;varchar(64);comment:'跳板机用户名'" json:"bastionUsername"`
	BastionAuthType   int        `gorm:"column:bastion_auth_type;comment:'跳板机认证类型:1->密码,2->私钥,3->免密'" json:"bastionAuthType"`
	BastionPassword   string     `gorm:"column:bastion_password;varchar(256);comment:'跳板机密码'" json:"bastionPassword"`
	BastionPrivateKey string     `gorm:"column:bastion_private_key;type:text;comment:'跳板机私钥'" json:"bastionPrivateKey"`
	TargetAuthType    int        `gorm:"column:target_auth_type;comment:'目标机器认证类型:1->密码,2->私钥,3->免密,4->跳板机免密'" json:"targetAuthType"`
	TargetPassword    string     `gorm:"column:target_password;varchar(256);comment:'目标机器密码'" json:"targetPassword"`
	TargetPrivateKey  string     `gorm:"column:target_private_key;type:text;comment:'目标机器私钥'" json:"targetPrivateKey"`
}

// TableName 指定表名
func (EcsAuth) TableName() string {
	return "config_ecsauth"
}

func (a *EcsAuth) DecryptSecrets() error {
	var err error

	a.Password, err = decryptAgentSecretIfPossible(a.Password)
	if err != nil {
		return err
	}
	a.PublicKey, err = decryptAgentSecretIfPossible(a.PublicKey)
	if err != nil {
		return err
	}
	a.BastionPassword, err = decryptAgentSecretIfPossible(a.BastionPassword)
	if err != nil {
		return err
	}
	a.BastionPrivateKey, err = decryptAgentSecretIfPossible(a.BastionPrivateKey)
	if err != nil {
		return err
	}
	a.TargetPassword, err = decryptAgentSecretIfPossible(a.TargetPassword)
	if err != nil {
		return err
	}
	a.TargetPrivateKey, err = decryptAgentSecretIfPossible(a.TargetPrivateKey)
	if err != nil {
		return err
	}

	return nil
}

func decryptAgentSecretIfPossible(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	decrypted, err := util.AESDecrypt(value)
	if err != nil {
		return value, nil
	}

	return decrypted, nil
}

// EcsAuthDao SSH密钥认证DAO接口
type EcsAuthDao interface {
	GetEcsAuthById(id uint) (EcsAuth, error)
}

// EcsAuthDaoImpl SSH密钥认证DAO实现
type EcsAuthDaoImpl struct{}

// NewEcsAuthDao 创建EcsAuthDao实例
func NewEcsAuthDao() EcsAuthDao {
	return &EcsAuthDaoImpl{}
}

// GetEcsAuthById 根据ID获取SSH密钥
func (d *EcsAuthDaoImpl) GetEcsAuthById(id uint) (EcsAuth, error) {
	// 从数据库查询SSH密钥
	var ecsAuth EcsAuth
	err := common.GetDB().Where("id = ?", id).First(&ecsAuth).Error
	if err != nil {
		return EcsAuth{}, fmt.Errorf("查询SSH密钥失败: %v", err)
	}
	if err = ecsAuth.DecryptSecrets(); err != nil {
		return EcsAuth{}, fmt.Errorf("解密SSH凭据失败: %v", err)
	}
	return ecsAuth, nil
}

// AgentServiceInterface agent服务接口
type AgentServiceInterface interface {
	DeployAgent(c *gin.Context, hostID uint)                            // 部署agent到指定主机
	DeployAgentWithVersion(c *gin.Context, hostID uint, version string) // 按指定版本部署agent到指定主机
	DeployAgentWithOptions(c *gin.Context, hostID uint, version string, options model.AgentDeployOptions)
	BatchDeployAgent(c *gin.Context, dto *model.BatchDeployAgentDto) // 批量部署agent
	BatchDeployAgentWithOptions(c *gin.Context, dto *model.BatchDeployAgentDto, options model.AgentDeployOptions)
	UninstallAgent(c *gin.Context, hostID uint)                                      // 卸载指定主机的agent
	BatchUninstallAgent(c *gin.Context, dto *model.BatchDeployAgentDto)              // 批量卸载agent
	RedeployAgent(c *gin.Context, hostID uint)                                       // 重新部署指定主机的agent
	BatchRedeployAgent(c *gin.Context, dto *model.RedeployAgentDto)                  // 批量重新部署agent
	DeleteAgent(c *gin.Context, hostID uint)                                         // 删除agent数据(用于离线服务器，通过hostID)
	DeleteAgentByID(c *gin.Context, agentID uint)                                    // 删除agent数据(通过agentID)
	GetAgentStatus(c *gin.Context, hostID uint)                                      // 获取agent状态
	RestartAgent(c *gin.Context, hostID uint)                                        // 重启agent
	GetAgentList(c *gin.Context, dto *model.AgentListDto)                            // 获取agent列表
	GetAgentCandidateHosts(c *gin.Context, dto *model.AgentCandidateListDto)         // 获取可部署agent的主机候选列表
	GetAgentCandidateGroups(c *gin.Context, dto *model.AgentCandidateGroupDto)       // 获取可部署agent的主机分组树
	GetAgentVersions(c *gin.Context)                                                 // 获取agent版本列表
	GetOfflineAssets(c *gin.Context)                                                 // 获取可选择的离线制品
	UploadOfflineAgentBinary(c *gin.Context)                                         // 上传离线Agent二进制
	CheckOfflineAssets(c *gin.Context)                                               // 检查离线制品上传状态
	PrecheckOfflineDeployment(c *gin.Context, dto *model.AgentOfflinePrecheckDto)    // 部署前检查离线制品和主机架构
	UpdateHeartbeat(c *gin.Context, hostID uint, heartbeat *model.AgentHeartbeatDto) // 更新心跳(旧版本，保持兼容)
	UpdateHeartbeatByIP(c *gin.Context, heartbeat *model.AgentHeartbeatDto)          // 通过IP更新心跳
	GetAgentStatistics(c *gin.Context)                                               // 获取统计信息
	CheckOfflineAgents()                                                             // 检查离线agent
}

// AgentServiceImpl agent服务实现
type AgentServiceImpl struct {
	hostDao    dao.CmdbHostDao
	agentDao   dao.AgentDao
	ecsAuthDao EcsAuthDao // 添加ECS认证DAO
}

// 全局主机锁管理器 - 防止对同一台物理主机并发操作导致进程冲突
var (
	hostLockManager = &HostLockManager{
		locks: make(map[string]*sync.Mutex),
		mu:    &sync.Mutex{},
	}
	agentBuildCache = newAgentBuildCacheManager()
)

// HostLockManager 主机锁管理器，用于防止并发部署时的竞争条件
type HostLockManager struct {
	locks map[string]*sync.Mutex // key: 主机IP, value: 该主机的互斥锁
	mu    *sync.Mutex            // 保护 locks map 的锁
}

// getLock 获取指定主机IP的锁
func (m *HostLockManager) getLock(hostIP string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()

	if lock, exists := m.locks[hostIP]; exists {
		return lock
	}

	// 创建新锁
	lock := &sync.Mutex{}
	m.locks[hostIP] = lock
	return lock
}

// Lock 锁定指定主机
func (m *HostLockManager) Lock(hostIP string) {
	lock := m.getLock(hostIP)
	log.Printf("尝试获取主机锁: %s", hostIP)
	lock.Lock()
	log.Printf("成功获取主机锁: %s", hostIP)
}

// Unlock 解锁指定主机
func (m *HostLockManager) Unlock(hostIP string) {
	lock := m.getLock(hostIP)
	lock.Unlock()
	log.Printf("释放主机锁: %s", hostIP)
}

// NewAgentService 创建agent服务实例
func NewAgentService() AgentServiceInterface {
	return &AgentServiceImpl{
		hostDao:    dao.NewCmdbHostDao(),
		agentDao:   dao.NewAgentDao(),
		ecsAuthDao: NewEcsAuthDao(), // 初始化ECS认证DAO
	}
}

func newAgentBuildCacheManager() *agentBuildCacheManager {
	baseDir := filepath.Join(os.TempDir(), "dodevops-agent-build-cache")
	return &agentBuildCacheManager{
		baseDir:  baseDir,
		ttl:      15 * time.Minute,
		inFlight: make(map[string]*agentBuildFlight),
	}
}

// 编译目标平台类型
type BuildTarget struct {
	GOOS   string
	GOARCH string
	Suffix string
}

type agentBuildRequest struct {
	TargetOS       string
	Version        string
	VersionTag     string
	GroupID        uint
	GroupName      string
	HeartbeatURL   string
	HeartbeatToken string
	PushgatewayURL string
}

type agentBuildArtifact struct {
	BinaryName string
	BinaryPath string
}

type agentBuildCacheManager struct {
	baseDir  string
	ttl      time.Duration
	mu       sync.Mutex
	inFlight map[string]*agentBuildFlight
}

type agentBuildFlight struct {
	wg       sync.WaitGroup
	artifact agentBuildArtifact
	err      error
}

// 支持的编译目标平台
var buildTargets = map[string]BuildTarget{
	"linux":   {GOOS: "linux", GOARCH: "amd64", Suffix: ""},
	"windows": {GOOS: "windows", GOARCH: "amd64", Suffix: ".exe"},
	"darwin":  {GOOS: "darwin", GOARCH: "amd64", Suffix: ""},
}

func configuredAgentVersion() string {
	if config.Config != nil && strings.TrimSpace(config.Config.Monitor.Agent.Version) != "" {
		return strings.TrimSpace(config.Config.Monitor.Agent.Version)
	}
	return "2.0.0"
}

func configuredAgentVersionTag() string {
	if config.Config != nil && strings.TrimSpace(config.Config.Monitor.Agent.VersionTag) != "" {
		return strings.TrimSpace(config.Config.Monitor.Agent.VersionTag)
	}
	return "v2"
}

func prioritizeAgentVersionStats(stats []model.AgentVersionStat, configuredVersion string) []model.AgentVersionStat {
	configuredVersion = strings.TrimSpace(configuredVersion)
	if configuredVersion == "" {
		return append([]model.AgentVersionStat(nil), stats...)
	}

	result := make([]model.AgentVersionStat, 0, len(stats)+1)
	found := false
	for _, stat := range stats {
		if strings.TrimSpace(stat.Version) != configuredVersion {
			continue
		}
		stat.Version = configuredVersion
		result = append(result, stat)
		found = true
		break
	}

	if !found {
		result = append(result, model.AgentVersionStat{Version: configuredVersion, Count: 0})
	}

	for _, stat := range stats {
		if strings.TrimSpace(stat.Version) == configuredVersion {
			continue
		}
		result = append(result, stat)
	}

	return result
}

func defaultHeartbeatURL() string {
	if config.Config != nil && config.Config.Server.PublicURL != "" {
		return strings.TrimRight(config.Config.Server.PublicURL, "/") + "/api/v1/cmdb/agent/heartbeat"
	}
	return "http://127.0.0.1:8000/api/v1/cmdb/agent/heartbeat"
}

func buildAgentRequest(targetOS, version string, groupID uint, groupName string) agentBuildRequest {
	version = strings.TrimSpace(version)
	if version == "" {
		version = configuredAgentVersion()
	}

	heartbeatURL := defaultHeartbeatURL()
	heartbeatToken := "agent-heartbeat-token-2024"
	pushgatewayURL := "http://8.130.14.34:9091"

	if config.Config != nil && config.Config.Monitor.Agent.HeartbeatServerURL != "" {
		heartbeatURL = config.Config.Monitor.Agent.HeartbeatServerURL
	}
	if config.Config != nil && config.Config.Monitor.Agent.HeartbeatToken != "" {
		heartbeatToken = config.Config.Monitor.Agent.HeartbeatToken
	}
	if config.Config != nil && config.Config.Monitor.Pushgateway.URL != "" {
		pushgatewayURL = config.Config.Monitor.Pushgateway.URL
	} else if config.Config != nil && config.Config.Monitor.MetricsPush.URL != "" {
		pushgatewayURL = config.Config.Monitor.MetricsPush.URL
	}

	return agentBuildRequest{
		TargetOS:       targetOS,
		Version:        version,
		VersionTag:     configuredAgentVersionTag(),
		GroupID:        groupID,
		GroupName:      groupName,
		HeartbeatURL:   heartbeatURL,
		HeartbeatToken: heartbeatToken,
		PushgatewayURL: pushgatewayURL,
	}
}

func (r agentBuildRequest) buildTarget() (BuildTarget, error) {
	buildTarget, exists := buildTargets[r.TargetOS]
	if !exists {
		return BuildTarget{}, fmt.Errorf("不支持的操作系统: %s", r.TargetOS)
	}
	return buildTarget, nil
}

func (r agentBuildRequest) binaryName() (string, error) {
	buildTarget, err := r.buildTarget()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("dodevops-agent%s", buildTarget.Suffix), nil
}

func (r agentBuildRequest) mainCode() (string, error) {
	mainCode := agent.CreateAgentMainFile(
		r.HeartbeatURL,
		r.HeartbeatToken,
		r.PushgatewayURL,
		r.GroupID,
		r.GroupName,
		r.Version,
		r.VersionTag,
	)
	if len(mainCode) == 0 {
		return "", fmt.Errorf("生成的main.go内容为空")
	}
	if err := agent.ValidateAgentCode(mainCode); err != nil {
		return "", fmt.Errorf("生成的Agent代码有语法错误: %v", err)
	}
	return mainCode, nil
}

func (r agentBuildRequest) cacheKey(mainCode string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		r.TargetOS,
		r.Version,
		r.VersionTag,
		mainCode,
	}, "\n---\n")))
	return fmt.Sprintf("%x", sum[:])
}

func (m *agentBuildCacheManager) ensureBinary(req agentBuildRequest) (agentBuildArtifact, error) {
	mainCode, err := req.mainCode()
	if err != nil {
		return agentBuildArtifact{}, err
	}
	key := req.cacheKey(mainCode)

	if artifact, ok := m.lookup(key, req); ok {
		log.Printf("复用Agent编译缓存: key=%s targetOS=%s version=%s group=%d", key[:12], req.TargetOS, req.Version, req.GroupID)
		return artifact, nil
	}

	return m.buildWithSingleFlight(key, req, mainCode)
}

func (m *agentBuildCacheManager) buildWithSingleFlight(key string, req agentBuildRequest, mainCode string) (agentBuildArtifact, error) {
	m.mu.Lock()
	if flight, ok := m.inFlight[key]; ok {
		m.mu.Unlock()
		flight.wg.Wait()
		return flight.artifact, flight.err
	}

	flight := &agentBuildFlight{}
	flight.wg.Add(1)
	m.inFlight[key] = flight
	m.mu.Unlock()

	defer func() {
		flight.wg.Done()
		m.mu.Lock()
		delete(m.inFlight, key)
		m.mu.Unlock()
	}()

	artifact, err := m.compileAndStore(key, req, mainCode)
	flight.artifact = artifact
	flight.err = err
	return artifact, err
}

func (m *agentBuildCacheManager) lookup(key string, req agentBuildRequest) (agentBuildArtifact, bool) {
	binaryName, err := req.binaryName()
	if err != nil {
		return agentBuildArtifact{}, false
	}
	binaryPath := filepath.Join(m.baseDir, key, binaryName)
	info, err := os.Stat(binaryPath)
	if err != nil || info.IsDir() {
		return agentBuildArtifact{}, false
	}
	if m.ttl > 0 && time.Since(info.ModTime()) > m.ttl {
		_ = os.RemoveAll(filepath.Join(m.baseDir, key))
		return agentBuildArtifact{}, false
	}
	return agentBuildArtifact{
		BinaryName: binaryName,
		BinaryPath: binaryPath,
	}, true
}

func (m *agentBuildCacheManager) compileAndStore(key string, req agentBuildRequest, mainCode string) (agentBuildArtifact, error) {
	if err := os.MkdirAll(m.baseDir, 0755); err != nil {
		return agentBuildArtifact{}, fmt.Errorf("创建Agent缓存目录失败: %v", err)
	}

	if err := m.cleanupExpired(); err != nil {
		log.Printf("清理Agent构建缓存失败(忽略): %v", err)
	}

	workDir, err := os.MkdirTemp("", "agent-build-cache-work-")
	if err != nil {
		return agentBuildArtifact{}, fmt.Errorf("创建Agent构建工作目录失败: %v", err)
	}
	defer os.RemoveAll(workDir)

	buildTarget, err := req.buildTarget()
	if err != nil {
		return agentBuildArtifact{}, err
	}
	binaryName, err := req.binaryName()
	if err != nil {
		return agentBuildArtifact{}, err
	}

	if err := writeAgentBuildModule(workDir, mainCode); err != nil {
		return agentBuildArtifact{}, err
	}

	binaryPath := filepath.Join(workDir, binaryName)
	if err := compileAgentBinary(workDir, binaryPath, buildTarget); err != nil {
		return agentBuildArtifact{}, err
	}

	cacheDir := filepath.Join(m.baseDir, key)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return agentBuildArtifact{}, fmt.Errorf("创建Agent缓存分组目录失败: %v", err)
	}

	cachedBinaryPath := filepath.Join(cacheDir, binaryName)
	if err := copyLocalFile(binaryPath, cachedBinaryPath, 0755); err != nil {
		return agentBuildArtifact{}, fmt.Errorf("写入Agent缓存二进制失败: %v", err)
	}

	return agentBuildArtifact{
		BinaryName: binaryName,
		BinaryPath: cachedBinaryPath,
	}, nil
}

func (m *agentBuildCacheManager) cleanupExpired() error {
	if m.ttl <= 0 {
		return nil
	}
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	now := time.Now()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(m.baseDir, entry.Name())
		info, err := os.Stat(dirPath)
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > m.ttl {
			_ = os.RemoveAll(dirPath)
		}
	}
	return nil
}

func writeAgentBuildModule(workDir, mainCode string) error {
	mainGoPath := filepath.Join(workDir, "main.go")
	if err := os.WriteFile(mainGoPath, []byte(mainCode), 0644); err != nil {
		return fmt.Errorf("创建main.go文件失败: %v", err)
	}

	goModContent := `module agent

go 1.24

require (
	github.com/prometheus/client_golang v1.19.1
	github.com/prometheus/client_model v0.6.1
	github.com/prometheus/common v0.48.0
	github.com/shirou/gopsutil/v3 v3.23.10
)`
	goModPath := filepath.Join(workDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		return fmt.Errorf("创建go.mod文件失败: %v", err)
	}
	return nil
}

func compileAgentBinary(workDir, binaryPath string, buildTarget BuildTarget) error {
	goTempDir := filepath.Join(workDir, ".gocache")
	goCacheDir := filepath.Join(goTempDir, "build")
	goTmpDir := filepath.Join(goTempDir, "tmp")
	goModCacheDir := filepath.Join(goTempDir, "mod")

	for _, dir := range []string{goTempDir, goCacheDir, goTmpDir, goModCacheDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建Go缓存目录失败: %v", err)
		}
	}

	baseEnvVars := []string{
		"GO111MODULE=on",
		"CGO_ENABLED=0",
		"GOFLAGS=-buildvcs=false",
		fmt.Sprintf("GOMODCACHE=%s", goModCacheDir),
		fmt.Sprintf("GOCACHE=%s", goCacheDir),
		fmt.Sprintf("GOTMPDIR=%s", goTmpDir),
		fmt.Sprintf("GOPATH=%s", goTempDir),
	}
	downloadEnvVars := append(baseEnvVars,
		"GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct",
		"GOSUMDB=sum.golang.google.cn",
	)
	goEnvVars := append(baseEnvVars,
		"GOPROXY=https://goproxy.cn,direct",
		"GOSUMDB=off",
	)

	if output, err := runGoCommand(workDir, downloadEnvVars, "mod", "download"); err != nil {
		log.Printf("go mod download 失败，继续尝试后续流程: %v, 输出: %s", err, output)
		if retryOutput, retryErr := runGoCommand(workDir, append(baseEnvVars, "GOPROXY=direct", "GOSUMDB=off"), "mod", "download"); retryErr != nil {
			log.Printf("go mod download 直连重试失败: %v, 输出: %s", retryErr, retryOutput)
		}
	}

	if output, err := runGoCommand(workDir, goEnvVars, "mod", "tidy"); err != nil {
		log.Printf("go mod tidy 失败，尝试清缓存重试: %v, 输出: %s", err, output)
		_, _ = runGoCommand(workDir, goEnvVars, "clean", "-modcache")
		for _, dir := range []string{goModCacheDir, goCacheDir} {
			_ = os.MkdirAll(dir, 0755)
		}
		if retryOutput, retryErr := runGoCommand(workDir, append(baseEnvVars, "GOPROXY=https://goproxy.cn,direct", "GOSUMDB=off"), "mod", "tidy"); retryErr != nil {
			return fmt.Errorf("运行 go mod tidy 失败: %v, 输出: %s", retryErr, retryOutput)
		}
	}

	buildFlags := []string{
		"build",
		"-a",
		"-ldflags", "-s -w -extldflags '-static'",
		"-installsuffix", "netgo",
		"-tags", "netgo,osusergo",
		"-o", binaryPath,
		filepath.Join(workDir, "main.go"),
	}
	buildEnvVars := append(goEnvVars,
		fmt.Sprintf("GOOS=%s", buildTarget.GOOS),
		fmt.Sprintf("GOARCH=%s", buildTarget.GOARCH),
		"GODEBUG=netdns=go",
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", buildFlags...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), buildEnvVars...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("编译超时（5分钟），可能是网络问题或依赖下载失败")
	}
	if err != nil {
		return fmt.Errorf("编译失败: %v, 输出: %s", err, string(output))
	}

	return nil
}

func runGoCommand(workDir string, envVars []string, args ...string) (string, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), envVars...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func copyLocalFile(srcPath, dstPath string, mode os.FileMode) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return nil
}

func joinRemotePath(targetOS string, elems ...string) string {
	if targetOS == "windows" {
		return filepath.Join(elems...)
	}
	return path.Join(elems...)
}

func remoteParentDir(remotePath string) string {
	if strings.Contains(remotePath, `\`) || strings.Contains(remotePath, ":") {
		return filepath.Dir(remotePath)
	}
	return path.Dir(remotePath)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func sudoPrefix(sshConfig util.SSHConfig) string {
	if strings.EqualFold(strings.TrimSpace(sshConfig.Username), "root") {
		return ""
	}
	return "sudo "
}

func normalizePhysicalHostKeys(host model.CmdbHost) []string {
	seen := make(map[string]struct{})
	keys := make([]string, 0, 4)
	add := func(prefix, value string) {
		value = normalizePhysicalHostValue(value)
		if value == "" {
			return
		}
		key := prefix + ":" + value
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	add("instance_id", host.InstanceID)
	add("private_ip", host.PrivateIP)
	add("ssh_ip", host.SSHIP)
	return keys
}

func normalizePhysicalHostValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Trim(value, "\"'")
	if value == "" {
		return ""
	}

	invalidValues := map[string]struct{}{
		"-":         {},
		"--":        {},
		"无":         {},
		"暂无":        {},
		"未知":        {},
		"none":      {},
		"null":      {},
		"nil":       {},
		"n/a":       {},
		"na":        {},
		"unknown":   {},
		"0.0.0.0":   {},
		"127.0.0.1": {},
		"localhost": {},
		"::":        {},
		"::1":       {},
	}
	if _, invalid := invalidValues[value]; invalid {
		return ""
	}
	return value
}

func normalizeAgentCandidatePrimaryHostKey(host model.CmdbHost) string {
	for _, value := range []string{host.PrivateIP, host.SSHIP, host.PublicIP, host.InstanceID} {
		if normalized := normalizePhysicalHostValue(value); normalized != "" {
			return normalized
		}
	}
	return ""
}

func shouldTreatAgentAsDeployed(agent *model.Agent) bool {
	if agent == nil {
		return false
	}
	switch agent.Status {
	case model.AgentStatusDeploying, model.AgentStatusRunning, model.AgentStatusStartError:
		return true
	default:
		return false
	}
}

func (s *AgentServiceImpl) buildDeployedPhysicalHostMap() (map[string]model.Agent, error) {
	agents, err := s.agentDao.GetAll()
	if err != nil {
		return nil, err
	}

	deployed := make(map[string]model.Agent)
	for _, record := range agents {
		agentRecord := record
		if !shouldTreatAgentAsDeployed(&agentRecord) {
			continue
		}

		host, hostErr := s.hostDao.GetCmdbHostById(record.HostID)
		if hostErr != nil {
			continue
		}
		for _, key := range normalizePhysicalHostKeys(host) {
			deployed[key] = record
		}
	}
	return deployed, nil
}

func findDeployedAgentForPhysicalHost(host model.CmdbHost, deployed map[string]model.Agent) (*model.Agent, bool) {
	for _, key := range normalizePhysicalHostKeys(host) {
		if agentRecord, ok := deployed[key]; ok {
			return &agentRecord, true
		}
	}
	return nil, false
}

func (s *AgentServiceImpl) findExistingPhysicalAgent(host model.CmdbHost) (*model.Agent, bool, error) {
	deployed, err := s.buildDeployedPhysicalHostMap()
	if err != nil {
		return nil, false, err
	}
	agentRecord, exists := findDeployedAgentForPhysicalHost(host, deployed)
	return agentRecord, exists, nil
}

func physicalHostDuplicateMessage(host model.CmdbHost, agent *model.Agent) string {
	if agent == nil {
		return "该真实主机已部署Agent"
	}
	return fmt.Sprintf("该真实主机已通过资产ID %d 部署Agent，请勿重复部署", agent.HostID)
}

func fillAgentCandidateStatus(item *model.AgentCandidateHostVO, agentRecord *model.Agent) {
	if item == nil || agentRecord == nil {
		return
	}

	agentVO := agentRecord.ToVO()
	fillAgentOnlineStatus(agentRecord, agentVO)
	item.AgentID = agentRecord.ID
	item.AgentHostID = agentRecord.HostID
	item.AgentStatus = agentRecord.Status
	item.AgentStatusText = agentVO.StatusText
	item.AgentOnlineStatus = agentVO.OnlineStatus
	item.AgentVersion = agentRecord.Version
	item.LastHeartbeat = agentRecord.LastHeartbeat
	item.InstallProgress = agentRecord.InstallProgress
	item.InstallProgressText = agentRecord.GetInstallProgressText()
}

func isAgentCandidateDeployable(host model.CmdbHost, deployed map[string]model.Agent) bool {
	_, exists := findDeployedAgentForPhysicalHost(host, deployed)
	return !exists
}

func isLinuxAgentCandidateHost(host model.CmdbHost) bool {
	if host.HostType == model.HostTypeWindows {
		return false
	}
	return !strings.Contains(strings.ToLower(strings.TrimSpace(host.OS)), "windows")
}

func markAgentCandidatePhysicalHostSeen(host model.CmdbHost, seen map[string]struct{}) bool {
	key := normalizeAgentCandidatePrimaryHostKey(host)
	if key == "" {
		return false
	}

	if _, exists := seen[key]; exists {
		return true
	}
	seen[key] = struct{}{}
	return false
}

func buildAgentCandidateGroupTree(groups []model.CmdbGroup, directDeployableCount map[uint]int) []model.AgentCandidateGroupVO {
	childrenByParent := make(map[uint][]model.CmdbGroup)
	for _, group := range groups {
		childrenByParent[group.ParentID] = append(childrenByParent[group.ParentID], group)
	}

	var build func(parentID uint) []model.AgentCandidateGroupVO
	build = func(parentID uint) []model.AgentCandidateGroupVO {
		nodes := make([]model.AgentCandidateGroupVO, 0, len(childrenByParent[parentID]))
		for _, group := range childrenByParent[parentID] {
			node := model.AgentCandidateGroupVO{
				ID:                  group.ID,
				ParentID:            group.ParentID,
				Name:                group.Name,
				HostCount:           directDeployableCount[group.ID],
				DeployableHostCount: directDeployableCount[group.ID],
			}

			node.Children = build(group.ID)
			for _, child := range node.Children {
				node.HostCount += child.HostCount
				node.DeployableHostCount += child.DeployableHostCount
			}
			if node.DeployableHostCount > 0 {
				nodes = append(nodes, node)
			}
		}
		return nodes
	}

	return build(0)
}

// DeployAgent 部署agent到指定主机
func (s *AgentServiceImpl) DeployAgent(c *gin.Context, hostID uint) {
	s.DeployAgentWithOptions(c, hostID, "", model.AgentDeployOptions{})
}

// DeployAgentWithVersion 部署agent到指定主机，可使用请求指定版本。
func (s *AgentServiceImpl) DeployAgentWithVersion(c *gin.Context, hostID uint, version string) {
	s.DeployAgentWithOptions(c, hostID, version, model.AgentDeployOptions{})
}

func (s *AgentServiceImpl) DeployAgentWithOptions(c *gin.Context, hostID uint, version string, options model.AgentDeployOptions) {
	if err := validateAgentDeployOptions(options); err != nil {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), err.Error())
		return
	}

	// 参数校验
	if hostID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "主机ID不能为空")
		return
	}

	version = strings.TrimSpace(version)
	if version == "" {
		version = configuredAgentVersion()
	}

	// 获取主机信息
	host, err := s.hostDao.GetCmdbHostById(hostID)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "获取主机信息失败")
		return
	}

	if host.Name == "" {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "主机名称为空")
		return
	}

	if existingAgent, exists, checkErr := s.findExistingPhysicalAgent(host); checkErr != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "检查Agent重复部署失败")
		return
	} else if exists {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), physicalHostDuplicateMessage(host, existingAgent))
		return
	}
	if strings.EqualFold(strings.TrimSpace(options.DeployMode), "offline") {
		if _, _, err := s.precheckOfflineHost(host, options); err != nil {
			result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), err.Error())
			return
		}
	}

	// 创建或更新Agent记录（Linux主机专用）
	agentRecord := &model.Agent{
		HostID:      hostID,
		HostName:    host.Name,
		Version:     version,
		Status:      model.AgentStatusDeploying,
		InstallPath: "/opt/agent", // Linux的标准安装路径
		Port:        12580,
		CreateTime:  util.HTime{Time: time.Now()},
	}

	// 先创建或更新数据库记录
	if err := s.createOrUpdateAgent(agentRecord); err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "创建Agent记录失败")
		return
	}

	// 异步部署
	go func() {
		deployErr := s.deployAgentAsyncWithOptions(host.ID, host, version, options)
		if deployErr != nil {
			log.Printf("Deploy agent to host %d failed: %v", hostID, deployErr)
			// 更新状态为部署失败
			s.agentDao.UpdateByHostID(hostID, map[string]interface{}{
				"status":           model.AgentStatusDeployFailed,
				"install_progress": model.InstallProgressFailed,
				"error_msg":        deployErr.Error(),
			})
		}
	}()

	result.Success(c, map[string]interface{}{
		"message": "Agent部署任务已启动，正在后台执行",
		"hostId":  hostID,
		"status":  "deploying",
		"version": version,
	})
}

// BatchDeployAgent 批量部署agent到多台主机
// 优化策略：编译一次，并行拷贝和启动到多台主机
func (s *AgentServiceImpl) BatchDeployAgent(c *gin.Context, dto *model.BatchDeployAgentDto) {
	s.BatchDeployAgentWithOptions(c, dto, dto.DeployOptions())
}

func (s *AgentServiceImpl) BatchDeployAgentWithOptions(c *gin.Context, dto *model.BatchDeployAgentDto, options model.AgentDeployOptions) {
	if err := validateAgentDeployOptions(options); err != nil {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), err.Error())
		return
	}

	// 参数校验
	if len(dto.HostIDs) == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "主机ID列表不能为空")
		return
	}

	// 设置默认版本
	version := dto.Version
	if version == "" {
		version = configuredAgentVersion()
	}

	var successHosts []uint
	var failedHosts []map[string]interface{}
	deployedHosts, deployedErr := s.buildDeployedPhysicalHostMap()
	if deployedErr != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "检查Agent重复部署失败")
		return
	}

	// 收集所有主机信息并创建Agent记录
	var validHosts []model.CmdbHost
	for _, hostID := range dto.HostIDs {
		// 获取主机信息
		host, err := s.hostDao.GetCmdbHostById(hostID)
		if err != nil {
			failedHosts = append(failedHosts, map[string]interface{}{
				"hostId": hostID,
				"error":  "获取主机信息失败",
			})
			continue
		}

		if host.Name == "" {
			failedHosts = append(failedHosts, map[string]interface{}{
				"hostId": hostID,
				"error":  "主机名称为空",
			})
			continue
		}

		if existingAgent, exists := findDeployedAgentForPhysicalHost(host, deployedHosts); exists {
			failedHosts = append(failedHosts, map[string]interface{}{
				"hostId": hostID,
				"error":  physicalHostDuplicateMessage(host, existingAgent),
			})
			continue
		}
		if strings.EqualFold(strings.TrimSpace(options.DeployMode), "offline") {
			if _, _, err := s.precheckOfflineHost(host, options); err != nil {
				failedHosts = append(failedHosts, map[string]interface{}{
					"hostId": hostID,
					"error":  err.Error(),
				})
				continue
			}
		}

		// 创建或更新Agent记录
		agentRecord := &model.Agent{
			HostID:      hostID,
			HostName:    host.Name,
			Version:     version,
			Status:      model.AgentStatusDeploying,
			InstallPath: "/opt/agent",
			Port:        12580,
			CreateTime:  util.HTime{Time: time.Now()},
		}

		// 创建或更新数据库记录
		if err := s.createOrUpdateAgent(agentRecord); err != nil {
			failedHosts = append(failedHosts, map[string]interface{}{
				"hostId": hostID,
				"error":  "创建Agent记录失败",
			})
			continue
		}

		successHosts = append(successHosts, hostID)
		validHosts = append(validHosts, host)
		for _, key := range normalizePhysicalHostKeys(host) {
			deployedHosts[key] = *agentRecord
		}
	}

	// 异步批量部署
	go func() {
		batchErr := s.batchDeployAgentAsyncWithOptions(validHosts, version, options)
		if batchErr != nil {
			log.Printf("Batch deploy agents failed: %v", batchErr)
		}
	}()

	result.Success(c, map[string]interface{}{
		"message":      fmt.Sprintf("批量部署任务已启动，成功启动 %d 台，失败 %d 台", len(successHosts), len(failedHosts)),
		"successCount": len(successHosts),
		"failedCount":  len(failedHosts),
		"successHosts": successHosts,
		"failedHosts":  failedHosts,
		"status":       "deploying",
		"version":      version,
	})
}

// GetAgentList 获取agent列表
func (s *AgentServiceImpl) GetAgentList(c *gin.Context, dto *model.AgentListDto) {
	agents, total, err := s.agentDao.GetList(dto)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "获取Agent列表失败")
		return
	}

	// 转换为VO并关联SSH IP
	var agentVOs []*model.AgentVO
	for _, agent := range agents {
		agentVO := agent.ToVO()
		// 获取主机信息以获得SSH IP
		if host, err := s.hostDao.GetCmdbHostById(agent.HostID); err == nil {
			agentVO.SSHIP = host.SSHIP
		}
		fillAgentOnlineStatus(&agent, agentVO)
		agentVOs = append(agentVOs, agentVO)
	}

	result.Success(c, map[string]interface{}{
		"list":     agentVOs,
		"total":    total,
		"page":     dto.Page,
		"pageSize": dto.PageSize,
	})
}

func (s *AgentServiceImpl) GetAgentCandidateHosts(c *gin.Context, dto *model.AgentCandidateListDto) {
	if dto.Page <= 0 {
		dto.Page = 1
	}
	if dto.PageSize <= 0 {
		dto.PageSize = 10
	}
	if dto.PageSize > 1000 {
		dto.PageSize = 1000
	}

	filter := model.CmdbHostFilter{
		Keyword:   strings.TrimSpace(dto.Keyword),
		GroupID:   dto.GroupID,
		Statuses:  []int{1},
		SortBy:    "update_time",
		SortOrder: "desc",
	}

	hosts, err := s.hostDao.QueryCmdbHosts(filter)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "获取Agent候选主机失败")
		return
	}
	deployedHosts, err := s.buildDeployedPhysicalHostMap()
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "检查Agent重复部署失败")
		return
	}

	allCandidates := make([]model.AgentCandidateHostVO, 0, len(hosts))
	seenPhysicalHosts := make(map[string]struct{})
	for _, host := range hosts {
		if !isLinuxAgentCandidateHost(host) {
			continue
		}

		item := model.AgentCandidateHostVO{
			HostID:     host.ID,
			HostName:   host.HostName,
			Name:       host.Name,
			GroupID:    host.GroupID,
			GroupName:  host.Group.Name,
			PrivateIP:  host.PrivateIP,
			PublicIP:   host.PublicIP,
			SSHIP:      host.SSHIP,
			HostType:   host.HostType,
			Status:     host.Status,
			OS:         host.OS,
			Remark:     host.Remark,
			Deployable: true,
		}

		if existingAgent, exists := findDeployedAgentForPhysicalHost(host, deployedHosts); exists {
			item.Deployable = false
			item.DeployBlockedReason = physicalHostDuplicateMessage(host, existingAgent)
			fillAgentCandidateStatus(&item, existingAgent)
		} else {
			if markAgentCandidatePhysicalHostSeen(host, seenPhysicalHosts) {
				item.Deployable = false
				item.DeployBlockedReason = "该真实主机在当前候选结果中已存在，请勿重复部署"
			}
			if agentRecord, agentErr := s.agentDao.GetByHostID(host.ID); agentErr == nil && agentRecord != nil {
				fillAgentCandidateStatus(&item, agentRecord)
			}
		}

		if dto.DeployableOnly && !item.Deployable {
			continue
		}

		allCandidates = append(allCandidates, item)
	}

	total := int64(len(allCandidates))
	start := (dto.Page - 1) * dto.PageSize
	if start > len(allCandidates) {
		start = len(allCandidates)
	}
	end := start + dto.PageSize
	if end > len(allCandidates) {
		end = len(allCandidates)
	}
	list := allCandidates[start:end]

	result.Success(c, map[string]interface{}{
		"list":     list,
		"total":    total,
		"page":     dto.Page,
		"pageSize": dto.PageSize,
	})
}

func (s *AgentServiceImpl) GetAgentCandidateGroups(c *gin.Context, dto *model.AgentCandidateGroupDto) {
	filter := model.CmdbHostFilter{
		Keyword:   strings.TrimSpace(dto.Keyword),
		Statuses:  []int{1},
		SortBy:    "update_time",
		SortOrder: "desc",
	}

	hosts, err := s.hostDao.QueryCmdbHosts(filter)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "获取Agent候选主机失败")
		return
	}
	deployedHosts, err := s.buildDeployedPhysicalHostMap()
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "检查Agent重复部署失败")
		return
	}

	directDeployableCount := make(map[uint]int)
	seenPhysicalHosts := make(map[string]struct{})
	total := 0
	for _, host := range hosts {
		if !isLinuxAgentCandidateHost(host) {
			continue
		}
		if !isAgentCandidateDeployable(host, deployedHosts) {
			continue
		}
		if markAgentCandidatePhysicalHostSeen(host, seenPhysicalHosts) {
			continue
		}
		directDeployableCount[host.GroupID]++
		total++
	}

	groupDao := dao.NewCmdbGroupDao()
	groups := groupDao.GetCmdbGroupList()
	tree := buildAgentCandidateGroupTree(groups, directDeployableCount)

	result.Success(c, map[string]interface{}{
		"list":  tree,
		"total": total,
	})
}

// GetAgentStatus 获取agent状态
func (s *AgentServiceImpl) GetAgentStatus(c *gin.Context, hostID uint) {
	if hostID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "主机ID不能为空")
		return
	}

	// 从数据库获取Agent信息
	agent, err := s.agentDao.GetByHostID(hostID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			result.FailedWithCode(c, int(result.ApiCode.FAILED), "Agent未部署")
			return
		}
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "获取Agent状态失败")
		return
	}

	// 如果Agent状态为运行中，启动异步健康检查
	if agent.Status == model.AgentStatusRunning {
		go s.performHealthCheck(hostID)
	}

	// 转换为VO并关联SSH IP
	agentVO := agent.ToVO()
	if host, err := s.hostDao.GetCmdbHostById(agent.HostID); err == nil {
		agentVO.SSHIP = host.SSHIP
	}
	fillAgentOnlineStatus(agent, agentVO)

	result.Success(c, agentVO)
}

func (s *AgentServiceImpl) GetAgentVersions(c *gin.Context) {
	stats, err := s.agentDao.GetVersionStats()
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "获取Agent版本列表失败")
		return
	}
	stats = prioritizeAgentVersionStats(stats, configuredAgentVersion())
	result.Success(c, stats)
}

// UpdateHeartbeat 更新心跳(旧版本，保持兼容性)
func (s *AgentServiceImpl) UpdateHeartbeat(c *gin.Context, hostID uint, heartbeat *model.AgentHeartbeatDto) {
	if hostID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "主机ID不能为空")
		return
	}

	if err := s.agentDao.UpdateHeartbeat(hostID, heartbeat); err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "更新心跳失败")
		return
	}

	result.Success(c, map[string]interface{}{
		"message": "心跳更新成功",
		"hostId":  hostID,
	})
}

// UpdateHeartbeatByIP 通过IP更新心跳
func (s *AgentServiceImpl) UpdateHeartbeatByIP(c *gin.Context, heartbeat *model.AgentHeartbeatDto) {
	var host model.CmdbHost
	var err error

	if strings.TrimSpace(heartbeat.IP) != "" {
		heartbeat.IP = strings.TrimSpace(heartbeat.IP)
		host, err = s.hostDao.GetCmdbHostByIP(heartbeat.IP)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			result.FailedWithCode(c, int(result.ApiCode.FAILED), "查询主机失败")
			return
		}
	}

	if host.ID == 0 && strings.TrimSpace(heartbeat.Hostname) != "" {
		identifiers := []string{strings.TrimSpace(heartbeat.Hostname)}
		if idx := strings.IndexByte(identifiers[0], '.'); idx > 0 {
			identifiers = append(identifiers, identifiers[0][:idx])
		}

		for _, identifier := range identifiers {
			if identifier == "" {
				continue
			}

			hosts, listErr := s.hostDao.GetCmdbHostsByHostnameIdentifier(identifier)
			if listErr != nil {
				result.FailedWithCode(c, int(result.ApiCode.FAILED), "查询主机失败")
				return
			}
			if len(hosts) == 0 {
				continue
			}
			if len(hosts) > 1 {
				log.Printf("agent heartbeat hostname matched multiple hosts: hostname=%s identifier=%s ip=%s candidates=%s",
					heartbeat.Hostname, identifier, heartbeat.IP, formatHeartbeatHostCandidates(hosts))
			}

			host = chooseHeartbeatHost(hosts, heartbeat.IP)
			break
		}
	}

	if host.ID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "无法匹配到主机记录")
		return
	}

	// 更新心跳信息
	if err := s.agentDao.UpdateHeartbeatByIP(host.ID, heartbeat); err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "更新心跳失败")
		return
	}

	result.Success(c, map[string]interface{}{
		"message":  "心跳更新成功",
		"hostId":   host.ID,
		"hostName": host.Name,
		"hostIP":   heartbeat.IP,
	})
}

func chooseHeartbeatHost(hosts []model.CmdbHost, heartbeatIP string) model.CmdbHost {
	heartbeatIP = strings.TrimSpace(heartbeatIP)
	if heartbeatIP != "" {
		for _, host := range hosts {
			if host.PrivateIP == heartbeatIP || host.PublicIP == heartbeatIP || host.SSHIP == heartbeatIP {
				return host
			}
		}
	}
	return hosts[0]
}

func formatHeartbeatHostCandidates(hosts []model.CmdbHost) string {
	var parts []string
	for _, host := range hosts {
		parts = append(parts, fmt.Sprintf("id=%d,name=%s,host_name=%s,private_ip=%s,public_ip=%s,ssh_ip=%s",
			host.ID, host.Name, host.HostName, host.PrivateIP, host.PublicIP, host.SSHIP))
	}
	return strings.Join(parts, "; ")
}

// GetAgentStatistics 获取统计信息
func (s *AgentServiceImpl) GetAgentStatistics(c *gin.Context) {
	stats, err := s.agentDao.GetAgentStatistics()
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "获取统计信息失败")
		return
	}

	result.Success(c, stats)
}

// CheckOfflineAgents 检查离线agent
func (s *AgentServiceImpl) CheckOfflineAgents() {
	// 检查5分钟内没有心跳的Agent
	offlineAgents, err := s.agentDao.GetOfflineAgents(agentHeartbeatTimeout())
	if err != nil {
		log.Printf("检查离线Agent失败: %v", err)
		return
	}

	for _, agent := range offlineAgents {
		log.Printf("Agent %d (主机: %s) 离线", agent.ID, agent.HostName)
		// 更新状态为启动异常
		s.agentDao.UpdateStatus(agent.ID, model.AgentStatusStartError, "心跳超时")
	}
}

// cleanupPushgatewayMetrics 清理Pushgateway中的监控数据
func (s *AgentServiceImpl) cleanupPushgatewayMetrics(jobName string) {
	if jobName == "" {
		log.Printf("job名称为空，跳过Pushgateway清理")
		return
	}

	// 从配置中获取Pushgateway URL
	pushgatewayURL := "http://8.130.14.34:9091"
	if config.Config != nil && config.Config.Monitor.Pushgateway.URL != "" {
		pushgatewayURL = config.Config.Monitor.Pushgateway.URL
	}

	// 构建DELETE请求的URL
	// Pushgateway API: DELETE /metrics/job/<job_name>
	deleteURL := fmt.Sprintf("%s/metrics/job/%s", pushgatewayURL, jobName)

	log.Printf("开始清理Pushgateway监控数据: %s", deleteURL)

	// 创建HTTP DELETE请求
	req, err := http.NewRequest("DELETE", deleteURL, nil)
	if err != nil {
		log.Printf("创建DELETE请求失败: %v", err)
		return
	}

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("清理Pushgateway数据失败: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 202 || resp.StatusCode == 200 {
		log.Printf("成功清理Pushgateway数据: job=%s", jobName)
	} else {
		log.Printf("清理Pushgateway数据返回状态码: %d, job=%s", resp.StatusCode, jobName)
	}
}

// cleanupPushgatewayMetricsWithHostname 清理Pushgateway监控数据（带hostname分组标签）
// 已废弃，请使用 cleanupPushgatewayMetricsWithGroup
func (s *AgentServiceImpl) cleanupPushgatewayMetricsWithHostname(jobName, hostname string) {
	if jobName == "" {
		log.Printf("job名称为空，跳过Pushgateway清理")
		return
	}

	// 从配置中获取Pushgateway URL
	pushgatewayURL := "http://8.130.14.34:9091"
	if config.Config != nil && config.Config.Monitor.Pushgateway.URL != "" {
		pushgatewayURL = config.Config.Monitor.Pushgateway.URL
	}

	// 构建DELETE请求的URL，包含hostname分组标签
	// Pushgateway API: DELETE /metrics/job/<job_name>/hostname/<hostname_value>
	deleteURL := fmt.Sprintf("%s/metrics/job/%s/hostname/%s", pushgatewayURL, jobName, hostname)

	log.Printf("开始清理Pushgateway监控数据: %s (job=%s, hostname=%s)", deleteURL, jobName, hostname)

	// 创建HTTP DELETE请求
	req, err := http.NewRequest("DELETE", deleteURL, nil)
	if err != nil {
		log.Printf("创建DELETE请求失败: %v", err)
		return
	}

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("清理Pushgateway数据失败: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 202 || resp.StatusCode == 200 {
		log.Printf("成功清理Pushgateway数据: job=%s, hostname=%s", jobName, hostname)
	} else {
		log.Printf("清理Pushgateway数据返回状态码: %d, job=%s, hostname=%s", resp.StatusCode, jobName, hostname)
	}
}

// cleanupPushgatewayMetricsWithGroup 清理Pushgateway监控数据（包含完整的分组标签）
func (s *AgentServiceImpl) cleanupPushgatewayMetricsWithGroup(jobName, hostname string, groupID uint, groupName string) {
	if jobName == "" {
		log.Printf("job名称为空，跳过Pushgateway清理")
		return
	}

	// 从配置中获取Pushgateway URL
	pushgatewayURL := "http://8.130.14.34:9091"
	if config.Config != nil && config.Config.Monitor.Pushgateway.URL != "" {
		pushgatewayURL = config.Config.Monitor.Pushgateway.URL
	}

	// 构建DELETE请求的URL，包含所有分组标签（必须与推送时的标签完全一致）
	// Pushgateway API: DELETE /metrics/job/<job_name>/label1/<value1>/label2/<value2>...
	// 顺序必须与推送时一致: hostname -> group_id -> group_name
	deleteURL := fmt.Sprintf("%s/metrics/job/%s/hostname/%s/group_id/%d/group_name/%s",
		pushgatewayURL, jobName, hostname, groupID, groupName)

	log.Printf("开始清理Pushgateway监控数据: %s (job=%s, hostname=%s, group_id=%d, group_name=%s)",
		deleteURL, jobName, hostname, groupID, groupName)

	// 创建HTTP DELETE请求
	req, err := http.NewRequest("DELETE", deleteURL, nil)
	if err != nil {
		log.Printf("创建DELETE请求失败: %v", err)
		return
	}

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("清理Pushgateway数据失败: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 202 || resp.StatusCode == 200 {
		log.Printf("✅ 成功清理Pushgateway数据: job=%s, hostname=%s, group=%s(%d)", jobName, hostname, groupName, groupID)
	} else {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("⚠️ 清理Pushgateway数据返回状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}
}

func (s *AgentServiceImpl) cleanupPrometheusSeries(jobName, hostname string, groupID uint, groupName string) {
	prometheusURL := ""
	if config.Config != nil {
		prometheusURL = strings.TrimRight(strings.TrimSpace(config.Config.Monitor.Prometheus.URL), "/")
	}
	if prometheusURL == "" {
		log.Printf("Prometheus URL 未配置，跳过历史数据清理")
		return
	}
	if jobName == "" {
		log.Printf("job名称为空，跳过Prometheus历史数据清理")
		return
	}

	matchers := buildPrometheusDeleteMatchers(jobName, hostname, groupID, groupName)
	if len(matchers) == 0 {
		log.Printf("未构造出Prometheus删除条件，跳过历史数据清理")
		return
	}

	deleteURL := prometheusURL + "/api/v1/admin/tsdb/delete_series"
	values := neturl.Values{}
	for _, matcher := range matchers {
		values.Add("match[]", matcher)
	}

	req, err := http.NewRequest(http.MethodPost, deleteURL, strings.NewReader(values.Encode()))
	if err != nil {
		log.Printf("创建Prometheus delete_series请求失败: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Prometheus历史数据清理失败: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		log.Printf("Prometheus历史数据清理返回异常状态码: %d, 响应: %s", resp.StatusCode, string(body))
		return
	}

	log.Printf("Prometheus历史数据清理请求已提交: job=%s, hostname=%s, group=%s(%d), matchers=%d",
		jobName, hostname, groupName, groupID, len(matchers))

	cleanReq, err := http.NewRequest(http.MethodPost, prometheusURL+"/api/v1/admin/tsdb/clean_tombstones", nil)
	if err != nil {
		log.Printf("创建Prometheus clean_tombstones请求失败: %v", err)
		return
	}

	cleanResp, err := client.Do(cleanReq)
	if err != nil {
		log.Printf("Prometheus clean_tombstones失败: %v", err)
		return
	}
	defer cleanResp.Body.Close()

	cleanBody, _ := io.ReadAll(cleanResp.Body)
	if cleanResp.StatusCode/100 != 2 {
		log.Printf("Prometheus clean_tombstones返回异常状态码: %d, 响应: %s", cleanResp.StatusCode, string(cleanBody))
		return
	}

	log.Printf("Prometheus tombstones清理已完成: job=%s, hostname=%s, group=%s(%d)", jobName, hostname, groupName, groupID)
}

func buildPrometheusDeleteMatchers(jobName, hostname string, groupID uint, groupName string) []string {
	labelParts := []string{fmt.Sprintf(`job=%q`, jobName)}
	if hostname != "" {
		labelParts = append(labelParts, fmt.Sprintf(`hostname=%q`, hostname))
	}
	if groupID > 0 {
		labelParts = append(labelParts, fmt.Sprintf(`group_id=%q`, strconv.FormatUint(uint64(groupID), 10)))
	}
	if groupName != "" {
		labelParts = append(labelParts, fmt.Sprintf(`group_name=%q`, groupName))
	}

	return []string{
		"{" + strings.Join(labelParts, ",") + "}",
		fmt.Sprintf(`{instance=%q}`, jobName),
		fmt.Sprintf(`{host=%q}`, jobName),
	}
}

// DeleteAgent 删除agent数据(通过hostID)
func (s *AgentServiceImpl) DeleteAgent(c *gin.Context, hostID uint) {
	// 参数校验
	if hostID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "主机ID不能为空")
		return
	}

	// 检查Agent是否存在
	agent, err := s.agentDao.GetByHostID(hostID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			result.FailedWithCode(c, int(result.ApiCode.FAILED), "Agent记录不存在")
			return
		}
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "获取Agent信息失败")
		return
	}

	// 获取主机信息以便清理Pushgateway
	host, err := s.hostDao.GetCmdbHostById(hostID)
	if err != nil {
		log.Printf("获取主机信息失败: %v, 跳过Pushgateway清理", err)
	} else {
		// 清理Pushgateway中的监控数据
		s.cleanupPushgatewayMetrics(host.Name)
	}

	// 直接删除数据库中的Agent记录
	if err := s.agentDao.DeleteByHostID(hostID); err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "删除Agent记录失败")
		return
	}

	result.Success(c, map[string]interface{}{
		"message":  "Agent数据删除成功",
		"hostId":   hostID,
		"hostName": agent.HostName,
	})
}

// DeleteAgentByID 删除agent数据(通过agentID)
func (s *AgentServiceImpl) DeleteAgentByID(c *gin.Context, agentID uint) {
	// 参数校验
	if agentID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "Agent ID不能为空")
		return
	}

	// 检查Agent是否存在
	agent, err := s.agentDao.GetByID(agentID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			result.FailedWithCode(c, int(result.ApiCode.FAILED), "Agent记录不存在")
			return
		}
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "获取Agent信息失败")
		return
	}

	// 直接删除数据库中的Agent记录
	if err := s.agentDao.Delete(agentID); err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "删除Agent记录失败")
		return
	}

	result.Success(c, map[string]interface{}{
		"message":  "Agent数据删除成功",
		"agentId":  agentID,
		"hostId":   agent.HostID,
		"hostName": agent.HostName,
	})
}

// getSSHKeyByID 根据ID获取SSH密钥
func (s *AgentServiceImpl) getSSHKeyByID(id uint) (*EcsAuth, error) {
	ecsAuth, err := s.ecsAuthDao.GetEcsAuthById(id)
	if err != nil {
		return nil, fmt.Errorf("获取SSH密钥失败: %v", err)
	}
	return &ecsAuth, nil
}

// batchDeployAgentAsync 批量部署agent - 优化版本：编译一次，并行拷贝启动
func (s *AgentServiceImpl) batchDeployAgentAsync(hosts []model.CmdbHost, version string) error {
	return s.batchDeployAgentAsyncWithOptions(hosts, version, model.AgentDeployOptions{})
}

func (s *AgentServiceImpl) batchDeployAgentAsyncWithOptions(hosts []model.CmdbHost, version string, options model.AgentDeployOptions) error {
	if strings.EqualFold(strings.TrimSpace(options.DeployMode), "offline") {
		if len(hosts) == 0 {
			return fmt.Errorf("主机列表为空")
		}
		type offlineDeployResult struct {
			hostID uint
			err    error
		}
		results := make(chan offlineDeployResult, len(hosts))
		for _, host := range hosts {
			go func(h model.CmdbHost) {
				results <- offlineDeployResult{hostID: h.ID, err: s.deployOfflineAgentAsync(h.ID, h, version, options)}
			}(host)
		}

		var errs []string
		for range hosts {
			deployment := <-results
			if deployment.err == nil {
				continue
			}
			s.updateDeployError(deployment.hostID, deployment.err.Error())
			errs = append(errs, fmt.Sprintf("主机 %d: %v", deployment.hostID, deployment.err))
		}
		if len(errs) > 0 {
			return fmt.Errorf("离线部署部分失败: %s", strings.Join(errs, "; "))
		}
		return nil
	}

	if len(hosts) == 0 {
		return fmt.Errorf("主机列表为空")
	}
	version = strings.TrimSpace(version)
	if version == "" {
		version = configuredAgentVersion()
	}

	log.Printf("开始批量部署Agent到 %d 台主机", len(hosts))

	// 1. 按分组和操作系统分类主机
	// key: "groupID_groupName_os"
	hostGroups := make(map[string][]model.CmdbHost)
	for _, host := range hosts {
		targetOS := "linux"
		if strings.Contains(strings.ToLower(host.OS), "windows") {
			targetOS = "windows"
		} else if strings.Contains(strings.ToLower(host.OS), "darwin") {
			targetOS = "darwin"
		}

		// 使用 groupID_groupName_os 作为键
		groupKey := fmt.Sprintf("%d_%s_%s", host.GroupID, host.Group.Name, targetOS)
		hostGroups[groupKey] = append(hostGroups[groupKey], host)
	}

	log.Printf("主机按分组和OS分类结果: 共 %d 个不同的组合", len(hostGroups))
	for key, groupHosts := range hostGroups {
		log.Printf("  - %s: %d 台主机", key, len(groupHosts))
	}

	// 2. 对每个分组+OS组合，编译并部署一次
	type groupDeployResult struct {
		groupName string
		hosts     []model.CmdbHost
		err       error
	}

	var wg sync.WaitGroup
	resultsChan := make(chan groupDeployResult, len(hostGroups))

	for groupKey, groupHosts := range hostGroups {
		wg.Add(1)
		go func(key string, hosts []model.CmdbHost) {
			defer wg.Done()

			// 解析key获取groupID、groupName和targetOS
			parts := strings.SplitN(key, "_", 3)
			if len(parts) != 3 {
				resultsChan <- groupDeployResult{
					groupName: key,
					hosts:     hosts,
					err:       fmt.Errorf("无效的分组键: %s", key),
				}
				return
			}

			groupIDStr := parts[0]
			groupName := parts[1]
			targetOS := parts[2]

			groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
			if err != nil {
				resultsChan <- groupDeployResult{
					groupName: groupName,
					hosts:     hosts,
					err:       fmt.Errorf("解析分组ID失败: %v", err),
				}
				return
			}

			log.Printf("开始为分组[%s]的 %d 台主机编译和部署Agent (OS: %s)", groupName, len(hosts), targetOS)

			if err := s.batchDeployByGroup(hosts, uint(groupID), groupName, targetOS, version); err != nil {
				resultsChan <- groupDeployResult{
					groupName: groupName,
					hosts:     hosts,
					err:       fmt.Errorf("分组[%s]部署失败: %v", groupName, err),
				}
				return
			}

			resultsChan <- groupDeployResult{
				groupName: groupName,
				hosts:     hosts,
			}
		}(groupKey, groupHosts)
	}

	wg.Wait()
	close(resultsChan)

	// 收集错误
	var errors []string
	for groupResult := range resultsChan {
		if groupResult.err == nil {
			continue
		}
		errors = append(errors, groupResult.err.Error())
		for _, host := range groupResult.hosts {
			s.updateDeployError(host.ID, groupResult.err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("部分分组部署失败: %s", strings.Join(errors, "; "))
	}

	log.Printf("所有分组的Agent部署完成")
	return nil
}

func agentOfflineUploadDir() string {
	dir := "./upload"
	if config.Config != nil && strings.TrimSpace(config.Config.ImageSettings.UploadDir) != "" {
		dir = strings.TrimSpace(config.Config.ImageSettings.UploadDir)
	}
	abs, err := filepath.Abs(dir)
	if err == nil {
		return abs
	}
	return dir
}

const maxOfflineAgentBinarySize int64 = 512 * 1024 * 1024

func offlineAssetArch(name string) string {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "arm64") || strings.Contains(lower, "aarch64") {
		return "arm64"
	}
	if strings.Contains(lower, "x86") || strings.Contains(lower, "amd64") {
		return "x86_64"
	}
	return ""
}

func offlineAssetArchForFile(filePath, name string) string {
	if file, err := elf.Open(filePath); err == nil {
		defer file.Close()
		switch file.Machine {
		case elf.EM_X86_64:
			return "x86_64"
		case elf.EM_AARCH64:
			return "arm64"
		}
	}
	return offlineAssetArch(name)
}

// UploadOfflineAgentBinary stores a pre-built Agent binary under upload/agent.
func (s *AgentServiceImpl) UploadOfflineAgentBinary(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxOfflineAgentBinarySize+1)
	file, err := c.FormFile("file")
	if err != nil {
		result.Failed(c, http.StatusBadRequest, "请上传Agent二进制文件")
		return
	}
	if file.Size <= 0 {
		result.Failed(c, http.StatusBadRequest, "Agent二进制文件不能为空")
		return
	}
	if file.Size > maxOfflineAgentBinarySize {
		result.Failed(c, http.StatusBadRequest, "Agent二进制文件不能超过512MB")
		return
	}

	originalName := strings.TrimSpace(file.Filename)
	name := filepath.Base(originalName)
	if originalName == "" || name == "." || name == ".." || name != originalName || strings.ContainsAny(originalName, `/\\`) {
		result.Failed(c, http.StatusBadRequest, "文件名不合法")
		return
	}
	lowerName := strings.ToLower(name)
	if strings.HasSuffix(lowerName, ".json") || strings.HasSuffix(lowerName, ".yaml") || strings.HasSuffix(lowerName, ".yml") {
		result.Failed(c, http.StatusBadRequest, "仅支持上传Agent二进制文件，配置由后端自动生成")
		return
	}

	dir := filepath.Join(agentOfflineUploadDir(), "agent")
	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Failed(c, http.StatusInternalServerError, "创建离线Agent目录失败: "+err.Error())
		return
	}
	target := filepath.Join(dir, name)
	if err := c.SaveUploadedFile(file, target); err != nil {
		result.Failed(c, http.StatusInternalServerError, "保存离线Agent二进制失败: "+err.Error())
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "读取已上传Agent二进制失败: "+err.Error())
		return
	}
	result.Success(c, model.AgentOfflineAssetVO{
		Name: name,
		Path: filepath.ToSlash(filepath.Join("agent", name)),
		Size: info.Size(),
		Kind: "binary",
		Arch: offlineAssetArchForFile(target, name),
	})
}

func resolveAgentOfflineAsset(rawPath string) (string, error) {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return "", fmt.Errorf("离线制品路径不能为空")
	}
	base, err := filepath.Abs(agentOfflineUploadDir())
	if err != nil {
		return "", fmt.Errorf("解析离线制品目录失败: %v", err)
	}
	allowedBase := filepath.Join(base, "agent")
	candidate := rawPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(base, candidate)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("解析离线制品路径失败: %v", err)
	}
	rel, err := filepath.Rel(allowedBase, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("离线制品必须位于 upload/agent 目录内")
	}
	// Resolve symlinks as well so an upload/agent/ link cannot escape the allowed directory.
	baseReal, err := filepath.EvalSymlinks(allowedBase)
	if err != nil {
		return "", fmt.Errorf("解析离线Agent目录失败: %v", err)
	}
	candidateReal, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("离线制品不存在: %v", err)
	}
	realRel, err := filepath.Rel(baseReal, candidateReal)
	if err != nil || realRel == ".." || strings.HasPrefix(realRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("离线制品必须位于 upload/agent 目录内")
	}
	info, err := os.Stat(candidate)
	if err != nil {
		return "", fmt.Errorf("离线制品不存在: %v", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("离线制品不能是目录")
	}
	return candidate, nil
}

func validateAgentDeployOptions(options model.AgentDeployOptions) error {
	mode := strings.ToLower(strings.TrimSpace(options.DeployMode))
	if mode == "" || mode == "online" {
		return nil
	}
	if mode != "offline" {
		return fmt.Errorf("不支持的Agent部署模式: %s", options.DeployMode)
	}
	if strings.TrimSpace(options.BinaryPath) == "" &&
		strings.TrimSpace(options.BinaryX86Path) == "" &&
		strings.TrimSpace(options.BinaryArm64Path) == "" {
		return fmt.Errorf("离线部署必须提供Agent二进制文件")
	}
	if strings.TrimSpace(options.BinaryPath) == "" &&
		(strings.TrimSpace(options.BinaryX86Path) == "" || strings.TrimSpace(options.BinaryArm64Path) == "") {
		return fmt.Errorf("离线部署必须同时提供x86_64和arm64二进制文件")
	}
	for _, binaryPath := range []string{options.BinaryPath, options.BinaryX86Path, options.BinaryArm64Path} {
		if strings.TrimSpace(binaryPath) == "" {
			continue
		}
		if _, err := resolveAgentOfflineAsset(binaryPath); err != nil {
			return err
		}
	}
	return nil
}

func collectOfflineAgentAssets() ([]model.AgentOfflineAssetVO, error) {
	uploadBase := agentOfflineUploadDir()
	base := filepath.Join(uploadBase, "agent")
	assets := make([]model.AgentOfflineAssetVO, 0)
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return nil
		}
		rel = filepath.Join("agent", rel)
		name := entry.Name()
		kind := "binary"
		lowerName := strings.ToLower(name)
		if strings.HasSuffix(lowerName, ".json") || strings.HasSuffix(lowerName, ".yaml") || strings.HasSuffix(lowerName, ".yml") {
			kind = "config"
		}
		arch := offlineAssetArchForFile(path, name)
		assets = append(assets, model.AgentOfflineAssetVO{Name: name, Path: filepath.ToSlash(rel), Size: info.Size(), Kind: kind, Arch: arch})
		return nil
	})
	if os.IsNotExist(err) {
		return assets, nil
	}
	if err != nil {
		return nil, err
	}
	return assets, nil
}

func (s *AgentServiceImpl) GetOfflineAssets(c *gin.Context) {
	assets, err := collectOfflineAgentAssets()
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "读取离线Agent制品失败")
		return
	}
	result.Success(c, map[string]interface{}{"list": assets})
}

// CheckOfflineAssets reports whether both architecture packages are available.
func (s *AgentServiceImpl) CheckOfflineAssets(c *gin.Context) {
	assets, err := collectOfflineAgentAssets()
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "读取离线Agent制品失败")
		return
	}
	x86Candidates := make([]model.AgentOfflineAssetVO, 0)
	arm64Candidates := make([]model.AgentOfflineAssetVO, 0)
	for _, asset := range assets {
		if asset.Kind != "binary" {
			continue
		}
		switch asset.Arch {
		case "x86_64":
			x86Candidates = append(x86Candidates, asset)
		case "arm64":
			arm64Candidates = append(arm64Candidates, asset)
		}
	}
	buildSlot := func(candidates []model.AgentOfflineAssetVO) model.AgentOfflineAssetSlotVO {
		slot := model.AgentOfflineAssetSlotVO{Candidates: candidates}
		if len(candidates) > 0 {
			slot.Uploaded = true
			asset := candidates[0]
			slot.Asset = &asset
		}
		return slot
	}
	x86Slot := buildSlot(x86Candidates)
	arm64Slot := buildSlot(arm64Candidates)
	ready := x86Slot.Uploaded && arm64Slot.Uploaded
	message := "x86_64 和 arm64 离线 Agent 包均已上传"
	if !ready {
		missing := make([]string, 0, 2)
		if !x86Slot.Uploaded {
			missing = append(missing, "x86_64")
		}
		if !arm64Slot.Uploaded {
			missing = append(missing, "arm64")
		}
		message = "缺少离线 Agent 包: " + strings.Join(missing, "、")
	}
	result.Success(c, model.AgentOfflineAssetCheckVO{
		Ready:   ready,
		X86_64:  x86Slot,
		Arm64:   arm64Slot,
		Message: message,
	})
}

func resolveAgentOfflineBinary(options model.AgentDeployOptions, arch string) (string, error) {
	pathValue := strings.TrimSpace(options.BinaryPath)
	if pathValue == "" {
		switch arch {
		case "arm64":
			pathValue = options.BinaryArm64Path
		default:
			pathValue = options.BinaryX86Path
		}
	}
	return resolveAgentOfflineAsset(pathValue)
}

func (s *AgentServiceImpl) offlineSSHConfig(host model.CmdbHost) (util.SSHConfig, error) {
	if host.SSHKeyID == 0 {
		return util.SSHConfig{}, fmt.Errorf("主机未配置SSH认证信息")
	}
	sshKey, err := s.getSSHKeyByID(host.SSHKeyID)
	if err != nil {
		return util.SSHConfig{}, err
	}
	cfg := util.SSHConfig{IP: host.SSHIP, Port: host.SSHPort, Username: host.SSHName, Type: sshKey.Type, Timeout: 30 * time.Second}
	switch sshKey.Type {
	case 1:
		cfg.Password = sshKey.Password
	case 2:
		cfg.PublicKey = sshKey.PublicKey
	case 3:
	case 4:
		cfg.BastionHost, cfg.BastionPort, cfg.BastionUsername = sshKey.BastionHost, sshKey.BastionPort, sshKey.BastionUsername
		cfg.BastionAuthType, cfg.BastionPassword, cfg.BastionPrivateKey = sshKey.BastionAuthType, sshKey.BastionPassword, sshKey.BastionPrivateKey
		cfg.TargetAuthType, cfg.TargetPassword, cfg.TargetPrivateKey = sshKey.TargetAuthType, sshKey.TargetPassword, sshKey.TargetPrivateKey
	default:
		return util.SSHConfig{}, fmt.Errorf("不支持的SSH认证类型: %d", sshKey.Type)
	}
	return cfg, nil
}

func (s *AgentServiceImpl) detectOfflineHostArch(host model.CmdbHost) (string, error) {
	if s.detectPlatform(host.OS) != "linux" {
		return "", fmt.Errorf("离线Agent当前仅支持Linux主机")
	}
	sshConfig, err := s.offlineSSHConfig(host)
	if err != nil {
		return "", fmt.Errorf("获取SSH认证信息失败: %v", err)
	}
	archOutput, err := ExecuteSSHCommandWithOutput(sshConfig, "uname -m")
	if err != nil {
		return "", fmt.Errorf("检测目标主机CPU架构失败: %v", err)
	}
	arch := strings.ToLower(strings.TrimSpace(archOutput))
	if strings.Contains(arch, "aarch64") || strings.Contains(arch, "arm64") {
		return "arm64", nil
	}
	if strings.Contains(arch, "x86_64") || strings.Contains(arch, "amd64") || strings.Contains(arch, "x86") {
		return "x86_64", nil
	}
	return "", fmt.Errorf("不支持的目标CPU架构: %s", archOutput)
}

func (s *AgentServiceImpl) precheckOfflineHost(host model.CmdbHost, options model.AgentDeployOptions) (string, string, error) {
	arch, err := s.detectOfflineHostArch(host)
	if err != nil {
		return "", "", err
	}
	binaryPath, err := resolveAgentOfflineBinary(options, arch)
	if err != nil {
		return arch, "", fmt.Errorf("目标主机架构为%s，但未找到对应二进制: %v", arch, err)
	}
	return arch, binaryPath, nil
}

// PrecheckOfflineDeployment verifies uploaded packages and target CPU architectures.
func (s *AgentServiceImpl) PrecheckOfflineDeployment(c *gin.Context, dto *model.AgentOfflinePrecheckDto) {
	options := model.AgentDeployOptions{
		DeployMode:      "offline",
		BinaryPath:      dto.BinaryPath,
		BinaryX86Path:   dto.BinaryX86Path,
		BinaryArm64Path: dto.BinaryArm64Path,
	}
	if err := validateAgentDeployOptions(options); err != nil {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), err.Error())
		return
	}
	check := model.AgentOfflinePrecheckVO{Ready: true, Hosts: make([]model.AgentOfflineHostPrecheckVO, 0, len(dto.HostIDs))}
	for _, hostID := range dto.HostIDs {
		hostResult := model.AgentOfflineHostPrecheckVO{HostID: hostID, Ready: false}
		host, err := s.hostDao.GetCmdbHostById(hostID)
		if err != nil {
			hostResult.Error = "获取主机信息失败"
			check.Ready = false
			check.Hosts = append(check.Hosts, hostResult)
			continue
		}
		hostResult.HostName = host.Name
		arch, binaryPath, err := s.precheckOfflineHost(host, options)
		hostResult.Arch = arch
		hostResult.BinaryPath = strings.TrimSpace(binaryPath)
		if err != nil {
			hostResult.Error = err.Error()
			check.Ready = false
		} else {
			hostResult.Ready = true
		}
		check.Hosts = append(check.Hosts, hostResult)
	}
	if len(check.Hosts) == 0 {
		check.Ready = false
		check.Error = "主机ID列表不能为空"
	} else if !check.Ready {
		check.Error = "存在未通过离线部署检查的主机"
	}
	result.Success(c, check)
}

type offlineAgentConfig struct {
	Version     string                  `json:"version"`
	VersionTag  string                  `json:"version_tag"`
	Pushgateway offlineAgentPushgateway `json:"pushgateway"`
	Heartbeat   offlineAgentHeartbeat   `json:"heartbeat"`
	Collect     string                  `json:"collect_interval"`
	Group       offlineAgentGroup       `json:"group"`
}

type offlineAgentPushgateway struct {
	URL          string `json:"url"`
	PushInterval string `json:"push_interval"`
	Job          string `json:"job,omitempty"`
	Instance     string `json:"instance,omitempty"`
}

type offlineAgentHeartbeat struct {
	ServerURL string `json:"server_url"`
	Interval  string `json:"interval"`
	Token     string `json:"token"`
}

type offlineAgentGroup struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// generateOfflineAgentConfig creates the host-specific runtime configuration.
// The selected host's CMDB group is embedded so every offline deployment keeps
// the same labels as an online build while using the current global endpoints.
func generateOfflineAgentConfig(host model.CmdbHost, version string) ([]byte, error) {
	groupName := strings.TrimSpace(host.Group.Name)
	request := buildAgentRequest("linux", version, host.GroupID, groupName)
	job := strings.TrimSpace(host.PrivateIP)
	if job == "" {
		job = strings.TrimSpace(host.SSHIP)
	}
	instance := strings.TrimSpace(host.HostName)
	if instance == "" {
		instance = strings.TrimSpace(host.Name)
	}
	data := offlineAgentConfig{
		Version:    request.Version,
		VersionTag: request.VersionTag,
		Pushgateway: offlineAgentPushgateway{
			URL:          request.PushgatewayURL,
			PushInterval: "60s",
			Job:          job,
			Instance:     instance,
		},
		Heartbeat: offlineAgentHeartbeat{
			ServerURL: request.HeartbeatURL,
			Interval:  "30s",
			Token:     request.HeartbeatToken,
		},
		Collect: "30s",
		Group: offlineAgentGroup{
			ID:   host.GroupID,
			Name: groupName,
		},
	}
	return json.MarshalIndent(data, "", "  ")
}

func (s *AgentServiceImpl) deployOfflineAgentAsync(hostID uint, host model.CmdbHost, version string, options model.AgentDeployOptions) error {
	_, binaryPath, err := s.precheckOfflineHost(host, options)
	if err != nil {
		return err
	}
	configData, err := generateOfflineAgentConfig(host, version)
	if err != nil {
		return fmt.Errorf("生成Agent配置文件失败: %v", err)
	}
	tempConfig, err := os.CreateTemp("", "dodevops-agent-config-*.json")
	if err != nil {
		return fmt.Errorf("创建Agent临时配置文件失败: %v", err)
	}
	tempConfigPath := tempConfig.Name()
	defer os.Remove(tempConfigPath)
	if err := tempConfig.Chmod(0600); err != nil {
		tempConfig.Close()
		return fmt.Errorf("设置Agent临时配置文件权限失败: %v", err)
	}
	if _, err := tempConfig.Write(configData); err != nil {
		tempConfig.Close()
		return fmt.Errorf("写入Agent临时配置文件失败: %v", err)
	}
	if err := tempConfig.Close(); err != nil {
		return fmt.Errorf("关闭Agent临时配置文件失败: %v", err)
	}
	if err := s.deployToSingleHost(host, binaryPath, "dodevops-agent", "linux", tempConfigPath); err != nil {
		return err
	}
	return nil
}

// batchDeployByGroup 为同一分组的主机批量部署（它们使用相同的group标签）
func (s *AgentServiceImpl) batchDeployByGroup(hosts []model.CmdbHost, groupID uint, groupName string, targetOS string, version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		version = configuredAgentVersion()
	}
	version = strings.TrimSpace(version)
	if version == "" {
		version = configuredAgentVersion()
	}

	// 更新所有主机进度：开始编译
	for _, host := range hosts {
		s.agentDao.UpdateInstallProgress(host.ID, model.InstallProgressCompiling)
	}

	log.Printf("开始为批量部署准备Agent二进制，目标OS: %s", targetOS)
	artifact, err := agentBuildCache.ensureBinary(buildAgentRequest(targetOS, version, groupID, groupName))
	if err != nil {
		return err
	}
	log.Printf("批量部署使用Agent二进制: %s", artifact.BinaryPath)

	// 更新所有主机进度：编译完成
	for _, host := range hosts {
		s.agentDao.UpdateInstallProgress(host.ID, model.InstallProgressCompiled)
	}

	// 3. 并行拷贝和启动到所有主机（错开启动避免SSH并发连接限制）
	type deployResult struct {
		hostID uint
		err    error
	}

	resultChan := make(chan deployResult, len(hosts))

	for i, host := range hosts {
		go func(h model.CmdbHost, index int) {
			// 错开启动：每个部署延迟 index * 800ms，避免同时建立SSH连接
			if index > 0 {
				delay := time.Duration(index) * 800 * time.Millisecond
				log.Printf("主机 %d: 延迟 %v 后开始部署（避免并发连接冲突）", h.ID, delay)
				time.Sleep(delay)
			}
			err := s.deployToSingleHost(h, artifact.BinaryPath, artifact.BinaryName, targetOS, "")
			resultChan <- deployResult{hostID: h.ID, err: err}
		}(host, i)
	}

	// 收集结果
	successCount := 0
	failedCount := 0
	for i := 0; i < len(hosts); i++ {
		result := <-resultChan
		if result.err != nil {
			failedCount++
			log.Printf("部署到主机 %d 失败: %v", result.hostID, result.err)
			s.agentDao.UpdateByHostID(result.hostID, map[string]interface{}{
				"status":           model.AgentStatusDeployFailed,
				"install_progress": model.InstallProgressFailed,
				"error_msg":        result.err.Error(),
			})
		} else {
			successCount++
			log.Printf("部署到主机 %d 成功", result.hostID)
		}
	}

	log.Printf("批量部署完成: 成功 %d 台, 失败 %d 台", successCount, failedCount)
	return nil
}

// deployToSingleHost 部署已编译的agent到单个主机（拷贝+启动）
func (s *AgentServiceImpl) deployToSingleHost(host model.CmdbHost, binaryPath, binaryName, targetOS, localConfigPath string) error {
	log.Printf("开始部署到主机 %d (%s)", host.ID, host.HostName)

	// 更新进度：传输中
	s.agentDao.UpdateInstallProgress(host.ID, model.InstallProgressTransfer)

	// 获取SSH配置
	var sshConfig util.SSHConfig
	if host.SSHKeyID > 0 {
		sshKey, err := s.getSSHKeyByID(host.SSHKeyID)
		if err != nil {
			return fmt.Errorf("获取SSH认证信息失败: %v", err)
		}

		sshConfig = util.SSHConfig{
			IP:       host.SSHIP,
			Port:     host.SSHPort,
			Username: host.SSHName,
			Type:     sshKey.Type,
			Timeout:  30 * time.Second,
		}

		switch sshKey.Type {
		case 1:
			sshConfig.Password = sshKey.Password
		case 2:
			sshConfig.PublicKey = sshKey.PublicKey
		case 3:
			// 公钥免认证
		case 4:
			// 跳板机认证
			sshConfig.BastionHost = sshKey.BastionHost
			sshConfig.BastionPort = sshKey.BastionPort
			sshConfig.BastionUsername = sshKey.BastionUsername
			sshConfig.BastionAuthType = sshKey.BastionAuthType
			sshConfig.BastionPassword = sshKey.BastionPassword
			sshConfig.BastionPrivateKey = sshKey.BastionPrivateKey
			sshConfig.TargetAuthType = sshKey.TargetAuthType
			sshConfig.TargetPassword = sshKey.TargetPassword
			sshConfig.TargetPrivateKey = sshKey.TargetPrivateKey
		default:
			return fmt.Errorf("不支持的SSH认证类型: %d", sshKey.Type)
		}
	} else {
		return fmt.Errorf("主机未配置SSH认证信息")
	}

	// 【关键修复】使用主机IP锁，防止并发部署同一台物理机时的进程冲突
	// 如果多个主机记录共享同一个IP，确保它们的部署操作是串行的
	hostLockManager.Lock(host.SSHIP)
	defer hostLockManager.Unlock(host.SSHIP)

	// 创建远程目录
	remotePath := "/opt/agent"
	if targetOS == "windows" {
		remotePath = "C:\\dodevops-agent"
	}

	createDirCmd := fmt.Sprintf("mkdir -p %s", remotePath)
	if targetOS == "windows" {
		createDirCmd = fmt.Sprintf("mkdir %s", remotePath)
	}

	if err := ExecuteSSHCommand(sshConfig, createDirCmd); err != nil {
		return fmt.Errorf("创建远程目录失败: %v", err)
	}

	remoteBinaryPath := joinRemotePath(targetOS, remotePath, binaryName)
	if targetOS != "windows" {
		if err := stopAndArchiveExistingAgent(sshConfig, remoteBinaryPath); err != nil {
			return fmt.Errorf("备份旧Agent文件失败: %v", err)
		}
	}

	// 拷贝二进制文件
	if err := CopyFileViaSSH(sshConfig, binaryPath, remoteBinaryPath); err != nil {
		return fmt.Errorf("拷贝文件失败: %v", err)
	}
	remoteConfigPath := ""
	if strings.TrimSpace(localConfigPath) != "" {
		remoteConfigPath = joinRemotePath(targetOS, remotePath, "config.json")
		if err := CopyFileViaSSH(sshConfig, localConfigPath, remoteConfigPath); err != nil {
			return fmt.Errorf("拷贝Agent配置文件失败: %v", err)
		}
		if targetOS != "windows" {
			if err := ExecuteSSHCommand(sshConfig, fmt.Sprintf("%schmod 0600 %s", sudoPrefix(sshConfig), shellQuote(remoteConfigPath))); err != nil {
				return fmt.Errorf("设置Agent配置文件权限失败: %v", err)
			}
		}
	}

	log.Printf("主机 %d: 文件拷贝成功", host.ID)

	// 更新进度：传输完成
	s.agentDao.UpdateInstallProgress(host.ID, model.InstallProgressTransferred)

	// 设置权限和systemd服务
	if targetOS != "windows" {
		chmodCmd := fmt.Sprintf("chmod +x %s", remoteBinaryPath)
		if err := ExecuteSSHCommand(sshConfig, chmodCmd); err != nil {
			return fmt.Errorf("设置执行权限失败: %v", err)
		}

		privilegePrefix := sudoPrefix(sshConfig)
		if err := ExecuteSSHCommand(sshConfig, privilegePrefix+"touch /var/log/agent.log && "+privilegePrefix+"chmod 0644 /var/log/agent.log"); err != nil {
			return fmt.Errorf("初始化agent日志文件失败: %v", err)
		}

		if err := archiveExistingAgentService(sshConfig); err != nil {
			return fmt.Errorf("备份旧systemd服务文件失败: %v", err)
		}

		// 创建systemd服务
		execStart := "/bin/sh -c \"exec /opt/agent/dodevops-agent >> /var/log/agent.log 2>&1\""
		if remoteConfigPath != "" {
			execStart = "/bin/sh -c \"exec /opt/agent/dodevops-agent --config /opt/agent/config.json >> /var/log/agent.log 2>&1\""
		}
		serviceContent := fmt.Sprintf(`[Unit]
Description=DevOps Monitoring Agent
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=always
RestartSec=5
User=root
WorkingDirectory=/opt/agent
StandardOutput=null
StandardError=null

[Install]
WantedBy=multi-user.target`, execStart)

		createServiceCmd := fmt.Sprintf("%sbash -c 'cat > /etc/systemd/system/agent.service << \"EOF\"\n%s\nEOF'", privilegePrefix, serviceContent)
		if err := ExecuteSSHCommand(sshConfig, createServiceCmd); err != nil {
			return agentSystemdStepError(sshConfig, "创建systemd服务文件失败", err)
		}

		if err := ExecuteSSHCommand(sshConfig, privilegePrefix+"systemctl daemon-reload"); err != nil {
			return agentSystemdStepError(sshConfig, "重新加载systemd配置失败", err)
		}
		if err := ExecuteSSHCommand(sshConfig, privilegePrefix+"systemctl enable agent.service"); err != nil {
			return agentSystemdStepError(sshConfig, "启用agent服务失败", err)
		}
	}

	// 更新进度：配置完成
	s.agentDao.UpdateInstallProgress(host.ID, model.InstallProgressConfigured)

	// 启动服务
	log.Printf("主机 %d (%s): 启动agent服务", host.ID, host.SSHIP)
	startCmd := s.getStartAgentCommand(targetOS, remoteBinaryPath, sshConfig)
	if remoteConfigPath != "" && targetOS == "windows" {
		startCmd = fmt.Sprintf("start /B %s --config %s", shellQuote(remoteBinaryPath), shellQuote(remoteConfigPath))
	}
	if err := ExecuteSSHCommand(sshConfig, startCmd); err != nil {
		return agentSystemdStepError(sshConfig, "启动agent服务失败", err)
	}

	// 验证启动
	time.Sleep(5 * time.Second)
	checkProcessCmd := "ps aux | grep dodevops-agent | grep -v grep"
	if err := ExecuteSSHCommand(sshConfig, checkProcessCmd); err != nil {
		return fmt.Errorf("服务启动验证失败: 进程未运行")
	}

	// 更新状态为运行中
	s.agentDao.UpdateByHostID(host.ID, map[string]interface{}{
		"status":           model.AgentStatusRunning,
		"install_progress": model.InstallProgressStarted,
		"error_msg":        "",
		"update_time":      time.Now(),
	})

	log.Printf("主机 %d: Agent部署成功", host.ID)
	return nil
}

func agentSystemdStepError(sshConfig util.SSHConfig, step string, cause error) error {
	diagnostics := collectAgentSystemdDiagnostics(sshConfig)
	if diagnostics == "" {
		return fmt.Errorf("%s: %v", step, cause)
	}
	return fmt.Errorf("%s: %v\n远端诊断:\n%s", step, cause, diagnostics)
}

func stopAndArchiveExistingAgent(sshConfig util.SSHConfig, remoteBinaryPath string) error {
	log.Printf("停止并备份远端旧Agent文件: %s", remoteBinaryPath)
	privilegePrefix := sudoPrefix(sshConfig)
	stopCommands := []string{
		privilegePrefix + "systemctl stop agent.service 2>/dev/null || true",
		privilegePrefix + "pkill -TERM -f dodevops-agent 2>/dev/null || true",
	}
	for _, cmd := range stopCommands {
		if err := ExecuteSSHCommand(sshConfig, cmd); err != nil {
			log.Printf("停止旧Agent命令失败(忽略): %s -> %v", cmd, err)
		}
	}
	time.Sleep(2 * time.Second)

	archiveCmd := fmt.Sprintf("backup_dir=/tmp/dodevops-agent-backup-$(date +%%Y%%m%%d%%H%%M%%S); if [ -e %[1]s ]; then mkdir -p \"$backup_dir\" && %[2]smv %[1]s \"$backup_dir/\"; fi", shellQuote(remoteBinaryPath), privilegePrefix)
	if err := ExecuteSSHCommand(sshConfig, archiveCmd); err != nil {
		return err
	}
	return nil
}

func archiveExistingAgentService(sshConfig util.SSHConfig) error {
	privilegePrefix := sudoPrefix(sshConfig)
	archiveCmd := fmt.Sprintf("backup_dir=/tmp/dodevops-agent-backup-$(date +%%Y%%m%%d%%H%%M%%S); if [ -e /etc/systemd/system/agent.service ]; then mkdir -p \"$backup_dir\" && %smv /etc/systemd/system/agent.service \"$backup_dir/\"; fi", privilegePrefix)
	if err := ExecuteSSHCommand(sshConfig, archiveCmd); err != nil {
		return err
	}
	return nil
}

func collectAgentSystemdDiagnostics(sshConfig util.SSHConfig) string {
	type diagCommand struct {
		title   string
		command string
	}

	commands := []diagCommand{
		{"当前用户", "whoami 2>&1 || true; id 2>&1 || true"},
		{"init/systemd", "ps -p 1 -o pid,comm,args 2>&1 || true; command -v systemctl 2>&1 || true; systemctl --version 2>&1 | head -3 || true"},
		{"sudo检查", "if [ \"$(id -u 2>/dev/null)\" = \"0\" ]; then echo 'root user, sudo not required'; else command -v sudo 2>&1 || true; sudo -n true 2>&1 && echo 'sudo ok' || echo 'sudo failed'; fi"},
		{"路径权限", "ls -ld /opt /opt/agent /etc/systemd/system 2>&1 || true; ls -l /opt/agent/dodevops-agent /etc/systemd/system/agent.service 2>&1 || true"},
		{"服务文件", "sed -n '1,120p' /etc/systemd/system/agent.service 2>&1 || true"},
		{"systemd状态", "systemctl status agent.service --no-pager -l 2>&1 || true"},
		{"agent日志", "tail -80 /var/log/agent.log 2>&1 || true; journalctl -u agent.service -n 20 --no-pager -l 2>&1 || true"},
	}

	var parts []string
	for _, item := range commands {
		output, err := ExecuteSSHCommandWithOutput(sshConfig, item.command)
		output = strings.TrimSpace(output)
		if err != nil {
			output = strings.TrimSpace(fmt.Sprintf("%s\n诊断命令错误: %v", output, err))
		}
		if output == "" {
			output = "<empty>"
		}
		parts = append(parts, fmt.Sprintf("=== %s ===\n%s", item.title, output))
	}

	diagnostics := strings.Join(parts, "\n\n")
	const maxDiagnosticsLength = 12000
	if len(diagnostics) > maxDiagnosticsLength {
		diagnostics = diagnostics[:maxDiagnosticsLength] + "\n...诊断信息过长，已截断"
	}
	return diagnostics
}

// deployAgentAsync 异步部署agent
func (s *AgentServiceImpl) deployAgentAsync(hostID uint, host model.CmdbHost, version string) error {
	return s.deployAgentAsyncWithOptions(hostID, host, version, model.AgentDeployOptions{})
}

func (s *AgentServiceImpl) deployAgentAsyncWithOptions(hostID uint, host model.CmdbHost, version string, options model.AgentDeployOptions) error {
	if strings.EqualFold(strings.TrimSpace(options.DeployMode), "offline") {
		return s.deployOfflineAgentAsync(hostID, host, version, options)
	}
	version = strings.TrimSpace(version)
	if version == "" {
		version = configuredAgentVersion()
	}

	// 检测目标操作系统
	targetOS := "linux" // 默认linux
	if strings.Contains(strings.ToLower(host.OS), "windows") {
		targetOS = "windows"
	} else if strings.Contains(strings.ToLower(host.OS), "darwin") {
		targetOS = "darwin"
	}

	if _, exists := buildTargets[targetOS]; !exists {
		return fmt.Errorf("不支持的操作系统: %s", host.OS)
	}

	// 更新进度：开始编译 (10%)
	s.agentDao.UpdateInstallProgress(hostID, model.InstallProgressCompiling)

	log.Printf("开始为主机 %d (%s) 部署Agent，目标OS: %s", hostID, host.HostName, targetOS)
	groupID := host.GroupID
	groupName := host.Group.Name
	log.Printf("主机[%s]所属分组: ID=%d, Name=%s", host.HostName, groupID, groupName)
	artifact, err := agentBuildCache.ensureBinary(buildAgentRequest(targetOS, version, groupID, groupName))
	if err != nil {
		return err
	}

	s.agentDao.UpdateInstallProgress(hostID, model.InstallProgressCompiled)

	// 5. 通过SSH拷贝文件到目标主机
	// 更新进度：开始传输 (50%)
	s.agentDao.UpdateInstallProgress(hostID, model.InstallProgressTransfer)

	remotePath := "/opt/agent"
	if targetOS == "windows" {
		remotePath = "C:\\dodevops-agent"
	}

	// 获取SSH认证信息
	var sshConfig util.SSHConfig
	if host.SSHKeyID > 0 {
		sshKey, err := s.getSSHKeyByID(host.SSHKeyID)
		if err != nil {
			return fmt.Errorf("获取SSH认证信息失败: %v", err)
		}

		// 创建SSH连接配置
		sshConfig = util.SSHConfig{
			IP:       host.SSHIP,
			Port:     host.SSHPort,
			Username: host.SSHName,
			Type:     sshKey.Type, // 使用数据库中的认证类型
			Timeout:  30 * time.Second,
		}

		// 根据认证类型设置相应的认证信息
		switch sshKey.Type {
		case 1: // 密码认证
			sshConfig.Password = sshKey.Password
		case 2: // 私钥认证
			sshConfig.PublicKey = sshKey.PublicKey
		case 3: // 公钥免认证
			// 不需要设置额外信息，自动查找本地私钥
		case 4: // 跳板机认证
			sshConfig.BastionHost = sshKey.BastionHost
			sshConfig.BastionPort = sshKey.BastionPort
			sshConfig.BastionUsername = sshKey.BastionUsername
			sshConfig.BastionAuthType = sshKey.BastionAuthType
			sshConfig.BastionPassword = sshKey.BastionPassword
			sshConfig.BastionPrivateKey = sshKey.BastionPrivateKey
			sshConfig.TargetAuthType = sshKey.TargetAuthType
			sshConfig.TargetPassword = sshKey.TargetPassword
			sshConfig.TargetPrivateKey = sshKey.TargetPrivateKey
		default:
			return fmt.Errorf("不支持的SSH认证类型: %d", sshKey.Type)
		}
	} else {
		return fmt.Errorf("主机未配置SSH认证信息")
	}

	// 【关键修复】使用主机IP锁，防止并发部署同一台物理机时的进程冲突
	// 如果多个主机记录共享同一个IP，确保它们的部署操作是串行的
	hostLockManager.Lock(host.SSHIP)
	defer hostLockManager.Unlock(host.SSHIP)

	// 创建远程目录并进行环境预检查
	log.Printf("创建远程目录并检查环境兼容性...")

	// 生产环境兼容性检查 - 不依赖Go环境
	envCheckCmds := []string{
		"whoami",
		"id",
		"uname -a",
		"cat /etc/os-release | head -5 || cat /etc/redhat-release || echo 'Unknown OS'",
		"df -h /opt || df -h /",
		"free -h",
		"which systemctl && echo 'systemctl available' || echo 'systemctl not found, will use alternative startup'",
		"ps --version 2>/dev/null && echo 'ps available' || echo 'ps command check'",
		"netstat --version 2>/dev/null || ss --version 2>/dev/null || echo 'network tools check'",
		"ls -la /lib/x86_64-linux-gnu/ 2>/dev/null | head -3 || echo 'lib check'",
		"ldd --version 2>/dev/null || echo 'ldd not available'",
	}

	log.Printf("环境检查结果:")
	for _, checkCmd := range envCheckCmds {
		if output, err := ExecuteSSHCommandWithOutput(sshConfig, checkCmd); err == nil {
			log.Printf("  %s: %s", checkCmd, strings.TrimSpace(output))
		}
	}

	createDirCmd := fmt.Sprintf("mkdir -p %s", remotePath)
	if targetOS == "windows" {
		createDirCmd = fmt.Sprintf("mkdir %s", remotePath)
	}

	if err := ExecuteSSHCommand(sshConfig, createDirCmd); err != nil {
		return fmt.Errorf("创建远程目录失败: %v", err)
	}

	// 确保完整的生产环境目录结构
	if targetOS != "windows" {
		log.Printf("设置生产环境目录结构...")
		privilegePrefix := sudoPrefix(sshConfig)

		// 创建必要的目录结构
		dirSetupCmds := []string{
			// 创建主目录
			fmt.Sprintf("%smkdir -p %s", privilegePrefix, shellQuote(remotePath)),
			// 创建日志目录
			privilegePrefix + "mkdir -p /var/log",
			// 创建临时目录(确保agent有写权限)
			privilegePrefix + "mkdir -p /tmp/agent",
			// 设置目录权限
			fmt.Sprintf("%schown -R root:root %s", privilegePrefix, shellQuote(remotePath)),
			fmt.Sprintf("%schmod 755 %s", privilegePrefix, shellQuote(remotePath)),
			privilegePrefix + "chmod 777 /tmp/agent",
			// 创建systemd目录(如果不存在)
			privilegePrefix + "mkdir -p /etc/systemd/system",
			// 确保日志目录权限
			privilegePrefix + "chmod 755 /var/log",
			// 初始化Agent独立日志文件
			privilegePrefix + "touch /var/log/agent.log",
			privilegePrefix + "chmod 0644 /var/log/agent.log",
		}

		for _, cmd := range dirSetupCmds {
			if err := ExecuteSSHCommand(sshConfig, cmd); err != nil {
				log.Printf("目录设置命令失败 [%s]: %v", cmd, err)
			}
		}

		// 检查依赖库是否存在(生产环境兼容性)
		libCheckCmds := []string{
			"ls -la /lib/x86_64-linux-gnu/libc.so* 2>/dev/null | head -2 || echo 'glibc check'",
			"ls -la /lib64/ld-linux-x86-64.so* 2>/dev/null || echo 'loader check'",
			"cat /etc/ld.so.conf 2>/dev/null | head -3 || echo 'ld config check'",
		}

		log.Printf("检查系统依赖库:")
		for _, cmd := range libCheckCmds {
			if output, err := ExecuteSSHCommandWithOutput(sshConfig, cmd); err == nil && output != "" {
				log.Printf("  %s", strings.TrimSpace(output))
			}
		}
	}

	// 拷贝二进制文件
	remoteBinaryPath := joinRemotePath(targetOS, remotePath, artifact.BinaryName)
	if targetOS != "windows" {
		if err := stopAndArchiveExistingAgent(sshConfig, remoteBinaryPath); err != nil {
			return fmt.Errorf("备份旧Agent文件失败: %v", err)
		}
	}

	if err := CopyFileViaSSH(sshConfig, artifact.BinaryPath, remoteBinaryPath); err != nil {
		return fmt.Errorf("拷贝文件失败: %v", err)
	}

	log.Printf("验证生产环境二进制文件完整性...")

	// 验证二进制文件是否正确传输
	verifyCommands := []string{
		// 检查文件是否存在
		fmt.Sprintf("ls -la %s", remoteBinaryPath),
		// 检查文件大小
		fmt.Sprintf("du -h %s", remoteBinaryPath),
		// 检查文件类型
		fmt.Sprintf("file %s", remoteBinaryPath),
		// 检查是否可执行
		fmt.Sprintf("test -x %s && echo 'executable' || echo 'not executable'", remoteBinaryPath),
	}

	for _, cmd := range verifyCommands {
		if output, err := ExecuteSSHCommandWithOutput(sshConfig, cmd); err == nil {
			log.Printf("文件验证: %s -> %s", cmd, strings.TrimSpace(output))
		} else {
			log.Printf("文件验证失败: %s -> %v", cmd, err)
		}
	}

	// 检查动态链接依赖(生产环境关键)
	if targetOS != "windows" {
		lddCmd := fmt.Sprintf("ldd %s 2>/dev/null | head -10 || echo 'Static binary or ldd not available'", remoteBinaryPath)
		if output, err := ExecuteSSHCommandWithOutput(sshConfig, lddCmd); err == nil && output != "" {
			log.Printf("二进制依赖检查: %s", strings.TrimSpace(output))
		}

		// 尝试快速测试运行(超时保护)
		testCmd := fmt.Sprintf("timeout 3 %s --help 2>/dev/null || timeout 3 %s -h 2>/dev/null || echo 'Binary help test completed'", remoteBinaryPath, remoteBinaryPath)
		if output, err := ExecuteSSHCommandWithOutput(sshConfig, testCmd); err == nil {
			log.Printf("二进制测试运行: %s", strings.TrimSpace(output))
		}
	}

	// 更新进度：传输完成 (70%)
	s.agentDao.UpdateInstallProgress(hostID, model.InstallProgressTransferred)

	// 6. 设置文件权限并创建systemd服务
	if targetOS != "windows" {
		// Linux/Unix系统设置执行权限
		chmodCmd := fmt.Sprintf("chmod +x %s", remoteBinaryPath)
		if err := ExecuteSSHCommand(sshConfig, chmodCmd); err != nil {
			return fmt.Errorf("设置执行权限失败: %v", err)
		}

		if err := archiveExistingAgentService(sshConfig); err != nil {
			return fmt.Errorf("备份旧systemd服务文件失败: %v", err)
		}

		// 创建systemd服务文件 - 增强生产环境兼容性
		serviceContent := `[Unit]
Description=DevOps Monitoring Agent
Documentation=https://dodevops.com
After=network.target network-online.target
Wants=network-online.target
StartLimitIntervalSec=60
StartLimitBurst=3

[Service]
Type=simple
ExecStart=/bin/sh -c "exec /opt/agent/dodevops-agent >> /var/log/agent.log 2>&1"
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=5
User=root
Group=root
WorkingDirectory=/opt/agent
StandardOutput=null
StandardError=null
Environment=HOME=/root
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
KillMode=mixed
KillSignal=SIGTERM
TimeoutStartSec=60
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target`

		// 方法1: 使用cat heredoc写入服务文件
		privilegePrefix := sudoPrefix(sshConfig)
		createServiceCmd := fmt.Sprintf("%sbash -c 'cat > /etc/systemd/system/agent.service << \"EOF\"\n%s\nEOF'", privilegePrefix, serviceContent)
		if err := ExecuteSSHCommand(sshConfig, createServiceCmd); err != nil {
			return agentSystemdStepError(sshConfig, "创建systemd服务文件失败", err)
		}

		// 验证服务文件是否创建成功
		checkServiceCmd := privilegePrefix + "test -f /etc/systemd/system/agent.service && echo 'Service file created successfully'"
		if err := ExecuteSSHCommand(sshConfig, checkServiceCmd); err != nil {
			return fmt.Errorf("验证systemd服务文件失败: %v", err)
		}

		// 重新加载systemd配置
		if err := ExecuteSSHCommand(sshConfig, privilegePrefix+"systemctl daemon-reload"); err != nil {
			return fmt.Errorf("重新加载systemd配置失败: %v", err)
		}

		// 启用服务开机自启
		if err := ExecuteSSHCommand(sshConfig, privilegePrefix+"systemctl enable agent.service"); err != nil {
			return fmt.Errorf("启用agent服务失败: %v", err)
		}
	}

	// 更新进度：配置完成 (90%)
	s.agentDao.UpdateInstallProgress(hostID, model.InstallProgressConfigured)

	// 8. 预检查：验证二进制文件可执行
	testRunCmd := fmt.Sprintf("timeout 5 %s --help 2>/dev/null || echo 'Binary test completed'", remoteBinaryPath)
	ExecuteSSHCommand(sshConfig, testRunCmd) // 忽略错误，只是测试

	// 9. 启动agent服务 - 增强启动策略
	log.Printf("启动agent服务...")
	startCmd := s.getStartAgentCommand(targetOS, remoteBinaryPath, sshConfig)

	// 尝试systemd启动
	if err := ExecuteSSHCommand(sshConfig, startCmd); err != nil {
		return agentSystemdStepError(sshConfig, "启动agent服务失败", err)
	}
	log.Printf("systemd启动成功")

	// 10. 综合验证服务是否启动成功 - 增强验证策略
	log.Printf("开始验证agent服务启动状态...")

	// 分阶段验证，增加容错性
	maxRetries := 3
	verifySuccess := false

	for i := 0; i < maxRetries && !verifySuccess; i++ {
		log.Printf("验证阶段 %d/%d...", i+1, maxRetries)

		// 等待服务启动
		waitTime := time.Duration(5+i*2) * time.Second
		log.Printf("等待 %v 后验证...", waitTime)
		time.Sleep(waitTime)

		// 步骤1: 检查进程是否运行
		processRunning := false
		checkProcessCmds := []string{
			"ps aux | grep dodevops-agent | grep -v grep",
			"pgrep -f dodevops-agent",
			"pidof dodevops-agent",
		}

		for _, processCmd := range checkProcessCmds {
			if err := ExecuteSSHCommand(sshConfig, processCmd); err == nil {
				processRunning = true
				log.Printf("进程检查通过: %s", processCmd)
				break
			}
		}

		if !processRunning {
			log.Printf("第%d次验证: agent进程未运行", i+1)
			if i == maxRetries-1 {
				// 最后一次失败，收集详细错误日志
				logCommands := []string{
					"tail -80 /var/log/agent.log 2>/dev/null",
					"journalctl -u agent.service -n 20 --no-pager 2>/dev/null",
					"tail -20 /tmp/agent.log 2>/dev/null",
					"systemctl status agent.service --no-pager 2>/dev/null",
				}

				allLogs := make([]string, 0)
				for _, logCmd := range logCommands {
					if output, err := ExecuteSSHCommandWithOutput(sshConfig, logCmd); err == nil && output != "" {
						allLogs = append(allLogs, fmt.Sprintf("=== %s ===\n%s", logCmd, output))
					}
				}

				errorLog := "无可用日志"
				if len(allLogs) > 0 {
					errorLog = strings.Join(allLogs, "\n\n")
				}

				return fmt.Errorf("agent进程未运行，详细日志:\n%s", errorLog)
			}
			continue
		}

		// 步骤2: 检查端口是否监听 (可选检查，允许失败)
		portListening := false
		checkPortCmds := []string{
			"netstat -tunlp | grep :12580",
			"ss -tunlp | grep :12580",
			"lsof -i :12580",
		}

		for _, portCmd := range checkPortCmds {
			if err := ExecuteSSHCommand(sshConfig, portCmd); err == nil {
				portListening = true
				log.Printf("端口检查通过: %s", portCmd)
				break
			}
		}

		if !portListening {
			log.Printf("警告: 端口12580暂未监听，可能需要更多时间初始化")
			// 不作为失败条件，继续验证其他方面
		}

		// 步骤3: 尝试HTTP健康检查 (可选)
		healthCheck := false
		checkHealthCmd := "curl -s http://localhost:12580/metrics | head -1 2>/dev/null || wget -qO- http://localhost:12580/metrics | head -1 2>/dev/null"
		if err := ExecuteSSHCommand(sshConfig, checkHealthCmd); err == nil {
			healthCheck = true
			log.Printf("HTTP健康检查通过")
		} else {
			log.Printf("HTTP健康检查失败，可能服务还在初始化")
		}

		// 综合判断
		if processRunning {
			if portListening || healthCheck {
				verifySuccess = true
				log.Printf("验证成功: 进程运行中且服务正常")
			} else if i == maxRetries-1 {
				// 最后一次验证：进程运行但端口/服务异常，记录警告但不失败
				log.Printf("警告: 进程运行但端口检查失败，可能需要更长时间初始化")
				verifySuccess = true
			}
		}
	}

	if !verifySuccess {
		return fmt.Errorf("服务验证失败: 经过%d次验证仍无法确认服务正常启动", maxRetries)
	}

	// 更新进度：启动成功 (100%) 并同时更新状态为运行中
	updateErr := s.agentDao.UpdateByHostID(hostID, map[string]interface{}{
		"status":           model.AgentStatusRunning,
		"install_progress": model.InstallProgressStarted,
		"error_msg":        "",
		"update_time":      time.Now(),
	})
	if updateErr != nil {
		log.Printf("Failed to update agent status to running for host %d: %v", hostID, updateErr)
		return fmt.Errorf("更新Agent状态失败: %v", updateErr)
	}

	log.Printf("Agent successfully deployed to host %d, status updated to running", hostID)
	return nil
}

// getStopAgentCommand 获取停止agent的命令
func (s *AgentServiceImpl) getStopAgentCommand(targetOS string, sshConfig util.SSHConfig) string {
	switch targetOS {
	case "windows":
		return "taskkill /F /IM dodevops-agent.exe"
	default:
		return sudoPrefix(sshConfig) + "systemctl stop agent.service"
	}
}

// getStartAgentCommand 获取启动agent的命令
func (s *AgentServiceImpl) getStartAgentCommand(targetOS, binaryPath string, sshConfig util.SSHConfig) string {
	switch targetOS {
	case "windows":
		return fmt.Sprintf("start /B %s", binaryPath)
	default:
		return sudoPrefix(sshConfig) + "systemctl start agent.service"
	}
}

// getCheckAgentCommand 获取检查agent状态的命令
func (s *AgentServiceImpl) getCheckAgentCommand(targetOS string) string {
	switch targetOS {
	case "windows":
		return "tasklist | findstr dodevops-agent.exe"
	default:
		return "systemctl status agent.service | grep Active | awk '{print $3}' | awk -F'(' '{print $2}' | awk -F')' '{print $1}'"
	}
}

// UninstallAgent 卸载指定主机的agent
func (s *AgentServiceImpl) UninstallAgent(c *gin.Context, hostID uint) {
	// 参数校验
	if hostID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "主机ID不能为空")
		return
	}

	// 获取主机信息
	host, err := s.hostDao.GetCmdbHostById(hostID)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "获取主机信息失败")
		return
	}

	// 异步卸载
	go func() {
		uninstallErr := s.uninstallAgentAsync(host.ID, host)
		if uninstallErr != nil {
			log.Printf("Uninstall agent from host %d failed: %v", hostID, uninstallErr)
			redis.RedisDb.Set(c, fmt.Sprintf("agent:uninstall:error:%d", hostID), uninstallErr.Error(), 30*time.Minute)
		}
	}()

	result.Success(c, map[string]interface{}{
		"message": "Agent卸载任务已启动，正在后台执行",
		"hostId":  hostID,
		"status":  "uninstalling",
	})
}

// BatchUninstallAgent 批量卸载agent
func (s *AgentServiceImpl) BatchUninstallAgent(c *gin.Context, dto *model.BatchDeployAgentDto) {
	// 参数校验
	if len(dto.HostIDs) == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "主机ID列表不能为空")
		return
	}

	var successHosts []uint
	var failedHosts []map[string]interface{}

	// 并发卸载每台主机
	for _, hostID := range dto.HostIDs {
		// 获取主机信息
		host, err := s.hostDao.GetCmdbHostById(hostID)
		if err != nil {
			failedHosts = append(failedHosts, map[string]interface{}{
				"hostId": hostID,
				"error":  "获取主机信息失败",
			})
			continue
		}

		successHosts = append(successHosts, hostID)

		// 异步卸载
		go func(hID uint, h model.CmdbHost) {
			uninstallErr := s.uninstallAgentAsync(hID, h)
			if uninstallErr != nil {
				log.Printf("Uninstall agent from host %d failed: %v", hID, uninstallErr)
			}
		}(hostID, host)
	}

	result.Success(c, map[string]interface{}{
		"message":      fmt.Sprintf("批量卸载任务已启动，成功启动 %d 台，失败 %d 台", len(successHosts), len(failedHosts)),
		"successCount": len(successHosts),
		"failedCount":  len(failedHosts),
		"successHosts": successHosts,
		"failedHosts":  failedHosts,
		"status":       "uninstalling",
	})
}

// RedeployAgent 重新部署指定主机的agent
func (s *AgentServiceImpl) RedeployAgent(c *gin.Context, hostID uint) {
	if hostID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "主机ID不能为空")
		return
	}

	host, err := s.hostDao.GetCmdbHostById(hostID)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "获取主机信息失败")
		return
	}

	if host.Name == "" {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "主机名称为空")
		return
	}

	version := configuredAgentVersion()
	if err := s.prepareAgentDeployRecord(hostID, host, version); err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "创建Agent重部署记录失败")
		return
	}

	go func() {
		if err := s.redeployAgentAsync(hostID, host, version); err != nil {
			log.Printf("Redeploy agent to host %d failed: %v", hostID, err)
			s.updateRedeployError(hostID, host, version, err)
		}
	}()

	result.Success(c, map[string]interface{}{
		"message": "Agent重新部署任务已启动，正在后台执行",
		"hostId":  hostID,
		"status":  "redeploying",
	})
}

// BatchRedeployAgent 批量重新部署agent
func (s *AgentServiceImpl) BatchRedeployAgent(c *gin.Context, dto *model.RedeployAgentDto) {
	if len(dto.HostIDs) == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "主机ID列表不能为空")
		return
	}

	var successHosts []uint
	var failedHosts []map[string]interface{}
	version := configuredAgentVersion()
	var validHosts []model.CmdbHost

	for _, hostID := range dto.HostIDs {
		host, err := s.hostDao.GetCmdbHostById(hostID)
		if err != nil {
			failedHosts = append(failedHosts, map[string]interface{}{
				"hostId": hostID,
				"error":  "获取主机信息失败",
			})
			continue
		}

		if host.Name == "" {
			failedHosts = append(failedHosts, map[string]interface{}{
				"hostId": hostID,
				"error":  "主机名称为空",
			})
			continue
		}

		if err := s.prepareAgentDeployRecord(hostID, host, version); err != nil {
			failedHosts = append(failedHosts, map[string]interface{}{
				"hostId": hostID,
				"error":  "创建Agent重部署记录失败",
			})
			continue
		}

		successHosts = append(successHosts, hostID)
		validHosts = append(validHosts, host)
	}

	go func(hosts []model.CmdbHost, deployVersion string) {
		if err := s.batchRedeployAgentAsync(hosts, deployVersion); err != nil {
			log.Printf("Batch redeploy agents failed: %v", err)
		}
	}(append([]model.CmdbHost(nil), validHosts...), version)

	result.Success(c, map[string]interface{}{
		"message":      fmt.Sprintf("批量重新部署任务已启动，成功启动 %d 台，失败 %d 台", len(successHosts), len(failedHosts)),
		"successCount": len(successHosts),
		"failedCount":  len(failedHosts),
		"successHosts": successHosts,
		"failedHosts":  failedHosts,
		"status":       "redeploying",
	})
}

func (s *AgentServiceImpl) redeployAgentAsync(hostID uint, host model.CmdbHost, version string) error {
	log.Printf("开始重新部署主机 %d 的 agent...", hostID)
	version = strings.TrimSpace(version)
	if version == "" {
		version = configuredAgentVersion()
	}

	if err := s.uninstallAgentAsync(hostID, host); err != nil {
		return fmt.Errorf("卸载旧Agent失败: %v", err)
	}

	if err := s.prepareAgentDeployRecord(hostID, host, version); err != nil {
		return fmt.Errorf("创建Agent部署记录失败: %v", err)
	}

	if err := s.deployAgentAsync(hostID, host, version); err != nil {
		return fmt.Errorf("部署新Agent失败: %v", err)
	}

	log.Printf("主机 %d 的 agent 重新部署成功", hostID)
	return nil
}

func (s *AgentServiceImpl) batchRedeployAgentAsync(hosts []model.CmdbHost, version string) error {
	if len(hosts) == 0 {
		return fmt.Errorf("主机列表为空")
	}
	version = strings.TrimSpace(version)
	if version == "" {
		version = configuredAgentVersion()
	}

	log.Printf("开始批量重新部署Agent到 %d 台主机", len(hosts))

	type uninstallResult struct {
		host model.CmdbHost
		err  error
	}

	resultsChan := make(chan uninstallResult, len(hosts))
	for _, host := range hosts {
		go func(h model.CmdbHost) {
			err := s.uninstallAgentAsync(h.ID, h)
			resultsChan <- uninstallResult{host: h, err: err}
		}(host)
	}

	var readyHosts []model.CmdbHost
	var errors []string
	for i := 0; i < len(hosts); i++ {
		result := <-resultsChan
		if result.err != nil {
			redeployErr := fmt.Errorf("卸载旧Agent失败: %v", result.err)
			s.updateRedeployError(result.host.ID, result.host, version, redeployErr)
			errors = append(errors, fmt.Sprintf("主机 %d 卸载失败: %v", result.host.ID, result.err))
			continue
		}

		if err := s.prepareAgentDeployRecord(result.host.ID, result.host, version); err != nil {
			redeployErr := fmt.Errorf("创建Agent部署记录失败: %v", err)
			s.updateRedeployError(result.host.ID, result.host, version, redeployErr)
			errors = append(errors, fmt.Sprintf("主机 %d 创建部署记录失败: %v", result.host.ID, err))
			continue
		}

		readyHosts = append(readyHosts, result.host)
	}

	if len(readyHosts) == 0 {
		if len(errors) == 0 {
			return fmt.Errorf("没有可用于重新部署的主机")
		}
		return fmt.Errorf("批量重新部署失败: %s", strings.Join(errors, "; "))
	}

	if err := s.batchDeployAgentAsync(readyHosts, version); err != nil {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("批量重新部署部分失败: %s", strings.Join(errors, "; "))
	}

	log.Printf("批量重新部署完成: 成功 %d 台", len(readyHosts))
	return nil
}

// uninstallAgentAsync 异步卸载agent
func (s *AgentServiceImpl) uninstallAgentAsync(hostID uint, host model.CmdbHost) error {
	log.Printf("开始卸载主机 %d 的 agent...", hostID)

	// 检测目标操作系统
	targetOS := "linux"
	if strings.Contains(strings.ToLower(host.OS), "windows") {
		targetOS = "windows"
	} else if strings.Contains(strings.ToLower(host.OS), "darwin") {
		targetOS = "darwin"
	}

	// 获取SSH认证信息
	var sshConfig util.SSHConfig
	if host.SSHKeyID > 0 {
		sshKey, err := s.getSSHKeyByID(host.SSHKeyID)
		if err != nil {
			return fmt.Errorf("获取SSH认证信息失败: %v", err)
		}

		// 创建SSH连接配置
		sshConfig = util.SSHConfig{
			IP:       host.SSHIP,
			Port:     host.SSHPort,
			Username: host.SSHName,
			Type:     sshKey.Type, // 使用数据库中的认证类型
			Timeout:  30 * time.Second,
		}

		// 根据认证类型设置相应的认证信息
		switch sshKey.Type {
		case 1: // 密码认证
			sshConfig.Password = sshKey.Password
		case 2: // 私钥认证
			sshConfig.PublicKey = sshKey.PublicKey
		case 3: // 公钥免认证
			// 不需要设置额外信息，自动查找本地私钥
		case 4: // 跳板机认证
			sshConfig.BastionHost = sshKey.BastionHost
			sshConfig.BastionPort = sshKey.BastionPort
			sshConfig.BastionUsername = sshKey.BastionUsername
			sshConfig.BastionAuthType = sshKey.BastionAuthType
			sshConfig.BastionPassword = sshKey.BastionPassword
			sshConfig.BastionPrivateKey = sshKey.BastionPrivateKey
			sshConfig.TargetAuthType = sshKey.TargetAuthType
			sshConfig.TargetPassword = sshKey.TargetPassword
			sshConfig.TargetPrivateKey = sshKey.TargetPrivateKey
		default:
			return fmt.Errorf("不支持的SSH认证类型: %d", sshKey.Type)
		}
	} else {
		return fmt.Errorf("主机未配置SSH认证信息")
	}

	// 【关键修复】使用主机IP锁，防止并发卸载同一台物理机时的进程冲突
	hostLockManager.Lock(host.SSHIP)
	defer hostLockManager.Unlock(host.SSHIP)

	log.Printf("开始卸载主机 %d 的 agent...", hostID)

	// 1. 停止agent进程
	stopCmd := s.getStopAgentCommand(targetOS, sshConfig)
	log.Printf("执行停止命令: %s", stopCmd)
	ExecuteSSHCommand(sshConfig, stopCmd)

	// 2. 禁用并归档systemd服务(仅Linux)
	if targetOS != "windows" {
		privilegePrefix := sudoPrefix(sshConfig)
		// 禁用服务
		ExecuteSSHCommand(sshConfig, privilegePrefix+"systemctl disable agent.service")
		// 归档服务文件，避免触发远端 rm 限制，同时保留现场
		ExecuteSSHCommand(sshConfig, fmt.Sprintf("backup_dir=/tmp/dodevops-agent-uninstall-$(date +%%Y%%m%%d%%H%%M%%S); if [ -e /etc/systemd/system/agent.service ]; then mkdir -p \"$backup_dir\" && %smv /etc/systemd/system/agent.service \"$backup_dir/\"; fi", privilegePrefix))
		// 重新加载systemd配置
		ExecuteSSHCommand(sshConfig, privilegePrefix+"systemctl daemon-reload")
	}

	// 3. 删除agent文件和目录
	remotePath := "/opt/agent"
	if targetOS == "windows" {
		remotePath = "C:\\dodevops-agent"
	}

	var removeCmd string
	switch targetOS {
	case "windows":
		removeCmd = fmt.Sprintf("rmdir /S /Q %s", remotePath)
	default:
		removeCmd = fmt.Sprintf("backup_dir=/tmp/dodevops-agent-uninstall-$(date +%%Y%%m%%d%%H%%M%%S); if [ -e %[1]s ]; then mkdir -p \"$backup_dir\" && %[2]smv %[1]s \"$backup_dir/\"; fi", shellQuote(remotePath), sudoPrefix(sshConfig))
	}

	log.Printf("执行归档命令: %s", removeCmd)
	if err := ExecuteSSHCommand(sshConfig, removeCmd); err != nil {
		return fmt.Errorf("归档agent文件失败: %v", err)
	}

	// 4. 获取主机的本地IP和hostname（用于清理Pushgateway数据）
	// Agent部署时使用的是主机的本地IP作为job名称，hostname作为分组标签
	localIPCmd := "ip route get 1 | awk '{print $7; exit}' 2>/dev/null || hostname -I | awk '{print $1}'"
	localIP, err := ExecuteSSHCommandWithOutput(sshConfig, localIPCmd)
	if err != nil || localIP == "" {
		log.Printf("获取主机本地IP失败，使用SSH IP作为备选: %v", err)
		localIP = host.SSHIP // 备选方案：使用SSH IP
	} else {
		localIP = strings.TrimSpace(localIP)
		log.Printf("获取到主机本地IP: %s", localIP)
	}

	// 获取主机hostname
	hostnameCmd := "hostname"
	remoteHostname, err := ExecuteSSHCommandWithOutput(sshConfig, hostnameCmd)
	if err != nil || remoteHostname == "" {
		log.Printf("获取主机hostname失败: %v", err)
		remoteHostname = host.Name // 备选方案：使用数据库中的主机名
	} else {
		remoteHostname = strings.TrimSpace(remoteHostname)
		log.Printf("获取到主机hostname: %s", remoteHostname)
	}

	// 5. 清理Pushgateway中的监控数据（使用主机本地IP作为job名称，hostname和分组作为分组标签）
	s.cleanupPushgatewayMetricsWithGroup(localIP, remoteHostname, host.GroupID, host.Group.Name)
	s.cleanupPrometheusSeries(localIP, remoteHostname, host.GroupID, host.Group.Name)

	// 6. 删除数据库中的Agent记录
	if err := s.agentDao.DeleteByHostID(hostID); err != nil {
		log.Printf("删除数据库Agent记录失败: %v", err)
		// 虽然删除数据库记录失败，但远程文件已删除，仍可认为卸载成功
		// 只记录错误日志，不返回错误
	}

	log.Printf("主机 %d 的 agent 卸载成功，已删除远程文件、Pushgateway数据、Prometheus历史数据和数据库记录", hostID)
	return nil
}

// RestartAgent 重启agent
func (s *AgentServiceImpl) RestartAgent(c *gin.Context, hostID uint) {
	// 参数校验
	if hostID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "主机ID不能为空")
		return
	}

	// 获取Agent信息
	agent, err := s.agentDao.GetByHostID(hostID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Agent未部署，执行部署
			s.DeployAgent(c, hostID)
			return
		}
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "获取Agent信息失败")
		return
	}

	// 检查Agent状态
	if agent.Status != model.AgentStatusRunning {
		// 状态不是运行中，执行部署
		s.DeployAgent(c, hostID)
		return
	}

	// 获取主机信息
	host, err := s.hostDao.GetCmdbHostById(hostID)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "获取主机信息失败")
		return
	}

	// 异步重启
	go func() {
		targetOS := s.detectPlatform(host.OS)
		var sshConfig util.SSHConfig
		if host.SSHKeyID > 0 {
			sshKey, err := s.getSSHKeyByID(host.SSHKeyID)
			if err == nil {
				// 创建SSH连接配置
				sshConfig = util.SSHConfig{
					IP:       host.SSHIP,
					Port:     host.SSHPort,
					Username: host.SSHName,
					Type:     sshKey.Type, // 使用数据库中的认证类型
					Timeout:  30 * time.Second,
				}

				// 根据认证类型设置相应的认证信息
				switch sshKey.Type {
				case 1: // 密码认证
					sshConfig.Password = sshKey.Password
				case 2: // 私钥认证
					sshConfig.PublicKey = sshKey.PublicKey
				case 3: // 公钥免认证
					// 不需要设置额外信息，自动查找本地私钥
				case 4: // 跳板机认证
					sshConfig.BastionHost = sshKey.BastionHost
					sshConfig.BastionPort = sshKey.BastionPort
					sshConfig.BastionUsername = sshKey.BastionUsername
					sshConfig.BastionAuthType = sshKey.BastionAuthType
					sshConfig.BastionPassword = sshKey.BastionPassword
					sshConfig.BastionPrivateKey = sshKey.BastionPrivateKey
					sshConfig.TargetAuthType = sshKey.TargetAuthType
					sshConfig.TargetPassword = sshKey.TargetPassword
					sshConfig.TargetPrivateKey = sshKey.TargetPrivateKey
				}

			}
		}

		// 1. 停止agent进程
		stopCmd := s.getStopAgentCommand(targetOS, sshConfig)
		ExecuteSSHCommand(sshConfig, stopCmd)

		// 2. 启动agent进程
		startCmd := s.getStartAgentCommand(targetOS, agent.InstallPath, sshConfig)
		if err := ExecuteSSHCommand(sshConfig, startCmd); err != nil {
			s.agentDao.UpdateByHostID(hostID, map[string]interface{}{
				"status":           model.AgentStatusStartError,
				"install_progress": model.InstallProgressFailed,
				"error_msg":        fmt.Sprintf("重启失败: %v", err),
			})
			return
		}

		// 3. 验证启动状态
		time.Sleep(3 * time.Second)
		checkCmd := s.getCheckAgentCommand(targetOS)
		if err := ExecuteSSHCommand(sshConfig, checkCmd); err != nil {
			s.agentDao.UpdateByHostID(hostID, map[string]interface{}{
				"status":           model.AgentStatusStartError,
				"install_progress": model.InstallProgressFailed,
				"error_msg":        "重启后验证失败",
			})
			return
		}

		// 更新状态为运行中
		s.agentDao.UpdateByHostID(hostID, map[string]interface{}{
			"status":           model.AgentStatusRunning,
			"install_progress": model.InstallProgressStarted,
			"error_msg":        "",
			"update_time":      time.Now(),
		})
	}()

	result.Success(c, map[string]interface{}{
		"message": "Agent重启任务已启动，正在后台执行",
		"hostId":  hostID,
		"status":  "restarting",
	})
}

// getCurrentWorkingDir 获取当前工作目录，确保返回包含go.mod的项目根目录
func getCurrentWorkingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		// 如果获取失败，尝试找到项目根目录
		return findProjectRoot()
	}

	// 检查当前目录是否包含go.mod文件
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return dir
	}

	// 如果当前目录没有go.mod，尝试找到项目根目录
	return findProjectRoot()
}

// findProjectRoot 查找包含go.mod的项目根目录
func findProjectRoot() string {
	// 获取当前执行文件的路径
	execPath, err := os.Executable()
	if err != nil {
		return os.TempDir() // 如果都失败了，返回系统临时目录
	}

	// 从执行文件所在目录向上查找go.mod
	dir := filepath.Dir(execPath)
	for i := 0; i < 10; i++ { // 最多向上查找10级目录
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir { // 到达根目录
			break
		}
		dir = parent
	}

	// 如果都找不到，使用一个临时的方案：不依赖本地go.mod
	return os.TempDir()
}

// ExecuteSSHCommand 执行SSH命令（带重试机制）
func ExecuteSSHCommand(config util.SSHConfig, command string) error {
	return ExecuteSSHCommandWithRetry(config, command, 3)
}

// ExecuteSSHCommandWithRetry 执行SSH命令，支持重试
func ExecuteSSHCommandWithRetry(config util.SSHConfig, command string, maxRetries int) error {
	log.Printf("执行SSH命令: %s@%s:%d -> %s", config.Username, config.IP, config.Port, command)

	sshUtil := util.NewSSHUtil()
	var lastErr error
	var output string

	for attempt := 1; attempt <= maxRetries; attempt++ {
		output, lastErr = sshUtil.ExecuteRemoteCommand(&config, command)

		if lastErr == nil {
			// 成功
			if output != "" {
				log.Printf("SSH命令输出: %s", strings.TrimSpace(output))
			}
			if attempt > 1 {
				log.Printf("SSH命令在第 %d 次尝试后成功", attempt)
			}
			return nil
		}

		// 检查是否是连接相关的错误（值得重试）
		errMsg := lastErr.Error()
		isConnectionError := strings.Contains(errMsg, "connection reset") ||
			strings.Contains(errMsg, "EOF") ||
			strings.Contains(errMsg, "handshake failed") ||
			strings.Contains(errMsg, "broken pipe") ||
			strings.Contains(errMsg, "connection refused")

		if !isConnectionError {
			// 不是连接错误，不重试
			log.Printf("SSH命令执行失败（非连接错误，不重试）: %v, 输出: %s", lastErr, output)
			return fmt.Errorf("SSH命令执行失败: %v (输出: %s)", lastErr, output)
		}

		// 连接错误，准备重试
		if attempt < maxRetries {
			retryDelay := time.Duration(attempt) * 500 * time.Millisecond
			log.Printf("SSH连接失败（第 %d/%d 次尝试）: %v，%v 后重试...", attempt, maxRetries, lastErr, retryDelay)
			time.Sleep(retryDelay)
		}
	}

	// 所有重试都失败
	log.Printf("SSH命令执行失败（已重试 %d 次）: %v, 输出: %s", maxRetries, lastErr, output)
	return fmt.Errorf("SSH命令执行失败（已重试 %d 次）: %v (输出: %s)", maxRetries, lastErr, output)
}

// ExecuteSSHCommandWithOutput 执行SSH命令并返回输出（带重试机制）
func ExecuteSSHCommandWithOutput(config util.SSHConfig, command string) (string, error) {
	log.Printf("执行SSH命令(带输出): %s@%s:%d -> %s", config.Username, config.IP, config.Port, command)

	sshUtil := util.NewSSHUtil()
	var lastErr error
	var output string
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		output, lastErr = sshUtil.ExecuteRemoteCommand(&config, command)

		if lastErr == nil {
			// 成功
			if output != "" {
				log.Printf("SSH命令输出: %s", strings.TrimSpace(output))
			}
			if attempt > 1 {
				log.Printf("SSH命令在第 %d 次尝试后成功", attempt)
			}
			return output, nil
		}

		// 检查是否是连接相关的错误
		errMsg := lastErr.Error()
		isConnectionError := strings.Contains(errMsg, "connection reset") ||
			strings.Contains(errMsg, "EOF") ||
			strings.Contains(errMsg, "handshake failed") ||
			strings.Contains(errMsg, "broken pipe")

		if !isConnectionError || attempt >= maxRetries {
			log.Printf("SSH命令执行失败: %v, 输出: %s", lastErr, output)
			return output, fmt.Errorf("SSH命令执行失败: %v", lastErr)
		}

		// 重试
		retryDelay := time.Duration(attempt) * 500 * time.Millisecond
		log.Printf("SSH连接失败（第 %d/%d 次尝试）: %v，%v 后重试...", attempt, maxRetries, lastErr, retryDelay)
		time.Sleep(retryDelay)
	}

	return output, fmt.Errorf("SSH命令执行失败: %v", lastErr)
}

// CopyFileViaSSH 通过SSH拷贝文件(使用SCP协议)
func CopyFileViaSSH(config util.SSHConfig, localPath, remotePath string) error {
	log.Printf("开始拷贝文件: %s -> %s@%s:%s", localPath, config.Username, config.IP, remotePath)

	// 验证文件路径是否存在并获取文件信息
	fileInfo, err := os.Stat(localPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", localPath)
	}
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %v", err)
	}

	log.Printf("源文件信息: %s (大小: %d bytes)", localPath, fileInfo.Size())

	// 使用 SSHUtil 创建SSH连接（支持跳板机）
	sshUtil := util.NewSSHUtil()

	var client *ssh.Client

	if config.Type == 4 {
		// 跳板机认证
		bastionAuth := &util.SSHConfig{
			IP:        config.BastionHost,
			Port:      config.BastionPort,
			Username:  config.BastionUsername,
			Type:      config.BastionAuthType,
			Password:  config.BastionPassword,
			PublicKey: config.BastionPrivateKey,
		}
		targetAuth := &util.SSHConfig{
			IP:        config.IP,
			Port:      config.Port,
			Username:  config.Username,
			Type:      config.TargetAuthType,
			Password:  config.TargetPassword,
			PublicKey: config.TargetPrivateKey,
		}
		var err2 error
		client, err2 = sshUtil.CreateBastionSSHClient(bastionAuth, targetAuth)
		if err2 != nil {
			return fmt.Errorf("创建跳板机SSH连接失败: %v", err2)
		}
	} else {
		// 普通认证方式 - 使用现有的认证方式创建连接
		sshConfig, err2 := sshUtil.GetSSHConfig(&config)
		if err2 != nil {
			return fmt.Errorf("创建SSH配置失败: %v", err2)
		}
		addr := fmt.Sprintf("%s:%d", config.IP, config.Port)
		var err3 error
		client, err3 = ssh.Dial("tcp", addr, sshConfig)
		if err3 != nil {
			return fmt.Errorf("创建SSH连接失败: %v", err3)
		}
	}
	defer client.Close()

	// 使用SFTP传输文件（更可靠）
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("创建SFTP客户端失败: %v", err)
	}
	defer sftpClient.Close()

	// 确保目标目录存在
	remoteDir := remoteParentDir(remotePath)
	if err := sftpClient.MkdirAll(remoteDir); err != nil {
		log.Printf("⚠️  创建远程目录失败: %v，尝试继续", err)
	}

	// 打开本地文件
	srcFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开本地文件失败: %v", err)
	}
	defer srcFile.Close()

	// 创建远程文件（使用 OpenFile 强制创建）
	dstFile, err := sftpClient.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("创建远程文件失败: %v", err)
	}
	defer dstFile.Close()

	// 拷贝文件内容
	written, err := io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("文件传输失败: %v", err)
	}

	log.Printf("✅ 文件传输成功: %d bytes", written)

	// 设置文件权限
	err = sftpClient.Chmod(remotePath, 0755)
	if err != nil {
		log.Printf("⚠️  设置文件权限失败: %v", err)
	}

	return nil
}

// updateDeployError 更新部署错误信息
func (s *AgentServiceImpl) updateDeployError(hostID uint, errorMsg string) {
	s.agentDao.UpdateByHostID(hostID, map[string]interface{}{
		"status":           model.AgentStatusDeployFailed,
		"install_progress": model.InstallProgressFailed,
		"error_msg":        errorMsg,
	})
}

// 新增辅助方法
func (s *AgentServiceImpl) detectPlatform(osInfo string) string {
	osInfo = strings.ToLower(osInfo)
	if strings.Contains(osInfo, "windows") {
		return "windows"
	} else if strings.Contains(osInfo, "darwin") || strings.Contains(osInfo, "macos") {
		return "darwin"
	}
	return "linux"
}

func (s *AgentServiceImpl) getInstallPath(osInfo string) string {
	if s.detectPlatform(osInfo) == "windows" {
		return "C:\\dodevops-agent"
	}
	return "/opt/agent"
}

func (s *AgentServiceImpl) prepareAgentDeployRecord(hostID uint, host model.CmdbHost, version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		version = configuredAgentVersion()
	}

	agentRecord := &model.Agent{
		HostID:          hostID,
		HostName:        host.Name,
		Version:         version,
		Status:          model.AgentStatusDeploying,
		InstallPath:     s.getInstallPath(host.OS),
		Port:            12580,
		CreateTime:      util.HTime{Time: time.Now()},
		ErrorMsg:        "",
		InstallProgress: model.InstallProgressStart,
	}

	return s.createOrUpdateAgent(agentRecord)
}

func (s *AgentServiceImpl) updateRedeployError(hostID uint, host model.CmdbHost, version string, err error) {
	if _, getErr := s.agentDao.GetByHostID(hostID); getErr == gorm.ErrRecordNotFound {
		if createErr := s.prepareAgentDeployRecord(hostID, host, version); createErr != nil {
			log.Printf("Create redeploy failure record for host %d failed: %v", hostID, createErr)
		}
	}

	s.agentDao.UpdateByHostID(hostID, map[string]interface{}{
		"status":           model.AgentStatusDeployFailed,
		"install_progress": model.InstallProgressFailed,
		"error_msg":        err.Error(),
	})
}

func (s *AgentServiceImpl) createOrUpdateAgent(agent *model.Agent) error {
	existing, err := s.agentDao.GetByHostID(agent.HostID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	if existing != nil {
		// 更新现有记录
		agent.ID = existing.ID
		agent.CreateTime = existing.CreateTime
		return s.agentDao.Update(agent)
	}

	// 创建新记录
	return s.agentDao.Create(agent)
}

func (s *AgentServiceImpl) performHealthCheck(hostID uint) {
	// 获取主机信息
	host, err := s.hostDao.GetCmdbHostById(hostID)
	if err != nil {
		return
	}

	// 获取SSH认证信息
	var sshConfig util.SSHConfig
	if host.SSHKeyID > 0 {
		sshKey, err := s.getSSHKeyByID(host.SSHKeyID)
		if err != nil {
			log.Printf("获取SSH密钥失败: %v", err)
			return
		}

		// 设置SSH连接基本信息
		sshConfig.IP = host.SSHIP
		sshConfig.Port = int(sshKey.Port)
		sshConfig.Username = sshKey.Username
		sshConfig.Type = sshKey.Type

		// 根据认证类型设置相应的认证信息
		switch sshKey.Type {
		case 1: // 密码认证
			sshConfig.Password = sshKey.Password
		case 2: // 私钥认证
			sshConfig.PublicKey = sshKey.PublicKey
		case 3: // 公钥免认证
			// 不需要设置额外信息，自动查找本地私钥
		case 4: // 跳板机认证
			sshConfig.BastionHost = sshKey.BastionHost
			sshConfig.BastionPort = sshKey.BastionPort
			sshConfig.BastionUsername = sshKey.BastionUsername
			sshConfig.BastionAuthType = sshKey.BastionAuthType
			sshConfig.BastionPassword = sshKey.BastionPassword
			sshConfig.BastionPrivateKey = sshKey.BastionPrivateKey
			sshConfig.TargetAuthType = sshKey.TargetAuthType
			sshConfig.TargetPassword = sshKey.TargetPassword
			sshConfig.TargetPrivateKey = sshKey.TargetPrivateKey
		default:
			log.Printf("不支持的SSH认证类型: %d", sshKey.Type)
			return
		}
	}

	// 检查agent进程是否运行
	targetOS := s.detectPlatform(host.OS)
	checkCmd := s.getCheckAgentCommand(targetOS)

	output, err := ExecuteSSHCommandWithOutput(sshConfig, checkCmd)
	if err != nil || (targetOS != "windows" && strings.TrimSpace(output) != "running") {
		// 进程不存在或状态不是运行中，更新状态为启动异常
		s.agentDao.UpdateByHostID(hostID, map[string]interface{}{
			"status": model.AgentStatusStartError,
		})
	}
}
