package controller

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/api/cmdb/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/common/util"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

var excelCabinetSlotPattern = regexp.MustCompile(`^(.*?[柜架])\s*[-－—_]\s*(\d+)\s*$`)

// DownloadCmdbIdcAssetTemplate 下载机房资产导入模板
func DownloadCmdbIdcAssetTemplate(c *gin.Context) {
	file := excelize.NewFile()
	defer file.Close()

	sheetName := file.GetSheetName(0)
	headers := []string{
		"机房", "机柜", "机柜位置", "IP", "品牌", "型号", "CPU", "硬盘", "内存", "用途",
		"远程管理卡", "备注", "服务器购买时间", "维保开始时间", "维保结束时间",
		"使用时长(年)", "资产评估",
	}

	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = file.SetCellValue(sheetName, cell, header)
	}

	buffer, err := file.WriteToBuffer()
	if err != nil {
		result.Failed(c, constant.FILE_OPERATION_ERROR, "生成Excel模板失败")
		return
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename=idc_asset_template.xlsx")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

// CreateCmdbIdcAsset 创建机房物理机资产
func CreateCmdbIdcAsset(c *gin.Context) {
	var dto model.CreateCmdbIdcAssetDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	service.GetCmdbIdcAssetService().CreateAsset(c, &dto)
}

// UpdateCmdbIdcAsset 更新机房物理机资产
func UpdateCmdbIdcAsset(c *gin.Context) {
	var dto model.UpdateCmdbIdcAssetDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	service.GetCmdbIdcAssetService().UpdateAsset(c, &dto)
}

// UpdateCmdbIdcAssetPosition 手动调整物理机机柜位置/U高度
func UpdateCmdbIdcAssetPosition(c *gin.Context) {
	var dto model.UpdateCmdbIdcAssetPositionDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	service.GetCmdbIdcAssetService().UpdateAssetPosition(c, &dto)
}

// DeleteCmdbIdcAsset 删除机房物理机资产
func DeleteCmdbIdcAsset(c *gin.Context) {
	var dto model.CmdbIdcAssetIdDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	service.GetCmdbIdcAssetService().DeleteAsset(c, dto.ID)
}

// GetCmdbIdcAssetById 获取单个机房物理机资产详情
func GetCmdbIdcAssetById(c *gin.Context) {
	id := util.StringToUint(c.Query("id"))
	if id == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "资产ID不能为空")
		return
	}
	service.GetCmdbIdcAssetService().GetAssetById(c, id)
}

// GetCmdbIdcAssetList 获取机房物理机资产列表
func GetCmdbIdcAssetList(c *gin.Context) {
	var query model.CmdbIdcAssetQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "分页参数错误")
		return
	}
	service.GetCmdbIdcAssetService().GetAssetList(c, &query)
}

// GetCmdbIdcAssetHardwareMonitor 获取物理机硬件监控信息
func GetCmdbIdcAssetHardwareMonitor(c *gin.Context) {
	var query model.CmdbIdcAssetHardwareMonitorQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	service.NewCmdbIdcAssetHardwareMonitorService().GetAssetHardwareMonitorBySelector(c, query.ID, query.RemoteManagementCard, query.LookbackMinutes, query.TrendHours, query.TrendStepMinutes)
}

// ImportCmdbIdcAssetsFromExcel Excel导入机房物理机资产
func ImportCmdbIdcAssetsFromExcel(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "请上传Excel文件")
		return
	}

	tempDir := "./upload/temp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		result.Failed(c, constant.FILE_OPERATION_ERROR, "创建临时目录失败: "+err.Error())
		return
	}

	timestamp := time.Now().UnixNano()
	ext := filepath.Ext(file.Filename)
	tempFilePath := filepath.Join(tempDir, fmt.Sprintf("idc_asset_import_%d%s", timestamp, ext))

	if err := c.SaveUploadedFile(file, tempFilePath); err != nil {
		result.Failed(c, constant.FILE_OPERATION_ERROR, "保存上传文件失败: "+err.Error())
		return
	}
	defer os.Remove(tempFilePath)

	excelFile, err := excelize.OpenFile(tempFilePath)
	if err != nil {
		result.Failed(c, constant.FILE_OPERATION_ERROR, "打开Excel文件失败")
		return
	}
	defer excelFile.Close()

	sheetName := excelFile.GetSheetName(0)
	if sheetName == "" {
		sheetList := excelFile.GetSheetList()
		if len(sheetList) == 0 {
			result.Failed(c, constant.FILE_OPERATION_ERROR, "Excel中没有可读取的工作表")
			return
		}
		sheetName = sheetList[0]
	}

	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		result.Failed(c, constant.FILE_OPERATION_ERROR, "读取Excel数据失败")
		return
	}

	assets, err := parseIdcAssetRows(rows)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}

	service.GetCmdbIdcAssetService().ImportAssetsFromExcel(c, assets)
}

func parseIdcAssetRows(rows [][]string) ([]model.ExcelCmdbIdcAssetRow, error) {
	if len(rows) < 2 {
		return nil, fmt.Errorf("Excel中没有可导入的数据")
	}

	headerIndex := make(map[string]int, len(rows[0]))
	for idx, header := range rows[0] {
		headerIndex[normalizeExcelHeader(header)] = idx
	}

	roomIdx, ok := findHeaderIndex(headerIndex, "机房")
	if !ok {
		return nil, fmt.Errorf("缺少机房列")
	}
	ipIdx, ok := findHeaderIndex(headerIndex, "IP", "ip")
	if !ok {
		return nil, fmt.Errorf("缺少IP列")
	}

	cabinetIdx, _ := findHeaderIndex(headerIndex, "机柜", "机架")
	cabinetPositionIdx, _ := findHeaderIndex(headerIndex, "机柜位置", "机架位置", "机柜内位置", "机架内位置", "位置", "U位", "u位")
	brandIdx, _ := findHeaderIndex(headerIndex, "品牌")
	modelIdx, _ := findHeaderIndex(headerIndex, "型号")
	cpuIdx, _ := findHeaderIndex(headerIndex, "CPU", "cpu")
	diskIdx, _ := findHeaderIndex(headerIndex, "硬盘")
	memoryIdx, _ := findHeaderIndex(headerIndex, "内存")
	purposeIdx, _ := findHeaderIndex(headerIndex, "用途")
	remoteIdx, _ := findHeaderIndex(headerIndex, "远程管理卡", "远程管理卡IP", "远程管理卡ip")
	remarkIdx, _ := findHeaderIndex(headerIndex, "备注")
	purchaseIdx, _ := findHeaderIndex(headerIndex, "服务器购买时间", "购买时间")
	warrantyStartIdx, _ := findHeaderIndex(headerIndex, "维保开始时间")
	warrantyEndIdx, _ := findHeaderIndex(headerIndex, "维保结束时间")
	usageIdx, _ := findHeaderIndex(headerIndex, "使用时长(年)", "使用时长（年）", "使用时长", "使用年限")
	evaluationIdx, _ := findHeaderIndex(headerIndex, "资产评估")

	resultRows := make([]model.ExcelCmdbIdcAssetRow, 0, len(rows)-1)
	lastRoomName := ""
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if isExcelRowEmpty(row) {
			continue
		}

		roomName := getExcelCell(row, roomIdx)
		if roomName == "" {
			roomName = lastRoomName
		} else {
			lastRoomName = roomName
		}

		cabinetName, cabinetPosition := parseImportedCabinetCell(getExcelCell(row, cabinetIdx))
		if parsedPosition := parseCabinetPositionValue(getExcelCell(row, cabinetPositionIdx)); parsedPosition > 0 {
			cabinetPosition = parsedPosition
		}

		assetRow := model.ExcelCmdbIdcAssetRow{
			RoomName:             roomName,
			CabinetName:          cabinetName,
			CabinetPosition:      cabinetPosition,
			IP:                   getExcelCell(row, ipIdx),
			Brand:                getExcelCell(row, brandIdx),
			Model:                getExcelCell(row, modelIdx),
			CPU:                  getExcelCell(row, cpuIdx),
			Disk:                 getExcelCell(row, diskIdx),
			Memory:               getExcelCell(row, memoryIdx),
			Purpose:              getExcelCell(row, purposeIdx),
			RemoteManagementCard: getExcelCell(row, remoteIdx),
			Remark:               getExcelCell(row, remarkIdx),
			PurchaseDate:         getExcelCell(row, purchaseIdx),
			WarrantyStartDate:    getExcelCell(row, warrantyStartIdx),
			WarrantyEndDate:      getExcelCell(row, warrantyEndIdx),
			UsageYears:           getExcelCell(row, usageIdx),
			AssetEvaluation:      getExcelCell(row, evaluationIdx),
		}

		if assetRow.RoomName == "" && assetRow.IP == "" {
			continue
		}
		resultRows = append(resultRows, assetRow)
	}

	return resultRows, nil
}

func findHeaderIndex(headerIndex map[string]int, aliases ...string) (int, bool) {
	for _, alias := range aliases {
		if idx, ok := headerIndex[normalizeExcelHeader(alias)]; ok {
			return idx, true
		}
	}
	return 0, false
}

func normalizeExcelHeader(value string) string {
	replacer := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "", "\u00a0", "", "\u3000", "")
	return strings.ToLower(replacer.Replace(strings.TrimSpace(value)))
}

func getExcelCell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func parseImportedCabinetCell(value string) (string, int) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", 0
	}

	matches := excelCabinetSlotPattern.FindStringSubmatch(trimmed)
	if len(matches) == 3 {
		return strings.TrimSpace(matches[1]), parseCabinetPositionValue(matches[2])
	}

	return trimmed, 0
}

func parseCabinetPositionValue(value string) int {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "-"))
	if trimmed == "" {
		return 0
	}

	position, err := strconv.Atoi(trimmed)
	if err != nil || position <= 0 {
		return 0
	}
	return position
}

func isExcelRowEmpty(row []string) bool {
	for _, item := range row {
		if strings.TrimSpace(item) != "" {
			return false
		}
	}
	return true
}
