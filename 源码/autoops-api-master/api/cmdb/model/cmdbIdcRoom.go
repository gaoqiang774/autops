package model

import "dodevops-api/common/util"

// CmdbIdcRoom 机房
type CmdbIdcRoom struct {
	ID           uint             `gorm:"column:id;primaryKey;not null" json:"id"`
	Name         string           `gorm:"column:name;size:100;not null;uniqueIndex:uk_cmdb_idc_room_name" json:"name"`
	Remark       string           `gorm:"column:remark;size:500" json:"remark"`
	CreateTime   util.HTime       `gorm:"column:create_time;not null" json:"createTime"`
	UpdateTime   util.HTime       `gorm:"column:update_time;not null" json:"updateTime"`
	Cabinets     []CmdbIdcCabinet `gorm:"foreignKey:RoomID" json:"cabinets"`
	AssetCount   int64            `gorm:"-" json:"assetCount"`
	CabinetCount int              `gorm:"-" json:"cabinetCount"`
}

func (CmdbIdcRoom) TableName() string {
	return "cmdb_idc_room"
}

// CmdbIdcCabinet 机柜
type CmdbIdcCabinet struct {
	ID         uint        `gorm:"column:id;primaryKey;not null" json:"id"`
	RoomID     uint        `gorm:"column:room_id;not null;uniqueIndex:uk_cmdb_idc_cabinet_room_name" json:"roomId"`
	Room       CmdbIdcRoom `gorm:"foreignKey:RoomID" json:"-"`
	Name       string      `gorm:"column:name;size:100;not null;uniqueIndex:uk_cmdb_idc_cabinet_room_name" json:"name"`
	TotalU     int         `gorm:"column:total_u;not null;default:42" json:"totalU"`
	Remark     string      `gorm:"column:remark;size:500" json:"remark"`
	CreateTime util.HTime  `gorm:"column:create_time;not null" json:"createTime"`
	UpdateTime util.HTime  `gorm:"column:update_time;not null" json:"updateTime"`
	AssetCount int64       `gorm:"-" json:"assetCount"`
}

func (CmdbIdcCabinet) TableName() string {
	return "cmdb_idc_cabinet"
}

type CreateCmdbIdcRoomDto struct {
	Name         string   `json:"name" binding:"required"`
	Remark       string   `json:"remark"`
	CabinetNames []string `json:"cabinetNames"`
}

type UpdateCmdbIdcRoomDto struct {
	ID     uint   `json:"id" binding:"required"`
	Name   string `json:"name" binding:"required"`
	Remark string `json:"remark"`
}

type CmdbIdcRoomIdDto struct {
	ID uint `json:"id" binding:"required"`
}

type CreateCmdbIdcCabinetDto struct {
	RoomID uint   `json:"roomId" binding:"required"`
	Name   string `json:"name" binding:"required"`
	TotalU int    `json:"totalU"`
	Remark string `json:"remark"`
}

type UpdateCmdbIdcCabinetDto struct {
	ID     uint   `json:"id" binding:"required"`
	RoomID uint   `json:"roomId" binding:"required"`
	Name   string `json:"name" binding:"required"`
	TotalU int    `json:"totalU"`
	Remark string `json:"remark"`
}

type CmdbIdcCabinetIdDto struct {
	ID uint `json:"id" binding:"required"`
}

type CmdbIdcCabinetLayoutQuery struct {
	CabinetID   uint   `form:"cabinetId"`
	RoomID      uint   `form:"roomId"`
	CabinetName string `form:"cabinetName"`
}

type CmdbIdcCabinetLayout struct {
	CabinetID          uint                       `json:"cabinetId"`
	CabinetName        string                     `json:"cabinetName"`
	RoomID             uint                       `json:"roomId"`
	RoomName           string                     `json:"roomName"`
	TotalU             int                        `json:"totalU"`
	UsedU              int                        `json:"usedU"`
	FreeU              int                        `json:"freeU"`
	UsageRate          float64                    `json:"usageRate"`
	IdleRate           float64                    `json:"idleRate"`
	IsFull             bool                       `json:"isFull"`
	AssetCount         int                        `json:"assetCount"`
	PlacedAssetCount   int                        `json:"placedAssetCount"`
	UnplacedAssetCount int                        `json:"unplacedAssetCount"`
	HardwareStatus     string                     `json:"hardwareStatus"`
	HardwareSummary    CmdbIdcHardwareSummary     `json:"hardwareSummary"`
	Slots              []CmdbIdcCabinetSlot       `json:"slots"`
	Assets             []CmdbIdcCabinetAssetBlock `json:"assets"`
}

type CmdbIdcHardwareSummary struct {
	NormalAssetCount   int                         `json:"normalAssetCount"`
	WarningAssetCount  int                         `json:"warningAssetCount"`
	CriticalAssetCount int                         `json:"criticalAssetCount"`
	UnknownAssetCount  int                         `json:"unknownAssetCount"`
	AlertAssetCount    int                         `json:"alertAssetCount"`
	Issues             []CmdbIdcHardwareIssueBrief `json:"issues"`
}

type CmdbIdcHardwareIssueBrief struct {
	AssetID              uint     `json:"assetId"`
	IP                   string   `json:"ip"`
	RemoteManagementCard string   `json:"remoteManagementCard"`
	Level                string   `json:"level"`
	Metrics              []string `json:"metrics"`
}

type CmdbIdcRoomLayout struct {
	RoomID            uint                       `json:"roomId"`
	RoomName          string                     `json:"roomName"`
	CabinetCount      int                        `json:"cabinetCount"`
	AssetCount        int64                      `json:"assetCount"`
	LayoutMode        string                     `json:"layoutMode"`
	Rows              []CmdbIdcRoomLayoutRow     `json:"rows"`
	UnmatchedCabinets []CmdbIdcRoomLayoutCabinet `json:"unmatchedCabinets"`
}

type CmdbIdcRoomLayoutRow struct {
	RowNo    int                        `json:"rowNo"`
	RowLabel string                     `json:"rowLabel"`
	Y        int                        `json:"y"`
	Cabinets []CmdbIdcRoomLayoutCabinet `json:"cabinets"`
}

type CmdbIdcRoomLayoutCabinet struct {
	CabinetID       uint                   `json:"cabinetId"`
	Name            string                 `json:"name"`
	RowNo           int                    `json:"rowNo"`
	CabinetNo       int                    `json:"cabinetNo"`
	X               int                    `json:"x"`
	Y               int                    `json:"y"`
	Direction       string                 `json:"direction"`
	TotalU          int                    `json:"totalU"`
	UsedU           int                    `json:"usedU"`
	FreeU           int                    `json:"freeU"`
	UsageRate       float64                `json:"usageRate"`
	IdleRate        float64                `json:"idleRate"`
	AssetCount      int64                  `json:"assetCount"`
	Status          string                 `json:"status"`
	HardwareStatus  string                 `json:"hardwareStatus"`
	HardwareSummary CmdbIdcHardwareSummary `json:"hardwareSummary"`
	ParseStatus     string                 `json:"parseStatus"`
}

type CmdbIdcCabinetSlot struct {
	U        int    `json:"u"`
	Occupied bool   `json:"occupied"`
	AssetIDs []uint `json:"assetIds"`
}

type CmdbIdcCabinetAssetBlock struct {
	AssetID              uint     `json:"assetId"`
	IP                   string   `json:"ip"`
	Brand                string   `json:"brand"`
	Model                string   `json:"model"`
	Purpose              string   `json:"purpose"`
	RemoteManagementCard string   `json:"remoteManagementCard"`
	HardwareStatus       string   `json:"hardwareStatus"`
	HardwareMetrics      []string `json:"hardwareMetrics"`
	CabinetPosition      int      `json:"cabinetPosition"`
	CabinetUSize         int      `json:"cabinetUSize"`
	StartU               int      `json:"startU"`
	EndU                 int      `json:"endU"`
	PlacementStatus      string   `json:"placementStatus"` // placed/unplaced/out_of_range/overlap
}
