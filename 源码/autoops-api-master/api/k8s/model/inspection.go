package model

import "time"

const (
	InspectionStatusRunning   = "running"
	InspectionStatusCompleted = "completed"
	InspectionStatusFailed    = "failed"

	CheckStatusSuccess = "success"
	CheckStatusWarning = "warning"
	CheckStatusError   = "error"
)

type ClusterInspection struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	ClusterID         uint       `gorm:"not null;index" json:"clusterId"`
	ClusterName       string     `gorm:"size:100" json:"clusterName"`
	Status            string     `gorm:"size:20;index" json:"status"`
	Progress          int        `gorm:"default:0" json:"progress"`
	CurrentStep       string     `gorm:"size:255" json:"currentStep"`
	CompletedClusters int        `gorm:"default:0" json:"completedClusters"`
	TotalClusters     int        `gorm:"default:1" json:"totalClusters"`
	Score             int        `json:"score"`
	CheckCount        int        `json:"checkCount"`
	PassCount         int        `json:"passCount"`
	WarningCount      int        `json:"warningCount"`
	FailCount         int        `json:"failCount"`
	Duration          int        `json:"duration"`
	ReportData        string     `gorm:"type:longtext" json:"reportData"`
	UserID            uint       `gorm:"index" json:"userId"`
	StartTime         time.Time  `json:"startTime"`
	EndTime           *time.Time `json:"endTime"`
	CreatedAt         time.Time  `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime" json:"updatedAt"`
}

func (ClusterInspection) TableName() string {
	return "k8s_cluster_inspections"
}

type InspectionResult struct {
	ClusterID   uint              `json:"clusterId"`
	ClusterName string            `json:"clusterName"`
	Score       int               `json:"score"`
	Summary     InspectionSummary `json:"summary"`
	ClusterInfo ClusterInfoResult `json:"clusterInfo"`
	NodeHealth  NodeHealthResult  `json:"nodeHealth"`
	Components  ComponentsResult  `json:"components"`
	Workloads   WorkloadsResult   `json:"workloads"`
	Network     NetworkResult     `json:"network"`
	Storage     StorageResult     `json:"storage"`
	Security    SecurityResult    `json:"security"`
	Config      ConfigResult      `json:"config"`
	Capacity    CapacityResult    `json:"capacity"`
	Events      EventsResult      `json:"events"`
}

type InspectionSummary struct {
	TotalChecks   int `json:"totalChecks"`
	PassedChecks  int `json:"passedChecks"`
	WarningChecks int `json:"warningChecks"`
	FailedChecks  int `json:"failedChecks"`
	Duration      int `json:"duration"`
}

type CheckItem struct {
	Category   string `json:"category"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Value      string `json:"value"`
	Expected   string `json:"expected"`
	Detail     string `json:"detail"`
	Suggestion string `json:"suggestion"`
}

type ClusterInfoResult struct {
	Version         string      `json:"version"`
	Platform        string      `json:"platform"`
	GitVersion      string      `json:"gitVersion"`
	GoVersion       string      `json:"goVersion"`
	BuildDate       string      `json:"buildDate"`
	ConnectionState string      `json:"connectionState"`
	ConnectionDelay int64       `json:"connectionDelay"`
	Items           []CheckItem `json:"items"`
}

type NodeHealthResult struct {
	TotalNodes      int            `json:"totalNodes"`
	ReadyNodes      int            `json:"readyNodes"`
	NotReadyNodes   int            `json:"notReadyNodes"`
	PressureNodes   int            `json:"pressureNodes"`
	TaintedNodes    int            `json:"taintedNodes"`
	NodeUtilization []NodeResource `json:"nodeUtilization"`
	Items           []CheckItem    `json:"items"`
}

type NodeResource struct {
	Name            string  `json:"name"`
	CPUCapacity     string  `json:"cpuCapacity"`
	CPUUsed         string  `json:"cpuUsed"`
	CPUUsagePercent float64 `json:"cpuUsagePercent"`
	MemoryCapacity  string  `json:"memoryCapacity"`
	MemoryUsed      string  `json:"memoryUsed"`
	MemoryPercent   float64 `json:"memoryPercent"`
	PodCount        int     `json:"podCount"`
	PodCapacity     int     `json:"podCapacity"`
	Status          string  `json:"status"`
}

type ComponentsResult struct {
	ControlPlane []ComponentStatus `json:"controlPlane"`
	Addons       []ComponentStatus `json:"addons"`
	Items        []CheckItem       `json:"items"`
}

type ComponentStatus struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Ready     string `json:"ready"`
	Restarts  int    `json:"restarts"`
	Age       string `json:"age"`
}

type WorkloadsResult struct {
	TotalDeployments    int            `json:"totalDeployments"`
	HealthyDeployments  int            `json:"healthyDeployments"`
	TotalDaemonSets     int            `json:"totalDaemonSets"`
	HealthyDaemonSets   int            `json:"healthyDaemonSets"`
	TotalStatefulSets   int            `json:"totalStatefulSets"`
	HealthyStatefulSets int            `json:"healthyStatefulSets"`
	TotalPods           int            `json:"totalPods"`
	RunningPods         int            `json:"runningPods"`
	PendingPods         int            `json:"pendingPods"`
	FailedPods          int            `json:"failedPods"`
	HighRestartPods     int            `json:"highRestartPods"`
	ImagePullErrors     int            `json:"imagePullErrors"`
	PodsByPhase         map[string]int `json:"podsByPhase"`
	Items               []CheckItem    `json:"items"`
	UnhealthyWorkloads  []WorkloadInfo `json:"unhealthyWorkloads"`
}

type WorkloadInfo struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Ready     string `json:"ready"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
}

type NetworkResult struct {
	TotalServices        int         `json:"totalServices"`
	ClusterIPServices    int         `json:"clusterIPServices"`
	NodePortServices     int         `json:"nodePortServices"`
	LoadBalancerServices int         `json:"loadBalancerServices"`
	NoEndpointServices   int         `json:"noEndpointServices"`
	TotalIngresses       int         `json:"totalIngresses"`
	NetworkPolicies      int         `json:"networkPolicies"`
	Items                []CheckItem `json:"items"`
}

type StorageResult struct {
	TotalPVs       int         `json:"totalPVs"`
	AvailablePVs   int         `json:"availablePVs"`
	BoundPVs       int         `json:"boundPVs"`
	ReleasedPVs    int         `json:"releasedPVs"`
	FailedPVs      int         `json:"failedPVs"`
	TotalPVCs      int         `json:"totalPVCs"`
	BoundPVCs      int         `json:"boundPVCs"`
	PendingPVCs    int         `json:"pendingPVCs"`
	StorageClasses int         `json:"storageClasses"`
	DefaultSC      string      `json:"defaultSC"`
	Items          []CheckItem `json:"items"`
}

type SecurityResult struct {
	ServiceAccounts      int         `json:"serviceAccounts"`
	Roles                int         `json:"roles"`
	ClusterRoles         int         `json:"clusterRoles"`
	RoleBindings         int         `json:"roleBindings"`
	ClusterRoleBindings  int         `json:"clusterRoleBindings"`
	PrivilegedPods       int         `json:"privilegedPods"`
	HostNetworkPods      int         `json:"hostNetworkPods"`
	RootUserContainers   int         `json:"rootUserContainers"`
	ClusterAdminBindings int         `json:"clusterAdminBindings"`
	Items                []CheckItem `json:"items"`
}

type ConfigResult struct {
	TotalConfigMaps    int         `json:"totalConfigMaps"`
	TotalSecrets       int         `json:"totalSecrets"`
	NamespaceCount     int         `json:"namespaceCount"`
	ResourceQuotaCount int         `json:"resourceQuotaCount"`
	LimitRangeCount    int         `json:"limitRangeCount"`
	NoRequestLimitPods int         `json:"noRequestLimitPods"`
	Items              []CheckItem `json:"items"`
}

type CapacityResult struct {
	TotalCPU              string      `json:"totalCPU"`
	AllocatedCPU          string      `json:"allocatedCPU"`
	CPUAllocatePercent    float64     `json:"cpuAllocatePercent"`
	TotalMemory           string      `json:"totalMemory"`
	AllocatedMemory       string      `json:"allocatedMemory"`
	MemoryAllocatePercent float64     `json:"memoryAllocatePercent"`
	TotalPodCapacity      int         `json:"totalPodCapacity"`
	CurrentPodCount       int         `json:"currentPodCount"`
	PodDensityPercent     float64     `json:"podDensityPercent"`
	Items                 []CheckItem `json:"items"`
}

type EventsResult struct {
	WarningEvents  int                   `json:"warningEvents"`
	ErrorEvents    int                   `json:"errorEvents"`
	RecentEvents   []InspectionEventInfo `json:"recentEvents"`
	HighFreqEvents []InspectionEventInfo `json:"highFreqEvents"`
	Items          []CheckItem           `json:"items"`
}

type InspectionEventInfo struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Object    string `json:"object"`
	Namespace string `json:"namespace"`
	Count     int    `json:"count"`
	LastSeen  string `json:"lastSeen"`
}

type StartInspectionRequest struct {
	ClusterIDs []uint            `json:"clusterIds"`
	Options    InspectionOptions `json:"options"`
	UserID     uint              `json:"userId"`
}

type InspectionOptions struct {
	CheckCluster   bool `json:"checkCluster"`
	CheckNodes     bool `json:"checkNodes"`
	CheckWorkloads bool `json:"checkWorkloads"`
	CheckNetwork   bool `json:"checkNetwork"`
	CheckStorage   bool `json:"checkStorage"`
	CheckSecurity  bool `json:"checkSecurity"`
	CheckConfig    bool `json:"checkConfig"`
	CheckCapacity  bool `json:"checkCapacity"`
	CheckEvents    bool `json:"checkEvents"`
}

type InspectionProgress struct {
	InspectionID      uint   `json:"inspectionId"`
	Status            string `json:"status"`
	Progress          int    `json:"progress"`
	CurrentStep       string `json:"currentStep"`
	CompletedClusters int    `json:"completedClusters"`
	TotalClusters     int    `json:"totalClusters"`
}

type InspectionHistoryItem struct {
	ID           uint   `json:"id"`
	ClusterID    uint   `json:"clusterId"`
	ClusterName  string `json:"clusterName"`
	Score        int    `json:"score"`
	Status       string `json:"status"`
	CheckCount   int    `json:"checkCount"`
	PassCount    int    `json:"passCount"`
	WarningCount int    `json:"warningCount"`
	FailCount    int    `json:"failCount"`
	Duration     int    `json:"duration"`
	CreatedAt    string `json:"createdAt"`
}
