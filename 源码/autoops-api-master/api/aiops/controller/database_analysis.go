package controller

import (
	"dodevops-api/api/aiops/model"
	"dodevops-api/api/aiops/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

func AnalyzeMySQL(c *gin.Context) {
	var req model.MySQLAnalyzeRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "invalid request body")
		return
	}

	data, err := service.NewMySQLAnalysisService().AnalyzeMySQL(c, &req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "analyze mysql failed: "+err.Error())
		return
	}
	result.Success(c, data)
}
