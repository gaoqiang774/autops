package controller

import (
	"errors"
	"net/http"
	"strconv"

	"dodevops-api/api/costcenter/service"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CloudBillDashboardController struct {
	service *service.CloudBillDashboardService
}

func NewCloudBillDashboardController(db *gorm.DB) *CloudBillDashboardController {
	return &CloudBillDashboardController{service: service.NewCloudBillDashboardService(db)}
}

func (c *CloudBillDashboardController) Overview(ctx *gin.Context) {
	data, err := c.service.Overview(ctx.Query("month"))
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}
	result.Success(ctx, data)
}

func (c *CloudBillDashboardController) ProviderDetail(ctx *gin.Context) {
	cloudAccountID, _ := strconv.ParseUint(ctx.Query("cloudAccountId"), 10, 32)
	data, err := c.service.ProviderDetail(ctx.Param("operator"), ctx.Query("month"), uint(cloudAccountID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Failed(ctx, http.StatusNotFound, "云厂商账单数据源不存在")
			return
		}
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}
	result.Success(ctx, data)
}
