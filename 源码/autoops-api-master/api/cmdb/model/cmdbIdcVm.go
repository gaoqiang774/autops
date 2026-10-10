package model

import "dodevops-api/common/util"

// CmdbIdcVm 物理机下的虚拟机关联
type CmdbIdcVm struct {
	ID              uint         `gorm:"column:id;primaryKey;not null" json:"id"`
	PhysicalAssetID uint         `gorm:"column:physical_asset_id;not null;index" json:"physicalAssetId"`
	PhysicalAsset   CmdbIdcAsset `gorm:"foreignKey:PhysicalAssetID" json:"-"`
	VmIP            string       `gorm:"column:vm_ip;size:255;not null;index" json:"vmIp"`
	Username        string       `gorm:"column:username;size:100" json:"username"`
	VmName          string       `gorm:"column:vm_name;size:150" json:"vmName"`
	Remark          string       `gorm:"column:remark;size:1000" json:"remark"`
	CreateTime      util.HTime   `gorm:"column:create_time;not null" json:"createTime"`
	UpdateTime      util.HTime   `gorm:"column:update_time;not null" json:"updateTime"`
}

func (CmdbIdcVm) TableName() string {
	return "cmdb_idc_vm"
}

type CreateCmdbIdcVmDto struct {
	PhysicalAssetID uint   `json:"physicalAssetId"`
	PhysicalIP      string `json:"physicalIp"`
	VmIP            string `json:"vmIp" binding:"required"`
	Username        string `json:"username"`
	VmName          string `json:"vmName"`
	Remark          string `json:"remark"`
}

type UpdateCmdbIdcVmDto struct {
	ID              uint   `json:"id" binding:"required"`
	PhysicalAssetID uint   `json:"physicalAssetId"`
	PhysicalIP      string `json:"physicalIp"`
	VmIP            string `json:"vmIp" binding:"required"`
	Username        string `json:"username"`
	VmName          string `json:"vmName"`
	Remark          string `json:"remark"`
}

type CmdbIdcVmIdDto struct {
	ID uint `json:"id" binding:"required"`
}

type CmdbIdcVmQuery struct {
	Page            int    `form:"page" binding:"required,min=1"`
	PageSize        int    `form:"pageSize" binding:"required,min=1,max=200"`
	PhysicalAssetID uint   `form:"physicalAssetId"`
	PhysicalIP      string `form:"physicalIp"`
	Keyword         string `form:"keyword"`
	SortBy          string `form:"sortBy"`
	SortOrder       string `form:"sortOrder"`
}

type CmdbIdcVmVo struct {
	ID              uint       `json:"id"`
	PhysicalAssetID uint       `json:"physicalAssetId"`
	PhysicalIP      string     `json:"physicalIp"`
	VmIP            string     `json:"vmIp"`
	Username        string     `json:"username"`
	VmName          string     `json:"vmName"`
	Remark          string     `json:"remark"`
	CreateTime      util.HTime `json:"createTime"`
	UpdateTime      util.HTime `json:"updateTime"`
}

type ExcelCmdbIdcVmRow struct {
	PhysicalIP string
	VmIP       string
	Username   string
	VmName     string
	Remark     string
}
