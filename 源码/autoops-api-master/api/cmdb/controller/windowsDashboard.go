package controller

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

// GetWindowsDashboardMetrics 获取 Windows 监控大盘数据
// @Summary 获取 Windows 监控大盘数据
// @Description 获取 Windows 主机核心概览、趋势图、磁盘分卷、Top 进程和关键服务状态
// @Tags 监控
// @Accept json
// @Produce json
// @Param id path uint true "主机ID"
// @Param start query string false "开始时间(格式: 2006-01-02 15:04:05)"
// @Param end query string false "结束时间(格式: 2006-01-02 15:04:05)"
// @Param duration query string false "时间范围(30m/1h/3h/6h/12h/24h)"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hosts/{id}/windows-dashboard [get]
// @Security ApiKeyAuth
func (c *MonitorController) GetWindowsDashboardMetrics(ctx *gin.Context) {
	hostIDStr := ctx.Param("id")
	hostID, err := strconv.ParseUint(hostIDStr, 10, 32)
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, "无效的主机ID")
		return
	}

	startStr, endStr, err := resolveMonitorTimeRange(ctx)
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}

	c.monitorService.GetWindowsDashboardMetrics(ctx, uint(hostID), startStr, endStr, "")
}

func resolveMonitorTimeRange(ctx *gin.Context) (string, string, error) {
	startStr := ctx.Query("start")
	endStr := ctx.Query("end")
	duration := ctx.Query("duration")

	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(loc)
	userTimeLayout := "2006-01-02 15:04:05"

	if duration != "" {
		validDurations := map[string]time.Duration{
			"30m": 30 * time.Minute,
			"1h":  1 * time.Hour,
			"3h":  3 * time.Hour,
			"6h":  6 * time.Hour,
			"12h": 12 * time.Hour,
			"24h": 24 * time.Hour,
		}

		durationValue, ok := validDurations[duration]
		if !ok {
			return "", "", errors.New("无效的时间范围参数，请使用 30m, 1h, 3h, 6h, 12h 或 24h")
		}

		endTime := now
		startTime := endTime.Add(-durationValue)
		return startTime.Format(userTimeLayout), endTime.Format(userTimeLayout), nil
	}

	if startStr == "" || endStr == "" {
		return "", "", errors.New("请提供开始时间和结束时间参数，或时间范围参数(duration)")
	}

	var err error
	startStr, err = url.QueryUnescape(startStr)
	if err != nil {
		return "", "", errors.New("开始时间参数解码失败")
	}
	endStr, err = url.QueryUnescape(endStr)
	if err != nil {
		return "", "", errors.New("结束时间参数解码失败")
	}

	startStr = strings.TrimSpace(strings.ReplaceAll(startStr, "+", " "))
	endStr = strings.TrimSpace(strings.ReplaceAll(endStr, "+", " "))

	if _, err := time.ParseInLocation(userTimeLayout, startStr, loc); err != nil {
		return "", "", errors.New("无效的开始时间格式，请使用 2006-01-02 15:04:05")
	}
	if _, err := time.ParseInLocation(userTimeLayout, endStr, loc); err != nil {
		return "", "", errors.New("无效的结束时间格式，请使用 2006-01-02 15:04:05")
	}

	return startStr, endStr, nil
}
