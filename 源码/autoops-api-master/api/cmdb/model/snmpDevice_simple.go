package model

import (
	"strings"

	"dodevops-api/common/util"
)

const snmpMaskedSecretValue = "******"

// CmdbSnmpDevice stores switch and other SNMP-capable device assets.
type CmdbSnmpDevice struct {
	ID uint `gorm:"column:id;comment:'primary key';primaryKey;NOT NULL" json:"id"`

	DeviceName   string `gorm:"column:device_name;type:varchar(100);comment:'device name';NOT NULL" json:"deviceName"`
	DeviceType   string `gorm:"column:device_type;type:varchar(20);comment:'device type';NOT NULL" json:"deviceType"`
	DeviceVendor string `gorm:"column:device_vendor;type:varchar(100);comment:'device vendor'" json:"deviceVendor"`
	DeviceModel  string `gorm:"column:device_model;type:varchar(100);comment:'device model'" json:"deviceModel"`
	IPAddress    string `gorm:"column:ip_address;type:varchar(50);comment:'device ip address';NOT NULL;uniqueIndex" json:"ipAddress"`
	SnmpURL      string `gorm:"column:snmp_url;type:varchar(500);comment:'external snmp monitor api url'" json:"snmpUrl"`

	SnmpVersion   string `gorm:"column:snmp_version;type:varchar(10);comment:'snmp version';default:'v2c'" json:"snmpVersion"`
	SnmpCommunity string `gorm:"column:snmp_community;type:varchar(100);comment:'snmp community';default:'public'" json:"snmpCommunity"`
	SnmpPort      int    `gorm:"column:snmp_port;comment:'snmp port';default:161" json:"snmpPort"`

	RemotePort     int    `gorm:"column:remote_port;comment:'remote login port'" json:"remotePort"`
	RemoteUsername string `gorm:"column:remote_username;type:varchar(100);comment:'remote login username'" json:"remoteUsername"`
	RemotePassword string `gorm:"column:remote_password;type:text;comment:'remote login password'" json:"remotePassword"`
	RemoteDomain   string `gorm:"column:remote_domain;type:varchar(100);comment:'remote login domain'" json:"remoteDomain"`

	ExporterInstalled int    `gorm:"column:exporter_installed;comment:'windows exporter install status';default:0" json:"exporterInstalled"`
	ExporterPort      int    `gorm:"column:exporter_port;comment:'windows exporter port';default:9182" json:"exporterPort"`
	ExporterVersion   string `gorm:"column:exporter_version;type:varchar(50);comment:'windows exporter version'" json:"exporterVersion"`

	MonitorEnabled int        `gorm:"column:monitor_enabled;comment:'monitor enabled';default:1" json:"monitorEnabled"`
	Location       string     `gorm:"column:location;type:varchar(200);comment:'device location'" json:"location"`
	Owner          string     `gorm:"column:owner;type:varchar(100);comment:'device owner'" json:"owner"`
	Remark         string     `gorm:"column:remark;type:text;comment:'remark'" json:"remark"`
	Status         string     `gorm:"column:status;type:varchar(20);comment:'device status';default:'offline'" json:"status"`
	LastSeen       util.HTime `gorm:"column:last_seen;comment:'last seen time'" json:"lastSeen"`

	CreateTime util.HTime `gorm:"column:create_time;comment:'create time';NOT NULL" json:"createTime"`
	UpdateTime util.HTime `gorm:"column:update_time;comment:'update time';NOT NULL" json:"updateTime"`
}

func (CmdbSnmpDevice) TableName() string {
	return "cmdb_snmp_devices"
}

func (d *CmdbSnmpDevice) EncryptRemotePassword() error {
	encrypted, err := EncryptSnmpSecretIfNeeded(d.RemotePassword)
	if err != nil {
		return err
	}
	d.RemotePassword = encrypted
	return nil
}

func (d *CmdbSnmpDevice) DecryptRemotePassword() (string, error) {
	return DecryptSnmpSecretIfPossible(d.RemotePassword)
}

func (d CmdbSnmpDevice) HasRemotePassword() bool {
	return d.RemotePassword != "" && !IsSnmpMaskedSecretValue(d.RemotePassword)
}

func EncryptSnmpSecretIfNeeded(value string) (string, error) {
	if value == "" || IsSnmpMaskedSecretValue(value) {
		return value, nil
	}

	if _, err := util.AESDecrypt(value); err == nil {
		return value, nil
	}

	return util.AESEncrypt(value)
}

func DecryptSnmpSecretIfPossible(value string) (string, error) {
	if value == "" || IsSnmpMaskedSecretValue(value) {
		return value, nil
	}

	decrypted, err := util.AESDecrypt(value)
	if err != nil {
		return value, nil
	}

	return decrypted, nil
}

func IsSnmpMaskedSecretValue(value string) bool {
	return value == snmpMaskedSecretValue
}

func maskSnmpSecret(value string) string {
	if value == "" {
		return ""
	}
	return snmpMaskedSecretValue
}

type CreateSnmpDeviceDto struct {
	DeviceName      string `json:"deviceName"`
	DeviceNameAlias string `json:"device_name"`
	DeviceType      string `json:"deviceType"`
	DeviceTypeAlias string `json:"device_type"`
	IPAddress       string `json:"ipAddress"`
	IPAddressAlias  string `json:"ip_address"`
	SnmpURL         string `json:"snmpUrl"`
	SnmpURLAlias    string `json:"snmp_url"`
	DeviceVendor    string `json:"deviceVendor"`
	DeviceVendorAlt string `json:"vendor"`
	DeviceVendorAli string `json:"device_vendor"`
	DeviceModel     string `json:"deviceModel"`
	DeviceModelAlt  string `json:"model"`
	DeviceModelAli  string `json:"device_model"`

	SnmpVersion   string `json:"snmpVersion"`
	SnmpCommunity string `json:"snmpCommunity"`
	SnmpPort      int    `json:"snmpPort"`

	RemotePort     int    `json:"remotePort"`
	RemoteUsername string `json:"remoteUsername"`
	RemotePassword string `json:"remotePassword"`
	RemoteDomain   string `json:"remoteDomain"`

	SSHName          string `json:"sshName"`
	SSHNameAlias     string `json:"ssh_name"`
	SSHPasswd        string `json:"sshPasswd"`
	SSHPasswdAlias   string `json:"ssh_passwd"`
	SSHPasswordAlias string `json:"sshPassword"`
	SSHPort          int    `json:"sshPort"`
	SSHPortAlias     int    `json:"ssh_port"`
	SSHProtAlias     int    `json:"ssh_prot"`

	Location string `json:"location"`
	Owner    string `json:"owner"`
	Remark   string `json:"remark"`
}

func (dto *CreateSnmpDeviceDto) Normalize() {
	dto.DeviceName = firstNonEmpty(dto.DeviceName, dto.DeviceNameAlias)
	dto.DeviceType = strings.ToLower(firstNonEmpty(dto.DeviceType, dto.DeviceTypeAlias))
	if dto.DeviceType == "" {
		dto.DeviceType = "switch"
	}

	dto.IPAddress = firstNonEmpty(dto.IPAddress, dto.IPAddressAlias)
	dto.SnmpURL = firstNonEmpty(dto.SnmpURL, dto.SnmpURLAlias)
	dto.DeviceVendor = firstNonEmpty(dto.DeviceVendor, dto.DeviceVendorAlt, dto.DeviceVendorAli)
	dto.DeviceModel = firstNonEmpty(dto.DeviceModel, dto.DeviceModelAlt, dto.DeviceModelAli)

	if dto.SnmpVersion == "" {
		dto.SnmpVersion = "v2c"
	}
	if dto.SnmpCommunity == "" {
		dto.SnmpCommunity = "public"
	}
	if dto.SnmpPort == 0 {
		dto.SnmpPort = 161
	}

	dto.RemoteUsername = firstNonEmpty(dto.RemoteUsername, dto.SSHName, dto.SSHNameAlias)
	dto.RemotePassword = firstNonEmpty(dto.RemotePassword, dto.SSHPasswd, dto.SSHPasswdAlias, dto.SSHPasswordAlias)
	if dto.RemotePort == 0 {
		dto.RemotePort = firstNonZero(dto.SSHPort, dto.SSHPortAlias, dto.SSHProtAlias)
	}
	if dto.RemotePort == 0 {
		if dto.DeviceType == "windows" {
			dto.RemotePort = 3389
		} else {
			dto.RemotePort = 22
		}
	}

	dto.Location = strings.TrimSpace(dto.Location)
	dto.Owner = strings.TrimSpace(dto.Owner)
	dto.Remark = strings.TrimSpace(dto.Remark)
}

type UpdateSnmpDeviceDto struct {
	ID uint `json:"id"`

	DeviceName      *string `json:"deviceName"`
	DeviceNameAlias *string `json:"device_name"`
	DeviceType      *string `json:"deviceType"`
	DeviceTypeAlias *string `json:"device_type"`
	IPAddress       *string `json:"ipAddress"`
	IPAddressAlias  *string `json:"ip_address"`
	SnmpURL         *string `json:"snmpUrl"`
	SnmpURLAlias    *string `json:"snmp_url"`
	DeviceVendor    *string `json:"deviceVendor"`
	DeviceVendorAlt *string `json:"vendor"`
	DeviceVendorAli *string `json:"device_vendor"`
	DeviceModel     *string `json:"deviceModel"`
	DeviceModelAlt  *string `json:"model"`
	DeviceModelAli  *string `json:"device_model"`

	SnmpVersion   *string `json:"snmpVersion"`
	SnmpCommunity *string `json:"snmpCommunity"`
	SnmpPort      *int    `json:"snmpPort"`

	RemotePort     *int    `json:"remotePort"`
	RemoteUsername *string `json:"remoteUsername"`
	RemotePassword *string `json:"remotePassword"`
	RemoteDomain   *string `json:"remoteDomain"`

	SSHName          *string `json:"sshName"`
	SSHNameAlias     *string `json:"ssh_name"`
	SSHPasswd        *string `json:"sshPasswd"`
	SSHPasswdAlias   *string `json:"ssh_passwd"`
	SSHPasswordAlias *string `json:"sshPassword"`
	SSHPort          *int    `json:"sshPort"`
	SSHPortAlias     *int    `json:"ssh_port"`
	SSHProtAlias     *int    `json:"ssh_prot"`

	MonitorEnabled *int    `json:"monitorEnabled"`
	Location       *string `json:"location"`
	Owner          *string `json:"owner"`
	Remark         *string `json:"remark"`
}

func (dto *UpdateSnmpDeviceDto) Normalize() {
	dto.DeviceName = pickStringPtr(dto.DeviceName, dto.DeviceNameAlias)
	dto.DeviceType = pickNormalizedLowerPtr(dto.DeviceType, dto.DeviceTypeAlias)
	dto.IPAddress = pickStringPtr(dto.IPAddress, dto.IPAddressAlias)
	dto.SnmpURL = pickStringPtr(dto.SnmpURL, dto.SnmpURLAlias)
	dto.DeviceVendor = pickStringPtr(dto.DeviceVendor, dto.DeviceVendorAlt, dto.DeviceVendorAli)
	dto.DeviceModel = pickStringPtr(dto.DeviceModel, dto.DeviceModelAlt, dto.DeviceModelAli)

	dto.SnmpVersion = trimStringPtr(dto.SnmpVersion)
	dto.SnmpCommunity = trimStringPtr(dto.SnmpCommunity)

	dto.RemoteUsername = pickStringPtr(dto.RemoteUsername, dto.SSHName, dto.SSHNameAlias)
	dto.RemotePassword = pickStringPtr(dto.RemotePassword, dto.SSHPasswd, dto.SSHPasswdAlias, dto.SSHPasswordAlias)
	dto.RemoteDomain = trimStringPtr(dto.RemoteDomain)
	dto.RemotePort = pickIntPtr(dto.RemotePort, dto.SSHPort, dto.SSHPortAlias, dto.SSHProtAlias)

	dto.Location = trimStringPtr(dto.Location)
	dto.Owner = trimStringPtr(dto.Owner)
	dto.Remark = trimStringPtr(dto.Remark)
}

type SnmpDeviceQueryDto struct {
	DeviceType     string `json:"deviceType" form:"deviceType"`
	Status         string `json:"status" form:"status"`
	MonitorEnabled int    `json:"monitorEnabled" form:"monitorEnabled"`
	Keyword        string `json:"keyword" form:"keyword"`
	Page           int    `json:"page" form:"page" validate:"min=1"`
	PageSize       int    `json:"pageSize" form:"pageSize" validate:"min=1,max=100"`
}

type SnmpDeviceListResult struct {
	Items    []CmdbSnmpDevice `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
}

type SnmpDeviceStatsDto struct {
	TotalCount       int `json:"totalCount"`
	OnlineCount      int `json:"onlineCount"`
	OfflineCount     int `json:"offlineCount"`
	MaintenanceCount int `json:"maintenanceCount"`
	SwitchCount      int `json:"switchCount"`
	WindowsCount     int `json:"windowsCount"`
	RouterCount      int `json:"routerCount"`
	OtherCount       int `json:"otherCount"`
}

type SnmpMonitorTemplateSummary struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type SnmpMonitorTemplateField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

type SnmpMonitorTemplatePanel struct {
	Key    string                     `json:"key"`
	Title  string                     `json:"title"`
	Fields []SnmpMonitorTemplateField `json:"fields"`
}

type SnmpMonitorTemplate struct {
	Code   string                     `json:"code"`
	Name   string                     `json:"name"`
	Panels []SnmpMonitorTemplatePanel `json:"panels"`
}

func (t SnmpMonitorTemplate) Summary() SnmpMonitorTemplateSummary {
	return SnmpMonitorTemplateSummary{
		Code: t.Code,
		Name: t.Name,
	}
}

func ResolveSnmpMonitorTemplate(vendor, model string) SnmpMonitorTemplate {
	vendor = strings.ToLower(strings.TrimSpace(vendor))
	model = strings.ToLower(strings.TrimSpace(model))

	switch {
	case strings.Contains(vendor, "huawei") && strings.Contains(model, "s6730-h24x6c"):
		return SnmpMonitorTemplate{
			Code: "huawei-s6730-h24x6c",
			Name: "Huawei S6730-H24X6C",
			Panels: []SnmpMonitorTemplatePanel{
				{
					Key:   "overview",
					Title: "Overview",
					Fields: []SnmpMonitorTemplateField{
						{Key: "cpuUsage", Label: "CPU", Type: "number"},
						{Key: "memoryUsage", Label: "Memory", Type: "number"},
						{Key: "temperature", Label: "Temperature", Type: "number"},
						{Key: "uptimeDays", Label: "Uptime", Type: "number"},
					},
				},
				{
					Key:   "interfaces",
					Title: "Interfaces",
					Fields: []SnmpMonitorTemplateField{
						{Key: "interfaceTrafficTop", Label: "Traffic Top", Type: "list"},
						{Key: "interfaceErrorsTop", Label: "Errors Top", Type: "list"},
						{Key: "opticalStatus", Label: "Optics", Type: "list"},
					},
				},
				{
					Key:   "environment",
					Title: "Environment",
					Fields: []SnmpMonitorTemplateField{
						{Key: "fanStatus", Label: "Fans", Type: "list"},
						{Key: "powerStatus", Label: "Power", Type: "list"},
						{Key: "temperatureTrend", Label: "Temperature Trend", Type: "trend"},
					},
				},
			},
		}
	case strings.Contains(vendor, "cisco") || strings.Contains(model, "cisco") || strings.Contains(model, "catalyst"):
		return SnmpMonitorTemplate{
			Code: "cisco-switch",
			Name: "Cisco Switch",
			Panels: []SnmpMonitorTemplatePanel{
				{
					Key:   "overview",
					Title: "Overview",
					Fields: []SnmpMonitorTemplateField{
						{Key: "cpuUsage", Label: "CPU", Type: "number"},
						{Key: "memoryUsage", Label: "Memory", Type: "number"},
						{Key: "temperature", Label: "Temperature", Type: "number"},
						{Key: "uptimeDays", Label: "Uptime", Type: "number"},
					},
				},
				{
					Key:   "interfaces",
					Title: "Interfaces",
					Fields: []SnmpMonitorTemplateField{
						{Key: "interfaceTrafficTop", Label: "Traffic Top", Type: "list"},
						{Key: "interfaceErrorsTop", Label: "Errors Top", Type: "list"},
						{Key: "stackStatus", Label: "Stack", Type: "list"},
					},
				},
				{
					Key:   "environment",
					Title: "Environment",
					Fields: []SnmpMonitorTemplateField{
						{Key: "environmentStatus", Label: "Environment", Type: "list"},
						{Key: "cpuTrend", Label: "CPU Trend", Type: "trend"},
						{Key: "memoryTrend", Label: "Memory Trend", Type: "trend"},
					},
				},
			},
		}
	case strings.Contains(vendor, "h3c") || strings.Contains(model, "h3c"):
		return SnmpMonitorTemplate{
			Code: "h3c-switch",
			Name: "H3C Switch",
			Panels: []SnmpMonitorTemplatePanel{
				{
					Key:   "overview",
					Title: "Overview",
					Fields: []SnmpMonitorTemplateField{
						{Key: "cpuUsage", Label: "CPU", Type: "number"},
						{Key: "memoryUsage", Label: "Memory", Type: "number"},
						{Key: "temperature", Label: "Temperature", Type: "number"},
						{Key: "uptimeDays", Label: "Uptime", Type: "number"},
					},
				},
				{
					Key:   "ports",
					Title: "Ports",
					Fields: []SnmpMonitorTemplateField{
						{Key: "portInTop5", Label: "Inbound Top5", Type: "list"},
						{Key: "portOutTop5", Label: "Outbound Top5", Type: "list"},
						{Key: "idlePortCount", Label: "Idle Ports", Type: "number"},
						{Key: "totalPortCount", Label: "Total Ports", Type: "number"},
					},
				},
				{
					Key:   "trends",
					Title: "Trends",
					Fields: []SnmpMonitorTemplateField{
						{Key: "cpuTrend", Label: "CPU Trend", Type: "trend"},
						{Key: "memoryTrend", Label: "Memory Trend", Type: "trend"},
						{Key: "temperatureTrend", Label: "Temperature Trend", Type: "trend"},
					},
				},
			},
		}
	default:
		return SnmpMonitorTemplate{
			Code: "generic-switch",
			Name: "Generic Switch",
			Panels: []SnmpMonitorTemplatePanel{
				{
					Key:   "overview",
					Title: "Overview",
					Fields: []SnmpMonitorTemplateField{
						{Key: "cpuUsage", Label: "CPU", Type: "number"},
						{Key: "memoryUsage", Label: "Memory", Type: "number"},
						{Key: "temperature", Label: "Temperature", Type: "number"},
						{Key: "uptimeDays", Label: "Uptime", Type: "number"},
					},
				},
				{
					Key:   "interfaces",
					Title: "Interfaces",
					Fields: []SnmpMonitorTemplateField{
						{Key: "interfaceTrafficTop", Label: "Traffic Top", Type: "list"},
						{Key: "interfaceErrorsTop", Label: "Errors Top", Type: "list"},
						{Key: "monitorPayload", Label: "Payload", Type: "raw"},
					},
				},
			},
		}
	}
}

type SnmpDeviceVO struct {
	ID                uint                       `json:"id"`
	DeviceName        string                     `json:"deviceName"`
	DeviceType        string                     `json:"deviceType"`
	DeviceVendor      string                     `json:"deviceVendor"`
	DeviceModel       string                     `json:"deviceModel"`
	IPAddress         string                     `json:"ipAddress"`
	SnmpURL           string                     `json:"snmpUrl"`
	SnmpVersion       string                     `json:"snmpVersion"`
	SnmpCommunity     string                     `json:"snmpCommunity"`
	SnmpPort          int                        `json:"snmpPort"`
	SshPort           int                        `json:"sshPort"`
	SshName           string                     `json:"sshName"`
	SshUsername       string                     `json:"sshUsername"`
	SshPasswd         string                     `json:"sshPasswd"`
	SshPassword       string                     `json:"sshPassword"`
	HasRemotePassword bool                       `json:"hasRemotePassword"`
	RdpPort           int                        `json:"rdpPort"`
	RdpUsername       string                     `json:"rdpUsername"`
	RdpPassword       string                     `json:"rdpPassword"`
	RdpDomain         string                     `json:"rdpDomain"`
	ExporterInstalled int                        `json:"exporterInstalled"`
	ExporterPort      int                        `json:"exporterPort"`
	ExporterVersion   string                     `json:"exporterVersion"`
	MonitorEnabled    int                        `json:"monitorEnabled"`
	Location          string                     `json:"location"`
	Owner             string                     `json:"owner"`
	Remark            string                     `json:"remark"`
	Status            string                     `json:"status"`
	LastSeen          string                     `json:"lastSeen"`
	CreateTime        string                     `json:"createTime"`
	UpdateTime        string                     `json:"updateTime"`
	MonitorTemplate   SnmpMonitorTemplateSummary `json:"monitorTemplate"`
}

func (d *CmdbSnmpDevice) ToVO() *SnmpDeviceVO {
	template := ResolveSnmpMonitorTemplate(d.DeviceVendor, d.DeviceModel)
	remotePassword := maskSnmpSecret(d.RemotePassword)

	return &SnmpDeviceVO{
		ID:                d.ID,
		DeviceName:        d.DeviceName,
		DeviceType:        d.DeviceType,
		DeviceVendor:      d.DeviceVendor,
		DeviceModel:       d.DeviceModel,
		IPAddress:         d.IPAddress,
		SnmpURL:           d.SnmpURL,
		SnmpVersion:       d.SnmpVersion,
		SnmpCommunity:     d.SnmpCommunity,
		SnmpPort:          d.SnmpPort,
		SshPort:           d.RemotePort,
		SshName:           d.RemoteUsername,
		SshUsername:       d.RemoteUsername,
		SshPasswd:         remotePassword,
		SshPassword:       remotePassword,
		HasRemotePassword: d.HasRemotePassword(),
		RdpPort:           d.RemotePort,
		RdpUsername:       d.RemoteUsername,
		RdpPassword:       remotePassword,
		RdpDomain:         d.RemoteDomain,
		ExporterInstalled: d.ExporterInstalled,
		ExporterPort:      d.ExporterPort,
		ExporterVersion:   d.ExporterVersion,
		MonitorEnabled:    d.MonitorEnabled,
		Location:          d.Location,
		Owner:             d.Owner,
		Remark:            d.Remark,
		Status:            d.Status,
		LastSeen:          d.LastSeen.String(),
		CreateTime:        d.CreateTime.String(),
		UpdateTime:        d.UpdateTime.String(),
		MonitorTemplate:   template.Summary(),
	}
}

type DeviceMonitorMetrics struct {
	DeviceID         uint                `json:"deviceId"`
	DeviceName       string              `json:"deviceName"`
	DeviceType       string              `json:"deviceType"`
	DeviceVendor     string              `json:"deviceVendor"`
	DeviceModel      string              `json:"deviceModel"`
	IPAddress        string              `json:"ipAddress"`
	Status           string              `json:"status"`
	MetricsAvailable bool                `json:"metricsAvailable"`
	MetricsSource    string              `json:"metricsSource"`
	MonitorTemplate  SnmpMonitorTemplate `json:"monitorTemplate"`

	CpuUsage    float64 `json:"cpuUsage"`
	MemoryUsage float64 `json:"memoryUsage"`
	Temperature float64 `json:"temperature"`
	UptimeDays  float64 `json:"uptimeDays"`

	PortInTop5     []PortTrafficStat `json:"portInTop5"`
	PortOutTop5    []PortTrafficStat `json:"portOutTop5"`
	IdlePortCount  int               `json:"idlePortCount"`
	TotalPortCount int               `json:"totalPortCount"`

	CpuTrend         []TrendPoint `json:"cpuTrend"`
	MemoryTrend      []TrendPoint `json:"memoryTrend"`
	TemperatureTrend []TrendPoint `json:"temperatureTrend"`

	LastUpdateTime string                 `json:"lastUpdateTime"`
	MonitorPayload map[string]interface{} `json:"monitorPayload,omitempty"`
}

type PortTrafficStat struct {
	PortIndex       int     `json:"portIndex"`
	PortName        string  `json:"portName"`
	TrafficRateMbps float64 `json:"trafficRateMbps"`
	TrafficBytes    float64 `json:"trafficBytes,omitempty"`
	Utilization     float64 `json:"utilization"`
	PortSpeed       float64 `json:"portSpeed"`
}

type TrendPoint struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

type DeviceResourceMetrics struct {
	DeviceID         uint                       `json:"deviceId"`
	DeviceName       string                     `json:"deviceName"`
	DeviceType       string                     `json:"deviceType"`
	DeviceVendor     string                     `json:"deviceVendor"`
	DeviceModel      string                     `json:"deviceModel"`
	IPAddress        string                     `json:"ipAddress"`
	Status           string                     `json:"status"`
	CpuUsage         float64                    `json:"cpuUsage"`
	MemoryUsage      float64                    `json:"memoryUsage"`
	UpdateTime       string                     `json:"updateTime"`
	MetricsAvailable bool                       `json:"metricsAvailable"`
	MetricsSource    string                     `json:"metricsSource"`
	MonitorTemplate  SnmpMonitorTemplateSummary `json:"monitorTemplate"`
}

type BatchResourceQueryDto struct {
	DeviceIDs []uint `json:"deviceIds" validate:"required,min=1,max=100"`
}

type BatchResourceMetricsResult struct {
	Items []DeviceResourceMetrics `json:"items"`
	Total int                     `json:"total"`
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func trimStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func pickStringPtr(values ...*string) *string {
	for _, value := range values {
		if value == nil {
			continue
		}
		trimmed := strings.TrimSpace(*value)
		return &trimmed
	}
	return nil
}

func pickNormalizedLowerPtr(values ...*string) *string {
	for _, value := range values {
		if value == nil {
			continue
		}
		normalized := strings.ToLower(strings.TrimSpace(*value))
		return &normalized
	}
	return nil
}

func pickIntPtr(values ...*int) *int {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
