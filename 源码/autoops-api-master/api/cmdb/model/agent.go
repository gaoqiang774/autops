package model

import (
	"dodevops-api/common/util"
)

// Agent Agent 管理模型。
type Agent struct {
	ID              uint       `gorm:"column:id;comment:'主键';primaryKey;NOT NULL" json:"id"`
	HostID          uint       `gorm:"column:host_id;comment:'主机ID';NOT NULL;index" json:"hostId"`
	HostName        string     `gorm:"column:host_name;varchar(128);comment:'主机名称'" json:"hostName"`
	Version         string     `gorm:"column:version;varchar(32);comment:'Agent版本';default:'2.0.0'" json:"version"`
	Status          int        `gorm:"column:status;comment:'状态:1->部署中,2->部署失败,3->运行中,4->启动异常'" json:"status"`
	InstallPath     string     `gorm:"column:install_path;varchar(256);comment:'安装路径'" json:"installPath"`
	Port            int        `gorm:"column:port;comment:'监听端口';default:12580" json:"port"`
	PID             int        `gorm:"column:pid;comment:'进程ID'" json:"pid"`
	LastHeartbeat   util.HTime `gorm:"column:last_heartbeat;comment:'最后心跳时间'" json:"lastHeartbeat"`
	UpdateTime      util.HTime `gorm:"column:update_time;comment:'更新时间'" json:"updateTime"`
	CreateTime      util.HTime `gorm:"column:create_time;comment:'创建时间';NOT NULL" json:"createTime"`
	ErrorMsg        string     `gorm:"column:error_msg;type:text;comment:'错误信息'" json:"errorMsg"`
	InstallProgress int        `gorm:"column:install_progress;comment:'安装进度(0-100)';default:0" json:"installProgress"`
}

func (Agent) TableName() string {
	return "monitor_agent"
}

// AgentStatus Agent 状态常量。
const (
	AgentStatusDeploying    = 1 // 部署中
	AgentStatusDeployFailed = 2 // 部署失败
	AgentStatusRunning      = 3 // 运行中
	AgentStatusStartError   = 4 // 启动异常
)

// AgentOnlineStatus Agent 在线状态常量。
const (
	AgentOnlineStatusOnline  = 1 // 在线
	AgentOnlineStatusOffline = 2 // 离线
)

// AgentInstallProgress Agent 安装进度常量。
const (
	InstallProgressFailed      = -1  // 部署失败
	InstallProgressStart       = 0   // 开始部署
	InstallProgressCompiling   = 10  // 开始编译
	InstallProgressCompiled    = 30  // 编译完成
	InstallProgressTransfer    = 50  // 传输中
	InstallProgressTransferred = 70  // 传输完成
	InstallProgressConfigured  = 90  // 配置完成
	InstallProgressStarted     = 100 // 启动成功
)

// GetStatusText 获取状态文本。
func (a *Agent) GetStatusText() string {
	statusMap := map[int]string{
		AgentStatusDeploying:    "部署中",
		AgentStatusDeployFailed: "部署失败",
		AgentStatusRunning:      "运行中",
		AgentStatusStartError:   "启动异常",
	}
	if text, ok := statusMap[a.Status]; ok {
		return text
	}
	return "未知状态"
}

// GetInstallProgressText 获取安装进度文本。
func (a *Agent) GetInstallProgressText() string {
	progressMap := map[int]string{
		InstallProgressFailed:      "部署失败",
		InstallProgressStart:       "开始部署",
		InstallProgressCompiling:   "开始编译",
		InstallProgressCompiled:    "编译完成",
		InstallProgressTransfer:    "传输中",
		InstallProgressTransferred: "传输完成",
		InstallProgressConfigured:  "配置完成",
		InstallProgressStarted:     "启动成功",
	}
	if text, ok := progressMap[a.InstallProgress]; ok {
		return text
	}
	return "未知进度"
}

// IsHealthy 判断 Agent 是否健康。
func (a *Agent) IsHealthy() bool {
	return a.Status == AgentStatusRunning
}

// CreateAgentDto 创建 Agent DTO。
type CreateAgentDto struct {
	HostID  uint   `validate:"required" json:"hostId"`
	Version string `json:"version"`
}

// BatchDeployAgentDto 批量部署 Agent DTO。
type BatchDeployAgentDto struct {
	HostIDs         []uint `validate:"required" json:"hostIds"`
	Version         string `json:"version"`
	DeployMode      string `json:"deployMode"`      // online（默认）或 offline
	BinaryPath      string `json:"binaryPath"`      // 离线模式：已明确选择的二进制
	BinaryX86Path   string `json:"binaryX86Path"`   // 离线模式：x86/amd64 二进制
	BinaryArm64Path string `json:"binaryArm64Path"` // 离线模式：arm64 二进制
}

// AgentDeployOptions controls how an Agent deployment obtains its artifact.
type AgentDeployOptions struct {
	DeployMode      string
	BinaryPath      string
	BinaryX86Path   string
	BinaryArm64Path string
}

func (d BatchDeployAgentDto) DeployOptions() AgentDeployOptions {
	return AgentDeployOptions{
		DeployMode:      d.DeployMode,
		BinaryPath:      d.BinaryPath,
		BinaryX86Path:   d.BinaryX86Path,
		BinaryArm64Path: d.BinaryArm64Path,
	}
}

// AgentOfflineAssetVO describes a selectable file under the configured upload directory.
type AgentOfflineAssetVO struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
	Kind string `json:"kind"`
	Arch string `json:"arch,omitempty"`
}

// AgentOfflineAssetSlotVO describes whether one architecture package is ready.
type AgentOfflineAssetSlotVO struct {
	Uploaded   bool                  `json:"uploaded"`
	Asset      *AgentOfflineAssetVO  `json:"asset,omitempty"`
	Candidates []AgentOfflineAssetVO `json:"candidates,omitempty"`
}

// AgentOfflineAssetCheckVO is returned by the offline package check endpoint.
type AgentOfflineAssetCheckVO struct {
	Ready   bool                    `json:"ready"`
	X86_64  AgentOfflineAssetSlotVO `json:"x86_64"`
	Arm64   AgentOfflineAssetSlotVO `json:"arm64"`
	Message string                  `json:"message"`
}

// AgentOfflinePrecheckDto checks package availability and target architectures before deployment.
type AgentOfflinePrecheckDto struct {
	HostIDs         []uint `validate:"required" json:"hostIds"`
	BinaryPath      string `json:"binaryPath"`
	BinaryX86Path   string `json:"binaryX86Path"`
	BinaryArm64Path string `json:"binaryArm64Path"`
}

// AgentOfflineHostPrecheckVO reports the architecture and selected package for one host.
type AgentOfflineHostPrecheckVO struct {
	HostID     uint   `json:"hostId"`
	HostName   string `json:"hostName"`
	Arch       string `json:"arch,omitempty"`
	BinaryPath string `json:"binaryPath,omitempty"`
	Ready      bool   `json:"ready"`
	Error      string `json:"error,omitempty"`
}

// AgentOfflinePrecheckVO is returned before an offline deployment is accepted.
type AgentOfflinePrecheckVO struct {
	Ready bool                         `json:"ready"`
	Hosts []AgentOfflineHostPrecheckVO `json:"hosts"`
	Error string                       `json:"error,omitempty"`
}

// RedeployAgentDto 重新部署 Agent DTO。
type RedeployAgentDto struct {
	HostIDs []uint `validate:"required" json:"hostIds"`
}

// UpdateAgentDto 更新 Agent DTO。
type UpdateAgentDto struct {
	ID      uint   `json:"id"`
	Status  int    `json:"status"`
	Version string `json:"version"`
}

// AgentListDto Agent 列表查询 DTO。
type AgentListDto struct {
	HostID       uint   `json:"hostId"`
	HostName     string `json:"hostName"`
	SSHIP        string `json:"sshIp"`
	Version      string `json:"version"`
	OnlineStatus int    `json:"onlineStatus"`
	Status       int    `json:"status"`
	Page         int    `json:"page"`
	PageSize     int    `json:"pageSize"`
}

// AgentCandidateListDto Agent部署候选主机查询 DTO。
type AgentCandidateListDto struct {
	Keyword        string `json:"keyword"`
	GroupID        uint   `json:"groupId"`
	Page           int    `json:"page"`
	PageSize       int    `json:"pageSize"`
	DeployableOnly bool   `json:"deployableOnly"`
}

// AgentCandidateGroupDto Agent部署候选分组查询 DTO。
type AgentCandidateGroupDto struct {
	Keyword string `json:"keyword"`
}

// AgentVersionStat Agent 版本统计。
type AgentVersionStat struct {
	Version string `json:"version"`
	Count   int64  `json:"count"`
}

// AgentVO Agent 视图对象。
type AgentVO struct {
	ID                  uint       `json:"id"`
	HostID              uint       `json:"hostId"`
	HostName            string     `json:"hostName"`
	SSHIP               string     `json:"sshIp"`
	Version             string     `json:"version"`
	Status              int        `json:"status"`
	StatusText          string     `json:"statusText"`
	OnlineStatus        int        `json:"onlineStatus"`
	InstallPath         string     `json:"installPath"`
	Port                int        `json:"port"`
	PID                 int        `json:"pid"`
	LastHeartbeat       util.HTime `json:"lastHeartbeat"`
	UpdateTime          util.HTime `json:"updateTime"`
	IsHealthy           bool       `json:"isHealthy"`
	CreateTime          util.HTime `json:"createTime"`
	ErrorMsg            string     `json:"errorMsg"`
	InstallProgress     int        `json:"installProgress"`
	InstallProgressText string     `json:"installProgressText"`
}

// AgentCandidateHostVO Agent部署候选主机视图。
type AgentCandidateHostVO struct {
	HostID              uint       `json:"hostId"`
	HostName            string     `json:"hostName"`
	Name                string     `json:"name"`
	GroupID             uint       `json:"groupId"`
	GroupName           string     `json:"groupName"`
	PrivateIP           string     `json:"privateIp"`
	PublicIP            string     `json:"publicIp"`
	SSHIP               string     `json:"sshIp"`
	HostType            int        `json:"hostType"`
	Status              int        `json:"status"`
	OS                  string     `json:"os"`
	Remark              string     `json:"remark"`
	AgentID             uint       `json:"agentId"`
	AgentHostID         uint       `json:"agentHostId"`
	AgentStatus         int        `json:"agentStatus"`
	AgentStatusText     string     `json:"agentStatusText"`
	AgentOnlineStatus   int        `json:"agentOnlineStatus"`
	AgentVersion        string     `json:"agentVersion"`
	LastHeartbeat       util.HTime `json:"lastHeartbeat"`
	InstallProgress     int        `json:"installProgress"`
	InstallProgressText string     `json:"installProgressText"`
	Deployable          bool       `json:"deployable"`
	DeployBlockedReason string     `json:"deployBlockedReason"`
}

// AgentCandidateGroupVO Agent部署候选分组视图。
type AgentCandidateGroupVO struct {
	ID                  uint                    `json:"id"`
	ParentID            uint                    `json:"parentId"`
	Name                string                  `json:"name"`
	HostCount           int                     `json:"hostCount"`
	DeployableHostCount int                     `json:"deployableHostCount"`
	Children            []AgentCandidateGroupVO `json:"children"`
}

// ToVO 转换为 VO。
func (a *Agent) ToVO() *AgentVO {
	return &AgentVO{
		ID:                  a.ID,
		HostID:              a.HostID,
		HostName:            a.HostName,
		Version:             a.Version,
		Status:              a.Status,
		StatusText:          a.GetStatusText(),
		OnlineStatus:        AgentOnlineStatusOffline,
		InstallPath:         a.InstallPath,
		Port:                a.Port,
		PID:                 a.PID,
		LastHeartbeat:       a.LastHeartbeat,
		UpdateTime:          a.UpdateTime,
		IsHealthy:           false,
		CreateTime:          a.CreateTime,
		ErrorMsg:            a.ErrorMsg,
		InstallProgress:     a.InstallProgress,
		InstallProgressText: a.GetInstallProgressText(),
	}
}

// AgentHeartbeatDto Agent 心跳 DTO。
type AgentHeartbeatDto struct {
	PID        int    `json:"pid"`
	IP         string `json:"ip"`
	Hostname   string `json:"hostname"`
	Port       int    `json:"port"`
	Token      string `json:"token"`
	Version    string `json:"version"`
	VersionTag string `json:"versionTag"`
}
