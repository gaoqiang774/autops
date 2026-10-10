package model

import "dodevops-api/common/util"

// CmdbIdcAsset 机房物理机资产
type CmdbIdcAsset struct {
	ID                   uint           `gorm:"column:id;primaryKey;not null" json:"id"`
	RoomID               uint           `gorm:"column:room_id;not null;index" json:"roomId"`
	Room                 CmdbIdcRoom    `gorm:"foreignKey:RoomID" json:"-"`
	CabinetID            uint           `gorm:"column:cabinet_id;default:0;index" json:"cabinetId"`
	Cabinet              CmdbIdcCabinet `gorm:"foreignKey:CabinetID" json:"-"`
	CabinetPosition      int            `gorm:"column:cabinet_position;default:0" json:"cabinetPosition"`
	CabinetUSize         int            `gorm:"column:cabinet_u_size;default:1" json:"cabinetUSize"`
	IP                   string         `gorm:"column:ip;size:64;not null;uniqueIndex:uk_cmdb_idc_asset_ip" json:"ip"`
	Brand                string         `gorm:"column:brand;size:100" json:"brand"`
	Model                string         `gorm:"column:model;size:100" json:"model"`
	CPU                  string         `gorm:"column:cpu;size:255" json:"cpu"`
	Disk                 string         `gorm:"column:disk;size:255" json:"disk"`
	Memory               string         `gorm:"column:memory;size:100" json:"memory"`
	Purpose              string         `gorm:"column:purpose;size:255" json:"purpose"`
	RemoteManagementCard string         `gorm:"column:remote_management_card;size:100" json:"remoteManagementCard"`
	Remark               string         `gorm:"column:remark;size:1000" json:"remark"`
	PurchaseDate         string         `gorm:"column:purchase_date;size:20" json:"purchaseDate"`
	WarrantyStartDate    string         `gorm:"column:warranty_start_date;size:20" json:"warrantyStartDate"`
	WarrantyEndDate      string         `gorm:"column:warranty_end_date;size:20" json:"warrantyEndDate"`
	UsageYears           float64        `gorm:"column:usage_years;type:decimal(10,2);default:0" json:"usageYears"`
	AssetEvaluation      string         `gorm:"column:asset_evaluation;size:1000" json:"assetEvaluation"`
	CreateTime           util.HTime     `gorm:"column:create_time;not null" json:"createTime"`
	UpdateTime           util.HTime     `gorm:"column:update_time;not null" json:"updateTime"`
}

func (CmdbIdcAsset) TableName() string {
	return "cmdb_idc_asset"
}

type CreateCmdbIdcAssetDto struct {
	RoomID               uint    `json:"roomId"`
	RoomName             string  `json:"roomName"`
	CabinetID            uint    `json:"cabinetId"`
	CabinetName          string  `json:"cabinetName"`
	CabinetPosition      int     `json:"cabinetPosition"`
	CabinetUSize         int     `json:"cabinetUSize"`
	IP                   string  `json:"ip" binding:"required"`
	Brand                string  `json:"brand"`
	Model                string  `json:"model"`
	CPU                  string  `json:"cpu"`
	Disk                 string  `json:"disk"`
	Memory               string  `json:"memory"`
	Purpose              string  `json:"purpose"`
	RemoteManagementCard string  `json:"remoteManagementCard"`
	Remark               string  `json:"remark"`
	PurchaseDate         string  `json:"purchaseDate"`
	WarrantyStartDate    string  `json:"warrantyStartDate"`
	WarrantyEndDate      string  `json:"warrantyEndDate"`
	UsageYears           float64 `json:"usageYears"`
	AssetEvaluation      string  `json:"assetEvaluation"`
}

type UpdateCmdbIdcAssetDto struct {
	ID                   uint    `json:"id" binding:"required"`
	RoomID               uint    `json:"roomId"`
	RoomName             string  `json:"roomName"`
	CabinetID            uint    `json:"cabinetId"`
	CabinetName          string  `json:"cabinetName"`
	CabinetPosition      int     `json:"cabinetPosition"`
	CabinetUSize         int     `json:"cabinetUSize"`
	IP                   string  `json:"ip" binding:"required"`
	Brand                string  `json:"brand"`
	Model                string  `json:"model"`
	CPU                  string  `json:"cpu"`
	Disk                 string  `json:"disk"`
	Memory               string  `json:"memory"`
	Purpose              string  `json:"purpose"`
	RemoteManagementCard string  `json:"remoteManagementCard"`
	Remark               string  `json:"remark"`
	PurchaseDate         string  `json:"purchaseDate"`
	WarrantyStartDate    string  `json:"warrantyStartDate"`
	WarrantyEndDate      string  `json:"warrantyEndDate"`
	UsageYears           float64 `json:"usageYears"`
	AssetEvaluation      string  `json:"assetEvaluation"`
}

type CmdbIdcAssetIdDto struct {
	ID uint `json:"id" binding:"required"`
}

type UpdateCmdbIdcAssetPositionDto struct {
	ID              uint `json:"id" binding:"required"`
	CabinetID       uint `json:"cabinetId"`
	CabinetPosition int  `json:"cabinetPosition"`
	CabinetUSize    int  `json:"cabinetUSize"`
}

type CmdbIdcAssetQuery struct {
	Page      int    `form:"page" binding:"required,min=1"`
	PageSize  int    `form:"pageSize" binding:"required,min=1,max=200"`
	RoomID    uint   `form:"roomId"`
	CabinetID uint   `form:"cabinetId"`
	Keyword   string `form:"keyword"`
	SortBy    string `form:"sortBy"`
	SortOrder string `form:"sortOrder"`
}

type CmdbIdcAssetVo struct {
	ID                   uint       `json:"id"`
	RoomID               uint       `json:"roomId"`
	RoomName             string     `json:"roomName"`
	CabinetID            uint       `json:"cabinetId"`
	CabinetName          string     `json:"cabinetName"`
	CabinetPosition      int        `json:"cabinetPosition"`
	CabinetUSize         int        `json:"cabinetUSize"`
	VmCount              int64      `json:"vmCount"`
	IP                   string     `json:"ip"`
	Brand                string     `json:"brand"`
	Model                string     `json:"model"`
	CPU                  string     `json:"cpu"`
	Disk                 string     `json:"disk"`
	Memory               string     `json:"memory"`
	Purpose              string     `json:"purpose"`
	RemoteManagementCard string     `json:"remoteManagementCard"`
	Remark               string     `json:"remark"`
	PurchaseDate         string     `json:"purchaseDate"`
	WarrantyStartDate    string     `json:"warrantyStartDate"`
	WarrantyEndDate      string     `json:"warrantyEndDate"`
	UsageYears           float64    `json:"usageYears"`
	AssetEvaluation      string     `json:"assetEvaluation"`
	CreateTime           util.HTime `json:"createTime"`
	UpdateTime           util.HTime `json:"updateTime"`
}

type CmdbIdcAssetHardwareMonitorQuery struct {
	ID                   uint   `form:"id"`
	RemoteManagementCard string `form:"remoteManagementCard"`
	LookbackMinutes      int    `form:"lookbackMinutes"`
	TrendHours           int    `form:"trendHours"`
	TrendStepMinutes     int    `form:"trendStepMinutes"`
}

type CmdbIdcAssetHardwareSourceVo struct {
	Job              string `json:"job"`
	MatchedLabel     string `json:"matchedLabel"`
	PrometheusTarget string `json:"prometheusTarget"`
}

type CmdbIdcAssetHardwareSummaryVo struct {
	OverallStatus             string  `json:"overallStatus"`
	CPUStatus                 string  `json:"cpuStatus"`
	MemoryStatus              string  `json:"memoryStatus"`
	DiskStatus                string  `json:"diskStatus"`
	FanStatus                 string  `json:"fanStatus"`
	PowerStatus               string  `json:"powerStatus"`
	TemperatureStatus         string  `json:"temperatureStatus"`
	CPUUsagePercent           float64 `json:"cpuUsagePercent"`
	MemoryUsagePercent        float64 `json:"memoryUsagePercent"`
	PowerWatts                float64 `json:"powerWatts"`
	HighestTemperatureCelsius float64 `json:"highestTemperatureCelsius"`
}

type CmdbIdcAssetHardwareComponentVo struct {
	Name      string   `json:"name"`
	Type      string   `json:"type,omitempty"`
	Status    string   `json:"status"`
	Index     string   `json:"index,omitempty"`
	Value     *float64 `json:"value,omitempty"`
	Unit      string   `json:"unit,omitempty"`
	RawMetric string   `json:"rawMetric,omitempty"`
}

type CmdbIdcAssetHardwareComponentsVo struct {
	Temperatures    []CmdbIdcAssetHardwareComponentVo `json:"temperatures"`
	Fans            []CmdbIdcAssetHardwareComponentVo `json:"fans"`
	PowerSupplies   []CmdbIdcAssetHardwareComponentVo `json:"powerSupplies"`
	Voltages        []CmdbIdcAssetHardwareComponentVo `json:"voltages"`
	Currents        []CmdbIdcAssetHardwareComponentVo `json:"currents"`
	Sensors         []CmdbIdcAssetHardwareComponentVo `json:"sensors"`
	Disks           []CmdbIdcAssetHardwareComponentVo `json:"disks"`
	RaidControllers []CmdbIdcAssetHardwareComponentVo `json:"raidControllers"`
}

type CmdbIdcAssetHardwareTrendSeriesVo struct {
	Name      string       `json:"name"`
	Metric    string       `json:"metric"`
	ValueType string       `json:"valueType,omitempty"`
	Unit      string       `json:"unit,omitempty"`
	Axis      string       `json:"axis,omitempty"`
	Component string       `json:"component,omitempty"`
	Points    []TrendPoint `json:"points"`
}

type CmdbIdcAssetHardwareTrendPanelVo struct {
	Key         string                              `json:"key"`
	Title       string                              `json:"title"`
	Category    string                              `json:"category"`
	ChartType   string                              `json:"chartType"`
	Status      string                              `json:"status,omitempty"`
	WindowHours int                                 `json:"windowHours"`
	StepMinutes int                                 `json:"stepMinutes"`
	Series      []CmdbIdcAssetHardwareTrendSeriesVo `json:"series"`
}

type CmdbIdcAssetHardwareTrendsVo struct {
	WindowHours int                                `json:"windowHours"`
	StepMinutes int                                `json:"stepMinutes"`
	Panels      []CmdbIdcAssetHardwareTrendPanelVo `json:"panels"`
}

type CmdbIdcAssetHardwareAlertVo struct {
	Severity        string   `json:"severity"`
	Category        string   `json:"category"`
	Metric          string   `json:"metric"`
	Component       string   `json:"component,omitempty"`
	Message         string   `json:"message"`
	Recommendation  string   `json:"recommendation,omitempty"`
	Evidence        string   `json:"evidence,omitempty"`
	CurrentValue    *float64 `json:"currentValue,omitempty"`
	Threshold       *float64 `json:"threshold,omitempty"`
	DurationMinutes int      `json:"durationMinutes,omitempty"`
}

type CmdbIdcAssetHardwareHealthVo struct {
	Score           int      `json:"score"`
	Level           string   `json:"level"`
	Conclusion      string   `json:"conclusion"`
	RiskCategories  []string `json:"riskCategories"`
	CriticalCount   int      `json:"criticalCount"`
	WarningCount    int      `json:"warningCount"`
	InfoCount       int      `json:"infoCount"`
	ImmediateAction bool     `json:"immediateAction"`
	Recommendations []string `json:"recommendations"`
}

type CmdbIdcAssetHardwareMonitorVo struct {
	Asset            CmdbIdcAssetVo                   `json:"asset"`
	MetricsAvailable bool                             `json:"metricsAvailable"`
	Vendor           string                           `json:"vendor"`
	RawVendor        string                           `json:"rawVendor"`
	Source           CmdbIdcAssetHardwareSourceVo     `json:"source"`
	Summary          CmdbIdcAssetHardwareSummaryVo    `json:"summary"`
	Components       CmdbIdcAssetHardwareComponentsVo `json:"components"`
	Trends           CmdbIdcAssetHardwareTrendsVo     `json:"trends"`
	Alerts           []CmdbIdcAssetHardwareAlertVo    `json:"alerts"`
	Health           CmdbIdcAssetHardwareHealthVo     `json:"health"`
	SampledAt        util.HTime                       `json:"sampledAt"`
}

type ExcelCmdbIdcAssetRow struct {
	RoomName             string
	CabinetName          string
	CabinetPosition      int
	CabinetUSize         int
	IP                   string
	Brand                string
	Model                string
	CPU                  string
	Disk                 string
	Memory               string
	Purpose              string
	RemoteManagementCard string
	Remark               string
	PurchaseDate         string
	WarrantyStartDate    string
	WarrantyEndDate      string
	UsageYears           string
	AssetEvaluation      string
}
