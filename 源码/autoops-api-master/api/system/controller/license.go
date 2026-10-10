package controller

import (
	"net/http"
	"strings"

	"dodevops-api/api/system/service"
	"dodevops-api/common/result"
	"dodevops-api/pkg/jwt"
	"dodevops-api/pkg/license"

	"github.com/gin-gonic/gin"
)

type activateLicenseRequest struct {
	LicenseKey string `json:"licenseKey" binding:"required"`
}

func GetLicenseStatus(c *gin.Context) {
	result.Success(c, license.DefaultManager().Status())
}

func GetLicenseSessionAccess(c *gin.Context) {
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		result.Failed(c, http.StatusUnauthorized, "未获取到当前登录用户")
		return
	}
	result.Success(c, service.BuildSessionAccess(admin.ID, admin.Username))
}

func ActivateLicense(c *gin.Context) {
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		result.Failed(c, http.StatusUnauthorized, "未获取到当前登录用户")
		return
	}
	username := strings.ToLower(strings.TrimSpace(admin.Username))
	if username != "admin" && username != "root" {
		result.Failed(c, http.StatusForbidden, "仅系统管理员可以激活授权")
		return
	}

	var request activateLicenseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		result.Failed(c, http.StatusBadRequest, "请输入授权码")
		return
	}
	status, err := license.DefaultManager().Activate(request.LicenseKey)
	if err != nil {
		result.Failed(c, http.StatusBadRequest, status.Message)
		return
	}
	result.Success(c, status)
}
