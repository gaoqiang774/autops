package model

import (
	"dodevops-api/common/util"
	"fmt"
	"strconv"
	"strings"
)

const (
	HostTypeLinux   = 1
	HostTypeWindows = 2
)

// 主机基本信息
type CmdbHost struct {
	ID          uint       `gorm:"column:id;comment:'主键';primaryKey;NOT NULL" json:"id"`
	HostName    string     `gorm:"column:host_name;varchar(64);comment:'名称';NOT NULL" json:"hostName"`
	GroupID     uint       `gorm:"column:group_id;comment:'分组ID';NOT NULL" json:"groupId"`
	Group       CmdbGroup  `gorm:"foreignKey:GroupID" json:"group"`
	PrivateIP   string     `gorm:"column:private_ip;varchar(64);comment:'私网IP'" json:"privateIp"`
	PublicIP    string     `gorm:"column:public_ip;varchar(64);comment:'公网IP'" json:"publicIp"`
	SSHIP       string     `gorm:"column:ssh_ip;varchar(64);comment:'SSH连接IP';NOT NULL" json:"sshIp"`
	SSHName     string     `gorm:"column:ssh_name;varchar(64);comment:'SSH用户名'" json:"sshName"`
	SSHKeyID    uint       `gorm:"column:ssh_key_id;comment:'SSH凭据ID'" json:"sshKeyId"`
	SSHPort     int        `gorm:"column:ssh_port;comment:'SSH端口';default:22" json:"sshPort"`
	Remark      string     `gorm:"column:remark;varchar(500);comment:'备注'" json:"remark"`
	Tag         string     `gorm:"column:tag;type:text;comment:'标签(格式:key=value,key=value)'" json:"tag"`
	Vendor      int        `gorm:"column:vendor;varchar(32);comment:'1->自建,2->阿里云,3->腾讯云,4->百度云,5->华为云,6->AWS'" json:"vendor"`
	HostType    int        `gorm:"column:host_type;comment:'主机类型:1->Linux,2->Windows';not null;default:1" json:"hostType"`
	Region      string     `gorm:"column:region;varchar(64);comment:'区域'" json:"region"`
	InstanceID  string     `gorm:"column:instance_id;varchar(128);comment:'实例ID'" json:"instanceId"`
	Name        string     `gorm:"column:name;varchar(64);comment:'ecs主机名称';NOT NULL" json:"name"`
	OS          string     `gorm:"column:os;varchar(128);comment:'操作系统'" json:"os"`
	Status      int        `gorm:"column:status;comment:'状态:1->认证成功,2->未认证,3->认证失败'" json:"status"`
	CPU         string     `gorm:"column:cpu;varchar(32);comment:'CPU信息'" json:"cpu"`
	Memory      string     `gorm:"column:memory;varchar(32);comment:'内存信息'" json:"memory"`
	Disk        string     `gorm:"column:disk;varchar(128);comment:'磁盘信息'" json:"disk"`
	BillingType string     `gorm:"column:billing_type;varchar(32);comment:'计费方式'" json:"billingType"`
	CreateTime  util.HTime `gorm:"column:create_time;comment:'创建时间';NOT NULL" json:"createTime"`
	ExpireTime  util.HTime `gorm:"column:expire_time;comment:'到期时间'" json:"expireTime"`
	UpdateTime  util.HTime `gorm:"column:update_time;comment:'更新时间'" json:"updateTime"`
}

func (CmdbHost) TableName() string {
	return "cmdb_host"
}

// CmdbHostAuth stores additional login credentials bound to a host.
type CmdbHostAuth struct {
	ID         uint       `gorm:"column:id;primaryKey;not null" json:"id"`
	HostID     uint       `gorm:"column:host_id;not null;uniqueIndex:uk_cmdb_host_auth;index" json:"hostId"`
	AuthID     uint       `gorm:"column:auth_id;not null;uniqueIndex:uk_cmdb_host_auth;index" json:"authId"`
	IsDefault  int        `gorm:"column:is_default;not null;default:0" json:"isDefault"`
	CreateTime util.HTime `gorm:"column:create_time;not null" json:"createTime"`
	UpdateTime util.HTime `gorm:"column:update_time;not null" json:"updateTime"`
}

func (CmdbHostAuth) TableName() string {
	return "cmdb_host_auth"
}

// 创建主机DTO - Linux走SSH自动采集，Windows走手工录入
type CreateCmdbHostDto struct {
	SSHKeyIDs []uint `json:"sshKeyIds"`                    // multiple login credential IDs
	AuthIDs   []uint `json:"authIds"`                      // alias for multiple login credential IDs
	HostName  string `validate:"required" json:"hostName"` // 主机名称(唯一标识)
	GroupID   uint   `validate:"required" json:"groupId"`  // 主机分组ID
	HostType  int    `json:"hostType"`                     // 主机类型:1->Linux,2->Windows，默认Linux
	PrivateIP string `json:"privateIp"`                    // Windows手工录入内网IP
	SSHName   string `json:"sshName"`                      // Linux必填，Windows可手工录入
	SSHIP     string `json:"sshIp"`                        // Linux必填，Windows自动等于PrivateIP
	SSHPort   int    `json:"sshPort"`                      // SSH端口(默认22)
	SSHKeyID  uint   `json:"sshKeyId"`                     // Linux必填，Windows可选传入用于账号管理
	OS        string `json:"os"`                           // Windows手工录入操作系统
	CPU       string `json:"cpu"`                          // Windows手工录入CPU
	Memory    string `json:"memory"`                       // Windows手工录入内存
	Disk      string `json:"disk"`                         // Windows手工录入磁盘
	Remark    string `json:"remark"`                       // 备注信息(可选)
	Tag       string `json:"tag"`                          // 标签(格式:key=value,key=value)
}

// 更新主机DTO - Linux维持SSH信息更新，Windows支持手工维护
type UpdateCmdbHostDto struct {
	SSHKeyIDs []uint `json:"sshKeyIds"`                    // multiple login credential IDs
	AuthIDs   []uint `json:"authIds"`                      // alias for multiple login credential IDs
	ID        uint   `json:"id"`                           // 主机ID
	HostName  string `validate:"required" json:"hostName"` // 主机名称(唯一标识)
	GroupID   uint   `validate:"required" json:"groupId"`  // 主机分组ID
	HostType  int    `json:"hostType"`                     // 主机类型:1->Linux,2->Windows，默认Linux
	PrivateIP string `json:"privateIp"`                    // Windows手工录入内网IP
	SSHIP     string `json:"sshIp"`                        // Linux必填，Windows自动等于PrivateIP
	SSHName   string `json:"sshName"`                      // Linux必填，Windows可手工录入
	SSHKeyID  uint   `json:"sshKeyId"`                     // Linux必填，Windows可选传入用于账号管理
	SSHPort   int    `json:"sshPort"`                      // SSH端口(默认22)
	Vendor    int    `json:"vendor"`                       // 厂商类型:1->自建,2->阿里云,3->腾讯云,4->百度云,5->华为云,6->AWS
	OS        string `json:"os"`                           // Windows手工录入操作系统
	CPU       string `json:"cpu"`                          // Windows手工录入CPU
	Memory    string `json:"memory"`                       // Windows手工录入内存
	Disk      string `json:"disk"`                         // Windows手工录入磁盘
	Remark    string `json:"remark"`                       // 备注信息(可选)
	Tag       string `json:"tag"`                          // 标签(格式:key=value,key=value)
}

// 主机ID DTO
type CmdbHostIdDto struct {
	ID uint `json:"id"`
}

// 批量更新主机分组DTO
type BatchUpdateHostGroupDto struct {
	HostIDs []uint `validate:"required" json:"hostIds"` // 主机ID列表
	GroupID uint   `validate:"required" json:"groupId"` // 目标分组ID(必须是二级分组)
}

// 批量删除主机DTO
type BatchDeleteHostDto struct {
	HostIDs []uint `validate:"required" json:"hostIds"` // 主机ID列表
}

// 创建云主机DTO
type CreateCmdbHostCloudDto struct {
	GroupID    uint   `validate:"required" json:"groupId"`   // 分组ID
	Vendor     int    `validate:"required" json:"vendor"`    // 云厂商:2->阿里云,3->腾讯云,4->百度云,5->华为云,6->AWS
	Region     string `validate:"required" json:"region"`    // 区域
	AccessKey  string `validate:"required" json:"accessKey"` // AK
	SecretKey  string `validate:"required" json:"secretKey"` // SK
	InstanceID string `json:"instanceId"`                    // 实例ID(可选)
	SSHPort    int    `json:"sshPort"`                       // SSH端口(默认22,可选)
}

// 批量导入阿里云主机DTO
type BatchImportAliyunHostsDto struct {
	GroupID   uint   `validate:"required" json:"groupId"`    // 分组ID
	Region    string `validate:"required" json:"region"`     // 区域
	AccessKey string `validate:"required" json:"secret_id"`  // AK
	SecretKey string `validate:"required" json:"secret_key"` // SK
}

// 批量导入腾讯云主机DTO
type BatchImportTencentHostsDto struct {
	GroupID   uint   `validate:"required" json:"groupId"`    // 分组ID
	AccessKey string `validate:"required" json:"secret_id"`  // AK
	SecretKey string `validate:"required" json:"secret_key"` // SK
}

// Excel导入主机DTO
type ImportHostsFromExcelDto struct {
	GroupID uint   `validate:"required" json:"groupId"` // 分组ID
	File    string `validate:"required" json:"file"`    // 上传的文件路径
}

// 主机CSV导出查询参数
type CmdbHostExportQuery struct {
	Keyword   string `form:"keyword"`   // 主机名称/IP关键字
	Name      string `form:"name"`      // 兼容旧字段
	IP        string `form:"ip"`        // 兼容旧字段
	GroupID   uint   `form:"groupId"`   // 分组ID
	Status    string `form:"status"`    // 认证状态，支持 1 或 1,2
	SortBy    string `form:"sortBy"`    // 排序字段
	SortOrder string `form:"sortOrder"` // 排序方向
}

func (q CmdbHostExportQuery) ResolvedKeyword() string {
	if keyword := strings.TrimSpace(q.Keyword); keyword != "" {
		return keyword
	}
	if name := strings.TrimSpace(q.Name); name != "" {
		return name
	}
	return strings.TrimSpace(q.IP)
}

func (q CmdbHostExportQuery) ParseStatuses() ([]int, error) {
	raw := strings.TrimSpace(q.Status)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	statuses := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		status, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid status: %s", part)
		}
		if status < 1 || status > 3 {
			return nil, fmt.Errorf("invalid status: %d", status)
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

type CmdbHostFilter struct {
	Keyword        string
	GroupID        uint
	Statuses       []int
	HostTypes      []int
	SortBy         string
	SortOrder      string
	AllowedHostIDs []uint
}

// Excel主机模板行数据
type ExcelHostTemplate struct {
	Name      string // 主机名称
	PrivateIP string // 内网IP
	SSHIP     string // SSH地址
	SSHPort   int    // SSH端口
	SSHName   string // SSH用户
	CPU       string // CPU
	Memory    string // 内存
	Disk      string // 磁盘
	Remark    string // 备注
	Env1      string // 环境字段1
	Env2      string // 环境字段2
}

// 主机VO
type CmdbHostVo struct {
	SSHKeyIDs   []uint                 `json:"sshKeyIds"`
	SSHAuths    []CmdbHostAuthOptionVo `json:"sshAuths"`
	ID          uint                   `json:"id"`
	HostName    string                 `json:"hostName"`
	Name        string                 `json:"name"` // ECS主机名称
	GroupID     uint                   `json:"groupId"`
	GroupName   string                 `json:"groupName"`
	PrivateIP   string                 `json:"privateIp"` // 内网IP
	PublicIP    string                 `json:"publicIp"`  // 外网IP
	SSHIP       string                 `json:"sshIp"`     // SSH连接IP
	SSHName     string                 `json:"sshName"`
	SSHKeyID    uint                   `json:"sshKeyId"`
	SSHKeyName  string                 `json:"sshKeyName"`
	SSHPort     int                    `json:"sshPort"`
	Remark      string                 `json:"remark"` // 备注
	Tag         string                 `json:"tag"`    // 标签
	Vendor      int                    `json:"vendor"`
	HostType    int                    `json:"hostType"`
	Region      string                 `json:"region"`
	InstanceID  string                 `json:"instanceId"`
	OS          string                 `json:"os"` // 操作系统
	Status      int                    `json:"status"`
	CPU         string                 `json:"cpu"`
	Memory      string                 `json:"memory"`
	Disk        string                 `json:"disk"`
	BillingType string                 `json:"billingType"`
	CreateTime  util.HTime             `json:"createTime"`
	ExpireTime  util.HTime             `json:"expireTime"`
	UpdateTime  util.HTime             `json:"updateTime"`
}

type CmdbHostAuthOptionVo struct {
	ID           uint   `json:"id"`
	AuthID       uint   `json:"authId"`
	Name         string `json:"name"`
	Type         int    `json:"type"`
	Username     string `json:"username"`
	Port         int    `json:"port"`
	Remark       string `json:"remark"`
	IsDefault    bool   `json:"isDefault"`
	HasPassword  bool   `json:"hasPassword"`
	HasPublicKey bool   `json:"hasPublicKey"`
}

type CmdbHostAuthSelectionVo struct {
	HostID   uint                   `json:"hostId"`
	Count    int                    `json:"count"`
	Auths    []CmdbHostAuthOptionVo `json:"auths"`
	Required bool                   `json:"required"`
}
