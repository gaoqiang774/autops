package model

type CmdbDashboardCards struct {
	Asset      CmdbDashboardAssetCard     `json:"asset"`
	Location   CmdbDashboardLocationCard  `json:"location"`
	Monitor    CmdbDashboardMonitorCard   `json:"monitor"`
	Warranty   CmdbDashboardWarrantyCard  `json:"warranty"`
	CloudHost  CmdbDashboardCloudHostCard `json:"cloudHost"`
	Auth       CmdbDashboardAuthCard      `json:"auth"`
	UpdateTime int64                      `json:"updateTime"`
}

type CmdbDashboardAssetCard struct {
	PhysicalTotal int64 `json:"physicalTotal"`
	VmTotal       int64 `json:"vmTotal"`
	SwitchTotal   int64 `json:"switchTotal"`
	Total         int64 `json:"total"`
}

type CmdbDashboardLocationCard struct {
	RoomTotal    int64 `json:"roomTotal"`
	CabinetTotal int64 `json:"cabinetTotal"`
}

type CmdbDashboardMonitorCard struct {
	HardwareCovered       int64   `json:"hardwareCovered"`
	HardwareTotal         int64   `json:"hardwareTotal"`
	HardwareCoverageRate  float64 `json:"hardwareCoverageRate"`
	SystemCovered         int64   `json:"systemCovered"`
	SystemTotal           int64   `json:"systemTotal"`
	SystemCoverageRate    float64 `json:"systemCoverageRate"`
	AgentTotal            int64   `json:"agentTotal"`
	RunningAgentTotal     int64   `json:"runningAgentTotal"`
	AgentInstallCoverRate float64 `json:"agentInstallCoverRate"`
}

type CmdbDashboardWarrantyCard struct {
	WindowDays        int     `json:"windowDays"`
	DueCount          int64   `json:"dueCount"`
	DueRate           float64 `json:"dueRate"`
	ExpiredCount      int64   `json:"expiredCount"`
	ExpiringSoonCount int64   `json:"expiringSoonCount"`
	Total             int64   `json:"total"`
	Deadline          string  `json:"deadline"`
}

type CmdbDashboardCloudHostCard struct {
	AliyunTotal  int64 `json:"aliyunTotal"`
	TencentTotal int64 `json:"tencentTotal"`
	Total        int64 `json:"total"`
}

type CmdbDashboardAuthCard struct {
	AuthenticatedTotal   int64   `json:"authenticatedTotal"`
	UnauthenticatedTotal int64   `json:"unauthenticatedTotal"`
	FailedTotal          int64   `json:"failedTotal"`
	HostTotal            int64   `json:"hostTotal"`
	AuthRate             float64 `json:"authRate"`
}

type CmdbDashboardCharts struct {
	BrandDistribution   []CmdbDashboardChartPoint `json:"brandDistribution"`
	RoomPhysicalAssets  []CmdbDashboardRoomBar    `json:"roomPhysicalAssets"`
	PurposeDistribution []CmdbDashboardChartPoint `json:"purposeDistribution"`
	UpdateTime          int64                     `json:"updateTime"`
}

type CmdbDashboardChartPoint struct {
	Name  string  `json:"name"`
	Value int64   `json:"value"`
	Rate  float64 `json:"rate"`
}

type CmdbDashboardRoomBar struct {
	RoomID       uint    `json:"roomId"`
	RoomName     string  `json:"roomName"`
	AssetCount   int64   `json:"assetCount"`
	CabinetCount int64   `json:"cabinetCount"`
	Rate         float64 `json:"rate"`
}

type CmdbDashboardTables struct {
	HighestUsage       CmdbDashboardUsageRankings       `json:"highestUsage"`
	LowestUsage        CmdbDashboardUsageRankings       `json:"lowestUsage"`
	HardwareExceptions []CmdbDashboardHardwareException `json:"hardwareExceptions"`
	HardwareScan       CmdbDashboardHardwareScan        `json:"hardwareScan"`
	Warnings           []string                         `json:"warnings"`
	UpdateTime         int64                            `json:"updateTime"`
}

type CmdbDashboardUsageRankings struct {
	CPU    []CmdbDashboardUsageRankItem `json:"cpu"`
	Memory []CmdbDashboardUsageRankItem `json:"memory"`
	Disk   []CmdbDashboardUsageRankItem `json:"disk"`
}

type CmdbDashboardUsageRankItem struct {
	HostID   uint    `json:"hostId"`
	HostName string  `json:"hostName"`
	Instance string  `json:"instance"`
	IP       string  `json:"ip"`
	Value    float64 `json:"value"`
	Status   string  `json:"status"`
}

type CmdbDashboardHardwareException struct {
	AssetID              uint     `json:"assetId"`
	IP                   string   `json:"ip"`
	RoomName             string   `json:"roomName"`
	CabinetName          string   `json:"cabinetName"`
	Brand                string   `json:"brand"`
	Model                string   `json:"model"`
	RemoteManagementCard string   `json:"remoteManagementCard"`
	Vendor               string   `json:"vendor"`
	Level                string   `json:"level"`
	CriticalCount        int      `json:"criticalCount"`
	WarningCount         int      `json:"warningCount"`
	Metrics              []string `json:"metrics"`
	Message              string   `json:"message"`
}

type CmdbDashboardHardwareScan struct {
	EnabledAssetTotal int64 `json:"enabledAssetTotal"`
	ExceptionTotal    int64 `json:"exceptionTotal"`
	ReturnedTotal     int   `json:"returnedTotal"`
	LookbackMinutes   int   `json:"lookbackMinutes"`
	Limit             int   `json:"limit"`
}
