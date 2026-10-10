package controller

import (
	"net/http"

	"dodevops-api/api/costcenter/service"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BalanceAlertController struct {
	service *service.BalanceAlertService
}

func NewBalanceAlertController(db *gorm.DB) *BalanceAlertController {
	return &BalanceAlertController{service: service.NewBalanceAlertService(db)}
}

func (c *BalanceAlertController) Overview(ctx *gin.Context) {
	data, err := c.service.Overview()
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	result.Success(ctx, data)
}

func (c *BalanceAlertController) GetRule(ctx *gin.Context) {
	id, ok := uintPath(ctx, "id")
	if !ok {
		return
	}
	data, err := c.service.GetRule(id)
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}
	result.Success(ctx, data)
}

func (c *BalanceAlertController) SaveRule(ctx *gin.Context) {
	id, ok := uintPath(ctx, "id")
	if !ok {
		return
	}
	var req service.SaveBalanceAlertRuleReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}
	data, err := c.service.SaveRule(id, req)
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}
	result.Success(ctx, data)
}
