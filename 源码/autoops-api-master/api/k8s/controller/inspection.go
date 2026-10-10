package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"dodevops-api/api/k8s/model"
	"dodevops-api/api/k8s/service"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type InspectionController struct {
	inspectionService *service.InspectionService
}

func NewInspectionController(db *gorm.DB) *InspectionController {
	return &InspectionController{
		inspectionService: service.NewInspectionService(db),
	}
}

type startInspectionPayload struct {
	ClusterIDs []uint                  `json:"clusterIds" binding:"required"`
	Options    model.InspectionOptions `json:"options"`
}

func (ctrl *InspectionController) StartInspection(c *gin.Context) {
	var payload startInspectionPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		inspectionFailed(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	var userID uint
	if admin, err := jwt.GetAdmin(c); err == nil && admin != nil {
		userID = admin.ID
	}

	inspection, err := ctrl.inspectionService.StartInspection(c.Request.Context(), &model.StartInspectionRequest{
		ClusterIDs: payload.ClusterIDs,
		Options:    payload.Options,
		UserID:     userID,
	})
	if err != nil {
		inspectionFailed(c, http.StatusInternalServerError, "启动巡检失败: "+err.Error())
		return
	}

	inspectionSuccess(c, gin.H{"inspectionId": inspection.ID})
}

func (ctrl *InspectionController) GetInspectionProgress(c *gin.Context) {
	inspectionID, ok := parseInspectionID(c)
	if !ok {
		return
	}

	progress, err := ctrl.inspectionService.GetInspectionProgress(c.Request.Context(), inspectionID)
	if err != nil {
		inspectionFailed(c, http.StatusInternalServerError, "获取进度失败: "+err.Error())
		return
	}
	inspectionSuccess(c, progress)
}

func (ctrl *InspectionController) GetInspectionResult(c *gin.Context) {
	inspectionID, ok := parseInspectionID(c)
	if !ok {
		return
	}

	inspection, result, err := ctrl.inspectionService.GetInspectionResult(c.Request.Context(), inspectionID)
	if err != nil {
		inspectionFailed(c, http.StatusInternalServerError, "获取结果失败: "+err.Error())
		return
	}
	inspectionSuccess(c, gin.H{
		"inspection": inspection,
		"result":     result,
	})
}

func (ctrl *InspectionController) GetInspectionHistory(c *gin.Context) {
	clusterID64, _ := strconv.ParseUint(c.Query("clusterId"), 10, 32)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", c.DefaultQuery("size", "10")))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	items, total, err := ctrl.inspectionService.GetInspectionHistory(c.Request.Context(), uint(clusterID64), page, pageSize)
	if err != nil {
		inspectionFailed(c, http.StatusInternalServerError, "获取历史失败: "+err.Error())
		return
	}
	inspectionSuccess(c, gin.H{
		"total":    total,
		"list":     items,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (ctrl *InspectionController) DeleteInspection(c *gin.Context) {
	inspectionID, ok := parseInspectionID(c)
	if !ok {
		return
	}
	if err := ctrl.inspectionService.DeleteInspection(c.Request.Context(), inspectionID); err != nil {
		inspectionFailed(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}
	inspectionSuccess(c, "删除成功")
}

func (ctrl *InspectionController) ExportInspection(c *gin.Context) {
	inspectionID, ok := parseInspectionID(c)
	if !ok {
		return
	}
	format := c.DefaultQuery("format", "excel")
	if format != "excel" {
		inspectionFailed(c, http.StatusBadRequest, "不支持的导出格式")
		return
	}

	inspection, result, err := ctrl.inspectionService.GetInspectionResult(c.Request.Context(), inspectionID)
	if err != nil {
		inspectionFailed(c, http.StatusInternalServerError, "获取结果失败: "+err.Error())
		return
	}
	if result == nil {
		inspectionFailed(c, http.StatusNotFound, "巡检结果不存在")
		return
	}

	buffer, err := buildInspectionExcel(inspection, result)
	if err != nil {
		inspectionFailed(c, http.StatusInternalServerError, "生成 Excel 失败: "+err.Error())
		return
	}

	filename := fmt.Sprintf("cluster-inspection-%s-%s.xlsx", inspection.ClusterName, time.Now().Format("20060102150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func parseInspectionID(c *gin.Context) (uint, bool) {
	value := c.Param("inspectionId")
	if value == "" {
		value = c.Param("id")
	}
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		inspectionFailed(c, http.StatusBadRequest, "无效的巡检ID")
		return 0, false
	}
	return uint(id), true
}

func inspectionSuccess(c *gin.Context, data interface{}) {
	if data == nil {
		data = gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

func inspectionFailed(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, gin.H{
		"code":    code,
		"message": message,
		"data":    gin.H{},
	})
}

func buildInspectionExcel(inspection *model.ClusterInspection, result *model.InspectionResult) (*bytes.Buffer, error) {
	file := excelize.NewFile()
	defer file.Close()

	titleStyle, _ := file.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 16, Color: "#FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#1F2937"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	headerStyle, _ := file.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#374151"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	successStyle, _ := file.NewStyle(&excelize.Style{Font: &excelize.Font{Color: "#16A34A"}, Alignment: &excelize.Alignment{Horizontal: "center"}})
	warningStyle, _ := file.NewStyle(&excelize.Style{Font: &excelize.Font{Color: "#D97706"}, Alignment: &excelize.Alignment{Horizontal: "center"}})
	errorStyle, _ := file.NewStyle(&excelize.Style{Font: &excelize.Font{Color: "#DC2626"}, Alignment: &excelize.Alignment{Horizontal: "center"}})

	sheet := "巡检概览"
	file.SetSheetName("Sheet1", sheet)
	file.MergeCell(sheet, "A1", "F1")
	file.SetCellValue(sheet, "A1", "集群巡检报告")
	file.SetCellStyle(sheet, "A1", "F1", titleStyle)
	file.SetRowHeight(sheet, 1, 34)

	file.SetCellValue(sheet, "A3", "集群名称")
	file.SetCellValue(sheet, "B3", inspection.ClusterName)
	file.SetCellValue(sheet, "A4", "健康评分")
	file.SetCellValue(sheet, "B4", fmt.Sprintf("%d/100", inspection.Score))
	file.SetCellValue(sheet, "A5", "巡检时间")
	file.SetCellValue(sheet, "B5", inspection.CreatedAt.Format("2006-01-02 15:04:05"))
	file.SetCellValue(sheet, "A6", "巡检耗时")
	file.SetCellValue(sheet, "B6", fmt.Sprintf("%d 秒", inspection.Duration))
	file.SetCellValue(sheet, "D3", "检查项总数")
	file.SetCellValue(sheet, "E3", inspection.CheckCount)
	file.SetCellValue(sheet, "D4", "通过项")
	file.SetCellValue(sheet, "E4", inspection.PassCount)
	file.SetCellValue(sheet, "D5", "告警项")
	file.SetCellValue(sheet, "E5", inspection.WarningCount)
	file.SetCellValue(sheet, "D6", "失败项")
	file.SetCellValue(sheet, "E6", inspection.FailCount)

	headers := []string{"类别", "检查项", "状态", "检查值", "详情", "建议"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 8)
		file.SetCellValue(sheet, cell, header)
	}
	file.SetCellStyle(sheet, "A8", "F8", headerStyle)

	row := 9
	for _, item := range inspectionItemsForExport(result) {
		file.SetCellValue(sheet, fmt.Sprintf("A%d", row), item.Category)
		file.SetCellValue(sheet, fmt.Sprintf("B%d", row), item.Name)
		statusCell := fmt.Sprintf("C%d", row)
		switch item.Status {
		case model.CheckStatusSuccess:
			file.SetCellValue(sheet, statusCell, "正常")
			file.SetCellStyle(sheet, statusCell, statusCell, successStyle)
		case model.CheckStatusWarning:
			file.SetCellValue(sheet, statusCell, "告警")
			file.SetCellStyle(sheet, statusCell, statusCell, warningStyle)
		case model.CheckStatusError:
			file.SetCellValue(sheet, statusCell, "异常")
			file.SetCellStyle(sheet, statusCell, statusCell, errorStyle)
		default:
			file.SetCellValue(sheet, statusCell, item.Status)
		}
		file.SetCellValue(sheet, fmt.Sprintf("D%d", row), item.Value)
		file.SetCellValue(sheet, fmt.Sprintf("E%d", row), item.Detail)
		file.SetCellValue(sheet, fmt.Sprintf("F%d", row), item.Suggestion)
		row++
	}

	file.SetColWidth(sheet, "A", "A", 16)
	file.SetColWidth(sheet, "B", "B", 24)
	file.SetColWidth(sheet, "C", "C", 10)
	file.SetColWidth(sheet, "D", "D", 22)
	file.SetColWidth(sheet, "E", "F", 42)

	addNodeSheet(file, headerStyle, successStyle, errorStyle, result)
	addWorkloadSheet(file, headerStyle, result)
	addEventSheet(file, headerStyle, result)

	buffer := &bytes.Buffer{}
	if err := file.Write(buffer); err != nil {
		return nil, err
	}
	return buffer, nil
}

func inspectionItemsForExport(result *model.InspectionResult) []model.CheckItem {
	items := []model.CheckItem{}
	items = append(items, result.ClusterInfo.Items...)
	items = append(items, result.NodeHealth.Items...)
	items = append(items, result.Components.Items...)
	items = append(items, result.Workloads.Items...)
	items = append(items, result.Network.Items...)
	items = append(items, result.Storage.Items...)
	items = append(items, result.Security.Items...)
	items = append(items, result.Config.Items...)
	items = append(items, result.Capacity.Items...)
	items = append(items, result.Events.Items...)
	return items
}

func addNodeSheet(file *excelize.File, headerStyle, successStyle, errorStyle int, result *model.InspectionResult) {
	if len(result.NodeHealth.NodeUtilization) == 0 {
		return
	}
	sheet := "节点健康"
	file.NewSheet(sheet)
	headers := []string{"节点名称", "状态", "CPU容量", "内存容量", "Pod数量", "Pod容量"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		file.SetCellValue(sheet, cell, header)
	}
	file.SetCellStyle(sheet, "A1", "F1", headerStyle)
	for i, node := range result.NodeHealth.NodeUtilization {
		row := i + 2
		file.SetCellValue(sheet, fmt.Sprintf("A%d", row), node.Name)
		statusCell := fmt.Sprintf("B%d", row)
		file.SetCellValue(sheet, statusCell, node.Status)
		if node.Status == "Ready" {
			file.SetCellStyle(sheet, statusCell, statusCell, successStyle)
		} else {
			file.SetCellStyle(sheet, statusCell, statusCell, errorStyle)
		}
		file.SetCellValue(sheet, fmt.Sprintf("C%d", row), node.CPUCapacity)
		file.SetCellValue(sheet, fmt.Sprintf("D%d", row), node.MemoryCapacity)
		file.SetCellValue(sheet, fmt.Sprintf("E%d", row), node.PodCount)
		file.SetCellValue(sheet, fmt.Sprintf("F%d", row), node.PodCapacity)
	}
	file.SetColWidth(sheet, "A", "F", 18)
}

func addWorkloadSheet(file *excelize.File, headerStyle int, result *model.InspectionResult) {
	if len(result.Workloads.UnhealthyWorkloads) == 0 {
		return
	}
	sheet := "异常工作负载"
	file.NewSheet(sheet)
	headers := []string{"类型", "命名空间", "名称", "就绪状态", "原因"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		file.SetCellValue(sheet, cell, header)
	}
	file.SetCellStyle(sheet, "A1", "E1", headerStyle)
	for i, item := range result.Workloads.UnhealthyWorkloads {
		row := i + 2
		file.SetCellValue(sheet, fmt.Sprintf("A%d", row), item.Kind)
		file.SetCellValue(sheet, fmt.Sprintf("B%d", row), item.Namespace)
		file.SetCellValue(sheet, fmt.Sprintf("C%d", row), item.Name)
		file.SetCellValue(sheet, fmt.Sprintf("D%d", row), item.Ready)
		file.SetCellValue(sheet, fmt.Sprintf("E%d", row), item.Reason)
	}
	file.SetColWidth(sheet, "A", "E", 22)
}

func addEventSheet(file *excelize.File, headerStyle int, result *model.InspectionResult) {
	if len(result.Events.RecentEvents) == 0 {
		return
	}
	sheet := "最近事件"
	file.NewSheet(sheet)
	headers := []string{"类型", "原因", "对象", "命名空间", "次数", "最后发生", "消息"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		file.SetCellValue(sheet, cell, header)
	}
	file.SetCellStyle(sheet, "A1", "G1", headerStyle)
	for i, event := range result.Events.RecentEvents {
		row := i + 2
		file.SetCellValue(sheet, fmt.Sprintf("A%d", row), event.Type)
		file.SetCellValue(sheet, fmt.Sprintf("B%d", row), event.Reason)
		file.SetCellValue(sheet, fmt.Sprintf("C%d", row), event.Object)
		file.SetCellValue(sheet, fmt.Sprintf("D%d", row), event.Namespace)
		file.SetCellValue(sheet, fmt.Sprintf("E%d", row), event.Count)
		file.SetCellValue(sheet, fmt.Sprintf("F%d", row), event.LastSeen)
		file.SetCellValue(sheet, fmt.Sprintf("G%d", row), event.Message)
	}
	file.SetColWidth(sheet, "A", "F", 18)
	file.SetColWidth(sheet, "G", "G", 56)
}
