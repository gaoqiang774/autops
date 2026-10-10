package controller

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/api/cmdb/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/common/util"

	"github.com/gin-gonic/gin"
)

// 创建机房
// @Summary 创建机房
// @Description 创建机房，可同时关联创建多个机柜
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.CreateCmdbIdcRoomDto true "机房信息"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/idc-room/create [post]
// @Security ApiKeyAuth
func CreateCmdbIdcRoom(c *gin.Context) {
	var dto model.CreateCmdbIdcRoomDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	service.GetCmdbIdcRoomService().CreateRoom(c, &dto)
}

// 更新机房
// @Summary 更新机房
// @Description 更新机房基本信息
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.UpdateCmdbIdcRoomDto true "机房信息"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/idc-room/update [put]
// @Security ApiKeyAuth
func UpdateCmdbIdcRoom(c *gin.Context) {
	var dto model.UpdateCmdbIdcRoomDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	service.GetCmdbIdcRoomService().UpdateRoom(c, &dto)
}

// 删除机房
// @Summary 删除机房
// @Description 删除机房，机房下存在资产时不允许删除
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.CmdbIdcRoomIdDto true "机房ID"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/idc-room/delete [delete]
// @Security ApiKeyAuth
func DeleteCmdbIdcRoom(c *gin.Context) {
	var dto model.CmdbIdcRoomIdDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	service.GetCmdbIdcRoomService().DeleteRoom(c, dto.ID)
}

// 获取机房详情
// @Summary 获取机房详情
// @Description 根据ID获取机房详情，包含机柜和资产数量
// @Tags CMDB资产管理
// @Produce json
// @Param id query int true "机房ID"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/idc-room/info [get]
// @Security ApiKeyAuth
func GetCmdbIdcRoomById(c *gin.Context) {
	id := util.StringToUint(c.Query("id"))
	if id == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "机房ID不能为空")
		return
	}
	service.GetCmdbIdcRoomService().GetRoomById(c, id)
}

// 获取机房列表
// @Summary 获取机房列表
// @Description 获取机房列表
// @Tags CMDB资产管理
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/idc-room/list [get]
// @Security ApiKeyAuth
func GetCmdbIdcRoomList(c *gin.Context) {
	service.GetCmdbIdcRoomService().GetRoomList(c)
}

// 获取机房可视化布局
// @Summary 获取机房可视化布局
// @Description 按机柜名称中的列号和柜号生成机房可视化布局
// @Tags CMDB资产管理
// @Produce json
// @Param roomId query int true "机房ID"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/idc-room/layout [get]
// @Security ApiKeyAuth
func GetCmdbIdcRoomLayout(c *gin.Context) {
	roomID := util.StringToUint(c.Query("roomId"))
	if roomID == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "机房ID不能为空")
		return
	}
	service.GetCmdbIdcRoomService().GetRoomLayout(c, roomID)
}

// 创建机柜
// @Summary 创建机柜
// @Description 创建机柜并关联到指定机房
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.CreateCmdbIdcCabinetDto true "机柜信息"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/idc-cabinet/create [post]
// @Security ApiKeyAuth
func CreateCmdbIdcCabinet(c *gin.Context) {
	var dto model.CreateCmdbIdcCabinetDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	service.GetCmdbIdcRoomService().CreateCabinet(c, &dto)
}

// 更新机柜
// @Summary 更新机柜
// @Description 更新机柜基本信息
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.UpdateCmdbIdcCabinetDto true "机柜信息"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/idc-cabinet/update [put]
// @Security ApiKeyAuth
func UpdateCmdbIdcCabinet(c *gin.Context) {
	var dto model.UpdateCmdbIdcCabinetDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	service.GetCmdbIdcRoomService().UpdateCabinet(c, &dto)
}

// 删除机柜
// @Summary 删除机柜
// @Description 删除机柜，机柜下存在资产时不允许删除
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.CmdbIdcCabinetIdDto true "机柜ID"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/idc-cabinet/delete [delete]
// @Security ApiKeyAuth
func DeleteCmdbIdcCabinet(c *gin.Context) {
	var dto model.CmdbIdcCabinetIdDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	service.GetCmdbIdcRoomService().DeleteCabinet(c, dto.ID)
}

// 获取机柜列表
// @Summary 获取机柜列表
// @Description 获取机柜列表，可按机房筛选
// @Tags CMDB资产管理
// @Produce json
// @Param roomId query int false "机房ID"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/idc-cabinet/list [get]
// @Security ApiKeyAuth
func GetCmdbIdcCabinetList(c *gin.Context) {
	roomID := util.StringToUint(c.Query("roomId"))
	service.GetCmdbIdcRoomService().GetCabinetList(c, roomID)
}

// GetCmdbIdcCabinetLayout 获取机柜可视化布局
// @Summary 获取机柜可视化布局
// @Description 支持按机柜ID或机柜名称查询机柜与物理机位置关系
// @Tags CMDB资产管理
// @Produce json
// @Param cabinetId query int false "机柜ID"
// @Param roomId query int false "机房ID(按名称查机柜时推荐传入)"
// @Param cabinetName query string false "机柜名称"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/idc-cabinet/layout [get]
// @Security ApiKeyAuth
func GetCmdbIdcCabinetLayout(c *gin.Context) {
	cabinetID := util.StringToUint(c.Query("cabinetId"))
	roomID := util.StringToUint(c.Query("roomId"))
	cabinetName := c.Query("cabinetName")
	if cabinetID == 0 && cabinetName == "" {
		result.Failed(c, constant.INVALID_PARAMS, "机柜ID和机柜名称不能同时为空")
		return
	}

	service.GetCmdbIdcRoomService().GetCabinetLayout(c, cabinetID, roomID, cabinetName)
}
