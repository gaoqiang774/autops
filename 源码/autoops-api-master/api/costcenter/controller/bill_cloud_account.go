package controller

import (
	"net/http"
	"strconv"

	"dodevops-api/api/costcenter/service"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BillCloudAccountController struct {
	service *service.BillCloudAccountService
}

func NewBillCloudAccountController(db *gorm.DB) *BillCloudAccountController {
	return &BillCloudAccountController{service: service.NewBillCloudAccountService(db)}
}

func (c *BillCloudAccountController) GetList(ctx *gin.Context) {
	page, pageSize := pageQuery(ctx)
	var enabled *int
	if value := ctx.Query("enabled"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || (parsed != 0 && parsed != 1) {
			result.Failed(ctx, http.StatusBadRequest, "enabled 仅支持 0 或 1")
			return
		}
		enabled = &parsed
	}
	list, total, err := c.service.GetList(ctx.Query("operator"), ctx.Query("syncStatus"), enabled, page, pageSize)
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	result.SuccessWithPage(ctx, list, total, page, pageSize)
}

func (c *BillCloudAccountController) Create(ctx *gin.Context) {
	var req service.CreateBillReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}
	data, err := c.service.Create(&req)
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}
	result.Success(ctx, data)
}

func (c *BillCloudAccountController) Update(ctx *gin.Context) {
	id, ok := uintPath(ctx, "id")
	if !ok {
		return
	}
	var req service.UpdateBillReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}
	if err := c.service.Update(id, &req); err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}
	result.Success(ctx, nil)
}

func (c *BillCloudAccountController) Delete(ctx *gin.Context) {
	id, ok := uintPath(ctx, "id")
	if !ok {
		return
	}
	if err := c.service.Delete(id); err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}
	result.Success(ctx, nil)
}

func (c *BillCloudAccountController) ManualSync(ctx *gin.Context) {
	id, ok := uintPath(ctx, "id")
	if !ok {
		return
	}
	if err := c.service.ManualSync(id); err != nil {
		result.Failed(ctx, http.StatusConflict, err.Error())
		return
	}
	result.Success(ctx, gin.H{"accepted": true})
}

func (c *BillCloudAccountController) RefreshBalance(ctx *gin.Context) {
	id, ok := uintPath(ctx, "id")
	if !ok {
		return
	}
	if err := c.service.RefreshBalance(id); err != nil {
		result.Failed(ctx, http.StatusConflict, err.Error())
		return
	}
	result.Success(ctx, gin.H{"accepted": true})
}

func (c *BillCloudAccountController) SyncAll(ctx *gin.Context) {
	accepted, err := c.service.SyncAll()
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	result.Success(ctx, gin.H{"accepted": accepted})
}

func (c *BillCloudAccountController) RefreshAllBalances(ctx *gin.Context) {
	accepted, err := c.service.RefreshAllBalances()
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	result.Success(ctx, gin.H{"accepted": accepted})
}

func pageQuery(ctx *gin.Context) (int, int) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func uintPath(ctx *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(ctx.Param(name), 10, 32)
	if err != nil || value == 0 {
		result.Failed(ctx, http.StatusBadRequest, "无效的ID")
		return 0, false
	}
	return uint(value), true
}
