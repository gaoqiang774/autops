package controller

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/api/cmdb/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/common/util"

	"github.com/gin-gonic/gin"
)

type BastionHostAuthorizationController struct {
	service service.BastionHostAuthorizationServiceInterface
}

func NewBastionHostAuthorizationController() *BastionHostAuthorizationController {
	return &BastionHostAuthorizationController{
		service: service.GetBastionHostAuthorizationService(),
	}
}

// @Summary Authorize hosts to user
// @Description Supports both shared lifecycle (`hostIds` + top-level `effectiveTime`/`expireTime`) and per-host lifecycle (`authorizations[]`).
// @Description Example(shared): {"userId":1001,"hostIds":[11,12,13],"effectiveTime":"2026-04-24 09:00:00","expireTime":"2026-04-30 23:59:59"}
// @Description Example(per-host): {"userId":1001,"authorizations":[{"hostId":11,"effectiveTime":"2026-04-24 09:00:00","expireTime":"2026-04-25 18:00:00"},{"hostId":12,"effectiveTime":"2026-04-24 10:00:00","expireTime":"2026-05-01 00:00:00"}]}
// @Tags Bastion Authorization
// @Accept json
// @Produce json
// @Param data body model.AuthorizeHostsToUserDto true "authorization payload"
// @Success 200 {object} result.Result
// @Router /api/v1/bastion/auth/user/authorize [post]
// @Security ApiKeyAuth
func (c *BastionHostAuthorizationController) AuthorizeHostsToUser(ctx *gin.Context) {
	var dto model.AuthorizeHostsToUserDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.AuthorizeHostsToUser(ctx, &dto)
}

// @Summary Authorize hosts to group
// @Description Supports both shared lifecycle (`hostIds` + top-level `effectiveTime`/`expireTime`) and per-host lifecycle (`authorizations[]`).
// @Description Example(shared): {"groupId":2001,"hostIds":[11,12,13],"effectiveTime":"2026-04-24 09:00:00","expireTime":"2026-04-30 23:59:59"}
// @Description Example(per-host): {"groupId":2001,"authorizations":[{"hostId":11,"effectiveTime":"2026-04-24 09:00:00","expireTime":"2026-04-25 18:00:00"},{"hostId":12,"effectiveTime":"2026-04-24 10:00:00","expireTime":"2026-05-01 00:00:00"}]}
// @Tags Bastion Authorization
// @Accept json
// @Produce json
// @Param data body model.AuthorizeHostsToGroupDto true "authorization payload"
// @Success 200 {object} result.Result
// @Router /api/v1/bastion/auth/group/authorize [post]
// @Security ApiKeyAuth
func (c *BastionHostAuthorizationController) AuthorizeHostsToGroup(ctx *gin.Context) {
	var dto model.AuthorizeHostsToGroupDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.AuthorizeHostsToGroup(ctx, &dto)
}

// @Summary Revoke user host authorization
// @Description Batch revoke host authorization from a user.
// @Tags Bastion Authorization
// @Accept json
// @Produce json
// @Param data body model.RevokeHostsFromUserDto true "revoke payload"
// @Success 200 {object} result.Result
// @Router /api/v1/bastion/auth/user/revoke [post]
// @Security ApiKeyAuth
func (c *BastionHostAuthorizationController) RevokeHostsFromUser(ctx *gin.Context) {
	var dto model.RevokeHostsFromUserDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.RevokeHostsFromUser(ctx, &dto)
}

// @Summary Revoke group host authorization
// @Description Batch revoke host authorization from a group.
// @Tags Bastion Authorization
// @Accept json
// @Produce json
// @Param data body model.RevokeHostsFromGroupDto true "revoke payload"
// @Success 200 {object} result.Result
// @Router /api/v1/bastion/auth/group/revoke [post]
// @Security ApiKeyAuth
func (c *BastionHostAuthorizationController) RevokeHostsFromGroup(ctx *gin.Context) {
	var dto model.RevokeHostsFromGroupDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.RevokeHostsFromGroup(ctx, &dto)
}

// @Summary Get authorized hosts
// @Description Get authorized host records for a user or group.
// @Tags Bastion Authorization
// @Accept json
// @Produce json
// @Param subjectType query string true "authorization subject type (user/group)"
// @Param subjectId query uint true "authorization subject id"
// @Param includeExpired query bool false "include expired authorization records"
// @Param page query int true "page number"
// @Param pageSize query int true "page size"
// @Success 200 {object} result.Result
// @Router /api/v1/bastion/auth/hosts [get]
// @Security ApiKeyAuth
func (c *BastionHostAuthorizationController) GetAuthorizedHosts(ctx *gin.Context) {
	var dto model.GetAuthorizedHostsDto
	if err := ctx.ShouldBindQuery(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.GetAuthorizedHosts(ctx, &dto)
}

// @Summary Get user authorized hosts
// @Description Get all currently accessible hosts for a user, including inherited group authorization.
// @Tags Bastion Authorization
// @Accept json
// @Produce json
// @Param userId query uint true "user id"
// @Param page query int true "page number"
// @Param pageSize query int true "page size"
// @Success 200 {object} result.Result
// @Router /api/v1/bastion/user/authorized-hosts [get]
// @Security ApiKeyAuth
func (c *BastionHostAuthorizationController) GetUserAuthorizedHosts(ctx *gin.Context) {
	userID := util.StringToUint(ctx.Query("userId"))
	page := util.StringToInt(ctx.Query("page"))
	pageSize := util.StringToInt(ctx.Query("pageSize"))

	if userID == 0 {
		result.Failed(ctx, constant.INVALID_PARAMS, "用户ID不能为空")
		return
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	c.service.GetUserAuthorizedHosts(ctx, userID, page, pageSize)
}

// @Summary Check user host permission
// @Description Check whether a user currently has access to a specific host.
// @Tags Bastion Authorization
// @Accept json
// @Produce json
// @Param userId query uint true "user id"
// @Param hostId query uint true "host id"
// @Success 200 {object} result.Result
// @Router /api/v1/bastion/user/check-permission [get]
// @Security ApiKeyAuth
func (c *BastionHostAuthorizationController) CheckUserHostPermission(ctx *gin.Context) {
	userID := util.StringToUint(ctx.Query("userId"))
	hostID := util.StringToUint(ctx.Query("hostId"))
	authID := util.StringToUint(ctx.Query("authId"))

	if userID == 0 || hostID == 0 {
		result.Failed(ctx, constant.INVALID_PARAMS, "用户ID和主机ID不能为空")
		return
	}

	c.service.CheckUserHostPermission(ctx, userID, hostID, authID)
}
