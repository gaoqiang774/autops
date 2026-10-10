package controller

import (
	"fmt"

	"dodevops-api/api/cmdb/model"
	"dodevops-api/api/cmdb/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

type CmdbHostCloudController struct {
	service service.CmdbHostCloudServiceInterface
}

func NewCmdbHostCloudController() *CmdbHostCloudController {
	return &CmdbHostCloudController{
		service: service.GetCmdbHostCloudService(),
	}
}

// @Summary 创建阿里云主机
// @Description 通过阿里云 API 获取主机信息
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.CreateCmdbHostCloudDto true "阿里云主机信息"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostcloudcreatealiyun [post]
// @Security ApiKeyAuth
func (c *CmdbHostCloudController) CreateAliyunHost(ctx *gin.Context) {
	var dto model.CreateCmdbHostCloudDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	fmt.Printf("[DEBUG] Aliyun Cloud DTO - AccessKey: '%s' (len=%d), SecretKey: '%s' (len=%d)\n",
		dto.AccessKey, len(dto.AccessKey), dto.SecretKey, len(dto.SecretKey))
	c.service.CreateAliyunHost(ctx, &dto)
}

// @Summary 创建腾讯云主机
// @Description 通过腾讯云 API 获取主机信息
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.CreateCmdbHostCloudDto true "腾讯云主机信息"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostcloudcreatetencent [post]
// @Security ApiKeyAuth
func (c *CmdbHostCloudController) CreateTencentHost(ctx *gin.Context) {
	var dto model.CreateCmdbHostCloudDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	fmt.Printf("[DEBUG] Tencent Cloud DTO - AccessKey: '%s' (len=%d), SecretKey: '%s' (len=%d)\n",
		dto.AccessKey, len(dto.AccessKey), dto.SecretKey, len(dto.SecretKey))
	c.service.CreateTencentHost(ctx, &dto)
}

// @Summary 创建百度云主机
// @Description 通过百度云 API 自动扫描区域并获取主机信息
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.CreateCmdbHostCloudDto true "百度云主机信息(AccessKey和SecretKey)"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostcloudcreatebaidu [post]
// @Security ApiKeyAuth
func (c *CmdbHostCloudController) CreateBaiduHost(ctx *gin.Context) {
	var dto model.CreateCmdbHostCloudDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.CreateBaiduHost(ctx, &dto)
}

// @Summary 创建华为云主机
// @Description 通过华为云 API 自动扫描区域并获取主机信息
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.CreateCmdbHostCloudDto true "华为云主机信息(AccessKey和SecretKey)"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostcloudcreatehuawei [post]
// @Security ApiKeyAuth
func (c *CmdbHostCloudController) CreateHuaweiHost(ctx *gin.Context) {
	var dto model.CreateCmdbHostCloudDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	fmt.Printf("[DEBUG] Huawei Cloud DTO - AccessKey: '%s' (len=%d), SecretKey: '%s' (len=%d)\n",
		dto.AccessKey, len(dto.AccessKey), dto.SecretKey, len(dto.SecretKey))
	if dto.AccessKey == "" || dto.SecretKey == "" {
		result.Failed(ctx, constant.INVALID_PARAMS, "AccessKey和SecretKey不能为空，请检查请求字段")
		return
	}
	c.service.CreateHuaweiHost(ctx, &dto)
}

// @Summary 创建AWS主机
// @Description 通过 AWS API 自动扫描区域并获取主机信息
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.CreateCmdbHostCloudDto true "AWS主机信息(AccessKey和SecretKey)"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostcloudcreateaws [post]
// @Security ApiKeyAuth
func (c *CmdbHostCloudController) CreateAWSHost(ctx *gin.Context) {
	var dto model.CreateCmdbHostCloudDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	fmt.Printf("[DEBUG] AWS Cloud DTO - AccessKey: '%s' (len=%d), SecretKey: '%s' (len=%d)\n",
		dto.AccessKey, len(dto.AccessKey), dto.SecretKey, len(dto.SecretKey))
	c.service.CreateAWSHost(ctx, &dto)
}

// @Summary 创建天翼云主机
// @Description 通过天翼云 API 自动扫描区域并获取主机信息
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.CreateCmdbHostCloudDto true "天翼云主机信息(AccessKey和SecretKey)"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostcloudcreatetianyi [post]
// @Security ApiKeyAuth
func (c *CmdbHostCloudController) CreateTianyiHost(ctx *gin.Context) {
	var dto model.CreateCmdbHostCloudDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	fmt.Printf("[DEBUG] Tianyi Cloud DTO - AccessKey: '%s' (len=%d), SecretKey: '%s' (len=%d)\n",
		dto.AccessKey, len(dto.AccessKey), dto.SecretKey, len(dto.SecretKey))
	if dto.AccessKey == "" || dto.SecretKey == "" {
		result.Failed(ctx, constant.INVALID_PARAMS, "AccessKey和SecretKey不能为空")
		return
	}
	c.service.CreateTianyiHost(ctx, &dto)
}
