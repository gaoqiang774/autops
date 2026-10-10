package model

import "dodevops-api/common/util"

const (
	DatabaseTypeMySQL         = 1
	DatabaseTypePostgreSQL    = 2
	DatabaseTypeOracle        = 3
	DatabaseTypeRedis         = 4
	DatabaseTypeMongoDB       = 5
	DatabaseTypeSQLServer     = 6
	DatabaseTypeElasticsearch = 7
)

const (
	DatabaseStatusRunning = 1
	DatabaseStatusStopped = 2
	DatabaseStatusError   = 3
	DatabaseStatusOffline = 4
)

const (
	DatabaseAuthStatusUnauthenticated = 0
	DatabaseAuthStatusFailed          = 1
	DatabaseAuthStatusSuccess         = 2
)

// CmdbSQL stores database assets in CMDB. Runtime operations such as SQL
// audit, backup and monitoring should reference this asset record.
type CmdbSQL struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	Name           string `gorm:"size:100;not null;index" json:"name"`
	InstanceName   string `gorm:"size:128;index" json:"instanceName"`
	Type           int    `gorm:"type:integer;not null;index" json:"type"`
	TypeName       string `gorm:"-" json:"typeName"`
	Version        string `gorm:"size:64" json:"version"`
	Environment    string `gorm:"size:32;index" json:"environment"`
	Status         int    `gorm:"type:integer;not null;default:1;index" json:"status"`
	AuthStatus     int    `gorm:"type:integer;not null;default:0;index" json:"authStatus"`
	AuthStatusName string `gorm:"-" json:"authStatusName"`

	AccountID    uint   `gorm:"not null;index" json:"accountId"`
	GroupID      uint   `gorm:"not null;index" json:"groupId"`
	HostID       uint   `gorm:"index" json:"hostId"`
	HostIP       string `gorm:"size:64;index" json:"hostIp"`
	Port         int    `gorm:"type:integer" json:"port"`
	Endpoint     string `gorm:"size:255" json:"endpoint"`
	Region       string `gorm:"size:64" json:"region"`
	IdcRoomID    uint   `gorm:"index" json:"idcRoomId"`
	NetworkArea  string `gorm:"size:128" json:"networkArea"`
	PublicAccess bool   `gorm:"default:false" json:"publicAccess"`

	BusinessSystem string `gorm:"size:128;index" json:"businessSystem"`
	Project        string `gorm:"size:128" json:"project"`
	Owner          string `gorm:"size:64;index" json:"owner"`
	DBAOwner       string `gorm:"size:64" json:"dbaOwner"`
	OpsOwner       string `gorm:"size:64" json:"opsOwner"`
	Department     string `gorm:"size:128" json:"department"`

	CPU           string `gorm:"size:64" json:"cpu"`
	Memory        string `gorm:"size:64" json:"memory"`
	StorageSizeGB int    `gorm:"type:integer" json:"storageSizeGb"`
	UsedStorageGB int    `gorm:"type:integer" json:"usedStorageGb"`
	StorageType   string `gorm:"size:64" json:"storageType"`
	DeployMode    string `gorm:"size:64" json:"deployMode"`
	NodeCount     int    `gorm:"type:integer" json:"nodeCount"`

	ClusterName string `gorm:"size:128;index" json:"clusterName"`
	Role        string `gorm:"size:32" json:"role"`
	PrimaryID   uint   `gorm:"index" json:"primaryId"`

	AuthMethod    string `gorm:"size:64" json:"authMethod"`
	SSLEnabled    bool   `gorm:"default:false" json:"sslEnabled"`
	AuditEnabled  bool   `gorm:"default:false" json:"auditEnabled"`
	SecurityLevel string `gorm:"size:32" json:"securityLevel"`
	RiskTags      string `gorm:"size:255" json:"riskTags"`

	BackupPolicy            string     `gorm:"size:255" json:"backupPolicy"`
	LastBackupTime          util.HTime `json:"lastBackupTime"`
	MonitorStatus           int        `gorm:"type:integer;default:0" json:"monitorStatus"`
	AlertStatus             int        `gorm:"type:integer;default:0" json:"alertStatus"`
	SlowLogPath             string     `gorm:"size:512" json:"slowLogPath"`
	PrometheusAccountID     uint       `gorm:"index" json:"prometheusAccountId"`
	PrometheusAccountAlias  string     `gorm:"-" json:"prometheusAccountAlias"`
	PrometheusEndpoint      string     `gorm:"-" json:"prometheusEndpoint"`
	PrometheusLabelSelector string     `gorm:"size:255" json:"prometheusLabelSelector"`
	LastInspectionTime      util.HTime `json:"lastInspectionTime"`
	ExpireTime              util.HTime `json:"expireTime"`

	Tags        string     `gorm:"size:255" json:"tags"`
	Description string     `gorm:"size:500" json:"description"`
	CreatedAt   util.HTime `json:"createdAt"`
	UpdatedAt   util.HTime `json:"updatedAt"`
}

func (CmdbSQL) TableName() string {
	return "cmdb_db_management"
}

type CmdbSQLQuery struct {
	Page           int    `form:"page"`
	PageSize       int    `form:"pageSize"`
	SortBy         string `form:"sortBy"`
	SortOrder      string `form:"sortOrder"`
	Keyword        string `form:"keyword"`
	Type           int    `form:"type"`
	Environment    string `form:"environment"`
	Status         int    `form:"status"`
	GroupID        uint   `form:"groupId"`
	BusinessSystem string `form:"businessSystem"`
	Owner          string `form:"owner"`
	HostIP         string `form:"hostIp"`
}

type CmdbSQLIdDto struct {
	ID uint `json:"id"`
}
