package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"dodevops-api/api/costcenter/service"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BillDetailController struct {
	service *service.BillDetailService
}

func NewBillDetailController(db *gorm.DB) *BillDetailController {
	return &BillDetailController{service: service.NewBillDetailService(db)}
}

func (c *BillDetailController) GetList(ctx *gin.Context) {
	query, ok := billDetailQuery(ctx, true)
	if !ok {
		return
	}
	rows, total, err := c.service.List(query)
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	result.SuccessWithPage(ctx, rows, total, query.Page, query.PageSize)
}

func (c *BillDetailController) GetByID(ctx *gin.Context) {
	id, ok := uintPath(ctx, "id")
	if !ok {
		return
	}
	detail, err := c.service.GetByID(id)
	if err != nil {
		status := http.StatusInternalServerError
		message := err.Error()
		if err == gorm.ErrRecordNotFound {
			status = http.StatusNotFound
			message = "账单明细不存在"
		}
		result.Failed(ctx, status, message)
		return
	}
	result.Success(ctx, detail)
}

func (c *BillDetailController) Export(ctx *gin.Context) {
	query, ok := billDetailQuery(ctx, false)
	if !ok {
		return
	}
	buffer, err := c.service.Export(query)
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}
	filename := fmt.Sprintf("cloud_bill_details_%s.xlsx", time.Now().Format("20060102150405"))
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	ctx.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func billDetailQuery(ctx *gin.Context, paged bool) (service.BillDetailQuery, bool) {
	query := service.BillDetailQuery{
		Month:       ctx.Query("month"),
		Operator:    ctx.Query("operator"),
		Keyword:     ctx.Query("keyword"),
		ProductName: ctx.Query("productName"),
		ChargeType:  ctx.Query("chargeType"),
		Region:      ctx.Query("region"),
	}
	if value := ctx.Query("cloudAccountId"); value != "" {
		id, err := strconv.ParseUint(value, 10, 32)
		if err != nil || id == 0 {
			result.Failed(ctx, http.StatusBadRequest, "cloudAccountId 无效")
			return query, false
		}
		query.CloudAccountID = uint(id)
	}
	if paged {
		query.Page, query.PageSize = pageQuery(ctx)
	}
	return query, true
}
