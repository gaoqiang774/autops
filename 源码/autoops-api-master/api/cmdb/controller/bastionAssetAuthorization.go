package controller

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/api/cmdb/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/common/util"

	"github.com/gin-gonic/gin"
)

type BastionAssetAuthorizationController struct {
	service service.BastionAssetAuthorizationServiceInterface
}

func NewBastionAssetAuthorizationController() *BastionAssetAuthorizationController {
	return &BastionAssetAuthorizationController{
		service: service.GetBastionAssetAuthorizationService(),
	}
}

func (c *BastionAssetAuthorizationController) AuthorizeAssetsToUser(ctx *gin.Context) {
	var dto model.AuthorizeAssetsToUserDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.AuthorizeAssetsToUser(ctx, &dto)
}

func (c *BastionAssetAuthorizationController) AuthorizeAssetsToGroup(ctx *gin.Context) {
	var dto model.AuthorizeAssetsToGroupDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.AuthorizeAssetsToGroup(ctx, &dto)
}

func (c *BastionAssetAuthorizationController) RevokeAssetsFromUser(ctx *gin.Context) {
	var dto model.RevokeAssetsFromUserDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.RevokeAssetsFromUser(ctx, &dto)
}

func (c *BastionAssetAuthorizationController) RevokeAssetsFromGroup(ctx *gin.Context) {
	var dto model.RevokeAssetsFromGroupDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.RevokeAssetsFromGroup(ctx, &dto)
}

func (c *BastionAssetAuthorizationController) GetAuthorizedAssets(ctx *gin.Context) {
	var dto model.GetAuthorizedAssetsDto
	if err := ctx.ShouldBindQuery(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.GetAuthorizedAssets(ctx, &dto)
}

func (c *BastionAssetAuthorizationController) CheckUserAssetPermission(ctx *gin.Context) {
	userID := util.StringToUint(ctx.Query("userId"))
	assetID := util.StringToUint(ctx.Query("assetId"))
	assetType := ctx.Query("assetType")
	scopeType := ctx.Query("scopeType")
	scopeValue := ctx.Query("scopeValue")

	if userID == 0 || assetID == 0 || assetType == "" {
		result.Failed(ctx, constant.INVALID_PARAMS, "用户ID、资产类型和资产ID不能为空")
		return
	}
	c.service.CheckUserAssetPermission(ctx, userID, assetType, assetID, scopeType, scopeValue)
}
