package controller

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"dodevops-api/api/cmdb/service"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

type WindowsExporterController struct {
	exporterService *service.WindowsExporterService
}

func NewWindowsExporterController() *WindowsExporterController {
	return &WindowsExporterController{
		exporterService: service.NewWindowsExporterService(),
	}
}

func (c *WindowsExporterController) InstallExporter(ctx *gin.Context) {
	var req service.InstallExporterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	go func(req service.InstallExporterRequest) {
		if err := c.exporterService.InstallWindowsExporter(&req); err != nil {
			log.Printf("install windows exporter failed for device %d: %v", req.DeviceID, err)
		}
	}(req)

	result.Success(ctx, gin.H{"message": "安装任务已提交，请稍后查询安装状态"})
}

func (c *WindowsExporterController) PrecheckExporter(ctx *gin.Context) {
	var req service.ExporterPrecheckRequest
	if deviceIDStr := ctx.Query("deviceId"); deviceIDStr != "" {
		deviceID, err := strconv.ParseUint(deviceIDStr, 10, 32)
		if err != nil {
			result.Failed(ctx, http.StatusBadRequest, "设备ID格式错误")
			return
		}
		req.DeviceID = uint(deviceID)
	}
	req.Host = ctx.Query("host")
	req.Username = ctx.Query("username")
	req.Password = ctx.Query("password")
	req.Domain = ctx.Query("domain")
	if winrmPortStr := ctx.Query("winrmPort"); winrmPortStr != "" {
		winrmPort, err := strconv.Atoi(winrmPortStr)
		if err != nil {
			result.Failed(ctx, http.StatusBadRequest, "winrmPort格式错误")
			return
		}
		req.WinRMPort = winrmPort
	}

	report, err := c.exporterService.PrecheckWindowsExporter(&req)
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, err.Error())
		return
	}

	result.Success(ctx, report)
}

func (c *WindowsExporterController) GetExporterStatus(ctx *gin.Context) {
	deviceID, err := parseExporterDeviceID(ctx.Param("deviceId"))
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, "设备ID格式错误")
		return
	}

	status, err := c.exporterService.GetExporterStatus(deviceID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrWindowsExporterDeviceNotFound):
			result.Failed(ctx, http.StatusNotFound, "设备不存在")
		case errors.Is(err, service.ErrWindowsExporterDeviceTypeInvalid):
			result.Failed(ctx, http.StatusBadRequest, "设备不是 Windows 类型")
		default:
			result.Failed(ctx, http.StatusInternalServerError, err.Error())
		}
		return
	}

	result.Success(ctx, status)
}

func (c *WindowsExporterController) UninstallExporter(ctx *gin.Context) {
	deviceID, err := parseExporterDeviceID(ctx.Param("deviceId"))
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, "设备ID格式错误")
		return
	}

	go func(deviceID uint) {
		if err := c.exporterService.UninstallExporter(deviceID); err != nil {
			log.Printf("uninstall windows exporter failed for device %d: %v", deviceID, err)
		}
	}(deviceID)

	result.Success(ctx, gin.H{"message": "卸载任务已提交"})
}

func (c *WindowsExporterController) BatchInstallExporter(ctx *gin.Context) {
	var req BatchInstallExporterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	go func(req BatchInstallExporterRequest) {
		installReq := &service.InstallExporterRequest{
			InstallerURL: req.InstallerURL,
			Version:      req.Version,
			ExporterPort: req.ExporterPort,
		}
		results := c.exporterService.BatchInstallExporter(installReq, req.DeviceIDs)
		for deviceID, err := range results {
			if err != nil {
				log.Printf("batch install windows exporter failed for device %d: %v", deviceID, err)
			}
		}
	}(req)

	result.Success(ctx, gin.H{"message": "批量安装任务已提交"})
}

func (c *WindowsExporterController) GetAllWindowsDevicesStatus(ctx *gin.Context) {
	statuses, err := c.exporterService.GetAllWindowsDevicesStatus()
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, err.Error())
		return
	}

	result.Success(ctx, statuses)
}

func (c *WindowsExporterController) GetExporterInstallLog(ctx *gin.Context) {
	deviceID, err := parseExporterDeviceID(ctx.Param("deviceId"))
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, "设备ID格式错误")
		return
	}

	data, err := c.exporterService.GetExporterInstallLog(deviceID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrWindowsExporterDeviceNotFound):
			result.Failed(ctx, http.StatusNotFound, "设备不存在")
		case errors.Is(err, service.ErrWindowsExporterRemoteCredsRequired):
			result.Failed(ctx, http.StatusBadRequest, "缺少远程账号或密码")
		default:
			result.Failed(ctx, http.StatusInternalServerError, err.Error())
		}
		return
	}

	result.Success(ctx, data)
}

func (c *WindowsExporterController) DownloadInitWinRMScript(ctx *gin.Context) {
	scriptPath, filename, err := c.exporterService.GetInitWinRMScriptPath()
	if err != nil {
		result.Failed(ctx, http.StatusNotFound, err.Error())
		return
	}

	ctx.FileAttachment(scriptPath, filename)
}

type BatchInstallExporterRequest struct {
	DeviceIDs    []uint `json:"deviceIds" binding:"required,min=1"`
	InstallerURL string `json:"installerUrl"`
	Version      string `json:"version"`
	ExporterPort int    `json:"exporterPort"`
}

func parseExporterDeviceID(value string) (uint, error) {
	deviceID, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(deviceID), nil
}
