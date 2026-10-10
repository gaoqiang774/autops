package controller

import (
	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	"dodevops-api/api/cmdb/service"
	"dodevops-api/common"
	"dodevops-api/common/result"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CmdbSQLController struct {
	service *service.CmdbSQLService
}

func NewCmdbSQLController() *CmdbSQLController {
	sqlService := service.NewCmdbSQLService(dao.NewCmdbSQLDao(common.GetDB()))
	return &CmdbSQLController{service: sqlService}
}

// CreateDatabase godoc
// @Summary Create database asset
// @Description Create database asset in CMDB
// @Tags CMDB Database Asset
// @Accept json
// @Produce json
// @Param data body model.CmdbSQL true "Database asset"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/database [post]
// @Security ApiKeyAuth
func (c *CmdbSQLController) CreateDatabase(ctx *gin.Context) {
	var database model.CmdbSQL
	if err := ctx.ShouldBindJSON(&database); err != nil {
		result.Failed(ctx, 400, "invalid request body")
		return
	}

	if err := c.service.CreateDatabase(&database); err != nil {
		result.Failed(ctx, 500, "create database asset failed: "+err.Error())
		return
	}

	result.Success(ctx, database)
}

// UpdateDatabase godoc
// @Summary Update database asset
// @Description Update database asset in CMDB
// @Tags CMDB Database Asset
// @Accept json
// @Produce json
// @Param data body model.CmdbSQL true "Database asset"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/database [put]
// @Security ApiKeyAuth
func (c *CmdbSQLController) UpdateDatabase(ctx *gin.Context) {
	var database model.CmdbSQL
	if err := ctx.ShouldBindJSON(&database); err != nil {
		result.Failed(ctx, 400, "invalid request body")
		return
	}

	if err := c.service.UpdateDatabase(&database); err != nil {
		result.Failed(ctx, 500, "update database asset failed: "+err.Error())
		return
	}

	result.Success(ctx, database)
}

// DeleteDatabase godoc
// @Summary Delete database asset
// @Description Delete database asset from CMDB
// @Tags CMDB Database Asset
// @Accept json
// @Produce json
// @Param id query int false "Database asset ID"
// @Param body body model.CmdbSQLIdDto false "Database asset ID"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/database [delete]
// @Security ApiKeyAuth
func (c *CmdbSQLController) DeleteDatabase(ctx *gin.Context) {
	id := parseDatabaseID(ctx)
	if id == 0 {
		var req model.CmdbSQLIdDto
		if err := ctx.ShouldBindJSON(&req); err == nil {
			id = req.ID
		}
	}
	if id == 0 {
		result.Failed(ctx, 400, "id is required")
		return
	}

	if err := c.service.DeleteDatabase(id); err != nil {
		result.Failed(ctx, 500, "delete database asset failed: "+err.Error())
		return
	}

	result.Success(ctx, nil)
}

// GetDatabase godoc
// @Summary Get database asset
// @Description Get database asset detail by ID
// @Tags CMDB Database Asset
// @Accept json
// @Produce json
// @Param id query int true "Database asset ID"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/database/info [get]
// @Security ApiKeyAuth
func (c *CmdbSQLController) GetDatabase(ctx *gin.Context) {
	id := parseDatabaseID(ctx)
	if id == 0 {
		result.Failed(ctx, 400, "id is required")
		return
	}

	database, err := c.service.GetDatabase(id)
	if err != nil {
		result.Failed(ctx, 500, "query database asset failed: "+err.Error())
		return
	}

	result.Success(ctx, database)
}

// ListDatabases godoc
// @Summary List database assets
// @Description List database assets with pagination and filters
// @Tags CMDB Database Asset
// @Accept json
// @Produce json
// @Param page query int false "Page" default(1)
// @Param pageSize query int false "Page size" default(10)
// @Param keyword query string false "Keyword"
// @Param type query int false "Database type"
// @Param environment query string false "Environment"
// @Param status query int false "Status"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/databaselist [get]
// @Security ApiKeyAuth
func (c *CmdbSQLController) ListDatabases(ctx *gin.Context) {
	var query model.CmdbSQLQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, 400, "invalid query params")
		return
	}
	query.Page, query.PageSize = normalizeDatabasePageParams(query.Page, query.PageSize)

	databases, total, err := c.service.QueryDatabases(query)
	if err != nil {
		result.Failed(ctx, 500, "query database assets failed: "+err.Error())
		return
	}

	result.SuccessWithPage(ctx, databases, total, query.Page, query.PageSize)
}

func (c *CmdbSQLController) GetDatabasesByName(ctx *gin.Context) {
	name := ctx.Query("name")
	if name == "" {
		result.Failed(ctx, 400, "name is required")
		return
	}

	databases, err := c.service.GetDatabasesByName(name)
	if err != nil {
		result.Failed(ctx, 500, "query database assets failed: "+err.Error())
		return
	}

	result.Success(ctx, databases)
}

func (c *CmdbSQLController) GetDatabasesByType(ctx *gin.Context) {
	dbType, err := strconv.Atoi(ctx.Query("type"))
	if err != nil {
		result.Failed(ctx, 400, "invalid database type")
		return
	}

	databases, err := c.service.GetDatabasesByType(dbType)
	if err != nil {
		result.Failed(ctx, 500, "query database assets failed: "+err.Error())
		return
	}

	result.Success(ctx, databases)
}

func (c *CmdbSQLController) GetDatabaseStatistics(ctx *gin.Context) {
	stats, err := c.service.GetDatabaseStatistics()
	if err != nil {
		result.Failed(ctx, 500, "query database asset statistics failed: "+err.Error())
		return
	}
	result.Success(ctx, stats)
}

func (c *CmdbSQLController) TestDatabaseAuth(ctx *gin.Context) {
	id := parseDatabaseID(ctx)
	if id == 0 {
		var req model.CmdbSQLIdDto
		if err := ctx.ShouldBindJSON(&req); err == nil {
			id = req.ID
		}
	}
	if id == 0 {
		result.Failed(ctx, 400, "id is required")
		return
	}

	data, err := c.service.TestDatabaseAuth(ctx, id)
	if err != nil {
		result.Failed(ctx, 500, "test database auth failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

func parseDatabaseID(ctx *gin.Context) uint {
	value := ctx.Query("id")
	if value == "" {
		value = ctx.Param("id")
	}
	id, _ := strconv.Atoi(value)
	if id < 0 {
		return 0
	}
	return uint(id)
}

func normalizeDatabasePageParams(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
