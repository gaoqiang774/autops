package model

import (
	"time"

	"dodevops-api/common/util"
)

const (
	LogAgentRuntimeHost = "host"

	VectorStatusNotInstalled = "not_installed"
	VectorStatusInstalled    = "installed"
	VectorStatusRunning      = "running"
	VectorStatusError        = "error"

	LogAgentConfigStatusPending  = "pending"
	LogAgentConfigStatusApplied  = "applied"
	LogAgentConfigStatusFailed   = "failed"
	LogAgentConfigStatusDisabled = "disabled"

	LogAgentOperationInstallVector   = "install_vector"
	LogAgentOperationUpgradeVector   = "upgrade_vector"
	LogAgentOperationValidateConfig  = "validate_config"
	LogAgentOperationReloadVector    = "reload_vector"
	LogAgentOperationRestartVector   = "restart_vector"
	LogAgentOperationStopVector      = "stop_vector"
	LogAgentOperationUninstallVector = "uninstall_vector"

	LogAgentOperationStatusPending  = "pending"
	LogAgentOperationStatusRunning  = "running"
	LogAgentOperationStatusSuccess  = "success"
	LogAgentOperationStatusFailed   = "failed"
	LogAgentOperationStatusCanceled = "canceled"

	LogK8SCollectorOperationInstall   = "install_vector"
	LogK8SCollectorOperationUpdate    = "update_vector"
	LogK8SCollectorOperationUninstall = "uninstall_vector"
	LogK8SCollectorOperationReinstall = "reinstall_vector"
)

type LogAgentStatus struct {
	ID                   uint       `gorm:"primaryKey" json:"id"`
	AgentID              uint       `gorm:"not null;uniqueIndex:uk_log_agent_status_agent_id;index:idx_log_agent_status_agent_id" json:"agentId"`
	HostID               uint       `gorm:"not null;index:idx_log_agent_status_host_id" json:"hostId"`
	RuntimeType          string     `gorm:"size:32;not null;default:host" json:"runtimeType"`
	ClusterID            uint       `gorm:"index:idx_log_agent_status_cluster_id" json:"clusterId"`
	NodeName             string     `gorm:"size:128;index:idx_log_agent_status_node_name" json:"nodeName"`
	VectorVersion        string     `gorm:"size:64" json:"vectorVersion"`
	VectorStatus         string     `gorm:"size:32;not null;default:not_installed;index:idx_log_agent_status_vector_status" json:"vectorStatus"`
	VectorPID            int        `gorm:"column:vector_pid" json:"vectorPid"`
	InstallPath          string     `gorm:"size:256" json:"installPath"`
	ConfigPath           string     `gorm:"size:256" json:"configPath"`
	DesiredConfigVersion int        `gorm:"not null;default:0" json:"desiredConfigVersion"`
	AppliedConfigVersion int        `gorm:"not null;default:0" json:"appliedConfigVersion"`
	DesiredConfigHash    string     `gorm:"size:128" json:"desiredConfigHash"`
	AppliedConfigHash    string     `gorm:"size:128" json:"appliedConfigHash"`
	ConfigStatus         string     `gorm:"size:32;not null;default:pending;index:idx_log_agent_status_config_status" json:"configStatus"`
	LastHeartbeatAt      *time.Time `gorm:"index:idx_log_agent_status_last_heartbeat_at" json:"lastHeartbeatAt"`
	LastReloadAt         *time.Time `json:"lastReloadAt"`
	LastError            string     `gorm:"type:text" json:"lastError"`
	LastErrorAt          *time.Time `json:"lastErrorAt"`
	MetricsJSON          string     `gorm:"column:metrics_json;type:text" json:"metricsJson"`
	CreatedAt            util.HTime `json:"createdAt"`
	UpdatedAt            util.HTime `json:"updatedAt"`
}

func (LogAgentStatus) TableName() string {
	return "log_agent_status"
}

type LogAgentOperation struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	AgentID       uint       `gorm:"not null;index:idx_log_agent_operation_agent_id" json:"agentId"`
	HostID        uint       `gorm:"not null;index:idx_log_agent_operation_host_id" json:"hostId"`
	OpType        string     `gorm:"size:64;not null;index:idx_log_agent_operation_op_type" json:"opType"`
	OpStatus      string     `gorm:"size:32;not null;default:pending;index:idx_log_agent_operation_op_status" json:"opStatus"`
	Progress      int        `gorm:"not null;default:0" json:"progress"`
	ProgressText  string     `gorm:"size:128" json:"progressText"`
	TargetVersion string     `gorm:"size:64" json:"targetVersion"`
	ConfigVersion int        `gorm:"not null;default:0" json:"configVersion"`
	RequestJSON   string     `gorm:"column:request_json;type:text" json:"requestJson"`
	ResultJSON    string     `gorm:"column:result_json;type:text" json:"resultJson"`
	ErrorMsg      string     `gorm:"type:text" json:"errorMsg"`
	CollectTaskID uint       `gorm:"-" json:"collectTaskId,omitempty"`
	TaskStatus    int        `gorm:"-" json:"taskStatus,omitempty"`
	Operator      string     `gorm:"size:64" json:"operator"`
	StartedAt     *time.Time `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
	CreatedAt     util.HTime `gorm:"index:idx_log_agent_operation_created_at" json:"createdAt"`
	UpdatedAt     util.HTime `json:"updatedAt"`
}

func (LogAgentOperation) TableName() string {
	return "log_agent_operation"
}

type LogK8SCollectorOperation struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	TaskID       uint       `gorm:"not null;index:idx_log_k8s_collector_op_task_id" json:"taskId"`
	TargetIndex  int        `gorm:"not null;default:0;index:idx_log_k8s_collector_op_target" json:"targetIndex"`
	ClusterID    uint       `gorm:"not null;index:idx_log_k8s_collector_op_cluster_id" json:"clusterId"`
	Namespace    string     `gorm:"size:128;not null;index:idx_log_k8s_collector_op_namespace" json:"namespace"`
	WorkloadType string     `gorm:"size:64;not null" json:"workloadType"`
	WorkloadName string     `gorm:"size:128;not null;index:idx_log_k8s_collector_op_workload" json:"workloadName"`
	K8SLogSource string     `gorm:"size:64;not null" json:"k8sLogSource"`
	OpType       string     `gorm:"size:64;not null;index:idx_log_k8s_collector_op_type" json:"opType"`
	OpStatus     string     `gorm:"size:32;not null;default:pending;index:idx_log_k8s_collector_op_status" json:"opStatus"`
	Progress     int        `gorm:"not null;default:0" json:"progress"`
	ProgressText string     `gorm:"size:128" json:"progressText"`
	RequestJSON  string     `gorm:"column:request_json;type:text" json:"requestJson"`
	ResultJSON   string     `gorm:"column:result_json;type:text" json:"resultJson"`
	ErrorMsg     string     `gorm:"type:text" json:"errorMsg"`
	Operator     string     `gorm:"size:64" json:"operator"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	CreatedAt    util.HTime `gorm:"index:idx_log_k8s_collector_op_created_at" json:"createdAt"`
	UpdatedAt    util.HTime `json:"updatedAt"`
}

func (LogK8SCollectorOperation) TableName() string {
	return "log_k8s_collector_operation"
}

type LogK8SClusterCollectorOperation struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	ClusterID       uint       `gorm:"not null;index:idx_log_k8s_cluster_collector_op_cluster_id" json:"clusterId"`
	ClusterName     string     `gorm:"size:128;not null" json:"clusterName"`
	OpType          string     `gorm:"size:64;not null;index:idx_log_k8s_cluster_collector_op_type" json:"opType"`
	OpStatus        string     `gorm:"size:32;not null;default:pending;index:idx_log_k8s_cluster_collector_op_status" json:"opStatus"`
	Progress        int        `gorm:"not null;default:0" json:"progress"`
	ProgressText    string     `gorm:"size:128" json:"progressText"`
	RequestJSON     string     `gorm:"column:request_json;type:text" json:"requestJson"`
	ResultJSON      string     `gorm:"column:result_json;type:text" json:"resultJson"`
	ErrorMsg        string     `gorm:"type:text" json:"errorMsg"`
	RuntimeStatus   string     `gorm:"-" json:"runtimeStatus,omitempty"`
	RuntimeMessage  string     `gorm:"-" json:"runtimeMessage,omitempty"`
	RuntimeProgress int        `gorm:"-" json:"runtimeProgress,omitempty"`
	RuntimeReady    bool       `gorm:"-" json:"runtimeReady"`
	Operator        string     `gorm:"size:64" json:"operator"`
	StartedAt       *time.Time `json:"startedAt"`
	FinishedAt      *time.Time `json:"finishedAt"`
	CreatedAt       util.HTime `gorm:"index:idx_log_k8s_cluster_collector_op_created_at" json:"createdAt"`
	UpdatedAt       util.HTime `json:"updatedAt"`
}

func (LogK8SClusterCollectorOperation) TableName() string {
	return "log_k8s_cluster_collector_operation"
}

type LogAgentQuery struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"pageSize"`
	Keyword      string `form:"keyword"`
	OnlineStatus int    `form:"onlineStatus"`
	VectorStatus string `form:"vectorStatus"`
	ConfigStatus string `form:"configStatus"`
}

type LogAgentOperationQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	OpType   string `form:"opType"`
	OpStatus string `form:"opStatus"`
}

type LogK8SCollectorQuery struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"pageSize"`
	ProjectID    uint   `form:"projectId"`
	StoreID      uint   `form:"storeId"`
	TaskID       uint   `form:"taskId"`
	ClusterID    uint   `form:"clusterId"`
	Namespace    string `form:"namespace"`
	WorkloadType string `form:"workloadType"`
	K8SLogSource string `form:"k8sLogSource"`
	Status       string `form:"status"`
	Keyword      string `form:"keyword"`
}

type LogK8SCollectorOperationQuery struct {
	Page        int    `form:"page"`
	PageSize    int    `form:"pageSize"`
	TaskID      uint   `form:"taskId"`
	TargetIndex *int   `form:"targetIndex"`
	ClusterID   uint   `form:"clusterId"`
	OpType      string `form:"opType"`
	OpStatus    string `form:"opStatus"`
}

type LogK8SClusterCollectorQuery struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"pageSize"`
	ClusterID uint   `form:"clusterId"`
	Status    string `form:"status"`
	Keyword   string `form:"keyword"`
}

type LogK8SClusterCollectorOperationQuery struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"pageSize"`
	ClusterID uint   `form:"clusterId"`
	OpType    string `form:"opType"`
	OpStatus  string `form:"opStatus"`
}

type LogAgentOperationRequest struct {
	OpType        string `json:"opType"`
	TargetVersion string `json:"targetVersion"`
	ConfigVersion int    `json:"configVersion"`
	RequestJSON   string `json:"requestJson"`
	Operator      string `json:"operator"`
	Force         bool   `json:"force"`
}

type LogK8SCollectorActionRequest struct {
	Operator    string `json:"operator"`
	Force       bool   `json:"force"`
	TargetIndex *int   `json:"targetIndex"`
}

type LogK8SClusterCollectorActionRequest struct {
	Operator    string `json:"operator"`
	Force       bool   `json:"force"`
	VectorImage string `json:"vectorImage"`
	Image       string `json:"image"`
}

type LogK8SCollectorListResponse struct {
	Total    int64                `json:"total"`
	List     []LogK8SCollectorVO  `json:"list"`
	Summary  LogK8SCollectorStats `json:"summary"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
}

type LogK8SClusterCollectorListResponse struct {
	Total    int64                      `json:"total"`
	List     []LogK8SClusterCollectorVO `json:"list"`
	Summary  LogK8SCollectorStats       `json:"summary"`
	Page     int                        `json:"page"`
	PageSize int                        `json:"pageSize"`
}

type LogK8SCollectorStats struct {
	Total        int64 `json:"total"`
	Running      int64 `json:"running"`
	Pending      int64 `json:"pending"`
	Failed       int64 `json:"failed"`
	NotInstalled int64 `json:"notInstalled"`
}

type LogK8SCollectorVO struct {
	TaskID             uint                       `json:"taskId"`
	TaskName           string                     `json:"taskName"`
	TaskCode           string                     `json:"taskCode"`
	TaskStatus         int                        `json:"taskStatus"`
	ProjectID          uint                       `json:"projectId"`
	StoreID            uint                       `json:"storeId"`
	TargetIndex        int                        `json:"targetIndex"`
	ClusterID          uint                       `json:"clusterId"`
	ClusterName        string                     `json:"clusterName"`
	Namespace          string                     `json:"namespace"`
	WorkloadType       string                     `json:"workloadType"`
	WorkloadName       string                     `json:"workloadName"`
	Containers         []string                   `json:"containers,omitempty"`
	K8SLogSource       string                     `json:"k8sLogSource"`
	ResourceKind       string                     `json:"resourceKind"`
	ResourceNamespace  string                     `json:"resourceNamespace"`
	ResourceName       string                     `json:"resourceName"`
	ConfigMapNamespace string                     `json:"configMapNamespace,omitempty"`
	ConfigMapName      string                     `json:"configMapName,omitempty"`
	Status             string                     `json:"status"`
	Progress           int                        `json:"progress"`
	ProgressText       string                     `json:"progressText"`
	Desired            int32                      `json:"desired"`
	Ready              int32                      `json:"ready"`
	Updated            int32                      `json:"updated"`
	Available          int32                      `json:"available"`
	ConfigStatus       string                     `json:"configStatus"`
	VectorStatus       string                     `json:"vectorStatus"`
	ConfigHash         string                     `json:"configHash,omitempty"`
	ConfigYAML         string                     `json:"configYaml,omitempty"`
	LastError          string                     `json:"lastError,omitempty"`
	LastErrorAt        *time.Time                 `json:"lastErrorAt,omitempty"`
	Components         []LogK8SCollectorComponent `json:"components,omitempty"`
	Pods               []LogK8SCollectorPod       `json:"pods,omitempty"`
	Events             []LogK8SCollectorEvent     `json:"events,omitempty"`
	LastOperation      *LogK8SCollectorOperation  `json:"lastOperation,omitempty"`
	UpdatedAt          string                     `json:"updatedAt,omitempty"`
}

type LogK8SCollectorComponent struct {
	Source             string `json:"source"`
	Name               string `json:"name"`
	ResourceKind       string `json:"resourceKind"`
	ResourceNamespace  string `json:"resourceNamespace"`
	ResourceName       string `json:"resourceName"`
	ConfigMapNamespace string `json:"configMapNamespace,omitempty"`
	ConfigMapName      string `json:"configMapName,omitempty"`
	Status             string `json:"status"`
	Progress           int    `json:"progress"`
	ProgressText       string `json:"progressText"`
	Desired            int32  `json:"desired"`
	Ready              int32  `json:"ready"`
	Updated            int32  `json:"updated"`
	Available          int32  `json:"available"`
	ConfigStatus       string `json:"configStatus"`
	VectorStatus       string `json:"vectorStatus"`
	PodCount           int    `json:"podCount"`
	ReadyPodCount      int    `json:"readyPodCount"`
	LastError          string `json:"lastError,omitempty"`
}

type LogK8SClusterCollectorVO struct {
	ClusterID         uint                             `json:"clusterId"`
	ClusterName       string                           `json:"clusterName"`
	ClusterStatus     int                              `json:"clusterStatus"`
	ResourceKind      string                           `json:"resourceKind"`
	ResourceNamespace string                           `json:"resourceNamespace"`
	ResourceName      string                           `json:"resourceName"`
	Status            string                           `json:"status"`
	Progress          int                              `json:"progress"`
	ProgressText      string                           `json:"progressText"`
	Desired           int32                            `json:"desired"`
	Ready             int32                            `json:"ready"`
	Updated           int32                            `json:"updated"`
	Available         int32                            `json:"available"`
	ConfigStatus      string                           `json:"configStatus"`
	VectorStatus      string                           `json:"vectorStatus"`
	VectorImage       string                           `json:"vectorImage"`
	ConfigHash        string                           `json:"configHash,omitempty"`
	ConfigYAML        string                           `json:"configYaml,omitempty"`
	ConfigFiles       []LogK8SVectorConfigFile         `json:"configFiles,omitempty"`
	LastError         string                           `json:"lastError,omitempty"`
	LastErrorAt       *time.Time                       `json:"lastErrorAt,omitempty"`
	Pods              []LogK8SCollectorPod             `json:"pods,omitempty"`
	Events            []LogK8SCollectorEvent           `json:"events,omitempty"`
	LastOperation     *LogK8SClusterCollectorOperation `json:"lastOperation,omitempty"`
	UpdatedAt         string                           `json:"updatedAt,omitempty"`
}

type LogK8SVectorConfigFile struct {
	Scope              string `json:"scope"`
	ProjectID          uint   `json:"projectId,omitempty"`
	ProjectName        string `json:"projectName,omitempty"`
	ProjectCode        string `json:"projectCode,omitempty"`
	ConfigMapNamespace string `json:"configMapNamespace"`
	ConfigMapName      string `json:"configMapName"`
	FileName           string `json:"fileName"`
	ConfigHash         string `json:"configHash"`
	ConfigYAML         string `json:"configYaml"`
}

type LogK8SCollectorPod struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	NodeName   string `json:"nodeName"`
	Phase      string `json:"phase"`
	Ready      bool   `json:"ready"`
	RestartSum int32  `json:"restartSum"`
	Message    string `json:"message,omitempty"`
	LastLog    string `json:"lastLog,omitempty"`
}

type LogK8SCollectorEvent struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Count     int32  `json:"count"`
	LastTime  string `json:"lastTime,omitempty"`
	ObjectRef string `json:"objectRef,omitempty"`
}

type LogCollectTaskAgentsResponse struct {
	Task     LogCollectTask             `json:"task"`
	Summary  LogCollectTaskAgentSummary `json:"summary"`
	Total    int64                      `json:"total"`
	List     []LogAgentVO               `json:"list"`
	Page     int                        `json:"page"`
	PageSize int                        `json:"pageSize"`
}

type LogCollectTaskAgentSummary struct {
	Total              int64 `json:"total"`
	Online             int64 `json:"online"`
	Offline            int64 `json:"offline"`
	VectorRunning      int64 `json:"vectorRunning"`
	VectorInstalled    int64 `json:"vectorInstalled"`
	VectorNotInstalled int64 `json:"vectorNotInstalled"`
	VectorError        int64 `json:"vectorError"`
	ConfigApplied      int64 `json:"configApplied"`
	ConfigPending      int64 `json:"configPending"`
	ConfigFailed       int64 `json:"configFailed"`
	ConfigDisabled     int64 `json:"configDisabled"`
}

type LogCollectTaskApplyRequest struct {
	Operator string `json:"operator"`
	Force    bool   `json:"force"`
}

type LogCollectTaskApplyResponse struct {
	Task    LogCollectTask              `json:"task"`
	Summary LogCollectTaskApplySummary  `json:"summary"`
	Results []LogCollectTaskApplyResult `json:"results"`
}

type LogCollectTaskApplySummary struct {
	MatchedAgents  int64 `json:"matchedAgents"`
	InstallCreated int64 `json:"installCreated"`
	ReloadCreated  int64 `json:"reloadCreated"`
	Skipped        int64 `json:"skipped"`
	Failed         int64 `json:"failed"`
}

type LogCollectTaskApplyResult struct {
	AgentID      uint     `json:"agentId"`
	HostID       uint     `json:"hostId"`
	HostName     string   `json:"hostName"`
	PrivateIP    string   `json:"privateIp"`
	VectorStatus string   `json:"vectorStatus"`
	ConfigStatus string   `json:"configStatus"`
	OpTypes      []string `json:"opTypes"`
	OperationIDs []uint   `json:"operationIds"`
	ConfigHash   string   `json:"configHash,omitempty"`
	ConfigYAML   string   `json:"configYaml,omitempty"`
	Status       string   `json:"status"`
	Message      string   `json:"message"`
}

type LogAgentVO struct {
	ID                           uint       `json:"id"`
	AgentID                      uint       `json:"agentId"`
	HostID                       uint       `json:"hostId"`
	HostName                     string     `json:"hostName"`
	Name                         string     `json:"name"`
	PrivateIP                    string     `json:"privateIp"`
	PublicIP                     string     `json:"publicIp"`
	SSHIP                        string     `json:"sshIp"`
	OS                           string     `json:"os"`
	AgentVersion                 string     `json:"agentVersion"`
	AgentStatus                  int        `json:"agentStatus"`
	AgentOnlineStatus            int        `json:"agentOnlineStatus"`
	AgentLastHeartbeat           *time.Time `json:"agentLastHeartbeat"`
	VectorVersion                string     `json:"vectorVersion"`
	VectorStatus                 string     `json:"vectorStatus"`
	VectorPID                    int        `json:"vectorPid"`
	InstallPath                  string     `json:"installPath"`
	ConfigPath                   string     `json:"configPath"`
	DesiredConfigVersion         int        `json:"desiredConfigVersion"`
	AppliedConfigVersion         int        `json:"appliedConfigVersion"`
	DesiredConfigHash            string     `json:"desiredConfigHash"`
	AppliedConfigHash            string     `json:"appliedConfigHash"`
	ConfigStatus                 string     `json:"configStatus"`
	ConfigStatusText             string     `json:"configStatusText,omitempty"`
	LastVectorHeartbeat          *time.Time `json:"lastVectorHeartbeat"`
	LastReloadAt                 *time.Time `json:"lastReloadAt"`
	LastError                    string     `json:"lastError"`
	LastErrorAt                  *time.Time `json:"lastErrorAt"`
	CollectTaskCount             int64      `json:"collectTaskCount"`
	CurrentOperationID           uint       `json:"currentOperationId"`
	CurrentOperationType         string     `json:"currentOperationType"`
	CurrentOperationStatus       string     `json:"currentOperationStatus"`
	CurrentOperationProgress     int        `json:"currentOperationProgress"`
	CurrentOperationProgressText string     `json:"currentOperationProgressText"`
}

type VectorPackageInfo struct {
	PackagePath string `json:"packagePath"`
	PackageURL  string `json:"packageUrl"`
	Exists      bool   `json:"exists"`
	FileName    string `json:"fileName"`
	SizeBytes   int64  `json:"sizeBytes"`
	SHA256      string `json:"sha256"`
	Version     string `json:"version"`
	ModTime     string `json:"modTime"`
	Error       string `json:"error,omitempty"`
}

type VectorPackageUploadTaskStatus string

const (
	VectorPackageUploadTaskPending   VectorPackageUploadTaskStatus = "pending"
	VectorPackageUploadTaskUploading VectorPackageUploadTaskStatus = "uploading"
	VectorPackageUploadTaskCompleted VectorPackageUploadTaskStatus = "completed"
	VectorPackageUploadTaskFailed    VectorPackageUploadTaskStatus = "failed"
)

type VectorPackageUploadTask struct {
	TaskID        string                        `json:"taskId"`
	FileName      string                        `json:"fileName"`
	FileSize      int64                         `json:"fileSize"`
	Replace       bool                          `json:"replace"`
	Status        VectorPackageUploadTaskStatus `json:"status"`
	Progress      float64                       `json:"progress"`
	UploadedBytes int64                         `json:"uploadedBytes"`
	TotalBytes    int64                         `json:"totalBytes"`
	Message       string                        `json:"message"`
	Error         string                        `json:"error,omitempty"`
	Package       *VectorPackageInfo            `json:"package,omitempty"`
	CreatedAt     time.Time                     `json:"createdAt"`
	StartedAt     *time.Time                    `json:"startedAt,omitempty"`
	CompletedAt   *time.Time                    `json:"completedAt,omitempty"`
}

type LogAgentConfigPreview struct {
	AgentID       uint              `json:"agentId"`
	HostID        uint              `json:"hostId"`
	HostName      string            `json:"hostName"`
	ConfigVersion int               `json:"configVersion"`
	ConfigHash    string            `json:"configHash"`
	ConfigPath    string            `json:"configPath"`
	Content       string            `json:"content"`
	Tasks         []LogCollectTask  `json:"tasks"`
	Package       VectorPackageInfo `json:"package"`
	HostAliases   []KafkaHostAlias  `json:"hostAliases,omitempty"`
}

type KafkaHostAlias struct {
	IP        string   `json:"ip"`
	Hostnames []string `json:"hostnames"`
}

type LogVectorAggregatorConfigPreview struct {
	DeliveryMode  string                               `json:"deliveryMode"`
	KafkaEnabled  bool                                 `json:"kafkaEnabled"`
	Brokers       []string                             `json:"brokers"`
	Topic         string                               `json:"topic"`
	ConsumerGroup string                               `json:"consumerGroup"`
	Image         string                               `json:"image"`
	ConfigHash    string                               `json:"configHash"`
	ConfigPath    string                               `json:"configPath"`
	Content       string                               `json:"content"`
	Docker        LogVectorAggregatorDockerPreview     `json:"docker"`
	Kubernetes    LogVectorAggregatorKubernetesPreview `json:"kubernetes"`
	Package       VectorPackageInfo                    `json:"package"`
	HostAliases   []KafkaHostAlias                     `json:"hostAliases,omitempty"`
}

type LogVectorAggregatorDockerPreview struct {
	ContainerName string `json:"containerName"`
	Image         string `json:"image"`
	ConfigPath    string `json:"configPath"`
	DataDir       string `json:"dataDir"`
	Command       string `json:"command"`
}

type LogVectorAggregatorKubernetesPreview struct {
	Namespace      string `json:"namespace"`
	DeploymentName string `json:"deploymentName"`
	ConfigMapName  string `json:"configMapName"`
	Replicas       int32  `json:"replicas"`
	FileName       string `json:"fileName"`
	ApplyCommand   string `json:"applyCommand"`
	YAML           string `json:"yaml"`
}

type LogAgentRuntimeRequest struct {
	Token          string `json:"token" form:"token"`
	AgentID        uint   `json:"agentId" form:"agentId"`
	HostID         uint   `json:"hostId" form:"hostId"`
	CurrentVersion int    `json:"currentVersion" form:"currentVersion"`
	CurrentHash    string `json:"currentHash" form:"currentHash"`
	Limit          int    `json:"limit" form:"limit"`
}

type LogAgentConfigResponse struct {
	NeedUpdate    bool              `json:"needUpdate"`
	AgentID       uint              `json:"agentId"`
	HostID        uint              `json:"hostId"`
	ConfigVersion int               `json:"configVersion"`
	ConfigHash    string            `json:"configHash"`
	ConfigPath    string            `json:"configPath"`
	Content       string            `json:"content"`
	Package       VectorPackageInfo `json:"package"`
	HostAliases   []KafkaHostAlias  `json:"hostAliases,omitempty"`
}

type LogAgentOperationDispatch struct {
	Operation LogAgentOperation       `json:"operation"`
	Package   VectorPackageInfo       `json:"package"`
	Config    *LogAgentConfigResponse `json:"config,omitempty"`
}

type LogAgentOperationDispatchResponse struct {
	AgentID    uint                        `json:"agentId"`
	HostID     uint                        `json:"hostId"`
	Operations []LogAgentOperationDispatch `json:"operations"`
}

type LogAgentStatusReportRequest struct {
	Token                string `json:"token"`
	AgentID              uint   `json:"agentId"`
	HostID               uint   `json:"hostId"`
	RuntimeType          string `json:"runtimeType"`
	ClusterID            uint   `json:"clusterId"`
	NodeName             string `json:"nodeName"`
	VectorVersion        string `json:"vectorVersion"`
	VectorStatus         string `json:"vectorStatus"`
	VectorPID            int    `json:"vectorPid"`
	InstallPath          string `json:"installPath"`
	ConfigPath           string `json:"configPath"`
	DesiredConfigVersion int    `json:"desiredConfigVersion"`
	AppliedConfigVersion int    `json:"appliedConfigVersion"`
	DesiredConfigHash    string `json:"desiredConfigHash"`
	AppliedConfigHash    string `json:"appliedConfigHash"`
	ConfigStatus         string `json:"configStatus"`
	LastError            string `json:"lastError"`
	MetricsJSON          string `json:"metricsJson"`
}

type LogAgentOperationReportRequest struct {
	Token        string `json:"token"`
	AgentID      uint   `json:"agentId"`
	HostID       uint   `json:"hostId"`
	OpStatus     string `json:"opStatus"`
	Progress     int    `json:"progress"`
	ProgressText string `json:"progressText"`
	ResultJSON   string `json:"resultJson"`
	ErrorMsg     string `json:"errorMsg"`
}
