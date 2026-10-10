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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

func DownloadCmdbIdcVmTemplate(c *gin.Context) {
	file := excelize.NewFile()
	defer file.Close()

	sheetName := file.GetSheetName(0)
	headers := []string{"物理机IP", "虚拟机IP", "用户名", "虚拟机名称", "备注"}
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
	c.Header("Content-Disposition", "attachment; filename=idc_vm_template.xlsx")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func CreateCmdbIdcVm(c *gin.Context) {
	var dto model.CreateCmdbIdcVmDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	service.GetCmdbIdcVmService().CreateVm(c, &dto)
}

func UpdateCmdbIdcVm(c *gin.Context) {
	var dto model.UpdateCmdbIdcVmDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	service.GetCmdbIdcVmService().UpdateVm(c, &dto)
}

func DeleteCmdbIdcVm(c *gin.Context) {
	var dto model.CmdbIdcVmIdDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	service.GetCmdbIdcVmService().DeleteVm(c, dto.ID)
}

func GetCmdbIdcVmById(c *gin.Context) {
	id := util.StringToUint(c.Query("id"))
	if id == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "虚拟机关联ID不能为空")
		return
	}
	service.GetCmdbIdcVmService().GetVmById(c, id)
}

func GetCmdbIdcVmList(c *gin.Context) {
	var query model.CmdbIdcVmQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "分页参数错误")
		return
	}
	service.GetCmdbIdcVmService().GetVmList(c, &query)
}

func ImportCmdbIdcVmsFromExcel(c *gin.Context) {
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
	tempFilePath := filepath.Join(tempDir, fmt.Sprintf("idc_vm_import_%d%s", timestamp, ext))

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

	vms, err := parseIdcVmRows(rows)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}

	service.GetCmdbIdcVmService().ImportVmsFromExcel(c, vms)
}

func parseIdcVmRows(rows [][]string) ([]model.ExcelCmdbIdcVmRow, error) {
	if len(rows) < 2 {
		return nil, fmt.Errorf("Excel中没有可导入的数据")
	}

	headerIndex := make(map[string]int, len(rows[0]))
	for idx, header := range rows[0] {
		headerIndex[normalizeExcelHeader(header)] = idx
	}

	physicalIPIdx, ok := findHeaderIndex(headerIndex, "物理机IP", "物理机ip", "宿主机IP", "宿主机ip", "宿主IP", "宿主ip")
	if !ok {
		return nil, fmt.Errorf("缺少物理机IP列")
	}
	vmIPIdx, ok := findHeaderIndex(headerIndex, "虚拟机IP", "虚拟机ip")
	if !ok {
		return nil, fmt.Errorf("缺少虚拟机IP列")
	}
	usernameIdx, _ := findHeaderIndex(headerIndex, "用户名", "登录用户名")
	vmNameIdx, _ := findHeaderIndex(headerIndex, "虚拟机名称", "虚拟机名", "主机名")
	remarkIdx, _ := findHeaderIndex(headerIndex, "备注")

	resultRows := make([]model.ExcelCmdbIdcVmRow, 0, len(rows)-1)
	lastPhysicalIP := ""
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if isExcelRowEmpty(row) {
			continue
		}

		physicalIP := getExcelCell(row, physicalIPIdx)
		if physicalIP == "" {
			physicalIP = lastPhysicalIP
		} else {
			lastPhysicalIP = physicalIP
		}

		vmRow := model.ExcelCmdbIdcVmRow{
			PhysicalIP: physicalIP,
			VmIP:       getExcelCell(row, vmIPIdx),
			Username:   getExcelCell(row, usernameIdx),
			VmName:     getExcelCell(row, vmNameIdx),
			Remark:     getExcelCell(row, remarkIdx),
		}

		if vmRow.PhysicalIP == "" && vmRow.VmIP == "" {
			continue
		}
		resultRows = append(resultRows, vmRow)
	}

	return resultRows, nil
}
