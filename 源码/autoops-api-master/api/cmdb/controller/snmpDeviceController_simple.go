package controller

import (
	"net/http"
	"strconv"
	"strings"

	"dodevops-api/api/cmdb/model"
	"dodevops-api/api/cmdb/service"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

type CmdbSnmpDeviceController struct {
	service        *service.CmdbSnmpDeviceService
	monitorService *service.SnmpMonitorService
}

func NewCmdbSnmpDeviceController() *CmdbSnmpDeviceController {
	return &CmdbSnmpDeviceController{
		service:        service.NewCmdbSnmpDeviceService(),
		monitorService: service.NewSnmpMonitorService(),
	}
}

func (ctrl *CmdbSnmpDeviceController) CreateSnmpDevice(c *gin.Context) {
	var dto model.CreateSnmpDeviceDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		result.Failed(c, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	if usernameStr == "" {
		usernameStr = "system"
	}

	device, err := ctrl.service.CreateSnmpDevice(&dto, usernameStr)
	if err != nil {
		result.Failed(c, http.StatusBadRequest, "create device failed: "+err.Error())
		return
	}

	result.Success(c, device.ToVO())
}

func (ctrl *CmdbSnmpDeviceController) UpdateSnmpDevice(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	var dto model.UpdateSnmpDeviceDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		result.Failed(c, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	dto.ID = id

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	if usernameStr == "" {
		usernameStr = "system"
	}

	if err := ctrl.service.UpdateSnmpDevice(&dto, usernameStr); err != nil {
		result.Failed(c, http.StatusBadRequest, "update device failed: "+err.Error())
		return
	}

	result.Success(c, nil)
}

func (ctrl *CmdbSnmpDeviceController) DeleteSnmpDevice(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	if err := ctrl.service.DeleteSnmpDevice(id); err != nil {
		result.Failed(c, http.StatusBadRequest, "delete device failed: "+err.Error())
		return
	}

	result.Success(c, nil)
}

func (ctrl *CmdbSnmpDeviceController) GetSnmpDeviceById(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	device, err := ctrl.service.GetSnmpDeviceById(id)
	if err != nil {
		result.Failed(c, http.StatusNotFound, "device not found")
		return
	}

	result.Success(c, device.ToVO())
}

func (ctrl *CmdbSnmpDeviceController) DecryptRemotePassword(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	password, err := ctrl.service.DecryptRemotePassword(id)
	if err != nil {
		result.Failed(c, http.StatusBadRequest, "decrypt password failed: "+err.Error())
		return
	}

	result.Success(c, gin.H{
		"password": password,
	})
}

func (ctrl *CmdbSnmpDeviceController) GetSnmpDeviceList(c *gin.Context) {
	query := model.SnmpDeviceQueryDto{
		DeviceType: c.Query("deviceType"),
		Status:     c.Query("status"),
		Keyword:    c.Query("keyword"),
		Page:       parseIntWithDefault(c.DefaultQuery("page", "1"), 1),
		PageSize:   parseIntWithDefault(c.DefaultQuery("pageSize", "10"), 10),
	}

	if monitorEnabledStr := c.Query("monitorEnabled"); monitorEnabledStr != "" {
		query.MonitorEnabled = parseIntWithDefault(monitorEnabledStr, -1)
	} else {
		query.MonitorEnabled = -1
	}

	listResult, err := ctrl.service.GetSnmpDeviceList(&query)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "get device list failed: "+err.Error())
		return
	}

	voItems := make([]*model.SnmpDeviceVO, 0, len(listResult.Items))
	for _, device := range listResult.Items {
		voItems = append(voItems, device.ToVO())
	}

	result.Success(c, map[string]interface{}{
		"items":    voItems,
		"total":    listResult.Total,
		"page":     listResult.Page,
		"pageSize": listResult.PageSize,
	})
}

func (ctrl *CmdbSnmpDeviceController) GetDeviceMonitorMetrics(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	metrics, err := ctrl.monitorService.GetDeviceMonitorMetrics(id)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "get monitor metrics failed: "+err.Error())
		return
	}

	result.Success(c, metrics)
}

func (ctrl *CmdbSnmpDeviceController) GetDeviceResourceMetrics(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	metrics, err := ctrl.monitorService.GetDeviceResourceMetrics(id)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "get resource metrics failed: "+err.Error())
		return
	}

	result.Success(c, metrics)
}

func (ctrl *CmdbSnmpDeviceController) GetBatchDeviceResourceMetrics(c *gin.Context) {
	deviceIDsParam := strings.TrimSpace(c.Query("deviceIds"))
	if deviceIDsParam == "" {
		result.Failed(c, http.StatusBadRequest, "deviceIds is required")
		return
	}

	parts := strings.Split(deviceIDsParam, ",")
	if len(parts) == 0 || len(parts) > 100 {
		result.Failed(c, http.StatusBadRequest, "deviceIds must contain 1 to 100 ids")
		return
	}

	deviceIDs := make([]uint, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			result.Failed(c, http.StatusBadRequest, "invalid device id")
			return
		}
		deviceIDs = append(deviceIDs, uint(parsed))
	}

	metrics, err := ctrl.monitorService.GetBatchDeviceResourceMetrics(deviceIDs)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "get batch resource metrics failed: "+err.Error())
		return
	}

	result.Success(c, metrics)
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	value := strings.TrimSpace(c.Param(name))
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		result.Failed(c, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return uint(parsed), true
}

func parseIntWithDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}
