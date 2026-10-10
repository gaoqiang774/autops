package model

// BigDataComponentRule describes how a big data component is matched from K8s names.
type BigDataComponentRule struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Category    string   `json:"category"`
	Prefixes    []string `json:"prefixes"`
}

// BigDataAppTreeResponse is the left tree payload for the big data app page.
type BigDataAppTreeResponse struct {
	ClusterID   uint                   `json:"clusterId"`
	ClusterName string                 `json:"clusterName"`
	Namespace   string                 `json:"namespace,omitempty"`
	Summary     BigDataAppSummary      `json:"summary"`
	Tree        []BigDataAppTreeNode   `json:"tree"`
	Components  []BigDataComponent     `json:"components"`
	Rules       []BigDataComponentRule `json:"rules"`
}

type BigDataAppSummary struct {
	Namespaces        int `json:"namespaces"`
	Components        int `json:"components"`
	Workloads         int `json:"workloads"`
	Pods              int `json:"pods"`
	ReadyPods         int `json:"readyPods"`
	AbnormalPods      int `json:"abnormalPods"`
	RunningWorkloads  int `json:"runningWorkloads"`
	AbnormalWorkloads int `json:"abnormalWorkloads"`
}

type BigDataAppTreeNode struct {
	ID           string                 `json:"id"`
	Label        string                 `json:"label"`
	Type         string                 `json:"type"`
	Status       string                 `json:"status,omitempty"`
	Namespace    string                 `json:"namespace,omitempty"`
	Component    string                 `json:"component,omitempty"`
	WorkloadType string                 `json:"workloadType,omitempty"`
	WorkloadName string                 `json:"workloadName,omitempty"`
	PodName      string                 `json:"podName,omitempty"`
	Children     []BigDataAppTreeNode   `json:"children,omitempty"`
	Meta         map[string]interface{} `json:"meta,omitempty"`
}

type BigDataComponent struct {
	Name        string                  `json:"name"`
	DisplayName string                  `json:"displayName"`
	Category    string                  `json:"category"`
	Status      string                  `json:"status"`
	Namespaces  []string                `json:"namespaces"`
	Workloads   []K8sWorkload           `json:"workloads"`
	Pods        []K8sPodInfo            `json:"pods"`
	Summary     BigDataComponentSummary `json:"summary"`
}

type BigDataComponentSummary struct {
	Workloads       int   `json:"workloads"`
	Pods            int   `json:"pods"`
	RunningPods     int   `json:"runningPods"`
	ReadyPods       int   `json:"readyPods"`
	AbnormalPods    int   `json:"abnormalPods"`
	DesiredReplicas int32 `json:"desiredReplicas"`
	CurrentReplicas int32 `json:"currentReplicas"`
	ReadyReplicas   int32 `json:"readyReplicas"`
	RestartCount    int32 `json:"restartCount"`
}

type BigDataComponentDetailResponse struct {
	ClusterID  uint                     `json:"clusterId"`
	Component  BigDataComponent         `json:"component"`
	Containers []BigDataContainerDetail `json:"containers"`
	Metrics    BigDataComponentMetrics  `json:"metrics"`
}

type BigDataContainerDetail struct {
	PodName   string                `json:"podName"`
	Namespace string                `json:"namespace"`
	NodeName  string                `json:"nodeName"`
	Container ContainerInfo         `json:"container"`
	Metrics   *ContainerMetricsInfo `json:"metrics,omitempty"`
}

type BigDataComponentMetrics struct {
	Available  bool                   `json:"available"`
	Message    string                 `json:"message,omitempty"`
	Pods       []PodMetricsInfo       `json:"pods"`
	TotalUsage ResourceUsage          `json:"totalUsage"`
	History    *ResourceMetricHistory `json:"history,omitempty"`
}

type BigDataActionRequest struct {
	Action        string   `json:"action" binding:"required"` // start,stop,restart,quickRestart,scale,delete
	Replicas      *int32   `json:"replicas,omitempty"`
	PodNames      []string `json:"podNames,omitempty"`
	ContainerName string   `json:"containerName,omitempty"`
	Force         bool     `json:"force,omitempty"`
	Reason        string   `json:"reason,omitempty"`
}

type BigDataActionResponse struct {
	ClusterID uint                `json:"clusterId"`
	Namespace string              `json:"namespace,omitempty"`
	Component string              `json:"component,omitempty"`
	Action    string              `json:"action"`
	Success   bool                `json:"success"`
	Message   string              `json:"message"`
	Items     []BigDataActionItem `json:"items"`
}

type BigDataActionItem struct {
	Namespace        string `json:"namespace,omitempty"`
	ResourceType     string `json:"resourceType"`
	ResourceName     string `json:"resourceName"`
	Action           string `json:"action"`
	Status           string `json:"status"` // success,failed,skipped
	Message          string `json:"message,omitempty"`
	PreviousReplicas *int32 `json:"previousReplicas,omitempty"`
	TargetReplicas   *int32 `json:"targetReplicas,omitempty"`
}
