package controller

import (
	"encoding/json"
	"strconv"

	"dodevops-api/api/cmdb/model"
	"dodevops-api/api/cmdb/service"

	"github.com/gin-gonic/gin"
)

type MonitorDatasourceController struct {
	svc *service.MonitorDatasourceService
}

func NewMonitorDatasourceController(svc *service.MonitorDatasourceService) *MonitorDatasourceController {
	return &MonitorDatasourceController{svc: svc}
}

func (c *MonitorDatasourceController) CreatePrometheus(ctx *gin.Context) {
	var req struct {
		Name       string                     `json:"name"`
		Properties model.PrometheusProperties `json:"properties"`
		Tags       map[string]interface{}     `json:"tags"`
		InstanceID string                     `json:"instance_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	tagsJSON, _ := json.Marshal(req.Tags)
	if err := c.svc.CreatePrometheus(req.Name, req.Properties, tagsJSON, req.InstanceID); err != nil {
		ctx.JSON(500, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	ctx.JSON(200, gin.H{"code": 0, "msg": "success"})
}

func (c *MonitorDatasourceController) Update(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 32)
	var updates map[string]interface{}
	if err := ctx.ShouldBindJSON(&updates); err != nil {
		ctx.JSON(400, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	delete(updates, "id")
	delete(updates, "created_at")

	if err := c.svc.Update(uint(id), updates); err != nil {
		ctx.JSON(500, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	ctx.JSON(200, gin.H{"code": 0, "msg": "success"})
}

func (c *MonitorDatasourceController) Delete(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err := c.svc.Delete(uint(id)); err != nil {
		ctx.JSON(500, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	ctx.JSON(200, gin.H{"code": 0, "msg": "success"})
}

func (c *MonitorDatasourceController) Get(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 32)
	res, err := c.svc.GetByID(uint(id))
	if err != nil {
		ctx.JSON(404, gin.H{"code": 404, "msg": "not found"})
		return
	}
	ctx.JSON(200, gin.H{"code": 0, "data": res})
}

func (c *MonitorDatasourceController) List(ctx *gin.Context) {
	resourceType := ctx.Query("resource_type")
	status := ctx.Query("status")
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))

	list, total, err := c.svc.List(resourceType, status, page, pageSize)
	if err != nil {
		ctx.JSON(500, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	ctx.JSON(200, gin.H{"code": 0, "data": gin.H{"list": list, "total": total, "page": page, "pageSize": pageSize}})
}
