package controller

import (
	"strconv"

	"dodevops-api/api/system/model"
	"dodevops-api/api/system/service"

	"github.com/gin-gonic/gin"
)

// @Tags System系统管理
// @Summary 新增导航分类
// @Description 新增导航分类，如个人常用、公共服务、AIOps运维等
// @Produce json
// @Param data body model.SysNavCategoryDto true "导航分类"
// @Success 200 {object} result.Result
// @Router /api/v1/system/nav/category/add [post]
// @Security ApiKeyAuth
func CreateSysNavCategory(c *gin.Context) {
	var dto model.SysNavCategoryDto
	_ = c.BindJSON(&dto)
	service.NewSysNavigationService().CreateCategory(c, dto)
}

// @Tags System系统管理
// @Summary 修改导航分类
// @Description 修改导航分类名称、编码、图标、排序、状态等信息
// @Produce json
// @Param data body model.SysNavCategoryDto true "导航分类"
// @Success 200 {object} result.Result
// @Router /api/v1/system/nav/category/update [put]
// @Security ApiKeyAuth
func UpdateSysNavCategory(c *gin.Context) {
	var dto model.SysNavCategoryDto
	_ = c.BindJSON(&dto)
	service.NewSysNavigationService().UpdateCategory(c, dto)
}

// @Tags System系统管理
// @Summary 删除导航分类
// @Description 删除导航分类，分类下存在导航链接时不允许删除
// @Produce json
// @Param data body model.SysNavIDDto true "分类ID"
// @Success 200 {object} result.Result
// @Router /api/v1/system/nav/category/delete [delete]
// @Security ApiKeyAuth
func DeleteSysNavCategory(c *gin.Context) {
	var dto model.SysNavIDDto
	_ = c.BindJSON(&dto)
	service.NewSysNavigationService().DeleteCategory(c, dto)
}

// @Tags System系统管理
// @Summary 获取导航分类详情
// @Description 根据分类ID获取导航分类详情
// @Produce json
// @Param id query int true "分类ID"
// @Success 200 {object} result.Result
// @Router /api/v1/system/nav/category/info [get]
// @Security ApiKeyAuth
func GetSysNavCategory(c *gin.Context) {
	id := parseUintQuery(c, "id")
	service.NewSysNavigationService().GetCategory(c, id)
}

// @Tags System系统管理
// @Summary 查询导航分类列表
// @Description 查询导航分类列表，可按分类名称和状态过滤
// @Produce json
// @Param name query string false "分类名称"
// @Param status query string false "状态：1->启用,2->禁用"
// @Success 200 {object} result.Result
// @Router /api/v1/system/nav/category/list [get]
// @Security ApiKeyAuth
func ListSysNavCategories(c *gin.Context) {
	query := model.SysNavCategoryQuery{
		Name:   c.Query("name"),
		Status: c.Query("status"),
	}
	service.NewSysNavigationService().ListCategories(c, query)
}

// @Tags System系统管理
// @Summary 新增导航链接
// @Description 新增导航链接，scope 支持 public/private/acl，分别表示公共、个人、授权可见
// @Produce json
// @Param data body model.SysNavItemDto true "导航链接"
// @Success 200 {object} result.Result
// @Router /api/v1/system/nav/item/add [post]
// @Security ApiKeyAuth
func CreateSysNavItem(c *gin.Context) {
	var dto model.SysNavItemDto
	_ = c.BindJSON(&dto)
	service.NewSysNavigationService().CreateItem(c, dto)
}

// @Tags System系统管理
// @Summary 修改导航链接
// @Description 修改导航链接基础信息、可见范围、SSO配置、排序和状态
// @Produce json
// @Param data body model.SysNavItemDto true "导航链接"
// @Success 200 {object} result.Result
// @Router /api/v1/system/nav/item/update [put]
// @Security ApiKeyAuth
func UpdateSysNavItem(c *gin.Context) {
	var dto model.SysNavItemDto
	_ = c.BindJSON(&dto)
	service.NewSysNavigationService().UpdateItem(c, dto)
}

// @Tags System系统管理
// @Summary 删除导航链接
// @Description 根据导航链接ID删除导航链接
// @Produce json
// @Param data body model.SysNavIDDto true "导航链接ID"
// @Success 200 {object} result.Result
// @Router /api/v1/system/nav/item/delete [delete]
// @Security ApiKeyAuth
func DeleteSysNavItem(c *gin.Context) {
	var dto model.SysNavIDDto
	_ = c.BindJSON(&dto)
	service.NewSysNavigationService().DeleteItem(c, dto)
}

// @Tags System系统管理
// @Summary 获取导航链接详情
// @Description 根据导航链接ID获取导航链接详情
// @Produce json
// @Param id query int true "导航链接ID"
// @Success 200 {object} result.Result
// @Router /api/v1/system/nav/item/info [get]
// @Security ApiKeyAuth
func GetSysNavItem(c *gin.Context) {
	id := parseUintQuery(c, "id")
	service.NewSysNavigationService().GetItem(c, id)
}

// @Tags System系统管理
// @Summary 分页查询导航链接
// @Description 后台管理分页查询导航链接，可按分类、名称、可见范围、状态、所属用户过滤
// @Produce json
// @Param pageNum query int false "分页页码"
// @Param pageSize query int false "每页数量"
// @Param categoryId query int false "分类ID"
// @Param name query string false "导航名称"
// @Param scope query string false "可见范围：public/private/acl"
// @Param status query string false "状态：1->启用,2->禁用"
// @Param ownerId query int false "所属用户ID"
// @Success 200 {object} result.Result
// @Router /api/v1/system/nav/item/list [get]
// @Security ApiKeyAuth
func ListSysNavItems(c *gin.Context) {
	query := model.SysNavItemQuery{
		PageNum:    parseIntQuery(c, "pageNum"),
		PageSize:   parseIntQuery(c, "pageSize"),
		CategoryID: parseUintQuery(c, "categoryId"),
		Name:       c.Query("name"),
		Scope:      c.Query("scope"),
		Status:     c.Query("status"),
		OwnerID:    parseUintQuery(c, "ownerId"),
	}
	service.NewSysNavigationService().ListItems(c, query)
}

// @Tags System系统管理
// @Summary 获取当前用户可见导航
// @Description 获取当前用户可见导航，返回公共导航、个人导航和授权导航，并按分类分组
// @Produce json
// @Param keyword query string false "搜索关键字"
// @Param categoryId query int false "分类ID，不传表示全部导航"
// @Success 200 {object} result.Result
// @Router /api/v1/system/nav/visible [get]
// @Security ApiKeyAuth
func GetVisibleSysNavigation(c *gin.Context) {
	service.NewSysNavigationService().GetVisibleNavigation(c, c.Query("keyword"), parseUintQuery(c, "categoryId"))
}

func parseIntQuery(c *gin.Context, key string) int {
	value, _ := strconv.Atoi(c.Query(key))
	return value
}

func parseUintQuery(c *gin.Context, key string) uint {
	value, _ := strconv.ParseUint(c.Query(key), 10, 64)
	return uint(value)
}
