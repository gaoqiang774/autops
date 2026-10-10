package model

type WindowsDashboardMetrics struct {
	HostID             uint                     `json:"hostId"`
	HostName           string                   `json:"hostName"`
	IP                 string                   `json:"ip"`
	Hostname           string                   `json:"hostname"`
	OSProduct          string                   `json:"osProduct"`
	OnlineStatus       int                      `json:"onlineStatus"`
	Overview           WindowsDashboardOverview `json:"overview"`
	Trends             WindowsDashboardTrends   `json:"trends"`
	Disks              []WindowsDiskVolume      `json:"disks"`
	TopCPUProcesses    []WindowsProcessItem     `json:"topCpuProcesses"`
	TopMemoryProcesses []WindowsProcessItem     `json:"topMemoryProcesses"`
	KeyServices        []WindowsServiceStatus   `json:"keyServices"`
	UpdateTime         int64                    `json:"updateTime"`
}

type WindowsDashboardOverview struct {
	CPUUsage               float64 `json:"cpuUsage"`
	CPUCores               int     `json:"cpuCores"`
	CPUQueueLength         float64 `json:"cpuQueueLength"`
	MemoryUsage            float64 `json:"memoryUsage"`
	MemoryUsed             uint64  `json:"memoryUsed"`
	MemoryUsedStr          string  `json:"memoryUsedStr"`
	MemoryTotal            uint64  `json:"memoryTotal"`
	MemoryTotalStr         string  `json:"memoryTotalStr"`
	DiskUsage              float64 `json:"diskUsage"`
	DiskUsed               uint64  `json:"diskUsed"`
	DiskUsedStr            string  `json:"diskUsedStr"`
	DiskTotal              uint64  `json:"diskTotal"`
	DiskTotalStr           string  `json:"diskTotalStr"`
	NetworkReceiveKBPerSec float64 `json:"networkReceiveKbPerSec"`
	NetworkSendKBPerSec    float64 `json:"networkSendKbPerSec"`
	ProcessCount           int     `json:"processCount"`
	ServiceCount           int     `json:"serviceCount"`
	RunningServiceCount    int     `json:"runningServiceCount"`
	Uptime                 float64 `json:"uptime"`
	UptimeStr              string  `json:"uptimeStr"`
}

type WindowsDashboardTrends struct {
	CPU            []MetricDataPoint `json:"cpu"`
	CPUQueueLength []MetricDataPoint `json:"cpuQueueLength"`
	MemoryUsage    []MetricDataPoint `json:"memoryUsage"`
	MemoryUsedGB   []MetricDataPoint `json:"memoryUsedGb"`
	DiskUsage      []MetricDataPoint `json:"diskUsage"`
	DiskReadKB     []MetricDataPoint `json:"diskReadKb"`
	DiskWriteKB    []MetricDataPoint `json:"diskWriteKb"`
	NetworkReceive []MetricDataPoint `json:"networkReceive"`
	NetworkSend    []MetricDataPoint `json:"networkSend"`
	ProcessCount   []MetricDataPoint `json:"processCount"`
}

type WindowsDiskVolume struct {
	Volume        string  `json:"volume"`
	VolumeName    string  `json:"volumeName"`
	FileSystem    string  `json:"fileSystem"`
	Type          string  `json:"type"`
	UsageRate     float64 `json:"usageRate"`
	Used          uint64  `json:"used"`
	UsedStr       string  `json:"usedStr"`
	Free          uint64  `json:"free"`
	FreeStr       string  `json:"freeStr"`
	Total         uint64  `json:"total"`
	TotalStr      string  `json:"totalStr"`
	ReadKBPerSec  float64 `json:"readKbPerSec"`
	WriteKBPerSec float64 `json:"writeKbPerSec"`
}

type WindowsProcessItem struct {
	PID         uint    `json:"pid"`
	Name        string  `json:"name"`
	CPUUsage    float64 `json:"cpuUsage"`
	MemoryBytes uint64  `json:"memoryBytes"`
	MemoryStr   string  `json:"memoryStr"`
}

type WindowsServiceStatus struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Running     bool   `json:"running"`
	Status      string `json:"status"`
}
