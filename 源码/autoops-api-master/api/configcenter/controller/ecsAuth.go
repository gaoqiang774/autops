package controller

import (
	"dodevops-api/api/configcenter/model"
	"dodevops-api/api/configcenter/service"
	"dodevops-api/common/result"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

type EcsAuthController struct {
	service service.EcsAuthServiceInterface
}

// 分页参数
type PageParams struct {
	Page     int `form:"page" binding:"required,min=1"`
	PageSize int `form:"pageSize" binding:"required,min=1,max=1000"`
}

func NewEcsAuthController() *EcsAuthController {
	return &EcsAuthController{
		service: service.GetEcsAuthService(),
	}
}

// GetEcsAuthList 获取所有凭据（分页）
// @Summary 获取所有凭据（分页）
// @Description 获取所有凭据（分页）
// @Tags Config配置中心
// @Param page query int true "页码"
// @Param pageSize query int true "每页数量"
// @Success 200 {object} result.Result{data=result.PageResult{list=[]model.EcsAuthVo}}
// @Router /api/v1/config/ecsauthlist [get]
// @Security ApiKeyAuth
func (c *EcsAuthController) GetEcsAuthList(ctx *gin.Context) {
	var params PageParams
	if err := ctx.ShouldBindQuery(&params); err != nil {
		result.Failed(ctx, int(result.ApiCode.FAILED), err.Error())
		return
	}
	c.service.GetEcsAuthListWithPage(ctx, params.Page, params.PageSize)
}

// CreateEcsAuth 创建凭据
// @Summary 创建凭据
// @Description 创建凭据
// @Param data body model.CreateEcsPasswordAuthDto true "凭据信息"
// @Success 200 {object} result.Result
// @Router /api/v1/config/ecsauthadd [post]
// @Tags Config配置中心
// @Security ApiKeyAuth
func (c *EcsAuthController) CreateEcsAuth(ctx *gin.Context) {
	var dto model.CreateEcsPasswordAuthDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, int(result.ApiCode.FAILED), err.Error())
		return
	}
	c.service.CreateEcsAuth(ctx, &dto)
}

// DownloadEcsAuthTemplate 下载ECS密码凭据导入模板
// @Summary 下载ECS密码凭据导入模板
// @Description 下载xlsx模板，列顺序：name、端口、用户名、密码
// @Tags Config配置中心
// @Produce octet-stream
// @Success 200 {file} file
// @Router /api/v1/config/ecsauthtemplate [get]
// @Security ApiKeyAuth
func (c *EcsAuthController) DownloadEcsAuthTemplate(ctx *gin.Context) {
	file := excelize.NewFile()
	defer file.Close()

	sheetName := file.GetSheetName(0)
	headers := []string{"name", "端口", "用户名", "密码"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = file.SetCellValue(sheetName, cell, header)
	}

	// 写入一行示例数据，便于用户理解模板格式
	example := []string{"172.16.100.132", "22", "root", "w123456"}
	for i, value := range example {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		_ = file.SetCellValue(sheetName, cell, value)
	}

	buffer, err := file.WriteToBuffer()
	if err != nil {
		result.Failed(ctx, int(result.ApiCode.FAILED), "生成Excel模板失败")
		return
	}

	ctx.Header("Content-Description", "File Transfer")
	ctx.Header("Content-Transfer-Encoding", "binary")
	ctx.Header("Content-Disposition", "attachment; filename=ecs_auth_template.xlsx")
	ctx.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	ctx.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

// ImportEcsPasswordAuth 批量导入密码凭据
// @Summary 批量导入密码凭据
// @Description 支持两种导入方式：1) JSON文本 tableData；2) multipart/form-data 上传xlsx文件(file)
// @Param data body model.ImportEcsPasswordAuthDto true "导入数据"
// @Success 200 {object} result.Result
// @Router /api/v1/config/ecsauthimport [post]
// @Tags Config配置中心
// @Security ApiKeyAuth
func (c *EcsAuthController) ImportEcsPasswordAuth(ctx *gin.Context) {
	contentType := strings.ToLower(ctx.ContentType())
	if strings.HasPrefix(contentType, "multipart/form-data") {
		c.importEcsPasswordAuthFromExcel(ctx)
		return
	}

	var dto model.ImportEcsPasswordAuthDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, int(result.ApiCode.FAILED), err.Error())
		return
	}
	c.service.ImportEcsPasswordAuth(ctx, &dto)
}

func (c *EcsAuthController) importEcsPasswordAuthFromExcel(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		result.Failed(ctx, int(result.ApiCode.FAILED), "请上传xlsx文件")
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".xlsx" {
		result.Failed(ctx, int(result.ApiCode.FAILED), "仅支持 .xlsx 文件")
		return
	}

	tempDir := "./upload/temp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		result.Failed(ctx, int(result.ApiCode.FAILED), "创建临时目录失败: "+err.Error())
		return
	}

	timestamp := time.Now().UnixNano()
	tempFilePath := filepath.Join(tempDir, fmt.Sprintf("ecsauth_import_%d%s", timestamp, ext))
	if err := ctx.SaveUploadedFile(file, tempFilePath); err != nil {
		result.Failed(ctx, int(result.ApiCode.FAILED), "保存上传文件失败: "+err.Error())
		return
	}
	defer os.Remove(tempFilePath)

	excelFile, err := excelize.OpenFile(tempFilePath)
	if err != nil {
		result.Failed(ctx, int(result.ApiCode.FAILED), "打开Excel文件失败")
		return
	}
	defer excelFile.Close()

	sheetName := excelFile.GetSheetName(0)
	if sheetName == "" {
		sheetList := excelFile.GetSheetList()
		if len(sheetList) == 0 {
			result.Failed(ctx, int(result.ApiCode.FAILED), "Excel中没有可读取的工作表")
			return
		}
		sheetName = sheetList[0]
	}

	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		result.Failed(ctx, int(result.ApiCode.FAILED), "读取Excel数据失败")
		return
	}

	tableData, err := buildEcsAuthImportTableData(rows)
	if err != nil {
		result.Failed(ctx, int(result.ApiCode.FAILED), err.Error())
		return
	}

	dto := model.ImportEcsPasswordAuthDto{
		TableData: tableData,
		Remark:    strings.TrimSpace(ctx.PostForm("remark")),
	}
	c.service.ImportEcsPasswordAuth(ctx, &dto)
}

func buildEcsAuthImportTableData(rows [][]string) (string, error) {
	if len(rows) == 0 {
		return "", fmt.Errorf("Excel中没有可导入的数据")
	}

	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		if isExcelRowEmpty(row) {
			continue
		}

		name := getExcelCell(row, 0)
		port := getExcelCell(row, 1)
		username := getExcelCell(row, 2)
		password := getExcelCell(row, 3)

		// 使用逗号分隔可尽量保留密码中的空格
		lines = append(lines, strings.Join([]string{name, port, username, password}, ","))
	}

	if len(lines) == 0 {
		return "", fmt.Errorf("Excel中没有可导入的数据")
	}

	return strings.Join(lines, "\n"), nil
}

func isExcelRowEmpty(row []string) bool {
	for _, item := range row {
		if strings.TrimSpace(item) != "" {
			return false
		}
	}
	return true
}

func getExcelCell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// GetEcsAuthByName 根据名称获取凭据
// @Summary 根据名称获取凭据
// @Description 根据名称获取凭据
// @Tags Config配置中心
// @Param name query string true "凭据名称"
// @Success 200 {object} result.Result{data=model.EcsAuthVo}
// @Router /api/v1/config/ecsauthinfo [get]
// @Security ApiKeyAuth
func (c *EcsAuthController) GetEcsAuthByName(ctx *gin.Context) {
	name := ctx.Query("name")
	if name == "" {
		result.Failed(ctx, int(result.ApiCode.FAILED), "name参数不能为空")
		return
	}
	c.service.GetEcsAuthByName(ctx, name)
}

// UpdateEcsAuth 更新凭据
// @Summary 更新凭据
// @Description 更新凭据
// @Tags Config配置中心
// @Param data body model.UpdateEcsAuthDto true "凭据信息"
// @Success 200 {object} result.Result
// @Router /api/v1/config/ecsauthupdate [put]
// @Security ApiKeyAuth
func (c *EcsAuthController) UpdateEcsAuth(ctx *gin.Context) {
	var dto model.UpdateEcsAuthDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, int(result.ApiCode.FAILED), err.Error())
		return
	}
	c.service.UpdateEcsAuth(ctx, dto.Id, &dto.CreateEcsPasswordAuthDto)
}

// DecryptEcsAuth 解密凭据敏感字段
// @Summary 解密凭据敏感字段
// @Description 根据ID解密返回凭据中的密码和私钥字段
// @Tags Config配置中心
// @Param id query uint true "凭据ID"
// @Success 200 {object} result.Result{data=model.EcsAuthSecretVo}
// @Router /api/v1/config/ecsauth/decrypt [post]
// @Security ApiKeyAuth
func (c *EcsAuthController) DecryptEcsAuth(ctx *gin.Context) {
	var req struct {
		ID uint `form:"id" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	c.service.DecryptEcsAuth(ctx, req.ID)
}

// MigrateEcsAuthSecrets 迁移历史明文凭据
// @Summary 迁移历史明文凭据
// @Description 将 config_ecsauth 表中的历史明文敏感字段批量加密
// @Tags Config配置中心
// @Success 200 {object} result.Result
// @Router /api/v1/config/ecsauthmigrate [post]
// @Security ApiKeyAuth
func (c *EcsAuthController) MigrateEcsAuthSecrets(ctx *gin.Context) {
	c.service.MigrateEcsAuthSecrets(ctx)
}

// DeleteEcsAuth 删除凭据
// @Summary 删除凭据
// @Description 删除凭据
// @Tags Config配置中心
// @Param data body model.EcsAuthIdDto true "凭据ID"
// @Success 200 {object} result.Result
// @Router /api/v1/config/ecsauthdelete [delete]
// @Security ApiKeyAuth
func (c *EcsAuthController) DeleteEcsAuth(ctx *gin.Context) {
	var idDto model.EcsAuthIdDto
	if err := ctx.ShouldBindJSON(&idDto); err != nil {
		result.Failed(ctx, int(result.ApiCode.FAILED), err.Error())
		return
	}
	c.service.DeleteEcsAuth(ctx, idDto.Id)
}

// GetEcsAuthById 根据ID获取凭据详情
// @Summary 根据ID获取凭据详情
// @Description 根据ID获取凭据详情
// @Tags Config配置中心
// @Param id query int true "凭据ID"
// @Success 200 {object} result.Result{data=model.EcsAuthVo}
// @Router /api/v1/config/ecsauthdetail [get]
// @Security ApiKeyAuth
func (c *EcsAuthController) GetEcsAuthById(ctx *gin.Context) {
	idStr := ctx.Query("id")
	if idStr == "" {
		result.Failed(ctx, int(result.ApiCode.FAILED), "id参数不能为空")
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		result.Failed(ctx, int(result.ApiCode.FAILED), "id参数格式错误")
		return
	}
	c.service.GetEcsAuthById(ctx, uint(id))
}
